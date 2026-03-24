package journals

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeQuerier struct {
	listJournalsFunc                     func(ctx context.Context, arg sqlc.ListJournalsParams) ([]sqlc.ListJournalsRow, error)
	createJournalFunc                    func(ctx context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error)
	getJournalByIDFunc                   func(ctx context.Context, id int64) (sqlc.GetJournalByIDRow, error)
	getCourseByIDFunc                    func(ctx context.Context, id int64) (sqlc.Course, error)
	updateJournalHeaderFunc              func(ctx context.Context, arg sqlc.UpdateJournalHeaderParams) (sqlc.UpdateJournalHeaderRow, error)
	deleteJournalFunc                    func(ctx context.Context, id int64) (int64, error)
	closeJournalFunc                     func(ctx context.Context, id int64) (int64, error)
	listJournalAttendeesFunc             func(ctx context.Context, journalID int64) ([]sqlc.ListJournalAttendeesRow, error)
	addJournalAttendeeFunc               func(ctx context.Context, arg sqlc.AddJournalAttendeeParams) (sqlc.AddJournalAttendeeRow, error)
	updateJournalAttendeeCertificateFunc func(ctx context.Context, arg sqlc.UpdateJournalAttendeeCertificateParams) (sqlc.UpdateJournalAttendeeCertificateRow, error)
	deleteJournalAttendeeFunc            func(ctx context.Context, arg sqlc.DeleteJournalAttendeeParams) (int64, error)
	listJournalSessionsFunc              func(ctx context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error)
	generateSessionsFunc                 func(ctx context.Context, journalID int64) (int64, error)
	updateJournalSessionFunc             func(ctx context.Context, arg sqlc.UpdateJournalSessionParams) (sqlc.TrainingJournalSession, error)
	listJournalAttendanceFunc            func(ctx context.Context, journalID int64) ([]sqlc.TrainingJournalAttendance, error)
	upsertAttendanceFunc                 func(ctx context.Context, arg sqlc.UpsertJournalAttendanceParams) (sqlc.TrainingJournalAttendance, error)
	upsertAttendanceScanFunc             func(ctx context.Context, arg sqlc.UpsertJournalAttendanceScanParams) (sqlc.UpsertJournalAttendanceScanRow, error)
	getJournalAttendanceScanFileFunc     func(ctx context.Context, journalID int64) (sqlc.GetJournalAttendanceScanFileRow, error)
	getJournalAttendanceScanMetaFunc     func(ctx context.Context, journalID int64) (sqlc.GetJournalAttendanceScanMetaRow, error)
	deleteJournalAttendanceScanFunc      func(ctx context.Context, journalID int64) (int64, error)
}

type fakeCertificateGenerator struct {
	generateFunc func(ctx context.Context, journalID, attendeeID int64) (GenerateAttendeeCertificateResult, error)
}

func (f fakeCertificateGenerator) GenerateAttendeeCertificate(ctx context.Context, journalID, attendeeID int64) (GenerateAttendeeCertificateResult, error) {
	if f.generateFunc == nil {
		return GenerateAttendeeCertificateResult{}, errors.New("unexpected GenerateAttendeeCertificate call")
	}

	return f.generateFunc(ctx, journalID, attendeeID)
}

func (f fakeQuerier) ListJournals(ctx context.Context, arg sqlc.ListJournalsParams) ([]sqlc.ListJournalsRow, error) {
	if f.listJournalsFunc == nil {
		return nil, errors.New("unexpected ListJournals call")
	}

	return f.listJournalsFunc(ctx, arg)
}

func (f fakeQuerier) CreateJournal(ctx context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
	if f.createJournalFunc == nil {
		return sqlc.CreateJournalRow{}, errors.New("unexpected CreateJournal call")
	}

	return f.createJournalFunc(ctx, arg)
}

func (f fakeQuerier) GetJournalByID(ctx context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
	if f.getJournalByIDFunc == nil {
		return sqlc.GetJournalByIDRow{}, errors.New("unexpected GetJournalByID call")
	}

	return f.getJournalByIDFunc(ctx, id)
}

func (f fakeQuerier) GetCourseByID(ctx context.Context, id int64) (sqlc.Course, error) {
	if f.getCourseByIDFunc == nil {
		return sqlc.Course{}, errors.New("unexpected GetCourseByID call")
	}

	return f.getCourseByIDFunc(ctx, id)
}

func (f fakeQuerier) UpdateJournalHeader(ctx context.Context, arg sqlc.UpdateJournalHeaderParams) (sqlc.UpdateJournalHeaderRow, error) {
	if f.updateJournalHeaderFunc == nil {
		return sqlc.UpdateJournalHeaderRow{}, errors.New("unexpected UpdateJournalHeader call")
	}

	return f.updateJournalHeaderFunc(ctx, arg)
}

func (f fakeQuerier) DeleteJournal(ctx context.Context, id int64) (int64, error) {
	if f.deleteJournalFunc == nil {
		return 0, errors.New("unexpected DeleteJournal call")
	}

	return f.deleteJournalFunc(ctx, id)
}

func (f fakeQuerier) CloseJournal(ctx context.Context, id int64) (int64, error) {
	if f.closeJournalFunc == nil {
		return 0, errors.New("unexpected CloseJournal call")
	}

	return f.closeJournalFunc(ctx, id)
}

func (f fakeQuerier) ListJournalAttendees(ctx context.Context, journalID int64) ([]sqlc.ListJournalAttendeesRow, error) {
	if f.listJournalAttendeesFunc == nil {
		return nil, errors.New("unexpected ListJournalAttendees call")
	}

	return f.listJournalAttendeesFunc(ctx, journalID)
}

func (f fakeQuerier) AddJournalAttendee(ctx context.Context, arg sqlc.AddJournalAttendeeParams) (sqlc.AddJournalAttendeeRow, error) {
	if f.addJournalAttendeeFunc == nil {
		return sqlc.AddJournalAttendeeRow{}, errors.New("unexpected AddJournalAttendee call")
	}

	return f.addJournalAttendeeFunc(ctx, arg)
}

func (f fakeQuerier) UpdateJournalAttendeeCertificate(ctx context.Context, arg sqlc.UpdateJournalAttendeeCertificateParams) (sqlc.UpdateJournalAttendeeCertificateRow, error) {
	if f.updateJournalAttendeeCertificateFunc == nil {
		return sqlc.UpdateJournalAttendeeCertificateRow{}, errors.New("unexpected UpdateJournalAttendeeCertificate call")
	}

	return f.updateJournalAttendeeCertificateFunc(ctx, arg)
}

func (f fakeQuerier) DeleteJournalAttendee(ctx context.Context, arg sqlc.DeleteJournalAttendeeParams) (int64, error) {
	if f.deleteJournalAttendeeFunc == nil {
		return 0, errors.New("unexpected DeleteJournalAttendee call")
	}

	return f.deleteJournalAttendeeFunc(ctx, arg)
}

func (f fakeQuerier) ListJournalSessions(ctx context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error) {
	if f.listJournalSessionsFunc == nil {
		return nil, errors.New("unexpected ListJournalSessions call")
	}

	return f.listJournalSessionsFunc(ctx, journalID)
}

func (f fakeQuerier) GenerateJournalSessionsFromCourse(ctx context.Context, journalID int64) (int64, error) {
	if f.generateSessionsFunc == nil {
		return 0, errors.New("unexpected GenerateJournalSessionsFromCourse call")
	}

	return f.generateSessionsFunc(ctx, journalID)
}

func (f fakeQuerier) UpdateJournalSession(ctx context.Context, arg sqlc.UpdateJournalSessionParams) (sqlc.TrainingJournalSession, error) {
	if f.updateJournalSessionFunc == nil {
		return sqlc.TrainingJournalSession{}, errors.New("unexpected UpdateJournalSession call")
	}

	return f.updateJournalSessionFunc(ctx, arg)
}

func (f fakeQuerier) ListJournalAttendance(ctx context.Context, journalID int64) ([]sqlc.TrainingJournalAttendance, error) {
	if f.listJournalAttendanceFunc == nil {
		return nil, errors.New("unexpected ListJournalAttendance call")
	}

	return f.listJournalAttendanceFunc(ctx, journalID)
}

func (f fakeQuerier) UpsertJournalAttendance(ctx context.Context, arg sqlc.UpsertJournalAttendanceParams) (sqlc.TrainingJournalAttendance, error) {
	if f.upsertAttendanceFunc == nil {
		return sqlc.TrainingJournalAttendance{}, errors.New("unexpected UpsertJournalAttendance call")
	}

	return f.upsertAttendanceFunc(ctx, arg)
}

func (f fakeQuerier) UpsertJournalAttendanceScan(ctx context.Context, arg sqlc.UpsertJournalAttendanceScanParams) (sqlc.UpsertJournalAttendanceScanRow, error) {
	if f.upsertAttendanceScanFunc == nil {
		return sqlc.UpsertJournalAttendanceScanRow{}, errors.New("unexpected UpsertJournalAttendanceScan call")
	}

	return f.upsertAttendanceScanFunc(ctx, arg)
}

func (f fakeQuerier) GetJournalAttendanceScanFile(ctx context.Context, journalID int64) (sqlc.GetJournalAttendanceScanFileRow, error) {
	if f.getJournalAttendanceScanFileFunc == nil {
		return sqlc.GetJournalAttendanceScanFileRow{}, errors.New("unexpected GetJournalAttendanceScanFile call")
	}

	return f.getJournalAttendanceScanFileFunc(ctx, journalID)
}

