package certificates

import (
	"context"
	"encoding/json"
	"errors"
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
	ListCourseCertificateTranslationsByCourseID(ctx context.Context, courseID int64) ([]sqlc.ListCourseCertificateTranslationsByCourseIDRow, error)
	GetCourseCertificateTranslationByCourseAndLanguage(ctx context.Context, arg sqlc.GetCourseCertificateTranslationByCourseAndLanguageParams) (sqlc.GetCourseCertificateTranslationByCourseAndLanguageRow, error)
	UpdateCertificate(ctx context.Context, arg sqlc.UpdateCertificateParams) (sqlc.UpdateCertificateRow, error)
	SoftDeleteCertificate(ctx context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error)
	ListCertificatesByCourseID(ctx context.Context, arg sqlc.ListCertificatesByCourseIDParams) ([]sqlc.ListCertificatesByCourseIDRow, error)
	CountCertificatesByCourseID(ctx context.Context, arg sqlc.CountCertificatesByCourseIDParams) (int64, error)
}
type Creator interface {
	Create(ctx context.Context, input CreateCertificateInput) (CreateCertificateResult, error)
	Update(ctx context.Context, certificateID int64, input UpdateCertificateInput) (sqlc.UpdateCertificateRow, error)
}

type Handler struct {
	querier Querier
	creator Creator
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

	certificate, err = h.resolveCertificatePDFVariant(r.Context(), certificate, r.URL.Query().Get("language"))
	if err != nil {
		if errors.Is(err, ErrCertificateTranslationNotFound) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "certificate translation not found")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to prepare certificate pdf")
		return
	}

	pdfBytes, err := renderCertificatePDF(r.Context(), buildCertificatePDFHTML(certificate))
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
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	certReq := CreateCertificateRequest{}
	err := decoder.Decode(&certReq)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	certID, err := h.creator.Create(r.Context(), mapCertificateRequest(certReq))
	if err != nil {
		if errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrInvalidRegistryDate) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate data")
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
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create certificate")
		return
	}
	response.WriteJSON(w, http.StatusCreated, CreateCertificateResponse{
		Data: CreateCertificateResponseData{
			ID: certID.ID,
		},
	})
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

	row, err := h.creator.Update(r.Context(), idInt, UpdateCertificateInput{
		StudentID:       req.StudentID,
		CertificateDate: req.CertificateDate,
		CourseDateStart: req.CourseDateStart,
		CourseDateEnd:   req.CourseDateEnd,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
			return
		}
		response.HandleDBError(w, err, "certificate")
		return
	}
	response.WriteJSON(w, http.StatusOK, CertificateResponse{
		Data: mapUpdateCertificateResponse(row),
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
	err = decoder.Decode(&req)
	if err != nil {
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

func optionalDate(value time.Time) pgtype.Date {
	if value.IsZero() {
		return pgtype.Date{}
	}

	return pgtype.Date{
		Time:  value,
		Valid: true,
	}
}

func mapCertificateRequest(cert CreateCertificateRequest) CreateCertificateInput {
	return CreateCertificateInput{
		StudentID:       cert.StudentID,
		CourseID:        cert.CourseID,
		CertificateDate: cert.CertificateDate,
		CourseDateStart: cert.CourseDateStart,
		CourseDateEnd:   cert.CourseDateEnd,
		RegistryYear:    cert.RegistryYear,
		RegistryNumber:  cert.RegistryNumber,
		LanguageCode:    cert.LanguageCode,
	}
}

func mapUpdateCertificateResponse(row sqlc.UpdateCertificateRow) CertificateDetailsDTO {
	certificate := sqlc.GetCertificateByIDRow(row)
	return mapCertificateDetailsResponse(certificate, []CertificatePrintVariantDTO{mapCertificatePrintVariantDTO(buildSnapshotPrintVariant(certificate))})
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

	var expiryDate *string
	expiryDate = pgutil.NullableString(certificate.ExpiryDate)

	return CertificateDetailsDTO{
		ID:                certificate.ID,
		Date:              certificate.Date.Time.Format(response.DateFormat),
		StudentID:         validation.SignedToInt64Clamped(certificate.StudentID),
		CourseID:          certificate.CourseID,
		StudentName:       certificate.StudentFirstname,
		StudentSecondname: certificate.StudentSecondname.String,
		StudentLastname:   certificate.StudentLastname,
		StudentBirthdate:  certificate.StudentBirthdate.Time.Format(response.DateFormat),
		StudentBirthplace: certificate.StudentBirthplace,
		StudentPesel:      certificate.StudentPesel.String,
		CompanyName:       certificate.CompanyName.String,
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
	return CertificatePrintVariantDTO{
		LanguageCode:  variant.LanguageCode,
		CourseName:    variant.CourseName,
		CourseProgram: variant.CourseProgram,
		CertFrontPage: variant.CertFrontPage,
		IsOriginal:    variant.IsOriginal,
	}
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
		ID:              row.ID,
		Date:            row.Date.Time.Format(response.DateFormat),
		StudentName:     studentName,
		CompanyName:     row.CompanyName.String,
		CourseName:      row.CourseName,
		CourseSymbol:    row.CourseSymbol,
		RegistryYear:    int(row.RegistryYear),
		RegistryNumber:  int(row.RegistryNumber),
		CourseDateStart: row.CourseDateStart.Time.Format(response.DateFormat),
		CourseDateEnd:   pgutil.NullableDate(row.CourseDateEnd),
		LanguageCode:    row.LanguageCode,
		ExpiryDate:      pgutil.NullableString(row.ExpiryDate),
	}
}
