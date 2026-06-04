package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildCompanyBatchesSkipsAlreadySentAndGroupsByCompanyRecipient(t *testing.T) {
	alreadySent := CertificateCandidate{
		CertificateID: 1,
		ExpiryDate:    "2026-06-10",
		Company: CandidateCompany{
			ID:             10,
			Name:           "ABC Sp. z o.o.",
			RecipientEmail: "kadry@abc.pl",
		},
	}

	candidates := []CertificateCandidate{
		alreadySent,
		{
			CertificateID: 2,
			ExpiryDate:    "2026-06-12",
			Company: CandidateCompany{
				ID:             10,
				Name:           "ABC Sp. z o.o.",
				RecipientEmail: "kadry@abc.pl",
			},
		},
		{
			CertificateID: 3,
			ExpiryDate:    "2026-06-11",
			Company: CandidateCompany{
				ID:             10,
				Name:           "ABC Sp. z o.o.",
				RecipientEmail: "kadry@abc.pl",
			},
		},
		{
			CertificateID: 4,
			ExpiryDate:    "2026-06-15",
			Company: CandidateCompany{
				ID:          11,
				CurrentName: "XYZ SA",
			},
		},
	}

	state := State{Sent: map[string]SentNotification{
		notificationKey(alreadySent): {},
	}}

	batches := buildCompanyBatches(candidates, state, 30)

	if len(batches) != 1 {
		t.Fatalf("expected one batch, got %+v", batches)
	}
	if batches[0].CompanyID != 10 || batches[0].RecipientEmail != "kadry@abc.pl" {
		t.Fatalf("unexpected batch: %+v", batches[0])
	}
	if len(batches[0].Candidates) != 2 {
		t.Fatalf("expected two pending candidates, got %+v", batches[0].Candidates)
	}
	if batches[0].Candidates[0].CertificateID != 3 || batches[0].Candidates[1].CertificateID != 2 {
		t.Fatalf("expected candidates sorted by expiry date, got %+v", batches[0].Candidates)
	}
}

func TestRunOnceCreatesStateFileWhenNoPendingNotifications(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/internal/notifications/expiring-certificates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		response := CandidateResponse{
			Data: []CertificateCandidate{},
			Meta: CandidateMeta{
				Limit:   500,
				HasMore: false,
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	stateFile := filepath.Join(t.TempDir(), "notifications-state.json")
	worker := Worker{
		cfg: Config{
			APIBaseURL:    server.URL,
			APIToken:      "token",
			LookaheadDays: 30,
			Limit:         500,
			StateFile:     stateFile,
			DryRun:        true,
		},
		client: server.Client(),
		now: func() time.Time {
			return time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
		},
	}

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}

	state, err := loadState(stateFile)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Sent) != 0 {
		t.Fatalf("expected empty sent state, got %+v", state.Sent)
	}
}

func TestBuildCompanyBatchesSkipsLegacyAlreadySentKey(t *testing.T) {
	candidate := CertificateCandidate{
		CertificateID: 123,
		ExpiryDate:    "2026-06-10",
		Company: CandidateCompany{
			ID:             10,
			Name:           "ABC Sp. z o.o.",
			RecipientEmail: "kadry@abc.pl",
		},
	}

	state := State{Sent: map[string]SentNotification{
		legacyNotificationKey(candidate, 30): {},
	}}

	batches := buildCompanyBatches([]CertificateCandidate{candidate}, state, 30)
	if len(batches) != 0 {
		t.Fatalf("expected legacy sent candidate to be skipped, got %+v", batches)
	}
}

func TestNotificationKeyIncludesCertificateAndExpiry(t *testing.T) {
	candidate := CertificateCandidate{
		CertificateID: 123,
		ExpiryDate:    "2026-06-10",
	}

	if got := notificationKey(candidate); got != "123:2026-06-10" {
		t.Fatalf("unexpected notification key: %q", got)
	}
}

func TestBuildEmailMessageContainsCertificateSummary(t *testing.T) {
	dateEnd := "2025-06-05"
	batch := CompanyBatch{
		CompanyName:    "ABC Sp. z o.o.",
		RecipientEmail: "kadry@abc.pl",
		Candidates: []CertificateCandidate{
			{
				ExpiryDate:     "2026-06-05",
				RegistryYear:   2025,
				RegistryNumber: 45,
				Student: CandidateStudent{
					FirstName: "Jan",
					LastName:  "Nowak",
				},
				Course: CandidateCourse{
					Name:    "Szkolenie okresowe BHP",
					Symbol:  "PK",
					DateEnd: &dateEnd,
				},
			},
		},
	}

	message := buildEmailMessage(batch, Config{LookaheadDays: 30}, "2026-06-01", "2026-06-30")

	if message.To != "kadry@abc.pl" {
		t.Fatalf("unexpected recipient: %q", message.To)
	}
	if message.Subject != "Zaświadczenia wygasające w ciągu 30 dni - ABC Sp. z o.o." {
		t.Fatalf("unexpected subject: %q", message.Subject)
	}
	if !containsAll(message.Body, "<table", "Nr z rejestru", "Jan Nowak", "Szkolenie okresowe BHP", "PK 45/2025", "2026-06-05") {
		t.Fatalf("message body does not contain certificate summary: %s", message.Body)
	}
}

func TestFormatRegistryNumberFallsBackToNumberAndYearWithoutCourseSymbol(t *testing.T) {
	candidate := CertificateCandidate{
		RegistryNumber: 45,
		RegistryYear:   2025,
	}

	if got := formatRegistryNumber(candidate); got != "45/2025" {
		t.Fatalf("unexpected registry number: %q", got)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