func (f fakeQuerier) GetJournalAttendanceScanMeta(ctx context.Context, journalID int64) (sqlc.GetJournalAttendanceScanMetaRow, error) {
	if f.getJournalAttendanceScanMetaFunc == nil {
		return sqlc.GetJournalAttendanceScanMetaRow{}, errors.New("unexpected GetJournalAttendanceScanMeta call")
	}

	return f.getJournalAttendanceScanMetaFunc(ctx, journalID)
}

func (f fakeQuerier) DeleteJournalAttendanceScan(ctx context.Context, journalID int64) (int64, error) {
	if f.deleteJournalAttendanceScanFunc == nil {
		return 0, errors.New("unexpected DeleteJournalAttendanceScan call")
	}

	return f.deleteJournalAttendanceScanFunc(ctx, journalID)
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

func newMultipartFileRequest(t *testing.T, method, target, fieldName, fileName string, content []byte) (*http.Request, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("failed to create multipart part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, writer.FormDataContentType()
}

func TestListReturnsJournals(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalsFunc: func(_ context.Context, arg sqlc.ListJournalsParams) ([]sqlc.ListJournalsRow, error) {
			if !arg.Search.Valid || arg.Search.String != "bhp" {
				t.Fatalf("unexpected search: %+v", arg.Search)
			}
			if !arg.CourseID.Valid || arg.CourseID.Int64 != 7 {
				t.Fatalf("unexpected course id: %+v", arg.CourseID)
			}
			if !arg.CompanyID.Valid || arg.CompanyID.Int64 != 12 {
				t.Fatalf("unexpected company id: %+v", arg.CompanyID)
			}
			if !arg.Status.Valid || arg.Status.String != "draft" {
				t.Fatalf("unexpected status: %+v", arg.Status)
			}
			if !arg.DateFrom.Valid || arg.DateFrom.Time.Format(response.DateFormat) != "2026-03-01" {
				t.Fatalf("unexpected dateFrom: %+v", arg.DateFrom)
			}
			if !arg.DateTo.Valid || arg.DateTo.Time.Format(response.DateFormat) != "2026-03-31" {
				t.Fatalf("unexpected dateTo: %+v", arg.DateTo)
			}
			if arg.LimitCount != 25 {
				t.Fatalf("unexpected limit: %d", arg.LimitCount)
			}

			return []sqlc.ListJournalsRow{
				{
					ID:             11,
					Title:          "Szkolenie BHP marzec",
					CourseSymbol:   "BHP_ROB",
					OrganizerName:  "Nasza Era",
					Location:       "Zyrardow",
					FormOfTraining: "instruktaz",
					DateStart:      pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
					DateEnd:        pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
					TotalHours:     pgtype.Numeric{Int: big.NewInt(65), Exp: -1, Valid: true},
					Status:         "draft",
					CreatedAt:      pgtype.Timestamptz{Time: time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), Valid: true},
					CourseID:       7,
					CourseName:     "Szkolenie okresowe",
					CompanyID:      pgtype.Int8{Int64: 12, Valid: true},
					CompanyName:    pgtype.Text{String: "ACME", Valid: true},
					AttendeesCount: 18,
					SessionsCount:  3,
				},
				{
					ID:             12,
					Title:          "Szkolenie SEP",
					CourseSymbol:   "SEP",
					OrganizerName:  "Nasza Era",
					Location:       "Warszawa",
					FormOfTraining: "kurs",
					DateStart:      pgtype.Date{Time: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC), Valid: true},
					DateEnd:        pgtype.Date{Time: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC), Valid: true},
					TotalHours:     pgtype.Numeric{Int: big.NewInt(8), Exp: 0, Valid: true},
					Status:         "closed",
					CreatedAt:      pgtype.Timestamptz{Time: time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC), Valid: true},
					CourseID:       8,
					CourseName:     "SEP do 1kV",
					AttendeesCount: 9,
					SessionsCount:  1,
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals?search=bhp&courseId=7&companyId=12&status=draft&dateFrom=2026-03-01&dateTo=2026-03-31&limit=25", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody ListJournalsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 2 {
		t.Fatalf("expected 2 journals, got %d", len(responseBody.Data))
	}

	first := responseBody.Data[0]
	if first.ID != 11 || first.TotalHours != "6.5" {
		t.Fatalf("unexpected first journal: %+v", first)
	}
	if first.Company == nil || first.Company.ID != 12 || first.Company.Name != "ACME" {
		t.Fatalf("unexpected company mapping: %+v", first.Company)
	}

	second := responseBody.Data[1]
	if second.Company != nil {
		t.Fatalf("expected nil company, got %+v", second.Company)
	}
	if second.TotalHours != "8" {
		t.Fatalf("unexpected total hours: %q", second.TotalHours)
	}
}

