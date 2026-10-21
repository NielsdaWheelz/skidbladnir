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
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
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
	tmux      tmuxclient.Client
	workdir   *workdir.Service
	catalogue catalog.Catalogue
	settings  atomic.Pointer[runtimeSettings]
	mutations chan struct{}
	recovery  workspaceRecovery
}

func New(config Config) (*Manager, error) {
	if err := validateShell(config.ShellPath); err != nil {
		return nil, err
	}
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
	if config.Machine.String() == "" || !filepath.IsAbs(config.CheckpointPath) || filepath.Clean(config.CheckpointPath) != config.CheckpointPath {
		return nil, errors.New("workspace checkpoint configuration is invalid")
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
	manager := &Manager{
		tmux:      tmux,
		workdir:   config.Workdir,
		catalogue: characters,
		mutations: make(chan struct{}, 1),
		recovery:  workspaceRecovery{path: config.CheckpointPath, machine: config.Machine, fact: Recovery{State: RecoveryTracking}},
	}
	manager.settings.Store(&runtimeSettings{profiles: profiles, byKey: profilesByKey, shellPath: config.ShellPath, revision: 1})
	return manager, nil
}

type runtimeSettings struct {
	profiles  []agentruntime.Profile
	byKey     map[agentruntime.ProfileKey]agentruntime.Profile
	shellPath string
	revision  uint64
}

func (manager *Manager) ApplySettings(ctx context.Context, profiles []agentruntime.Profile, shellPath string) error {
	validated, err := agentruntime.ValidateProfiles(profiles)
	if err != nil {
		return err
	}
	if err := validateShell(shellPath); err != nil {
		return err
	}
	if err := manager.acquire(ctx); err != nil {
		return err
	}
	defer manager.release()
	current := manager.settings.Load()
	if current.shellPath == shellPath && slices.Equal(current.profiles, validated) {
		return nil
	}
	byKey := make(map[agentruntime.ProfileKey]agentruntime.Profile, len(validated))
	for _, p := range validated {
		byKey[p.Key] = p
	}
	manager.settings.Store(&runtimeSettings{profiles: validated, byKey: byKey, shellPath: shellPath, revision: manager.SettingsRevision() + 1})
	return nil
}
func validateShell(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("shell must be an absolute bash or zsh executable")
	}
	if filepath.Base(path) != "bash" && filepath.Base(path) != "zsh" {
		return errors.New("shell must be bash or zsh")
	}
	if err := requireExecutable(path); err != nil {
		return fmt.Errorf("shell executable: %w", err)
	}
	return nil
}

func (manager *Manager) SettingsRevision() uint64 { return manager.settings.Load().revision }
func (manager *Manager) Profiles() []agentruntime.Profile {
	return agentruntime.CloneProfiles(manager.settings.Load().profiles)
}

func (manager *Manager) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case manager.mutations <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		manager.release()
		return err
	}
	return nil
}
func (manager *Manager) release() { <-manager.mutations }

func (manager *Manager) List(ctx context.Context) (Inventory, ListDiagnostics, error) {
	diagnostics := ListDiagnostics{Phase: "admission"}
	started := time.Now()
	err := manager.acquire(ctx)
	if err != nil {
		elapsed := time.Since(started)
		diagnostics.Admission = &elapsed
		return Inventory{}, diagnostics, err
	}
	locked := true
	defer func() {
		if locked {
			manager.release()
		}
	}()
	observed, present, err := manager.tmux.InspectServer(ctx)
	if err == nil && (!present || observed.SessionCount == 0) {
		elapsed := time.Since(started)
		diagnostics.Admission = &elapsed
		diagnostics.Phase = "complete"
		return Inventory{ObservedAt: time.Now().UTC(), Sessions: []Session{}, Recovery: manager.recovery.fact}, diagnostics, nil
	}
	var server tmuxclient.ServerIdentity
	if err == nil {
		server, err = manager.admitRecoveryServer(ctx, observed.Identity)
	}
	admissionElapsed := time.Since(started)
	diagnostics.Admission = &admissionElapsed
	if err != nil {
		return Inventory{}, diagnostics, err
	}
	diagnostics.Phase = "capture"
	started = time.Now()
	capture, err := manager.tmux.ReadInventory(ctx, server)
	captureElapsed := time.Since(started)
	diagnostics.Capture = &captureElapsed
	if err != nil {
		return Inventory{}, diagnostics, err
	}
	diagnostics.Phase = "normalize"
	started = time.Now()
	scan := manager.scanCapture(capture)
	visible, err := manager.normalizeCharacters(ctx, scan, server)
	if err == nil {
		visible, err = manager.normalizeNames(ctx, visible, scan.names, server)
	}
	if err == nil {
		err = manager.requireServerIdentity(ctx, server)
	}
	normalizeElapsed := time.Since(started)
	diagnostics.Normalize = &normalizeElapsed
	if err != nil {
		return Inventory{}, diagnostics, err
	}
	observations := make([]inspectedSession, 0, len(visible))
	for _, row := range visible {
		token, err := makeIdentityToken(server, row.id)
		if err != nil {
			return Inventory{}, diagnostics, err
		}
		session := Session{TmuxID: row.id, TmuxName: row.tmuxName, NameMode: effectiveNameMode(row.tmuxName, row.autoMarker), IdentityToken: token, Character: row.character}
		observations = append(observations, inspectedSession{session: session, paneID: row.row.PaneID, panePID: processinfo.PID(row.row.PanePID), attachedClients: row.row.AttachedClients, row: row.row})
	}
	observedAt := time.Now().UTC()
	recovery := manager.recovery.fact
	manager.release()
	locked = false
	diagnostics.Phase = "foreground"
	started = time.Now()
	sessions := make([]Session, 0, len(observations))
	for _, observation := range observations {
		if err = ctx.Err(); err != nil {
			break
		}
		sessions = append(sessions, manager.enrichSession(ctx, observation))
	}
	if err == nil {
		err = manager.requireServerIdentity(ctx, server)
	}
	foregroundElapsed := time.Since(started)
	diagnostics.Foreground = &foregroundElapsed
	if err != nil {
		return Inventory{}, diagnostics, err
	}
	diagnostics.Phase = "complete"
	return Inventory{ObservedAt: observedAt, Sessions: sessions, Recovery: recovery}, diagnostics, nil
}

