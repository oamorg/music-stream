package playback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"music-stream/internal/auth"
	"music-stream/internal/catalog"
	"music-stream/internal/media"
	"music-stream/internal/platform/httpx"
)

func TestHandlerCreateSessionAuthorized(t *testing.T) {
	now := time.Unix(1712360000, 0).UTC()
	token, authenticator := issuePlaybackAccessToken(t, now)
	manifestKey := "hls/asset-8/index.m3u8"

	service := NewService(
		stubRepository{
			findActiveEntitlementFunc: func(context.Context, int64, int64, time.Time) (UserEntitlement, error) {
				return UserEntitlement{ID: 1, UserID: 1, TrackID: 9, AccessType: AccessTypeStream}, nil
			},
			createSessionFunc: func(_ context.Context, userID, trackID, assetID int64, manifestURL string, expiresAt time.Time) (PlaybackSession, error) {
				return PlaybackSession{
					ID:          11,
					UserID:      userID,
					TrackID:     trackID,
					AssetID:     assetID,
					ManifestURL: manifestURL,
					ExpiresAt:   expiresAt,
					CreatedAt:   now,
				}, nil
			},
		},
		stubCatalogReader{
			findReadyByIDFunc: func(_ context.Context, trackID int64) (catalog.Track, error) {
				return catalog.Track{ID: trackID, Status: catalog.TrackStatusReady}, nil
			},
		},
		stubMediaReader{
			findReadyAssetByTrackIDFunc: func(_ context.Context, trackID int64) (media.TrackAsset, error) {
				return media.TrackAsset{ID: 8, TrackID: trackID, Status: media.TrackAssetStatusReady, HLSManifestKey: &manifestKey}, nil
			},
		},
		"http://localhost:8080/media",
		10*time.Minute,
		func() time.Time { return now },
	)

	handler := NewHandler(service, authenticator, HandlerOptions{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/playback/sessions", strings.NewReader(`{"trackId":9}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var session PlaybackSession
	if err := json.Unmarshal(recorder.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if session.TrackID != 9 {
		t.Fatalf("track id = %d, want %d", session.TrackID, 9)
	}
	if !strings.Contains(session.ManifestURL, "/media/hls/asset-8/index.m3u8") {
		t.Fatalf("manifest url = %q, want media manifest path", session.ManifestURL)
	}
}

func TestHandlerReportEventRateLimited(t *testing.T) {
	now := time.Unix(1712360000, 0).UTC()
	token, authenticator := issuePlaybackAccessToken(t, now)

	service := NewService(
		stubRepository{
			findSessionByIDAndUserFunc: func(_ context.Context, sessionID, userID int64) (PlaybackSession, error) {
				return PlaybackSession{ID: sessionID, UserID: userID, TrackID: 9}, nil
			},
			createEventFunc: func(_ context.Context, input ReportEventInput, userID, trackID int64) (PlayEvent, error) {
				return PlayEvent{
					ID:              1,
					SessionID:       input.SessionID,
					UserID:          userID,
					TrackID:         trackID,
					EventType:       input.EventType,
					PositionSec:     input.PositionSec,
					ClientTimestamp: input.ClientTimestamp,
				}, nil
			},
		},
		stubCatalogReader{},
		stubMediaReader{},
		"http://localhost:8080/media",
		10*time.Minute,
		func() time.Time { return now },
	)

	handler := NewHandler(service, authenticator, HandlerOptions{
		EventLimiter: httpx.NewFixedWindowRateLimiter(1, time.Minute, func() time.Time { return now }),
	})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	first := httptest.NewRequest(http.MethodPost, "/api/v1/playback/events", strings.NewReader(`{"sessionId":12,"eventType":"START","positionSec":0,"clientTimestamp":"2026-04-06T12:00:00Z"}`))
	first.RemoteAddr = "127.0.0.1:9000"
	first.Header.Set("Authorization", "Bearer "+token)
	first.Header.Set("Content-Type", "application/json")
	firstRecorder := httptest.NewRecorder()
	mux.ServeHTTP(firstRecorder, first)

	if firstRecorder.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, want %d", firstRecorder.Code, http.StatusAccepted)
	}

	second := httptest.NewRequest(http.MethodPost, "/api/v1/playback/events", strings.NewReader(`{"sessionId":12,"eventType":"HEARTBEAT","positionSec":30,"clientTimestamp":"2026-04-06T12:00:30Z"}`))
	second.RemoteAddr = "127.0.0.1:9000"
	second.Header.Set("Authorization", "Bearer "+token)
	second.Header.Set("Content-Type", "application/json")
	secondRecorder := httptest.NewRecorder()
	mux.ServeHTTP(secondRecorder, second)

	if secondRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", secondRecorder.Code, http.StatusTooManyRequests)
	}
}

func issuePlaybackAccessToken(t *testing.T, now time.Time) (string, *auth.Authenticator) {
	t.Helper()

	users := auth.NewInMemoryUserRepository()
	user, err := users.Create(context.Background(), auth.CreateUserParams{
		Email:        "user@example.com",
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

	return token, auth.NewAuthenticator(users, manager, func() time.Time { return now })
}
