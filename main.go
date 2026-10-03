package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/PedroEvaldt/job-searcher/internal/sources"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"go.yaml.in/yaml/v3"
)

//go:embed db/migrations/*.sql
var migrationsFS embed.FS

type Source interface {
	Fetch(string) ([]json.RawMessage, error)
	ExternalID(json.RawMessage) (string, error)
	ToJob(json.RawMessage) (sources.Job, error)
}

func For(name string) (Source, error) {
	switch name {
	case "remoteok":
		return sources.RemoteOK{}, nil
	case "remotive":
		return sources.Remotive{}, nil
	default:
		return nil, fmt.Errorf("unknown source: %s", name)
	}
}

// querier is the minimal set of pgx methods the store helpers need. Declaring it
// at the consumer lets those helpers accept either a *pgx.Conn or a pgx.Tx.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Config is the parsed config.yaml: the list of sources the collector may pull from.
type Config struct {
	SourcesConfig []sources.SourceConfig `yaml:"sources"`
}

// Load reads the YAML file at path and unmarshals it into a Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}
	return &cfg, nil
}

// main wires the process together: load .env and DATABASE_URL, load config.yaml,
// then run a single collection pass. It is the only place that exits the process.
func main() {
	logger := slog.Default()

	err := godotenv.Load()
	if err != nil {
		logger.Debug("failed to load env configs", "error", err)
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	cfg, err := Load("./config.yaml")
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := run(logger, dbURL, cfg); err != nil {
		logger.Error("failed to run", "error", err)
		os.Exit(1)
	}
}

// migrate applies every pending goose migration embedded under db/migrations,
// logging each one. Running it at startup lets a fresh database come up with the
// full schema without anyone invoking goose by hand.
func migrate(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	migrationsFS, err := fs.Sub(migrationsFS, "db/migrations")
	if err != nil {
		return fmt.Errorf("failed to remove FS prefix: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationsFS)
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

// run performs one full collection pass: open the Postgres connection, apply
// migrations, then process each enabled source in turn. A failure in one source is
// logged and skipped, not fatal to the others.
func run(logger *slog.Logger, dbURL string, cfg *Config) (err error) {
	ctx := context.Background()
	conn, err := openConn(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to open db conn: %w", err)
	}
	defer func() {
		err := conn.Close(ctx)
		if err != nil {
			logger.Error("failed to close connection", "error", err)
		}
	}()
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			logger.Error("failed to close database", "error", err)
		}
	}()

	err = migrate(ctx, db, logger)
	if err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}
	for _, sourceConfig := range cfg.SourcesConfig {
		if sourceConfig.Enabled {
			source, err := For(sourceConfig.Name)
			if err != nil {
				logger.Warn("config name does not match valid source", "error", err)
				continue
			}
			err = processSource(ctx, conn, logger, source, sourceConfig)
			if err != nil {
				logger.Warn("failed to process source", "source", sourceConfig.Name, "error", err)
				continue
			}
		}
	}
	return nil
}

// processSource handles a single source end to end and records the attempt in the
// runs table. It opens the run row first (closed via a deferred closeRun over the
// named return) so even a failure before fetching leaves a trace, then resolves the
// source id, fetches its items, and stores them.
func processSource(ctx context.Context, conn *pgx.Conn, logger *slog.Logger, source Source, sourceConfig sources.SourceConfig) (err error) {
	var (
		fetched     int
		failed      int
		sourceID    int
		sourceIDPtr *int
	)

	runID, err := openRun(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	defer func() {
		closeErr := closeRun(ctx, conn, fetched, failed, runID, sourceIDPtr, err)
		if closeErr != nil {
			logger.Error("failed to close run", "error", closeErr)
		}
	}()
	sourceID, err = getSourceID(ctx, conn, sourceConfig.Name)
	if err != nil {
		return fmt.Errorf("failed to get source id: %w", err)
	}
	sourceIDPtr = &sourceID

	items, err := source.Fetch(sourceConfig.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
	}

	fetched = len(items)
	failed, err = collectAndStoreData(ctx, conn, logger, items, source, sourceID)
	if err != nil {
		return fmt.Errorf("failed to collect and insert data: %w", err)
	}
	logger.Info("source collected", "source", sourceConfig.Name, "fetched", fetched, "failed", failed)
	return nil
}

// openConn opens a pgx connection and verifies it is reachable with a Ping.
func openConn(ctx context.Context, dbURL string) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed start db connection: %w", err)
	}
	err = conn.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed ping db: %w", err)
	}
	return conn, nil
}

