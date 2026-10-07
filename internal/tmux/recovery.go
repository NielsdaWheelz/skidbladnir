package tmux

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

const maximumWorkspaceBytes = 8 << 20

var layoutPanePattern = regexp.MustCompile(`[0-9]+x[0-9]+,[0-9]+,[0-9]+,([0-9]+)([,\]}>]|$)`)

// ServerObservation is read-only. An externally started server may have no
// canonical epoch yet; capture and restoration require a canonical identity.
type ServerObservation struct {
	Identity     ServerIdentity
	SessionCount int
}

// Workspace keys belong to this snapshot, never to a later server. NativeGroup
// identifies membership only: tmux owns the names of reconstructed groups.
type Workspace struct {
	Server   ServerIdentity     `json:"-"`
	Sessions []WorkspaceSession `json:"sessions"`
	Windows  []WorkspaceWindow  `json:"windows"`
}

type WorkspaceSession struct {
	Key               string          `json:"key"`
	Name              string          `json:"name"`
	Character         string          `json:"character"`
	Group             string          `json:"group,omitempty"`
	NativeGroup       string          `json:"nativeGroup,omitempty"`
	ActiveWindowIndex int             `json:"activeWindowIndex"`
	Links             []WorkspaceLink `json:"links"`
}

type WorkspaceLink struct {
	Index     int    `json:"index"`
	WindowKey string `json:"windowKey"`
}

type WorkspaceWindow struct {
	Key           string          `json:"key"`
	Name          string          `json:"name"`
	Width         int             `json:"width"`
	Height        int             `json:"height"`
	Layout        string          `json:"layout"`
	ActivePaneKey string          `json:"activePaneKey"`
	Zoomed        bool            `json:"zoomed"`
	Panes         []WorkspacePane `json:"panes"`
}

type WorkspacePane struct {
	Key string `json:"key"`
	CWD string `json:"cwd"`
}

func (client Client) InspectServer(ctx context.Context) (ServerObservation, bool, error) {
	var stdout, stderr bytes.Buffer
	command := client.commandWithStderr(ctx, &stdout, &stderr, "-N", "display-message", "-p",
		"#{"+ServerEpochOption+"}|#{pid}|#{start_time}|#{server_sessions}")
	if err := command.Run(); err != nil {
		if missingServer(err, stderr.String()) {
			return ServerObservation{}, false, nil
		}
		return ServerObservation{}, false, fmt.Errorf("inspect tmux server: %w", err)
	}
	fields := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "|")
	if len(fields) != 4 || len(fields[0]) > 128 || !serverPIDPattern.MatchString(fields[1]) || !startTimePattern.MatchString(fields[2]) {
		return ServerObservation{}, false, errors.New("tmux server observation is invalid")
	}
	count, err := strconv.Atoi(fields[3])
	if err != nil || count < 0 {
		return ServerObservation{}, false, errors.New("tmux server session count is invalid")
	}
	return ServerObservation{Identity: ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}, SessionCount: count}, true, nil
}

func missingServer(err error, stderr string) bool {
	var exitError *exec.ExitError
	message := strings.TrimSpace(stderr)
	return errors.As(err, &exitError) && exitError.ExitCode() == 1 &&
		(strings.HasPrefix(message, "no server running on ") || strings.HasPrefix(message, "error connecting to ") && strings.HasSuffix(message, "(No such file or directory)"))
}

func (client Client) StartRecoveryServer(ctx context.Context) error {
	return client.Run(ctx, "start-recovery-server", "start-server")
}

func (client Client) ValidateRecoveryServer(ctx context.Context, expected ServerIdentity) error {
	_, err := client.recoveryShell(ctx, expected)
	return err
}

