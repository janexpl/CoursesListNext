package journals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
	"github.com/janexpl/CoursesListNext/api/internal/validation"
)

type Querier interface {
	ListJournals(ctx context.Context, arg sqlc.ListJournalsParams) ([]sqlc.ListJournalsRow, error)
	CreateJournal(ctx context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error)
	GetJournalByID(ctx context.Context, id int64) (sqlc.GetJournalByIDRow, error)
	GetCourseByID(ctx context.Context, id int64) (sqlc.Course, error)
	DeleteJournal(ctx context.Context, id int64) (int64, error)
	CloseJournal(ctx context.Context, id int64) (int64, error)
	ListJournalAttendees(ctx context.Context, journalID int64) ([]sqlc.ListJournalAttendeesRow, error)
	AddJournalAttendee(ctx context.Context, arg sqlc.AddJournalAttendeeParams) (sqlc.AddJournalAttendeeRow, error)
	UpdateJournalAttendeeCertificate(ctx context.Context, arg sqlc.UpdateJournalAttendeeCertificateParams) (sqlc.UpdateJournalAttendeeCertificateRow, error)
	DeleteJournalAttendee(ctx context.Context, arg sqlc.DeleteJournalAttendeeParams) (int64, error)
	ListJournalSessions(ctx context.Context, journalID int64) ([]sqlc.TrainingJournalSession, error)
	GenerateJournalSessionsFromCourse(ctx context.Context, journalID int64) (int64, error)
	UpdateJournalSession(ctx context.Context, arg sqlc.UpdateJournalSessionParams) (sqlc.TrainingJournalSession, error)
	ListJournalAttendance(ctx context.Context, journalID int64) ([]sqlc.TrainingJournalAttendance, error)
	UpsertJournalAttendance(ctx context.Context, arg sqlc.UpsertJournalAttendanceParams) (sqlc.TrainingJournalAttendance, error)
	UpdateJournalHeader(ctx context.Context, arg sqlc.UpdateJournalHeaderParams) (sqlc.UpdateJournalHeaderRow, error)
	UpsertJournalAttendanceScan(ctx context.Context, arg sqlc.UpsertJournalAttendanceScanParams) (sqlc.UpsertJournalAttendanceScanRow, error)
	GetJournalAttendanceScanFile(ctx context.Context, journalID int64) (sqlc.GetJournalAttendanceScanFileRow, error)
	GetJournalAttendanceScanMeta(ctx context.Context, journalID int64) (sqlc.GetJournalAttendanceScanMetaRow, error)
	DeleteJournalAttendanceScan(ctx context.Context, journalID int64) (int64, error)
	UpsertJournalSignedScan(ctx context.Context, arg sqlc.UpsertJournalSignedScanParams) (sqlc.UpsertJournalSignedScanRow, error)
	GetJournalSignedScanFile(ctx context.Context, journalID int64) (sqlc.GetJournalSignedScanFileRow, error)
	GetJournalSignedScanMeta(ctx context.Context, journalID int64) (sqlc.GetJournalSignedScanMetaRow, error)
	DeleteJournalSignedScan(ctx context.Context, journalID int64) (int64, error)
}

type Handler struct {
	querier   Querier
	generator CertificateGenerator
}

func NewHandler(querier Querier, generators ...CertificateGenerator) *Handler {
	var generator CertificateGenerator
	if len(generators) > 0 {
		generator = generators[0]
	}

	return &Handler{querier: querier, generator: generator}
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	rowsAffected, err := h.querier.DeleteJournal(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to delete journal")
		return
	}
	if rowsAffected == 0 {
		response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "journal not found")
		return
	}

	var resp DeleteJournalResponse
	resp.Data.ID = id
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	journal, err := h.querier.GetJournalByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}

	if journal.Status == "closed" {
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "journal is already closed")
		return
	}

	if journal.AttendeesCount == 0 || journal.SessionsCount == 0 {
		response.WriteError(
			w,
			http.StatusBadRequest,
			response.CodeBadRequest,
			"journal must include at least one attendee and one session before closing",
		)
		return
	}

	rowsAffected, err := h.querier.CloseJournal(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to close journal")
		return
	}
	if rowsAffected == 0 {
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "journal is already closed")
		return
	}

	updatedJournal, err := h.querier.GetJournalByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}

	response.WriteJSON(w, http.StatusOK, JournalDetailResponse{
		Data: mapJournalDetailsRow(sqlc.CreateJournalRow(updatedJournal)),
	})
}

