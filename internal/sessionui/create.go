package sessionui

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
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
	formParent
	formCreate
)

type createDraft struct {
	machine, name, directory, group string
	launch                          launchChoice
	parent                          *api.SessionRef
	parentQuery                     string
	parentAccepted                  bool
}

type launchChoice struct {
	kind    fleetclient.LaunchKind
	profile string
}

func (choice launchChoice) label() string {
	switch {
	case choice.kind == fleetclient.LaunchTerminal:
		return "terminal"
	case choice.profile == "terminal":
		return "terminal (profile)"
	default:
		return choice.profile
	}
}

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

func (m *model) hostRefusal(label string) string {
	peer := m.peerLabeled(label)
	switch {
	case label == "":
		return "no destination machine is selected; nothing was sent."
	case peer == nil:
		return label + " is no longer listed; nothing was sent."
	case checking(*peer):
		return label + " is checking inventory; nothing was sent."
	case !peer.OK:
		return label + " is unavailable; nothing was sent."
	default:
		return ""
	}
}

func (m *model) directoryAvailable() bool {
	peer := m.peerLabeled(m.form.machine)
	return peer != nil && peer.OK
}

func declaresLaunch(peer *fleetclient.Peer, launch string) bool {
	return launch != "" && slices.ContainsFunc(peer.Profiles, func(profile fleetclient.Profile) bool {
		return profile.Key == launch
	})
}

func (m *model) openDraft(form createDraft, field int, seeded bool) tea.Cmd {
	m.fold = fold{kind: foldDraft, seeded: seeded}
	m.form, m.field = form, field
	m.defaultDirectories()
	m.groupSelection.prefill(form.group, m.groupChoices(form.group))
	m.inform("")
	return nil
}

func (m *model) seededDraft() tea.Cmd {
	row := m.selectedRow()
	session := row.Session
	launch := launchChoice{}
	switch {
	case declaresLaunch(m.peerLabeled(row.Label), session.LaunchProfile):
		launch = launchChoice{kind: fleetclient.LaunchAgent, profile: session.LaunchProfile}
	case session.LaunchProfile == "" && session.Agent == nil:
		launch = launchChoice{kind: fleetclient.LaunchTerminal}
	}
	field := formCreate
	if launch.kind == "" {
		field = formLaunch
	}
	return m.openDraft(createDraft{machine: row.Label, launch: launch, directory: session.CWD, group: session.Group.String()}, field, true)
}

// The choice is provisional until tab or enter accepts it.
func (m *model) defaultDirectories() {
	m.clearDirectorySearch()
	own := m.form.directory
	if own == "" {
		own = "~"
	}
	choices := []string{own}
	if own != "~" {
		choices = append(choices, "~")
	}
	if peer := m.peerLabeled(m.form.machine); peer != nil {
		counts := make(map[string]int)
		var paths []string
		for _, session := range peer.Sessions {
			path := session.CWD
			if path == "" || slices.Contains(choices, path) {
				continue
			}
			if counts[path] == 0 {
				paths = append(paths, path)
			}
			counts[path]++
		}
		slices.SortFunc(paths, func(a, b string) int {
			if rank := cmp.Compare(counts[b], counts[a]); rank != 0 {
				return rank
			}
			return strings.Compare(a, b)
		})
		choices = append(choices, paths[:min(4, len(paths))]...)
	}
	m.searchDirectories, m.directorySelection.choice = choices, &choices[0]
}

// Keep the selected lifetime even if it disappears while the form is open.
func (m *model) parentChoices() []api.SessionRef {
	var choices []api.SessionRef
	query := strings.Fields(strings.ToLower(m.form.parentQuery))
	for _, peer := range m.peers {
		if !peer.OK {
			continue
		}
		sessions := slices.Clone(peer.Sessions)
		slices.SortStableFunc(sessions, fleetclient.CompareSessions)
		for _, s := range sessions {
			label := strings.ToLower(s.Name + " on " + peer.Label)
			if !slices.ContainsFunc(query, func(word string) bool { return !strings.Contains(label, word) }) {
				choices = append(choices, s.Ref.SessionRef())
			}
		}
	}
	return choices
}
func (m *model) parentChoice() (string, string) {
	if id := m.form.parent; id != nil {
		node := m.tree[*id]
		if node == nil || node.row.Session.Ref.SessionRef() != *id || !node.row.Available {
			return "selected parent unavailable", "selected parent is unavailable. choose a parent or clear the field."
		}
		return node.row.Session.Name + " on " + node.row.Label, ""
	}
	if m.form.parentQuery != "" {
		return "no parent selected", "choose a matching parent or clear the field."
	}
	return "top-level", ""
}

