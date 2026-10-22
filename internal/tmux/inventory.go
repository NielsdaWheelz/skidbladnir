package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type InventoryCapture struct {
	Server ServerIdentity // the server that produced Rows (B == E); zero when none runs
	Rows   []InventoryRow
}
type InventoryRow struct {
	ID, Name, PaneID         string
	PanePID, AttachedClients int
	Title, CWD, Command      string
	// Selected pane screen facts: a2/a3 inputs, carried unused for now.
	PaneWidth, PaneHeight int
	AlternateOn           bool
	WindowActivity        int64 // unix seconds
	// Format holds the raw #{@skid_…} values as tmux format lookup, and so every
	// guard, sees them; it is the sample and is never projected. LocalSession
	// holds the session-local values admitted as [A-Za-z0-9_.-]+ tokens, else "".
	Format, LocalSession SessionMetadata
}
type SessionMetadata struct{ Character, AutoName, Group, Profile, Objective string }

var metadataOptions = [...]string{"@skid_character", AutoNameOption, GroupOption, "@skid_profile", "@skid_objective_b64"}

// rowFormat has 17 fields: 8 builtins, the 5 metadata options, then the
// selected pane's screen facts.
var rowFormat = "S" + inventoryFields("session_id", "session_name", "pane_id", "pane_pid", "session_attached", "pane_title", "pane_current_path", "pane_current_command") +
	inventoryFields(metadataOptions[:]...) + inventoryFields("pane_width", "pane_height", "alternate_on", "window_activity")

// ReadCurrentInventory reads every session of whichever server runs, in one
// client (two when an inherited @skid_ definition exists). Server is that
// server, zero with no rows when none runs. Errors: WorkspaceSourceChanged (E,
// or the fallback client's B/E, differs from B; or the read failed while a
// nonzero admitted lifetime no longer runs), WorkspaceProtocolInvalid,
// WorkspaceLimit, ctx and child errors.
func (client Client) ReadCurrentInventory(ctx context.Context, admitted ServerIdentity) (_ InventoryCapture, err error) {
	defer func() {
		if err != nil && admitted != (ServerIdentity{}) {
			err = client.lost(ctx, admitted, err)
		}
	}()
	inherited := inheritedScript()
	server, transcript, err := client.batch(ctx, inherited+" ; "+commandText("if-shell", "-F", "#{server_sessions}", commandText("list-sessions", "-F", rowFormat), "")+" ; "+inherited)
	if err != nil || server == (ServerIdentity{}) {
		return InventoryCapture{Server: server}, err
	}
	var rows []InventoryRow
	seen := make(map[string]bool)
	shadowed := false
	for len(transcript) > 0 {
		// show-options prints the option name first, and a row starts with S.
		if transcript[0] == '@' {
			_, rest, found := bytes.Cut(transcript, []byte{'\n'})
			if !found {
				return InventoryCapture{}, WorkspaceProtocolInvalid
			}
			shadowed = true
			transcript = rest
			continue
		}
		row, rest, err := parseRow(transcript)
		if err != nil {
			return InventoryCapture{}, err
		}
		if seen[row.ID] {
			return InventoryCapture{}, WorkspaceProtocolInvalid
		}
		seen[row.ID] = true
		transcript = rest
		row.LocalSession = SessionMetadata{Character: token(row.Format.Character), AutoName: token(row.Format.AutoName), Group: token(row.Format.Group), Profile: token(row.Format.Profile), Objective: token(row.Format.Objective)}
		rows = append(rows, row)
	}
	if !shadowed || len(rows) == 0 {
		return InventoryCapture{Server: server, Rows: rows}, nil
	}
	// An inherited definition makes format values unattributable, so the local
	// options are read exactly, replacing the format-derived LocalSession.
	scripts := make([]string, len(rows))
	for index, row := range rows {
		scripts[index] = sessionScript(row.ID, "")
	}
	fallback, transcript, err := client.batch(ctx, strings.Join(scripts, " ; "))
	if err != nil {
		return InventoryCapture{}, err
	}
	if fallback != server {
		return InventoryCapture{}, WorkspaceSourceChanged
	}
	kept := rows[:0]
	for _, row := range rows {
		metadata, found, rest, err := parseMetadata(transcript, row.ID)
		if err != nil {
			return InventoryCapture{}, err
		}
		transcript = rest
		if !found {
			continue // proven gone on the same server between the two clients
		}
		row.LocalSession = metadata
		kept = append(kept, row)
	}
	if len(transcript) != 0 {
		return InventoryCapture{}, WorkspaceProtocolInvalid
	}
	return InventoryCapture{Server: server, Rows: kept}, nil
}