func (h *Handler) ListAttendees(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	rows, err := h.querier.ListJournalAttendees(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}
	if len(rows) == 0 {
		_, err := h.querier.GetJournalByID(r.Context(), id)
		if err != nil {
			response.HandleDBError(w, err, "journal")
			return
		}
	}

	resp := ListJournalAttendeeResponse{
		Data: make([]JournalAttendeeDTO, 0, len(rows)),
	}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapJournalAttendeesRowFromList(row))
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) AddJournalAttendee(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := AddJournalAttendeeRequest{}
	err = decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if req.StudentID <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	row, err := h.querier.AddJournalAttendee(r.Context(), sqlc.AddJournalAttendeeParams{
		JournalID: id,
		StudentID: req.StudentID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "journal or student not found")
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "student already added to journal")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to add attendee to journal")
		return
	}

	response.WriteJSON(w, http.StatusCreated, AddJournalAttendeeResponse{
		Data: mapJournalAttendeesRowFromAdd(row),
	})
}

func (h *Handler) GenerateAttendeeCertificate(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	attendeeID, err := response.ParsePositiveInt64PathValue(r, "attendeeId")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid attendee id")
		return
	}

	if h.generator == nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "certificate generator is not configured")
		return
	}

	result, err := h.generator.GenerateAttendeeCertificate(r.Context(), journalID, attendeeID)
	if err != nil {
		switch {
		case errors.Is(err, ErrJournalAttendeeNotFound):
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "journal attendee not found")
		case errors.Is(err, ErrJournalAttendeeCertificateLinked):
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "certificate already linked to journal attendee")
		case errors.Is(err, ErrJournalCertificateGeneration):
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "failed to generate certificate")
		default:
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to generate certificate")
		}
		return
	}

	var resp GenerateJournalAttendeeCertificateResponse
	resp.Data.ID = result.CertificateID
	response.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) PatchAttendeeCertificate(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	attendeeID, err := response.ParsePositiveInt64PathValue(r, "attendeeId")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid attendee id")
		return
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := UpdateJournalAttendeeCertificateRequest{}
	if err := decoder.Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if req.CertificateID != nil && *req.CertificateID <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.querier.UpdateJournalAttendeeCertificate(r.Context(), sqlc.UpdateJournalAttendeeCertificateParams{
		JournalID:     journalID,
		AttendeeID:    attendeeID,
		CertificateID: pgutil.OptionalInt8(req.CertificateID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "journal attendee or certificate not found")
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "certificate already linked to another journal attendee")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to update journal attendee certificate")
		return
	}

	response.WriteJSON(w, http.StatusOK, JournalAttendeeResponse{
		Data: mapJournalAttendeesRowFromUpdate(row),
	})
}

func (h *Handler) DeleteAttendee(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	attendeeID, err := response.ParsePositiveInt64PathValue(r, "attendeeId")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid attendee id")
		return
	}

	journal, err := h.querier.GetJournalByID(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}
	if journal.Status == "closed" {
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "journal is closed")
		return
	}

	rowsAffected, err := h.querier.DeleteJournalAttendee(r.Context(), sqlc.DeleteJournalAttendeeParams{
		JournalID: journalID,
		ID:        attendeeID,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to delete journal attendee")
		return
	}
	if rowsAffected == 0 {
		response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "journal attendee not found")
		return
	}

	var resp DeleteJournalAttendeeResponse
	resp.Data.ID = attendeeID
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	rows, err := h.querier.ListJournalSessions(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to get journal sessions")
		return
	}
	if len(rows) == 0 {
		_, err := h.querier.GetJournalByID(r.Context(), id)
		if err != nil {
			response.HandleDBError(w, err, "journal")
			return
		}
	}

	resp := ListJournalSessionsResponse{
		Data: make([]JournalSessionDTO, 0, len(rows)),
	}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapJournalSessionRow(row))
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListAttendance(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	rows, err := h.querier.ListJournalAttendance(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to get journal attendance")
		return
	}

	if len(rows) == 0 {
		_, err := h.querier.GetJournalByID(r.Context(), id)
		if err != nil {
			response.HandleDBError(w, err, "journal")
			return
		}
	}

	resp := ListJournalAttendanceResponse{
		Data: make([]JournalAttendanceDTO, 0, len(rows)),
	}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapJournalAttendanceRow(row))
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) PatchAttendance(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := UpdateJournalAttendanceRequest{}
	if err := decoder.Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	if req.JournalSessionID <= 0 || req.JournalAttendeeID <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	journal, err := h.querier.GetJournalByID(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}
	if journal.Status == "closed" {
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "journal is closed")
		return
	}

	row, err := h.querier.UpsertJournalAttendance(r.Context(), sqlc.UpsertJournalAttendanceParams{
		JournalID:         journalID,
		JournalSessionID:  req.JournalSessionID,
		JournalAttendeeID: req.JournalAttendeeID,
		Present:           req.Present,
	})
	if err != nil {
		response.HandleDBError(w, err, "journal attendance")
		return
	}

	response.WriteJSON(w, http.StatusOK, JournalAttendanceResponse{
		Data: mapJournalAttendanceRow(row),
	})
}

