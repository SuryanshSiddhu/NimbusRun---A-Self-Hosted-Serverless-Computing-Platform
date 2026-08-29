package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nimbusrun/nimbusrun/internal/db"
	"github.com/nimbusrun/nimbusrun/internal/models"
)

// VersionRepository handles function versions.
type VersionRepository struct {
	db *db.DB
}

// NewVersionRepository creates a new VersionRepository.
func NewVersionRepository(database *db.DB) *VersionRepository {
	return &VersionRepository{db: database}
}

// Create inserts a new function version.
func (r *VersionRepository) Create(ctx context.Context, v *models.FunctionVersion) error {
	query := `
		INSERT INTO function_versions (id, function_id, version_number, image_uri, status)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		v.ID, v.FunctionID, v.VersionNum, v.ImageURI, v.Status,
	)
	return err
}

// GetByID retrieves a version by ID.
func (r *VersionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.FunctionVersion, error) {
	query := `
		SELECT id, function_id, version_number, image_uri, status, created_at
		FROM function_versions WHERE id = $1
	`
	var v models.FunctionVersion
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&v.ID, &v.FunctionID, &v.VersionNum, &v.ImageURI, &v.Status, &v.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ListByFunctionID lists all versions for a function.
func (r *VersionRepository) ListByFunctionID(ctx context.Context, functionID uuid.UUID) ([]*models.FunctionVersion, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, function_id, version_number, image_uri, status, created_at
		FROM function_versions WHERE function_id = $1 ORDER BY version_number DESC
	`, functionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*models.FunctionVersion
	for rows.Next() {
		var v models.FunctionVersion
		err := rows.Scan(
			&v.ID, &v.FunctionID, &v.VersionNum, &v.ImageURI, &v.Status, &v.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		funcs := append(versions, &v)
		versions = funcs
	}
	return versions, nil
}

// LatestVersionNumber returns the latest version number for a function.
func (r *VersionRepository) LatestVersionNumber(ctx context.Context, functionID uuid.UUID) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(version_number), 0) FROM function_versions WHERE function_id = $1`,
		functionID,
	).Scan(&n)
	return n, err
}

// UpdateStatus updates a version's status.
func (r *VersionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE function_versions SET status = $1 WHERE id = $2`,
		status, id,
	)
	return err
}