func TestCreateReturnsCreatedJournal(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		createJournalFunc: func(_ context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
			if arg.CourseID != 7 {
				t.Fatalf("unexpected course id: %d", arg.CourseID)
			}
			if !arg.CompanyID.Valid || arg.CompanyID.Int64 != 12 {
				t.Fatalf("unexpected company id: %+v", arg.CompanyID)
			}
			if arg.Title != "Szkolenie BHP marzec" {
				t.Fatalf("unexpected title: %q", arg.Title)
			}
			if arg.OrganizerName != "Nasza Era" {
				t.Fatalf("unexpected organizer name: %q", arg.OrganizerName)
			}
			if !arg.OrganizerAddress.Valid || arg.OrganizerAddress.String != "ul. Testowa 1" {
				t.Fatalf("unexpected organizer address: %+v", arg.OrganizerAddress)
			}
			if arg.Location != "Zyrardow" {
				t.Fatalf("unexpected location: %q", arg.Location)
			}
			if arg.FormOfTraining != "instruktaz" {
				t.Fatalf("unexpected formOfTraining: %q", arg.FormOfTraining)
			}
			if arg.LegalBasis != "§ 16 ust. 3" {
				t.Fatalf("unexpected legal basis: %q", arg.LegalBasis)
			}
			if arg.DateStart.Time.Format(response.DateFormat) != "2026-03-10" {
				t.Fatalf("unexpected dateStart: %+v", arg.DateStart)
			}
			if arg.DateEnd.Time.Format(response.DateFormat) != "2026-03-11" {
				t.Fatalf("unexpected dateEnd: %+v", arg.DateEnd)
			}
			if !arg.Notes.Valid || arg.Notes.String != "Grupa produkcyjna" {
				t.Fatalf("unexpected notes: %+v", arg.Notes)
			}
			if arg.CreatedByUserID != 5 {
				t.Fatalf("unexpected createdByUserID: %d", arg.CreatedByUserID)
			}

			return sqlc.CreateJournalRow{
				ID:               21,
				CourseID:         arg.CourseID,
				CourseName:       "Szkolenie okresowe",
				CompanyID:        arg.CompanyID,
				CompanyName:      pgtype.Text{String: "ACME", Valid: true},
				Title:            arg.Title,
				CourseSymbol:     "BHP_ROB",
				OrganizerName:    arg.OrganizerName,
				OrganizerAddress: arg.OrganizerAddress,
				Location:         arg.Location,
				FormOfTraining:   arg.FormOfTraining,
				LegalBasis:       arg.LegalBasis,
				DateStart:        arg.DateStart,
				DateEnd:          arg.DateEnd,
				TotalHours:       pgtype.Numeric{Int: big.NewInt(65), Exp: -1, Valid: true},
				Notes:            arg.Notes,
				Status:           "draft",
				CreatedByUserID:  arg.CreatedByUserID,
				CreatedAt:        pgtype.Timestamptz{Time: time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), Valid: true},
				UpdatedAt:        pgtype.Timestamptz{Time: time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), Valid: true},
				AttendeesCount:   0,
				SessionsCount:    0,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals", strings.NewReader(`{
		"courseId": 7,
		"companyId": 12,
		"title": " Szkolenie BHP marzec ",
		"organizerName": " Nasza Era ",
		"organizerAddress": " ul. Testowa 1 ",
		"location": " Zyrardow ",
		"formOfTraining": " instruktaz ",
		"legalBasis": " § 16 ust. 3 ",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11",
		"notes": " Grupa produkcyjna "
	}`))
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 5, Role: 2}))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody JournalDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 21 || responseBody.Data.TotalHours != 6.5 {
		t.Fatalf("unexpected response body: %+v", responseBody.Data)
	}
	if responseBody.Data.CompanyID == nil || *responseBody.Data.CompanyID != 12 {
		t.Fatalf("unexpected company id: %+v", responseBody.Data.CompanyID)
	}
	if responseBody.Data.CompanyName == nil || *responseBody.Data.CompanyName != "ACME" {
		t.Fatalf("unexpected company name: %+v", responseBody.Data.CompanyName)
	}
}

func TestCreateReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		createJournalFunc: func(_ context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
			t.Fatalf("CreateJournal should not be called for invalid JSON, got %+v", arg)
			return sqlc.CreateJournalRow{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateReturnsBadRequestForMissingRequiredFields(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		createJournalFunc: func(_ context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
			t.Fatalf("CreateJournal should not be called for invalid body, got %+v", arg)
			return sqlc.CreateJournalRow{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals", strings.NewReader(`{
		"courseId": 0,
		"title": "",
		"organizerName": "",
		"location": "",
		"formOfTraining": "",
		"legalBasis": "",
		"dateStart": "",
		"dateEnd": "",
		"notes": ""
	}`))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateReturnsBadRequestForUnknownField(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		createJournalFunc: func(_ context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
			t.Fatalf("CreateJournal should not be called for unknown field, got %+v", arg)
			return sqlc.CreateJournalRow{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals", strings.NewReader(`{
		"courseId": 7,
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Zyrardow",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11",
		"extra": "oops"
	}`))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateReturnsBadRequestForInvalidDates(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		createJournalFunc: func(_ context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
			t.Fatalf("CreateJournal should not be called for invalid dates, got %+v", arg)
			return sqlc.CreateJournalRow{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals", strings.NewReader(`{
		"courseId": 7,
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Zyrardow",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-12",
		"dateEnd": "2026-03-11"
	}`))
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 5, Role: 2}))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateReturnsUnauthorizedWhenUserMissingInContext(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		createJournalFunc: func(_ context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
			t.Fatalf("CreateJournal should not be called without user in context, got %+v", arg)
			return sqlc.CreateJournalRow{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals", strings.NewReader(`{
		"courseId": 7,
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Zyrardow",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11"
	}`))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestCreateReturnsNotFoundWhenCourseDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		createJournalFunc: func(_ context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
			return sqlc.CreateJournalRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals", strings.NewReader(`{
		"courseId": 7,
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Zyrardow",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11"
	}`))
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 5, Role: 2}))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestCreateReturnsInternalServerErrorWhenCreateFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		createJournalFunc: func(_ context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
			return sqlc.CreateJournalRow{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals", strings.NewReader(`{
		"courseId": 7,
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Zyrardow",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11"
	}`))
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 5, Role: 2}))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestUpdateHeaderReturnsUpdatedJournal(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			if id != 21 {
				t.Fatalf("unexpected journal id: %d", id)
			}

			return sqlc.GetJournalByIDRow{
				ID:     21,
				Status: "draft",
			}, nil
		},
		listJournalSessionsFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error) {
			return []sqlc.TrainingJournalSession{
				{
					ID:          3,
					JournalID:   21,
					SessionDate: pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				},
			}, nil
		},
		updateJournalHeaderFunc: func(_ context.Context, arg sqlc.UpdateJournalHeaderParams) (sqlc.UpdateJournalHeaderRow, error) {
			if arg.JournalID != 21 {
				t.Fatalf("unexpected journal id: %d", arg.JournalID)
			}
			if !arg.CompanyID.Valid || arg.CompanyID.Int64 != 12 {
				t.Fatalf("unexpected company id: %+v", arg.CompanyID)
			}
			if arg.Title != "Szkolenie BHP marzec" {
				t.Fatalf("unexpected title: %q", arg.Title)
			}
			if arg.OrganizerName != "Nasza Era" {
				t.Fatalf("unexpected organizer name: %q", arg.OrganizerName)
			}
			if !arg.OrganizerAddress.Valid || arg.OrganizerAddress.String != "ul. Testowa 1" {
				t.Fatalf("unexpected organizer address: %+v", arg.OrganizerAddress)
			}
			if arg.Location != "Żyrardów" {
				t.Fatalf("unexpected location: %q", arg.Location)
			}
			if arg.FormOfTraining != "instruktaz" {
				t.Fatalf("unexpected form of training: %q", arg.FormOfTraining)
			}
			if arg.LegalBasis != "§ 16 ust. 3" {
				t.Fatalf("unexpected legal basis: %q", arg.LegalBasis)
			}
			if arg.DateStart.Time.Format(response.DateFormat) != "2026-03-10" {
				t.Fatalf("unexpected dateStart: %+v", arg.DateStart)
			}
			if arg.DateEnd.Time.Format(response.DateFormat) != "2026-03-11" {
				t.Fatalf("unexpected dateEnd: %+v", arg.DateEnd)
			}
			if !arg.Notes.Valid || arg.Notes.String != "Grupa produkcyjna" {
				t.Fatalf("unexpected notes: %+v", arg.Notes)
			}

			return sqlc.UpdateJournalHeaderRow{
				ID:               21,
				CourseID:         7,
				CourseName:       "Szkolenie okresowe",
				CompanyID:        pgtype.Int8{Int64: 12, Valid: true},
				CompanyName:      pgtype.Text{String: "ACME", Valid: true},
				Title:            arg.Title,
				CourseSymbol:     "BHP_ROB",
				OrganizerName:    arg.OrganizerName,
				OrganizerAddress: arg.OrganizerAddress,
				Location:         arg.Location,
				FormOfTraining:   arg.FormOfTraining,
				LegalBasis:       arg.LegalBasis,
				DateStart:        arg.DateStart,
				DateEnd:          arg.DateEnd,
				TotalHours:       pgtype.Numeric{Int: big.NewInt(65), Exp: -1, Valid: true},
				Notes:            arg.Notes,
				Status:           "draft",
				CreatedByUserID:  5,
				CreatedAt:        pgtype.Timestamptz{Time: time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), Valid: true},
				UpdatedAt:        pgtype.Timestamptz{Time: time.Date(2026, 3, 2, 10, 15, 0, 0, time.UTC), Valid: true},
				AttendeesCount:   18,
				SessionsCount:    3,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21", strings.NewReader(`{
		"companyId": 12,
		"title": " Szkolenie BHP marzec ",
		"organizerName": " Nasza Era ",
		"organizerAddress": "ul. Testowa 1",
		"location": " Żyrardów ",
		"formOfTraining": " instruktaz ",
		"legalBasis": " § 16 ust. 3 ",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11",
		"notes": "Grupa produkcyjna"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.UpdateHeader(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody JournalDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 21 || responseBody.Data.Title != "Szkolenie BHP marzec" {
		t.Fatalf("unexpected journal payload: %+v", responseBody.Data)
	}
	if responseBody.Data.CompanyID == nil || *responseBody.Data.CompanyID != 12 {
		t.Fatalf("unexpected company id mapping: %+v", responseBody.Data.CompanyID)
	}
}

func TestUpdateHeaderReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/foo", strings.NewReader(`{}`))
	req.SetPathValue("id", "foo")
	rec := httptest.NewRecorder()

	handler.UpdateHeader(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestUpdateHeaderReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21", strings.NewReader(`{`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.UpdateHeader(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestUpdateHeaderReturnsBadRequestForInvalidBody(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21", strings.NewReader(`{
		"companyId": 0,
		"title": "",
		"organizerName": "Nasza Era",
		"location": "Żyrardów",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.UpdateHeader(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestUpdateHeaderReturnsBadRequestForInvalidDates(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21", strings.NewReader(`{
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Żyrardów",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-11",
		"dateEnd": "2026-03-10"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.UpdateHeader(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestUpdateHeaderReturnsNotFoundWhenJournalDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21", strings.NewReader(`{
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Żyrardów",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.UpdateHeader(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestUpdateHeaderReturnsConflictWhenJournalIsClosed(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{
				ID:     21,
				Status: "closed",
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21", strings.NewReader(`{
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Żyrardów",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.UpdateHeader(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestUpdateHeaderReturnsConflictWhenSessionFallsOutsideRange(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{
				ID:     21,
				Status: "draft",
			}, nil
		},
		listJournalSessionsFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error) {
			return []sqlc.TrainingJournalSession{
				{
					ID:          3,
					JournalID:   21,
					SessionDate: pgtype.Date{Time: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), Valid: true},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21", strings.NewReader(`{
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Żyrardów",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.UpdateHeader(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestUpdateHeaderReturnsInternalServerErrorWhenUpdateFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{
				ID:     21,
				Status: "draft",
			}, nil
		},
		listJournalSessionsFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error) {
			return []sqlc.TrainingJournalSession{}, nil
		},
		updateJournalHeaderFunc: func(_ context.Context, arg sqlc.UpdateJournalHeaderParams) (sqlc.UpdateJournalHeaderRow, error) {
			return sqlc.UpdateJournalHeaderRow{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21", strings.NewReader(`{
		"title": "Szkolenie",
		"organizerName": "Nasza Era",
		"location": "Żyrardów",
		"formOfTraining": "instruktaz",
		"legalBasis": "§ 16 ust. 3",
		"dateStart": "2026-03-10",
		"dateEnd": "2026-03-11"
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.UpdateHeader(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPDFReturnsRenderedJournalPDF(t *testing.T) {
	originalRenderer := renderJournalPDF
	t.Cleanup(func() {
		renderJournalPDF = originalRenderer
	})

	renderJournalPDF = func(ctx context.Context, pageHTML string) ([]byte, error) {
		if !strings.Contains(pageHTML, "Szkolenie BHP marzec") {
			t.Fatalf("expected journal title in pdf html, got %q", pageHTML)
		}
		if !strings.Contains(pageHTML, "Jan Nowak") {
			t.Fatalf("expected attendee name in pdf html, got %q", pageHTML)
		}
		if !strings.Contains(pageHTML, "Godziny teorii") {
			t.Fatalf("expected program columns in pdf html, got %q", pageHTML)
		}
		if !strings.Contains(pageHTML, ">X<") {
			t.Fatalf("expected attendance mark in pdf html, got %q", pageHTML)
		}

		return []byte("%PDF-1.4 fake"), nil
	}

	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			if id != 21 {
				t.Fatalf("unexpected journal id: %d", id)
			}

			return sqlc.GetJournalByIDRow{
				ID:               21,
				CourseID:         7,
				CourseName:       "Szkolenie okresowe",
				CompanyName:      pgtype.Text{String: "ACME", Valid: true},
				Title:            "Szkolenie BHP marzec",
				CourseSymbol:     "BHP_ROB",
				OrganizerName:    "Nasza Era",
				OrganizerAddress: pgtype.Text{String: "ul. Testowa 1", Valid: true},
				Location:         "Żyrardów",
				FormOfTraining:   "instruktaz",
				LegalBasis:       "§ 16 ust. 3",
				DateStart:        pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:          pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
				TotalHours:       pgtype.Numeric{Int: big.NewInt(65), Exp: -1, Valid: true},
				Notes:            pgtype.Text{String: "Grupa produkcyjna", Valid: true},
				Status:           "closed",
			}, nil
		},
		getCourseByIDFunc: func(_ context.Context, id int64) (sqlc.Course, error) {
			if id != 7 {
				t.Fatalf("unexpected course id: %d", id)
			}

			return sqlc.Course{
				ID:            7,
				Name:          "Szkolenie okresowe",
				Symbol:        "BHP_ROB",
				Courseprogram: []byte(`[{"Subject":"Wprowadzenie","TheoryTime":"2","PracticeTime":"1"},{"Subject":"Ćwiczenia praktyczne z maszyną","TheoryTime":"1","PracticeTime":"2"}]`),
			}, nil
		},
		listJournalAttendeesFunc: func(_ context.Context, journalID int64) ([]sqlc.ListJournalAttendeesRow, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journal id: %d", journalID)
			}

			return []sqlc.ListJournalAttendeesRow{
				{
					ID:                  1,
					JournalID:           21,
					StudentID:           15,
					CertificateID:       pgtype.Int8{Int64: 91, Valid: true},
					FullNameSnapshot:    "Jan Nowak",
					BirthdateSnapshot:   pgtype.Date{Time: time.Date(1990, 1, 10, 0, 0, 0, 0, time.UTC), Valid: true},
					CompanyNameSnapshot: pgtype.Text{String: "ACME", Valid: true},
					CertificateRegistryYear: pgtype.Int8{
						Int64: 2026,
						Valid: true,
					},
					CertificateRegistryNumber: int64(18),
					CertificateCourseSymbol:   pgtype.Text{String: "BHP_ROB", Valid: true},
				},
			}, nil
		},
		listJournalSessionsFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journal id: %d", journalID)
			}

			return []sqlc.TrainingJournalSession{
				{
					ID:          5,
					JournalID:   21,
					SessionDate: pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
					Hours:       pgtype.Numeric{Int: big.NewInt(3), Exp: 0, Valid: true},
					Topic:       "Wprowadzenie",
					TrainerName: "Jan Kowalski",
					SortOrder:   1,
				},
				{
					ID:          6,
					JournalID:   21,
					SessionDate: pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
					Hours:       pgtype.Numeric{Int: big.NewInt(3), Exp: 0, Valid: true},
					Topic:       "Ćwiczenia praktyczne z maszyną",
					TrainerName: "Anna Nowak",
					SortOrder:   2,
				},
			}, nil
		},
		listJournalAttendanceFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalAttendance, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journal id: %d", journalID)
			}

			return []sqlc.TrainingJournalAttendance{
				{
					ID:                1,
					JournalSessionID:  5,
					JournalAttendeeID: 1,
					Present:           true,
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/pdf", nil)
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
		t.Fatalf("unexpected pdf body: %q", rec.Body.String())
	}
}

func TestPDFReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/foo/pdf", nil)
	req.SetPathValue("id", "foo")
	rec := httptest.NewRecorder()

	handler.PDF(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPDFReturnsNotFoundWhenJournalDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/pdf", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.PDF(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPDFReturnsInternalServerErrorWhenRendererFails(t *testing.T) {
	originalRenderer := renderJournalPDF
	t.Cleanup(func() {
		renderJournalPDF = originalRenderer
	})

	renderJournalPDF = func(ctx context.Context, pageHTML string) ([]byte, error) {
		return nil, errors.New("render failed")
	}

	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{
				ID:             id,
				CourseID:       7,
				CourseName:     "Szkolenie okresowe",
				Title:          "Szkolenie",
				CourseSymbol:   "BHP",
				OrganizerName:  "Nasza Era",
				Location:       "Żyrardów",
				FormOfTraining: "instruktaz",
				LegalBasis:     "§ 16 ust. 3",
				DateStart:      pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:        pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
				TotalHours:     pgtype.Numeric{Int: big.NewInt(8), Exp: 0, Valid: true},
			}, nil
		},
		getCourseByIDFunc: func(_ context.Context, id int64) (sqlc.Course, error) {
			return sqlc.Course{ID: id}, nil
		},
		listJournalAttendeesFunc: func(_ context.Context, journalID int64) ([]sqlc.ListJournalAttendeesRow, error) {
			return []sqlc.ListJournalAttendeesRow{}, nil
		},
		listJournalSessionsFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error) {
			return []sqlc.TrainingJournalSession{}, nil
		},
		listJournalAttendanceFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalAttendance, error) {
			return []sqlc.TrainingJournalAttendance{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/pdf", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.PDF(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestDeleteReturnsDeletedJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		deleteJournalFunc: func(_ context.Context, id int64) (int64, error) {
			if id != 21 {
				t.Fatalf("unexpected journal id: %d", id)
			}

			return 1, nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody DeleteJournalResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 21 {
		t.Fatalf("unexpected response body: %+v", responseBody)
	}
}

func TestDeleteReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		deleteJournalFunc: func(_ context.Context, id int64) (int64, error) {
			t.Fatalf("DeleteJournal should not be called for invalid journal id, got %d", id)
			return 0, nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestDeleteReturnsNotFoundWhenJournalMissing(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		deleteJournalFunc: func(_ context.Context, id int64) (int64, error) {
			return 0, nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestDeleteReturnsInternalServerErrorWhenDeleteFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		deleteJournalFunc: func(_ context.Context, id int64) (int64, error) {
			return 0, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestCloseReturnsClosedJournal(t *testing.T) {
	getCallCount := 0

	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			getCallCount++

			if getCallCount == 1 {
				return sqlc.GetJournalByIDRow{
					ID:              id,
					CourseID:        7,
					CourseName:      "Szkolenie okresowe",
					Title:           "Szkolenie BHP marzec",
					CourseSymbol:    "BHP_ROB",
					OrganizerName:   "Nasza Era",
					Location:        "Zyrardow",
					FormOfTraining:  "instruktaz",
					LegalBasis:      "§ 16 ust. 3",
					DateStart:       pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
					DateEnd:         pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
					TotalHours:      pgtype.Numeric{Int: big.NewInt(65), Exp: -1, Valid: true},
					Status:          "draft",
					CreatedByUserID: 5,
					CreatedAt:       pgtype.Timestamptz{Time: time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), Valid: true},
					UpdatedAt:       pgtype.Timestamptz{Time: time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC), Valid: true},
					AttendeesCount:  3,
					SessionsCount:   4,
				}, nil
			}

			return sqlc.GetJournalByIDRow{
				ID:              id,
				CourseID:        7,
				CourseName:      "Szkolenie okresowe",
				Title:           "Szkolenie BHP marzec",
				CourseSymbol:    "BHP_ROB",
				OrganizerName:   "Nasza Era",
				Location:        "Zyrardow",
				FormOfTraining:  "instruktaz",
				LegalBasis:      "§ 16 ust. 3",
				DateStart:       pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:         pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
				TotalHours:      pgtype.Numeric{Int: big.NewInt(65), Exp: -1, Valid: true},
				Status:          "closed",
				CreatedByUserID: 5,
				CreatedAt:       pgtype.Timestamptz{Time: time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), Valid: true},
				UpdatedAt:       pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC), Valid: true},
				ClosedAt:        pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC), Valid: true},
				AttendeesCount:  3,
				SessionsCount:   4,
			}, nil
		},
		closeJournalFunc: func(_ context.Context, id int64) (int64, error) {
			if id != 21 {
				t.Fatalf("unexpected journal id: %d", id)
			}
			return 1, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/close", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Close(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody JournalDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.Status != "closed" || responseBody.Data.ClosedAt == nil {
		t.Fatalf("unexpected response body: %+v", responseBody.Data)
	}
}

func TestCloseReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/abc/close", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.Close(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCloseReturnsNotFoundWhenJournalMissing(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/close", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Close(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestCloseReturnsConflictWhenJournalAlreadyClosed(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "closed"}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/close", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Close(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestCloseReturnsBadRequestWhenJournalHasNoAttendeesOrSessions(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "draft", AttendeesCount: 0, SessionsCount: 2}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/close", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Close(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCloseReturnsInternalServerErrorWhenCloseFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "draft", AttendeesCount: 1, SessionsCount: 1}, nil
		},
		closeJournalFunc: func(_ context.Context, id int64) (int64, error) {
			return 0, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/close", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.Close(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListAttendeesReturnsAttendees(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalAttendeesFunc: func(_ context.Context, journalID int64) ([]sqlc.ListJournalAttendeesRow, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journalID: %d", journalID)
			}

			return []sqlc.ListJournalAttendeesRow{
				{
					ID:                  1,
					JournalID:           21,
					StudentID:           7,
					FullNameSnapshot:    "Nowak Jan Adam",
					BirthdateSnapshot:   pgtype.Date{Time: time.Date(1990, 1, 10, 0, 0, 0, 0, time.UTC), Valid: true},
					CompanyNameSnapshot: pgtype.Text{String: "ACME", Valid: true},
					SortOrder:           1,
					CreatedAt:           pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC), Valid: true},
				},
				{
					ID:                2,
					JournalID:         21,
					StudentID:         8,
					FullNameSnapshot:  "Kowalska Anna",
					BirthdateSnapshot: pgtype.Date{Time: time.Date(1992, 5, 3, 0, 0, 0, 0, time.UTC), Valid: true},
					SortOrder:         2,
					CreatedAt:         pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 10, 1, 0, 0, time.UTC), Valid: true},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendees", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListAttendees(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody ListJournalAttendeeResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 2 {
		t.Fatalf("expected 2 attendees, got %d", len(responseBody.Data))
	}
	if responseBody.Data[0].CompanyNameSnapshot == nil || *responseBody.Data[0].CompanyNameSnapshot != "ACME" {
		t.Fatalf("unexpected first attendee: %+v", responseBody.Data[0])
	}
	if responseBody.Data[1].CompanyNameSnapshot != nil {
		t.Fatalf("expected nil company snapshot, got %+v", responseBody.Data[1].CompanyNameSnapshot)
	}
}

