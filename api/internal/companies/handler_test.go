package companies

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeDB struct {
	query    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (f fakeDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec call")
}

func (f fakeDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.query == nil {
		return nil, errors.New("unexpected query call")
	}
	return f.query(ctx, sql, args...)
}

func (f fakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRow == nil {
		return fakeRow{err: errors.New("unexpected query row call")}
	}
	return f.queryRow(ctx, sql, args...)
}

type fakeRow struct {
	err  error
	scan func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
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

type fakeCreator struct {
	createFunc func(ctx context.Context, req CreateCompanyRequest) (CompanyDetailsDTO, error)
	updateFunc func(ctx context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error)
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

func (f fakeCreator) Create(ctx context.Context, req CreateCompanyRequest) (CompanyDetailsDTO, error) {
	if f.createFunc == nil {
		return CompanyDetailsDTO{}, errors.New("unexpected Create call")
	}
	return f.createFunc(ctx, req)
}

func (f fakeCreator) Update(ctx context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error) {
	if f.updateFunc == nil {
		return CompanyDetailsDTO{}, errors.New("unexpected Update call")
	}
	return f.updateFunc(ctx, companyID, req)
}

func ptrString(value string) *string {
	return &value
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

func TestListReturnsCompaniesResponse(t *testing.T) {
	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error {
				*(dest[0].(*int64)) = 1
				*(dest[1].(*string)) = "ABC Sp. z o.o."
				*(dest[2].(*string)) = "Warszawa"
				*(dest[3].(*string)) = "1234567890"
				*(dest[4].(*pgtype.Text)) = pgtype.Text{String: "Jan Nowak", Valid: true}
				*(dest[5].(*string)) = "500600700"
				return nil
			},
			func(dest ...any) error {
				*(dest[0].(*int64)) = 2
				*(dest[1].(*string)) = "XYZ SA"
				*(dest[2].(*string)) = "Krakow"
				*(dest[3].(*string)) = "0987654321"
				*(dest[4].(*pgtype.Text)) = pgtype.Text{}
				*(dest[5].(*string)) = "111222333"
				return nil
			},
		},
	}

	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
			if len(args) != 2 {
				t.Fatalf("expected 2 query args, got %d", len(args))
			}

			searchArg, ok := args[0].(pgtype.Text)
			if !ok {
				t.Fatalf("expected search arg type pgtype.Text, got %T", args[0])
			}
			if searchArg.Valid {
				t.Fatalf("expected empty search arg, got %+v", searchArg)
			}

			limitArg, ok := args[1].(int32)
			if !ok {
				t.Fatalf("expected limit arg type int32, got %T", args[1])
			}
			if limitArg != 50 {
				t.Fatalf("expected default limit 50, got %d", limitArg)
			}
			return rows, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var response ListCompaniesResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Fatalf("expected 2 companies, got %d", len(response.Data))
	}

	first := response.Data[0]
	if first.ID != 1 || first.Name != "ABC Sp. z o.o." || first.City != "Warszawa" {
		t.Fatalf("unexpected first company payload: %+v", first)
	}
	if first.NIP != "1234567890" || first.ContactPerson != "Jan Nowak" || first.Telephone != "500600700" {
		t.Fatalf("unexpected first company contact payload: %+v", first)
	}

	second := response.Data[1]
	if second.ID != 2 || second.Name != "XYZ SA" || second.City != "Krakow" {
		t.Fatalf("unexpected second company payload: %+v", second)
	}
	if second.ContactPerson != "" {
		t.Fatalf("expected empty contact person, got %q", second.ContactPerson)
	}
}

func TestListReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
			if len(args) != 2 {
				t.Fatalf("expected 2 query args, got %d", len(args))
			}
			return nil, errors.New("db error")
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListReturnsBadRequestForInvalidLimit(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(context.Context, string, ...any) (pgx.Rows, error) {
			t.Fatal("query should not be called for invalid limit")
			return nil, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies?limit=abc", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListReturnsBadRequestForOutOfRangeLimit(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(context.Context, string, ...any) (pgx.Rows, error) {
			t.Fatal("query should not be called for invalid limit")
			return nil, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies?limit=101", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListPassesFiltersToQuery(t *testing.T) {
	rows := &fakeRows{}

	handler := NewHandler(dbsqlc.New(fakeDB{
		query: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
			if len(args) != 2 {
				t.Fatalf("expected 2 query args, got %d", len(args))
			}

			searchArg := args[0].(pgtype.Text)
			if !searchArg.Valid || searchArg.String != "abc" {
				t.Fatalf("expected search arg %q, got %+v", "abc", searchArg)
			}

			limitArg := args[1].(int32)
			if limitArg != 20 {
				t.Fatalf("expected limit arg 20, got %d", limitArg)
			}

			return rows, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies?search=abc&limit=20", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestGetReturnsCompanyDetailsResponse(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, args ...any) pgx.Row {
			if len(args) != 1 {
				t.Fatalf("expected 1 query arg, got %d", len(args))
			}

			idArg, ok := args[0].(int64)
			if !ok || idArg != 15 {
				t.Fatalf("expected company id 15, got %+v", args[0])
			}

			return fakeRow{
				scan: func(dest ...any) error {
					*(dest[0].(*int64)) = 15
					*(dest[1].(*string)) = "ABC Sp. z o.o."
					*(dest[2].(*string)) = "Koszykowa 1"
					*(dest[3].(*string)) = "Warszawa"
					*(dest[4].(*string)) = "00-001"
					*(dest[5].(*string)) = "1234567890"
					*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "biuro@abc.pl", Valid: true}
					*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Jan Nowak", Valid: true}
					*(dest[8].(*string)) = "500600700"
					*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "Kluczowy klient", Valid: true}
					return nil
				},
			}
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/15", nil)
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody CompanyDetailsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 15 || responseBody.Data.Name != "ABC Sp. z o.o." {
		t.Fatalf("unexpected company details payload: %+v", responseBody.Data)
	}
	if responseBody.Data.Email == nil || *responseBody.Data.Email != "biuro@abc.pl" {
		t.Fatalf("expected email to be mapped, got %+v", responseBody.Data.Email)
	}
	if responseBody.Data.Contactperson == nil || *responseBody.Data.Contactperson != "Jan Nowak" {
		t.Fatalf("expected contact person to be mapped, got %+v", responseBody.Data.Contactperson)
	}
	if responseBody.Data.Note == nil || *responseBody.Data.Note != "Kluczowy klient" {
		t.Fatalf("expected note to be mapped, got %+v", responseBody.Data.Note)
	}
}

func TestGetReturnsBadRequestForInvalidCompanyID(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGetReturnsNotFoundWhenCompanyDoesNotExist(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...any) pgx.Row {
			return fakeRow{err: pgx.ErrNoRows}
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/22", nil)
	req.SetPathValue("id", "22")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestGetReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(context.Context, string, ...any) pgx.Row {
			return fakeRow{err: errors.New("db error")}
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/22", nil)
	req.SetPathValue("id", "22")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPatchReturnsUpdatedCompanyResponse(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{}), fakeCreator{
		updateFunc: func(_ context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error) {
			if companyID != 15 {
				t.Fatalf("expected company id 15, got %d", companyID)
			}
			if req.Name != "ABC Sp. z o.o." || req.Street != "Koszykowa 1" || req.City != "Warszawa" || req.Zipcode != "00-001" || req.Nip != "1234567890" || req.Telephone != "500600700" {
				t.Fatalf("unexpected update request: %+v", req)
			}
			if req.Email == nil || *req.Email != "  biuro@abc.pl " {
				t.Fatalf("expected raw email pointer, got %+v", req.Email)
			}
			return CompanyDetailsDTO{
				ID:            15,
				Name:          "ABC Sp. z o.o.",
				Street:        "Koszykowa 1",
				City:          "Warszawa",
				Zipcode:       "00-001",
				Nip:           "1234567890",
				Email:         ptrString("biuro@abc.pl"),
				Contactperson: ptrString("Jan Nowak"),
				Telephoneno:   "500600700",
				Note:          ptrString("Kluczowy klient"),
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/15", strings.NewReader(`{
		"name": "  ABC Sp. z o.o.  ",
		"street": "  Koszykowa 1 ",
		"city": " Warszawa ",
		"zipcode": " 00-001 ",
		"nip": " 1234567890 ",
		"email": "  biuro@abc.pl ",
		"contactPerson": "  Jan Nowak ",
		"telephone": " 500600700 ",
		"note": "  Kluczowy klient "
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody CompanyDetailsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 15 || responseBody.Data.Name != "ABC Sp. z o.o." {
		t.Fatalf("unexpected company details payload: %+v", responseBody.Data)
	}
	if responseBody.Data.Email == nil || *responseBody.Data.Email != "biuro@abc.pl" {
		t.Fatalf("expected email to be mapped, got %+v", responseBody.Data.Email)
	}
}

func TestPatchReturnsBadRequestForInvalidCompanyID(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			t.Fatal("query row should not be called for invalid company id")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/abc", strings.NewReader(`{}`))
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			t.Fatal("query row should not be called for invalid body")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/15", strings.NewReader(`{`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForUnknownField(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			t.Fatal("query row should not be called for invalid body")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/15", strings.NewReader(`{
		"name": "ABC",
		"street": "Koszykowa 1",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"telephone": "500600700",
		"unknown": "x"
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForMissingRequiredField(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			t.Fatal("query row should not be called for invalid body")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/15", strings.NewReader(`{
		"name": "ABC",
		"street": "",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"telephone": "500600700"
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchAllowsNilOptionalFields(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{}), fakeCreator{
		updateFunc: func(_ context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error) {
			if req.Email != nil || req.ContactPerson != nil || req.Note != nil {
				t.Fatalf("expected nil optional fields, got %+v", req)
			}
			return CompanyDetailsDTO{
				ID:          15,
				Name:        "ABC",
				Street:      "Koszykowa 1",
				City:        "Warszawa",
				Zipcode:     "00-001",
				Nip:         "1234567890",
				Telephoneno: "500600700",
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/15", strings.NewReader(`{
		"name": "ABC",
		"street": "Koszykowa 1",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"email": null,
		"contactPerson": null,
		"telephone": "500600700",
		"note": null
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestPatchReturnsNotFoundWhenCompanyDoesNotExist(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{}), fakeCreator{
		updateFunc: func(_ context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error) {
			return CompanyDetailsDTO{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/99", strings.NewReader(`{
		"name": "ABC",
		"street": "Koszykowa 1",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"telephone": "500600700"
	}`))
	req.SetPathValue("id", "99")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{}), fakeCreator{
		updateFunc: func(_ context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error) {
			return CompanyDetailsDTO{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/15", strings.NewReader(`{
		"name": "ABC",
		"street": "Koszykowa 1",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"telephone": "500600700"
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPatchReturnsConflictWhenNIPAlreadyExists(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{}), fakeCreator{
		updateFunc: func(_ context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error) {
			return CompanyDetailsDTO{}, &pgconn.PgError{Code: "23505", ConstraintName: "check_unique_nip"}
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/15", strings.NewReader(`{
		"name": "ABC",
		"street": "Koszykowa 1",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"telephone": "500600700"
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}

	var responseBody response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if responseBody.Error.Code != response.CodeConflict {
		t.Fatalf("expected error code %q, got %q", response.CodeConflict, responseBody.Error.Code)
	}
	if responseBody.Error.Message != "company with this NIP already exists" {
		t.Fatalf("expected unique NIP error message, got %q", responseBody.Error.Message)
	}
}

func TestCreateCompanyReturnsCreatedCompanyResponse(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{}), fakeCreator{
		createFunc: func(_ context.Context, req CreateCompanyRequest) (CompanyDetailsDTO, error) {
			if req.Name != "ABC Sp. z o.o." || req.Street != "Koszykowa 1" || req.City != "Warszawa" || req.Zipcode != "00-001" || req.Nip != "1234567890" || req.Telephone != "500600700" {
				t.Fatalf("unexpected create request: %+v", req)
			}
			return CompanyDetailsDTO{
				ID:            15,
				Name:          "ABC Sp. z o.o.",
				Street:        "Koszykowa 1",
				City:          "Warszawa",
				Zipcode:       "00-001",
				Nip:           "1234567890",
				Email:         ptrString("biuro@abc.pl"),
				Contactperson: ptrString("Jan Nowak"),
				Telephoneno:   "500600700",
				Note:          ptrString("Kluczowy klient"),
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/companies", strings.NewReader(`{
		"name": "  ABC Sp. z o.o.  ",
		"street": "  Koszykowa 1 ",
		"city": " Warszawa ",
		"zipcode": " 00-001 ",
		"nip": " 1234567890 ",
		"email": "  biuro@abc.pl ",
		"contactPerson": "  Jan Nowak ",
		"telephone": " 500600700 ",
		"note": "  Kluczowy klient "
	}`))
	rec := httptest.NewRecorder()

	handler.CreateCompany(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody CompanyDetailsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 15 || responseBody.Data.Name != "ABC Sp. z o.o." {
		t.Fatalf("unexpected company payload: %+v", responseBody.Data)
	}
	if responseBody.Data.Contactperson == nil || *responseBody.Data.Contactperson != "Jan Nowak" {
		t.Fatalf("expected contact person to be mapped, got %+v", responseBody.Data.Contactperson)
	}
}

func TestCreateCompanyReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			t.Fatal("query row should not be called for invalid json")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/companies", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	handler.CreateCompany(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateCompanyReturnsBadRequestForUnknownField(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			t.Fatal("query row should not be called for invalid body")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/companies", strings.NewReader(`{
		"name": "ABC",
		"street": "Koszykowa 1",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"telephone": "500600700",
		"extra": "oops"
	}`))
	rec := httptest.NewRecorder()

	handler.CreateCompany(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateCompanyReturnsBadRequestForMissingRequiredField(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			t.Fatal("query row should not be called for invalid body")
			return fakeRow{}
		},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/companies", strings.NewReader(`{
		"name": "   ",
		"street": "Koszykowa 1",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"telephone": "500600700"
	}`))
	rec := httptest.NewRecorder()

	handler.CreateCompany(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateCompanyReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{}), fakeCreator{
		createFunc: func(_ context.Context, req CreateCompanyRequest) (CompanyDetailsDTO, error) {
			return CompanyDetailsDTO{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/companies", strings.NewReader(`{
		"name": "ABC",
		"street": "Koszykowa 1",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"telephone": "500600700"
	}`))
	rec := httptest.NewRecorder()

	handler.CreateCompany(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestCreateCompanyReturnsConflictWhenNIPAlreadyExists(t *testing.T) {
	handler := NewHandler(dbsqlc.New(fakeDB{}), fakeCreator{
		createFunc: func(_ context.Context, req CreateCompanyRequest) (CompanyDetailsDTO, error) {
			return CompanyDetailsDTO{}, &pgconn.PgError{Code: "23505", ConstraintName: "check_unique_nip"}
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/companies", strings.NewReader(`{
		"name": "ABC",
		"street": "Koszykowa 1",
		"city": "Warszawa",
		"zipcode": "00-001",
		"nip": "1234567890",
		"telephone": "500600700"
	}`))
	rec := httptest.NewRecorder()

	handler.CreateCompany(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}

	var responseBody response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if responseBody.Error.Code != response.CodeConflict {
		t.Fatalf("expected error code %q, got %q", response.CodeConflict, responseBody.Error.Code)
	}
	if responseBody.Error.Message != "company with this NIP already exists" {
		t.Fatalf("expected unique NIP error message, got %q", responseBody.Error.Message)
	}
}
