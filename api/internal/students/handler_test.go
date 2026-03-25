package students

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeDB struct {
	query    func(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	queryRow func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (f fakeDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec call")
}

func (f fakeDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if f.query == nil {
		return nil, errors.New("unexpected query call")
	}
	return f.query(ctx, sql, args...)
}

func (f fakeDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if f.queryRow == nil {
		return fakeRow{err: errors.New("unexpected query row call")}
	}
	return f.queryRow(ctx, sql, args...)
}

type fakeRow struct {
	err  error
	scan func(dest ...any) error
}

func (r fakeRow) Scan(dest ...interface{}) error {
	if r.scan != nil {
		return r.scan(dest...)
	}
	return r.err
}

type fakeRows struct {
	index int
	scans []func(dest ...any) error
	err   error
}

func (r *fakeRows) Close() {}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	if r.index >= len(r.scans) {
		return false
	}
	r.index++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.scans) {
		return errors.New("scan called without current row")
	}
	return r.scans[r.index-1](dest...)
}

func (r *fakeRows) Values() ([]any, error) {
	return nil, nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expectedCode string) {
	t.Helper()

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if responseBody.Error.Code != expectedCode {
		t.Fatalf("expected error code %q, got %q", expectedCode, responseBody.Error.Code)
	}
}

func TestListReturnsStudentsResponse(t *testing.T) {
	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error {
				*(dest[0].(*int64)) = 1
				*(dest[1].(*string)) = "Jan"
				*(dest[2].(*string)) = "Nowak"
				*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
				*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[5].(*string)) = "Warszawa"
				*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "12345678901", Valid: true}
				*(dest[7].(*pgtype.Int8)) = pgtype.Int8{Int64: 9, Valid: true}
				*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
				return nil
			},
			func(dest ...any) error {
				*(dest[0].(*int64)) = 2
				*(dest[1].(*string)) = "Anna"
				*(dest[2].(*string)) = "Kowalska"
				*(dest[3].(*pgtype.Text)) = pgtype.Text{}
				*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2023, time.June, 2, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[5].(*string)) = "Krakow"
				*(dest[6].(*pgtype.Text)) = pgtype.Text{}
				*(dest[7].(*pgtype.Int8)) = pgtype.Int8{}
				*(dest[8].(*pgtype.Text)) = pgtype.Text{}
				return nil
			},
		},
	}

	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(_ context.Context, _ string, args ...interface{}) (pgx.Rows, error) {
			if len(args) != 3 {
				t.Fatalf("expected 3 query args, got %d", len(args))
			}

			searchArg, ok := args[0].(pgtype.Text)
			if !ok {
				t.Fatalf("expected search arg type pgtype.Text, got %T", args[0])
			}
			if searchArg.Valid {
				t.Fatalf("expected empty search arg, got %+v", searchArg)
			}

			companyArg, ok := args[1].(pgtype.Int8)
			if !ok {
				t.Fatalf("expected company arg type pgtype.Int8, got %T", args[1])
			}
			if companyArg.Valid {
				t.Fatalf("expected empty company arg, got %+v", companyArg)
			}

			limitArg, ok := args[2].(int32)
			if !ok {
				t.Fatalf("expected limit arg type int32, got %T", args[2])
			}
			if limitArg != 50 {
				t.Fatalf("expected default limit 50, got %d", limitArg)
			}
			return rows, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var response ListStudentsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Fatalf("expected 2 students, got %d", len(response.Data))
	}

	first := response.Data[0]
	if first.ID != 1 || first.FirstName != "Jan" || first.LastName != "Nowak" {
		t.Fatalf("unexpected first student payload: %+v", first)
	}
	if first.BirthDate != "2024-01-15" {
		t.Fatalf("expected birth date 2024-01-15, got %q", first.BirthDate)
	}
	if first.Pesel == nil || *first.Pesel != "12345678901" {
		t.Fatalf("expected pesel to be mapped, got %+v", first.Pesel)
	}
	if first.Company == nil || first.Company.ID != 9 || first.Company.Name != "ABC Sp. z o.o." {
		t.Fatalf("expected company to be mapped, got %+v", first.Company)
	}

	second := response.Data[1]
	if second.ID != 2 || second.FirstName != "Anna" || second.LastName != "Kowalska" {
		t.Fatalf("unexpected second student payload: %+v", second)
	}
	if second.Pesel != nil {
		t.Fatalf("expected nil pesel, got %+v", second.Pesel)
	}
	if second.Company != nil {
		t.Fatalf("expected nil company, got %+v", second.Company)
	}
}

func TestListReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(context.Context, string, ...interface{}) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListReturnsBadRequestForInvalidCompanyID(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(context.Context, string, ...interface{}) (pgx.Rows, error) {
			t.Fatal("query should not be called for invalid company id")
			return nil, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students?companyId=abc", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListReturnsBadRequestForInvalidLimit(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(context.Context, string, ...interface{}) (pgx.Rows, error) {
			t.Fatal("query should not be called for invalid limit")
			return nil, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students?limit=101", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListPassesFiltersToQuery(t *testing.T) {
	rows := &fakeRows{}

	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(_ context.Context, _ string, args ...interface{}) (pgx.Rows, error) {
			if len(args) != 3 {
				t.Fatalf("expected 3 query args, got %d", len(args))
			}

			searchArg := args[0].(pgtype.Text)
			if !searchArg.Valid || searchArg.String != "nowak" {
				t.Fatalf("expected search arg %q, got %+v", "nowak", searchArg)
			}

			companyArg := args[1].(pgtype.Int8)
			if !companyArg.Valid || companyArg.Int64 != 7 {
				t.Fatalf("expected company arg 7, got %+v", companyArg)
			}

			limitArg := args[2].(int32)
			if limitArg != 20 {
				t.Fatalf("expected limit arg 20, got %d", limitArg)
			}

			return rows, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students?search=nowak&companyId=7&limit=20", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestListUsesTokenizedStudentSearchQuery(t *testing.T) {
	rows := &fakeRows{}

	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(_ context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
			if !strings.Contains(sql, "regexp_split_to_array") {
				t.Fatalf("expected tokenized search SQL, got %q", sql)
			}
			if !strings.Contains(sql, "COALESCE(s.firstname, '') NOT ILIKE '%' || term || '%'") {
				t.Fatalf("expected first name token match clause, got %q", sql)
			}
			if !strings.Contains(sql, "COALESCE(s.lastname, '') NOT ILIKE '%' || term || '%'") {
				t.Fatalf("expected last name token match clause, got %q", sql)
			}
			if !strings.Contains(sql, "COALESCE(s.pesel, '') NOT ILIKE '%' || term || '%'") {
				t.Fatalf("expected pesel token match clause, got %q", sql)
			}

			searchArg, ok := args[0].(pgtype.Text)
			if !ok {
				t.Fatalf("expected search arg type pgtype.Text, got %T", args[0])
			}
			if !searchArg.Valid || searchArg.String != "Nowak Jan" {
				t.Fatalf("expected search arg %q, got %+v", "Nowak Jan", searchArg)
			}

			return rows, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students?search=Nowak%20Jan", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestGetReturnsStudentDetailsResponse(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, args ...interface{}) pgx.Row {
			if len(args) != 1 {
				t.Fatalf("expected 1 query arg, got %d", len(args))
			}

			idArg, ok := args[0].(int64)
			if !ok || idArg != 21 {
				t.Fatalf("expected student id 21, got %+v", args[0])
			}

			return fakeRow{
				scan: func(dest ...any) error {
					*(dest[0].(*int64)) = 21
					*(dest[1].(*string)) = "Jan"
					*(dest[2].(*string)) = "Nowak"
					*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
					*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
					*(dest[5].(*string)) = "Warszawa"
					*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
					*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Koszykowa 1", Valid: true}
					*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "Warszawa", Valid: true}
					*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "00-001", Valid: true}
					*(dest[10].(*pgtype.Text)) = pgtype.Text{String: "123456789", Valid: true}
					*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 8, Valid: true}
					*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
					return nil
				},
			}
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/21", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody StudentDetailsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 21 || responseBody.Data.FirstName != "Jan" || responseBody.Data.LastName != "Nowak" {
		t.Fatalf("unexpected student details payload: %+v", responseBody.Data)
	}
	if responseBody.Data.SecondName == nil || *responseBody.Data.SecondName != "Adam" {
		t.Fatalf("expected secondName to be mapped, got %+v", responseBody.Data.SecondName)
	}
	if responseBody.Data.Telephone == nil || *responseBody.Data.Telephone != "123456789" {
		t.Fatalf("expected telephone to be mapped, got %+v", responseBody.Data.Telephone)
	}
	if responseBody.Data.Company == nil || responseBody.Data.Company.ID != 8 || responseBody.Data.Company.Name != "ABC Sp. z o.o." {
		t.Fatalf("expected company to be mapped, got %+v", responseBody.Data.Company)
	}
}

func TestGetReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid id")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/not-a-number", nil)
	req.SetPathValue("id", "not-a-number")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGetReturnsNotFoundWhenStudentDoesNotExist(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			return fakeRow{err: pgx.ErrNoRows}
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/999", nil)
	req.SetPathValue("id", "999")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestGetReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			return fakeRow{err: errors.New("db error")}
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/21", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListCertificatesByStudentReturnsCertificatesHistory(t *testing.T) {
	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error {
				*(dest[0].(*int64)) = 101
				*(dest[1].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[2].(*string)) = "Szkolenie BHP"
				*(dest[3].(*string)) = "BHP"
				*(dest[4].(*int64)) = 2026
				*(dest[5].(*int64)) = 18
				*(dest[6].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 8, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[7].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[8].(*any)) = "2029-03-10"
				return nil
			},
			func(dest ...any) error {
				*(dest[0].(*int64)) = 77
				*(dest[1].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2025, time.January, 15, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[2].(*string)) = "Instruktaż"
				*(dest[3].(*string)) = "INS"
				*(dest[4].(*int64)) = 2025
				*(dest[5].(*int64)) = 3
				*(dest[6].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2025, time.January, 15, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[7].(*pgtype.Date)) = pgtype.Date{}
				*(dest[8].(*any)) = ""
				return nil
			},
		},
	}

	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(_ context.Context, _ string, args ...interface{}) (pgx.Rows, error) {
			if len(args) != 1 {
				t.Fatalf("expected 1 query arg, got %d", len(args))
			}

			studentID, ok := args[0].(int32)
			if !ok || studentID != 21 {
				t.Fatalf("expected student id 21, got %+v", args[0])
			}

			return rows, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/21/certificates", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListCertificatesByStudent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody ListCertificatesByStudentResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 2 {
		t.Fatalf("expected 2 certificates, got %d", len(responseBody.Data))
	}

	first := responseBody.Data[0]
	if first.ID != 101 || first.CourseName != "Szkolenie BHP" || first.RegistryNumber != 18 {
		t.Fatalf("unexpected first certificate payload: %+v", first)
	}
	if first.CourseDateEnd == nil || *first.CourseDateEnd != "2026-03-10" {
		t.Fatalf("expected courseDateEnd 2026-03-10, got %+v", first.CourseDateEnd)
	}
	if first.ExpiryDate == nil || *first.ExpiryDate != "2029-03-10" {
		t.Fatalf("expected expiryDate 2029-03-10, got %+v", first.ExpiryDate)
	}

	second := responseBody.Data[1]
	if second.ID != 77 || second.CourseSymbol != "INS" {
		t.Fatalf("unexpected second certificate payload: %+v", second)
	}
	if second.ExpiryDate != nil {
		t.Fatalf("expected empty expiryDate, got %+v", second.ExpiryDate)
	}
}

func TestListCertificatesByStudentReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(context.Context, string, ...interface{}) (pgx.Rows, error) {
			t.Fatal("query should not be called for invalid id")
			return nil, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/not-a-number/certificates", nil)
	req.SetPathValue("id", "not-a-number")
	rec := httptest.NewRecorder()

	handler.ListCertificatesByStudent(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListCertificatesByStudentReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(context.Context, string, ...interface{}) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/21/certificates", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListCertificatesByStudent(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListStudentsByCompanyIdReturnsStudentsResponse(t *testing.T) {
	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error {
				*(dest[0].(*int64)) = 11
				*(dest[1].(*string)) = "Jan"
				*(dest[2].(*string)) = "Nowak"
				*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
				*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.March, 3, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[5].(*string)) = "Warszawa"
				*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90030312345", Valid: true}
				return nil
			},
			func(dest ...any) error {
				*(dest[0].(*int64)) = 12
				*(dest[1].(*string)) = "Anna"
				*(dest[2].(*string)) = "Kowalska"
				*(dest[3].(*pgtype.Text)) = pgtype.Text{}
				*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1988, time.July, 11, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[5].(*string)) = "Krakow"
				*(dest[6].(*pgtype.Text)) = pgtype.Text{}
				return nil
			},
		},
	}

	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(_ context.Context, _ string, args ...interface{}) (pgx.Rows, error) {
			if len(args) != 1 {
				t.Fatalf("expected 1 query arg, got %d", len(args))
			}

			companyArg, ok := args[0].(pgtype.Int8)
			if !ok {
				t.Fatalf("expected company arg type pgtype.Int8, got %T", args[0])
			}
			if !companyArg.Valid || companyArg.Int64 != 7 {
				t.Fatalf("expected company arg 7, got %+v", companyArg)
			}

			return rows, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/7/students", nil)
	req.SetPathValue("id", "7")
	rec := httptest.NewRecorder()

	handler.ListStudentsByCompanyId(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody ListStudentsByCompanyIdResult
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 2 {
		t.Fatalf("expected 2 students, got %d", len(responseBody.Data))
	}

	first := responseBody.Data[0]
	if first.ID != 11 || first.Firstname != "Jan" || first.Lastname != "Nowak" {
		t.Fatalf("unexpected first student payload: %+v", first)
	}
	if first.Birthplace != "Warszawa" {
		t.Fatalf("expected birthplace Warszawa, got %q", first.Birthplace)
	}
	if first.Secondname == nil || *first.Secondname != "Adam" {
		t.Fatalf("expected second name to be mapped, got %+v", first.Secondname)
	}
	if first.Pesel == nil || *first.Pesel != "90030312345" {
		t.Fatalf("expected pesel to be mapped, got %+v", first.Pesel)
	}

	second := responseBody.Data[1]
	if second.ID != 12 || second.Firstname != "Anna" || second.Lastname != "Kowalska" {
		t.Fatalf("unexpected second student payload: %+v", second)
	}
	if second.Secondname != nil {
		t.Fatalf("expected nil second name, got %+v", second.Secondname)
	}
	if second.Pesel != nil {
		t.Fatalf("expected nil pesel, got %+v", second.Pesel)
	}
}

func TestListStudentsByCompanyIdReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(context.Context, string, ...interface{}) (pgx.Rows, error) {
			t.Fatal("query should not be called for invalid company id")
			return nil, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/abc/students", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.ListStudentsByCompanyId(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListStudentsByCompanyIdReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(context.Context, string, ...interface{}) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/7/students", nil)
	req.SetPathValue("id", "7")
	rec := httptest.NewRecorder()

	handler.ListStudentsByCompanyId(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPatchReturnsUpdatedStudent(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, args ...interface{}) pgx.Row {
			if len(args) != 12 {
				t.Fatalf("expected 12 query args, got %d", len(args))
			}

			firstNameArg, ok := args[0].(string)
			if !ok || firstNameArg != "Jan" {
				t.Fatalf("unexpected firstname arg: %+v", args[0])
			}

			lastNameArg, ok := args[1].(string)
			if !ok || lastNameArg != "Nowak" {
				t.Fatalf("unexpected lastname arg: %+v", args[1])
			}

			secondNameArg, ok := args[2].(pgtype.Text)
			if !ok || !secondNameArg.Valid || secondNameArg.String != "Adam" {
				t.Fatalf("unexpected secondname arg: %+v", args[2])
			}

			birthdateArg, ok := args[3].(pgtype.Date)
			if !ok || !birthdateArg.Valid || birthdateArg.Time.Format(response.DateFormat) != "1990-01-10" {
				t.Fatalf("unexpected birthdate arg: %+v", args[3])
			}

			birthplaceArg, ok := args[4].(string)
			if !ok || birthplaceArg != "Warszawa" {
				t.Fatalf("unexpected birthplace arg: %+v", args[4])
			}

			peselArg, ok := args[5].(pgtype.Text)
			if !ok || !peselArg.Valid || peselArg.String != "90011012345" {
				t.Fatalf("unexpected pesel arg: %+v", args[5])
			}

			addressStreetArg, ok := args[6].(pgtype.Text)
			if !ok || !addressStreetArg.Valid || addressStreetArg.String != "Koszykowa 1" {
				t.Fatalf("unexpected addressstreet arg: %+v", args[6])
			}

			addressCityArg, ok := args[7].(pgtype.Text)
			if !ok || !addressCityArg.Valid || addressCityArg.String != "Warszawa" {
				t.Fatalf("unexpected addresscity arg: %+v", args[7])
			}

			addressZipArg, ok := args[8].(pgtype.Text)
			if !ok || !addressZipArg.Valid || addressZipArg.String != "00-001" {
				t.Fatalf("unexpected addresszip arg: %+v", args[8])
			}

			telephoneArg, ok := args[9].(pgtype.Text)
			if !ok || !telephoneArg.Valid || telephoneArg.String != "123456789" {
				t.Fatalf("unexpected telephoneno arg: %+v", args[9])
			}

			companyArg, ok := args[10].(pgtype.Int8)
			if !ok || !companyArg.Valid || companyArg.Int64 != 8 {
				t.Fatalf("unexpected company arg: %+v", args[10])
			}

			studentIDArg, ok := args[11].(int64)
			if !ok || studentIDArg != 21 {
				t.Fatalf("unexpected student id arg: %+v", args[11])
			}

			return fakeRow{
				scan: func(dest ...any) error {
					*(dest[0].(*int64)) = 21
					*(dest[1].(*string)) = "Jan"
					*(dest[2].(*string)) = "Nowak"
					*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
					*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
					*(dest[5].(*string)) = "Warszawa"
					*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
					*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Koszykowa 1", Valid: true}
					*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "Warszawa", Valid: true}
					*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "00-001", Valid: true}
					*(dest[10].(*pgtype.Text)) = pgtype.Text{String: "123456789", Valid: true}
					*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 8, Valid: true}
					*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
					return nil
				},
			}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/students/21", strings.NewReader(`{
		"firstName":"  Jan  ",
		"lastName":"  Nowak ",
		"secondName":" Adam ",
		"birthDate":"1990-01-10",
		"birthPlace":"  Warszawa ",
		"pesel":"90011012345",
		"addressStreet":"Koszykowa 1",
		"addressCity":"Warszawa",
		"addressZip":"00-001",
		"telephone":" 123456789 ",
		"companyId":8
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody StudentDetailsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 21 || responseBody.Data.FirstName != "Jan" || responseBody.Data.LastName != "Nowak" {
		t.Fatalf("unexpected updated student payload: %+v", responseBody.Data)
	}
	if responseBody.Data.Company == nil || responseBody.Data.Company.ID != 8 {
		t.Fatalf("expected company to be mapped, got %+v", responseBody.Data.Company)
	}
}

func TestPatchReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid id")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/students/abc", strings.NewReader(`{}`))
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid json")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/students/21", strings.NewReader(`{`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForMissingRequiredField(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid body")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/students/21", strings.NewReader(`{
		"firstName":"Jan",
		"lastName":"",
		"birthDate":"1990-01-10",
		"birthPlace":"Warszawa"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForInvalidBirthDate(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid date")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/students/21", strings.NewReader(`{
		"firstName":"Jan",
		"lastName":"Nowak",
		"birthDate":"1990-15-99",
		"birthPlace":"Warszawa",
		"telephone":"123456789"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForInvalidCompanyID(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid company id")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/students/21", strings.NewReader(`{
		"firstName":"Jan",
		"lastName":"Nowak",
		"birthDate":"1990-01-10",
		"birthPlace":"Warszawa",
		"telephone":"123456789",
		"companyId":0
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsNotFoundWhenStudentDoesNotExist(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			return fakeRow{err: pgx.ErrNoRows}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/students/21", strings.NewReader(`{
		"firstName":"Jan",
		"lastName":"Nowak",
		"birthDate":"1990-01-10",
		"birthPlace":"Warszawa",
		"telephone":"123456789"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			return fakeRow{err: errors.New("db error")}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/students/21", strings.NewReader(`{
		"firstName":"Jan",
		"lastName":"Nowak",
		"birthDate":"1990-01-10",
		"birthPlace":"Warszawa",
		"telephone":"123456789"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestCreateStudentReturnsCreatedStudentResponse(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, args ...interface{}) pgx.Row {
			if len(args) != 11 {
				t.Fatalf("expected 11 query args, got %d", len(args))
			}

			if got, ok := args[0].(string); !ok || got != "Jan" {
				t.Fatalf("expected trimmed firstName, got %+v", args[0])
			}
			if got, ok := args[1].(string); !ok || got != "Nowak" {
				t.Fatalf("expected trimmed lastName, got %+v", args[1])
			}

			secondNameArg, ok := args[2].(pgtype.Text)
			if !ok || !secondNameArg.Valid || secondNameArg.String != "Adam" {
				t.Fatalf("expected secondName arg to be valid, got %+v", args[2])
			}

			birthDateArg, ok := args[3].(pgtype.Date)
			if !ok || !birthDateArg.Valid || birthDateArg.Time.Format(response.DateFormat) != "1990-01-10" {
				t.Fatalf("expected birthDate arg 1990-01-10, got %+v", args[3])
			}

			if got, ok := args[4].(string); !ok || got != "Warszawa" {
				t.Fatalf("expected trimmed birthPlace, got %+v", args[4])
			}

			peselArg, ok := args[5].(pgtype.Text)
			if !ok || !peselArg.Valid || peselArg.String != "90011012345" {
				t.Fatalf("expected pesel arg to be valid, got %+v", args[5])
			}

			addressStreetArg, ok := args[6].(pgtype.Text)
			if !ok || !addressStreetArg.Valid || addressStreetArg.String != "Koszykowa 1" {
				t.Fatalf("expected addressStreet arg to be valid, got %+v", args[6])
			}

			addressCityArg, ok := args[7].(pgtype.Text)
			if !ok || !addressCityArg.Valid || addressCityArg.String != "Warszawa" {
				t.Fatalf("expected addressCity arg to be valid, got %+v", args[7])
			}

			addressZipArg, ok := args[8].(pgtype.Text)
			if !ok || !addressZipArg.Valid || addressZipArg.String != "00-001" {
				t.Fatalf("expected addressZip arg to be valid, got %+v", args[8])
			}

			telephoneArg, ok := args[9].(pgtype.Text)
			if !ok || telephoneArg.Valid {
				t.Fatalf("expected telephone arg to be null, got %+v", args[9])
			}

			companyArg, ok := args[10].(pgtype.Int8)
			if !ok || !companyArg.Valid || companyArg.Int64 != 8 {
				t.Fatalf("expected company arg 8, got %+v", args[10])
			}

			return fakeRow{
				scan: func(dest ...any) error {
					*(dest[0].(*int64)) = 21
					*(dest[1].(*string)) = "Jan"
					*(dest[2].(*string)) = "Nowak"
					*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
					*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
					*(dest[5].(*string)) = "Warszawa"
					*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
					*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Koszykowa 1", Valid: true}
					*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "Warszawa", Valid: true}
					*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "00-001", Valid: true}
					*(dest[10].(*pgtype.Text)) = pgtype.Text{}
					*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 8, Valid: true}
					*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
					return nil
				},
			}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/students", strings.NewReader(`{
		"firstName": "  Jan ",
		"lastName": " Nowak  ",
		"secondName": " Adam ",
		"birthDate": "1990-01-10",
		"birthPlace": "  Warszawa ",
		"pesel": " 90011012345 ",
		"addressStreet": " Koszykowa 1 ",
		"addressCity": " Warszawa ",
		"addressZip": " 00-001 ",
		"telephone": null,
		"companyId": 8
	}`))
	rec := httptest.NewRecorder()

	handler.CreateStudent(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody StudentDetailsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 21 || responseBody.Data.FirstName != "Jan" || responseBody.Data.LastName != "Nowak" {
		t.Fatalf("unexpected student payload: %+v", responseBody.Data)
	}
	if responseBody.Data.Telephone != nil {
		t.Fatalf("expected nil telephone, got %+v", responseBody.Data.Telephone)
	}
	if responseBody.Data.Company == nil || responseBody.Data.Company.ID != 8 {
		t.Fatalf("expected company to be mapped, got %+v", responseBody.Data.Company)
	}
}

func TestCreateStudentReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid json")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/students", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	handler.CreateStudent(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateStudentReturnsBadRequestForMissingRequiredField(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid body")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/students", strings.NewReader(`{
		"firstName":"   ",
		"lastName":"Nowak",
		"birthDate":"1990-01-10",
		"birthPlace":"Warszawa"
	}`))
	rec := httptest.NewRecorder()

	handler.CreateStudent(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateStudentReturnsBadRequestForInvalidBirthDate(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid date")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/students", strings.NewReader(`{
		"firstName":"Jan",
		"lastName":"Nowak",
		"birthDate":"1990-15-99",
		"birthPlace":"Warszawa"
	}`))
	rec := httptest.NewRecorder()

	handler.CreateStudent(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateStudentReturnsBadRequestForInvalidCompanyID(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid company id")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/students", strings.NewReader(`{
		"firstName":"Jan",
		"lastName":"Nowak",
		"birthDate":"1990-01-10",
		"birthPlace":"Warszawa",
		"companyId":0
	}`))
	rec := httptest.NewRecorder()

	handler.CreateStudent(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateStudentReturnsBadRequestForUnknownField(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			t.Fatal("query row should not be called for invalid body")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/students", strings.NewReader(`{
		"firstName":"Jan",
		"lastName":"Nowak",
		"birthDate":"1990-01-10",
		"birthPlace":"Warszawa",
		"extra":"oops"
	}`))
	rec := httptest.NewRecorder()

	handler.CreateStudent(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateStudentReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...interface{}) pgx.Row {
			return fakeRow{err: errors.New("db error")}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/students", strings.NewReader(`{
		"firstName":"Jan",
		"lastName":"Nowak",
		"birthDate":"1990-01-10",
		"birthPlace":"Warszawa"
	}`))
	rec := httptest.NewRecorder()

	handler.CreateStudent(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}
