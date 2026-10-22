package notifier

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

type phase string

const (
	quiet phase = "quiet"
	armed phase = "armed"
	ready phase = "ready"
)

// record is one session lifetime's attention with the owner's acknowledgement.
// Episodes and ready generations are observer revisions, so they only grow.
type record struct {
	Key                  fleetclient.Reference `json:"key"`
	Foreground           *attention.Foreground `json:"foreground,omitempty"`
	Phase                phase                 `json:"phase"`
	ReadyGeneration      int64                 `json:"readyGeneration"`
	AttentionEpisode     int64                 `json:"attentionEpisode"`
	Cause                *cause                `json:"cause,omitempty"`
	HandledEpisode       int64                 `json:"handledEpisode"`
	AckedReadyGeneration int64                 `json:"ackedReadyGeneration"`
}

type cause struct {
	Kind    string               `json:"kind"` // ready | action
	Request sessions.Interaction `json:"request,omitempty"`
	Notice  sessions.Notice      `json:"notice,omitempty"`
}

// valid admits the closed sets project and text switch on; the rest of the
// file is this observer's own output.
func (r record) valid() bool {
	if r.Phase != quiet && r.Phase != armed && r.Phase != ready {
		return false
	}
	c := r.Cause
	return c == nil ||
		c.Kind == "ready" && c.Request == "" && c.Notice == "" ||
		c.Kind == "action" && (c.Request != "" || c.Notice != "") &&
			(c.Request == "" || c.Request.Valid() && c.Request.Request()) &&
			(c.Notice == "" || c.Notice == sessions.NoticeError || c.Notice == sessions.NoticeInterrupted)
}

// slotID names one session lifetime: the native notice identifier.
func slotID(ref fleetclient.Reference) string {
	data, _ := json.Marshal(struct {
		Machine       string `json:"machine"`
		TmuxID        string `json:"tmuxId"`
		IdentityToken string `json:"identityToken"`
	}{ref.Machine, ref.TmuxID, ref.IdentityToken}) // string fields always encode
	return base64.RawURLEncoding.EncodeToString(data)
}

func nonIdle(status sessions.TerminalStatus) bool {
	return status.Interaction.Request() || status.Interaction == sessions.InteractionMenu || status.Activity == sessions.ActivityStarting || status.Activity == sessions.ActivityWorking
}

func idle(status sessions.TerminalStatus) bool {
	return status.Activity == sessions.ActivityIdle && status.Interaction == sessions.InteractionNone
}

func (c cause) text() string {
	if c.Kind == "ready" {
		return "ready"
	}
	parts := []string{}
	if c.Request != "" {
		parts = append(parts, attention.RequestText(c.Request))
	}
	if c.Notice != "" {
		parts = append(parts, attention.NoticeText(c.Notice))
	}
	return strings.Join(parts, " · ")
}

// current reports that the terminal still shows the cause.
func current(c cause, status sessions.TerminalStatus) bool {
	if c.Kind == "ready" {
		return idle(status) && status.Notice == sessions.NoticeNone
	}
	return (c.Request == "" && !status.Interaction.Request() || c.Request == status.Interaction) &&
		(c.Notice == "" && status.Notice != sessions.NoticeError && status.Notice != sessions.NoticeInterrupted || c.Notice == status.Notice)
}

// observe retains uncertainty but replaces positive identity and complete
// causes. It reports whether this evidence is qualified to post or clear.
func observe(previous record, present bool, ref fleetclient.Reference, session fleetclient.Session, revision int64) (record, bool) {
	foreground := fleetclient.NotificationForeground(session)
	status := session.TerminalStatus
	identityKnown := foreground != nil || session.Connection != nil || status.Source == sessions.SourceTerminal
	same := present && previous.Key == ref && (!identityKnown || attention.SameForeground(previous.Foreground, foreground))
	next := record{Key: ref, Foreground: foreground, Phase: quiet}
	if same {
		next = previous
	}
	if foreground != nil {
		next.Foreground = foreground
	}
	qualified := identityKnown && !same
	if foreground != nil && status.Source == sessions.SourceTerminal {
		busy, settled := nonIdle(status), idle(status)
		if busy {
			next.Phase = armed
		} else if settled && next.Phase == armed {
			next.Phase = ready
			next.ReadyGeneration = revision
		}
		// Retire a known obsolete visible cause before choosing its successor.
		if next.Cause != nil {
			if next.Cause.Kind == "ready" && busy || next.Cause.Kind == "action" && (status.Interaction == sessions.InteractionNone || status.Interaction == sessions.InteractionMenu) && status.Notice == sessions.NoticeNone {
				next.Cause = nil
				qualified = true
			}
		}
		var successor *cause
		if status.Interaction.Request() || status.Notice != sessions.NoticeNone {
			successor = &cause{Kind: "action"}
			if status.Interaction.Request() {
				successor.Request = status.Interaction
			}
			if status.Notice != sessions.NoticeNone {
				successor.Notice = status.Notice
			}
			qualified = true
		} else if settled && next.Phase == ready {
			successor = &cause{Kind: "ready"}
			qualified = true
		} else if settled || next.Cause == nil && (status.Interaction == sessions.InteractionNone || status.Interaction == sessions.InteractionMenu) && status.Notice == sessions.NoticeNone {
			qualified = true
		}
		if successor != nil {
			if next.Cause == nil || *next.Cause != *successor {
				next.AttentionEpisode = revision
			}
			next.Cause = successor
		}
	}
	return next, qualified
}

// project renders one session's verdict from its record and latest evidence.
func project(rec record, s seen) attention.Session {
	row := attention.Session{Slot: s.slot, Ref: s.ref, Delivery: attention.Clear}
	terminal := s.qualified && rec.Foreground != nil && s.status.Source == sessions.SourceTerminal
	pending := terminal && rec.Phase == ready && idle(s.status) && rec.ReadyGeneration > rec.AckedReadyGeneration
	if pending {
		row.ReadyGeneration, row.Foreground = rec.ReadyGeneration, rec.Foreground
	}
	c := rec.Cause
	if c == nil || rec.AttentionEpisode == rec.HandledEpisode || c.Kind == "ready" && rec.ReadyGeneration <= rec.AckedReadyGeneration {
		return row
	}
	row.Title, row.Body, row.Episode, row.Delivery = attention.SingleLine(s.name), c.text(), rec.AttentionEpisode, attention.Keep
	if terminal && current(*c, s.status) && (c.Kind != "ready" || pending) {
		row.Delivery = attention.Post
	}
	return row
}
