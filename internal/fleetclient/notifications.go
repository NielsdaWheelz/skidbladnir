package fleetclient

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

var ErrNotificationsUnavailable = errors.New("notifications unavailable")

type TerminalKey struct {
	Machine       string `json:"machine"`
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}

func NotificationKey(ref Reference) TerminalKey {
	return TerminalKey{ref.Machine, ref.TmuxID, ref.IdentityToken, ref.PaneID}
}

type Foreground struct {
	Provider      string `json:"provider"`
	PID           int64  `json:"pid"`
	StartIdentity string `json:"startIdentity"`
}

type NotificationRecord struct {
	Key             TerminalKey `json:"key"`
	Revision        int64       `json:"revision"`
	Foreground      *Foreground `json:"foreground,omitempty"`
	Pending         bool        `json:"pending"`
	BaselinePending bool        `json:"baselinePending"`
}

func (record *NotificationRecord) UnmarshalJSON(encoded []byte) error {
	var value struct {
		Key             TerminalKey `json:"key"`
		Revision        *int64      `json:"revision"`
		Foreground      *Foreground `json:"foreground,omitempty"`
		Pending         *bool       `json:"pending"`
		BaselinePending *bool       `json:"baselinePending"`
	}
	if strictjson.Decode(encoded, &value) != nil || value.Revision == nil || value.Pending == nil || value.BaselinePending == nil {
		return ErrNotificationsUnavailable
	}
	*record = NotificationRecord{value.Key, *value.Revision, value.Foreground, *value.Pending, *value.BaselinePending}
	return nil
}

type NotificationSnapshot struct {
	Schema    int                  `json:"schema"`
	Terminals []NotificationRecord `json:"terminals"`
}

func (snapshot NotificationSnapshot) Record(key TerminalKey) (NotificationRecord, bool) {
	for _, record := range snapshot.Terminals {
		if record.Key == key {
			return record, true
		}
	}
	return NotificationRecord{}, false
}

type WorkingPredecessor struct {
	Foreground Foreground
	Revision   int64
}

type NotificationStore struct{ path string }

func NewNotificationStore(path string) (*NotificationStore, error) {
	if !filepath.IsAbs(path) {
		return nil, ErrNotificationsUnavailable
	}
	return &NotificationStore{path: path}, nil
}
func DefaultNotificationStore() (*NotificationStore, error) {
	directory := os.Getenv("XDG_STATE_HOME")
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, ErrNotificationsUnavailable
		}
		directory = filepath.Join(home, ".local", "state")
	}
	return NewNotificationStore(filepath.Join(directory, "skidbladnir", "notifications.json"))
}
func (store *NotificationStore) Read() (NotificationSnapshot, error) {
	return store.merge(func(*NotificationSnapshot) (bool, error) { return false, nil })
}

// Observe commits one inventory batch. expected is read before its network
// dispatch; predecessors belong to this client, never to the persisted cache.
// Conflicting observations are discarded, not rebased onto another client's work.
func (store *NotificationStore) Observe(peers []Peer, machines []Machine, expected NotificationSnapshot, predecessors map[TerminalKey]WorkingPredecessor) (NotificationSnapshot, map[TerminalKey]WorkingPredecessor, error) {
	return store.observe(peers, machines, expected, predecessors, true)
}

// ObserveSession settles a standalone visit using an exact info observation.
// A single-session observation cannot retire unrelated sessions.
func (store *NotificationStore) ObserveSession(session Session, machines []Machine, expected NotificationSnapshot) (NotificationSnapshot, map[TerminalKey]WorkingPredecessor, error) {
	ref, _ := DecodeReference(session.Ref)
	return store.observe([]Peer{{Machine: ref.Machine, OK: true, Sessions: []Session{session}}}, machines, expected, nil, false)
}

