// Package companies ...
package companies

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
	"github.com/janexpl/CoursesListNext/api/internal/validation"
)

type Querier interface {
	GetCompanyByID(ctx context.Context, id int64) (dbsqlc.Company, error)
	ListCompanies(ctx context.Context, arg dbsqlc.ListCompaniesParams) ([]dbsqlc.ListCompaniesRow, error)
}

type Creator interface {
	Create(ctx context.Context, req CreateCompanyRequest) (CompanyDetailsDTO, error)
	Update(ctx context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error)
}

// ExternalIDUpserter obsługuje PUT /companies/by-external-id/{externalId}. Osobny
// interfejs, żeby nie rozszerzać Creator o metodę, której nie potrzebuje reszta tras.
type ExternalIDUpserter interface {
	UpsertByExternalID(ctx context.Context, externalID string, req CreateCompanyRequest) (CompanyDetailsDTO, bool, error)
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
	// Telefon jest opcjonalny (brak = pusty string, jak w istniejących danych).
	telephone := strings.TrimSpace(req.Telephone)

	if name == "" || street == "" || city == "" || zipcode == "" || nip == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.creator.Update(r.Context(), idInt, UpdateCompanyDTO{
		Name:                       name,
		Street:                     street,
		City:                       city,
		Zipcode:                    zipcode,
		Nip:                        nip,
		Email:                      req.Email,
		ContactPerson:              req.ContactPerson,
		Telephone:                  telephone,
		Note:                       req.Note,
		ExpiryNotificationsEnabled: req.ExpiryNotificationsEnabled,
		ExpiryNotificationEmail:    req.ExpiryNotificationEmail,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidNIP) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, nipValidationMessage(err))
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
			return
		}
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
	// Telefon jest opcjonalny (brak = pusty string, jak w istniejących danych).
	telephone := strings.TrimSpace(req.Telephone)

	if name == "" || street == "" || city == "" || zipcode == "" || nip == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.creator.Create(r.Context(), CreateCompanyRequest{
		Name:                       name,
		Street:                     street,
		City:                       city,
		Zipcode:                    zipcode,
		Nip:                        nip,
		Email:                      req.Email,
		ContactPerson:              req.ContactPerson,
		Telephone:                  telephone,
		Note:                       req.Note,
		ExpiryNotificationsEnabled: req.ExpiryNotificationsEnabled,
		ExpiryNotificationEmail:    req.ExpiryNotificationEmail,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidNIP) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, nipValidationMessage(err))
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
			return
		}
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

// PutByExternalID zakłada (201) albo nadpisuje (200) firmę o identyfikatorze platformy.
// Ciało to CompanyWrite - te same reguły co POST i PATCH.
func (h *Handler) PutByExternalID(w http.ResponseWriter, r *http.Request) {
	externalID := r.PathValue("externalId")
	if externalID == "" || utf8.RuneCountInString(externalID) > MaxExternalIDLength {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid external id")
		return
	}
	upserter, ok := h.creator.(ExternalIDUpserter)
	if !ok {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to save company")
		return
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := CreateCompanyRequest{}
	if err := decoder.Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	company, created, err := upserter.UpsertByExternalID(r.Context(), externalID, req)
	if err != nil {
		var conflict *NaturalKeyConflictError
		switch {
		case errors.Is(err, ErrInvalidNIP):
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, nipValidationMessage(err))
		case errors.Is(err, ErrInvalidInput):
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		case errors.As(err, &conflict):
			if conflict.ID > 0 {
				response.WriteErrorWithID(w, http.StatusConflict, response.CodeConflict, conflict.Error(), conflict.ID)
			} else {
				response.WriteError(w, http.StatusConflict, response.CodeConflict, conflict.Error())
			}
		default:
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to save company")
		}
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	response.WriteJSON(w, status, CompanyDetailsResponse{Data: company})
}

// nipValidationMessage buduje komunikat w tym samym formacie, którego używa
// GET /companies/lookup-by-nip, żeby klient obsługiwał oba miejsca jednakowo.
func nipValidationMessage(err error) string {
	for _, reason := range []error{validation.ErrInvalidLength, validation.ErrInvalidFormat, validation.ErrInvalidChecksum} {
		if errors.Is(err, reason) {
			return "nip validation error: " + reason.Error()
		}
	}
	return "nip validation error: invalid nip"
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
		ContactPerson: pgutil.NullableString(row.Contactperson),
		Telephone:     row.Telephoneno,
	}

	return dto
}

func mapCompanyDetailRow(row dbsqlc.Company) CompanyDetailsDTO {
	dto := CompanyDetailsDTO{
		ID:                         row.ID,
		Name:                       row.Name,
		Street:                     row.Street,
		City:                       row.City,
		Zipcode:                    row.Zipcode,
		Nip:                        row.Nip,
		Email:                      pgutil.NullableString(row.Email),
		Contactperson:              pgutil.NullableString(row.Contactperson),
		Telephoneno:                row.Telephoneno,
		Note:                       pgutil.NullableString(row.Note),
		ExpiryNotificationsEnabled: row.ExpiryNotificationsEnabled,
		ExpiryNotificationEmail:    pgutil.NullableString(row.ExpiryNotificationEmail),
		ExternalID:                 pgutil.NullableString(row.ExternalID),
	}
	return dto
}
