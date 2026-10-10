package attention

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

func required(data []byte, fields ...string) bool {
	var members map[string]json.RawMessage
	if strictjson.Decode(data, &members) != nil || members == nil {
		return false
	}
	for _, field := range fields {
		value, present := members[field]
		if !present || string(value) == "null" {
			return false
		}
	}
	return true
}
func (key *Key) UnmarshalJSON(data []byte) error {
	type wire Key
	var value wire
	if !required(data, "machine", "tmuxId", "identityToken", "paneId") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*key = Key(value)
	return nil
}
func (foreground *Foreground) UnmarshalJSON(data []byte) error {
	type wire Foreground
	var value wire
	if !required(data, "provider", "pid", "startIdentity") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*foreground = Foreground(value)
	return nil
}
func (cause *Cause) UnmarshalJSON(data []byte) error {
	type wire Cause
	var value wire
	if !required(data, "kind") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	var members map[string]json.RawMessage
	if strictjson.Decode(data, &members) != nil {
		return ErrUnavailable
	}
	if value.Kind == "ready" && (members["request"] != nil || members["notice"] != nil) || members["request"] != nil && !requestValid(value.Request) || members["notice"] != nil && value.Notice != "interrupted" && value.Notice != "error" {
		return ErrUnavailable
	}
	*cause = Cause(value)
	return nil
}
func (record *Record) UnmarshalJSON(data []byte) error {
	type wire Record
	var value wire
	if !required(data, "key", "phase", "readyGeneration", "attentionEpisode") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*record = Record(value)
	return nil
}
func (row *SessionSnapshot) UnmarshalJSON(data []byte) error {
	type wire SessionSnapshot
	var value wire
	if !required(data, "ref", "name", "terminalStatus", "attentionQualified", "record") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*row = SessionSnapshot(value)
	return nil
}
func (snapshot *MachineSnapshot) UnmarshalJSON(data []byte) error {
	type wire MachineSnapshot
	var value wire
	if !required(data, "machine", "availability") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	if value.Availability == "fresh" && !required(data, "observedAt", "sessions") {
		return ErrUnavailable
	}
	if value.Availability == "gap" {
		var fields map[string]json.RawMessage
		if strictjson.Decode(data, &fields) != nil || fields["observedAt"] != nil || fields["sessions"] != nil {
			return ErrUnavailable
		}
	}
	*snapshot = MachineSnapshot(value)
	return nil
}
func (snapshot *Snapshot) UnmarshalJSON(data []byte) error {
	type wire Snapshot
	var value wire
	if !required(data, "schema", "epoch", "revision", "receiverTag", "machines") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*snapshot = Snapshot(value)
	return nil
}
func (hint *Hint) UnmarshalJSON(data []byte) error {
	type wire Hint
	var value wire
	if !required(data, "schema", "epoch", "revision") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*hint = Hint(value)
	return nil
}
func (config *Config) UnmarshalJSON(data []byte) error {
	type wire Config
	var value wire
	if !required(data, "observerMachine", "ntfyOrigin") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*config = Config(value)
	return nil
}
func (subscription *Subscription) UnmarshalJSON(data []byte) error {
	type wire Subscription
	var value wire
	if !required(data, "endpoint", "keys") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	var members map[string]json.RawMessage
	if strictjson.Decode(data, &members) != nil || !required(members["keys"], "p256dh", "auth") {
		return ErrUnavailable
	}
	*subscription = Subscription(value)
	return nil
}
func (token *ReadyToken) UnmarshalJSON(data []byte) error {
	type wire ReadyToken
	var value wire
	if !required(data, "epoch", "generation") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*token = ReadyToken(value)
	return nil
}
func (record *DeviceRecord) UnmarshalJSON(data []byte) error {
	type wire DeviceRecord
	var value wire
	if !required(data, "key", "receivedReadyGeneration", "acknowledgedReadyGeneration", "handledAttentionEpisode", "presentation") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*record = DeviceRecord(value)
	return nil
}
func (desired *DesiredRegistration) UnmarshalJSON(data []byte) error {
	type wire DesiredRegistration
	var value wire
	if !required(data, "generation", "subscription") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*desired = DesiredRegistration(value)
	return nil
}
func (snapshot *DeviceSnapshot) UnmarshalJSON(data []byte) error {
	type wire DeviceSnapshot
	var value wire
	if !required(data, "schema", "admittedRevision", "localRevision", "records") || strictjson.Decode(data, &value) != nil {
		return ErrUnavailable
	}
	*snapshot = DeviceSnapshot(value)
	return nil
}
func ReceiverRevision(epoch, tag string) (int64, bool) {
	prefix, number, found := strings.Cut(tag, ":")
	if !found || prefix != epoch {
		return 0, false
	}
	revision, err := strconv.ParseInt(number, 10, 64)
	return revision, err == nil && revision >= 0 && revision < math.MaxInt64 && strconv.FormatInt(revision, 10) == number
}
