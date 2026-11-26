package compositor

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
)

const (
	RailColumns  = 28
	SplitColumns = 110
	StageEnded   = 3 // The host ended the connection.
)

//go:embed compositor.conf
var compositorConfig string

var version = regexp.MustCompile(`[0-9]+\.[0-9]+`)

// Launch runs the browser as pane %0 of a new private tmux attached to this
// terminal and returns the browser's exit status.
func Launch(ctx context.Context, tmux, command, config string, input, output *os.File) (status int, result error) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGHUP)
	defer stop()

	probe, probeErr := tmuxCommand(ctx, tmux, "-V").Output()
	if err := ctx.Err(); err != nil {
		return 1, err
	}
	report := strings.TrimSpace(string(probe))
	majorText, minorText, _ := strings.Cut(version.FindString(report), ".")
	major, majorErr := strconv.Atoi(majorText)
	minor, minorErr := strconv.Atoi(minorText)
	if probeErr != nil || majorErr != nil || minorErr != nil {
		return 1, fmt.Errorf("needs tmux 3.8 or later; %s -V printed %q; skid list and enter work without it", tmux, report)
	}
	if major < 3 || major == 3 && minor < 8 {
		return 1, fmt.Errorf("needs tmux 3.8 or later; %s is %d.%d; skid list and enter work without it", tmux, major, minor)
	}

	directory, err := os.MkdirTemp("", "skid-rail-")
	if err != nil {
		return 1, fmt.Errorf("private directory unavailable: %w", err)
	}
	remove := true
	defer func() {
		if remove {
			if err := os.RemoveAll(directory); err != nil {
				status = 1
				result = fmt.Errorf("remove private directory: %w", err)
			}
		}
	}()
	socket := filepath.Join(directory, "tmux")
	conf := filepath.Join(directory, "conf")
	body := strings.NewReplacer("RAIL_COLUMNS", strconv.Itoa(RailColumns), "SPLIT_COLUMNS", strconv.Itoa(SplitColumns)).Replace(compositorConfig)
	if err := os.WriteFile(conf, []byte(body), 0o600); err != nil {
		return 1, fmt.Errorf("private directory unavailable: %w", err)
	}

	args := []string{"-u", "-S", socket, "-T", "RGB", "-f", conf, "new-session", "-s", "skid", "--"}
	args = append(args, paneCommand(command, "rail", config, "--compositor", directory, "--tmux", tmux)...)
	attach := tmuxCommand(ctx, tmux, args...)
	// A normal tmux shutdown restores the outer TTY before its client exits.
	attach.Cancel = func() error { return attach.Process.Signal(syscall.SIGTERM) }
	attach.WaitDelay = 250 * time.Millisecond
	var stderr bytes.Buffer
	attach.Stdin, attach.Stdout, attach.Stderr = input, output, &stderr
	err = attach.Run()

	cleanup, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	// The private server also exits by itself when its attached client goes away.
	if err := tmuxCommand(cleanup, tmux, "-N", "-S", socket, "kill-server").Run(); err != nil {
		// Tmux can exit before the kill and leave the pathname without a listener.
		connection, dialErr := (&net.Dialer{}).DialContext(cleanup, "unix", socket)
		if dialErr == nil {
			_ = connection.Close()
		}
		if !errors.Is(dialErr, os.ErrNotExist) && !errors.Is(dialErr, syscall.ECONNREFUSED) {
			remove = false // keep an address at which the server can still be stopped
			return 1, fmt.Errorf("stop private tmux at %s: %w", socket, err)
		}
	}
	if canceled := ctx.Err(); canceled != nil {
		return 1, canceled
	}
	if err == nil {
		return 0, nil
	}
	if line := attention.SingleLine(strings.TrimSpace(stderr.String())); line != "" {
		return 1, fmt.Errorf("tmux: %s", line)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 1, fmt.Errorf("tmux: %w", err)
}

func tmuxCommand(ctx context.Context, tmux string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, tmux, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	cmd.Env = []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "TMUX=") && !strings.HasPrefix(value, "TMUX_PANE=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	return cmd
}

func paneCommand(command, mode, config string, flags ...string) []string {
	args := []string{command, mode}
	if config != "" {
		args = append(args, "--config", config)
	}
	args = append(args, flags...)
	for i, arg := range args {
		// Tmux consumes a trailing semicolon even in the direct-argv form.
		if strings.HasSuffix(arg, ";") {
			args[i] = strings.TrimSuffix(arg, ";") + `\;`
		}
	}
	return args
}
