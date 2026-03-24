package students

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Handler struct {
	queries *dbsqlc.Queries
}

func NewHandler(queries *dbsqlc.Queries) *Handler {
	return &Handler{
		queries: queries,
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid student id")
		return
	}

	student, err := h.queries.GetStudentByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "student")
		return
	}

	dto := StudentDetailsDTO{
		ID:            student.ID,
		FirstName:     student.Firstname,
		LastName:      student.Lastname,
		BirthDate:     student.Birthdate.Time.Format(response.DateFormat),
		BirthPlace:    student.Birthplace,
		Pesel:         pgutil.NullableString(student.Pesel),
		AddressStreet: pgutil.NullableString(student.Addressstreet),
		AddressCity:   pgutil.NullableString(student.Addresscity),
		AddressZip:    pgutil.NullableString(student.Addresszip),
		Telephone:     pgutil.NullableString(student.Telephoneno),
		SecondName:    pgutil.NullableString(student.Secondname),
	}
	if student.CompanyID.Valid && student.CompanyName.Valid {
		dto.Company = &CompanyDTO{
			ID:   student.CompanyID.Int64,
			Name: student.CompanyName.String,
		}
	}

	resp := StudentDetailsResponse{Data: dto}
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

	rows, err := h.queries.ListStudents(r.Context(), dbsqlc.ListStudentsParams{
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

	rows, err := h.queries.ListCertificatesByStudentID(r.Context(), int32(id))
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list certificates for student")
		return
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

	rows, err := h.queries.ListStudentsByCompanyID(r.Context(), idPgType)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list students")
		return
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
	date, err := time.Parse(response.DateFormat, birthDate)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	row, err := h.queries.UpdateStudent(r.Context(), dbsqlc.UpdateStudentParams{
		Firstname:     firstName,
		Lastname:      lastName,
		Secondname:    pgutil.OptionalText(req.SecondName),
		Birthdate:     pgtype.Date{Time: date, InfinityModifier: 0, Valid: true},
		Birthplace:    birthPlace,
		Pesel:         pgutil.OptionalText(req.Pesel),
		Addressstreet: pgutil.OptionalText(req.AddressStreet),
		Addresscity:   pgutil.OptionalText(req.AddressCity),
		Addresszip:    pgutil.OptionalText(req.AddressZip),
		Telephoneno:   pgutil.OptionalText(req.Telephone),
		CompanyID:     pgutil.OptionalInt8(req.CompanyID),
		StudentID:     idInt,
	})
	if err != nil {
		response.HandleDBError(w, err, "student")
		return
	}
	resp := StudentDetailsResponse{
		Data: mapStudentDetailsRow(row),
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

	date, err := time.Parse(response.DateFormat, birthDate)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.queries.CreateStudent(r.Context(), dbsqlc.CreateStudentParams{
		Firstname:     firstName,
		Lastname:      lastName,
		Secondname:    pgutil.OptionalText(req.SecondName),
		Birthdate:     pgtype.Date{Time: date, InfinityModifier: 0, Valid: true},
		Birthplace:    birthPlace,
		Pesel:         pgutil.OptionalText(req.Pesel),
		Addressstreet: pgutil.OptionalText(req.AddressStreet),
		Addresscity:   pgutil.OptionalText(req.AddressCity),
		Addresszip:    pgutil.OptionalText(req.AddressZip),
		Telephoneno:   pgutil.OptionalText(req.Telephone),
		CompanyID:     pgutil.OptionalInt8(req.CompanyID),
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create student")
		return
	}

	response.WriteJSON(w, http.StatusCreated, StudentDetailsResponse{
		Data: mapCreateStudentRow(row),
	})

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
