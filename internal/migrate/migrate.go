// Package migrate runs embedded SQL migrations on startup.
// This is a minimalist migration runner — it applies "up" migrations
// in order, tracking applied versions in a schema_migrations table.
package migrate

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.up.sql
var migrations embed.FS

// Run applies all pending .up.sql migrations in /sql, in filename order.
// Safe to call on every startup.
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	// Ensure the tracking table exists.
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Collect migration files.
	entries, err := fs.ReadDir(migrations, "sql")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// Apply each that hasn't been applied yet.
	for _, name := range files {
		version := strings.TrimSuffix(name, ".up.sql")

		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`,
			version,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check version %s: %w", version, err)
		}
		if exists {
			continue
		}

		body, err := migrations.ReadFile("sql/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`,
			version,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record %s: %w", name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}

		slog.Info("migration applied", "version", version)
	}

	return nil
}
