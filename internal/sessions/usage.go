package sessions

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
