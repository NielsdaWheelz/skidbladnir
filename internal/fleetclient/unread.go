package fleetclient

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

var ErrUnreadUnavailable = errors.New("unread unavailable")

type UnreadKey struct {
	Machine        string                `json:"machine"`
	Provider       agentruntime.Provider `json:"provider"`
	HistoryScope   string                `json:"historyScope"`
	ConversationID string                `json:"conversationId"`
}

func ReplyKey(machine string, conversation agentruntime.Conversation) UnreadKey {
	return UnreadKey{machine, conversation.Provider, conversation.HistoryScope, conversation.ConversationID}
}

type UnreadRecord struct {
	Key             UnreadKey `json:"key"`
	AcknowledgedIDs []string  `json:"acknowledgedIds"`
	UnreadIDs       []string  `json:"unreadIds"`
}
type UnreadAssociation struct {
	Machine       string                    `json:"machine"`
	TmuxID        string                    `json:"tmuxId"`
	IdentityToken string                    `json:"identityToken"`
	PaneID        string                    `json:"paneId"`
	Conversation  agentruntime.Conversation `json:"conversation"`
}
type UnreadSnapshot struct {
	Schema        int                 `json:"schema"`
	Conversations []UnreadRecord      `json:"conversations"`
	Associations  []UnreadAssociation `json:"associations"`
}

func (snapshot UnreadSnapshot) Record(key UnreadKey) (UnreadRecord, bool) {
	for _, record := range snapshot.Conversations {
		if record.Key == key {
			return record, true
		}
	}
	return UnreadRecord{}, false
}
func (snapshot UnreadSnapshot) Conversation(ref Reference, session Session) (agentruntime.Conversation, bool) {
	if ref.Conversation != nil {
		return ref.Conversation.Binding.Conversation, true
	}
	if session.Conversation != nil {
		return *session.Conversation, true
	}
	if session.Agent != nil {
		return agentruntime.Conversation{}, false
	}
	for _, association := range snapshot.Associations {
		if association.Machine == ref.Machine && association.TmuxID == ref.TmuxID && association.IdentityToken == ref.IdentityToken && association.PaneID == session.ActivePaneID && association.Conversation.Provider == agentruntime.ProviderClaude {
			return association.Conversation, true
		}
	}
	return agentruntime.Conversation{}, false
}

type UnreadStore struct{ path string }

func NewUnreadStore(path string) (*UnreadStore, error) {
	if !filepath.IsAbs(path) {
		return nil, ErrUnreadUnavailable
	}
	return &UnreadStore{path: path}, nil
}
func (store *UnreadStore) Read() (UnreadSnapshot, error) {
	return store.merge(func(*UnreadSnapshot) bool { return false })
}

