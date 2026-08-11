package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recordingMailer is a fake Mailer that records recipients, can fail for a
// specific address, and optionally runs a hook on each send.
type recordingMailer struct {
	failFor string
	sentTo  []string
	onSend  func(EmailMessage)
}

func (m *recordingMailer) Send(_ context.Context, _ func() time.Time, msg EmailMessage) error {
	m.sentTo = append(m.sentTo, msg.To)
	if m.onSend != nil {
		m.onSend(msg)
	}
	if msg.To == m.failFor {
		return errors.New("smtp failure")
	}
	return nil
}

// startCandidatesServer serves the given candidates from the expiring
// certificates endpoint in a single page.
func startCandidatesServer(t *testing.T, candidates []CertificateCandidate) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/internal/notifications/expiring-certificates" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		response := CandidateResponse{
			Data: candidates,
			Meta: CandidateMeta{Limit: 500, HasMore: false},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newRunOnceWorker(server *httptest.Server, mailer Mailer, stateFile string) Worker {
	return Worker{
		cfg: Config{
			APIBaseURL:    server.URL,
			APIToken:      "token",
			LookaheadDays: 30,
			Limit:         500,
			StateFile:     stateFile,
			DryRun:        false,
		},
		client: server.Client(),
		mailer: mailer,
		now: func() time.Time {
			return time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
		},
	}
}

// Alfa sorts before Beta, so the failing company is processed first; this
// proves the loop keeps going after a failure instead of aborting.
var twoCompanyCandidates = []CertificateCandidate{
	{
		CertificateID: 1,
		ExpiryDate:    "2026-06-20",
		Company:       CandidateCompany{ID: 10, Name: "Alfa", RecipientEmail: "alfa@example.com"},
	},
	{
		CertificateID: 2,
		ExpiryDate:    "2026-06-21",
		Company:       CandidateCompany{ID: 11, Name: "Beta", RecipientEmail: "beta@example.com"},
	},
}

func TestRunOnceContinuesAfterSendFailure(t *testing.T) {
	server := startCandidatesServer(t, twoCompanyCandidates)
	mailer := &recordingMailer{failFor: "alfa@example.com"}
	stateFile := filepath.Join(t.TempDir(), "state.json")
	worker := newRunOnceWorker(server, mailer, stateFile)

	err := worker.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected aggregated error when a send fails")
	}
	if !strings.Contains(err.Error(), "alfa@example.com") {
		t.Fatalf("expected error to mention the failing recipient, got %v", err)
	}

	if len(mailer.sentTo) != 2 {
		t.Fatalf("expected both companies to be attempted, got %v", mailer.sentTo)
	}

	state, loadErr := loadState(stateFile)
	if loadErr != nil {
		t.Fatalf("load state: %v", loadErr)
	}
	if _, ok := state.Sent["2:2026-06-21:beta@example.com"]; !ok {
		t.Fatalf("expected successful company to be persisted, got %+v", state.Sent)
	}
	if _, ok := state.Sent["1:2026-06-20:alfa@example.com"]; ok {
		t.Fatalf("failed company must not be marked as sent, got %+v", state.Sent)
	}
}

func TestRunOnceReturnsNilAndPersistsAllWhenSendsSucceed(t *testing.T) {
	server := startCandidatesServer(t, twoCompanyCandidates)
	mailer := &recordingMailer{}
	stateFile := filepath.Join(t.TempDir(), "state.json")
	worker := newRunOnceWorker(server, mailer, stateFile)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("expected no error when all sends succeed, got %v", err)
	}
	if len(mailer.sentTo) != 2 {
		t.Fatalf("expected both companies to be sent, got %v", mailer.sentTo)
	}

	state, err := loadState(stateFile)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	for _, key := range []string{"1:2026-06-20:alfa@example.com", "2:2026-06-21:beta@example.com"} {
		if _, ok := state.Sent[key]; !ok {
			t.Fatalf("expected key %q in persisted state, got %+v", key, state.Sent)
		}
	}
}

func TestRunOnceStopsRemainingBatchesWhenContextCancelled(t *testing.T) {
	server := startCandidatesServer(t, twoCompanyCandidates)
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel during the first send so the second batch hits the ctx guard.
	mailer := &recordingMailer{onSend: func(EmailMessage) { cancel() }}
	stateFile := filepath.Join(t.TempDir(), "state.json")
	worker := newRunOnceWorker(server, mailer, stateFile)

	err := worker.RunOnce(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled in aggregated error, got %v", err)
	}
	if len(mailer.sentTo) != 1 {
		t.Fatalf("expected only the first company to be attempted, got %v", mailer.sentTo)
	}

	state, loadErr := loadState(stateFile)
	if loadErr != nil {
		t.Fatalf("load state: %v", loadErr)
	}
	if _, ok := state.Sent["1:2026-06-20:alfa@example.com"]; !ok {
		t.Fatalf("expected first company to be persisted before cancellation, got %+v", state.Sent)
	}
	if _, ok := state.Sent["2:2026-06-21:beta@example.com"]; ok {
		t.Fatalf("second company must not be sent after cancellation, got %+v", state.Sent)
	}
}

func TestRunOnceRetriesOnlyFailedRecipientFromSameCompany(t *testing.T) {
	candidate := CertificateCandidate{
		CertificateID: 1,
		ExpiryDate:    "2026-06-20",
		Company: CandidateCompany{
			ID:             10,
			Name:           "Alfa",
			RecipientEmail: "fail@example.com, ok@example.com",
		},
	}
	server := startCandidatesServer(t, []CertificateCandidate{candidate})
	stateFile := filepath.Join(t.TempDir(), "state.json")

	firstMailer := &recordingMailer{failFor: "fail@example.com"}
	firstWorker := newRunOnceWorker(server, firstMailer, stateFile)
	if err := firstWorker.RunOnce(context.Background()); err == nil {
		t.Fatal("expected first run to report failed recipient")
	}
	if len(firstMailer.sentTo) != 2 {
		t.Fatalf("expected both recipients to be attempted, got %v", firstMailer.sentTo)
	}

	state, err := loadState(stateFile)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if _, ok := state.Sent[recipientNotificationKey(candidate, "ok@example.com")]; !ok {
		t.Fatalf("expected successful recipient to be persisted, got %+v", state.Sent)
	}
	if _, ok := state.Sent[recipientNotificationKey(candidate, "fail@example.com")]; ok {
		t.Fatalf("failed recipient must not be persisted, got %+v", state.Sent)
	}

	retryMailer := &recordingMailer{failFor: "fail@example.com"}
	retryWorker := newRunOnceWorker(server, retryMailer, stateFile)
	if err := retryWorker.RunOnce(context.Background()); err == nil {
		t.Fatal("expected retry to report failed recipient")
	}
	if len(retryMailer.sentTo) != 1 || retryMailer.sentTo[0] != "fail@example.com" {
		t.Fatalf("expected only failed recipient to be retried, got %v", retryMailer.sentTo)
	}
}