func (store *NotificationStore) observe(peers []Peer, machines []Machine, expected NotificationSnapshot, predecessors map[TerminalKey]WorkingPredecessor, inventory bool) (NotificationSnapshot, map[TerminalKey]WorkingPredecessor, error) {
	next := map[TerminalKey]WorkingPredecessor{}
	snapshot, err := store.merge(func(snapshot *NotificationSnapshot) (bool, error) {
		changed := false
		snapshot.Terminals = slices.DeleteFunc(snapshot.Terminals, func(record NotificationRecord) bool {
			configured := false
			for _, m := range machines {
				if m.Handle == record.Key.Machine {
					configured = true
					break
				}
			}
			if !configured {
				changed = true
				return true
			}
			if !inventory {
				return false
			}
			before, expectedPresent := expected.Record(record.Key)
			if !expectedPresent || before.Revision != record.Revision {
				return false
			}
			for _, peer := range peers {
				if peer.Machine != record.Key.Machine || !peer.OK {
					continue
				}
				for _, session := range peer.Sessions {
					ref, _ := DecodeReference(session.Ref)
					if ref.TmuxID == record.Key.TmuxID && ref.IdentityToken == record.Key.IdentityToken {
						return false
					}
				}
				changed = true
				return true
			}
			return false
		})
		// A scoped inventory says nothing about other hosts. Preserve their local
		// predecessors only while the durable revision still matches exactly.
		for key, predecessor := range predecessors {
			sampled := slices.ContainsFunc(peers, func(peer Peer) bool { return peer.Machine == key.Machine })
			record, found := snapshot.Record(key)
			if !sampled && found && record.Revision == predecessor.Revision {
				next[key] = predecessor
			}
		}
		for _, peer := range peers {
			if !peer.OK {
				continue
			}
			for _, session := range peer.Sessions {
				ref, _ := DecodeReference(session.Ref)
				key := NotificationKey(ref)
				index := slices.IndexFunc(snapshot.Terminals, func(record NotificationRecord) bool { return record.Key == key })
				previous, present := snapshot.Record(key)
				before, expectedPresent := expected.Record(key)
				if present != expectedPresent || present && previous.Revision != before.Revision {
					continue
				}
				if previous.Revision == math.MaxInt64 {
					return false, ErrNotificationsUnavailable
				}
				// A changed selection proves only that the former pane is unselected.
				// Keep its revision so late samples cannot recreate consumed attention.
				for i := range snapshot.Terminals {
					old := &snapshot.Terminals[i]
					if old.Key.Machine != key.Machine || old.Key.TmuxID != key.TmuxID || old.Key.IdentityToken != key.IdentityToken || old.Key.PaneID == key.PaneID {
						continue
					}
					before, found := expected.Record(old.Key)
					if !found || before.Revision != old.Revision {
						continue
					}
					if old.Revision == math.MaxInt64 {
						return false, ErrNotificationsUnavailable
					}
					old.Pending = false
					old.BaselinePending = false
					old.Revision++
					changed = true
				}
				record := previous
				record.Key = key
				qualified := session.TerminalStatus.Source == "terminal" && session.TerminalStatus.State != "unknown"
				var foreground *Foreground
				if session.Agent != nil && session.Connection == nil {
					foreground = &Foreground{session.Agent.Provider, session.Agent.PID, session.Agent.StartIdentity}
				}
				same := foreground != nil && previous.Foreground != nil && *foreground == *previous.Foreground
				// An unavailable capture lacks positive exit evidence. Fresh captured shell
				// or changed process facts can clear the former foreground's attention.
				positive := session.TerminalStatus.Source == "terminal" || foreground != nil
				if positive && !same {
					record.Pending = false
					record.Foreground = foreground
				}
				record.Revision++
				working := false
				switch {
				case record.BaselinePending && (qualified || session.TerminalStatus.Source == "terminal" && foreground == nil):
					record.Pending = false
					record.BaselinePending = false
					working = foreground != nil && session.TerminalStatus.State == "working"
				case !qualified || foreground == nil:
				case session.TerminalStatus.State == "working":
					record.Pending = false
					working = true
				case session.TerminalStatus.State == "blocked":
					record.Pending = false
				case session.TerminalStatus.State == "idle":
					predecessor, found := predecessors[key]
					if same && found && predecessor.Foreground == *foreground && predecessor.Revision == previous.Revision {
						record.Pending = true
					}
				default:
					panic("invalid owned terminal status")
				}
				if index < 0 {
					snapshot.Terminals = append(snapshot.Terminals, record)
				} else {
					snapshot.Terminals[index] = record
				}
				if working {
					next[key] = WorkingPredecessor{*foreground, record.Revision}
				}
				changed = true
			}
		}
		return changed, nil
	})
	if err != nil {
		return NotificationSnapshot{}, nil, err
	}
	return snapshot, next, nil
}

