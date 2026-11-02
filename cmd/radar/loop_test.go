package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// TestDigestorKeepsGoingAfterError proves a failing cycle does not stop the loop. The
// fake fails on the 1st call, succeeds on the 2nd and cancels on the 3rd: reaching the
// 3rd call at all means the loop carried on past the error. Were it to give up on the
// error, it would stop at 1 call.
func TestDigestorKeepsGoingAfterError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := 0
	cycle := func() error {
		calls++
		switch calls {
		case 1:
			return errors.New("postgres is down")
		case 3:
			cancel()
		}
		return nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		digestor(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Millisecond, cycle)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("digestor did not return after its context was cancelled")
	}
	// Reading calls after <-done is race-free: the loop goroutine has finished, and
	// closing done happens-before the receive.
	if calls != 3 {
		t.Errorf("cycle called %d times, want 3", calls)
	}
}

// TestDigestorStopsDuringWait proves a cancel in the middle of a long wait ends the
// loop right away instead of after the interval — the "Ctrl+C exits in < 1s" check.
func TestDigestorStopsDuringWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	cycle := func() error { calls++; return nil }

	done := make(chan struct{})
	go func() {
		defer close(done)
		digestor(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour, cycle)
	}()

	time.Sleep(20 * time.Millisecond) // let the first cycle run and the wait begin
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("digestor kept waiting for the 1h tick after cancel")
	}
	if calls != 1 {
		t.Errorf("cycle called %d times, want 1 (only the start-up cycle)", calls)
	}
}
