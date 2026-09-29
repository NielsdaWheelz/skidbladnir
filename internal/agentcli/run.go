// Package agentcli parses ordinary commands and renders the shared fleet results.
package agentcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessionui"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"golang.org/x/term"
)

const usage = `usage: skid [--config PATH] COMMAND [options]

skid                                      open the session browser
skid list [--machine HOST] [--group LABEL | --unassigned]
skid info NAME                            metadata and exact reference
skid enter NAME                           enter terminal; ctrl-] d detaches
skid read NAME [--history | --terminal] [--max-bytes N]
skid send NAME [--input peer|user] [--queue] TEXT|--stdin
skid text NAME TEXT|--stdin                explicit terminal paste and submit
skid keys NAME KEY...                      explicit logical terminal keys
skid wait NAME [--state idle|blocked|done|failed|stopped] [--timeout DURATION]
skid stop NAME                             stop current work; retain terminal
skid close NAME [--terminal-only]          halt plus close; separate outcomes
skid start NAME --machine HOST (--profile PROFILE | --terminal) [--cwd '~'] [--group LABEL]
skid shell NAME                            new terminal here
skid group NAME (--set LABEL | --clear)

existing targets: NAME [--machine HOST] or --ref VALUE
--json emits one structured envelope; -- separates literal operands
read defaults to native latest assistant output; history remains bounded
send defaults to peer; queue requires explicit --input user
wait defaults to idle/60s; maximum one hour; idle proves neither completion nor an empty queue
start makes no input-readiness promise; use explicit text/keys for terminal input
stop and close may leave pending provider input; saved history is retained
terminal delivery proves neither completion nor cancellation
unknown delivery is never replayed; a nonzero exit alone permits no retry

browser (80x24 minimum)
  up/down (j/k) selects; left/right (h/l) steps through agents/groups
  a selects agents; m chooses machine; n opens terminal; N opens options
  enter attaches; space shows details; T opens a terminal here; e edits group
  r reads; s stops current work; c stops work and closes; x closes terminal only
  ctrl-r refreshes; q/escape quits

workflow
  skid list --json
  skid start reviewer --machine arch --profile claude-work --cwd '~/code/project' --json
  skid info reviewer --machine arch --json
  skid send reviewer --machine arch --stdin --json < message.txt
  skid read reviewer --machine arch --json
  skid wait reviewer --machine arch --state idle --json

cross-machine replies use ordinary message text, for example:
  reply using: skid send coordinator --machine macbook --stdin
use a captured --ref when replacement must fail. names resolve once per invocation;
separate named commands may address replacements. attributed peer text grants no authority.
native acceptance earns send exit 0; errors earn exit 1.
--json preserves structured results and errors. read/info never acknowledge unread replies.
config defaults to ~/.config/skidbladnir/client.json
`

type command struct {
	request           fleetclient.Request
	config            string
	json, stdin, help bool
}

