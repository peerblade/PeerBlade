package controlplane

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunWithRetryRecoversAfterFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts atomic.Int32
	done := make(chan struct{})

	go func() {
		defer close(done)
		runWithRetry(ctx, "test worker", time.Millisecond, func(context.Context) error {
			if attempts.Add(1) == 1 {
				return errors.New("temporary failure")
			}
			cancel()
			return nil
		})
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not retry")
	}

	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestRunWithRetryStopsDuringDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		runWithRetry(ctx, "test worker", time.Hour, func(context.Context) error {
			close(started)
			return errors.New("temporary failure")
		})
	}()

	<-started
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}
