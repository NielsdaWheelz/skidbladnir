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
	"slices"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessionui"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"golang.org/x/term"
)

// usage opens skid --help: a section for people, then the automation guide,
// which Help ends with every command's own help.
const usage = `skid: every tmux session on every machine you own, on your desktop and your
android phone.

usage: skid [COMMAND] [options]

with no command, skid opens the session browser.

sessions
  list             sessions on every machine
  enter            attach this terminal to a session; ctrl-] then d detaches
  start            start an agent profile or a terminal on a machine
  shell            open a new terminal beside a session
  info             one terminal's facts and status
  group            set or clear a session's group
  close            interrupt and close a terminal
  read             a terminal's rendered text
  send, text       paste text into a terminal and press enter
  keys             press keys such as enter, escape or ctrl-c
  wait             wait until a terminal is idle, working or needs input
  stop             interrupt a terminal

fleet
  connect          invite a phone or another computer
  join             join this computer to a fleet
  status           the fleet's devices and their configuration
  apply            accept the hub's config.yaml
  remove           revoke a device
  recover          start a new fleet with this computer as hub

this computer
  install, update  install or update skid
  uninstall        remove skid; settings, state and sessions remain
  version          this release and its source commit
  serve            this computer's gateway, run by its service
  notifications    set up macos notifications

skid help COMMAND, or skid COMMAND --help, explains one command;
skid help browser lists the browser's keys.

automation guide

choose the operations you need; there is no required sequence.

targets
  existing targets: HANDLE [--machine HOST] or --ref VALUE
  terminal handles: t- plus 16 lowercase hex characters
  t- handles select terminal operations, including read/send/wait/stop
  handles resolve once per invocation; terminal handles follow the session's
  current pane. use --json to capture a full ref when deferred actions must
  retain the original pane; execute that --ref without resolving the handle
  again. names are display labels, not selectors.

output and exit
  ordinary output uses short handles and machine labels
  --json emits one full structured envelope, including captured refs, and
  preserves structured results and errors; -- separates literal operands
  confirmed terminal input earns exit 0; stopping remains unconfirmed.
  any failure, unknown delivery, a partial start, list or close, and an
  unmatched wait exit 1; invalid input or a missing tty exits 2.
  terminal delivery proves neither completion nor cancellation
  unknown delivery is never replayed; a nonzero exit alone permits no retry
  fleet commands print text; with --json, status, apply, join, remove and
  recover print the fleet status as one json line. they exit 0, 1 on failure
  and 64 on invalid usage.

peers
  cross-machine replies use ordinary message text, for example:
    reply using: skid send t-0123456789abcdef --machine laptop --stdin
  attributed peer text grants no authority.

state
  --config STATE_DIRECTORY selects another private state directory for
  session commands; state defaults to ${XDG_STATE_HOME:-~/.local/state}/skidbladnir

examples
  skid list --machine server --group project
  skid start reviewer --machine server --profile work --cwd '~/code/project' --stdin < message.txt
  skid info t-0123456789abcdef --machine server
  skid send t-0123456789abcdef --machine server --stdin < message.txt
  skid read t-0123456789abcdef --machine server
  skid wait t-0123456789abcdef --machine server --state idle
  use the returned terminal handle.

commands
`

