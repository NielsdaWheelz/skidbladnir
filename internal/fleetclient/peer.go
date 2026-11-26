package fleetclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
)

// PeerSource is the exact terminal and foreground captured by the launcher.
type PeerSource struct {
	Reference
	Foreground *api.PeerForeground `json:"foreground,omitempty"`
}

func (source PeerSource) Encode() string {
	encoded, _ := json.Marshal(source)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func DecodePeerSource(value string) (PeerSource, error) {
	var source PeerSource
	if len(value) <= 4096 {
		decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
		if err == nil && json.Unmarshal(decoded, &source) == nil && source.Reference.valid() && source.Foreground != nil && source.Encode() == value {
			return source, nil
		}
	}
	return PeerSource{}, errors.New("invalid peer source")
}

// ReplyAddress routes only to an enrolled machine; its gateway resolves the
// digest against live foregrounds.
type ReplyAddress struct{ prefix, digest string }

func ParseReplyAddress(value string) (ReplyAddress, error) {
	prefix, digest, _ := strings.Cut(value, ".")
	if len(prefix) < 6 || len(prefix) > 32 || strings.Trim(prefix, "0123456789abcdef") != "" || len(digest) != 16 {
		return ReplyAddress{}, errors.New("invalid reply address")
	}
	if decoded, err := base64.RawURLEncoding.Strict().DecodeString(digest); err != nil || len(decoded) != 12 {
		return ReplyAddress{}, errors.New("invalid reply address")
	}
	return ReplyAddress{prefix, digest}, nil
}

func (client *Client) reply(ctx context.Context, request Request) Result {
	address := request.ReplyAddress
	var selected peer
	for _, candidate := range client.peerSnapshot() {
		if !strings.HasPrefix(strings.TrimPrefix(candidate.Machine, "mh-"), address.prefix) {
			continue
		}
		if selected.Machine != "" {
			return Failed("reply_address_ambiguous", "not_sent")
		}
		selected = candidate
	}
	if selected.Machine == "" {
		return Failed("reply_address_unavailable", "not_sent")
	}
	body, _ := json.Marshal(struct {
		Text string `json:"text"`
	}{request.Text})
	if len(body) > MaximumInputBytes {
		return Failed("input_limit", "not_sent")
	}
	return client.call(ctx, selected, "peer_reply", address.digest, body)
}

// withReplyAddress verifies one source and composes one line before either
// write. An unavailable implicit source means ordinary one-way delivery.
func (client *Client) withReplyAddress(ctx context.Context, request Request) (text string, included bool, failure *Failure) {
	source := request.PeerSource
	explicit := !request.ReplyTo.IsZero()
	if explicit {
		source = PeerSource{Reference: request.ReplyTo}
	}
	if source.Reference.IsZero() {
		return request.Text, false, nil
	}
	peers := client.peerSnapshot()
	index := slices.IndexFunc(peers, func(candidate peer) bool { return candidate.Machine == source.Machine })
	machineID := strings.TrimPrefix(source.Machine, "mh-")
	if index < 0 || len(machineID) != 32 {
		if explicit {
			return "", false, Failed("reply_address_unavailable", "not_sent").Error
		}
		return request.Text, false, nil
	}
	target := peers[index]
	// Apply the same shortest unambiguous prefix the receiver's local
	// roster resolves. The full identity never appears in terminal text.
	prefixLength := 6
	for _, candidate := range peers {
		if candidate.Machine == source.Machine {
			continue
		}
		other := strings.TrimPrefix(candidate.Machine, "mh-")
		for prefixLength < len(machineID) && strings.HasPrefix(other, machineID[:prefixLength]) {
			prefixLength++
		}
	}
	body, _ := json.Marshal(struct {
		IdentityToken string              `json:"identityToken"`
		PaneID        string              `json:"paneId"`
		Foreground    *api.PeerForeground `json:"foreground,omitempty"`
	}{source.IdentityToken, source.PaneID, source.Foreground})
	// An unreachable implicit source must leave time for one-way delivery.
	addressContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	foundAddress := client.call(addressContext, target, "terminal_address", source.TmuxID, body)
	if !foundAddress.OK {
		if explicit {
			return "", false, foundAddress.Error
		}
		return request.Text, false, nil
	}
	value := foundAddress.Value.(api.PeerAddress)
	text = attention.SingleLine(value.Sender) + " on " + attention.SingleLine(target.Label) + " · skid reply " +
		machineID[:prefixLength] + "." + value.Digest + "\n" + request.Text
	if len(text) > 32768 {
		return "", false, Failed("input_limit", "not_sent").Error
	}
	return text, true, nil
}
