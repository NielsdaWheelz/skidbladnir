package terminal_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
)

func TestOutboundQueuePreservesFramesAndOwnsTheirBytes(t *testing.T) {
	queue := terminal.NewOutboundQueue()
	binary := []byte("terminal output")
	if err := queue.EnqueueBinary(binary); err != nil {
		t.Fatalf("enqueue binary frame: %v", err)
	}
	binary[0] = 'X'
	if err := queue.EnqueueText([]byte(`{"kind":"Presence","attachedClients":1,"geometry":"Owner"}`)); err != nil {
		t.Fatalf("enqueue text frame: %v", err)
	}

	first, err := queue.Next(context.Background())
	if err != nil {
		t.Fatalf("read binary frame: %v", err)
	}
	if first.Kind != terminal.OutboundBinary || string(first.Payload) != "terminal output" {
		t.Fatalf("unexpected first frame: kind=%v payload=%q", first.Kind, first.Payload)
	}
	second, err := queue.Next(context.Background())
	if err != nil {
		t.Fatalf("read text frame: %v", err)
	}
	if second.Kind != terminal.OutboundText || string(second.Payload) != `{"kind":"Presence","attachedClients":1,"geometry":"Owner"}` {
		t.Fatalf("unexpected second frame: kind=%v payload=%q", second.Kind, second.Payload)
	}
}

func TestOutboundQueueOverflowFailsClosedAndReplaysNothing(t *testing.T) {
	queue := terminal.NewOutboundQueue()
	frame := make([]byte, terminal.MaximumFrameBytes)
	for queued := 0; queued < terminal.MaximumQueueBytes; queued += len(frame) {
		if err := queue.EnqueueBinary(frame); err != nil {
			t.Fatalf("fill queue at %d bytes: %v", queued, err)
		}
	}
	if err := queue.EnqueueBinary([]byte{1}); !errors.Is(err, terminal.ErrBackpressure) {
		t.Fatalf("expected backpressure at byte limit; got %v", err)
	}
	if _, err := queue.Next(context.Background()); !errors.Is(err, terminal.ErrBackpressure) {
		t.Fatalf("expected failed queue to discard pending frames; got %v", err)
	}
	if err := queue.EnqueueBinary([]byte{2}); !errors.Is(err, terminal.ErrBackpressure) {
		t.Fatalf("expected failed queue to remain terminal; got %v", err)
	}
}

func TestOutboundQueueCloseDropsPendingFrames(t *testing.T) {
	queue := terminal.NewOutboundQueue()
	if err := queue.EnqueueBinary([]byte("never replay")); err != nil {
		t.Fatalf("enqueue before close: %v", err)
	}
	queue.Close()
	if _, err := queue.Next(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("expected a closed empty queue; got %v", err)
	}
	if err := queue.EnqueueBinary([]byte("late")); !errors.Is(err, terminal.ErrQueueClosed) {
		t.Fatalf("expected enqueue after close to fail; got %v", err)
	}
}

func TestOutboundQueueNextHonorsCancellation(t *testing.T) {
	queue := terminal.NewOutboundQueue()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := queue.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled wait; got %v", err)
	}
}

func TestOutboundQueueRejectsOversizedFrameByFailingClosed(t *testing.T) {
	queue := terminal.NewOutboundQueue()
	if err := queue.EnqueueBinary(make([]byte, terminal.MaximumFrameBytes+1)); !errors.Is(err, terminal.ErrFrameTooLarge) {
		t.Fatalf("expected oversized-frame error; got %v", err)
	}
	if _, err := queue.Next(context.Background()); !errors.Is(err, terminal.ErrFrameTooLarge) {
		t.Fatalf("expected oversized frame to terminate queue; got %v", err)
	}
}