func (client Client) recoveryShell(ctx context.Context, expected ServerIdentity) (string, error) {
	format := recoveryFields("@skid_server_epoch", "pid", "start_time", "server_sessions", "default-shell", "default-command", "exit-empty", "exit-unattached")
	output, err := client.Output(ctx, "validate-recovery-server", "-N", "display-message", "-p", format)
	if err != nil {
		return "", err
	}
	fields, rest, err := parseRecoveryFields([]byte(output), 8)
	if err != nil || len(rest) != 0 {
		return "", errors.New("tmux recovery configuration observation is invalid")
	}
	if (ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}) != expected || fields[3] != "0" || fields[5] != "" || fields[6] != "0" || fields[7] != "0" {
		return "", errors.New("tmux recovery destination or configuration changed")
	}
	shell := fields[4]
	if !filepath.IsAbs(shell) || filepath.Base(shell) != "bash" && filepath.Base(shell) != "zsh" {
		return "", errors.New("tmux recovery shell must be absolute bash or zsh")
	}
	info, err := os.Stat(shell)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", errors.New("tmux recovery shell is not executable")
	}
	return shell, nil
}

// RestoreWorkspace requires a nonempty recipe admitted by ValidateWorkspace and
// the manager's metadata/cwd policy. It uses only freshly returned native ids.
// Any error can leave visible partial work; the manager must never replay it.
func (client Client) RestoreWorkspace(ctx context.Context, server ServerIdentity, recipe Workspace) (Workspace, error) {
	if !server.valid() {
		panic("invalid recovery destination identity") // justify-defect: the manager initialized and admitted the canonical destination.
	}
	shell, err := client.recoveryShell(ctx, server)
	if err != nil {
		return Workspace{}, err
	}
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return Workspace{}, err
	}
	prefix := "skid-recovery-" + hex.EncodeToString(entropy[:])
	sessionIDs, windowIDs, paneIDs := make(map[string]string), make(map[string]string), make(map[string]string)
	leaders, representatives := make(map[string]string), make(map[string]string)
	placeholders := make(map[string]bool)
	for index, session := range recipe.Sessions {
		name := prefix + "-" + strconv.Itoa(index)
		var branch string
		leader := leaders[session.NativeGroup]
		if session.NativeGroup != "" && leader != "" {
			branch = commandText("new-session", "-d", "-P", "-F", "#{session_id}|#{window_id}|#{window_index}", "-s", name, "-t", leader)
			representatives[session.Key] = leader
		} else {
			window := recipe.Windows[0]
			for _, candidate := range recipe.Windows {
				if candidate.Key == session.Links[0].WindowKey {
					window = candidate
					break
				}
			}
			terminal, err := client.TerminalCommand(window.Panes[0].CWD, "")
			if err != nil {
				return Workspace{}, err
			}
			args := []string{"-d", "-P", "-F", "#{session_id}|#{window_id}|#{window_index}", "-s", name, "-x", strconv.Itoa(window.Width), "-y", strconv.Itoa(window.Height), "-c", formatLiteral(window.Panes[0].CWD)}
			branch = commandText("new-session", append(args, terminal...)...)
		}
		branch += " ; " + commandText("set-option", "-t", "="+name+":", "renumber-windows", "off")
		condition := serverLifetimeConditions(server)
		if index == 0 {
			condition = append(condition, "#{==:#{server_sessions},0}", "#{==:#{default-shell},"+formatLiteral(shell)+"}", "#{==:#{default-command},}", "#{==:#{exit-empty},0}", "#{==:#{exit-unattached},0}")
		}
		output, err := client.recoveryQueue(ctx, condition, branch)
		if err != nil {
			return Workspace{}, err
		}
		fields := strings.Split(output, "|")
		if len(fields) != 3 || !sessionIDPattern.MatchString(fields[0]) || !workspaceID(fields[1], '@') {
			return Workspace{}, errors.New("tmux recovery session receipt is invalid")
		}
		if _, err := strconv.Atoi(fields[2]); err != nil {
			return Workspace{}, errors.New("tmux recovery placeholder index is invalid")
		}
		sessionIDs[session.Key], placeholders[fields[1]] = fields[0], true
		if representatives[session.Key] == "" {
			representatives[session.Key] = fields[0]
			if session.NativeGroup != "" {
				leaders[session.NativeGroup] = fields[0]
			}
		}
	}
	// A native singleton group needs a temporary second member to establish
	// membership. Close only that fresh auxiliary id after synchronization.
	for group, leader := range leaders {
		count := 0
		for _, session := range recipe.Sessions {
			if session.NativeGroup == group {
				count++
			}
		}
		if count != 1 {
			continue
		}
		output, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("new-session", "-d", "-P", "-F", "#{session_id}", "-s", prefix+"-aux", "-t", leader))
		if err != nil {
			return Workspace{}, err
		}
		if !sessionIDPattern.MatchString(output) {
			return Workspace{}, errors.New("tmux recovery auxiliary receipt is invalid")
		}
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("kill-session", "-t", output)); err != nil {
			return Workspace{}, err
		}
	}
	// tmux rejects linking between members of the same native group. One
	// ungrouped source supports every link, including repeated indexed links.
	first := recipe.Windows[0]
	terminal, err := client.TerminalCommand(first.Panes[0].CWD, "")
	if err != nil {
		return Workspace{}, err
	}
	name := prefix + "-workspace"
	args := []string{"-d", "-P", "-F", "#{session_id}|#{window_index}", "-s", name, "-x", strconv.Itoa(first.Width), "-y", strconv.Itoa(first.Height), "-c", formatLiteral(first.Panes[0].CWD)}
	output, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("new-session", append(args, terminal...)...)+" ; "+commandText("set-option", "-t", "="+name+":", "renumber-windows", "off"))
	if err != nil {
		return Workspace{}, err
	}
	fields := strings.Split(output, "|")
	if len(fields) != 2 || !sessionIDPattern.MatchString(fields[0]) {
		return Workspace{}, errors.New("tmux recovery staging receipt is invalid")
	}
	placeholderIndex, err := strconv.Atoi(fields[1])
	if err != nil {
		return Workspace{}, errors.New("tmux recovery staging index is invalid")
	}
	stagingID := fields[0]
	creationIndices := make(map[string]int)
	index := 0
	for _, window := range recipe.Windows {
		if index == placeholderIndex {
			index++
		}
		terminal, err := client.TerminalCommand(window.Panes[0].CWD, "")
		if err != nil {
			return Workspace{}, err
		}
		args := []string{"-d", "-P", "-F", "#{window_id}|#{pane_id}", "-t", stagingID + ":" + strconv.Itoa(index), "-n", prefix, "-c", formatLiteral(window.Panes[0].CWD)}
		output, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("new-window", append(args, terminal...)...))
		if err != nil {
			return Workspace{}, err
		}
		fields := strings.Split(output, "|")
		if len(fields) != 2 || !workspaceID(fields[0], '@') || !paneIDPattern.MatchString(fields[1]) {
			return Workspace{}, errors.New("tmux recovery window receipt is invalid")
		}
		windowIDs[window.Key], paneIDs[window.Panes[0].Key] = fields[0], fields[1]
		creationIndices[window.Key] = index
		index++
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("resize-window", "-t", fields[0], "-x", strconv.Itoa(window.Width), "-y", strconv.Itoa(window.Height))); err != nil {
			return Workspace{}, err
		}
		last := fields[1]
		for _, pane := range window.Panes[1:] {
			size, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("display-message", "-p", "-t", last, "#{pane_width}|#{pane_height}"))
			if err != nil {
				return Workspace{}, err
			}
			dimensions := strings.Split(size, "|")
			if len(dimensions) != 2 {
				return Workspace{}, errors.New("tmux recovery pane size is invalid")
			}
			width, widthErr := strconv.Atoi(dimensions[0])
			height, heightErr := strconv.Atoi(dimensions[1])
			if widthErr != nil || heightErr != nil {
				return Workspace{}, errors.New("tmux recovery pane size is invalid")
			}
			direction := "-v"
			if width > height {
				direction = "-h"
			}
			terminal, err := client.TerminalCommand(pane.CWD, "")
			if err != nil {
				return Workspace{}, err
			}
			args := []string{"-d", "-P", "-F", "#{pane_id}", direction, "-t", last, "-c", formatLiteral(pane.CWD)}
			output, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("split-window", append(args, terminal...)...)+" ; "+commandText("select-layout", "-t", windowIDs[window.Key], "tiled"))
			if err != nil {
				return Workspace{}, err
			}
			if !paneIDPattern.MatchString(output) {
				return Workspace{}, errors.New("tmux recovery pane receipt is invalid")
			}
			paneIDs[pane.Key], last = output, output
		}
	}
	linked := make(map[string]bool)
	for _, session := range recipe.Sessions {
		owner := representatives[session.Key]
		if linked[owner] {
			continue
		}
		linked[owner] = true
		for _, link := range session.Links {
			branch := commandText("link-window", "-k", "-s", stagingID+":"+strconv.Itoa(creationIndices[link.WindowKey]), "-t", owner+":"+strconv.Itoa(link.Index))
			if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), branch); err != nil {
				return Workspace{}, err
			}
		}
	}
	if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("kill-session", "-t", stagingID)); err != nil {
		return Workspace{}, err
	}
	live, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("list-windows", "-a", "-F", "#{window_id}"))
	if err != nil {
		return Workspace{}, err
	}
	liveWindows := strings.Fields(live)
	for placeholder := range placeholders {
		// Replacing a placeholder's last link may have already destroyed it.
		if !slices.Contains(liveWindows, placeholder) {
			continue
		}
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("kill-window", "-t", placeholder)); err != nil {
			return Workspace{}, err
		}
	}
	for _, session := range recipe.Sessions {
		id := sessionIDs[session.Key]
		branch := commandText("set-option", "-u", "-t", id, "renumber-windows") + " ; " +
			commandText("set-option", "-u", "-t", id, AutoNameOption) + " ; " +
			commandText("set-option", "-t", id, "@skid_character", session.Character) + " ; " +
			commandText("select-window", "-t", id+":"+strconv.Itoa(session.ActiveWindowIndex))
		if session.Group != "" {
			branch += " ; " + commandText("set-option", "-t", id, GroupOption, base64.RawURLEncoding.EncodeToString([]byte(session.Group)))
		}
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), branch); err != nil {
			return Workspace{}, err
		}
	}
	for _, window := range recipe.Windows {
		id := windowIDs[window.Key]
		branch := commandText("select-layout", "-t", id, window.Layout) + " ; " +
			commandText("select-pane", "-t", paneIDs[window.ActivePaneKey]) + " ; " +
			commandText("set-window-option", "-u", "-t", id, "window-size") + " ; " +
			commandText("set-window-option", "-t", id, "automatic-rename", "off") + " ; " +
			commandText("set-window-option", "-t", id, "allow-rename", "off")
		if window.Zoomed {
			branch += " ; " + commandText("resize-pane", "-Z", "-t", paneIDs[window.ActivePaneKey])
		}
		branch += " ; " + commandText("rename-window", "-t", id, formatLiteral(window.Name))
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), branch); err != nil {
			return Workspace{}, err
		}
	}
	for _, session := range recipe.Sessions {
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("rename-session", "-t", sessionIDs[session.Key], formatLiteral(session.Name))); err != nil {
			return Workspace{}, err
		}
	}
	actual, present, err := client.CaptureWorkspace(ctx)
	if err != nil {
		return Workspace{}, err
	}
	if !present || actual.Server != server {
		return Workspace{}, errors.New("tmux recovery destination vanished")
	}
	if err := verifyRestoredWorkspace(recipe, actual, sessionIDs, windowIDs, paneIDs); err != nil {
		return Workspace{}, err
	}
	return actual, nil
}

