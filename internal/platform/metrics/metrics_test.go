package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestRegistryRenderIncludesRequestsAndDependencies(t *testing.T) {
	registry := NewRegistry("music-stream")
	registry.ObserveRequest("GET", "/health/live", 200, 50*time.Millisecond)
	registry.ObserveRequest("POST", "/api/v1/auth/login", 429, 120*time.Millisecond)
	registry.SetDependencies(map[string]bool{
		"database": true,
		"redis":    false,
	})

	output := registry.render()

	assertContains(t, output, `app_build_info{service="music-stream",version="dev"} 1`)
	assertContains(t, output, `http_requests_total{service="music-stream",method="GET",path="/health/live",status="200"} 1`)
	assertContains(t, output, `http_requests_total{service="music-stream",method="POST",path="/api/v1/auth/login",status="429"} 1`)
	assertContains(t, output, `http_request_duration_seconds_count{service="music-stream",method="GET",path="/health/live"} 1`)
	assertContains(t, output, `dependency_up{service="music-stream",dependency="database"} 1`)
	assertContains(t, output, `dependency_up{service="music-stream",dependency="redis"} 0`)
}

func TestRegistryRenderUsesCumulativeHistogramBuckets(t *testing.T) {
	registry := NewRegistry("music-stream")
	registry.ObserveRequest("GET", "/api/v1/search", 200, 50*time.Millisecond)

	output := registry.render()

	assertContains(t, output, `http_request_duration_seconds_bucket{service="music-stream",method="GET",path="/api/v1/search",le="0.025"} 0`)
	assertContains(t, output, `http_request_duration_seconds_bucket{service="music-stream",method="GET",path="/api/v1/search",le="0.05"} 1`)
	assertContains(t, output, `http_request_duration_seconds_bucket{service="music-stream",method="GET",path="/api/v1/search",le="+Inf"} 1`)
}

func assertContains(t *testing.T, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("output does not contain %q\noutput:\n%s", want, output)
	}
}
