package terminal

import (
	"context"
	"errors"
	"io"
	"sync"
)

const MaximumQueueBytes = 1024 * 1024

var (
	ErrBackpressure = errors.New("terminal queue exceeds 1 MiB")
	ErrQueueClosed  = errors.New("terminal queue is closed")
)

type OutboundKind uint8

const (
	OutboundText OutboundKind = iota + 1
	OutboundBinary
)

type OutboundFrame struct {
	Kind    OutboundKind
	Payload []byte
}

type OutboundQueue struct {
	mutex      sync.Mutex
	frames     []OutboundFrame
	bytes      int
	closed     bool
	terminated bool
	failure    error
	wake       chan struct{}
}

func NewOutboundQueue() *OutboundQueue {
	return &OutboundQueue{wake: make(chan struct{}, 1)}
}

func (queue *OutboundQueue) EnqueueText(payload []byte) error {
	return queue.enqueue(OutboundText, payload)
}

func (queue *OutboundQueue) EnqueueBinary(payload []byte) error {
	return queue.enqueue(OutboundBinary, payload)
}

// Next is consumed by the connection's sole WebSocket writer.
func (queue *OutboundQueue) Next(ctx context.Context) (OutboundFrame, error) {
	for {
		queue.mutex.Lock()
		if queue.failure != nil {
			failure := queue.failure
			queue.mutex.Unlock()
			return OutboundFrame{}, failure
		}
		if queue.closed {
			queue.mutex.Unlock()
			return OutboundFrame{}, io.EOF
		}
		if len(queue.frames) > 0 {
			frame := queue.frames[0]
			queue.frames[0] = OutboundFrame{}
			queue.frames = queue.frames[1:]
			queue.bytes -= len(frame.Payload)
			queue.mutex.Unlock()
			return frame, nil
		}
		if queue.terminated {
			queue.mutex.Unlock()
			return OutboundFrame{}, io.EOF
		}
		wake := queue.wake
		queue.mutex.Unlock()

		select {
		case <-ctx.Done():
			return OutboundFrame{}, ctx.Err()
		case <-wake:
		}
	}
}

// TerminateWithText replaces pending output with one final text frame. The
// first successful call wins; subsequent calls are idempotent.
func (queue *OutboundQueue) TerminateWithText(payload []byte) error {
	if len(payload) == 0 {
		return ErrInvalidFrame
	}
	if len(payload) > MaximumFrameBytes {
		return ErrFrameTooLarge
	}

	queue.mutex.Lock()
	defer queue.mutex.Unlock()
	if queue.failure != nil {
		return queue.failure
	}
	if queue.terminated {
		return nil
	}
	if queue.closed {
		return ErrQueueClosed
	}
	queue.terminated = true
	queue.frames = []OutboundFrame{{Kind: OutboundText, Payload: append([]byte(nil), payload...)}}
	queue.bytes = len(payload)
	queue.signal()
	return nil
}

func (queue *OutboundQueue) Close() {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()
	if queue.closed || queue.failure != nil {
		return
	}
	queue.closed = true
	queue.frames = nil
	queue.bytes = 0
	queue.signal()
}

func (queue *OutboundQueue) enqueue(kind OutboundKind, payload []byte) error {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()
	if queue.failure != nil {
		return queue.failure
	}
	if queue.closed {
		return ErrQueueClosed
	}
	if queue.terminated {
		return ErrQueueClosed
	}
	if len(payload) == 0 {
		return queue.fail(ErrInvalidFrame)
	}
	if len(payload) > MaximumFrameBytes {
		return queue.fail(ErrFrameTooLarge)
	}
	if queue.bytes > MaximumQueueBytes-len(payload) {
		return queue.fail(ErrBackpressure)
	}
	owned := append([]byte(nil), payload...)
	queue.frames = append(queue.frames, OutboundFrame{Kind: kind, Payload: owned})
	queue.bytes += len(owned)
	queue.signal()
	return nil
}

func (queue *OutboundQueue) fail(failure error) error {
	queue.failure = failure
	queue.frames = nil
	queue.bytes = 0
	queue.signal()
	return failure
}

func (queue *OutboundQueue) signal() {
	select {
	case queue.wake <- struct{}{}:
	default:
	}
}