func (m *model) draftRequest() (fleetclient.Request, string, int) {
	machine := m.form.machine
	cwd, terms, problem := parseDirectoryDraft(m.form.directory)
	if problem == "" && len(terms) != 0 {
		problem = "choose a matching directory"
	}
	if problem != "" {
		return fleetclient.Request{}, machine + ": " + problem + "; nothing was sent.", formDirectory
	}
	label, err := group.ParseDraft(m.form.group)
	if err != nil {
		return fleetclient.Request{}, machine + ": " + group.ErrInvalid.Error() + "; nothing was sent.", formGroup
	}
	if m.form.parent != nil && !m.form.parentAccepted {
		return fleetclient.Request{}, "confirm the selected parent before creating.", formParent
	}
	if _, problem := m.parentChoice(); problem != "" {
		return fleetclient.Request{}, problem, formParent
	}
	if refusal := m.hostRefusal(machine); refusal != "" {
		return fleetclient.Request{}, refusal, formCreate
	}
	launch := m.form.launch
	if launch.kind != fleetclient.LaunchTerminal && !declaresLaunch(m.peerLabeled(machine), launch.profile) {
		return fleetclient.Request{}, machine + ": choose a launch; nothing was sent.", formCreate
	}
	if refusal := m.foldRoom(m.fold); refusal != "" {
		return fleetclient.Request{}, refusal, formCreate
	}
	if refusal := m.attachRefusal("enter"); refusal != "" {
		return fleetclient.Request{}, refusal, formCreate
	}
	return fleetclient.Request{Operation: "start", Kind: launch.kind, Machine: machine, Profile: launch.profile, Name: m.form.name, CWD: cwd, Group: label, Parent: m.form.parent}, "", formCreate
}

func (m *model) advanceChoice() (string, string) {
	switch m.field {
	case formDirectory:
		return m.directoryAcceptance()
	case formParent:
		return m.parentChoice()
	case formGroup:
		label, err := m.groupSelection.accept(m.form.group, m.groupChoices(m.form.group))
		if err != nil {
			return "", group.ErrInvalid.Error()
		}
		return label, ""
	default:
		return "", ""
	}
}

func (m *model) advanceForm() tea.Cmd {
	switch m.field {
	case formDirectory:
		m.form.directory, _ = m.directoryAcceptance()
		m.clearDirectorySearch()
	case formParent:
		m.form.parentAccepted = true
	case formGroup:
		m.form.group, _ = m.advanceChoice()
	}
	m.inform("")
	return m.focusForm((m.field + 1) % (formCreate + 1))
}

func (m *model) focusForm(field int) tea.Cmd {
	if m.field == formDirectory && field != formDirectory && m.searchState == directorySearchLoading {
		m.clearDirectorySearch()
	}
	m.field = field
	if field == formGroup {
		m.groupSelection.prefill(m.form.group, m.groupChoices(m.form.group))
	}
	if field == formDirectory && m.searchState == directorySearchIdle && len(m.searchDirectories) == 0 {
		if _, terms, problem := parseDirectoryDraft(m.form.directory); problem == "" && len(terms) == 0 {
			m.defaultDirectories()
		} else {
			return m.searchDirectory()
		}
	}
	return nil
}

func cycleChoice[T comparable](choices []T, current T, direction int) T {
	if len(choices) == 0 {
		return current
	}
	index := slices.Index(choices, current)
	if index < 0 && direction < 0 {
		index = 0
	}
	return choices[(index+len(choices)+direction)%len(choices)]
}

