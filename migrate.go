package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"

	"github.com/pressly/goose/v3"
)

//go:embed db/migrations/*.sql
var migrationsFS embed.FS

// migrate applies every pending goose migration embedded under db/migrations,
// logging each one. Running it at startup lets a fresh database come up with the
// full schema without anyone invoking goose by hand.
func migrate(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	sub, err := fs.Sub(migrationsFS, "db/migrations")
	if err != nil {
		return fmt.Errorf("failed to remove FS prefix: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sub)
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
