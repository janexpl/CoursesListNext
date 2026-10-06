// Package webhooks zapisuje zdarzenia dla odbiorców zewnętrznych (platforma e-learningowa)
// i doręcza je z bazy danych.
//
// Zdarzenie powstaje w tej samej transakcji co zmiana, której dotyczy (Publisher), a wysyła
// je Dispatcher po zatwierdzeniu transakcji. Ciało zdarzenia jest budowane raz, przy zapisie,
// i wysyłane przy każdej próbie bajt w bajt - podpis HMAC liczony jest z tych bajtów.
//
// Pola ciała są w snake_case - celowo inaczej niż camelCase w REST API (kontrakt odbiorcy).
package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

const (
	EventCertificateIssued          = "certificate.issued"
	EventCertificateDuplicateIssued = "certificate.duplicate_issued"
	EventCertificateRevoked         = "certificate.revoked"
	// EventCertificateRenewed - kursant ma już nowe zaświadczenie z tego kursu.
	// Nazwa mówi, co się stało; poprzednik NIE przestaje być ważny, więc nie jest to
	// "superseded" z wycofanego modelu (migracja 0023 wprowadzała supersedes_id,
	// 0025 go skasowała).
	EventCertificateRenewed         = "certificate.renewed"
	EventCertificateValidityChanged = "certificate.validity_changed"
	EventProgramUpdated             = "program.updated"
)

// timestampFormat to ISO 8601 w UTC z Z. Mikrosekundy gwarantują, że znaczniki kolejnych
// zdarzeń tego samego dokumentu są różne.
const timestampFormat = "2006-01-02T15:04:05.000000Z"

// Publisher zapisuje zdarzenia w transakcji wywołującego. Zero-wartość jest gotowa do użycia.
type Publisher struct {
	// PublicBaseURL - publiczny adres API (bez /api/v1) do budowy pdf_url; pusty pomija pole.
	PublicBaseURL string
}