func (m *model) chooseForm(direction int) tea.Cmd {
	switch m.field {
	case formDirectory:
		m.directorySelection.cycle(m.searchDirectories, direction < 0)
	case formParent:
		m.form.parentAccepted = false
		choices := m.parentChoices()
		var choice api.SessionRef
		if m.form.parent != nil {
			choice = *m.form.parent
		}
		if m.form.parentQuery == "" {
			choices = append([]api.SessionRef{{}}, choices...)
		}
		choice = cycleChoice(choices, choice, direction)
		m.form.parent = nil
		if choice != (api.SessionRef{}) {
			m.form.parent = &choice
		}
	case formGroup:
		m.groupSelection.cycle(m.groupChoices(m.form.group), direction < 0)
	case formLaunch:
		if peer := m.peerLabeled(m.form.machine); peer != nil && peer.OK {
			options := make([]launchChoice, 0, len(peer.Profiles)+1)
			for _, profile := range peer.Profiles {
				options = append(options, launchChoice{kind: fleetclient.LaunchAgent, profile: profile.Key})
			}
			m.form.launch = cycleChoice(append(options, launchChoice{kind: fleetclient.LaunchTerminal}), m.form.launch, direction)
		}
	case formMachine:
		var labels []string
		for _, peer := range m.peers {
			if peer.OK {
				labels = append(labels, peer.Label)
			}
		}
		label := cycleChoice(labels, m.form.machine, direction)
		if label != m.form.machine {
			m.form.machine, m.form.directory = label, ""
			if m.form.launch.kind != fleetclient.LaunchTerminal {
				m.form.launch = launchChoice{kind: fleetclient.LaunchTerminal}
				if peer := m.peerLabeled(label); len(peer.Profiles) != 0 {
					m.form.launch = launchChoice{kind: fleetclient.LaunchAgent, profile: peer.Profiles[0].Key}
				}
			}
			m.defaultDirectories()
			m.inform("")
		}
	}
	return nil
}

func (m *model) draftText() string {
	switch m.field {
	case formName:
		return m.form.name
	case formDirectory:
		return m.form.directory
	case formParent:
		return m.form.parentQuery
	case formGroup:
		return m.form.group
	default:
		return ""
	}
}

func (m *model) setDraftText(value string) tea.Cmd {
	switch m.field {
	case formName:
		m.form.name = attention.SingleLine(value)
	case formDirectory:
		m.form.directory = value
		return m.searchDirectory()
	case formParent:
		m.form.parentAccepted = false
		m.form.parentQuery = value
		m.form.parent = nil
		if choices := m.parentChoices(); value != "" && len(choices) > 0 {
			m.form.parent = &choices[0]
		}
	case formGroup:
		m.form.group = value
		m.groupSelection.reset(value, m.groupChoices(value))
	}
	return nil
}

func (m *model) draftKeys() []key {
	advance := key{keys: []string{"tab", "enter"}, show: "tab", label: "uses and continues", rank: 1,
		can: func(m *model) string { _, refusal := m.advanceChoice(); return refusal },
		act: (*model).advanceForm}
	keys := []key{advance,
		{keys: []string{"shift+tab"}, show: "shift-tab", label: "back", rank: 3,
			act: func(m *model) tea.Cmd { return m.focusForm((m.field + formCreate) % (formCreate + 1)) }},
		{keys: []string{"esc"}, show: "escape", label: "cancel", keep: true,
			act: func(m *model) tea.Cmd { m.fold = fold{}; m.clearDirectorySearch(); return nil }},
	}
	switch m.field {
	case formCreate:
		keys[0].keys, keys[0].label = []string{"tab"}, "first field"
		request, refusal, field := m.draftRequest()
		keys = append(keys, key{keys: []string{"enter"}, show: "enter", label: "create", rank: 2,
			can: func(*model) string { return refusal },
			onRefusal: func(m *model) {
				m.field = field
				if field == formGroup {
					m.groupSelection.prefill(m.form.group, m.groupChoices(m.form.group))
				}
			},
			act: func(m *model) tea.Cmd { return m.execute(request, request.Machine) }})
	case formMachine, formLaunch, formDirectory, formGroup, formParent:
		keys = append(keys,
			key{keys: []string{"left"}, show: "←→", label: "choose", rank: 2,
				act: func(m *model) tea.Cmd { return m.chooseForm(-1) }},
			key{keys: []string{"right"}, act: func(m *model) tea.Cmd { return m.chooseForm(1) }},
		)
	}
	if m.field >= formName && m.field < formCreate {
		keys = append(keys, key{keys: []string{"backspace"}, act: func(m *model) tea.Cmd {
			draft := m.draftText()
			if draft != "" {
				_, size := utf8.DecodeLastRuneInString(draft)
				return m.setDraftText(draft[:len(draft)-size])
			}
			return nil
		}})
	}
	switch m.field {
	case formDirectory:
		keys = append(keys, key{keys: []string{"ctrl+u"}, show: "ctrl-u", label: "home", rank: 4,
			act: func(m *model) tea.Cmd { return m.setDraftText("") }})
	case formParent:
		keys = append(keys, key{keys: []string{"ctrl+u"}, show: "ctrl-u", label: "top-level", rank: 4, act: func(m *model) tea.Cmd { return m.setDraftText("") }})
	case formGroup:
		keys = append(keys, key{keys: []string{"ctrl+u"}, show: "ctrl-u", label: "unassigned", rank: 4,
			act: func(m *model) tea.Cmd { return m.setDraftText("") }})
	}
	return keys
}

