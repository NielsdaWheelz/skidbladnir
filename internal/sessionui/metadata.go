package sessionui

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

// A metadata editor owns its draft and exact target through post-write reads,
// including when escape returns to info before reconciliation completes.
type metadataEditor struct {
	request      fleetclient.Request
	draft        string
	checking     bool
	acknowledged bool
	failure      *fleetclient.Failure
}

func (m *model) openMetadata(operation string) {
	if m.metadata != nil && m.metadata.checking {
		m.inform("checking previous metadata change; refresh before editing")
		return
	}
	row := m.rowForReference(m.pageRow.session.Ref)
	if row == nil || !row.available {
		m.inform("session unavailable; refresh before editing")
		return
	}
	if m.metadata != nil && m.metadata.request.Operation == operation {
		target, _ := fleetclient.DecodeReference(m.metadata.request.Ref)
		current, _ := fleetclient.DecodeReference(row.session.Ref)
		if target.SessionEqual(current) {
			m.page = "group-edit"
			if operation == "rename" {
				m.page = "name-edit"
			}
			if m.metadata.acknowledged {
				m.fail("metadata changed after saving. review and save again.")
			} else if m.metadata.failure != nil {
				m.fail(fleetclient.ErrorMessage(*m.metadata.failure, m.metadata.request, false))
			}
			return
		}
	}
	editor := &metadataEditor{request: fleetclient.Request{Operation: operation, Ref: row.session.Ref}, draft: row.session.Group.String()}
	m.page = "group-edit"
	if operation == "rename" {
		naming := row.session.Naming()
		editor.request.ExpectedNaming = &naming
		editor.draft = row.session.Name
		m.page = "name-edit"
	}
	m.metadata = editor
	m.inform("")
}

func (m *model) editMetadata(key tea.KeyPressMsg) tea.Cmd {
	editor := m.metadata
	if key.String() == "esc" {
		m.page = "details"
		if !editor.checking {
			m.metadata = nil
		}
		return nil
	}
	if editor.checking {
		return nil
	}
	switch key.String() {
	case "enter", "ctrl+s", "ctrl+a":
		current := m.rowForReference(editor.request.Ref)
		if current == nil || !current.available {
			m.inform("session unavailable; refresh before saving")
			return nil
		}
		if editor.request.Operation == "group" {
			if key.String() == "ctrl+a" {
				return nil
			}
			label, err := group.ParseDraft(editor.draft)
			if err != nil {
				m.fail(group.ErrInvalid.Error())
				return nil
			}
			if current.session.Group == label {
				return nil
			}
			editor.request.Group = label
		} else {
			naming := fleetclient.Naming{Mode: "manual", Name: editor.draft}
			if key.String() == "ctrl+a" {
				if current.session.NameMode != "manual" {
					return nil
				}
				naming = fleetclient.Naming{Mode: "automatic"}
			} else if !fleetclient.ValidSessionName(editor.draft) {
				m.fail(fleetclient.ErrorMessage(fleetclient.Failure{Code: "SessionNameInvalid"}, editor.request, false))
				return nil
			}
			if current.session.Naming() == naming {
				return nil
			}
			editor.request.Naming = &naming
		}
		editor.failure = nil
		editor.acknowledged = false
		return m.execute(editor.request)
	case "ctrl+u":
		editor.draft = ""
	case "left", "right":
		if editor.request.Operation == "group" {
			editor.draft = m.nextGroupDraft(editor.draft, key.String() == "left")
		}
	case "backspace":
		if editor.draft != "" {
			_, size := utf8.DecodeLastRuneInString(editor.draft)
			editor.draft = editor.draft[:len(editor.draft)-size]
		}
	default:
		editor.draft += key.Text
	}
	return nil
}

func (m *model) reconcileMetadata() {
	editor := m.metadata
	if editor == nil || m.busy {
		return
	}
	row := m.rowForReference(editor.request.Ref)
	ref, _ := fleetclient.DecodeReference(editor.request.Ref)
	pageRef, _ := fleetclient.DecodeReference(m.pageRow.session.Ref)
	visible := (m.page == "details" || m.page == "name-edit" || m.page == "group-edit") && ref.SessionEqual(pageRef)
	if row == nil {
		for _, peer := range m.scopedPeers() {
			if peer.Machine == ref.Machine && peer.OK {
				if m.page == "name-edit" || m.page == "group-edit" {
					m.page = "details"
				}
				m.metadata = nil
				if visible {
					m.fail("session unavailable; editor closed")
				}
				return
			}
		}
		return
	}
	if !row.available {
		return
	}
	if editor.checking {
		editor.checking = false
		matches := row.session.Group == editor.request.Group
		if editor.request.Operation == "rename" {
			matches = row.session.Naming() == *editor.request.Naming
		}
		uncertain := editor.failure != nil && editor.failure.Dispatch == "unknown"
		if matches && (editor.acknowledged || uncertain) {
			if m.page == "name-edit" || m.page == "group-edit" {
				m.page = "details"
			}
			m.metadata = nil
			if !visible {
				return
			}
			if uncertain {
				m.fail("request outcome unknown; requested metadata observed")
			} else if editor.request.Operation == "rename" {
				m.inform("name saved")
				if editor.request.Naming.Mode == "automatic" {
					m.inform("automatic naming enabled")
				}
			} else if editor.request.Group.IsUnassigned() {
				m.inform("group cleared")
			} else {
				m.inform("group assigned")
			}
			return
		}
		if editor.acknowledged && visible {
			m.fail("metadata changed after saving. review and save again.")
		}
	}
	if editor.request.Operation == "rename" {
		naming := row.session.Naming()
		if naming != *editor.request.ExpectedNaming {
			editor.request.ExpectedNaming = &naming
			if editor.failure == nil && visible {
				m.fail("the session name changed. review and save again.")
			}
		}
	}
}
