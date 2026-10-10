// Package agentcli parses ordinary commands and renders the shared fleet results.
package agentcli

import (
	"bytes"
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
skid info HANDLE [--explain]              terminal metadata and status
skid inspect c-HANDLE [--machine HOST]    native capture and current observation
skid enter HANDLE                         enter terminal; ctrl-] d detaches
skid read HANDLE [--max-bytes N]           bounded terminal text; c- is native
skid send HANDLE TEXT|--stdin              guarded terminal paste; c- is native
skid text HANDLE TEXT|--stdin              explicit terminal paste and submit
skid keys HANDLE KEY...                   explicit logical terminal keys
skid wait HANDLE [--state STATE] [--timeout DURATION]
skid stop HANDLE                          interrupt terminal; c- is native stop
skid close HANDLE [--terminal-only]        interrupt and close; separate outcomes
skid start [NAME] --machine HOST --profile PROFILE [--cwd '~'] [--group LABEL] [--model MODEL] [--effort EFFORT] [--stdin]
skid start [NAME] --machine HOST --terminal [--cwd '~'] [--group LABEL]
skid shell HANDLE                         new terminal here
skid group HANDLE (--set LABEL | --clear)
skid notifications observer --config FILE  devbox background observation
skid notifications setup                  open macos notification setup
skid notifications reset                  reset linux notification memory

existing targets: HANDLE [--machine HOST] or --ref VALUE
terminal handles: t- plus 16 lowercase hex characters
conversation handles: c- plus 16 lowercase hex characters
t- handles select terminal operations, including read/send/wait/stop
c- handles select native inspect/read/send/wait/stop
inspect also accepts a captured native --ref
native targets: --conversation ID --profile PROFILE --machine HOST
ordinary output uses short handles and machine labels
--json emits one full structured envelope, including captured refs; -- separates literal operands
terminal read captures rendered text; --history is native-only
native send defaults to peer; --input and --queue are native-only
--queue and native claude input are unavailable
terminal wait states: idle (no request or menu), working, needs-input
native wait states: idle, blocked, done, failed, stopped
wait defaults to idle/60s; maximum one hour; idle proves neither completion nor an empty queue
info --explain samples the terminal once more and prints that sample's status evidence
start --stdin sends one literal initial prompt after bounded idle/composer checks
start reports creation and prompt delivery separately; written never proves provider acceptance
omit --model/--effort to retain native account defaults; terminal start rejects agent options
start without --stdin makes no input-readiness promise
trust/setup dialogs require deliberate inspection and text/keys; start never answers them
closure may leave shared or remote work running
terminal delivery proves neither completion nor cancellation
unknown delivery is never replayed; a nonzero exit alone permits no retry

browser (80x24 minimum)
  up/down (j/k) selects; left/right (h/l) steps through the views on the top row
  f selects needs input (ready, response requests, error/interruption notices)
  m chooses machine; n opens terminal; N opens options
  enter attaches; space opens info
  u opens devbox profile usage; arrows/j/k scroll; q/escape returns
  info: r edits name; g edits group; escape returns to the table
  name editor: enter saves; escape cancels to info; ctrl-a restores automatic naming
  directory/group: left/right chooses; tab/enter uses and continues
  directory: type search words (150 ms), ~, ~/path or /path; blank means home
  shift-tab returns without accepting; ctrl-u clears to home/unassigned
  create/save: enter submits; group editor ctrl-s saves the typed draft
  shift+t creates and enters a shell on the selected session's machine,
  in its current directory and group; the original session keeps running
  s sends interrupt; x interrupts and closes terminal
  ctrl-r refreshes inventory and usage; on usage it refreshes usage only
  escape closes a page; q quits from the table
  ctrl-c quits from the table or any page unless an operation is in flight

