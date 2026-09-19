package certificates

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
	"github.com/janexpl/CoursesListNext/api/internal/validation"
)

type Querier interface {
	ListCertificates(ctx context.Context, arg sqlc.ListCertificatesParams) ([]sqlc.ListCertificatesRow, error)
	GetCertificateByID(ctx context.Context, id int64) (sqlc.GetCertificateByIDRow, error)
	GetCourseByID(ctx context.Context, id int64) (sqlc.Course, error)
	GetCompanyByID(ctx context.Context, id int64) (sqlc.Company, error)
	ListCourseCertificateTranslationsByCourseID(ctx context.Context, courseID int64) ([]sqlc.ListCourseCertificateTranslationsByCourseIDRow, error)
	GetCourseCertificateTranslationByCourseAndLanguage(ctx context.Context, arg sqlc.GetCourseCertificateTranslationByCourseAndLanguageParams) (sqlc.GetCourseCertificateTranslationByCourseAndLanguageRow, error)
	UpdateCertificate(ctx context.Context, arg sqlc.UpdateCertificateParams) (sqlc.UpdateCertificateRow, error)
	SoftDeleteCertificate(ctx context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error)
	ListCertificatesByCourseID(ctx context.Context, arg sqlc.ListCertificatesByCourseIDParams) ([]sqlc.ListCertificatesByCourseIDRow, error)
	CountCertificatesByCourseID(ctx context.Context, arg sqlc.CountCertificatesByCourseIDParams) (int64, error)
	ListCertificatesByCompanyID(ctx context.Context, arg sqlc.ListCertificatesByCompanyIDParams) ([]sqlc.ListCertificatesByCompanyIDRow, error)
	CountCertificatesByCompanyID(ctx context.Context, arg sqlc.CountCertificatesByCompanyIDParams) (int64, error)
	ListExpiringCertificateNotificationCandidates(ctx context.Context, arg sqlc.ListExpiringCertificateNotificationCandidatesParams) ([]sqlc.ListExpiringCertificateNotificationCandidatesRow, error)
}
type Creator interface {
	Create(ctx context.Context, input CreateCertificateInput) (CreateCertificateResult, error)
	Update(ctx context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error)
}

type Handler struct {
	querier Querier
	creator Creator
	// verificationURLTemplate - wzorzec adresu publicznej weryfikacji, z którego powstaje
	// kod QR na wydruku. Pusty oznacza wydruk bez QR.
	verificationURLTemplate string
}

// SetVerificationURLTemplate wpina konfigurację adresu weryfikacji. Osobny setter,
// bo NewHandler jest wołane w kilkunastu testach i nie ma powodu ich ruszać.
func (h *Handler) SetVerificationURLTemplate(template string) {
	h.verificationURLTemplate = template
}

func NewHandler(querier Querier, creator Creator) *Handler {
	return &Handler{querier: querier, creator: creator}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pgSearch, limitInt, err := response.ParseListParams(r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	dateFrom, err := response.ParseDateQueryValue(r, "dateFrom")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateFrom value")
		return
	}
	dateTo, err := response.ParseDateQueryValue(r, "dateTo")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateTo value")
		return
	}
	if !dateFrom.IsZero() && !dateTo.IsZero() && dateFrom.After(dateTo) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "dateFrom cannot be after dateTo")
		return
	}
	rows, err := h.querier.ListCertificates(r.Context(), sqlc.ListCertificatesParams{
		Search:     pgSearch,
		DateFrom:   optionalDate(dateFrom),
		DateTo:     optionalDate(dateTo),
		LimitCount: limitInt,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list certificates")
		return
	}

	resp := ListCertificatesResponse{Data: make([]CertificateDTO, 0, len(rows))}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapCertificatesResponse(row))
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate id")
		return
	}

	certificate, err := h.querier.GetCertificateByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}

	resp := CertificateResponse{Data: mapCertificateDetailsResponse(certificate, h.loadCertificatePrintVariants(r.Context(), certificate))}
	response.WriteJSON(w, http.StatusOK, resp)
}

