//go:build live

package live

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/inventory"
)

const (
	p0InventoryFixtureMode  = "SKIDBLADNIR_P0_INVENTORY_FIXTURE_MODE"
	p0InventoryFixtureReady = "SKIDBLADNIR_P0_INVENTORY_FIXTURE_READY"
	p0InventoryProfileHome  = "/profiles/p0-inventory"
)

func TestP0UnmarkedTUIInventoryProcessTrees(t *testing.T) {
	requireExactTmux(t)
	pinned, err := filepath.EvalSymlinks(os.Args[0])
	if err != nil {
		t.Fatalf("resolve live-test fixture executable: %v", err)
	}

	t.Run("pane root", func(t *testing.T) {
		fixture := startP0InventoryFixture(t, "hold", p0InventoryProfileHome, false)
		pane, processes := fixture.awaitFacts(t, pinned, func(pane inventory.PaneFact, processes []inventory.ProcessFact) bool {
			return pane.RootPID == pane.ForegroundProcessGroup && len(p0InventoryExactProcesses(processes, pinned, pane.TTY)) == 1
		})
		want := []inventory.Candidate{p0InventoryCandidate(pane, processes, pane.RootPID)}
		if got := inventory.ClassifyUnmarkedTUIs([]inventory.PaneFact{pane}, processes, pinned, []string{p0InventoryProfileHome}); !reflect.DeepEqual(got, want) {
			t.Fatalf("pane-root inventory = %#v, want %#v; ancestry=%#v", got, want, p0InventoryAncestry(processes, pane.RootPID))
		}
	})

	t.Run("unique foreground process group", func(t *testing.T) {
		fixture := startP0InventoryForegroundFixture(t, p0InventoryProfileHome)
		pane, processes := fixture.awaitFacts(t, pinned, func(pane inventory.PaneFact, processes []inventory.ProcessFact) bool {
			return pane.RootPID != pane.ForegroundProcessGroup && p0InventoryProcess(processes, pane.ForegroundProcessGroup).Executable == pinned
		})
		want := []inventory.Candidate{p0InventoryCandidate(pane, processes, pane.ForegroundProcessGroup)}
		if got := inventory.ClassifyUnmarkedTUIs([]inventory.PaneFact{pane}, processes, pinned, []string{p0InventoryProfileHome}); !reflect.DeepEqual(got, want) {
			t.Fatalf("foreground-group inventory = %#v, want %#v; ancestry=%#v", got, want, p0InventoryAncestry(processes, pane.ForegroundProcessGroup))
		}
	})

	t.Run("nested child exclusion", func(t *testing.T) {
		fixture := startP0InventoryFixture(t, "nested", p0InventoryProfileHome, false)
		pane, processes := fixture.awaitFacts(t, pinned, func(pane inventory.PaneFact, processes []inventory.ProcessFact) bool {
			exact := p0InventoryExactProcesses(processes, pinned, pane.TTY)
			return len(exact) == 2 && pane.RootPID == pane.ForegroundProcessGroup
		})
		exact := p0InventoryExactProcesses(processes, pinned, pane.TTY)
		if exact[0].ParentPID != exact[1].PID && exact[1].ParentPID != exact[0].PID {
			t.Fatalf("fixture exact processes are not a parent/child pair: %#v", exact)
		}
		want := []inventory.Candidate{p0InventoryCandidate(pane, processes, pane.RootPID)}
		if got := inventory.ClassifyUnmarkedTUIs([]inventory.PaneFact{pane}, processes, pinned, []string{p0InventoryProfileHome}); !reflect.DeepEqual(got, want) {
			t.Fatalf("nested-child inventory = %#v, want only pane root %#v; ancestry=%#v", got, want, p0InventoryAncestry(processes, pane.RootPID))
		}
	})

	t.Run("ambiguous pane exclusion", func(t *testing.T) {
		fixture := startP0InventoryFixture(t, "ambiguous", p0InventoryProfileHome, false)
		pane, processes := fixture.awaitFacts(t, pinned, func(pane inventory.PaneFact, processes []inventory.ProcessFact) bool {
			exact := p0InventoryExactProcesses(processes, pinned, pane.TTY)
			return len(exact) == 2 && pane.RootPID != pane.ForegroundProcessGroup && p0InventoryProcess(processes, pane.ForegroundProcessGroup).Executable == pinned
		})
		if got := inventory.ClassifyUnmarkedTUIs([]inventory.PaneFact{pane}, processes, pinned, []string{p0InventoryProfileHome}); len(got) != 0 {
			t.Fatalf("ambiguous pane inventory = %#v, want exclusion", got)
		}
	})

	t.Run("redirected pinned parent exclusion", func(t *testing.T) {
		fixture := startP0InventoryRedirectedParentFixture(t, p0InventoryProfileHome)
		pane, processes := fixture.awaitFacts(t, pinned, func(pane inventory.PaneFact, processes []inventory.ProcessFact) bool {
			child := p0InventoryProcess(processes, pane.ForegroundProcessGroup)
			parent := p0InventoryProcess(processes, child.ParentPID)
			return child.Executable == pinned && child.TTY == pane.TTY && parent.Executable == pinned && parent.TTY != pane.TTY
		})
		if got := inventory.ClassifyUnmarkedTUIs([]inventory.PaneFact{pane}, processes, pinned, []string{p0InventoryProfileHome}); len(got) != 0 {
			t.Fatalf("redirected pinned-parent inventory = %#v, want exclusion", got)
		}
	})

	t.Run("runtime marker exclusion", func(t *testing.T) {
		fixture := startP0InventoryFixture(t, "hold", p0InventoryProfileHome, true)
		pane, processes := fixture.awaitFacts(t, pinned, func(pane inventory.PaneFact, processes []inventory.ProcessFact) bool {
			exact := p0InventoryExactProcesses(processes, pinned, pane.TTY)
			return len(exact) == 1 && exact[0].RuntimeMarked
		})
		if got := inventory.ClassifyUnmarkedTUIs([]inventory.PaneFact{pane}, processes, pinned, []string{p0InventoryProfileHome}); len(got) != 0 {
			t.Fatalf("marked runtime inventory = %#v, want exclusion", got)
		}
	})

	t.Run("unrecognized profile exclusion", func(t *testing.T) {
		fixture := startP0InventoryFixture(t, "hold", "/profiles/not-recognized", false)
		pane, processes := fixture.awaitFacts(t, pinned, func(pane inventory.PaneFact, processes []inventory.ProcessFact) bool {
			return len(p0InventoryExactProcesses(processes, pinned, pane.TTY)) == 1
		})
		if got := inventory.ClassifyUnmarkedTUIs([]inventory.PaneFact{pane}, processes, pinned, []string{p0InventoryProfileHome}); len(got) != 0 {
			t.Fatalf("unrecognized profile inventory = %#v, want exclusion", got)
		}
	})

	t.Run("wrong executable exclusion", func(t *testing.T) {
		fixture := startP0InventorySleepFixture(t)
		pane, processes := fixture.awaitFacts(t, pinned, func(pane inventory.PaneFact, processes []inventory.ProcessFact) bool {
			fact := p0InventoryProcess(processes, pane.RootPID)
			return fact.PID != 0 && fact.Executable != pinned
		})
		if got := inventory.ClassifyUnmarkedTUIs([]inventory.PaneFact{pane}, processes, pinned, []string{p0InventoryProfileHome}); len(got) != 0 {
			t.Fatalf("wrong-executable inventory = %#v, want exclusion", got)
		}
	})

	t.Run("exact pinned ordinary TUI", func(t *testing.T) {
		lock := readDirectTUILock(t)
		profile := directTUIProfile{name: "personal", home: "/home/niels/.codex-personal"}
		assertDirectTUIProfile(t, profile)
		exactPin, err := filepath.EvalSymlinks(lock.BinaryPath)
		if err != nil {
			t.Fatalf("resolve exact pinned Codex executable: %v", err)
		}
		fixture := startP0InventoryExactCodexFixture(t, lock.BinaryPath, profile.home)
		pane, processes := fixture.awaitFacts(t, exactPin, func(pane inventory.PaneFact, processes []inventory.ProcessFact) bool {
			exact := p0InventoryExactProcesses(processes, exactPin, pane.TTY)
			return len(exact) == 1 && exact[0].PID == pane.RootPID && exact[0].ProfileHome == profile.home && !exact[0].RuntimeMarked
		})
		want := []inventory.Candidate{p0InventoryCandidate(pane, processes, pane.RootPID)}
		if got := inventory.ClassifyUnmarkedTUIs([]inventory.PaneFact{pane}, processes, exactPin, []string{profile.home}); !reflect.DeepEqual(got, want) {
			t.Fatalf("exact pinned unmarked TUI inventory = %#v, want %#v; ancestry=%#v", got, want, p0InventoryAncestry(processes, pane.RootPID))
		}
	})
}

