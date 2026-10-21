package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type InventoryCapture struct {
	Server ServerIdentity
	Rows   []InventoryRow
}
type InventoryRow struct {
	ID, Name, PaneID         string
	PanePID, AttachedClients int
	Title, CWD, Command      string
	LocalSession             SessionMetadata
}
type SessionMetadata struct{ Character, AutoName, Group, Profile, Objective string }

func (client Client) ReadInventory(ctx context.Context, expected ServerIdentity) (InventoryCapture, error) {
	return client.readInventory(ctx, expected, "")
}

// ReadInventoryRow reobserves only an affected session after normalization.
func (client Client) ReadInventoryRow(ctx context.Context, expected ServerIdentity, id string) (InventoryRow, bool, error) {
	if !sessionIDPattern.MatchString(id) {
		return InventoryRow{}, false, WorkspaceProtocolInvalid
	}
	capture, err := client.readInventory(ctx, expected, id)
	if err != nil {
		return InventoryRow{}, false, err
	}
	if len(capture.Rows) == 0 {
		return InventoryRow{}, false, nil
	}
	return capture.Rows[0], true, nil
}

func inventoryFields(names ...string) string {
	var result strings.Builder
	for _, name := range names {
		result.WriteString("#{q:" + name + "}|")
	}
	return result.String()
}

func inventorySessionExists(id string) string {
	return "#{S:#{?#{==:#{session_id}," + id + "},1,}}"
}

func (client Client) readInventory(ctx context.Context, expected ServerIdentity, id string) (InventoryCapture, error) {
	if !expected.valid() || id != "" && !sessionIDPattern.MatchString(id) {
		return InventoryCapture{}, WorkspaceProtocolInvalid
	}
	format := "S" + inventoryFields("session_id", "session_name", "pane_id", "pane_pid", "session_attached", "pane_title", "pane_current_path", "pane_current_command")
	var body string
	if id == "" {
		body = commandText("if-shell", "-F", "#{server_sessions}", commandText("list-sessions", "-F", format), "")
	} else {
		body = commandText("if-shell", "-F", inventorySessionExists(id), commandText("display-message", "-p", "-t", id, format), "")
	}
	output, err := client.inventoryBatch(ctx, expected, body)
	if err != nil {
		return InventoryCapture{}, err
	}
	capture := InventoryCapture{Server: expected, Rows: []InventoryRow{}}
	seen := make(map[string]bool)
	for len(output) > 0 {
		if output[0] != 'S' {
			return InventoryCapture{}, WorkspaceProtocolInvalid
		}
		fields, rest, err := parseInventoryFields(output[1:], 8)
		if err != nil {
			return InventoryCapture{}, err
		}
		output = rest
		pid, pidErr := strconv.Atoi(fields[3])
		count, countErr := strconv.Atoi(fields[4])
		if !sessionIDPattern.MatchString(fields[0]) || fields[1] == "" || !paneIDPattern.MatchString(fields[2]) || pidErr != nil || pid < 0 || countErr != nil || count < 0 || seen[fields[0]] || id != "" && fields[0] != id {
			return InventoryCapture{}, WorkspaceProtocolInvalid
		}
		seen[fields[0]] = true
		capture.Rows = append(capture.Rows, InventoryRow{ID: fields[0], Name: fields[1], PaneID: fields[2], PanePID: pid, AttachedClients: count, Title: fields[5], CWD: fields[6], Command: fields[7]})
	}
	var metadata strings.Builder
	for _, row := range capture.Rows {
		branch := commandText("display-message", "-p", "-l", "M"+row.ID)
		for _, option := range []string{"@skid_character", AutoNameOption, GroupOption, "@skid_profile", "@skid_objective_b64"} {
			branch += " ; " + commandText("show-options", "-q", "-t", row.ID, option)
		}
		branch += " ; " + commandText("display-message", "-p", "-l", "N")
		metadata.WriteString(commandText("if-shell", "-F", inventorySessionExists(row.ID), branch, commandText("display-message", "-p", "-l", "D"+row.ID)) + " ; ")
	}
	output, err = client.inventoryBatch(ctx, expected, strings.TrimSuffix(metadata.String(), " ; "))
	if err != nil {
		return InventoryCapture{}, err
	}
	rows := make([]InventoryRow, 0, len(capture.Rows))
	for _, row := range capture.Rows {
		line, rest, found := bytes.Cut(output, []byte{'\n'})
		if !found {
			return InventoryCapture{}, WorkspaceProtocolInvalid
		}
		output = rest
		if string(line) == "D"+row.ID {
			continue
		}
		if string(line) != "M"+row.ID {
			return InventoryCapture{}, WorkspaceProtocolInvalid
		}
		options := make(map[string]string)
		for {
			line, rest, found = bytes.Cut(output, []byte{'\n'})
			if !found {
				return InventoryCapture{}, WorkspaceProtocolInvalid
			}
			output = rest
			if string(line) == "N" {
				break
			}
			option, value, separated := strings.Cut(string(line), " ")
			if !separated {
				return InventoryCapture{}, WorkspaceProtocolInvalid
			}
			if _, duplicate := options[option]; duplicate {
				return InventoryCapture{}, WorkspaceProtocolInvalid
			}
			switch option {
			case "@skid_character", AutoNameOption, GroupOption, "@skid_profile", "@skid_objective_b64":
			default:
				return InventoryCapture{}, WorkspaceProtocolInvalid
			}
			options[option] = ""
			if tmuxCommandTokenPattern.MatchString(value) {
				options[option] = value
			}
		}
		row.LocalSession = SessionMetadata{Character: options["@skid_character"], AutoName: options[AutoNameOption], Group: options[GroupOption], Profile: options["@skid_profile"], Objective: options["@skid_objective_b64"]}
		rows = append(rows, row)
	}
	if len(output) != 0 {
		return InventoryCapture{}, WorkspaceProtocolInvalid
	}
	capture.Rows = rows
	return capture, nil
}

