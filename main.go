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
	"net/http"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"go.yaml.in/yaml/v3"
)

//go:embed db/migrations/*.sql
var migrationsFS embed.FS

var source = "https://remoteok.com/api"

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Config struct {
	Sources []Source `yaml:"sources"`
}

type Source struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
	Url     string `yaml:"url"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}
	return &cfg, nil
}

func main() {
	logger := slog.Default()

	err := godotenv.Load()
	if err != nil {
		logger.Warn("failed to load env configs", "error", err)
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
	for _, source := range cfg.Sources {
		if source.Enabled {
			err = processSource(ctx, conn, logger, source)
			if err != nil {
				logger.Warn("failed to process source", "source", source.Name, "error", err)
				continue
			}
		}
	}
	return nil
}

func processSource(ctx context.Context, conn *pgx.Conn, logger *slog.Logger, source Source) error {
	var (
		fetched     int
		failed      int
		sourceID    int
		sourceIDPtr *int
	)

	runId, err := openRun(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	defer func() {
		closeErr := closeRun(ctx, conn, fetched, failed, runId, sourceIDPtr, err)
		if closeErr != nil {
			logger.Error("failed to close run", "error", closeErr)
		}
	}()
	sourceID, err = getSourceID(ctx, conn, source.Name)
	if err != nil {
		return fmt.Errorf("failed to get source id: %w", err)
	}
	sourceIDPtr = &sourceID

	items, err := fetchData(source.Url)
	if err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
	}

	fetched = len(items)
	failed, err = collectAndStoreData(ctx, conn, logger, items, sourceID)
	if err != nil {
		return fmt.Errorf("failed to collect and insert data: %w", err)
	}
	return nil
}

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

func openRun(ctx context.Context, conn *pgx.Conn) (int, error) {
	var id int
	sql := "INSERT INTO runs (started_at) VALUES (now()) RETURNING id;"
	err := conn.QueryRow(ctx, sql).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

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

func collectAndStoreData(ctx context.Context, conn *pgx.Conn, logger *slog.Logger, items []json.RawMessage, sourceID int) (int, error) {
	failed, err := insertRawJobs(ctx, conn, logger, items, sourceID)
	if err != nil {
		return failed, fmt.Errorf("failed to insert raw jobs: %w", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return failed, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			logger.Error("failed to rollback transaction", "error", rollbackErr)
		}
	}()
	jobsMap, err := normalizeRawJobs(ctx, tx)
	if err != nil {
		return failed, fmt.Errorf("failed to normalize raw jobs: %w", err)
	}
	for externals, job := range jobsMap {
		err = insertJob(ctx, tx, externals.RawJobID, externals.SourceID, externals.ExternalID, job)
		if err != nil {
			return failed, fmt.Errorf("failed to insert job %s: %w", job.ID, err)
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		return failed, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return failed, nil
}

func getSourceID(ctx context.Context, conn *pgx.Conn, source string) (int, error) {
	sql := "SELECT id from sources WHERE name = $1"

	var id int
	err := conn.QueryRow(ctx, sql, source).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to query db: %w", err)
	}
	return id, nil
}

func insertRawJobs(ctx context.Context, conn querier, logger *slog.Logger, items []json.RawMessage, sourceID int) (int, error) {
	failed := 0
	for _, item := range items {
		var jobID JobID
		if err := json.Unmarshal(item, &jobID); err != nil {
			logger.Error("failed to unmarshal raw json to job", "error", err)
			failed++
			continue
		}
		if jobID.ID == "" {
			continue
		}
		var compactedBuffer bytes.Buffer
		err := json.Compact(&compactedBuffer, item)
		if err != nil {
			return failed, fmt.Errorf("failed to compact raw json: %w", err)
		}
		h := sha256.Sum256(compactedBuffer.Bytes())
		hash := hex.EncodeToString(h[:])

		var rawJobID int
		sql := "INSERT INTO raw_jobs (source_id, external_id, payload, content_hash) VALUES ($1, $2, $3, $4) ON CONFLICT (source_id, external_id, content_hash) DO NOTHING RETURNING id;"
		err = conn.QueryRow(ctx, sql, sourceID, jobID.ID, item, hash).Scan(&rawJobID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				logger.Info("item already exists, jumping", "source_id", sourceID, "external_id", jobID.ID)
				continue
			}
			logger.Error("failed to insert item into raw_jobs", "error", err)
			failed++
			continue
		}
		logger.Info("new raw_job inserted", "id", rawJobID)
	}
	return failed, nil
}

func normalizeRawJobs(ctx context.Context, conn querier) (map[JobExternals]Job, error) {
	sql := "SELECT id, source_id, external_id, payload FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY source_id, external_id ORDER BY fetched_at DESC) AS rn FROM raw_jobs)t WHERE rn = 1;"
	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("failed to query raw jobs: %w", err)
	}
	defer rows.Close()
	jobs := make(map[JobExternals]Job, 0)
	for rows.Next() {
		var (
			id         int
			sourceID   int
			externalID string
			payload    json.RawMessage
			job        Job
		)
		err := rows.Scan(&id, &sourceID, &externalID, &payload)
		if err != nil {
			return nil, fmt.Errorf("failed to scan raw job: %w", err)
		}
		if err := json.Unmarshal(payload, &job); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
		external := JobExternals{
			RawJobID:   id,
			SourceID:   sourceID,
			ExternalID: externalID,
		}
		jobs[external] = job
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}
	return jobs, nil
}

func insertJob(ctx context.Context, conn querier, rawjobID, sourceID int, externalID string, job Job) error {
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
	_, err := conn.Exec(ctx, sql, rawjobID, sourceID, externalID, job.Position, job.Company, job.URL, job.Description)
	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}
	return nil
}

func fetchData(sourceURL string) ([]json.RawMessage, error) {
	resp, err := http.Get(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", sourceURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch %s, statusCode %v", sourceURL, resp.StatusCode)
	}
	var items []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to decode request body: %w", err)
	}
	return items, nil
}
