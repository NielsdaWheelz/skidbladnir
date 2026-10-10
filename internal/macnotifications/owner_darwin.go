//go:build darwin && cgo

package macnotifications

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

type visitState struct {
	connection uint64
	args       RegisterArgs
	readyFence attention.ReadyToken
}

type launchState struct {
	slot    attention.Slot
	surface *SurfaceAssociation
	waiters []ownerRequest
}

type ownerRequest struct {
	ctx        context.Context
	op         string
	connection uint64
	value      any
	reply      chan ownerReply
}

type ownerReply struct {
	value any
	err   error
}

type fetchResult struct {
	generation uint64
	hintFence  uint64
	snapshot   attention.Snapshot
	err        error
}

type receivedHint struct {
	generation uint64
	hint       attention.Hint
}

type nativeNotice struct {
	Slot            string `json:"slot"`
	Episode         int64  `json:"episode"`
	Epoch           string `json:"epoch"`
	Ref             string `json:"ref"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	Silent          bool   `json:"silent,omitempty"`
	ReadyGeneration int64  `json:"readyGeneration"`
}

func (notice nativeNotice) sameIdentity(other nativeNotice) bool {
	return notice.Slot == other.Slot && notice.Epoch == other.Epoch && notice.Episode == other.Episode && notice.Ref == other.Ref && notice.ReadyGeneration == other.ReadyGeneration
}

func (notice nativeNotice) sameCopy(other nativeNotice) bool {
	return notice.sameIdentity(other) && notice.Title == other.Title && notice.Body == other.Body
}

func (notice nativeNotice) acknowledged(epoch string, record attention.DeviceRecord) bool {
	if notice.ReadyGeneration <= 0 || notice.Epoch != epoch || notice.Slot != record.Key.Slot().Identifier() || notice.Episode != record.HandledAttentionEpisode || notice.ReadyGeneration > record.AcknowledgedReadyGeneration {
		return false
	}
	ref, err := fleetclient.DecodeReference(notice.Ref)
	return err == nil && ref.Conversation == nil && ref.Machine == record.Key.Machine && ref.TmuxID == record.Key.TmuxID && ref.IdentityToken == record.Key.IdentityToken && ref.PaneID == record.Key.PaneID
}

type inspection struct {
	notices    []nativeNotice
	known      bool
	focus      string
	allowed    bool
	permission bool
}

type nativeEffect struct {
	key          attention.Key
	notice       nativeNotice
	cancel       bool
	rename       bool
	confirmReset bool
}

type owner struct {
	ctx            context.Context
	client         *fleetclient.Client
	store          *fleetclient.NotificationStore
	configPath     string
	skidPath       string
	requests       chan ownerRequest
	fetched        chan fetchResult
	refresh        chan struct{}
	effects        chan struct{}
	device         attention.DeviceSnapshot
	producer       *attention.Snapshot
	available      bool
	fetching       bool
	hintFloor      attention.Hint
	hintFence      uint64
	refetch        bool
	streamStop     context.CancelFunc
	network        sync.WaitGroup
	generation     uint64
	waiters        []ownerRequest
	visits         map[string]visitState
	launches       map[string]launchState
	focusChecks    map[attention.Slot]int64
	pendingNotices map[string]nativeNotice
	resetting      bool
	health         string
}

func newOwner(ctx context.Context, client *fleetclient.Client, store *fleetclient.NotificationStore, configPath, skidPath string) (*owner, error) {
	device, err := store.Read()
	if err != nil {
		return nil, err
	}
	return &owner{ctx: ctx, client: client, store: store, configPath: configPath, skidPath: skidPath,
		requests: make(chan ownerRequest), fetched: make(chan fetchResult, 1), refresh: make(chan struct{}, 1),
		effects: make(chan struct{}, 1), device: device, visits: make(map[string]visitState),
		hintFloor: attention.Hint{Schema: 1, Epoch: device.ObserverEpoch, Revision: device.AdmittedRevision},
		launches:  make(map[string]launchState), focusChecks: make(map[attention.Slot]int64),
		pendingNotices: make(map[string]nativeNotice), health: "notification setup required."}, nil
}

func (owner *owner) request(ctx context.Context, op string, connection uint64, value any) (any, error) {
	request := ownerRequest{ctx: ctx, op: op, connection: connection, value: value, reply: make(chan ownerReply, 1)}
	select {
	case owner.requests <- request:
	case <-ctx.Done():
		return nil, ErrUnavailable
	}
	select {
	case reply := <-request.reply:
		return reply.value, reply.err
	case <-ctx.Done():
		return nil, ErrUnavailable
	}
}

func (owner *owner) run() {
	owner.startStream()
	defer func() {
		if owner.streamStop != nil {
			owner.streamStop()
		}
		owner.network.Wait()
	}()
	owner.startFetch(owner.ctx)
	for {
		select {
		case <-owner.ctx.Done():
			return
		case <-owner.refresh:
			owner.startFetch(owner.ctx)
		case result := <-owner.fetched:
			owner.fetching = false
			if result.generation != owner.generation || owner.resetting {
				owner.refetch = false
				owner.startFetch(owner.ctx)
				continue
			}
			obsolete := result.hintFence != owner.hintFence || result.err == nil && result.snapshot.Epoch == owner.hintFloor.Epoch && result.snapshot.Revision < owner.hintFloor.Revision
			refetch := owner.refetch
			owner.refetch = false
			owner.available = false
			if refetch && (obsolete || result.err != nil) {
				owner.startFetch(owner.ctx)
				continue
			}
			if result.err == nil && !obsolete {
				device, err := owner.store.Admit(result.snapshot)
				if err == nil && result.snapshot.Revision >= owner.device.AdmittedRevision {
					owner.device, owner.producer, owner.available = device, &result.snapshot, true
					if owner.hintFloor.Epoch != result.snapshot.Epoch {
						owner.hintFloor = attention.Hint{Schema: 1, Epoch: result.snapshot.Epoch, Revision: result.snapshot.Revision}
					} else {
						owner.hintFloor.Revision = max(owner.hintFloor.Revision, result.snapshot.Revision)
					}
					owner.health = "allow notifications for needs input."
				} else if errors.Is(err, attention.ErrEpochChanged) {
					owner.health = "notification memory reset required."
				} else {
					owner.health = "notification delivery unavailable."
				}
			} else {
				owner.health = "notification delivery unavailable."
			}
			for _, waiter := range owner.waiters {
				waiter.reply <- ownerReply{owner.view(), nil}
			}
			owner.waiters = nil
			owner.signalEffects()
		case request := <-owner.requests:
			if request.ctx.Err() != nil {
				continue
			}
			owner.handle(request)
		}
	}
}

func (owner *owner) view() View {
	return View{DeviceSnapshot: owner.device, ProducerAvailable: owner.available, CurrentProducerSnapshot: owner.producer}
}

func (owner *owner) startFetch(ctx context.Context) {
	if owner.resetting {
		owner.signalEffects()
		for _, waiter := range owner.waiters {
			waiter.reply <- ownerReply{owner.view(), nil}
		}
		owner.waiters = nil
		return
	}
	if owner.fetching {
		return
	}
	if owner.client == nil {
		owner.available = false
		owner.health = "notification setup required."
		for _, waiter := range owner.waiters {
			waiter.reply <- ownerReply{owner.view(), nil}
		}
		owner.waiters = nil
		owner.signalEffects()
		return
	}
	if _, configured := owner.client.NotificationConfig(); !configured {
		owner.available = false
		owner.health = "notification setup required."
		for _, waiter := range owner.waiters {
			waiter.reply <- ownerReply{owner.view(), nil}
		}
		owner.waiters = nil
		owner.signalEffects()
		return
	}
	owner.fetching = true
	generation := owner.generation
	hintFence := owner.hintFence
	client := owner.client
	owner.network.Go(func() {
		ctx, cancel := context.WithTimeout(ctx, operationTimeout)
		defer cancel()
		snapshot, err := client.NotificationState(ctx)
		select {
		case owner.fetched <- fetchResult{generation: generation, hintFence: hintFence, snapshot: snapshot, err: err}:
		case <-owner.ctx.Done():
		}
	})
}

func (owner *owner) signalEffects() {
	select {
	case owner.effects <- struct{}{}:
	default:
	}
}

func (owner *owner) handle(request ownerRequest) {
	answer := func(value any, err error) { request.reply <- ownerReply{value, err} }
	switch request.op {
	case "hint":
		received := request.value.(receivedHint) // justify-assertion: the configured stream submits its typed hint and owner generation.
		if received.generation != owner.generation {
			answer(struct{}{}, nil)
			return
		}
		hint := received.hint
		if hint.Epoch == owner.hintFloor.Epoch {
			owner.hintFloor.Revision = max(owner.hintFloor.Revision, hint.Revision)
		} else {
			// Different epochs have no order. Invalidate only reads already
			// dispatched; the following snapshot remains authoritative.
			owner.hintFence++
			if owner.hintFloor.Epoch == "" {
				owner.hintFloor = hint
			}
		}
		if owner.producer == nil || hint.Epoch != owner.producer.Epoch || hint.Revision > owner.producer.Revision {
			owner.available = false
			owner.health = "notification delivery unavailable."
		}
		if owner.fetching {
			owner.refetch = true
		} else {
			owner.startFetch(owner.ctx)
		}
		answer(struct{}{}, nil)
	case "read":
		owner.waiters = append(owner.waiters, request)
		owner.startFetch(request.ctx)
	case "register":
		args := request.value.(RegisterArgs) // justify-assertion: socket admission decodes this operation's owning shape.
		if args.LaunchNonce != "" && args.Surface == nil {
			launch, found := owner.launches[args.LaunchNonce]
			if !found || launch.slot != args.Key.Slot() {
				answer(nil, ErrUnavailable)
				return
			}
			if launch.surface == nil {
				launch.waiters = append(launch.waiters, request)
				owner.launches[args.LaunchNonce] = launch
				return
			}
			args.Surface = launch.surface
		}
		id := randomID()
		fence := attention.ReadyToken{Epoch: owner.device.ObserverEpoch}
		if record, found := owner.device.Record(args.Key); found && attention.SameForeground(record.Foreground, args.Foreground) {
			fence.Generation = record.ReceivedReadyGeneration
		}
		owner.visits[id] = visitState{connection: request.connection, args: args, readyFence: fence}
		if args.LaunchNonce != "" {
			delete(owner.launches, args.LaunchNonce)
		}
		answer(registration{id, args.Surface}, nil)
		owner.signalEffects()
	case "presented":
		args := request.value.(presentedArgs) // justify-assertion: socket admission owns this closed operation.
		visit, found := owner.visits[args.VisitID]
		if !found || visit.connection != request.connection {
			answer(nil, ErrUnavailable)
			return
		}
		if visit.args.Presented {
			answer(struct{}{}, nil)
			return
		}
		visit.args.Presented = true
		if record, found := owner.device.Record(visit.args.Key); found && attention.SameForeground(record.Foreground, visit.args.Foreground) {
			visit.readyFence = attention.ReadyToken{Epoch: owner.device.ObserverEpoch, Generation: record.ReceivedReadyGeneration}
		}
		owner.visits[args.VisitID] = visit
		if visit.args.ReadyToken != nil && args.Token != *visit.args.ReadyToken || visit.args.ReadyToken == nil && args.Token.Generation != 0 {
			answer(nil, ErrUnavailable)
			owner.signalEffects()
			return
		}
		device, err := owner.store.Presented(visit.args.Key, visit.args.Foreground, args.Token)
		if err == nil {
			owner.device = device
		}
		answer(struct{}{}, err)
		owner.signalEffects()
	case "end":
		id := request.value.(string) // justify-assertion: socket admission owns this closed operation.
		if visit, found := owner.visits[id]; found && visit.connection == request.connection {
			delete(owner.visits, id)
		}
		answer(struct{}{}, nil)
	case "disconnect":
		for id, visit := range owner.visits {
			if visit.connection == request.connection {
				delete(owner.visits, id)
			}
		}
		answer(struct{}{}, nil)
	case "focus-needed":
		needed := false
		if owner.available && owner.producer != nil {
			for _, host := range owner.producer.Machines {
				for _, row := range host.Sessions {
					record, found := owner.device.Record(row.Record.Key)
					checked := owner.focusChecks[row.Record.Key.Slot()]
					if !found || !attention.Current(row) || record.HandledAttentionEpisode == row.Record.AttentionEpisode && checked >= row.Record.ReadyGeneration {
						continue
					}
					for _, visit := range owner.visits {
						if visit.args.Presented && visit.args.Surface != nil && visit.args.Key.Slot() == row.Record.Key.Slot() {
							needed = true
						}
					}
				}
			}
		}
		answer(needed, nil)
	case "plan":
		plan, err := owner.plan(request.value.(inspection)) // justify-assertion: only the single native lane submits inspections.
		answer(plan, err)
	case "health":
		answer(owner.health, nil)
	case "reload":
		client, err := fleetclient.Open(owner.configPath)
		if err != nil {
			client = nil
		}
		owner.client, owner.available = client, false
		owner.generation++
		owner.hintFloor = attention.Hint{Schema: 1, Epoch: owner.device.ObserverEpoch, Revision: owner.device.AdmittedRevision}
		owner.hintFence, owner.refetch = 0, false
		if owner.streamStop != nil {
			owner.streamStop()
			owner.streamStop = nil
		}
		owner.startStream()
		owner.startFetch(owner.ctx)
		answer(struct{}{}, nil)
	case "submit":
		effect := request.value.(nativeEffect) // justify-assertion: the native lane retains the plan's typed effects.
		if !owner.authorize(effect) {
			answer((*nativeReceipt)(nil), nil)
			return
		}
		// Keep eligibility and actual add initiation in one owner turn. Native
		// completion is awaited by the effect lane, never by this state owner.
		operation, value := "post", any(effect.notice)
		if effect.cancel {
			operation, value = "cancel", struct {
				Slots []string `json:"slots"`
			}{[]string{effect.notice.Slot}}
		}
		receipt, err := bridge.begin(operation, value)
		if err == nil && !effect.cancel {
			owner.pendingNotices[effect.notice.Slot] = effect.notice
		}
		answer(receipt, err)
	case "rejected":
		effect := request.value.(nativeEffect) // justify-assertion: the native lane reports only definite post rejection.
		if pending, found := owner.pendingNotices[effect.notice.Slot]; found && pending == effect.notice {
			delete(owner.pendingNotices, effect.notice.Slot)
		}
		answer(struct{}{}, nil)
	case "settle":
		effect := request.value.(nativeEffect) // justify-assertion: the native lane reports its completed effect.
		cancel, err := owner.settle(effect)
		answer(cancel, err)
	case "reset":
		// Keep cancellation intent durable until the native lane confirms that
		// no owned copy or unresolved add remains. Re-admission waits for removal.
		device, err := owner.store.Update(func(device *attention.DeviceSnapshot) (bool, error) {
			changed := false
			for index := range device.Records {
				if device.Records[index].Presentation != attention.Closed {
					device.Records[index].Presentation = attention.Closed
					changed = true
				}
			}
			return changed, nil
		})
		if err == nil {
			owner.device, owner.producer, owner.available = device, nil, false
			owner.resetting = true
			owner.generation++
			owner.hintFloor, owner.hintFence, owner.refetch = attention.Hint{}, 0, false
			owner.focusChecks = make(map[attention.Slot]int64)
			for id, visit := range owner.visits {
				visit.readyFence = attention.ReadyToken{}
				owner.visits[id] = visit
			}
			owner.signalEffects()
			if owner.streamStop != nil {
				owner.streamStop()
				owner.streamStop = nil
			}
		}
		answer(struct{}{}, err)
	case "launch":
		ref := request.value.(string) // justify-assertion: native events carry only their captured session reference.
		decoded, err := fleetclient.DecodeReference(ref)
		if err != nil || decoded.Conversation != nil {
			answer(nil, ErrUnavailable)
			return
		}
		nonce := randomID()
		script, err := ghosttyScript(owner.skidPath, owner.configPath, ref, nonce)
		if err != nil {
			answer(nil, err)
			return
		}
		owner.launches[nonce] = launchState{slot: attention.Slot{Machine: decoded.Machine, TmuxID: decoded.TmuxID, IdentityToken: decoded.IdentityToken}}
		answer(struct {
			Nonce  string
			Script string
		}{nonce, script}, nil)
	case "launched":
		result := request.value.(launchResult) // justify-assertion: only launch completion submits this result.
		launch, found := owner.launches[result.nonce]
		if found {
			if result.id != "" {
				launch.surface = &SurfaceAssociation{Nonce: result.nonce, ID: result.id}
				owner.launches[result.nonce] = launch
			}
			for _, waiter := range launch.waiters {
				if result.id == "" || waiter.ctx.Err() != nil {
					waiter.reply <- ownerReply{nil, ErrUnavailable}
					continue
				}
				args := waiter.value.(RegisterArgs) // justify-assertion: only admitted pending registrations wait for their creator.
				args.Surface = launch.surface
				waiter.value = args
				owner.handle(waiter)
			}
			if result.id == "" {
				delete(owner.launches, result.nonce)
			} else if len(launch.waiters) != 0 {
				// Their successful registration consumes the creator receipt.
				delete(owner.launches, result.nonce)
			}
		}
		answer(struct{}{}, nil)
	case "dismiss", "absent":
		event := request.value.(nativeEvent) // justify-assertion: native adapter callbacks carry captured metadata.
		if request.op == "absent" {
			if pending, found := owner.pendingNotices[event.Slot]; found && pending.Epoch == event.Epoch && pending.Episode == event.Episode {
				answer(struct{}{}, nil)
				return
			}
		}
		device, err := owner.store.Update(func(device *attention.DeviceSnapshot) (bool, error) {
			for index := range device.Records {
				record := &device.Records[index]
				if device.ObserverEpoch == event.Epoch && record.Key.Slot().Identifier() == event.Slot && record.HandledAttentionEpisode == event.Episode && record.Presentation != attention.Closed {
					record.Presentation = attention.Closed
					return true, nil
				}
			}
			return false, nil
		})
		if err == nil {
			owner.device = device
		}
		answer(struct{}{}, err)
		owner.signalEffects()
	default:
		panic("unsupported mac notification owner operation")
	}
}

type launchResult struct{ nonce, id string }

func randomID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic("notification identity randomness unavailable")
	} // justify-defect: the os cryptographic source is required for ephemeral identities.
	return hex.EncodeToString(bytes[:])
}

func (owner *owner) plan(observation inspection) ([]nativeEffect, error) {
	if !observation.known {
		observation.notices = nil
	}
	if owner.resetting && observation.known && len(observation.notices) == 0 && len(owner.pendingNotices) == 0 {
		device, err := owner.store.Reset()
		if err == nil {
			owner.device, owner.resetting = device, false
			owner.startStream()
			owner.startFetch(owner.ctx)
		}
		return nil, err
	}
	plan := []nativeEffect{}
	device, err := owner.store.Update(func(device *attention.DeviceSnapshot) (bool, error) {
		changed := false
		for index := range device.Records {
			record := &device.Records[index]
			row, fresh := attention.SessionSnapshot{}, false
			if owner.available && owner.producer != nil {
				row, fresh = owner.producer.Session(record.Key)
			}
			pending, submitted := owner.pendingNotices[record.Key.Slot().Identifier()]
			var notice *nativeNotice
			for i := range observation.notices {
				item := &observation.notices[i]
				if item.Epoch == device.ObserverEpoch && item.Slot == record.Key.Slot().Identifier() && item.Episode == record.HandledAttentionEpisode {
					if notice == nil || submitted && pending.sameCopy(*item) || !submitted && fresh && item.Title == attention.SingleLine(row.Name) {
						notice = item
					}
				}
			}
			if submitted && notice != nil && pending.sameCopy(*notice) {
				delete(owner.pendingNotices, pending.Slot)
				submitted = false
			}
			unresolved := submitted && pending.Epoch == device.ObserverEpoch && pending.Episode == record.HandledAttentionEpisode
			if observation.known && record.Presentation == attention.Claimed && notice != nil && !unresolved {
				record.Presentation = attention.Posted
				changed = true
			} else if observation.known && record.Presentation == attention.Posted && notice == nil && !unresolved {
				record.Presentation = attention.Closed
				changed = true
			}
			acknowledged := observation.known && notice != nil && notice.acknowledged(device.ObserverEpoch, *record) || unresolved && pending.acknowledged(device.ObserverEpoch, *record)
			if acknowledged && record.Presentation != attention.Closed {
				record.Presentation = attention.Closed
				changed = true
			}
			foreground := false
			for id, visit := range owner.visits {
				if visit.args.Key.Slot() != record.Key.Slot() {
					continue
				}
				if visit.readyFence.Epoch != device.ObserverEpoch && visit.args.Key == record.Key && attention.SameForeground(visit.args.Foreground, record.Foreground) {
					visit.readyFence = attention.ReadyToken{Epoch: device.ObserverEpoch, Generation: record.ReceivedReadyGeneration}
					owner.visits[id] = visit
				}
				if !fresh || !attention.Current(row) || observation.focus == "" || !visit.args.Presented || visit.args.Surface == nil || visit.args.Surface.ID != observation.focus {
					continue
				}
				foreground = true
				if visit.args.Key == record.Key && attention.SameForeground(visit.args.Foreground, record.Foreground) && record.ReceivedReadyGeneration > visit.readyFence.Generation && device.Pending(row) {
					if attention.Presented(device, record.Key, record.Foreground, attention.ReadyToken{Epoch: device.ObserverEpoch, Generation: record.ReceivedReadyGeneration}) {
						changed = true
					}
				}
			}
			if fresh && attention.Current(row) {
				owner.focusChecks[record.Key.Slot()] = row.Record.ReadyGeneration
			}
			current := fresh && attention.Current(row) && (row.Record.Cause.Kind != "ready" || device.Pending(row))
			cleared := fresh && row.AttentionQualified && (!current || row.Record.AttentionEpisode != record.HandledAttentionEpisode)
			if cleared && record.Presentation != attention.Closed {
				record.Presentation = attention.Closed
				changed = true
			}
			if current && record.HandledAttentionEpisode < row.Record.AttentionEpisode {
				if foreground || observation.known && observation.permission && observation.allowed {
					record.HandledAttentionEpisode = row.Record.AttentionEpisode
					record.Presentation = attention.Claimed
					if foreground {
						record.Presentation = attention.Closed
					}
					changed = true
				}
			}
			if notice != nil && notice.Episode != record.HandledAttentionEpisode {
				notice = nil
			}
			unresolved = submitted && pending.Epoch == device.ObserverEpoch && pending.Episode == record.HandledAttentionEpisode
			if fresh && record.Presentation == attention.Posted && notice != nil && record.HandledAttentionEpisode == row.Record.AttentionEpisode && notice.Title != attention.SingleLine(row.Name) {
				renamed := *notice
				renamed.Title, renamed.Silent = attention.SingleLine(row.Name), true
				if !unresolved || !pending.sameCopy(renamed) {
					plan = append(plan, nativeEffect{key: record.Key, notice: renamed, rename: true})
				}
				continue
			}
			if !current || record.HandledAttentionEpisode != row.Record.AttentionEpisode || record.Presentation == attention.Closed {
				continue
			}
			desired := nativeNotice{Slot: record.Key.Slot().Identifier(), Epoch: device.ObserverEpoch, Episode: record.HandledAttentionEpisode, Ref: row.Ref, Title: attention.SingleLine(row.Name), Body: row.Record.Cause.Text()}
			if row.Record.Cause.Kind == "ready" {
				desired.ReadyGeneration = row.Record.ReadyGeneration
			}
			if observation.known && record.Presentation == attention.Claimed && notice == nil && !unresolved && observation.permission && observation.allowed {
				// A claim already present at inspection permits only silent repair.
				previous, exists := owner.device.Record(record.Key)
				desired.Silent = exists && previous.HandledAttentionEpisode == record.HandledAttentionEpisode
				plan = append(plan, nativeEffect{key: record.Key, notice: desired})
			}
		}
		return changed, nil
	})
	if err != nil {
		return nil, err
	}
	owner.device = device
	notices := []nativeNotice{}
	for _, pending := range owner.pendingNotices {
		notices = append(notices, pending)
	}
	notices = append(notices, observation.notices...)
	cancellations := []nativeEffect{}
	retained := make(map[string]bool)
	for _, notice := range notices {
		keep := slices.ContainsFunc(device.Records, func(record attention.DeviceRecord) bool {
			return device.ObserverEpoch == notice.Epoch && record.Key.Slot().Identifier() == notice.Slot && record.HandledAttentionEpisode == notice.Episode && record.Presentation != attention.Closed
		})
		if keep {
			retained[notice.Slot] = true
		}
	}
	for _, notice := range notices {
		if !retained[notice.Slot] && !slices.ContainsFunc(cancellations, func(effect nativeEffect) bool { return effect.notice.Slot == notice.Slot }) {
			cancellations = append(cancellations, nativeEffect{notice: notice, cancel: true, confirmReset: owner.resetting})
		}
	}
	return append(cancellations, plan...), nil
}

func (owner *owner) authorize(effect nativeEffect) bool {
	if effect.cancel {
		pending, found := owner.pendingNotices[effect.notice.Slot]
		return !found || pending.sameIdentity(effect.notice)
	}
	if pending, found := owner.pendingNotices[effect.notice.Slot]; found && pending.sameCopy(effect.notice) {
		return false
	}
	if effect.notice.Epoch != owner.device.ObserverEpoch {
		return false
	}
	if !owner.available || owner.producer == nil {
		return false
	}
	row, found := owner.producer.Session(effect.key)
	record, exists := owner.device.Record(effect.key)
	if effect.rename {
		return found && exists && row.Record.AttentionEpisode == effect.notice.Episode && record.HandledAttentionEpisode == effect.notice.Episode && record.Presentation == attention.Posted
	}
	return found && exists && attention.Current(row) && row.Record.AttentionEpisode == effect.notice.Episode && record.HandledAttentionEpisode == effect.notice.Episode && record.Presentation != attention.Closed && (row.Record.Cause.Kind != "ready" || owner.device.Pending(row))
}

func (owner *owner) settle(effect nativeEffect) (bool, error) {
	if effect.cancel {
		if pending, found := owner.pendingNotices[effect.notice.Slot]; found && pending.sameIdentity(effect.notice) {
			delete(owner.pendingNotices, effect.notice.Slot)
			if owner.resetting {
				owner.signalEffects()
			}
		}
		return false, nil
	}
	if effect.notice.Epoch != owner.device.ObserverEpoch {
		return true, nil
	}
	cancel := false
	device, err := owner.store.Update(func(device *attention.DeviceSnapshot) (bool, error) {
		for index := range device.Records {
			record := &device.Records[index]
			if record.Key != effect.key || record.HandledAttentionEpisode != effect.notice.Episode || record.Presentation == attention.Closed {
				continue
			}
			if effect.notice.acknowledged(device.ObserverEpoch, *record) {
				record.Presentation, cancel = attention.Closed, true
				return true, nil
			}
			if owner.available && owner.producer != nil {
				row, fresh := owner.producer.Session(effect.key)
				if fresh && row.AttentionQualified && (!attention.Current(row) || row.Record.AttentionEpisode != effect.notice.Episode || row.Record.Cause.Kind == "ready" && !device.Pending(row)) {
					record.Presentation, cancel = attention.Closed, true
					return true, nil
				}
			}
			return false, nil
		}
		cancel = true
		return false, nil
	})
	if err == nil {
		owner.device = device
	}
	return cancel, err
}

func (owner *owner) startStream() {
	if owner.client == nil || owner.resetting {
		return
	}
	if _, configured := owner.client.NotificationConfig(); !configured {
		return
	}
	ctx, stop := context.WithCancel(owner.ctx)
	owner.streamStop = stop
	client := owner.client
	generation := owner.generation
	owner.network.Go(func() { owner.stream(ctx, client, generation) })
}

func (owner *owner) stream(ctx context.Context, client *fleetclient.Client, generation uint64) {
	const maximumDelay = 30 * time.Second
	backoff := time.Second
	for ctx.Err() == nil {
		started := time.Now()
		_ = client.NotificationEvents(ctx, func(hint attention.Hint) error {
			_, err := owner.request(ctx, "hint", 0, receivedHint{generation: generation, hint: hint})
			return err
		}) // justify-ignore-error: every stream ending retries; a hint alone does not prove connection stability.
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) >= maximumDelay {
			backoff = time.Second
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(2*backoff, maximumDelay)
	}
}
