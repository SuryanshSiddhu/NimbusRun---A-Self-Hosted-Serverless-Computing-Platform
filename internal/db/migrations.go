package db

import (
	"context"
	"fmt"
	"os"
)

// RunMigrations executes all SQL files in the migrations directory.
func RunMigrations(pool *DB, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	ctx := context.Background()
	for _, entry := range entries {
		if entry.IsDir() || entry.Name()[0] == '.' {
			continue
		}
		data, err := os.ReadFile(migrationsDir + "/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", entry.Name(), err)
		}
		_, err = pool.Pool.Exec(ctx, string(data))
		if err != nil {
			return fmt.Errorf("executing migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}
