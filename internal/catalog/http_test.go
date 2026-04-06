package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"music-stream/internal/platform/store"
)

func TestHandlerSearchTracks(t *testing.T) {
	releaseDate := time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)
	service := NewService(nil, stubRepository{
		searchReadyFunc: func(_ context.Context, _ store.DBTX, query string, limit int) ([]Track, error) {
			if query != "Song" {
				t.Fatalf("query = %q, want %q", query, "Song")
			}
			if limit != 5 {
				t.Fatalf("limit = %d, want %d", limit, 5)
			}
			return []Track{{
				ID:          9,
				Title:       "Song A",
				ArtistName:  "Artist A",
				DurationSec: 215,
				ReleaseDate: &releaseDate,
				Status:      TrackStatusReady,
			}}, nil
		},
	})

	handler := NewHandler(service)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=Song&limit=5", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload struct {
		Query string  `json:"query"`
		Items []Track `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Query != "Song" {
		t.Fatalf("query = %q, want %q", payload.Query, "Song")
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != 9 {
		t.Fatalf("items = %#v, want one track with id 9", payload.Items)
	}
}

func TestHandlerGetTrack(t *testing.T) {
	service := NewService(nil, stubRepository{
		findReadyByIDFunc: func(_ context.Context, _ store.DBTX, trackID int64) (Track, error) {
			return Track{
				ID:          trackID,
				Title:       "Song A",
				ArtistName:  "Artist A",
				DurationSec: 215,
				Status:      TrackStatusReady,
			}, nil
		},
	})

	handler := NewHandler(service)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/tracks/12", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload Track
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ID != 12 {
		t.Fatalf("track id = %d, want %d", payload.ID, 12)
	}
}

func TestHandlerGetTrackRejectsInvalidID(t *testing.T) {
	handler := NewHandler(NewService(nil, stubRepository{}))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/tracks/not-a-number", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandlerGetTrackReturnsNotFound(t *testing.T) {
	service := NewService(nil, stubRepository{
		findReadyByIDFunc: func(_ context.Context, _ store.DBTX, trackID int64) (Track, error) {
			return Track{}, ErrNotFound
		},
	})

	handler := NewHandler(service)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/tracks/12", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
