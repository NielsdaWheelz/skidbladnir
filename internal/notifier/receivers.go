package notifier

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fcmpush"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
)

// receiver is one phone's fcm destination for the file's push configuration.
// Floor is the highest generation the phone registered; it survives
// reconfiguration so a stale registration cannot return.
type receiver struct {
	DeviceID string `json:"deviceId"`
	Floor    uint64 `json:"floor"`
	FID      string `json:"fid,omitempty"` // "" = no destination
	Failed   bool   `json:"failed"`        // the last send to FID failed
}

type receiverStatus struct {
	Status string `json:"status"` // disabled | unconfigured | active | unavailable
}

// phones keeps the receivers of current phone members.
func phones(receivers []receiver, members []fleet.Member) []receiver {
	return slices.DeleteFunc(slices.Clone(receivers), func(value receiver) bool {
		return !slices.ContainsFunc(members, func(member fleet.Member) bool { return member.Kind == "phone" && member.DeviceID == value.DeviceID })
	})
}

// status is one phone's delivery as the hub knows it. The caller holds the mutex.
func (observer *Observer) status(device string) receiverStatus {
	index := slices.IndexFunc(observer.file.Receivers, func(value receiver) bool { return value.DeviceID == device })
	switch {
	case observer.file.Configuration == "":
		return receiverStatus{"disabled"}
	case observer.sender == nil:
		return receiverStatus{"unconfigured"}
	case index < 0 || observer.file.Receivers[index].FID == "" || observer.file.Receivers[index].Failed:
		return receiverStatus{"unavailable"}
	default:
		return receiverStatus{"active"}
	}
}

// ConfigurePush changes only the push transport; it is synchronous and offline.
// A configuration change erases destinations but keeps floors.
func (observer *Observer) ConfigurePush(push *hostconfig.AndroidPush, sender *fcmpush.Client) error {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.unavailable {
		return attention.ErrUnavailable
	}
	configuration := ""
	if push != nil {
		configuration = push.ID()
	} else {
		sender = nil
	}
	if configuration != observer.file.Configuration {
		file := observer.file
		file.Configuration = configuration
		file.Receivers = slices.Clone(file.Receivers)
		for index := range file.Receivers {
			file.Receivers[index].FID, file.Receivers[index].Failed = "", false
		}
		if err := observer.save(file); err != nil {
			return err
		}
	}
	observer.sender = sender
	wake(observer.send)
	return nil
}

// ReconcileMembers deletes the receivers of devices no longer in the fleet; a
// removed member can no longer authenticate to register again.
func (observer *Observer) ReconcileMembers(members []fleet.Member) error {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.unavailable {
		return attention.ErrUnavailable
	}
	receivers := phones(observer.file.Receivers, members)
	if len(receivers) == len(observer.file.Receivers) {
		return nil
	}
	file := observer.file
	file.Receivers = receivers
	return observer.save(file)
}

// register activates a phone's destination for a newer generation; the same
// destination again is idempotent and anything else conflicts.
func (observer *Observer) register(writer http.ResponseWriter, request *http.Request, device string) {
	var input struct {
		Configuration string `json:"configuration"`
		Generation    uint64 `json:"generation"`
		FID           string `json:"fid"`
	}
	if !readBody(writer, request, &input) {
		return
	}
	if !digestValid(input.Configuration) || input.Generation == 0 || input.Generation >= math.MaxInt64 || !token(input.FID, 256) {
		writeError(writer, http.StatusBadRequest, "InvalidRequest", "the request is not valid")
		return
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.unavailable {
		WriteUnavailable(writer)
		return
	}
	index := slices.IndexFunc(observer.file.Receivers, func(value receiver) bool { return value.DeviceID == device })
	var current receiver
	if index >= 0 {
		current = observer.file.Receivers[index]
	}
	// Push off has configuration "", which no valid digest equals.
	configured := input.Configuration == observer.file.Configuration
	switch {
	case configured && input.Generation == current.Floor && input.FID == current.FID && current.FID != "":
		writer.WriteHeader(http.StatusNoContent)
	case configured && input.Generation > current.Floor:
		file := observer.file
		file.Receivers = slices.Clone(file.Receivers)
		next := receiver{DeviceID: device, Floor: input.Generation, FID: input.FID}
		if index < 0 {
			file.Receivers = append(file.Receivers, next)
		} else {
			file.Receivers[index] = next
		}
		if observer.save(file) != nil {
			WriteUnavailable(writer)
			return
		}
		wake(observer.send)
		writer.WriteHeader(http.StatusNoContent)
	default:
		writeError(writer, http.StatusConflict, "ReceiverConflict", "receiver generation changed")
	}
}

func digestValid(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}

func token(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum {
		return false
	}
	for _, ch := range value {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-') {
			return false
		}
	}
	return true
}

// sendHints is the only fcm sender. It wakes on every projection change,
// registration and push reconfiguration, sends one content-free hint to each
// destination, and stops with ctx. A failed send waits for the next change.
func (observer *Observer) sendHints(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-observer.send:
		}
		observer.mutex.Lock()
		if observer.unavailable || observer.sender == nil {
			observer.mutex.Unlock()
			continue
		}
		sender, configuration := observer.sender, observer.file.Configuration
		urgency, priority := "sync", "NORMAL"
		if observer.urgent {
			urgency, priority = "attention", "HIGH"
		}
		observer.urgent = false
		targets := slices.DeleteFunc(slices.Clone(observer.file.Receivers), func(value receiver) bool { return value.FID == "" })
		observer.mutex.Unlock()
		for _, target := range targets {
			request, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := sender.Send(request, fcmpush.Message{FID: target.FID, Data: map[string]string{"kind": "hint"}, Android: fcmpush.AndroidConfig{CollapseKey: urgency, Priority: priority, TTL: "2419200s", RestrictedPackageName: "dev.niels.skidbladnir"}})
			cancel()
			if ctx.Err() != nil {
				return
			}
			observer.mutex.Lock()
			// A reconfiguration or re-registration during the send owns the receiver now.
			index := slices.IndexFunc(observer.file.Receivers, func(value receiver) bool { return value.DeviceID == target.DeviceID })
			if !observer.unavailable && observer.sender == sender && observer.file.Configuration == configuration && index >= 0 && observer.file.Receivers[index].Floor == target.Floor && observer.file.Receivers[index].FID == target.FID {
				next := observer.file.Receivers[index]
				next.Failed = err != nil
				var failure *fcmpush.Error
				if errors.As(err, &failure) && failure.Status == http.StatusGone {
					next.FID = ""
				}
				if next != observer.file.Receivers[index] {
					file := observer.file
					file.Receivers = slices.Clone(file.Receivers)
					file.Receivers[index] = next
					_ = observer.save(file) // a failed save latches the observer unavailable
				}
			}
			observer.mutex.Unlock()
		}
	}
}
