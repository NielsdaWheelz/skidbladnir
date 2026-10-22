//go:build darwin && cgo

package macnotifications

import "github.com/NielsdaWheelz/skidbladnir/internal/attention"

// notice is one native notification, as posted and as the notification
// center reports it. Its identifier is the slot.
type notice struct {
	Slot    string `json:"slot"`
	Machine string `json:"machine"`
	Ref     string `json:"ref"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Episode int64  `json:"episode"`
}

type effect struct {
	notice  notice
	cancel  bool
	audible bool // a post with sound; false is a silent replacement
}

// reconcile returns, in order, the native effects that make the delivered
// notices show the hub's state. alerted (slot -> episode) holds this process's
// audible posts and the owner's dismissals and clicks: an episode alerts at
// most once per process, even when its copy was removed without a callback or
// has not surfaced yet. A gap machine's notices are neither posted nor removed.
func reconcile(state attention.State, delivered []notice, alerted map[string]int64, allowed bool) []effect {
	copies := make(map[string]notice, len(delivered))
	for _, shown := range delivered {
		copies[shown.Slot] = shown
	}
	gaps := make(map[string]bool)
	settled := make(map[string]bool) // slots with a row, or already cancelled
	effects := []effect{}
	for _, machine := range state.Machines {
		if machine.Availability == attention.Gap {
			gaps[machine.Machine] = true
			continue
		}
		for _, row := range machine.Sessions {
			settled[row.Slot] = true
			want := notice{Slot: row.Slot, Machine: machine.Machine, Ref: row.Ref, Title: row.Title, Body: row.Body, Episode: row.Episode}
			shown, found := copies[row.Slot]
			_, posted := alerted[row.Slot]
			switch row.Delivery {
			case attention.Clear:
				if found || posted {
					effects = append(effects, effect{notice: want, cancel: true})
				}
			case attention.Keep:
				if found && shown.Episode == row.Episode && shown != want {
					effects = append(effects, effect{notice: want})
				}
			case attention.Post:
				switch {
				case found && shown.Episode == row.Episode:
					if shown != want {
						effects = append(effects, effect{notice: want})
					}
				case alerted[row.Slot] == row.Episode:
				case allowed:
					effects = append(effects, effect{notice: want, audible: true})
				case found:
					// Superseded while notifications are blocked: the new
					// episode posts once permission allows.
					effects = append(effects, effect{notice: shown, cancel: true})
				}
			}
		}
	}
	for _, shown := range delivered {
		if !settled[shown.Slot] && !gaps[shown.Machine] {
			settled[shown.Slot] = true
			effects = append(effects, effect{notice: shown, cancel: true})
		}
	}
	return effects
}
