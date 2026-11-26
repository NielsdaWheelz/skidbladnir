package sessions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

const (
	ErrorBranchConfirmationRequired ErrorCode = "BranchConfirmationRequired"
	ErrorBranchChanged              ErrorCode = "BranchChanged"
	ErrorBranchUnresolved           ErrorCode = "BranchUnresolved"
)

func branchChanged() error {
	return newSessionError(ErrorBranchChanged, "the branch changed. review its current sessions.")
}

func branchUnresolved() error {
	return newSessionError(ErrorBranchUnresolved, "branch ownership is unresolved. inspect the involved machines.")
}

func branchConfirmation() error {
	return newSessionError(ErrorBranchConfirmationRequired, "closing this branch requires confirmation.")
}

func ValidSessionRef(ref api.SessionRef) bool {
	_, err := machine.Parse(ref.Machine)
	_, lifetimeErr := parseLifetime(ref.TmuxID, ref.IdentityToken)
	return err == nil && lifetimeErr == nil
}

func ValidBranchID(id string) bool {
	return len(id) == 32 && strings.Trim(id, "0123456789abcdef") == ""
}

func equalParent(a, b *api.SessionRef) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (manager *Manager) inspectBranch(ctx context.Context, ref api.SessionRef, store branchStore, lookupPrevious bool) (api.BranchNode, error) {
	if lookupPrevious {
		if current, ok := store.Restored[ref.IdentityToken]; ok {
			ref = current
		}
	}
	server, err := parseLifetime(ref.TmuxID, ref.IdentityToken)
	if err != nil {
		return api.BranchNode{}, err
	}
	capture, err := manager.tmux.ReadSession(ctx, ref.TmuxID)
	if err != nil {
		return api.BranchNode{}, err
	}
	if capture.Server == (tmux.ServerIdentity{}) {
		// Some tmux versions reject the missing target before evaluating the guard.
		// A fresh bracketed inventory can still establish absence on an exact server.
		inventory, err := manager.tmux.ReadCurrentInventory(ctx, tmux.ServerIdentity{})
		if err != nil {
			return api.BranchNode{}, err
		}
		if inventory.Server == (tmux.ServerIdentity{}) {
			return api.BranchNode{}, branchUnresolved()
		}
		capture.Server = inventory.Server
		for _, row := range inventory.Rows {
			if row.ID == ref.TmuxID {
				capture.Rows = []tmux.InventoryRow{row}
				break
			}
		}
	}
	if manager.recovery.recipe != nil && manager.recovery.recipe.Attempted {
		return api.BranchNode{}, branchUnresolved()
	}
	if err := manager.requireManaged(ctx, tmux.ServerObservation{Identity: capture.Server, SessionCount: len(capture.Rows)}, true); err != nil {
		return api.BranchNode{}, err
	}
	if capture.Server != server {
		if _, mapped := store.Restored[ref.IdentityToken]; mapped {
			return api.BranchNode{}, branchChanged()
		}
		if proven, ok := store.Completed[server.SessionToken("$0")]; !ok || proven != capture.Server {
			return api.BranchNode{}, branchUnresolved()
		}
	}
	if capture.Server != server || len(capture.Rows) == 0 {
		// A missing target can still be resurrected by the last workspace recipe.
		// Require a healthy checkpoint for this server before retiring a remote edge.
		recipe := manager.recovery.recipe
		if manager.recovery.fact.State == api.RecoveryBroken || manager.recovery.fact.State == api.RecoveryDelayed ||
			recipe != nil && (recipe.Source.Server != capture.Server ||
				recipe.Source.Server == server && slices.ContainsFunc(recipe.Sessions, func(s tmux.WorkspaceSession) bool { return s.Key == ref.TmuxID })) {
			return api.BranchNode{}, branchUnresolved()
		}
		return api.BranchNode{}, sessionNotFound()
	}
	row := capture.Rows[0]
	family, err := decodeFamily(row.LocalSession.Family)
	if err != nil {
		return api.BranchNode{}, err
	}
	claims := store.claims(ref.IdentityToken)
	return api.BranchNode{Target: api.Target{TmuxID: row.ID, IdentityToken: ref.IdentityToken, PaneID: row.PaneID}, Name: row.Name, Parent: family.Parent, Previous: family.Previous, Children: append([]api.SessionRef{}, claims.Children...), Pending: len(claims.Pending) > 0}, nil
}

