package fleetclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

// ClosePlan is a confirmation's exact mode and targets, in dispatch order.
// Its observations are only a preview. ConfirmClose acquires fresh fences.
type ClosePlan struct {
	Targets      []CloseTarget
	TerminalOnly bool
	observations []closeObservation
}

type CloseTarget struct {
	Ref   Reference `json:"ref"`
	Name  string    `json:"name"`
	Label string    `json:"machine"`
}

type CloseOutcome struct {
	CloseTarget
	Terminal  string `json:"terminal"`
	Interrupt string `json:"interrupt"`
}

type FenceReleaseFailure struct {
	CloseTarget
	Failure *Failure `json:"failure"`
}

type CloseReport struct {
	TerminalOnly   bool                  `json:"terminalOnly"`
	Proposed       []CloseTarget         `json:"proposed,omitempty"`
	Targets        []CloseOutcome        `json:"targets,omitempty"`
	Failure        *Failure              `json:"failure,omitempty"`
	CleanupFailure *Failure              `json:"cleanupFailure,omitempty"`
	FenceFailures  []FenceReleaseFailure `json:"fenceFailures,omitempty"`
}

type closeObservation struct {
	node     api.BranchNode
	parent   int            // postorder index, -1 for the root
	edge     api.SessionRef // the exact registration held by that parent
	obsolete []api.SessionRef
}

func newBranchID() string {
	var id [16]byte
	rand.Read(id[:])
	return hex.EncodeToString(id[:])
}

func (client *Client) branch(ctx context.Context, ref api.SessionRef, input api.BranchRequest) Result {
	host, found := client.peerByMachine(ref.Machine)
	if !found {
		return Failed("machine_unknown", "not_sent")
	}
	input.IdentityToken = ref.IdentityToken
	data, _ := json.Marshal(input)
	if len(data) > MaximumInputBytes {
		return Failed("input_limit", "not_sent")
	}
	operation := "branch_mutate"
	switch input.Action {
	case "inspect":
		operation = "branch_inspect"
	case "reserve":
		operation = "branch_reserve"
	}
	return client.call(ctx, host, operation, ref.TmuxID, data)
}

// PreviewClose consults only the relevant parent-owned registrations. A missing
// registered child must be established by its host; unreachable is not absent.
func (client *Client) PreviewClose(ctx context.Context, request Request) (ClosePlan, *Failure) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	return client.previewClose(ctx, request)
}

func (client *Client) previewClose(ctx context.Context, request Request) (ClosePlan, *Failure) {
	request.Operation = "close"
	root, failure := client.resolve(ctx, request)
	if failure != nil {
		return ClosePlan{}, failure
	}
	plan := ClosePlan{TerminalOnly: request.TerminalOnly}
	seen := map[api.SessionRef]bool{}
	var visit func(api.SessionRef, *api.BranchNode, api.SessionRef) (int, *Failure)
	visit = func(edge api.SessionRef, parent *api.BranchNode, parentRef api.SessionRef) (int, *Failure) {
		result := client.branch(ctx, edge, api.BranchRequest{Action: "inspect"})
		if !result.OK {
			if parent != nil && result.Error.Code == "SessionNotFound" {
				return -1, nil
			}
			return -1, result.Error
		}
		node := result.Value.(api.BranchNode)
		ref := Reference{Machine: edge.Machine, TmuxID: node.Target.TmuxID, IdentityToken: node.Target.IdentityToken, PaneID: node.Target.PaneID}
		lifetime := ref.SessionRef()
		if parent == nil {
			if !ref.SessionEqual(root.ref) || !request.TerminalOnly && ref.PaneID != root.ref.PaneID {
				return -1, Failed("BranchChanged", "not_sent").Error
			}
		} else {
			if lifetime != edge && !slices.Contains(node.Previous, edge) {
				return -1, Failed("BranchUnresolved", "not_sent").Error
			}
			if node.Parent == nil { // Promotion is irreversible.
				return -1, nil
			}
			if *node.Parent != parentRef && !slices.Contains(parent.Previous, *node.Parent) {
				return -1, Failed("BranchUnresolved", "not_sent").Error
			}
		}
		if seen[lifetime] {
			return -1, Failed("BranchUnresolved", "not_sent").Error
		}
		if node.Pending {
			return -1, Failed("BranchPending", "not_sent").Error
		}
		seen[lifetime] = true
		observation := closeObservation{node: node, parent: -1, edge: edge}
		var direct []int
		for _, child := range node.Children {
			index, failure := visit(child, &node, lifetime)
			if failure != nil {
				return -1, failure
			}
			if index < 0 {
				observation.obsolete = append(observation.obsolete, child)
			} else {
				direct = append(direct, index)
			}
		}
		index := len(plan.Targets)
		for _, child := range direct {
			plan.observations[child].parent = index
		}
		host, _ := client.peerByMachine(ref.Machine)
		plan.Targets = append(plan.Targets, CloseTarget{Ref: ref, Name: node.Name, Label: host.Label})
		plan.observations = append(plan.observations, observation)
		return index, nil
	}
	if _, failure := visit(root.ref.SessionRef(), nil, api.SessionRef{}); failure != nil {
		return ClosePlan{}, failure
	}
	return plan, nil
}