// certificateLifecycle obsługuje unieważnienie i duplikat. Osobny interfejs, żeby nie
// rozszerzać Creator o metody potrzebne tylko tym trasom.
type certificateLifecycle interface {
	Revoke(ctx context.Context, certificateID int64, reason string) error
	Duplicate(ctx context.Context, certificateID int64, reason string) error
}

// decodeLifecycleRequest czyta {"reason": "..."}; powód jest wymagany i niepusty.
func decodeLifecycleRequest(w http.ResponseWriter, r *http.Request) (int64, string, certificateLifecycle, bool) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate ID")
		return 0, "", nil, false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := LifecycleRequest{}
	if err := decoder.Decode(&req); err != nil || req.Reason == nil || strings.TrimSpace(*req.Reason) == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return 0, "", nil, false
	}
	return id, *req.Reason, nil, true
}

func (h *Handler) writeLifecycleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
	case errors.Is(err, ErrInvalidRegistryDate):
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate data")
	case errors.Is(err, ErrCertificateNotFound):
		response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "certificate not found")
	case errors.Is(err, ErrCertificateAlreadyRevoked):
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "certificate already revoked")
	case errors.Is(err, ErrCertificateRevoked):
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "certificate is revoked")
	default:
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to update certificate")
	}
}

func (h *Handler) writeCertificateDetails(w http.ResponseWriter, r *http.Request, status int, id int64) {
	certificate, err := h.querier.GetCertificateByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}
	response.WriteJSON(w, status, CertificateResponse{
		Data: mapCertificateDetailsResponse(certificate, h.loadCertificatePrintVariants(r.Context(), certificate)),
	})
}

// Revoke unieważnia zaświadczenie i zwraca je po zmianie.
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	lifecycle, ok := h.creator.(certificateLifecycle)
	if !ok {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to update certificate")
		return
	}
	id, reason, _, ok := decodeLifecycleRequest(w, r)
	if !ok {
		return
	}
	if err := lifecycle.Revoke(r.Context(), id, reason); err != nil {
		h.writeLifecycleError(w, err)
		return
	}
	h.writeCertificateDetails(w, r, http.StatusOK, id)
}

// Duplicate odnotowuje wystawienie duplikatu na istniejącym zaświadczeniu i zwraca je
// po zmianie. Duplikat nie jest nowym dokumentem - to ten sam numer z adnotacją na wydruku.
func (h *Handler) Duplicate(w http.ResponseWriter, r *http.Request) {
	lifecycle, ok := h.creator.(certificateLifecycle)
	if !ok {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to update certificate")
		return
	}
	id, reason, _, ok := decodeLifecycleRequest(w, r)
	if !ok {
		return
	}
	if err := lifecycle.Duplicate(r.Context(), id, reason); err != nil {
		h.writeLifecycleError(w, err)
		return
	}
	h.writeCertificateDetails(w, r, http.StatusOK, id)
}

// verificationCodeFinder to osobny interfejs, żeby nie rozszerzać Querier o metodę
// potrzebną tylko jednej trasie.
type verificationCodeFinder interface {
	GetCertificateIDByVerificationCode(ctx context.Context, verificationCode string) (int64, error)
}

// GetByVerificationCode zwraca to samo co GET /certificates/{id} dla zaświadczenia
// o podanym kodzie. Kod jest normalizowany do wielkich liter (człowiek przepisuje go
// z papieru); usunięte zaświadczenie daje 404 jak w GET po id.
func (h *Handler) GetByVerificationCode(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	if !validation.IsCertificateVerificationCode(code) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid verification code")
		return
	}
	finder, ok := h.querier.(verificationCodeFinder)
	if !ok {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to get certificate")
		return
	}
	id, err := finder.GetCertificateIDByVerificationCode(r.Context(), code)
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}
	certificate, err := h.querier.GetCertificateByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}
	response.WriteJSON(w, http.StatusOK, CertificateResponse{
		Data: mapCertificateDetailsResponse(certificate, h.loadCertificatePrintVariants(r.Context(), certificate)),
	})
}