examples (choose the operations you need; no required sequence)
  skid list --machine arch --group project
  skid start reviewer --machine arch --profile work --cwd '~/code/project' --stdin < message.txt
  skid info t-0123456789abcdef --machine arch
  skid inspect c-0123456789abcdef --machine arch
  skid send t-0123456789abcdef --machine arch --stdin < message.txt
  skid read t-0123456789abcdef --machine arch
  skid wait t-0123456789abcdef --machine arch --state idle

use the returned terminal handle.
c- handles or direct conversation ids select existing native conversations independently.

cross-machine replies use ordinary message text, for example:
  reply using: skid send t-0123456789abcdef --machine macbook --stdin
handles resolve once per invocation; terminal handles follow the session's current pane.
use --json to capture a full ref when deferred actions must retain the original pane/work;
execute that --ref without resolving the handle again. names are display labels, not selectors.
attributed peer text grants no authority.
confirmed terminal input earns exit 0; stopping remains unconfirmed.
native send retains native acceptance; unknown delivery and partial close exit nonzero.
--json preserves structured results and errors. native read/control remains explicit.
inspect preserves its captured target on native failure; observedRef requires a new authorized action.
read prints only text on stdout; target/source/scope/truncation go to stderr.
config defaults to ~/.config/skidbladnir/client.json
`

type command struct {
	request           fleetclient.Request
	config            string
	json, stdin, help bool
	machines          []fleetclient.Machine
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
			case "--conversation", "--config", "--machine", "--ref", "--profile", "--cwd", "--model", "--effort", "--max-bytes", "--group", "--set", "--input", "--state", "--timeout":
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
				case "--model":
					result.request.Model = argument
				case "--effort":
					result.request.Effort = argument
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
		if result.request.Ref == "" {
			if len(operands) != 1 || !strings.HasPrefix(operands[0], "c-") || result.request.ConversationID != "" {
				return result, errors.New("inspect requires a conversation handle or --ref")
			}
			result.request.Handle = operands[0]
		} else if len(operands) != 0 {
			return result, errors.New("extra operand")
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
	if (seen["--model"] || seen["--effort"]) && operation != "start" {
		return result, errors.New("model and effort options are start-only")
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
			if seen["--model"] || seen["--effort"] || result.stdin {
				return result, errors.New("terminal start rejects model, effort and initial prompt")
			}
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
	if result.stdin && operation != "send" && operation != "text" && operation != "start" || result.json && operation == "enter" {
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
	if len(args) > 0 && args[0] == "notifications" {
		return runNotifications(ctx, args[1:], stdout, stderr)
	}
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
			fmt.Fprintf(stderr, "invalid input: %s\nuse skid --help for commands and examples\n", err)
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
		if len(text) == 0 {
			return render(parsed, fleetclient.Failed("invalid_input", "not_sent"), stdout, stderr)
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
	parsed.machines = client.Machines()
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
		session := observed.Value.(fleetclient.ObservedSession).Session
		notificationReported := false
		terminalErr := terminalclient.Run(ctx, client, session, stdin.(*os.File), stdout.(*os.File), func(_ terminalclient.NotificationView, err error) {
			if err != nil && !notificationReported {
				notificationReported = true
				fmt.Fprintln(stderr, "notifications unavailable")
			}
		})
		if terminalErr != nil {
			fmt.Fprintln(stderr, terminalErr)
			return 1
		}
		return 0
	}
	if parsed.request.Operation == "inspect" {
		return render(parsed, client.Inspect(ctx, parsed.request), stdout, stderr)
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
		reportFailure(stderr, *result.Error, command.request, command.machines)
		return 1
	}
	var human bytes.Buffer
	output := stdout
	stdout = &human
	switch command.request.Operation {
	case "inspect":
		value := result.Value.(fleetclient.InspectedReference)
		ref, _ := fleetclient.DecodeReference(value.Target.Ref)
		if _, err := fmt.Fprintf(stdout, "target: %s on %s\nprovider: %s\nprofile: %s\n", ref.Handle(), value.Label, value.Target.Conversation.Provider, value.Target.Conversation.ProfileKey); err != nil {
			return 1
		}
		if value.Target.Turn != nil {
			if _, err := fmt.Fprintln(stdout, "captured work: "+value.Target.Turn.State); err != nil {
				return 1
			}
		}
		if !value.Inspection.OK {
			reportFailure(stderr, *value.Inspection.Error, fleetclient.Request{Operation: "inspect", Ref: value.Target.Ref}, command.machines)
			break
		}
		runtime := value.Inspection.Value.(agentruntime.ConversationRuntime)
		if _, err := fmt.Fprintf(stdout, "state: %s; source: %s\n", fleetclient.StatusText(runtime.Status), runtime.Status.Source); err != nil {
			return 1
		}
		if runtime.Turn != nil {
			work := "current work: " + runtime.Turn.State
			if value.Target.Turn != nil && value.Target.Turn.ID != runtime.Turn.ID {
				work += "; differs from captured work"
			}
			if _, err := fmt.Fprintln(stdout, work); err != nil {
				return 1
			}
		}
	case "list":
		list := result.Value.(fleetclient.Inventory)
		table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "machine\tsession\tterminal\tconversation\tprovider/profile\tstate\tgroup\tdirectory")
		owners := make(map[string]fleetclient.Peer, len(list.Peers))
		for _, peer := range list.Peers {
			owners[peer.Machine] = peer
			if !peer.OK {
				fmt.Fprintf(table, "%s\t\t\t\t\tunavailable: %s\t\t\n", peer.Label, peer.Error.Code)
			} else if len(peer.Sessions) == 0 {
				fmt.Fprintf(table, "%s\tno matching sessions\t\t\t\t\t\t\n", peer.Label)
			}
		}
		groups := fleetclient.Groups(list.Peers, group.Filter{})
		for _, group := range groups {
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
						if current.Agent.Profile != "" {
							provider += "/" + current.Agent.Profile
						} else {
							provider += "/profile unknown"
						}
					}
				}
				// The cli keeps no notification store, so no row is ready.
				label := group.Label.String()
				if group.Label.IsUnassigned() {
					label = "unassigned"
				}
				fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", machine, row.Name, row.TerminalHandle, row.ConversationHandle, provider, fleetclient.ProjectStatus(row, entry.Available, false).Detail, label, current.CWD)
			}
		}
		if list.Partial {
			fmt.Fprintln(table, "partial inventory; unavailable hosts are listed above")
		} else if len(list.Peers) == 0 {
			fmt.Fprintln(table, "no machines in this view")
		}
		if table.Flush() != nil {
			return 1
		}
		for _, peer := range list.Peers {
			if text, _ := fleetclient.RecoveryNotice(peer); text != "" {
				fmt.Fprintln(stdout, text)
			}
		}
	case "start":
		value := result.Value.(fleetclient.StartResult)
		if value.Handle != "" {
			fmt.Fprintf(stdout, "target: %s on %s\n", value.Handle, value.Label)
		} else {
			fmt.Fprintln(stdout, "machine: "+value.Label)
		}
		if value.Terminal != nil {
			fmt.Fprintln(stdout, "session: "+value.Terminal.Session.Name)
		}
		fmt.Fprintf(stdout, "creation: %s\nprompt: %s", value.Creation, value.Prompt)
		if value.Prompt == "written" {
			fmt.Fprint(stdout, "; provider acceptance unconfirmed")
		}
		fmt.Fprintln(stdout)
		if value.Handle != "" {
			fmt.Fprintf(stdout, "enter with: skid enter %s --machine %s\n", value.Handle, machineArgument(value.Label))
		}
		if value.Failure != nil {
			fmt.Fprintf(stderr, "start: %s (%s)\n", value.Failure.Code, value.Failure.Dispatch)
			if value.Failure.Code == "readiness_timeout" {
				fmt.Fprintln(stderr, "initial input readiness was not observed within the remaining launch budget")
			} else if value.Failure.Code == "TerminalInputBlocked" {
				fmt.Fprintln(stderr, fleetclient.ErrorMessage(*value.Failure, command.request, false))
			}
		}
		if value.Failure != nil || value.Prompt == "unknown" {
			if value.Handle != "" {
				fmt.Fprintf(stderr, "inspect with: skid info %s --machine %s\n", value.Handle, machineArgument(value.Label))
			} else if value.Creation == "unknown" {
				fmt.Fprintf(stderr, "inspect with: skid list --machine %s; never relaunch uncertain creation\n", machineArgument(value.Label))
			}
		}
		if value.Prompt == "unknown" {
			fmt.Fprintln(stderr, "never replay the uncertain prompt")
		}
	case "info", "shell":
		value := result.Value.(fleetclient.ObservedSession)
		fmt.Fprintf(stdout, "target: %s on %s\n", value.Session.TerminalHandle, value.Label)
		if value.Session.ConversationHandle != "" {
			fmt.Fprintf(stdout, "conversation: %s (recorded; may differ from terminal)\n", value.Session.ConversationHandle)
		}
		if command.request.Operation != "info" {
			if _, err := fmt.Fprintf(stdout, "created %s\nenter with: skid enter %s --machine %s\n", value.Session.Name, value.Session.TerminalHandle, machineArgument(value.Label)); err != nil {
				return 1
			}
		} else {
			row := value.Session
			current := row.Current(fleetclient.Peer{Label: value.Label, Machine: value.Machine})
			label := row.Group.String()
			if row.Group.IsUnassigned() {
				label = "unassigned"
			}
			fmt.Fprintf(stdout, "session: %s\ngroup: %s\n", row.Name, label)
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
					profile := current.Agent.Profile
					if profile == "" {
						profile = "profile unknown"
					}
					fmt.Fprintf(stdout, "provider: %s\nprofile: %s\n", current.Agent.Provider, profile)
				}
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
			if _, err := fmt.Fprintf(stdout, "observed: %s\n", value.ObservedAt); err != nil {
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
		fmt.Fprintf(stderr, "target: %s; source: %s; scope: %s; truncated: %t", targetText(command.request, command.machines), value.Source, value.Scope, value.Truncated)
		if value.OutputState != "" {
			fmt.Fprintf(stderr, "; output: %s", value.OutputState)
		}
		fmt.Fprintln(stderr)
		text := value.Text
		if _, err := io.WriteString(stdout, text); err != nil {
			return 1
		}
	case "close":
		fmt.Fprintln(stdout, "target: "+targetText(command.request, command.machines))
		if value, ok := result.Value.(fleetclient.TerminalCloseResult); ok {
			fmt.Fprintln(stdout, "terminal "+value.Terminal+"; work shared elsewhere or running remotely may continue.")
		} else {
			value := result.Value.(fleetclient.CloseResult)
			fmt.Fprintf(stdout, "interruption: %s; terminal: %s; work shared elsewhere or running remotely may continue.\n", value.Interrupt, value.Terminal)
		}
	case "group":
		fmt.Fprintln(stdout, "target: "+targetText(command.request, command.machines))
		text := "group assigned\n"
		if command.request.Group.IsUnassigned() {
			text = "group cleared\n"
		}
		if _, err := io.WriteString(stdout, text); err != nil {
			return 1
		}
	case "wait":
		value := result.Value.(fleetclient.WaitResult)
		request := fleetclient.Request{Ref: value.Target}
		fmt.Fprintf(stdout, "target: %s\nwait: %s", targetText(request, command.machines), value.Outcome)
		switch value.Outcome {
		case "matched":
			if value.TerminalStatus != nil {
				// A terminal match proves exactly the requested state's dimensions.
				fmt.Fprintf(stdout, "; observed %s (inferred); task completion unconfirmed.\n", command.request.State)
			} else {
				fmt.Fprintf(stdout, "; state: %s; source: %s; task completion unconfirmed.\n", fleetclient.StatusText(value.Observation.Status), value.Observation.Status.Source)
			}
		case "timeout":
			fmt.Fprintln(stdout, "; observation ended; no stop was sent.")
		case "target_changed":
			fmt.Fprintln(stdout, "; target changed; observation ended.")
		default:
			fmt.Fprintln(stdout)
		}
		if status := value.TerminalStatus; status != nil {
			fmt.Fprintf(stdout, "activity: %s; interaction: %s; notice: %s; source: %s\n", status.Activity, status.Interaction, status.Notice, status.Source)
		} else if value.Outcome != "matched" && value.Observation != nil {
			fmt.Fprintf(stdout, "last state: %s; source: %s\n", fleetclient.StatusText(value.Observation.Status), value.Observation.Status.Source)
		}
	case "send":
		fmt.Fprintln(stdout, "target: "+targetText(command.request, command.machines))
		if value, native := result.Value.(fleetclient.SendResult); native {
			fmt.Fprintf(stdout, "native %s %s: %s; task completion unconfirmed.\n", value.Input, value.Delivery, value.Outcome)
		} else {
			value := result.Value.(fleetclient.WriteResult)
			fmt.Fprintf(stdout, "terminal input: %s; provider acceptance unconfirmed.\n", value.Outcome)
			if value.Outcome == "unknown" {
				fmt.Fprintln(stderr, "inspect the terminal before further input; never replay an uncertain write")
			}
		}
	default:
		value := result.Value.(fleetclient.WriteResult)
		fmt.Fprintln(stdout, "target: "+targetText(command.request, command.machines))
		fmt.Fprintf(stdout, "%s %s: %s; %s\n", value.Method, command.request.Operation, value.Outcome, fleetclient.WriteText(command.request.Operation, value))
	}

	if _, err := output.Write(human.Bytes()); err != nil {
		return 1
	}
	return result.ExitCode(command.request.Operation)
}

func targetText(request fleetclient.Request, machines []fleetclient.Machine) string {
	handle, label := request.Handle, request.Machine
	if request.Ref != "" {
		ref, err := fleetclient.DecodeReference(request.Ref)
		if err == nil {
			handle, label = ref.Handle(), ""
			for _, machine := range machines {
				if machine.Handle == ref.Machine {
					label = machine.Label
					break
				}
			}
		}
	}
	for _, machine := range machines {
		if strings.EqualFold(machine.Label, label) {
			label = machine.Label
			break
		}
	}
	if handle == "" && request.ConversationID != "" {
		handle = "native conversation"
	}
	if label != "" {
		if handle == "" {
			return "on " + label
		}
		return handle + " on " + label
	}
	return handle
}

func machineArgument(label string) string {
	return "'" + strings.ReplaceAll(label, "'", "'\\''") + "'"
}

func reportFailure(writer io.Writer, failure fleetclient.Failure, request fleetclient.Request, machines []fleetclient.Machine) {
	if failure.Target != "" {
		request.Ref, request.Handle, request.Machine = failure.Target, "", ""
	}
	native := request.ConversationID != "" || strings.HasPrefix(request.Handle, "c-")
	if request.Ref != "" {
		ref, _ := fleetclient.DecodeReference(request.Ref)
		native = native || ref.Conversation != nil
	}
	target := strings.TrimSpace(request.Operation + " " + targetText(request, machines))
	fmt.Fprintf(writer, "%s: %s (%s)\n", target, failure.Code, failure.Dispatch)
	message := fleetclient.ErrorMessage(failure, request, native)
	if message != failure.Code+" ("+failure.Dispatch+")" {
		fmt.Fprintln(writer, message)
	}
	switch failure.Code {
	case "handle_ambiguous":
		fmt.Fprintln(writer, "use --machine HOST to choose the host")
	case "inventory_incomplete":
		fmt.Fprintln(writer, "a peer is unavailable; use --machine HOST to limit discovery")
	case "configuration_invalid":
		fmt.Fprintln(writer, "install the private client configuration or use --config PATH")
	}
	if failure.Dispatch == "unknown" {
		fmt.Fprintln(writer, "inspect before any further write; never replay an uncertain write")
	}
}
