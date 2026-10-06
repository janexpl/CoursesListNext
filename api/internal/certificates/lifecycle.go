// Operacje cyklu życia zaświadczenia: unieważnienie, duplikat, przedłużenie.
//
// KOLEJNOŚĆ BLOKAD obowiązująca cały pakiet - żadna ścieżka nie bierze ich w innym
// porządku, więc cyklu oczekiwań nie da się zbudować:
//
//	L1  pg_advisory_xact_lock('idempotency:<klucz>')   AcquireIdempotencyKeyLock
//	L2  blokada wiersza zaświadczenia                  LockCertificateForLifecycle
//	L3  pg_advisory_xact_lock('registry:<kurs>:<rok>') AcquireRegistryLock
//	L4  blokada podmiotu webhooka                      AcquireWebhookSubjectLock
//
// Create bierze L1, L3, L4. Revoke i Duplicate biorą L2, L4. Renew bierze wszystkie
// cztery. Blokada wiersza wchodzi PRZED rejestrem, bo rozstrzyga, czy operacja jest
// w ogóle dozwolona (404, 409), nie szeregując się wcześniej na gorącej blokadzie
// rejestru całego kursu.
//
// Przyszła trasa zbiorcza musiałaby dodatkowo blokować dokumenty rosnąco po id.
package certificates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

var (
	// ErrCertificateNotFound - zaświadczenie nie istnieje albo zostało usunięte.
	ErrCertificateNotFound = errors.New("certificate not found")
	// ErrCertificateAlreadyRevoked - ponowne unieważnienie.
	ErrCertificateAlreadyRevoked = errors.New("certificate already revoked")
	// ErrCertificateRevoked - operacja niedozwolona na unieważnionym dokumencie
	// (duplikat, edycja, PDF).
	ErrCertificateRevoked = errors.New("certificate is revoked")
	// ErrCertificateAlreadyRenewed - dokument ma już następcę. Jeden dokument
	// przedłuża się raz; kolejne szkolenie okresowe przedłuża już następcę.
	ErrCertificateAlreadyRenewed = errors.New("certificate already renewed")
)

// RenewCertificateInput to dane nowego dokumentu. Kursanta, kursu ani numeru rejestru
// NIE przyjmujemy: pierwsze dwa biorą się z przedłużanego zaświadczenia (inaczej
// renewed_by_certificate_id byłoby kłamstwem), a numer nadaje serwer.
type RenewCertificateInput struct {
	CertificateDate string
	CourseDateStart string
	CourseDateEnd   *string
	// LanguageCode pusty oznacza język poprzednika.
	LanguageCode string
	// IdempotencyKey - wartość nagłówka Idempotency-Key; pusta wyłącza idempotencję.
	IdempotencyKey string
}