func (manager *Manager) Create(ctx context.Context, input CreateInput) (ObservedSession, error) {
	if err := manager.acquire(ctx); err != nil {
		return ObservedSession{}, err
	}
	defer manager.release()

	return manager.create(ctx, input, "", tmuxclient.ServerIdentity{})
}

// ErrCreateDispatchUnknown means a session may exist; callers must not replay.
var ErrCreateDispatchUnknown = errors.New("session creation completion is unknown")

func (manager *Manager) CreateShell(ctx context.Context, input ShellInput) (ObservedSession, error) {
	if err := manager.acquire(ctx); err != nil {
		return ObservedSession{}, err
	}
	defer manager.release()

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
	return manager.create(ctx, CreateInput{Kind: LaunchTerminal, CWD: cwd, Group: group.DecodeMetadata(encoded)}, input.TmuxID, server)
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
		launch, err = agentruntime.EncodeLaunch(nativeLaunch)
		if err != nil {
			return ObservedSession{}, err
		}
	}
	terminal, err := manager.tmux.WithShell(manager.settings.Load().shellPath).TerminalCommand(cwd.String(), launch)
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
	// Lifetime and pane are now captured by the existing required inspection
	// and server validation. Later projection failure must retain this target.
	result = ObservedSession{ObservedAt: observedAt, Session: session.session}
	result.Session.ActivePaneID = session.paneID
	projected := manager.enrichSession(ctx, session)
	if err := manager.requireServerIdentity(ctx, server); err != nil {
		return result, err
	}
	manager.checkpointAfterMutation(ctx, server)
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
	if err := manager.acquire(ctx); err != nil {
		return err
	}
	defer manager.release()

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
	manager.checkpointAfterMutation(ctx, identity)
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
	row := inspected.row
	if row == nil {
		// single-target controls sample their own terminal facts.
		// justify-ignore-error: these optional reads omit unavailable display facts;
		// required lifetime and anchor validation remain independent.
		row = &tmuxclient.InventoryRow{}
		row.LocalSession.Group, _ = manager.sessionOption(ctx, session.TmuxID, tmuxclient.GroupOption)
		row.CWD, _ = manager.tmux.Output(ctx, "read-pane-cwd", "display-message", "-p", "-t", inspected.paneID, "#{pane_current_path}")
		row.Command, _ = manager.tmux.Output(ctx, "read-pane-command", "display-message", "-p", "-t", inspected.paneID, "#{pane_current_command}")
		row.LocalSession.Profile, _ = manager.sessionOption(ctx, session.TmuxID, "@skid_profile")
		row.LocalSession.Objective, _ = manager.sessionOption(ctx, session.TmuxID, "@skid_objective_b64")
	}
	session.Group = group.DecodeMetadata(row.LocalSession.Group)
	session.AttachedClients = inspected.attachedClients
	session.CWD, session.ActiveCommand = row.CWD, row.Command
	key := agentruntime.ProfileKey(row.LocalSession.Profile)
	if _, valid := manager.Profile(key); valid {
		session.LaunchProfile = key
	}
	if encoded := row.LocalSession.Objective; encoded != "" {
		objective, decodeErr := base64.RawURLEncoding.Strict().DecodeString(encoded)
		if decodeErr == nil && base64.RawURLEncoding.EncodeToString(objective) == encoded && validateObjective(string(objective)) == nil {
			session.Objective = string(objective)
		}
	}
	var foregroundErr error
	session.Agent, session.foreground, foregroundErr = manager.observeAgent(inspected.paneID, inspected.panePID)
	session.foregroundFailed = foregroundErr != nil
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
	} else if agent := observeRemoteAgent(foreground, environment, manager.Profiles()); agent != nil {
		result.Agent = &RemoteAgent{Provider: agent.Provider, Profile: agent.Profile}
	}
	return result, nil
}