func (client Client) recoveryQueue(ctx context.Context, conditions []string, branch string) (string, error) {
	const complete = "SKIDBLADNIR_RECOVERY_COMPLETE"
	output, err := client.Output(ctx, "restore-workspace", "-N", "if-shell", "-F", andFormatConditions(conditions), branch+" ; display-message -p -l '"+complete+"'", "display-message -p -l '"+identityMismatchMarker+"'")
	if err != nil {
		operation, _, _ := strings.Cut(branch, " ")
		return "", fmt.Errorf("tmux recovery %s: %w", operation, err)
	}
	if output == complete {
		return "", nil
	}
	if !strings.HasSuffix(output, "\n"+complete) {
		return "", errors.New("tmux recovery destination changed or receipt is incomplete")
	}
	return strings.TrimSuffix(output, "\n"+complete), nil
}

func verifyRestoredWorkspace(expected, actual Workspace, sessionIDs, windowIDs, paneIDs map[string]string) error {
	invalid := errors.New("tmux reconstructed workspace does not match checkpoint")
	if len(expected.Sessions) != len(actual.Sessions) || len(expected.Windows) != len(actual.Windows) {
		return invalid
	}
	sessions := make(map[string]WorkspaceSession)
	for _, session := range actual.Sessions {
		sessions[session.Key] = session
	}
	groups := make(map[string]string)
	for _, session := range expected.Sessions {
		fresh, exists := sessions[sessionIDs[session.Key]]
		if !exists || fresh.Name != session.Name || fresh.Character != session.Character || fresh.Group != session.Group || fresh.ActiveWindowIndex != session.ActiveWindowIndex || len(fresh.Links) != len(session.Links) || (session.NativeGroup == "") != (fresh.NativeGroup == "") {
			return fmt.Errorf("%w: session facts", invalid)
		}
		for _, link := range session.Links {
			if !slices.Contains(fresh.Links, WorkspaceLink{Index: link.Index, WindowKey: windowIDs[link.WindowKey]}) {
				return invalid
			}
		}
		if session.NativeGroup != "" {
			if previous, exists := groups[session.NativeGroup]; exists && previous != fresh.NativeGroup {
				return invalid
			}
			for old, current := range groups {
				if old != session.NativeGroup && current == fresh.NativeGroup {
					return invalid
				}
			}
			groups[session.NativeGroup] = fresh.NativeGroup
		}
	}
	windows := make(map[string]WorkspaceWindow)
	for _, window := range actual.Windows {
		windows[window.Key] = window
	}
	for _, window := range expected.Windows {
		fresh, exists := windows[windowIDs[window.Key]]
		if !exists || fresh.Name != window.Name || fresh.Width != window.Width || fresh.Height != window.Height || fresh.Zoomed != window.Zoomed || fresh.ActivePaneKey != paneIDs[window.ActivePaneKey] || len(fresh.Panes) != len(window.Panes) {
			return fmt.Errorf("%w: window facts", invalid)
		}
		for i, pane := range window.Panes {
			if fresh.Panes[i].Key != paneIDs[pane.Key] || fresh.Panes[i].CWD != pane.CWD {
				return fmt.Errorf("%w: pane facts", invalid)
			}
		}
		left, err := normalizedLayout(window.Layout, paneIDs)
		if err != nil {
			return err
		}
		right, err := normalizedLayout(fresh.Layout, nil)
		if err != nil {
			return err
		}
		if left != right {
			return fmt.Errorf("%w: layout", invalid)
		}
	}
	return nil
}

