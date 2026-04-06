package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"music-stream/internal/platform/httpx"
)

func TestHandlerLoginRateLimited(t *testing.T) {
	now := time.Unix(1712360000, 0).UTC()
	service := NewService(
		NewInMemoryUserRepository(),
		NewInMemoryRefreshTokenRepository(),
		NewPasswordHasher(1000, 32, 16),
		NewTokenManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour),
		func() time.Time { return now },
	)

	if _, err := service.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "super-secret-password",
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	handler := NewHandler(service, HandlerOptions{
		LoginLimiter: httpx.NewFixedWindowRateLimiter(1, time.Minute, func() time.Time { return now }),
	})

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	first := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"super-secret-password"}`))
	first.RemoteAddr = "127.0.0.1:9000"
	first.Header.Set("Content-Type", "application/json")
	firstRecorder := httptest.NewRecorder()
	mux.ServeHTTP(firstRecorder, first)

	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first login status = %d, want %d", firstRecorder.Code, http.StatusOK)
	}

	second := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"super-secret-password"}`))
	second.RemoteAddr = "127.0.0.1:9000"
	second.Header.Set("Content-Type", "application/json")
	secondRecorder := httptest.NewRecorder()
	mux.ServeHTTP(secondRecorder, second)

	if secondRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("second login status = %d, want %d", secondRecorder.Code, http.StatusTooManyRequests)
	}
}
