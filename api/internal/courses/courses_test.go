package courses

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeQuerier struct {
	ListCoursesFunc                                 func(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error)
	GetCourseByIDFunc                               func(ctx context.Context, id int64) (sqlc.Course, error)
	UpdateCourseFunc                                func(ctx context.Context, arg sqlc.UpdateCourseParams) (sqlc.Course, error)
	CreateCourseFunc                                func(ctx context.Context, arg sqlc.CreateCourseParams) (sqlc.Course, error)
	ListCourseCertificateTranslationsByCourseIDFunc func(ctx context.Context, courseID int64) ([]sqlc.ListCourseCertificateTranslationsByCourseIDRow, error)
}

type fakeCreator struct {
	CreateFunc func(ctx context.Context, input CreateCourseInput) (CourseDetailDTO, error)
	UpdateFunc func(ctx context.Context, courseID int64, input UpdateCourseInput) (CourseDetailDTO, error)
}

func (f fakeQuerier) ListCourses(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error) {
	return f.ListCoursesFunc(ctx, arg)
}

func (f fakeQuerier) GetCourseByID(ctx context.Context, id int64) (sqlc.Course, error) {
	return f.GetCourseByIDFunc(ctx, id)
}

func (f fakeQuerier) UpdateCourse(ctx context.Context, arg sqlc.UpdateCourseParams) (sqlc.Course, error) {
	return f.UpdateCourseFunc(ctx, arg)
}

func (f fakeQuerier) CreateCourse(ctx context.Context, arg sqlc.CreateCourseParams) (sqlc.Course, error) {
	return f.CreateCourseFunc(ctx, arg)
}

func (f fakeQuerier) ListCourseCertificateTranslationsByCourseID(ctx context.Context, courseID int64) ([]sqlc.ListCourseCertificateTranslationsByCourseIDRow, error) {
	if f.ListCourseCertificateTranslationsByCourseIDFunc == nil {
		return []sqlc.ListCourseCertificateTranslationsByCourseIDRow{}, nil
	}
	return f.ListCourseCertificateTranslationsByCourseIDFunc(ctx, courseID)
}

func (f fakeCreator) Create(ctx context.Context, input CreateCourseInput) (CourseDetailDTO, error) {
	if f.CreateFunc == nil {
		return CourseDetailDTO{}, errors.New("unexpected Create call")
	}
	return f.CreateFunc(ctx, input)
}

