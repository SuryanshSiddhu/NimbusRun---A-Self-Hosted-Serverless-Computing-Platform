package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered user.
type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Function represents a user-defined function.
type Function struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	Name            string    `json:"name"`
	Entrypoint      string    `json:"entrypoint"`
	MemoryLimit     int       `json:"memory_limit"` // MB
	CPULimit        int       `json:"cpu_limit"`    // millicores
	Timeout         int       `json:"timeout"`      // seconds
	MaxConcurrency  int       `json:"max_concurrency"`
	ActiveVersionID uuid.UUID `json:"active_version_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// FunctionVersion represents an immutable snapshot of a function's code.
type FunctionVersion struct {
	ID          uuid.UUID `json:"id"`
	FunctionID  uuid.UUID `json:"function_id"`
	VersionNum  int       `json:"version_number"`
	ImageURI    string    `json:"image_uri"`
	Status      string    `json:"status"` // QUEUED, BUILDING, READY, FAILED
	CreatedAt   time.Time `json:"created_at"`
}

// Worker represents an execution node.
type Worker struct {
	ID          uuid.UUID `json:"id"`
	WorkerID    string    `json:"worker_id"`
	Hostname    string    `json:"hostname"`
	Status      string    `json:"status"` // HEALTHY, UNHEALTHY, DRAINING
	CPUCapacity int       `json:"cpu_capacity"`
	MemCapacity int       `json:"memory_capacity"`
	LastHB      time.Time `json:"last_heartbeat"`
	CreatedAt   time.Time `json:"created_at"`
}

// Invocation tracks a single function execution.
type Invocation struct {
	ID          uuid.UUID `json:"id"`
	FunctionID  uuid.UUID `json:"function_id"`
	VersionID   uuid.UUID `json:"version_id"`
	WorkerID    uuid.UUID `json:"worker_id"`
	Status      string    `json:"status"` // PENDING, RUNNING, SUCCESS, FAILED, TIMEOUT
	DurationMs  int       `json:"duration_ms,omitempty"`
	ColdStart   bool      `json:"cold_start,omitempty"`
	RetryCount  int       `json:"retry_count,omitempty"`
	ErrorMsg    string    `json:"error_message,omitempty"`
	IdempKey    string    `json:"idempotency_key,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// LogEntry represents a structured log line.
type LogEntry struct {
	ID        uuid.UUID `json:"id"`
	InvocationID uuid.UUID `json:"invocation_id"`
	Level     string    `json:"level"` // DEBUG, INFO, WARN, ERROR
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// DLQEntry holds a failed invocation after max retries.
type DLQEntry struct {
	ID         uuid.UUID `json:"id"`
	FunctionID uuid.UUID `json:"function_id"`
	VersionID  uuid.UUID `json:"version_id"`
	Attempts   int       `json:"attempt_count"`
	LastError  string    `json:"last_error"`
	CreatedAt  time.Time `json:"created_at"`
}