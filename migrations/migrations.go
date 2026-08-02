package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/fmotalleb/go-tools/log"
	"go.uber.org/zap"
)

func All() []Migration {
	return []Migration{
		createUpdatedAtFn(),
		createDNSApiSchema(),
		settings(),
		optimizeForwardZones(),
		hijack(),
	}
}

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Migration struct {
	Version int64
	Name    string
	Up      func(context.Context, DBTX) error
}

type Runner struct {
	db *sql.DB
}

func NewRunner(db *sql.DB) *Runner {
	return &Runner{db: db}
}

func (r *Runner) Run(ctx context.Context) error {
	return Apply(ctx, r.db, All()...)
}

func Apply(ctx context.Context, db *sql.DB, migrations ...Migration) error {
	ctx, logger := log.AsNamedChild(ctx, "migrations")

	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	for _, migration := range migrations {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, migration.Version).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %d: %w", migration.Version, err)
		}
		if exists {
			logger.Info("skipping migration (already applied)", zap.Int64("version", migration.Version), zap.String("name", migration.Name))
			continue
		}

		logger.Info("applying migration", zap.Int64("version", migration.Version), zap.String("name", migration.Name))

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.Version, err)
		}

		if err := migration.Up(ctx, tx); err != nil {
			_ = tx.Rollback()
			logger.Error("failed to apply migration", zap.Int64("version", migration.Version), zap.String("name", migration.Name), zap.Error(err))
			return fmt.Errorf("apply migration %d: %w", migration.Version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`, migration.Version, time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.Version, err)
		}

		logger.Info("successfully applied migration", zap.Int64("version", migration.Version), zap.String("name", migration.Name))
	}

	return nil
}