func TestP0InventoryProcessFixture(t *testing.T) {
	mode := os.Getenv(p0InventoryFixtureMode)
	if mode == "" {
		t.Skip("fixture subprocess only")
	}
	if mode == "hold" {
		p0InventorySignalReady(t)
		select {}
	}
	if mode != "nested" && mode != "ambiguous" && mode != "redirected-parent" {
		t.Fatalf("unknown inventory fixture mode %q", mode)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestP0InventoryProcessFixture$", "-test.count=1")
	command.Env = p0InventoryEnvironment(os.Environ(), map[string]string{
		p0InventoryFixtureMode:  "hold",
		p0InventoryFixtureReady: "",
	})
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	foregroundTTY := 0
	if mode == "redirected-parent" {
		terminalPath, err := os.Readlink("/proc/self/fd/1")
		if err != nil {
			t.Fatalf("resolve redirected-parent controlling TTY: %v", err)
		}
		terminal, err := os.OpenFile(terminalPath, os.O_RDWR, 0)
		if err != nil {
			t.Fatalf("open redirected-parent controlling TTY: %v", err)
		}
		defer terminal.Close()
		command.Stdin = terminal
		command.Stdout = terminal
		command.Stderr = terminal
		foregroundTTY = int(terminal.Fd())
	}
	if mode == "ambiguous" || mode == "redirected-parent" {
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Foreground: true, Ctty: foregroundTTY}
	}
	if err := command.Start(); err != nil {
		t.Fatalf("start %s inventory child: %v", mode, err)
	}
	p0InventorySignalReady(t)
	if err := command.Wait(); err != nil {
		t.Fatalf("wait for %s inventory child: %v", mode, err)
	}
}

