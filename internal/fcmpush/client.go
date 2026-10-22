// Package fcmpush sends content-free Android data messages with explicit local credentials.
package fcmpush

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
)

type Message struct {
	FID     string            `json:"fid"`
	Data    map[string]string `json:"data"`
	Android AndroidConfig     `json:"android"`
}
type AndroidConfig struct {
	CollapseKey           string `json:"collapse_key"`
	Priority              string `json:"priority"`
	TTL                   string `json:"ttl"`
	RestrictedPackageName string `json:"restricted_package_name"`
}

// Error exposes only a closed transport code, never Google's response or credentials.
type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return e.Code }

// Client owns one explicit project credential. Load performs no network requests.
type Client struct {
	httpClient  *http.Client
	endpoint    string
	credentials *jwt.Config
	mutex       sync.Mutex
	token       *oauth2.Token
}

// Load reads the credential for project, a hostconfig.AndroidPush.Validate
// project id; the send url is built from it.
func Load(path, project string) (*Client, error) {
	unavailable := errors.New("android_push_credentials_unavailable")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 64<<10 {
		return nil, unavailable
	}
	data, err := os.ReadFile(path)
	var account struct {
		ProjectID   string `json:"project_id"`
		PrivateKey  string `json:"private_key"`
		ClientEmail string `json:"client_email"`
		TokenURI    string `json:"token_uri"`
	}
	if err != nil || json.Unmarshal(data, &account) != nil || account.ProjectID != project || account.PrivateKey == "" || account.ClientEmail == "" || account.TokenURI != "https://oauth2.googleapis.com/token" {
		return nil, unavailable
	}
	// the library refuses any type but service_account; oauth2/jwt parses the key
	// (pem pkcs8 or pkcs1, rsa) at each token fetch.
	credentials, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return nil, unavailable
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{httpClient: client, endpoint: "https://fcm.googleapis.com/v1/projects/" + project + "/messages:send", credentials: credentials}, nil
}

type tokenTransport struct {
	context context.Context
	base    http.RoundTripper
}

func (transport tokenTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport.base.RoundTrip(request.WithContext(transport.context))
}

func (f *Client) accessToken(ctx context.Context) (*oauth2.Token, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.token.Valid() {
		return f.token, nil
	}
	// oauth2/jwt uses PostForm without attaching its context to the request.
	// Bind token HTTP explicitly so rotation and drain cancel it with the send.
	client := *f.httpClient
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = tokenTransport{ctx, transport}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &client)
	value, err := f.credentials.TokenSource(ctx).Token()
	if err == nil {
		f.token = value
	}
	return value, err
}
func (f *Client) Send(ctx context.Context, message Message) error {
	body, err := json.Marshal(struct {
		Message Message `json:"message"`
	}{message})
	if err != nil {
		return &Error{Status: 503, Code: "sender_unavailable"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint, bytes.NewReader(body))
	if err != nil {
		return &Error{Status: 503, Code: "sender_unavailable"}
	}
	token, err := f.accessToken(ctx)
	if err != nil {
		return &Error{Status: 503, Code: "sender_unavailable"}
	}
	token.SetAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	response, err := f.httpClient.Do(req)
	if err != nil {
		return &Error{Status: 503, Code: "sender_unavailable"}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	closeErr := response.Body.Close()
	if err != nil || closeErr != nil || len(data) > 65536 {
		return &Error{Status: 503, Code: "sender_unavailable"}
	}
	if response.StatusCode == 200 {
		return nil
	}
	// Only exact FCM error codes identify an invalid destination; a generic 400 does not.
	var failure struct {
		Error struct {
			Details []struct {
				Type      string `json:"@type"`
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &failure) == nil {
		for _, detail := range failure.Error.Details {
			if detail.Type == "type.googleapis.com/google.firebase.fcm.v1.FcmError" && (detail.ErrorCode == "UNREGISTERED" || detail.ErrorCode == "SENDER_ID_MISMATCH") {
				return &Error{Status: 410, Code: "destination_unavailable"}
			}
		}
	}
	if response.StatusCode == 429 {
		return &Error{Status: 429, Code: "rate_limited"}
	}
	return &Error{Status: 503, Code: "sender_unavailable"}
}
func MessageFor(fid, collapse, priority string, data map[string]string) Message {
	return Message{FID: fid, Data: data, Android: AndroidConfig{CollapseKey: collapse, Priority: priority, TTL: "120s", RestrictedPackageName: "dev.niels.skidbladnir"}}
}
