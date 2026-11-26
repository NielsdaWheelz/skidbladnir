package sessionui

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

// A metadata editor owns its draft and exact target through post-write reads,
// including when escape returns to the open row before reconciliation completes.
type metadataEditor struct {
	request      fleetclient.Request
	draft        string
	selection    groupSelection
	saveFocused  bool
	checking     bool
	acknowledged bool
	failure      *fleetclient.Failure
}

func editGuard(kind foldKind, operation string) func(*model) string {
	guard := rowGuard(operation, false)
	return func(m *model) string {
		if m.metadata != nil && m.metadata.checking {
			row := m.selectedRow()
			return row.Session.Name + " on " + row.Label + ": checking previous metadata change; refresh before editing"
		}
		if refusal := guard(m); refusal != "" {
			return refusal
		}
		row := m.selectedRow()
		return m.foldRoom(fold{kind: kind, name: row.Session.Name, machine: row.Label})
	}
}

func (m *model) openMetadata(operation string) {
	row := m.selectedRow()
	m.fold.kind, m.fold.name, m.fold.machine = foldGroup, row.Session.Name, row.Label
	if operation == "rename" {
		m.fold.kind = foldRename
	}
	if editor := m.metadata; editor != nil && editor.request.Operation == operation && editor.request.Ref.SessionEqual(row.Session.Ref.Reference) {
		subject := row.Session.Name + " on " + row.Label + ": "
		if editor.acknowledged {
			m.fail(subject + "metadata changed after saving. review and save again.")
		} else if editor.failure != nil {
			m.fail(subject + editor.failureText())
		}
		return
	}
	editor := &metadataEditor{request: fleetclient.Request{Operation: operation, Ref: row.Session.Ref.Reference}, draft: row.Session.Group.String()}
	if operation == "rename" {
		naming := row.Session.Naming()
		editor.request.ExpectedNaming = &naming
		editor.draft = row.Session.Name
	} else {
		editor.selection.prefill(editor.draft, m.groupChoices(editor.draft))
	}
	m.metadata = editor
	m.inform("")
}

func (m *model) metadataKeys() []key {
	editor := m.metadata
	entries := []key{{keys: []string{"esc"}, show: "escape", label: "open", keep: true,
		act: func(m *model) tea.Cmd {
			m.fold.kind = foldOpen
			if !editor.checking {
				m.metadata = nil
			}
			return nil
		}}}
	if editor.checking {
		return entries
	}
	save := func(automatic bool) key {
		request, refusal, invalidGroup := m.metadataRequest(automatic)
		entry := key{keys: []string{"enter", "ctrl+s"}, show: "enter", label: "save", rank: 1,
			can: func(m *model) string {
				if refusal != "" {
					return refusal
				}
				return m.foldRoom(m.fold)
			},
			act: func(m *model) tea.Cmd {
				editor.request, editor.failure, editor.acknowledged = request, nil, false
				return m.execute(request, m.fold.name+" on "+m.fold.machine)
			}}
		if invalidGroup {
			entry.onRefusal = func(*model) { editor.saveFocused = false }
		}
		return entry
	}
	commit := save(false)
	if m.fold.kind == foldRename {
		automatic := save(true)
		automatic.keys, automatic.show, automatic.label, automatic.rank = []string{"ctrl+a"}, "ctrl-a", "use automatic title", 2
		entries = append(entries, commit, automatic,
			key{keys: []string{"ctrl+u"}, act: func(m *model) tea.Cmd { m.editMetadataText(""); return nil }})
	} else {
		toggle := key{keys: []string{"shift+tab"}, show: "shift-tab", label: "back", rank: 4,
			act: func(m *model) tea.Cmd {
				editor.saveFocused = !editor.saveFocused
				if !editor.saveFocused {
					editor.selection.prefill(editor.draft, m.groupChoices(editor.draft))
				}
				return nil
			}}
		if editor.saveFocused {
			toggle.keys = append(toggle.keys, "tab")
			return append(entries, toggle, commit)
		}
		commit.keys, commit.show, commit.label = []string{"ctrl+s"}, "ctrl-s", "save draft"
		choices := m.groupChoices(editor.draft)
		draft, err := editor.selection.accept(editor.draft, choices)
		entries = append(entries, toggle, commit,
			key{keys: []string{"tab", "enter"}, show: "enter", label: "use and continue", rank: 2,
				can: func(*model) string {
					if err != nil {
						return group.ErrInvalid.Error()
					}
					return ""
				},
				act: func(*model) tea.Cmd {
					editor.draft, editor.saveFocused = draft, true
					editor.selection.prefill(draft, choices)
					return nil
				}},
			key{keys: []string{"left"}, show: "←→", label: "choose", rank: 3,
				act: func(*model) tea.Cmd { editor.selection.cycle(choices, true); return nil }},
			key{keys: []string{"right"}, act: func(*model) tea.Cmd { editor.selection.cycle(choices, false); return nil }},
			key{keys: []string{"ctrl+u"}, show: "ctrl-u", label: "unassigned", rank: 5,
				act: func(m *model) tea.Cmd { m.editMetadataText(""); return nil }})
	}
	return append(entries,
		key{keys: []string{"backspace"}, act: func(m *model) tea.Cmd {
			_, bytes := utf8.DecodeLastRuneInString(editor.draft)
			m.editMetadataText(editor.draft[:len(editor.draft)-bytes])
			return nil
		}})
}

