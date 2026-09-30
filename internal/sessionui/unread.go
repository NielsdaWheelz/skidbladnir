package sessionui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

type replyScan struct {
	cursor string
	staged []string
}
type repliesMsg struct {
	ref          fleetclient.Reference
	paneID       string
	conversation agentruntime.Conversation
	result       fleetclient.Result
}

// The existing foreground inventory cadence admits one result page per host.
// Round-robin selection gives every represented conversation and page a turn.
func (m *model) recoverReplies() tea.Cmd {
	if m.unreadStore == nil || m.unreadFailed {
		return nil
	}
	hosts := map[string][]repliesMsg{}
	seen := map[fleetclient.UnreadKey]bool{}
	for _, row := range m.rows {
		if !row.available || m.replyBusy[row.machine] {
			continue
		}
		ref, _ := fleetclient.DecodeReference(row.session.Ref)
		conversation, found := m.unreadSnapshot.Conversation(ref, row.session)
		if !found {
			continue
		}
		key := fleetclient.ReplyKey(row.machine, conversation)
		if seen[key] {
			continue
		}
		seen[key] = true
		hosts[row.machine] = append(hosts[row.machine], repliesMsg{ref: ref, paneID: row.session.ActivePaneID, conversation: conversation})
	}
	commands := []tea.Cmd{}
	for host, targets := range hosts {
		index := m.replyNext[host] % len(targets)
		m.replyNext[host] = index + 1
		message := targets[index]
		key := fleetclient.ReplyKey(host, message.conversation)
		scan := m.replyScans[key]
		if scan == nil {
			scan = &replyScan{}
			m.replyScans[key] = scan
		}
		cursor := scan.cursor
		m.replyBusy[host] = true
		commands = append(commands, func() tea.Msg {
			message.result = m.client.Results(m.ctx, message.ref, message.conversation, cursor)
			return message
		})
	}
	return tea.Batch(commands...)
}

func (m *model) receiveReplies(message repliesMsg) {
	m.replyBusy[message.ref.Machine] = false
	key := fleetclient.ReplyKey(message.ref.Machine, message.conversation)
	represented := false
	for _, row := range m.rows {
		if !row.available || row.machine != message.ref.Machine || row.session.ActivePaneID != message.paneID {
			continue
		}
		current, _ := fleetclient.DecodeReference(row.session.Ref)
		conversation, found := m.unreadSnapshot.Conversation(current, row.session)
		if current.SessionEqual(message.ref) && found && conversation == message.conversation {
			represented = true
			break
		}
	}
	if !represented {
		return
	}
	if !message.result.OK {
		m.repliesUnavailable[key] = true
		delete(m.replyScans, key)
		return
	}
	page := message.result.Value.(fleetclient.ResultsResult)
	scan := m.replyScans[key]
	if scan == nil {
		return
	}
	_, initialized := m.unreadSnapshot.Record(key)
	ids := page.ResultIDs
	if !initialized {
		for _, id := range ids {
			if !slices.Contains(scan.staged, id) {
				scan.staged = append(scan.staged, id)
			}
		}
		ids = scan.staged
	}
	if initialized || page.NextCursor == "" {
		snapshot, err := m.unreadStore.Observe(key, ids, !initialized)
		if err != nil {
			m.unreadFailed = true
			delete(m.replyScans, key)
			return
		}
		m.unreadFailed = false
		m.unreadSnapshot = snapshot
	}
	m.repliesUnavailable[key] = false
	scan.cursor = page.NextCursor
	if page.NextCursor == "" {
		scan.staged = nil
	}
}

func (m *model) acknowledgement(request fleetclient.Request) func() error {
	if m.unreadStore == nil {
		return nil
	}
	ref, err := fleetclient.DecodeReference(request.Ref)
	if err != nil {
		return nil
	}
	for _, row := range m.rows {
		current, _ := fleetclient.DecodeReference(row.session.Ref)
		if !current.SessionEqual(ref) {
			continue
		}
		conversation, found := m.unreadSnapshot.Conversation(ref, row.session)
		if !found {
			return nil
		}
		key := fleetclient.ReplyKey(ref.Machine, conversation)
		record, found := m.unreadSnapshot.Record(key)
		if !found || len(record.UnreadIDs) == 0 {
			return nil
		}
		ids := append([]string(nil), record.UnreadIDs...)
		return func() error { _, err := m.unreadStore.Acknowledge(key, ids); return err }
	}
	return nil
}

func (m *model) replyText(row listedRow) string {
	ref, _ := fleetclient.DecodeReference(row.session.Ref)
	conversation, found := m.unreadSnapshot.Conversation(ref, row.session)
	if !found {
		return ""
	}
	key := fleetclient.ReplyKey(row.machine, conversation)
	record, found := m.unreadSnapshot.Record(key)
	if !found || len(record.UnreadIDs) == 0 {
		return ""
	}
	if conversation.Provider == agentruntime.ProviderClaude && row.session.Conversation == nil {
		return "new reply · previous agent"
	}
	return "new reply"
}

func (m *model) replyAvailable(row listedRow) bool {
	ref, _ := fleetclient.DecodeReference(row.session.Ref)
	_, found := m.unreadSnapshot.Conversation(ref, row.session)
	return found
}
