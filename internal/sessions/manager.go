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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
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
	activeShadows map[string]struct{}
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
		activeShadows: make(map[string]struct{}),
	}, nil
}

func (manager *Manager) Profiles() []agentruntime.Profile {
	return agentruntime.CloneProfiles(manager.profiles)
}

func (manager *Manager) List(ctx context.Context) (Inventory, error) {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

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
	if _, err := manager.tmux.ReconcilePhoneShadows(ctx, server, manager.protectedPhoneShadows()); err != nil {
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
	projected := make([]inspectedSession, 0, len(observations))
	reconciled := false
	for _, observation := range observations {
		session, projectErr := projectSession(observation, observedAt)
		if projectErr != nil {
			present, reconcileErr := manager.classifyRequiredObservationFailure(
				ctx, observation.session.TmuxID, projectErr,
			)
			if reconcileErr != nil {
				return Inventory{}, reconcileErr
			}
			if !present {
				reconciled = true
				continue
			}
		}
		observation.session = session
		projected = append(projected, observation)
	}
	if reconciled {
		if err := manager.requireServerIdentity(ctx, server); err != nil {
			return Inventory{}, err
		}
	}
	sessions := make([]Session, 0, len(projected))
	for _, observation := range projected {
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

	candidate, err := manager.workdir.ParseCandidate(input.CWD)
	if err != nil {
		return ObservedSession{}, mapWorkingDirectoryError(err)
	}
	cwd, err := manager.workdir.ValidateStart(candidate)
	if err != nil {
		return ObservedSession{}, mapWorkingDirectoryError(err)
	}
	profile, found := manager.profilesByKey[agentruntime.ProfileKey(input.Profile)]
	if !found {
		return ObservedSession{}, newSessionError(ErrorProfileUnknown, "Choose an available profile.")
	}
	if input.OptionalTmuxName != "" {
		if err := validateTmuxName(input.OptionalTmuxName); err != nil {
			return ObservedSession{}, err
		}
	}
	if err := validateObjective(input.Objective); err != nil {
		return ObservedSession{}, err
	}

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
		name = generatedTmuxName(scan.names, string(profile.Key))
	} else if _, occupied := scan.names[name]; occupied {
		return ObservedSession{}, newSessionError(ErrorSessionNameConflict, "A tmux session already uses that name.")
	}
	character := selectCharacter(manager.catalogue.Characters(), scan.characterUse, epochCandidate)
	commandArgs := []string{"-d", "-P", "-F", "#{session_id}", "-s", name, "-c", cwd.String()}
	for _, variable := range profile.Environment {
		commandArgs = append(commandArgs, "-e", variable.Name+"="+variable.Value)
	}
	commandArgs = append(commandArgs, "--", profile.Command)
	commandArgs = append(commandArgs, agentruntime.LaunchArguments(profile, name)...)
	exactName := "=" + name + ":"
	commandArgs = append(commandArgs,
		";", "set-option", "-soq", tmuxclient.ServerEpochOption, epochCandidate,
		";", "set-option", "-t", exactName, "--", "@skid_profile", string(profile.Key),
		";", "set-option", "-t", exactName, "--", "@skid_character", character.Key)
	if input.Objective != "" {
		encodedObjective := base64.RawURLEncoding.EncodeToString([]byte(input.Objective))
		commandArgs = append(commandArgs, ";", "set-option", "-t", exactName, "--", "@skid_objective_b64", encodedObjective)
	}
	commandArgs = append(commandArgs,
		";", "display-message", "-p", "-t", exactName,
		"#{"+tmuxclient.ServerEpochOption+"}|#{pid}|#{start_time}|#{session_id}")
	if _, err := manager.workdir.ValidateStart(candidate); err != nil {
		return ObservedSession{}, mapWorkingDirectoryError(err)
	}
	output, err := manager.tmux.Output(ctx, "create-session", "new-session", commandArgs...)
	if err != nil {
		firstLine, _, _ := strings.Cut(output, "\n")
		if sessionIDPattern.MatchString(firstLine) {
			return ObservedSession{}, fmt.Errorf("tmux create sequence failed after session creation: %w", err)
		}
		observed, listErr := manager.scanSessions(ctx)
		if listErr != nil {
			return ObservedSession{}, fmt.Errorf("create tmux session: %w; classify name conflict: %v", err, listErr)
		}
		_, occupied := observed.names[name]
		if occupied {
			return ObservedSession{}, newSessionError(ErrorSessionNameConflict, "A tmux session already uses that name.")
		}
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
	if !found || observed.phoneShadow {
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
	projected, err := projectSession(session, observedAt)
	if err != nil {
		present, reconcileErr := manager.classifyRequiredObservationFailure(ctx, observed.id, err)
		if reconcileErr != nil {
			return ObservedSession{}, reconcileErr
		}
		if !present {
			return ObservedSession{}, errors.New("created tmux session is absent from inventory")
		}
	}
	session.session = projected
	projected = manager.enrichSession(ctx, session)
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

func (manager *Manager) Kill(ctx context.Context, input KillInput) error {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	identity, err := manager.mutationIdentity(ctx, input.TmuxID, input.TmuxName, input.IdentityToken)
	if err != nil {
		return err
	}
	if _, err := manager.tmux.ReconcilePhoneShadows(ctx, identity.server, manager.protectedPhoneShadows()); err != nil {
		return err
	}
	identity, err = manager.mutationIdentity(ctx, input.TmuxID, input.TmuxName, input.IdentityToken)
	if err != nil {
		return err
	}
	killed, err := manager.tmux.KillSessionIfIdentityAndIsolated(ctx, input.TmuxID, input.TmuxName, identity.server)
	if err != nil {
		return manager.classifyMissingSession(ctx, input.TmuxID, err)
	}
	if !killed {
		if _, identityErr := manager.mutationIdentity(ctx, input.TmuxID, input.TmuxName, input.IdentityToken); identityErr != nil {
			return identityErr
		}
		return newSessionError(ErrorSessionGroupedConflict, "This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it.")
	}
	return nil
}

func (manager *Manager) ValidateKill(ctx context.Context, input KillInput) error {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	_, err := manager.mutationIdentity(ctx, input.TmuxID, input.TmuxName, input.IdentityToken)
	return err
}

type sessionMutationIdentity struct {
	server      tmuxclient.ServerIdentity
	phoneShadow bool
}

func (manager *Manager) Rename(ctx context.Context, input RenameInput) error {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	if err := validateTmuxName(input.NewTmuxName); err != nil {
		return err
	}
	identity, err := manager.mutationIdentity(ctx, input.TmuxID, input.TmuxName, input.IdentityToken)
	if err != nil {
		return err
	}
	if identity.phoneShadow {
		return sessionIdentityMismatch()
	}
	if input.NewTmuxName == input.TmuxName {
		return newSessionError(ErrorSessionNameConflict, "A tmux session already uses that name.")
	}
	renamed, err := manager.tmux.RenameSessionIfIdentityAndOrdinary(
		ctx, input.TmuxID, input.TmuxName, input.NewTmuxName, identity.server,
	)
	if err != nil {
		return manager.classifyRenameFailure(ctx, input, identity.server, err)
	}
	if !renamed {
		return manager.classifyRenameFailure(
			ctx, input, identity.server, errors.New("tmux conditional rename rejected an exact preflight identity"),
		)
	}
	return nil
}

func (manager *Manager) mutationIdentity(
	ctx context.Context,
	tmuxID string,
	tmuxName string,
	identityToken string,
) (sessionMutationIdentity, error) {
	if !sessionIDPattern.MatchString(tmuxID) {
		return sessionMutationIdentity{}, newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	server, validToken := parseIdentityToken(identityToken, tmuxID)
	if tmuxName == "" || !validToken {
		return sessionMutationIdentity{}, sessionIdentityMismatch()
	}
	name, found, err := manager.sessionIdentity(ctx, tmuxID)
	if err != nil {
		return sessionMutationIdentity{}, err
	}
	if !found {
		return sessionMutationIdentity{}, newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	if name != tmuxName {
		return sessionMutationIdentity{}, sessionIdentityMismatch()
	}
	internal, found, err := manager.sessionOptionIfPresent(ctx, tmuxID, "@skid_internal")
	if err != nil {
		return sessionMutationIdentity{}, err
	}
	if !found {
		return sessionMutationIdentity{}, newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	observed, err := manager.tmux.ServerIdentity(ctx)
	if err != nil || observed != server {
		return sessionMutationIdentity{}, sessionIdentityMismatch()
	}
	return sessionMutationIdentity{server: server, phoneShadow: tmuxclient.IsPhoneShadow(name, internal)}, nil
}

func (manager *Manager) classifyRenameFailure(
	ctx context.Context,
	input RenameInput,
	expectedServer tmuxclient.ServerIdentity,
	cause error,
) error {
	name, found, err := manager.sessionIdentity(ctx, input.TmuxID)
	if err != nil {
		return fmt.Errorf("reread failed rename source: %w", err)
	}
	if !found {
		return newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	server, err := manager.tmux.ServerIdentity(ctx)
	if err != nil || server != expectedServer {
		return sessionIdentityMismatch()
	}
	internal, found, err := manager.sessionOptionIfPresent(ctx, input.TmuxID, "@skid_internal")
	if err != nil {
		return fmt.Errorf("reread failed rename source marker: %w", err)
	}
	if !found {
		return newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	if name == input.NewTmuxName {
		return fmt.Errorf("tmux rename failed after the source acquired the desired name: %w", cause)
	}
	if name != input.TmuxName || tmuxclient.IsPhoneShadow(name, internal) {
		return sessionIdentityMismatch()
	}
	destinationID, occupied, err := manager.sessionIDNamed(ctx, input.NewTmuxName)
	if err != nil {
		return fmt.Errorf("reread failed rename destination: %w", err)
	}
	if occupied && destinationID != input.TmuxID {
		identity, identityErr := manager.mutationIdentity(
			ctx, input.TmuxID, input.TmuxName, input.IdentityToken,
		)
		if identityErr != nil {
			return identityErr
		}
		if identity.phoneShadow {
			return sessionIdentityMismatch()
		}
		return newSessionError(ErrorSessionNameConflict, "A tmux session already uses that name.")
	}
	return fmt.Errorf("tmux rename failed for an unchanged exact identity: %w", cause)
}

func (manager *Manager) sessionIDNamed(ctx context.Context, name string) (string, bool, error) {
	output, err := manager.tmux.Output(ctx, "read-session-name-owner", "list-sessions",
		"-f", "#{==:#{session_name},"+name+"}", "-F", "#{session_id}")
	if err != nil {
		return "", false, err
	}
	if output == "" {
		return "", false, nil
	}
	if strings.ContainsRune(output, '\n') || !sessionIDPattern.MatchString(output) {
		return "", false, errors.New("tmux returned an invalid session name owner")
	}
	return output, true, nil
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
	inspected := inspectedSession{session: Session{
		TmuxID: observed.id, TmuxName: observed.tmuxName,
		IdentityToken: identityToken, Character: observed.character,
	}}
	anchor, err := manager.tmux.Output(ctx, "read-card-anchor", "display-message", "-p", "-t", observed.id,
		"#{session_id}|#{pane_id}|#{pane_pid}|#{window_activity}|#{session_attached}|#{session_group_attached}")
	if err != nil {
		return manager.reconcileFailedInspection(ctx, observed.id, fmt.Errorf("read required tmux card anchor: %w", err))
	}
	fields := strings.Split(anchor, "|")
	if len(fields) != 6 || fields[0] != observed.id {
		return manager.reconcileFailedInspection(ctx, observed.id, errors.New("tmux returned an invalid card anchor"))
	}
	activity, err := parseActivitySecond(fields[3])
	if err != nil {
		return manager.reconcileFailedInspection(ctx, observed.id, err)
	}
	inspected.activitySecond = activity
	if !paneIDPattern.MatchString(fields[1]) {
		return inspected, true, nil
	}
	panePID, paneErr := strconv.Atoi(fields[2])
	attached, attachedErr := strconv.Atoi(fields[4])
	groupAttached := attached
	var groupErr error
	if fields[5] != "" {
		groupAttached, groupErr = strconv.Atoi(fields[5])
	}
	if paneErr != nil || panePID <= 0 || attachedErr != nil || attached < 0 || groupErr != nil || groupAttached < 0 {
		return inspected, true, nil
	}
	inspected.paneID = fields[1]
	inspected.panePID = processinfo.PID(panePID)
	inspected.attachedClients = groupAttached
	return inspected, true, nil
}

func (manager *Manager) enrichSession(ctx context.Context, inspected inspectedSession) Session {
	session := inspected.session
	if inspected.paneID == "" {
		return session
	}
	session.AttachedClients = inspected.attachedClients
	// justify-ignore-error: after the required current-window activity is valid,
	// every remaining field is optional card metadata. A failed read omits only
	// that field and cannot manufacture or suppress activity.
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
	registration := ""
	if observedRegistration, optionErr := manager.paneOption(ctx, inspected.paneID, agentruntime.PaneOption); optionErr == nil {
		registration = observedRegistration
	}
	// justify-ignore-error: process identity is optional card metadata. An exited
	// process or unstable read omits agent identity without changing the required
	// activity observation.
	foreground, observeErr := processinfo.ObserveForeground(inspected.panePID)
	if observeErr != nil {
		foreground = processinfo.Observation{}
	}
	session.Agent = deriveAgent(manager.profiles, foreground, registration)
	return session
}

type inspectedSession struct {
	session         Session
	activitySecond  activitySecond
	paneID          string
	panePID         processinfo.PID
	attachedClients int
}

func projectSession(inspected inspectedSession, observedAt time.Time) (Session, error) {
	activity, err := deriveActivity(inspected.activitySecond, observedAt)
	if err != nil {
		return Session{}, err
	}
	projected := inspected.session
	projected.Activity = activity
	return projected, nil
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
	characterRaw string
	character    catalog.Character
	phoneShadow  bool
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
		if observed.phoneShadow {
			continue
		}
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
	internal, found, err := manager.sessionOptionIfPresent(ctx, id, "@skid_internal")
	if err != nil || !found {
		return scannedSession{}, found, err
	}
	observed := scannedSession{
		id: id, tmuxName: name, phoneShadow: tmuxclient.IsPhoneShadow(name, internal),
	}
	if observed.phoneShadow {
		return observed, true, nil
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
			if !found || reread.phoneShadow {
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

func (manager *Manager) protectedPhoneShadows() []string {
	protected := make([]string, 0, len(manager.activeShadows))
	for name := range manager.activeShadows {
		protected = append(protected, name)
	}
	sort.Strings(protected)
	return protected
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
