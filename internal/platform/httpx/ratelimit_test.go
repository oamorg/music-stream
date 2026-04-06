package httpx

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestFixedWindowRateLimiter(t *testing.T) {
	current := time.Unix(1712360000, 0).UTC()
	limiter := NewFixedWindowRateLimiter(2, time.Minute, func() time.Time { return current })

	if !limiter.Allow("client-a") {
		t.Fatalf("first request rejected unexpectedly")
	}
	if !limiter.Allow("client-a") {
		t.Fatalf("second request rejected unexpectedly")
	}
	if limiter.Allow("client-a") {
		t.Fatalf("third request allowed unexpectedly")
	}

	current = current.Add(time.Minute)
	if !limiter.Allow("client-a") {
		t.Fatalf("window reset did not allow next request")
	}
}

func TestClientIP(t *testing.T) {
	request := httptest.NewRequest("GET", "/health/live", nil)
	request.RemoteAddr = "127.0.0.1:8080"
	request.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")

	if got := ClientIP(request); got != "203.0.113.10" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.10")
	}
}
