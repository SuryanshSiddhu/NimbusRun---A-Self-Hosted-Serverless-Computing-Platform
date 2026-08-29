package observability

import (
	"context"
	"io"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the global structured logger.
var Logger *zap.Logger

// InitLogger initializes the global structured logger.
// In production, use JSON output. In development, use console output.
func InitLogger(production bool) error {
	var cfg zap.Config
	if production {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	Logger = logger
	return nil
}

// Sync flushes any buffered log entries.
func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}

// WithContext returns a logger with context fields.
func WithContext(ctx context.Context) *zap.Logger {
	if Logger == nil {
		return zap.NewNop()
	}

	// Extract trace/span info from context if available
	fields := []zap.Field{}

	if reqID, ok := ctx.Value("request_id").(string); ok {
		fields = append(fields, zap.String("request_id", reqID))
	}
	if fnID, ok := ctx.Value("function_id").(string); ok {
		fields = append(fields, zap.String("function_id", fnID))
	}
	if workerID, ok := ctx.Value("worker_id").(string); ok {
		fields = append(fields, zap.String("worker_id", workerID))
	}

	return Logger.With(fields...)
}

// InvocLogger logs invocation-specific structured data.
type InvocLogger struct {
	logger   *zap.Logger
	invID    string
	fnID     string
	workerID string
}

// NewInvocLogger creates a logger scoped to a specific invocation.
func NewInvocLogger(invID, fnID, workerID string) *InvocLogger {
	l := Logger.With(
		zap.String("invocation_id", invID),
		zap.String("function_id", fnID),
		zap.String("worker_id", workerID),
	)
	return &InvocLogger{logger: l, invID: invID, fnID: fnID, workerID: workerID}
}

// LogJobReceived logs when a job is received by a worker.
func (l *InvocLogger) LogJobReceived() {
	l.logger.Info("job received", zap.String("event", "job_received"), zap.Time("timestamp", time.Now()))
}

// LogJobStarted logs when a container starts.
func (l *InvocLogger) LogJobStarted(containerID string) {
	l.logger.Info("job started",
		zap.String("event", "job_started"),
		zap.String("container_id", containerID),
		zap.Time("timestamp", time.Now()),
	)
}

// LogJobCompleted logs when a job completes successfully.
func (l *InvocLogger) LogJobCompleted(exitCode int, durationMs int) {
	l.logger.Info("job completed",
		zap.String("event", "job_completed"),
		zap.Int("exit_code", exitCode),
		zap.Int("duration_ms", durationMs),
		zap.Time("timestamp", time.Now()),
	)
}

// LogJobFailed logs when a job fails.
func (l *InvocLogger) LogJobFailed(reason string, durationMs int) {
	l.logger.Warn("job failed",
		zap.String("event", "job_failed"),
		zap.String("reason", reason),
		zap.Int("duration_ms", durationMs),
		zap.Time("timestamp", time.Now()),
	)
}

// LogJobTimeout logs when a job times out.
func (l *InvocLogger) LogJobTimeout(timeoutSeconds int) {
	l.logger.Warn("job timed out",
		zap.String("event", "job_timeout"),
		zap.Int("timeout_seconds", timeoutSeconds),
		zap.Time("timestamp", time.Now()),
	)
}

// LogRetry logs a retry attempt.
func (l *InvocLogger) LogRetry(attempt, maxRetries int, delayMs int) {
	l.logger.Info("job retrying",
		zap.String("event", "job_retry"),
		zap.Int("attempt", attempt),
		zap.Int("max_retries", maxRetries),
		zap.Int("delay_ms", delayMs),
		zap.Time("timestamp", time.Now()),
	)
}

// LogDLQ logs when a job is moved to the dead-letter queue.
func (l *InvocLogger) LogDLQ(reason string, attemptCount int) {
	l.logger.Error("job moved to DLQ",
		zap.String("event", "job_dlq"),
		zap.String("reason", reason),
		zap.Int("attempt_count", attemptCount),
		zap.Time("timestamp", time.Now()),
	)
}

// LogContainerKilled logs when a container is killed (OOM, timeout, etc.).
func (l *InvocLogger) LogContainerKilled(signal, reason string) {
	l.logger.Warn("container killed",
		zap.String("event", "container_killed"),
		zap.String("signal", signal),
		zap.String("reason", reason),
		zap.Time("timestamp", time.Now()),
	)
}

// LogInfra logs infrastructure-level events.
func LogInfra(component, event string, fields ...zap.Field) {
	Logger.With(zap.String("component", component)).Info(event, fields...)
}

// StdoutWriter returns an io.Writer that writes to stdout.
func StdoutWriter() io.Writer {
	return os.Stdout
}