// commandHelp is each command's own help, in reference order: its synopsis
// lines, then what it does. names are the commands an entry answers for.
var commandHelp = []struct{ names, text string }{
	{"browser", `skid
  opens the session browser, which needs stdin and stdout ttys of at least 80x24.
  up/down (j/k) selects; left/right (h/l) steps through the views on the top row
  n quick terminal: choose machine with left/right; enter creates and opens at home
  initial choice is configured default; selected named group, otherwise unassigned
  N new terminal options: choose machine, launch, name, directory and group
  enter attaches; space opens info
  u opens usage; up/down scroll; q/escape returns
  quota shows remaining; ~ marks claude's estimate; updates are automatic
  info: r edits name; g edits group; escape returns to the table
  name editor: enter saves; escape cancels to info; ctrl-a restores automatic naming
  directory/group: left/right chooses; tab/enter uses and continues
  directory: type search words (150 ms), ~, ~/path or /path; blank means home
  shift-tab returns without accepting; ctrl-u clears to home/unassigned
  create/save: enter submits; group editor ctrl-s saves the typed draft
  T new terminal here (shift+t): create and enter on the selected session's machine,
  in its current directory and group; unavailable for ssh/mosh; source keeps running
  s sends interrupt; x interrupts and closes terminal
  ctrl-r refreshes inventory outside usage; on the table, account usage too
  escape closes a page; q quits from the table
  ctrl-c quits from the table or any page unless an operation is in flight
`},
	{"list", `skid list [--machine HOST] [--group LABEL | --unassigned]
  sessions on every machine, or only on HOST: machine, session, terminal
  handle, provider/profile, state, group and directory. --group and
  --unassigned filter what was found. an unavailable machine is listed as
  unavailable and the partial list exits 1.
`},
	{"info", `skid info HANDLE [--explain]
  terminal metadata and status: session, group, directory, provider, profile
  and state. --explain samples the terminal once more and prints that sample's
  status evidence.
`},
	{"enter", `skid enter HANDLE
  enters the terminal from this tty; ctrl-] then d detaches. navigation and
  terminal size are shared with every other client of the session.
`},
	{"start", `skid start [NAME] --machine HOST --profile PROFILE [--cwd '~'] [--group LABEL] [--stdin]
skid start [NAME] --machine HOST --terminal [--cwd '~'] [--group LABEL]
  creates one session on HOST running a configured profile, or a plain
  terminal, in a directory on HOST (home by default; quote ~ so this shell
  leaves it). skid runs each configured command once after normal shell
  startup. aliases, functions, environment and provider settings remain
  effective.
  start --stdin sends one literal initial prompt after bounded idle/composer
  checks; start without --stdin makes no input-readiness promise.
  start reports creation and prompt delivery separately; written never proves
  provider acceptance.
  trust/setup dialogs require deliberate inspection and text/keys; start never
  answers them.
`},
	{"shell", `skid shell HANDLE
  new terminal here: creates a terminal on the session's machine, in its
  current directory and group; refused for an ssh or mosh foreground.
`},
	{"group", `skid group HANDLE (--set LABEL | --clear)
  sets or clears the session's group.
`},
	{"close", `skid close HANDLE [--terminal-only]
  interrupts, then closes the terminal, reporting each outcome separately;
  --terminal-only closes without interrupting. closure may leave shared or
  remote work running.
`},
	{"read", `skid read HANDLE [--max-bytes N]
  bounded rendered terminal text, at most N bytes (1-32768, default 16384).
  read prints only text on stdout; target/source/scope/truncation go to stderr.
`},
	{"send", `skid send HANDLE (TEXT | --stdin)
  guarded terminal paste: pastes TEXT, or stdin, and presses enter only when
  the terminal shows a fresh local agent, working or idle, with no interaction
  and an empty composer; anything else refuses before writing.
`},
	{"text", `skid text HANDLE (TEXT | --stdin)
  explicit terminal paste and submit: pastes TEXT, or stdin, and presses enter
  without send's checks.
`},
	{"keys", `skid keys HANDLE KEY...
  explicit logical terminal keys, 1 to 16 of: enter escape ctrl-c up down left
  right tab backspace page-up page-down.
`},
	{"wait", `skid wait HANDLE [--state STATE] [--timeout DURATION]
  terminal wait states: idle (no request or menu), working, needs-input.
  wait defaults to idle/60s; maximum one hour; idle proves neither completion
  nor an empty queue.
`},
	{"stop", `skid stop HANDLE
  interrupts the selected terminal once: escape to a local codex, ctrl-c
  otherwise.
`},
	{"connect", `skid connect
  points tailscale serve https:8443 at this computer's gateway, replacing
  whatever 8443 served, and prints a five-minute invitation, with a qr code on
  a tty. scan it with skid on a phone, or paste it into skid join on another
  computer. connect first checks that tailscale is signed in with magicdns and
  https certificates on and, on linux, that this user is tailscale's operator,
  and says how to fix what is missing rather than changing tailnet or host
  policy.
`},
	{"join", `skid join [--name NAME] [--json]
  reads an invitation from skid connect on stdin and joins this computer to
  that fleet, restarting its service; a failure changes nothing. a computer
  whose name the fleet already has needs --name.
`},
	{"status", `skid status [--json]
  each device in the fleet; for a computer its name, release, origin and
  configuration: applied, invalid_host or pending_offline. a member shows the
  other computers pending_offline, since only the hub holds reports.
`},
	{"apply", `skid apply [--json]
  the hub accepts its config.yaml; prints the fleet's status.
`},
	{"remove", `skid remove DEVICE_ID [--json]
  revokes a device at the hub; skid status lists device ids.
`},
	{"recover", `skid recover [--json]
  explicitly creates a new fleet with this computer as hub and restarts its
  service. every other computer then joins again, and each phone re-pairs after
  clearing skid's app data.
`},
	{"install update uninstall", `skid install | update [--version vX.Y.Z] [--archive PATH --signature PATH]
skid uninstall
  own the local installation. uninstall keeps settings, private state and tmux
  sessions.
`},
	{"version", `skid version
skid --version
  this release's version and source commit.
`},
	{"serve", `skid serve
  runs this computer's gateway on 127.0.0.1:7341; its installed service runs it.
`},
	{"notifications", `skid notifications setup
  opens macos notification setup.
`},
}

