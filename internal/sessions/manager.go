package sessions

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalcontext"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

var (
	sessionIDPattern     = regexp.MustCompile(`^\$[0-9]+$`)
	paneIDPattern        = regexp.MustCompile(`^%[0-9]+$`)
	serverEpochPattern   = regexp.MustCompile(`^v1-[0-9a-f]{32}$`)
	serverPIDPattern     = regexp.MustCompile(`^[1-9][0-9]*$`)
	serverStartPattern   = regexp.MustCompile(`^[1-9][0-9]*$`)
	identityTokenPattern = regexp.MustCompile(`^v1-[0-9a-f]{32}\.[1-9][0-9]*\.[1-9][0-9]*\.[0-9]+$`)
)

type Manager struct {
	tmux          tmuxclient.Client
	workdir       *workdir.Service
	catalogue     catalog.Catalogue
	profiles      []agentruntime.Profile
	profilesByKey map[agentruntime.ProfileKey]agentruntime.Profile
	mutations     sync.RWMutex
}

func New(config Config) (*Manager, error) {
	if err := requireExecutable(config.TmuxPath); err != nil {
		return nil, fmt.Errorf("tmux executable: %w", err)
	}
	tmux, err := tmuxclient.New(config.TmuxPath, config.SocketName)
	if err != nil {
		return nil, err
	}
	if config.Workdir == nil {
		return nil, errors.New("working directory service is not configured")
	}
	characters, err := catalog.Load(config.CataloguePath)
	if err != nil {
		return nil, err
	}
	profiles, err := agentruntime.ValidateProfiles(config.Profiles)
	if err != nil {
		return nil, err
	}
	profilesByKey := make(map[agentruntime.ProfileKey]agentruntime.Profile, len(profiles))
	for _, profile := range profiles {
		profilesByKey[profile.Key] = profile
	}
	return &Manager{
		tmux:          tmux,
		workdir:       config.Workdir,
		catalogue:     characters,
		profiles:      profiles,
		profilesByKey: profilesByKey,
	}, nil
}

func (manager *Manager) Profiles() []agentruntime.Profile {
	return agentruntime.CloneProfiles(manager.profiles)
}

func (manager *Manager) List(ctx context.Context) (Inventory, error) {
	manager.mutations.Lock()
	locked := true
	defer func() {
		if locked {
			manager.mutations.Unlock()
		}
	}()

	ids, err := manager.tmux.ListSessionIDs(ctx)
	if err != nil {
		return Inventory{}, err
	}
	if len(ids) == 0 {
		// ListSessionIDs reports a host with no tmux server as empty, so this
		// branch also covers "no server exists". It must stay ahead of
		// ensureServerIdentity: that call sets a server option, which would
		// start a tmux server on an idle host every poll. There is no snapshot
		// to validate, so the poll time is the whole honest projection.
		return Inventory{ObservedAt: time.Now().UTC(), Sessions: []Session{}}, nil
	}
	server, err := manager.ensureServerIdentity(ctx)
	if err != nil {
		return Inventory{}, err
	}
	scan, err := manager.scanSessions(ctx)
	if err != nil {
		return Inventory{}, err
	}
	visible, err := manager.normalizeCharacters(ctx, scan, server)
	if err != nil {
		return Inventory{}, err
	}
	visible, err = manager.normalizeNames(ctx, visible, scan.names, server)
	if err != nil {
		return Inventory{}, err
	}
	observations := make([]inspectedSession, 0, len(visible))
	for _, observed := range visible {
		sessionObservation, present, err := manager.inspectRequired(ctx, observed, server)
		if err != nil {
			return Inventory{}, err
		}
		if present {
			observations = append(observations, sessionObservation)
		}
	}
	if err := manager.requireServerIdentity(ctx, server); err != nil {
		return Inventory{}, err
	}
	// One clock for the whole projection, minted only once the snapshot and the
	// server identity that produced it are both validated.
	observedAt := time.Now().UTC()
	manager.mutations.Unlock()
	locked = false
	sessions := make([]Session, 0, len(observations))
	for _, observation := range observations {
		sessions = append(sessions, manager.enrichSession(ctx, observation))
	}
	if err := manager.requireServerIdentity(ctx, server); err != nil {
		return Inventory{}, err
	}
	return Inventory{ObservedAt: observedAt, Sessions: sessions}, nil
}