func (store *NotificationStore) Presented(key TerminalKey) (NotificationSnapshot, error) {
	return store.consume(key, false)
}
func (store *NotificationStore) EndVisit(key TerminalKey) (NotificationSnapshot, error) {
	return store.consume(key, true)
}
func (store *NotificationStore) consume(key TerminalKey, ended bool) (NotificationSnapshot, error) {
	return store.merge(func(snapshot *NotificationSnapshot) (bool, error) {
		index := slices.IndexFunc(snapshot.Terminals, func(record NotificationRecord) bool { return record.Key == key })
		if index < 0 {
			snapshot.Terminals = append(snapshot.Terminals, NotificationRecord{Key: key})
			index = len(snapshot.Terminals) - 1
		}
		record := &snapshot.Terminals[index]
		if record.Revision == math.MaxInt64 {
			return false, ErrNotificationsUnavailable
		}
		record.Pending = false
		record.BaselinePending = ended
		record.Revision++
		return true, nil
	})
}

// AttentionText is the single notification/status projection. Fresh inventory
// and an exact matching idle foreground are necessary to display saved attention.
func AttentionText(session Session, machine string, fresh bool, snapshot NotificationSnapshot) string {
	text := SessionStatus(session)
	if !fresh || session.Agent == nil || session.Connection != nil || session.TerminalStatus.Source != "terminal" || session.TerminalStatus.State != "idle" {
		return text
	}
	ref, _ := DecodeReference(session.Ref)
	record, found := snapshot.Record(NotificationKey(ref))
	foreground := Foreground{session.Agent.Provider, session.Agent.PID, session.Agent.StartIdentity}
	if found && record.Pending && !record.BaselinePending && record.Key.Machine == machine && record.Foreground != nil && *record.Foreground == foreground {
		return "ready"
	}
	return text
}

// A stable sidecar is locked across read–merge–atomic-replace. Locking the
// replaced data inode would let another process overwrite a notification clear.
func (store *NotificationStore) merge(update func(*NotificationSnapshot) (bool, error)) (snapshot NotificationSnapshot, resultErr error) {
	if err := os.MkdirAll(filepath.Dir(store.path), 0700); err != nil {
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	}
	lock, err := os.OpenFile(store.path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	}
	defer func() {
		if lock.Close() != nil {
			resultErr = ErrNotificationsUnavailable
		}
	}()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	}
	defer func() {
		if syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) != nil {
			resultErr = ErrNotificationsUnavailable
		}
	}()
	snapshot = NotificationSnapshot{Schema: 1, Terminals: []NotificationRecord{}}
	encoded, err := os.ReadFile(store.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	}
	if err == nil {
		var stored NotificationSnapshot
		if !nonNullJSON(encoded) || strictjson.Decode(encoded, &stored) != nil || !validNotificationSnapshot(stored) {
			return NotificationSnapshot{}, ErrNotificationsUnavailable
		}
		snapshot = stored
	}
	changed, updateErr := update(&snapshot)
	if updateErr != nil {
		return NotificationSnapshot{}, updateErr
	}
	if !changed {
		return snapshot, nil
	}
	encoded, err = json.Marshal(snapshot)
	if err != nil {
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	}
	file, err := os.CreateTemp(filepath.Dir(store.path), ".notifications-*")
	if err != nil {
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	}
	name := file.Name()
	defer func() {
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = ErrNotificationsUnavailable
		}
	}()
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	} // justify-ignore-error: failed write is primary; no replacement occurred.
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	} // justify-ignore-error: failed sync is primary; no replacement occurred.
	if file.Close() != nil || os.Rename(name, store.path) != nil {
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	}
	directory, err := os.Open(filepath.Dir(store.path))
	if err != nil {
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return NotificationSnapshot{}, ErrNotificationsUnavailable
	}
	return snapshot, nil
}

func validNotificationSnapshot(snapshot NotificationSnapshot) bool {
	if snapshot.Schema != 1 || snapshot.Terminals == nil {
		return false
	}
	keys := map[TerminalKey]bool{}
	for _, record := range snapshot.Terminals {
		if _, err := machine.Parse(record.Key.Machine); err != nil {
			return false
		}
		if !tmuxAddress(record.Key.TmuxID, '$') || !tmuxAddress(record.Key.PaneID, '%') || record.Key.IdentityToken == "" || keys[record.Key] || record.Revision < 0 {
			return false
		}
		if foreground := record.Foreground; foreground != nil && (foreground.Provider != "Codex" && foreground.Provider != "Claude" || foreground.PID <= 0 || foreground.StartIdentity == "") {
			return false
		}
		if record.Pending && (record.Foreground == nil || record.BaselinePending) {
			return false
		}
		keys[record.Key] = true
	}
	return true
}
