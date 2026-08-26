package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var socketNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const (
	ServerEpochOption      = "@skid_server_epoch"
	identityMismatchMarker = "SKIDBLADNIR_IDENTITY_MISMATCH_V1"
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

func (client Client) KillSessionIfIdentityAndIsolated(ctx context.Context, id, name string, server ServerIdentity) (bool, error) {
	if !sessionIDPattern.MatchString(id) || name == "" || !server.valid() {
		return false, errors.New("tmux kill identity is invalid")
	}
	condition := killEligibilityCondition(id, name, server)
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

func killEligibilityCondition(id, name string, server ServerIdentity) string {
	isolated := "#{||:#{==:#{session_group_size},},#{==:#{session_group_size},1}}"
	return "#{&&:" + killIdentityCondition(id, name, server) + "," + isolated + "}"
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

func killIdentityCondition(id, name string, server ServerIdentity) string {
	conditions := []string{
		"#{==:#{" + ServerEpochOption + "}," + formatLiteral(server.Epoch) + "}",
		"#{==:#{pid}," + formatLiteral(server.PID) + "}",
		"#{==:#{start_time}," + formatLiteral(server.StartTime) + "}",
		"#{==:#{session_id}," + formatLiteral(id) + "}",
		"#{==:#{session_name}," + formatLiteral(name) + "}",
	}
	return "#{&&:" + conditions[0] + ",#{&&:" + conditions[1] + ",#{&&:" +
		conditions[2] + ",#{&&:" + conditions[3] + "," + conditions[4] + "}}}}"
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
