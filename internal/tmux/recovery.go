package tmux

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

const maximumWorkspaceBytes = 8 << 20

// WorkspaceError preserves the native evidence needed by checkpoint policy.
// Execution errors retain their original causes instead of becoming this type.
type WorkspaceError string

const (
	WorkspaceTopologyChanged WorkspaceError = "topology_changed"
	WorkspaceSourceChanged   WorkspaceError = "source_changed"
	WorkspaceUnsupported     WorkspaceError = "unsupported"
	WorkspaceProtocolInvalid WorkspaceError = "protocol_invalid"
	WorkspaceLimit           WorkspaceError = "limit"
)

func (failure WorkspaceError) Error() string { return "tmux workspace " + string(failure) }

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
	Sessions []WorkspaceSession
	Windows  []WorkspaceWindow
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
	Key   string `json:"key"`
	CWD   string `json:"cwd"`
	PID   int    `json:"-"`
	Title string `json:"-"`
}

// WorkspaceStartup is opaque encoded shell startup for one saved pane. Only resume
// requests may change directory before restoration verifies the fresh graph.
type WorkspaceStartup struct {
	Launch       string
	MayChangeCwd bool
}

// InspectServer observes the running server, if any. It never returns
// WorkspaceLimit: recovery's source phase has no limit cause.
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
	if len(fields) != 4 {
		return ServerObservation{}, false, WorkspaceProtocolInvalid
	}
	identity, err := ParseServerIdentity(fields[0], fields[1], fields[2])
	if err != nil {
		return ServerObservation{}, false, err
	}
	count, err := strconv.Atoi(fields[3])
	if err != nil || count < 0 {
		return ServerObservation{}, false, WorkspaceProtocolInvalid
	}
	return ServerObservation{Identity: identity, SessionCount: count}, true, nil
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

// ValidateRecoveryServer admits expected as an empty restore destination and
// returns its default shell: no sessions, an empty default-command, exit-empty
// and exit-unattached off, and an executable bash or zsh default-shell. A
// missing shell is the stat error, so it matches fs.ErrNotExist.
func (client Client) ValidateRecoveryServer(ctx context.Context, expected ServerIdentity) (string, error) {
	format := inventoryFields(ServerEpochOption, "pid", "start_time", "server_sessions", "default-shell", "default-command", "exit-empty", "exit-unattached")
	output, err := client.Output(ctx, "validate-recovery-server", "-N", "display-message", "-p", format)
	if err != nil {
		return "", err
	}
	fields, rest, err := parseInventoryFields([]byte(output+"\n"), 8)
	if err != nil || len(rest) != 0 {
		return "", WorkspaceProtocolInvalid
	}
	// Preflight can precede epoch admission on an empty external server.
	identity, err := ParseServerIdentity(fields[0], fields[1], fields[2])
	if err != nil {
		return "", err
	}
	count, countErr := strconv.Atoi(fields[3])
	if countErr != nil || count < 0 || fields[6] != "0" && fields[6] != "1" || fields[7] != "0" && fields[7] != "1" {
		return "", WorkspaceProtocolInvalid
	}
	if identity != expected {
		return "", WorkspaceSourceChanged
	}
	if count != 0 {
		return "", WorkspaceTopologyChanged
	}
	if fields[5] != "" || fields[6] != "0" || fields[7] != "0" {
		return "", WorkspaceUnsupported
	}
	shell := fields[4]
	if !filepath.IsAbs(shell) || filepath.Base(shell) != "bash" && filepath.Base(shell) != "zsh" {
		return "", WorkspaceUnsupported
	}
	info, err := os.Stat(shell)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", WorkspaceUnsupported
	}
	return shell, nil
}

