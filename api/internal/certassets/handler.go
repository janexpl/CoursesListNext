package certassets

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/guilloche"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	ListCertificatePrintAssetsMeta(ctx context.Context) ([]sqlc.ListCertificatePrintAssetsMetaRow, error)
	GetCertificatePrintAssetFile(ctx context.Context, kind string) (sqlc.GetCertificatePrintAssetFileRow, error)
}

type Handler struct {
	querier Querier
	service *Service
}

func NewHandler(querier Querier, service *Service) *Handler {
	return &Handler{querier: querier, service: service}
}

// List zwraca metadane wgranych nadruków - bez bajtów, bo lista służy do pokazania,
// co jest wgrane, a nie do rysowania.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.querier.ListCertificatePrintAssetsMeta(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list print assets")
		return
	}

	assets := make([]PrintAssetDTO, 0, len(rows))
	for _, row := range rows {
		assets = append(assets, PrintAssetDTO{
			Kind:         row.Kind,
			FileName:     row.FileName,
			ContentType:  row.ContentType,
			FileSize:     row.FileSize,
			PrintWidthMm: int(row.PrintWidthMm),
			UploadedAt:   row.UpdatedAt.Time.Format(response.TimestampzFormat),
		})
	}

	response.WriteJSON(w, http.StatusOK, ListPrintAssetsResponse{Data: assets})
}

// GetFile zwraca sam obraz. Używa go podgląd zaświadczenia w przeglądarce, żeby pokazać
// dokładnie to, co wydrukuje serwer.
func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if !IsValidKind(kind) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid print asset kind")
		return
	}

	row, err := h.querier.GetCertificatePrintAssetFile(r.Context(), kind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "print asset not found")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to get print asset")
		return
	}

	w.Header().Set("Content-Type", row.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(row.FileData); err != nil {
		log.Printf("failed to write print asset %s: %v", kind, err)
	}
}

// Guilloche zwraca wzór tła. Nie ma go w bazie (jest wkompilowany w binarkę), więc trasa
// jest jedynym sposobem, żeby podgląd w przeglądarce pokazał to samo co wydruk.
// Domyślnie przód; ?side=back zwraca wzór odwrotu.
func (h *Handler) Guilloche(w http.ResponseWriter, r *http.Request) {
	pattern := guilloche.FrontPNG()
	if r.URL.Query().Get("side") == "back" {
		pattern = guilloche.BackPNG()
	}

	w.Header().Set("Content-Type", "image/png")
	// Deseń jest deterministyczny i nie zmienia się między wdrożeniami.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(pattern); err != nil {
		log.Printf("failed to write guilloche: %v", err)
	}
}

// Upsert przyjmuje plik z formularza multipart, sprowadza go do PNG w rozsądnej
// rozdzielczości i zapisuje. Wzorzec jak przy skanach dzienników.
func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if !IsValidKind(kind) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid print asset kind")
		return
	}

	if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "file is required")
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, MaxUploadBytes+1))
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "unable to read file")
		return
	}
	if int64(len(raw)) > MaxUploadBytes {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "file is too large")
		return
	}

	// Walidacja przez faktyczne zdekodowanie obrazu, nie przez http.DetectContentType:
	// plik idzie na wydruk, więc musi dać się narysować, a nie tylko wyglądać na obraz.
	normalized, _, _, err := Normalize(raw)
	if err != nil {
		if errors.Is(err, ErrImageTooLarge) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "image is too large")
			return
		}
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "unsupported file type")
		return
	}

	width, err := parseWidth(r.FormValue("printWidthMm"), kind)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid print width")
		return
	}

	row, err := h.service.Upsert(r.Context(), kind, UpsertInput{
		FileName:     header.Filename,
		FileData:     normalized,
		PrintWidthMm: width,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid print asset")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to save print asset")
		return
	}

	response.WriteJSON(w, http.StatusOK, PrintAssetResponse{Data: PrintAssetDTO{
		Kind:         row.Kind,
		FileName:     row.FileName,
		ContentType:  row.ContentType,
		FileSize:     row.FileSize,
		PrintWidthMm: int(row.PrintWidthMm),
		UploadedAt:   row.UpdatedAt.Time.Format(response.TimestampzFormat),
	}})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if !IsValidKind(kind) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid print asset kind")
		return
	}

	if err := h.service.Delete(r.Context(), kind); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "print asset not found")
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid print asset kind")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to delete print asset")
		return
	}

	response.WriteNoContent(w)
}

// parseWidth przyjmuje pustą wartość jako "zostaw domyślną dla tego rodzaju" -
// administrator nie musi znać milimetrów, żeby wgrać pieczątkę.
func parseWidth(value, kind string) (int, error) {
	if value == "" {
		return DefaultWidthMM(kind), nil
	}
	width, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if width < minWidthMM || width > maxWidthMM {
		return 0, ErrInvalidInput
	}
	return width, nil
}
