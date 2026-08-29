package observability

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all platform Prometheus metrics.
type Metrics struct {
	// Invocation metrics
	InvocationsTotal    *prometheus.CounterVec
	InvocationDuration  *prometheus.HistogramVec
	InvocationErrors    *prometheus.CounterVec
	ColdStartTotal      prometheus.Counter

	// Queue metrics
	QueueDepth          prometheus.Gauge
	QueueLatency        *prometheus.HistogramVec

	// Worker metrics
	WorkerCount         *prometheus.GaugeVec
	WorkerCPUUsage      *prometheus.GaugeVec
	WorkerMemoryUsage   *prometheus.GaugeVec
	WorkerRunningTasks  *prometheus.GaugeVec

	// Build metrics
	BuildDuration       *prometheus.HistogramVec
	BuildSuccess        *prometheus.CounterVec
	BuildFailure        *prometheus.CounterVec

	// Auth metrics
	AuthAttempts        *prometheus.CounterVec

	// HTTP metrics
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPRequestsTotal   *prometheus.CounterVec

	registry *prometheus.Registry
}

// NewMetrics creates and registers all platform metrics.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	invocations := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimbusrun_invocations_total",
			Help: "Total number of function invocations",
		},
		[]string{"function_id", "status"},
	)

	invocationDuration := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nimbusrun_invocation_duration_ms",
			Help:    "Invocation duration in milliseconds",
			Buckets: []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000},
		},
		[]string{"function_id"},
	)

	invocationErrors := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimbusrun_invocation_errors_total",
			Help: "Total number of invocation errors",
		},
		[]string{"function_id", "error_type"},
	)

	coldStart := promauto.With(reg).NewCounter(
		prometheus.CounterOpts{
			Name: "nimbusrun_cold_starts_total",
			Help: "Total number of cold starts",
		},
	)

	queueDepth := promauto.With(reg).NewGauge(
		prometheus.GaugeOpts{
			Name: "nimbusrun_queue_depth",
			Help: "Current queue depth",
		},
	)

	queueLatency := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nimbusrun_queue_latency_ms",
			Help:    "Time jobs spend waiting in the queue",
			Buckets: []float64{1, 5, 10, 50, 100, 500, 1000, 5000},
		},
		[]string{"function_id"},
	)

	workerCount := promauto.With(reg).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nimbusrun_worker_count",
			Help: "Number of workers by status",
		},
		[]string{"status"},
	)

	workerCPU := promauto.With(reg).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nimbusrun_worker_cpu_usage",
			Help: "Worker CPU usage (0-1)",
		},
		[]string{"worker_id"},
	)

	workerMem := promauto.With(reg).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nimbusrun_worker_memory_usage",
			Help: "Worker memory usage (0-1)",
		},
		[]string{"worker_id"},
	)

	workerTasks := promauto.With(reg).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nimbusrun_worker_running_tasks",
			Help: "Number of tasks currently running on a worker",
		},
		[]string{"worker_id"},
	)

	buildDuration := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nimbusrun_build_duration_seconds",
			Help:    "Build duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"function_id"},
	)

	buildSuccess := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimbusrun_builds_success_total",
			Help: "Total successful builds",
		},
		[]string{"function_id"},
	)

	buildFailure := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimbusrun_builds_failure_total",
			Help: "Total failed builds",
		},
		[]string{"function_id", "reason"},
	)

	authAttempts := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimbusrun_auth_attempts_total",
			Help: "Total authentication attempts",
		},
		[]string{"result"}, // success, failure
	)

	httpDuration := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nimbusrun_http_request_duration_ms",
			Help:    "HTTP request duration in milliseconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	httpTotal := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimbusrun_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	return &Metrics{
		InvocationsTotal:   invocations,
		InvocationDuration: invocationDuration,
		InvocationErrors:   invocationErrors,
		ColdStartTotal:     coldStart,
		QueueDepth:         queueDepth,
		QueueLatency:       queueLatency,
		WorkerCount:        workerCount,
		WorkerCPUUsage:     workerCPU,
		WorkerMemoryUsage:  workerMem,
		WorkerRunningTasks: workerTasks,
		BuildDuration:      buildDuration,
		BuildSuccess:       buildSuccess,
		BuildFailure:       buildFailure,
		AuthAttempts:       authAttempts,
		HTTPRequestDuration: httpDuration,
		HTTPRequestsTotal:   httpTotal,
		registry:           reg,
	}
}

// Handler returns the HTTP handler that serves the metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordInvocation records a completed invocation.
func (m *Metrics) RecordInvocation(functionID, status string, durationMs int, coldStart bool, errorType string) {
	m.InvocationsTotal.WithLabelValues(functionID, status).Inc()
	if durationMs > 0 {
		m.InvocationDuration.WithLabelValues(functionID).Observe(float64(durationMs))
	}
	if coldStart {
		m.ColdStartTotal.Inc()
	}
	if status == "FAILED" {
		m.InvocationErrors.WithLabelValues(functionID, errorType).Inc()
	}
}

// RecordQueueLatency records how long a job waited in the queue.
func (m *Metrics) RecordQueueLatency(functionID string, latencyMs int) {
	m.QueueLatency.WithLabelValues(functionID).Observe(float64(latencyMs))
}

// UpdateWorker updates worker metrics from a heartbeat.
func (m *Metrics) UpdateWorker(workerID string, cpu, mem float64, runningTasks int, healthy bool) {
	if healthy {
		m.WorkerCount.WithLabelValues("healthy").Inc()
		m.WorkerCount.WithLabelValues("unhealthy").Dec()
	} else {
		m.WorkerCount.WithLabelValues("unhealthy").Inc()
		m.WorkerCount.WithLabelValues("healthy").Dec()
	}
	m.WorkerCPUUsage.WithLabelValues(workerID).Set(cpu)
	m.WorkerMemoryUsage.WithLabelValues(workerID).Set(mem)
	m.WorkerRunningTasks.WithLabelValues(workerID).Set(float64(runningTasks))
}

// RecordBuild records the outcome of a build.
func (m *Metrics) RecordBuild(functionID string, duration time.Duration, success bool, reason string) {
	m.BuildDuration.WithLabelValues(functionID).Observe(duration.Seconds())
	if success {
		m.BuildSuccess.WithLabelValues(functionID).Inc()
	} else {
		m.BuildFailure.WithLabelValues(functionID, reason).Inc()
	}
}

// RecordAuthAttempt records a login or registration attempt.
func (m *Metrics) RecordAuthAttempt(success bool) {
	if success {
		m.AuthAttempts.WithLabelValues("success").Inc()
	} else {
		m.AuthAttempts.WithLabelValues("failure").Inc()
	}
}

// RecordHTTPRequest records an HTTP request.
func (m *Metrics) RecordHTTPRequest(method, path string, status int, durationMs int) {
	statusStr := fmt.Sprintf("%d", status)
	m.HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path, statusStr).Observe(float64(durationMs))
}

// HTTPMiddleware returns a Gin middleware that records HTTP metrics.
func (m *Metrics) HTTPMiddleware() func(method, path string, status int, duration time.Duration) {
	return func(method, path string, status int, duration time.Duration) {
		m.RecordHTTPRequest(method, path, status, int(duration.Milliseconds()))
	}
}

// Server runs a metrics HTTP server.
func (m *Metrics) Server(ctx context.Context, port string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	return srv.ListenAndServe()
}
