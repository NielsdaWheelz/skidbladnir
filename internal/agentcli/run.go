// Package agentcli parses ordinary commands and renders the shared fleet results.
package agentcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessionui"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"golang.org/x/term"
)

const usage = `usage: skid [--config PATH] COMMAND [options]

skid                                      open the session browser
skid list [--machine HOST]                 list the fleet
skid info NAME                            show metadata and exact reference
skid enter NAME                           enter terminal; ctrl-] d detaches
skid read NAME [--terminal] [--max-bytes N] read bounded output
skid send NAME TEXT [--terminal]           submit text once
skid send NAME --stdin [--terminal]        submit literal stdin, up to 32 kib
skid keys NAME KEY...                     send 1–16 logical keys
skid interrupt NAME                       cancel current work, keep session
skid stop NAME                            attempt agent halt, close session
skid kill NAME                            close session without requesting agent halt
skid start NAME --machine HOST --profile PROFILE [--cwd '~']

existing targets: use NAME [--machine HOST] or --ref VALUE
--json: one structured envelope for noninteractive commands
--: remaining operands are literal; --help: this guide
keys: enter escape ctrl-c up down left right tab backspace page-up page-down
config defaults to ~/.config/skidbladnir/client.json
shared window/pane navigation and latest-client sizing are intentional.
stop may halt shared work; kill removes one session and shared work may survive.
written means delivered; unknown never means safe to resend.
`

type command struct {
	request           fleetclient.Request
	config            string
	json, stdin, help bool
}

func parse(args []string) (command, error) {
	var result command
	var operands []string
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
			case "--json", "--stdin", "--terminal", "--help":
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
				}
			case "--config", "--machine", "--ref", "--profile", "--cwd", "--max-bytes":
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
		if result.json || result.stdin || result.request.Machine != "" || result.request.Ref != "" || result.request.Profile != "" || result.request.CWD != "" || result.request.Mode != "" || result.request.MaxBytes != 0 {
			return result, errors.New("missing command")
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
	case "info", "enter", "read", "send", "keys", "interrupt", "stop", "kill":
		if result.request.Ref == "" {
			if len(operands) == 0 {
				return result, errors.New("missing target")
			}
			result.request.Name = operands[0]
			operands = operands[1:]
		}
		switch result.request.Operation {
		case "send":
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
	if result.stdin && result.request.Operation != "send" || result.json && result.request.Operation == "enter" {
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
		if err := terminalclient.Run(ctx, client, parsed.request, stdin.(*os.File), stdout.(*os.File)); err != nil {
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
		fmt.Fprintf(stderr, "%s (%s)\n", result.Error.Code, result.Error.Dispatch)
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
		var list fleetclient.Inventory
		if json.Unmarshal(result.Value, &list) != nil {
			return 1
		}
		table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "machine\tsession\tprovider/profile\tstate · source\tdirectory")
		for _, peer := range list.Peers {
			if !peer.OK {
				fmt.Fprintf(table, "%s\tunavailable\t\t%s\t\n", peer.Label, peer.Error.Code)
				continue
			}
			for _, row := range peer.Sessions {
				provider, state := "shell", "—"
				if row.Agent != nil {
					provider = row.Agent.Provider
					if row.Agent.Profile != "" {
						provider += "/" + row.Agent.Profile
					}
					state = row.Agent.Status.State + " · " + row.Agent.Status.Source
					if row.Agent.Status.Reason != "" {
						state += " · " + row.Agent.Status.Reason
					}
				}
				fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", peer.Label, row.Name, provider, state, row.CWD)
			}
		}
		if table.Flush() != nil {
			return 1
		}
	case "info", "start":
		var value fleetclient.ObservedSession
		if json.Unmarshal(result.Value, &value) != nil {
			return 1
		}
		if command.request.Operation == "start" {
			if _, err := fmt.Fprintf(stdout, "created %s on %s; provider may still be starting\nenter with: skid enter %q --machine %q\n", value.Session.Name, value.Label, value.Session.Name, value.Label); err != nil {
				return 1
			}
		} else {
			row := value.Session
			fmt.Fprintf(stdout, "session: %s\nmachine: %s\nmachine id: %s\ndirectory: %s\ncommand: %s\nattached clients: %d\n", row.Name, value.Label, value.Machine, row.CWD, row.ActiveCommand, row.AttachedClients)
			if row.Agent == nil {
				fmt.Fprintln(stdout, "agent: none (shell)")
			} else {
				a := row.Agent
				fmt.Fprintf(stdout, "provider: %s\nprofile: %s\nstate: %s (%s)\nreason: %s\nread: %s; send: %s; interrupt: %s\n", a.Provider, a.Profile, a.Status.State, a.Status.Source, a.Status.Reason, a.Methods.Read, a.Methods.Send, a.Methods.Interrupt)
				if a.ProviderSession != nil {
					fmt.Fprintf(stdout, "provider session: %s %s\n", a.ProviderSession.ID, a.ProviderSession.Name)
				}
			}
			if row.LaunchProfile != "" {
				fmt.Fprintf(stdout, "launch profile: %s\n", row.LaunchProfile)
			}
			if _, err := fmt.Fprintf(stdout, "observed: %s\nreference: %s\n", value.ObservedAt, row.Ref); err != nil {
				return 1
			}
		}
	case "read":
		var value fleetclient.ReadResult
		if json.Unmarshal(result.Value, &value) != nil {
			return 1
		}
		fmt.Fprintf(stderr, "source: %s; scope: %s; truncated: %t\n", value.Source, value.Scope, value.Truncated)
		if _, err := io.WriteString(stdout, value.Text); err != nil {
			return 1
		}
	case "stop":
		var value fleetclient.StopResult
		if json.Unmarshal(result.Value, &value) != nil {
			return 1
		}
		if _, err := fmt.Fprintf(stdout, "agent halt: %s; terminal: %s\n", value.Agent, value.Terminal); err != nil {
			return 1
		}
	case "kill":
		if _, err := io.WriteString(stdout, "terminal: closed; shared work may continue\n"); err != nil {
			return 1
		}
	default:
		var value fleetclient.WriteResult
		if json.Unmarshal(result.Value, &value) != nil {
			return 1
		}
		if _, err := fmt.Fprintf(stdout, "%s: %s\n", value.Method, value.Outcome); err != nil {
			return 1
		}
	}
	return result.ExitCode(command.request.Operation)
}
