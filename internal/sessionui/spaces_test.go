package sessionui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/space"
)

func spaceRow(t *testing.T, name, id, label string) fleetclient.Session {
	t.Helper()
	result := row(name, id, 11)
	var err error
	result.Space, err = space.Parse(label)
	if err != nil {
		t.Fatal("invalid synthetic space")
	}
	return result
}

func TestSpaceFilterRetainsUnfilteredOfflineRowsAndExactSelection(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.Update(observation(spaceRow(t, "one", "$1", "alpha"), spaceRow(t, "two", "$2", "beta")))
	m.Update(key("g"))
	m.Update(key("j"))
	m.Update(key("j"))
	m.Update(key("enter"))
	if len(m.rows) != 1 || m.rows[0].session.Name != "one" {
		t.Fatal("named picker did not intersect collection")
	}
	m.Update(inventoryMsg{value: fleetclient.Inventory{Partial: true, Peers: []fleetclient.Peer{{Label: "arch", Machine: "mh-11111111111111111111111111111111", Error: &fleetclient.Failure{Code: "unavailable", Dispatch: "not_sent"}}}}})
	if len(m.rows) != 1 || m.rows[0].available {
		t.Fatal("filtered stale source became actionable")
	}
	m.Update(key("g"))
	m.Update(key("k"))
	m.Update(key("k"))
	m.Update(key("enter"))
	if len(m.rows) != 2 || m.rows[1].session.Name != "two" || m.rows[1].available {
		t.Fatal("space filter erased hidden source rows")
	}
}

func TestSpaceEditorKeepsDraftAndComparesLatestPinnedMembership(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	original := spaceRow(t, "original", "$1", "alpha")
	m.Update(observation(original))
	m.Update(key("e"))
	m.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m.Update(tea.PasteMsg{Content: "beta"})
	updated := spaceRow(t, "renamed", "$1", "beta")
	updated.Agent = nil
	m.Update(observation(updated))
	if m.pending.Ref != original.Ref || m.spaceDraft != "beta" || !strings.Contains(m.View().Content, "unchanged; save disabled") {
		t.Fatal("poll changed editor draft, target, or latest-membership comparison")
	}
	if _, cmd := m.Update(key("enter")); cmd != nil {
		t.Fatal("unchanged latest membership dispatched")
	}
	updated.Space, _ = space.Parse("gamma")
	m.Update(observation(updated))
	if _, cmd := m.Update(key("enter")); cmd == nil || m.pending.Space.String() != "beta" {
		t.Fatal("draft differing from latest membership was disabled")
	}
	m.busy = false
	replaced := spaceRow(t, "replacement", "$2", "gamma")
	m.Update(observation(replaced))
	if m.page != "" || !strings.Contains(m.notice, "unavailable") {
		t.Fatal("replacement lifetime inherited an open editor")
	}
}

func TestSpaceUnknownWaitsForPostWriteReadAndNeverBecomesAcknowledged(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.Update(observation(spaceRow(t, "one", "$1", "alpha")))
	m.Update(key("e"))
	m.spaceDraft = "beta"
	m.Update(key("enter"))
	m.refreshing = true
	m.Update(actionMsg{operation: "space", result: fleetclient.Failed("unavailable", "unknown")})
	if !m.spaceChecking || m.rows[0].available {
		t.Fatal("uncertain write admitted stale source")
	}
	newer := observation(spaceRow(t, "one", "$1", "beta"))
	_, followup := m.Update(newer)
	if followup == nil || !m.spaceChecking || m.rows[0].available {
		t.Fatal("pre-write inventory released membership fence")
	}
	m.Update(newer)
	if m.spaceChecking || m.spaceAcknowledged || m.page != "space-edit" || !strings.Contains(m.notice, "unknown") || m.spaceDraft != "beta" {
		t.Fatal("matching inventory fabricated uncertain acknowledgement or erased draft")
	}
	if _, cmd := m.Update(key("enter")); cmd != nil {
		t.Fatal("matching inventory automatically repeated assignment")
	}
}

