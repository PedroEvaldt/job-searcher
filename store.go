package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/sources"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the minimal set of pgx methods the store helpers need. Declaring it
// at the consumer lets those helpers accept either a *pgx.Conn or a pgx.Tx.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// openConn opens a pgx connection and verifies it is reachable with a Ping.
func openConn(ctx context.Context, dbURL string) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to start db connection: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close(ctx) // already failing; the ping error is the one that matters
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}
	return conn, nil
}

// openRun inserts a new row into runs with started_at = now() and returns its id.
func openRun(ctx context.Context, conn *pgx.Conn) (int, error) {
	const query = "INSERT INTO runs (started_at) VALUES (now()) RETURNING id;"
	var id int
	if err := conn.QueryRow(ctx, query).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// closeRun fills in the final counters, source id, error (if any) and finished_at on
// the run row opened by openRun. A nil pointer is stored as NULL. It uses its own
// context so the row is closed even when the source budget has already run out.
func closeRun(conn *pgx.Conn, fetched, failed, id int, sourceID *int, runErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const query = "UPDATE runs SET fetched = $1, failed = $2, source_id = $3, error = $4, finished_at = now() WHERE id = $5;"
	var runErrStr *string
	if runErr != nil {
		errStr := runErr.Error()
		runErrStr = &errStr
	}
	if _, err := conn.Exec(ctx, query, fetched, failed, sourceID, runErrStr, id); err != nil {
		return fmt.Errorf("failed to close run: %w", err)
	}
	return nil
}

// getSourceID looks up the surrogate id of a source by its natural name in the
// sources table.
func getSourceID(ctx context.Context, conn *pgx.Conn, source string) (int, error) {
	const query = "SELECT id FROM sources WHERE name = $1;"
	var id int
	if err := conn.QueryRow(ctx, query, source).Scan(&id); err != nil {
		return 0, fmt.Errorf("failed to query db: %w", err)
	}
	return id, nil
}

// contentHash returns the hex SHA-256 of the compacted JSON payload. Compacting
// drops insignificant whitespace but does not reorder keys (see D8).
func contentHash(item json.RawMessage) (string, error) {
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, item); err != nil {
		return "", fmt.Errorf("failed to compact raw json: %w", err)
	}
	h := sha256.Sum256(compacted.Bytes())
	return hex.EncodeToString(h[:]), nil
}

// insertRawJob appends one raw payload to raw_jobs, keyed by (source_id, external_id,
// content_hash). existed reports that the same payload was already stored (ON
// CONFLICT DO NOTHING), which is not a failure. Insert errors are logged here.
func insertRawJob(ctx context.Context, conn querier, logger *slog.Logger, item json.RawMessage, sourceID int, externalID, hash string) (id int, existed bool, err error) {
	const query = `
		INSERT INTO raw_jobs (source_id, external_id, payload, content_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (source_id, external_id, content_hash) DO NOTHING
		RETURNING id;`
	err = conn.QueryRow(ctx, query, sourceID, externalID, item, hash).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Debug("payload already stored, skipping", "source_id", sourceID, "external_id", externalID)
			return 0, true, nil
		}
		logger.Error("failed to insert item into raw_jobs", "error", err)
		return 0, false, err
	}
	logger.Debug("new raw_job inserted", "id", id)
	return id, false, nil
}

// insertJob upserts one normalized job into the jobs table. On conflict it updates
// the row in place and resets scored_at to NULL whenever the underlying raw_job
// changed, pushing a job whose text changed back into the scoring queue.
func insertJob(ctx context.Context, conn querier, rawJobID, sourceID int, externalID string, job sources.Job) error {
	const query = `
		INSERT INTO jobs (raw_job_id, source_id, external_id, title, company, url, description)
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
			END;`
	_, err := conn.Exec(ctx, query, rawJobID, sourceID, externalID, job.Title, job.Company, job.URL, job.Description)
	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}
	return nil
}

// sourceLooksBroken reports whether each of the last k runs of source failed, with
// an error or with zero items fetched (D36). A source with fewer than k runs is not
// broken yet: it may just be new.
func sourceLooksBroken(ctx context.Context, conn *pgx.Conn, k int, source string) (bool, error) {
	sourceID, err := getSourceID(ctx, conn, source)
	if err != nil {
		return false, err
	}

	const query = `SELECT error IS NOT NULL, fetched FROM runs WHERE source_id = $1 ORDER BY id DESC LIMIT $2;`
	rows, err := conn.Query(ctx, query, sourceID, k)
	if err != nil {
		return false, fmt.Errorf("failed to query last runs: %w", err)
	}
	defer rows.Close()

	var runs, failed int
	for rows.Next() {
		var (
			hasErr  bool
			fetched int
		)
		if err := rows.Scan(&hasErr, &fetched); err != nil {
			return false, fmt.Errorf("failed to scan run: %w", err)
		}
		runs++
		if hasErr || fetched == 0 {
			failed++
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("failed to read last runs: %w", err)
	}
	return runs == k && failed == k, nil
}
