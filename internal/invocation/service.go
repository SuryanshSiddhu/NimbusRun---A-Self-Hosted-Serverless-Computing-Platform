package invocation

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nimbusrun/nimbusrun/internal/db"
	"github.com/nimbusrun/nimbusrun/internal/idempotency"
	"github.com/nimbusrun/nimbusrun/internal/models"
	"github.com/nimbusrun/nimbusrun/internal/queue"
	"github.com/nimbusrun/nimbusrun/internal/retry"
	"github.com/nimbusrun/nimbusrun/internal/repository"
)

// Service handles function invocation lifecycle including retries and DLQ.
type Service struct {
	db            *db.DB
	fnRepo        *repository.FunctionRepository
	verRepo       *repository.VersionRepository
	invRepo       *repository.InvocationRepository
	jobQueue      *queue.RedisStreamQueue
	dlqService    *retry.DLQService
	idempotency   *idempotency.Service
	retryPolicy   retry.Config
}

// NewService creates a new invocation Service.
func NewService(
	database *db.DB,
	fnRepo *repository.FunctionRepository,
	verRepo *repository.VersionRepository,
	invRepo *repository.InvocationRepository,
	jobQueue *queue.RedisStreamQueue,
	dlq *retry.DLQService,
	idem *idempotency.Service,
) *Service {
	return &Service{
		db:          database,
		fnRepo:      fnRepo,
		verRepo:     verRepo,
		invRepo:     invRepo,
		jobQueue:    jobQueue,
		dlqService:  dlq,
		idempotency: idem,
		retryPolicy: retry.DefaultConfig(),
	}
}

// InvokeRequest describes a function invocation request.
type InvokeRequest struct {
	FunctionID     uuid.UUID
	IdempotencyKey string
	Payload        map[string]interface{}
}

// InvokeResult is the outcome of an invocation.
type InvokeResult struct {
	InvocationID uuid.UUID
	Status      string
	Message     string
	IsDuplicate bool
}

// Invoke submits a function for execution.
func (s *Service) Invoke(ctx context.Context, req *InvokeRequest) (*InvokeResult, error) {
	// Check idempotency first
	if req.IdempotencyKey != "" {
		cached, err := s.idempotency.Check(ctx, req.IdempotencyKey)
		if err != nil {
			log.Printf("Idempotency check error: %v", err)
		}
		if cached != nil {
			return &InvokeResult{
				InvocationID: uuid.MustParse(cached.InvocationID),
				Status:      cached.Status,
				IsDuplicate: true,
				Message:    "Duplicate request — returning cached result",
			}, nil
		}

		// Try to acquire lock to prevent race conditions
		acquired, _ := s.idempotency.AcquireLock(ctx, req.IdempotencyKey, 30*time.Second)
		if !acquired {
			// Another request is processing this key
			return nil, fmt.Errorf("concurrent request with same idempotency key")
		}
		defer s.idempotency.ReleaseLock(ctx, req.IdempotencyKey, "")
	}

	// Get function
	fn, err := s.fnRepo.GetByID(ctx, req.FunctionID)
	if err != nil {
		return nil, fmt.Errorf("function not found: %w", err)
	}

	// Get active version
	if fn.ActiveVersionID == uuid.Nil {
		return nil, fmt.Errorf("no active version set for function")
	}

	ver, err := s.verRepo.GetByID(ctx, fn.ActiveVersionID)
	if err != nil {
		return nil, fmt.Errorf("active version not found: %w", err)
	}

	if ver.Status != "READY" {
		return nil, fmt.Errorf("function version not ready: %s", ver.Status)
	}

	// Create invocation record
	inv := &models.Invocation{
		ID:        uuid.New(),
		FunctionID: fn.ID,
		VersionID: ver.ID,
		Status:    "PENDING",
		IdempKey: req.IdempotencyKey,
	}

	if err := s.invRepo.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("creating invocation record: %w", err)
	}

	// Build job
	job := &queue.Job{
		ID:              uuid.New(),
		InvocationID:    inv.ID.String(),
		FunctionID:      fn.ID.String(),
		VersionID:       ver.ID.String(),
		ImageURI:        ver.ImageURI,
		Entrypoint:      fn.Entrypoint,
		Payload:         req.Payload,
		MemoryLimitMB:   fn.MemoryLimit,
		CPULimitMillicores: fn.CPULimit,
		TimeoutSeconds:  fn.Timeout,
		RetryCount:      0,
		MaxRetries:      s.retryPolicy.MaxRetries,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:       time.Now(),
	}

	// Enqueue job
	if err := s.jobQueue.EnqueueJob(ctx, job); err != nil {
		s.invRepo.UpdateStatus(ctx, inv.ID, "FAILED", 0, err.Error())
		return nil, fmt.Errorf("enqueuing job: %w", err)
	}

	// Cache the pending result
	if req.IdempotencyKey != "" {
		s.idempotency.Store(ctx, req.IdempotencyKey, inv.ID.String(), "PENDING", "", "")
	}

	return &InvokeResult{
		InvocationID: inv.ID,
		Status:       "PENDING",
		Message:      "Invocation queued successfully",
	}, nil
}

