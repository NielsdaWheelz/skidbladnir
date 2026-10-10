package attention

import (
	"crypto/ecdh"
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

func (config Config) Valid() bool {
	if _, err := machine.Parse(config.ObserverMachine); err != nil {
		return false
	}
	origin, err := url.Parse(config.NtfyOrigin)
	return err == nil && origin.Scheme == "https" && origin.Hostname() != "" && origin.Port() == "8444" && origin.User == nil && origin.Path == "" && origin.RawPath == "" && origin.RawQuery == "" && !origin.ForceQuery && origin.Fragment == "" && origin.Opaque == "" && origin.String() == config.NtfyOrigin && strings.ToLower(origin.Host) == origin.Host
}
func (subscription Subscription) Valid(origin string) bool {
	endpoint, err := url.Parse(subscription.Endpoint)
	if err != nil || endpoint.Scheme+"://"+endpoint.Host != origin || endpoint.User != nil || endpoint.RawPath != "" || endpoint.RawQuery != "up=1" || endpoint.ForceQuery || endpoint.Fragment != "" || endpoint.Opaque != "" || endpoint.String() != subscription.Endpoint {
		return false
	}
	topic := strings.TrimPrefix(endpoint.Path, "/")
	if !strings.HasPrefix(topic, "up") || len(topic) < 3 || len(topic) > 64 || strings.Trim(topic, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
		return false
	}
	_, _, err = subscription.PublicKeys()
	return err == nil
}
func (subscription Subscription) PublicKeys() (*ecdh.PublicKey, []byte, error) {
	encoding := base64.RawURLEncoding.Strict()
	key, err := encoding.DecodeString(subscription.Keys.P256DH)
	if err != nil || len(key) != 65 || encoding.EncodeToString(key) != subscription.Keys.P256DH {
		return nil, nil, ErrUnavailable
	}
	public, err := ecdh.P256().NewPublicKey(key)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	auth, err := encoding.DecodeString(subscription.Keys.Auth)
	if err != nil || len(auth) != 16 || encoding.EncodeToString(auth) != subscription.Keys.Auth {
		return nil, nil, ErrUnavailable
	}
	return public, auth, nil
}
