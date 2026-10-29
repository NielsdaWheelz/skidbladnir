package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() { os.Exit(run()) }

func run() int {
	if len(os.Args) != 2 || !filepath.IsAbs(os.Args[1]) {
		fmt.Fprintln(os.Stderr, "usage: skid-terminals /absolute/path/to/tmux")
		return 1
	}
	stopping := make(chan os.Signal, 1)
	signal.Notify(stopping, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(stopping)
	// -c bypasses a foreign server's default-client-command on collision.
	// Its shell can still run normal shell initialization before exiting.
	child := exec.Command(os.Args[1], "-D", "-c", "exit 78")
	// Keep the inherited process group: launchd cleans it if this host dies.
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "the terminal runtime could not start tmux.")
		return 1
	}
	completed := make(chan error, 1)
	go func() { completed <- child.Wait() }()
	select {
	case err := <-completed:
		if failure, ok := err.(*exec.ExitError); ok && failure.ExitCode() == 78 {
			fmt.Fprintln(os.Stderr, "the default tmux server is not managed by skid. complete terminal-service setup on this machine.")
			return 0 // successful exits are not restarted by the launch agent.
		}
		return 1 // launchd owns restart after unexpected completion, including exit 0.
	case <-stopping:
		_ = child.Process.Signal(syscall.SIGTERM)
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-completed:
		case <-timer.C:
			_ = child.Process.Kill()
			<-completed
		}
		return 0
	}
}