// ReadInventory is ReadCurrentInventory pinned to the admitted server
// expected: any other server, or none, is WorkspaceSourceChanged.
func (client Client) ReadInventory(ctx context.Context, expected ServerIdentity) (InventoryCapture, error) {
	capture, err := client.ReadCurrentInventory(ctx, expected)
	if err != nil {
		return InventoryCapture{}, err
	}
	if capture.Server != expected {
		return InventoryCapture{}, WorkspaceSourceChanged
	}
	return capture, nil
}

// ReadSession reads session id's row and its exact local options in one client,
// unguarded by any expected identity. Server is the server observed (B == E,
// else WorkspaceSourceChanged), zero when none runs. Rows holds the row, or is
// empty when id is absent. When the client fails (not ctx, not a
// WorkspaceError) and a listing then lacks id, the result is absent with a zero
// Server and a nil error.
func (client Client) ReadSession(ctx context.Context, id string) (InventoryCapture, error) {
	server, transcript, err := client.batch(ctx, sessionScript(id, commandText("display-message", "-p", "-t", id, rowFormat)))
	if err != nil {
		var workspaceErr WorkspaceError
		if ctx.Err() == nil && !errors.As(err, &workspaceErr) {
			ids, listErr := client.ListSessionIDs(ctx)
			if listErr == nil && !slices.Contains(ids, id) {
				return InventoryCapture{}, nil
			}
		}
		return InventoryCapture{}, err
	}
	if server == (ServerIdentity{}) || string(transcript) == "D"+id+"\n" {
		return InventoryCapture{Server: server}, nil
	}
	row, rest, err := parseRow(transcript)
	if err != nil {
		return InventoryCapture{}, err
	}
	metadata, found, rest, err := parseMetadata(rest, id)
	if err != nil || !found || len(rest) != 0 || row.ID != id {
		return InventoryCapture{}, WorkspaceProtocolInvalid
	}
	row.LocalSession = metadata
	return InventoryCapture{Server: server, Rows: []InventoryRow{row}}, nil
}

func parseRow(input []byte) (InventoryRow, []byte, error) {
	if len(input) == 0 || input[0] != 'S' {
		return InventoryRow{}, nil, WorkspaceProtocolInvalid
	}
	fields, rest, err := parseInventoryFields(input[1:], 17)
	if err != nil {
		return InventoryRow{}, nil, err
	}
	pid, pidErr := strconv.Atoi(fields[3])
	attached, attachedErr := strconv.Atoi(fields[4])
	width, widthErr := strconv.Atoi(fields[13])
	height, heightErr := strconv.Atoi(fields[14])
	activity, activityErr := strconv.ParseInt(fields[16], 10, 64)
	if !sessionIDPattern.MatchString(fields[0]) || fields[1] == "" || !paneIDPattern.MatchString(fields[2]) ||
		pidErr != nil || pid < 0 || attachedErr != nil || attached < 0 || widthErr != nil || width < 0 || heightErr != nil || height < 0 ||
		fields[15] != "0" && fields[15] != "1" || activityErr != nil || activity < 0 {
		return InventoryRow{}, nil, WorkspaceProtocolInvalid
	}
	return InventoryRow{
		ID: fields[0], Name: fields[1], PaneID: fields[2], PanePID: pid, AttachedClients: attached,
		Title: fields[5], CWD: fields[6], Command: fields[7],
		PaneWidth: width, PaneHeight: height, AlternateOn: fields[15] == "1", WindowActivity: activity,
		Format: SessionMetadata{Character: fields[8], AutoName: fields[9], Group: fields[10], Profile: fields[11], Objective: fields[12]},
	}, rest, nil
}

