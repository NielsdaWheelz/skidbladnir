// Package jobs owns the host's dispatch receipts and delivery of worker answers.
// A gateway is the sole writer of receipts. Each worker owns its own result files.
package jobs

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/managedworker"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
	"github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var (
	ErrInvalid      = errors.New("invalid job")
	ErrConflict     = errors.New("job id belongs to another request")
	ErrNotFound     = errors.New("job not found")
	ErrPending      = errors.New("job pending")
	ErrAcknowledged = errors.New("job acknowledged")
)

type Store struct {
	sessions *sessions.Manager
	dir      string
	sockets  string

	mu          sync.Mutex // serializes durable receipt transitions only
	changed     chan struct{}
	dispatching map[string]struct{}
}

func New(stateDir string, manager *sessions.Manager) *Store {
	return &Store{
		sessions:    manager,
		dir:         filepath.Join(stateDir, "jobs"),
		sockets:     filepath.Join(stateDir, "j"),
		changed:     make(chan struct{}),
		dispatching: make(map[string]struct{}),
	}
}

type lifetime struct {
	PID   process.PID           `json:"pid"`
	Start process.StartIdentity `json:"start"`
}

type record struct {
	api.JobReceipt
	Digest string    `json:"digest"`
	Worker string    `json:"worker"`
	Owner  *lifetime `json:"owner,omitempty"`
	Agent  *lifetime `json:"agent,omitempty"`
	Acked  bool      `json:"acked,omitempty"`
}

type outcome struct {
	State  api.JobState `json:"state"`
	Reason string       `json:"reason"`
	Length int          `json:"length"`
	SHA256 string       `json:"sha256"`
	file   *os.File
	body   int64
}

func (result outcome) close() {
	if result.file != nil {
		_ = result.file.Close() // a read-only descriptor; no payload is committed here
	}
}

func Validate(id string, request api.JobRequest) error {
	if _, ok := api.JobIDBytes(id); !ok || request.Brief == "" || len(request.Brief) > api.JobPageSize ||
		!utf8.ValidString(request.Brief) || strings.ContainsRune(request.Brief, 0) {
		return ErrInvalid
	}
	if request.After == "" {
		if request.Profile == "" || request.CWD == "" {
			return ErrInvalid
		}
		return nil
	}
	if _, ok := api.JobIDBytes(request.After); !ok || id == request.After || request.Profile != "" || request.CWD != "" || request.Model != nil || request.Effort != nil {
		return ErrInvalid
	}
	return nil
}

func (store *Store) socket(id string) string {
	data, _ := api.JobIDBytes(id) // request and receipt owners admit canonical job IDs
	return filepath.Join(store.sockets, base64.RawURLEncoding.EncodeToString(data))
}

func (store *Store) read(id string) (record, error) {
	data, err := privatefile.Read(filepath.Join(store.dir, id+".json"), 8192)
	if errors.Is(err, os.ErrNotExist) {
		return record{}, ErrNotFound
	}
	if err != nil {
		return record{}, err
	}
	var value record
	err = json.Unmarshal(data, &value)
	return value, err
}

// write holds mu so waiters subscribe before their next read of the record.
func (store *Store) write(value record) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	err = privatefile.WriteFile(filepath.Join(store.dir, value.JobID+".json"), data)
	close(store.changed)
	store.changed = make(chan struct{})
	return err
}

func (store *Store) reserve(id string, request api.JobRequest) (record, bool, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return record{}, false, err
	}
	digest := sha256.Sum256(body)
	digestText := hex.EncodeToString(digest[:])
	store.mu.Lock()
	defer store.mu.Unlock()
	existing, err := store.read(id)
	if err == nil {
		if existing.Digest != digestText {
			return record{}, false, ErrConflict
		}
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return record{}, false, err
	}
	owner, err := process.Observe(process.PID(os.Getpid()))
	if err != nil {
		return record{}, false, err
	}
	value := record{
		JobReceipt: api.JobReceipt{JobID: id, State: api.JobAccepted},
		Digest:     digestText,
		Worker:     id,
		Owner:      &lifetime{PID: owner.PID, Start: owner.StartIdentity},
	}
	if request.After != "" {
		prior, err := store.read(request.After)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return record{}, false, err
		}
		value.Worker, value.Terminal, value.Agent = prior.Worker, prior.Terminal, prior.Agent
	}
	if err := store.write(value); err != nil {
		return record{}, false, err
	}
	store.dispatching[id] = struct{}{}
	return value, true, nil
}

