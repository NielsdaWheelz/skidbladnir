package sessions

import (
	"encoding/json"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

// TerminalUsage is literal displayed provider usage, not a native account report.
type TerminalUsage struct {
	FiveHourUsedPercent *int `json:"fiveHourUsedPercent,omitempty"`
	SevenDayUsedPercent *int `json:"sevenDayUsedPercent,omitempty"`
}

func (usage TerminalUsage) Valid() bool {
	if usage.FiveHourUsedPercent == nil && usage.SevenDayUsedPercent == nil {
		return false
	}
	for _, percent := range []*int{usage.FiveHourUsedPercent, usage.SevenDayUsedPercent} {
		if percent != nil && (*percent < 0 || *percent > 100) {
			return false
		}
	}
	return true
}

// ValidFor excludes statuses from invalidated or failed observation stages.
func (usage TerminalUsage) ValidFor(status TerminalStatus) bool {
	return usage.Valid() && successfulTerminalSample(status)
}

func successfulTerminalSample(status TerminalStatus) bool {
	if status.Source != SourceTerminal {
		return false
	}
	switch status.Reason {
	case ReasonRecognized, ReasonPartial, ReasonLayoutUnknown, ReasonEvidenceClipped, ReasonEvidenceConflict:
		return true
	default:
		return false
	}
}

func (usage *TerminalUsage) UnmarshalJSON(encoded []byte) error {
	var wire *struct {
		FiveHourUsedPercent json.RawMessage `json:"fiveHourUsedPercent"`
		SevenDayUsedPercent json.RawMessage `json:"sevenDayUsedPercent"`
	}
	if strictjson.Decode(encoded, &wire) != nil || wire == nil {
		return errors.New("invalid terminal usage")
	}
	var accepted TerminalUsage
	for _, window := range []struct {
		encoded json.RawMessage
		target  **int
	}{{wire.FiveHourUsedPercent, &accepted.FiveHourUsedPercent}, {wire.SevenDayUsedPercent, &accepted.SevenDayUsedPercent}} {
		if len(window.encoded) == 0 {
			continue
		}
		var percent *int
		if strictjson.Decode(window.encoded, &percent) != nil || percent == nil {
			return errors.New("invalid terminal usage percentage")
		}
		*window.target = percent
	}
	if !accepted.Valid() {
		return errors.New("invalid terminal usage percentage")
	}
	*usage = accepted
	return nil
}
