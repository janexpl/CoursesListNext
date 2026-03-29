// Package courses
package courses

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	ListCourses(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error)
	GetCourseByID(ctx context.Context, id int64) (sqlc.Course, error)
	ListCourseCertificateTranslationsByCourseID(ctx context.Context, courseID int64) ([]sqlc.ListCourseCertificateTranslationsByCourseIDRow, error)
}

type Creator interface {
	Create(ctx context.Context, input CreateCourseInput) (CourseDetailDTO, error)
	Update(ctx context.Context, courseID int64, input UpdateCourseInput) (CourseDetailDTO, error)
}

type Handler struct {
	queries Querier
	creator Creator
}

func NewHandler(queries Querier, creator Creator) *Handler {
	return &Handler{
		queries: queries,
		creator: creator,
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
	translations, err := h.queries.ListCourseCertificateTranslationsByCourseID(r.Context(), idInt)
	if err != nil {
		response.HandleDBError(w, err, "translation")
		return
	}

	resp := GetCourseResponse{
		Data: makeCourseDetailDTO(course, translations),
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

	row, err := h.creator.Update(r.Context(), idInt, UpdateCourseInput{
		MainName:                mainName,
		Name:                    name,
		Symbol:                  symbol,
		ExpiryTime:              *req.ExpiryTime,
		CourseProgram:           courseProgram,
		CertFrontPage:           certFrontPage,
		CertificateTranslations: mapCourseTranslationInputs(req.CertificateTranslations),
	})
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "course not found")
			return
		}
		response.HandleDBError(w, err, "course")
		return
	}

	response.WriteJSON(w, http.StatusOK, GetCourseResponse{
		Data: row,
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
	row, err := h.creator.Create(r.Context(), CreateCourseInput{
		MainName:                mainName,
		Name:                    name,
		Symbol:                  symbol,
		ExpiryTime:              *req.ExpiryTime,
		CourseProgram:           courseProgram,
		CertFrontPage:           certFrontPage,
		CertificateTranslations: mapCourseTranslationInputs(req.CertificateTranslations),
	})
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
			return
		}
		if isCourseSymbolConflict(err) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "failed to create course: symbol exist")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create course")
		return
	}
	response.WriteJSON(w, http.StatusCreated, GetCourseResponse{
		Data: row,
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

func makeCourseDetailDTO(row sqlc.Course, translations []sqlc.ListCourseCertificateTranslationsByCourseIDRow) CourseDetailDTO {
	var expiryTime *string
	if row.Expirytime.Valid {
		expiryTime = &row.Expirytime.String
	}

	return CourseDetailDTO{
		ID:                      row.ID,
		MainName:                row.Mainname.String,
		Name:                    row.Name,
		Symbol:                  row.Symbol,
		ExpiryTime:              expiryTime,
		CourseProgram:           string(row.Courseprogram),
		CertFrontPage:           row.Certfrontpage.String,
		CertificateTranslations: makeCourseCertificateTranslationsDTO(translations),
	}
}

func isCourseSymbolConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "check_unique_symbol"
}

func makeCourseCertificateTranslationsDTO(
	rows []sqlc.ListCourseCertificateTranslationsByCourseIDRow,
) []CourseCertificateTranslationDTO {
	translationsDTO := make([]CourseCertificateTranslationDTO, 0, len(rows))
	for _, translation := range rows {
		translationsDTO = append(translationsDTO, CourseCertificateTranslationDTO{
			LanguageCode:  translation.LanguageCode,
			CourseName:    translation.CourseName,
			CourseProgram: translation.CourseProgram,
			CertFrontPage: translation.CertFrontPage,
		})
	}
	return translationsDTO
}

func mapCourseTranslationInputs(translations []CourseCertificateTranslationDTO) []CourseTranslationInput {
	result := make([]CourseTranslationInput, 0, len(translations))
	for _, translation := range translations {
		result = append(result, CourseTranslationInput(translation))
	}
	return result
}