func (h *Handler) GenerateSessionsFromCourse(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	_, err = h.querier.GetJournalByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}

	existingRows, err := h.querier.ListJournalSessions(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to generate journal sessions")
		return
	}
	if len(existingRows) > 0 {
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "journal sessions already exist")
		return
	}

	generatedCount, err := h.querier.GenerateJournalSessionsFromCourse(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to generate journal sessions")
		return
	}
	if generatedCount == 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "course program is empty")
		return
	}

	var resp GenerateJournalSessionsResponse
	resp.Data.GeneratedCount = generatedCount

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) PatchSession(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	sessionID, err := response.ParsePositiveInt64PathValue(r, "sessionId")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid session id")
		return
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := UpdateJournalSessionRequest{}
	if err := decoder.Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	sessionDate := strings.TrimSpace(req.SessionDate)
	trainerName := strings.TrimSpace(req.TrainerName)
	if sessionDate == "" || trainerName == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	journal, err := h.querier.GetJournalByID(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}
	if journal.Status == "closed" {
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "journal is closed")
		return
	}

	parsedSessionDate, err := time.Parse(response.DateFormat, sessionDate)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid sessionDate")
		return
	}
	if parsedSessionDate.Before(journal.DateStart.Time) || parsedSessionDate.After(journal.DateEnd.Time) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "session date must fit within journal dates")
		return
	}

	row, err := h.querier.UpdateJournalSession(r.Context(), sqlc.UpdateJournalSessionParams{
		JournalID:   journalID,
		SessionID:   sessionID,
		SessionDate: pgtype.Date{Time: parsedSessionDate, Valid: true},
		TrainerName: trainerName,
	})
	if err != nil {
		response.HandleDBError(w, err, "journal session")
		return
	}

	response.WriteJSON(w, http.StatusOK, JournalSessionResponse{
		Data: mapJournalSessionRow(row),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	row, err := h.querier.GetJournalByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}
	response.WriteJSON(w, http.StatusOK, JournalDetailResponse{
		Data: mapJournalDetailsRow(sqlc.CreateJournalRow(row)),
	})
}

