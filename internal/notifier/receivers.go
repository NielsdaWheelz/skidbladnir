package notifier

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"slices"
	"time"

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

func phoneMember(members []fleet.Member, device string) bool {
	return slices.ContainsFunc(members, func(member fleet.Member) bool {
		return member.Kind == "phone" && member.DeviceID == device
	})
}

// phones keeps the receivers of current phone members.
func phones(receivers []receiver, members []fleet.Member) []receiver {
	return slices.DeleteFunc(slices.Clone(receivers), func(value receiver) bool {
		return !phoneMember(members, value.DeviceID)
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

// Configure retains the latest accepted settings even while storage is unavailable.
// Recovery reconciles those settings before reopening notification routes.
func (observer *Observer) Configure(members []fleet.Member, push *hostconfig.AndroidPush, sender *fcmpush.Client) {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	configuration := ""
	if push != nil {
		configuration = push.ID()
	} else {
		sender = nil
	}
	transportChanged := observer.configuration != configuration || observer.sender != sender
	observer.members = slices.Clone(members)
	observer.configuration, observer.sender = configuration, sender
	if observer.failure != nil {
		return
	}
	if file, changed := observer.reconcileConfiguration(observer.file); changed {
		if observer.save(file) != nil {
			return
		}
	}
	if transportChanged {
		wake(observer.send)
	}
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
	// Membership can change after authentication while the request body arrives.
	if !phoneMember(observer.members, device) {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		writeError(writer, http.StatusUnauthorized, "Unauthenticated", "authentication required")
		return
	}
	if observer.failure != nil {
		writeUnavailable(writer)
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
			writeUnavailable(writer)
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

// currentReceiver binds a send and its completion to the current desired
// transport, membership and registered destination. The caller holds the mutex.
func (observer *Observer) currentReceiver(sender *fcmpush.Client, configuration string, target receiver) int {
	if observer.failure != nil || observer.sender != sender || observer.configuration != configuration ||
		!phoneMember(observer.members, target.DeviceID) {
		return -1
	}
	return slices.IndexFunc(observer.file.Receivers, func(current receiver) bool {
		return current.DeviceID == target.DeviceID && current.Floor == target.Floor && current.FID == target.FID
	})
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
		if observer.failure != nil || observer.sender == nil {
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
			observer.mutex.Lock()
			if ctx.Err() != nil {
				observer.mutex.Unlock()
				return
			}
			current := observer.currentReceiver(sender, configuration, target) >= 0
			observer.mutex.Unlock()
			if !current {
				continue
			}
			request, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := sender.Send(request, fcmpush.Message{FID: target.FID, Data: map[string]string{"kind": "hint"}, Android: fcmpush.AndroidConfig{CollapseKey: urgency, Priority: priority, TTL: "2419200s", RestrictedPackageName: "dev.niels.skidbladnir"}})
			cancel()
			observer.mutex.Lock()
			if ctx.Err() != nil {
				observer.mutex.Unlock()
				return
			}
			if index := observer.currentReceiver(sender, configuration, target); index >= 0 {
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
					if observer.save(file) != nil {
						observer.mutex.Unlock()
						break
					}
				}
			}
			observer.mutex.Unlock()
		}
	}
}
