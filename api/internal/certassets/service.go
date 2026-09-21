package certassets

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

var (
	// ErrInvalidInput - zły rodzaj nadruku albo szerokość poza dopuszczalnym zakresem.
	ErrInvalidInput = errors.New("invalid input")

	// ErrNotFound - nadruk tego rodzaju nie był wgrany.
	ErrNotFound = errors.New("print asset not found")
)

const (
	// Zakres szerokości nadruku pilnuje też ograniczenie CHECK w bazie; tu odrzucamy
	// wartość wcześniej, żeby wołający dostał czytelny komunikat zamiast błędu bazy.
	minWidthMM = 10
	maxWidthMM = 60
)

type txScope struct {
	queries  *sqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

type Service struct {
	recorder *auditlog.Recorder
	beginTx  func(context.Context) (txScope, error)
}

func NewService(pool *pgxpool.Pool, queries *sqlc.Queries, recorder *auditlog.Recorder) *Service {
	return &Service{
		recorder: recorder,
		beginTx: func(ctx context.Context) (txScope, error) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return txScope{}, err
			}

			return txScope{
				queries:  queries.WithTx(tx),
				commit:   tx.Commit,
				rollback: tx.Rollback,
			}, nil
		},
	}
}

// UpsertInput to znormalizowany plik gotowy do zapisu.
type UpsertInput struct {
	FileName     string
	FileData     []byte
	PrintWidthMm int
}

// Upsert podmienia nadruk danego rodzaju. Zapis i wpis do audytu idą jedną transakcją:
// pieczątka i podpis decydują o wyglądzie każdego dokumentu wychodzącego z instytucji,
// więc musi zostać ślad, kto i kiedy je zmienił. Bez tego nie dałoby się później
// odtworzyć, która pieczątka obowiązywała w dniu wystawienia danego zaświadczenia.
func (s *Service) Upsert(ctx context.Context, kind string, input UpsertInput) (sqlc.UpsertCertificatePrintAssetRow, error) {
	if !IsValidKind(kind) {
		return sqlc.UpsertCertificatePrintAssetRow{}, ErrInvalidInput
	}
	if input.PrintWidthMm < minWidthMM || input.PrintWidthMm > maxWidthMM {
		return sqlc.UpsertCertificatePrintAssetRow{}, ErrInvalidInput
	}
	if len(input.FileData) == 0 {
		return sqlc.UpsertCertificatePrintAssetRow{}, ErrInvalidInput
	}

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return sqlc.UpsertCertificatePrintAssetRow{}, auditlog.ErrUnauthorized
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return sqlc.UpsertCertificatePrintAssetRow{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.rollback(ctx); err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	previous, err := tx.queries.ListCertificatePrintAssetsMeta(ctx)
	if err != nil {
		return sqlc.UpsertCertificatePrintAssetRow{}, err
	}

	row, err := tx.queries.UpsertCertificatePrintAsset(ctx, sqlc.UpsertCertificatePrintAssetParams{
		Kind:     kind,
		FileName: input.FileName,
		// Normalizacja sprowadza każdy plik do PNG (patrz image.go), a baza tego pilnuje
		// ograniczeniem CHECK. Typ nie może pochodzić z nagłówka żądania, bo trafia
		// na wydruk do atrybutu src obrazu.
		ContentType:      "image/png",
		FileSize:         int64(len(input.FileData)),
		FileData:         input.FileData,
		PrintWidthMm:     int16(input.PrintWidthMm),
		UploadedByUserID: user.ID,
	})
	if err != nil {
		return sqlc.UpsertCertificatePrintAssetRow{}, err
	}

	// Bajtów pliku nie zapisujemy do audytu - liczy się to, co pozwala odtworzyć historię,
	// a nie kopia obrazu w tabeli dziennika zmian.
	if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "certificate_print_asset",
		EntityID:   row.ID,
		Action:     "update",
		Before:     findAssetMeta(previous, kind),
		After: map[string]any{
			"kind":         row.Kind,
			"fileName":     row.FileName,
			"fileSize":     row.FileSize,
			"printWidthMm": row.PrintWidthMm,
		},
	}); err != nil {
		return sqlc.UpsertCertificatePrintAssetRow{}, err
	}

	if err := tx.commit(ctx); err != nil {
		return sqlc.UpsertCertificatePrintAssetRow{}, err
	}
	committed = true

	return row, nil
}

// Delete usuwa nadruk danego rodzaju. Dokumenty drukowane od tej chwili nie będą go nosić.
func (s *Service) Delete(ctx context.Context, kind string) error {
	if !IsValidKind(kind) {
		return ErrInvalidInput
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.rollback(ctx); err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	existing, err := tx.queries.ListCertificatePrintAssetsMeta(ctx)
	if err != nil {
		return err
	}
	before := findAssetMeta(existing, kind)
	if before == nil {
		return ErrNotFound
	}

	affected, err := tx.queries.DeleteCertificatePrintAsset(ctx, kind)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}

	if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "certificate_print_asset",
		EntityID:   before["id"].(int64),
		Action:     "delete",
		Before:     before,
		After:      nil,
	}); err != nil {
		return err
	}

	if err := tx.commit(ctx); err != nil {
		return err
	}
	committed = true

	return nil
}

func findAssetMeta(rows []sqlc.ListCertificatePrintAssetsMetaRow, kind string) map[string]any {
	for _, row := range rows {
		if row.Kind != kind {
			continue
		}
		return map[string]any{
			"id":           row.ID,
			"kind":         row.Kind,
			"fileName":     row.FileName,
			"fileSize":     row.FileSize,
			"printWidthMm": row.PrintWidthMm,
		}
	}
	return nil
}