// openRun inserts a new row into runs with started_at = now() and returns its id.
func openRun(ctx context.Context, conn *pgx.Conn) (int, error) {
	var id int
	sql := "INSERT INTO runs (started_at) VALUES (now()) RETURNING id;"
	err := conn.QueryRow(ctx, sql).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// closeRun fills in the final counters, source id and error (if any) on the run row
// opened by openRun. A nil pointer is stored as NULL.
func closeRun(ctx context.Context, conn *pgx.Conn, fetched, failed, id int, sourceID *int, runErr error) error {
	var runErrStr *string
	sql := "UPDATE runs SET fetched = $1, failed = $2, source_id = $3, error = $4 WHERE id = $5;"
	if runErr != nil {
		errStr := runErr.Error()
		runErrStr = &errStr
	}
	_, err := conn.Exec(ctx, sql, fetched, failed, sourceID, runErrStr, id)
	if err != nil {
		return fmt.Errorf("failed to close run: %w", err)
	}
	return nil
}

func compactJSON(item json.RawMessage) (string, error) {
	var compactedJSON bytes.Buffer
	err := json.Compact(&compactedJSON, item)
	if err != nil {
		return "", fmt.Errorf("failed to compact raw json: %w", err)
	}
	h := sha256.Sum256(compactedJSON.Bytes())
	hash := hex.EncodeToString(h[:])
	return hash, nil
}

// collectAndStoreData persists one source's items in two stages: first append the
// raw payloads to raw_jobs (outside any transaction, since that table is append-only
// and idempotent), then, inside a single transaction, normalize raw_jobs into the
// jobs table so a reader never sees a half-written batch. Returns the number of items
// that failed to land in raw_jobs.
func collectAndStoreData(ctx context.Context, conn *pgx.Conn, logger *slog.Logger, items []json.RawMessage, source Source, sourceID int) (int, error) {
	failed := 0
	for _, item := range items {
		externalID, err := source.ExternalID(item)
		if err != nil {
			logger.Warn("failed to get external id", "error", err)
			failed++
			continue
		}
		hash, err := compactJSON(item)
		if err != nil {
			logger.Warn("failed to hash payload", "external_id", externalID, "error", err)
			failed++
			continue
		}
		rawJobID, existed, err := insertRawJobs(ctx, conn, logger, item, sourceID, externalID, hash)

		if existed {
			continue
		}
		if err != nil {
			// insertRawJobs already logged the detail.
			failed++
			continue
		}

		job, err := source.ToJob(item)
		if err != nil {
			logger.Warn("failed to map item to job", "external_id", externalID, "error", err)
			failed++
			continue
		}
		err = insertJob(ctx, conn, rawJobID, sourceID, externalID, job)
		if err != nil {
			logger.Warn("failed to insert job", "external_id", externalID, "error", err)
			failed++
			continue
		}
	}
	return failed, nil
}

// getSourceID looks up the surrogate id of a source by its natural name in the
// sources table.
func getSourceID(ctx context.Context, conn *pgx.Conn, source string) (int, error) {
	sql := "SELECT id from sources WHERE name = $1"

	var id int
	err := conn.QueryRow(ctx, sql, source).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to query db: %w", err)
	}
	return id, nil
}

// insertRawJobs appends each item's raw payload to raw_jobs, keyed by
// (source_id, external_id, content_hash). A payload whose hash already exists is
// skipped via ON CONFLICT DO NOTHING and is not counted as a failure; only items
// that cannot be parsed or inserted increment the returned failed count.
func insertRawJobs(ctx context.Context, conn querier, logger *slog.Logger, item json.RawMessage, sourceID int, externalID, hash string) (int, bool, error) {
	var rawJobID int
	sql := "INSERT INTO raw_jobs (source_id, external_id, payload, content_hash) VALUES ($1, $2, $3, $4) ON CONFLICT (source_id, external_id, content_hash) DO NOTHING RETURNING id;"
	err := conn.QueryRow(ctx, sql, sourceID, externalID, item, hash).Scan(&rawJobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Debug("item already exists, jumping", "source_id", sourceID, "external_id", externalID)
			return 0, true, nil
		}
		logger.Error("failed to insert item into raw_jobs", "error", err)
		return 0, false, err
	}
	logger.Info("new raw_job inserted", "id", rawJobID)
	return rawJobID, false, nil
}

// insertJob upserts one normalized job into the jobs table. On conflict it updates
// the row in place and resets scored_at to NULL whenever the underlying raw_job
// changed, pushing a job whose text changed back into the scoring queue.
func insertJob(ctx context.Context, conn querier, rawjobID, sourceID int, externalID string, job sources.Job) error {
	sql := `
		INSERT into jobs (raw_job_id, source_id, external_id, title, company, url, description) 
		VALUES ($1, $2, $3, $4, $5, $6, $7) 
		ON CONFLICT ON CONSTRAINT uq_source_id_external_id 
		DO UPDATE SET 
			title = EXCLUDED.title, 
			company = EXCLUDED.company, 
			url = EXCLUDED.url, 
			description = EXCLUDED.description, 
			raw_job_id = EXCLUDED.raw_job_id,
			scored_at = CASE
							WHEN EXCLUDED.raw_job_id = jobs.raw_job_id THEN jobs.scored_at
							ELSE NULL
						END;
		`
	_, err := conn.Exec(ctx, sql, rawjobID, sourceID, externalID, job.Title, job.Company, job.URL, job.Description)
	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}
	return nil
}
