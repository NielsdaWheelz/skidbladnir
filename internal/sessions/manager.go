package sessions

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
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
	home          string
	catalogue     catalog.Catalogue
	profiles      []Profile
	profilesByKey map[string]Profile
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
	if !filepath.IsAbs(config.Home) {
		return nil, errors.New("service home must be absolute")
	}
	if err := requireSearchableDirectory(config.Home); err != nil {
		return nil, fmt.Errorf("service home: %w", err)
	}
	characters, err := catalog.Load(config.CataloguePath)
	if err != nil {
		return nil, err
	}
	profiles, err := validateProfiles(config.Profiles)
	if err != nil {
		return nil, err
	}
	profilesByKey := make(map[string]Profile, len(profiles))
	for _, profile := range profiles {
		profilesByKey[profile.Key] = profile
	}
	return &Manager{
		tmux:          tmux,
		home:          filepath.Clean(config.Home),
		catalogue:     characters,
		profiles:      profiles,
		profilesByKey: profilesByKey,
		activeShadows: make(map[string]struct{}),
	}, nil
}

func (manager *Manager) Profiles() []Profile {
	profiles := make([]Profile, len(manager.profiles))
	for index, profile := range manager.profiles {
		profile.Environment = append([]EnvironmentVariable(nil), profile.Environment...)
		profile.ForegroundSignatures = append([]ForegroundSignature(nil), profile.ForegroundSignatures...)
		profile.Arguments = append([]string(nil), profile.Arguments...)
		profiles[index] = profile
	}
	return profiles
}

func (manager *Manager) List(ctx context.Context) ([]Session, error) {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	ids, err := manager.tmux.ListSessionIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []Session{}, nil
	}
	server, err := manager.ensureServerIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := manager.tmux.ReconcilePhoneShadows(ctx, server, manager.protectedPhoneShadows()); err != nil {
		return nil, err
	}
	ids, err = manager.tmux.ListSessionIDs(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, 0, len(ids))
	for _, id := range ids {
		if !sessionIDPattern.MatchString(id) {
			return nil, errors.New("tmux returned an invalid session id")
		}
		session, include, err := manager.inspect(ctx, id, server)
		if err != nil {
			exists, existsErr := manager.tmux.HasSession(ctx, id)
			if existsErr != nil {
				return nil, existsErr
			}
			if !exists {
				continue
			}
			return nil, err
		}
		if include {
			sessions = append(sessions, session)
		}
	}
	if err := manager.requireServerIdentity(ctx, server); err != nil {
		return nil, err
	}
	sort.Slice(sessions, func(left, right int) bool {
		if sessions[left].Attention != sessions[right].Attention {
			return sessions[left].Attention
		}
		leftRank, rightRank := statusRank(sessions[left].Status.Kind), statusRank(sessions[right].Status.Kind)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if sessions[left].Name != sessions[right].Name {
			return sessions[left].Name < sessions[right].Name
		}
		return sessions[left].ID < sessions[right].ID
	})
	return sessions, nil
}

