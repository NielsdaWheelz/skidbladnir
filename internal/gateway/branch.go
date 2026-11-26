package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

func (g *Gateway) sessionBranch(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[api.BranchRequest](w, r)
	if !ok {
		return
	}
	id := r.PathValue("tmuxId")
	ref := api.SessionRef{Machine: g.machine.String(), TmuxID: id, IdentityToken: req.IdentityToken}
	if !sessions.ValidSessionRef(ref) || !validBranchRequest(req) {
		writeError(w, errorInvalidRequest)
		return
	}
	result, err := g.sessions.Branch(r.Context(), id, req)
	if err != nil {
		writeError(w, branchFailure(err))
		return
	}
	if result == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func validBranchRequest(req api.BranchRequest) bool {
	if req.ID != "" && !sessions.ValidBranchID(req.ID) || req.CloseID != "" && !sessions.ValidBranchID(req.CloseID) ||
		req.Parent != nil && !sessions.ValidSessionRef(*req.Parent) || req.Child != nil && !sessions.ValidSessionRef(*req.Child) {
		return false
	}
	fields := api.BranchRequest{Action: req.Action, IdentityToken: req.IdentityToken}
	switch req.Action {
	case "inspect":
	case "reserve":
		if _, err := machine.Parse(req.Machine); err != nil || req.ID == "" {
			return false
		}
		fields.Machine, fields.ID = req.Machine, req.ID
	case "complete":
		if req.ID == "" {
			return false
		}
		fields.ID, fields.Child = req.ID, req.Child
	case "prepare":
		if req.CloseID == "" || req.Mode != "interrupt" && req.Mode != "terminal" || req.Expected == nil {
			return false
		}
		if req.Mode == "interrupt" && !tmux.ValidID(req.Expected.Target.PaneID, '%') {
			return false
		}
		fields.CloseID, fields.Mode, fields.Expected = req.CloseID, req.Mode, req.Expected
	case "release":
		if req.CloseID == "" {
			return false
		}
		fields.CloseID = req.CloseID
	case "promote":
		if req.Parent == nil {
			return false
		}
		fields.Parent = req.Parent
	case "unlink":
		if req.Child == nil {
			return false
		}
		fields.Child, fields.CloseID = req.Child, req.CloseID
	default:
		return false
	}
	return fields == req
}

func branchFailure(err error) apiError {
	if errors.Is(err, sessions.ErrBranchWriteUnknown) {
		e := errorInternal
		e.Dispatch = "unknown"
		return e
	}
	return sessionFailure(err)
}

// finishClose can outlive a disconnected HTTP request. Once tmux has replied,
// cleanup must preserve its established outcome.
func (g *Gateway) finishClose(ctx context.Context, token, closeID string, closed bool) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := g.sessions.FinishClose(cleanup, token, closeID, closed); err != nil {
		g.logger.Error("Session.CloseFenceCleanupFailed")
	}
}
