package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nimbusrun/nimbusrun/internal/db"
	"github.com/nimbusrun/nimbusrun/internal/models"
)

// InvocationRepository handles invocation CRUD.
type InvocationRepository struct {
	db *db.DB
}

// NewInvocationRepository creates a new InvocationRepository.
func NewInvocationRepository(database *db.DB) *InvocationRepository {
	return &InvocationRepository{db: database}
}

// Create inserts a new invocation record.
func (r *InvocationRepository) Create(ctx context.Context, inv *models.Invocation) error {
	query := `
		INSERT INTO invocations (id, function_id, version_id, worker_id, status, duration_ms, cold_start, retry_count, error_message, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		inv.ID, inv.FunctionID, inv.VersionID, inv.WorkerID,
		inv.Status, inv.DurationMs, inv.ColdStart, inv.RetryCount,
		inv.ErrorMsg, inv.IdempKey,
	)
	return err
}

// UpdateStatus updates invocation status.
func (r *InvocationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, durationMs int, errMsg string) error {
	query := `
		UPDATE invocations SET status = $1, duration_ms = $2, error_message = $3
		WHERE id = $4
	`
	_, err := r.db.Pool.Exec(ctx, query, status, durationMs, errMsg, id)
	return err
}

// GetByID retrieves an invocation by ID.
func (r *InvocationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Invocation, error) {
	var inv models.Invocation
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, function_id, version_id, worker_id, status, duration_ms, cold_start, retry_count, error_message, idempotency_key, created_at
		FROM invocations WHERE id = $1
	`, id).Scan(
		&inv.ID, &inv.FunctionID, &inv.VersionID, &inv.WorkerID,
		&inv.Status, &inv.DurationMs, &inv.ColdStart, &inv.RetryCount,
		&inv.ErrorMsg, &inv.IdempKey, &inv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// GetByIdempotencyKey checks if an invocation with this key exists.
func (r *InvocationRepository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Invocation, error) {
	var inv models.Invocation
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, function_id, version_id, worker_id, status, duration_ms, cold_start, retry_count, error_message, idempotency_key, created_at
		FROM invocations WHERE idempotency_key = $1
	`, key).Scan(
		&inv.ID, &inv.FunctionID, &inv.VersionID, &inv.WorkerID,
		&inv.Status, &inv.DurationMs, &inv.ColdStart, &inv.RetryCount,
		&inv.ErrorMsg, &inv.IdempKey, &inv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// ListByFunctionID lists recent invocations for a function.
func (r *InvocationRepository) ListByFunctionID(ctx context.Context, functionID uuid.UUID, limit int) ([]*models.Invocation, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, function_id, version_id, worker_id, status, duration_ms, cold_start, retry_count, error_message, idempotency_key, created_at
		FROM invocations WHERE function_id = $1
		ORDER BY created_at DESC LIMIT $2
	`, functionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invocations []*models.Invocation
	for rows.Next() {
		var inv models.Invocation
		err := rows.Scan(
			&inv.ID, &inv.FunctionID, &inv.VersionID, &inv.WorkerID,
			&inv.Status, &inv.DurationMs, &inv.ColdStart, &inv.RetryCount,
			&inv.ErrorMsg, &inv.IdempKey, &inv.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		invocations = append(invocations, &inv)
	}
	return invocations, nil
}

// IncrementRetryCount increments the retry counter for an invocation.
func (r *InvocationRepository) IncrementRetryCount(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE invocations SET retry_count = retry_count + 1 WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	return err
}