func (manager *Manager) Create(ctx context.Context, input CreateInput) (Session, error) {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	cwd, err := validateWorkingDirectory(input.CWD, manager.home)
	if err != nil {
		return Session{}, err
	}
	profile, found := manager.profilesByKey[input.Profile]
	if !found {
		return Session{}, newSessionError(ErrorProfileUnknown, "Choose an available profile.")
	}
	if err := validateOptionalName(input.OptionalName); err != nil {
		return Session{}, err
	}
	if err := validateObjective(input.Objective); err != nil {
		return Session{}, err
	}

	names, characterUse, err := manager.namesAndCharacterUse(ctx)
	if err != nil {
		return Session{}, err
	}
	name := input.OptionalName
	var character catalog.Character
	if name == "" {
		name, character = manager.generatedIdentity(names)
	} else {
		if _, occupied := names[name]; occupied {
			return Session{}, newSessionError(ErrorSessionNameConflict, "A tmux session already uses that name.")
		}
		character = manager.leastUsedCharacter(characterUse)
	}

	epochCandidate, err := newServerEpoch()
	if err != nil {
		return Session{}, err
	}
	commandArgs := []string{"-d", "-P", "-F", "#{session_id}", "-s", name, "-c", cwd}
	for _, variable := range profile.Environment {
		commandArgs = append(commandArgs, "-e", variable.Name+"="+variable.Value)
	}
	commandArgs = append(commandArgs, "--", profile.Command)
	commandArgs = append(commandArgs, profile.Arguments...)
	exactName := "=" + name + ":"
	commandArgs = append(commandArgs,
		";", "set-option", "-soq", tmuxclient.ServerEpochOption, epochCandidate,
		";", "set-option", "-t", exactName, "--", "@skid_profile", profile.Key,
		";", "set-option", "-t", exactName, "--", "@skid_character", character.Key)
	if input.Objective != "" {
		encodedObjective := base64.RawURLEncoding.EncodeToString([]byte(input.Objective))
		commandArgs = append(commandArgs, ";", "set-option", "-t", exactName, "--", "@skid_objective_b64", encodedObjective)
	}
	commandArgs = append(commandArgs,
		";", "display-message", "-p", "-t", exactName,
		"#{"+tmuxclient.ServerEpochOption+"}|#{pid}|#{start_time}|#{session_id}")
	output, err := manager.tmux.Output(ctx, "create-session", "new-session", commandArgs...)
	if err != nil {
		firstLine, _, _ := strings.Cut(output, "\n")
		if sessionIDPattern.MatchString(firstLine) {
			return Session{}, fmt.Errorf("tmux create sequence failed after session creation: %w", err)
		}
		occupied, listErr := manager.sessionNameExists(ctx, name)
		if listErr != nil {
			return Session{}, fmt.Errorf("create tmux session: %w; classify name conflict: %v", err, listErr)
		}
		if occupied {
			return Session{}, newSessionError(ErrorSessionNameConflict, "A tmux session already uses that name.")
		}
		return Session{}, err
	}
	firstLine, identityLine, separated := strings.Cut(output, "\n")
	if !separated || strings.ContainsRune(identityLine, '\n') {
		return Session{}, errors.New("tmux create returned an invalid identity transcript")
	}
	id := firstLine
	if !sessionIDPattern.MatchString(id) {
		return Session{}, errors.New("tmux create returned an invalid session id")
	}
	identityFields := strings.Split(identityLine, "|")
	if len(identityFields) != 4 || identityFields[3] != id {
		return Session{}, errors.New("tmux create returned an invalid server identity")
	}
	server := tmuxclient.ServerIdentity{
		Epoch: identityFields[0], PID: identityFields[1], StartTime: identityFields[2],
	}
	if !validServerIdentity(server) {
		return Session{}, errors.New("tmux create returned an invalid server identity")
	}
	session, include, err := manager.inspect(ctx, id, server)
	if err != nil {
		return Session{}, err
	}
	if !include {
		return Session{}, errors.New("created tmux session is absent from inventory")
	}
	if err := manager.requireServerIdentity(ctx, server); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (manager *Manager) Kill(ctx context.Context, input KillInput) error {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	server, err := manager.killIdentity(ctx, input)
	if err != nil {
		return err
	}
	if _, err := manager.tmux.ReconcilePhoneShadows(ctx, server, manager.protectedPhoneShadows()); err != nil {
		return err
	}
	server, err = manager.killIdentity(ctx, input)
	if err != nil {
		return err
	}
	killed, err := manager.tmux.KillSessionIfIdentityAndIsolated(ctx, input.ID, input.DisplayedName, server)
	if err != nil {
		return manager.classifyMissingSession(ctx, input.ID, err)
	}
	if !killed {
		if _, identityErr := manager.killIdentity(ctx, input); identityErr != nil {
			return identityErr
		}
		return newSessionError(ErrorSessionGroupedConflict, "This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it.")
	}
	return nil
}

func (manager *Manager) ValidateKill(ctx context.Context, input KillInput) error {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	_, err := manager.killIdentity(ctx, input)
	return err
}

func (manager *Manager) killIdentity(ctx context.Context, input KillInput) (tmuxclient.ServerIdentity, error) {
	if !sessionIDPattern.MatchString(input.ID) {
		return tmuxclient.ServerIdentity{}, newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	server, validToken := parseIdentityToken(input.IdentityToken, input.ID)
	if input.DisplayedName == "" || !validToken {
		return tmuxclient.ServerIdentity{}, newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before killing it.")
	}
	name, found, err := manager.sessionIdentity(ctx, input.ID)
	if err != nil {
		return tmuxclient.ServerIdentity{}, err
	}
	if !found {
		return tmuxclient.ServerIdentity{}, newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	if name != input.DisplayedName {
		return tmuxclient.ServerIdentity{}, newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before killing it.")
	}
	observed, err := manager.tmux.ServerIdentity(ctx)
	if err != nil || observed != server {
		return tmuxclient.ServerIdentity{}, newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before killing it.")
	}
	return server, nil
}

func (manager *Manager) inspect(ctx context.Context, id string, server tmuxclient.ServerIdentity) (Session, bool, error) {
	name, found, err := manager.sessionIdentity(ctx, id)
	if err != nil {
		return Session{}, false, err
	}
	if !found {
		return Session{}, false, nil
	}
	internal, err := manager.sessionOption(ctx, id, "@skid_internal")
	if err != nil {
		return Session{}, false, err
	}
	if tmuxclient.IsPhoneShadow(name, internal) {
		return Session{}, false, nil
	}

	identityToken, err := makeIdentityToken(server, id)
	if err != nil {
		return Session{}, false, err
	}
	session := Session{ID: id, Name: name, IdentityToken: identityToken}
	anchor, err := manager.tmux.Output(ctx, "read-card-anchor", "display-message", "-p", "-t", id,
		"#{session_id}|#{pane_id}|#{pane_pid}|#{window_bell_flag}|#{session_attached}|#{session_group_attached}")
	if err != nil {
		session.Status = unknownStatus()
		return session, true, nil
	}
	fields := strings.Split(anchor, "|")
	if len(fields) != 6 || fields[0] != id || !paneIDPattern.MatchString(fields[1]) {
		session.Status = unknownStatus()
		return session, true, nil
	}
	panePID, paneErr := strconv.Atoi(fields[2])
	bell, bellErr := parseTmuxBoolean(fields[3])
	attached, attachedErr := strconv.Atoi(fields[4])
	groupAttached := attached
	var groupErr error
	if fields[5] != "" {
		groupAttached, groupErr = strconv.Atoi(fields[5])
	}
	if paneErr != nil || panePID <= 0 || bellErr != nil || attachedErr != nil || attached < 0 || groupErr != nil || groupAttached < 0 {
		session.Status = unknownStatus()
		return session, true, nil
	}
	session.AttachedClients = groupAttached

	session.CWD, err = manager.tmux.Output(ctx, "read-pane-cwd", "display-message", "-p", "-t", fields[1], "#{pane_current_path}")
	if err != nil {
		session.Status = unknownStatus()
		return session, true, nil
	}
	session.ActiveCommand, err = manager.tmux.Output(ctx, "read-pane-command", "display-message", "-p", "-t", fields[1], "#{pane_current_command}")
	if err != nil {
		session.Status = unknownStatus()
		return session, true, nil
	}
	if profile, optionErr := manager.sessionOption(ctx, id, "@skid_profile"); optionErr != nil {
		session.Status = unknownStatus()
		return session, true, nil
	} else if _, valid := manager.profilesByKey[profile]; valid {
		session.Profile = profile
	}
	if encodedObjective, optionErr := manager.sessionOption(ctx, id, "@skid_objective_b64"); optionErr != nil {
		session.Status = unknownStatus()
		return session, true, nil
	} else if encodedObjective != "" {
		objective, decodeErr := base64.RawURLEncoding.DecodeString(encodedObjective)
		if decodeErr == nil && validateObjective(string(objective)) == nil {
			session.Objective = string(objective)
		}
	}
	if characterKey, optionErr := manager.sessionOption(ctx, id, "@skid_character"); optionErr != nil {
		session.Status = unknownStatus()
		return session, true, nil
	} else if character, valid := manager.catalogue.Character(characterKey); valid {
		session.CharacterKey = character.Key
		session.CharacterDisplayName = character.DisplayName
	}
	attention, err := manager.paneOption(ctx, fields[1], "@skid_attention")
	if err != nil {
		session.Status = unknownStatus()
		return session, true, nil
	}
	lifecycle, err := manager.paneOption(ctx, fields[1], "@skid_lifecycle")
	if err != nil {
		session.Status = unknownStatus()
		return session, true, nil
	}
	now := time.Now().UTC()
	session.Status = manager.deriveStatus(panePID, lifecycle, now)
	_, notified := parseAttentionTime(attention, now)
	session.Attention = bell || notified
	return session, true, nil
}

func (manager *Manager) namesAndCharacterUse(ctx context.Context) (map[string]struct{}, map[string]int, error) {
	ids, err := manager.tmux.ListSessionIDs(ctx)
	if err != nil {
		return nil, nil, err
	}
	names := make(map[string]struct{}, len(ids))
	characterUse := make(map[string]int, len(manager.catalogue.Characters()))
	for _, id := range ids {
		if !sessionIDPattern.MatchString(id) {
			return nil, nil, errors.New("tmux returned an invalid session id")
		}
		name, found, nameErr := manager.sessionIdentity(ctx, id)
		if nameErr != nil {
			return nil, nil, nameErr
		}
		if !found {
			continue
		}
		names[name] = struct{}{}
		internal, optionErr := manager.sessionOption(ctx, id, "@skid_internal")
		if optionErr != nil {
			return nil, nil, optionErr
		}
		if tmuxclient.IsPhoneShadow(name, internal) {
			continue
		}
		characterKey, optionErr := manager.sessionOption(ctx, id, "@skid_character")
		if optionErr != nil {
			return nil, nil, optionErr
		}
		if _, valid := manager.catalogue.Character(characterKey); valid {
			characterUse[characterKey]++
		}
	}
	return names, characterUse, nil
}

func (manager *Manager) generatedIdentity(names map[string]struct{}) (string, catalog.Character) {
	characters := manager.catalogue.Characters()
	for suffix := 1; suffix < math.MaxInt; suffix++ {
		for _, character := range characters {
			name := "ga-" + character.NameSuffix
			if suffix > 1 {
				name += "-" + strconv.Itoa(suffix)
			}
			if _, occupied := names[name]; !occupied {
				return name, character
			}
		}
	}
	panic("unreachable generated session namespace exhaustion")
}

func (manager *Manager) protectedPhoneShadows() []string {
	protected := make([]string, 0, len(manager.activeShadows))
	for name := range manager.activeShadows {
		protected = append(protected, name)
	}
	sort.Strings(protected)
	return protected
}

func (manager *Manager) leastUsedCharacter(characterUse map[string]int) catalog.Character {
	characters := manager.catalogue.Characters()
	selected := characters[0]
	for _, character := range characters[1:] {
		if characterUse[character.Key] < characterUse[selected.Key] {
			selected = character
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

func (manager *Manager) sessionNameExists(ctx context.Context, expected string) (bool, error) {
	ids, err := manager.tmux.ListSessionIDs(ctx)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		name, found, nameErr := manager.sessionIdentity(ctx, id)
		if nameErr != nil {
			return false, nameErr
		}
		if found && name == expected {
			return true, nil
		}
	}
	return false, nil
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

func parseTmuxBoolean(value string) (bool, error) {
	switch value {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, errors.New("tmux returned an invalid boolean")
	}
}

func unknownStatus() Status {
	return Status{Kind: StatusUnknown, Signal: StatusSignalPollFailure, SignalAt: time.Now().UTC()}
}

func statusRank(status StatusKind) int {
	switch status {
	case StatusWorking:
		return 0
	case StatusRunning:
		return 1
	case StatusIdle:
		return 2
	case StatusShell:
		return 3
	case StatusUnknown:
		return 4
	default:
		panic("unknown session status")
	}
}