func TestOutboundQueueTerminationReplacesPendingOutputWithOneFinalTextFrame(t *testing.T) {
	queue := terminal.NewOutboundQueue()
	if err := queue.EnqueueBinary([]byte("discarded PTY output")); err != nil {
		t.Fatalf("enqueue PTY output: %v", err)
	}
	final, err := terminal.EncodeError(terminal.ErrorInvalidRequest)
	if err != nil {
		t.Fatalf("encode terminal error: %v", err)
	}
	if err := queue.TerminateWithText(final); err != nil {
		t.Fatalf("terminate queue: %v", err)
	}
	final[0] = 'X'
	if err := queue.EnqueueBinary([]byte("late PTY output")); !errors.Is(err, terminal.ErrQueueClosed) {
		t.Fatalf("expected terminal queue to reject producers; got %v", err)
	}

	frame, err := queue.Next(context.Background())
	if err != nil {
		t.Fatalf("read final frame: %v", err)
	}
	want := `{"kind":"Error","error":{"code":"InvalidRequest","message":"The request is not valid."}}`
	if frame.Kind != terminal.OutboundText || string(frame.Payload) != want {
		t.Fatalf("unexpected final frame: kind=%v payload=%q", frame.Kind, frame.Payload)
	}
	if _, err := queue.Next(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF after final frame; got %v", err)
	}
}

func TestOutboundQueueTerminationIsConcurrentAndFirstWriterWins(t *testing.T) {
	queue := terminal.NewOutboundQueue()
	final := []byte("final error")
	const callers = 12
	start := make(chan struct{})
	errorsByCaller := make(chan error, callers)
	for range callers {
		go func() {
			<-start
			errorsByCaller <- queue.TerminateWithText(final)
		}()
	}
	close(start)
	for range callers {
		if err := <-errorsByCaller; err != nil {
			t.Fatalf("idempotent termination failed: %v", err)
		}
	}
	if err := queue.TerminateWithText([]byte("replacement")); err != nil {
		t.Fatalf("repeated termination must be an idempotent no-op: %v", err)
	}

	frame, err := queue.Next(context.Background())
	if err != nil {
		t.Fatalf("read final frame: %v", err)
	}
	if frame.Kind != terminal.OutboundText || string(frame.Payload) != string(final) {
		t.Fatalf("unexpected final frame: kind=%v payload=%q", frame.Kind, frame.Payload)
	}
	if _, err := queue.Next(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("expected exactly one final frame; got %v", err)
	}
}

func TestOutboundQueueRejectsInvalidTerminationWithoutChangingIt(t *testing.T) {
	queue := terminal.NewOutboundQueue()
	if err := queue.TerminateWithText(nil); !errors.Is(err, terminal.ErrInvalidFrame) {
		t.Fatalf("expected empty terminal frame to fail: %v", err)
	}
	if err := queue.TerminateWithText(make([]byte, terminal.MaximumFrameBytes+1)); !errors.Is(err, terminal.ErrFrameTooLarge) {
		t.Fatalf("expected oversized terminal frame to fail: %v", err)
	}
	if err := queue.EnqueueBinary([]byte("still open")); err != nil {
		t.Fatalf("invalid termination changed queue state: %v", err)
	}
	frame, err := queue.Next(context.Background())
	if err != nil || string(frame.Payload) != "still open" {
		t.Fatalf("unexpected queue state after invalid termination: frame=%q err=%v", frame.Payload, err)
	}
}

func TestOutboundQueueFailureOutranksFinalText(t *testing.T) {
	queue := terminal.NewOutboundQueue()
	if err := queue.EnqueueBinary(make([]byte, terminal.MaximumFrameBytes+1)); !errors.Is(err, terminal.ErrFrameTooLarge) {
		t.Fatalf("fail queue: %v", err)
	}
	if err := queue.TerminateWithText([]byte("too late")); !errors.Is(err, terminal.ErrFrameTooLarge) {
		t.Fatalf("expected original failure to remain authoritative; got %v", err)
	}
	if _, err := queue.Next(context.Background()); !errors.Is(err, terminal.ErrFrameTooLarge) {
		t.Fatalf("expected original queue failure; got %v", err)
	}
}
