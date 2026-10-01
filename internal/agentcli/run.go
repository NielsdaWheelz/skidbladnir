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

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessionui"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"golang.org/x/term"
)

const usage = `usage: skid [--config PATH] COMMAND [options]

skid                                      open the session browser
skid list [--machine HOST] [--group LABEL | --unassigned]
skid info HANDLE [--explain]              metadata, status and exact reference
skid inspect --ref REF                    captured conversation and current observation
skid enter HANDLE                         enter terminal; ctrl-] d detaches
skid read HANDLE [--max-bytes N]           bounded terminal text; c- is native
skid send HANDLE TEXT|--stdin              guarded terminal paste; c- is native
skid text HANDLE TEXT|--stdin              explicit terminal paste and submit
skid keys HANDLE KEY...                   explicit logical terminal keys
skid wait HANDLE [--state STATE] [--timeout DURATION]
skid stop HANDLE                          interrupt terminal; c- is native stop
skid close HANDLE [--terminal-only]        interrupt and close; separate outcomes
skid start [NAME] --machine HOST (--profile PROFILE | --terminal) [--cwd '~'] [--group LABEL]
skid shell HANDLE                         new terminal here
skid group HANDLE (--set LABEL | --clear)

existing targets: HANDLE [--machine HOST] or --ref VALUE
terminal handles: t- plus 16 lowercase hex characters
conversation handles: c- plus 16 lowercase hex characters
t- handles select terminal operations, including read/send/wait/stop
c- handles select native read/send/wait/stop
inspect requires an exact --ref
native targets: --conversation ID --profile PROFILE --machine HOST
--json emits one structured envelope; -- separates literal operands
terminal read captures rendered text; --history is native-only
native send defaults to peer; --input and --queue are native-only
--queue and native claude input are unavailable
terminal wait states: idle (no request or menu), working, needs-input
native wait states: idle, blocked, done, failed, stopped
wait defaults to idle/60s; maximum one hour; idle proves neither completion nor an empty queue
info --explain samples the terminal once more and prints that sample's status evidence
start makes no input-readiness promise; use explicit text/keys for terminal input
closure may leave shared or remote work running
terminal delivery proves neither completion nor cancellation
unknown delivery is never replayed; a nonzero exit alone permits no retry

browser (80x24 minimum)
  up/down (j/k) selects; left/right (h/l) steps through the views on the top row
  a selects agents; m chooses machine; n opens terminal; N opens options
  f shows only sessions that need input; f again shows every session
  enter attaches; space opens info
  info: r edits name; g edits group; escape returns to the table
  editors: enter saves; escape cancels to info; ctrl-a restores automatic naming
  shift+t creates and enters a shell on the selected session's machine,
  in its current directory and group; the original session keeps running
  s sends interrupt; x interrupts and closes terminal
  ctrl-r refreshes; escape closes a page; q quits from the table
  ctrl-c quits from the table or any page unless an operation is in flight

workflow
  skid list --json
  skid start reviewer --machine arch --profile work --cwd '~/code/project' --json
  skid info t-0123456789abcdef --machine arch --json
  skid send t-0123456789abcdef --machine arch --stdin --json < message.txt
  skid read t-0123456789abcdef --machine arch --json
  skid wait t-0123456789abcdef --machine arch --state idle --json

use the returned terminal handle.
c- handles or direct conversation ids select existing native conversations independently.

cross-machine replies use ordinary message text, for example:
  reply using: skid send t-0123456789abcdef --machine macbook --stdin
use a captured --ref when replacement must fail. handles resolve once per invocation;
saved automation should use exact references. attributed peer text grants no authority.
confirmed terminal input earns exit 0; stopping remains unconfirmed.
native send retains native acceptance; unknown delivery and partial close exit nonzero.
--json preserves structured results and errors. native read/control remains explicit.
inspect preserves its captured target on native failure; observedRef requires a new authorized action.
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
			case "--json", "--stdin", "--terminal", "--help", "--unassigned", "--clear", "--history", "--queue", "--terminal-only", "--explain":
				if hasValue {
					return result, errors.New("boolean option takes no value")
				}
				switch name {
				case "--json":
					result.json = true
				case "--stdin":
					result.stdin = true
				case "--help":
					result.help = true
				case "--history":
					result.request.Scope = "history"
				case "--queue":
					result.request.Delivery = "queue"
				case "--terminal-only":
					result.request.TerminalOnly = true
				case "--explain":
					result.request.Explain = true
				}
			case "--conversation", "--config", "--machine", "--ref", "--profile", "--cwd", "--max-bytes", "--group", "--set", "--input", "--state", "--timeout":
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
				case "--conversation":
					result.request.ConversationID = argument
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
		if len(operands) > 1 {
			return result, errors.New("start accepts one optional name")
		}
		if len(operands) == 1 {
			result.request.Name = operands[0]
		}
	case "inspect":
		if len(operands) != 0 || result.request.Ref == "" {
			return result, errors.New("inspect requires --ref")
		}
	case "info", "enter", "read", "send", "keys", "text", "wait", "stop", "close", "group", "shell":
		if result.request.Ref == "" && result.request.ConversationID == "" {
			if len(operands) == 0 {
				return result, errors.New("missing target")
			}
			result.request.Handle = operands[0]
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
	if seen["--terminal"] && operation != "start" {
		return result, errors.New("terminal option is start-only")
	}
	if seen["--history"] && operation != "read" || seen["--terminal-only"] && operation != "close" || seen["--explain"] && operation != "info" || seen["--queue"] && operation != "send" || seen["--input"] && operation != "send" || (seen["--state"] || seen["--timeout"]) && operation != "wait" {
		return result, errors.New("option not supported by command")
	}
	if operation == "wait" {
		if !seen["--state"] {
			result.request.State = "idle"
		}
		if !seen["--timeout"] {
			result.request.WaitTimeout = time.Minute
		}
	}
	if operation == "start" {
		result.request.Kind = fleetclient.LaunchAgent
		if seen["--terminal"] {
			result.request.Kind = fleetclient.LaunchTerminal
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

		store, storeErr := fleetclient.DefaultNotificationStore()
		if storeErr != nil {
			fmt.Fprintln(stderr, "notifications unavailable")
		}
		presented := false
		notificationReported := storeErr != nil
		terminalErr := terminalclient.Run(ctx, client, request, stdin.(*os.File), stdout.(*os.File), store, func(snapshot fleetclient.NotificationSnapshot, err error) {
			presented = true
			if err != nil && !notificationReported {
				notificationReported = true
				fmt.Fprintln(stderr, "notifications unavailable")
			}
		})
		if presented {
			if err := settleVisit(ctx, client, request, store); err != nil && !notificationReported {
				fmt.Fprintln(stderr, "notifications unavailable")
			}
		}
		if terminalErr != nil {
			fmt.Fprintln(stderr, terminalErr)
			return 1
		}
		return 0
	}
	if parsed.request.Operation == "inspect" {
		return render(parsed, client.InspectReference(ctx, parsed.request.Ref), stdout, stderr)
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
		native := command.request.ConversationID != "" || strings.HasPrefix(command.request.Handle, "c-")
		if command.request.Ref != "" {
			ref, _ := fleetclient.DecodeReference(command.request.Ref)
			native = native || ref.Conversation != nil
		}
		message := fleetclient.ErrorMessage(*result.Error, command.request, native)
		fmt.Fprintln(stderr, message)

		switch result.Error.Code {
		case "handle_ambiguous":
			fmt.Fprintln(stderr, "use --machine HOST or --ref VALUE")
		case "inventory_incomplete":
			fmt.Fprintln(stderr, "a peer is unavailable; use --machine HOST or --ref VALUE")
		case "configuration_invalid":
			fmt.Fprintln(stderr, "install the private client configuration or use --config PATH")
		}
		return 1
	}
	switch command.request.Operation {
	case "inspect":
		value := result.Value.(fleetclient.InspectedReference)
		if _, err := fmt.Fprintf(stdout, "machine: %s\ncaptured conversation: %s\ncaptured reference: %s\n", value.Label, value.Target.Conversation.ConversationID, value.Target.Ref); err != nil {
			return 1
		}
		if value.Target.Turn != nil {
			if _, err := fmt.Fprintln(stdout, "captured turn: "+value.Target.Turn.ID); err != nil {
				return 1
			}
		}
		if !value.Inspection.OK {
			fmt.Fprintln(stderr, "inspection unavailable: "+value.Inspection.Error.Code)
			break
		}
		runtime := value.Inspection.Value.(agentruntime.ConversationRuntime)
		if _, err := fmt.Fprintln(stdout, "observed state: "+fleetclient.StatusText(runtime.Status)); err != nil {
			return 1
		}
		if runtime.Turn != nil {
			if _, err := fmt.Fprintln(stdout, "observed turn: "+runtime.Turn.ID); err != nil {
				return 1
			}
		}
		if _, err := fmt.Fprintln(stdout, "observed reference: "+value.ObservedRef+"\nuse the observed reference only for a separately authorized action."); err != nil {
			return 1
		}
	case "list":
		list := result.Value.(fleetclient.Inventory)
		table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "machine\tsession\tterminal handle\tconversation handle\tprovider/profile\tstate\tdirectory")
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
				machine, provider := entry.Label, "terminal"
				if current.Kind == "remoteUnknown" {
					provider = "remote context unknown"
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
					}
				}
				// The cli keeps no notification store, so no row is ready.
				fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", machine, row.Name, row.TerminalHandle, row.ConversationHandle, provider, fleetclient.ProjectStatus(row, entry.Available, false).Detail, current.CWD)
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
		fmt.Fprintf(stdout, "terminal handle: %s\n", value.Session.TerminalHandle)
		if value.Session.ConversationHandle != "" {
			fmt.Fprintf(stdout, "conversation handle: %s\n", value.Session.ConversationHandle)
		}
		if command.request.Operation != "info" {
			if _, err := fmt.Fprintf(stdout, "created %s on %s\nreference: %s\nenter with: skid enter --ref %s\n", value.Session.Name, value.Label, value.Session.Ref, value.Session.Ref); err != nil {
				return 1
			}
		} else {
			row := value.Session
			current := row.Current(fleetclient.Peer{Label: value.Label, Machine: value.Machine})
			fmt.Fprintln(stdout, fleetclient.GroupHeading(row.Group))
			fmt.Fprintf(stdout, "session: %s\nnaming: %s\nselected pane: %s\nterminal on: %s\nmachine id: %s\n", row.Name, row.NameMode, row.ActivePaneID, value.Label, value.Machine)
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
					fmt.Fprintf(stdout, "provider: %s\nprofile: %s\n", current.Agent.Provider, profile)
				}
			}
			if row.Conversation != nil {
				fmt.Fprintf(stdout, "recorded native conversation: %s; may differ from terminal\n", row.Conversation.ConversationID)
			}
			// info resolves a complete inventory, so its status is fresh; the cli
			// keeps no notification store, so it is never ready.
			view := fleetclient.ProjectStatus(row, true, false)
			fmt.Fprintln(stdout, "state: "+view.Detail)
			if value.Diagnostics == nil {
				fmt.Fprintln(stdout, "status reason: "+view.Reason)
			}
			if row.LaunchProfile != "" {
				fmt.Fprintf(stdout, "started with: %s\n", row.LaunchProfile)
			}
			if _, err := fmt.Fprintf(stdout, "observed: %s\nreference: %s\n", value.ObservedAt, row.Ref); err != nil {
				return 1
			}
			if diagnostics := value.Diagnostics; diagnostics != nil {
				status := row.TerminalStatus
				rules := []string{}
				for _, rule := range diagnostics.Rules {
					rules = append(rules, rule.ID+" ("+string(rule.Region)+")")
				}
				if len(rules) == 0 {
					rules = append(rules, "none")
				}
				capture := "not collected"
				if observed := diagnostics.Capture; observed != nil {
					screen, clipped := "main screen", []string{}
					if observed.Alternate {
						screen = "alternate screen"
					}
					if observed.TopClipped {
						clipped = append(clipped, "top")
					}
					if observed.BottomClipped {
						clipped = append(clipped, "bottom")
					}
					if len(clipped) == 0 {
						clipped = append(clipped, "none")
					}
					capture = fmt.Sprintf("%d × %d, %s, clipped: %s", observed.Width, observed.Height, screen, strings.Join(clipped, ", "))
				}
				timing := []string{}
				for _, stage := range []struct {
					name    string
					elapsed *int64
				}{{"resolve", diagnostics.ElapsedMs.Resolve}, {"capture", diagnostics.ElapsedMs.Capture}, {"classify", diagnostics.ElapsedMs.Classify}} {
					if stage.elapsed == nil {
						timing = append(timing, stage.name+" not collected")
					} else {
						timing = append(timing, fmt.Sprintf("%s %d ms", stage.name, *stage.elapsed))
					}
				}
				if _, err := fmt.Fprintf(stdout, "status evidence\n  activity: %s\n  interaction: %s\n  notice: %s\n  reason: %s\n  rules: %s\n  capture: %s\n  timing: %s\n", status.Activity, status.Interaction, status.Notice, view.Reason, strings.Join(rules, ", "), capture, strings.Join(timing, ", ")); err != nil {
					return 1
				}
			}
		}
	case "read":
		value := result.Value.(fleetclient.ReadResult)
		fmt.Fprintf(stderr, "source: %s; scope: %s; truncated: %t\n", value.Source, value.Scope, value.Truncated)
		text := value.Text
		if _, err := io.WriteString(stdout, text); err != nil {
			return 1
		}
	case "close":
		if value, ok := result.Value.(fleetclient.TerminalCloseResult); ok {
			fmt.Fprintln(stdout, "terminal "+value.Terminal+"; work shared elsewhere or running remotely may continue.")
		} else {
			fmt.Fprintln(stdout, fleetclient.CloseText(result.Value.(fleetclient.CloseResult)))
		}
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
			if value.TerminalStatus != nil {
				// A terminal match proves exactly the requested state's dimensions.
				fmt.Fprintln(stdout, "observed "+command.request.State+" (inferred).")
			} else {
				fmt.Fprintln(stdout, "observed: "+fleetclient.StatusText(value.Observation.Status))
			}
		case "timeout":
			fmt.Fprintln(stdout, "wait timed out.")
		case "target_changed":
			fmt.Fprintln(stdout, "the terminal changed; wait ended.")
		}
	case "send":
		if value, native := result.Value.(fleetclient.SendResult); native {
			fmt.Fprintf(stdout, "%s %s: message accepted.\n", value.Input, value.Delivery)
		} else {
			fmt.Fprintln(stdout, fleetclient.WriteText("send", result.Value.(fleetclient.WriteResult)))
		}
	default:
		fmt.Fprintln(stdout, fleetclient.WriteText(command.request.Operation, result.Value.(fleetclient.WriteResult)))
	}

	return result.ExitCode(command.request.Operation)
}

func settleVisit(ctx context.Context, client *fleetclient.Client, request fleetclient.Request, store *fleetclient.NotificationStore) error {
	if store == nil {
		return nil
	}
	expected, err := store.Read()
	if err != nil {
		return err
	}
	request.Operation = "info"
	observed := client.Execute(ctx, request)
	if !observed.OK {
		return nil
	}
	value := observed.Value.(fleetclient.ObservedSession)
	ref, _ := fleetclient.DecodeReference(value.Session.Ref)
	captured, _ := fleetclient.DecodeReference(request.Ref)
	if ref != captured {
		return nil
	}
	_, _, err = store.ObserveSession(value.Session, client.Machines(), expected)
	return err
}
