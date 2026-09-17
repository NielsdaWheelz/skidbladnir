package sessionui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/charmbracelet/x/ansi"
)

func TestBrowserNavigationTargetsVisibleSessionWithoutDispatch(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	first, second := spaceRow(t, "alpha", "$1", "project"), spaceRow(t, "bravo", "$2", "other")
	m.Update(observation(first, second))
	if _, cmd := m.Update(key("l")); cmd != nil || m.selectedRow().session.Name != "alpha" {
		t.Fatal("right arrow did not select the next grouped tab locally")
	}
	m.Update(key("g"))
	if _, cmd := m.Update(key("j")); cmd != nil || len(m.rows) != 0 || !strings.Contains(m.View().Content, "no sessions in this view") {
		t.Fatal("space arrows did not immediately filter to unassigned")
	}
	if _, cmd := m.Update(key("T")); cmd != nil {
		t.Fatal("space focus admitted a session action")
	}
	m.Update(key("a"))
	if _, cmd := m.Update(key("j")); cmd != nil || m.selectedRow().session.Name != "alpha" || m.spaceFilter.Label() != first.Space {
		t.Fatal("agent arrow did not reveal that exact session and space")
	}
	m.Update(key("t"))
	if m.busy || m.selectedRow().session.Ref != first.Ref {
		t.Fatal("tabs focus changed selection or invoked old shell key")
	}
	m.Update(key("x"))
	if m.pending.Ref != first.Ref || !strings.Contains(m.View().Content, "kill alpha on arch") {
		t.Fatal("action target differs from selected summary")
	}
}

func TestBrowserReadSnapshotRetainsCapturedHeader(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.Update(observation(row("original", "$1", 11), row("next", "$2", 22)))
	m.Update(key("r"))
	m.Update(observation(row("next", "$2", 22)))
	encoded, _ := json.Marshal(fleetclient.ReadResult{Text: "fixture snapshot", Source: "terminal", Scope: "visible", Truncated: true})
	m.Update(actionMsg{operation: "read", result: fleetclient.Result{OK: true, Value: encoded}})
	if !strings.Contains(m.View().Content, "original on arch") || !strings.Contains(m.View().Content, "truncated: true") {
		t.Fatal("snapshot lost its captured identity or coverage")
	}
}

func TestBrowserMinimumLayoutAndModalInput(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.width, m.height = 80, 24
	sessions := []fleetclient.Session{}
	for i := 0; i < 7; i++ {
		sessions = append(sessions, spaceRow(t, fmt.Sprintf("agent-%d", i), fmt.Sprintf("$%d", i+1), fmt.Sprintf("project-%d", i%4)))
	}
	m.Update(observation(sessions...))
	content := m.View().Content
	for _, wanted := range []string{"spaces", "agents", "tabs", "agent-6", "project-3", "T terminal here", "t tabs"} {
		if !strings.Contains(content, wanted) {
			t.Errorf("working-set layout omits %q", wanted)
		}
	}
	assertScreenBounds(t, m)
	m.Update(key("n"))
	m.field = 3
	m.Update(tea.PasteMsg{Content: strings.Repeat("/long-directory", 30)})
	if !strings.Contains(m.View().Content, "> directory:") || !strings.Contains(m.View().Content, "escape cancels") {
		t.Fatal("long focused form field hid its label or controls")
	}
	assertScreenBounds(t, m)
	m.Update(tea.WindowSizeMsg{Width: 79, Height: 24})
	previous := m.form
	m.Update(tea.PasteMsg{Content: "ignored"})
	m.Update(key("z"))
	if m.form != previous || !strings.Contains(m.View().Content, "80") {
		t.Fatal("undersize browser accepted edits or hid size requirement")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.page != "" {
		t.Fatal("undersize modal cannot cancel")
	}
}

func assertScreenBounds(t *testing.T, m *model) {
	t.Helper()
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) > m.height {
		t.Errorf("screen rows=%d, height=%d", len(lines), m.height)
	}
	for i, line := range lines {
		if ansi.StringWidth(line) > m.width {
			t.Errorf("row %d width=%d exceeds %d", i, ansi.StringWidth(line), m.width)
		}
	}
}