// Native layout leaves end in x,y,pane-id. Only comparison drops the checksum
// and maps those ids; the original layout is sent unchanged to select-layout.
func normalizedLayout(layout string, paneIDs map[string]string) (string, error) {
	_, body, ok := strings.Cut(layout, ",")
	if !ok {
		return "", errors.New("tmux layout is invalid")
	}
	if paneIDs == nil {
		return body, nil
	}
	var result strings.Builder
	previous := 0
	for _, match := range layoutPanePattern.FindAllStringSubmatchIndex(body, -1) {
		start, end := match[2], match[3]
		mapped, exists := paneIDs["%"+body[start:end]]
		if !exists {
			return "", errors.New("tmux layout refers to an unknown pane")
		}
		result.WriteString(body[previous:start])
		result.WriteString(strings.TrimPrefix(mapped, "%"))
		previous = end
	}
	result.WriteString(body[previous:])
	return result.String(), nil
}

// CaptureWorkspace runs all graph reads in one synchronous native command
// queue. Raw fields are byte-length-prefixed, including names with newlines.
func (client Client) CaptureWorkspace(ctx context.Context) (Workspace, bool, error) {
	ids, err := client.ListSessionIDs(ctx)
	if err != nil {
		return Workspace{}, false, err
	}
	if len(ids) > 256 {
		return Workspace{}, false, errors.New("too many tmux sessions")
	}
	identity := recoveryFields("@skid_server_epoch", "pid", "start_time")
	graph := commandText("list-sessions", "-F", "S"+recoveryFields("session_id", "session_name", "session_group", "window_index")) + " ; " +
		commandText("list-windows", "-a", "-F", "W"+recoveryFields("session_id", "window_index", "window_id", "window_name", "window_width", "window_height", "window_layout", "pane_id", "window_zoomed_flag")) + " ; " +
		commandText("list-panes", "-a", "-F", "P"+recoveryFields("window_id", "pane_id", "pane_index", "pane_current_path"))
	script := commandText("display-message", "-p", "B"+identity) + " ; " + commandText("if-shell", "-F", "#{server_sessions}", graph, "")
	for _, id := range ids {
		if !workspaceID(id, '$') {
			return Workspace{}, false, errors.New("tmux session id is invalid")
		}
		// show-options without -A owns local metadata. Its escaped output stays
		// on one line; only canonical catalogue/base64 tokens are admitted.
		script += " ; " + commandText("display-message", "-p", "-l", "M"+strconv.Itoa(len(id))+":"+id) + " ; " +
			commandText("show-options", "-q", "-t", id, "@skid_character") + " ; " +
			commandText("show-options", "-q", "-t", id, GroupOption) + " ; " +
			commandText("display-message", "-p", "-l", "N")
	}
	script += " ; " + commandText("display-message", "-p", "E"+identity) + "\n"
	output := recoveryOutput{}
	var stderr bytes.Buffer
	command := client.commandWithStderr(ctx, nil, &stderr, "-N", "source-file", "-")
	command.Stdin = strings.NewReader(script)
	command.Stdout = &output
	if err := command.Run(); err != nil {
		if missingServer(err, stderr.String()) {
			return Workspace{}, false, nil
		}
		return Workspace{}, false, fmt.Errorf("capture tmux workspace: %w", err)
	}
	if output.overflow {
		return Workspace{}, false, errors.New("tmux workspace capture exceeds limit")
	}
	workspace, err := parseWorkspace(output.bytes)
	return workspace, err == nil, err
}

