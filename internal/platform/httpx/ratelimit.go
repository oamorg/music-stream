package httpx

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type FixedWindowRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	buckets map[string]fixedWindowBucket
}

type fixedWindowBucket struct {
	startedAt time.Time
	count     int
}

func NewFixedWindowRateLimiter(limit int, window time.Duration, now func() time.Time) *FixedWindowRateLimiter {
	if now == nil {
		now = time.Now
	}

	return &FixedWindowRateLimiter{
		limit:   limit,
		window:  window,
		now:     now,
		buckets: make(map[string]fixedWindowBucket),
	}
}

func (l *FixedWindowRateLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 || l.window <= 0 {
		return true
	}

	key = strings.TrimSpace(key)
	if key == "" {
		key = "anonymous"
	}

	now := l.now().UTC()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.compact(now)

	bucket := l.buckets[key]
	if bucket.startedAt.IsZero() || now.Sub(bucket.startedAt) >= l.window {
		l.buckets[key] = fixedWindowBucket{
			startedAt: now,
			count:     1,
		}
		return true
	}

	if bucket.count >= l.limit {
		return false
	}

	bucket.count++
	l.buckets[key] = bucket
	return true
}

func (l *FixedWindowRateLimiter) compact(now time.Time) {
	if len(l.buckets) < 1024 {
		return
	}

	expireBefore := now.Add(-2 * l.window)
	for key, bucket := range l.buckets {
		if bucket.startedAt.Before(expireBefore) {
			delete(l.buckets, key)
		}
	}
}

func ClientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	if host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil && host != "" {
		return host
	}

	if remoteAddr := strings.TrimSpace(r.RemoteAddr); remoteAddr != "" {
		return remoteAddr
	}

	return "unknown"
}
