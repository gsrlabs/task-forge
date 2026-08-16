package middleware

import (
	"strconv"
	"time"

	"task-forge/internal/metrics"

	"github.com/gin-gonic/gin"
)

// Prometheus collects HTTP metrics for incoming requests.
func Prometheus(m *metrics.HTTPMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		// /metrics itself is not included in application metrics.
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()

		m.InFlight.Inc()
		defer m.InFlight.Dec()

		c.Next()

		duration := time.Since(start).Seconds()

		method := c.Request.Method
		status := c.Writer.Status()
		statusLabel := strconv.Itoa(status)

		// Gin returns the registered route pattern:
		// /api/v1/tasks/:id
		//
		// This prevents high-cardinality labels caused by UUIDs
		// and other dynamic path parameters.
		path := c.FullPath()

		// For unmatched routes (404), Gin may not have a route pattern.
		if path == "" {
			path = "unknown"
		}

		m.RequestsTotal.WithLabelValues(
			method,
			path,
			statusLabel,
		).Inc()

		m.Duration.WithLabelValues(
			method,
			path,
		).Observe(duration)

		if status >= 400 {
			m.ErrorsTotal.WithLabelValues(
				method,
				path,
				statusLabel,
			).Inc()
		}
	}
}