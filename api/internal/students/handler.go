package students

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	GetStudentByID(ctx context.Context, id int64) (dbsqlc.GetStudentByIDRow, error)
	ListStudents(ctx context.Context, arg dbsqlc.ListStudentsParams) ([]dbsqlc.ListStudentsRow, error)
	ListCertificatesByStudentID(ctx context.Context, studentID int32) ([]dbsqlc.ListCertificatesByStudentIDRow, error)
	ListStudentsByCompanyID(ctx context.Context, companyID pgtype.Int8) ([]dbsqlc.ListStudentsByCompanyIDRow, error)
	GetCompanyByID(ctx context.Context, id int64) (dbsqlc.Company, error)
}

type Creator interface {
	Create(ctx context.Context, req CreateStudentRequest) (StudentDetailsDTO, error)
	Update(ctx context.Context, studentID int64, req UpdateStudentRequest) (StudentDetailsDTO, error)
}

// ExternalIDUpserter obsługuje PUT /students/by-external-id/{externalId}. Osobny
// interfejs, żeby nie rozszerzać Creator o metodę, której nie potrzebuje reszta tras.
type ExternalIDUpserter interface {
	UpsertByExternalID(ctx context.Context, externalID string, req CreateStudentRequest) (StudentDetailsDTO, bool, error)
}

type Handler struct {
	querier Querier
	creator Creator
}

func NewHandler(querier Querier, creators ...Creator) *Handler {
	var creator Creator
	if len(creators) > 0 {
		creator = creators[0]
	}

	return &Handler{
		querier: querier,
		creator: creator,
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid student id")
		return
	}

	student, err := h.querier.GetStudentByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "student")
		return
	}

	resp := StudentDetailsResponse{Data: mapStudentGetRow(student)}
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pgSearch, limitInt, err := response.ParseListParams(r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	companyID := strings.TrimSpace(r.URL.Query().Get("companyId"))
	cIDint := pgtype.Int8{}
	if companyID != "" {
		cid, err := strconv.Atoi(companyID)
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "failed to convert company id")
			return
		}
		if cid <= 0 {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "incorrect company id")
			return
		}
		cIDint = pgtype.Int8{
			Int64: int64(cid),
			Valid: true,
		}
	}

	rows, err := h.querier.ListStudents(r.Context(), dbsqlc.ListStudentsParams{
		Search:     pgSearch,
		CompanyID:  cIDint,
		LimitCount: limitInt,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list students")
		return
	}

	resp := ListStudentsResponse{
		Data: make([]StudentDTO, 0, len(rows)),
	}

	for _, row := range rows {
		resp.Data = append(resp.Data, mapStudentRow(row))
	}
	response.WriteJSON(w, http.StatusOK, resp)

}