type p0InventoryFixture struct {
	socket    string
	session   string
	server    directTUIServer
	ownedPIDs map[int]uint64
}

func startP0InventoryFixture(t *testing.T, mode, profileHome string, marked bool) *p0InventoryFixture {
	t.Helper()
	ready := filepath.Join(t.TempDir(), "ready")
	environment := []string{
		"CODEX_HOME=" + profileHome,
		p0InventoryFixtureMode + "=" + mode,
		p0InventoryFixtureReady + "=" + ready,
	}
	if marked {
		environment = append(environment, "SKIDBLADNIR_RUNTIME_ID=11111111-1111-4111-8111-111111111111")
	}
	fixture := newP0InventoryFixture(t)
	arguments := []string{"-L", fixture.socket, "-f", "/dev/null", "new-session", "-d", "-s", fixture.session, "-x", "80", "-y", "24"}
	for _, value := range environment {
		arguments = append(arguments, "-e", value)
	}
	arguments = append(arguments, os.Args[0], "-test.run=^TestP0InventoryProcessFixture$", "-test.count=1")
	if output, err := exec.Command("tmux", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("start %s inventory fixture: %v: %s", mode, err, boundedDirectTUIError(output))
	}
	fixture.server = resolveDirectTUIServer(t, fixture.socket)
	waitForP0InventoryReady(t, ready)
	return fixture
}

func startP0InventoryForegroundFixture(t *testing.T, profileHome string) *p0InventoryFixture {
	return startP0InventoryShellFixture(t, profileHome, "hold", false)
}

func startP0InventoryRedirectedParentFixture(t *testing.T, profileHome string) *p0InventoryFixture {
	return startP0InventoryShellFixture(t, profileHome, "redirected-parent", true)
}

func startP0InventoryShellFixture(t *testing.T, profileHome, mode string, redirectStdin bool) *p0InventoryFixture {
	t.Helper()
	ready := filepath.Join(t.TempDir(), "ready")
	fixture := newP0InventoryFixture(t)
	arguments := []string{
		"-L", fixture.socket, "-f", "/dev/null", "new-session", "-d", "-s", fixture.session, "-x", "80", "-y", "24",
		"-e", "CODEX_HOME=" + profileHome,
		"-e", p0InventoryFixtureMode + "=" + mode,
		"-e", p0InventoryFixtureReady + "=" + ready,
		"/bin/bash", "--noprofile", "--norc", "-i",
	}
	if output, err := exec.Command("tmux", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("start foreground inventory shell: %v: %s", err, boundedDirectTUIError(output))
	}
	fixture.server = resolveDirectTUIServer(t, fixture.socket)
	command := shellQuote(os.Args[0]) + " -test.run=^TestP0InventoryProcessFixture$ -test.count=1"
	if redirectStdin {
		command += " < /dev/null"
	}
	if output, err := exec.Command("tmux", "-L", fixture.socket, "send-keys", "-t", fixture.session+":0.0", "-l", command).CombinedOutput(); err != nil {
		t.Fatalf("type foreground inventory command: %v: %s", err, boundedDirectTUIError(output))
	}
	if output, err := exec.Command("tmux", "-L", fixture.socket, "send-keys", "-t", fixture.session+":0.0", "Enter").CombinedOutput(); err != nil {
		t.Fatalf("run foreground inventory command: %v: %s", err, boundedDirectTUIError(output))
	}
	waitForP0InventoryReady(t, ready)
	return fixture
}

