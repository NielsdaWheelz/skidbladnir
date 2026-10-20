package notifier

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fcmpush"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
)

type destination struct {
	Generation  uint64 `json:"generation"`
	FID         string `json:"fid"`
	ClientNonce string `json:"clientNonce"`
	Challenge   string `json:"challenge"`
	Expires     int64  `json:"expires"`
}
type receiver struct {
	DeviceID          string       `json:"deviceId"`
	Configuration     string       `json:"configuration"`
	Floor             uint64       `json:"floor"`
	Active            *destination `json:"active,omitempty"`
	Confirmed         *destination `json:"confirmed,omitempty"`
	Desired           *destination `json:"desired,omitempty"`
	Removed           bool         `json:"removed"`
	RegistrationError bool         `json:"registrationError"`
	PushError         bool         `json:"pushError"`
}
type receiverStatus struct {
	Configuration     string `json:"configuration,omitempty"`
	ActiveGeneration  uint64 `json:"activeGeneration"`
	DesiredGeneration uint64 `json:"desiredGeneration"`
	Status            string `json:"status"`
}

func (value *receiver) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if attention.Decode(data, &fields) != nil {
		return attention.ErrUnavailable
	}
	for _, field := range []string{"deviceId", "configuration", "floor", "removed", "registrationError", "pushError"} {
		if fields[field] == nil {
			return attention.ErrUnavailable
		}
	}
	type wire receiver
	var decoded wire
	if attention.Decode(data, &decoded) != nil {
		return attention.ErrUnavailable
	}
	*value = receiver(decoded)
	return nil
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
func (value receiver) valid() bool {
	if !fleet.ValidDeviceID(value.DeviceID) || value.Floor >= math.MaxInt64 || value.Configuration != "" && !digestValid(value.Configuration) {
		return false
	}
	if value.Removed {
		return value.Floor > 0 && value.Active == nil && value.Desired == nil && value.Confirmed == nil
	}
	if value.Configuration == "" {
		return value.Active == nil && value.Desired == nil && value.Confirmed == nil
	}
	for _, target := range []*destination{value.Active, value.Desired, value.Confirmed} {
		if target != nil && (target.Generation == 0 || target.Generation > value.Floor || !token(target.FID, 256) || !token(target.ClientNonce, 128) || !token(target.Challenge, 128) || target.Expires <= 0) {
			return false
		}
	}
	if value.Active != nil && (value.Confirmed == nil || *value.Active != *value.Confirmed || value.Desired != nil && (value.Active.Generation > value.Desired.Generation || value.Active.Generation == value.Desired.Generation && *value.Active != *value.Desired)) {
		return false
	}
	return value.Desired == nil || value.Desired.Generation == value.Floor
}
func (observer *Observer) status(value receiver) receiverStatus {
	result := receiverStatus{Configuration: observer.file.Configuration, DesiredGeneration: value.Floor, Status: "pending"}
	if value.Active != nil {
		result.ActiveGeneration = value.Active.Generation
	}
	if value.Active != nil && value.Desired != nil && *value.Active == *value.Desired {
		result.Status = "active"
	}
	if value.RegistrationError || value.PushError {
		result.Status = "unavailable"
	}
	if observer.push == nil {
		result.Status = "disabled"
	} else if observer.sender == nil {
		result.Status = "unconfigured"
	}
	if value.Removed {
		result.Status = "removed"
	}
	return result
}
func (observer *Observer) receiverIndex(device string) int {
	return slices.IndexFunc(observer.file.Receivers, func(value receiver) bool { return value.DeviceID == device })
}
func (observer *Observer) saveReceiver(value receiver) error {
	file := observer.file
	file.Receivers = slices.Clone(file.Receivers)
	index := observer.receiverIndex(value.DeviceID)
	if index < 0 {
		file.Receivers = append(file.Receivers, value)
	} else {
		file.Receivers[index] = value
	}
	return observer.save(file)
}
func revoked(value receiver) (receiver, error) {
	if value.Floor >= math.MaxInt64-1 {
		return value, attention.ErrUnavailable
	}
	return receiver{DeviceID: value.DeviceID, Configuration: value.Configuration, Floor: value.Floor + 1, Removed: true}, nil
}
func retireAbsent(file observerFile, members []fleet.Member) (observerFile, bool, error) {
	changed := false
	file.Receivers = slices.Clone(file.Receivers)
	for index, value := range file.Receivers {
		if value.Removed || slices.ContainsFunc(members, func(member fleet.Member) bool { return member.Kind == "phone" && member.DeviceID == value.DeviceID }) {
			continue
		}
		value, err := revoked(value)
		if err != nil {
			return file, false, err
		}
		file.Receivers[index] = value
		changed = true
	}
	return file, changed, nil
}

