// internal/metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	RequestsTotal *prometheus.CounterVec
	ErrorsTotal   *prometheus.CounterVec
	Duration      *prometheus.HistogramVec
	InFlight      prometheus.Gauge
}

func NewHTTPMetrics() *HTTPMetrics {
	return newHTTPMetricsWithRegistry(prometheus.DefaultRegisterer)
}

func newHTTPMetricsWithRegistry(
	registerer prometheus.Registerer,
) *HTTPMetrics {
	m := &HTTPMetrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "taskforge_http_requests_total",
				Help: "Total number of HTTP requests.",
			},
			[]string{"method", "path", "status"},
		),

		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "taskforge_http_errors_total",
				Help: "Total number of HTTP requests resulting in errors.",
			},
			[]string{"method", "path", "status"},
		),

		Duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "taskforge_http_request_duration_seconds",
				Help: "HTTP request duration in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),

		InFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "taskforge_http_requests_in_flight",
				Help: "Current number of HTTP requests being processed.",
			},
		),
	}

	registerer.MustRegister(
		m.RequestsTotal,
		m.ErrorsTotal,
		m.Duration,
		m.InFlight,
	)

	return m
}