// RestoreWorkspace builds a nonempty recipe admitted by ValidateWorkspace, with
// preflighted cwds, into the empty server whose default shell
// ValidateRecoveryServer returned; the first construction guard pins that
// identity, emptiness, shell and options. It uses only freshly returned native
// ids and returns their old-to-new pane map after verifying a fresh capture.
// Startup applies only to final unique panes. Any error can leave visible
// partial work; the manager must never replay it.
func (client Client) RestoreWorkspace(ctx context.Context, server ServerIdentity, shell string, recipe Workspace, startupByPane map[string]WorkspaceStartup) (map[string]string, error) {
	var entropy [16]byte
	rand.Read(entropy[:]) // never fails since go 1.24
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
				return nil, err
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
			return nil, err
		}
		fields := strings.Split(output, "|")
		if len(fields) != 3 || !ValidID(fields[0], '$') || !ValidID(fields[1], '@') {
			return nil, WorkspaceProtocolInvalid
		}
		if _, err := strconv.Atoi(fields[2]); err != nil {
			return nil, WorkspaceProtocolInvalid
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
			return nil, err
		}
		if !ValidID(output, '$') {
			return nil, WorkspaceProtocolInvalid
		}
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("kill-session", "-t", output)); err != nil {
			return nil, err
		}
	}
	// tmux rejects linking between members of the same native group. One
	// ungrouped source supports every link, including repeated indexed links.
	first := recipe.Windows[0]
	terminal, err := client.TerminalCommand(first.Panes[0].CWD, "")
	if err != nil {
		return nil, err
	}
	name := prefix + "-workspace"
	args := []string{"-d", "-P", "-F", "#{session_id}|#{window_index}", "-s", name, "-x", strconv.Itoa(first.Width), "-y", strconv.Itoa(first.Height), "-c", formatLiteral(first.Panes[0].CWD)}
	output, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("new-session", append(args, terminal...)...)+" ; "+commandText("set-option", "-t", "="+name+":", "renumber-windows", "off"))
	if err != nil {
		return nil, err
	}
	fields := strings.Split(output, "|")
	if len(fields) != 2 || !ValidID(fields[0], '$') {
		return nil, WorkspaceProtocolInvalid
	}
	placeholderIndex, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, WorkspaceProtocolInvalid
	}
	stagingID := fields[0]
	creationIndices := make(map[string]int)
	index := 0
	for _, window := range recipe.Windows {
		if index == placeholderIndex {
			index++
		}
		terminal, err := client.TerminalCommand(window.Panes[0].CWD, startupByPane[window.Panes[0].Key].Launch)
		if err != nil {
			return nil, err
		}
		args := []string{"-d", "-P", "-F", "#{window_id}|#{pane_id}", "-t", stagingID + ":" + strconv.Itoa(index), "-n", prefix, "-c", formatLiteral(window.Panes[0].CWD)}
		output, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("new-window", append(args, terminal...)...))
		if err != nil {
			return nil, err
		}
		fields := strings.Split(output, "|")
		if len(fields) != 2 || !ValidID(fields[0], '@') || !ValidID(fields[1], '%') {
			return nil, WorkspaceProtocolInvalid
		}
		windowIDs[window.Key], paneIDs[window.Panes[0].Key] = fields[0], fields[1]
		creationIndices[window.Key] = index
		index++
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("resize-window", "-t", fields[0], "-x", strconv.Itoa(window.Width), "-y", strconv.Itoa(window.Height))); err != nil {
			return nil, err
		}
		last := fields[1]
		for _, pane := range window.Panes[1:] {
			size, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("display-message", "-p", "-t", last, "#{pane_width}|#{pane_height}"))
			if err != nil {
				return nil, err
			}
			dimensions := strings.Split(size, "|")
			if len(dimensions) != 2 {
				return nil, WorkspaceProtocolInvalid
			}
			width, widthErr := strconv.Atoi(dimensions[0])
			height, heightErr := strconv.Atoi(dimensions[1])
			if widthErr != nil || heightErr != nil {
				return nil, WorkspaceProtocolInvalid
			}
			direction := "-v"
			if width > height {
				direction = "-h"
			}
			terminal, err := client.TerminalCommand(pane.CWD, startupByPane[pane.Key].Launch)
			if err != nil {
				return nil, err
			}
			args := []string{"-d", "-P", "-F", "#{pane_id}", direction, "-t", last, "-c", formatLiteral(pane.CWD)}
			output, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("split-window", append(args, terminal...)...)+" ; "+commandText("select-layout", "-t", windowIDs[window.Key], "tiled"))
			if err != nil {
				return nil, err
			}
			if !ValidID(output, '%') {
				return nil, WorkspaceProtocolInvalid
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
				return nil, err
			}
		}
	}
	if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("kill-session", "-t", stagingID)); err != nil {
		return nil, err
	}
	live, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("list-windows", "-a", "-F", "#{window_id}"))
	if err != nil {
		return nil, err
	}
	liveWindows := strings.Fields(live)
	for placeholder := range placeholders {
		// Replacing a placeholder's last link may have already destroyed it.
		if !slices.Contains(liveWindows, placeholder) {
			continue
		}
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("kill-window", "-t", placeholder)); err != nil {
			return nil, err
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
			return nil, err
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
			return nil, err
		}
	}
	for _, session := range recipe.Sessions {
		if _, err := client.recoveryQueue(ctx, serverLifetimeConditions(server), commandText("rename-session", "-t", sessionIDs[session.Key], formatLiteral(session.Name))); err != nil {
			return nil, err
		}
	}
	freshIDs := make([]string, len(recipe.Sessions))
	for index, session := range recipe.Sessions {
		freshIDs[index] = sessionIDs[session.Key]
	}
	actual, err := client.CaptureWorkspace(ctx, server, freshIDs)
	if err != nil {
		return nil, err
	}
	if err := verifyRestoredWorkspace(recipe, actual, sessionIDs, windowIDs, paneIDs, startupByPane); err != nil {
		return nil, err
	}
	return paneIDs, nil
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
	if output == identityMismatchMarker {
		// The first construction guard also pins emptiness and shell options.
		return "", WorkspaceTopologyChanged
	}
	if !strings.HasSuffix(output, "\n"+complete) {
		return "", WorkspaceProtocolInvalid
	}
	return strings.TrimSuffix(output, "\n"+complete), nil
}