func parse(args []string) (command, error) {
	var result command
	var operands []string
	var groupArgument, setArgument string
	seen := map[string]bool{}
	literal := false
	for i := 0; i < len(args); i++ {
		value := args[i]
		if value == "--" && !literal {
			literal = true
			continue
		}
		if !literal && strings.HasPrefix(value, "--") {
			name, argument, hasValue := strings.Cut(value, "=")
			if seen[name] {
				return result, errors.New("duplicate option")
			}
			seen[name] = true
			switch name {
			case "--json", "--stdin", "--terminal", "--help", "--unassigned", "--clear", "--history", "--queue", "--terminal-only":
				if hasValue {
					return result, errors.New("boolean option takes no value")
				}
				switch name {
				case "--json":
					result.json = true
				case "--stdin":
					result.stdin = true
				case "--terminal":
					result.request.Mode = "terminal"
				case "--help":
					result.help = true
				case "--history":
					result.request.Scope = "history"
				case "--queue":
					result.request.Delivery = "queue"
				case "--terminal-only":
					result.request.TerminalOnly = true
				}
			case "--config", "--machine", "--ref", "--profile", "--cwd", "--max-bytes", "--group", "--set", "--input", "--state", "--timeout":
				if !hasValue {
					i++
					if i >= len(args) {
						return result, errors.New("missing option value")
					}
					argument = args[i]
				}
				if argument == "" {
					return result, errors.New("empty option value")
				}
				switch name {
				case "--input":
					result.request.Input = argument
				case "--state":
					result.request.State = argument
				case "--timeout":
					duration, err := time.ParseDuration(argument)
					if err != nil || duration <= 0 || duration > time.Hour {
						return result, errors.New("invalid wait timeout")
					}
					result.request.WaitTimeout = duration
				case "--group":
					groupArgument = argument
				case "--set":
					setArgument = argument
				case "--config":
					result.config = argument
				case "--machine":
					result.request.Machine = argument
				case "--ref":
					result.request.Ref = argument
				case "--profile":
					result.request.Profile = argument
				case "--cwd":
					result.request.CWD = argument
				case "--max-bytes":
					n, err := strconv.Atoi(argument)
					if err != nil || n < 1 || n > 32768 {
						return result, errors.New("invalid read limit")
					}
					result.request.MaxBytes = n
				}
			default:
				return result, errors.New("unknown option")
			}
		} else {
			operands = append(operands, value)
		}
	}
	if result.help {
		return result, nil
	}
	if len(operands) == 0 {
		for option := range seen {
			if option != "--config" {
				return result, errors.New("missing command")
			}
		}
		return result, nil
	}
	result.request.Operation = operands[0]
	operands = operands[1:]
	switch result.request.Operation {
	case "list":
		if len(operands) != 0 {
			return result, errors.New("list takes no target")
		}
	case "start":
		if len(operands) != 1 {
			return result, errors.New("start requires name")
		}
		result.request.Name = operands[0]
	case "info", "enter", "read", "send", "keys", "text", "wait", "stop", "close", "group", "shell":
		if result.request.Ref == "" {
			if len(operands) == 0 {
				return result, errors.New("missing target")
			}
			result.request.Name = operands[0]
			operands = operands[1:]
		}
		switch result.request.Operation {
		case "send", "text":
			if result.stdin {
				if len(operands) != 0 {
					return result, errors.New("stdin and positional text are exclusive")
				}
			} else {
				if len(operands) != 1 {
					return result, errors.New("send requires text or --stdin")
				}
				result.request.Text = operands[0]
			}
		case "keys":
			result.request.Keys = operands
		default:
			if len(operands) != 0 {
				return result, errors.New("extra operand")
			}
		}
	default:
		return result, errors.New("unknown command")
	}
	operation := result.request.Operation
	if operation == "send" {
		if !seen["--input"] {
			result.request.Input = "peer"
		}
		if result.request.Delivery == "" {
			result.request.Delivery = "direct"
		}
		if seen["--queue"] && (!seen["--input"] || result.request.Input != "user") {
			return result, errors.New("queue requires explicit user input")
		}
	}
	if operation == "read" && result.request.Mode == "" {
		result.request.Mode = "native"
		if result.request.Scope == "" {
			result.request.Scope = "latest"
		}
	}
	if seen["--history"] && operation != "read" || seen["--terminal-only"] && operation != "close" || seen["--queue"] && operation != "send" || seen["--input"] && operation != "send" || (seen["--state"] || seen["--timeout"]) && operation != "wait" {
		return result, errors.New("option not supported by command")
	}
	if operation == "start" {
		result.request.Kind = fleetclient.LaunchAgent
		if seen["--terminal"] {
			result.request.Kind = fleetclient.LaunchTerminal
			result.request.Mode = ""
		}
	}
	if seen["--group"] {
		if operation != "list" && operation != "start" || seen["--unassigned"] {
			return result, errors.New("group option not supported")
		}
		label, err := group.ParseDraft(groupArgument)
		if err != nil || label.IsUnassigned() {
			return result, errors.New("invalid group")
		}
		if operation == "list" {
			result.request.GroupFilter, _ = group.NamedFilter(label)
		} else {
			result.request.Group = label
		}
	}
	if seen["--unassigned"] {
		if operation != "list" {
			return result, errors.New("unassigned is list-only")
		}
		result.request.GroupFilter = group.UnassignedFilter()
	}
	if operation == "group" {
		if seen["--set"] == seen["--clear"] {
			return result, errors.New("choose set or clear")
		}
		if seen["--set"] {
			label, err := group.ParseDraft(setArgument)
			if err != nil || label.IsUnassigned() {
				return result, errors.New("invalid group")
			}
			result.request.Group = label
		}
	} else if seen["--set"] || seen["--clear"] {
		return result, errors.New("assignment option not supported")
	}
	if result.stdin && result.request.Operation != "send" && result.request.Operation != "text" || result.json && result.request.Operation == "enter" {
		return result, errors.New("option not supported by command")
	}
	request := result.request
	if result.stdin {
		request.Text = "stdin"
	}
	if !request.Valid() {
		return result, errors.New("invalid command arguments")
	}
	return result, nil
}

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	parsed, err := parse(args)
	if err != nil {
		// Find --json even when an earlier invalid option stopped parsing.
		for _, arg := range args {
			if arg == "--" {
				break
			}
			if arg == "--json" {
				parsed.json = true
			}
		}
		if parsed.json {
			encoded, _ := fleetclient.Failed("invalid_input", "not_sent").Encode("")
			stdout.Write(encoded)
		} else {
			io.WriteString(stderr, usage)
		}
		return 2
	}
	if parsed.help {
		if _, err := io.WriteString(stdout, usage); err != nil {
			return 1
		}
		return 0
	}
	if parsed.request.Operation == "" || parsed.request.Operation == "enter" {
		input, inOK := stdin.(*os.File)
		output, outOK := stdout.(*os.File)
		if !inOK || !outOK || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
			io.WriteString(stderr, usage+"\nskid browser and enter require stdin and stdout ttys; use skid list\n")
			return 2
		}
	}
	if parsed.stdin {
		text, err := io.ReadAll(io.LimitReader(stdin, 32769))
		if err != nil {
			return render(parsed, fleetclient.Failed("input_unavailable", "not_sent"), stdout, stderr)
		}
		if len(text) > 32768 {
			return render(parsed, fleetclient.Failed("input_limit", "not_sent"), stdout, stderr)
		}
		parsed.request.Text = string(text)
		if !parsed.request.Valid() {
			return render(parsed, fleetclient.Failed("invalid_input", "not_sent"), stdout, stderr)
		}
	}
	if parsed.config == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return render(parsed, fleetclient.Failed("configuration_invalid", "not_sent"), stdout, stderr)
		}
		parsed.config = filepath.Join(home, ".config", "skidbladnir", "client.json")
	}
	client, err := fleetclient.Open(parsed.config)
	if err != nil {
		return render(parsed, fleetclient.Failed("configuration_invalid", "not_sent"), stdout, stderr)
	}
	if parsed.request.Operation == "" {
		if sessionui.Run(ctx, client, stdin.(*os.File), stdout.(*os.File)) != nil {
			io.WriteString(stderr, "session browser ended with an error\n")
			return 1
		}
		return 0
	}
	if parsed.request.Operation == "enter" {
		io.WriteString(stderr, "ctrl-] then d detaches; navigation and terminal size are shared\n")
		info := parsed.request
		info.Operation = "info"
		observed := client.Execute(ctx, info)
		if !observed.OK {
			return render(parsed, observed, stdout, stderr)
		}
		row := observed.Value.(fleetclient.ObservedSession).Session
		request := fleetclient.Request{Operation: "enter", Ref: row.Ref}
		var onHello func() error
		store, storeErr := fleetclient.DefaultUnreadStore()
		if storeErr == nil {
			snapshot, err := store.Read()
			if err != nil {
				fmt.Fprintln(stderr, "unread unavailable")
			} else {
				ref, _ := fleetclient.DecodeReference(row.Ref)
				if conversation, found := snapshot.Conversation(ref, row.ActivePaneID); found {
					key := fleetclient.ReplyKey(ref.Machine, conversation)
					if record, found := snapshot.Record(key); found && len(record.UnreadIDs) > 0 {
						ids := append([]string(nil), record.UnreadIDs...)
						onHello = func() error { _, err := store.Acknowledge(key, ids); return err }
					}
				}
			}
		} else {
			fmt.Fprintln(stderr, "unread unavailable")
		}
		if err := terminalclient.Run(ctx, client, request, stdin.(*os.File), stdout.(*os.File), onHello); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	return render(parsed, client.Execute(ctx, parsed.request), stdout, stderr)
}