func token(value string) string {
	if tmuxCommandTokenPattern.MatchString(value) {
		return value
	}
	return ""
}

// sessionScript prints rowCommand's output, then M<id>, the local metadata and
// N, or D<id> when the session is absent.
func sessionScript(id, rowCommand string) string {
	branch := commandText("display-message", "-p", "-l", "M"+id)
	if rowCommand != "" {
		branch = rowCommand + " ; " + branch
	}
	for _, option := range metadataOptions {
		branch += " ; " + commandText("show-options", "-q", "-t", id, option)
	}
	branch += " ; " + commandText("display-message", "-p", "-l", "N")
	return commandText("if-shell", "-F", "-t", id, sessionIDCondition(id), branch, commandText("display-message", "-p", "-l", "D"+id))
}

// parseMetadata reads one sessionScript record. show-options without -A owns
// local metadata; its escaped output stays on one line, and only canonical
// tokens are admitted.
func parseMetadata(input []byte, id string) (SessionMetadata, bool, []byte, error) {
	line, input, found := bytes.Cut(input, []byte{'\n'})
	if !found {
		return SessionMetadata{}, false, nil, WorkspaceProtocolInvalid
	}
	if string(line) == "D"+id {
		return SessionMetadata{}, false, input, nil
	}
	if string(line) != "M"+id {
		return SessionMetadata{}, false, nil, WorkspaceProtocolInvalid
	}
	options := make(map[string]string)
	for {
		line, input, found = bytes.Cut(input, []byte{'\n'})
		if !found {
			return SessionMetadata{}, false, nil, WorkspaceProtocolInvalid
		}
		if string(line) == "N" {
			break
		}
		option, value, separated := strings.Cut(string(line), " ")
		if _, duplicate := options[option]; !separated || duplicate || !slices.Contains(metadataOptions[:], option) {
			return SessionMetadata{}, false, nil, WorkspaceProtocolInvalid
		}
		options[option] = token(value)
	}
	return SessionMetadata{Character: options["@skid_character"], AutoName: options[AutoNameOption], Group: options[GroupOption], Profile: options["@skid_profile"], Objective: options["@skid_objective_b64"]}, true, input, nil
}

// inheritedScript prints a line for every @skid_ definition that format lookup
// finds before or instead of a session-local value. Format lookup checks
// server, then pane, window and global-window, then session and global-session
// options, so -s and -g -w shadow a session value and -g fills its absence.
func inheritedScript() string {
	commands := make([]string, 0, 3*len(metadataOptions))
	for _, option := range metadataOptions {
		commands = append(commands, commandText("show-options", "-q", "-s", option), commandText("show-options", "-q", "-g", option), commandText("show-options", "-q", "-g", "-w", option))
	}
	return strings.Join(commands, " ; ")
}

