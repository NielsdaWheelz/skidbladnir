//go:build darwin && cgo

package macnotifications

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -fblocks -mmacosx-version-min=13.0
#cgo LDFLAGS: -framework AppKit -framework UserNotifications -framework Foundation -framework CoreServices
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"
)

type nativeEvent struct {
	Kind    string          `json:"kind"`
	Request uint64          `json:"request,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Value   json.RawMessage `json:"value,omitempty"`
	Slot    string          `json:"slot,omitempty"`
	Episode int64           `json:"episode,omitempty"`
	Epoch   string          `json:"epoch,omitempty"`
	Ref     string          `json:"ref,omitempty"`
}

type nativeBridge struct {
	mu       sync.Mutex
	next     uint64
	requests map[uint64]chan nativeEvent
	events   []nativeEvent
	wake     chan struct{}
	closed   bool
}

type nativeReceipt struct {
	id       uint64
	response <-chan nativeEvent
}

var bridge = nativeBridge{requests: make(map[uint64]chan nativeEvent), wake: make(chan struct{}, 1)}

var errNativeRejected = errors.New("native notification operation unavailable")

// NativeRegister admits one installed app to Launch Services. It does not start
// the notification runtime or request permission; installation owns this call.
func NativeRegister(appPath string) error {
	path := C.CString(appPath)
	defer C.free(unsafe.Pointer(path))
	if status := C.skid_native_register(path); status != 0 {
		return fmt.Errorf("native app registration failed (%d)", status)
	}
	return nil
}

// NativeStatus observes the exact installed app without starting its owner.
func NativeStatus(appPath string) (int, error) {
	path := C.CString(appPath)
	defer C.free(unsafe.Pointer(path))
	pid := int(C.skid_native_status(path))
	if pid <= 0 {
		return 0, errors.New("notification app unavailable")
	}
	return pid, nil
}

// NativeStopApplication waits for the exact app to quit after its waiter stops.
// Its caller must remain on the original main thread while Cocoa advances.
func NativeStopApplication(appPath string) error {
	path := C.CString(appPath)
	defer C.free(unsafe.Pointer(path))
	if C.skid_native_stop_application(path) != 0 {
		return errors.New("notification app did not stop")
	}
	return nil
}

//export goSkidNativeEvent
func goSkidNativeEvent(encoded *C.char) {
	var event nativeEvent
	if json.Unmarshal([]byte(C.GoString(encoded)), &event) != nil {
		return
	}
	if event.Request != 0 {
		bridge.mu.Lock()
		response := bridge.requests[event.Request]
		delete(bridge.requests, event.Request)
		bridge.mu.Unlock()
		if response != nil {
			response <- event
		}
		return
	}
	// Native callbacks only append under the bridge's lock. One app-owned
	// consumer drains them; no callback starts a detached background task.
	bridge.mu.Lock()
	if !bridge.closed {
		bridge.events = append(bridge.events, event)
		select {
		case bridge.wake <- struct{}{}:
		default:
		}
	}
	bridge.mu.Unlock()
}

func (native *nativeBridge) drain() []nativeEvent {
	native.mu.Lock()
	defer native.mu.Unlock()
	events := native.events
	native.events = nil
	return events
}

func (native *nativeBridge) close() {
	native.mu.Lock()
	defer native.mu.Unlock()
	native.closed = true
	native.events = nil
}

func (native *nativeBridge) call(ctx context.Context, operation string, value, result any) error {
	receipt, err := native.begin(operation, value)
	if err != nil {
		return err
	}
	return native.await(ctx, receipt, result)
}

// Post initiates UserNotifications on the calling owner thread. Other operations
// dispatch to main. Completion stays buffered for the caller to await elsewhere.
func (native *nativeBridge) begin(operation string, value any) (*nativeReceipt, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	native.mu.Lock()
	native.next++
	id := native.next
	response := make(chan nativeEvent, 1)
	native.requests[id] = response
	native.mu.Unlock()
	op, payload := C.CString(operation), C.CString(string(encoded))
	C.skid_native_send(C.uint64_t(id), op, payload)
	C.free(unsafe.Pointer(op))
	C.free(unsafe.Pointer(payload))
	return &nativeReceipt{id: id, response: response}, nil
}

func (native *nativeBridge) await(ctx context.Context, receipt *nativeReceipt, result any) error {
	select {
	case <-ctx.Done():
		native.mu.Lock()
		delete(native.requests, receipt.id)
		native.mu.Unlock()
		fmt.Fprintln(os.Stderr, "notification native call: context")
		return ctx.Err()
	case event := <-receipt.response:
		if !event.OK {
			fmt.Fprintln(os.Stderr, "notification native call: unavailable")
			return errNativeRejected
		}
		if result != nil {
			err := json.Unmarshal(event.Value, result)
			if err != nil {
				fmt.Fprintln(os.Stderr, "notification native call: decode")
			}
			return err
		}
		return nil
	}
}

func nativeLoop() { C.skid_native_run() }

func nativeStop() { C.skid_native_stop() }