func (h *Handler) PDF(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	journal, err := h.querier.GetJournalByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}

	course, err := h.querier.GetCourseByID(r.Context(), journal.CourseID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to prepare journal pdf")
		return
	}

	attendees, err := h.querier.ListJournalAttendees(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to prepare journal pdf")
		return
	}

	sessions, err := h.querier.ListJournalSessions(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to prepare journal pdf")
		return
	}

	attendance, err := h.querier.ListJournalAttendance(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to prepare journal pdf")
		return
	}

	pdfBytes, err := renderJournalPDF(r.Context(), buildJournalPDFHTML(journal, course, attendees, sessions, attendance))
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to render journal pdf")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+buildJournalPDFFilename(journal)+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := CreateJournalRequest{}
	err := decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	dateStart := strings.TrimSpace(req.DateStart)
	dateEnd := strings.TrimSpace(req.DateEnd)
	formOfTraining := strings.TrimSpace(req.FormOfTraining)
	legalBasis := strings.TrimSpace(req.LegalBasis)
	location := strings.TrimSpace(req.Location)
	organizerName := strings.TrimSpace(req.OrganizerName)
	title := strings.TrimSpace(req.Title)

	if req.CourseID <= 0 || dateStart == "" || dateEnd == "" || formOfTraining == "" || legalBasis == "" || location == "" || organizerName == "" || title == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	parsedDateStart, err := time.Parse(response.DateFormat, req.DateStart)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateStart")
		return
	}
	parsedDateEnd, err := time.Parse(response.DateFormat, req.DateEnd)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateEnd")
		return
	}
	if parsedDateEnd.Before(parsedDateStart) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	row, err := h.querier.CreateJournal(r.Context(), sqlc.CreateJournalParams{
		CourseID:         req.CourseID,
		CompanyID:        pgutil.OptionalInt8(req.CompanyID),
		Title:            title,
		OrganizerName:    organizerName,
		OrganizerAddress: pgutil.OptionalText(req.OrganizerAddress),
		Location:         location,
		FormOfTraining:   formOfTraining,
		LegalBasis:       legalBasis,
		DateStart:        pgtype.Date{Time: parsedDateStart, Valid: true},
		DateEnd:          pgtype.Date{Time: parsedDateEnd, Valid: true},
		Notes:            pgutil.OptionalText(req.Notes),
		CreatedByUserID:  user.ID,
	})
	if err != nil {
		response.HandleDBError(w, err, "course")
		return
	}

	response.WriteJSON(w, http.StatusCreated, JournalDetailResponse{
		Data: mapJournalDetailsRow(row),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	searchText, limitInt, err := response.ParseListParams(r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	var courseID pgtype.Int8
	if rawCourseID := strings.TrimSpace(r.URL.Query().Get("courseId")); rawCourseID != "" {
		parsed, err := strconv.ParseInt(rawCourseID, 10, 64)
		if err != nil || parsed <= 0 {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid course id")
			return
		}
		courseID = pgtype.Int8{Int64: parsed, Valid: true}
	}

	var companyID pgtype.Int8
	if rawCompanyID := strings.TrimSpace(r.URL.Query().Get("companyId")); rawCompanyID != "" {
		parsed, err := strconv.ParseInt(rawCompanyID, 10, 64)
		if err != nil || parsed <= 0 {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid company id")
			return
		}
		companyID = pgtype.Int8{Int64: parsed, Valid: true}
	}

	var dateFrom pgtype.Date
	if rawDateFrom := strings.TrimSpace(r.URL.Query().Get("dateFrom")); rawDateFrom != "" {
		t, err := time.Parse(response.DateFormat, rawDateFrom)
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateFrom")
			return
		}
		dateFrom = pgtype.Date{Time: t, Valid: true}
	}

	var dateTo pgtype.Date
	if rawDateTo := strings.TrimSpace(r.URL.Query().Get("dateTo")); rawDateTo != "" {
		t, err := time.Parse(response.DateFormat, rawDateTo)
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid dateTo")
			return
		}
		dateTo = pgtype.Date{Time: t, Valid: true}
	}

	var statusText pgtype.Text
	if status != "" {
		if status != "draft" && status != "closed" {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid status")
			return
		}
		statusText = pgtype.Text{String: status, Valid: true}
	}

	rows, err := h.querier.ListJournals(r.Context(), sqlc.ListJournalsParams{
		Search:     searchText,
		CourseID:   courseID,
		CompanyID:  companyID,
		Status:     statusText,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		LimitCount: limitInt,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list journals")
		return
	}

	resp := ListJournalsResponse{
		Data: make([]JournalListItemDTO, 0, len(rows)),
	}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapJournalListRow(row))
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) UpdateHeader(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	req := UpdateJournalHeaderRequest{}
	err = decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	title := strings.TrimSpace(req.Title)
	organizerName := strings.TrimSpace(req.OrganizerName)
	location := strings.TrimSpace(req.Location)
	formOfTraining := strings.TrimSpace(req.FormOfTraining)
	legalBasis := strings.TrimSpace(req.LegalBasis)
	dateStartValue := strings.TrimSpace(req.DateStart)
	dateEndValue := strings.TrimSpace(req.DateEnd)

	if req.CompanyID != nil && *req.CompanyID <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if dateStartValue == "" || dateEndValue == "" || title == "" || organizerName == "" || location == "" || formOfTraining == "" || legalBasis == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	dateStart, err := time.Parse(response.DateFormat, dateStartValue)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	dateEnd, err := time.Parse(response.DateFormat, dateEndValue)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if dateEnd.Before(dateStart) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	journal, err := h.querier.GetJournalByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}
	if journal.Status == "closed" {
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "unable to change header because journal is closed")
		return
	}
	rows, err := h.querier.ListJournalSessions(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list journal sessions")
		return
	}
	for _, row := range rows {
		if row.SessionDate.Time.Before(dateStart) || row.SessionDate.Time.After(dateEnd) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "session outside range")
			return
		}
	}

	row, err := h.querier.UpdateJournalHeader(r.Context(), sqlc.UpdateJournalHeaderParams{
		CompanyID:        pgutil.OptionalInt8(req.CompanyID),
		Title:            title,
		OrganizerName:    organizerName,
		OrganizerAddress: pgutil.OptionalText(req.OrganizerAddress),
		Location:         location,
		FormOfTraining:   formOfTraining,
		LegalBasis:       legalBasis,
		DateStart:        pgtype.Date{Time: dateStart, Valid: true},
		DateEnd:          pgtype.Date{Time: dateEnd, Valid: true},
		Notes:            pgutil.OptionalText(req.Notes),
		JournalID:        id,
	})
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}

	response.WriteJSON(w, http.StatusOK, JournalDetailResponse{
		Data: mapJournalDetailsRow(sqlc.CreateJournalRow(row)),
	})
}

