package tmux

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	socketNamePattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	tmuxCommandTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

const (
	AutoNameOption         = "@skid_auto_name_b64"
	ServerEpochOption      = "@skid_server_epoch"
	identityMismatchMarker = "SKIDBLADNIR_IDENTITY_MISMATCH_V1"
	// GroupOption retains the persisted tmux key so existing memberships remain visible.
	GroupOption = "@skid_space_b64"
)

var (
	serverEpochPattern = regexp.MustCompile(`^v1-[0-9a-f]{32}$`)
	positivePattern    = regexp.MustCompile(`^[1-9][0-9]*$`)
)

type ServerIdentity struct {
	Epoch     string `json:"epoch"` // "" until skid stamps the server
	PID       string `json:"pid"`
	StartTime string `json:"startTime"`
}

// ParseServerIdentity is the one identity grammar for tmux output, client tokens
// and checkpoints: epoch "" or v1-<32 lowercase hex>, canonical positive pid and
// start time. Anything else is WorkspaceProtocolInvalid.
func ParseServerIdentity(epoch, pid, start string) (ServerIdentity, error) {
	if epoch != "" && !serverEpochPattern.MatchString(epoch) || !positivePattern.MatchString(pid) || !positivePattern.MatchString(start) {
		return ServerIdentity{}, WorkspaceProtocolInvalid
	}
	return ServerIdentity{Epoch: epoch, PID: pid, StartTime: start}, nil
}

// ValidID reports whether value is prefix ('$' session, '@' window, '%' pane)
// followed by one or more ASCII digits.
func ValidID(value string, prefix byte) bool {
	return len(value) > 1 && value[0] == prefix && strings.TrimLeft(value[1:], "0123456789") == ""
}

type Client struct {
	path       string
	socketName string
	shellPath  string
}

func New(path, socketName string) (Client, error) {
	if !filepath.IsAbs(path) {
		return Client{}, errors.New("tmux path must be absolute")
	}
	if socketName != "" && !socketNamePattern.MatchString(socketName) {
		return Client{}, errors.New("tmux socket name is invalid")
	}
	return Client{path: path, socketName: socketName}, nil
}

// WithShell pins the resolved destination shell for new and reconstructed panes.
func (client Client) WithShell(path string) Client {
	client.shellPath = path
	return client
}

func (client Client) Output(ctx context.Context, operation, commandName string, args ...string) (string, error) {
	var stdout bytes.Buffer
	err := client.command(ctx, &stdout, commandName, args...).Run()
	output := strings.TrimSuffix(stdout.String(), "\n")
	if err != nil {
		return output, fmt.Errorf("tmux %s failed: %w", operation, err)
	}
	return output, nil
}

// TerminalCommand crosses tmux's argv boundary without exposing launch data to
// its format parser or to a shell interpreter.
func (client Client) TerminalCommand(directory, launch string) ([]string, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return []string{"--", executable, "terminal-exec",
		base64.RawURLEncoding.EncodeToString([]byte(directory)),
		launch, base64.RawURLEncoding.EncodeToString([]byte(client.shellPath))}, nil
}

// CreateSession preserves the agent argv path. Terminal launches supply the
// subprocess cwd; a source supplies the existing lifetime gate in the same
// tmux command queue. Any command error may follow creation.
func (client Client) CreateSession(ctx context.Context, directory, sourceID string, server ServerIdentity, args []string) (string, bool, error) {
	commandName := "new-session"
	if sourceID != "" {
		var branch strings.Builder
		branch.WriteString("new-session")
		for _, argument := range args {
			branch.WriteByte(' ')
			if argument == ";" {
				branch.WriteByte(';')
			} else {
				branch.WriteString(commandToken(argument))
			}
		}
		commandName = "-N" // A vanished source must never start a replacement server.
		args = []string{"if-shell", "-F", "-t", sourceID, andFormatConditions(sessionLifetimeConditions(sourceID, server)),
			branch.String(), "display-message -p -l '" + identityMismatchMarker + "'"}
	} else {
		// cmd_parse_from_arguments consumes a trailing semicolon even in direct
		// argv. Escape that delimiter once; all other bytes remain literal.
		for index, argument := range args {
			if len(argument) > 1 && strings.HasSuffix(argument, ";") {
				args[index] = argument[:len(argument)-1] + "\\;"
			}
		}
	}
	var stdout bytes.Buffer
	command := client.command(ctx, &stdout, commandName, args...)
	command.Dir = directory
	if err := command.Start(); err != nil {
		return "", false, err
	}
	err := command.Wait()
	output := strings.TrimSuffix(stdout.String(), "\n")
	if err != nil {
		return output, true, fmt.Errorf("tmux create session failed: %w", err)
	}
	if output == identityMismatchMarker {
		return "", false, nil
	}
	return output, true, nil
}

func (client Client) Run(ctx context.Context, operation, commandName string, args ...string) error {
	if err := client.command(ctx, nil, commandName, args...).Run(); err != nil {
		return fmt.Errorf("tmux %s failed: %w", operation, err)
	}
	return nil
}

