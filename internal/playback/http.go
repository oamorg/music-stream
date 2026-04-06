package playback

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"music-stream/internal/auth"
	"music-stream/internal/catalog"
	"music-stream/internal/platform/httpx"
)

type Handler struct {
	service       *Service
	authenticator *auth.Authenticator
	eventLimiter  *httpx.FixedWindowRateLimiter
}

type HandlerOptions struct {
	EventLimiter *httpx.FixedWindowRateLimiter
}

func NewHandler(service *Service, authenticator *auth.Authenticator, opts HandlerOptions) *Handler {
	return &Handler{
		service:       service,
		authenticator: authenticator,
		eventLimiter:  opts.EventLimiter,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/v1/playback/sessions", h.authenticator.Require(h.handleCreateSession))
	mux.Handle("/api/v1/playback/events", h.authenticator.Require(h.handleReportEvent))
}

func (h *Handler) handleCreateSession(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		writePlaybackError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var input CreateSessionInput
	if err := decodePlaybackJSON(r, &input); err != nil {
		writePlaybackError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	session, err := h.service.CreateSession(r.Context(), user, input)
	if err != nil {
		writeMappedPlaybackError(w, err)
		return
	}

	writePlaybackJSON(w, http.StatusOK, session)
}

func (h *Handler) handleReportEvent(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		writePlaybackError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.eventLimiter != nil && !h.eventLimiter.Allow(playbackEventRateLimitKey(r, user)) {
		writePlaybackError(w, http.StatusTooManyRequests, "rate_limited", "too many playback events")
		return
	}

	var input ReportEventInput
	if err := decodePlaybackJSON(r, &input); err != nil {
		writePlaybackError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	event, err := h.service.ReportEvent(r.Context(), user, input)
	if err != nil {
		writeMappedPlaybackError(w, err)
		return
	}

	writePlaybackJSON(w, http.StatusAccepted, event)
}

func decodePlaybackJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func writeMappedPlaybackError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidTrackID):
		writePlaybackError(w, http.StatusBadRequest, "invalid_track_id", "track id must be a positive integer")
	case errors.Is(err, ErrInvalidSessionID):
		writePlaybackError(w, http.StatusBadRequest, "invalid_session_id", "session id must be a positive integer")
	case errors.Is(err, ErrInvalidEventType):
		writePlaybackError(w, http.StatusBadRequest, "invalid_event_type", "event type is invalid")
	case errors.Is(err, ErrInvalidPosition):
		writePlaybackError(w, http.StatusBadRequest, "invalid_position", "position must be zero or positive")
	case errors.Is(err, ErrInvalidClientTime):
		writePlaybackError(w, http.StatusBadRequest, "invalid_client_timestamp", "client timestamp is required")
	case errors.Is(err, catalog.ErrNotFound):
		writePlaybackError(w, http.StatusNotFound, "track_not_found", "track not found")
	case errors.Is(err, ErrForbidden):
		writePlaybackError(w, http.StatusForbidden, "forbidden", "user does not have playback permission")
	case errors.Is(err, ErrManifestUnavailable):
		writePlaybackError(w, http.StatusConflict, "manifest_unavailable", "track manifest is not ready")
	default:
		writePlaybackError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writePlaybackJSON(w http.ResponseWriter, status int, payload any) {
	httpx.WriteJSON(w, status, payload)
}

func writePlaybackError(w http.ResponseWriter, status int, code, message string) {
	httpx.WriteError(w, status, code, message)
}

func playbackEventRateLimitKey(r *http.Request, user auth.User) string {
	return fmt.Sprintf("%d:%s", user.ID, httpx.ClientIP(r))
}
