package certificates

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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeQuerier struct {
	listCertificatesFunc                                   func(ctx context.Context, arg sqlc.ListCertificatesParams) ([]sqlc.ListCertificatesRow, error)
	getCertificateByIDFunc                                 func(ctx context.Context, id int64) (sqlc.GetCertificateByIDRow, error)
	getCourseByIDFunc                                      func(ctx context.Context, id int64) (sqlc.Course, error)
	listCourseCertificateTranslationsByCourseIDFunc        func(ctx context.Context, courseID int64) ([]sqlc.ListCourseCertificateTranslationsByCourseIDRow, error)
	getCourseCertificateTranslationByCourseAndLanguageFunc func(ctx context.Context, arg sqlc.GetCourseCertificateTranslationByCourseAndLanguageParams) (sqlc.GetCourseCertificateTranslationByCourseAndLanguageRow, error)
	updateCertificateFunc                                  func(ctx context.Context, arg sqlc.UpdateCertificateParams) (sqlc.UpdateCertificateRow, error)
	softDeleteFunc                                         func(ctx context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error)
}

type fakeCreator struct {
	createFunc func(ctx context.Context, input CreateCertificateInput) (CreateCertificateResult, error)
	updateFunc func(ctx context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error)
}

func (f fakeQuerier) ListCertificates(ctx context.Context, arg sqlc.ListCertificatesParams) ([]sqlc.ListCertificatesRow, error) {
	if f.listCertificatesFunc == nil {
		return nil, errors.New("unexpected ListCertificates call")
	}
	return f.listCertificatesFunc(ctx, arg)
}

func (f fakeQuerier) GetCertificateByID(ctx context.Context, id int64) (sqlc.GetCertificateByIDRow, error) {
	if f.getCertificateByIDFunc == nil {
		return sqlc.GetCertificateByIDRow{}, errors.New("unexpected GetCertificateByID call")
	}
	return f.getCertificateByIDFunc(ctx, id)
}

func (f fakeQuerier) GetCourseByID(ctx context.Context, id int64) (sqlc.Course, error) {
	if f.getCourseByIDFunc == nil {
		return sqlc.Course{}, errors.New("unexpected GetCourseByID call")
	}
	return f.getCourseByIDFunc(ctx, id)
}

func (f fakeQuerier) ListCourseCertificateTranslationsByCourseID(ctx context.Context, courseID int64) ([]sqlc.ListCourseCertificateTranslationsByCourseIDRow, error) {
	if f.listCourseCertificateTranslationsByCourseIDFunc == nil {
		return nil, errors.New("unexpected ListCourseCertificateTranslationsByCourseID call")
	}
	return f.listCourseCertificateTranslationsByCourseIDFunc(ctx, courseID)
}

func (f fakeQuerier) GetCourseCertificateTranslationByCourseAndLanguage(ctx context.Context, arg sqlc.GetCourseCertificateTranslationByCourseAndLanguageParams) (sqlc.GetCourseCertificateTranslationByCourseAndLanguageRow, error) {
	if f.getCourseCertificateTranslationByCourseAndLanguageFunc == nil {
		return sqlc.GetCourseCertificateTranslationByCourseAndLanguageRow{}, errors.New("unexpected GetCourseCertificateTranslationByCourseAndLanguage call")
	}
	return f.getCourseCertificateTranslationByCourseAndLanguageFunc(ctx, arg)
}

func (f fakeQuerier) UpdateCertificate(ctx context.Context, arg sqlc.UpdateCertificateParams) (sqlc.UpdateCertificateRow, error) {
	if f.updateCertificateFunc == nil {
		return sqlc.UpdateCertificateRow{}, errors.New("unexpected UpdateCertificate call")
	}
	return f.updateCertificateFunc(ctx, arg)
}

func (f fakeQuerier) SoftDeleteCertificate(ctx context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error) {
	if f.softDeleteFunc == nil {
		return 0, errors.New("unexpected SoftDeleteCertificate call")
	}
	return f.softDeleteFunc(ctx, arg)
}

