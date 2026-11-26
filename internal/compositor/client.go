package compositor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// StageSize gives the pane size for a session entered at this window size.
func StageSize(columns, rows int) (int, int) {
	if columns >= SplitColumns {
		columns -= RailColumns + 1
	}
	return columns, rows
}

type State struct {
	Columns, Rows int
	Visit         string
	Live          bool
	End           string
	ExitStatus    int
	StageKeys     bool
	Returns       int
}

type Client struct {
	directory string
	tmux      string
	command   string
	config    string
	woken     <-chan struct{}
	closeWake func()
	nextVisit int
}

func Open(directory, tmux, command, config string) (*Client, error) {
	client := &Client{directory: directory, tmux: tmux, command: command, config: config}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	if _, err := client.run("set-option", "-s", "@skid_ready", "1"); err != nil {
		signal.Stop(signals)
		return nil, err
	}

	woken := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-signals:
				select {
				case woken <- struct{}{}:
				default:
				}
			}
		}
	}()
	client.woken = woken
	client.closeWake = func() {
		signal.Stop(signals)
		cancel()
		<-done
	}
	return client, nil
}

func (client *Client) Close() { client.closeWake() }

func (client *Client) Woken() <-chan struct{} { return client.woken }

func (client *Client) State() (State, error) {
	const format = "#{window_width}\t#{window_height}\t#{@skid_status}\t#{@skid_returns}\t" +
		"#{@skid_live}\t#{?pane_active,0,1}\t#{@skid_visit}\t#{@skid_end}"
	output, err := client.run("list-panes", "-t", "%0", "-f", "#{==:#{pane_id},%0}", "-F", format)
	if err != nil {
		return State{}, err
	}
	fields := strings.Split(strings.TrimSuffix(output, "\n"), "\t")
	if len(fields) != 8 {
		return State{}, errors.New("invalid compositor state")
	}
	var state State
	for index, target := range []*int{&state.Columns, &state.Rows, &state.ExitStatus, &state.Returns} {
		*target, err = strconv.Atoi(fields[index])
		if err != nil {
			return State{}, errors.New("invalid compositor state")
		}
	}
	if fields[4] != "0" && fields[4] != "1" || fields[5] != "0" && fields[5] != "1" {
		return State{}, errors.New("invalid compositor state")
	}
	switch fields[7] {
	case "", "rail", "narrow", "exit":
	default:
		return State{}, errors.New("invalid compositor state")
	}
	state.Live, state.StageKeys = fields[4] == "1", fields[5] == "1"
	state.Visit, state.End = fields[6], fields[7]
	return state, nil
}

// Visit returns the proposed id even if the tmux result is uncertain.
func (client *Client) Visit(ref string) (string, error) {
	client.nextVisit++
	visit := strconv.Itoa(client.nextVisit)
	outcome := filepath.Join(client.directory, "outcome-"+visit)
	args := []string{
		"if-shell", "-F", "-t", "%0", "#{==:#{window_panes},1}",
		"split-window -dhE -t %0", "if -F '#{@skid_live}' 'skid-retire rail'",
		";", "skid-select-stage",
		";", "skid-visit-layout",
		";", "respawn-pane", "-k", "-t", "%1", "--",
	}
	args = append(args, paneCommand(client.command, "stage", client.config, "--outcome", outcome, "--ref", ref)...)
	args = append(args, ";", "set-option", "-s", "@skid_visit", visit,
		";", "set-option", "-s", "@skid_live", "1",
		";", "set-option", "-s", "@skid_end", "",
		";", "set-option", "-s", "@skid_status", "-1",
		";", "skid-mark-return",
		";", "skid-layout")
	if _, err := client.run(args...); err != nil {
		// A failed spawn can leave the empty stage selected at its proposed size.
		// Layout reads the recorded visit, so it also preserves a successful spawn.
		_, layoutErr := client.run("skid-layout")
		return visit, errors.Join(err, layoutErr)
	}
	return visit, nil
}

func (client *Client) Keys(visit string, stage bool) error {
	action := "skid-rail-keys"
	if stage {
		action = "skid-stage-keys"
	}
	return client.whileLive(visit, action+" ; skid-layout")
}

func (client *Client) Table(visit string) error {
	return client.whileLive(visit, "skid-retire rail ; skid-layout")
}

func (client *Client) whileLive(visit, action string) error {
	condition := "#{&&:#{@skid_live},#{==:#{@skid_visit}," + visit + "}}"
	_, err := client.run("if-shell", "-F", "-t", "%0", condition, action)
	return err
}

func (client *Client) Outcome(visit string) (string, error) {
	path := filepath.Join(client.directory, "outcome-"+visit)
	line, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(line), os.Remove(path)
}

func (client *Client) Paint(gold, quiet string, dim bool) error {
	border := "default"
	if quiet != "" {
		border = "fg=" + quiet
	}
	args := []string{"set-option", "-w", "pane-border-style", border,
		";", "set-option", "-w", "pane-active-border-style", "fg=" + gold,
		";", "set-option", "-p", "-t", "%0", "pane-active-border-style", border}
	if dim {
		args = append(args, ";", "set-option", "-w", "window-style", "fg="+quiet,
			";", "set-option", "-w", "window-active-style", "fg=terminal")
	}
	_, err := client.run(args...)
	return err
}

func (client *Client) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	command := tmuxCommand(ctx, client.tmux, append([]string{"-N", "-u", "-S", filepath.Join(client.directory, "tmux")}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("control compositor: %w", err)
	}
	return string(output), nil
}