type inspectedSession struct {
	row             *tmuxclient.InventoryRow
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
	row        *tmuxclient.InventoryRow
	id         string
	tmuxName   string
	autoMarker string
	character  catalog.Character
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
			return sessionScan{}, tmuxclient.WorkspaceProtocolInvalid
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
	raw, found, err := manager.sessionOptionIfPresent(ctx, id, "@skid_character")
	if err != nil || !found {
		return scannedSession{}, found, err
	}
	observed.character, _ = manager.catalogue.Character(raw)
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

func (manager *Manager) scanCapture(capture tmuxclient.InventoryCapture) sessionScan {
	scan := sessionScan{names: make(map[string]struct{}, len(capture.Rows)), visible: make([]scannedSession, 0, len(capture.Rows)), characterUse: make(map[string]int)}
	for _, row := range capture.Rows {
		observed := manager.scannedRow(row)
		scan.names[row.Name] = struct{}{}
		scan.visible = append(scan.visible, observed)
		if observed.character.Key != "" {
			scan.characterUse[observed.character.Key]++
		}
	}
	return scan
}
func (manager *Manager) scannedRow(row tmuxclient.InventoryRow) scannedSession {
	character, _ := manager.catalogue.Character(row.LocalSession.Character)
	return scannedSession{id: row.ID, tmuxName: row.Name, autoMarker: row.LocalSession.AutoName, character: character, row: &row}
}
func (manager *Manager) rereadSession(ctx context.Context, id string, server tmuxclient.ServerIdentity) (scannedSession, bool, error) {
	row, found, err := manager.tmux.ReadInventoryRow(ctx, server, id)
	if err != nil || !found {
		return scannedSession{}, found, err
	}
	return manager.scannedRow(row), true, nil
}
func (manager *Manager) normalizeCharacters(ctx context.Context, scan sessionScan, server tmuxclient.ServerIdentity) ([]scannedSession, error) {
	normalized := append([]scannedSession(nil), scan.visible...)
	pending := make([]int, 0)
	for i, observed := range normalized {
		if observed.character.Key == "" {
			pending = append(pending, i)
		}
	}
	sort.Slice(pending, func(i, j int) bool { return normalized[pending[i]].id < normalized[pending[j]].id })
	characters := manager.catalogue.Characters()
	removed := make(map[string]bool)
	for _, index := range pending {
		observed := normalized[index]
		for attempt := 0; attempt < 2; attempt++ {
			// Presence is distinct from an empty local value; show-options without -A
			// reports only the local definition. Its value needs no shell parsing.
			local, err := manager.tmux.Output(ctx, "read-character-presence", "show-options", "-q", "-t", observed.id, "@skid_character")
			if err != nil {
				present, classified := manager.classifyRequiredObservationFailure(ctx, observed.id, err)
				if classified != nil {
					return nil, classified
				}
				if !present {
					removed[observed.id] = true
					break
				}
			}
			raw, found, err := manager.sessionOptionIfPresent(ctx, observed.id, "@skid_character")
			if err != nil {
				return nil, err
			}
			if !found {
				removed[observed.id] = true
				break
			}
			var expected *string
			if local != "" {
				expected = &raw
			}
			var committed bool
			var writeErr error
			if _, valid := manager.catalogue.Character(raw); !valid {
				selected := selectCharacter(characters, scan.characterUse, server.Epoch+"\x00"+observed.id)
				committed, writeErr = manager.tmux.AssignCharacterIfUnchanged(ctx, observed.id, expected, selected.Key, server)
			}
			if errors.Is(writeErr, tmuxclient.WorkspaceProtocolInvalid) {
				return nil, writeErr
			}
			actual, found, err := manager.rereadSession(ctx, observed.id, server)
			if err != nil {
				return nil, err
			}
			if !found {
				removed[observed.id] = true
				break
			}
			if actual.character.Key != "" {
				scan.characterUse[actual.character.Key]++
				normalized[index] = actual
				break
			}
			if writeErr != nil {
				return nil, writeErr
			}
			if committed || attempt == 1 {
				return nil, tmuxclient.WorkspaceTopologyChanged
			}
			observed = actual
		}
	}
	visible := make([]scannedSession, 0, len(normalized))
	for _, observed := range normalized {
		if !removed[observed.id] {
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
	if !separated || !sessionIDPattern.MatchString(observedID) {
		return "", false, tmuxclient.WorkspaceProtocolInvalid
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

func (manager *Manager) requireServerIdentity(ctx context.Context, expected tmuxclient.ServerIdentity) error {
	observed, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		return err
	}
	if !present || observed.Identity != expected {
		return tmuxclient.WorkspaceSourceChanged
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
