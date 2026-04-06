package catalog

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"music-stream/internal/platform/httpx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/tracks", h.handleListTracks)
	mux.HandleFunc("/api/v1/tracks/", h.handleGetTrack)
	mux.HandleFunc("/api/v1/search", h.handleSearchTracks)
}

func (h *Handler) handleListTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCatalogError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tracks, err := h.service.ListReady(r.Context(), queryInt(r, "limit", DefaultPageSize))
	if err != nil {
		writeCatalogError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	writeCatalogJSON(w, http.StatusOK, map[string]any{"items": tracks})
}

func (h *Handler) handleGetTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCatalogError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	trackID, err := parseTrackID(r.URL.Path)
	if err != nil {
		writeCatalogError(w, http.StatusBadRequest, "invalid_track_id", "track id must be a positive integer")
		return
	}

	track, err := h.service.FindReadyByID(r.Context(), trackID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeCatalogError(w, http.StatusNotFound, "track_not_found", "track not found")
			return
		}
		writeCatalogError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	writeCatalogJSON(w, http.StatusOK, track)
}

func (h *Handler) handleSearchTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCatalogError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	tracks, err := h.service.SearchReady(r.Context(), query, queryInt(r, "limit", DefaultPageSize))
	if err != nil {
		writeCatalogError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	writeCatalogJSON(w, http.StatusOK, map[string]any{
		"query": query,
		"items": tracks,
	})
}

func parseTrackID(path string) (int64, error) {
	idPart := strings.TrimPrefix(path, "/api/v1/tracks/")
	idPart = strings.TrimSpace(idPart)
	return strconv.ParseInt(idPart, 10, 64)
}

func queryInt(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func writeCatalogJSON(w http.ResponseWriter, status int, payload any) {
	httpx.WriteJSON(w, status, payload)
}

func writeCatalogError(w http.ResponseWriter, status int, code, message string) {
	httpx.WriteError(w, status, code, message)
}