func verifyRestoredWorkspace(expected, actual Workspace, sessionIDs, windowIDs, paneIDs map[string]string, startupByPane map[string]WorkspaceStartup) error {
	invalid := WorkspaceTopologyChanged
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
			// Resume startup may change cwd after entering the preflighted directory.
			if fresh.Panes[i].Key != paneIDs[pane.Key] || !startupByPane[pane.Key].MayChangeCwd && fresh.Panes[i].CWD != pane.CWD {
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

// Only comparison maps native pane ids; select-layout receives the original.
// Native layouts use JSON v2 or checksum-prefixed classic encoding.
func normalizedLayout(layout string, paneIDs map[string]string) (string, error) {
	if strings.HasPrefix(strings.TrimSpace(layout), "{") {
		document, err := layoutObject(json.RawMessage(layout))
		if err != nil || string(document["V"]) != "2" {
			return "", WorkspaceProtocolInvalid
		}
		document["L"], err = normalizedLayoutCell(document["L"], paneIDs)
		if err != nil {
			return "", err
		}
		encoded, err := json.Marshal(document)
		return string(encoded), err
	}
	_, body, ok := strings.Cut(layout, ",")
	if !ok {
		return "", WorkspaceProtocolInvalid
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
			return "", WorkspaceProtocolInvalid
		}
		result.WriteString(body[previous:start])
		result.WriteString(strings.TrimPrefix(mapped, "%"))
		previous = end
	}
	result.WriteString(body[previous:])
	return result.String(), nil
}

// Retain opaque native fields exactly while rejecting duplicate cell members.
// The bounded native layout owns its geometry and nesting, not strictjson's
// smaller product-document nesting limit.
func layoutObject(encoded json.RawMessage) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, WorkspaceProtocolInvalid
	}
	object := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return nil, WorkspaceProtocolInvalid
		}
		if _, duplicate := object[key]; duplicate {
			return nil, WorkspaceProtocolInvalid
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, WorkspaceProtocolInvalid
		}
		object[key] = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(bytes.TrimSpace(encoded[decoder.InputOffset():])) != 0 {
		return nil, WorkspaceProtocolInvalid
	}
	return object, nil
}

func normalizedLayoutCell(encoded json.RawMessage, paneIDs map[string]string) (json.RawMessage, error) {
	cell, err := layoutObject(encoded)
	if err != nil {
		return nil, err
	}
	var kind string
	if json.Unmarshal(cell["t"], &kind) != nil {
		return nil, WorkspaceProtocolInvalid
	}
	switch kind {
	case "p":
		var id string
		if json.Unmarshal(cell["I"], &id) != nil || !ValidID(id, '%') {
			return nil, WorkspaceProtocolInvalid
		}
		if paneIDs != nil {
			mapped, exists := paneIDs[id]
			if !exists || !ValidID(mapped, '%') {
				return nil, WorkspaceProtocolInvalid
			}
			cell["I"] = json.RawMessage(`"` + mapped + `"`)
		}
	case "h", "v":
		var children []json.RawMessage
		if json.Unmarshal(cell["c"], &children) != nil || len(children) == 0 {
			return nil, WorkspaceProtocolInvalid
		}
		for index, child := range children {
			children[index], err = normalizedLayoutCell(child, paneIDs)
			if err != nil {
				return nil, err
			}
		}
		cell["c"], err = json.Marshal(children)
		if err != nil {
			return nil, err
		}
	default:
		return nil, WorkspaceProtocolInvalid
	}
	return json.Marshal(cell)
}

