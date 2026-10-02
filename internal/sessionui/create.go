package sessionui

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

func (m *model) createAvailable() bool {
	if !m.scopeReady {
		return false
	}
	for _, peer := range m.peers {
		if peer.Label != m.form[0] || !peer.OK {
			continue
		}
		if m.form[1] == "terminal" {
			return true
		}
		for _, profile := range peer.Profiles {
			if profile.Key == m.form[1] {
				return true
			}
		}
	}
	return false
}

func (m *model) editForm(key tea.KeyPressMsg) tea.Cmd {
	previousDirectory := m.form[3]
	previousGroup := m.form[4]
	switch key.String() {
	case "esc":
		m.page = ""
		m.clearDirectorySearch()
	case "tab", "enter":
		if m.field == 3 && !m.acceptDirectory() {
			return nil
		}
		if m.field == 4 {
			draft, err := m.groupSelection.accept(m.form[4])
			if err != nil {
				m.fail(group.ErrInvalid.Error())
				return nil
			}
			m.form[4] = draft
			m.groupSelection.prefill(draft, m.groupChoices(draft))
		}
		if key.String() == "tab" || m.field < len(m.form) {
			m.focusForm((m.field + 1) % (len(m.form) + 1))
			return nil
		}
		if directoryIsQuery(m.form[3]) {
			m.field = 3
			return nil
		}
		label, err := group.ParseDraft(m.form[4])
		if err != nil {
			m.focusForm(4)
			m.inform(group.ErrInvalid.Error())
			return nil
		}
		if !m.createAvailable() {
			m.inform("host unavailable; refresh before creating")
			return nil
		}
		cwd := m.form[3]
		if strings.TrimSpace(cwd) == "" {
			cwd = "~"
		}
		request := fleetclient.Request{Operation: "start", Kind: fleetclient.LaunchAgent, Machine: m.form[0], Profile: m.form[1], Name: m.form[2], CWD: cwd, Group: label}
		if m.form[1] == "terminal" {
			request.Kind, request.Profile = fleetclient.LaunchTerminal, ""
		}
		if !request.Valid() {
			m.inform("machine, launch, and directory are required")
			return nil
		}
		return m.execute(request)
	case "shift+tab":
		m.focusForm((m.field + len(m.form)) % (len(m.form) + 1))
	case "left", "right":
		if m.field == 3 && len(m.searchDirectories) > 0 {
			step := 1
			if key.String() == "left" {
				step = -1
			}
			m.searchCursor = (m.searchCursor + step + len(m.searchDirectories)) % len(m.searchDirectories)
			return nil
		}
		if m.field == 4 {
			m.groupSelection.cycle(m.groupChoices(m.form[4]), key.String() == "left")
			return nil
		}
		if m.field > 1 {
			return nil
		}
		options := []string{}
		for _, peer := range m.peers {
			if !peer.OK {
				continue
			}
			if m.field == 0 {
				options = append(options, peer.Label)
			}
			if m.field == 1 && peer.Label == m.form[0] {
				for _, profile := range peer.Profiles {
					options = append(options, profile.Key)
				}
				options = append(options, "terminal")
			}
		}
		if len(options) == 0 {
			return nil
		}
		index := 0
		for i, value := range options {
			if value == m.form[m.field] {
				index = i
				break
			}
		}
		if key.String() == "left" {
			index = (index + len(options) - 1) % len(options)
		} else {
			index = (index + 1) % len(options)
		}
		m.form[m.field] = options[index]
		if m.field == 0 {
			m.form[3] = ""
			m.clearDirectorySearch()
			m.inform("")
			if m.form[1] != "terminal" {
				m.form[1] = "terminal"
				for _, peer := range m.peers {
					if peer.Label == m.form[0] && len(peer.Profiles) != 0 {
						m.form[1] = peer.Profiles[0].Key
						break
					}
				}
			}
		}
	case "ctrl+u":
		if m.field == 3 || m.field == 4 {
			m.form[m.field] = ""
		}
	case "backspace":
		if m.field >= 2 && m.field < len(m.form) && m.form[m.field] != "" {
			_, size := utf8.DecodeLastRuneInString(m.form[m.field])
			m.form[m.field] = m.form[m.field][:len(m.form[m.field])-size]
		}
	default:
		if m.field >= 2 && m.field < len(m.form) {
			if m.field == 4 {
				m.form[m.field] += key.Text
			} else {
				m.form[m.field] += singleLine(key.Text)
			}
		}
	}
	if m.field == 3 && m.form[3] != previousDirectory {
		return m.searchDirectory()
	}
	if m.field == 4 && (m.form[4] != previousGroup || key.String() == "ctrl+u") {
		m.groupSelection.reset(m.form[4], m.groupChoices(m.form[4]))
	}
	return nil
}

func (m *model) focusForm(field int) {
	m.field = field
	if field == 4 {
		m.groupSelection.prefill(m.form[4], m.groupChoices(m.form[4]))
	}
}

func directoryIsQuery(draft string) bool {
	return strings.TrimSpace(draft) != "" && !strings.HasPrefix(draft, "/") && !strings.HasPrefix(draft, "~")
}

func (m *model) clearDirectorySearch() {
	if m.searchCancel != nil {
		m.searchCancel()
		m.searchCancel = nil
	}
	m.searchRevision++
	m.searching, m.searchDirectories, m.searchCursor = false, nil, 0
}

func (m *model) searchDirectory() tea.Cmd {
	m.clearDirectorySearch()
	m.inform("")
	if !directoryIsQuery(m.form[3]) {
		return nil
	}
	m.searching = true
	ctx, cancel := context.WithCancel(m.ctx)
	m.searchCancel = cancel
	client, machine, revision := m.client, m.form[0], m.searchRevision
	terms := strings.Fields(m.form[3])
	return func() tea.Msg {
		defer cancel()
		timer := time.NewTimer(150 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return searchMsg{machine: machine, revision: revision, result: client.SearchDirectories(ctx, machine, terms)}
		}
	}
}

// Suggestions never edit the query. Advancing accepts the current choice once.
func (m *model) acceptDirectory() bool {
	if !directoryIsQuery(m.form[3]) {
		return true
	}
	if m.searching {
		m.inform("searching…")
		return false
	}
	if len(m.searchDirectories) == 0 {
		return false
	}
	m.form[3] = m.searchDirectories[m.searchCursor]
	m.clearDirectorySearch()
	m.inform("")
	return true
}
