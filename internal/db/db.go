package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps *pgxpool.Pool with helper methods.
type DB struct {
	Pool *pgxpool.Pool
}

// New creates a new DB connection pool.
func New(databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &DB{Pool: pool}, nil
}

// Close releases the connection pool.
func (db *DB) Close() {
	db.Pool.Close()
}

// Health checks if the database is reachable.
func (db *DB) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	return db.Pool.Ping(ctx)
}