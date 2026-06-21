package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakePurger struct {
	calls   atomic.Int64
	deleted int64
	err     error
	onCall  func()
}

func (f *fakePurger) DeleteExpiredSessions(context.Context) (int64, error) {
	f.calls.Add(1)
	if f.onCall != nil {
		f.onCall()
	}
	return f.deleted, f.err
}

func TestPurgeExpiredSessionsCallsPurger(t *testing.T) {
	purger := &fakePurger{deleted: 3}
	purgeExpiredSessions(context.Background(), purger)
	if got := purger.calls.Load(); got != 1 {
		t.Fatalf("expected purger to be called once, got %d", got)
	}
}

func TestPurgeExpiredSessionsSwallowsError(t *testing.T) {
	purger := &fakePurger{err: errors.New("db down")}
	// Must not panic and must still have attempted the delete.
	purgeExpiredSessions(context.Background(), purger)
	if got := purger.calls.Load(); got != 1 {
		t.Fatalf("expected purger to be called once, got %d", got)
	}
}

func TestStartSessionCleanupRunsAtStartupAndStopsOnContextCancel(t *testing.T) {
	called := make(chan struct{}, 1)
	purger := &fakePurger{onCall: func() {
		select {
		case called <- struct{}{}:
		default:
		}
	}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		// A long interval ensures only the startup run fires before cancel.
		startSessionCleanup(ctx, purger, time.Hour)
		close(done)
	}()

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected an immediate startup purge")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected cleanup loop to return after context cancellation")
	}

	if got := purger.calls.Load(); got < 1 {
		t.Fatalf("expected at least one purge, got %d", got)
	}
}