func (h *Handler) UpsertJournalAttendanceScan(w http.ResponseWriter, r *http.Request) {
	const maxFileSize = 16 << 20

	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	_, err = h.querier.GetJournalByID(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}

	err = r.ParseMultipartForm(maxFileSize)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "file is required")
		return
	}
	defer file.Close()
	fileByte, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "unable to read file")
		return
	}
	if int64(len(fileByte)) > maxFileSize {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "file is too large")
		return
	}
	if len(fileByte) == 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "file is required")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	contentType := http.DetectContentType(fileByte)
	switch contentType {
	case "application/pdf", "image/jpeg", "image/png":
		row, err := h.querier.UpsertJournalAttendanceScan(r.Context(), sqlc.UpsertJournalAttendanceScanParams{
			JournalID:        journalID,
			FileName:         header.Filename,
			ContentType:      contentType,
			FileSize:         int64(len(fileByte)),
			FileData:         fileByte,
			UploadedByUserID: user.ID,
		})
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to upload file")
			return
		}
		response.WriteJSON(w, http.StatusOK, JournalScanResponse{
			Data: mapUpsertJournalAttendanceScanRow(row),
		})
	default:
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "unsupported file type")
		return
	}
}

func (h *Handler) GetJournalAttendanceScanMeta(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	row, err := h.querier.GetJournalAttendanceScanMeta(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "file")
		return
	}

	response.WriteJSON(w, http.StatusOK, JournalScanResponse{
		Data: mapUpsertJournalAttendanceScanRow(sqlc.UpsertJournalAttendanceScanRow(row)),
	})
}

func (h *Handler) GetJournalAttendanceScanFile(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}
	row, err := h.querier.GetJournalAttendanceScanFile(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "file")
		return
	}

	w.Header().Set("Content-Type", row.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+row.FileName+`"`)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(row.FileData)
	if err != nil {
		log.Printf("failed to write file response: %v", err)
	}
}

func (h *Handler) DeleteJournalAttendanceScanFile(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	rowsAffected, err := h.querier.DeleteJournalAttendanceScan(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "file")
		return
	}
	if rowsAffected == 0 {
		response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "file not found")
		return
	}
	response.WriteNoContent(w)
}

