package httpx

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"music-stream/internal/platform/config"
)

type fakeReadiness struct {
	ready  bool
	checks map[string]bool
}

func (f fakeReadiness) Check(context.Context) (bool, map[string]bool) {
	return f.ready, f.checks
}

func TestNewRouterExposesMetricsForRequestsAndDependencies(t *testing.T) {
	router := NewRouter(log.New(io.Discard, "", 0), config.Config{
		ServiceName: "music-stream",
		AppEnv:      "test",
	}, Dependencies{
		Readiness: fakeReadiness{
			ready: false,
			checks: map[string]bool{
				"database": true,
				"redis":    false,
			},
		},
	})

	live := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	liveRecorder := httptest.NewRecorder()
	router.ServeHTTP(liveRecorder, live)

	ready := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	readyRecorder := httptest.NewRecorder()
	router.ServeHTTP(readyRecorder, ready)

	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRecorder := httptest.NewRecorder()
	router.ServeHTTP(metricsRecorder, metricsRequest)

	if metricsRecorder.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", metricsRecorder.Code, http.StatusOK)
	}

	body := metricsRecorder.Body.String()
	if !strings.Contains(body, `http_requests_total{service="music-stream",method="GET",path="/health/live",status="200"} 1`) {
		t.Fatalf("/metrics missing live counter:\n%s", body)
	}
	if !strings.Contains(body, `http_requests_total{service="music-stream",method="GET",path="/health/ready",status="503"} 1`) {
		t.Fatalf("/metrics missing readiness counter:\n%s", body)
	}
	if !strings.Contains(body, `dependency_up{service="music-stream",dependency="database"} 1`) {
		t.Fatalf("/metrics missing database dependency gauge:\n%s", body)
	}
	if !strings.Contains(body, `dependency_up{service="music-stream",dependency="redis"} 0`) {
		t.Fatalf("/metrics missing redis dependency gauge:\n%s", body)
	}
}

func TestMetricsPathLabel(t *testing.T) {
	tests := map[string]string{
		"/api/v1/tracks/123":       "/api/v1/tracks/{trackId}",
		"/media/hls/asset-1/a.ts":  "/media/*",
		"/api/v1/playback/events":  "/api/v1/playback/events",
		"/unknown/path":            "/unknown/path",
	}

	for input, want := range tests {
		if got := metricsPathLabel(input); got != want {
			t.Fatalf("metricsPathLabel(%q) = %q, want %q", input, got, want)
		}
	}
}
