package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nimbusrun/nimbusrun/internal/db"
	"github.com/nimbusrun/nimbusrun/internal/models"
)

// FunctionRepository handles function CRUD operations.
type FunctionRepository struct {
	db *db.DB
}

// NewFunctionRepository creates a new FunctionRepository.
func NewFunctionRepository(database *db.DB) *FunctionRepository {
	return &FunctionRepository{db: database}
}

// Create inserts a new function.
func (r *FunctionRepository) Create(ctx context.Context, fn *models.Function) error {
	query := `
		INSERT INTO functions (id, user_id, name, entrypoint, memory_limit, cpu_limit, timeout, max_concurrency)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		fn.ID, fn.UserID, fn.Name, fn.Entrypoint,
		fn.MemoryLimit, fn.CPULimit, fn.Timeout, fn.MaxConcurrency,
	)
	return err
}

// GetByID retrieves a function by ID.
func (r *FunctionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Function, error) {
	query := `
		SELECT id, user_id, name, entrypoint, memory_limit, cpu_limit, timeout, max_concurrency, active_version_id, created_at, updated_at
		FROM functions WHERE id = $1
	`
	var fn models.Function
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&fn.ID, &fn.UserID, &fn.Name, &fn.Entrypoint,
		&fn.MemoryLimit, &fn.CPULimit, &fn.Timeout, &fn.MaxConcurrency,
		&fn.ActiveVersionID, &fn.CreatedAt, &fn.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &fn, nil
}

// GetByUserID lists functions for a user.
func (r *FunctionRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Function, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, name, entrypoint, memory_limit, cpu_limit, timeout, max_concurrency, active_version_id, created_at, updated_at
		FROM functions WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var funcs []*models.Function
	for rows.Next() {
		var fn models.Function
		err := rows.Scan(
			&fn.ID, &fn.UserID, &fn.Name, &fn.Entrypoint,
			&fn.MemoryLimit, &fn.CPULimit, &fn.Timeout, &fn.MaxConcurrency,
			&fn.ActiveVersionID, &fn.CreatedAt, &fn.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, &fn)
	}
	return funcs, nil
}

// UpdateActiveVersion sets the active version.
func (r *FunctionRepository) UpdateActiveVersion(ctx context.Context, functionID, versionID uuid.UUID) error {
	query := `UPDATE functions SET active_version_id = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, versionID, functionID)
	return err
}

// Delete removes a function and all its versions.
func (r *FunctionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM functions WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	return err
}

// DeleteVersion removes a function version.
func (r *FunctionRepository) DeleteVersion(ctx context.Context, versionID uuid.UUID) error {
	query := `DELETE FROM function_versions WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, versionID)
	return err
}

// Tx handles database transactions.
type Tx struct {
	Tx pgx.Tx
}

// WithTx runs the callback within a transaction.
func WithTx(ctx context.Context, db *db.DB, fn func(ctx context.Context, tx *Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(ctx, &Tx{Tx: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}