func TestListAttendeesReturnsEmptyListWhenJournalExistsWithoutAttendees(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalAttendeesFunc: func(_ context.Context, journalID int64) ([]sqlc.ListJournalAttendeesRow, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journalID: %d", journalID)
			}
			return []sqlc.ListJournalAttendeesRow{}, nil
		},
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			if id != 21 {
				t.Fatalf("unexpected journalID in GetJournalByID: %d", id)
			}
			return sqlc.GetJournalByIDRow{
				ID:              21,
				CourseID:        7,
				CourseName:      "Szkolenie okresowe",
				Title:           "Szkolenie BHP marzec",
				CourseSymbol:    "BHP_ROB",
				OrganizerName:   "Nasza Era",
				Location:        "Zyrardow",
				FormOfTraining:  "instruktaz",
				LegalBasis:      "§ 16 ust. 3",
				DateStart:       pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:         pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
				TotalHours:      pgtype.Numeric{Int: big.NewInt(65), Exp: -1, Valid: true},
				Status:          "draft",
				CreatedByUserID: 5,
				CreatedAt:       pgtype.Timestamptz{Time: time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendees", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListAttendees(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody ListJournalAttendeeResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 0 {
		t.Fatalf("expected empty attendees list, got %d", len(responseBody.Data))
	}
}

