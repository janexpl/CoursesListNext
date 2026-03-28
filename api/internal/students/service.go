package students

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

var ErrInvalidInput = errors.New("invalid input")

type txScope struct {
	queries  *dbsqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

type Service struct {
	recorder  *auditlog.Recorder
	beginTxFn func(context.Context) (txScope, error)
}

func NewService(pool *pgxpool.Pool, queries *dbsqlc.Queries, recorder *auditlog.Recorder) *Service {
	return &Service{
		recorder: recorder,
		beginTxFn: func(ctx context.Context) (txScope, error) {
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

func (s *Service) Create(ctx context.Context, req CreateStudentRequest) (StudentDetailsDTO, error) {
	params, err := buildCreateStudentParams(req)
	if err != nil {
		return StudentDetailsDTO{}, err
	}

	tx, err := s.beginTxFn(ctx)
	if err != nil {
		return StudentDetailsDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	createdStudent, err := tx.queries.CreateStudent(ctx, params)
	if err != nil {
		return StudentDetailsDTO{}, err
	}

	createdSnapshot := mapCreateStudentRow(createdStudent)
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "student",
			EntityID:   createdStudent.ID,
			Action:     "create",
			Before:     nil,
			After:      createdSnapshot,
			Metadata:   nil,
		}); err != nil {
			return StudentDetailsDTO{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return StudentDetailsDTO{}, err
	}
	committed = true

	return createdSnapshot, nil
}

func (s *Service) Update(ctx context.Context, studentID int64, req UpdateStudentRequest) (StudentDetailsDTO, error) {
	if studentID <= 0 {
		return StudentDetailsDTO{}, ErrInvalidInput
	}

	params, err := buildUpdateStudentParams(studentID, req)
	if err != nil {
		return StudentDetailsDTO{}, err
	}

	tx, err := s.beginTxFn(ctx)
	if err != nil {
		return StudentDetailsDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	beforeStudent, err := tx.queries.GetStudentByID(ctx, studentID)
	if err != nil {
		return StudentDetailsDTO{}, err
	}

	updatedStudent, err := tx.queries.UpdateStudent(ctx, params)
	if err != nil {
		return StudentDetailsDTO{}, err
	}

	beforeSnapshot := mapStudentGetRow(beforeStudent)
	afterSnapshot := mapStudentDetailsRow(updatedStudent)
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "student",
			EntityID:   studentID,
			Action:     "update",
			Before:     beforeSnapshot,
			After:      afterSnapshot,
			Metadata:   nil,
		}); err != nil {
			return StudentDetailsDTO{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return StudentDetailsDTO{}, err
	}
	committed = true

	return afterSnapshot, nil
}

func buildCreateStudentParams(req CreateStudentRequest) (dbsqlc.CreateStudentParams, error) {
	payload, birthDate, err := normalizeStudentPayload(req.studentPayload)
	if err != nil {
		return dbsqlc.CreateStudentParams{}, err
	}

	return dbsqlc.CreateStudentParams{
		Firstname:     payload.FirstName,
		Lastname:      payload.LastName,
		Secondname:    pgutil.OptionalText(payload.SecondName),
		Birthdate:     pgtype.Date{Time: birthDate, Valid: true},
		Birthplace:    payload.BirthPlace,
		Pesel:         pgutil.OptionalText(payload.Pesel),
		Addressstreet: pgutil.OptionalText(payload.AddressStreet),
		Addresscity:   pgutil.OptionalText(payload.AddressCity),
		Addresszip:    pgutil.OptionalText(payload.AddressZip),
		Telephoneno:   pgutil.OptionalText(payload.Telephone),
		CompanyID:     pgutil.OptionalInt8(payload.CompanyID),
	}, nil
}

func buildUpdateStudentParams(studentID int64, req UpdateStudentRequest) (dbsqlc.UpdateStudentParams, error) {
	params, err := buildCreateStudentParams(CreateStudentRequest(req))
	if err != nil {
		return dbsqlc.UpdateStudentParams{}, err
	}

	return dbsqlc.UpdateStudentParams{
		Firstname:     params.Firstname,
		Lastname:      params.Lastname,
		Secondname:    params.Secondname,
		Birthdate:     params.Birthdate,
		Birthplace:    params.Birthplace,
		Pesel:         params.Pesel,
		Addressstreet: params.Addressstreet,
		Addresscity:   params.Addresscity,
		Addresszip:    params.Addresszip,
		Telephoneno:   params.Telephoneno,
		CompanyID:     params.CompanyID,
		StudentID:     studentID,
	}, nil
}

func normalizeStudentPayload(payload studentPayload) (studentPayload, time.Time, error) {
	firstName := strings.TrimSpace(payload.FirstName)
	lastName := strings.TrimSpace(payload.LastName)
	birthDateRaw := strings.TrimSpace(payload.BirthDate)
	birthPlace := strings.TrimSpace(payload.BirthPlace)

	if firstName == "" || lastName == "" || birthDateRaw == "" || birthPlace == "" {
		return studentPayload{}, time.Time{}, ErrInvalidInput
	}
	if payload.CompanyID != nil && *payload.CompanyID <= 0 {
		return studentPayload{}, time.Time{}, ErrInvalidInput
	}

	birthDate, err := time.Parse(response.DateFormat, birthDateRaw)
	if err != nil {
		return studentPayload{}, time.Time{}, ErrInvalidInput
	}

	payload.FirstName = firstName
	payload.LastName = lastName
	payload.BirthDate = birthDateRaw
	payload.BirthPlace = birthPlace

	return payload, birthDate, nil
}

func mapStudentGetRow(row dbsqlc.GetStudentByIDRow) StudentDetailsDTO {
	dto := StudentDetailsDTO{
		ID:            row.ID,
		FirstName:     row.Firstname,
		LastName:      row.Lastname,
		SecondName:    pgutil.NullableString(row.Secondname),
		BirthDate:     row.Birthdate.Time.Format(response.DateFormat),
		BirthPlace:    row.Birthplace,
		Pesel:         pgutil.NullableString(row.Pesel),
		AddressStreet: pgutil.NullableString(row.Addressstreet),
		AddressCity:   pgutil.NullableString(row.Addresscity),
		AddressZip:    pgutil.NullableString(row.Addresszip),
		Telephone:     pgutil.NullableString(row.Telephoneno),
	}
	if row.CompanyID.Valid && row.CompanyName.Valid {
		dto.Company = &CompanyDTO{ID: row.CompanyID.Int64, Name: row.CompanyName.String}
	}
	return dto
}