type recoveryOutput struct {
	bytes    []byte
	overflow bool
}

func (output *recoveryOutput) Write(value []byte) (int, error) {
	count := len(value)
	if count > maximumWorkspaceBytes-len(output.bytes) {
		output.overflow = true
		value = value[:maximumWorkspaceBytes-len(output.bytes)]
	}
	output.bytes = append(output.bytes, value...)
	return count, nil
}

func recoveryFields(names ...string) string {
	var output strings.Builder
	for _, name := range names {
		output.WriteString("#{n:" + name + "}:#{" + name + "}")
	}
	return output.String()
}

func parseRecoveryFields(input []byte, count int) ([]string, []byte, error) {
	fields := make([]string, 0, count)
	for range count {
		length, rest, found := bytes.Cut(input, []byte{':'})
		if !found || len(length) == 0 || len(length) > 8 {
			return nil, nil, errors.New("invalid tmux field length")
		}
		n, err := strconv.Atoi(string(length))
		if err != nil || n < 0 || n > len(rest) {
			return nil, nil, errors.New("invalid tmux field length")
		}
		fields = append(fields, string(rest[:n]))
		input = rest[n:]
	}
	return fields, input, nil
}

func parseWorkspace(input []byte) (Workspace, error) {
	workspace := Workspace{Sessions: []WorkspaceSession{}, Windows: []WorkspaceWindow{}}
	sessions := make(map[string]int)
	windows := make(map[string]int)
	paneIndices := make(map[string]map[int]string)
	paneRecords := make(map[string]WorkspacePane)
	metadata := make(map[string]bool)
	started, ended := false, false
	for len(input) > 0 {
		kind := input[0]
		if kind == 'M' {
			fields, rest, err := parseRecoveryFields(input[1:], 1)
			if err != nil || len(rest) == 0 || rest[0] != '\n' || !started || ended {
				return Workspace{}, errors.New("invalid tmux local metadata header")
			}
			session, exists := sessions[fields[0]]
			if !exists || metadata[fields[0]] {
				return Workspace{}, errors.New("tmux capture session set changed")
			}
			metadata[fields[0]] = true
			input = rest[1:]
			seen := make(map[string]bool)
			for {
				line, rest, complete := bytes.Cut(input, []byte{'\n'})
				if !complete {
					return Workspace{}, errors.New("tmux local metadata is incomplete")
				}
				input = rest
				if string(line) == "N" {
					break
				}
				name, value, found := strings.Cut(string(line), " ")
				if !found || seen[name] {
					return Workspace{}, errors.New("tmux local metadata is invalid")
				}
				seen[name] = true
				switch name {
				case "@skid_character":
					workspace.Sessions[session].Character = value
				case GroupOption:
					workspace.Sessions[session].Group = group.DecodeMetadata(value).String()
				default:
					return Workspace{}, errors.New("unexpected tmux local metadata")
				}
			}
			continue
		}
		count := 0
		switch kind {
		case 'B', 'E':
			count = 3
		case 'S':
			count = 4
		case 'W':
			count = 9
		case 'P':
			count = 4
		default:
			return Workspace{}, errors.New("invalid tmux workspace record")
		}
		fields, rest, err := parseRecoveryFields(input[1:], count)
		if err != nil || len(rest) == 0 || rest[0] != '\n' || ended {
			return Workspace{}, errors.New("invalid tmux workspace transcript")
		}
		input = rest[1:]
		if kind == 'B' || kind == 'E' {
			server := ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}
			if !server.valid() || kind == 'B' && started || kind == 'E' && (!started || server != workspace.Server) {
				return Workspace{}, errors.New("tmux capture server lifetime changed")
			}
			if kind == 'B' {
				workspace.Server, started = server, true
			} else {
				ended = true
			}
			continue
		}
		if !started {
			return Workspace{}, errors.New("tmux workspace source is absent")
		}
		switch kind {
		case 'S':
			if _, exists := sessions[fields[0]]; exists || len(sessions) == 256 {
				return Workspace{}, errors.New("invalid tmux session set")
			}
			active, err := strconv.Atoi(fields[3])
			if err != nil {
				return Workspace{}, errors.New("invalid tmux active window index")
			}
			sessions[fields[0]] = len(workspace.Sessions)
			workspace.Sessions = append(workspace.Sessions, WorkspaceSession{Key: fields[0], Name: fields[1], NativeGroup: fields[2], ActiveWindowIndex: active, Links: []WorkspaceLink{}})
		case 'W':
			session, exists := sessions[fields[0]]
			if !exists {
				return Workspace{}, errors.New("tmux window has no session")
			}
			index, indexErr := strconv.Atoi(fields[1])
			width, widthErr := strconv.Atoi(fields[4])
			height, heightErr := strconv.Atoi(fields[5])
			if indexErr != nil || widthErr != nil || heightErr != nil || fields[8] != "0" && fields[8] != "1" {
				return Workspace{}, errors.New("invalid tmux window record")
			}
			window := WorkspaceWindow{Key: fields[2], Name: fields[3], Width: width, Height: height, Layout: fields[6], ActivePaneKey: fields[7], Zoomed: fields[8] == "1", Panes: []WorkspacePane{}}
			if previous, found := windows[window.Key]; found {
				prior := workspace.Windows[previous]
				if window.Name != prior.Name || window.Width != prior.Width || window.Height != prior.Height || window.Layout != prior.Layout || window.ActivePaneKey != prior.ActivePaneKey || window.Zoomed != prior.Zoomed {
					return Workspace{}, errors.New("inconsistent shared tmux window")
				}
			} else {
				if len(windows) == 512 {
					return Workspace{}, errors.New("too many tmux windows")
				}
				windows[window.Key] = len(workspace.Windows)
				workspace.Windows = append(workspace.Windows, window)
			}
			workspace.Sessions[session].Links = append(workspace.Sessions[session].Links, WorkspaceLink{Index: index, WindowKey: window.Key})
		case 'P':
			window, exists := windows[fields[0]]
			index, err := strconv.Atoi(fields[2])
			if !exists || err != nil || index < 0 {
				return Workspace{}, errors.New("invalid tmux pane record")
			}
			if paneIndices[fields[0]] == nil {
				paneIndices[fields[0]] = make(map[int]string)
			}
			pane := WorkspacePane{Key: fields[1], CWD: fields[3]}
			if previous, duplicate := paneIndices[fields[0]][index]; duplicate {
				if previous != pane.Key || paneRecords[pane.Key] != pane {
					return Workspace{}, errors.New("inconsistent shared tmux pane")
				}
				continue
			}
			paneIndices[fields[0]][index] = fields[1]
			paneRecords[pane.Key] = pane
			workspace.Windows[window].Panes = append(workspace.Windows[window].Panes, pane)
		}
	}
	if !ended || len(metadata) != len(sessions) {
		return Workspace{}, errors.New("tmux workspace capture is incomplete")
	}
	for i := range workspace.Windows {
		indices := make(map[string]int)
		for index, key := range paneIndices[workspace.Windows[i].Key] {
			indices[key] = index
		}
		slices.SortFunc(workspace.Windows[i].Panes, func(a, b WorkspacePane) int { return indices[a.Key] - indices[b.Key] })
	}
	if err := ValidateWorkspace(workspace); err != nil {
		return Workspace{}, err
	}
	return workspace, nil
}