// batch runs body in one client between B and E records of the server's
// identity, and returns B and the bytes between. A client talks to one server
// process for its whole life, so B == E proves the body ran on that server and
// no lifetime guard wraps it. No server returns a zero identity.
func (client Client) batch(ctx context.Context, body string) (ServerIdentity, []byte, error) {
	if err := ctx.Err(); err != nil {
		return ServerIdentity{}, nil, err
	}
	identity := inventoryFields(ServerEpochOption, "pid", "start_time")
	script := commandText("display-message", "-p", "B"+identity) + " ; " + body + " ; " + commandText("display-message", "-p", "E"+identity) + "\n"
	output := recoveryOutput{}
	var stderr bytes.Buffer
	command := client.commandWithStderr(ctx, nil, &stderr, "-N", "source-file", "-")
	command.Stdin = strings.NewReader(script)
	command.Stdout = &output
	err := command.Run()
	if output.overflow {
		return ServerIdentity{}, nil, errors.Join(WorkspaceLimit, err, ctx.Err())
	}
	if err != nil {
		if missingServer(err, stderr.String()) {
			return ServerIdentity{}, nil, nil
		}
		return ServerIdentity{}, nil, fmt.Errorf("run tmux batch: %w", err)
	}
	server, rest, err := parseIdentityRecord(output.bytes, 'B')
	if err != nil {
		return ServerIdentity{}, nil, err
	}
	// Identity records contain only canonical single-line tokens. The final
	// record can be found from the end without splitting any builtin field.
	if len(rest) == 0 || rest[len(rest)-1] != '\n' {
		return ServerIdentity{}, nil, WorkspaceProtocolInvalid
	}
	endStart := bytes.LastIndexByte(rest[:len(rest)-1], '\n') + 1
	end, tail, err := parseIdentityRecord(rest[endStart:], 'E')
	if err != nil || len(tail) != 0 {
		return ServerIdentity{}, nil, WorkspaceProtocolInvalid
	}
	if end != server {
		return ServerIdentity{}, nil, WorkspaceSourceChanged
	}
	return server, rest[:endStart], nil
}

func parseIdentityRecord(input []byte, kind byte) (ServerIdentity, []byte, error) {
	if len(input) == 0 || input[0] != kind {
		return ServerIdentity{}, nil, WorkspaceProtocolInvalid
	}
	fields, rest, err := parseInventoryFields(input[1:], 3)
	if err != nil {
		return ServerIdentity{}, nil, err
	}
	server := ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}
	if !serverPIDPattern.MatchString(server.PID) || !startTimePattern.MatchString(server.StartTime) {
		return ServerIdentity{}, nil, WorkspaceProtocolInvalid
	}
	return server, rest, nil
}

// lost reclassifies a failed read of the admitted server expected. A server may
// disappear after connecting: native shutdown can produce either a child error
// or a successful but unfinished transcript. Only a positive observation of
// loss replaces that provisional classification.
func (client Client) lost(ctx context.Context, expected ServerIdentity, err error) error {
	if errors.Is(err, WorkspaceLimit) || errors.Is(err, WorkspaceSourceChanged) {
		return err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if !errors.Is(err, ctxErr) {
			return errors.Join(ctxErr, err)
		}
		return err
	}
	observed, present, inspectErr := client.InspectServer(ctx)
	if inspectErr != nil {
		return errors.Join(ctx.Err(), err, inspectErr)
	}
	if !present || observed.Identity != expected {
		if errors.Is(err, WorkspaceProtocolInvalid) {
			return WorkspaceSourceChanged
		}
		return errors.Join(WorkspaceSourceChanged, err)
	}
	return err
}

func inventoryFields(names ...string) string {
	var result strings.Builder
	for _, name := range names {
		result.WriteString("#{q:" + name + "}|")
	}
	return result.String()
}

// Decode quoted bytes before recognizing record boundaries. Newlines inside a
// field are data; only an unescaped pipe closes that field.
func parseInventoryFields(input []byte, count int) ([]string, []byte, error) {
	fields := make([]string, 0, count)
	for range count {
		var field strings.Builder
		closed := false
		for len(input) > 0 {
			value := input[0]
			input = input[1:]
			if value == '\\' {
				if len(input) == 0 {
					return nil, nil, WorkspaceProtocolInvalid
				}
				field.WriteByte(input[0])
				input = input[1:]
				continue
			}
			if value == '|' {
				closed = true
				break
			}
			field.WriteByte(value)
		}
		if !closed {
			return nil, nil, WorkspaceProtocolInvalid
		}
		fields = append(fields, field.String())
	}
	if len(input) == 0 || input[0] != '\n' {
		return nil, nil, WorkspaceProtocolInvalid
	}
	return fields, input[1:], nil
}