func (h *Handler) UpsertJournalSignedScan(w http.ResponseWriter, r *http.Request) {
	const maxFileSize = 16 << 20

	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	_, err = h.querier.GetJournalByID(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "journal")
		return
	}

	err = r.ParseMultipartForm(maxFileSize)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "file is required")
		return
	}
	defer file.Close()
	fileByte, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "unable to read file")
		return
	}
	if int64(len(fileByte)) > maxFileSize {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "file is too large")
		return
	}
	if len(fileByte) == 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "file is required")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	contentType := http.DetectContentType(fileByte)
	switch contentType {
	case "application/pdf", "image/jpeg", "image/png":
		row, err := h.querier.UpsertJournalSignedScan(r.Context(), sqlc.UpsertJournalSignedScanParams{
			JournalID:        journalID,
			FileName:         header.Filename,
			ContentType:      contentType,
			FileSize:         int64(len(fileByte)),
			FileData:         fileByte,
			UploadedByUserID: user.ID,
		})
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to upload file")
			return
		}
		response.WriteJSON(w, http.StatusOK, JournalScanResponse{
			Data: JournalScanDTO(mapUpsertJournalSignedScanRow(row)),
		})
	default:
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "unsupported file type")
		return
	}
}

func (h *Handler) GetJournalSignedScanMeta(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	row, err := h.querier.GetJournalSignedScanMeta(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "file")
		return
	}

	response.WriteJSON(w, http.StatusOK, JournalScanResponse{
		Data: mapUpsertJournalSignedScanRow(sqlc.UpsertJournalSignedScanRow(row)),
	})
}

func (h *Handler) GetJournalSignedScanFile(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}
	row, err := h.querier.GetJournalSignedScanFile(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "file")
		return
	}

	w.Header().Set("Content-Type", row.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+row.FileName+`"`)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(row.FileData)
	if err != nil {
		log.Printf("failed to write file response: %v", err)
	}
}

func (h *Handler) DeleteJournalSignedScanFile(w http.ResponseWriter, r *http.Request) {
	journalID, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid journal id")
		return
	}

	rowsAffected, err := h.querier.DeleteJournalSignedScan(r.Context(), journalID)
	if err != nil {
		response.HandleDBError(w, err, "file")
		return
	}
	if rowsAffected == 0 {
		response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "file not found")
		return
	}
	response.WriteNoContent(w)
}

func mapUpsertJournalAttendanceScanRow(row sqlc.UpsertJournalAttendanceScanRow) JournalScanDTO {
	return JournalScanDTO{
		ID:               row.ID,
		FileName:         row.FileName,
		ContentType:      row.ContentType,
		FileSize:         row.FileSize,
		UploadedByUserID: row.UploadedByUserID,
		CreatedAt:        row.CreatedAt.Time.Format(response.TimestampzFormat),
		UpdatedAt:        row.UpdatedAt.Time.Format(response.TimestampzFormat),
	}
}

func mapUpsertJournalSignedScanRow(row sqlc.UpsertJournalSignedScanRow) JournalScanDTO {
	return JournalScanDTO{
		ID:               row.ID,
		FileName:         row.FileName,
		ContentType:      row.ContentType,
		FileSize:         row.FileSize,
		UploadedByUserID: row.UploadedByUserID,
		CreatedAt:        row.CreatedAt.Time.Format(response.TimestampzFormat),
		UpdatedAt:        row.UpdatedAt.Time.Format(response.TimestampzFormat),
	}
}

func mapJournalDetailsRow(row sqlc.CreateJournalRow) JournalDetailsDTO {
	var (
		companyID   *int64
		companyName *string
	)
	if row.CompanyID.Valid {
		id := row.CompanyID.Int64
		companyID = &id
		if row.CompanyName.Valid {
			name := row.CompanyName.String
			companyName = &name
		}
	}

	totalHours := numericToFloat64(row.TotalHours)

	return JournalDetailsDTO{
		ID:               row.ID,
		CourseID:         row.CourseID,
		CourseName:       row.CourseName,
		CompanyID:        companyID,
		CompanyName:      companyName,
		Title:            row.Title,
		CourseSymbol:     row.CourseSymbol,
		OrganizerName:    row.OrganizerName,
		OrganizerAddress: pgutil.NullableString(row.OrganizerAddress),
		Location:         row.Location,
		FormOfTraining:   row.FormOfTraining,
		LegalBasis:       row.LegalBasis,
		DateStart:        row.DateStart.Time.Format(response.DateFormat),
		DateEnd:          row.DateEnd.Time.Format(response.DateFormat),
		TotalHours:       totalHours,
		Notes:            pgutil.NullableString(row.Notes),
		Status:           row.Status,
		CreatedByUserID:  row.CreatedByUserID,
		CreatedAt:        row.CreatedAt.Time.Format(response.TimestampzFormat),
		UpdatedAt:        pgutil.NullableTimestampz(row.UpdatedAt),
		ClosedAt:         pgutil.NullableTimestampz(row.ClosedAt),
		AttendeesCount:   row.AttendeesCount,
		SessionsCount:    row.SessionsCount,
	}
}

