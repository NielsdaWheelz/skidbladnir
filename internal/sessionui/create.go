package sessionui

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
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

// The create form's fields in focus order; formCreate is the create button after them.
const (
	formMachine = iota
	formLaunch
	formName
	formDirectory
	formGroup
	formCreate
)

func (m *model) peerLabeled(label string) *fleetclient.Peer {
	for index := range m.peers {
		if m.peers[index].Label == label {
			return &m.peers[index]
		}
	}
	return nil
}

// checking is a peer without a current inventory that has not failed a read:
// unobserved, or invalidated by a write until its next inventory.
func checking(peer fleetclient.Peer) bool { return !peer.OK && peer.Error == nil }

// targetChecking distinguishes an unobserved or invalidated peer from a failed host.
func (m *model) targetChecking(label string) bool {
	peer := m.peerLabeled(label)
	return peer != nil && checking(*peer)
}

func (m *model) editQuickCreate(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "esc", "q":
		m.page = ""
		m.pending = fleetclient.Request{}
	case "left", "right":
		machines := m.client.Machines()
		for index, machine := range machines {
			if machine.Label != m.pending.Machine {
				continue
			}
			delta := 1
			if key.String() == "left" {
				delta = -1
			}
			m.pending.Machine = machines[(index+len(machines)+delta)%len(machines)].Label
			break
		}
	case "enter":
		peer := m.peerLabeled(m.pending.Machine)
		if peer == nil || !peer.OK {
			return nil
		}
		request := m.pending
		m.page = ""
		m.inform("opening terminal on " + request.Machine + "…")
		return m.execute(request)
	}
	return nil
}

func (m *model) directoryAvailable() bool {
	peer := m.peerLabeled(m.form[formMachine])
	return peer != nil && peer.OK
}

func (m *model) createAvailable() bool {
	peer := m.peerLabeled(m.form[formMachine])
	return peer != nil && peer.OK && (m.form[formLaunch] == "terminal" ||
		slices.ContainsFunc(peer.Profiles, func(profile fleetclient.Profile) bool { return profile.Key == m.form[formLaunch] }))
}

func (m *model) editForm(key tea.KeyPressMsg) tea.Cmd {
	previousDirectory := m.form[formDirectory]
	previousGroup := m.form[formGroup]
	switch key.String() {
	case "esc":
		m.page = ""
		m.clearDirectorySearch()
	case "tab", "enter":
		if m.field == formDirectory && !m.acceptDirectory() {
			return nil
		}
		if m.field == formGroup {
			draft, err := m.groupSelection.accept(m.form[formGroup], m.groupChoices(m.form[formGroup]))
			if err != nil {
				m.fail(group.ErrInvalid.Error())
				return nil
			}
			m.form[formGroup] = draft
			m.groupSelection.prefill(draft, m.groupChoices(draft))
		}
		if key.String() == "tab" || m.field < formCreate {
			return m.focusForm((m.field + 1) % (formCreate + 1))
		}
		cwd, terms, problem := parseDirectoryDraft(m.form[formDirectory])
		if problem != "" || len(terms) > 0 {
			if problem == "" {
				problem = "choose a matching directory"
			}
			m.inform(problem)
			return m.focusForm(formDirectory)
		}
		label, err := group.ParseDraft(m.form[formGroup])
		if err != nil {
			m.focusForm(formGroup)
			m.inform(group.ErrInvalid.Error())
			return nil
		}
		if !m.createAvailable() {
			m.inform("host unavailable; refresh before creating")
			if m.targetChecking(m.form[formMachine]) {
				m.inform("checking inventory; refresh before creating")
			}
			return nil
		}
		request := fleetclient.Request{Operation: "start", Kind: fleetclient.LaunchAgent, Machine: m.form[formMachine], Profile: m.form[formLaunch], Name: m.form[formName], CWD: cwd, Group: label}
		if m.form[formLaunch] == "terminal" {
			request.Kind, request.Profile = fleetclient.LaunchTerminal, ""
		}
		return m.execute(request)
	case "shift+tab":
		return m.focusForm((m.field + formCreate) % (formCreate + 1))
	case "left", "right":
		if m.field == formDirectory {
			m.directorySelection.cycle(m.searchDirectories, key.String() == "left")
			return nil
		}
		if m.field == formGroup {
			m.groupSelection.cycle(m.groupChoices(m.form[formGroup]), key.String() == "left")
			return nil
		}
		if m.field > formLaunch {
			return nil
		}
		options := []string{}
		for _, peer := range m.peers {
			if !peer.OK {
				continue
			}
			if m.field == formMachine {
				options = append(options, peer.Label)
			}
			if m.field == formLaunch && peer.Label == m.form[formMachine] {
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
		if m.field == formMachine {
			m.form[formDirectory] = ""
			m.clearDirectorySearch()
			m.inform("")
			if m.form[formLaunch] != "terminal" {
				m.form[formLaunch] = "terminal"
				if peer := m.peerLabeled(m.form[formMachine]); peer != nil && len(peer.Profiles) != 0 {
					m.form[formLaunch] = peer.Profiles[0].Key
				}
			}
		}
	case "ctrl+u":
		if m.field == formDirectory || m.field == formGroup {
			m.form[m.field] = ""
		}
	case "backspace":
		if m.field >= formName && m.field < formCreate && m.form[m.field] != "" {
			_, size := utf8.DecodeLastRuneInString(m.form[m.field])
			m.form[m.field] = m.form[m.field][:len(m.form[m.field])-size]
		}
	default:
		if m.field >= formName && m.field < formCreate {
			if m.field == formDirectory || m.field == formGroup {
				m.form[m.field] += key.Text
			} else {
				m.form[m.field] += attention.SingleLine(key.Text)
			}
		}
	}
	if m.field == formDirectory && m.form[formDirectory] != previousDirectory {
		return m.searchDirectory()
	}
	if m.field == formGroup && (m.form[formGroup] != previousGroup || key.String() == "ctrl+u") {
		m.groupSelection.reset(m.form[formGroup], m.groupChoices(m.form[formGroup]))
	}
	return nil
}

func (m *model) focusForm(field int) tea.Cmd {
	m.field = field
	if field == formGroup {
		m.groupSelection.prefill(m.form[formGroup], m.groupChoices(m.form[formGroup]))
	}
	if field == formDirectory && m.searchState == directorySearchIdle {
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
	_, terms, problem := parseDirectoryDraft(m.form[formDirectory])
	m.inform(problem)
	if problem != "" || len(terms) == 0 || m.blurred {
		return nil
	}
	if !m.directoryAvailable() {
		m.inform("host unavailable; refresh before searching")
		if m.targetChecking(m.form[formMachine]) {
			m.inform("checking inventory; refresh before searching")
		}
		return nil
	}
	m.searchState = directorySearchLoading
	ctx, cancel := context.WithCancel(m.ctx)
	m.searchCancel = cancel
	client, machine, revision := m.client, m.form[formMachine], m.searchRevision
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
	_, terms, problem := parseDirectoryDraft(m.form[formDirectory])
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
	m.form[formDirectory] = choice
	m.clearDirectorySearch()
	m.inform("")
	return true
}

func (m *model) directoryStatus() string {
	_, terms, problem := parseDirectoryDraft(m.form[formDirectory])
	if problem != "" || len(terms) == 0 {
		return problem
	}
	if !m.directoryAvailable() {
		if m.targetChecking(m.form[formMachine]) {
			return "checking inventory; refresh before searching"
		}
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
