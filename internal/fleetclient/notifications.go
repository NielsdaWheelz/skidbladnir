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
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
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

type NotificationState string

const (
	NotificationQuiet NotificationState = "quiet"
	NotificationArmed NotificationState = "armed"
	NotificationReady NotificationState = "ready"
)

type NotificationRecord struct {
	Key        TerminalKey       `json:"key"`
	Revision   int64             `json:"revision"`
	Foreground *Foreground       `json:"foreground,omitempty"`
	State      NotificationState `json:"state"`
}

func (record *NotificationRecord) UnmarshalJSON(encoded []byte) error {
	var value struct {
		Key        TerminalKey        `json:"key"`
		Revision   *int64             `json:"revision"`
		Foreground *Foreground        `json:"foreground,omitempty"`
		State      *NotificationState `json:"state"`
	}
	if strictjson.Decode(encoded, &value) != nil || value.Revision == nil || value.State == nil {
		return ErrNotificationsUnavailable
	}
	*record = NotificationRecord{value.Key, *value.Revision, value.Foreground, *value.State}
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
	return NewNotificationStore(filepath.Join(directory, "skidbladnir", "notifications-v2.json"))
}
func (store *NotificationStore) Read() (NotificationSnapshot, error) {
	return store.merge(func(*NotificationSnapshot) (bool, error) { return false, nil })
}

// Observe commits one inventory batch. expected is read before network dispatch.
// Conflicting observations are discarded, never rebased onto another client's work.
func (store *NotificationStore) Observe(peers []Peer, machines []Machine, expected NotificationSnapshot) (NotificationSnapshot, error) {
	return store.observe(peers, machines, expected, true)
}

// ObserveSession admits one fresh scoped terminal observation without retiring unrelated sessions.
func (store *NotificationStore) ObserveSession(session Session, machines []Machine, expected NotificationSnapshot) (NotificationSnapshot, error) {
	ref, _ := DecodeReference(session.Ref)
	return store.observe([]Peer{{Machine: ref.Machine, OK: true, Sessions: []Session{session}}}, machines, expected, false)
}

func (store *NotificationStore) observe(peers []Peer, machines []Machine, expected NotificationSnapshot, inventory bool) (NotificationSnapshot, error) {
	return store.merge(func(snapshot *NotificationSnapshot) (bool, error) {
		changed := false
		snapshot.Terminals = slices.DeleteFunc(snapshot.Terminals, func(record NotificationRecord) bool {
			if !slices.ContainsFunc(machines, func(machine Machine) bool { return machine.Handle == record.Key.Machine }) {
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
				// Selection is one session observation: either admit the selected pane
				// and every sibling fence, or discard the whole sample.
				selectionCurrent := true
				for _, sibling := range snapshot.Terminals {
					if sibling.Key.Machine != key.Machine || sibling.Key.TmuxID != key.TmuxID || sibling.Key.IdentityToken != key.IdentityToken || sibling.Key.PaneID == key.PaneID {
						continue
					}
					before, found := expected.Record(sibling.Key)
					if !found || before.Revision != sibling.Revision {
						selectionCurrent = false
						break
					}
				}
				for _, sibling := range expected.Terminals {
					if sibling.Key.Machine == key.Machine && sibling.Key.TmuxID == key.TmuxID && sibling.Key.IdentityToken == key.IdentityToken && sibling.Key.PaneID != key.PaneID {
						if _, found := snapshot.Record(sibling.Key); !found {
							selectionCurrent = false
							break
						}
					}
				}
				if !selectionCurrent {
					continue
				}
				if previous.Revision == math.MaxInt64 {
					return false, ErrNotificationsUnavailable
				}
				// A changed selection clears the former pane but retains its revision fence.
				for i := range snapshot.Terminals {
					old := &snapshot.Terminals[i]
					if old.Key.Machine != key.Machine || old.Key.TmuxID != key.TmuxID || old.Key.IdentityToken != key.IdentityToken || old.Key.PaneID == key.PaneID {
						continue
					}
					if old.Revision == math.MaxInt64 {
						return false, ErrNotificationsUnavailable
					}
					old.State, old.Foreground = NotificationQuiet, nil
					old.Revision++
					changed = true
				}
				record := previous
				record.Key = key
				if !present {
					record.State = NotificationQuiet
				}
				status := session.TerminalStatus
				var foreground *Foreground
				if session.Agent != nil && session.Connection == nil {
					foreground = &Foreground{session.Agent.Provider, session.Agent.PID, session.Agent.StartIdentity}
				}
				same := foreground != nil && previous.Foreground != nil && *foreground == *previous.Foreground
				// Missing identity in a failed observation supplies no exit evidence.
				// A captured shell, local process or remote transport does.
				positive := status.Source == sessions.SourceTerminal || foreground != nil || session.Connection != nil
				if positive && !same {
					record.State, record.Foreground = NotificationQuiet, foreground
				}
				record.Revision++
				if foreground != nil && status.Source == sessions.SourceTerminal {
					switch {
					case status.Interaction.Request() || status.Interaction == sessions.InteractionMenu || status.Activity == sessions.ActivityStarting || status.Activity == sessions.ActivityWorking:
						record.State = NotificationArmed
					case status.Activity == sessions.ActivityIdle && status.Interaction == sessions.InteractionNone && record.State == NotificationArmed:
						record.State = NotificationReady
					}
				}
				if index < 0 {
					snapshot.Terminals = append(snapshot.Terminals, record)
				} else {
					snapshot.Terminals[index] = record
				}
				changed = true
			}
		}
		return changed, nil
	})
}

func (store *NotificationStore) Presented(key TerminalKey) (NotificationSnapshot, error) {
	return store.visit(key, true)
}
func (store *NotificationStore) EndVisit(key TerminalKey) (NotificationSnapshot, error) {
	return store.visit(key, false)
}
func (store *NotificationStore) visit(key TerminalKey, presented bool) (NotificationSnapshot, error) {
	return store.merge(func(snapshot *NotificationSnapshot) (bool, error) {
		index := slices.IndexFunc(snapshot.Terminals, func(record NotificationRecord) bool { return record.Key == key })
		if index < 0 {
			snapshot.Terminals = append(snapshot.Terminals, NotificationRecord{Key: key, State: NotificationQuiet})
			index = len(snapshot.Terminals) - 1
		}
		record := &snapshot.Terminals[index]
		if record.Revision == math.MaxInt64 {
			return false, ErrNotificationsUnavailable
		}
		if presented && record.State == NotificationReady {
			record.State = NotificationQuiet
		}
		record.Revision++
		return true, nil
	})
}

// Ready reports this session's committed attention. The stored
// foreground must equal the observed one, so a replacement agent cannot show
// its predecessor's attention before the store update commits.
func (snapshot NotificationSnapshot) Ready(session Session) bool {
	if session.Agent == nil || session.Connection != nil {
		return false
	}
	ref, _ := DecodeReference(session.Ref)
	record, found := snapshot.Record(NotificationKey(ref))
	foreground := Foreground{session.Agent.Provider, session.Agent.PID, session.Agent.StartIdentity}
	return found && record.State == NotificationReady && record.Foreground != nil && *record.Foreground == foreground
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
	snapshot = NotificationSnapshot{Schema: 2, Terminals: []NotificationRecord{}}
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
	if snapshot.Schema != 2 || snapshot.Terminals == nil {
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
		switch record.State {
		case NotificationQuiet:
		case NotificationArmed, NotificationReady:
			if record.Foreground == nil {
				return false
			}
		default:
			return false
		}
		keys[record.Key] = true
	}
	return true
}