// CaptureWorkspace reads server's graph and the local metadata of ids in one
// client. Quoted fields evaluate each native value once, including changing
// cwd. Any other server, or none, is WorkspaceSourceChanged; sessions other
// than exactly ids are WorkspaceTopologyChanged.
func (client Client) CaptureWorkspace(ctx context.Context, server ServerIdentity, ids []string) (Workspace, error) {
	graph := commandText("list-sessions", "-F", "S"+inventoryFields("session_id", "session_name", "session_group", "window_index")) + " ; " +
		commandText("list-windows", "-a", "-F", "W"+inventoryFields("session_id", "window_index", "window_id", "window_name", "window_width", "window_height", "window_layout", "pane_id", "window_zoomed_flag")) + " ; " +
		commandText("list-panes", "-a", "-F", "P"+inventoryFields("window_id", "pane_id", "pane_index", "pane_current_path", "pane_pid", "pane_title"))
	body := commandText("if-shell", "-F", "#{server_sessions}", graph, "")
	for _, id := range ids {
		body += " ; " + sessionScript(id, "")
	}
	observed, transcript, err := client.batch(ctx, body)
	if err != nil {
		return Workspace{}, err
	}
	if observed != server {
		return Workspace{}, WorkspaceSourceChanged
	}
	return parseWorkspace(transcript, ids)
}

