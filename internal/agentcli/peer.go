package agentcli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

func preparePeer(ctx context.Context, request *fleetclient.Request, machine string) {
	if request.Text == "" || request.Operation != "send" && request.Operation != "start" || !request.ReplyTo.IsZero() {
		return
	}
	if value := os.Getenv("SKID_REPLY_TO"); value != "" {
		// A stale stamped address must never retarget to the current tmux pane.
		request.PeerSource, _ = fleetclient.DecodePeerSource(value)
		return
	}
	ref, panePID, err := callerTerminal(ctx, machine, true)
	if err != nil {
		return
	}
	foreground, err := process.ObserveForeground(panePID)
	if err != nil {
		return
	}
	agent, recognized := agentruntime.Classify(foreground)
	if !recognized {
		return
	}
	// Inherited tmux variables outlive the agent that originally exported
	// them. Require a short kernel ancestry proof of the current foreground.
	pid := process.PID(os.Getppid())
	for range 16 {
		if pid == foreground.PID {
			if after, err := process.ObserveForeground(panePID); err == nil && after == foreground {
				request.PeerSource = peerSource(ref, agent.Provider, foreground)
			}
			return
		}
		parent, err := process.Observe(pid)
		if err != nil || parent.ParentPID <= 1 || parent.ParentPID == pid {
			return
		}
		pid = parent.ParentPID
	}
}

// StampPeer describes the process about to exec the provider. Its caller must
// preserve pid across exec. Issuing a return address still requires the live
// foreground to match this description.
func StampPeer(ctx context.Context, provider string, pid int) (string, error) {
	var native agentruntime.Provider
	switch provider {
	case "codex":
		native = agentruntime.ProviderCodex
	case "claude":
		native = agentruntime.ProviderClaude
	default:
		return "", errors.New("unknown peer provider")
	}
	directory, err := attention.StateDirectory()
	if err != nil {
		return "", err
	}
	local, err := fleet.ReadLocal(directory)
	if err != nil {
		return "", err
	}
	ref, _, err := callerTerminal(ctx, local.Machine, true)
	if err != nil {
		return "", err
	}
	observed, err := process.Observe(process.PID(pid))
	if err != nil {
		return "", err
	}
	return peerSource(ref, native, observed).Encode(), nil
}

func peerSource(ref fleetclient.Reference, provider agentruntime.Provider, observed process.Observation) fleetclient.PeerSource {
	return fleetclient.PeerSource{
		Reference: ref,
		Foreground: &api.PeerForeground{
			Provider: provider.String(), PID: int64(observed.PID), StartIdentity: string(observed.StartIdentity),
		},
	}
}

// callerTerminal reads the inherited client socket without starting a
// server. The gateway validates this session lifetime and selected pane again.
func callerTerminal(ctx context.Context, machine string, active bool) (fleetclient.Reference, process.PID, error) {
	unavailable := errors.New("peer terminal unavailable")
	pane := os.Getenv("TMUX_PANE")
	// A socket path can contain commas. Only the last two components
	// are numeric. TMUX stores the session id without its dollar sign.
	environment := os.Getenv("TMUX")
	last := strings.LastIndexByte(environment, ',')
	if last < 0 {
		return fleetclient.Reference{}, 0, unavailable
	}
	prior := strings.LastIndexByte(environment[:last], ',')
	if prior < 1 {
		return fleetclient.Reference{}, 0, unavailable
	}
	socket, pid := environment[:prior], environment[prior+1:last]
	sessionID := "$" + environment[last+1:]
	if !tmux.ValidID(pane, '%') || !tmux.ValidID(sessionID, '$') {
		return fleetclient.Reference{}, 0, unavailable
	}
	child, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	const format = "#{session_id} #{pane_id} #{@skid_server_epoch} #{pid} #{start_time} #{pane_pid}"
	args := []string{"list-panes", "-s", "-t", sessionID, "-F", format}
	if active {
		args = []string{"display-message", "-p", "-t", sessionID, format}
	}
	command := exec.CommandContext(child, "tmux", append([]string{"-N", "-S", socket}, args...)...)
	command.WaitDelay = 100 * time.Millisecond
	output, err := command.Output()
	if err != nil {
		return fleetclient.Reference{}, 0, unavailable
	}
	var fields []string
	for line := range strings.SplitSeq(string(output), "\n") {
		row := strings.Fields(line)
		if len(row) == 6 && row[0] == sessionID && row[1] == pane && row[3] == pid {
			fields = row
			break
		}
	}
	if fields == nil {
		return fleetclient.Reference{}, 0, unavailable
	}
	server, err := tmux.ParseServerIdentity(fields[2], fields[3], fields[4])
	panePID, pidErr := strconv.Atoi(fields[5])
	if err != nil || server.Epoch == "" || pidErr != nil || panePID <= 0 {
		return fleetclient.Reference{}, 0, unavailable
	}
	return fleetclient.Reference{Machine: machine, TmuxID: sessionID, PaneID: pane,
		IdentityToken: server.SessionToken(sessionID)}, process.PID(panePID), nil
}

// A live tmux environment can be inherited by an unrelated process. Confirm
// that the actual caller descends from the pane's process, including shells.
func delegateParent(ctx context.Context, machine string) (api.SessionRef, error) {
	ref, panePID, err := callerTerminal(ctx, machine, false)
	if err != nil {
		return api.SessionRef{}, err
	}
	for pid := process.PID(os.Getpid()); pid > 1; {
		if pid == panePID {
			return ref.SessionRef(), nil
		}
		parent, err := process.Observe(pid)
		if err != nil || parent.ParentPID == pid {
			break
		}
		pid = parent.ParentPID
	}
	return api.SessionRef{}, errors.New("caller does not belong to a skid session")
}
