package fleetclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// start owns one bounded create/readiness/input composition. It never retries a write.
func (client *Client) start(ctx context.Context, request Request) Result {
	selected, found := client.peerByLabel(request.Machine)
	if !found {
		return Failed("machine_unknown", "not_sent")
	}
	value := StartResult{Label: selected.Label, Machine: selected.Machine, Creation: "not_sent", Prompt: "not_requested"}
	if request.Text != "" {
		value.Prompt = "not_sent"
	}
	finish := func() Result {
		result := success(value)
		if _, err := result.Encode("start"); err != nil {
			// A bounded receipt must retain the captured identity even if the
			// optional full observation cannot fit its transport envelope.
			value.Terminal = nil
			value.Failure = &Failure{Code: "output_limit", Dispatch: "unknown"}
			result = success(value)
		}
		return result
	}
	inputTarget := struct {
		IdentityToken  string `json:"identityToken"`
		PaneID         string `json:"paneId"`
		Text           string `json:"text"`
		InitialProfile string `json:"initialProfile"`
	}{
		// The shortest valid session lifetime and pane, not a metadata reserve.
		IdentityToken:  "v1-00000000000000000000000000000000.1.1.0",
		PaneID:         "%0",
		Text:           request.Text,
		InitialProfile: request.Profile,
	}
	if request.Text != "" {
		minimumInput, _ := json.Marshal(inputTarget)
		if len(minimumInput) > MaximumInputBytes {
			value.Failure = &Failure{Code: "input_limit", Dispatch: "not_sent"}
			return finish()
		}
	}
	cwd := request.CWD
	if cwd == "" {
		cwd = "~"
	}
	body, _ := json.Marshal(struct {
		Kind    LaunchKind `json:"kind"`
		CWD     string     `json:"cwd"`
		Profile string     `json:"profile,omitempty"`
		Name    string     `json:"optionalTmuxName,omitempty"`
		Group   string     `json:"group,omitempty"`
		Model   string     `json:"model,omitempty"`
		Effort  string     `json:"effort,omitempty"`
	}{request.Kind, cwd, request.Profile, request.Name, request.Group.String(), request.Model, request.Effort})
	if len(body) > MaximumInputBytes {
		value.Failure = &Failure{Code: "input_limit", Dispatch: "not_sent"}
		return finish()
	}
	created := client.call(ctx, selected, "start", "/v1/sessions", body)
	if !created.OK {
		failure := *created.Error
		value.Target, failure.Target = failure.Target, ""
		value.Failure = &failure
		if value.Target != "" {
			value.Creation = "created"
			ref, _ := DecodeReference(value.Target)
			value.Handle = ref.Handle()
		} else if failure.Dispatch == "unknown" {
			value.Creation = "unknown"
		}
		return finish()
	}
	observed := created.Value.(ObservedSession)
	value.Creation, value.Target, value.Handle, value.Terminal = "created", observed.Session.Ref, observed.Session.TerminalHandle, &observed
	if request.Text == "" {
		return finish()
	}
	ref, _ := DecodeReference(value.Target)
	inputTarget.IdentityToken, inputTarget.PaneID = ref.IdentityToken, ref.PaneID
	input, _ := json.Marshal(inputTarget)
	if len(input) > MaximumInputBytes {
		value.Failure = &Failure{Code: "input_limit", Dispatch: "not_sent"}
		return finish()
	}
	target := map[string]any{"identityToken": ref.IdentityToken, "paneId": ref.PaneID}
	inspectBody, _ := json.Marshal(target)
	path := "/v1/sessions/" + ref.TmuxID + "/terminal/"
	// justify-polling: one start observes only its captured terminal, every
	// 500 ms after the previous request, until the original 15-second deadline.
	for {
		if ctx.Err() != nil {
			value.Failure = &Failure{Code: "readiness_timeout", Dispatch: "not_sent"}
			return finish()
		}
		inspected := client.call(ctx, selected, "terminal_inspect", path+"inspect", inspectBody)
		if !inspected.OK {
			value.Failure = inspected.Error
			return finish()
		}
		status := inspected.Value.(TerminalInspectResult).TerminalStatus
		if status.Source == sessions.SourceUnavailable {
			value.Failure = &Failure{Code: "TerminalUnavailable", Dispatch: "not_sent"}
			return finish()
		}
		if status.Notice != sessions.NoticeNone || status.Interaction != sessions.InteractionNone && status.Interaction != sessions.InteractionUnknown {
			value.Failure = &Failure{Code: "TerminalInputBlocked", Dispatch: "not_sent", terminalInputMessage: "the requested agent is not ready for its initial prompt. inspect the terminal before sending."}
			if status.Interaction.Request() {
				value.Failure.terminalInputMessage = "respond to the dialog in the terminal."
			}
			return finish()
		}
		if status.Activity == sessions.ActivityIdle && status.Interaction == sessions.InteractionNone {
			sent := client.call(ctx, selected, "terminal_send", path+"send", input)
			if !sent.OK {
				value.Failure = sent.Error
				if sent.Error.Dispatch == "unknown" {
					value.Prompt = "unknown"
				}
			} else {
				value.Prompt = sent.Value.(WriteResult).Outcome
			}
			return finish()
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
