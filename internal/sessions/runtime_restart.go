package sessions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

// RuntimeRestart holds the checkpoint writer while the installer rechecks and
// stops the explicitly selected server. It never admits a server for live use.
// The owner must have paused session creators and finished terminal work.
type RuntimeRestart struct {
	Process                                         process.Observation
	Sessions, Panes, AgentPanes, InferredAgentPanes int
	pending                                         bool
	manager                                         *Manager
	observed                                        tmux.ServerObservation
	present                                         bool
	recipe                                          checkpoint
	lock                                            *os.File
}

// PrepareRuntimeRestart validates a complete recoverable checkpoint after the
// gateway has stopped. A missing recipe or an uncertain prior claim refuses.
func PrepareRuntimeRestart(ctx context.Context, config Config, observed tmux.ServerObservation, present bool) (_ *RuntimeRestart, result error) {
	manager, err := New(config)
	if err != nil {
		return nil, err
	}
	lock, failure := manager.acquireRecoveryLock()
	if failure != nil {
		return nil, errors.New("the workspace checkpoint writer is still active")
	}
	restart := &RuntimeRestart{manager: manager, observed: observed, present: present, lock: lock}
	defer func() {
		if result != nil {
			result = errors.Join(result, restart.Close())
		}
	}()
	recipe, failure := manager.readCheckpoint()
	if failure != nil || recipe == nil {
		return nil, errors.New("a valid installed workspace checkpoint is required before restarting terminals")
	}
	if recipe.Attempted {
		return nil, errors.New("the workspace checkpoint has already been attempted; repair recovery before restarting terminals")
	}
	restart.recipe = *recipe
	if present {
		pid, err := strconv.Atoi(observed.Identity.PID)
		if err != nil {
			return nil, err
		}
		restart.Process, err = process.Observe(process.PID(pid))
		if err != nil {
			return nil, err
		}
		if err := RefuseRuntimeAncestor(ctx, restart.Process); err != nil {
			return nil, err
		}
	}
	gone, failure := sourceGone(recipe.Source)
	if failure != nil {
		return nil, errors.New("the workspace checkpoint source cannot be inspected")
	}
	restart.pending = gone
	fresh, err := restart.validate(ctx)
	if err != nil {
		return nil, err
	}
	restart.Sessions = len(recipe.Sessions)
	for _, window := range recipe.Windows {
		restart.Panes += len(window.Panes)
	}
	restart.AgentPanes = len(recipe.Agents)
	_, _, counts := planRecoveryStartups(recipe, manager.settings.Load())
	restart.InferredAgentPanes = counts.InferredPanes
	if !gone {
		if failure := manager.writeCheckpoint(fresh); failure != nil {
			return nil, errors.New("the validated workspace checkpoint could not be saved")
		}
		restart.recipe = fresh
	}
	return restart, nil
}

// RefuseRuntimeAncestor prevents an installer from terminating its own terminal
// before it can provision the replacement. Root launchd/systemd jobs are outside
// this ancestry; an inherited TMUX string alone is not a process witness.
func RefuseRuntimeAncestor(ctx context.Context, server process.Observation) error {
	before, err := process.Observe(server.PID)
	if err != nil || before.StartIdentity != server.StartIdentity {
		return errors.New("the selected terminal server changed during ancestry inspection")
	}
	pid := process.PID(os.Getpid())
	for pid > 1 {
		if pid == server.PID {
			return errors.New("the installer is running inside the terminal runtime it would stop; run it from an independent native job")
		}
		output, err := exec.CommandContext(ctx, "ps", "-o", "ppid=", "-p", strconv.Itoa(int(pid))).Output()
		if err != nil {
			return err
		}
		parent, err := strconv.Atoi(strings.TrimSpace(string(output)))
		if err != nil || parent < 0 || process.PID(parent) == pid {
			return errors.New("a parent process cannot be inspected during terminal restart")
		}
		pid = process.PID(parent)
	}
	after, err := process.Observe(server.PID)
	if err != nil || after.StartIdentity != server.StartIdentity {
		return errors.New("the selected terminal server changed during ancestry inspection")
	}
	return nil
}

// Recheck proves the selected kernel lifetime, complete graph and every resume
// target again immediately before the installer dispatches its guarded stop.
func (restart *RuntimeRestart) Recheck(ctx context.Context) error {
	_, err := restart.validate(ctx)
	return err
}

func (restart *RuntimeRestart) Close() error {
	if restart.lock == nil {
		return nil
	}
	lock := restart.lock
	restart.lock = nil
	return lock.Close()
}