// ProcessResult handles a completed job result, applying retry logic if needed.
func (s *Service) ProcessResult(ctx context.Context, result *queue.JobResult) error {
	invID, _ := uuid.Parse(result.InvocationID)
	fnID, _ := uuid.Parse(result.FunctionID)

	// Update invocation in DB
	if result.Status == "SUCCESS" || result.ExitCode == 0 {
		s.invRepo.UpdateStatus(ctx, invID, "SUCCESS", result.DurationMs, "")
		// Cache successful result
		if result.InvocationID != "" {
			s.idempotency.Store(ctx, "", result.InvocationID, "SUCCESS", result.Stdout, "")
		}
		return nil
	}

	// Job failed — check retry policy
	job := &queue.Job{
		InvocationID: result.InvocationID,
		FunctionID:   result.FunctionID,
		VersionID:    result.VersionID,
		RetryCount:   result.DurationMs, // would use actual retry count field
		MaxRetries:   s.retryPolicy.MaxRetries,
	}

	if s.retryPolicy.ShouldRetry(job.RetryCount) {
		// Calculate backoff delay
		delay := s.retryPolicy.Calculate(job.RetryCount)
		log.Printf("Job %s failed, retrying in %v (attempt %d/%d)",
			result.InvocationID, delay, job.RetryCount+1, s.retryPolicy.MaxRetries)

		// Re-enqueue with incremented retry count
		job.RetryCount++
		time.AfterFunc(delay, func() {
			s.jobQueue.EnqueueJob(context.Background(), job)
		})

		// Update status to RETRYING
		s.invRepo.UpdateStatus(ctx, invID, "RETRYING", 0, result.Error)
	} else {
		// Max retries exceeded — move to DLQ
		log.Printf("Job %s exceeded max retries, moving to DLQ", result.InvocationID)
		s.invRepo.UpdateStatus(ctx, invID, "FAILED", result.DurationMs, result.Error)
		s.dlqService.Enqueue(context.Background(), job, result.Error)

		// Cache failure
		s.idempotency.Store(ctx, "", result.InvocationID, "FAILED", "", result.Error)
	}

	return nil
}

// GetInvocation retrieves an invocation by ID.
func (s *Service) GetInvocation(ctx context.Context, invID uuid.UUID) (*models.Invocation, error) {
	return s.invRepo.GetByID(ctx, invID)
}

// ListInvocations lists recent invocations for a function.
func (s *Service) ListInvocations(ctx context.Context, fnID uuid.UUID, limit int) ([]*models.Invocation, error) {
	return s.invRepo.ListByFunctionID(ctx, fnID, limit)
}
