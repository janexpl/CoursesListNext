package main

import (
	"testing"
	"time"
)

func TestPruneStateRemovesEntriesPastRetention(t *testing.T) {
	today := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)

	state := State{Sent: map[string]SentNotification{
		"1:2026-01-01":     {}, // expired well before cutoff -> removed
		"2:2026-06-10":     {}, // recent, within retention -> kept
		"3:2026-12-01":     {}, // future expiry -> kept
		"55:2026-01-01:30": {}, // legacy key, expired -> removed
		"66:2026-06-10:30": {}, // legacy key, recent -> kept
		"77:not-a-date":    {}, // unparseable expiry -> kept
		"weird-key":        {}, // no date field -> kept
	}}

	removed := pruneState(state, today, stateRetention)
	if removed != 2 {
		t.Fatalf("expected 2 entries removed, got %d (state: %+v)", removed, state.Sent)
	}

	wantKept := []string{"2:2026-06-10", "3:2026-12-01", "66:2026-06-10:30", "77:not-a-date", "weird-key"}
	for _, key := range wantKept {
		if _, ok := state.Sent[key]; !ok {
			t.Fatalf("expected key %q to be kept, state: %+v", key, state.Sent)
		}
	}
	for _, key := range []string{"1:2026-01-01", "55:2026-01-01:30"} {
		if _, ok := state.Sent[key]; ok {
			t.Fatalf("expected key %q to be removed, state: %+v", key, state.Sent)
		}
	}
}

func TestPruneStateKeepsEntryExactlyAtCutoff(t *testing.T) {
	today := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	cutoff := today.Add(-stateRetention)

	state := State{Sent: map[string]SentNotification{
		"1:" + cutoff.Format(dateFormat): {}, // expiry == cutoff is not strictly before -> kept
	}}

	if removed := pruneState(state, today, stateRetention); removed != 0 {
		t.Fatalf("expected entry at cutoff to be kept, removed %d", removed)
	}
}
