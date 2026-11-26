package sessions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

type sessionFamily struct {
	Parent    *api.SessionRef  `json:"parent,omitempty"`
	Admission string           `json:"admission,omitempty"`
	Promoted  bool             `json:"promoted,omitempty"`
	Previous  []api.SessionRef `json:"previous,omitempty"`
}

func encodeFamily(family sessionFamily) string {
	data, _ := json.Marshal(family)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeFamily(encoded string) (sessionFamily, error) {
	var family sessionFamily
	if encoded == "" {
		return family, nil
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || !json.Valid(data) {
		return family, branchUnresolved()
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(&family) != nil {
		return sessionFamily{}, branchUnresolved()
	}
	if family.Parent != nil {
		if family.Promoted || !ValidSessionRef(*family.Parent) || !ValidBranchID(family.Admission) {
			return sessionFamily{}, branchUnresolved()
		}
	} else if family.Admission != "" || !family.Promoted && len(family.Previous) == 0 {
		return sessionFamily{}, branchUnresolved()
	}
	for _, previous := range family.Previous {
		if !ValidSessionRef(previous) {
			return sessionFamily{}, branchUnresolved()
		}
	}
	return family, nil
}

type closeFence struct {
	ID          string `json:"id"`
	Mode        string `json:"mode"`
	PaneID      string `json:"paneId,omitempty"`
	Dispatching bool   `json:"dispatching,omitempty"`
}

type branchClaims struct {
	Canceled []string          `json:"canceled,omitempty"`
	Promoted bool              `json:"promoted,omitempty"`
	Children []api.SessionRef  `json:"children,omitempty"`
	Pending  map[string]string `json:"pending,omitempty"`
	Fence    *closeFence       `json:"fence,omitempty"`
}

// A parent owns its admissions even if a remote child is offline. Keep them
// independent of the periodically captured workspace recipe. Recovery moves
// these records only across its verified session mapping.
type branchStore struct {
	Completed map[string]tmux.ServerIdentity `json:"completed,omitempty"`
	Nodes     map[string]*branchClaims       `json:"nodes"`
	Restored  map[string]api.SessionRef      `json:"restored,omitempty"`
}

func (manager *Manager) branchFile() string {
	return filepath.Join(filepath.Dir(manager.recovery.path), "session-branches-"+manager.recovery.machine.String()+".json")
}

func (manager *Manager) readBranches() (branchStore, error) {
	var store branchStore
	data, err := privatefile.Read(manager.branchFile(), 8<<20)
	if errors.Is(err, os.ErrNotExist) {
		return branchStore{Nodes: map[string]*branchClaims{}}, nil
	}
	if err != nil {
		return branchStore{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if !json.Valid(data) || decoder.Decode(&store) != nil || store.Nodes == nil {
		return branchStore{}, branchUnresolved()
	}
	for token, node := range store.Nodes {
		if token == "" || node == nil {
			return branchStore{}, branchUnresolved()
		}
		for _, child := range node.Children {
			if !ValidSessionRef(child) {
				return branchStore{}, branchUnresolved()
			}
		}
		for id, machineID := range node.Pending {
			if _, err := machine.Parse(machineID); err != nil || !ValidBranchID(id) {
				return branchStore{}, branchUnresolved()
			}
		}
		if fence := node.Fence; fence != nil && (!ValidBranchID(fence.ID) || fence.Mode != "terminal" && fence.Mode != "interrupt") {
			return branchStore{}, branchUnresolved()
		}
	}
	for _, ref := range store.Restored {
		if !ValidSessionRef(ref) {
			return branchStore{}, branchUnresolved()
		}
	}
	return store, nil
}

func (manager *Manager) saveBranches(store branchStore) error {
	for key, n := range store.Nodes {
		if len(n.Canceled) == 0 && !n.Promoted && n.Fence == nil && len(n.Pending) == 0 && len(n.Children) == 0 {
			delete(store.Nodes, key)
		}
	}
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	if len(data) > 8<<20 {
		return branchUnresolved()
	}
	if err = privatefile.WriteFile(manager.branchFile(), data); err != nil {
		return errors.Join(ErrBranchWriteUnknown, err)
	}
	return nil
}

var ErrBranchWriteUnknown = errors.New("branch mutation completion is unknown")

func (store branchStore) claims(token string) *branchClaims {
	n := store.Nodes[token]
	if n == nil {
		n = &branchClaims{}
		store.Nodes[token] = n
	}
	return n
}

// RestoreWorkspace proved the native id map with a fresh capture. A recovery
// never carries an old close confirmation into a replacement lifetime.
func (manager *Manager) restoreBranches(ctx context.Context, source checkpoint, destination tmux.ServerIdentity, ids map[string]string) error {
	store, err := manager.readBranches()
	if err != nil {
		return err
	}
	if store.Restored == nil {
		store.Restored = map[string]api.SessionRef{}
	}
	if store.Completed == nil {
		store.Completed = map[string]tmux.ServerIdentity{}
	}
	local := manager.recovery.machine.String()
	for oldID, newID := range ids {
		oldToken := source.Source.Server.SessionToken(oldID)
		current := api.SessionRef{Machine: local, TmuxID: newID, IdentityToken: destination.SessionToken(newID)}
		store.Restored[oldToken] = current
		if claims := store.Nodes[oldToken]; claims != nil {
			claims.Fence = nil
			claims.Canceled = nil
			store.Nodes[current.IdentityToken] = claims
			delete(store.Nodes, oldToken)
		}
	}
	for token, ref := range store.Restored {
		if current, found := store.Restored[ref.IdentityToken]; found {
			store.Restored[token] = current
		}
	}
	for _, claims := range store.Nodes {
		for index, child := range claims.Children {
			if child.Machine == local {
				if current, found := store.Restored[child.IdentityToken]; found {
					claims.Children[index] = current
				}
			}
		}
	}
	for old, previous := range store.Completed {
		if previous == source.Source.Server {
			store.Completed[old] = destination
		}
	}
	store.Completed[source.Source.Server.SessionToken("$0")] = destination
	for _, s := range source.Sessions {
		previous := api.SessionRef{Machine: local, TmuxID: s.Key, IdentityToken: source.Source.Server.SessionToken(s.Key)}
		current := store.Restored[previous.IdentityToken]
		family, err := decodeFamily(s.Family)
		if err != nil {
			return err
		}
		family.Previous = append(family.Previous, previous)
		if claims := store.Nodes[current.IdentityToken]; claims != nil && claims.Promoted {
			family.Parent = nil
			family.Admission = ""
			family.Promoted = true
		}
		if p := family.Parent; p != nil && p.Machine == local {
			if mapped, ok := store.Restored[p.IdentityToken]; ok {
				family.Parent = &mapped
			}
		}
		applied, err := manager.tmux.SetSessionFamily(ctx, current.TmuxID, destination, s.Family, encodeFamily(family))
		if err != nil {
			return err
		}
		if !applied {
			return branchChanged()
		}
	}
	return manager.saveBranches(store)
}
