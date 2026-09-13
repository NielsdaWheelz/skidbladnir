package fleetclient

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

var errConfiguration = errors.New("client configuration is invalid or unavailable")
var errOutputLimit = errors.New("output limit")

type peer struct {
	Label   string `json:"label"`
	Origin  string `json:"origin"`
	Machine string `json:"machine"`
	Bearer  string `json:"bearer"`
}

// Open reads the explicit private peer file; there is no ambient routing config.
func Open(path string) (*Client, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errConfiguration
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errConfiguration
	}
	defer file.Close()
	encoded, err := io.ReadAll(io.LimitReader(file, MaximumInputBytes+1))
	if err != nil || len(encoded) > MaximumInputBytes {
		return nil, errConfiguration
	}
	var config *struct {
		Peers []peer `json:"peers"`
	}
	if strictjson.Decode(encoded, &config) != nil || config == nil || len(config.Peers) == 0 {
		return nil, errConfiguration
	}
	labels, origins, handles := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for index, target := range config.Peers {
		if target.Label == "" || strings.TrimSpace(target.Label) != target.Label || strings.IndexFunc(target.Label, unicode.IsControl) >= 0 {
			return nil, errConfiguration
		}
		if _, err := machine.Parse(target.Machine); err != nil {
			return nil, errConfiguration
		}
		origin, err := url.Parse(target.Origin)
		if err != nil || origin.Scheme != "https" || origin.Hostname() == "" || origin.Port() != "8443" || origin.User != nil || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || origin.Opaque != "" || origin.String() != target.Origin {
			return nil, errConfiguration
		}
		decoded, err := base64.RawURLEncoding.Strict().DecodeString(target.Bearer)
		if err != nil || len(decoded) != 32 || len(target.Bearer) != 43 {
			return nil, errConfiguration
		}
		originKey := "https://" + strings.ToLower(origin.Host)
		config.Peers[index].Origin = originKey
		labelKey := strings.ToLower(target.Label)
		if labels[labelKey] || origins[originKey] || handles[target.Machine] {
			return nil, errConfiguration
		}
		labels[labelKey], origins[originKey], handles[target.Machine] = true, true, true
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	transport.Proxy = nil
	return &Client{
		peers: config.Peers,
		http:  &http.Client{Transport: transport, Timeout: Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}, nil
}
