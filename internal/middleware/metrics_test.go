// internal/middleware/metrics_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"task-forge/internal/metrics"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestHTTPMetrics creates HTTPMetrics with a fresh registry for isolation.
func newTestHTTPMetrics(t *testing.T) *metrics.HTTPMetrics {
	t.Helper()

	//registry := prometheus.NewRegistry()

	return &metrics.HTTPMetrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		Duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "test_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_http_errors_total",
				Help: "Total number of HTTP errors",
			},
			[]string{"method", "path", "status"},
		),
		InFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "test_http_requests_in_flight",
				Help: "Number of HTTP requests in flight",
			},
		),
	}
}

// ============================================================================
// TestPrometheus
// ============================================================================

func TestPrometheus_SkipsMetricsEndpoint(t *testing.T) {
	httpMetrics := newTestHTTPMetrics(t)
	handler := Prometheus(httpMetrics)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/metrics", nil)
	c.Request = req

	nextCalled := true
	c.Next()

	handler(c)

	assert.True(t, nextCalled, "Should call c.Next() for /metrics")
}

func TestPrometheus_MetricsEndpointExcluded(t *testing.T) {
	httpMetrics := newTestHTTPMetrics(t)

	router := gin.New()
	router.Use(Prometheus(httpMetrics))

	router.GET("/metrics", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/metrics",
		nil,
	)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.Equal(
		t,
		float64(0),
		testutil.ToFloat64(httpMetrics.InFlight),
	)
}

func TestPrometheus_SuccessfulRequest(t *testing.T) {
	httpMetrics := newTestHTTPMetrics(t)
	handler := Prometheus(httpMetrics)

	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)

	router.GET("/api/v1/tasks/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/api/v1/tasks/550e8400-e29b-41d4-a716-446655440000", nil)
	c.Request = req

	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPrometheus_ClientError(t *testing.T) {
	httpMetrics := newTestHTTPMetrics(t)

	router := gin.New()
	router.Use(Prometheus(httpMetrics))

	router.GET("/api/v1/tasks/:id", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "bad request",
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/tasks/invalid",
		nil,
	)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPrometheus_ServerError(t *testing.T) {
	httpMetrics := newTestHTTPMetrics(t)

	router := gin.New()
	router.Use(Prometheus(httpMetrics))

	router.GET("/api/v1/tasks/:id", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/tasks/550e8400-e29b-41d4-a716-446655440000",
		nil,
	)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPrometheus_UsesFullPath(t *testing.T) {
	httpMetrics := newTestHTTPMetrics(t)

	router := gin.New()
	router.Use(Prometheus(httpMetrics))

	router.GET("/api/v1/tasks/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id": c.Param("id"),
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/tasks/550e8400-e29b-41d4-a716-446655440000",
		nil,
	)

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPrometheus_UnknownRoute(t *testing.T) {
	httpMetrics := newTestHTTPMetrics(t)

	router := gin.New()
	router.Use(Prometheus(httpMetrics))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/does-not-exist",
		nil,
	)

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPrometheus_InFlight(t *testing.T) {
	httpMetrics := newTestHTTPMetrics(t)

	router := gin.New()
	router.Use(Prometheus(httpMetrics))

	var inFlightDuringRequest float64

	router.GET("/api/v1/test", func(c *gin.Context) {
		inFlightDuringRequest = testutil.ToFloat64(httpMetrics.InFlight)

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/test",
		nil,
	)

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(1), inFlightDuringRequest)
	assert.Equal(t, float64(0), testutil.ToFloat64(httpMetrics.InFlight))
}

func TestPrometheus_FullRouterIntegration(t *testing.T) {
	httpMetrics := newTestHTTPMetrics(t)

	router := gin.New()
	router.Use(Prometheus(httpMetrics))

	router.GET("/api/v1/teams", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"teams": []string{}})
	})

	router.POST("/api/v1/tasks", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": "task-123"})
	})

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/api/v1/teams", nil)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/tasks", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusCreated, w2.Code)

	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/nonexistent", nil)
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusNotFound, w3.Code)

	require.NotPanics(t, func() {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/teams", nil))
	})
}
