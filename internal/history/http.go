package history

import (
	"net/http"
	"strconv"

	"music-stream/internal/auth"
	"music-stream/internal/platform/httpx"
)

type Handler struct {
	service       *Service
	authenticator *auth.Authenticator
}

func NewHandler(service *Service, authenticator *auth.Authenticator) *Handler {
	return &Handler{
		service:       service,
		authenticator: authenticator,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/v1/me/history", h.authenticator.Require(h.handleListHistory))
}

func (h *Handler) handleListHistory(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodGet {
		writeHistoryError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	items, err := h.service.ListRecentByUser(r.Context(), user.ID, historyLimit(r))
	if err != nil {
		writeHistoryError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	writeHistoryJSON(w, http.StatusOK, map[string]any{"items": items})
}

func historyLimit(r *http.Request) int {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return DefaultHistoryLimit
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return DefaultHistoryLimit
	}

	return parsed
}

func writeHistoryJSON(w http.ResponseWriter, status int, payload any) {
	httpx.WriteJSON(w, status, payload)
}

func writeHistoryError(w http.ResponseWriter, status int, code, message string) {
	httpx.WriteError(w, status, code, message)
}