func (m *model) editMetadataText(draft string) {
	editor := m.metadata
	editor.draft = draft
	if m.fold.kind == foldGroup {
		editor.selection.reset(draft, m.groupChoices(draft))
	}
}

func (m *model) metadataRequest(automatic bool) (request fleetclient.Request, refusal string, invalidGroup bool) {
	editor := m.metadata
	request = editor.request
	if editor.checking {
		return request, "checking inventory", false
	}
	if m.fold.kind == foldGroup {
		label, err := group.ParseDraft(editor.draft)
		if err != nil {
			return request, group.ErrInvalid.Error(), true
		}
		request.Group = label
	}
	current := m.rowForReference(request.Ref)
	if !m.rowAvailable(*current) {
		return request, m.rowRefusal(*current, "save"), false
	}
	if m.fold.kind == foldGroup {
		if current.Session.Group == request.Group {
			return request, "unchanged; save disabled", false
		}
		return request, "", false
	}
	naming := fleetclient.Naming{Mode: "manual", Name: editor.draft}
	if automatic {
		naming = fleetclient.Naming{Mode: "automatic"}
	} else if !fleetclient.ValidSessionName(editor.draft) {
		return request, fleetview.ErrorMessage(fleetclient.Failure{Code: "SessionNameInvalid"}, request), false
	}
	if automatic && current.Session.NameMode != "manual" || current.Session.Naming() == naming {
		return request, "unchanged; save disabled", false
	}
	request.Naming = &naming
	return request, "", false
}

func (m *model) completeMetadata(subject string, failure *fleetclient.Failure) {
	editor := m.metadata
	editor.failure = failure
	editor.acknowledged = failure == nil
	editor.checking = m.fold.loss == "" && (failure == nil || failure.Dispatch == "unknown" || accessRejected(failure) ||
		failure.Code == "SessionNotFound" || failure.Code == "SessionIdentityMismatch" || failure.Code == "InternalError" ||
		failure.Code == "SessionNameConflict" || failure.Code == "SessionNameChanged")
	if failure != nil {
		m.fail(subject + ": " + editor.failureText())
		return
	}
	m.inform(subject + ": metadata saved")
}

func (editor *metadataEditor) failureText() string {
	if editor.failure.Dispatch == "unknown" {
		return "metadata change outcome unknown."
	}
	return fleetview.ErrorMessage(*editor.failure, editor.request)
}

func (m *model) reconcileMetadata(machine string, observed []group.Label) {
	editor := m.metadata
	editing := m.fold.metadata()
	if editor == nil || editing && m.fold.loss != "" {
		return
	}
	if editor.request.Operation == "group" {
		editor.selection.retain(groupChoices(editor.draft, observed))
	}
	if m.busy || editor.request.Ref.Machine != machine {
		return
	}
	row := m.rowForReference(editor.request.Ref)
	if !row.Available {
		return
	}
	selected := m.selectedRow()
	visible := m.fold.kind != foldNone && m.fold.kind != foldConfirm && m.fold.kind != foldBranch && selected != nil && editor.request.Ref.SessionEqual(selected.Session.Ref.Reference)
	if editor.checking {
		editor.checking = false
		matches := row.Session.Group == editor.request.Group
		if editor.request.Operation == "rename" {
			matches = row.Session.Naming() == *editor.request.Naming
		}
		uncertain := editor.failure != nil && editor.failure.Dispatch == "unknown"
		if matches && (editor.acknowledged || uncertain) {
			if visible && editing {
				m.fold.kind = foldOpen
			}
			m.metadata = nil
			if !visible {
				return
			}
			subject := row.Session.Name + " on " + row.Label + ": "
			if uncertain {
				m.fail(subject + "request outcome unknown; requested metadata observed")
			} else if editor.request.Operation == "rename" {
				m.inform(subject + "name saved")
				if editor.request.Naming.Mode == "automatic" {
					m.inform(subject + "automatic naming enabled")
				}
			} else if editor.request.Group.IsUnassigned() {
				m.inform(subject + "group cleared")
			} else {
				m.inform(subject + "group assigned")
			}
			return
		}
		if editor.acknowledged && visible {
			m.fail(row.Session.Name + " on " + row.Label + ": metadata changed after saving. review and save again.")
		}
	}
	if editor.request.Operation == "rename" {
		naming := row.Session.Naming()
		if naming != *editor.request.ExpectedNaming {
			editor.request.ExpectedNaming = &naming
			if editor.failure == nil && visible {
				m.fail(row.Session.Name + " on " + row.Label + ": the session name changed. review and save again.")
			}
		}
	}
}
