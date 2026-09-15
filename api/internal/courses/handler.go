// Package courses
package courses

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// courseListParams to wspólne parametry GET /courses i GET /courses/details.
type courseListParams struct {
	search              pgtype.Text
	limit               int32
	page                int32
	updatedSince        pgtype.Timestamptz
	deliveredByPlatform pgtype.Bool
}

// maxCoursePage ogranicza page tak, żeby (page-1)*limit mieściło się w int32.
const maxCoursePage = 1_000_000

// parseCourseListParams odczytuje parametry list kursów. Komunikat błędu nadaje się
// wprost do odpowiedzi 400.
func parseCourseListParams(r *http.Request) (courseListParams, string) {
	searchPg, limitInt, err := response.ParseListParams(r)
	if err != nil {
		return courseListParams{}, err.Error()
	}
	params := courseListParams{search: searchPg, limit: limitInt, page: 1}

	query := r.URL.Query()
	if raw := query.Get("page"); raw != "" {
		page, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || page < 1 || page > maxCoursePage {
			return courseListParams{}, "invalid page value"
		}
		params.page = int32(page)
	}
	if raw := query.Get("updatedSince"); raw != "" {
		// RFC 3339 to profil ISO 8601 z obowiązkową strefą - moment bez strefy byłby niejednoznaczny.
		since, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return courseListParams{}, "invalid updatedSince value"
		}
		params.updatedSince = pgtype.Timestamptz{Time: since, Valid: true}
	}
	if raw := query.Get("deliveredByPlatform"); raw != "" {
		switch raw {
		case "true":
			params.deliveredByPlatform = pgtype.Bool{Bool: true, Valid: true}
		case "false":
			params.deliveredByPlatform = pgtype.Bool{Bool: false, Valid: true}
		default:
			return courseListParams{}, "invalid deliveredByPlatform value"
		}
	}
	return params, ""
}

func (p courseListParams) offset() int32 {
	return (p.page - 1) * p.limit
}

// pagination liczy kopertę. total pochodzi z COUNT(*) OVER () w zapytaniu; gdy strona
// wykracza poza wyniki, zapytanie nie zwraca wierszy, więc total dopytuje fetchTotal.
func (p courseListParams) pagination(windowTotal int64, rows int, fetchTotal func() (int64, error)) (PaginationDTO, error) {
	total := windowTotal
	if rows == 0 && p.page > 1 {
		var err error
		if total, err = fetchTotal(); err != nil {
			return PaginationDTO{}, err
		}
	}
	// Zwrócone wiersze to dolna granica liczby wyników (gdy total nie przyszedł z zapytania).
	if minimum := int64(p.offset()) + int64(rows); rows > 0 && total < minimum {
		total = minimum
	}
	return PaginationDTO{
		Page:       p.page,
		Limit:      p.limit,
		Total:      total,
		TotalPages: int32((total + int64(p.limit) - 1) / int64(p.limit)),
	}, nil
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	params, message := parseCourseListParams(r)
	if message != "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, message)
		return
	}
	queryParams := sqlc.ListCoursesParams{
		Search:              params.search,
		UpdatedSince:        params.updatedSince,
		DeliveredByPlatform: params.deliveredByPlatform,
		OffsetCount:         params.offset(),
		LimitCount:          params.limit,
	}
	courses, err := h.queries.ListCourses(r.Context(), queryParams)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list courses")
		return
	}
	var windowTotal int64
	if len(courses) > 0 {
		windowTotal = courses[0].TotalCount
	}
	pagination, err := params.pagination(windowTotal, len(courses), func() (int64, error) {
		firstPage := queryParams
		firstPage.OffsetCount, firstPage.LimitCount = 0, 1
		rows, err := h.queries.ListCourses(r.Context(), firstPage)
		if err != nil || len(rows) == 0 {
			return 0, err
		}
		return rows[0].TotalCount, nil
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list courses")
		return
	}
	resp := ListCoursesResponse{
		Data:       make([]CourseDTO, 0, len(courses)),
		Pagination: pagination,
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
	params, message := parseCourseListParams(r)
	if message != "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, message)
		return
	}
	queryParams := sqlc.ListCoursesDetailsParams{
		Search:              params.search,
		UpdatedSince:        params.updatedSince,
		DeliveredByPlatform: params.deliveredByPlatform,
		OffsetCount:         params.offset(),
		LimitCount:          params.limit,
	}
	rows, err := h.queries.ListCoursesDetails(r.Context(), queryParams)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list courses")
		return
	}
	var windowTotal int64
	if len(rows) > 0 {
		windowTotal = rows[0].TotalCount
	}
	pagination, err := params.pagination(windowTotal, len(rows), func() (int64, error) {
		// Lżejsze ListCourses z tymi samymi filtrami wystarczy do policzenia wyników.
		countRows, err := h.queries.ListCourses(r.Context(), sqlc.ListCoursesParams{
			Search:              params.search,
			UpdatedSince:        params.updatedSince,
			DeliveredByPlatform: params.deliveredByPlatform,
			LimitCount:          1,
		})
		if err != nil || len(countRows) == 0 {
			return 0, err
		}
		return countRows[0].TotalCount, nil
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list courses")
		return
	}
	resp := ListCoursesDetailsResponse{
		Data:       make([]CourseDetailDTO, 0, len(rows)),
		Pagination: pagination,
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

// PlatformDeliverySetter zmienia flagę deliveredByPlatform. Osobny interfejs, żeby nie
// rozszerzać Creator o metodę potrzebną jednej trasie.
type PlatformDeliverySetter interface {
	SetDeliveredByPlatform(ctx context.Context, courseID int64, delivered bool) (bool, error)
}

// GetPlatformDelivery zwraca flagę deliveredByPlatform kursu.
func (h *Handler) GetPlatformDelivery(w http.ResponseWriter, r *http.Request) {
	courseID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid course ID")
		return
	}
	course, err := h.queries.GetCourseByID(r.Context(), courseID)
	if err != nil {
		response.HandleDBError(w, err, "course")
		return
	}
	response.WriteJSON(w, http.StatusOK, PlatformDeliveryResponse{
		Data: PlatformDeliveryDTO{DeliveredByPlatform: course.DeliveredByPlatform},
	})
}

// PutPlatformDelivery ustawia flagę deliveredByPlatform kursu.
func (h *Handler) PutPlatformDelivery(w http.ResponseWriter, r *http.Request) {
	courseID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid course ID")
		return
	}
	setter, ok := h.creator.(PlatformDeliverySetter)
	if !ok {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to update course")
		return
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := PlatformDeliveryRequest{}
	if err := decoder.Decode(&req); err != nil || req.DeliveredByPlatform == nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	delivered, err := setter.SetDeliveredByPlatform(r.Context(), courseID, *req.DeliveredByPlatform)
	if err != nil {
		response.HandleDBError(w, err, "course")
		return
	}
	response.WriteJSON(w, http.StatusOK, PlatformDeliveryResponse{
		Data: PlatformDeliveryDTO{DeliveredByPlatform: delivered},
	})
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
		ID:                  row.ID,
		MainName:            row.Mainname.String,
		Name:                row.Name,
		Symbol:              row.Symbol,
		ExpiryTime:          expiryTime,
		DeliveredByPlatform: row.DeliveredByPlatform,
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