func (manager *Manager) Branch(ctx context.Context, id string, request api.BranchRequest) (any, error) {
	if err := manager.acquire(ctx); err != nil {
		return nil, err
	}
	locked := true
	defer func() {
		if locked {
			manager.release()
		}
	}()
	store, err := manager.readBranches()
	if err != nil {
		return nil, err
	}
	self := api.SessionRef{Machine: manager.recovery.machine.String(), TmuxID: id, IdentityToken: request.IdentityToken}
	claimsToken := request.IdentityToken
	if request.Action == "complete" {
		if current, ok := store.Restored[claimsToken]; ok {
			claimsToken = current.IdentityToken
		}
	}
	claims := store.claims(claimsToken)
	// These operations settle an existing exact-lifetime record. External
	// tmux removal between the effect and this acknowledgement is harmless.
	switch request.Action {
	case "complete":
		reserved, ok := claims.Pending[request.ID]
		if !ok {
			if request.Child == nil {
				return nil, manager.cancelUndelivered(store, claims, request.ID)
			}
			if slices.Contains(claims.Children, *request.Child) {
				return nil, nil
			}
			return nil, branchChanged()
		}
		if request.Child != nil {
			if request.Child.Machine != reserved || *request.Child == self {
				return nil, branchChanged()
			}
			if !slices.Contains(claims.Children, *request.Child) {
				claims.Children = append(claims.Children, *request.Child)
			}
		}
		delete(claims.Pending, request.ID)
		return nil, manager.saveBranches(store)
	case "release":
		if claims.Fence == nil || claims.Fence.ID != request.CloseID {
			if _, restored := store.Restored[claimsToken]; restored {
				return nil, nil
			}
			return nil, manager.cancelUndelivered(store, claims, request.CloseID)
		}
		if claims.Fence.Dispatching {
			return nil, branchChanged()
		}
		claims.Fence = nil
		return nil, manager.saveBranches(store)
	case "unlink":
		if claims.Fence != nil && (claims.Fence.ID != request.CloseID || claims.Fence.Dispatching) {
			return nil, branchChanged()
		}
		if i := slices.Index(claims.Children, *request.Child); i >= 0 {
			claims.Children = slices.Delete(claims.Children, i, i+1)
			return nil, manager.saveBranches(store)
		}
		return nil, nil
	}

	node, err := manager.inspectBranch(ctx, self, store, request.Action == "inspect")
	if err != nil {
		return nil, err
	}
	switch request.Action {
	case "inspect":
		return node, nil
	case "reserve":
		if slices.Contains(claims.Canceled, request.ID) || claims.Fence != nil {
			return nil, branchChanged()
		}
		if claims.Pending == nil {
			claims.Pending = map[string]string{}
		}
		if prior, ok := claims.Pending[request.ID]; ok && prior != request.Machine {
			return nil, branchChanged()
		}
		claims.Pending[request.ID] = request.Machine
		return api.BranchReservation{ID: request.ID}, manager.saveBranches(store)
	case "prepare":
		want := request.Expected
		if slices.Contains(claims.Canceled, request.CloseID) {
			return nil, branchChanged()
		}
		if node.Pending || want.Pending || !equalParent(node.Parent, want.Parent) || !slices.Equal(node.Children, want.Children) || node.Target.TmuxID != want.Target.TmuxID || node.Target.IdentityToken != want.Target.IdentityToken || request.Mode == "interrupt" && node.Target.PaneID != want.Target.PaneID {
			return nil, branchChanged()
		}
		fence := closeFence{ID: request.CloseID, Mode: request.Mode}
		if request.Mode == "interrupt" {
			fence.PaneID = node.Target.PaneID
		}
		if claims.Fence != nil {
			if *claims.Fence == fence {
				return nil, nil
			}
			return nil, branchChanged()
		}
		claims.Fence = &fence
		return nil, manager.saveBranches(store)
	case "promote":
		if claims.Fence != nil || !equalParent(node.Parent, request.Parent) {
			return nil, branchChanged()
		}
		server, row, err := manager.resolve(ctx, id, request.IdentityToken)
		if err != nil {
			return nil, err
		}
		family, err := decodeFamily(row.LocalSession.Family)
		if err != nil {
			return nil, err
		}
		family.Parent = nil
		family.Admission = ""
		family.Promoted = true
		applied, err := manager.tmux.SetSessionFamily(ctx, id, server, row.Format.Family, encodeFamily(family))
		if errors.Is(err, tmux.ErrDispatchUnknown) {
			return nil, ErrBranchWriteUnknown
		}
		if err != nil {
			return nil, err
		}
		if !applied {
			return nil, branchChanged()
		}
		claims.Promoted = true
		if err := manager.saveBranches(store); err != nil {
			return nil, err
		}
		manager.release()
		locked = false
		manager.checkpointAfterMutation(ctx, server)
		return nil, nil
	}
	return nil, branchChanged()
}