// ConfirmClose fences every exact lifetime before any terminal dispatch, then
// spends the plan in postorder. Independent host effects can fail partway.
func (client *Client) ConfirmClose(ctx context.Context, plan ClosePlan) (report CloseReport) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	report.TerminalOnly = plan.TerminalOnly
	for _, target := range plan.Targets {
		report.Targets = append(report.Targets, CloseOutcome{CloseTarget: target, Terminal: "remaining", Interrupt: "not_sent"})
	}
	if len(report.Targets) == 0 {
		report.Failure = Failed("BranchChanged", "not_sent").Error
		return report
	}
	id := newBranchID()
	firstPrepared := len(plan.Targets)
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		var waiting sync.WaitGroup
		failures := make([]*Failure, len(plan.Targets)-firstPrepared)
		for index, target := range report.Targets[firstPrepared:] {
			if target.Terminal == "closed" {
				continue
			}
			waiting.Go(func() {
				result := client.branch(cleanup, target.Ref.SessionRef(), api.BranchRequest{Action: "release", CloseID: id})
				if !result.OK && result.Error.Code != "SessionNotFound" {
					failures[index] = result.Error
				}
			})
		}
		waiting.Wait()
		for index, failure := range failures {
			if failure != nil {
				report.FenceFailures = append(report.FenceFailures, FenceReleaseFailure{CloseTarget: plan.Targets[firstPrepared+index], Failure: failure})
			}
		}
	}()
	mode := "interrupt"
	if plan.TerminalOnly {
		mode = "terminal"
	}
	// Reverse postorder fences a parent before any of its children.
	for index := len(plan.Targets) - 1; index >= 0; index-- {
		target, observed := plan.Targets[index], plan.observations[index]
		result := client.branch(ctx, target.Ref.SessionRef(), api.BranchRequest{Action: "prepare", CloseID: id, Mode: mode, Expected: &observed.node})
		if result.OK || result.Error.Dispatch == "unknown" {
			firstPrepared = index
		}
		if !result.OK {
			report.Failure = result.Error
			return report
		}
	}
	// Workspace recovery can supersede a preview's proof of absence.
	for _, observed := range plan.observations {
		for _, child := range observed.obsolete {
			result := client.branch(ctx, child, api.BranchRequest{Action: "inspect"})
			if !result.OK {
				if result.Error.Code == "SessionNotFound" {
					continue
				}
				report.Failure = result.Error
				return report
			}
			node := result.Value.(api.BranchNode)
			ref := api.SessionRef{Machine: child.Machine, TmuxID: node.Target.TmuxID, IdentityToken: node.Target.IdentityToken}
			if node.Parent != nil || ref != child && !slices.Contains(node.Previous, child) {
				report.Failure = Failed("BranchChanged", "not_sent").Error
				return report
			}
		}
	}
	for index, target := range plan.Targets {
		ref, observed := target.Ref, plan.observations[index]
		for _, child := range observed.obsolete {
			result := client.branch(ctx, ref.SessionRef(), api.BranchRequest{Action: "unlink", CloseID: id, Child: &child})
			if !result.OK {
				report.Failure = result.Error
				return report
			}
		}
		selected, found := client.peerByMachine(ref.Machine)
		if !found {
			report.Failure = Failed("machine_unknown", "not_sent").Error
			return report
		}
		body := map[string]string{"identityToken": ref.IdentityToken, "closeId": id}
		operation := "terminal_close"
		if !plan.TerminalOnly {
			operation = "close"
			body["paneId"] = ref.PaneID
		}
		data, _ := json.Marshal(body)
		result := client.call(ctx, selected, operation, ref.TmuxID, data)
		outcome := &report.Targets[index]
		if !result.OK {
			report.Failure = result.Error
			if result.Error.Dispatch == "unknown" {
				outcome.Terminal = "unknown"
				if !plan.TerminalOnly {
					outcome.Interrupt = "unknown"
				}
			}
			return report
		}
		outcome.Terminal = "closed"
		if !plan.TerminalOnly {
			closed := result.Value.(api.CloseResult)
			outcome.Interrupt = closed.Interrupt
			switch closed.Terminal {
			case "not_closed":
				outcome.Terminal = "remaining"
			case "unknown":
				outcome.Terminal = "unknown"
			}
			if outcome.Terminal != "closed" {
				return report
			}
		}
		// The exact lifetime stopped. A stale checkpoint may still be able to
		// restore a new lifetime under this parent. Its host proves absence
		// before we erase that parent's only admission record.
		if observed.parent >= 0 || observed.node.Parent != nil {
			settled := client.branch(ctx, ref.SessionRef(), api.BranchRequest{Action: "inspect"})
			if settled.OK || settled.Error.Code != "SessionNotFound" {
				report.Failure = settled.Error
				if settled.OK {
					report.Failure = Failed("BranchChanged", "not_sent").Error
				}
				return report
			}
		}
		if observed.parent < 0 {
			report.CleanupFailure = client.unlinkFromParent(ctx, ref.SessionRef(), observed.node)
		} else {
			parent := plan.Targets[observed.parent].Ref.SessionRef()
			result := client.branch(ctx, parent, api.BranchRequest{Action: "unlink", CloseID: id, Child: &observed.edge})
			if !result.OK {
				report.Failure = result.Error
				return report
			}
		}
		if outcome.Interrupt == "unknown" {
			return report
		}
	}
	return report
}