// Renew wystawia nowe zaświadczenie dla kursanta i kursu wskazanego dokumentu,
// a stary oznacza jako przedłużony - obie zmiany w jednej transakcji.
//
// Znacznik NIE jest unieważnieniem: poprzednik zostaje ważny do swojej daty, dalej się
// drukuje, dalej weryfikuje publicznie jako ważny i dalej wolno go edytować oraz
// unieważnić. Wypada wyłącznie z przypomnień o wygasaniu i z kafelka na pulpicie.
//
// Łańcuch A→B→C jest dozwolony; cyklu nie da się zbudować, bo następca powstaje
// dopiero w tej transakcji.
func (s *Service) Renew(ctx context.Context, certificateID int64, input RenewCertificateInput) (CreateCertificateResult, error) {
	if certificateID <= 0 {
		return CreateCertificateResult{}, ErrInvalidInput
	}

	var requestHash string
	if input.IdempotencyKey != "" {
		requestHash = renewIdempotencyRequestHash(certificateID, input)
		result, found, err := replayIdempotentCreate(ctx, s.queries, input.IdempotencyKey, requestHash)
		if err != nil || found {
			return result, err
		}
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return CreateCertificateResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	// L1 przed L2: createCertificateTx weźmie tę samą blokadę ponownie (advisory
	// w transakcji jest wielokrotna), ale wziąć ją musimy tutaj, żeby nie zejść
	// poniżej L2 i nie odwrócić porządku z nagłówka pliku.
	if input.IdempotencyKey != "" {
		if err := tx.queries.AcquireIdempotencyKeyLock(ctx, input.IdempotencyKey); err != nil {
			return CreateCertificateResult{}, err
		}
	}
	// L2
	if _, err := tx.queries.LockCertificateForLifecycle(ctx, certificateID); err != nil {
		return CreateCertificateResult{}, notFoundAs(err, ErrCertificateNotFound)
	}
	state, err := tx.queries.GetCertificateLifecycleState(ctx, certificateID)
	if err != nil {
		return CreateCertificateResult{}, err
	}
	if state.Revoked {
		return CreateCertificateResult{}, ErrCertificateRevoked
	}
	if state.Renewed {
		return CreateCertificateResult{}, ErrCertificateAlreadyRenewed
	}

	before, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return CreateCertificateResult{}, notFoundAs(err, ErrCertificateNotFound)
	}

	// Przygotowanie idzie POD blokadą wiersza, bo kursanta i kurs czytamy ze starego
	// dokumentu - wzięcie ich przed blokadą byłoby wyścigiem (patrz komentarz przy
	// LockCertificateForLifecycle).
	prepared, err := s.prepareCertificate(ctx, renewalCreateInput(before, input), requestHash)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	// L3 i L4 bierze createCertificateTx.
	result, err := s.createCertificateTx(ctx, tx, prepared)
	if err != nil {
		return CreateCertificateResult{}, err
	}
	if result.Replayed {
		// Znacznik postawiło pierwotne żądanie; nic tu nie zapisaliśmy.
		return result, nil
	}

	// Oznaczenie idzie PO utworzeniu, bo dopiero teraz znamy id następcy - dlatego
	// blokada rejestru jest trzymana o kilka instrukcji dłużej niż w Create.
	renewedBy := pgtype.Int8{}
	if user, ok := auth.UserFromContext(ctx); ok {
		renewedBy = pgtype.Int8{Int64: user.ID, Valid: true}
	}
	if err := tx.queries.MarkCertificateRenewed(ctx, dbsqlc.MarkCertificateRenewedParams{
		RenewedByCertificateID: pgtype.Int8{Int64: result.ID, Valid: true},
		RenewedByUserID:        renewedBy,
		ID:                     certificateID,
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			pgErr.ConstraintName == "certificates_renewed_by_certificate_id_uidx" {
			return CreateCertificateResult{}, ErrCertificateAlreadyRenewed
		}
		return CreateCertificateResult{}, err
	}

	after, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return CreateCertificateResult{}, err
	}
	successor, err := tx.queries.GetCertificateByID(ctx, result.ID)
	if err != nil {
		return CreateCertificateResult{}, err
	}
	if s.publisher != nil {
		if err := s.publisher.CertificateRenewed(ctx, tx.queries, after, successor); err != nil {
			return CreateCertificateResult{}, err
		}
	}
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "certificate",
			EntityID:   certificateID,
			Action:     "update",
			Before:     mapCertificateDetailsResponse(before, nil),
			After:      mapCertificateDetailsResponse(after, nil),
			Metadata:   map[string]any{"operation": "renew", "renewedByCertificateId": result.ID},
		}); err != nil {
			return CreateCertificateResult{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return CreateCertificateResult{}, err
	}
	committed = true
	return result, nil
}

// renewalCreateInput sprowadza przedłużenie do zwykłego wystawienia. Kursant i kurs
// pochodzą WYŁĄCZNIE ze starego wiersza, numer rejestru nadaje serwer.
func renewalCreateInput(before dbsqlc.GetCertificateByIDRow, input RenewCertificateInput) CreateCertificateInput {
	languageCode := strings.TrimSpace(input.LanguageCode)
	if languageCode == "" {
		// Przedłużenie zostaje w języku poprzednika.
		languageCode = before.LanguageCode
	}
	return CreateCertificateInput{
		StudentID:            int64(before.StudentID),
		CourseID:             before.CourseID,
		CertificateDate:      input.CertificateDate,
		CourseDateStart:      input.CourseDateStart,
		CourseDateEnd:        input.CourseDateEnd,
		LanguageCode:         languageCode,
		AssignRegistryNumber: true,
		IdempotencyKey:       input.IdempotencyKey,
	}
}

