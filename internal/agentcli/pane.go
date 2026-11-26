package agentcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/compositor"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessionui"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"golang.org/x/term"
)

func openFleet(config string) (string, *fleetclient.Client, error) {
	if config == "" {
		var err error
		config, err = attention.StateDirectory()
		if err != nil {
			return "", nil, err
		}
	}
	config, err := filepath.Abs(config)
	if err != nil {
		return "", nil, err
	}
	client, err := fleetclient.Open(config)
	return config, client, err
}

func launchBrowser(ctx context.Context, config string, input, output *os.File, stderr io.Writer) int {
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		fmt.Fprintln(stderr, "skid browser: needs tmux 3.8 or later on PATH; skid list and enter work without it")
		return 1
	}
	command, err := exec.LookPath(os.Args[0])
	if err == nil {
		command, err = filepath.Abs(command)
	}
	if err != nil {
		fmt.Fprintf(stderr, "skid browser: command path unavailable: %v\n", err)
		return 1
	}
	status, err := compositor.Launch(ctx, tmux, command, config, input, output)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintf(stderr, "skid browser: %v\n", err)
		}
		return 1
	}
	if status != 0 {
		fmt.Fprintln(stderr, "session browser ended with an error")
		return 1
	}
	return 0
}

// RunPane dispatches the compositor's private commands before public parsing.
func RunPane(ctx context.Context, pane string, args []string, input *os.File, stdout io.Writer) int {
	output, ok := stdout.(*os.File)
	if !ok || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return 2
	}
	first, second := "--compositor", "--tmux"
	if pane == "stage" {
		first, second = "--outcome", "--ref"
	}
	if len(args)%2 != 0 {
		return 2
	}
	flags := make(map[string]string, 3)
	for i := 0; i < len(args); i += 2 {
		name, value := args[i], args[i+1]
		switch name {
		case "--config", first, second:
		default:
			return 2
		}
		if value == "" || flags[name] != "" {
			return 2
		}
		flags[name] = value
	}
	if flags[first] == "" || flags[second] == "" {
		return 2
	}
	if pane == "stage" {
		return runStage(ctx, flags["--config"], flags["--outcome"], flags["--ref"], input, output)
	}

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	config, client, err := openFleet(flags["--config"])
	if err != nil {
		return 1
	}
	comp, err := compositor.Open(flags["--compositor"], flags["--tmux"], os.Args[0], config)
	if err != nil {
		return 1
	}
	defer comp.Close()
	applyCtx, cancelApply := context.WithTimeout(ctx, 10*time.Second)
	applied := make(chan struct{})
	go func() {
		defer close(applied)
		_ = client.Apply(applyCtx) // cached direct access stays available; skid status reports pending configuration.
	}()
	defer func() { cancelApply(); <-applied }()
	if sessionui.Run(ctx, client, input, output, comp) != nil {
		return 1
	}
	return 0
}

func runStage(ctx context.Context, config, outcome, encodedRef string, input, output *os.File) int {
	var result terminalclient.Result
	ref, err := fleetclient.DecodeReference(encodedRef)
	if err != nil {
		result.Err = &fleetclient.Failure{Code: "invalid_input", Dispatch: "not_sent"}
	} else if _, client, err := openFleet(config); err != nil {
		result.Err = &fleetclient.Failure{Code: "configuration_invalid", Dispatch: "not_sent"}
	} else {
		// Tmux 3.8's server_client_check_pane_resize defers a follow-up pane
		// resize ioctl for 250 ms. Wait past that interval for the final size.
		const settle = 275 * time.Millisecond
		result = terminalclient.RunStage(ctx, client, ref, input, output, settle)
	}
	temporary, err := os.CreateTemp(filepath.Dir(outcome), ".outcome-")
	if err != nil {
		return 1
	}
	defer os.Remove(temporary.Name())
	_, writeErr := io.WriteString(temporary, stageOutcome(result))
	closeErr := temporary.Close()
	if writeErr != nil || closeErr != nil || os.Rename(temporary.Name(), outcome) != nil {
		return 1
	}
	if errors.Is(result.Err, terminalclient.ErrClientEnded) {
		return compositor.StageEnded
	}
	return 1
}

func stageOutcome(result terminalclient.Result) string {
	errs := []error{result.Err}
	if joined, ok := result.Err.(interface{ Unwrap() []error }); ok {
		errs = joined.Unwrap()
	}
	parts := make([]string, 0, len(errs)+1)
	for _, err := range errs {
		var failure *fleetclient.Failure
		switch {
		case err == nil:
			parts = append(parts, "terminal detached")
		case errors.Is(err, terminalclient.ErrClientEnded):
			parts = append(parts, "the host ended the terminal connection; work may continue")
		case errors.As(err, &failure):
			parts = append(parts, fleetview.ErrorMessage(*failure, fleetclient.Request{}))
		default:
			parts = append(parts, err.Error())
		}
	}
	if result.NotificationErr != nil {
		parts = append(parts, "notifications unavailable")
	}
	line := attention.SingleLine(strings.Join(parts, "; "))
	if limit := 512; len(line) > limit {
		for !utf8.RuneStart(line[limit]) {
			limit--
		}
		line = line[:limit]
	}
	return line
}