func (restart *RuntimeRestart) validate(ctx context.Context) (checkpoint, error) {
	manager := restart.manager
	recipe := &restart.recipe
	observed, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		return checkpoint{}, err
	}
	if present != restart.present || present && observed.Identity != restart.observed.Identity {
		return checkpoint{}, errors.New("the selected terminal server lifetime changed")
	}
	if present {
		current, err := process.Observe(restart.Process.PID)
		if err != nil || current.StartIdentity != restart.Process.StartIdentity {
			return checkpoint{}, errors.New("the selected terminal server kernel identity changed")
		}
	}
	for _, window := range recipe.Windows {
		for _, pane := range window.Panes {
			parsed, err := manager.workdir.ParseStart(pane.CWD)
			if err != nil || parsed != pane.CWD || workdir.CheckStart(parsed) != nil {
				return checkpoint{}, errors.New("a checkpoint pane directory is unavailable for reconstruction")
			}
		}
	}
	_, _, counts := planRecoveryStartups(recipe, manager.settings.Load())
	if counts.SkippedAgentPanes != 0 || counts.RequestedPanes != len(recipe.Agents) {
		return checkpoint{}, errors.New("the workspace checkpoint cannot resume every saved agent with the current profiles")
	}
	if restart.pending {
		gone, failure := sourceGone(recipe.Source)
		if failure != nil || !gone || present && observed.SessionCount != 0 {
			return checkpoint{}, errors.New("pending workspace recovery requires a proved lost source and an empty destination")
		}
		return *recipe, nil
	}
	if !present || observed.Identity != recipe.Source.Server {
		return checkpoint{}, errors.New("the workspace checkpoint does not name the selected live terminal server")
	}
	ids := make([]string, len(recipe.Sessions))
	for index, session := range recipe.Sessions {
		ids[index] = session.Key
	}
	fresh, failure := manager.captureCheckpoint(ctx, observed.Identity, ids)
	if failure != nil || fresh.Source != recipe.Source {
		return checkpoint{}, errors.New("the workspace checkpoint source or graph changed")
	}
	if !sameRestartWorkspace(recipe.workspace(), fresh.workspace()) {
		return checkpoint{}, errors.New("the workspace checkpoint is stale; let the current gateway capture the unchanged workspace before restarting")
	}
	settings := manager.settings.Load()
	before := observeRecoveryAgents(ctx, fresh.workspace(), settings)
	foreground := make(map[string]process.Observation)
	for _, window := range fresh.Windows {
		for _, pane := range window.Panes {
			current, err := process.ObserveForeground(process.PID(pane.PID))
			if err != nil {
				return checkpoint{}, errors.New("a terminal foreground process cannot be inspected")
			}
			foreground[pane.Key] = current
			if _, recognized := agentruntime.Classify(current); !recognized {
				if _, saved := recipe.Agents[pane.Key]; saved {
					return checkpoint{}, errors.New("a saved agent is no longer the terminal foreground")
				}
				if current.ExecutableBase() != "bash" && current.ExecutableBase() != "zsh" {
					return checkpoint{}, errors.New("a terminal has unrecognized foreground work; finish it before restarting terminals")
				}
				continue
			}
			observation, found := before[pane.Key]
			saved, hasSaved := recipe.Agents[pane.Key]
			if !found || !hasSaved || saved.Resume == nil || observation.reason != "" || observation.foreground != current || observation.provider != saved.Provider || observation.profile != saved.Resume.Profile || observation.scope != saved.Resume.AccountScope {
				return checkpoint{}, errors.New("a terminal agent has no current, attributable resume target")
			}
		}
	}
	if len(before) != len(recipe.Agents) {
		return checkpoint{}, errors.New("the checkpoint does not cover every current terminal agent")
	}
	discovery, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	confirmed := discoverRecoveryConversations(discovery, settings.profiles, before)
	for _, found := range confirmed {
		saved := recipe.Agents[found.PaneKey]
		if found.Reason != "" || found.ID != saved.Resume.ID || found.Source != saved.Resume.Source {
			return checkpoint{}, errors.New("a saved conversation resume target cannot be freshly confirmed")
		}
	}
	final, failure := manager.captureCheckpoint(ctx, observed.Identity, ids)
	if failure != nil || final.Source != fresh.Source || !sameRestartWorkspace(fresh.workspace(), final.workspace()) {
		return checkpoint{}, errors.New("the terminal workspace changed during checkpoint validation")
	}
	after := observeRecoveryAgents(ctx, final.workspace(), settings)
	for key, first := range before {
		last, found := after[key]
		if !found || last.reason != "" || first.foreground != last.foreground || first.profile != last.profile || first.scope != last.scope || !maps.Equal(first.environment, last.environment) {
			return checkpoint{}, errors.New("a terminal agent changed during checkpoint validation")
		}
		result := confirmed[key]
		if result.Source == agentruntime.ConversationTitleMatch && !agentruntime.ConversationTitleMatches(first.provider, result, last.pane.Title) {
			return checkpoint{}, errors.New("a terminal conversation title changed during checkpoint validation")
		}
	}
	for _, window := range final.Windows {
		for _, pane := range window.Panes {
			current, err := process.ObserveForeground(process.PID(pane.PID))
			if err != nil || current != foreground[pane.Key] {
				return checkpoint{}, errors.New("a terminal foreground process changed during checkpoint validation")
			}
		}
	}
	if ctx.Err() != nil || discovery.Err() != nil {
		return checkpoint{}, errors.New("terminal restart checkpoint validation timed out")
	}
	final.Agents = recipe.Agents
	return final, nil
}

func sameRestartWorkspace(before, after tmux.Workspace) bool {
	// The persisted graph excludes live PID/title evidence; all saved attributes,
	// including names, cwd, labels and selection, must match exactly.
	left, leftErr := json.Marshal(before)
	right, rightErr := json.Marshal(after)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}
