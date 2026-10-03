package sessionui

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

type directorySearchState uint8

const (
	directorySearchIdle directorySearchState = iota
	directorySearchLoading
	directorySearchReady
	directorySearchFailed
)

func (m *model) directoryAvailable() bool {
	if !m.scopeReady {
		return false
	}
	for _, peer := range m.peers {
		if peer.Label == m.form[0] {
			return peer.OK
		}
	}
	return false
}

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
			draft, err := m.groupSelection.accept(m.form[4], m.groupChoices(m.form[4]))
			if err != nil {
				m.fail(group.ErrInvalid.Error())
				return nil
			}
			m.form[4] = draft
			m.groupSelection.prefill(draft, m.groupChoices(draft))
		}
		if key.String() == "tab" || m.field < len(m.form) {
			return m.focusForm((m.field + 1) % (len(m.form) + 1))
		}
		cwd, terms, problem := parseDirectoryDraft(m.form[3])
		if problem != "" || len(terms) > 0 {
			if problem == "" {
				problem = "choose a matching directory"
			}
			m.inform(problem)
			return m.focusForm(3)
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
		return m.focusForm((m.field + len(m.form)) % (len(m.form) + 1))
	case "left", "right":
		if m.field == 3 {
			m.directorySelection.cycle(m.searchDirectories, key.String() == "left")
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
			if m.field == 3 || m.field == 4 {
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

func (m *model) focusForm(field int) tea.Cmd {
	m.field = field
	if field == 4 {
		m.groupSelection.prefill(m.form[4], m.groupChoices(m.form[4]))
	}
	if field == 3 && m.searchState == directorySearchIdle {
		return m.searchDirectory()
	}
	return nil
}

// Exactly one of path, terms or problem is present. This is lexical admission;
// only the selected host can expand home, normalize and validate existence.
func parseDirectoryDraft(draft string) (path string, terms []string, problem string) {
	if draft == "" {
		return "~", nil, ""
	}
	if !utf8.ValidString(draft) || strings.IndexFunc(draft, func(character rune) bool {
		return unicode.IsControl(character) || character == '\u061c' ||
			character >= '\u200e' && character <= '\u200f' ||
			character >= '\u2028' && character <= '\u202e' ||
			character >= '\u2066' && character <= '\u2069'
	}) >= 0 {
		return "", nil, "directory must not contain controls or directional formatting"
	}
	if strings.HasPrefix(draft, "/") || strings.HasPrefix(draft, "~") {
		if len(draft) > 4096 || draft != "~" && !strings.HasPrefix(draft, "~/") && !strings.HasPrefix(draft, "/") {
			return "", nil, "use ~, ~/… or an absolute path, at most 4096 bytes"
		}
		return draft, nil, ""
	}
	trimmed := strings.TrimLeftFunc(draft, unicode.IsSpace)
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "~") {
		return "", nil, "remove leading whitespace before the directory path"
	}
	terms = strings.Fields(draft)
	if len(terms) == 0 {
		return "", nil, "clear the directory for home or enter a path or search words"
	}
	bytes := 0
	for _, term := range terms {
		bytes += len(term)
	}
	if len(terms) > 8 || bytes > 256 {
		return "", nil, "enter 1–8 search words, at most 256 bytes"
	}
	return "", terms, ""
}

func (m *model) clearDirectorySearch() {
	if m.searchCancel != nil {
		m.searchCancel()
		m.searchCancel = nil
	}
	m.searchRevision++
	m.searchState, m.searchDirectories, m.directorySelection.choice = directorySearchIdle, nil, nil
	m.searchProblem, m.searchOmitted = "", false
}

func (m *model) searchDirectory() tea.Cmd {
	m.clearDirectorySearch()
	_, terms, problem := parseDirectoryDraft(m.form[3])
	m.inform(problem)
	if problem != "" || len(terms) == 0 || m.blurred {
		return nil
	}
	if !m.directoryAvailable() {
		m.inform("host unavailable; refresh before searching")
		return nil
	}
	m.searchState = directorySearchLoading
	ctx, cancel := context.WithCancel(m.ctx)
	m.searchCancel = cancel
	client, machine, revision := m.client, m.form[0], m.searchRevision
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
	_, terms, problem := parseDirectoryDraft(m.form[3])
	if problem != "" {
		m.inform(problem)
		return false
	}
	if len(terms) == 0 {
		return true
	}
	if m.searchState == directorySearchLoading {
		m.inform(m.directoryStatus())
		return false
	}
	choice, _, selected := m.directorySelection.current(m.searchDirectories)
	if !selected {
		m.inform("choose a matching directory")
		return false
	}
	m.form[3] = choice
	m.clearDirectorySearch()
	m.inform("")
	return true
}

func (m *model) directoryStatus() string {
	_, terms, problem := parseDirectoryDraft(m.form[3])
	if problem != "" || len(terms) == 0 {
		return problem
	}
	if !m.directoryAvailable() {
		return "host unavailable; refresh before searching"
	}
	switch m.searchState {
	case directorySearchIdle:
		return "choose a matching directory"
	case directorySearchLoading:
		return "searching…"
	case directorySearchFailed:
		return m.searchProblem
	case directorySearchReady:
		if len(m.searchDirectories) == 0 {
			return "no matching directories"
		}
		if m.searchOmitted {
			return "some directories are not shown"
		}
		return ""
	default:
		panic("invalid directory search state") // justify-defect: only these four states populate the model.
	}
}