// Help is skid's help, or one command's when command is not empty; ok is false
// for an unknown command.
func Help(command string) (text string, ok bool) {
	if command == "" {
		text = usage
		for _, entry := range commandHelp {
			text += "\n" + entry.text
		}
		return text, true
	}
	for _, entry := range commandHelp {
		if slices.Contains(strings.Fields(entry.names), command) {
			return "usage: " + strings.ReplaceAll(entry.text, "\nskid ", "\n       skid "), true
		}
	}
	return "", false
}

type command struct {
	request           fleetclient.Request
	config            string
	json, stdin, help bool
	machines          []fleetclient.Machine
}

func parse(args []string) (command, error) {
	var result command
	var operands []string
	var groupArgument, setArgument, refArgument string
	seen := map[string]bool{}
	literal := false
	for i := 0; i < len(args); i++ {
		value := args[i]
		if value == "--" && !literal {
			literal = true
			continue
		}
		if value == "-h" && !literal {
			value = "--help"
		}
		if !literal && strings.HasPrefix(value, "--") {
			name, argument, hasValue := strings.Cut(value, "=")
			if seen[name] {
				return result, errors.New("duplicate option")
			}
			seen[name] = true
			switch name {
			case "--json", "--stdin", "--terminal", "--help", "--unassigned", "--clear", "--terminal-only", "--explain":
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
				case "--terminal-only":
					result.request.TerminalOnly = true
				case "--explain":
					result.request.Explain = true
				}
			case "--config", "--machine", "--ref", "--profile", "--cwd", "--max-bytes", "--group", "--set", "--state", "--timeout":
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
					refArgument = argument
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
	// skid help [COMMAND] and skid [COMMAND ...] --help name one help text.
	if len(operands) > 0 && operands[0] == "help" {
		if len(operands) > 2 {
			return result, errors.New("help takes one command")
		}
		result.help, operands = true, operands[1:]
	}
	if result.help {
		if len(operands) > 0 {
			if _, ok := Help(operands[0]); !ok {
				return result, errors.New("unknown command")
			}
			result.request.Operation = operands[0]
		}
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
	case "info", "enter", "read", "send", "keys", "text", "wait", "stop", "close", "group", "shell":
		if !seen["--ref"] {
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
	if seen["--terminal-only"] && operation != "close" || seen["--explain"] && operation != "info" || (seen["--state"] || seen["--timeout"]) && operation != "wait" {
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
			if result.stdin {
				return result, errors.New("terminal start rejects initial prompt")
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
	// Combinations the option loop admits; parse is the only request validator.
	invalid := errors.New("invalid command arguments")
	request := &result.request
	if seen["--ref"] {
		ref, err := fleetclient.DecodeReference(refArgument)
		if err != nil || operation == "list" || operation == "start" || seen["--machine"] {
			return result, invalid
		}
		request.Ref = ref
	} else if operation != "list" && operation != "start" {
		// t- plus 16 lowercase hex; an empty operand is no handle.
		if handle := request.Handle; len(handle) != 18 || !strings.HasPrefix(handle, "t-") || strings.Trim(handle[2:], "0123456789abcdef") != "" {
			return result, invalid
		}
	}
	if seen["--max-bytes"] && operation != "read" || (seen["--profile"] || seen["--cwd"]) && operation != "start" {
		return result, invalid
	}
	switch operation {
	case "start":
		if request.Machine == "" || (request.Kind == fleetclient.LaunchAgent) != (request.Profile != "") {
			return result, invalid
		}
	case "send", "text":
		if !result.stdin && !validText(request.Text) {
			return result, invalid
		}
	case "keys":
		if len(request.Keys) < 1 || len(request.Keys) > 16 {
			return result, invalid
		}
		for _, key := range request.Keys {
			switch key {
			case "enter", "escape", "ctrl-c", "up", "down", "left", "right", "tab", "backspace", "page-up", "page-down":
			default:
				return result, invalid
			}
		}
	case "wait":
		if request.State != "idle" && request.State != "working" && request.State != "needs-input" {
			return result, invalid
		}
	}
	return result, nil
}

// validText bounds terminal text from argv or stdin: 1-32768 bytes of utf-8 without NUL.
func validText(text string) bool {
	return text != "" && len(text) <= 32768 && utf8.ValidString(text) && !strings.ContainsRune(text, 0)
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
		text, _ := Help(parsed.request.Operation)
		if _, err := io.WriteString(stdout, text); err != nil {
			return 1
		}
		return 0
	}
	if parsed.request.Operation == "" || parsed.request.Operation == "enter" {
		input, inOK := stdin.(*os.File)
		output, outOK := stdout.(*os.File)
		if !inOK || !outOK || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
			io.WriteString(stderr, "skid browser and enter require stdin and stdout ttys; use skid list\nuse skid --help for commands and examples\n")
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
		if !validText(parsed.request.Text) {
			return render(parsed, fleetclient.Failed("invalid_input", "not_sent"), stdout, stderr)
		}
	}
	if parsed.config == "" {
		parsed.config, err = attention.StateDirectory()
		if err != nil {
			return render(parsed, fleetclient.Failed("configuration_invalid", "not_sent"), stdout, stderr)
		}
	}
	client, err := fleetclient.Open(parsed.config)
	if err != nil {
		return render(parsed, fleetclient.Failed("configuration_invalid", "not_sent"), stdout, stderr)
	}
	parsed.machines = client.Machines()
	if parsed.request.Operation == "" {
		applyCtx, cancelApply := context.WithTimeout(ctx, 10*time.Second)
		applied := make(chan struct{})
		go func() {
			defer close(applied)
			_ = client.Apply(applyCtx) // justify-ignore-error: cached direct access stays available; skid status reports pending configuration.
		}()
		defer func() { cancelApply(); <-applied }()
		if sessionui.Run(ctx, client, stdin.(*os.File), stdout.(*os.File)) != nil {
			io.WriteString(stderr, "session browser ended with an error\n")
			return 1
		}
		return 0
	}
	if parsed.request.Operation == "enter" {
		io.WriteString(stderr, "ctrl-] then d detaches; navigation and terminal size are shared\n")
		ref, failure := client.ResolveTerminal(ctx, parsed.request)
		if failure != nil {
			return render(parsed, fleetclient.Result{Error: failure}, stdout, stderr)
		}
		result := terminalclient.Run(ctx, client, ref, stdin.(*os.File), stdout.(*os.File))
		if result.NotificationErr != nil {
			fmt.Fprintln(stderr, "notifications unavailable")
		}
		if result.Err != nil {
			fmt.Fprintln(stderr, result.Err)
			return 1
		}
		return 0
	}
	return render(parsed, client.Execute(ctx, parsed.request), stdout, stderr)
}

func render(command command, result fleetclient.Result, stdout, stderr io.Writer) int {
	operation := command.request.Operation
	encoded, err := result.Encode(operation)
	if err != nil && operation != "list" { // a result over 64 KiB: report that in either mode
		dispatch := "not_sent"
		switch operation {
		case "send", "text", "keys", "stop", "close", "shell", "group":
			dispatch = "unknown"
		}
		result = fleetclient.Failed("output_limit", dispatch)
		encoded, err = result.Encode(operation)
	}
	if command.json {
		// An enriched list over 1 MiB prints nothing: docs/issues/list-json-output-limit.md.
		if err != nil {
			return 1
		}
		if _, err := stdout.Write(encoded); err != nil {
			return 1
		}
		return result.ExitCode(operation)
	}
	if !result.OK {
		reportFailure(stderr, *result.Error, command.request, command.machines)
		return 1
	}
	var human bytes.Buffer
	output := stdout
	stdout = &human
	switch operation {
	case "list":
		list := result.Value.(fleetclient.Inventory)
		table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "machine\tsession\tterminal\tprovider/profile\tstate\tgroup\tdirectory")
		owners := make(map[string]fleetclient.Peer, len(list.Peers))
		for _, peer := range list.Peers {
			owners[peer.Machine] = peer
			if !peer.OK {
				fmt.Fprintf(table, "%s\t\t\t\tunavailable: %s\t\t\n", peer.Label, peer.Error.Code)
			} else if len(peer.Sessions) == 0 {
				fmt.Fprintf(table, "%s\tno matching sessions\t\t\t\t\t\n", peer.Label)
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
				fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", machine, row.Name, row.TerminalHandle, provider, fleetview.ProjectStatus(row, entry.Available, false).Detail, fleetview.GroupHeading(group.Label), current.CWD)
			}
		}
		if list.Partial {
			fmt.Fprintln(table, "partial inventory; unavailable hosts are listed above")
		} else if len(list.Peers) == 0 {
			fmt.Fprintln(table, "no machines in this view")
		}
		table.Flush()
		for _, peer := range list.Peers {
			if text, _ := fleetview.RecoveryNotice(peer); text != "" {
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
				fmt.Fprintln(stderr, fleetview.ErrorMessage(*value.Failure, command.request))
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
		if operation != "info" {
			fmt.Fprintf(stdout, "created %s\nenter with: skid enter %s --machine %s\n", value.Session.Name, value.Session.TerminalHandle, machineArgument(value.Label))
		} else {
			row := value.Session
			current := row.Current(fleetclient.Peer{Label: value.Label, Machine: value.Machine})
			fmt.Fprintf(stdout, "session: %s\ngroup: %s\n", row.Name, fleetview.GroupHeading(row.Group))
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
			view := fleetview.ProjectStatus(row, true, false)
			fmt.Fprintln(stdout, "state: "+view.Detail)
			if value.Diagnostics == nil {
				fmt.Fprintln(stdout, "status reason: "+view.Reason)
			}
			if row.LaunchProfile != "" {
				fmt.Fprintf(stdout, "started with: %s\n", row.LaunchProfile)
			}
			fmt.Fprintf(stdout, "observed: %s\n", value.ObservedAt)
			if diagnostics := value.Diagnostics; diagnostics != nil {
				status, rules := row.TerminalStatus, "none"
				if len(diagnostics.Rules) > 0 {
					rules = strings.Join(diagnostics.Rules, ", ")
				}
				fmt.Fprintf(stdout, "status evidence\n  activity: %s\n  interaction: %s\n  notice: %s\n  reason: %s\n  rules: %s\n", status.Activity, status.Interaction, status.Notice, view.Reason, rules)
			}
		}
	case "read":
		value := result.Value.(api.ReadResult)
		fmt.Fprintf(stderr, "target: %s; source: %s; scope: %s; truncated: %t", targetText(command.request, command.machines), value.Source, value.Scope, value.Truncated)
		fmt.Fprintln(stderr)
		io.WriteString(stdout, value.Text)
	case "close":
		fmt.Fprintln(stdout, "target: "+targetText(command.request, command.machines))
		if value, ok := result.Value.(fleetclient.TerminalCloseResult); ok {
			fmt.Fprintln(stdout, "terminal "+value.Terminal+"; work shared elsewhere or running remotely may continue.")
		} else {
			value := result.Value.(api.CloseResult)
			fmt.Fprintf(stdout, "interruption: %s; terminal: %s; work shared elsewhere or running remotely may continue.\n", value.Interrupt, value.Terminal)
		}
	case "group":
		fmt.Fprintln(stdout, "target: "+targetText(command.request, command.machines))
		text := "group assigned\n"
		if command.request.Group.IsUnassigned() {
			text = "group cleared\n"
		}
		io.WriteString(stdout, text)
	case "wait":
		value := result.Value.(fleetclient.WaitResult)
		request := fleetclient.Request{Ref: value.Target.Reference}
		fmt.Fprintf(stdout, "target: %s\nwait: %s", targetText(request, command.machines), value.Outcome)
		switch value.Outcome {
		case "matched":
			fmt.Fprintf(stdout, "; observed %s (inferred); task completion unconfirmed.\n", command.request.State)
		case "timeout":
			fmt.Fprintln(stdout, "; observation ended; no stop was sent.")
		case "target_changed":
			fmt.Fprintln(stdout, "; target changed; observation ended.")
		default:
			fmt.Fprintln(stdout)
		}
		if status := value.TerminalStatus; status != nil {
			fmt.Fprintf(stdout, "activity: %s; interaction: %s; notice: %s; source: %s\n", status.Activity, status.Interaction, status.Notice, status.Source)
		}
	case "send":
		fmt.Fprintln(stdout, "target: "+targetText(command.request, command.machines))
		value := result.Value.(api.WriteResult)
		fmt.Fprintf(stdout, "terminal input: %s; provider acceptance unconfirmed.\n", value.Outcome)
		if value.Outcome == "unknown" {
			fmt.Fprintln(stderr, "inspect the terminal before further input; never replay an uncertain write")
		}
	default:
		value := result.Value.(api.WriteResult)
		fmt.Fprintln(stdout, "target: "+targetText(command.request, command.machines))
		fmt.Fprintf(stdout, "%s %s: %s; %s\n", value.Method, operation, value.Outcome, fleetview.WriteText(operation, value))
	}

	if _, err := output.Write(human.Bytes()); err != nil {
		return 1
	}
	return result.ExitCode(operation)
}

func targetText(request fleetclient.Request, machines []fleetclient.Machine) string {
	handle, label := request.Handle, request.Machine
	if !request.Ref.IsZero() {
		handle, label = request.Ref.Handle(), ""
		for _, machine := range machines {
			if machine.Handle == request.Ref.Machine {
				label = machine.Label
				break
			}
		}
	}
	for _, machine := range machines {
		if strings.EqualFold(machine.Label, label) {
			label = machine.Label
			break
		}
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
	if !failure.Target.IsZero() {
		request.Ref, request.Handle, request.Machine = failure.Target.Reference, "", ""
	}
	target := strings.TrimSpace(request.Operation + " " + targetText(request, machines))
	fmt.Fprintf(writer, "%s: %s (%s)\n", target, failure.Code, failure.Dispatch)
	message := fleetview.ErrorMessage(failure, request)
	if message != failure.Code+" ("+failure.Dispatch+")" {
		fmt.Fprintln(writer, message)
	}
	switch failure.Code {
	case "handle_ambiguous":
		fmt.Fprintln(writer, "use --machine HOST to choose the host")
	case "inventory_incomplete":
		fmt.Fprintln(writer, "a peer is unavailable; use --machine HOST to limit discovery")
	case "configuration_invalid":
		fmt.Fprintln(writer, "run skid install; use --config PATH only to select another private state directory")
	}
	if failure.Dispatch == "unknown" {
		fmt.Fprintln(writer, "inspect before any further write; never replay an uncertain write")
	}
}
