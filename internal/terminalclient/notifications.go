package terminalclient

import (
	"context"
	"runtime"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/macnotifications"
)

// NotificationView joins producer evidence with this device's acknowledgement.
// A direct inventory can qualify that evidence, but cannot create readiness.
type NotificationView struct {
	Device   attention.DeviceSnapshot
	Producer attention.Snapshot
}

func ReadNotifications(ctx context.Context, client *fleetclient.Client) (NotificationView, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if runtime.GOOS == "darwin" {
		view, err := macnotifications.Read(ctx)
		if err != nil {
			return NotificationView{}, attention.ErrUnavailable
		}
		if !view.ProducerAvailable || view.CurrentProducerSnapshot == nil {
			return NotificationView{Device: view.DeviceSnapshot}, attention.ErrUnavailable
		}
		return NotificationView{Device: view.DeviceSnapshot, Producer: *view.CurrentProducerSnapshot}, nil
	}
	store, err := fleetclient.DefaultNotificationStore()
	if err != nil {
		return NotificationView{}, err
	}
	producer, err := client.NotificationState(ctx)
	if err != nil {
		device, readErr := store.Read()
		if readErr != nil {
			return NotificationView{}, readErr
		}
		return NotificationView{Device: device}, err
	}
	device, err := store.Admit(producer)
	if err != nil {
		return NotificationView{}, err
	}
	return NotificationView{Device: device, Producer: producer}, nil
}

func (view NotificationView) Ready(session fleetclient.Session) bool {
	return fleetclient.NotificationReady(view.Device, session, view.Producer)
}

func (view NotificationView) readyToken(key attention.Key, foreground *attention.Foreground) attention.ReadyToken {
	record, found := view.Device.Record(key)
	if !found || !attention.SameForeground(record.Foreground, foreground) {
		return attention.ReadyToken{}
	}
	return attention.ReadyToken{Epoch: view.Device.ObserverEpoch, Generation: record.ReceivedReadyGeneration}
}
