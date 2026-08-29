package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Job represents a function execution request.
type Job struct {
	ID                 uuid.UUID            `json:"id"`
	InvocationID       string               `json:"invocation_id"`
	FunctionID         string               `json:"function_id"`
	VersionID          string               `json:"version_id"`
	ImageURI           string               `json:"image_uri"`
	Entrypoint         string               `json:"entrypoint"`
	Payload            map[string]interface{} `json:"payload"`
	MemoryLimitMB      int                  `json:"memory_limit_mb"`
	CPULimitMillicores int                  `json:"cpu_limit_millicores"`
	TimeoutSeconds     int                  `json:"timeout_seconds"`
	RetryCount         int                  `json:"retry_count"`
	MaxRetries         int                  `json:"max_retries"`
	IdempotencyKey     string               `json:"idempotency_key,omitempty"`
	WorkerID           string               `json:"worker_id,omitempty"`
	CreatedAt          time.Time            `json:"created_at"`
}

// QueuedJob wraps a job with its stream message ID and claim info.
type QueuedJob struct {
	ID     string
	Job    *Job
	Claim  string
}

// JobResult holds the outcome of a completed job.
type JobResult struct {
	InvocationID string                 `json:"invocation_id"`
	FunctionID   string                 `json:"function_id"`
	VersionID    string                 `json:"version_id"`
	Status       string                 `json:"status"`
	ExitCode     int                    `json:"exit_code"`
	DurationMs   int                    `json:"duration_ms"`
	Stdout       string                 `json:"stdout"`
	Stderr       string                 `json:"stderr"`
	WorkerID     string                 `json:"worker_id"`
	Error        string                 `json:"error,omitempty"`
	MessageID    string                 `json:"-"`
	Timestamp    time.Time              `json:"timestamp"`
}

// WorkerHeartbeat represents a worker's status for scheduling decisions.
type WorkerHeartbeat struct {
	WorkerID       string  `json:"worker_id"`
	Hostname       string  `json:"hostname"`
	CPUUsage       float64 `json:"cpu_usage"`       // 0.0-1.0
	MemoryUsage    float64 `json:"memory_usage"`    // 0.0-1.0
	RunningTasks   int     `json:"running_tasks"`
	AvailableSlots int     `json:"available_slots"`
	TotalSlots     int     `json:"total_slots"`
	Timestamp      int64   `json:"timestamp"`
}

// CalculateLoad computes the normalized load score for a worker.
// Formula: 0.5 * cpuUsage + 0.5 * (runningTasks / availableSlots)
func (h *WorkerHeartbeat) CalculateLoad() float64 {
	if h.AvailableSlots <= 0 {
		return 1.0 // full if no slots available
	}
	taskLoad := float64(h.RunningTasks) / float64(h.AvailableSlots)
	return 0.5*h.CPUUsage + 0.5*taskLoad
}

// MarshalJSON custom marshal for WorkerHeartbeat.
func (h *WorkerHeartbeat) MarshalJSON() ([]byte, error) {
	type alias WorkerHeartbeat
	return json.Marshal(&struct {
		*alias
		Load float64 `json:"load_score"`
	}{
		alias: (*alias)(h),
		Load:  h.CalculateLoad(),
	})
}

// Validate checks that the heartbeat has valid values.
func (h *WorkerHeartbeat) Validate() error {
	if h.WorkerID == "" {
		return fmt.Errorf("worker_id required")
	}
	if h.AvailableSlots < 0 {
		return fmt.Errorf("available_slots must be non-negative")
	}
	return nil
}

// IsHealthy returns true if the heartbeat is recent (within 15 seconds).
func (h *WorkerHeartbeat) IsHealthy(now time.Time) bool {
	return now.Sub(time.Unix(h.Timestamp, 0)) <= 15*time.Second
}