func (client *Client) promote(ctx context.Context, request Request) Result {
	ref := request.Ref.SessionRef()
	inspected := client.branch(ctx, ref, api.BranchRequest{Action: "inspect"})
	if !inspected.OK {
		return inspected
	}
	node := inspected.Value.(api.BranchNode)
	promoted := client.branch(ctx, ref, api.BranchRequest{Action: "promote", Parent: request.Parent})
	if promoted.OK {
		if failure := client.unlinkFromParent(ctx, ref, node); failure != nil {
			promoted.Value = failure
		}
	}
	return promoted
}

// A positive child-side acknowledgement permits retiring its registration.
// The old parent and child references may have proven restore mappings.
func (client *Client) unlinkFromParent(ctx context.Context, child api.SessionRef, node api.BranchNode) *Failure {
	if node.Parent == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	observed := client.branch(ctx, *node.Parent, api.BranchRequest{Action: "inspect"})
	if !observed.OK {
		if observed.Error.Code == "SessionNotFound" {
			return nil
		}
		return observed.Error
	}
	parent := observed.Value.(api.BranchNode)
	current := api.SessionRef{Machine: node.Parent.Machine, TmuxID: parent.Target.TmuxID, IdentityToken: parent.Target.IdentityToken}
	if current != *node.Parent && !slices.Contains(parent.Previous, *node.Parent) {
		return Failed("BranchUnresolved", "not_sent").Error
	}
	for _, edge := range parent.Children {
		if child == edge || slices.Contains(node.Previous, edge) {
			return client.branch(ctx, current, api.BranchRequest{Action: "unlink", Child: &edge}).Error
		}
	}
	return nil
}