// renewIdempotencyRequestHash wiąże odcisk z przedłużanym dokumentem, owijając odcisk
// zwykłego wystawienia zamiast dokładać pole do jego znormalizowanej struktury:
// tamta zmiana przesunęłaby odcisk WSZYSTKIM wystawieniom i dała 409 na ponowieniach
// żądań sprzed wdrożenia.
//
// Surowy languageCode wchodzi do odcisku osobno, bo idempotencyRequestHash normalizuje
// pusty na "pl", a tutaj pusty znaczy "język poprzednika" - dwa różne żądania.
func renewIdempotencyRequestHash(certificateID int64, input RenewCertificateInput) string {
	inner := idempotencyRequestHash(CreateCertificateInput{
		CertificateDate: input.CertificateDate,
		CourseDateStart: input.CourseDateStart,
		CourseDateEnd:   input.CourseDateEnd,
		LanguageCode:    input.LanguageCode,
	})
	sum := sha256.Sum256([]byte("renew:" + strconv.FormatInt(certificateID, 10) +
		":" + strings.ToLower(strings.TrimSpace(input.LanguageCode)) + ":" + inner))
	return hex.EncodeToString(sum[:])
}

// Revoke unieważnia zaświadczenie. Dokument zostaje w bazie i na listach, a jego numer
// rejestru pozostaje zajęty (patrz zapytania w registries.sql).
func (s *Service) Revoke(ctx context.Context, certificateID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if certificateID <= 0 || reason == "" {
		return ErrInvalidInput
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	if _, err := tx.queries.LockCertificateForLifecycle(ctx, certificateID); err != nil {
		return notFoundAs(err, ErrCertificateNotFound)
	}
	state, err := tx.queries.GetCertificateLifecycleState(ctx, certificateID)
	if err != nil {
		return err
	}
	if state.Revoked {
		return ErrCertificateAlreadyRevoked
	}

	before, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return notFoundAs(err, ErrCertificateNotFound)
	}

	revokedBy := pgtype.Int8{}
	if user, ok := auth.UserFromContext(ctx); ok {
		revokedBy = pgtype.Int8{Int64: user.ID, Valid: true}
	}
	if err := tx.queries.RevokeCertificate(ctx, dbsqlc.RevokeCertificateParams{
		ID:              certificateID,
		Reason:          reason,
		RevokedByUserID: revokedBy,
	}); err != nil {
		return err
	}

	after, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return err
	}
	if s.publisher != nil {
		if err := s.publisher.CertificateRevoked(ctx, tx.queries, after, reason); err != nil {
			return err
		}
	}
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "certificate",
			EntityID:   certificateID,
			Action:     "update",
			Before:     mapCertificateDetailsResponse(before, nil),
			After:      mapCertificateDetailsResponse(after, nil),
			Metadata:   map[string]any{"operation": "revoke", "reason": reason},
		}); err != nil {
			return err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

// Duplicate odnotowuje wystawienie duplikatu (wtórnika) na istniejącym zaświadczeniu.
// Duplikat to ten sam dokument o tym samym numerze rejestru, z adnotacją "DUPLIKAT"
// i datą na wydruku - nie powstaje nowe zaświadczenie i nie jest zajmowany nowy numer.
// Kolejne wystawienie nadpisuje datę i powód; każde zostaje w historii zmian.
func (s *Service) Duplicate(ctx context.Context, certificateID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if certificateID <= 0 || reason == "" {
		return ErrInvalidInput
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	if _, err := tx.queries.LockCertificateForLifecycle(ctx, certificateID); err != nil {
		return notFoundAs(err, ErrCertificateNotFound)
	}
	state, err := tx.queries.GetCertificateLifecycleState(ctx, certificateID)
	if err != nil {
		return err
	}
	if state.Revoked {
		return ErrCertificateRevoked
	}

	before, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return notFoundAs(err, ErrCertificateNotFound)
	}

	issuedBy := pgtype.Int8{}
	if user, ok := auth.UserFromContext(ctx); ok {
		issuedBy = pgtype.Int8{Int64: user.ID, Valid: true}
	}
	if err := tx.queries.MarkCertificateDuplicateIssued(ctx, dbsqlc.MarkCertificateDuplicateIssuedParams{
		ID:                      certificateID,
		Reason:                  reason,
		DuplicateIssuedByUserID: issuedBy,
	}); err != nil {
		return err
	}

	after, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return err
	}
	if s.publisher != nil {
		if err := s.publisher.CertificateDuplicateIssued(ctx, tx.queries, after, reason); err != nil {
			return err
		}
	}
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "certificate",
			EntityID:   certificateID,
			Action:     "update",
			Before:     mapCertificateDetailsResponse(before, nil),
			After:      mapCertificateDetailsResponse(after, nil),
			Metadata:   map[string]any{"operation": "duplicate", "reason": reason},
		}); err != nil {
			return err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}