func mapJournalListRow(row sqlc.ListJournalsRow) JournalListItemDTO {
	var company *CompanyRefDTO
	if row.CompanyID.Valid {
		company = &CompanyRefDTO{
			ID:   row.CompanyID.Int64,
			Name: row.CompanyName.String,
		}
	}

	return JournalListItemDTO{
		ID:             row.ID,
		Title:          row.Title,
		CourseSymbol:   row.CourseSymbol,
		OrganizerName:  row.OrganizerName,
		Location:       row.Location,
		FormOfTraining: row.FormOfTraining,
		DateStart:      row.DateStart.Time.Format(response.DateFormat),
		DateEnd:        row.DateEnd.Time.Format(response.DateFormat),
		TotalHours:     formatNumeric(row.TotalHours),
		Status:         row.Status,
		Course: CourseRefDTO{
			ID:   row.CourseID,
			Name: row.CourseName,
		},
		Company:        company,
		AttendeesCount: row.AttendeesCount,
		SessionsCount:  row.SessionsCount,
		CreatedAt:      row.CreatedAt.Time.Format(time.RFC3339),
	}
}

type journalAttendeeSource struct {
	ID                        int64
	JournalID                 int64
	StudentID                 int64
	CertificateID             pgtype.Int8
	FullNameSnapshot          string
	BirthdateSnapshot         pgtype.Date
	CompanyNameSnapshot       pgtype.Text
	SortOrder                 int32
	CreatedAt                 pgtype.Timestamptz
	CertificateDate           pgtype.Date
	CertificateRegistryYear   pgtype.Int8
	CertificateRegistryNumber any
	CertificateCourseSymbol   pgtype.Text
}

func mapJournalAttendeesRow(source journalAttendeeSource) JournalAttendeeDTO {
	var certificate *JournalAttendeeCertificateDTO
	if source.CertificateID.Valid {
		certificate = &JournalAttendeeCertificateDTO{
			ID:             source.CertificateID.Int64,
			Date:           source.CertificateDate.Time.Format(response.DateFormat),
			RegistryYear:   source.CertificateRegistryYear.Int64,
			RegistryNumber: registryNumberFromValue(source.CertificateRegistryNumber),
			CourseSymbol:   source.CertificateCourseSymbol.String,
		}
	}
	return JournalAttendeeDTO{
		ID:                  source.ID,
		JournalID:           source.JournalID,
		StudentID:           source.StudentID,
		FullNameSnapshot:    source.FullNameSnapshot,
		BirthdateSnapshot:   source.BirthdateSnapshot.Time.Format(response.DateFormat),
		CompanyNameSnapshot: pgutil.NullableString(source.CompanyNameSnapshot),
		Certificate:         certificate,
		SortOrder:           source.SortOrder,
		CreatedAt:           source.CreatedAt.Time.Format(response.TimestampzFormat),
	}
}

func mapJournalAttendeesRowFromList(row sqlc.ListJournalAttendeesRow) JournalAttendeeDTO {
	return mapJournalAttendeesRow(journalAttendeeSource{
		ID:                        row.ID,
		JournalID:                 row.JournalID,
		StudentID:                 row.StudentID,
		CertificateID:             row.CertificateID,
		FullNameSnapshot:          row.FullNameSnapshot,
		BirthdateSnapshot:         row.BirthdateSnapshot,
		CompanyNameSnapshot:       row.CompanyNameSnapshot,
		SortOrder:                 row.SortOrder,
		CreatedAt:                 row.CreatedAt,
		CertificateDate:           row.CertificateDate,
		CertificateRegistryYear:   row.CertificateRegistryYear,
		CertificateRegistryNumber: row.CertificateRegistryNumber,
		CertificateCourseSymbol:   row.CertificateCourseSymbol,
	})
}

