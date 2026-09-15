package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type fakeIdempotencyKeyPurger struct {
	cutoffs []pgtype.Timestamptz
}

func (f *fakeIdempotencyKeyPurger) DeleteIdempotencyKeysOlderThan(_ context.Context, cutoff pgtype.Timestamptz) (int64, error) {
	f.cutoffs = append(f.cutoffs, cutoff)
	return 0, nil
}

func TestPurgeStaleIdempotencyKeysUsesThirtyDayRetention(t *testing.T) {
	purger := &fakeIdempotencyKeyPurger{}
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)

	purgeStaleIdempotencyKeys(context.Background(), purger, now)

	if len(purger.cutoffs) != 1 {
		t.Fatalf("expected one purge call, got %d", len(purger.cutoffs))
	}
	want := now.Add(-30 * 24 * time.Hour)
	if got := purger.cutoffs[0]; !got.Valid || !got.Time.Equal(want) {
		t.Fatalf("expected cutoff %s, got %+v", want, got)
	}
}
