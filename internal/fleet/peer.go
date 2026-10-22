package fleet

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

const invitationPrefix = "skid://connect/"

// peerClient carries every fleet call between hosts and from the cli. Callers
// bound its context; it never follows a redirect or uses a proxy.
var peerClient = &http.Client{
	Transport: func() *http.Transport {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		return transport
	}(),
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// NewRequest builds one fleet request: json body, exact machine header, and the
// bearer when the target has one.
func NewRequest(ctx context.Context, t Target, method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, t.Origin+path, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Skidbladnir-Machine", t.Machine)
	if t.Bearer != "" {
		request.Header.Set("Authorization", "Bearer "+t.Bearer)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

// Call makes one fleet request. A non-2xx response is an *Error whose Code is
// empty when the body is not a {code,message} envelope; a transport or read
// failure is a plain error; a 2xx body decodes tolerantly into result.
func Call(ctx context.Context, t Target, method, path string, body, result any) error {
	request, err := NewRequest(ctx, t, method, path, body)
	if err != nil {
		return err
	}
	response, err := peerClient.Do(request)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maximumStateBytes+1))
	if err = errors.Join(err, response.Body.Close()); err != nil {
		return err
	}
	if len(data) > maximumStateBytes {
		return errors.New("fleet response too large")
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		failure := &Error{Status: response.StatusCode}
		var envelope Error
		if json.Unmarshal(data, &envelope) == nil {
			failure.Code, failure.Message = envelope.Code, envelope.Message
		}
		return failure
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(data, result)
}

// ParseInvitation admits one pasted or scanned invitation line. The hub stays
// authoritative for one-use and expiry; this only rejects what cannot work.
func ParseInvitation(line string, now time.Time) (Invitation, error) {
	invalid := errors.New("invalid invitation")
	encoded, ok := strings.CutPrefix(line, invitationPrefix)
	if !ok || len(encoded) > base64.RawURLEncoding.EncodedLen(4096) {
		return Invitation{}, invalid
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return Invitation{}, invalid
	}
	var i Invitation
	if strictjson.Decode(data, &i) != nil {
		return Invitation{}, invalid
	}
	id, err := hex.DecodeString(i.FleetID)
	_, handleErr := machine.Parse(i.Hub.Machine)
	_, tokenErr := auth.ParseCredential(i.Token)
	if err != nil || len(id) != 32 || hex.EncodeToString(id) != i.FleetID || handleErr != nil || !machine.ValidName(i.Hub.Name) || !machine.ValidOrigin(i.Hub.Origin) || tokenErr != nil || !now.Before(i.ExpiresAt) {
		return Invitation{}, invalid
	}
	return i, nil
}

func EncodeInvitation(i Invitation) string {
	data, _ := json.Marshal(i) // fixed fields always encode
	return invitationPrefix + base64.RawURLEncoding.EncodeToString(data)
}
