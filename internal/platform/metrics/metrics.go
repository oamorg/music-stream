package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var requestDurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type Registry struct {
	mu               sync.Mutex
	serviceName      string
	requestTotals    map[requestKey]uint64
	requestDurations map[durationKey]*durationMetric
	dependencies     map[string]bool
}

type requestKey struct {
	Method string
	Path   string
	Status int
}

type durationKey struct {
	Method string
	Path   string
}

type durationMetric struct {
	Count   uint64
	CountBy []uint64
	Sum     float64
}

func NewRegistry(serviceName string) *Registry {
	return &Registry{
		serviceName:      strings.TrimSpace(serviceName),
		requestTotals:    make(map[requestKey]uint64),
		requestDurations: make(map[durationKey]*durationMetric),
		dependencies:     make(map[string]bool),
	}
}

func (r *Registry) ObserveRequest(method, path string, statusCode int, duration time.Duration) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	requestKey := requestKey{
		Method: strings.ToUpper(strings.TrimSpace(method)),
		Path:   normalizePath(path),
		Status: statusCode,
	}
	r.requestTotals[requestKey]++

	durationKey := durationKey{
		Method: requestKey.Method,
		Path:   requestKey.Path,
	}

	metric := r.requestDurations[durationKey]
	if metric == nil {
		metric = &durationMetric{
			CountBy: make([]uint64, len(requestDurationBuckets)),
		}
		r.requestDurations[durationKey] = metric
	}

	seconds := duration.Seconds()
	metric.Count++
	metric.Sum += seconds
	for i, boundary := range requestDurationBuckets {
		if seconds <= boundary {
			metric.CountBy[i]++
		}
	}
}

func (r *Registry) SetDependencies(checks map[string]bool) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for dependency, up := range checks {
		dependency = strings.TrimSpace(dependency)
		if dependency == "" {
			continue
		}
		r.dependencies[dependency] = up
	}
}

func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(r.render()))
	})
}

func (r *Registry) render() string {
	if r == nil {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var builder strings.Builder

	fmt.Fprintf(&builder, "# HELP app_build_info Static build metadata for the scaffold.\n")
	fmt.Fprintf(&builder, "# TYPE app_build_info gauge\n")
	fmt.Fprintf(&builder, "app_build_info{service=%q,version=%q} 1\n", r.serviceName, "dev")

	fmt.Fprintf(&builder, "# HELP http_requests_total Total HTTP requests handled.\n")
	fmt.Fprintf(&builder, "# TYPE http_requests_total counter\n")
	for _, key := range sortedRequestKeys(r.requestTotals) {
		fmt.Fprintf(
			&builder,
			"http_requests_total{service=%q,method=%q,path=%q,status=%q} %d\n",
			r.serviceName,
			key.Method,
			key.Path,
			fmt.Sprintf("%d", key.Status),
			r.requestTotals[key],
		)
	}

	fmt.Fprintf(&builder, "# HELP http_request_duration_seconds HTTP request duration in seconds.\n")
	fmt.Fprintf(&builder, "# TYPE http_request_duration_seconds histogram\n")
	for _, key := range sortedDurationKeys(r.requestDurations) {
		metric := r.requestDurations[key]
		for i, boundary := range requestDurationBuckets {
			fmt.Fprintf(
				&builder,
				"http_request_duration_seconds_bucket{service=%q,method=%q,path=%q,le=%q} %d\n",
				r.serviceName,
				key.Method,
				key.Path,
				formatBoundary(boundary),
				metric.CountBy[i],
			)
		}
		fmt.Fprintf(
			&builder,
			"http_request_duration_seconds_bucket{service=%q,method=%q,path=%q,le=%q} %d\n",
			r.serviceName,
			key.Method,
			key.Path,
			"+Inf",
			metric.Count,
		)
		fmt.Fprintf(
			&builder,
			"http_request_duration_seconds_sum{service=%q,method=%q,path=%q} %.6f\n",
			r.serviceName,
			key.Method,
			key.Path,
			metric.Sum,
		)
		fmt.Fprintf(
			&builder,
			"http_request_duration_seconds_count{service=%q,method=%q,path=%q} %d\n",
			r.serviceName,
			key.Method,
			key.Path,
			metric.Count,
		)
	}

	fmt.Fprintf(&builder, "# HELP dependency_up Current dependency availability observed by readiness checks.\n")
	fmt.Fprintf(&builder, "# TYPE dependency_up gauge\n")
	for _, dependency := range sortedDependencies(r.dependencies) {
		value := 0
		if r.dependencies[dependency] {
			value = 1
		}
		fmt.Fprintf(&builder, "dependency_up{service=%q,dependency=%q} %d\n", r.serviceName, dependency, value)
	}

	return builder.String()
}

func sortedRequestKeys(values map[requestKey]uint64) []requestKey {
	keys := make([]requestKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		if keys[i].Path != keys[j].Path {
			return keys[i].Path < keys[j].Path
		}
		return keys[i].Status < keys[j].Status
	})
	return keys
}

func sortedDurationKeys(values map[durationKey]*durationMetric) []durationKey {
	keys := make([]durationKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		return keys[i].Path < keys[j].Path
	})
	return keys
}

func sortedDependencies(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}

	return path
}

func formatBoundary(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", value), "0"), ".")
}
