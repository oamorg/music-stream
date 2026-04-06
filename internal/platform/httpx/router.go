package httpx

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"music-stream/internal/platform/config"
	"music-stream/internal/platform/metrics"
)

type healthResponse struct {
	Status      string            `json:"status"`
	Service     string            `json:"service"`
	Environment string            `json:"environment"`
	Timestamp   time.Time         `json:"timestamp"`
	Checks      map[string]bool   `json:"checks,omitempty"`
}

type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

type Readiness interface {
	Check(ctx context.Context) (bool, map[string]bool)
}

type Dependencies struct {
	Auth      RouteRegistrar
	Catalog   RouteRegistrar
	Playback  RouteRegistrar
	History   RouteRegistrar
	Readiness Readiness
}

func NewRouter(logger *log.Logger, cfg config.Config, deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	registry := metrics.NewRegistry(cfg.ServiceName)

	if cfg.LocalMediaRoot != "" {
		fileServer := http.StripPrefix("/media/", http.FileServer(http.Dir(cfg.LocalMediaRoot)))
		mux.Handle("/media/", fileServer)
	}

	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{
			Status:      "ok",
			Service:     cfg.ServiceName,
			Environment: cfg.AppEnv,
			Timestamp:   time.Now().UTC(),
		})
	})

	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		statusCode := http.StatusOK
		status := "ok"
		checks := map[string]bool{}
		if deps.Readiness != nil {
			ready, results := deps.Readiness.Check(r.Context())
			checks = results
			registry.SetDependencies(results)
			if !ready {
				statusCode = http.StatusServiceUnavailable
				status = "not_ready"
			}
		}

		writeJSON(w, statusCode, healthResponse{
			Status:      status,
			Service:     cfg.ServiceName,
			Environment: cfg.AppEnv,
			Timestamp:   time.Now().UTC(),
			Checks:      checks,
		})
	})

	register(mux, deps.Auth, deps.Catalog, deps.Playback, deps.History)
	mux.Handle("/metrics", registry.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": cfg.ServiceName,
			"status":  "bootstrapped",
			"message": "MVP scaffold is ready. Implement module routes in Phase 1+.",
		})
	})

	return withRequestLogging(logger, mux, registry)
}

func withRequestLogging(logger *log.Logger, next http.Handler, registry *metrics.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}

		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		recorder.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(recorder, r)
		if registry != nil {
			registry.ObserveRequest(r.Method, metricsPathLabel(r.URL.Path), recorder.statusCode, time.Since(start))
		}

		logger.Printf(
			"request_id=%s method=%s path=%s status_code=%d latency_ms=%s remote_addr=%s user_agent=%q",
			requestID,
			r.Method,
			r.URL.Path,
			recorder.statusCode,
			strconv.FormatInt(time.Since(start).Milliseconds(), 10),
			r.RemoteAddr,
			r.UserAgent(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	WriteJSON(w, status, payload)
}

func register(mux *http.ServeMux, registrars ...RouteRegistrar) {
	for _, registrar := range registrars {
		if registrar != nil {
			registrar.RegisterRoutes(mux)
		}
	}
}

func metricsPathLabel(path string) string {
	switch {
	case path == "/":
		return "/"
	case path == "/health/live":
		return "/health/live"
	case path == "/health/ready":
		return "/health/ready"
	case path == "/metrics":
		return "/metrics"
	case path == "/api/v1/auth/register":
		return "/api/v1/auth/register"
	case path == "/api/v1/auth/login":
		return "/api/v1/auth/login"
	case path == "/api/v1/auth/refresh":
		return "/api/v1/auth/refresh"
	case path == "/api/v1/auth/logout":
		return "/api/v1/auth/logout"
	case path == "/api/v1/tracks":
		return "/api/v1/tracks"
	case strings.HasPrefix(path, "/api/v1/tracks/"):
		return "/api/v1/tracks/{trackId}"
	case path == "/api/v1/search":
		return "/api/v1/search"
	case path == "/api/v1/playback/sessions":
		return "/api/v1/playback/sessions"
	case path == "/api/v1/playback/events":
		return "/api/v1/playback/events"
	case path == "/api/v1/me/history":
		return "/api/v1/me/history"
	case strings.HasPrefix(path, "/media/"):
		return "/media/*"
	default:
		return path
	}
}
