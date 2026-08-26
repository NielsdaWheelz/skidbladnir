//go:build integration

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const execChurnHelper = "SKIDBLADNIR_PROCESS_EXEC_CHURN_HELPER"

func TestObserveNeverCombinesFactsAcrossExec(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal("resolve process test executable")
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal("canonicalize process test executable")
	}
	if os.Getenv(execChurnHelper) == "1" {
		command := fmt.Sprintf("exec %s -test.run=^TestObserveNeverCombinesFactsAcrossExec$", shellQuote(executable))
		if err := syscall.Exec("/bin/sh", []string{"/bin/sh", "-c", command}, os.Environ()); err != nil {
			panic("exec-churn helper could not replace its process")
		}
	}

	helper := exec.Command(executable, "-test.run=^TestObserveNeverCombinesFactsAcrossExec$")
	helper.Env = append(os.Environ(), execChurnHelper+"=1")
	if err := helper.Start(); err != nil {
		t.Fatal("start exec-churn helper")
	}
	t.Cleanup(func() {
		_ = helper.Process.Kill()
		_ = helper.Wait()
	})

	deadline := time.Now().Add(3 * time.Second)
	successes := 0
	for attempts := 0; attempts < 20_000 && time.Now().Before(deadline); attempts++ {
		observed, observeErr := Observe(PID(helper.Process.Pid))
		if observeErr != nil {
			continue
		}
		successes++
		isHelper := observed.Executable == executable && len(observed.Argv) >= 2 && observed.Argv[0] == executable && observed.Argv[1] == "-test.run=^TestObserveNeverCombinesFactsAcrossExec$"
		isShell := observed.Executable != executable && len(observed.Argv) >= 3 && observed.Argv[0] == "/bin/sh" && observed.Argv[1] == "-c" && strings.Contains(observed.Argv[2], executable)
		if !isHelper && !isShell {
			t.Fatal("process observation combined facts across exec boundaries")
		}
	}
	if successes < 100 {
		t.Fatalf("only %d coherent observations succeeded during exec churn", successes)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