func (f fakeCreator) Create(ctx context.Context, input CreateCertificateInput) (CreateCertificateResult, error) {
	if f.createFunc == nil {
		return CreateCertificateResult{}, errors.New("unexpected Create call")
	}
	return f.createFunc(ctx, input)
}

func (f fakeCreator) Update(ctx context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error) {
	if f.updateFunc == nil {
		return sqlc.UpdateCertificateRow{}, errors.New("unexpected Update call")
	}
	return f.updateFunc(ctx, certificateID, input)
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

func TestListReturnsCertificatesResponse(t *testing.T) {
	rows := []sqlc.ListCertificatesRow{
		{
			ID:               15,
			Date:             pgtype.Date{Time: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			StudentFirstname: "Jan",
			StudentLastname:  "Nowak",
			CompanyName:      pgtype.Text{String: "ABC Sp. z o.o.", Valid: true},
			CourseName:       "Szkolenie BHP",
			CourseSymbol:     "BHP",
			RegistryYear:     2026,
			RegistryNumber:   14,
			CourseDateStart:  pgtype.Date{Time: time.Date(2026, time.February, 27, 0, 0, 0, 0, time.UTC), Valid: true},
			CourseDateEnd:    pgtype.Date{Time: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			ExpiryDate:       "2027-03-01",
		},
		{
			ID:               16,
			Date:             pgtype.Date{Time: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC), Valid: true},
			StudentFirstname: "Anna",
			StudentLastname:  "Kowalska",
			CompanyName:      pgtype.Text{},
			CourseName:       "Instruktaz",
			CourseSymbol:     "INS",
			RegistryYear:     2026,
			RegistryNumber:   15,
			CourseDateStart:  pgtype.Date{Time: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC), Valid: true},
			CourseDateEnd:    pgtype.Date{},
			ExpiryDate:       "",
		},
	}

	handler := NewHandler(fakeQuerier{
		listCertificatesFunc: func(_ context.Context, arg sqlc.ListCertificatesParams) ([]sqlc.ListCertificatesRow, error) {
			if arg.Search.Valid {
				t.Fatalf("expected empty search param, got %+v", arg.Search)
			}
			if arg.LimitCount != 50 {
				t.Fatalf("expected default limit 50, got %d", arg.LimitCount)
			}
			return rows, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody ListCertificatesResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 2 {
		t.Fatalf("expected 2 certificates, got %d", len(responseBody.Data))
	}

	first := responseBody.Data[0]
	if first.ID != 15 || first.StudentName != "Jan Nowak" || first.CompanyName != "ABC Sp. z o.o." {
		t.Fatalf("unexpected first certificate payload: %+v", first)
	}
	if first.CourseDateEnd == nil || *first.CourseDateEnd != "2026-03-01" {
		t.Fatalf("expected first courseDateEnd to be %q, got %+v", "2026-03-01", first.CourseDateEnd)
	}
	if first.ExpiryDate == nil || *first.ExpiryDate != "2027-03-01" {
		t.Fatalf("expected first expiryDate to be %q, got %+v", "2027-03-01", first.ExpiryDate)
	}

	second := responseBody.Data[1]
	if second.ID != 16 || second.StudentName != "Anna Kowalska" {
		t.Fatalf("unexpected second certificate payload: %+v", second)
	}
	if second.CourseDateEnd != nil {
		t.Fatalf("expected second courseDateEnd to be nil, got %+v", second.CourseDateEnd)
	}
	if second.ExpiryDate != nil {
		t.Fatalf("expected second expiryDate to be nil, got %+v", second.ExpiryDate)
	}
}

func TestListReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listCertificatesFunc: func(_ context.Context, arg sqlc.ListCertificatesParams) ([]sqlc.ListCertificatesRow, error) {
			if arg.LimitCount != 50 {
				t.Fatalf("expected default limit 50, got %d", arg.LimitCount)
			}
			return nil, errors.New("db error")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListPassesSearchAndLimitToQuery(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listCertificatesFunc: func(_ context.Context, arg sqlc.ListCertificatesParams) ([]sqlc.ListCertificatesRow, error) {
			if !arg.Search.Valid || arg.Search.String != "nowak" {
				t.Fatalf("expected search=nowak, got %+v", arg.Search)
			}
			if arg.LimitCount != 20 {
				t.Fatalf("expected limit 20, got %d", arg.LimitCount)
			}
			return []sqlc.ListCertificatesRow{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates?search=nowak&limit=20", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestListReturnsBadRequestForInvalidLimit(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listCertificatesFunc: func(_ context.Context, arg sqlc.ListCertificatesParams) ([]sqlc.ListCertificatesRow, error) {
			t.Fatalf("ListCertificates should not be called for invalid limit, got %+v", arg)
			return nil, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates?limit=abc", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListReturnsBadRequestForLimitAboveMaximum(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listCertificatesFunc: func(_ context.Context, arg sqlc.ListCertificatesParams) ([]sqlc.ListCertificatesRow, error) {
			t.Fatalf("ListCertificates should not be called for invalid limit, got %+v", arg)
			return nil, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates?limit=101", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGetReturnsCertificateDetails(t *testing.T) {
	row := sqlc.GetCertificateByIDRow{
		ID:                21,
		Date:              pgtype.Date{Time: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC), Valid: true},
		StudentFirstname:  "Jan",
		StudentSecondname: pgtype.Text{String: "Adam", Valid: true},
		StudentLastname:   "Nowak",
		StudentBirthdate:  pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true},
		StudentBirthplace: "Warszawa",
		StudentPesel:      pgtype.Text{String: "90011012345", Valid: true},
		CompanyName:       pgtype.Text{String: "ABC Sp. z o.o.", Valid: true},
		CourseDateStart:   pgtype.Date{Time: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		CourseDateEnd:     pgtype.Date{Time: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC), Valid: true},
		RegistryYear:      2026,
		RegistryNumber:    17,
		CourseID:          3,
		CourseName:        "Szkolenie BHP",
		CourseSymbol:      "BHP",
		CourseExpiryTime:  pgtype.Text{String: "3", Valid: true},
		CourseProgram:     `{"sections":["intro"]}`,
		CertFrontPage:     "<p>Front</p>",
		LanguageCode:      "pl",
		JournalAttendeeID: pgtype.Int8{Int64: 7, Valid: true},
		JournalID:         pgtype.Int8{Int64: 4, Valid: true},
		JournalTitle:      pgtype.Text{String: "Szkolenie okresowe BHP - marzec 2026", Valid: true},
		JournalStatus:     pgtype.Text{String: "closed", Valid: true},
		ExpiryDate:        "2029-03-05",
	}

	handler := NewHandler(fakeQuerier{
		getCertificateByIDFunc: func(_ context.Context, id int64) (sqlc.GetCertificateByIDRow, error) {
			if id != 21 {
				t.Fatalf("expected certificate id %d, got %d", 21, id)
			}
			return row, nil
		},
		getCourseByIDFunc: func(_ context.Context, id int64) (sqlc.Course, error) {
			if id != 3 {
				t.Fatalf("expected course id %d, got %d", 3, id)
			}
			return sqlc.Course{
				ID:            3,
				Name:          "Szkolenie BHP",
				Courseprogram: []byte(`[ {"Subject":"Podstawy"} ]`),
				Certfrontpage: pgtype.Text{String: "<p>Polski szablon</p>", Valid: true},
			}, nil
		},
		listCourseCertificateTranslationsByCourseIDFunc: func(_ context.Context, courseID int64) ([]sqlc.ListCourseCertificateTranslationsByCourseIDRow, error) {
			return []sqlc.ListCourseCertificateTranslationsByCourseIDRow{
				{
					LanguageCode:  "en",
					CourseName:    "Health and Safety Training",
					CourseProgram: `[ {"Subject":"Intro"} ]`,
					CertFrontPage: "<p>English template</p>",
				},
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates/21", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody CertificateResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 21 || responseBody.Data.StudentName != "Jan" || responseBody.Data.StudentLastname != "Nowak" {
		t.Fatalf("unexpected certificate details payload: %+v", responseBody.Data)
	}
	if responseBody.Data.CourseExpiryTime == nil || *responseBody.Data.CourseExpiryTime != 3 {
		t.Fatalf("expected courseExpiryTime to be %d, got %+v", 3, responseBody.Data.CourseExpiryTime)
	}
	if responseBody.Data.CourseProgram != `{"sections":["intro"]}` {
		t.Fatalf("unexpected courseProgram: %q", responseBody.Data.CourseProgram)
	}
	if responseBody.Data.ExpiryDate == nil || *responseBody.Data.ExpiryDate != "2029-03-05" {
		t.Fatalf("expected expiryDate to be %q, got %+v", "2029-03-05", responseBody.Data.ExpiryDate)
	}
	if responseBody.Data.Journal == nil {
		t.Fatalf("expected linked journal to be mapped, got nil")
	}
	if responseBody.Data.Journal.ID != 4 || responseBody.Data.Journal.Title != "Szkolenie okresowe BHP - marzec 2026" || responseBody.Data.Journal.Status != "closed" {
		t.Fatalf("unexpected journal payload: %+v", responseBody.Data.Journal)
	}
	if len(responseBody.Data.PrintVariants) != 2 {
		t.Fatalf("expected 2 print variants, got %+v", responseBody.Data.PrintVariants)
	}
	if !responseBody.Data.PrintVariants[0].IsOriginal || responseBody.Data.PrintVariants[0].LanguageCode != responseBody.Data.LanguageCode {
		t.Fatalf("expected original variant to be first, got %+v", responseBody.Data.PrintVariants[0])
	}
}

func TestGetReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getCertificateByIDFunc: func(_ context.Context, id int64) (sqlc.GetCertificateByIDRow, error) {
			t.Fatalf("GetCertificateByID should not be called for invalid id, got %d", id)
			return sqlc.GetCertificateByIDRow{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates/not-a-number", nil)
	req.SetPathValue("id", "not-a-number")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGetReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getCertificateByIDFunc: func(_ context.Context, id int64) (sqlc.GetCertificateByIDRow, error) {
			return sqlc.GetCertificateByIDRow{}, errors.New("db error")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates/21", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPDFReturnsRenderedPDF(t *testing.T) {
	originalRenderer := renderCertificatePDF
	t.Cleanup(func() {
		renderCertificatePDF = originalRenderer
	})

	row := sqlc.GetCertificateByIDRow{
		ID:                21,
		Date:              pgtype.Date{Time: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC), Valid: true},
		StudentFirstname:  "Jan",
		StudentSecondname: pgtype.Text{String: "Adam", Valid: true},
		StudentLastname:   "Nowak",
		StudentBirthdate:  pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true},
		StudentBirthplace: "Warszawa",
		StudentPesel:      pgtype.Text{String: "90011012345", Valid: true},
		CompanyName:       pgtype.Text{String: "ABC Sp. z o.o.", Valid: true},
		CourseDateStart:   pgtype.Date{Time: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		CourseDateEnd:     pgtype.Date{Time: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC), Valid: true},
		RegistryYear:      2026,
		RegistryNumber:    17,
		CourseName:        "Szkolenie BHP",
		CourseSymbol:      "BHP",
		CourseProgram:     `[{"Subject":"Intro","TheoryTime":"2","PracticeTime":"1"}]`,
		CertFrontPage:     "<p>{{ imie }} {{ nazwisko }}</p><p>{{ data_urodzenia }}</p><p>{{ numer_zaswiadczenia }}</p>",
	}

	renderCertificatePDF = func(ctx context.Context, pageHTML string) ([]byte, error) {
		if !strings.Contains(pageHTML, "Jan Nowak") {
			t.Fatalf("expected rendered HTML to contain substituted name, got %q", pageHTML)
		}
		if !strings.Contains(pageHTML, "10.01.1990") {
			t.Fatalf("expected rendered HTML to contain substituted birth date, got %q", pageHTML)
		}
		if !strings.Contains(pageHTML, "17/BHP/2026") {
			t.Fatalf("expected rendered HTML to contain substituted certificate number, got %q", pageHTML)
		}
		return []byte("%PDF-1.4 fake"), nil
	}

	handler := NewHandler(fakeQuerier{
		getCertificateByIDFunc: func(_ context.Context, id int64) (sqlc.GetCertificateByIDRow, error) {
			if id != 21 {
				t.Fatalf("expected certificate id %d, got %d", 21, id)
			}
			return row, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates/21/pdf", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.PDF(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("expected application/pdf content type, got %q", got)
	}

	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment;") {
		t.Fatalf("expected attachment content disposition, got %q", got)
	}

	if rec.Body.String() != "%PDF-1.4 fake" {
		t.Fatalf("unexpected PDF body: %q", rec.Body.String())
	}
}

func TestPDFUsesRequestedLanguageVariant(t *testing.T) {
	originalRenderer := renderCertificatePDF
	t.Cleanup(func() {
		renderCertificatePDF = originalRenderer
	})

	row := sqlc.GetCertificateByIDRow{
		ID:                21,
		Date:              pgtype.Date{Time: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC), Valid: true},
		StudentFirstname:  "Jan",
		StudentLastname:   "Nowak",
		StudentBirthdate:  pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true},
		StudentBirthplace: "Warszawa",
		CourseDateStart:   pgtype.Date{Time: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		CourseDateEnd:     pgtype.Date{Time: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC), Valid: true},
		RegistryYear:      2026,
		RegistryNumber:    17,
		CourseID:          3,
		CourseName:        "Szkolenie BHP",
		CourseSymbol:      "BHP",
		LanguageCode:      "pl",
		CourseProgram:     `[{"Subject":"Podstawy","TheoryTime":"2","PracticeTime":"1"}]`,
		CertFrontPage:     "<p>{{ nazwa_kursu }}</p>",
	}

	renderCertificatePDF = func(ctx context.Context, pageHTML string) ([]byte, error) {
		if !strings.Contains(pageHTML, "Health and Safety Training") {
			t.Fatalf("expected rendered HTML to use translated course name, got %q", pageHTML)
		}
		if !strings.Contains(pageHTML, "Training topic") {
			t.Fatalf("expected rendered HTML to use translated program labels, got %q", pageHTML)
		}
		return []byte("%PDF-1.4 fake"), nil
	}

	handler := NewHandler(fakeQuerier{
		getCertificateByIDFunc: func(_ context.Context, id int64) (sqlc.GetCertificateByIDRow, error) {
			return row, nil
		},
		getCourseCertificateTranslationByCourseAndLanguageFunc: func(_ context.Context, arg sqlc.GetCourseCertificateTranslationByCourseAndLanguageParams) (sqlc.GetCourseCertificateTranslationByCourseAndLanguageRow, error) {
			if arg.CourseID != 3 || arg.LanguageCode != "en" {
				t.Fatalf("unexpected translation lookup: %+v", arg)
			}
			return sqlc.GetCourseCertificateTranslationByCourseAndLanguageRow{
				CourseID:      3,
				LanguageCode:  "en",
				CourseName:    "Health and Safety Training",
				CourseProgram: `[{"Subject":"Introduction","TheoryTime":"2","PracticeTime":"1"}]`,
				CertFrontPage: "<p>{{ nazwa_kursu }}</p>",
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates/21/pdf?language=en", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.PDF(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestPDFReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getCertificateByIDFunc: func(_ context.Context, id int64) (sqlc.GetCertificateByIDRow, error) {
			t.Fatalf("GetCertificateByID should not be called for invalid id, got %d", id)
			return sqlc.GetCertificateByIDRow{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates/not-a-number/pdf", nil)
	req.SetPathValue("id", "not-a-number")
	rec := httptest.NewRecorder()

	handler.PDF(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPDFReturnsInternalServerErrorWhenRendererFails(t *testing.T) {
	originalRenderer := renderCertificatePDF
	t.Cleanup(func() {
		renderCertificatePDF = originalRenderer
	})

	renderCertificatePDF = func(ctx context.Context, pageHTML string) ([]byte, error) {
		return nil, errors.New("render failed")
	}

	handler := NewHandler(fakeQuerier{
		getCertificateByIDFunc: func(_ context.Context, id int64) (sqlc.GetCertificateByIDRow, error) {
			return sqlc.GetCertificateByIDRow{
				ID:               id,
				Date:             pgtype.Date{Time: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC), Valid: true},
				CourseDateStart:  pgtype.Date{Time: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Valid: true},
				StudentBirthdate: pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				StudentFirstname: "Jan",
				StudentLastname:  "Nowak",
				CertFrontPage:    "<p>Test</p>",
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/certificates/21/pdf", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.PDF(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestCreateReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		createFunc: func(context.Context, CreateCertificateInput) (CreateCertificateResult, error) {
			t.Fatal("Create should not be called for invalid JSON")
			return CreateCertificateResult{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/certificates", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateReturnsBadRequestForInvalidBusinessInput(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		createFunc: func(_ context.Context, input CreateCertificateInput) (CreateCertificateResult, error) {
			if input.StudentID != 12 || input.CourseID != 3 || input.RegistryNumber != 18 {
				t.Fatalf("unexpected create input: %+v", input)
			}
			if input.CourseDateEnd != nil {
				t.Fatalf("expected nil courseDateEnd, got %+v", input.CourseDateEnd)
			}
			return CreateCertificateResult{}, ErrInvalidInput
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/certificates", strings.NewReader(`{
		"studentId": 12,
		"courseId": 3,
		"certificateDate": "2026-03-15",
		"courseDateStart": "2026-03-10",
		"registryYear": 2026,
		"registryNumber": 18
	}`))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateReturnsInternalServerErrorWhenServiceFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		createFunc: func(_ context.Context, input CreateCertificateInput) (CreateCertificateResult, error) {
			if input.CourseDateEnd == nil || *input.CourseDateEnd != "2026-03-15" {
				t.Fatalf("expected courseDateEnd to be mapped, got %+v", input.CourseDateEnd)
			}
			return CreateCertificateResult{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/certificates", strings.NewReader(`{
		"studentId": 12,
		"courseId": 3,
		"certificateDate": "2026-03-15",
		"courseDateStart": "2026-03-10",
		"courseDateEnd": "2026-03-15",
		"registryYear": 2026,
		"registryNumber": 18
	}`))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestCreateReturnsCreatedResponse(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		createFunc: func(_ context.Context, input CreateCertificateInput) (CreateCertificateResult, error) {
			if input.StudentID != 12 || input.CourseID != 3 || input.RegistryYear != 2026 {
				t.Fatalf("unexpected create input: %+v", input)
			}
			return CreateCertificateResult{ID: 101}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/certificates", strings.NewReader(`{
		"studentId": 12,
		"courseId": 3,
		"certificateDate": "2026-03-15",
		"courseDateStart": "2026-03-10",
		"registryYear": 2026,
		"registryNumber": 18
	}`))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody CreateCertificateResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}

	if responseBody.Data.ID != 101 {
		t.Fatalf("expected created certificate id 101, got %d", responseBody.Data.ID)
	}
}

func TestPatchReturnsUpdatedCertificateResponse(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		updateFunc: func(_ context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error) {
			if certificateID != 21 {
				t.Fatalf("expected certificate id 21, got %d", certificateID)
			}
			if input.StudentID != 12 {
				t.Fatalf("expected student id 12, got %d", input.StudentID)
			}
			if input.CertificateDate != "2026-03-15" {
				t.Fatalf("unexpected certificate date: %q", input.CertificateDate)
			}
			if input.CourseDateStart != "2026-03-10" {
				t.Fatalf("unexpected course start date: %q", input.CourseDateStart)
			}
			if input.CourseDateEnd == nil || *input.CourseDateEnd != "2026-03-15" {
				t.Fatalf("unexpected course end date: %+v", input.CourseDateEnd)
			}

			return sqlc.UpdateCertificateRow{
				ID:                21,
				Date:              pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true},
				StudentID:         12,
				StudentFirstname:  "Jan",
				StudentSecondname: pgtype.Text{String: "Adam", Valid: true},
				StudentLastname:   "Nowak",
				StudentBirthdate:  pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				StudentBirthplace: "Warszawa",
				StudentPesel:      pgtype.Text{String: "90011012345", Valid: true},
				CompanyName:       pgtype.Text{String: "ABC Sp. z o.o.", Valid: true},
				CourseDateStart:   pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				CourseDateEnd:     pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true},
				RegistryYear:      2026,
				RegistryNumber:    18,
				CourseName:        "Szkolenie BHP",
				CourseSymbol:      "BHP",
				CourseExpiryTime:  pgtype.Text{String: "3", Valid: true},
				CourseProgram:     `[{"Subject":"Intro","TheoryTime":"2","PracticeTime":"1"}]`,
				CertFrontPage:     "<p>Front</p>",
				ExpiryDate:        "2029-03-15",
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/certificates/21", strings.NewReader(`{
		"studentId": 12,
		"certificateDate": "2026-03-15",
		"courseDateStart": "2026-03-10",
		"courseDateEnd": "2026-03-15"
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

	var responseBody CertificateResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 21 || responseBody.Data.RegistryNumber != 18 {
		t.Fatalf("unexpected updated certificate payload: %+v", responseBody.Data)
	}
	if responseBody.Data.CourseDateEnd == nil || *responseBody.Data.CourseDateEnd != "2026-03-15" {
		t.Fatalf("expected courseDateEnd to be mapped, got %+v", responseBody.Data.CourseDateEnd)
	}
	if responseBody.Data.ExpiryDate == nil || *responseBody.Data.ExpiryDate != "2029-03-15" {
		t.Fatalf("expected expiryDate to be mapped, got %+v", responseBody.Data.ExpiryDate)
	}
	if responseBody.Data.Journal != nil {
		t.Fatalf("expected journal to be nil when certificate is not linked, got %+v", responseBody.Data.Journal)
	}
}

func TestPatchReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		updateFunc: func(_ context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error) {
			t.Fatalf("Update should not be called for invalid id, got id=%d input=%+v", certificateID, input)
			return sqlc.UpdateCertificateRow{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/certificates/abc", strings.NewReader(`{}`))
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		updateFunc: func(_ context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error) {
			t.Fatalf("Update should not be called for invalid JSON, got id=%d input=%+v", certificateID, input)
			return sqlc.UpdateCertificateRow{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/certificates/21", strings.NewReader(`{`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForMissingRequiredFields(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		updateFunc: func(_ context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error) {
			if certificateID != 21 {
				t.Fatalf("expected certificate id 21, got %d", certificateID)
			}
			if input.StudentID != 0 || input.CertificateDate != "" || input.CourseDateStart != "" {
				t.Fatalf("expected raw invalid body to be forwarded, got %+v", input)
			}
			return sqlc.UpdateCertificateRow{}, ErrInvalidInput
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/certificates/21", strings.NewReader(`{
		"studentId": 0,
		"certificateDate": "",
		"courseDateStart": ""
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForInvalidDates(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		updateFunc: func(_ context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error) {
			if certificateID != 21 {
				t.Fatalf("expected certificate id 21, got %d", certificateID)
			}
			if input.CourseDateEnd == nil || *input.CourseDateEnd != "2026-03-10" {
				t.Fatalf("expected raw invalid courseDateEnd to be forwarded, got %+v", input.CourseDateEnd)
			}
			return sqlc.UpdateCertificateRow{}, ErrInvalidInput
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/certificates/21", strings.NewReader(`{
		"studentId": 12,
		"certificateDate": "2026-03-15",
		"courseDateStart": "2026-03-15",
		"courseDateEnd": "2026-03-10"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsNotFoundWhenCertificateDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		updateFunc: func(_ context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error) {
			return sqlc.UpdateCertificateRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/certificates/21", strings.NewReader(`{
		"studentId": 12,
		"certificateDate": "2026-03-15",
		"courseDateStart": "2026-03-10",
		"courseDateEnd": "2026-03-15"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		updateFunc: func(_ context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error) {
			return sqlc.UpdateCertificateRow{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/certificates/21", strings.NewReader(`{
		"studentId": 12,
		"certificateDate": "2026-03-15",
		"courseDateStart": "2026-03-10",
		"courseDateEnd": "2026-03-15"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestSoftDeleteCertificateReturnsDeletedCertificateResponse(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		softDeleteFunc: func(_ context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error) {
			if arg.ID != 21 {
				t.Fatalf("expected certificate id 21, got %d", arg.ID)
			}
			if !arg.DeletedByUserID.Valid || arg.DeletedByUserID.Int64 != 7 {
				t.Fatalf("expected deletedByUserId 7, got %+v", arg.DeletedByUserID)
			}
			if !arg.DeleteReason.Valid || arg.DeleteReason.String != "Wystawione omylkowo" {
				t.Fatalf("expected delete reason to be mapped, got %+v", arg.DeleteReason)
			}
			return 21, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/certificates/21", strings.NewReader(`{
		"deleteReason": "Wystawione omylkowo"
	}`))
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   7,
		Role: 1,
	}))
	rec := httptest.NewRecorder()

	handler.SoftDeleteCertificate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody DeleteCertificateResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 21 {
		t.Fatalf("expected deleted certificate id 21, got %d", responseBody.Data.ID)
	}
}

func TestSoftDeleteCertificateReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		softDeleteFunc: func(_ context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error) {
			t.Fatalf("SoftDeleteCertificate should not be called for invalid id, got %+v", arg)
			return 0, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/certificates/abc", strings.NewReader(`{}`))
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.SoftDeleteCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestSoftDeleteCertificateReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		softDeleteFunc: func(_ context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error) {
			t.Fatalf("SoftDeleteCertificate should not be called for invalid JSON, got %+v", arg)
			return 0, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/certificates/21", strings.NewReader(`{`))
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 7, Role: 1}))
	rec := httptest.NewRecorder()

	handler.SoftDeleteCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestSoftDeleteCertificateReturnsUnauthorizedWithoutUserInContext(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		softDeleteFunc: func(_ context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error) {
			t.Fatalf("SoftDeleteCertificate should not be called without user in context, got %+v", arg)
			return 0, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/certificates/21", strings.NewReader(`{}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.SoftDeleteCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestSoftDeleteCertificateReturnsNotFoundWhenCertificateDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		softDeleteFunc: func(_ context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error) {
			return 0, pgx.ErrNoRows
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/certificates/21", strings.NewReader(`{}`))
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 7, Role: 1}))
	rec := httptest.NewRecorder()

	handler.SoftDeleteCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestSoftDeleteCertificateReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		softDeleteFunc: func(_ context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error) {
			return 0, errors.New("db error")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/certificates/21", strings.NewReader(`{}`))
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 7, Role: 1}))
	rec := httptest.NewRecorder()

	handler.SoftDeleteCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}
