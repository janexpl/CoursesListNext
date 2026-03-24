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
	ListCoursesFunc   func(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error)
	GetCourseByIDFunc func(ctx context.Context, id int64) (sqlc.Course, error)
	UpdateCourseFunc  func(ctx context.Context, arg sqlc.UpdateCourseParams) (sqlc.Course, error)
	CreateCourseFunc  func(ctx context.Context, arg sqlc.CreateCourseParams) (sqlc.Course, error)
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
	})

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
	})

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
	})

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
	})

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
	})

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
	})

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
	})

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
	})

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
	})

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
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/courses?search=bhp&limit=20", nil)

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestPatchCourseReturnsUpdatedCourse(t *testing.T) {
	expiryTime := "5"
	handler := NewHandler(fakeQuerier{
		UpdateCourseFunc: func(ctx context.Context, arg sqlc.UpdateCourseParams) (sqlc.Course, error) {
			if arg.ID != 12 {
				t.Fatalf("expected course id 12, got %d", arg.ID)
			}
			if !arg.Mainname.Valid || arg.Mainname.String != "BHP" {
				t.Fatalf("unexpected mainname: %+v", arg.Mainname)
			}
			if arg.Name != "Szkolenie okresowe" {
				t.Fatalf("unexpected name: %q", arg.Name)
			}
			if arg.Symbol != "BHP-OKR" {
				t.Fatalf("unexpected symbol: %q", arg.Symbol)
			}
			if !arg.Expirytime.Valid || arg.Expirytime.String != "5" {
				t.Fatalf("unexpected expirytime: %+v", arg.Expirytime)
			}
			if string(arg.Courseprogram) != `[{"Subject":"Intro"}]` {
				t.Fatalf("unexpected courseprogram: %q", string(arg.Courseprogram))
			}
			if !arg.Certfrontpage.Valid || arg.Certfrontpage.String != "<p>Front</p>" {
				t.Fatalf("unexpected certfrontpage: %+v", arg.Certfrontpage)
			}

			return sqlc.Course{
				ID:            12,
				Mainname:      pgtype.Text{String: "BHP", Valid: true},
				Name:          "Szkolenie okresowe",
				Symbol:        "BHP-OKR",
				Expirytime:    pgtype.Text{String: expiryTime, Valid: true},
				Courseprogram: []byte(`[{"Subject":"Intro"}]`),
				Certfrontpage: pgtype.Text{String: "<p>Front</p>", Valid: true},
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

func TestPatchCourseReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		UpdateCourseFunc: func(context.Context, sqlc.UpdateCourseParams) (sqlc.Course, error) {
			t.Fatal("UpdateCourse should not be called for invalid id")
			return sqlc.Course{}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/abc", strings.NewReader(`{}`))
	req.SetPathValue("id", "abc")

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchCourseReturnsBadRequestForInvalidBody(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		UpdateCourseFunc: func(context.Context, sqlc.UpdateCourseParams) (sqlc.Course, error) {
			t.Fatal("UpdateCourse should not be called for invalid body")
			return sqlc.Course{}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/courses/12", strings.NewReader(`{"mainName":`))
	req.SetPathValue("id", "12")

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchCourseReturnsBadRequestForMissingRequiredField(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		UpdateCourseFunc: func(context.Context, sqlc.UpdateCourseParams) (sqlc.Course, error) {
			t.Fatal("UpdateCourse should not be called for invalid body")
			return sqlc.Course{}, nil
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
	handler := NewHandler(fakeQuerier{
		UpdateCourseFunc: func(context.Context, sqlc.UpdateCourseParams) (sqlc.Course, error) {
			t.Fatal("UpdateCourse should not be called for invalid body")
			return sqlc.Course{}, nil
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
	handler := NewHandler(fakeQuerier{
		UpdateCourseFunc: func(context.Context, sqlc.UpdateCourseParams) (sqlc.Course, error) {
			return sqlc.Course{}, pgx.ErrNoRows
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
	handler := NewHandler(fakeQuerier{
		UpdateCourseFunc: func(context.Context, sqlc.UpdateCourseParams) (sqlc.Course, error) {
			return sqlc.Course{}, errors.New("database error")
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
	handler := NewHandler(fakeQuerier{
		CreateCourseFunc: func(ctx context.Context, arg sqlc.CreateCourseParams) (sqlc.Course, error) {
			if !arg.Mainname.Valid || arg.Mainname.String != "BHP" {
				t.Fatalf("unexpected mainname: %+v", arg.Mainname)
			}
			if arg.Name != "Szkolenie okresowe" {
				t.Fatalf("unexpected name: %q", arg.Name)
			}
			if arg.Symbol != "BHP-OKR" {
				t.Fatalf("unexpected symbol: %q", arg.Symbol)
			}
			if !arg.Expirytime.Valid || arg.Expirytime.String != "5" {
				t.Fatalf("unexpected expirytime: %+v", arg.Expirytime)
			}
			if string(arg.Courseprogram) != `[{"Subject":"Intro"}]` {
				t.Fatalf("unexpected courseprogram: %q", string(arg.Courseprogram))
			}
			if !arg.Certfrontpage.Valid || arg.Certfrontpage.String != "<p>Front</p>" {
				t.Fatalf("unexpected certfrontpage: %+v", arg.Certfrontpage)
			}

			return sqlc.Course{
				ID:            13,
				Mainname:      pgtype.Text{String: "BHP", Valid: true},
				Name:          "Szkolenie okresowe",
				Symbol:        "BHP-OKR",
				Expirytime:    pgtype.Text{String: expiryTime, Valid: true},
				Courseprogram: []byte(`[{"Subject":"Intro"}]`),
				Certfrontpage: pgtype.Text{String: "<p>Front</p>", Valid: true},
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

func TestCreateCourseReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		CreateCourseFunc: func(context.Context, sqlc.CreateCourseParams) (sqlc.Course, error) {
			t.Fatal("CreateCourse should not be called for invalid body")
			return sqlc.Course{}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/courses", strings.NewReader(`{"mainName":`))

	handler.CreateCourse(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateCourseReturnsBadRequestForMissingRequiredField(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		CreateCourseFunc: func(context.Context, sqlc.CreateCourseParams) (sqlc.Course, error) {
			t.Fatal("CreateCourse should not be called for invalid body")
			return sqlc.Course{}, nil
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
	handler := NewHandler(fakeQuerier{
		CreateCourseFunc: func(context.Context, sqlc.CreateCourseParams) (sqlc.Course, error) {
			t.Fatal("CreateCourse should not be called for invalid body")
			return sqlc.Course{}, nil
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
	handler := NewHandler(fakeQuerier{
		CreateCourseFunc: func(context.Context, sqlc.CreateCourseParams) (sqlc.Course, error) {
			return sqlc.Course{}, errors.New("database error")
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
