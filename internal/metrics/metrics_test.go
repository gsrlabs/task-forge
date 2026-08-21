// internal/metrics/metrics_test.go
package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()

	metrics := newHTTPMetricsWithRegistry(registry)

	require.NotNil(t, metrics)
	require.NotNil(t, metrics.RequestsTotal)
	require.NotNil(t, metrics.ErrorsTotal)
	require.NotNil(t, metrics.Duration)
	require.NotNil(t, metrics.InFlight)

	metrics.RequestsTotal.WithLabelValues(
		"GET",
		"/api/v1/tasks",
		"200",
	)

	metrics.ErrorsTotal.WithLabelValues(
		"GET",
		"/api/v1/tasks",
		"500",
	)

	metrics.Duration.WithLabelValues(
		"GET",
		"/api/v1/tasks",
	)

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	require.Len(t, metricFamilies, 4)

	names := make(map[string]bool)

	for _, family := range metricFamilies {
		names[family.GetName()] = true
	}

	require.True(t, names["taskforge_http_requests_total"])
	require.True(t, names["taskforge_http_errors_total"])
	require.True(t, names["taskforge_http_request_duration_seconds"])
	require.True(t, names["taskforge_http_requests_in_flight"])
}