func (h *Handler) PDF(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate id")
		return
	}

	certificate, err := h.querier.GetCertificateByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}

	// Decyzja: unieważniony dokument nie ma wydruku (409), żeby nie krążył PDF
	// wyglądający na ważny. Dane pozostają dostępne w GET /certificates/{id}.
	if certificate.RevokedAt.Valid {
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "certificate is revoked")
		return
	}

	certificate, err = h.resolveCertificatePDFVariant(r.Context(), certificate, r.URL.Query().Get("language"))
	if err != nil {
		if errors.Is(err, ErrCertificateTranslationNotFound) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "certificate translation not found")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to prepare certificate pdf")
		return
	}

	pdfBytes, err := renderCertificatePDF(r.Context(), buildCertificatePDFHTML(certificate, h.verificationURLTemplate))
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to render certificate pdf")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+buildCertificateFilename(certificate)+`"`)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(pdfBytes)
	if err != nil {
		log.Printf("failed to write PDF responsee %v", err)
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	idempotencyKey, err := parseIdempotencyKey(r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid Idempotency-Key header")
		return
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	certReq := CreateCertificateRequest{}
	err = decoder.Decode(&certReq)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	input, err := mapCertificateRequest(certReq)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate data")
		return
	}
	input.IdempotencyKey = idempotencyKey

	result, err := h.creator.Create(r.Context(), input)
	if err != nil {
		if errors.Is(err, ErrIdempotencyKeyReused) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "idempotency key reused with different payload")
			return
		}
		if errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrInvalidRegistryDate) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate data")
			return
		}
		if errors.Is(err, ErrCertificateDateBeforeCourseEnd) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "certificate date cannot be before course end date")
			return
		}
		if errors.Is(err, ErrCertificateTranslationNotFound) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "certificate translation not found")
			return
		}
		if errors.Is(err, ErrRegistryNumberTaken) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "registry number already taken for the given year")
			return
		}
		if errors.Is(err, ErrStudentNotFound) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "student not found")
			return
		}
		if errors.Is(err, ErrCourseNotFound) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "course not found")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create certificate")
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		// Ponowienie z tym samym kluczem i ciałem: to samo ciało co pierwotne 201.
		status = http.StatusOK
	}
	response.WriteJSON(w, status, CreateCertificateResponse{
		Data: CreateCertificateResponseData{
			ID:               result.ID,
			RegistryYear:     result.RegistryYear,
			RegistryNumber:   result.RegistryNumber,
			VerificationCode: result.VerificationCode,
		},
	})
}

const maxIdempotencyKeyLength = 255

var errInvalidIdempotencyKey = errors.New("invalid idempotency key")

// parseIdempotencyKey zwraca pusty klucz, gdy nagłówka nie ma. Obecny nagłówek musi
// być pojedynczy, niepusty, do 255 znaków i złożony z widocznych znaków ASCII -
// klucz porównywany jest bajt w bajt, więc spacje czy znaki narodowe byłyby
// źródłem trudnych do wykrycia rozbieżności między ponowieniami.
func parseIdempotencyKey(r *http.Request) (string, error) {
	values, present := r.Header[http.CanonicalHeaderKey("Idempotency-Key")]
	if !present {
		return "", nil
	}
	if len(values) != 1 {
		return "", errInvalidIdempotencyKey
	}
	key := values[0]
	if key == "" || len(key) > maxIdempotencyKeyLength {
		return "", errInvalidIdempotencyKey
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 0x21 || key[i] > 0x7e {
			return "", errInvalidIdempotencyKey
		}
	}
	return key, nil
}

func (h *Handler) ListExpiringNotificationCandidates(w http.ResponseWriter, r *http.Request) {
	dateFrom, err := response.ParseDateQueryValue(r, "dateFrom")
	if err != nil || dateFrom.IsZero() {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateFrom value")
		return
	}

	dateTo, err := response.ParseDateQueryValue(r, "dateTo")
	if err != nil || dateTo.IsZero() {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateTo value")
		return
	}

	if dateFrom.After(dateTo) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "dateFrom cannot be after dateTo")
		return
	}

	limit, err := response.ParsePositiveInt32QueryValue(r, "limit", 500)
	if err != nil || limit > 1000 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid limit value")
		return
	}

	afterExpiryDate, err := response.ParseDateQueryValue(r, "afterExpiryDate")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid afterExpiryDate value")
		return
	}

	afterCertificateID := pgtype.Int8{}
	afterCertificateIDRaw := strings.TrimSpace(r.URL.Query().Get("afterCertificateId"))
	if !afterExpiryDate.IsZero() {
		parsedID, err := strconv.ParseInt(afterCertificateIDRaw, 10, 64)
		if err != nil || parsedID <= 0 {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid afterCertificateId value")
			return
		}
		afterCertificateID = pgtype.Int8{Int64: parsedID, Valid: true}
	} else if afterCertificateIDRaw != "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "afterExpiryDate is required with afterCertificateId")
		return
	}

	rows, err := h.querier.ListExpiringCertificateNotificationCandidates(r.Context(), sqlc.ListExpiringCertificateNotificationCandidatesParams{
		DateFrom:           optionalDate(dateFrom),
		DateTo:             optionalDate(dateTo),
		AfterExpiryDate:    optionalDate(afterExpiryDate),
		AfterCertificateID: afterCertificateID,
		LimitCount:         limit + 1,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list expiring certificate notification candidates")
		return
	}

	hasMore := len(rows) > int(limit)
	if hasMore {
		rows = rows[:limit]
	}

	resp := ListExpiringCertificateNotificationCandidatesResponse{
		Data: make([]ExpiringCertificateNotificationCandidateDTO, 0, len(rows)),
		Meta: ExpiringCertificateNotificationCandidatesMetaDTO{
			Limit:   limit,
			HasMore: hasMore,
		},
	}

	for _, row := range rows {
		resp.Data = append(resp.Data, mapExpiringNotificationCandidate(row))
	}

	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		resp.Meta.NextCursor = &ExpiringCertificateNotificationCandidatesCursorDTO{
			AfterExpiryDate:    last.ExpiryDate.Time.Format(response.DateFormat),
			AfterCertificateID: last.CertificateID,
		}
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	idInt, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate ID")
		return
	}
	req := UpdateCertificateRequest{}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.creator.Update(r.Context(), idInt, UpdateCertificateInput(req))
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
			return
		}
		if errors.Is(err, ErrCertificateDateBeforeCourseEnd) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "certificate date cannot be before course end date")
			return
		}
		if errors.Is(err, ErrStudentNotFound) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "student not found")
			return
		}
		if errors.Is(err, ErrCertificateRevoked) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "certificate is revoked")
			return
		}
		response.HandleDBError(w, err, "certificate")
		return
	}
	certificate := sqlc.GetCertificateByIDRow(row)
	response.WriteJSON(w, http.StatusOK, CertificateResponse{
		Data: mapCertificateDetailsResponse(certificate, h.loadCertificatePrintVariants(r.Context(), certificate)),
	})
}

func (h *Handler) SoftDeleteCertificate(w http.ResponseWriter, r *http.Request) {
	idInt, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate ID")
		return
	}
	req := SoftDeleteCertificateRequest{}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	// Ciało jest opcjonalne - niesie tylko powód usunięcia. Klienci HTTP zwykle nie
	// wysyłają ciała z DELETE, więc jego brak (io.EOF) to po prostu brak powodu.
	if err = decoder.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.querier.SoftDeleteCertificate(r.Context(), sqlc.SoftDeleteCertificateParams{
		ID:              idInt,
		DeletedByUserID: pgutil.OptionalInt8(&user.ID),
		DeleteReason:    pgutil.OptionalText(req.DeleteReason),
	})
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}
	response.WriteJSON(w, http.StatusOK, DeleteCertificateResponse{
		Data: DeleteCertificateDTO{ID: result},
	})
}

func (h *Handler) ListByCourseID(w http.ResponseWriter, r *http.Request) {
	dateFrom, err := response.ParseDateQueryValue(r, "dateFrom")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateFrom value")
		return
	}
	dateTo, err := response.ParseDateQueryValue(r, "dateTo")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateTo value")
		return
	}
	if !dateFrom.IsZero() && !dateTo.IsZero() && dateFrom.After(dateTo) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "dateFrom cannot be after dateTo")
		return
	}
	page, err := response.ParsePositiveInt32QueryValue(r, "page", 1)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid page value")
		return
	}
	limit, err := response.ParsePositiveInt32QueryValue(r, "limit", 10)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid limit value")
		return
	}
	if limit > 100 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid limit value")
		return
	}

	courseID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid course ID")
		return
	}
	countParams := sqlc.CountCertificatesByCourseIDParams{
		CourseID: courseID,
		DateFrom: optionalDate(dateFrom),
		DateTo:   optionalDate(dateTo),
	}
	count, err := h.querier.CountCertificatesByCourseID(r.Context(), countParams)
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}
	// Zero wyników może oznaczać kurs bez zaświadczeń albo nieistniejący kurs.
	if count == 0 {
		if _, err := h.querier.GetCourseByID(r.Context(), courseID); err != nil {
			response.HandleDBError(w, err, "course")
			return
		}
	}
	offset := (page - 1) * limit
	totalPages := int(math.Ceil(float64(count) / float64(limit)))
	rows, err := h.querier.ListCertificatesByCourseID(r.Context(), sqlc.ListCertificatesByCourseIDParams{
		CourseID:    courseID,
		DateFrom:    countParams.DateFrom,
		DateTo:      countParams.DateTo,
		OffsetCount: offset,
		LimitCount:  limit,
	})
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}

	resp := ListCertificatesByCourseResponse{Data: make([]CertificateDTO, 0, len(rows))}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapCertificatesResponse(sqlc.ListCertificatesRow(row)))
	}

	resp.Pagination = PaginationDTO{
		Page:       page,
		Limit:      limit,
		Total:      count,
		TotalPages: int32(totalPages),
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListByCompanyID(w http.ResponseWriter, r *http.Request) {
	dateFrom, err := response.ParseDateQueryValue(r, "dateFrom")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateFrom value")
		return
	}
	dateTo, err := response.ParseDateQueryValue(r, "dateTo")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateTo value")
		return
	}
	if !dateFrom.IsZero() && !dateTo.IsZero() && dateFrom.After(dateTo) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "dateFrom cannot be after dateTo")
		return
	}
	page, err := response.ParsePositiveInt32QueryValue(r, "page", 1)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid page value")
		return
	}
	limit, err := response.ParsePositiveInt32QueryValue(r, "limit", 10)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid limit value")
		return
	}
	if limit > 100 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid limit value")
		return
	}

	companyID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid company ID")
		return
	}
	countParams := sqlc.CountCertificatesByCompanyIDParams{
		CompanyID: pgtype.Int8{
			Int64: companyID,
			Valid: true,
		},
		DateFrom: optionalDate(dateFrom),
		DateTo:   optionalDate(dateTo),
	}
	count, err := h.querier.CountCertificatesByCompanyID(r.Context(), countParams)
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}
	if count == 0 {
		if _, err := h.querier.GetCompanyByID(r.Context(), companyID); err != nil {
			response.HandleDBError(w, err, "company")
			return
		}
	}
	offset := (page - 1) * limit
	totalPages := int(math.Ceil(float64(count) / float64(limit)))
	rows, err := h.querier.ListCertificatesByCompanyID(r.Context(), sqlc.ListCertificatesByCompanyIDParams{
		CompanyID:   countParams.CompanyID,
		DateFrom:    countParams.DateFrom,
		DateTo:      countParams.DateTo,
		OffsetCount: offset,
		LimitCount:  limit,
	})
	if err != nil {
		response.HandleDBError(w, err, "certificate")
		return
	}

	resp := ListCertificatesByCompanyResponse{Data: make([]CertificateDTO, 0, len(rows))}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapCertificatesResponse(sqlc.ListCertificatesRow(row)))
	}

	resp.Pagination = PaginationDTO{
		Page:       page,
		Limit:      limit,
		Total:      count,
		TotalPages: int32(totalPages),
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func optionalDate(value time.Time) pgtype.Date {
	if value.IsZero() {
		return pgtype.Date{}
	}

	return pgtype.Date{
		Time:  value,
		Valid: true,
	}
}

// mapCertificateRequest rozstrzyga, czy numer rejestru podał klient, czy nada go serwer.
// Jawne 0 lub wartość ujemna pozostają błędem jak dotąd; numer bez roku jest
// niejednoznaczny i też jest odrzucany.
func mapCertificateRequest(cert CreateCertificateRequest) (CreateCertificateInput, error) {
	input := CreateCertificateInput{
		StudentID:       cert.StudentID,
		CourseID:        cert.CourseID,
		CertificateDate: cert.CertificateDate,
		CourseDateStart: cert.CourseDateStart,
		CourseDateEnd:   cert.CourseDateEnd,
		LanguageCode:    cert.LanguageCode,
	}
	if cert.RegistryYear != nil {
		if *cert.RegistryYear <= 0 {
			return CreateCertificateInput{}, ErrInvalidInput
		}
		input.RegistryYear = *cert.RegistryYear
	}
	if cert.RegistryNumber == nil {
		input.AssignRegistryNumber = true
		return input, nil
	}
	if cert.RegistryYear == nil {
		return CreateCertificateInput{}, ErrInvalidInput
	}
	input.RegistryNumber = *cert.RegistryNumber
	return input, nil
}

func mapCertificateDetailsResponse(certificate sqlc.GetCertificateByIDRow, printVariants []CertificatePrintVariantDTO) CertificateDetailsDTO {
	var journal *CertificateJournalRefDTO
	if certificate.JournalID.Valid {
		journal = &CertificateJournalRefDTO{
			ID:     certificate.JournalID.Int64,
			Title:  certificate.JournalTitle.String,
			Status: certificate.JournalStatus.String,
		}
	}

	var courseExpiryTime *int
	if certificate.CourseExpiryTime.Valid {
		if value, err := strconv.Atoi(certificate.CourseExpiryTime.String); err == nil {
			courseExpiryTime = &value
		}
	}

	expiryDate := pgutil.NullableString(certificate.ExpiryDate)

	return CertificateDetailsDTO{
		ID:                certificate.ID,
		Date:              certificate.Date.Time.Format(response.DateFormat),
		StudentID:         validation.SignedToInt64Clamped(certificate.StudentID),
		CourseID:          certificate.CourseID,
		StudentFirstname:  certificate.StudentFirstname,
		StudentSecondname: pgutil.NullableString(certificate.StudentSecondname),
		StudentLastname:   certificate.StudentLastname,
		StudentBirthdate:  certificate.StudentBirthdate.Time.Format(response.DateFormat),
		StudentBirthplace: certificate.StudentBirthplace,
		StudentPesel:      pgutil.NullableString(certificate.StudentPesel),
		CompanyName:       pgutil.NullableString(certificate.CompanyName),
		CourseDateStart:   certificate.CourseDateStart.Time.Format(response.DateFormat),
		CourseDateEnd:     pgutil.NullableDate(certificate.CourseDateEnd),
		RegistryYear:      int(certificate.RegistryYear),
		RegistryNumber:    int(certificate.RegistryNumber),
		CourseName:        certificate.CourseName,
		CourseSymbol:      certificate.CourseSymbol,
		CourseExpiryTime:  courseExpiryTime,
		CourseProgram:     certificate.CourseProgram,
		CertFrontPage:     certificate.CertFrontPage,
		LanguageCode:      certificate.LanguageCode,
		ExpiryDate:        expiryDate,
		VerificationCode:  certificate.VerificationCode,
		RevokedAt:         pgutil.NullableTimestampz(certificate.RevokedAt),
		RevokeReason:      pgutil.NullableString(certificate.RevokeReason),
		DuplicateIssuedAt: pgutil.NullableTimestampz(certificate.DuplicateIssuedAt),
		DuplicateReason:   pgutil.NullableString(certificate.DuplicateReason),
		Journal:           journal,
		PrintVariants:     printVariants,
	}
}

type certificatePrintVariant struct {
	LanguageCode  string
	CourseName    string
	CourseProgram string
	CertFrontPage string
	IsOriginal    bool
}

func (h *Handler) loadCertificatePrintVariants(ctx context.Context, certificate sqlc.GetCertificateByIDRow) []CertificatePrintVariantDTO {
	fallback := []CertificatePrintVariantDTO{mapCertificatePrintVariantDTO(buildSnapshotPrintVariant(certificate))}

	course, err := h.querier.GetCourseByID(ctx, certificate.CourseID)
	if err != nil {
		log.Printf("failed to load course %d for certificate %d print variants: %v", certificate.CourseID, certificate.ID, err)
		return fallback
	}

	translations, err := h.querier.ListCourseCertificateTranslationsByCourseID(ctx, certificate.CourseID)
	if err != nil {
		log.Printf("failed to load translations for course %d and certificate %d: %v", certificate.CourseID, certificate.ID, err)
		return fallback
	}

	return buildCertificatePrintVariantDTOs(certificate, course, translations)
}

func (h *Handler) resolveCertificatePDFVariant(ctx context.Context, certificate sqlc.GetCertificateByIDRow, requestedLanguage string) (sqlc.GetCertificateByIDRow, error) {
	languageCode := normalizeCertificatePrintLanguage(requestedLanguage)
	if languageCode == "" || languageCode == certificate.LanguageCode {
		return certificate, nil
	}

	if languageCode == "pl" {
		course, err := h.querier.GetCourseByID(ctx, certificate.CourseID)
		if err != nil {
			return sqlc.GetCertificateByIDRow{}, err
		}

		return applyCertificatePrintVariant(certificate, certificatePrintVariant{
			LanguageCode:  "pl",
			CourseName:    course.Name,
			CourseProgram: string(course.Courseprogram),
			CertFrontPage: course.Certfrontpage.String,
			IsOriginal:    false,
		}), nil
	}

	translation, err := h.querier.GetCourseCertificateTranslationByCourseAndLanguage(ctx, sqlc.GetCourseCertificateTranslationByCourseAndLanguageParams{
		CourseID:     certificate.CourseID,
		LanguageCode: languageCode,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetCertificateByIDRow{}, ErrCertificateTranslationNotFound
		}
		return sqlc.GetCertificateByIDRow{}, err
	}

	return applyCertificatePrintVariant(certificate, certificatePrintVariant{
		LanguageCode:  translation.LanguageCode,
		CourseName:    translation.CourseName,
		CourseProgram: translation.CourseProgram,
		CertFrontPage: translation.CertFrontPage,
		IsOriginal:    false,
	}), nil
}

func buildSnapshotPrintVariant(certificate sqlc.GetCertificateByIDRow) certificatePrintVariant {
	return certificatePrintVariant{
		LanguageCode:  certificate.LanguageCode,
		CourseName:    certificate.CourseName,
		CourseProgram: certificate.CourseProgram,
		CertFrontPage: certificate.CertFrontPage,
		IsOriginal:    true,
	}
}

func buildCertificatePrintVariantDTOs(
	certificate sqlc.GetCertificateByIDRow,
	course sqlc.Course,
	translations []sqlc.ListCourseCertificateTranslationsByCourseIDRow,
) []CertificatePrintVariantDTO {
	variants := []CertificatePrintVariantDTO{mapCertificatePrintVariantDTO(buildSnapshotPrintVariant(certificate))}
	seen := map[string]struct{}{certificate.LanguageCode: {}}

	if certificate.LanguageCode != "pl" {
		variants = append(variants, mapCertificatePrintVariantDTO(certificatePrintVariant{
			LanguageCode:  "pl",
			CourseName:    course.Name,
			CourseProgram: string(course.Courseprogram),
			CertFrontPage: course.Certfrontpage.String,
			IsOriginal:    false,
		}))
		seen["pl"] = struct{}{}
	}

	for _, translation := range translations {
		if _, exists := seen[translation.LanguageCode]; exists {
			continue
		}

		variants = append(variants, mapCertificatePrintVariantDTO(certificatePrintVariant{
			LanguageCode:  translation.LanguageCode,
			CourseName:    translation.CourseName,
			CourseProgram: translation.CourseProgram,
			CertFrontPage: translation.CertFrontPage,
			IsOriginal:    false,
		}))
		seen[translation.LanguageCode] = struct{}{}
	}

	return variants
}

func mapCertificatePrintVariantDTO(variant certificatePrintVariant) CertificatePrintVariantDTO {
	return CertificatePrintVariantDTO(variant)
}

func applyCertificatePrintVariant(certificate sqlc.GetCertificateByIDRow, variant certificatePrintVariant) sqlc.GetCertificateByIDRow {
	certificate.LanguageCode = variant.LanguageCode
	certificate.CourseName = variant.CourseName
	certificate.CourseProgram = variant.CourseProgram
	certificate.CertFrontPage = variant.CertFrontPage
	return certificate
}

func normalizeCertificatePrintLanguage(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mapCertificatesResponse(row sqlc.ListCertificatesRow) CertificateDTO {
	studentName := row.StudentFirstname + " " + row.StudentLastname

	return CertificateDTO{
		ID:                row.ID,
		Date:              row.Date.Time.Format(response.DateFormat),
		StudentName:       studentName,
		CompanyName:       row.CompanyName.String,
		CourseName:        row.CourseName,
		CourseSymbol:      row.CourseSymbol,
		RegistryYear:      int(row.RegistryYear),
		RegistryNumber:    int(row.RegistryNumber),
		CourseDateStart:   row.CourseDateStart.Time.Format(response.DateFormat),
		CourseDateEnd:     pgutil.NullableDate(row.CourseDateEnd),
		LanguageCode:      row.LanguageCode,
		ExpiryDate:        pgutil.NullableString(row.ExpiryDate),
		RevokedAt:         pgutil.NullableTimestampz(row.RevokedAt),
		DuplicateIssuedAt: pgutil.NullableTimestampz(row.DuplicateIssuedAt),
	}
}

func mapExpiringNotificationCandidate(row sqlc.ListExpiringCertificateNotificationCandidatesRow) ExpiringCertificateNotificationCandidateDTO {
	companyName := row.CompanyNameSnapshot.String
	if !row.CompanyNameSnapshot.Valid || strings.TrimSpace(companyName) == "" {
		companyName = row.CompanyCurrentName
	}

	return ExpiringCertificateNotificationCandidateDTO{
		CertificateID:   row.CertificateID,
		CertificateDate: row.CertificateDate.Time.Format(response.DateFormat),
		ExpiryDate:      row.ExpiryDate.Time.Format(response.DateFormat),
		RegistryYear:    row.RegistryYear,
		RegistryNumber:  row.RegistryNumber,
		LanguageCode:    row.LanguageCode,
		Student: ExpiringCertificateNotificationStudentDTO{
			ID:        row.StudentID,
			FirstName: row.StudentFirstnameSnapshot,
			LastName:  row.StudentLastnameSnapshot,
			PESEL:     pgutil.NullableString(row.StudentPeselSnapshot),
		},
		Company: ExpiringCertificateNotificationCompanyDTO{
			ID:             row.CompanyID,
			Name:           companyName,
			CurrentName:    row.CompanyCurrentName,
			RecipientEmail: row.RecipientEmail,
		},
		Course: ExpiringCertificateNotificationCourseDTO{
			Name:      row.CourseNameSnapshot,
			Symbol:    row.CourseSymbolSnapshot,
			DateStart: row.CourseDateStart.Time.Format(response.DateFormat),
			DateEnd:   pgutil.NullableDate(row.CourseDateEnd),
		},
	}
}