// Each transcript is independently bounded and bracketed by the admitted server.
func (client Client) inventoryBatch(ctx context.Context, expected ServerIdentity, body string) (_ []byte, batchErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	defer func() {
		if batchErr == nil || errors.Is(batchErr, WorkspaceLimit) || errors.Is(batchErr, WorkspaceSourceChanged) {
			return
		}
		if err := ctx.Err(); err != nil {
			if !errors.Is(batchErr, err) {
				batchErr = errors.Join(err, batchErr)
			}
			return
		}
		// A server may disappear after connecting: native shutdown can produce
		// either a child error or a successful but unfinished transcript. Only a
		// positive observation of loss replaces that provisional classification.
		observed, present, err := client.InspectServer(ctx)
		if err != nil {
			batchErr = errors.Join(ctx.Err(), batchErr, err)
			return
		}
		if !present || observed.Identity != expected {
			if errors.Is(batchErr, WorkspaceProtocolInvalid) {
				batchErr = WorkspaceSourceChanged
			} else {
				batchErr = errors.Join(WorkspaceSourceChanged, batchErr)
			}
		}
	}()
	identity := inventoryFields(ServerEpochOption, "pid", "start_time")
	script := commandText("display-message", "-p", "B"+identity) + " ; "
	if body != "" {
		script += commandText("if-shell", "-F", andFormatConditions(serverLifetimeConditions(expected)), body, commandText("display-message", "-p", "-l", identityMismatchMarker)) + " ; "
	}
	script += commandText("display-message", "-p", "E"+identity) + "\n"
	output := recoveryOutput{}
	var stderr bytes.Buffer
	command := client.commandWithStderr(ctx, nil, &stderr, "-N", "source-file", "-")
	command.Stdin = strings.NewReader(script)
	command.Stdout = &output
	err := command.Run()
	if output.overflow {
		return nil, errors.Join(WorkspaceLimit, err, ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("capture tmux inventory: %w", err)
	}
	input := output.bytes
	if len(input) == 0 || input[0] != 'B' {
		return nil, WorkspaceProtocolInvalid
	}
	fields, rest, err := parseInventoryFields(input[1:], 3)
	if err != nil {
		return nil, err
	}
	observed := ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}
	if !serverPIDPattern.MatchString(observed.PID) || !startTimePattern.MatchString(observed.StartTime) {
		return nil, WorkspaceProtocolInvalid
	}
	if observed != expected {
		return nil, WorkspaceSourceChanged
	}
	// Identity records contain only canonical single-line tokens. The final
	// record can be found from the end without splitting any builtin field.
	if len(rest) == 0 || rest[len(rest)-1] != '\n' {
		return nil, WorkspaceProtocolInvalid
	}
	endStart := bytes.LastIndexByte(rest[:len(rest)-1], '\n') + 1
	if rest[endStart] != 'E' {
		return nil, WorkspaceProtocolInvalid
	}
	fields, tail, err := parseInventoryFields(rest[endStart+1:], 3)
	if err != nil || len(tail) != 0 {
		return nil, WorkspaceProtocolInvalid
	}
	observed = ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}
	if !serverPIDPattern.MatchString(observed.PID) || !startTimePattern.MatchString(observed.StartTime) {
		return nil, WorkspaceProtocolInvalid
	}
	if observed != expected {
		return nil, WorkspaceSourceChanged
	}
	return rest[:endStart], nil
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