func TestSpaceAssignmentKeepsEmptyFilterAndCreateFollowsObservedMembership(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	initial := spaceRow(t, "one", "$1", "alpha")
	m.Update(observation(initial))
	m.spaceFilter = filterFor(initial.Space)
	m.rebuild()
	m.Update(key("e"))
	m.spaceDraft = "beta"
	m.Update(key("enter"))
	m.Update(actionMsg{operation: "space", result: fleetclient.Result{OK: true, Value: json.RawMessage(`{"space":"beta"}`)}})
	m.Update(observation(spaceRow(t, "one", "$1", "beta")))
	if m.spaceFilter.Label() != initial.Space || len(m.rows) != 0 || m.cursor != -1 || !strings.Contains(m.View().Content, "no sessions in this view") {
		t.Fatal("assignment changed filter or lost honest empty state")
	}
	m.machine = "arch"
	created := spaceRow(t, "created", "$2", "gamma")
	ref, _ := fleetclient.DecodeReference(created.Ref)
	ref.Machine = "mh-22222222222222222222222222222222"
	created.Ref = ref.Encode()
	encoded, _ := json.Marshal(fleetclient.ObservedSession{Label: "laptop", Machine: ref.Machine, Session: created})
	m.page, m.busy, m.pending = "create", true, fleetclient.Request{Operation: "start"}
	m.Update(actionMsg{operation: "start", result: fleetclient.Result{OK: true, Value: encoded}})
	if m.machine != "laptop" || m.spaceFilter.Label() != created.Space || m.selectedRow() == nil || m.selectedRow().session.Ref != created.Ref {
		t.Fatal("confirmed creation did not follow returned host and observed membership")
	}
}

func TestTabSelectionSurvivesSpaceRegroup(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.Update(observation(spaceRow(t, "one", "$1", "alpha"), spaceRow(t, "two", "$2", "beta"), spaceRow(t, "three", "$3", "gamma")))
	m.Update(key("l"))
	m.Update(key("l"))
	m.Update(observation(spaceRow(t, "one", "$1", "zulu"), spaceRow(t, "two", "$2", "beta"), spaceRow(t, "three", "$3", "gamma")))
	if m.selectedRow() == nil || m.selectedRow().session.Name != "three" || !strings.Contains(m.View().Content, "[three]") {
		t.Fatal("regroup lost or hid selected session lifetime")
	}
}

func TestMachineScopeRequiresNewReadAndRestoresConfiguredOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.json")
	if os.WriteFile(path, []byte(`{"peers":[{"label":"arch","machine":"mh-11111111111111111111111111111111","origin":"https://first.example:8443","bearer":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},{"label":"laptop","machine":"mh-22222222222222222222222222222222","origin":"https://second.example:8443","bearer":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]}`), 0600) != nil {
		t.Fatal("write private synthetic configuration")
	}
	client, err := fleetclient.Open(path)
	if err != nil {
		t.Fatal("open synthetic configuration")
	}
	m := newModel(context.Background(), client, nil, nil)
	m.Update(key("m"))
	m.Update(key("j"))
	m.Update(key("j"))
	m.Update(key("enter"))
	if m.machine != "laptop" || m.scopeReady {
		t.Fatal("machine scope admitted old inventory")
	}
	first := observation(row("first", "$1", 11))
	if _, cmd := m.Update(first); cmd == nil || m.scopeReady {
		t.Fatal("old all-machine result satisfied new scope")
	}
	second := observation(row("second", "$2", 11))
	second.machine = "laptop"
	second.value.Peers[0].Label = "laptop"
	second.value.Peers[0].Machine = "mh-22222222222222222222222222222222"
	ref, _ := fleetclient.DecodeReference(second.value.Peers[0].Sessions[0].Ref)
	ref.Machine = second.value.Peers[0].Machine
	second.value.Peers[0].Sessions[0].Ref = ref.Encode()
	m.Update(second)
	m.Update(key("m"))
	m.Update(key("k"))
	m.Update(key("k"))
	m.Update(key("enter"))
	if m.scopeReady || m.rows[0].available {
		t.Fatal("returning to all admitted stale scoped rows")
	}
	first.value.Peers = append(first.value.Peers, second.value.Peers...)
	m.Update(first)
	if len(m.rows) != 2 || m.rows[0].label != "arch" || m.rows[1].label != "laptop" {
		t.Fatal("scoped-first inventory changed configured peer order")
	}
}

func TestCreateAndConfirmationDoNotDispatchFromUnavailableSource(t *testing.T) {
	for _, page := range []string{"create", "confirm"} {
		m := newModel(context.Background(), testFleetClient(t), nil, nil)
		m.Update(observation(row("one", "$1", 11)))
		if page == "create" {
			m.Update(key("n"))
			m.form[2] = "created"
			m.field = 4
		} else {
			m.Update(key("x"))
		}
		m.Update(inventoryMsg{failure: &fleetclient.Failure{Code: "unavailable", Dispatch: "not_sent"}})
		if _, cmd := m.Update(key("enter")); cmd != nil {
			t.Errorf("unavailable source admitted pending action: %s", page)
		}
	}
}