// BeginClose reserves the session before interrupt or attachment cleanup.
// Dispatching persists across a gateway crash so a lost write cannot be
// replayed or raced by a new admission.
func (manager *Manager) BeginClose(ctx context.Context, id, token, pane, mode, closeID string) (string, error) {
	if err := manager.acquire(ctx); err != nil {
		return "", err
	}
	defer manager.release()
	store, err := manager.readBranches()
	if err != nil {
		return "", err
	}
	node, err := manager.inspectBranch(ctx, api.SessionRef{Machine: manager.recovery.machine.String(), TmuxID: id, IdentityToken: token}, store, false)
	if err != nil {
		return "", err
	}
	claims := store.claims(token)
	if len(claims.Pending) > 0 {
		return "", branchUnresolved()
	}
	if len(claims.Children) > 0 {
		return "", branchConfirmation()
	}
	if pane != "" && pane != node.Target.PaneID {
		return "", ErrTerminalTargetChanged
	}
	if closeID != "" {
		if claims.Fence == nil || claims.Fence.ID != closeID || claims.Fence.Mode != mode || claims.Fence.PaneID != pane || claims.Fence.Dispatching {
			return "", branchChanged()
		}
	} else {
		if claims.Fence != nil {
			return "", branchChanged()
		}
		var random [16]byte
		rand.Read(random[:])
		closeID = hex.EncodeToString(random[:])
		claims.Fence = &closeFence{ID: closeID, Mode: mode, PaneID: pane}
	}
	claims.Fence.Dispatching = true
	if err := manager.saveBranches(store); err != nil {
		if errors.Is(err, ErrBranchWriteUnknown) {
			return "", branchUnresolved()
		}
		return "", err
	}
	return closeID, nil
}

// FinishClose retires the barrier only when the gateway has finished its
// dispatch. It never settles a possibly dispatched tmux deletion.
func (manager *Manager) FinishClose(ctx context.Context, token, closeID string, closed bool) error {
	if err := manager.acquire(ctx); err != nil {
		return err
	}
	defer manager.release()
	store, err := manager.readBranches()
	if err != nil {
		return err
	}
	claims := store.claims(token)
	if claims.Fence == nil || claims.Fence.ID != closeID {
		return branchChanged()
	}
	if closed {
		delete(store.Nodes, token)
	} else {
		claims.Fence = nil
	}
	return manager.saveBranches(store)
}

// A cleanup sent after losing a reply can reach this gateway before the
// original reservation or preparation. Remember that cancellation until the
// session ends so later delivery cannot strand a pending child or close fence.
func (manager *Manager) cancelUndelivered(store branchStore, claims *branchClaims, id string) error {
	if slices.Contains(claims.Canceled, id) {
		return nil
	}
	claims.Canceled = append(claims.Canceled, id)
	return manager.saveBranches(store)
}
