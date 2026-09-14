// Package courses
package courses

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	ListCourses(ctx context.Context, arg sqlc.ListCoursesParams) ([]sqlc.ListCoursesRow, error)
	ListCoursesDetails(ctx context.Context, arg sqlc.ListCoursesDetailsParams) ([]sqlc.ListCoursesDetailsRow, error)
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

// ListDetails zwraca tę samą listę co List, ale z programem szkolenia,
// szablonem zaświadczenia i tłumaczeniami. Wszystko przychodzi jednym
// zapytaniem, więc liczba kursów nie przekłada się na liczbę zapytań.
func (h *Handler) ListDetails(w http.ResponseWriter, r *http.Request) {
	searchPg, limitInt, err := response.ParseListParams(r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	rows, err := h.queries.ListCoursesDetails(r.Context(), sqlc.ListCoursesDetailsParams{
		Search:     searchPg,
		LimitCount: limitInt,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list courses")
		return
	}
	resp := ListCoursesDetailsResponse{
		Data: make([]CourseDetailDTO, 0, len(rows)),
	}
	for _, row := range rows {
		dto, err := makeCourseDetailDTOFromListRow(row)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list courses")
			return
		}
		resp.Data = append(resp.Data, dto)
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
		if message, ok := courseValidationMessage(err); ok {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, message)
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
		if message, ok := courseValidationMessage(err); ok {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, message)
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
	expiryTime := parseExpiryTime(row.Expirytime)

	return CourseDTO{
		ID:         row.ID,
		MainName:   row.Mainname.String,
		Name:       row.Name,
		Symbol:     row.Symbol,
		ExpiryTime: expiryTime,
	}
}

func makeCourseDetailDTO(row sqlc.Course, translations []sqlc.ListCourseCertificateTranslationsByCourseIDRow) CourseDetailDTO {
	expiryTime := parseExpiryTime(row.Expirytime)

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

// makeCourseDetailDTOFromListRow rozpakowuje tłumaczenia, które
// ListCoursesDetails agreguje po stronie bazy do tablicy JSON.
//
// Klucze budowane w json_build_object (internal/db/queries/courses.sql) muszą
// odpowiadać tagom JSON w CourseCertificateTranslationDTO - rozjazd nie da
// błędu, tylko ciche puste tłumaczenia. Pilnuje tego
// TestListCoursesDetailsUnpacksAggregatedTranslations.
func makeCourseDetailDTOFromListRow(row sqlc.ListCoursesDetailsRow) (CourseDetailDTO, error) {
	expiryTime := parseExpiryTime(row.Expirytime)

	translations := make([]CourseCertificateTranslationDTO, 0)
	if len(row.CertificateTranslations) > 0 {
		if err := json.Unmarshal(row.CertificateTranslations, &translations); err != nil {
			return CourseDetailDTO{}, err
		}
	}

	return CourseDetailDTO{
		ID:                      row.ID,
		MainName:                row.Mainname.String,
		Name:                    row.Name,
		Symbol:                  row.Symbol,
		ExpiryTime:              expiryTime,
		CourseProgram:           string(row.Courseprogram),
		CertFrontPage:           row.Certfrontpage.String,
		CertificateTranslations: translations,
	}, nil
}

// parseExpiryTime odczytuje okres ważności zapisany tekstowo. Wartość, której nie
// da się odczytać jako nieujemnej liczby całkowitej, jest zwracana jako brak
// terminu - tak samo traktują ją zapytania liczące datę wygaśnięcia zaświadczeń.
func parseExpiryTime(value pgtype.Text) *int {
	if !value.Valid {
		return nil
	}
	years, err := strconv.Atoi(strings.TrimSpace(value.String))
	if err != nil || years < 0 {
		return nil
	}
	return &years
}

// courseValidationMessage zamienia błąd walidacji na komunikat dla klienta.
// Szczegółowe błędy mają pierwszeństwo przed ogólnym "invalid request body".
func courseValidationMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrInvalidCourseProgram):
		return "course program must be a JSON array", true
	case errors.Is(err, ErrUnsupportedTranslationLanguage):
		return "unsupported translation language", true
	case errors.Is(err, ErrDuplicateTranslationLanguage):
		return "duplicate translation language", true
	case errors.Is(err, ErrIncompleteTranslation):
		return "translation fields are required", true
	case errors.Is(err, ErrInvalidInput):
		return "invalid request body", true
	default:
		return "", false
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

// mapCourseTranslationInputs przenosi nil dalej: brak pola w JSON-ie oznacza
// "nie zmieniaj tłumaczeń", a pusta tablica - "usuń wszystkie".
func mapCourseTranslationInputs(translations []CourseCertificateTranslationDTO) []CourseTranslationInput {
	if translations == nil {
		return nil
	}
	result := make([]CourseTranslationInput, 0, len(translations))
	for _, translation := range translations {
		result = append(result, CourseTranslationInput(translation))
	}
	return result
}
