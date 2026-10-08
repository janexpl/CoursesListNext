package legalbases

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("legal basis not found")
	// ErrNameTaken - nazwa służy do wyboru z listy, więc nie może się powtarzać.
	ErrNameTaken = errors.New("legal basis name already exists")
	// ErrInUse - podstawy, którą wskazuje kurs, nie da się usunąć: kurs zostałby z cichą
	// pustką w miejscu podstawy na wydruku.
	ErrInUse = errors.New("legal basis in use")
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

// Input to znormalizowana treść wpisu.
type Input struct {
	Name    string
	Content string
}

func normalize(input Input) (Input, error) {
	input.Name = strings.TrimSpace(input.Name)
	// Wewnętrzne odstępy treści sprowadzamy do pojedynczych spacji: tekst trafia do jednego
	// akapitu na wydruku, a wklejka z PDF-u rozporządzenia przynosi łamania wierszy.
	input.Content = strings.Join(strings.Fields(input.Content), " ")
	if input.Name == "" || input.Content == "" {
		return Input{}, ErrInvalidInput
	}
	return input, nil
}

// Create dodaje podstawę. Dostępne dla każdego, kto edytuje kursy: nowa podstawa nie
// zmienia niczego, dopóki kurs jej nie wybierze.
func (s *Service) Create(ctx context.Context, input Input) (LegalBasisDetailsDTO, error) {
	input, err := normalize(input)
	if err != nil {
		return LegalBasisDetailsDTO{}, err
	}
	return s.inTx(ctx, func(tx txScope) (LegalBasisDetailsDTO, error) {
		row, err := tx.queries.CreateLegalBasis(ctx, sqlc.CreateLegalBasisParams{Name: input.Name, Content: input.Content})
		if err != nil {
			return LegalBasisDetailsDTO{}, mapWriteError(err)
		}
		created := LegalBasisDetailsDTO{LegalBasisDTO: makeDTO(row, 0), Courses: []LegalBasisCourseDTO{}}
		if err := s.record(ctx, tx, row.ID, "create", nil, created); err != nil {
			return LegalBasisDetailsDTO{}, err
		}
		return created, nil
	})
}

// Update zmienia nazwę i treść. Wydane zaświadczenia mają treść zamrożoną przy wystawieniu,
// więc zmiana dotyczy wyłącznie dokumentów wystawionych później.
func (s *Service) Update(ctx context.Context, id int64, input Input) (LegalBasisDetailsDTO, error) {
	input, err := normalize(input)
	if err != nil {
		return LegalBasisDetailsDTO{}, err
	}
	return s.inTx(ctx, func(tx txScope) (LegalBasisDetailsDTO, error) {
		before, err := loadDetails(ctx, tx.queries, id)
		if err != nil {
			return LegalBasisDetailsDTO{}, err
		}
		row, err := tx.queries.UpdateLegalBasis(ctx, sqlc.UpdateLegalBasisParams{ID: id, Name: input.Name, Content: input.Content})
		if err != nil {
			return LegalBasisDetailsDTO{}, mapWriteError(err)
		}
		after := LegalBasisDetailsDTO{LegalBasisDTO: makeDTO(row, before.CourseCount), Courses: before.Courses}
		if err := s.record(ctx, tx, id, "update", before, after); err != nil {
			return LegalBasisDetailsDTO{}, err
		}
		return after, nil
	})
}

// Delete usuwa wyłącznie podstawę, której nie wskazuje żaden kurs.
func (s *Service) Delete(ctx context.Context, id int64) error {
	_, err := s.inTx(ctx, func(tx txScope) (LegalBasisDetailsDTO, error) {
		before, err := loadDetails(ctx, tx.queries, id)
		if err != nil {
			return LegalBasisDetailsDTO{}, err
		}
		if before.CourseCount > 0 {
			return LegalBasisDetailsDTO{}, ErrInUse
		}
		if _, err := tx.queries.DeleteLegalBasis(ctx, id); err != nil {
			return LegalBasisDetailsDTO{}, mapWriteError(err)
		}
		return LegalBasisDetailsDTO{}, s.record(ctx, tx, id, "delete", before, nil)
	})
	return err
}

func (s *Service) inTx(ctx context.Context, fn func(tx txScope) (LegalBasisDetailsDTO, error)) (LegalBasisDetailsDTO, error) {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return LegalBasisDetailsDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.rollback(ctx); err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()
	result, err := fn(tx)
	if err != nil {
		return LegalBasisDetailsDTO{}, err
	}
	if err := tx.commit(ctx); err != nil {
		return LegalBasisDetailsDTO{}, err
	}
	committed = true
	return result, nil
}

func (s *Service) record(ctx context.Context, tx txScope, id int64, action string, before, after any) error {
	if s.recorder == nil {
		return nil
	}
	return s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "legal_basis",
		EntityID:   id,
		Action:     action,
		Before:     before,
		After:      after,
	})
}

type detailsReader interface {
	GetLegalBasisByID(ctx context.Context, id int64) (sqlc.LegalBasis, error)
	ListCoursesByLegalBasis(ctx context.Context, legalBasisID pgtype.Int8) ([]sqlc.ListCoursesByLegalBasisRow, error)
}

func loadDetails(ctx context.Context, q detailsReader, id int64) (LegalBasisDetailsDTO, error) {
	row, err := q.GetLegalBasisByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LegalBasisDetailsDTO{}, ErrNotFound
		}
		return LegalBasisDetailsDTO{}, err
	}
	courses, err := q.ListCoursesByLegalBasis(ctx, pgtype.Int8{Int64: id, Valid: true})
	if err != nil {
		return LegalBasisDetailsDTO{}, err
	}
	out := LegalBasisDetailsDTO{LegalBasisDTO: makeDTO(row, int64(len(courses))), Courses: make([]LegalBasisCourseDTO, 0, len(courses))}
	for _, c := range courses {
		out.Courses = append(out.Courses, LegalBasisCourseDTO{ID: c.ID, Symbol: c.Symbol, Name: c.Name})
	}
	return out, nil
}

func makeDTO(row sqlc.LegalBasis, courseCount int64) LegalBasisDTO {
	return LegalBasisDTO{
		ID:          row.ID,
		Name:        row.Name,
		Content:     row.Content,
		CourseCount: courseCount,
		UpdatedAt:   row.UpdatedAt.Time.Format(response.TimestampzFormat),
	}
}

func mapWriteError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && pgErr.ConstraintName == "legal_bases_name_uidx":
			return ErrNameTaken
		case pgErr.Code == "23503":
			// Kurs wskazał podstawę między sprawdzeniem a usunięciem.
			return ErrInUse
		}
	}
	return err
}
