package main

import (
	"strings"
	"testing"
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
		notificationKey(alreadySent, 30): {},
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

func TestNotificationKeyIncludesCertificateExpiryAndLookahead(t *testing.T) {
	candidate := CertificateCandidate{
		CertificateID: 123,
		ExpiryDate:    "2026-06-10",
	}

	if got := notificationKey(candidate, 30); got != "123:2026-06-10:30" {
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
	if !containsAll(message.Body, "Jan Nowak", "Szkolenie okresowe BHP", "45/2025", "2026-06-05") {
		t.Fatalf("message body does not contain certificate summary: %s", message.Body)
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