func TestDismissedSpaceCheckDoesNotCloseAnotherForm(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	original := observation(spaceRow(t, "one", "$1", "alpha"))
	second := original.value.Peers[0]
	second.Label, second.Machine, second.Sessions = "laptop", "mh-22222222222222222222222222222222", nil
	original.value.Peers = append(original.value.Peers, second)
	m.Update(original)
	m.Update(key("e"))
	m.spaceDraft = "beta"
	m.Update(key("enter"))
	m.Update(actionMsg{operation: "space", result: fleetclient.Result{OK: true, Value: json.RawMessage(`{"space":"beta"}`)}})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(key("n"))
	if m.page != "create" {
		t.Fatal("available other host could not open create")
	}
	m.Update(original)
	if m.page != "create" {
		t.Fatal("dismissed membership check closed unrelated form")
	}
}

func TestUnknownSpaceOutcomeSurvivesFailedInventoryRead(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.Update(observation(spaceRow(t, "one", "$1", "alpha")))
	m.Update(key("e"))
	m.spaceDraft = "beta"
	m.Update(key("enter"))
	m.Update(actionMsg{operation: "space", result: fleetclient.Failed("unavailable", "unknown")})
	m.Update(inventoryMsg{failure: &fleetclient.Failure{Code: "unavailable", Dispatch: "not_sent"}})
	m.Update(observation(spaceRow(t, "one", "$1", "beta")))
	if !strings.Contains(m.View().Content, "unknown") {
		t.Fatal("later inventory erased uncertain write outcome")
	}
}

func TestCreateReplyDoesNotDuplicateAlreadyObservedSession(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	created := spaceRow(t, "created", "$2", "alpha")
	m.Update(observation(row("existing", "$1", 11), created))
	encoded, _ := json.Marshal(fleetclient.ObservedSession{Label: "arch", Machine: "mh-11111111111111111111111111111111", Session: created})
	m.page, m.busy, m.pending = "create", true, fleetclient.Request{Operation: "start"}
	m.Update(actionMsg{operation: "start", result: fleetclient.Result{OK: true, Value: encoded}})
	if len(m.rows) != 2 || m.selectedRow() == nil || m.selectedRow().session.Ref != created.Ref {
		t.Fatal("create reply duplicated already observed lifetime")
	}
}

func TestInvalidSpaceDraftEscapesDisplayWithoutChangingInput(t *testing.T) {
	for _, page := range []string{"create", "space-edit"} {
		m := newModel(context.Background(), testFleetClient(t), nil, nil)
		m.Update(observation(row("one", "$1", 11)))
		if page == "create" {
			m.Update(key("n"))
			m.field = 4
		} else {
			m.Update(key("e"))
		}
		draft := "a\u202eb"
		m.Update(tea.PasteMsg{Content: draft})
		content := m.View().Content
		if strings.Contains(content, draft) || !strings.Contains(content, `\u202e`) || !strings.Contains(content, "display") || !strings.Contains(content, "controls.") {
			t.Errorf("invalid draft affected display: page=%s", page)
		}
		if page == "create" && m.form[4] != draft || page == "space-edit" && m.spaceDraft != draft {
			t.Fatal("rendering modified raw editable draft")
		}
	}
}

func TestSpaceSourceRejectionRequiresOrderedInventory(t *testing.T) {
	for _, code := range []string{"SessionNotFound", "SessionIdentityMismatch", "InternalError", "Unauthenticated", "MachineIdentityMismatch"} {
		m := newModel(context.Background(), testFleetClient(t), nil, nil)
		m.Update(observation(spaceRow(t, "one", "$1", "alpha")))
		m.Update(key("e"))
		m.spaceDraft = "beta"
		m.Update(key("enter"))
		m.refreshing = true
		m.Update(actionMsg{operation: "space", result: fleetclient.Failed(code, "not_sent")})
		if !m.spaceChecking || m.rows[0].available {
			t.Fatal("definite source rejection revived obsolete source admission")
		}
		newer := observation(spaceRow(t, "one", "$1", "alpha"))
		if _, cmd := m.Update(newer); cmd == nil || !m.spaceChecking || m.rows[0].available {
			t.Fatal("pre-write read satisfied source rejection fence")
		}
		m.Update(newer)
		if m.spaceChecking || !m.rows[0].available || m.spaceDraft != "beta" {
			t.Fatal("fresh source read did not preserve editable rejected draft")
		}
	}
}
