package inventory

import (
	"reflect"
	"testing"
)

func TestClassifyUnmarkedTUIs(t *testing.T) {
	t.Parallel()
	const (
		pinned = "/opt/codex-0.149.1/bin/codex"
		home   = "/profiles/personal"
		tty    = "/dev/pts/40"
	)
	pane := PaneFact{ID: "%40", RootPID: 100, TTY: tty, ForegroundProcessGroup: 100}
	process := func(pid, parent, group int) ProcessFact {
		return ProcessFact{
			PID:          pid,
			ParentPID:    parent,
			ProcessGroup: group,
			TTY:          tty,
			Executable:   pinned,
			ProfileHome:  home,
			StartTime:    uint64(pid * 100),
		}
	}
	namespaceRoot := ProcessFact{PID: 1, ParentPID: 0, ProcessGroup: 1, StartTime: 1}
	candidate := func(pid int) Candidate {
		return Candidate{PaneID: pane.ID, PID: pid, StartTime: uint64(pid * 100), TTY: tty, ProfileHome: home}
	}

	tests := []struct {
		name      string
		pane      PaneFact
		processes []ProcessFact
		want      []Candidate
	}{
		{
			name:      "exact pane root",
			pane:      pane,
			processes: []ProcessFact{process(100, 1, 100), namespaceRoot},
			want:      []Candidate{candidate(100)},
		},
		{
			name:      "missing parent ancestry is unproved",
			pane:      pane,
			processes: []ProcessFact{process(100, 99, 100)},
		},
		{
			name:      "non-root zero parent is not namespace root",
			pane:      pane,
			processes: []ProcessFact{process(100, 0, 100)},
		},
		{
			name: "unreadable parent ancestry is unproved",
			pane: pane,
			processes: []ProcessFact{
				process(100, 99, 100),
				{PID: 99, ParentPID: 1, ProcessGroup: 99, StartTime: 9900},
			},
		},
		{
			name: "unique foreground process group leader",
			pane: PaneFact{ID: pane.ID, RootPID: 90, TTY: tty, ForegroundProcessGroup: 100},
			processes: []ProcessFact{
				{PID: 90, ParentPID: 1, ProcessGroup: 90, TTY: tty, Executable: "/bin/bash", StartTime: 9000},
				process(100, 90, 100),
				namespaceRoot,
			},
			want: []Candidate{candidate(100)},
		},
		{
			name: "nested non-leader is excluded without hiding pane root",
			pane: pane,
			processes: []ProcessFact{
				process(100, 1, 100),
				process(101, 100, 100),
				namespaceRoot,
			},
			want: []Candidate{candidate(100)},
		},
		{
			name: "sole foreground candidate nested below Codex is excluded",
			pane: PaneFact{ID: pane.ID, RootPID: 90, TTY: tty, ForegroundProcessGroup: 101},
			processes: []ProcessFact{
				{PID: 90, ParentPID: 1, ProcessGroup: 90, TTY: tty, Executable: "/bin/bash", StartTime: 9000},
				process(100, 90, 100),
				process(101, 100, 101),
				namespaceRoot,
			},
		},
		{
			name: "pane root and distinct foreground candidate are ambiguous",
			pane: PaneFact{ID: pane.ID, RootPID: 100, TTY: tty, ForegroundProcessGroup: 101},
			processes: []ProcessFact{
				process(100, 1, 100),
				process(101, 1, 101),
			},
		},
		{
			name: "runtime marker",
			pane: pane,
			processes: []ProcessFact{
				func() ProcessFact {
					fact := process(100, 1, 100)
					fact.RuntimeMarked = true
					return fact
				}(),
			},
		},
		{
			name: "wrong executable",
			pane: pane,
			processes: []ProcessFact{
				func() ProcessFact {
					fact := process(100, 1, 100)
					fact.Executable = "/usr/bin/codex"
					return fact
				}(),
			},
		},
		{
			name: "unrecognized profile home",
			pane: pane,
			processes: []ProcessFact{
				func() ProcessFact {
					fact := process(100, 1, 100)
					fact.ProfileHome = "/profiles/foreign"
					return fact
				}(),
			},
		},
		{
			name: "wrong TTY",
			pane: pane,
			processes: []ProcessFact{
				func() ProcessFact {
					fact := process(100, 1, 100)
					fact.TTY = "/dev/pts/41"
					return fact
				}(),
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyUnmarkedTUIs([]PaneFact{test.pane}, test.processes, pinned, []string{home})
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ClassifyUnmarkedTUIs() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestClassifyUnmarkedTUIsReturnsStablePaneOrder(t *testing.T) {
	t.Parallel()
	panes := []PaneFact{
		{ID: "%9", RootPID: 9, TTY: "/dev/pts/9", ForegroundProcessGroup: 9},
		{ID: "%2", RootPID: 2, TTY: "/dev/pts/2", ForegroundProcessGroup: 2},
	}
	processes := []ProcessFact{
		{PID: 9, ParentPID: 1, ProcessGroup: 9, TTY: "/dev/pts/9", Executable: "/pin", ProfileHome: "/home", StartTime: 90},
		{PID: 2, ParentPID: 1, ProcessGroup: 2, TTY: "/dev/pts/2", Executable: "/pin", ProfileHome: "/home", StartTime: 20},
		{PID: 1, ParentPID: 0, ProcessGroup: 1, Executable: "/sbin/init", StartTime: 1},
	}
	want := []Candidate{
		{PaneID: "%2", PID: 2, StartTime: 20, TTY: "/dev/pts/2", ProfileHome: "/home"},
		{PaneID: "%9", PID: 9, StartTime: 90, TTY: "/dev/pts/9", ProfileHome: "/home"},
	}
	if got := ClassifyUnmarkedTUIs(panes, processes, "/pin", []string{"/home"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("ClassifyUnmarkedTUIs() = %#v, want stable order %#v", got, want)
	}
}

func TestClassifyUnmarkedTUIsRejectsBlankPin(t *testing.T) {
	t.Parallel()
	pane := PaneFact{ID: "%1", RootPID: 10, TTY: "/dev/pts/1", ForegroundProcessGroup: 10}
	processes := []ProcessFact{{PID: 10, ParentPID: 0, ProcessGroup: 10, TTY: pane.TTY, ProfileHome: "/home", StartTime: 100}}
	if got := ClassifyUnmarkedTUIs([]PaneFact{pane}, processes, "", []string{"/home"}); len(got) != 0 {
		t.Fatalf("blank pin inventory = %#v, want fail-closed exclusion", got)
	}
}