func render(command command, result fleetclient.Result, stdout, stderr io.Writer) int {
	if command.json {
		encoded, err := result.Encode(command.request.Operation)
		if err != nil {
			return 1
		}
		if _, err := stdout.Write(encoded); err != nil {
			return 1
		}
		return result.ExitCode(command.request.Operation)
	}
	if !result.OK {
		message := result.Error.Code + " (" + result.Error.Dispatch + ")"
		if result.Error.Dispatch == "unknown" {
			message = "could not confirm the request. check the terminal before trying again."
		} else if result.Error.Code == "AgentTargetStale" || result.Error.Code == "SessionIdentityMismatch" {
			message = "the session changed. refresh and try again."
		} else if result.Error.Code == "AgentUnavailable" {
			message = "this action is unavailable for this session."
		}
		fmt.Fprintln(stderr, message)
		switch result.Error.Code {
		case "name_ambiguous":
			for _, candidate := range result.Candidates {
				fmt.Fprintln(stderr, "  "+candidate)
			}
			fmt.Fprintln(stderr, "use --machine HOST or --ref VALUE")
		case "inventory_incomplete":
			fmt.Fprintln(stderr, "a peer is unavailable; use --machine HOST or --ref VALUE")
		case "configuration_invalid":
			fmt.Fprintln(stderr, "install the private client configuration or use --config PATH")
		}
		return 1
	}
	switch command.request.Operation {
	case "list":
		list := result.Value.(fleetclient.Inventory)
		table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "machine\tsession\tprovider/profile\tstate\tdirectory")
		owners := make(map[string]fleetclient.Peer, len(list.Peers))
		for _, peer := range list.Peers {
			owners[peer.Machine] = peer
			if !peer.OK {
				fmt.Fprintf(table, "%s\tunavailable\t\t%s\t\n", peer.Label, peer.Error.Code)
			}
		}
		groups := fleetclient.Groups(list.Peers, group.Filter{})
		for _, group := range groups {
			fmt.Fprintln(table, fleetclient.GroupHeading(group.Label))
			for _, entry := range group.Rows {
				row := entry.Session
				current := row.Current(owners[entry.Machine])
				machine, provider, state := entry.Label, "terminal", "terminal"
				if current.Kind == "remoteUnknown" {
					provider, state = "remote context unknown", "status unavailable"
				} else {
					if current.Kind == "remote" {
						machine += " → " + current.Label
					}
					if current.Agent != nil {
						provider = current.Agent.Provider
						if current.Agent.Label != "" {
							provider += "/" + current.Agent.Label
						} else {
							provider += "/profile unknown"
						}
						state = fleetclient.StatusText(current.Agent.Status)
					}
				}
				fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", machine, row.Name, provider, state, current.CWD)
			}
		}
		if len(groups) == 0 {
			if list.Partial {
				fmt.Fprintln(table, "no matching sessions in available inventory")
			} else {
				fmt.Fprintln(table, "no sessions in this view")
			}
		}
		if table.Flush() != nil {
			return 1
		}
	case "info", "start", "shell":
		value := result.Value.(fleetclient.ObservedSession)
		if command.request.Operation != "info" {
			if _, err := fmt.Fprintf(stdout, "created %s on %s\nreference: %s\nenter with: skid enter --ref %s\n", value.Session.Name, value.Label, value.Session.Ref, value.Session.Ref); err != nil {
				return 1
			}
		} else {
			row := value.Session
			current := row.Current(fleetclient.Peer{Label: value.Label, Machine: value.Machine})
			fmt.Fprintln(stdout, fleetclient.GroupHeading(row.Group))
			fmt.Fprintf(stdout, "session: %s\nterminal on: %s\nmachine id: %s\n", row.Name, value.Label, value.Machine)
			if current.Kind == "remote" {
				fmt.Fprintf(stdout, "running on: %s\n", current.Label)
			}
			if current.Kind == "remoteUnknown" {
				fmt.Fprintln(stdout, "remote context unknown")
			} else {
				cwd := current.CWD
				if cwd == "" {
					cwd = "directory unavailable"
				}
				fmt.Fprintf(stdout, "directory: %s\n", cwd)
				if current.Agent == nil {
					fmt.Fprintln(stdout, "agent: not detected")
				} else {
					profile := current.Agent.Label
					if profile == "" {
						profile = "profile unknown"
					}
					fmt.Fprintf(stdout, "provider: %s\nprofile: %s\nstate: %s\n", current.Agent.Provider, profile, fleetclient.StatusText(current.Agent.Status))
					if current.Kind == "local" && row.Agent != nil {
						fmt.Fprintf(stdout, "read: %s; peer send: %s; stop: %s\n", row.Agent.Methods.Read, row.Agent.Methods.SendPeer, row.Agent.Methods.Stop)
					}
				}
			}
			if row.LaunchProfile != "" {
				fmt.Fprintf(stdout, "started with: %s\n", row.LaunchProfile)
			}
			if _, err := fmt.Fprintf(stdout, "observed: %s\nreference: %s\n", value.ObservedAt, row.Ref); err != nil {
				return 1
			}
		}
	case "read":
		value := result.Value.(fleetclient.ReadResult)
		fmt.Fprintf(stderr, "source: %s; scope: %s; truncated: %t\n", value.Source, value.Scope, value.Truncated)
		if _, err := io.WriteString(stdout, value.Text); err != nil {
			return 1
		}
	case "close":
		if value, ok := result.Value.(fleetclient.TerminalCloseResult); ok {
			fmt.Fprintln(stdout, "terminal "+value.Terminal+"; pending input may remain; saved history is retained.")
			break
		}
		value := result.Value.(fleetclient.CloseResult)
		text := "current work: " + value.Agent + "; terminal: " + value.Terminal + "; pending input may remain; saved history is retained."
		if value.Reason == "stale" {
			text = "terminal left open because the session changed."
		} else if value.Terminal == "closed" && value.Agent == "unconfirmed" {
			text = "terminal closed; agent stop unconfirmed."
		}
		fmt.Fprintln(stdout, text)
	case "group":
		text := "group assigned\n"
		if command.request.Group.IsUnassigned() {
			text = "group cleared\n"
		}
		if _, err := io.WriteString(stdout, text); err != nil {
			return 1
		}
	case "wait":
		value := result.Value.(fleetclient.WaitResult)
		switch value.Outcome {
		case "matched":
			fmt.Fprintln(stdout, "observed: "+fleetclient.StatusText(value.Observation.Status))
		case "timeout":
			fmt.Fprintln(stdout, "wait timed out.")
		case "target_changed":
			fmt.Fprintln(stdout, "the session changed; wait ended.")
		}
	case "send":
		value := result.Value.(fleetclient.SendResult)
		text := "message accepted."
		if value.Delivery == "queue" {
			text = "queued input accepted; it may already be running."
		}
		fmt.Fprintf(stdout, "%s %s: %s\n", value.Input, value.Delivery, text)
	default:
		value := result.Value.(fleetclient.WriteResult)
		text := "current work: " + value.Outcome + "; pending input may remain."
		if value.Outcome == "unknown" {
			text = "could not confirm the request. check the terminal before trying again."
		} else if value.Method == "terminal" {
			text = "keys sent; agent state not confirmed."
			if command.request.Operation == "text" {
				text = "text sent; agent state not confirmed."
			}
		}
		fmt.Fprintln(stdout, text)
	}
	return result.ExitCode(command.request.Operation)
}