func (store *Store) Submit(ctx context.Context, id string, request api.JobRequest) (api.JobReceipt, error) {
	value, reserved, err := store.reserve(id, request)
	if err != nil {
		return api.JobReceipt{}, err
	}
	if !reserved {
		return value.JobReceipt, nil
	}
	// The reservation has committed. Finish this bounded dispatch even if its
	// client goes away, without launching any background operation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 12*time.Second)
	defer cancel()
	state, reason := store.dispatch(ctx, &value, request)
	value.State, value.Reason, value.Owner = state, reason, nil
	store.mu.Lock()
	delete(store.dispatching, id)
	err = store.write(value)
	store.mu.Unlock()
	return value.JobReceipt, err
}

func (store *Store) dispatch(ctx context.Context, value *record, request api.JobRequest) (api.JobState, string) {
	if request.After == "" {
		socket := store.socket(value.Worker)
		if len(socket) > 103 || os.MkdirAll(store.sockets, 0700) != nil {
			return api.JobFailed, "worker_socket_unavailable"
		}
		created, err := store.sessions.Create(ctx, sessions.CreateInput{
			Kind: sessions.LaunchAgent, CWD: request.CWD, Profile: request.Profile,
			Model: request.Model, Effort: request.Effort, WorkerSocket: socket, WorkerResults: store.dir,
		})
		if created.Session.TmuxID != "" {
			value.Terminal = &api.JobTerminal{
				TmuxID: created.Session.TmuxID, IdentityToken: created.Session.IdentityToken, PaneID: created.Session.ActivePaneID,
			}
		}
		var refused *sessions.Error
		switch {
		case errors.Is(err, sessions.ErrCreateDispatchUnknown):
			return api.JobFailed, "launch_unknown"
		case errors.As(err, &refused):
			return api.JobFailed, string(refused.Code)
		case err != nil:
			return api.JobFailed, "unavailable"
		}
		// Empty sockets only occur before this newly launched worker is ready.
		for {
			_, err = managedworker.Call(ctx, socket, managedworker.Request{Op: "status"})
			if err == nil {
				break
			}
			if !errors.Is(err, syscall.ENOENT) && !errors.Is(err, syscall.ECONNREFUSED) {
				return api.JobFailed, "unavailable"
			}
			select {
			case <-ctx.Done():
				return api.JobFailed, "unavailable"
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	if value.Terminal == nil {
		return api.JobFailed, "unavailable"
	}
	socket := store.socket(value.Worker)
	session, err := store.sessions.ResolveTerminal(ctx, sessions.TerminalTarget{
		TmuxID: value.Terminal.TmuxID, IdentityToken: value.Terminal.IdentityToken, PaneID: value.Terminal.PaneID,
	})
	if err != nil || session.Agent == nil || session.Agent.WorkerSocket != socket {
		return api.JobFailed, "unavailable"
	}
	exact := lifetime{PID: session.Agent.PID, Start: session.Agent.StartIdentity}
	if request.After != "" && (value.Agent == nil || *value.Agent != exact) {
		return api.JobFailed, "unavailable"
	}
	value.Agent = &exact
	// This ref is useful during work, including when the submit receipt is lost.
	store.mu.Lock()
	err = store.write(*value)
	store.mu.Unlock()
	if err != nil {
		return api.JobUnknown, "receipt_unavailable"
	}
	response, err := managedworker.Call(ctx, socket, managedworker.Request{Op: "submit", JobID: value.JobID, Brief: request.Brief})
	switch {
	case err != nil:
		// The worker may already have claimed the turn. Wait on that exact UUID.
		return api.JobAccepted, ""
	case response.State == string(api.JobAccepted):
		return api.JobAccepted, ""
	case response.State == string(api.JobFailed):
		return api.JobFailed, response.Reason
	default:
		return api.JobAccepted, ""
	}
}

func (store *Store) sealed(value record) (outcome, bool, error) {
	if value.State.Terminal() {
		empty := sha256.Sum256(nil)
		return outcome{State: value.State, Reason: value.Reason, SHA256: hex.EncodeToString(empty[:])}, true, nil
	}
	file, err := privatefile.Open(filepath.Join(store.dir, value.JobID+".result"))
	if errors.Is(err, os.ErrNotExist) {
		return outcome{}, false, nil
	}
	if err != nil {
		return outcome{}, false, err
	}
	reader := bufio.NewReaderSize(file, 1024)
	header, err := reader.ReadSlice('\n')
	if err != nil {
		return outcome{}, false, errors.Join(err, file.Close())
	}
	var result outcome
	if err := json.Unmarshal(header, &result); err != nil {
		return outcome{}, false, errors.Join(err, file.Close())
	}
	if !result.State.Terminal() || result.Length < 0 || result.Length > api.JobAnswerLimit {
		return outcome{}, false, errors.Join(errors.New("invalid worker result"), file.Close())
	}
	result.file, result.body = file, int64(len(header))
	return result, true, nil
}

func (store *Store) readWait(id string) (record, outcome, bool, <-chan struct{}, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, err := store.read(id)
	if err != nil {
		return record{}, outcome{}, false, nil, err
	}
	if value.Acked {
		return record{}, outcome{}, false, nil, ErrAcknowledged
	}
	if _, active := store.dispatching[id]; active {
		return value, outcome{}, false, store.changed, nil
	}
	result, sealed, err := store.sealed(value)
	return value, result, sealed, nil, err
}

func (store *Store) Result(ctx context.Context, id string, offset int) (api.JobResult, error) {
	for {
		value, result, sealed, changed, err := store.readWait(id)
		if err != nil {
			return api.JobResult{}, err
		}
		if sealed {
			defer result.close()
			if offset < 0 || offset > result.Length || offset%api.JobPageSize != 0 {
				return api.JobResult{}, ErrInvalid
			}
			page := make([]byte, min(api.JobPageSize, result.Length-offset))
			if len(page) != 0 {
				if _, err := result.file.ReadAt(page, result.body+int64(offset)); err != nil {
					return api.JobResult{}, err
				}
			}
			value.State, value.Reason = result.State, result.Reason
			return api.JobResult{
				JobReceipt: value.JobReceipt, Length: result.Length, SHA256: result.SHA256,
				Offset: offset, Page: page,
			}, nil
		}
		if changed != nil {
			select {
			case <-ctx.Done():
				return api.JobResult{}, ErrPending
			case <-changed:
				continue
			}
		}
		if value.Owner != nil && value.Owner.PID != process.PID(os.Getpid()) {
			previous, err := process.Observe(value.Owner.PID)
			if err != nil && !errors.Is(err, process.ErrProcessAbsent) {
				return api.JobResult{}, err
			}
			if err == nil && previous.StartIdentity == value.Owner.Start {
				return api.JobResult{}, ErrPending
			}
		}
		workerStillLive := false
		if value.Agent != nil {
			// The worker registers this waiter before checking the file, and
			// signals after replacing it. Closure also requires a fresh read.
			response, err := managedworker.Call(ctx, store.socket(value.Worker), managedworker.Request{Op: "wait", JobID: id})
			if ctx.Err() != nil {
				return api.JobResult{}, ErrPending
			}
			if err == nil && response.State == "ready" {
				continue
			}
			missing := err == nil && response.State == "missing"
			if !missing {
				if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.ECONNRESET) &&
					!errors.Is(err, syscall.ENOENT) && !errors.Is(err, syscall.ECONNREFUSED) {
					return api.JobResult{}, errors.New("worker wait unavailable")
				}
				current, err := process.Observe(value.Agent.PID)
				if err != nil && !errors.Is(err, process.ErrProcessAbsent) {
					return api.JobResult{}, err
				}
				workerStillLive = err == nil && current.StartIdentity == value.Agent.Start
			}
		}
		if err := store.resolveLoss(id, workerStillLive); err != nil {
			return api.JobResult{}, err
		}
	}
}

func (store *Store) resolveLoss(id string, workerStillLive bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, err := store.read(id)
	if err != nil {
		return err
	}
	result, sealed, err := store.sealed(value)
	result.close()
	if err != nil || sealed {
		return err
	}
	if workerStillLive {
		// A closed socket alone cannot settle a live worker's assignment.
		return ErrPending
	}
	value.State, value.Reason, value.Owner = api.JobUnknown, "worker_outcome_unavailable", nil
	return store.write(value)
}

func (store *Store) Ack(id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, err := store.read(id)
	if err != nil {
		return err
	}
	if _, active := store.dispatching[id]; active {
		return ErrPending
	}
	if !value.Acked {
		result, sealed, err := store.sealed(value)
		result.close()
		if err != nil {
			return err
		}
		if !sealed {
			return ErrPending
		}
		value.State, value.Reason, value.Owner, value.Acked = result.State, result.Reason, nil, true
		if err := store.write(value); err != nil {
			return err
		}
	}
	return privatefile.Remove(filepath.Join(store.dir, id+".result"))
}
