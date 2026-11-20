package api

import "time"

// ClaudeUsage carries independently observed quota fields for one verified Claude account.
type ClaudeUsage struct {
	Account       string       `json:"account"`
	FiveHour      ClaudeWindow `json:"fiveHour,omitzero"`
	SevenDay      ClaudeWindow `json:"sevenDay,omitzero"`
	FableSevenDay ClaudeWindow `json:"fableSevenDay,omitzero"`
}

type ClaudeWindow struct {
	Usage *ClaudeAmount `json:"usage,omitempty"`
	Reset *ClaudeReset  `json:"reset,omitempty"`
}

type ClaudeAmount struct {
	UsedPercent float64   `json:"usedPercent"`
	FetchedAt   time.Time `json:"fetchedAt"`
}

type ClaudeReset struct {
	ResetsAt  time.Time `json:"resetsAt"`
	FetchedAt time.Time `json:"fetchedAt"`
}

func (r ClaudeUsage) HasReading() bool {
	return r.FiveHour != (ClaudeWindow{}) || r.SevenDay != (ClaudeWindow{}) || r.FableSevenDay != (ClaudeWindow{})
}

// Merge orders each supplied field by the Claude cache's original fetch time.
// On an exact tie, the larger value wins so every reader converges in any order.
func (r *ClaudeUsage) Merge(next ClaudeUsage) {
	if next.Account == "" {
		return
	}
	if r.Account == "" {
		*r = next
		return
	}
	if r.Account != next.Account {
		return
	}
	r.FiveHour.merge(next.FiveHour)
	r.SevenDay.merge(next.SevenDay)
	r.FableSevenDay.merge(next.FableSevenDay)
}

func (w *ClaudeWindow) merge(next ClaudeWindow) {
	if n, old := next.Usage, w.Usage; n != nil && (old == nil || n.FetchedAt.After(old.FetchedAt) ||
		n.FetchedAt.Equal(old.FetchedAt) && n.UsedPercent > old.UsedPercent) {
		w.Usage = n
	}
	if n, old := next.Reset, w.Reset; n != nil && (old == nil || n.FetchedAt.After(old.FetchedAt) ||
		n.FetchedAt.Equal(old.FetchedAt) && n.ResetsAt.After(old.ResetsAt)) {
		w.Reset = n
	}
}
