package history

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"music-stream/internal/auth"
)

func TestHandlerListHistoryAuthorized(t *testing.T) {
	now := time.Unix(1712360000, 0).UTC()
	token, authenticator, userID := issueHistoryAccessToken(t, now)

	var gotUserID int64
	service := NewService(stubRepository{
		listRecentByUserFunc: func(_ context.Context, currentUserID int64, limit int) ([]Item, error) {
			gotUserID = currentUserID
			if limit != 10 {
				t.Fatalf("limit = %d, want %d", limit, 10)
			}
			return []Item{{
				TrackID:         9,
				Title:           "Song A",
				ArtistName:      "Artist A",
				LastPlayedAt:    now,
				LastEventType:   "HEARTBEAT",
				LastPositionSec: 30,
			}}, nil
		},
	})

	handler := NewHandler(service, authenticator)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/history?limit=10", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if gotUserID != userID {
		t.Fatalf("user id = %d, want %d", gotUserID, userID)
	}

	var payload struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].TrackID != 9 {
		t.Fatalf("items = %#v, want one history item for track 9", payload.Items)
	}
}

func TestHandlerListHistoryUnauthorized(t *testing.T) {
	handler := NewHandler(NewService(stubRepository{
		listRecentByUserFunc: func(context.Context, int64, int) ([]Item, error) {
			t.Fatalf("repository should not be called for unauthorized request")
			return nil, nil
		},
	}), auth.NewAuthenticator(auth.NewInMemoryUserRepository(), auth.NewTokenManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour), time.Now))

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/history", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func issueHistoryAccessToken(t *testing.T, now time.Time) (string, *auth.Authenticator, int64) {
	t.Helper()

	users := auth.NewInMemoryUserRepository()
	user, err := users.Create(context.Background(), auth.CreateUserParams{
		Email:        "history@example.com",
		PasswordHash: "irrelevant",
		Status:       auth.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	manager := auth.NewTokenManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour)
	token, _, err := manager.IssueAccessToken(user, now)
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	return token, auth.NewAuthenticator(users, manager, func() time.Time { return now }), user.ID
}
