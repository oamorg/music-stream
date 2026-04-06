package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"music-stream/internal/platform/httpx"
)

type Handler struct {
	service      *Service
	loginLimiter *httpx.FixedWindowRateLimiter
}

type HandlerOptions struct {
	LoginLimiter *httpx.FixedWindowRateLimiter
}

func NewHandler(service *Service, opts HandlerOptions) *Handler {
	return &Handler{
		service:      service,
		loginLimiter: opts.LoginLimiter,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/register", h.handleRegister)
	mux.HandleFunc("/api/v1/auth/login", h.handleLogin)
	mux.HandleFunc("/api/v1/auth/refresh", h.handleRefresh)
	mux.HandleFunc("/api/v1/auth/logout", h.handleLogout)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var input RegisterInput
	if err := decodeJSON(r, &input); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	user, err := h.service.Register(r.Context(), input)
	if err != nil {
		writeMappedAuthError(w, err)
		return
	}

	writeAuthJSON(w, http.StatusCreated, map[string]any{
		"user": sanitizeUser(user),
	})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.loginLimiter != nil && !h.loginLimiter.Allow(httpx.ClientIP(r)) {
		writeAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many login attempts")
		return
	}

	var input LoginInput
	if err := decodeJSON(r, &input); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	result, err := h.service.Login(r.Context(), input)
	if err != nil {
		writeMappedAuthError(w, err)
		return
	}

	writeAuthJSON(w, http.StatusOK, result)
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var input RefreshInput
	if err := decodeJSON(r, &input); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	result, err := h.service.Refresh(r.Context(), input)
	if err != nil {
		writeMappedAuthError(w, err)
		return
	}

	writeAuthJSON(w, http.StatusOK, result)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var input LogoutInput
	if err := decodeJSON(r, &input); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	if err := h.service.Logout(r.Context(), input); err != nil {
		writeMappedAuthError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func writeMappedAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidEmail):
		writeAuthError(w, http.StatusBadRequest, "invalid_email", "email is invalid")
	case errors.Is(err, ErrWeakPassword):
		writeAuthError(w, http.StatusBadRequest, "weak_password", "password must be at least 8 characters")
	case errors.Is(err, ErrEmailAlreadyExists):
		writeAuthError(w, http.StatusConflict, "email_exists", "email already exists")
	case errors.Is(err, ErrInvalidCredentials):
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, ErrInvalidRefreshToken):
		writeAuthError(w, http.StatusUnauthorized, "invalid_refresh_token", "refresh token is invalid or expired")
	case errors.Is(err, ErrUserDisabled):
		writeAuthError(w, http.StatusForbidden, "user_disabled", "user is disabled")
	default:
		writeAuthError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	httpx.WriteError(w, status, code, message)
}

func writeAuthJSON(w http.ResponseWriter, status int, payload any) {
	httpx.WriteJSON(w, status, payload)
}