// parseWorkspace reads CaptureWorkspace's S, W and P records, then one
// sessionScript record per id. Membership is compared only once the whole
// transcript has parsed.
func parseWorkspace(input []byte, ids []string) (Workspace, error) {
	workspace := Workspace{Sessions: []WorkspaceSession{}, Windows: []WorkspaceWindow{}}
	sessions := make(map[string]int)
	windows := make(map[string]int)
	paneIndices := make(map[string]map[int]string)
	for len(input) > 0 && (input[0] == 'S' || input[0] == 'W' || input[0] == 'P') {
		kind := input[0]
		count := 4
		if kind == 'W' {
			count = 9
		} else if kind == 'P' {
			count = 6
		}
		fields, rest, err := parseInventoryFields(input[1:], count)
		if err != nil {
			return Workspace{}, err
		}
		input = rest
		switch kind {
		case 'S':
			if _, exists := sessions[fields[0]]; exists {
				return Workspace{}, WorkspaceProtocolInvalid
			}
			active, err := strconv.Atoi(fields[3])
			if err != nil {
				return Workspace{}, WorkspaceProtocolInvalid
			}
			sessions[fields[0]] = len(workspace.Sessions)
			workspace.Sessions = append(workspace.Sessions, WorkspaceSession{Key: fields[0], Name: fields[1], NativeGroup: fields[2], ActiveWindowIndex: active, Links: []WorkspaceLink{}})
		case 'W':
			session, exists := sessions[fields[0]]
			if !exists {
				return Workspace{}, WorkspaceProtocolInvalid
			}
			index, indexErr := strconv.Atoi(fields[1])
			width, widthErr := strconv.Atoi(fields[4])
			height, heightErr := strconv.Atoi(fields[5])
			if indexErr != nil || widthErr != nil || heightErr != nil || fields[8] != "0" && fields[8] != "1" {
				return Workspace{}, WorkspaceProtocolInvalid
			}
			window := WorkspaceWindow{Key: fields[2], Name: fields[3], Width: width, Height: height, Layout: fields[6], ActivePaneKey: fields[7], Zoomed: fields[8] == "1", Panes: []WorkspacePane{}}
			if previous, found := windows[window.Key]; found {
				prior := workspace.Windows[previous]
				if window.Name != prior.Name || window.Width != prior.Width || window.Height != prior.Height || window.Layout != prior.Layout || window.ActivePaneKey != prior.ActivePaneKey || window.Zoomed != prior.Zoomed {
					return Workspace{}, WorkspaceProtocolInvalid
				}
			} else {
				windows[window.Key] = len(workspace.Windows)
				workspace.Windows = append(workspace.Windows, window)
			}
			workspace.Sessions[session].Links = append(workspace.Sessions[session].Links, WorkspaceLink{Index: index, WindowKey: window.Key})
		case 'P':
			window, exists := windows[fields[0]]
			index, err := strconv.Atoi(fields[2])
			pid, pidErr := strconv.Atoi(fields[4])
			if !exists || err != nil || index < 0 || pidErr != nil || pid < 0 {
				return Workspace{}, WorkspaceProtocolInvalid
			}
			if paneIndices[fields[0]] == nil {
				paneIndices[fields[0]] = make(map[int]string)
			}
			if previous, duplicate := paneIndices[fields[0]][index]; duplicate {
				if previous != fields[1] {
					return Workspace{}, WorkspaceProtocolInvalid
				}
				// Linked windows repeat pane rows. Keep the first complete sample;
				// the process may change cwd or title between native reads.
				continue
			}
			paneIndices[fields[0]][index] = fields[1]
			workspace.Windows[window].Panes = append(workspace.Windows[window].Panes, WorkspacePane{Key: fields[1], CWD: fields[3], PID: pid, Title: fields[5]})
		}
	}
	changed := len(sessions) != len(ids)
	for _, id := range ids {
		metadata, found, rest, err := parseMetadata(input, id)
		if err != nil {
			return Workspace{}, err
		}
		input = rest
		session, exists := sessions[id]
		if !found || !exists {
			changed = true
			continue
		}
		workspace.Sessions[session].Character = metadata.Character
		workspace.Sessions[session].Group = group.DecodeMetadata(metadata.Group).String()
	}
	if len(input) != 0 {
		return Workspace{}, WorkspaceProtocolInvalid
	}
	if changed {
		return Workspace{}, WorkspaceTopologyChanged
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

// ValidateWorkspace admits what RestoreWorkspace needs to rebuild and verify:
// native ids as keys, unique across windows, panes and sessions, and unique
// session names; windows of positive size with a layout, holding their active
// pane; absolute pane cwds; named sessions with a character, whose links
// resolve to windows at unique nonnegative indices, one of them active; equal
// links within a native group; and every window linked. Catalogue keys and
// directory admission are the manager's policy.
func ValidateWorkspace(workspace Workspace) error {
	keys, names := make(map[string]bool), make(map[string]bool)
	windows, linked := make(map[string]bool), make(map[string]bool)
	for _, window := range workspace.Windows {
		if !ValidID(window.Key, '@') || keys[window.Key] || window.Width < 1 || window.Height < 1 {
			return WorkspaceProtocolInvalid
		}
		if window.Layout == "" {
			return WorkspaceUnsupported
		}
		keys[window.Key], windows[window.Key] = true, true
		active := false
		for _, pane := range window.Panes {
			if !ValidID(pane.Key, '%') || keys[pane.Key] {
				return WorkspaceProtocolInvalid
			}
			if !filepath.IsAbs(pane.CWD) {
				return WorkspaceUnsupported
			}
			keys[pane.Key] = true
			active = active || pane.Key == window.ActivePaneKey
		}
		if !active {
			return WorkspaceProtocolInvalid
		}
	}
	groups := make(map[string][]WorkspaceLink)
	for _, session := range workspace.Sessions {
		if !ValidID(session.Key, '$') || keys[session.Key] || names[session.Name] {
			return WorkspaceProtocolInvalid
		}
		// tmux refuses an empty name and rewrites ':' and '.', so verification
		// could never match them.
		if session.Name == "" || strings.ContainsAny(session.Name, ":.") || session.Character == "" {
			return WorkspaceUnsupported
		}
		keys[session.Key], names[session.Name] = true, true
		indices := make(map[int]bool)
		active := false
		for _, link := range session.Links {
			if link.Index < 0 || indices[link.Index] || !windows[link.WindowKey] {
				return WorkspaceProtocolInvalid
			}
			indices[link.Index], linked[link.WindowKey] = true, true
			active = active || link.Index == session.ActiveWindowIndex
		}
		if !active {
			return WorkspaceProtocolInvalid
		}
		if session.NativeGroup != "" {
			links := slices.Clone(session.Links)
			slices.SortFunc(links, func(a, b WorkspaceLink) int { return a.Index - b.Index })
			if previous, exists := groups[session.NativeGroup]; exists && !slices.Equal(links, previous) {
				return WorkspaceProtocolInvalid
			}
			groups[session.NativeGroup] = links
		}
	}
	if len(linked) != len(windows) {
		return WorkspaceProtocolInvalid
	}
	return nil
}
