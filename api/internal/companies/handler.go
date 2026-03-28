// Package companies ...
package companies

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	GetCompanyByID(ctx context.Context, id int64) (dbsqlc.Company, error)
	ListCompanies(ctx context.Context, arg dbsqlc.ListCompaniesParams) ([]dbsqlc.ListCompaniesRow, error)
}

type Creator interface {
	Create(ctx context.Context, req CreateCompanyRequest) (CompanyDetailsDTO, error)
	Update(ctx context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error)
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
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid company id")
		return
	}
	row, err := h.querier.GetCompanyByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "company")
		return
	}

	response.WriteJSON(w, http.StatusOK, CompanyDetailsResponse{
		Data: mapCompanyDetailRow(row),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	searchPg, limitInt, err := response.ParseListParams(r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	rows, err := h.querier.ListCompanies(r.Context(), dbsqlc.ListCompaniesParams{
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

	row, err := h.creator.Update(r.Context(), idInt, UpdateCompanyDTO{
		Name:          name,
		Street:        street,
		City:          city,
		Zipcode:       zipcode,
		Nip:           nip,
		Email:         req.Email,
		ContactPerson: req.ContactPerson,
		Telephone:     telephone,
		Note:          req.Note,
	})

	if err != nil {
		if isCompanyNIPConflict(err) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "company with this NIP already exists")
			return
		}
		response.HandleDBError(w, err, "company")
		return
	}

	response.WriteJSON(w, http.StatusOK, CompanyDetailsResponse{
		Data: row,
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

	row, err := h.creator.Create(r.Context(), CreateCompanyRequest{
		Name:          name,
		Street:        street,
		City:          city,
		Zipcode:       zipcode,
		Nip:           nip,
		Email:         req.Email,
		ContactPerson: req.ContactPerson,
		Telephone:     telephone,
		Note:          req.Note,
	})

	if err != nil {
		if isCompanyNIPConflict(err) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "company with this NIP already exists")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create company")
		return
	}
	response.WriteJSON(w, http.StatusCreated, CompanyDetailsResponse{
		Data: row,
	})
}

func isCompanyNIPConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "check_unique_nip"
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
