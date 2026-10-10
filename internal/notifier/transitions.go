package notifier

import (
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// observe retains uncertainty but replaces positive identity and complete causes.
func observe(previous attention.Record, present bool, session fleetclient.Session, revision int64) attention.SessionSnapshot {
	ref, err := fleetclient.DecodeReference(session.Ref)
	if err != nil {
		panic("unadmitted inventory reference")
	} // justify-defect: fleetclient inventory decoder admitted the reference.
	key := fleetclient.NotificationKey(ref)
	foreground := fleetclient.NotificationForeground(session)
	status := session.TerminalStatus
	identityKnown := foreground != nil || session.Connection != nil || status.Source == sessions.SourceTerminal
	same := present && previous.Key == key && (!identityKnown || attention.SameForeground(previous.Foreground, foreground))
	record := attention.Record{Key: key, Foreground: foreground, Phase: attention.Quiet}
	if same {
		record = previous
	}
	if foreground != nil {
		record.Foreground = foreground
	}
	qualified := identityKnown && !same
	if foreground != nil && status.Source == sessions.SourceTerminal {
		nonIdle := attention.NonIdle(status)
		idle := attention.Idle(status)
		if nonIdle {
			record.Phase = attention.Armed
		} else if idle && record.Phase == attention.Armed {
			record.Phase = attention.Ready
			record.ReadyGeneration = revision
		}
		// Retire a known obsolete visible cause before choosing its successor.
		if record.Cause != nil {
			if record.Cause.Kind == "ready" && nonIdle || record.Cause.Kind == "action" && (status.Interaction == sessions.InteractionNone || status.Interaction == sessions.InteractionMenu) && status.Notice == sessions.NoticeNone {
				record.Cause = nil
				qualified = true
			}
		}
		var cause *attention.Cause
		if status.Interaction.Request() || status.Notice != sessions.NoticeNone {
			cause = &attention.Cause{Kind: "action"}
			if status.Interaction.Request() {
				cause.Request = status.Interaction
			}
			if status.Notice != sessions.NoticeNone {
				cause.Notice = status.Notice
			}
			qualified = true
		} else if idle && record.Phase == attention.Ready {
			cause = &attention.Cause{Kind: "ready"}
			qualified = true
		} else if idle || record.Cause == nil && (status.Interaction == sessions.InteractionNone || status.Interaction == sessions.InteractionMenu) && status.Notice == sessions.NoticeNone {
			qualified = true
		}
		if cause != nil {
			if !attention.SameCause(record.Cause, cause) {
				record.AttentionEpisode = revision
			}
			record.Cause = cause
		}
	}
	return attention.SessionSnapshot{Ref: session.Ref, Name: session.Name, TerminalStatus: status, AttentionQualified: qualified, Record: record}
}

func meaningful(before, after attention.MachineSnapshot) bool {
	if before.Availability != after.Availability || len(before.Sessions) != len(after.Sessions) {
		return true
	}
	for _, row := range after.Sessions {
		found := false
		for _, old := range before.Sessions {
			if old.Record.Key.Slot() != row.Record.Key.Slot() {
				continue
			}
			found = true
			a, b := old.Record, row.Record
			if old.Name != row.Name || old.Ref != row.Ref || old.AttentionQualified != row.AttentionQualified || a.Key != b.Key || !attention.SameForeground(a.Foreground, b.Foreground) || a.Phase != b.Phase || a.ReadyGeneration != b.ReadyGeneration || a.AttentionEpisode != b.AttentionEpisode || !attention.SameCause(a.Cause, b.Cause) {
				return true
			}
			break
		}
		if !found {
			return true
		}
	}
	return false
}
