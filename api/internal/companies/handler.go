package companies

import (
	"encoding/json"
	"net/http"
	"strings"

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
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid company id")
		return
	}
	row, err := h.queries.GetCompanyByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "company")
		return
	}

	response.WriteJSON(w, http.StatusOK, CompanyDetailsResponse{
		Data: mapCompanyDetailRow(row)})
}
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	searchPg, limitInt, err := response.ParseListParams(r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	}
	rows, err := h.queries.ListCompanies(r.Context(), dbsqlc.ListCompaniesParams{
		Search:     searchPg,
		LimitCount: limitInt,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list companies")
		return
	}
	resp := ListCompaniesResponse{
		Data: make([]CompanyDTO, 0, len(rows)),
	}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapCompanyRow(row))
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	idInt, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid company ID")
		return
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	req := UpdateCompanyDTO{}
	err = decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	street := strings.TrimSpace(req.Street)
	city := strings.TrimSpace(req.City)
	zipcode := strings.TrimSpace(req.Zipcode)
	nip := strings.TrimSpace(req.Nip)
	telephone := strings.TrimSpace(req.Telephone)

	if name == "" || street == "" || city == "" || zipcode == "" || nip == "" || telephone == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.queries.UpdateCompany(r.Context(), dbsqlc.UpdateCompanyParams{
		ID:            idInt,
		Name:          name,
		Street:        street,
		City:          city,
		Zipcode:       zipcode,
		Nip:           nip,
		Email:         pgutil.OptionalText(req.Email),
		Contactperson: pgutil.OptionalText(req.ContactPerson),
		Telephoneno:   telephone,
		Note:          pgutil.OptionalText(req.Note),
	})

	if err != nil {
		response.HandleDBError(w, err, "company")
		return
	}

	response.WriteJSON(w, http.StatusOK, CompanyDetailsResponse{
		Data: mapCompanyDetailRow(row),
	})

}

func (h *Handler) CreateCompany(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	req := CreateCompanyRequest{}
	err := decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	street := strings.TrimSpace(req.Street)
	city := strings.TrimSpace(req.City)
	zipcode := strings.TrimSpace(req.Zipcode)
	nip := strings.TrimSpace(req.Nip)
	telephone := strings.TrimSpace(req.Telephone)

	if name == "" || street == "" || city == "" || zipcode == "" || nip == "" || telephone == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.queries.CreateCompany(r.Context(), dbsqlc.CreateCompanyParams{
		Name:          name,
		Street:        street,
		City:          city,
		Zipcode:       zipcode,
		Nip:           nip,
		Email:         pgutil.OptionalText(req.Email),
		Contactperson: pgutil.OptionalText(req.ContactPerson),
		Telephoneno:   telephone,
		Note:          pgutil.OptionalText(req.Note),
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create company")
		return
	}
	response.WriteJSON(w, http.StatusCreated, CompanyDetailsResponse{
		Data: mapCompanyDetailRow(row),
	})

}

func mapCompanyRow(row dbsqlc.ListCompaniesRow) CompanyDTO {
	dto := CompanyDTO{
		ID:            row.ID,
		Name:          row.Name,
		City:          row.City,
		NIP:           row.Nip,
		ContactPerson: row.Contactperson.String,
		Telephone:     row.Telephoneno,
	}

	return dto

}

func mapCompanyDetailRow(row dbsqlc.Company) CompanyDetailsDTO {
	dto := CompanyDetailsDTO{
		ID:            row.ID,
		Name:          row.Name,
		Street:        row.Street,
		City:          row.City,
		Zipcode:       row.Zipcode,
		Nip:           row.Nip,
		Email:         pgutil.NullableString(row.Email),
		Contactperson: pgutil.NullableString(row.Contactperson),
		Telephoneno:   row.Telephoneno,
		Note:          pgutil.NullableString(row.Note),
	}
	return dto
}
