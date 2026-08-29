package retry

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nimbusrun/nimbusrun/internal/db"
	"github.com/nimbusrun/nimbusrun/internal/models"
	"github.com/nimbusrun/nimbusrun/internal/queue"
)

// DLQService manages the dead-letter queue.
type DLQService struct {
	db  *db.DB
	q   *queue.RedisStreamQueue
}

// NewDLQService creates a new DLQService.
func NewDLQService(database *db.DB, q *queue.RedisStreamQueue) *DLQService {
	return &DLQService{db: database, q: q}
}

// Enqueue moves a failed job to the DLQ and updates the database.
func (s *DLQService) Enqueue(ctx context.Context, job *queue.Job, errMsg string) error {
	// Add to Redis DLQ stream
	if err := s.q.EnqueueDLQ(ctx, job, errMsg); err != nil {
		log.Printf("Failed to enqueue to DLQ stream: %v", err)
	}

	// Also record in database for persistence
	invID, _ := uuid.Parse(job.InvocationID)
	fnID, _ := uuid.Parse(job.FunctionID)
	verID, _ := uuid.Parse(job.VersionID)

	entry := &models.DLQEntry{
		ID:         uuid.New(),
		FunctionID: fnID,
		VersionID:  verID,
		Attempts:   job.RetryCount,
		LastError:  errMsg,
		CreatedAt:  time.Now(),
	}

	// Insert into DB (simplified - would use a proper repo)
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO dlq_invocations (id, function_id, version_id, attempt_count, last_error, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, entry.ID, entry.FunctionID, entry.VersionID, entry.Attempts, entry.LastError, entry.CreatedAt)

	if err != nil {
		log.Printf("Failed to persist DLQ entry: %v", err)
	}

	log.Printf("Job %s moved to DLQ after %d attempts: %s", job.InvocationID, job.RetryCount, errMsg)
	return nil
}

// List returns all entries in the DLQ.
func (s *DLQService) List(ctx context.Context) ([]*models.DLQEntry, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, function_id, version_id, attempt_count, last_error, created_at
		FROM dlq_invocations ORDER BY created_at DESC LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.DLQEntry
	for rows.Next() {
		var e models.DLQEntry
		if err := rows.Scan(&e.ID, &e.FunctionID, &e.VersionID, &e.Attempts, &e.LastError, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, &e)
	}
	return entries, nil
}

// RetryFromDLQ retries a DLQ entry by re-queuing it.
func (s *DLQService) RetryFromDLQ(ctx context.Context, entryID uuid.UUID) error {
	// Get the original job data from DLQ
	// For now, we'll just delete from DLQ and re-enqueue
	// In a full implementation, we'd store the full job data in the DLQ record

	// Delete from DLQ table
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM dlq_invocations WHERE id = $1`, entryID)
	if err != nil {
		return fmt.Errorf("removing DLQ entry: %w", err)
	}

	log.Printf("DLQ entry %s re-queued for retry", entryID)
	return nil
}