func (h *Handler) ListCertificatesByStudent(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid student id")
		return
	}
	if id <= 0 || id > math.MaxInt32 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid student id")
		return
	}

	rows, err := h.querier.ListCertificatesByStudentID(r.Context(), int32(id))
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list certificates for student")
		return
	}
	// Pusta lista może oznaczać kursanta bez zaświadczeń albo nieistniejącego kursanta -
	// tylko w tym przypadku dopytujemy o kursanta, żeby odróżnić 200 od 404.
	if len(rows) == 0 {
		if _, err := h.querier.GetStudentByID(r.Context(), id); err != nil {
			response.HandleDBError(w, err, "student")
			return
		}
	}
	resp := ListCertificatesByStudentResponse{
		Data: make([]CertificateByStudentDTO, 0, len(rows)),
	}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapCertByStudentsRow(row))
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListStudentsByCompanyId(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid company id")
		return
	}
	idPgType := pgtype.Int8{
		Int64: id,
		Valid: true,
	}

	rows, err := h.querier.ListStudentsByCompanyID(r.Context(), idPgType)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list students")
		return
	}
	if len(rows) == 0 {
		if _, err := h.querier.GetCompanyByID(r.Context(), id); err != nil {
			response.HandleDBError(w, err, "company")
			return
		}
	}

	resp := ListStudentsByCompanyIdResult{
		Data: make([]ListStudentsByCompanyIdDTO, 0, len(rows)),
	}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapStudentsByCompanyRow(row))
	}
	response.WriteJSON(w, http.StatusOK, resp)

}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	idInt, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid student ID")
		return
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := UpdateStudentRequest{}
	if err = decoder.Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	firstName := strings.TrimSpace(req.FirstName)
	lastName := strings.TrimSpace(req.LastName)
	birthDate := strings.TrimSpace(req.BirthDate)
	birthPlace := strings.TrimSpace(req.BirthPlace)

	if firstName == "" || lastName == "" || birthDate == "" || birthPlace == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if req.CompanyID != nil && *req.CompanyID <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if _, err := time.Parse(response.DateFormat, birthDate); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	row, err := h.creator.Update(r.Context(), idInt, UpdateStudentRequest{
		studentPayload: studentPayload{
			FirstName:     firstName,
			LastName:      lastName,
			SecondName:    req.SecondName,
			BirthDate:     birthDate,
			BirthPlace:    birthPlace,
			Pesel:         req.Pesel,
			AddressStreet: req.AddressStreet,
			AddressCity:   req.AddressCity,
			AddressZip:    req.AddressZip,
			Telephone:     req.Telephone,
			CompanyID:     req.CompanyID,
		},
	})
	if err != nil {
		if errors.Is(err, ErrCompanyNotFound) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "company not found")
			return
		}
		if errors.Is(err, ErrDuplicateStudent) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "student with the same name and birth date already exists")
			return
		}
		response.HandleDBError(w, err, "student")
		return
	}
	resp := StudentDetailsResponse{
		Data: row,
	}
	response.WriteJSON(w, http.StatusOK, resp)

}

func (h *Handler) CreateStudent(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := CreateStudentRequest{}
	err := decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	firstName := strings.TrimSpace(req.FirstName)
	lastName := strings.TrimSpace(req.LastName)
	birthDate := strings.TrimSpace(req.BirthDate)
	birthPlace := strings.TrimSpace(req.BirthPlace)

	if firstName == "" || lastName == "" || birthDate == "" || birthPlace == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if req.CompanyID != nil && *req.CompanyID <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	if _, err := time.Parse(response.DateFormat, birthDate); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.creator.Create(r.Context(), CreateStudentRequest{
		studentPayload: studentPayload{
			FirstName:     firstName,
			LastName:      lastName,
			SecondName:    req.SecondName,
			BirthDate:     birthDate,
			BirthPlace:    birthPlace,
			Pesel:         req.Pesel,
			AddressStreet: req.AddressStreet,
			AddressCity:   req.AddressCity,
			AddressZip:    req.AddressZip,
			Telephone:     req.Telephone,
			CompanyID:     req.CompanyID,
		},
	})
	if err != nil {
		if errors.Is(err, ErrCompanyNotFound) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "company not found")
			return
		}
		if errors.Is(err, ErrDuplicateStudent) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "student with the same name and birth date already exists")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create student")
		return
	}

	response.WriteJSON(w, http.StatusCreated, StudentDetailsResponse{
		Data: row,
	})

}

