package fleetclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
)

func NotificationKey(ref Reference) attention.Key {
	return attention.Key{Machine: ref.Machine, TmuxID: ref.TmuxID, IdentityToken: ref.IdentityToken, PaneID: ref.PaneID}
}
func NotificationForeground(session Session) *attention.Foreground {
	if session.Agent == nil || session.Connection != nil {
		return nil
	}
	return &attention.Foreground{Provider: session.Agent.Provider, PID: session.Agent.PID, StartIdentity: session.Agent.StartIdentity}
}

// NotificationReady qualifies the queue against both current direct and producer evidence.
func NotificationReady(device attention.DeviceSnapshot, session Session, producer attention.Snapshot) bool {
	if device.ObserverEpoch != producer.Epoch || !attention.Idle(session.TerminalStatus) {
		return false
	}
	ref, err := DecodeReference(session.Ref)
	if err != nil {
		return false
	}
	row, found := producer.Session(NotificationKey(ref))
	return found && attention.SameForeground(row.Record.Foreground, NotificationForeground(session)) && row.Record.Foreground != nil && device.Pending(row)
}

type NotificationStore struct{ path string }

func NewNotificationStore(path string) (*NotificationStore, error) {
	if !filepath.IsAbs(path) {
		return nil, attention.ErrUnavailable
	}
	return &NotificationStore{path: path}, nil
}
func DefaultNotificationStore() (*NotificationStore, error) {
	directory, err := attention.StateDirectory()
	if err != nil {
		return nil, err
	}
	return NewNotificationStore(filepath.Join(directory, "notifications-v4.json"))
}
func (store *NotificationStore) Read(ctx context.Context) (attention.DeviceSnapshot, error) {
	return store.Update(ctx, func(*attention.DeviceSnapshot) (bool, error) { return false, nil })
}
func (store *NotificationStore) Admit(ctx context.Context, producer attention.Snapshot) (attention.DeviceSnapshot, error) {
	return store.Update(ctx, func(device *attention.DeviceSnapshot) (bool, error) { return attention.Admit(device, producer) })
}
func (store *NotificationStore) Presented(ctx context.Context, resetID string, key attention.Key, foreground *attention.Foreground, token attention.ReadyToken) (attention.DeviceSnapshot, error) {
	return store.Update(ctx, func(device *attention.DeviceSnapshot) (bool, error) {
		if resetID != device.ResetID {
			return false, nil
		}
		return attention.Presented(device, key, foreground, token), nil
	})
}
func (store *NotificationStore) Reset(ctx context.Context) (attention.DeviceSnapshot, error) {
	return store.Update(ctx, func(device *attention.DeviceSnapshot) (bool, error) {
		var identity [16]byte
		if _, err := rand.Read(identity[:]); err != nil {
			return false, attention.ErrUnavailable
		}
		device.ResetID = hex.EncodeToString(identity[:])
		device.ObserverEpoch = ""
		device.AdmittedRevision = 0
		device.Records = []attention.DeviceRecord{}
		return true, nil
	})
}

// Update holds the stable sidecar lock across read, merge and atomic replacement.
func (store *NotificationStore) Update(ctx context.Context, update func(*attention.DeviceSnapshot) (bool, error)) (snapshot attention.DeviceSnapshot, result error) {
	if err := ctx.Err(); err != nil {
		return attention.DeviceSnapshot{}, err
	}
	if os.MkdirAll(filepath.Dir(store.path), 0700) != nil {
		return attention.DeviceSnapshot{}, attention.ErrUnavailable
	}
	lock, err := os.OpenFile(store.path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return attention.DeviceSnapshot{}, attention.ErrUnavailable
	}
	defer func() {
		if lock.Close() != nil {
			result = attention.ErrUnavailable
		}
	}()
	for {
		err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return attention.DeviceSnapshot{}, attention.ErrUnavailable
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return attention.DeviceSnapshot{}, ctx.Err()
		case <-timer.C:
		}
	}
	defer func() {
		if syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) != nil {
			result = attention.ErrUnavailable
		}
	}()
	if err := ctx.Err(); err != nil {
		return attention.DeviceSnapshot{}, err
	}
	snapshot = attention.DeviceSnapshot{Schema: 4, Records: []attention.DeviceRecord{}}
	file, err := os.Open(store.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return attention.DeviceSnapshot{}, attention.ErrUnavailable
	}
	initializing := errors.Is(err, os.ErrNotExist)
	if initializing {
		var identity [16]byte
		if _, err := rand.Read(identity[:]); err != nil {
			return attention.DeviceSnapshot{}, attention.ErrUnavailable
		}
		snapshot.ResetID = hex.EncodeToString(identity[:])
	} else {
		info, statErr := file.Stat()
		if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			_ = file.Close()
			return attention.DeviceSnapshot{}, attention.ErrUnavailable
		} // justify-ignore-error: private-file admission failure is primary.
		data, readErr := io.ReadAll(io.LimitReader(file, attention.MaximumSnapshotBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || len(data) > attention.MaximumSnapshotBytes || attention.Decode(data, &snapshot) != nil || !snapshot.Valid() {
			return attention.DeviceSnapshot{}, attention.ErrUnavailable
		}
	}
	if err := ctx.Err(); err != nil {
		return attention.DeviceSnapshot{}, err
	}
	changed, err := update(&snapshot)
	if err != nil {
		return attention.DeviceSnapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return attention.DeviceSnapshot{}, err
	}
	if !changed && !initializing {
		return snapshot, nil
	}
	if snapshot.LocalRevision >= math.MaxInt64-1 {
		return attention.DeviceSnapshot{}, attention.ErrUnavailable
	}
	snapshot.LocalRevision++
	if !snapshot.Valid() {
		return attention.DeviceSnapshot{}, attention.ErrUnavailable
	}
	data, err := json.Marshal(snapshot)
	if err != nil || len(data) > attention.MaximumSnapshotBytes {
		return attention.DeviceSnapshot{}, attention.ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return attention.DeviceSnapshot{}, err
	}
	if attention.WriteFile(store.path, data) != nil {
		return attention.DeviceSnapshot{}, attention.ErrUnavailable
	}
	return snapshot, nil
}
