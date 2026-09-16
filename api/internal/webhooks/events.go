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
	EventCertificateRevoked         = "certificate.revoked"
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
	// SupersedesCertificateNumber występuje wyłącznie przy duplikacie i niesie numer
	// oryginału. Duplikat ma klucz idempotencji oryginału, więc bez tego pola odbiorca
	// odnalazłby po kluczu oryginał i nadpisałby mu numer, kod i adres PDF danymi
	// duplikatu. Obecność pola mówi: utwórz dokument powiązany, nie nadpisuj znalezionego.
	SupersedesCertificateNumber string `json:"supersedes_certificate_number,omitempty"`
}

type certificateRevokedPayload struct {
	Event             string `json:"event"`
	Timestamp         string `json:"timestamp"`
	CertificateNumber string `json:"certificate_number"`
	Reason            string `json:"reason"`
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
	supersedesNumber := ""
	if cert.SupersedesID.Valid {
		original, err := q.GetCertificateNumberByID(ctx, cert.SupersedesID.Int64)
		if err != nil {
			return err
		}
		supersedesNumber = fmt.Sprintf("%d/%s/%d", original.RegistryNumber, original.CourseSymbol, original.RegistryYear)
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

			SupersedesCertificateNumber: supersedesNumber,
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
