package courses

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	ListCourses(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error)
	GetCourseByID(ctx context.Context, id int64) (sqlc.Course, error)
	UpdateCourse(ctx context.Context, arg sqlc.UpdateCourseParams) (sqlc.Course, error)
	CreateCourse(ctx context.Context, arg sqlc.CreateCourseParams) (sqlc.Course, error)
}

type Handler struct {
	queries Querier
}

func NewHandler(queries Querier) *Handler {
	return &Handler{
		queries: queries,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	searchPg, limitInt, err := response.ParseListParams(r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	courses, err := h.queries.ListCourses(r.Context(), sqlc.ListCoursesParams{
		Search:     searchPg,
		LimitCount: limitInt,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list courses")
		return
	}
	resp := ListCoursesResponse{
		Data: make([]CourseDTO, 0, len(courses)),
	}
	for _, row := range courses {
		resp.Data = append(resp.Data, makeCourseDTO(row))
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	idInt, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid course ID")
		return
	}
	course, err := h.queries.GetCourseByID(r.Context(), idInt)
	if err != nil {
		response.HandleDBError(w, err, "course")
		return
	}
	resp := GetCourseResponse{
		Data: makeCourseDetailDTO(course),
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	idInt, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid course ID")
		return
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := UpdateCourseRequest{}
	err = decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	mainName := strings.TrimSpace(req.MainName)
	name := strings.TrimSpace(req.Name)
	symbol := strings.TrimSpace(req.Symbol)
	courseProgram := strings.TrimSpace(req.CourseProgram)
	certFrontPage := strings.TrimSpace(req.CertFrontPage)

	if name == "" || mainName == "" || symbol == "" || certFrontPage == "" || courseProgram == "" || req.ExpiryTime == nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	expiryValue := strings.TrimSpace(*req.ExpiryTime)

	expiryInt, err := strconv.Atoi(expiryValue)
	if err != nil || expiryInt < 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.queries.UpdateCourse(r.Context(), sqlc.UpdateCourseParams{
		ID:            idInt,
		Mainname:      pgtype.Text{String: mainName, Valid: true},
		Name:          name,
		Symbol:        symbol,
		Expirytime:    pgtype.Text{String: expiryValue, Valid: true},
		Courseprogram: []byte(courseProgram),
		Certfrontpage: pgtype.Text{String: certFrontPage, Valid: true},
	})

	if err != nil {
		response.HandleDBError(w, err, "course")
		return
	}

	response.WriteJSON(w, http.StatusOK, GetCourseResponse{
		Data: makeCourseDetailDTO(row),
	})
}

func (h *Handler) CreateCourse(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	req := CreateCourseRequest{}
	err := decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	mainName := strings.TrimSpace(req.MainName)
	name := strings.TrimSpace(req.Name)
	symbol := strings.TrimSpace(req.Symbol)
	courseProgram := strings.TrimSpace(req.CourseProgram)
	certFrontPage := strings.TrimSpace(req.CertFrontPage)

	if name == "" || mainName == "" || symbol == "" || certFrontPage == "" || courseProgram == "" || req.ExpiryTime == nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	expiryValue := strings.TrimSpace(*req.ExpiryTime)

	expiryInt, err := strconv.Atoi(expiryValue)
	if err != nil || expiryInt < 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	row, err := h.queries.CreateCourse(r.Context(), sqlc.CreateCourseParams{
		Mainname:      pgtype.Text{String: mainName, Valid: true},
		Name:          name,
		Symbol:        symbol,
		Expirytime:    pgtype.Text{String: expiryValue, Valid: true},
		Courseprogram: []byte(courseProgram),
		Certfrontpage: pgtype.Text{String: certFrontPage, Valid: true},
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create course")
		return
	}
	response.WriteJSON(w, http.StatusCreated, GetCourseResponse{
		Data: makeCourseDetailDTO(row),
	})

}
func makeCourseDTO(row sqlc.ListCoursesRow) CourseDTO {
	var expiryTime *string
	if row.Expirytime.Valid {
		expiryTime = &row.Expirytime.String
	}

	return CourseDTO{
		ID:         row.ID,
		MainName:   row.Mainname.String,
		Name:       row.Name,
		Symbol:     row.Symbol,
		ExpiryTime: expiryTime,
	}
}

func makeCourseDetailDTO(row sqlc.Course) CourseDetailDTO {
	var expiryTime *string
	if row.Expirytime.Valid {
		expiryTime = &row.Expirytime.String
	}

	return CourseDetailDTO{
		ID:            row.ID,
		MainName:      row.Mainname.String,
		Name:          row.Name,
		Symbol:        row.Symbol,
		ExpiryTime:    expiryTime,
		CourseProgram: string(row.Courseprogram),
		CertFrontPage: row.Certfrontpage.String,
	}
}
