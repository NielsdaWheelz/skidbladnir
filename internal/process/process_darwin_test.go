//go:build integration && darwin && cgo

package process

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

const (
	unlinkedExecutableHelper  = "SKIDBLADNIR_UNLINKED_EXECUTABLE_HELPER"
	processTestHelperLifetime = 30 * time.Second
)

func TestObserveTreatsUnlinkedDarwinExecutableAsObservationBoundary(t *testing.T) {
	if os.Getenv(unlinkedExecutableHelper) == "1" {
		if _, err := os.Stdout.WriteString("ready\n"); err != nil {
			panic("unlinked-executable helper could not announce readiness") // justify-defect: the private test protocol requires this write.
		}
		time.Sleep(processTestHelperLifetime)
		return
	}

	source, err := os.Executable()
	if err != nil {
		t.Fatal("resolve process test executable")
	}
	contents, err := os.ReadFile(source)
	if err != nil {
		t.Fatal("read process test executable")
	}
	executable := filepath.Join(t.TempDir(), "unlinked-helper")
	if err := os.WriteFile(executable, contents, 0o700); err != nil {
		t.Fatal("create private process test executable")
	}
	command := exec.Command(executable, "-test.run=^TestObserveTreatsUnlinkedDarwinExecutableAsObservationBoundary$")
	command.Env = append(os.Environ(), unlinkedExecutableHelper+"=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal("create unlinked-executable helper stdout pipe")
	}
	if err := command.Start(); err != nil {
		t.Fatal("start unlinked-executable helper")
	}
	t.Cleanup(func() {
		_ = command.Process.Kill() // justify-ignore-error: test cleanup accepts an already-exited helper process.
		_ = command.Wait()         // justify-ignore-error: an intentional cleanup kill returns the helper exit status.
	})
	if ready, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || ready != "ready\n" {
		t.Fatal("unlinked-executable helper did not become ready")
	}
	if err := os.Remove(executable); err != nil {
		t.Fatal("unlink running process test executable")
	}

	if _, err := Observe(PID(command.Process.Pid)); !errors.Is(err, ErrProcessNotPermitted) {
		t.Fatalf("observing a process with an unlinked executable = %v, want ErrProcessNotPermitted", err)
	}
}

func TestObservePreservesEmptyDarwinArguments(t *testing.T) {
	if os.Getenv("SKIDBLADNIR_EMPTY_ARGV_HELPER") == "1" {
		if _, err := os.Stdout.WriteString("ready\n"); err != nil {
			panic("empty-argument helper could not announce readiness") // justify-defect: the private test protocol requires this write.
		}
		time.Sleep(processTestHelperLifetime)
		return
	}

	expected := []string{
		os.Args[0],
		"-test.run=^TestObservePreservesEmptyDarwinArguments$",
		"--",
		"before",
		"",
		"after",
	}
	command := exec.Command(expected[0], expected[1:]...)
	command.Env = append(os.Environ(), "SKIDBLADNIR_EMPTY_ARGV_HELPER=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal("create empty-argument helper stdout pipe")
	}
	if err := command.Start(); err != nil {
		t.Fatal("start empty-argument helper")
	}
	t.Cleanup(func() {
		_ = command.Process.Kill() // justify-ignore-error: test cleanup accepts an already-exited helper process.
		_ = command.Wait()         // justify-ignore-error: an intentional cleanup kill returns the helper exit status.
	})
	if ready, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || ready != "ready\n" {
		t.Fatal("empty-argument helper did not become ready")
	}

	observed, err := Observe(PID(command.Process.Pid))
	if err != nil {
		t.Fatal("observe empty-argument helper process")
	}
	if !slices.Equal(observed.Argv, expected) {
		t.Fatalf("observed argument vector = %q, want %q", observed.Argv, expected)
	}
}