func NewPublisher(publicBaseURL string) *Publisher {
	return &Publisher{PublicBaseURL: strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")}
}

type certificateIssuedPayload struct {
	Event             string  `json:"event"`
	Timestamp         string  `json:"timestamp"`
	IdempotencyKey    string  `json:"idempotency_key"`
	CertificateNumber string  `json:"certificate_number"`
	IssuedAt          string  `json:"issued_at"`
	ValidUntil        *string `json:"valid_until,omitempty"`
	PDFURL            string  `json:"pdf_url,omitempty"`
	VerificationCode  string  `json:"verification_code,omitempty"`
}

// certificateDuplicateIssuedPayload - wtórnik tego samego dokumentu. Numer rejestru,
// kod weryfikacyjny i ważność się nie zmieniają, więc odbiorca tylko odnotowuje fakt
// i datę wystawienia duplikatu.
type certificateDuplicateIssuedPayload struct {
	Event             string `json:"event"`
	Timestamp         string `json:"timestamp"`
	CertificateNumber string `json:"certificate_number"`
	DuplicateIssuedAt string `json:"duplicate_issued_at"`
	Reason            string `json:"reason"`
}

type certificateRevokedPayload struct {
	Event             string `json:"event"`
	Timestamp         string `json:"timestamp"`
	CertificateNumber string `json:"certificate_number"`
	Reason            string `json:"reason"`
}

// certificateRenewedPayload - dokument został przedłużony, więc odbiorca ma przestać
// przypominać o tym numerze. Niesie dane OBU dokumentów, bo kolejność zdarzeń między
// podmiotami nie jest gwarantowana: certificate.issued następcy może dojść później.
type certificateRenewedPayload struct {
	Event                      string  `json:"event"`
	Timestamp                  string  `json:"timestamp"`
	CertificateNumber          string  `json:"certificate_number"`
	ValidUntil                 *string `json:"valid_until"`
	RenewedAt                  string  `json:"renewed_at"`
	RenewedByCertificateNumber string  `json:"renewed_by_certificate_number"`
	RenewedByIssuedAt          string  `json:"renewed_by_issued_at"`
	RenewedByValidUntil        *string `json:"renewed_by_valid_until"`
	RenewedByVerificationCode  string  `json:"renewed_by_verification_code"`
}

type certificateValidityChangedPayload struct {
	Event             string  `json:"event"`
	Timestamp         string  `json:"timestamp"`
	CertificateNumber string  `json:"certificate_number"`
	ValidUntil        *string `json:"valid_until"`
}

type programUpdatedPayload struct {
	Event             string `json:"event"`
	Timestamp         string `json:"timestamp"`
	ExternalProgramID int64  `json:"external_program_id"`
}

// IsPlatformCertificate mówi, czy zdarzenia o dokumencie są wysyłane. Odbiorca rozpoznaje
// dokumenty po kluczu idempotencji, więc dotyczy to tylko zaświadczeń wystawionych
// z Idempotency-Key (i ich duplikatów, które dziedziczą klucz).
func IsPlatformCertificate(cert dbsqlc.GetCertificateByIDRow) bool {
	return cert.IdempotencyKey.Valid && cert.IdempotencyKey.String != ""
}

// CertificateIssued zapisuje certificate.issued (wystawienie lub duplikat).
func (p *Publisher) CertificateIssued(ctx context.Context, q *dbsqlc.Queries, cert dbsqlc.GetCertificateByIDRow) error {
	if !IsPlatformCertificate(cert) {
		return nil
	}
	return p.enqueue(ctx, q, EventCertificateIssued, certificateSubject(cert.ID), func(timestamp string) any {
		payload := certificateIssuedPayload{
			Event:             EventCertificateIssued,
			Timestamp:         timestamp,
			IdempotencyKey:    cert.IdempotencyKey.String,
			CertificateNumber: CertificateNumber(cert),
			IssuedAt:          cert.Date.Time.Format(time.DateOnly),
			ValidUntil:        validUntil(cert),
			VerificationCode:  cert.VerificationCode,
		}
		if p.PublicBaseURL != "" {
			payload.PDFURL = fmt.Sprintf("%s/api/v1/certificates/%d/pdf", p.PublicBaseURL, cert.ID)
		}
		return payload
	})
}

// CertificateRevoked zapisuje certificate.revoked.
func (p *Publisher) CertificateRevoked(ctx context.Context, q *dbsqlc.Queries, cert dbsqlc.GetCertificateByIDRow, reason string) error {
	if !IsPlatformCertificate(cert) {
		return nil
	}
	return p.enqueue(ctx, q, EventCertificateRevoked, certificateSubject(cert.ID), func(timestamp string) any {
		return certificateRevokedPayload{
			Event:             EventCertificateRevoked,
			Timestamp:         timestamp,
			CertificateNumber: CertificateNumber(cert),
			Reason:            reason,
		}
	})
}

// CertificateDuplicateIssued zapisuje certificate.duplicate_issued: na dokumencie
// wystawiono wtórnik. Dokument zachowuje numer, kod weryfikacyjny i ważność.
func (p *Publisher) CertificateDuplicateIssued(ctx context.Context, q *dbsqlc.Queries, cert dbsqlc.GetCertificateByIDRow, reason string) error {
	if !IsPlatformCertificate(cert) {
		return nil
	}
	return p.enqueue(ctx, q, EventCertificateDuplicateIssued, certificateSubject(cert.ID), func(timestamp string) any {
		return certificateDuplicateIssuedPayload{
			Event:             EventCertificateDuplicateIssued,
			Timestamp:         timestamp,
			CertificateNumber: CertificateNumber(cert),
			DuplicateIssuedAt: cert.DuplicateIssuedAt.Time.Format(time.DateOnly),
			Reason:            reason,
		}
	})
}

// CertificateRenewed zapisuje certificate.renewed na PRZEDŁUŻONYM dokumencie.
//
// Bramka stoi na poprzedniku, nie na następcy: przedłużenie dokumentu platformowego
// z aplikacji webowej ma dojść do odbiorcy, a przedłużenie dokumentu, którego odbiorca
// nigdy nie widział, nie ma po co iść - dostalibyśmy 422 i WEBHOOK ALERT przy każdym
// przedłużeniu wystawionym w aplikacji. Zdarzenie certificate.issued następcy rządzi
// się własnym kryterium (ma klucz idempotencji albo nie).
func (p *Publisher) CertificateRenewed(ctx context.Context, q *dbsqlc.Queries, cert, successor dbsqlc.GetCertificateByIDRow) error {
	if !IsPlatformCertificate(cert) {
		return nil
	}
	return p.enqueue(ctx, q, EventCertificateRenewed, certificateSubject(cert.ID), func(timestamp string) any {
		return certificateRenewedPayload{
			Event:                      EventCertificateRenewed,
			Timestamp:                  timestamp,
			CertificateNumber:          CertificateNumber(cert),
			ValidUntil:                 validUntil(cert),
			RenewedAt:                  cert.RenewedAt.Time.Format(time.DateOnly),
			RenewedByCertificateNumber: CertificateNumber(successor),
			RenewedByIssuedAt:          successor.Date.Time.Format(time.DateOnly),
			RenewedByValidUntil:        validUntil(successor),
			RenewedByVerificationCode:  successor.VerificationCode,
		}
	})
}

// CertificateValidityChanged zapisuje certificate.validity_changed. valid_until jest null,
// gdy dokument przestał mieć termin ważności (np. usunięto datę zakończenia kursu).
func (p *Publisher) CertificateValidityChanged(ctx context.Context, q *dbsqlc.Queries, cert dbsqlc.GetCertificateByIDRow) error {
	if !IsPlatformCertificate(cert) {
		return nil
	}
	return p.enqueue(ctx, q, EventCertificateValidityChanged, certificateSubject(cert.ID), func(timestamp string) any {
		return certificateValidityChangedPayload{
			Event:             EventCertificateValidityChanged,
			Timestamp:         timestamp,
			CertificateNumber: CertificateNumber(cert),
			ValidUntil:        validUntil(cert),
		}
	})
}

// ProgramUpdated zapisuje program.updated dla kursu.
func (p *Publisher) ProgramUpdated(ctx context.Context, q *dbsqlc.Queries, courseID int64) error {
	return p.enqueue(ctx, q, EventProgramUpdated, "course:"+strconv.FormatInt(courseID, 10), func(timestamp string) any {
		return programUpdatedPayload{
			Event:             EventProgramUpdated,
			Timestamp:         timestamp,
			ExternalProgramID: courseID,
		}
	})
}

func (p *Publisher) enqueue(ctx context.Context, q *dbsqlc.Queries, eventType, subject string, build func(timestamp string) any) error {
	if err := q.AcquireWebhookSubjectLock(ctx, subject); err != nil {
		return err
	}
	occurredAt, err := q.NextWebhookEventTimestamp(ctx, subject)
	if err != nil {
		return err
	}
	body, err := json.Marshal(build(occurredAt.Time.UTC().Format(timestampFormat)))
	if err != nil {
		return err
	}
	if _, err := q.InsertWebhookEvent(ctx, dbsqlc.InsertWebhookEventParams{
		EventType:  eventType,
		SubjectKey: subject,
		OccurredAt: pgtype.Timestamptz{Time: occurredAt.Time, Valid: true},
		Payload:    body,
	}); err != nil {
		return err
	}
	return q.NotifyWebhookDispatcher(ctx)
}

func certificateSubject(id int64) string {
	return "certificate:" + strconv.FormatInt(id, 10)
}

// CertificateNumber to numer w formacie numer/SYMBOL/rok, jak w REST API.
func CertificateNumber(cert dbsqlc.GetCertificateByIDRow) string {
	return fmt.Sprintf("%d/%s/%d", cert.RegistryNumber, cert.CourseSymbol, cert.RegistryYear)
}

func validUntil(cert dbsqlc.GetCertificateByIDRow) *string {
	value := strings.TrimSpace(fmt.Sprint(cert.ExpiryDate))
	if cert.ExpiryDate == nil || value == "" {
		return nil
	}
	return &value
}