// ConfigurePush changes only push transport. It is synchronous, offline and safe
// under the fleet lock. Configuration changes erase destinations but keep floors.
func (observer *Observer) ConfigurePush(push *hostconfig.AndroidPush, sender *fcmpush.Client) error {
	if push != nil && push.Validate() != nil {
		return attention.ErrUnavailable
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.drained || observer.unavailable {
		return attention.ErrUnavailable
	}
	if observer.sendCancel != nil {
		observer.sendCancel()
	}
	configuration := ""
	if push != nil {
		copy := *push
		push = &copy
		configuration = push.ID()
	} else {
		sender = nil
	}
	if configuration != observer.file.Configuration {
		file := observer.file
		file.Configuration = configuration
		file.Receivers = slices.Clone(file.Receivers)
		for index, value := range file.Receivers {
			value.Configuration = configuration
			value.Active = nil
			value.Confirmed = nil
			value.Desired = nil
			value.RegistrationError = false
			value.PushError = false
			file.Receivers[index] = value
		}
		if observer.save(file) != nil {
			return attention.ErrUnavailable
		}
	}
	observer.push = push
	observer.sender = sender
	observer.signal(false)
	return nil
}

// ReconcileMembers closes the construction/admission race without a fleet call.
func (observer *Observer) ReconcileMembers(members []fleet.Member) error {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.drained || observer.unavailable {
		return attention.ErrUnavailable
	}
	file, changed, err := retireAbsent(observer.file, members)
	if err != nil {
		observer.unavailable = true
		return err
	}
	if !changed {
		return nil
	}
	if observer.sendCancel != nil {
		observer.sendCancel()
	}
	if err = observer.save(file); err != nil {
		return err
	}
	observer.signal(false)
	return nil
}

// Revoke erases destinations in the local durable commit. There is no remote delete.
func (observer *Observer) Revoke(device string) error {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.drained || observer.unavailable || !fleet.ValidDeviceID(device) {
		return attention.ErrUnavailable
	}
	value := receiver{DeviceID: device, Configuration: observer.file.Configuration}
	if index := observer.receiverIndex(device); index >= 0 {
		value = observer.file.Receivers[index]
	}
	if value.Removed {
		return nil
	}
	value, err := revoked(value)
	if err != nil {
		observer.unavailable = true
		return err
	}
	if observer.sendCancel != nil {
		observer.sendCancel()
	}
	if err = observer.saveReceiver(value); err != nil {
		return err
	}
	observer.signal(false)
	return nil
}
func (observer *Observer) receiverState(writer http.ResponseWriter, device string) {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.drained || observer.unavailable {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	value := receiver{}
	if index := observer.receiverIndex(device); index >= 0 {
		value = observer.file.Receivers[index]
	}
	writeJSON(writer, observer.status(value))
}
func (observer *Observer) register(writer http.ResponseWriter, request *http.Request, device string) {
	var input struct {
		Configuration string `json:"configuration"`
		Generation    uint64 `json:"generation"`
		FID           string `json:"fid"`
		ClientNonce   string `json:"clientNonce"`
	}
	if !readRegistration(writer, request, &input) {
		return
	}
	if !digestValid(input.Configuration) || input.Generation == 0 || input.Generation >= math.MaxInt64 || !token(input.FID, 256) || !token(input.ClientNonce, 128) {
		writeError(writer, "InvalidRequest")
		return
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.drained || observer.unavailable {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	if observer.push == nil || input.Configuration != observer.file.Configuration {
		writeError(writer, "ReceiverConflict")
		return
	}
	value := receiver{DeviceID: device, Configuration: input.Configuration}
	if index := observer.receiverIndex(device); index >= 0 {
		value = observer.file.Receivers[index]
	}
	if value.Removed {
		writeError(writer, "Forbidden")
		return
	}
	exact := value.Desired != nil && input.Generation == value.Desired.Generation && input.FID == value.Desired.FID && input.ClientNonce == value.Desired.ClientNonce
	if !exact && input.Generation <= value.Floor {
		writeError(writer, "ReceiverConflict")
		return
	}
	if !exact {
		desired, err := pendingDestination(input.Generation, input.FID, input.ClientNonce)
		if err != nil {
			writeError(writer, "NotificationsUnavailable")
			return
		}
		value.Floor = input.Generation
		value.Desired = desired
		value.RegistrationError = false
		if observer.sendCancel != nil {
			observer.sendCancel()
		}
		if observer.saveReceiver(value) != nil {
			writeError(writer, "NotificationsUnavailable")
			return
		}
	}
	observer.signal(false)
	writeJSON(writer, observer.status(value))
}
func pendingDestination(generation uint64, fid, nonce string) (*destination, error) {
	challenge := [32]byte{}
	if _, err := rand.Read(challenge[:]); err != nil {
		return nil, err
	}
	return &destination{Generation: generation, FID: fid, ClientNonce: nonce, Challenge: hex.EncodeToString(challenge[:]), Expires: time.Now().Add(10 * time.Minute).Unix()}, nil
}
func (observer *Observer) confirm(writer http.ResponseWriter, request *http.Request, device string) {
	var input struct {
		Configuration string `json:"configuration"`
		Generation    uint64 `json:"generation"`
		Challenge     string `json:"challenge"`
	}
	if !readRegistration(writer, request, &input) {
		return
	}
	if !digestValid(input.Configuration) || input.Generation == 0 || input.Generation >= math.MaxInt64 || !token(input.Challenge, 128) {
		writeError(writer, "InvalidRequest")
		return
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.drained || observer.unavailable {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	if observer.push == nil || input.Configuration != observer.file.Configuration {
		writeError(writer, "ReceiverConflict")
		return
	}
	index := observer.receiverIndex(device)
	if index < 0 {
		writeError(writer, "ReceiverConflict")
		return
	}
	value := observer.file.Receivers[index]
	if value.Removed {
		writeError(writer, "Forbidden")
		return
	}
	// Preserve the last committed proof even while its successor is pending.
	if value.Confirmed != nil && value.Confirmed.Generation == input.Generation && value.Confirmed.Challenge == input.Challenge {
		writeJSON(writer, observer.status(value))
		return
	}
	if value.Desired == nil || value.Desired.Generation != input.Generation || value.Desired.Challenge != input.Challenge || value.Desired.Expires <= time.Now().Unix() {
		observer.signal(false)
		writeError(writer, "ReceiverConflict")
		return
	}
	value.Active = value.Desired
	value.Confirmed = value.Desired
	value.RegistrationError = false
	if observer.saveReceiver(value) != nil {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	observer.signal(false)
	writeJSON(writer, observer.status(value))
}

// The existing worker owns all sends. Its snapshot is checked again after Google
// HTTP; neither the fleet lock nor the observer lock is held across the request.
func (observer *Observer) sendReceiver(ctx context.Context, device, urgency string, enrollment bool) {
	observer.mutex.Lock()
	if observer.drained || observer.unavailable || observer.push == nil || observer.sender == nil {
		observer.mutex.Unlock()
		return
	}
	index := observer.receiverIndex(device)
	if index < 0 || observer.file.Receivers[index].Removed {
		observer.mutex.Unlock()
		return
	}
	value := observer.file.Receivers[index]
	if enrollment && value.Desired != nil && (value.Active == nil || value.Desired.Generation != value.Active.Generation) && value.Desired.Expires <= time.Now().Unix() {
		desired, err := pendingDestination(value.Desired.Generation, value.Desired.FID, value.Desired.ClientNonce)
		if err != nil {
			observer.mutex.Unlock()
			return
		}
		value.Desired = desired
		if observer.saveReceiver(value) != nil {
			observer.mutex.Unlock()
			return
		}
	}
	configuration := observer.file.Configuration
	sender := observer.sender
	if enrollment && (value.Desired == nil || value.Active != nil && value.Desired.Generation == value.Active.Generation) {
		observer.mutex.Unlock()
		return
	}
	var message fcmpush.Message
	if enrollment {
		target := value.Desired
		message = fcmpush.MessageFor(target.FID, "enrollment", "NORMAL", map[string]string{"kind": "enrollment", "fleetId": observer.file.FleetID, "deviceId": device, "configuration": configuration, "generation": strconv.FormatUint(target.Generation, 10), "clientNonce": target.ClientNonce, "challenge": target.Challenge})
	} else if value.Active != nil {
		priority := "NORMAL"
		if urgency == "attention" {
			priority = "HIGH"
		}
		message = fcmpush.MessageFor(value.Active.FID, urgency, priority, map[string]string{"kind": "hint", "schema": "1", "fleetId": observer.file.FleetID, "deviceId": device, "configuration": configuration, "receiverGeneration": strconv.FormatUint(value.Active.Generation, 10), "epoch": observer.file.Epoch, "revision": strconv.FormatInt(observer.file.Revision, 10), "urgency": urgency})
	} else {
		observer.mutex.Unlock()
		return
	}
	request, cancel := context.WithTimeout(ctx, 10*time.Second)
	observer.sendCancel = cancel
	observer.mutex.Unlock()
	err := sender.Send(request, message)
	cancel()
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	observer.sendCancel = nil
	if observer.drained || observer.unavailable || observer.file.Configuration != configuration || observer.sender != sender {
		return
	}
	index = observer.receiverIndex(device)
	if index < 0 {
		return
	}
	current := observer.file.Receivers[index]
	if current.Removed || enrollment && (current.Desired != value.Desired || current.Active != nil && current.Active.Generation == value.Desired.Generation) || !enrollment && current.Active != value.Active {
		return
	}
	if enrollment {
		current.RegistrationError = err != nil
	} else {
		current.PushError = err != nil
	}
	var invalid *fcmpush.Error
	if errors.As(err, &invalid) && invalid.Status == http.StatusGone {
		if enrollment {
			current.Desired = nil
		} else {
			current.Active = nil
			if current.Desired != nil && *current.Desired == *value.Active {
				current.Desired = nil
			}
		}
	}
	_ = observer.saveReceiver(current) // justify-ignore-error: save fences the observer on durable write failure.
}
func (observer *Observer) publish(ctx context.Context) {
	pulse := time.NewTicker(time.Minute)
	defer pulse.Stop()
	observer.mutex.Lock()
	observer.signal(false)
	observer.mutex.Unlock()
	for {
		select {
		case <-observer.changed:
		case <-pulse.C:
			observer.mutex.Lock()
			for subscriber := range observer.subscribers {
				select {
				case subscriber <- struct{}{}:
				default:
				}
			}
			observer.mutex.Unlock()
		case <-ctx.Done():
			return
		}
		observer.mutex.Lock()
		if observer.drained || observer.unavailable {
			observer.mutex.Unlock()
			return
		}
		urgency := "sync"
		if observer.attentionPending {
			urgency = "attention"
		}
		observer.attentionPending = false
		devices := make([]string, 0, len(observer.file.Receivers))
		for _, value := range observer.file.Receivers {
			if !value.Removed {
				devices = append(devices, value.DeviceID)
			}
		}
		observer.mutex.Unlock()
		for _, device := range devices {
			if ctx.Err() != nil {
				return
			}
			observer.sendReceiver(ctx, device, urgency, true)
			observer.sendReceiver(ctx, device, urgency, false)
		}
	}
}