func startP0InventorySleepFixture(t *testing.T) *p0InventoryFixture {
	t.Helper()
	fixture := newP0InventoryFixture(t)
	arguments := []string{
		"-L", fixture.socket, "-f", "/dev/null", "new-session", "-d", "-s", fixture.session, "-x", "80", "-y", "24",
		"-e", "CODEX_HOME=" + p0InventoryProfileHome,
		"/usr/bin/sleep", "60",
	}
	if output, err := exec.Command("tmux", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("start wrong-executable inventory fixture: %v: %s", err, boundedDirectTUIError(output))
	}
	fixture.server = resolveDirectTUIServer(t, fixture.socket)
	return fixture
}

func startP0InventoryExactCodexFixture(t *testing.T, binary, profileHome string) *p0InventoryFixture {
	t.Helper()
	fixture := newP0InventoryFixture(t)
	arguments := []string{
		"-L", fixture.socket, "-f", "/dev/null", "new-session", "-d", "-s", fixture.session, "-x", "100", "-y", "30", "-c", directTUICWD,
		"-e", "CODEX_HOME=" + profileHome,
		binary, "--strict-config", "--dangerously-bypass-approvals-and-sandbox", "-C", directTUICWD,
	}
	command := exec.Command("tmux", arguments...)
	command.Env = p0InventoryEnvironment(withoutTmuxEnvironment(os.Environ()), map[string]string{"SKIDBLADNIR_RUNTIME_ID": ""})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start exact pinned unmarked TUI: %v: %s", err, boundedDirectTUIError(output))
	}
	fixture.server = resolveDirectTUIServer(t, fixture.socket)
	return fixture
}

func newP0InventoryFixture(t *testing.T) *p0InventoryFixture {
	t.Helper()
	fixture := &p0InventoryFixture{
		socket:    fmt.Sprintf("skidbladnir_inventory_%d_%d", os.Getpid(), time.Now().UnixNano()),
		session:   "inventory",
		ownedPIDs: make(map[int]uint64),
	}
	t.Cleanup(func() {
		if fixture.server.process.pid > 0 {
			cleanupDirectTUIServer(t, fixture.socket, fixture.server)
		}
		for pid, start := range fixture.ownedPIDs {
			if processStartTime(pid) == start {
				_ = syscall.Kill(pid, syscall.SIGKILL) // justify-ignore-error: exact test-owned PID/start cleanup accepts an already-exited fixture.
			}
		}
		for pid, start := range fixture.ownedPIDs {
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) && processStartTime(pid) == start {
				time.Sleep(20 * time.Millisecond)
			}
			if processStartTime(pid) == start {
				t.Errorf("inventory fixture PID survived cleanup: %d/%d", pid, start)
			}
		}
	})
	return fixture
}