func (store *UnreadStore) Observe(key UnreadKey, ids []string, baseline bool) (UnreadSnapshot, error) {
	return store.merge(func(snapshot *UnreadSnapshot) bool {
		for index := range snapshot.Conversations {
			record := &snapshot.Conversations[index]
			if record.Key != key {
				continue
			}
			for _, id := range ids {
				if !slices.Contains(record.AcknowledgedIDs, id) && !slices.Contains(record.UnreadIDs, id) {
					record.UnreadIDs = append(record.UnreadIDs, id)
				}
			}
			return true
		}
		if baseline {
			snapshot.Conversations = append(snapshot.Conversations, UnreadRecord{Key: key, AcknowledgedIDs: uniqueIDs(ids), UnreadIDs: []string{}})
			return true
		}
		return false
	})
}
func (store *UnreadStore) Acknowledge(key UnreadKey, ids []string) (UnreadSnapshot, error) {
	return store.merge(func(snapshot *UnreadSnapshot) bool {
		for index := range snapshot.Conversations {
			record := &snapshot.Conversations[index]
			if record.Key != key {
				continue
			}
			for _, id := range ids {
				if !slices.Contains(record.UnreadIDs, id) {
					continue
				}
				record.UnreadIDs = slices.DeleteFunc(record.UnreadIDs, func(value string) bool { return value == id })
				if !slices.Contains(record.AcknowledgedIDs, id) {
					record.AcknowledgedIDs = append(record.AcknowledgedIDs, id)
				}
			}
			return true
		}
		return false
	})
}
func uniqueIDs(ids []string) []string {
	result := []string{}
	for _, id := range ids {
		if !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	return result
}

// Sync replaces exact known bindings and retires deleted terminal associations.
// Unavailable hosts retain their associations. Conversation membership survives
// terminal deletion and is removed only with the configured machine.
func (store *UnreadStore) Sync(peers []Peer, machines []Machine) (UnreadSnapshot, error) {
	return store.merge(func(snapshot *UnreadSnapshot) bool {
		configured := func(handle string) bool {
			for _, machine := range machines {
				if machine.Handle == handle {
					return true
				}
			}
			return false
		}
		snapshot.Conversations = slices.DeleteFunc(snapshot.Conversations, func(record UnreadRecord) bool { return !configured(record.Key.Machine) })
		snapshot.Associations = slices.DeleteFunc(snapshot.Associations, func(association UnreadAssociation) bool {
			if !configured(association.Machine) {
				return true
			}
			for _, peer := range peers {
				if peer.Machine != association.Machine || !peer.OK {
					continue
				}
				for _, session := range peer.Sessions {
					ref, _ := DecodeReference(session.Ref)
					if ref.TmuxID == association.TmuxID && ref.IdentityToken == association.IdentityToken {
						return false
					}
				}
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
				if session.Conversation == nil || session.Conversation.Provider != agentruntime.ProviderClaude {
					continue
				}
				association := UnreadAssociation{Machine: peer.Machine, TmuxID: ref.TmuxID, IdentityToken: ref.IdentityToken, PaneID: session.ActivePaneID, Conversation: *session.Conversation}
				found := false
				for index, previous := range snapshot.Associations {
					if previous.Machine == association.Machine && previous.TmuxID == association.TmuxID && previous.IdentityToken == association.IdentityToken && previous.PaneID == association.PaneID {
						snapshot.Associations[index] = association
						found = true
						break
					}
				}
				if !found {
					snapshot.Associations = append(snapshot.Associations, association)
				}
			}
		}
		return true
	})
}

// A stable sidecar is locked across read–merge–atomic-replace. Locking the
// replaced data inode would let another process overwrite an acknowledgement.
func (store *UnreadStore) merge(update func(*UnreadSnapshot) bool) (snapshot UnreadSnapshot, resultErr error) {
	if err := os.MkdirAll(filepath.Dir(store.path), 0700); err != nil {
		return UnreadSnapshot{}, ErrUnreadUnavailable
	}
	lock, err := os.OpenFile(store.path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return UnreadSnapshot{}, ErrUnreadUnavailable
	}
	defer func() {
		if lock.Close() != nil {
			resultErr = ErrUnreadUnavailable
		}
	}()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return UnreadSnapshot{}, ErrUnreadUnavailable
	}
	defer func() {
		if syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) != nil {
			resultErr = ErrUnreadUnavailable
		}
	}()
	snapshot = UnreadSnapshot{Schema: 1, Conversations: []UnreadRecord{}, Associations: []UnreadAssociation{}}
	encoded, err := os.ReadFile(store.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return UnreadSnapshot{}, ErrUnreadUnavailable
	}
	if err == nil {
		var stored UnreadSnapshot
		if !nonNullJSON(encoded) || strictjson.Decode(encoded, &stored) != nil || !validUnreadSnapshot(stored) {
			return UnreadSnapshot{}, ErrUnreadUnavailable
		}
		snapshot = stored
	}
	if !update(&snapshot) {
		return snapshot, nil
	}
	encoded, err = json.Marshal(snapshot)
	if err != nil {
		return UnreadSnapshot{}, ErrUnreadUnavailable
	}
	file, err := os.CreateTemp(filepath.Dir(store.path), ".unread-*")
	if err != nil {
		return UnreadSnapshot{}, ErrUnreadUnavailable
	}
	name := file.Name()
	defer func() {
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = ErrUnreadUnavailable
		}
	}()
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return UnreadSnapshot{}, ErrUnreadUnavailable
	} // justify-ignore-error: failed write is primary; no replacement occurred.
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return UnreadSnapshot{}, ErrUnreadUnavailable
	} // justify-ignore-error: failed sync is primary; no replacement occurred.
	if file.Close() != nil || os.Rename(name, store.path) != nil {
		return UnreadSnapshot{}, ErrUnreadUnavailable
	}
	directory, err := os.Open(filepath.Dir(store.path))
	if err != nil {
		return UnreadSnapshot{}, ErrUnreadUnavailable
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return UnreadSnapshot{}, ErrUnreadUnavailable
	}
	return snapshot, nil
}

func validUnreadSnapshot(snapshot UnreadSnapshot) bool {
	if snapshot.Schema != 1 || snapshot.Conversations == nil || snapshot.Associations == nil {
		return false
	}
	keys := make(map[UnreadKey]bool, len(snapshot.Conversations))
	for _, record := range snapshot.Conversations {
		if _, err := machine.Parse(record.Key.Machine); err != nil {
			return false
		}
		scope, err := hex.DecodeString(record.Key.HistoryScope)
		if err != nil || len(scope) != 32 || strings.ToLower(record.Key.HistoryScope) != record.Key.HistoryScope || record.Key.Provider != agentruntime.ProviderCodex && record.Key.Provider != agentruntime.ProviderClaude || record.Key.ConversationID == "" || len(record.Key.ConversationID) > 128 || keys[record.Key] || record.AcknowledgedIDs == nil || record.UnreadIDs == nil {
			return false
		}
		keys[record.Key] = true
		ids := map[string]bool{}
		for _, list := range [][]string{record.AcknowledgedIDs, record.UnreadIDs} {
			for _, id := range list {
				if !validReplyID(id) || ids[id] {
					return false
				}
				ids[id] = true
			}
		}
	}
	associations := map[string]bool{}
	for _, association := range snapshot.Associations {
		if _, err := machine.Parse(association.Machine); err != nil {
			return false
		}
		if !association.Conversation.Valid() || !tmuxAddress(association.TmuxID, '$') || !tmuxAddress(association.PaneID, '%') || association.IdentityToken == "" {
			return false
		}
		key := association.Machine + "\x00" + association.TmuxID + "\x00" + association.IdentityToken + "\x00" + association.PaneID
		if associations[key] {
			return false
		}
		associations[key] = true
	}
	return true
}

func DefaultUnreadStore() (*UnreadStore, error) {
	directory := os.Getenv("XDG_STATE_HOME")
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, ErrUnreadUnavailable
		}
		directory = filepath.Join(home, ".local", "state")
	}
	return NewUnreadStore(filepath.Join(directory, "skidbladnir", "unread.json"))
}

func validReplyID(id string) bool {
	return id != "" && len(id) <= 128 && utf8.ValidString(id) && strings.IndexFunc(id, unicode.IsControl) < 0
}
