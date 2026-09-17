package sessionui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func TestTerminalCreationFormWithoutProfiles(t *testing.T) {
	m := &model{ctx: context.Background(), width: 100, height: 30, cursor: -1}
	observed := observation()
	observed.value.Peers[0].Profiles = []fleetclient.Profile{}
	m.Update(observed)
	m.Update(key("n"))
	if m.page != "create" || !strings.Contains(m.View().Content, "launch: terminal") || !m.createAvailable() {
		t.Fatal("available host with zero profiles did not offer terminal creation")
	}
}

func TestTerminalChoiceSurvivesMachineChangeAndResetsDirectory(t *testing.T) {
	m := &model{ctx: context.Background(), width: 100, height: 30, cursor: -1}
	observed := observation()
	observed.value.Peers = append(observed.value.Peers, fleetclient.Peer{Label: "laptop", Machine: "mh-22222222222222222222222222222222", OK: true, Profiles: []fleetclient.Profile{}})
	m.Update(observed)
	m.Update(key("n"))
	m.field = 1
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if !strings.Contains(m.View().Content, "launch: terminal") {
		t.Fatal("terminal was not appended after advertised profiles")
	}
	m.form[2], m.form[3], m.form[4] = "draft", "/host-specific", "project"
	m.field = 0
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.form[0] != "laptop" || m.form[2] != "draft" || m.form[3] != "~" || m.form[4] != "project" || !strings.Contains(m.View().Content, "launch: terminal") || !m.createAvailable() {
		t.Fatal("machine change lost terminal choice or shared drafts, or retained host directory")
	}
}

func TestShellShortcutPinsSessionAndSuppressesDuplicate(t *testing.T) {
	m := &model{ctx: context.Background(), width: 100, height: 30, cursor: -1}
	source := row("source", "$1", 11)
	source.Agent = nil
	m.Update(observation(source))
	_, create := m.Update(key("T"))
	if create == nil || !m.busy || m.pending.Operation != "shell" || m.pending.Ref != source.Ref {
		t.Fatal("terminal row did not start new terminal here")
	}
	if _, duplicate := m.Update(key("T")); duplicate != nil {
		t.Fatal("pending shell creation accepted another submission")
	}
	m.Update(observation(row("renamed", "$1", 22)))
	if m.pending.Ref != source.Ref {
		t.Fatal("inventory replaced the initiating session reference")
	}
}

func TestCanceledBrowserCannotAdoptLateCreation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &model{ctx: ctx, width: 100, height: 30, cursor: -1}
	m.Update(observation(row("source", "$1", 11)))
	m.Update(key("n"))
	m.form[2], m.field = "late", 4
	if _, submit := m.Update(key("enter")); submit == nil {
		t.Fatal("creation form did not admit the initiating interaction")
	}
	cancel()
	created := row("late", "$2", 22)
	encoded, _ := json.Marshal(fleetclient.ObservedSession{Label: "arch", Machine: "mh-11111111111111111111111111111111", Session: created})
	m.Update(actionMsg{operation: "start", result: fleetclient.Result{OK: true, Value: encoded}})
	if m.page != "create" || m.selectedRow() == nil || m.selectedRow().session.Name != "source" {
		t.Fatal("abandoned browser adopted a late creation")
	}
}

func TestInactiveCreationCompletionCannotReplaceSelection(t *testing.T) {
	m := &model{ctx: context.Background(), width: 100, height: 30, cursor: -1}
	m.Update(observation(row("source", "$1", 11)))
	created := row("late", "$2", 22)
	encoded, _ := json.Marshal(fleetclient.ObservedSession{Label: "arch", Machine: "mh-11111111111111111111111111111111", Session: created})
	m.Update(actionMsg{operation: "start", result: fleetclient.Result{OK: true, Value: encoded}})
	if m.page != "" || m.selectedRow() == nil || m.selectedRow().session.Name != "source" {
		t.Fatal("creation without an active initiating interaction replaced selection")
	}
}
