package response

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
)

const (
	CodeBadRequest         = "bad_request"
	CodeUnauthorized       = "unauthorized"
	CodeInvalidCredentials = "invalid_credentials"
	CodeInternalError      = "internal_error"
	CodeNotFound           = "not_found"
	CodeForbidden          = "forbidden"
	CodeConflict           = "conflict"
)

const (
	DateFormat       = "2006-01-02"
	TimestampzFormat = "2006-01-02 15:04:05"
)

func WriteJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("response: failed to encode JSON: %v", err)
	}
}

func WriteError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: ErrorBody{Code: code, Message: message}}); err != nil {
		log.Printf("response: failed to encode error JSON: %v", err)
	}
}

func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

func HandleDBError(w http.ResponseWriter, err error, entityName string) {
	if errors.Is(err, pgx.ErrNoRows) {
		WriteError(w, http.StatusNotFound, CodeNotFound, entityName+" not found")
		return
	}
	WriteError(w, http.StatusInternalServerError, CodeInternalError, "failed to get "+entityName)
}