func (manager *Manager) Create(ctx context.Context, input CreateInput) (ObservedSession, error) {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	return manager.create(ctx, input, "", tmuxclient.ServerIdentity{})
}

// ErrCreateDispatchUnknown means a session may exist; callers must not replay.
var ErrCreateDispatchUnknown = errors.New("session creation completion is unknown")

func (manager *Manager) CreateShell(ctx context.Context, input ShellInput) (ObservedSession, error) {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	server, _, err := manager.sessionLifetimeIdentity(ctx, input.TmuxID, input.IdentityToken)
	if err != nil {
		return ObservedSession{}, err
	}
	anchor, err := manager.tmux.Output(ctx, "read-shell-source-pane", "display-message", "-p", "-t", input.TmuxID, "#{session_id}|#{pane_id}|#{pane_pid}")
	if err != nil {
		return ObservedSession{}, newSessionError(ErrorWorkingDirectoryUnavailable, "The source directory is unavailable.")
	}
	fields := strings.Split(anchor, "|")
	if len(fields) != 3 || fields[0] != input.TmuxID || !paneIDPattern.MatchString(fields[1]) {
		return ObservedSession{}, newSessionError(ErrorWorkingDirectoryUnavailable, "The source directory is unavailable.")
	}
	panePID, err := strconv.Atoi(fields[2])
	if err != nil || panePID <= 0 {
		return ObservedSession{}, newSessionError(ErrorWorkingDirectoryUnavailable, "The source directory is unavailable.")
	}
	foreground, err := processinfo.ObserveForeground(processinfo.PID(panePID))
	if err != nil {
		return ObservedSession{}, newSessionError(ErrorWorkingDirectoryUnavailable, "The source directory is unavailable.")
	}
	if _, _, remote := observedTransport(foreground, nil); remote {
		return ObservedSession{}, newSessionError(ErrorWorkingDirectoryUnavailable, "The source is a remote connection.")
	}
	pane := fields[1]
	cwd, err := manager.tmux.Output(ctx, "read-shell-source-cwd", "display-message", "-p", "-t", pane, "#{pane_current_path}")
	if err != nil || cwd == "" {
		return ObservedSession{}, newSessionError(ErrorWorkingDirectoryUnavailable, "The source directory is unavailable.")
	}
	encoded, err := manager.sessionOption(ctx, input.TmuxID, tmuxclient.GroupOption)
	if err != nil {
		return ObservedSession{}, err
	}
	return manager.create(ctx, CreateInput{Kind: LaunchTerminal, CWD: cwd, Group: decodeGroupMetadata(encoded)}, input.TmuxID, server)
}