func TestBrowserAgentOrderFreezesAndDoesNotInventSelection(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	idle, blocked, arrival := row("idle", "$1", 11), row("blocked", "$2", 22), row("arrival", "$3", 33)
	blocked.Agent.Status.State = "blocked"
	arrival.Agent.Status.State = "failed"
	shell := row("shell", "$4", 44)
	shell.Agent = nil
	m.Update(observation(idle, blocked, shell))
	m.Update(key("a"))
	m.Update(key("k"))
	if m.selectedRow().session.Name != "blocked" {
		t.Fatal("agent order did not prioritize blocker")
	}
	blocked.Agent.Status.State = "idle"
	idle.Agent.Status.State = "failed"
	m.Update(observation(idle, blocked, arrival, shell))
	m.Update(key("j"))
	if m.selectedRow().session.Name != "idle" {
		t.Fatal("status refresh reordered focused survivors")
	}
	m.Update(key("j"))
	if m.selectedRow().session.Name != "arrival" {
		t.Fatal("arrival did not append after focused survivors")
	}
	m.Update(key("t"))
	m.Update(key("a"))
	m.Update(key("k"))
	if m.selectedRow().session.Name != "idle" {
		t.Fatal("leaving agent focus did not restore status order")
	}
	m.Update(observation(blocked, arrival, shell))
	if m.selectedRow().session.Name != "blocked" {
		t.Fatal("removed selection did not clamp prior tab position")
	}
	m.Update(key("t"))
	m.Update(key("l"))
	m.Update(key("l"))
	m.Update(key("a"))
	if m.selectedRow().session.Name != "shell" || !strings.Contains(m.View().Content, "no active agent") {
		t.Fatal("focusing agents invented an agent selection for a shell")
	}
	if _, cmd := m.Update(key("enter")); cmd != nil {
		t.Fatal("agent focus attached the shell without active row")
	}
	m.Update(key("j"))
	if m.selectedRow().session.Name != "arrival" {
		t.Fatal("first down from no active agent did not select first status row")
	}
}

func TestBrowserFocusModalAndExplicitFilterSelection(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	initial := observation(spaceRow(t, "first", "$1", "alpha"), spaceRow(t, "second", "$2", "alpha"), spaceRow(t, "third", "$3", "beta"), spaceRow(t, "fourth", "$4", "beta"))
	m.Update(initial)
	m.Update(key("l"))
	m.Update(key("l"))
	m.Update(key("l"))
	original := m.selectedRow().session.Ref
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyTab}, {Code: tea.KeyTab}, {Code: tea.KeyTab, Mod: tea.ModShift}, {Code: tea.KeyTab, Mod: tea.ModShift}} {
		m.Update(key)
		if m.selectedRow().session.Ref != original {
			t.Fatal("focus cycling changed selected session")
		}
	}
	m.Update(key("g"))
	m.Update(key("j"))
	m.Update(key("j"))
	if m.selectedRow().session.Name != "first" {
		t.Fatal("explicit mismatching filter retained old tab index instead of first")
	}
	m.Update(key("enter"))
	m.Update(key("l"))
	m.Update(key("g"))
	m.Update(key("k"))
	m.Update(key("k"))
	if m.selectedRow().session.Name != "first" {
		t.Fatal("empty-to-all filter did not select first tab")
	}
	m.Update(key("a"))
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m.Update(key("t"))
	if m.page != "details" || m.selectedRow().session.Name != "first" {
		t.Fatal("modal accepted global navigation")
	}
	m.Update(key("q"))
	m.Update(key("j"))
	if m.selectedRow().session.Name != "second" {
		t.Fatal("modal close lost initiating agent focus")
	}
	m.Update(key("g"))
	m.Update(key("k"))
	m.Update(key("k"))
	m.Update(key("enter"))
	m.Update(key("l"))
	m.Update(observation(initial.value.Peers[0].Sessions[0], initial.value.Peers[0].Sessions[2], initial.value.Peers[0].Sessions[3]))
	if m.selectedRow().session.Name != "third" {
		t.Fatal("refresh removal failed to retain prior clamped tab index")
	}
}

