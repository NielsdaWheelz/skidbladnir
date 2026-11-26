package fleetclient

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

// Job runs exactly one approved host operation, addressed by a fleet label or
// an immutable machine handle. Observation waits longer than ordinary calls.
func (client *Client) Job(parent context.Context, operation, host, id string, input api.JobRequest, offset int) Result {
	target, found := client.peerByLabel(host)
	if handle, err := machine.Parse(host); err == nil {
		target, found = client.peerByMachine(handle.String())
	}
	if !found {
		return Failed("machine_unknown", "not_sent")
	}
	timeout := Timeout
	var body []byte
	switch operation {
	case "submit":
		body, _ = json.Marshal(input)
		if len(body) > MaximumInputBytes {
			return Failed("input_limit", "not_sent")
		}
	case "result":
		timeout = 30 * time.Second
		if offset != 0 {
			id += "?offset=" + strconv.Itoa(offset)
		}
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	result := client.call(ctx, target, "job_"+operation, id, body)
	if result.OK && operation == "ack" {
		result.Value = struct{}{}
	}
	return result
}

type jobReceipt struct {
	JobID    string       `json:"job_id"`
	State    api.JobState `json:"state"`
	Reason   string       `json:"reason,omitempty"`
	Terminal string       `json:"terminal,omitempty"`
	Handle   string       `json:"handle,omitempty"`
}

type jobResult struct {
	jobReceipt
	Length int    `json:"length"`
	SHA256 string `json:"sha256"`
	Offset int    `json:"offset"`
	Page   []byte `json:"page"`
}

func projectJob(receipt api.JobReceipt, machine string) jobReceipt {
	value := jobReceipt{JobID: receipt.JobID, State: receipt.State, Reason: receipt.Reason}
	if target := receipt.Terminal; target != nil {
		ref := Reference{Machine: machine, TmuxID: target.TmuxID, IdentityToken: target.IdentityToken, PaneID: target.PaneID}
		value.Terminal, value.Handle = ref.Encode(), ref.Handle()
	}
	return value
}