func (manager *Manager) create(ctx context.Context, input CreateInput, sourceID string, sourceServer tmuxclient.ServerIdentity) (result ObservedSession, resultErr error) {
	cwd, profile, err := manager.validateCreate(input)
	if err != nil {
		return ObservedSession{}, err
	}
	candidate, err := manager.workdir.ParseCandidate(cwd.String())
	if err != nil {
		panic("validated cwd became invalid")
	} // justify-defect: ValidateStart returns a canonical validated directory.

	epochCandidate, err := newServerEpoch()
	if err != nil {
		return ObservedSession{}, err
	}
	scan, err := manager.scanSessions(ctx)
	if err != nil {
		return ObservedSession{}, err
	}
	name := input.OptionalTmuxName
	if name == "" {
		name = input.preparedName
	}
	_, occupiedPrepared := scan.names[name]
	if name == "" || input.OptionalTmuxName == "" && occupiedPrepared {
		prefix := string(profile.Key)
		if input.Kind == LaunchTerminal {
			prefix = "terminal"
		}
		name = generatedTmuxName(scan.names, prefix)
	} else if _, occupied := scan.names[name]; occupied {
		return ObservedSession{}, newSessionError(ErrorSessionNameConflict, "A tmux session already uses that name.")
	}
	character := selectCharacter(manager.catalogue.Characters(), scan.characterUse, epochCandidate)
	commandArgs := []string{"-d", "-P", "-F", "#{session_id}", "-s", name}
	launch := ""
	if input.Kind == LaunchAgent {
		nativeLaunch := agentruntime.NewLaunch(profile)
		if profile.Provider == agentruntime.ProviderCodex {
			nativeLaunch.Arguments = append(nativeLaunch.Arguments, "--remote", "unix://"+agentruntime.CodexEndpoint(profile), "--cd", cwd.String())
		}
		launch, err = agentruntime.EncodeLaunch(nativeLaunch)
		if err != nil {
			return ObservedSession{}, err
		}
	}
	terminal, err := manager.tmux.TerminalCommand(cwd.String(), launch)
	if err != nil {
		return ObservedSession{}, err
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
	if _, err := manager.workdir.ValidateStart(candidate); err != nil {
		return ObservedSession{}, mapWorkingDirectoryError(err)
	}
	output, accepted, err := manager.tmux.CreateSession(ctx, cwd.String(), sourceID, sourceServer, commandArgs)
	if !accepted {
		if err != nil {
			var pathError *os.PathError
			if errors.As(err, &pathError) && pathError.Op == "chdir" {
				return ObservedSession{}, newSessionError(ErrorWorkingDirectoryUnavailable, "That directory is unavailable.")
			}
			return ObservedSession{}, err
		}
		return ObservedSession{}, sessionIdentityMismatch()
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(ErrCreateDispatchUnknown, resultErr)
		}
	}()
	if err != nil {
		return ObservedSession{}, err
	}
	firstLine, identityLine, separated := strings.Cut(output, "\n")
	if !separated || strings.ContainsRune(identityLine, '\n') {
		return ObservedSession{}, errors.New("tmux create returned an invalid identity transcript")
	}
	id := firstLine
	if !sessionIDPattern.MatchString(id) {
		return ObservedSession{}, errors.New("tmux create returned an invalid session id")
	}
	identityFields := strings.Split(identityLine, "|")
	if len(identityFields) != 4 || identityFields[3] != id {
		return ObservedSession{}, errors.New("tmux create returned an invalid server identity")
	}
	server := tmuxclient.ServerIdentity{
		Epoch: identityFields[0], PID: identityFields[1], StartTime: identityFields[2],
	}
	if !validServerIdentity(server) {
		return ObservedSession{}, errors.New("tmux create returned an invalid server identity")
	}
	observed, found, err := manager.scanSession(ctx, id)
	if err != nil {
		return ObservedSession{}, err
	}
	if !found {
		return ObservedSession{}, errors.New("created tmux session is absent from inventory")
	}
	session, present, err := manager.inspectRequired(ctx, observed, server)
	if err != nil {
		return ObservedSession{}, err
	}
	if !present {
		return ObservedSession{}, errors.New("created tmux session is absent from inventory")
	}
	if err := manager.requireServerIdentity(ctx, server); err != nil {
		return ObservedSession{}, err
	}
	observedAt := time.Now().UTC()
	projected := manager.enrichSession(ctx, session)
	if err := manager.requireServerIdentity(ctx, server); err != nil {
		return ObservedSession{}, err
	}
	return ObservedSession{ObservedAt: observedAt, Session: projected}, nil
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

// Kill deletes only the captured session lifetime. Pane/process changes and
// display names do not participate in authority. The tmux queue owns the guard.
func (manager *Manager) Kill(ctx context.Context, input KillInput) error {
	identity, _, err := manager.sessionLifetimeIdentity(ctx, input.TmuxID, input.IdentityToken)
	if err != nil {
		return err
	}
	killed, err := manager.tmux.KillSessionIfIdentity(ctx, input.TmuxID, identity)
	if errors.Is(err, tmuxclient.ErrDeleteUnknown) {
		return ErrSessionDeleteUnknown
	}
	if err != nil {
		return err
	}
	if !killed {
		return sessionIdentityMismatch()
	}
	return nil
}

func sessionIdentityMismatch() *Error {
	return newSessionError(ErrorSessionIdentityMismatch, "The session changed. Refresh and try again.")
}

func (manager *Manager) inspectRequired(
	ctx context.Context,
	observed scannedSession,
	server tmuxclient.ServerIdentity,
) (inspectedSession, bool, error) {
	if observed.character.Key == "" {
		return inspectedSession{}, false, errors.New("tmux session has no valid character after normalization")
	}

	identityToken, err := makeIdentityToken(server, observed.id)
	if err != nil {
		return inspectedSession{}, false, err
	}
	return manager.inspectAnchor(ctx, Session{
		TmuxID: observed.id, TmuxName: observed.tmuxName, NameMode: effectiveNameMode(observed.tmuxName, observed.autoMarker),
		IdentityToken: identityToken, Character: observed.character,
	})
}

func (manager *Manager) inspectAnchor(ctx context.Context, session Session) (inspectedSession, bool, error) {
	inspected := inspectedSession{session: session}
	id := session.TmuxID
	anchor, err := manager.tmux.Output(ctx, "read-card-anchor", "display-message", "-p", "-t", id,
		"#{session_id}|#{pane_id}|#{pane_pid}|#{session_attached}")
	if err != nil {
		return manager.reconcileFailedInspection(ctx, id, fmt.Errorf("read required tmux card anchor: %w", err))
	}
	fields := strings.Split(anchor, "|")
	if len(fields) != 4 || fields[0] != id {
		return manager.reconcileFailedInspection(ctx, id, errors.New("tmux returned an invalid card anchor"))
	}
	panePID, paneErr := strconv.Atoi(fields[2])
	attached, attachedErr := strconv.Atoi(fields[3])
	if !paneIDPattern.MatchString(fields[1]) || paneErr != nil || panePID < 0 || attachedErr != nil || attached < 0 {
		return manager.reconcileFailedInspection(ctx, id, errors.New("tmux returned an invalid card anchor"))
	}
	inspected.paneID = fields[1]
	inspected.panePID = processinfo.PID(panePID)
	inspected.attachedClients = attached
	return inspected, true, nil
}

// enrichSession projects optional metadata and one foreground sample. A failed
// sample is recorded by ForegroundFailed; the session remains a terminal.
func (manager *Manager) enrichSession(ctx context.Context, inspected inspectedSession) Session {
	session := inspected.session
	session.ActivePaneID = inspected.paneID
	session.panePID = inspected.panePID
	// justify-ignore-error: unreadable optional membership is unassigned and never repaired.
	if encoded, err := manager.sessionOption(ctx, session.TmuxID, tmuxclient.GroupOption); err == nil {
		session.Group = decodeGroupMetadata(encoded)
	}
	session.AttachedClients = inspected.attachedClients
	// justify-ignore-error: optional pane metadata does not suppress an ordinary terminal.
	if cwd, readErr := manager.tmux.Output(ctx, "read-pane-cwd", "display-message", "-p", "-t", inspected.paneID, "#{pane_current_path}"); readErr == nil {
		session.CWD = cwd
	}
	if activeCommand, readErr := manager.tmux.Output(ctx, "read-pane-command", "display-message", "-p", "-t", inspected.paneID, "#{pane_current_command}"); readErr == nil {
		session.ActiveCommand = activeCommand
	}
	if profile, optionErr := manager.sessionOption(ctx, inspected.session.TmuxID, "@skid_profile"); optionErr == nil {
		key := agentruntime.ProfileKey(profile)
		if _, valid := manager.profilesByKey[key]; valid {
			session.LaunchProfile = key
		}
	}
	if encodedObjective, optionErr := manager.sessionOption(ctx, inspected.session.TmuxID, "@skid_objective_b64"); optionErr == nil && encodedObjective != "" {
		objective, decodeErr := base64.RawURLEncoding.DecodeString(encodedObjective)
		if decodeErr == nil && validateObjective(string(objective)) == nil {
			session.Objective = string(objective)
		}
	}
	if encoded, err := manager.sessionOption(ctx, session.TmuxID, conversationOption); err == nil && encoded != "" {
		if conversation, err := decodeConversation(encoded); err == nil {
			session.Conversation = &conversation
		}
	}
	var foregroundErr error
	session.Agent, session.foreground, foregroundErr = manager.observeAgent(ctx, inspected.paneID, inspected.panePID)
	session.foregroundFailed = foregroundErr != nil
	manager.projectClaudeConversation(&session)
	if session.foreground != nil {
		foreground := *session.foreground
		if transport, _, recognized := observedTransport(foreground, nil); recognized {
			environment, err := processinfo.ObserveForegroundEnvironment(inspected.panePID, foreground)
			if errors.Is(err, processinfo.ErrForegroundMismatch) {
				session.Agent, session.foreground, session.foregroundFailed = nil, nil, true
				return session
			}
			_, id, _ := observedTransport(foreground, environment)
			session.Connection = &Connection{Transport: transport, ID: id}
			session.Agent = nil
			session.CWD = ""
		}
	}
	return session
}

func (manager *Manager) TerminalContext(connectionID string) (TerminalContext, error) {
	root, err := terminalcontext.Observe(connectionID)
	if err != nil {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	foreground, err := processinfo.ObserveForeground(root.PID)
	if err != nil || foreground.SessionID != root.PID || foreground.TerminalDevice != root.TerminalDevice {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	if foreground.ExecutableBase() == "tmux" || foreground.ExecutableBase() == "screen" {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	environment, err := processinfo.ObserveForegroundEnvironment(root.PID, foreground)
	if errors.Is(err, processinfo.ErrForegroundMismatch) {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	if err != nil {
		environment = nil
	}
	cwd, cwdErr := processinfo.ObserveCurrentDirectory(foreground.PID, foreground)
	if errors.Is(cwdErr, processinfo.ErrForegroundMismatch) {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	if cwdErr != nil {
		cwd = ""
	}
	currentRoot, err := terminalcontext.Observe(connectionID)
	if err != nil || !processinfo.SameObservation(currentRoot, root) {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	currentForeground, err := processinfo.ObserveForeground(root.PID)
	if err != nil || !processinfo.SameObservation(currentForeground, foreground) {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	result := TerminalContext{ObservedAt: time.Now().UTC(), CWD: cwd}
	if transport, id, recognized := observedTransport(foreground, environment); recognized {
		result.CWD = ""
		result.Connection = &Connection{Transport: transport, ID: id}
	} else if agent := observeRemoteAgent(foreground, environment, manager.profiles); agent != nil {
		result.Agent = &RemoteAgent{Provider: agent.Provider, Profile: agent.Profile}
	}
	return result, nil
}

type inspectedSession struct {
	session         Session
	paneID          string
	panePID         processinfo.PID
	attachedClients int
}

func (manager *Manager) reconcileFailedInspection(
	ctx context.Context,
	id string,
	cause error,
) (inspectedSession, bool, error) {
	present, err := manager.classifyRequiredObservationFailure(ctx, id, cause)
	return inspectedSession{}, present, err
}

func (manager *Manager) classifyRequiredObservationFailure(
	ctx context.Context,
	id string,
	cause error,
) (bool, error) {
	exists, err := manager.tmux.HasSession(ctx, id)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	return true, cause
}

type scannedSession struct {
	id           string
	tmuxName     string
	autoMarker   string
	characterRaw string
	character    catalog.Character
}

type sessionScan struct {
	names        map[string]struct{}
	visible      []scannedSession
	characterUse map[string]int
}

func (manager *Manager) scanSessions(ctx context.Context) (sessionScan, error) {
	ids, err := manager.tmux.ListSessionIDs(ctx)
	if err != nil {
		return sessionScan{}, err
	}
	scan := sessionScan{
		names:        make(map[string]struct{}, len(ids)),
		visible:      make([]scannedSession, 0, len(ids)),
		characterUse: make(map[string]int, len(manager.catalogue.Characters())),
	}
	for _, id := range ids {
		if !sessionIDPattern.MatchString(id) {
			return sessionScan{}, errors.New("tmux returned an invalid session id")
		}
		observed, found, err := manager.scanSession(ctx, id)
		if err != nil {
			return sessionScan{}, err
		}
		if !found {
			continue
		}
		scan.names[observed.tmuxName] = struct{}{}
		scan.visible = append(scan.visible, observed)
		if observed.character.Key != "" {
			scan.characterUse[observed.character.Key]++
		}
	}
	return scan, nil
}

func (manager *Manager) scanSession(ctx context.Context, id string) (scannedSession, bool, error) {
	name, found, err := manager.sessionIdentity(ctx, id)
	if err != nil || !found {
		return scannedSession{}, found, err
	}
	observed := scannedSession{id: id, tmuxName: name}
	observed.autoMarker, found, err = manager.sessionOptionIfPresent(ctx, id, tmuxclient.AutoNameOption)
	if err != nil || !found {
		return scannedSession{}, found, err
	}
	observed.characterRaw, found, err = manager.sessionOptionIfPresent(ctx, id, "@skid_character")
	if err != nil || !found {
		return scannedSession{}, found, err
	}
	observed.character, _ = manager.catalogue.Character(observed.characterRaw)
	return observed, true, nil
}

func (manager *Manager) sessionOptionIfPresent(ctx context.Context, id, option string) (string, bool, error) {
	value, err := manager.sessionOption(ctx, id, option)
	if err == nil {
		return value, true, nil
	}
	exists, existsErr := manager.tmux.HasSession(ctx, id)
	if existsErr != nil {
		return "", false, existsErr
	}
	if !exists {
		return "", false, nil
	}
	return "", false, err
}

func (manager *Manager) normalizeCharacters(
	ctx context.Context,
	scan sessionScan,
	server tmuxclient.ServerIdentity,
) ([]scannedSession, error) {
	normalized := append([]scannedSession(nil), scan.visible...)
	included := make([]bool, len(normalized))
	pending := make([]int, 0, len(normalized))
	for index, observed := range normalized {
		included[index] = true
		if observed.character.Key == "" {
			pending = append(pending, index)
		}
	}
	sort.Slice(pending, func(left, right int) bool {
		return normalized[pending[left]].id < normalized[pending[right]].id
	})
	characters := manager.catalogue.Characters()
	for _, index := range pending {
		observed := normalized[index]
		selected := selectCharacter(characters, scan.characterUse, server.Epoch+"\x00"+observed.id)
		committed, err := manager.tmux.AssignCharacterIfUnchanged(
			ctx, observed.id, observed.characterRaw, selected.Key, server,
		)
		if err != nil || !committed {
			reread, found, rereadErr := manager.scanSession(ctx, observed.id)
			if rereadErr != nil {
				return nil, rereadErr
			}
			if !found {
				included[index] = false
				continue
			}
			if reread.character.Key != "" {
				scan.characterUse[reread.character.Key]++
				normalized[index] = reread
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("assign tmux character to %s: %w", observed.id, err)
			}
			observed = reread
			selected = selectCharacter(characters, scan.characterUse, server.Epoch+"\x00"+observed.id)
			committed, err = manager.tmux.AssignCharacterIfUnchanged(
				ctx, observed.id, observed.characterRaw, selected.Key, server,
			)
			if err != nil {
				return nil, fmt.Errorf("retry tmux character assignment for %s: %w", observed.id, err)
			}
			if !committed {
				return nil, fmt.Errorf("tmux character assignment for %s did not converge", observed.id)
			}
		}
		observed.character = selected
		scan.characterUse[selected.Key]++
		normalized[index] = observed
	}
	visible := make([]scannedSession, 0, len(normalized))
	for index, observed := range normalized {
		if included[index] {
			visible = append(visible, observed)
		}
	}
	return visible, nil
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

func selectCharacter(characters []catalog.Character, characterUse map[string]int, seed string) catalog.Character {
	selected := characters[0]
	selectedUse := characterUse[selected.Key]
	selectedScore := sha256.Sum256([]byte(seed + "\x00" + selected.Key))
	for _, character := range characters[1:] {
		use := characterUse[character.Key]
		score := sha256.Sum256([]byte(seed + "\x00" + character.Key))
		comparison := bytes.Compare(score[:], selectedScore[:])
		if use < selectedUse || use == selectedUse &&
			(comparison > 0 || comparison == 0 && character.Key > selected.Key) {
			selected = character
			selectedUse = use
			selectedScore = score
		}
	}
	return selected
}

func (manager *Manager) sessionOption(ctx context.Context, id, option string) (string, error) {
	return manager.tmux.Output(ctx, "read-session-option", "show-options", "-qv", "-t", id, option)
}

func (manager *Manager) paneOption(ctx context.Context, paneID, option string) (string, error) {
	return manager.tmux.Output(ctx, "read-pane-option", "show-options", "-pqv", "-t", paneID, option)
}

func (manager *Manager) sessionIdentity(ctx context.Context, id string) (string, bool, error) {
	identity, err := manager.tmux.Output(ctx, "read-session-identity", "display-message", "-p", "-t", id, "#{session_id}|#{session_name}")
	if err != nil {
		exists, existsErr := manager.tmux.HasSession(ctx, id)
		if existsErr != nil {
			return "", false, existsErr
		}
		if !exists {
			return "", false, nil
		}
		return "", false, err
	}
	observedID, name, separated := strings.Cut(identity, "|")
	if !separated {
		return "", false, errors.New("tmux returned an invalid session identity")
	}
	if observedID != id {
		return "", false, nil
	}
	return name, true, nil
}

func (manager *Manager) classifyMissingSession(ctx context.Context, id string, cause error) error {
	exists, err := manager.tmux.HasSession(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	return cause
}

func (manager *Manager) ensureServerIdentity(ctx context.Context) (tmuxclient.ServerIdentity, error) {
	proposed, err := newServerEpoch()
	if err != nil {
		return tmuxclient.ServerIdentity{}, err
	}
	return manager.tmux.EnsureServerIdentity(ctx, proposed)
}

func (manager *Manager) requireServerIdentity(ctx context.Context, expected tmuxclient.ServerIdentity) error {
	observed, err := manager.tmux.ServerIdentity(ctx)
	if err != nil {
		return err
	}
	if observed != expected {
		return errors.New("tmux server identity changed during operation")
	}
	return nil
}

func newServerEpoch() (string, error) {
	return serverEpochFromEntropy(rand.Reader)
}

func serverEpochFromEntropy(entropy io.Reader) (string, error) {
	random := make([]byte, 16)
	if _, err := io.ReadFull(entropy, random); err != nil {
		return "", fmt.Errorf("mint tmux server epoch: %w", err)
	}
	return "v1-" + hex.EncodeToString(random), nil
}

func makeIdentityToken(server tmuxclient.ServerIdentity, id string) (string, error) {
	if !validServerIdentity(server) || !sessionIDPattern.MatchString(id) {
		return "", errors.New("invalid tmux lifetime identity")
	}
	return strings.Join([]string{server.Epoch, server.PID, server.StartTime, strings.TrimPrefix(id, "$")}, "."), nil
}

func parseIdentityToken(token, id string) (tmuxclient.ServerIdentity, bool) {
	if !identityTokenPattern.MatchString(token) || !sessionIDPattern.MatchString(id) {
		return tmuxclient.ServerIdentity{}, false
	}
	fields := strings.Split(token, ".")
	if len(fields) != 4 || "$"+fields[3] != id {
		return tmuxclient.ServerIdentity{}, false
	}
	server := tmuxclient.ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}
	return server, validServerIdentity(server)
}

func validServerIdentity(identity tmuxclient.ServerIdentity) bool {
	return serverEpochPattern.MatchString(identity.Epoch) &&
		serverPIDPattern.MatchString(identity.PID) && serverStartPattern.MatchString(identity.StartTime)
}