func TestBrowserOverflowAndUnavailableMetadata(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.width, m.height = 80, 24
	sessions := []fleetclient.Session{}
	for index := 0; index < 18; index++ {
		sessions = append(sessions, spaceRow(t, fmt.Sprintf("agent-%02d-界界界界界界界界", index), fmt.Sprintf("$%d", index), fmt.Sprintf("project-%02d-long-label", index)))
	}
	m.Update(observation(sessions...))
	for range 17 {
		m.Update(key("l"))
	}
	if !strings.Contains(m.View().Content, "[agent-17-") || !strings.Contains(m.View().Content, "<") {
		t.Fatal("overflow hid selected tab or its overflow indicator")
	}
	m.Update(key("a"))
	if !strings.Contains(m.View().Content, "* agent-17-") {
		t.Fatal("agent viewport did not reveal selected lifetime")
	}
	m.Update(key("g"))
	for range 19 {
		m.Update(key("j"))
	}
	if !strings.Contains(m.View().Content, "* space: project-17") {
		t.Fatal("space viewport hid selected filter")
	}
	assertScreenBounds(t, m)
	m.Update(key("t"))
	m.Update(inventoryMsg{failure: &fleetclient.Failure{Code: "unavailable", Dispatch: "not_sent"}})
	if _, cmd := m.Update(key("enter")); cmd != nil {
		t.Fatal("stale row admitted remote attachment")
	}
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if m.page != "details" || !strings.Contains(strings.Join(m.text, "\n"), "agent-17") {
		t.Fatal("unavailable metadata was not locally readable")
	}
	m.Update(key("q"))
	m.Update(key("g"))
	for range 18 {
		m.Update(key("k"))
	}
	if len(m.rows) != 0 || !strings.Contains(m.View().Content, "arch: unavailable") || !strings.Contains(m.View().Content, "no matching sessions in available inventory") {
		t.Fatal("empty filter concealed unavailable source or overstated emptiness")
	}
	assertScreenBounds(t, m)
}

func TestBrowserLongIdentityKeepsEffectCoverageAndOutcomeVisible(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.width, m.height = 80, 24
	session := row(strings.Repeat("long-session-", 100), "$1", 11)
	observed := observation(session)
	observed.value.Peers[0].Label = strings.Repeat("long-machine-", 100)
	m.Update(observed)
	m.Update(key("x"))
	if !strings.Contains(m.View().Content, "close this session.") {
		t.Fatal("long target hid destructive confirmation effect")
	}
	m.Update(key("n"))
	m.Update(key("r"))
	encoded, _ := json.Marshal(fleetclient.ReadResult{Text: "snapshot body", Source: "terminal", Scope: "visible", Truncated: true})
	m.Update(actionMsg{operation: "read", result: fleetclient.Result{OK: true, Value: encoded}})
	content := m.View().Content
	if !strings.Contains(content, "truncated: true") || !strings.Contains(content, "snapshot body") {
		t.Fatal("long captured identity hid coverage or output body")
	}
	m.page = ""
	m.notice = "delivery unknown; not repeated"
	m.Update(inventoryMsg{value: fleetclient.Inventory{Peers: []fleetclient.Peer{{Label: observed.value.Peers[0].Label, Machine: observed.value.Peers[0].Machine, OK: false}}}})
	if !strings.Contains(m.View().Content, "delivery unknown; not repeated") {
		t.Fatal("long unavailable host label concealed operation outcome")
	}
	assertScreenBounds(t, m)
}

func TestBrowserPageScrollUsesVisibleSnapshotCapacity(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.width, m.height = 80, 24
	m.Update(observation(row("snapshot", "$1", 11)))
	m.Update(key("r"))
	var body strings.Builder
	for i := 0; i < 70; i++ {
		fmt.Fprintf(&body, "snapshot-line-%02d\n", i)
	}
	encoded, _ := json.Marshal(fleetclient.ReadResult{Text: body.String(), Source: "terminal", Scope: "visible"})
	m.Update(actionMsg{operation: "read", result: fleetclient.Result{OK: true, Value: encoded}})
	m.notice = "a retained operation notice"
	before := m.View().Content
	last := -1
	for i := 0; i < 70; i++ {
		if strings.Contains(before, fmt.Sprintf("snapshot-line-%02d", i)) {
			last = i
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if last < 0 || !strings.Contains(m.View().Content, fmt.Sprintf("snapshot-line-%02d", last+1)) {
		t.Fatal("page-down skipped output hidden behind browser chrome")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	if !strings.Contains(m.View().Content, "snapshot-line-00") {
		t.Fatal("page-up did not return to preceding visible page")
	}
}
