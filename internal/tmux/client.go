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

	"github.com/NielsdaWheelz/skidbladnir/internal/space"
)

var (
	socketNamePattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	tmuxCommandTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

const (
	ServerEpochOption      = "@skid_server_epoch"
	identityMismatchMarker = "SKIDBLADNIR_IDENTITY_MISMATCH_V1"
	renameSuccessMarker    = "SKIDBLADNIR_RENAME_SUCCESS_V1"
	SpaceOption            = "@skid_space_b64"
	spaceSuccessMarker     = "SKIDBLADNIR_SPACE_SUCCESS_V1"
)

var (
	serverEpochPattern = regexp.MustCompile(`^v1-[0-9a-f]{32}$`)
	sessionIDPattern   = regexp.MustCompile(`^\$[0-9]+$`)
	serverPIDPattern   = regexp.MustCompile(`^[1-9][0-9]*$`)
	startTimePattern   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

type ServerIdentity struct {
	Epoch     string
	PID       string
	StartTime string
}

type Client struct {
	path       string
	socketName string
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
func (client Client) TerminalCommand(directory string) ([]string, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return []string{"--", executable, "terminal-exec",
		base64.RawURLEncoding.EncodeToString([]byte(directory)),
		base64.RawURLEncoding.EncodeToString([]byte(client.path)), client.socketName}, nil
}

// CreateSession preserves the agent argv path. Terminal launches supply the
// subprocess cwd; a source supplies the existing lifetime gate in the same
// tmux command queue. Any command error may follow creation.
func (client Client) CreateSession(ctx context.Context, directory, sourceID string, server ServerIdentity, args []string) (string, bool, error) {
	commandName := "new-session"
	if sourceID != "" {
		if !sessionIDPattern.MatchString(sourceID) || !server.valid() {
			panic("invalid source creation identity") // justify-defect: sessions admitted this exact source lifetime.
		}
		var branch strings.Builder
		branch.WriteString("new-session")
		for _, argument := range args {
			branch.WriteByte(' ')
			if argument == ";" {
				branch.WriteByte(';')
			} else {
				branch.WriteString("'" + strings.ReplaceAll(argument, "'", "'\\''") + "'")
			}
		}
		commandName = "-N" // A vanished source must never start a replacement server.
		args = []string{"if-shell", "-F", "-t", sourceID, andFormatConditions(sessionLifetimeConditions(sourceID, server)),
			branch.String(), "display-message -p -l '" + identityMismatchMarker + "'"}
	} else if directory != "" {
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
		var exitError *exec.ExitError
		message := strings.TrimSpace(stderr.String())
		missingSocket := strings.HasPrefix(message, "error connecting to ") && strings.HasSuffix(message, "(No such file or directory)")
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 &&
			(strings.HasPrefix(message, "no server running on ") || missingSocket) {
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

func (client Client) HasSession(ctx context.Context, id string) (bool, error) {
	ids, err := client.ListSessionIDs(ctx)
	if err != nil {
		return false, err
	}
	for _, candidate := range ids {
		if candidate == id {
			return true, nil
		}
	}
	return false, nil
}

func (client Client) EnsureServerIdentity(ctx context.Context, proposed string) (ServerIdentity, error) {
	if !serverEpochPattern.MatchString(proposed) {
		return ServerIdentity{}, errors.New("proposed tmux server epoch is invalid")
	}
	epoch, err := client.readServerEpoch(ctx, true)
	if err != nil {
		return ServerIdentity{}, err
	}
	if epoch == "" {
		setErr := client.Run(ctx, "initialize-server-epoch", "set-option", "-soq", ServerEpochOption, proposed)
		epoch, err = client.readServerEpoch(ctx, false)
		if err != nil {
			if setErr != nil {
				return ServerIdentity{}, fmt.Errorf("set tmux server epoch: %v; read canonical epoch: %w", setErr, err)
			}
			return ServerIdentity{}, err
		}
	}
	if !serverEpochPattern.MatchString(epoch) {
		return ServerIdentity{}, errors.New("tmux server epoch is invalid")
	}
	identity, err := client.ServerIdentity(ctx)
	if err != nil {
		return ServerIdentity{}, err
	}
	if identity.Epoch != epoch {
		return ServerIdentity{}, errors.New("tmux server identity changed during initialization")
	}
	return identity, nil
}

func (client Client) ServerIdentity(ctx context.Context) (ServerIdentity, error) {
	output, err := client.Output(ctx, "read-server-identity", "display-message", "-p",
		"#{"+ServerEpochOption+"}|#{pid}|#{start_time}")
	if err != nil {
		return ServerIdentity{}, err
	}
	fields := strings.Split(output, "|")
	if len(fields) != 3 {
		return ServerIdentity{}, errors.New("tmux server identity is invalid")
	}
	identity := ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}
	if !identity.valid() {
		return ServerIdentity{}, errors.New("tmux server identity is invalid")
	}
	return identity, nil
}

func (client Client) KillSessionIfIdentity(ctx context.Context, id, name string, server ServerIdentity) (bool, error) {
	if !sessionIDPattern.MatchString(id) || name == "" || !server.valid() {
		return false, errors.New("tmux kill identity is invalid")
	}
	condition := mutationIdentityCondition(id, name, server)
	output, err := client.Output(ctx, "kill-session-if-identity", "if-shell", "-F", "-t", id, condition,
		"kill-session -t '"+id+"'",
		"display-message -p -l '"+identityMismatchMarker+"'")
	if err != nil {
		return false, err
	}
	switch output {
	case "":
		return true, nil
	case identityMismatchMarker:
		return false, nil
	default:
		return false, errors.New("tmux conditional kill returned unexpected output")
	}
}

func (client Client) RenameSessionIfIdentity(
	ctx context.Context,
	id string,
	expectedName string,
	newName string,
	server ServerIdentity,
) (bool, error) {
	arguments, err := renameSessionArguments(id, expectedName, newName, server)
	if err != nil {
		return false, err
	}
	output, err := client.Output(ctx, "rename-session-if-identity", arguments[0], arguments[1:]...)
	if err != nil {
		return false, err
	}
	switch output {
	case renameSuccessMarker:
		return true, nil
	case identityMismatchMarker:
		return false, nil
	default:
		return false, errors.New("tmux conditional rename returned unexpected output")
	}
}

func renameSessionArguments(id, expectedName, newName string, server ServerIdentity) ([]string, error) {
	if !sessionIDPattern.MatchString(id) || expectedName == "" ||
		!tmuxCommandTokenPattern.MatchString(newName) || !server.valid() {
		return nil, errors.New("tmux rename identity is invalid")
	}
	return []string{
		"if-shell", "-F", "-t", id, mutationIdentityCondition(id, expectedName, server),
		"rename-session -t '" + id + "' '" + newName + "' ; display-message -p -l '" + renameSuccessMarker + "'",
		"display-message -p -l '" + identityMismatchMarker + "'",
	}, nil
}

func (client Client) AssignCharacterIfUnchanged(
	ctx context.Context,
	id string,
	expected string,
	character string,
	server ServerIdentity,
) (bool, error) {
	arguments, err := characterAssignmentArguments(id, expected, character, server)
	if err != nil {
		return false, err
	}
	output, err := client.Output(ctx, "assign-character-if-unchanged", arguments[0], arguments[1:]...)
	if err != nil {
		return false, err
	}
	switch output {
	case "":
		return true, nil
	case identityMismatchMarker:
		return false, nil
	default:
		return false, errors.New("tmux conditional character assignment returned unexpected output")
	}
}

func (client Client) SetSessionSpaceIfIdentity(ctx context.Context, id string, label space.Label, server ServerIdentity) (bool, error) {
	if !sessionIDPattern.MatchString(id) || !server.valid() {
		return false, errors.New("tmux space assignment identity is invalid")
	}
	assignment := "set-option -u -t '" + id + "' -- " + SpaceOption
	if !label.IsUnassigned() {
		encoded := base64.RawURLEncoding.EncodeToString([]byte(label.String()))
		assignment = "set-option -t '" + id + "' -- " + SpaceOption + " " + encoded
	}
	output, err := client.Output(ctx, "set-space-if-identity", "if-shell", "-F", "-t", id,
		andFormatConditions(sessionLifetimeConditions(id, server)),
		assignment+" ; display-message -p -l '"+spaceSuccessMarker+"'",
		"display-message -p -l '"+identityMismatchMarker+"'")
	if err != nil {
		return false, err
	}
	switch output {
	case spaceSuccessMarker:
		return true, nil
	case identityMismatchMarker:
		return false, nil
	default:
		return false, errors.New("tmux conditional space assignment returned unexpected output")
	}
}

func characterAssignmentArguments(id, expected, character string, server ServerIdentity) ([]string, error) {
	if !sessionIDPattern.MatchString(id) || !tmuxCommandTokenPattern.MatchString(character) || !server.valid() {
		return nil, errors.New("tmux character assignment identity is invalid")
	}
	conditions := append(sessionLifetimeConditions(id, server), "#{==:#{@skid_character},"+formatLiteral(expected)+"}")
	return []string{
		"if-shell", "-F", "-t", id, andFormatConditions(conditions),
		"set-option -t '" + id + "' -- @skid_character " + character,
		"display-message -p -l '" + identityMismatchMarker + "'",
	}, nil
}

func (client Client) readServerEpoch(ctx context.Context, allowAbsent bool) (string, error) {
	epoch, err := client.Output(ctx, "read-server-epoch", "show-options", "-sqv", ServerEpochOption)
	if err != nil {
		return "", err
	}
	if epoch == "" && allowAbsent {
		return "", nil
	}
	return epoch, nil
}

func formatLiteral(value string) string {
	return strings.NewReplacer("#", "##", ",", "#,", "}", "#}").Replace(value)
}

func mutationIdentityCondition(id, name string, server ServerIdentity) string {
	return andFormatConditions(append(sessionLifetimeConditions(id, server), "#{==:#{session_name},"+formatLiteral(name)+"}"))
}

func sessionLifetimeConditions(id string, server ServerIdentity) []string {
	return []string{
		"#{==:#{" + ServerEpochOption + "}," + formatLiteral(server.Epoch) + "}",
		"#{==:#{pid}," + formatLiteral(server.PID) + "}",
		"#{==:#{start_time}," + formatLiteral(server.StartTime) + "}",
		"#{==:#{session_id}," + formatLiteral(id) + "}",
	}
}

func andFormatConditions(conditions []string) string {
	condition := conditions[len(conditions)-1]
	for index := len(conditions) - 2; index >= 0; index-- {
		condition = "#{&&:" + conditions[index] + "," + condition + "}"
	}
	return condition
}

func (identity ServerIdentity) valid() bool {
	return serverEpochPattern.MatchString(identity.Epoch) &&
		serverPIDPattern.MatchString(identity.PID) && startTimePattern.MatchString(identity.StartTime)
}

func (client Client) command(ctx context.Context, stdout *bytes.Buffer, operation string, args ...string) *exec.Cmd {
	return client.commandWithStderr(ctx, stdout, &bytes.Buffer{}, operation, args...)
}

func (client Client) commandWithStderr(ctx context.Context, stdout, stderr *bytes.Buffer, operation string, args ...string) *exec.Cmd {
	commandArgs := make([]string, 0, len(args)+5)
	if client.socketName != "" {
		commandArgs = append(commandArgs, "-L", client.socketName, "-f", "/dev/null")
	}
	commandArgs = append(commandArgs, operation)
	commandArgs = append(commandArgs, args...)
	command := exec.CommandContext(ctx, client.path, commandArgs...)
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