func TestListAttendeesReturnsNotFoundWhenJournalDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalAttendeesFunc: func(_ context.Context, _ int64) ([]sqlc.ListJournalAttendeesRow, error) {
			return []sqlc.ListJournalAttendeesRow{}, nil
		},
		getJournalByIDFunc: func(_ context.Context, _ int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendees", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListAttendees(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestListAttendeesReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/foo/attendees", nil)
	req.SetPathValue("id", "foo")
	rec := httptest.NewRecorder()

	handler.ListAttendees(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListAttendeesReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalAttendeesFunc: func(_ context.Context, _ int64) ([]sqlc.ListJournalAttendeesRow, error) {
			return nil, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendees", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListAttendees(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestAddJournalAttendeeReturnsCreatedAttendee(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		addJournalAttendeeFunc: func(_ context.Context, arg sqlc.AddJournalAttendeeParams) (sqlc.AddJournalAttendeeRow, error) {
			if arg.JournalID != 21 {
				t.Fatalf("unexpected journalID: %d", arg.JournalID)
			}
			if arg.StudentID != 7 {
				t.Fatalf("unexpected studentID: %d", arg.StudentID)
			}

			return sqlc.AddJournalAttendeeRow{
				ID:                  1,
				JournalID:           arg.JournalID,
				StudentID:           arg.StudentID,
				FullNameSnapshot:    "Nowak Jan Adam",
				BirthdateSnapshot:   pgtype.Date{Time: time.Date(1990, 1, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				CompanyNameSnapshot: pgtype.Text{String: "ACME", Valid: true},
				SortOrder:           1,
				CreatedAt:           pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees", strings.NewReader(`{"studentId":7}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.AddJournalAttendee(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var responseBody AddJournalAttendeeResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 1 || responseBody.Data.StudentID != 7 {
		t.Fatalf("unexpected response body: %+v", responseBody.Data)
	}
}

func TestAddJournalAttendeeReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/foo/attendees", strings.NewReader(`{"studentId":7}`))
	req.SetPathValue("id", "foo")
	rec := httptest.NewRecorder()

	handler.AddJournalAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestAddJournalAttendeeReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees", strings.NewReader(`{`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.AddJournalAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestAddJournalAttendeeReturnsBadRequestForInvalidBody(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees", strings.NewReader(`{"studentId":0}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.AddJournalAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestAddJournalAttendeeReturnsNotFound(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		addJournalAttendeeFunc: func(_ context.Context, _ sqlc.AddJournalAttendeeParams) (sqlc.AddJournalAttendeeRow, error) {
			return sqlc.AddJournalAttendeeRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees", strings.NewReader(`{"studentId":7}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.AddJournalAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestAddJournalAttendeeReturnsConflictForDuplicateStudent(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		addJournalAttendeeFunc: func(_ context.Context, _ sqlc.AddJournalAttendeeParams) (sqlc.AddJournalAttendeeRow, error) {
			return sqlc.AddJournalAttendeeRow{}, &pgconn.PgError{Code: "23505"}
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees", strings.NewReader(`{"studentId":7}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.AddJournalAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestAddJournalAttendeeReturnsInternalServerErrorWhenInsertFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		addJournalAttendeeFunc: func(_ context.Context, _ sqlc.AddJournalAttendeeParams) (sqlc.AddJournalAttendeeRow, error) {
			return sqlc.AddJournalAttendeeRow{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees", strings.NewReader(`{"studentId":7}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.AddJournalAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPatchAttendeeCertificateReturnsUpdatedAttendee(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		updateJournalAttendeeCertificateFunc: func(_ context.Context, arg sqlc.UpdateJournalAttendeeCertificateParams) (sqlc.UpdateJournalAttendeeCertificateRow, error) {
			if arg.JournalID != 21 || arg.AttendeeID != 7 {
				t.Fatalf("unexpected ids: %+v", arg)
			}
			if !arg.CertificateID.Valid || arg.CertificateID.Int64 != 99 {
				t.Fatalf("unexpected certificate id: %+v", arg.CertificateID)
			}

			return sqlc.UpdateJournalAttendeeCertificateRow{
				ID:                        7,
				JournalID:                 21,
				StudentID:                 12,
				CertificateID:             pgtype.Int8{Int64: 99, Valid: true},
				FullNameSnapshot:          "Nowak Jan",
				BirthdateSnapshot:         pgtype.Date{Time: time.Date(1990, 1, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				CompanyNameSnapshot:       pgtype.Text{String: "ACME", Valid: true},
				SortOrder:                 1,
				CreatedAt:                 pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC), Valid: true},
				CertificateDate:           pgtype.Date{Time: time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC), Valid: true},
				CertificateRegistryYear:   pgtype.Int8{Int64: 2026, Valid: true},
				CertificateRegistryNumber: 44,
				CertificateCourseSymbol:   pgtype.Text{String: "BHP_ROB", Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendees/7/certificate", strings.NewReader(`{"certificateId":99}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.PatchAttendeeCertificate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody JournalAttendeeResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.Certificate == nil || responseBody.Data.Certificate.ID != 99 {
		t.Fatalf("unexpected certificate mapping: %+v", responseBody.Data)
	}
}

func TestPatchAttendeeCertificateAllowsDetach(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		updateJournalAttendeeCertificateFunc: func(_ context.Context, arg sqlc.UpdateJournalAttendeeCertificateParams) (sqlc.UpdateJournalAttendeeCertificateRow, error) {
			if arg.CertificateID.Valid {
				t.Fatalf("expected null certificate id, got %+v", arg.CertificateID)
			}
			return sqlc.UpdateJournalAttendeeCertificateRow{
				ID:                7,
				JournalID:         21,
				StudentID:         12,
				FullNameSnapshot:  "Nowak Jan",
				BirthdateSnapshot: pgtype.Date{Time: time.Date(1990, 1, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				SortOrder:         1,
				CreatedAt:         pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendees/7/certificate", strings.NewReader(`{"certificateId":null}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.PatchAttendeeCertificate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody JournalAttendeeResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.Certificate != nil {
		t.Fatalf("expected detached certificate, got %+v", responseBody.Data.Certificate)
	}
}

func TestPatchAttendeeCertificateReturnsBadRequestForInvalidIDs(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/foo/attendees/0/certificate", strings.NewReader(`{"certificateId":1}`))
	req.SetPathValue("id", "foo")
	req.SetPathValue("attendeeId", "0")
	rec := httptest.NewRecorder()

	handler.PatchAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchAttendeeCertificateReturnsBadRequestForInvalidBody(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendees/7/certificate", strings.NewReader(`{"certificateId":0}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.PatchAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchAttendeeCertificateReturnsNotFound(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		updateJournalAttendeeCertificateFunc: func(_ context.Context, _ sqlc.UpdateJournalAttendeeCertificateParams) (sqlc.UpdateJournalAttendeeCertificateRow, error) {
			return sqlc.UpdateJournalAttendeeCertificateRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendees/7/certificate", strings.NewReader(`{"certificateId":99}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.PatchAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchAttendeeCertificateReturnsConflictWhenCertificateAlreadyLinked(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		updateJournalAttendeeCertificateFunc: func(_ context.Context, _ sqlc.UpdateJournalAttendeeCertificateParams) (sqlc.UpdateJournalAttendeeCertificateRow, error) {
			return sqlc.UpdateJournalAttendeeCertificateRow{}, &pgconn.PgError{Code: "23505"}
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendees/7/certificate", strings.NewReader(`{"certificateId":99}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.PatchAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestPatchAttendeeCertificateReturnsInternalServerError(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		updateJournalAttendeeCertificateFunc: func(_ context.Context, _ sqlc.UpdateJournalAttendeeCertificateParams) (sqlc.UpdateJournalAttendeeCertificateRow, error) {
			return sqlc.UpdateJournalAttendeeCertificateRow{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendees/7/certificate", strings.NewReader(`{"certificateId":99}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.PatchAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestGenerateAttendeeCertificateReturnsCreatedCertificateID(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCertificateGenerator{
		generateFunc: func(_ context.Context, journalID, attendeeID int64) (GenerateAttendeeCertificateResult, error) {
			if journalID != 21 || attendeeID != 7 {
				t.Fatalf("unexpected ids: journal=%d attendee=%d", journalID, attendeeID)
			}

			return GenerateAttendeeCertificateResult{CertificateID: 101}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees/7/certificate/generate", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.GenerateAttendeeCertificate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var responseBody GenerateJournalAttendeeCertificateResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 101 {
		t.Fatalf("unexpected response body: %+v", responseBody.Data)
	}
}

func TestGenerateAttendeeCertificateReturnsBadRequestForInvalidIDs(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCertificateGenerator{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/foo/attendees/0/certificate/generate", nil)
	req.SetPathValue("id", "foo")
	req.SetPathValue("attendeeId", "0")
	rec := httptest.NewRecorder()

	handler.GenerateAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGenerateAttendeeCertificateReturnsNotFound(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCertificateGenerator{
		generateFunc: func(_ context.Context, _, _ int64) (GenerateAttendeeCertificateResult, error) {
			return GenerateAttendeeCertificateResult{}, ErrJournalAttendeeNotFound
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees/7/certificate/generate", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.GenerateAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestGenerateAttendeeCertificateReturnsConflictWhenAlreadyLinked(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCertificateGenerator{
		generateFunc: func(_ context.Context, _, _ int64) (GenerateAttendeeCertificateResult, error) {
			return GenerateAttendeeCertificateResult{}, ErrJournalAttendeeCertificateLinked
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees/7/certificate/generate", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.GenerateAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestGenerateAttendeeCertificateReturnsBadRequestForGenerationFailure(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCertificateGenerator{
		generateFunc: func(_ context.Context, _, _ int64) (GenerateAttendeeCertificateResult, error) {
			return GenerateAttendeeCertificateResult{}, ErrJournalCertificateGeneration
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees/7/certificate/generate", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.GenerateAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGenerateAttendeeCertificateReturnsInternalServerError(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCertificateGenerator{
		generateFunc: func(_ context.Context, _, _ int64) (GenerateAttendeeCertificateResult, error) {
			return GenerateAttendeeCertificateResult{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendees/7/certificate/generate", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.GenerateAttendeeCertificate(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestDeleteAttendeeReturnsDeletedAttendeeID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "draft"}, nil
		},
		deleteJournalAttendeeFunc: func(_ context.Context, arg sqlc.DeleteJournalAttendeeParams) (int64, error) {
			if arg.JournalID != 21 || arg.ID != 7 {
				t.Fatalf("unexpected ids: journal=%d attendee=%d", arg.JournalID, arg.ID)
			}
			return 1, nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21/attendees/7", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.DeleteAttendee(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody DeleteJournalAttendeeResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 7 {
		t.Fatalf("unexpected response body: %+v", responseBody.Data)
	}
}

func TestDeleteAttendeeReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/foo/attendees/7", nil)
	req.SetPathValue("id", "foo")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.DeleteAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestDeleteAttendeeReturnsBadRequestForInvalidAttendeeID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21/attendees/foo", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "foo")
	rec := httptest.NewRecorder()

	handler.DeleteAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestDeleteAttendeeReturnsNotFoundWhenJournalMissing(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, _ int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21/attendees/7", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.DeleteAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestDeleteAttendeeReturnsConflictWhenJournalClosed(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "closed"}, nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21/attendees/7", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.DeleteAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestDeleteAttendeeReturnsNotFoundWhenAttendeeMissing(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "draft"}, nil
		},
		deleteJournalAttendeeFunc: func(_ context.Context, _ sqlc.DeleteJournalAttendeeParams) (int64, error) {
			return 0, nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21/attendees/7", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.DeleteAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestDeleteAttendeeReturnsInternalServerErrorWhenDeleteFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "draft"}, nil
		},
		deleteJournalAttendeeFunc: func(_ context.Context, _ sqlc.DeleteJournalAttendeeParams) (int64, error) {
			return 0, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21/attendees/7", nil)
	req.SetPathValue("id", "21")
	req.SetPathValue("attendeeId", "7")
	rec := httptest.NewRecorder()

	handler.DeleteAttendee(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListAttendanceReturnsAttendanceEntries(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalAttendanceFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalAttendance, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journalID: %d", journalID)
			}
			return []sqlc.TrainingJournalAttendance{
				{
					ID:                1,
					JournalSessionID:  3,
					JournalAttendeeID: 7,
					Present:           true,
					CreatedAt:         pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC), Valid: true},
					UpdatedAt:         pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 12, 5, 0, 0, time.UTC), Valid: true},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendance", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListAttendance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody ListJournalAttendanceResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 1 || !responseBody.Data[0].Present {
		t.Fatalf("unexpected response body: %+v", responseBody.Data)
	}
}

func TestListAttendanceReturnsEmptyListWhenJournalExistsWithoutAttendance(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalAttendanceFunc: func(_ context.Context, _ int64) ([]sqlc.TrainingJournalAttendance, error) {
			return []sqlc.TrainingJournalAttendance{}, nil
		},
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendance", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListAttendance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody ListJournalAttendanceResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 0 {
		t.Fatalf("expected empty response, got %+v", responseBody.Data)
	}
}

func TestListAttendanceReturnsNotFoundWhenJournalMissing(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalAttendanceFunc: func(_ context.Context, _ int64) ([]sqlc.TrainingJournalAttendance, error) {
			return []sqlc.TrainingJournalAttendance{}, nil
		},
		getJournalByIDFunc: func(_ context.Context, _ int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendance", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListAttendance(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchAttendanceReturnsUpdatedAttendance(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "draft"}, nil
		},
		upsertAttendanceFunc: func(_ context.Context, arg sqlc.UpsertJournalAttendanceParams) (sqlc.TrainingJournalAttendance, error) {
			if arg.JournalID != 21 || arg.JournalSessionID != 3 || arg.JournalAttendeeID != 7 || !arg.Present {
				t.Fatalf("unexpected params: %+v", arg)
			}
			return sqlc.TrainingJournalAttendance{
				ID:                1,
				JournalSessionID:  arg.JournalSessionID,
				JournalAttendeeID: arg.JournalAttendeeID,
				Present:           arg.Present,
				CreatedAt:         pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC), Valid: true},
				UpdatedAt:         pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 12, 10, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendance", strings.NewReader(`{
		"journalSessionId": 3,
		"journalAttendeeId": 7,
		"present": true
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.PatchAttendance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody JournalAttendanceResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.JournalSessionID != 3 || responseBody.Data.JournalAttendeeID != 7 || !responseBody.Data.Present {
		t.Fatalf("unexpected response: %+v", responseBody.Data)
	}
}

func TestPatchAttendanceReturnsBadRequestForInvalidBody(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendance", strings.NewReader(`{
		"journalSessionId": 0,
		"journalAttendeeId": 0,
		"present": true
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.PatchAttendance(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchAttendanceReturnsConflictWhenJournalClosed(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "closed"}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendance", strings.NewReader(`{
		"journalSessionId": 3,
		"journalAttendeeId": 7,
		"present": true
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.PatchAttendance(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestPatchAttendanceReturnsNotFoundWhenTargetMissing(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id, Status: "draft"}, nil
		},
		upsertAttendanceFunc: func(_ context.Context, _ sqlc.UpsertJournalAttendanceParams) (sqlc.TrainingJournalAttendance, error) {
			return sqlc.TrainingJournalAttendance{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/attendance", strings.NewReader(`{
		"journalSessionId": 3,
		"journalAttendeeId": 7,
		"present": true
	}`))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.PatchAttendance(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestListSessionsReturnsSessions(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalSessionsFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journalID: %d", journalID)
			}

			return []sqlc.TrainingJournalSession{
				{
					ID:          1,
					JournalID:   21,
					SessionDate: pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
					StartTime:   pgtype.Time{Microseconds: 8 * 60 * 60 * 1_000_000, Valid: true},
					EndTime:     pgtype.Time{Microseconds: (11*60 + 30) * 60 * 1_000_000, Valid: true},
					Hours:       pgtype.Numeric{Int: big.NewInt(35), Exp: -1, Valid: true},
					Topic:       "Przepisy ogólne BHP",
					TrainerName: "Jan Prowadzacy",
					SortOrder:   1,
					CreatedAt:   pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC), Valid: true},
				},
				{
					ID:          2,
					JournalID:   21,
					SessionDate: pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
					Hours:       pgtype.Numeric{Int: big.NewInt(25), Exp: -1, Valid: true},
					Topic:       "Pierwsza pomoc",
					TrainerName: "Jan Prowadzacy",
					SortOrder:   2,
					CreatedAt:   pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 11, 5, 0, 0, time.UTC), Valid: true},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/sessions", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody ListJournalSessionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(responseBody.Data))
	}
	if responseBody.Data[0].Hours != "3.5" || responseBody.Data[0].StartTime == nil || *responseBody.Data[0].StartTime != "08:00:00" {
		t.Fatalf("unexpected first session: %+v", responseBody.Data[0])
	}
	if responseBody.Data[1].StartTime != nil || responseBody.Data[1].EndTime != nil {
		t.Fatalf("expected nil times in second session, got %+v", responseBody.Data[1])
	}
}

func TestListSessionsReturnsEmptyListWhenJournalExistsWithoutSessions(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalSessionsFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journalID: %d", journalID)
			}
			return []sqlc.TrainingJournalSession{}, nil
		},
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			if id != 21 {
				t.Fatalf("unexpected journalID in GetJournalByID: %d", id)
			}
			return sqlc.GetJournalByIDRow{
				ID:              21,
				CourseID:        7,
				CourseName:      "Szkolenie okresowe",
				Title:           "Szkolenie BHP marzec",
				CourseSymbol:    "BHP_ROB",
				OrganizerName:   "Nasza Era",
				Location:        "Zyrardow",
				FormOfTraining:  "instruktaz",
				LegalBasis:      "§ 16 ust. 3",
				DateStart:       pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:         pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
				TotalHours:      pgtype.Numeric{Int: big.NewInt(65), Exp: -1, Valid: true},
				Status:          "draft",
				CreatedByUserID: 5,
				CreatedAt:       pgtype.Timestamptz{Time: time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/sessions", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody ListJournalSessionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 0 {
		t.Fatalf("expected empty session list, got %d", len(responseBody.Data))
	}
}

func TestListSessionsReturnsNotFoundWhenJournalDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalSessionsFunc: func(_ context.Context, _ int64) ([]sqlc.TrainingJournalSession, error) {
			return []sqlc.TrainingJournalSession{}, nil
		},
		getJournalByIDFunc: func(_ context.Context, _ int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/sessions", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.ListSessions(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestGenerateSessionsFromCourseReturnsGeneratedCount(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
		listJournalSessionsFunc: func(_ context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journalID: %d", journalID)
			}
			return []sqlc.TrainingJournalSession{}, nil
		},
		generateSessionsFunc: func(_ context.Context, journalID int64) (int64, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journalID: %d", journalID)
			}
			return 4, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/sessions/generate-from-course", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GenerateSessionsFromCourse(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody GenerateJournalSessionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.GeneratedCount != 4 {
		t.Fatalf("expected generated count 4, got %d", responseBody.Data.GeneratedCount)
	}
}

func TestGenerateSessionsFromCourseReturnsConflictWhenSessionsAlreadyExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
		listJournalSessionsFunc: func(_ context.Context, _ int64) ([]sqlc.TrainingJournalSession, error) {
			return []sqlc.TrainingJournalSession{{ID: 1, JournalID: 21}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/sessions/generate-from-course", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GenerateSessionsFromCourse(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestGenerateSessionsFromCourseReturnsBadRequestWhenProgramEmpty(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
		listJournalSessionsFunc: func(_ context.Context, _ int64) ([]sqlc.TrainingJournalSession, error) {
			return []sqlc.TrainingJournalSession{}, nil
		},
		generateSessionsFunc: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/sessions/generate-from-course", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GenerateSessionsFromCourse(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGenerateSessionsFromCourseReturnsNotFoundWhenJournalDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, _ int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/sessions/generate-from-course", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GenerateSessionsFromCourse(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestGenerateSessionsFromCourseReturnsInternalServerErrorWhenGenerationFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
		listJournalSessionsFunc: func(_ context.Context, _ int64) ([]sqlc.TrainingJournalSession, error) {
			return []sqlc.TrainingJournalSession{}, nil
		},
		generateSessionsFunc: func(_ context.Context, _ int64) (int64, error) {
			return 0, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/sessions/generate-from-course", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GenerateSessionsFromCourse(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPatchSessionReturnsUpdatedSession(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{
				ID:        id,
				Status:    "draft",
				DateStart: pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:   pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
			}, nil
		},
		updateJournalSessionFunc: func(_ context.Context, arg sqlc.UpdateJournalSessionParams) (sqlc.TrainingJournalSession, error) {
			if arg.JournalID != 21 {
				t.Fatalf("unexpected journalID: %d", arg.JournalID)
			}
			if arg.SessionID != 3 {
				t.Fatalf("unexpected sessionID: %d", arg.SessionID)
			}
			if arg.SessionDate.Time.Format(response.DateFormat) != "2026-03-11" {
				t.Fatalf("unexpected session date: %+v", arg.SessionDate)
			}
			if arg.TrainerName != "Anna Prowadzaca" {
				t.Fatalf("unexpected trainer name: %q", arg.TrainerName)
			}

			return sqlc.TrainingJournalSession{
				ID:          3,
				JournalID:   21,
				SessionDate: arg.SessionDate,
				Hours:       pgtype.Numeric{Int: big.NewInt(25), Exp: -1, Valid: true},
				Topic:       "Pierwsza pomoc",
				TrainerName: arg.TrainerName,
				SortOrder:   2,
				CreatedAt:   pgtype.Timestamptz{Time: time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/sessions/3", strings.NewReader(`{
		"sessionDate": "2026-03-11",
		"trainerName": " Anna Prowadzaca "
	}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("sessionId", "3")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody JournalSessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 3 || responseBody.Data.SessionDate != "2026-03-11" || responseBody.Data.TrainerName != "Anna Prowadzaca" {
		t.Fatalf("unexpected response body: %+v", responseBody.Data)
	}
}

func TestPatchSessionReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/foo/sessions/3", strings.NewReader(`{"sessionDate":"2026-03-10","trainerName":"Jan"}`))
	req.SetPathValue("id", "foo")
	req.SetPathValue("sessionId", "3")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchSessionReturnsBadRequestForInvalidSessionID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/sessions/foo", strings.NewReader(`{"sessionDate":"2026-03-10","trainerName":"Jan"}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("sessionId", "foo")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchSessionReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/sessions/3", strings.NewReader(`{`))
	req.SetPathValue("id", "21")
	req.SetPathValue("sessionId", "3")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchSessionReturnsBadRequestForInvalidBody(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/sessions/3", strings.NewReader(`{
		"sessionDate": "",
		"trainerName": ""
	}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("sessionId", "3")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchSessionReturnsBadRequestForInvalidDate(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{
				ID:        id,
				Status:    "draft",
				DateStart: pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:   pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/sessions/3", strings.NewReader(`{
		"sessionDate": "2026-03-15",
		"trainerName": "Jan"
	}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("sessionId", "3")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchSessionReturnsConflictWhenJournalClosed(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{
				ID:        id,
				Status:    "closed",
				DateStart: pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:   pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/sessions/3", strings.NewReader(`{
		"sessionDate": "2026-03-10",
		"trainerName": "Jan"
	}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("sessionId", "3")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestPatchSessionReturnsNotFoundWhenJournalMissing(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, _ int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/sessions/3", strings.NewReader(`{
		"sessionDate": "2026-03-10",
		"trainerName": "Jan"
	}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("sessionId", "3")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchSessionReturnsNotFoundWhenSessionMissing(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{
				ID:        id,
				Status:    "draft",
				DateStart: pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:   pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
			}, nil
		},
		updateJournalSessionFunc: func(_ context.Context, _ sqlc.UpdateJournalSessionParams) (sqlc.TrainingJournalSession, error) {
			return sqlc.TrainingJournalSession{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/sessions/3", strings.NewReader(`{
		"sessionDate": "2026-03-10",
		"trainerName": "Jan"
	}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("sessionId", "3")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchSessionReturnsInternalServerErrorWhenUpdateFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{
				ID:        id,
				Status:    "draft",
				DateStart: pgtype.Date{Time: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Valid: true},
				DateEnd:   pgtype.Date{Time: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC), Valid: true},
			}, nil
		},
		updateJournalSessionFunc: func(_ context.Context, _ sqlc.UpdateJournalSessionParams) (sqlc.TrainingJournalSession, error) {
			return sqlc.TrainingJournalSession{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/journals/21/sessions/3", strings.NewReader(`{
		"sessionDate": "2026-03-10",
		"trainerName": "Jan"
	}`))
	req.SetPathValue("id", "21")
	req.SetPathValue("sessionId", "3")
	rec := httptest.NewRecorder()

	handler.PatchSession(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListUsesDefaultLimit(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalsFunc: func(_ context.Context, arg sqlc.ListJournalsParams) ([]sqlc.ListJournalsRow, error) {
			if arg.LimitCount != 50 {
				t.Fatalf("expected default limit 50, got %d", arg.LimitCount)
			}
			if arg.Search.Valid || arg.CourseID.Valid || arg.CompanyID.Valid || arg.Status.Valid || arg.DateFrom.Valid || arg.DateTo.Valid {
				t.Fatalf("expected empty filters, got %+v", arg)
			}
			return []sqlc.ListJournalsRow{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestListReturnsBadRequestForInvalidLimit(t *testing.T) {
	handler := NewHandler(fakeQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals?limit=abc", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListReturnsBadRequestForInvalidCourseID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals?courseId=0", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListReturnsBadRequestForInvalidCompanyID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals?companyId=foo", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListReturnsBadRequestForInvalidStatus(t *testing.T) {
	handler := NewHandler(fakeQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals?status=archived", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListReturnsBadRequestForInvalidDateFrom(t *testing.T) {
	handler := NewHandler(fakeQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals?dateFrom=2026-99-99", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListReturnsBadRequestForInvalidDateTo(t *testing.T) {
	handler := NewHandler(fakeQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals?dateTo=not-a-date", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listJournalsFunc: func(_ context.Context, arg sqlc.ListJournalsParams) ([]sqlc.ListJournalsRow, error) {
			return nil, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestUpsertJournalAttendanceScanReturnsMetadata(t *testing.T) {
	pdfBytes := []byte("%PDF-1.4 fake attendance scan")

	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			if id != 21 {
				t.Fatalf("unexpected journal id: %d", id)
			}
			return sqlc.GetJournalByIDRow{ID: 21}, nil
		},
		upsertAttendanceScanFunc: func(_ context.Context, arg sqlc.UpsertJournalAttendanceScanParams) (sqlc.UpsertJournalAttendanceScanRow, error) {
			if arg.JournalID != 21 {
				t.Fatalf("unexpected journal id: %d", arg.JournalID)
			}
			if arg.FileName != "lista.pdf" {
				t.Fatalf("unexpected file name: %s", arg.FileName)
			}
			if arg.ContentType != "application/pdf" {
				t.Fatalf("unexpected content type: %s", arg.ContentType)
			}
			if arg.FileSize != int64(len(pdfBytes)) {
				t.Fatalf("unexpected file size: %d", arg.FileSize)
			}
			if !bytes.Equal(arg.FileData, pdfBytes) {
				t.Fatalf("unexpected file bytes: %q", arg.FileData)
			}
			if arg.UploadedByUserID != 7 {
				t.Fatalf("unexpected uploadedByUserID: %d", arg.UploadedByUserID)
			}

			return sqlc.UpsertJournalAttendanceScanRow{
				ID:               3,
				JournalID:        21,
				FileName:         "lista.pdf",
				ContentType:      "application/pdf",
				FileSize:         int64(len(pdfBytes)),
				UploadedByUserID: 7,
				CreatedAt:        pgtype.Timestamptz{Time: time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC), Valid: true},
				UpdatedAt:        pgtype.Timestamptz{Time: time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req, _ := newMultipartFileRequest(t, http.MethodPost, "/api/v1/journals/21/attendance-scan", "file", "lista.pdf", pdfBytes)
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 7, Role: 2}))
	rec := httptest.NewRecorder()

	handler.UpsertJournalAttendanceScan(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody JournalAttendanceScanResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 3 {
		t.Fatalf("expected scan id 3, got %d", responseBody.Data.ID)
	}
	if responseBody.Data.FileName != "lista.pdf" {
		t.Fatalf("unexpected file name: %s", responseBody.Data.FileName)
	}
	if responseBody.Data.ContentType != "application/pdf" {
		t.Fatalf("unexpected content type: %s", responseBody.Data.ContentType)
	}
}

func TestUpsertJournalAttendanceScanReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/foo/attendance-scan", nil)
	req.SetPathValue("id", "foo")
	rec := httptest.NewRecorder()

	handler.UpsertJournalAttendanceScan(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestUpsertJournalAttendanceScanReturnsBadRequestWhenFileMissing(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/journals/21/attendance-scan", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 7, Role: 2}))
	rec := httptest.NewRecorder()

	handler.UpsertJournalAttendanceScan(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestUpsertJournalAttendanceScanReturnsUnauthorizedWithoutUser(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
	})

	req, _ := newMultipartFileRequest(t, http.MethodPost, "/api/v1/journals/21/attendance-scan", "file", "lista.pdf", []byte("%PDF-1.4 fake"))
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.UpsertJournalAttendanceScan(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestUpsertJournalAttendanceScanReturnsBadRequestForUnsupportedFileType(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
	})

	req, _ := newMultipartFileRequest(t, http.MethodPost, "/api/v1/journals/21/attendance-scan", "file", "lista.txt", []byte("plain text"))
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 7, Role: 2}))
	rec := httptest.NewRecorder()

	handler.UpsertJournalAttendanceScan(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestUpsertJournalAttendanceScanReturnsNotFoundForMissingJournal(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{}, pgx.ErrNoRows
		},
	})

	req, _ := newMultipartFileRequest(t, http.MethodPost, "/api/v1/journals/21/attendance-scan", "file", "lista.pdf", []byte("%PDF-1.4 fake"))
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 7, Role: 2}))
	rec := httptest.NewRecorder()

	handler.UpsertJournalAttendanceScan(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestUpsertJournalAttendanceScanReturnsBadRequestForFileTooLarge(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
	})

	tooLarge := bytes.Repeat([]byte("a"), (16<<20)+1)
	req, _ := newMultipartFileRequest(t, http.MethodPost, "/api/v1/journals/21/attendance-scan", "file", "lista.pdf", tooLarge)
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 7, Role: 2}))
	rec := httptest.NewRecorder()

	handler.UpsertJournalAttendanceScan(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestUpsertJournalAttendanceScanReturnsInternalServerErrorWhenSaveFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalByIDFunc: func(_ context.Context, id int64) (sqlc.GetJournalByIDRow, error) {
			return sqlc.GetJournalByIDRow{ID: id}, nil
		},
		upsertAttendanceScanFunc: func(_ context.Context, _ sqlc.UpsertJournalAttendanceScanParams) (sqlc.UpsertJournalAttendanceScanRow, error) {
			return sqlc.UpsertJournalAttendanceScanRow{}, errors.New("db error")
		},
	})

	req, _ := newMultipartFileRequest(t, http.MethodPost, "/api/v1/journals/21/attendance-scan", "file", "lista.pdf", []byte("%PDF-1.4 fake"))
	req.SetPathValue("id", "21")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{ID: 7, Role: 2}))
	rec := httptest.NewRecorder()

	handler.UpsertJournalAttendanceScan(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestGetJournalAttendanceScanMetaReturnsMetadata(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalAttendanceScanMetaFunc: func(_ context.Context, journalID int64) (sqlc.GetJournalAttendanceScanMetaRow, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journal id: %d", journalID)
			}
			return sqlc.GetJournalAttendanceScanMetaRow{
				ID:               4,
				JournalID:        21,
				FileName:         "lista-obecnosci.pdf",
				ContentType:      "application/pdf",
				FileSize:         2048,
				UploadedByUserID: 7,
				CreatedAt:        pgtype.Timestamptz{Time: time.Date(2026, 3, 23, 11, 0, 0, 0, time.UTC), Valid: true},
				UpdatedAt:        pgtype.Timestamptz{Time: time.Date(2026, 3, 23, 11, 15, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendance-scan/meta", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GetJournalAttendanceScanMeta(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody JournalAttendanceScanResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 4 {
		t.Fatalf("expected scan id 4, got %d", responseBody.Data.ID)
	}
	if responseBody.Data.FileName != "lista-obecnosci.pdf" {
		t.Fatalf("unexpected file name: %s", responseBody.Data.FileName)
	}
	if responseBody.Data.FileSize != 2048 {
		t.Fatalf("unexpected file size: %d", responseBody.Data.FileSize)
	}
}

func TestGetJournalAttendanceScanMetaReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/foo/attendance-scan/meta", nil)
	req.SetPathValue("id", "foo")
	rec := httptest.NewRecorder()

	handler.GetJournalAttendanceScanMeta(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGetJournalAttendanceScanMetaReturnsNotFound(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalAttendanceScanMetaFunc: func(_ context.Context, journalID int64) (sqlc.GetJournalAttendanceScanMetaRow, error) {
			return sqlc.GetJournalAttendanceScanMetaRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendance-scan/meta", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GetJournalAttendanceScanMeta(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestGetJournalAttendanceScanMetaReturnsInternalServerError(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalAttendanceScanMetaFunc: func(_ context.Context, journalID int64) (sqlc.GetJournalAttendanceScanMetaRow, error) {
			return sqlc.GetJournalAttendanceScanMetaRow{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendance-scan/meta", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GetJournalAttendanceScanMeta(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestGetJournalAttendanceScanFileReturnsAttachment(t *testing.T) {
	fileBytes := []byte("%PDF-1.4 attendance scan")

	handler := NewHandler(fakeQuerier{
		getJournalAttendanceScanFileFunc: func(_ context.Context, journalID int64) (sqlc.GetJournalAttendanceScanFileRow, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journal id: %d", journalID)
			}
			return sqlc.GetJournalAttendanceScanFileRow{
				ID:          5,
				JournalID:   21,
				FileName:    "lista.pdf",
				ContentType: "application/pdf",
				FileData:    fileBytes,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendance-scan", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GetJournalAttendanceScanFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("expected content type application/pdf, got %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, `attachment; filename="lista.pdf"`) {
		t.Fatalf("expected attachment content disposition, got %q", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), fileBytes) {
		t.Fatalf("unexpected file body: %q", rec.Body.Bytes())
	}
}

func TestGetJournalAttendanceScanFileReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/foo/attendance-scan", nil)
	req.SetPathValue("id", "foo")
	rec := httptest.NewRecorder()

	handler.GetJournalAttendanceScanFile(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGetJournalAttendanceScanFileReturnsNotFound(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalAttendanceScanFileFunc: func(_ context.Context, journalID int64) (sqlc.GetJournalAttendanceScanFileRow, error) {
			return sqlc.GetJournalAttendanceScanFileRow{}, pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendance-scan", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GetJournalAttendanceScanFile(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestGetJournalAttendanceScanFileReturnsInternalServerError(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getJournalAttendanceScanFileFunc: func(_ context.Context, journalID int64) (sqlc.GetJournalAttendanceScanFileRow, error) {
			return sqlc.GetJournalAttendanceScanFileRow{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/journals/21/attendance-scan", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.GetJournalAttendanceScanFile(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestDeleteJournalAttendanceScanReturnsNoContent(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		deleteJournalAttendanceScanFunc: func(_ context.Context, journalID int64) (int64, error) {
			if journalID != 21 {
				t.Fatalf("unexpected journal id: %d", journalID)
			}
			return 1, nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21/attendance-scan", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.DeleteJournalAttendanceScanFile(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestDeleteJournalAttendanceScanReturnsBadRequestForInvalidJournalID(t *testing.T) {
	handler := NewHandler(fakeQuerier{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/foo/attendance-scan", nil)
	req.SetPathValue("id", "foo")
	rec := httptest.NewRecorder()

	handler.DeleteJournalAttendanceScanFile(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestDeleteJournalAttendanceScanReturnsNotFoundWhenRowsAffectedIsZero(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		deleteJournalAttendanceScanFunc: func(_ context.Context, journalID int64) (int64, error) {
			return 0, nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21/attendance-scan", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.DeleteJournalAttendanceScanFile(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestDeleteJournalAttendanceScanReturnsInternalServerError(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		deleteJournalAttendanceScanFunc: func(_ context.Context, journalID int64) (int64, error) {
			return 0, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/journals/21/attendance-scan", nil)
	req.SetPathValue("id", "21")
	rec := httptest.NewRecorder()

	handler.DeleteJournalAttendanceScanFile(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}
