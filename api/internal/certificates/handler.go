package certificates

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	ListCertificates(ctx context.Context, arg sqlc.ListCertificatesParams) ([]sqlc.ListCertificatesRow, error)
	GetCertificateByID(ctx context.Context, id int64) (sqlc.GetCertificateByIDRow, error)
	UpdateCertificate(ctx context.Context, arg sqlc.UpdateCertificateParams) (sqlc.UpdateCertificateRow, error)
	SoftDeleteCertificate(ctx context.Context, arg sqlc.SoftDeleteCertificateParams) (int64, error)
}
type Creator interface {
	Create(ctx context.Context, input CreateCertificateInput) (CreateCertificateResult, error)
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
	rows, err := h.querier.ListCertificates(r.Context(), sqlc.ListCertificatesParams{
		Search:     pgSearch,
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

	resp := CertificateResponse{Data: mapCertificateDetailsResponse(certificate)}
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

	certId, err := h.creator.Create(r.Context(), mapCertificateRequest(certReq))
	if err != nil {
		if errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrInvalidRegistryDate) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid certificate data")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create certificate")
		return
	}
	response.WriteJSON(w, http.StatusCreated, CreateCertificateResponse{
		Data: CreateCertificateResponseData{
			ID: certId.ID,
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
	if req.StudentID <= 0 || req.StudentID > math.MaxInt32 || req.CertificateDate == "" || req.CourseDateStart == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	pgDate, err := parseDate(req.CertificateDate)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	pgCourseDateStart, err := parseDate(req.CourseDateStart)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	pgCourseDateEnd, err := parseOptionalDate(req.CourseDateEnd)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if pgCourseDateEnd.Valid && pgCourseDateEnd.Time.Before(pgCourseDateStart.Time) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.querier.UpdateCertificate(r.Context(), sqlc.UpdateCertificateParams{
		Date:            pgDate,
		StudentID:       int32(req.StudentID),
		CourseDateStart: pgCourseDateStart,
		CourseDateEnd:   pgCourseDateEnd,
		CertificateID:   idInt,
	})

	if err != nil {
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

func mapCertificateRequest(cert CreateCertificateRequest) CreateCertificateInput {
	return CreateCertificateInput{
		StudentID:       cert.StudentID,
		CourseID:        cert.CourseID,
		CertificateDate: cert.CertificateDate,
		CourseDateStart: cert.CourseDateStart,
		CourseDateEnd:   cert.CourseDateEnd,
		RegistryYear:    cert.RegistryYear,
		RegistryNumber:  cert.RegistryNumber,
	}
}

func mapUpdateCertificateResponse(row sqlc.UpdateCertificateRow) CertificateDetailsDTO {
	return mapCertificateDetailsResponse(sqlc.GetCertificateByIDRow(row))
}

func mapCertificateDetailsResponse(certificate sqlc.GetCertificateByIDRow) CertificateDetailsDTO {
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
		StudentID:         certificate.StudentID,
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
		CertFrontPage:     certificate.CertFrontPage.String,
		ExpiryDate:        expiryDate,
		Journal:           journal,
	}
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
		ExpiryDate:      pgutil.NullableString(row.ExpiryDate),
	}
}