// PutByExternalID zakłada (201) albo nadpisuje (200) kursanta o identyfikatorze platformy.
// Ciało to StudentWrite - te same reguły co POST i PATCH.
func (h *Handler) PutByExternalID(w http.ResponseWriter, r *http.Request) {
	externalID := r.PathValue("externalId")
	if externalID == "" || utf8.RuneCountInString(externalID) > MaxExternalIDLength {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid external id")
		return
	}
	upserter, ok := h.creator.(ExternalIDUpserter)
	if !ok {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to save student")
		return
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := CreateStudentRequest{}
	if err := decoder.Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if req.CompanyID != nil && *req.CompanyID <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	student, created, err := upserter.UpsertByExternalID(r.Context(), externalID, req)
	if err != nil {
		var conflict *NaturalKeyConflictError
		switch {
		case errors.Is(err, ErrInvalidInput):
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		case errors.Is(err, ErrCompanyNotFound):
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "company not found")
		case errors.As(err, &conflict):
			if conflict.ID > 0 {
				response.WriteErrorWithID(w, http.StatusConflict, response.CodeConflict, conflict.Error(), conflict.ID)
			} else {
				response.WriteError(w, http.StatusConflict, response.CodeConflict, conflict.Error())
			}
		default:
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to save student")
		}
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	response.WriteJSON(w, status, StudentDetailsResponse{Data: student})
}

func mapStudentDetailsRow(row dbsqlc.UpdateStudentRow) StudentDetailsDTO {

	dto := StudentDetailsDTO{
		ID:            row.ID,
		FirstName:     row.Firstname,
		LastName:      row.Lastname,
		BirthDate:     row.Birthdate.Time.Format(response.DateFormat),
		BirthPlace:    row.Birthplace,
		Pesel:         pgutil.NullableString(row.Pesel),
		AddressStreet: pgutil.NullableString(row.Addressstreet),
		AddressCity:   pgutil.NullableString(row.Addresscity),
		AddressZip:    pgutil.NullableString(row.Addresszip),
		Telephone:     pgutil.NullableString(row.Telephoneno),
		SecondName:    pgutil.NullableString(row.Secondname),
		ExternalID:    pgutil.NullableString(row.ExternalID),
	}
	if row.CompanyID.Valid && row.CompanyName.Valid {
		dto.Company = &CompanyDTO{
			ID:   row.CompanyID.Int64,
			Name: row.CompanyName.String,
		}
	}
	return dto
}

func mapCreateStudentRow(row dbsqlc.CreateStudentRow) StudentDetailsDTO {
	return mapStudentDetailsRow(dbsqlc.UpdateStudentRow(row))
}

func mapStudentsByCompanyRow(row dbsqlc.ListStudentsByCompanyIDRow) ListStudentsByCompanyIdDTO {
	dto := ListStudentsByCompanyIdDTO{
		ID:         row.ID,
		Firstname:  row.Firstname,
		Lastname:   row.Lastname,
		Secondname: pgutil.NullableString(row.Secondname),
		Birthdate:  row.Birthdate.Time.Format(response.DateFormat),
		Birthplace: row.Birthplace,
		Pesel:      pgutil.NullableString(row.Pesel),
	}
	return dto
}

func mapCertByStudentsRow(row dbsqlc.ListCertificatesByStudentIDRow) CertificateByStudentDTO {
	dto := CertificateByStudentDTO{
		ID:              row.ID,
		Date:            row.Date.Time.Format(response.DateFormat),
		CourseName:      row.CourseName,
		CourseSymbol:    row.CourseSymbol,
		RegistryYear:    row.RegistryYear,
		RegistryNumber:  row.RegistryNumber,
		CourseDateStart: row.CourseDateStart.Time.Format(response.DateFormat),
		CourseDateEnd:   pgutil.NullableDate(row.CourseDateEnd),
		ExpiryDate:      pgutil.NullableString(row.ExpiryDate),
		RevokedAt:       pgutil.NullableTimestampz(row.RevokedAt),
		SupersededByID:  pgutil.NullableInt64(row.SupersededByID),
	}
	return dto
}

func mapStudentRow(row dbsqlc.ListStudentsRow) StudentDTO {
	dto := StudentDTO{
		ID:        row.ID,
		FirstName: row.Firstname,
		LastName:  row.Lastname,
		BirthDate: row.Birthdate.Time.Format(response.DateFormat),
	}

	if row.Pesel.Valid {
		dto.Pesel = &row.Pesel.String
	}

	if row.CompanyID.Valid && row.CompanyName.Valid {
		dto.Company = &CompanyDTO{
			ID:   row.CompanyID.Int64,
			Name: row.CompanyName.String,
		}
	}

	return dto
}