func (m *model) terminalKeys() []key {
	return []key{
		{keys: []string{"p"}, show: "p", label: "choose parent", rank: 3,
			act: func(m *model) tea.Cmd {
				r := m.fold.request
				return m.openDraft(createDraft{machine: r.Machine, launch: launchChoice{kind: fleetclient.LaunchTerminal}, group: r.Group.String()}, formParent, false)
			}},

		{keys: []string{"enter"}, show: "enter", label: "open terminal", rank: 1, can: (*model).terminalRefusal,
			act: func(m *model) tea.Cmd {
				request := m.fold.request
				m.fold = fold{}
				m.inform("opening terminal on " + request.Machine + "…")
				return m.execute(request, request.Machine)
			}},
		{keys: []string{"left"}, show: "←→", label: "machine", rank: 2,
			act: func(m *model) tea.Cmd { m.chooseTerminalHost(-1); return nil }},
		{keys: []string{"right"},
			act: func(m *model) tea.Cmd { m.chooseTerminalHost(1); return nil }},
		{keys: []string{"esc", "q"}, show: "escape", label: "cancel", keep: true,
			act: func(m *model) tea.Cmd { m.fold = fold{}; return nil }},
	}
}

func (m *model) terminalRefusal() string {
	if refusal := m.hostRefusal(m.fold.request.Machine); refusal != "" {
		return refusal
	}
	if refusal := m.foldRoom(m.fold); refusal != "" {
		return refusal
	}
	return m.attachRefusal("enter")
}

func (m *model) chooseTerminalHost(direction int) {
	machines := m.client.Machines()
	labels := make([]string, len(machines))
	for index, machine := range machines {
		labels[index] = machine.Label
	}
	m.fold.request.Machine = cycleChoice(labels, m.fold.request.Machine, direction)
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
	_, terms, problem := parseDirectoryDraft(m.form.directory)
	m.inform(problem)
	if problem != "" || len(terms) == 0 || m.blurred {
		return nil
	}
	if refusal := m.hostRefusal(m.form.machine); refusal != "" {
		m.inform(refusal)
		return nil
	}
	m.searchState = directorySearchLoading
	ctx, cancel := context.WithCancel(m.ctx)
	m.searchCancel = cancel
	client, machine, revision := m.client, m.form.machine, m.searchRevision
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
func (m *model) directoryAcceptance() (string, string) {
	draft := m.form.directory
	_, terms, problem := parseDirectoryDraft(draft)
	if problem != "" {
		return "", problem
	}
	if choice, _, selected := m.directorySelection.current(m.searchDirectories); selected {
		return choice, ""
	}
	if len(terms) == 0 {
		return draft, ""
	}
	if m.searchState == directorySearchLoading {
		return "", m.directoryStatus()
	}
	return "", "choose a matching directory"
}

func (m *model) directoryStatus() string {
	_, terms, problem := parseDirectoryDraft(m.form.directory)
	if problem != "" || len(terms) == 0 {
		return problem
	}
	if refusal := m.hostRefusal(m.form.machine); refusal != "" {
		return refusal
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
		panic("invalid directory search state")
	}
}