func (client Client) ListSessionIDs(ctx context.Context) ([]string, error) {
	var stdout, stderr bytes.Buffer
	command := client.commandWithStderr(ctx, &stdout, &stderr, "list-sessions", "-F", "#{session_id}")
	if err := command.Run(); err != nil {
		if missingServer(err, stderr.String()) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("tmux list sessions failed: %w", err)
	}
	output := strings.TrimSuffix(stdout.String(), "\n")
	if output == "" {
		return []string{}, nil
	}
	return strings.Split(output, "\n"), nil
}

// EnsureServerIdentity stamps proposed on the server expected names when that
// server has no epoch yet, and returns its stamped identity. expected is a
// parsed identity and proposed a minted epoch. Errors: WorkspaceSourceChanged
// (another lifetime, or another epoch), WorkspaceProtocolInvalid, ctx and child
// errors; never WorkspaceLimit.
func (client Client) EnsureServerIdentity(ctx context.Context, expected ServerIdentity, proposed string) (ServerIdentity, error) {
	branch := commandText("display-message", "-p", "#{"+ServerEpochOption+"}|#{pid}|#{start_time}")
	if expected.Epoch == "" {
		branch = commandText("set-option", "-soq", ServerEpochOption, proposed) + " ; " + branch
	}
	output, err := client.Output(ctx, "initialize-server-epoch", "-N", "if-shell", "-F", andFormatConditions(serverLifetimeConditions(expected)),
		branch, "display-message -p -l '"+identityMismatchMarker+"'")
	if err != nil {
		return ServerIdentity{}, err
	}
	if output == identityMismatchMarker {
		return ServerIdentity{}, WorkspaceSourceChanged
	}
	fields := strings.Split(output, "|")
	if len(fields) != 3 {
		return ServerIdentity{}, WorkspaceProtocolInvalid
	}
	identity, err := ParseServerIdentity(fields[0], fields[1], fields[2])
	if err != nil || identity.Epoch == "" {
		return ServerIdentity{}, WorkspaceProtocolInvalid
	}
	if identity.PID != expected.PID || identity.StartTime != expected.StartTime || expected.Epoch != "" && identity.Epoch != expected.Epoch {
		return ServerIdentity{}, WorkspaceSourceChanged
	}
	return identity, nil
}

func formatLiteral(value string) string {
	return "#{l:" + strings.NewReplacer("#", "##", ",", "#,", "}", "#}").Replace(value) + "}"
}

func commandText(command string, arguments ...string) string {
	var result strings.Builder
	result.WriteString(command)
	for _, argument := range arguments {
		result.WriteByte(' ')
		result.WriteString(commandToken(argument))
	}
	return result.String()
}

func commandToken(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func sessionLifetimeConditions(id string, server ServerIdentity) []string {
	return append(serverLifetimeConditions(server), sessionIDCondition(id))
}

// sessionIDCondition holds under if-shell -F -t id only while session id
// exists: -t naming an absent id evaluates the format without a target, so
// #{session_id} is not id and the condition is false.
func sessionIDCondition(id string) string {
	return "#{==:#{session_id}," + formatLiteral(id) + "}"
}

func serverLifetimeConditions(server ServerIdentity) []string {
	return []string{
		"#{==:#{" + ServerEpochOption + "}," + formatLiteral(server.Epoch) + "}",
		"#{==:#{pid}," + formatLiteral(server.PID) + "}",
		"#{==:#{start_time}," + formatLiteral(server.StartTime) + "}",
	}
}

func andFormatConditions(conditions []string) string {
	condition := conditions[len(conditions)-1]
	for index := len(conditions) - 2; index >= 0; index-- {
		condition = "#{&&:" + conditions[index] + "," + condition + "}"
	}
	return condition
}

func (client Client) command(ctx context.Context, stdout *bytes.Buffer, operation string, args ...string) *exec.Cmd {
	return client.commandWithStderr(ctx, stdout, &bytes.Buffer{}, operation, args...)
}

func (client Client) commandWithStderr(ctx context.Context, stdout, stderr *bytes.Buffer, operation string, args ...string) *exec.Cmd {
	commandArgs := make([]string, 0, len(args)+6)
	// Tmux otherwise sanitizes stdout under a non-UTF-8 client locale. Names,
	// titles and literal comparisons require the exact UTF-8 server values.
	// Attachment already supplies the same global flag as its first argument.
	if operation != "-u" {
		commandArgs = append(commandArgs, "-u")
	}
	if client.socketName != "" {
		commandArgs = append(commandArgs, "-L", client.socketName, "-f", "/dev/null")
	}
	commandArgs = append(commandArgs, operation)
	commandArgs = append(commandArgs, args...)
	command := exec.CommandContext(ctx, client.path, commandArgs...)
	// A tmux client passes its stdin and stdout to the server while
	// identifying, and a stalled server keeps a killed client's descriptors
	// queued, so Wait would block until the server resumed. Outside control
	// mode (unused here) the server closes stdout and a non-tty stdin at
	// identify, before the command runs, so a normal exit reaches EOF at once.
	command.WaitDelay = 100 * time.Millisecond
	command.Env = tmuxEnvironment()
	if stdout != nil {
		command.Stdout = stdout
	}
	if stderr != nil {
		command.Stderr = stderr
	}
	return command
}

func tmuxEnvironment() []string {
	return filterTmuxEnvironment(os.Environ())
}

func filterTmuxEnvironment(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if strings.HasPrefix(entry, "TMUX=") || strings.HasPrefix(entry, "TMUX_PANE=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