// ValidateWorkspace admits the complete native graph. Metadata policy and cwd
// filesystem admission belong to the manager, not this tmux boundary.
func ValidateWorkspace(workspace Workspace) error {
	invalid := errors.New("tmux workspace graph is invalid")
	if len(workspace.Sessions) > 256 || len(workspace.Windows) > 512 || (len(workspace.Sessions) == 0) != (len(workspace.Windows) == 0) {
		return invalid
	}
	keys, names := make(map[string]bool), make(map[string]bool)
	windows, linked := make(map[string]WorkspaceWindow), make(map[string]bool)
	panes := 0
	for _, window := range workspace.Windows {
		if !workspaceID(window.Key, '@') || keys[window.Key] || !workspaceText(window.Name, 4096, false) || window.Width < 1 || window.Height < 1 || window.Width > 65535 || window.Height > 65535 || !workspaceText(window.Layout, 65536, true) || len(window.Panes) == 0 {
			return invalid
		}
		keys[window.Key], windows[window.Key] = true, window
		active := false
		for _, pane := range window.Panes {
			panes++
			if panes > 1024 || !workspaceID(pane.Key, '%') || keys[pane.Key] || !workspaceText(pane.CWD, 4096, true) || !filepath.IsAbs(pane.CWD) {
				return invalid
			}
			keys[pane.Key] = true
			active = active || pane.Key == window.ActivePaneKey
		}
		if !active {
			return invalid
		}
	}
	groups := make(map[string][]WorkspaceLink)
	for _, session := range workspace.Sessions {
		if !workspaceID(session.Key, '$') || keys[session.Key] || !workspaceText(session.Name, 4096, true) || strings.ContainsAny(session.Name, ":.") || names[session.Name] || !workspaceText(session.Character, 4096, true) || !workspaceText(session.Group, 4096, false) || !workspaceText(session.NativeGroup, 4096, false) || len(session.Links) == 0 {
			return invalid
		}
		keys[session.Key], names[session.Name] = true, true
		indices := make(map[int]bool)
		active := false
		for _, link := range session.Links {
			if link.Index < 0 || link.Index > 2147483647 || indices[link.Index] {
				return invalid
			}
			if _, exists := windows[link.WindowKey]; !exists {
				return invalid
			}
			indices[link.Index], linked[link.WindowKey] = true, true
			active = active || link.Index == session.ActiveWindowIndex
		}
		if !active {
			return invalid
		}
		if session.NativeGroup != "" {
			links := slices.Clone(session.Links)
			slices.SortFunc(links, func(a, b WorkspaceLink) int { return a.Index - b.Index })
			if previous, exists := groups[session.NativeGroup]; exists && !slices.Equal(links, previous) {
				return invalid
			}
			groups[session.NativeGroup] = links
		}
	}
	if len(linked) != len(windows) {
		return invalid
	}
	return nil
}

func workspaceText(value string, limit int, required bool) bool {
	return (!required || value != "") && len(value) <= limit && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func workspaceID(value string, prefix byte) bool {
	if len(value) < 2 || len(value) > 32 || value[0] != prefix {
		return false
	}
	for _, r := range value[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
