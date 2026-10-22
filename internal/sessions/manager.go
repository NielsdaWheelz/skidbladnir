package sessions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type Manager struct {
	tmux      tmuxclient.Client
	workdir   *workdir.Service
	settings  atomic.Pointer[runtimeSettings]
	mutations chan struct{}
	// admitted is the last lifetime List admitted; read and written only under
	// mutations. It is a hint, never authority: every use is confirmed by the
	// B and E records of the read it skips admission for, and a failed read is
	// a changed lifetime only once InspectServer no longer finds it.
	admitted tmuxclient.ServerIdentity
	recovery workspaceRecovery
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
	if config.Machine.String() == "" || !filepath.IsAbs(config.CheckpointPath) || filepath.Clean(config.CheckpointPath) != config.CheckpointPath {
		return nil, errors.New("workspace checkpoint configuration is invalid")
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

// List returns one admitted lifetime's sessions (tmux-exec-budget.md a1). The
// inventory's B and E records prove that lifetime for every row and token.
// Epoch admission runs only for a lifetime List has not admitted and never
// stamps an absent or empty server.
func (manager *Manager) List(ctx context.Context) (Inventory, error) {
	if err := manager.acquire(ctx); err != nil {
		return Inventory{}, err
	}
	locked := true
	defer func() {
		if locked {
			manager.release()
		}
	}()
	capture, err := manager.tmux.ReadCurrentInventory(ctx, manager.admitted)
	if err != nil {
		return Inventory{}, err
	}
	if len(capture.Rows) > 0 && capture.Server != manager.admitted {
		observed, present, err := manager.tmux.InspectServer(ctx)
		admit := err == nil && present && observed.SessionCount > 0
		var server tmuxclient.ServerIdentity
		if admit {
			server, err = manager.tmux.EnsureServerIdentity(ctx, observed.Identity, newServerEpoch())
		}
		if err != nil {
			return Inventory{}, err
		}
		capture = tmuxclient.InventoryCapture{}
		if admit {
			manager.admitted = server
			capture, err = manager.tmux.ReadInventory(ctx, server)
			if err != nil {
				return Inventory{}, err
			}
		}
	}
	if len(capture.Rows) == 0 {
		return Inventory{ObservedAt: time.Now().UTC(), Sessions: []Session{}, Recovery: manager.recovery.fact}, nil
	}
	scan := manager.scanCapture(capture)
	rows, err := manager.normalizeCharacters(ctx, scan, capture.Server)
	if err == nil {
		rows, err = manager.normalizeNames(ctx, rows, scan.names, capture.Server)
	}
	if err != nil {
		return Inventory{}, err
	}
	observedAt := time.Now().UTC()
	recovery := manager.recovery.fact
	manager.release()
	locked = false
	sessions := make([]Session, 0, len(rows))
	for _, row := range rows {
		if err = ctx.Err(); err != nil {
			break
		}
		sessions = append(sessions, manager.enrichSession(capture.Server, row))
	}
	if err == nil {
		// Enrichment ran outside any tmux client; a2 folds this into its bracket.
		err = manager.requireServerIdentity(ctx, capture.Server)
	}
	if err != nil {
		return Inventory{}, err
	}
	return Inventory{ObservedAt: observedAt, Sessions: sessions, Recovery: recovery}, nil
}

// Kill deletes only the captured session lifetime. Pane/process changes and
// display names do not participate in authority. The tmux queue owns the guard;
// one read classifies its refusal.
func (manager *Manager) Kill(ctx context.Context, input KillInput) error {
	if err := manager.acquire(ctx); err != nil {
		return err
	}
	defer manager.release()

	server, err := parseLifetime(input.TmuxID, input.IdentityToken)
	if err != nil {
		return err
	}
	killed, err := manager.tmux.KillSessionIfIdentity(ctx, input.TmuxID, server)
	if errors.Is(err, tmuxclient.ErrDispatchUnknown) {
		return ErrSessionDeleteUnknown
	}
	if err != nil {
		return err
	}
	if !killed {
		return manager.refused(ctx, input.TmuxID)
	}
	manager.checkpointAfterMutation(ctx, server)
	return nil
}

// enrichSession projects one inventory row of server and one foreground sample.
// server must be validated (admitted, or create's validated identity) and the
// row id came from tmux ingress, so the token needs no validation. A failed
// sample is recorded by ForegroundFailed; the session remains a terminal.
func (manager *Manager) enrichSession(server tmuxclient.ServerIdentity, row tmuxclient.InventoryRow) Session {
	character, _ := catalog.Character(row.LocalSession.Character)
	panePID := processinfo.PID(row.PanePID)
	session := Session{
		TmuxID: row.ID, TmuxName: row.Name, NameMode: effectiveNameMode(row.Name, row.LocalSession.AutoName),
		IdentityToken: strings.Join([]string{server.Epoch, server.PID, server.StartTime, strings.TrimPrefix(row.ID, "$")}, "."), Character: character,
		ActivePaneID: row.PaneID, panePID: panePID, AttachedClients: row.AttachedClients,
		CWD: row.CWD, ActiveCommand: row.Command, Group: group.DecodeMetadata(row.LocalSession.Group),
	}
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
	session.Agent, session.foreground, foregroundErr = manager.observeAgent(row.PaneID, panePID)
	session.foregroundFailed = foregroundErr != nil
	if session.foreground != nil {
		foreground := *session.foreground
		if transport := connectionTransport(foreground); transport != "" {
			environment, err := processinfo.ObserveForegroundEnvironment(panePID, foreground)
			if errors.Is(err, processinfo.ErrForegroundMismatch) {
				session.Agent, session.foreground, session.foregroundFailed = nil, nil, true
				return session
			}
			session.Connection = &Connection{Transport: transport, ID: connectionID(environment)}
			session.Agent = nil
			session.CWD = ""
		}
	}
	return session
}

// sessionScan is one inventory's rows with their name set and catalogue key
// use. recovery reads only names.
type sessionScan struct {
	names        map[string]struct{}
	rows         []tmuxclient.InventoryRow
	characterUse map[string]int
}

func (manager *Manager) scanCapture(capture tmuxclient.InventoryCapture) sessionScan {
	scan := sessionScan{names: make(map[string]struct{}, len(capture.Rows)), rows: capture.Rows, characterUse: make(map[string]int)}
	for _, row := range capture.Rows {
		scan.names[row.Name] = struct{}{}
		if character, valid := catalog.Character(row.LocalSession.Character); valid {
			scan.characterUse[character.Key]++
		}
	}
	return scan
}

// normalizeCharacters gives each row without a catalogue key the least-used
// one (automatic-dwarf-identity.md), written only while the session's
// format-visible raw value is the sampled one. A lost race re-reads the row
// once: a vanished session is omitted, a now-valid key is accepted, otherwise
// one more guarded attempt runs against the newly observed raw value.
func (manager *Manager) normalizeCharacters(ctx context.Context, scan sessionScan, server tmuxclient.ServerIdentity) ([]tmuxclient.InventoryRow, error) {
	rows := slices.Clone(scan.rows)
	pending := make([]int, 0)
	for i, row := range rows {
		if _, valid := catalog.Character(row.LocalSession.Character); !valid {
			pending = append(pending, i)
		}
	}
	sort.Slice(pending, func(i, j int) bool { return rows[pending[i]].ID < rows[pending[j]].ID })
	removed := make(map[string]bool)
	for _, index := range pending {
		id := rows[index].ID
		key := selectCharacter(scan.characterUse, server.Epoch+"\x00"+id).Key
		expected := rows[index].Format.Character
		for attempt := 0; ; attempt++ {
			applied, err := manager.tmux.AssignCharacterIfUnchanged(ctx, id, expected, key, server)
			if err != nil {
				return nil, err
			}
			if applied {
				// tmux printed the applied marker after the write in the same queue.
				rows[index].LocalSession.Character = key
				scan.characterUse[key]++
				break
			}
			capture, err := manager.tmux.ReadSession(ctx, id)
			if err != nil {
				return nil, err
			}
			if len(capture.Rows) == 0 {
				removed[id] = true
				break
			}
			if capture.Server != server {
				return nil, tmuxclient.WorkspaceSourceChanged
			}
			current := capture.Rows[0]
			if character, valid := catalog.Character(current.LocalSession.Character); valid {
				rows[index] = current
				scan.characterUse[character.Key]++
				break
			}
			if attempt == 1 {
				return nil, tmuxclient.WorkspaceTopologyChanged
			}
			expected = current.Format.Character
		}
	}
	return slices.DeleteFunc(rows, func(row tmuxclient.InventoryRow) bool { return removed[row.ID] }), nil
}

func selectCharacter(characterUse map[string]int, seed string) catalog.Entry {
	characters := catalog.Characters()
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
