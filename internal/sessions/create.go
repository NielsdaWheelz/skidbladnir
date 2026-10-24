package sessions

import (
	"context"
	"encoding/base64"
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

// ErrCreateDispatchUnknown means a session may exist; callers must not replay.
var ErrCreateDispatchUnknown = errors.New("session creation completion is unknown")

func (manager *Manager) Create(ctx context.Context, input CreateInput) (ObservedSession, error) {
	if err := manager.acquire(ctx); err != nil {
		return ObservedSession{}, err
	}
	observed, server, err := manager.create(ctx, input, "", tmuxclient.ServerIdentity{})
	manager.release()
	if err == nil {
		manager.checkpointAfterMutation(ctx, server)
	}
	return observed, err
}

// CreateShell starts a terminal at the source's selected-pane directory and in
// its group. One read of the source supplies lifetime, pane, cwd and group; the
// create queue re-checks the source lifetime before dispatch.
func (manager *Manager) CreateShell(ctx context.Context, input ShellInput) (ObservedSession, error) {
	if err := manager.acquire(ctx); err != nil {
		return ObservedSession{}, err
	}
	locked := true
	defer func() {
		if locked {
			manager.release()
		}
	}()

	server, row, err := manager.resolve(ctx, input.TmuxID, input.IdentityToken)
	if err != nil {
		return ObservedSession{}, err
	}
	unavailable := newSessionError(ErrorWorkingDirectoryUnavailable, "The source directory is unavailable.")
	if row.PanePID <= 0 {
		return ObservedSession{}, unavailable
	}
	foreground, err := processinfo.ObserveForeground(processinfo.PID(row.PanePID))
	if err != nil {
		return ObservedSession{}, unavailable
	}
	if connectionTransport(foreground) != "" {
		return ObservedSession{}, newSessionError(ErrorWorkingDirectoryUnavailable, "The source is a remote connection.")
	}
	if row.CWD == "" {
		return ObservedSession{}, unavailable
	}
	observed, createdServer, err := manager.create(ctx, CreateInput{Kind: LaunchTerminal, CWD: row.CWD, Group: group.DecodeMetadata(row.LocalSession.Group)}, input.TmuxID, server)
	manager.release()
	locked = false
	if err == nil {
		manager.checkpointAfterMutation(ctx, createdServer)
	}
	return observed, err
}

func (manager *Manager) create(ctx context.Context, input CreateInput, sourceID string, sourceServer tmuxclient.ServerIdentity) (_ ObservedSession, _ tmuxclient.ServerIdentity, resultErr error) {
	cwd, profile, err := manager.validateCreate(input)
	if err != nil {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, err
	}
	epochCandidate := newServerEpoch()
	// Names and character use are advisory samples of whichever server runs:
	// tmux enforces name uniqueness, and the create queue stamps the epoch, so
	// no lifetime is held.
	current, err := manager.tmux.ReadCurrentInventory(ctx, tmuxclient.ServerIdentity{})
	if err != nil {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, err
	}
	scan := manager.scanCapture(current)
	name := input.OptionalTmuxName
	if name == "" {
		prefix := string(profile.Key)
		if input.Kind == LaunchTerminal {
			prefix = "terminal"
		}
		name = generatedTmuxName(scan.names, prefix)
	} else if _, occupied := scan.names[name]; occupied {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, newSessionError(ErrorSessionNameConflict, "A tmux session already uses that name.")
	}
	character := selectCharacter(scan.characterUse, epochCandidate)
	commandArgs := []string{"-d", "-P", "-F", "#{session_id}", "-s", name}
	// The launch is the profile's command text, opaque to every hop until
	// agent-exec decodes it for the initialized shell (one binary produces and
	// consumes it).
	launch := ""
	if input.Kind == LaunchAgent {
		launch = base64.RawURLEncoding.EncodeToString([]byte(profile.Command))
	}
	terminal, err := manager.tmux.WithShell(manager.settings.Load().shellPath).TerminalCommand(cwd, launch)
	if err != nil {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, err
	}
	commandArgs = append(commandArgs, terminal...)
	exactName := "=" + name + ":"
	commandArgs = append(commandArgs, ";", "set-option", "-soq", tmuxclient.ServerEpochOption, epochCandidate)
	if input.Kind == LaunchAgent {
		commandArgs = append(commandArgs, ";", "set-option", "-t", exactName, "--", "@skid_profile", string(profile.Key))
	}
	if input.OptionalTmuxName == "" {
		commandArgs = append(commandArgs, ";", "set-option", "-t", exactName, "--", tmuxclient.AutoNameOption, encodeAutoName(name))
	}
	commandArgs = append(commandArgs, ";", "set-option", "-t", exactName, "--", "@skid_character", character.Key)
	if input.Objective != "" {
		encodedObjective := base64.RawURLEncoding.EncodeToString([]byte(input.Objective))
		commandArgs = append(commandArgs, ";", "set-option", "-t", exactName, "--", "@skid_objective_b64", encodedObjective)
	}
	if !input.Group.IsUnassigned() {
		encodedGroup := base64.RawURLEncoding.EncodeToString([]byte(input.Group.String()))
		commandArgs = append(commandArgs, ";", "set-option", "-t", exactName, "--", tmuxclient.GroupOption, encodedGroup)
	}
	commandArgs = append(commandArgs,
		";", "display-message", "-p", "-t", exactName,
		"#{"+tmuxclient.ServerEpochOption+"}|#{pid}|#{start_time}|#{session_id}")
	// A directory that vanished since validation fails the tmux client's chdir,
	// which maps to the same answer.
	output, accepted, err := manager.tmux.CreateSession(ctx, cwd, sourceID, sourceServer, commandArgs)
	if !accepted {
		if err != nil {
			var pathError *os.PathError
			if errors.As(err, &pathError) && pathError.Op == "chdir" {
				return ObservedSession{}, tmuxclient.ServerIdentity{}, newSessionError(ErrorWorkingDirectoryUnavailable, "That directory is unavailable.")
			}
			return ObservedSession{}, tmuxclient.ServerIdentity{}, err
		}
		return ObservedSession{}, tmuxclient.ServerIdentity{}, sessionIdentityMismatch()
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(ErrCreateDispatchUnknown, resultErr)
		}
	}()
	if err != nil {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, err
	}
	firstLine, identityLine, separated := strings.Cut(output, "\n")
	if !separated || strings.ContainsRune(identityLine, '\n') {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, errors.New("tmux create returned an invalid identity transcript")
	}
	id := firstLine
	if !tmuxclient.ValidID(id, '$') {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, errors.New("tmux create returned an invalid session id")
	}
	identityFields := strings.Split(identityLine, "|")
	if len(identityFields) != 4 || identityFields[3] != id {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, errors.New("tmux create returned an invalid server identity")
	}
	server, err := tmuxclient.ParseServerIdentity(identityFields[0], identityFields[1], identityFields[2])
	if err != nil || server.Epoch == "" {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, errors.New("tmux create returned an invalid server identity")
	}
	created, err := manager.tmux.ReadSession(ctx, id)
	if err != nil {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, err
	}
	if len(created.Rows) == 0 {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, errors.New("created tmux session is absent from inventory")
	}
	if created.Server != server {
		return ObservedSession{}, tmuxclient.ServerIdentity{}, tmuxclient.WorkspaceSourceChanged
	}
	// Under the mutation lock, this is the snapshot validateCreate admitted
	// the profile from.
	observed := ObservedSession{ObservedAt: time.Now().UTC(), Session: enrichSession(server, created.Rows[0], manager.settings.Load())}
	// The target is captured; a later failure retains it for the caller.
	if err := manager.RequireServer(ctx, server); err != nil {
		return observed, server, err
	}
	return observed, server, nil
}

func mapWorkingDirectoryError(err error) error {
	code, classified := workdir.ErrorCodeOf(err)
	if !classified {
		return err
	}
	switch code {
	case workdir.Invalid:
		return newSessionError(ErrorWorkingDirectoryInvalid, "Use an absolute directory path or ~/… without terminal controls.")
	case workdir.Unavailable:
		return newSessionError(ErrorWorkingDirectoryUnavailable, "That directory is unavailable.")
	default:
		return err
	}
}

func generatedTmuxName(names map[string]struct{}, profile string) string {
	prefix := "skidbladnir-" + profile + "-"
	for suffix := 1; suffix < math.MaxInt; suffix++ {
		name := prefix + strconv.Itoa(suffix)
		if _, occupied := names[name]; !occupied {
			return name
		}
	}
	panic("unreachable generated session namespace exhaustion")
}