func (fixture *p0InventoryFixture) awaitFacts(t *testing.T, pinned string, ready func(inventory.PaneFact, []inventory.ProcessFact) bool) (inventory.PaneFact, []inventory.ProcessFact) {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		pane, err := p0InventoryPaneFact(fixture.socket, fixture.session)
		if err == nil {
			processes := p0InventoryProcessFacts(t)
			fixture.recordOwnedProcesses(processes, pinned, pane.TTY)
			if ready(pane, processes) {
				return pane, processes
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("inventory fixture did not reach the required process-tree state")
	return inventory.PaneFact{}, nil
}

func (fixture *p0InventoryFixture) recordOwnedProcesses(processes []inventory.ProcessFact, pinned, tty string) {
	byPID := make(map[int]inventory.ProcessFact, len(processes))
	for _, process := range processes {
		byPID[process.PID] = process
		if process.Executable == pinned && process.TTY == tty {
			fixture.ownedPIDs[process.PID] = process.StartTime
		}
	}
	for pid := range fixture.ownedPIDs {
		parent := byPID[pid].ParentPID
		for parent > 0 {
			ancestor, exists := byPID[parent]
			if !exists {
				break
			}
			if ancestor.Executable == pinned {
				fixture.ownedPIDs[ancestor.PID] = ancestor.StartTime
			}
			parent = ancestor.ParentPID
		}
	}
}

func p0InventoryPaneFact(socket, session string) (inventory.PaneFact, error) {
	output, err := exec.Command("tmux", "-L", socket, "list-panes", "-t", session, "-F", "#{pane_id}|#{pane_pid}|#{pane_tty}").Output()
	if err != nil {
		return inventory.PaneFact{}, err
	}
	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) != 3 {
		return inventory.PaneFact{}, fmt.Errorf("invalid pane facts %q", output)
	}
	pid, err := strconv.Atoi(parts[1])
	if err != nil {
		return inventory.PaneFact{}, err
	}
	fields, err := procStatFieldsForLive(pid)
	if err != nil || len(fields) < 6 {
		return inventory.PaneFact{}, fmt.Errorf("read pane-root stat: %w", err)
	}
	foreground, err := strconv.Atoi(fields[5])
	if err != nil {
		return inventory.PaneFact{}, err
	}
	return inventory.PaneFact{ID: parts[0], RootPID: pid, TTY: parts[2], ForegroundProcessGroup: foreground}, nil
}

func p0InventoryProcessFacts(t *testing.T) []inventory.ProcessFact {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatalf("read /proc for inventory fixture: %v", err)
	}
	var facts []inventory.ProcessFact
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		processTTY, _ := os.Readlink(filepath.Join("/proc", entry.Name(), "fd", "0"))
		fields, err := procStatFieldsForLive(pid)
		if err != nil || len(fields) < 20 {
			continue
		}
		parent, parentErr := strconv.Atoi(fields[1])
		group, groupErr := strconv.Atoi(fields[2])
		start, startErr := strconv.ParseUint(fields[19], 10, 64)
		executable, executableErr := filepath.EvalSymlinks(filepath.Join("/proc", entry.Name(), "exe"))
		if parentErr != nil || groupErr != nil || startErr != nil {
			continue
		}
		if executableErr != nil && !(pid == 1 && parent == 0) {
			continue
		}
		environment, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "environ"))
		values := strings.Split(strings.TrimRight(string(environment), "\x00"), "\x00")
		facts = append(facts, inventory.ProcessFact{
			PID:           pid,
			ParentPID:     parent,
			ProcessGroup:  group,
			TTY:           processTTY,
			Executable:    executable,
			ProfileHome:   p0InventoryEnvironmentValue(values, "CODEX_HOME"),
			RuntimeMarked: p0InventoryEnvironmentValue(values, "SKIDBLADNIR_RUNTIME_ID") != "",
			StartTime:     start,
		})
	}
	sort.Slice(facts, func(left, right int) bool { return facts[left].PID < facts[right].PID })
	return facts
}

func p0InventoryExactProcesses(processes []inventory.ProcessFact, pinned, tty string) []inventory.ProcessFact {
	var exact []inventory.ProcessFact
	for _, process := range processes {
		if process.Executable == pinned && process.TTY == tty {
			exact = append(exact, process)
		}
	}
	return exact
}

func p0InventoryProcess(processes []inventory.ProcessFact, pid int) inventory.ProcessFact {
	for _, process := range processes {
		if process.PID == pid {
			return process
		}
	}
	return inventory.ProcessFact{}
}

func p0InventoryAncestry(processes []inventory.ProcessFact, pid int) []inventory.ProcessFact {
	var ancestry []inventory.ProcessFact
	seen := make(map[int]struct{})
	for pid > 0 {
		if _, exists := seen[pid]; exists {
			break
		}
		seen[pid] = struct{}{}
		process := p0InventoryProcess(processes, pid)
		if process.PID == 0 {
			break
		}
		ancestry = append(ancestry, process)
		pid = process.ParentPID
	}
	return ancestry
}

func p0InventoryCandidate(pane inventory.PaneFact, processes []inventory.ProcessFact, pid int) inventory.Candidate {
	process := p0InventoryProcess(processes, pid)
	return inventory.Candidate{PaneID: pane.ID, PID: pid, StartTime: process.StartTime, TTY: pane.TTY, ProfileHome: process.ProfileHome}
}

func p0InventorySignalReady(t *testing.T) {
	t.Helper()
	path := os.Getenv(p0InventoryFixtureReady)
	if path == "" {
		return
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("signal inventory fixture readiness: %v", err)
	}
}

func waitForP0InventoryReady(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("inventory fixture did not signal readiness")
}

func p0InventoryEnvironment(environment []string, replacements map[string]string) []string {
	result := make([]string, 0, len(environment)+len(replacements))
	for _, value := range environment {
		name, _, ok := strings.Cut(value, "=")
		if ok {
			if _, replaced := replacements[name]; replaced {
				continue
			}
		}
		result = append(result, value)
	}
	for name, value := range replacements {
		if value != "" {
			result = append(result, name+"="+value)
		}
	}
	return result
}

func p0InventoryEnvironmentValue(environment []string, name string) string {
	prefix := name + "="
	for _, value := range environment {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return ""
}
