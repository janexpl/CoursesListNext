package gusclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/janexpl/CoursesListNext/api/internal/config"
	"github.com/janexpl/CoursesListNext/api/internal/response"
	"github.com/janexpl/CoursesListNext/api/internal/validation"
	nip "github.com/janexpl/guslookup"
)

type Handler struct {
	config *config.Config
}

const gusLogoutTimeout = 5 * time.Second

func NewHandler(config *config.Config) *Handler {
	return &Handler{
		config: config,
	}
}

func (h *Handler) FindCompany(w http.ResponseWriter, r *http.Request) {
	nipQuery := strings.TrimSpace(r.URL.Query().Get("nip"))
	if nipQuery == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "no nip value in request")
		return
	}
	normalizedNIP := validation.NormalizeNIP(nipQuery)
	if err := validation.ValidateNIP(normalizedNIP); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, fmt.Sprintf("nip validation error: %v", err))
		return
	}
	if strings.TrimSpace(h.config.GUSToken) == "" {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "gus lookup is not configured")
		return
	}

	gusClient := nip.NewClient(h.config.GUSUrl, h.config.GUSToken)

	if err := gusClient.Login(r.Context()); err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "unable to login to BIR")
		return
	}
	defer closeClientAsync(gusClient)

	company, err := gusClient.LookupNIP(r.Context(), normalizedNIP)
	if err != nil {
		if fault, ok := errors.AsType[*nip.FaultError](err); ok {
			if fault.Code == "4" {
				response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "company not found")
				return
			}
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "nip lookup failed")
		return
	}
	response.WriteJSON(w, http.StatusOK, GUSCompanyResponse{
		Data: mapCompanyRow(company),
	})
}

func closeClientAsync(client *nip.Client) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gusLogoutTimeout)
		defer cancel()

		_ = client.Close(ctx)
	}()
}

func mapCompanyRow(row nip.Company) GUSCompanyDTO {
	return GUSCompanyDTO{
		NIP:         row.NIP,
		REGON:       row.REGON,
		Name:        row.Name,
		Voivodeship: row.Voivodeship,
		County:      row.County,
		Commune:     row.Commune,
		City:        row.City,
		PostalCode:  row.PostalCode,
		Street:      row.Street,
		HouseNumber: row.HouseNumber,
		Apartment:   row.Apartment,
		Status:      row.Status,
	}
}
