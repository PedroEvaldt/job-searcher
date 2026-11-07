package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"

	schema "github.com/PedroEvaldt/job-searcher/db"
	"github.com/pressly/goose/v3"
)

// applyMigrations applies every pending goose migration embedded in package db,
// logging each one. Running it at startup lets a fresh database come up with the
// full schema without anyone invoking goose by hand.
func applyMigrations(ctx context.Context, sqlDB *sql.DB, logger *slog.Logger) error {
	sub, err := fs.Sub(schema.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to remove FS prefix: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, sub)
	if err != nil {
		return fmt.Errorf("failed to get goose provider: %w", err)
	}
	migrations, err := provider.Up(ctx)
	if err != nil {
		var partialErr *goose.PartialError
		if errors.As(err, &partialErr) {
			logResults(logger, partialErr.Applied)
			if f := partialErr.Failed; f != nil {
				logger.Error("migration failed",
					"version", f.Source.Version,
					"file", filepath.Base(f.Source.Path),
					"duration", f.Duration,
					"error", f.Error)
			}
		}
		return fmt.Errorf("migrations failed: %w", err)
	}

	if len(migrations) == 0 {
		logger.Info("no migrations pending")
	} else {
		logResults(logger, migrations)
	}
	return nil
}

// logResults logs one INFO line per migration that goose reported.
func logResults(logger *slog.Logger, migrations []*goose.MigrationResult) {
	for _, migration := range migrations {
		logger.Info("migration applied",
			"version", migration.Source.Version,
			"file", filepath.Base(migration.Source.Path),
			"type", migration.Source.Type,
			"direction", migration.Direction,
			"duration", migration.Duration,
			"empty", migration.Empty)
	}
}

// Migrate opens a database/sql handle just for goose, applies every pending migration
// and closes it. It backs the collector's -migrate flag.
func Migrate(ctx context.Context, logger *slog.Logger, dbURL string) (err error) {
	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close database: %w", closeErr)
		}
	}()
	return applyMigrations(ctx, sqlDB, logger)
}