func mapJournalAttendeesRowFromAdd(row sqlc.AddJournalAttendeeRow) JournalAttendeeDTO {
	return mapJournalAttendeesRow(journalAttendeeSource{
		ID:                        row.ID,
		JournalID:                 row.JournalID,
		StudentID:                 row.StudentID,
		CertificateID:             row.CertificateID,
		FullNameSnapshot:          row.FullNameSnapshot,
		BirthdateSnapshot:         row.BirthdateSnapshot,
		CompanyNameSnapshot:       row.CompanyNameSnapshot,
		SortOrder:                 row.SortOrder,
		CreatedAt:                 row.CreatedAt,
		CertificateDate:           row.CertificateDate,
		CertificateRegistryYear:   row.CertificateRegistryYear,
		CertificateRegistryNumber: row.CertificateRegistryNumber,
		CertificateCourseSymbol:   row.CertificateCourseSymbol,
	})
}

func mapJournalAttendeesRowFromUpdate(row sqlc.UpdateJournalAttendeeCertificateRow) JournalAttendeeDTO {
	return mapJournalAttendeesRow(journalAttendeeSource{
		ID:                        row.ID,
		JournalID:                 row.JournalID,
		StudentID:                 row.StudentID,
		CertificateID:             row.CertificateID,
		FullNameSnapshot:          row.FullNameSnapshot,
		BirthdateSnapshot:         row.BirthdateSnapshot,
		CompanyNameSnapshot:       row.CompanyNameSnapshot,
		SortOrder:                 row.SortOrder,
		CreatedAt:                 row.CreatedAt,
		CertificateDate:           row.CertificateDate,
		CertificateRegistryYear:   row.CertificateRegistryYear,
		CertificateRegistryNumber: row.CertificateRegistryNumber,
		CertificateCourseSymbol:   row.CertificateCourseSymbol,
	})
}

func mapJournalSessionRow(row sqlc.TrainingJournalSession) JournalSessionDTO {
	return JournalSessionDTO{
		ID:          row.ID,
		JournalID:   row.JournalID,
		SessionDate: row.SessionDate.Time.Format(response.DateFormat),
		StartTime:   formatTimeValue(row.StartTime),
		EndTime:     formatTimeValue(row.EndTime),
		Hours:       formatNumeric(row.Hours),
		Topic:       row.Topic,
		TrainerName: row.TrainerName,
		SortOrder:   row.SortOrder,
		CreatedAt:   row.CreatedAt.Time.Format(response.TimestampzFormat),
	}
}

func mapJournalAttendanceRow(row sqlc.TrainingJournalAttendance) JournalAttendanceDTO {
	return JournalAttendanceDTO{
		ID:                row.ID,
		JournalSessionID:  row.JournalSessionID,
		JournalAttendeeID: row.JournalAttendeeID,
		Present:           row.Present,
		CreatedAt:         row.CreatedAt.Time.Format(response.TimestampzFormat),
		UpdatedAt:         row.UpdatedAt.Time.Format(response.TimestampzFormat),
	}
}

func formatNumeric(value pgtype.Numeric) string {
	if !value.Valid {
		return ""
	}

	raw, err := value.MarshalJSON()
	if err != nil {
		return ""
	}

	return string(raw)
}

func numericToFloat64(value pgtype.Numeric) float64 {
	if !value.Valid {
		return 0
	}

	converted, err := value.Float64Value()
	if err != nil || !converted.Valid {
		return 0
	}

	return converted.Float64
}

func formatTimeValue(value pgtype.Time) *string {
	if !value.Valid {
		return nil
	}

	formatted := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC).
		Add(time.Duration(value.Microseconds) * time.Microsecond).
		Format("15:04:05")

	return &formatted
}

func registryNumberFromValue(value any) int64 {
	switch typed := value.(type) {
	case nil:
		return 0
	case int64:
		return typed
	case int32:
		return validation.SignedToInt64Clamped(typed)
	case int:
		return validation.SignedToInt64Clamped(typed)
	case uint64:
		return validation.UnsignedToInt64Clamped(typed)
	case uint32:
		return validation.UnsignedToInt64Clamped(typed)
	case []byte:
		parsed, err := strconv.ParseInt(string(typed), 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		parsed, err := strconv.ParseInt(fmt.Sprint(typed), 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	}
}
