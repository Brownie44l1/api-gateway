package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Brownie44l1/api-gateway/internal/metrics"
)

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &metricsResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()

		statusStr := strconv.Itoa(wrapped.statusCode)
		metrics.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
		metrics.RequestLatency.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (mrw *metricsResponseWriter) WriteHeader(code int) {
	mrw.statusCode = code
	mrw.ResponseWriter.WriteHeader(code)
}