func (f fakeCreator) Update(ctx context.Context, courseID int64, input UpdateCourseInput) (CourseDetailDTO, error) {
	if f.UpdateFunc == nil {
		return CourseDetailDTO{}, errors.New("unexpected Update call")
	}
	return f.UpdateFunc(ctx, courseID, input)
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

func TestListCourses(t *testing.T) {
	courses := []sqlc.ListCoursesRow{
		{
			ID:         1,
			Mainname:   pgtype.Text{String: "Course 1", Valid: true},
			Name:       "Course 1",
			Symbol:     "C1",
			Expirytime: pgtype.Text{String: "5", Valid: true},
		},
		{
			ID:         2,
			Mainname:   pgtype.Text{String: "Course 2", Valid: true},
			Name:       "Course 2",
			Symbol:     "C2",
			Expirytime: pgtype.Text{},
		},
	}

	handler := NewHandler(fakeQuerier{
		ListCoursesFunc: func(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error) {
			if arg.Search.Valid {
				t.Fatalf("expected empty search arg, got %+v", arg.Search)
			}
			if arg.LimitCount != 50 {
				t.Fatalf("expected default limit 50, got %d", arg.LimitCount)
			}
			return courses, nil
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/courses", nil)

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody ListCoursesResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != len(courses) {
		t.Fatalf("expected %d courses, got %d", len(courses), len(responseBody.Data))
	}

	for i, course := range responseBody.Data {
		expected := courses[i]
		if course.ID != expected.ID || course.MainName != expected.Mainname.String || course.Name != expected.Name || course.Symbol != expected.Symbol {
			t.Errorf("course at index %d does not match expected value", i)
		}
	}

	if responseBody.Data[0].ExpiryTime == nil || *responseBody.Data[0].ExpiryTime != "5" {
		t.Fatalf("expected first course expiryTime to be %q, got %+v", "5", responseBody.Data[0].ExpiryTime)
	}

	if responseBody.Data[1].ExpiryTime != nil {
		t.Fatalf("expected second course expiryTime to be nil, got %+v", responseBody.Data[1].ExpiryTime)
	}
}

func TestListCoursesReturnsInternalError(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		ListCoursesFunc: func(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error) {
			if arg.LimitCount != 50 {
				t.Fatalf("expected default limit 50, got %d", arg.LimitCount)
			}
			return nil, errors.New("database error")
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/courses", nil)

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListCoursesReturnsEmptyList(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		ListCoursesFunc: func(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error) {
			if arg.LimitCount != 50 {
				t.Fatalf("expected default limit 50, got %d", arg.LimitCount)
			}
			return []sqlc.ListCoursesRow{}, nil
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/courses", nil)

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody ListCoursesResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 0 {
		t.Fatalf("expected 0 courses, got %d", len(responseBody.Data))
	}
}

func TestGetCourseReturnsCourseDetail(t *testing.T) {
	course := sqlc.Course{
		ID:            7,
		Mainname:      pgtype.Text{String: "Main Course", Valid: true},
		Name:          "Course Detail",
		Symbol:        "CD-1",
		Expirytime:    pgtype.Text{String: "3", Valid: true},
		Courseprogram: []byte(`{"sections":["intro"]}`),
		Certfrontpage: pgtype.Text{String: "<p>Front</p>", Valid: true},
	}

	handler := NewHandler(fakeQuerier{
		GetCourseByIDFunc: func(ctx context.Context, id int64) (sqlc.Course, error) {
			if id != 7 {
				t.Fatalf("expected course id %d, got %d", 7, id)
			}
			return course, nil
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/courses/7", nil)
	req.SetPathValue("id", "7")

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody GetCourseResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 7 || responseBody.Data.MainName != "Main Course" || responseBody.Data.Name != "Course Detail" || responseBody.Data.Symbol != "CD-1" {
		t.Fatalf("unexpected course detail payload: %+v", responseBody.Data)
	}

	if responseBody.Data.ExpiryTime == nil || *responseBody.Data.ExpiryTime != "3" {
		t.Fatalf("expected expiryTime to be %q, got %+v", "3", responseBody.Data.ExpiryTime)
	}

	if responseBody.Data.CourseProgram != `{"sections":["intro"]}` {
		t.Fatalf("unexpected courseProgram: %q", responseBody.Data.CourseProgram)
	}

	if responseBody.Data.CertFrontPage != "<p>Front</p>" {
		t.Fatalf("unexpected certFrontPage: %q", responseBody.Data.CertFrontPage)
	}
}

func TestGetCourseReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		GetCourseByIDFunc: func(ctx context.Context, id int64) (sqlc.Course, error) {
			t.Fatalf("GetCourseByID should not be called for invalid id, got %d", id)
			return sqlc.Course{}, nil
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/courses/not-a-number", nil)
	req.SetPathValue("id", "not-a-number")

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGetCourseReturnsInternalError(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		GetCourseByIDFunc: func(ctx context.Context, id int64) (sqlc.Course, error) {
			return sqlc.Course{}, errors.New("database error")
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/courses/7", nil)
	req.SetPathValue("id", "7")

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestGetCourseReturnsNotFound(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		GetCourseByIDFunc: func(ctx context.Context, id int64) (sqlc.Course, error) {
			return sqlc.Course{}, pgx.ErrNoRows
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/courses/7", nil)
	req.SetPathValue("id", "7")

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestListCoursesReturnsBadRequestForInvalidLimit(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		ListCoursesFunc: func(context.Context, sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error) {
			t.Fatal("ListCourses should not be called for invalid limit")
			return nil, nil
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/courses?limit=abc", nil)

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListCoursesReturnsBadRequestForOutOfRangeLimit(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		ListCoursesFunc: func(context.Context, sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error) {
			t.Fatal("ListCourses should not be called for invalid limit")
			return nil, nil
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/courses?limit=101", nil)

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListCoursesPassesFiltersToQuery(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		ListCoursesFunc: func(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error) {
			if !arg.Search.Valid || arg.Search.String != "bhp" {
				t.Fatalf("expected search arg %q, got %+v", "bhp", arg.Search)
			}
			if arg.LimitCount != 20 {
				t.Fatalf("expected limit 20, got %d", arg.LimitCount)
			}
			return []sqlc.ListCoursesRow{}, nil
		},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/courses?search=bhp&limit=20", nil)

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestPatchCourseReturnsUpdatedCourse(t *testing.T) {
	expiryTime := "5"
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		UpdateFunc: func(ctx context.Context, courseID int64, input UpdateCourseInput) (CourseDetailDTO, error) {
			if courseID != 12 {
				t.Fatalf("expected course id 12, got %d", courseID)
			}
			if input.MainName != "BHP" || input.Name != "Szkolenie okresowe" || input.Symbol != "BHP-OKR" {
				t.Fatalf("unexpected update input: %+v", input)
			}
			if input.ExpiryTime != "5" {
				t.Fatalf("unexpected expirytime: %q", input.ExpiryTime)
			}
			if input.CourseProgram != `[{"Subject":"Intro"}]` {
				t.Fatalf("unexpected courseprogram: %q", input.CourseProgram)
			}
			if input.CertFrontPage != "<p>Front</p>" {
				t.Fatalf("unexpected certfrontpage: %q", input.CertFrontPage)
			}

			return CourseDetailDTO{
				ID:            12,
				MainName:      "BHP",
				Name:          "Szkolenie okresowe",
				Symbol:        "BHP-OKR",
				ExpiryTime:    &expiryTime,
				CourseProgram: `[{"Subject":"Intro"}]`,
				CertFrontPage: "<p>Front</p>",
			}, nil
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"Szkolenie okresowe",
		"symbol":"BHP-OKR",
		"expiryTime":"5",
		"courseProgram":"[{\"Subject\":\"Intro\"}]",
		"certFrontPage":"<p>Front</p>"
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/12", strings.NewReader(body))
	req.SetPathValue("id", "12")

	handler.Patch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody GetCourseResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 12 || responseBody.Data.MainName != "BHP" || responseBody.Data.Symbol != "BHP-OKR" {
		t.Fatalf("unexpected patch response payload: %+v", responseBody.Data)
	}
	if responseBody.Data.ExpiryTime == nil || *responseBody.Data.ExpiryTime != "5" {
		t.Fatalf("expected expiryTime to be %q, got %+v", "5", responseBody.Data.ExpiryTime)
	}
}

func TestPatchCoursePassesCertificateTranslationsToService(t *testing.T) {
	expiryTime := "5"
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		UpdateFunc: func(ctx context.Context, courseID int64, input UpdateCourseInput) (CourseDetailDTO, error) {
			if courseID != 12 {
				t.Fatalf("expected course id 12, got %d", courseID)
			}
			if len(input.CertificateTranslations) != 2 {
				t.Fatalf("expected 2 certificate translations, got %d", len(input.CertificateTranslations))
			}
			if input.CertificateTranslations[0].LanguageCode != "en" || input.CertificateTranslations[0].CourseName != "Periodic training" {
				t.Fatalf("unexpected first translation input: %+v", input.CertificateTranslations[0])
			}
			if input.CertificateTranslations[1].LanguageCode != "de" || input.CertificateTranslations[1].CertFrontPage != "<p>DE</p>" {
				t.Fatalf("unexpected second translation input: %+v", input.CertificateTranslations[1])
			}

			return CourseDetailDTO{
				ID:            12,
				MainName:      "BHP",
				Name:          "Szkolenie okresowe",
				Symbol:        "BHP-OKR",
				ExpiryTime:    &expiryTime,
				CourseProgram: `[ {"Subject":"Intro"} ]`,
				CertFrontPage: "<p>Front</p>",
				CertificateTranslations: []CourseCertificateTranslationDTO{
					{
						LanguageCode:  "en",
						CourseName:    "Periodic training",
						CourseProgram: `[{"Subject":"Introduction"}]`,
						CertFrontPage: "<p>EN</p>",
					},
					{
						LanguageCode:  "de",
						CourseName:    "Wiederholungsschulung",
						CourseProgram: `[{"Subject":"Einfuhrung"}]`,
						CertFrontPage: "<p>DE</p>",
					},
				},
			}, nil
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"Szkolenie okresowe",
		"symbol":"BHP-OKR",
		"expiryTime":"5",
		"courseProgram":"[ {\"Subject\":\"Intro\"} ]",
		"certFrontPage":"<p>Front</p>",
		"certificateTranslations":[
			{
				"languageCode":"en",
				"courseName":"Periodic training",
				"courseProgram":"[{\"Subject\":\"Introduction\"}]",
				"certFrontPage":"<p>EN</p>"
			},
			{
				"languageCode":"de",
				"courseName":"Wiederholungsschulung",
				"courseProgram":"[{\"Subject\":\"Einfuhrung\"}]",
				"certFrontPage":"<p>DE</p>"
			}
		]
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/12", strings.NewReader(body))
	req.SetPathValue("id", "12")

	handler.Patch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody GetCourseResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data.CertificateTranslations) != 2 {
		t.Fatalf("expected 2 certificate translations in response, got %d", len(responseBody.Data.CertificateTranslations))
	}
	if responseBody.Data.CertificateTranslations[0].LanguageCode != "en" {
		t.Fatalf("unexpected first translation in response: %+v", responseBody.Data.CertificateTranslations[0])
	}
}

func TestPatchCourseReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		UpdateFunc: func(context.Context, int64, UpdateCourseInput) (CourseDetailDTO, error) {
			t.Fatal("Update should not be called for invalid id")
			return CourseDetailDTO{}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/abc", strings.NewReader(`{}`))
	req.SetPathValue("id", "abc")

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchCourseReturnsBadRequestForInvalidBody(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		UpdateFunc: func(context.Context, int64, UpdateCourseInput) (CourseDetailDTO, error) {
			t.Fatal("Update should not be called for invalid body")
			return CourseDetailDTO{}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/12", strings.NewReader(`{"mainName":`))
	req.SetPathValue("id", "12")

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchCourseReturnsBadRequestForMissingRequiredField(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		UpdateFunc: func(context.Context, int64, UpdateCourseInput) (CourseDetailDTO, error) {
			t.Fatal("Update should not be called for invalid body")
			return CourseDetailDTO{}, nil
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"Szkolenie okresowe",
		"symbol":"BHP-OKR",
		"courseProgram":"[]",
		"certFrontPage":"<p>Front</p>"
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/12", strings.NewReader(body))
	req.SetPathValue("id", "12")

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchCourseReturnsBadRequestForUnknownField(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		UpdateFunc: func(context.Context, int64, UpdateCourseInput) (CourseDetailDTO, error) {
			t.Fatal("Update should not be called for invalid body")
			return CourseDetailDTO{}, nil
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"Szkolenie okresowe",
		"symbol":"BHP-OKR",
		"expiryTime":"5",
		"courseProgram":"[]",
		"certFrontPage":"<p>Front</p>",
		"extra":"x"
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/12", strings.NewReader(body))
	req.SetPathValue("id", "12")

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchCourseReturnsNotFoundWhenCourseDoesNotExist(t *testing.T) {
	expiryTime := "5"
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		UpdateFunc: func(context.Context, int64, UpdateCourseInput) (CourseDetailDTO, error) {
			return CourseDetailDTO{}, pgx.ErrNoRows
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"Szkolenie okresowe",
		"symbol":"BHP-OKR",
		"expiryTime":"` + expiryTime + `",
		"courseProgram":"[]",
		"certFrontPage":"<p>Front</p>"
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/12", strings.NewReader(body))
	req.SetPathValue("id", "12")

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchCourseReturnsInternalError(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		UpdateFunc: func(context.Context, int64, UpdateCourseInput) (CourseDetailDTO, error) {
			return CourseDetailDTO{}, errors.New("database error")
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"Szkolenie okresowe",
		"symbol":"BHP-OKR",
		"expiryTime":"5",
		"courseProgram":"[]",
		"certFrontPage":"<p>Front</p>"
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/12", strings.NewReader(body))
	req.SetPathValue("id", "12")

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestCreateCourseReturnsCreatedCourse(t *testing.T) {
	expiryTime := "5"
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		CreateFunc: func(ctx context.Context, input CreateCourseInput) (CourseDetailDTO, error) {
			if input.MainName != "BHP" || input.Name != "Szkolenie okresowe" || input.Symbol != "BHP-OKR" {
				t.Fatalf("unexpected create input: %+v", input)
			}
			if input.ExpiryTime != "5" {
				t.Fatalf("unexpected expirytime: %q", input.ExpiryTime)
			}
			if input.CourseProgram != `[{"Subject":"Intro"}]` {
				t.Fatalf("unexpected courseprogram: %q", input.CourseProgram)
			}
			if input.CertFrontPage != "<p>Front</p>" {
				t.Fatalf("unexpected certfrontpage: %q", input.CertFrontPage)
			}

			return CourseDetailDTO{
				ID:            13,
				MainName:      "BHP",
				Name:          "Szkolenie okresowe",
				Symbol:        "BHP-OKR",
				ExpiryTime:    &expiryTime,
				CourseProgram: `[{"Subject":"Intro"}]`,
				CertFrontPage: "<p>Front</p>",
			}, nil
		},
	})

	body := `{
		"mainName":"  BHP ",
		"name":" Szkolenie okresowe ",
		"symbol":" BHP-OKR ",
		"expiryTime":"5",
		"courseProgram":" [{\"Subject\":\"Intro\"}] ",
		"certFrontPage":" <p>Front</p> "
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/courses", strings.NewReader(body))

	handler.CreateCourse(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody GetCourseResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 13 || responseBody.Data.MainName != "BHP" || responseBody.Data.Symbol != "BHP-OKR" {
		t.Fatalf("unexpected create response payload: %+v", responseBody.Data)
	}
	if responseBody.Data.ExpiryTime == nil || *responseBody.Data.ExpiryTime != "5" {
		t.Fatalf("expected expiryTime to be %q, got %+v", "5", responseBody.Data.ExpiryTime)
	}
}

func TestCreateCoursePassesCertificateTranslationsToService(t *testing.T) {
	expiryTime := "5"
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		CreateFunc: func(ctx context.Context, input CreateCourseInput) (CourseDetailDTO, error) {
			if len(input.CertificateTranslations) != 2 {
				t.Fatalf("expected 2 certificate translations, got %d", len(input.CertificateTranslations))
			}
			if input.CertificateTranslations[0].LanguageCode != "en" || input.CertificateTranslations[0].CourseName != "Periodic training" {
				t.Fatalf("unexpected first translation input: %+v", input.CertificateTranslations[0])
			}
			if input.CertificateTranslations[1].LanguageCode != "de" || input.CertificateTranslations[1].CourseProgram != `[{"Subject":"Einfuhrung"}]` {
				t.Fatalf("unexpected second translation input: %+v", input.CertificateTranslations[1])
			}

			return CourseDetailDTO{
				ID:            13,
				MainName:      "BHP",
				Name:          "Szkolenie okresowe",
				Symbol:        "BHP-OKR",
				ExpiryTime:    &expiryTime,
				CourseProgram: `[ {"Subject":"Intro"} ]`,
				CertFrontPage: "<p>Front</p>",
				CertificateTranslations: []CourseCertificateTranslationDTO{
					{
						LanguageCode:  "en",
						CourseName:    "Periodic training",
						CourseProgram: `[{"Subject":"Introduction"}]`,
						CertFrontPage: "<p>EN</p>",
					},
					{
						LanguageCode:  "de",
						CourseName:    "Wiederholungsschulung",
						CourseProgram: `[{"Subject":"Einfuhrung"}]`,
						CertFrontPage: "<p>DE</p>",
					},
				},
			}, nil
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"Szkolenie okresowe",
		"symbol":"BHP-OKR",
		"expiryTime":"5",
		"courseProgram":"[ {\"Subject\":\"Intro\"} ]",
		"certFrontPage":"<p>Front</p>",
		"certificateTranslations":[
			{
				"languageCode":"en",
				"courseName":"Periodic training",
				"courseProgram":"[{\"Subject\":\"Introduction\"}]",
				"certFrontPage":"<p>EN</p>"
			},
			{
				"languageCode":"de",
				"courseName":"Wiederholungsschulung",
				"courseProgram":"[{\"Subject\":\"Einfuhrung\"}]",
				"certFrontPage":"<p>DE</p>"
			}
		]
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/courses", strings.NewReader(body))

	handler.CreateCourse(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var responseBody GetCourseResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data.CertificateTranslations) != 2 {
		t.Fatalf("expected 2 certificate translations in response, got %d", len(responseBody.Data.CertificateTranslations))
	}
	if responseBody.Data.CertificateTranslations[1].LanguageCode != "de" {
		t.Fatalf("unexpected second translation in response: %+v", responseBody.Data.CertificateTranslations[1])
	}
}

func TestCreateCourseReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		CreateFunc: func(context.Context, CreateCourseInput) (CourseDetailDTO, error) {
			t.Fatal("Create should not be called for invalid body")
			return CourseDetailDTO{}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/courses", strings.NewReader(`{"mainName":`))

	handler.CreateCourse(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateCourseReturnsBadRequestForMissingRequiredField(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		CreateFunc: func(context.Context, CreateCourseInput) (CourseDetailDTO, error) {
			t.Fatal("Create should not be called for invalid body")
			return CourseDetailDTO{}, nil
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"  ",
		"symbol":"BHP-OKR",
		"courseProgram":"[]",
		"certFrontPage":"<p>Front</p>",
		"expiryTime":"5"
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/courses", strings.NewReader(body))

	handler.CreateCourse(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateCourseReturnsBadRequestForUnknownField(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		CreateFunc: func(context.Context, CreateCourseInput) (CourseDetailDTO, error) {
			t.Fatal("Create should not be called for invalid body")
			return CourseDetailDTO{}, nil
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"Szkolenie okresowe",
		"symbol":"BHP-OKR",
		"expiryTime":"5",
		"courseProgram":"[]",
		"certFrontPage":"<p>Front</p>",
		"extra":"x"
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/courses", strings.NewReader(body))

	handler.CreateCourse(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateCourseReturnsInternalError(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		CreateFunc: func(context.Context, CreateCourseInput) (CourseDetailDTO, error) {
			return CourseDetailDTO{}, errors.New("database error")
		},
	})

	body := `{
		"mainName":"BHP",
		"name":"Szkolenie okresowe",
		"symbol":"BHP-OKR",
		"expiryTime":"5",
		"courseProgram":"[]",
		"certFrontPage":"<p>Front</p>"
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/courses", strings.NewReader(body))

	handler.CreateCourse(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}
