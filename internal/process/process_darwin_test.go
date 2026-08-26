//go:build integration && darwin && cgo

package process

import (
	"bufio"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"
)

func TestObservePreservesEmptyDarwinArguments(t *testing.T) {
	if os.Getenv("SKIDBLADNIR_EMPTY_ARGV_HELPER") == "1" {
		_, _ = os.Stdout.WriteString("ready\n")
		time.Sleep(30 * time.Second)
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
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	if ready, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || ready != "ready\n" {
		t.Fatal("empty-argument helper did not become ready")
	}

	observed, err := Observe(PID(command.Process.Pid))
	if err != nil {
		t.Fatal("observe empty-argument helper process")
	}
	if !slices.Equal(observed.Argv, expected) {
		t.Fatalf("observed argument vector did not preserve empty arguments: argument_count=%d want=%d", len(observed.Argv), len(expected))
	}
}
