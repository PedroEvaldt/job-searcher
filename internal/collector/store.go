package collector

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
// changed, pushing a job whose text changed back into the scoring queue. It also
// writes the dedupe keys (normalize) and returns the job's id, inserted or updated.
func insertJob(ctx context.Context, conn querier, rawJobID, sourceID int, externalID string, job sources.Job) (int, error) {
	companyKey, titleKey := normalize(job.Company, job.Title)
	const query = `
		INSERT INTO jobs (raw_job_id, source_id, external_id, title, company, url, description, salary_min, salary_max, currency, salary_period, posted_at, company_key, title_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT ON CONSTRAINT uq_source_id_external_id
		DO UPDATE SET
			title = EXCLUDED.title,
			company = EXCLUDED.company,
			url = EXCLUDED.url,
			description = EXCLUDED.description,
			raw_job_id = EXCLUDED.raw_job_id,
			salary_min = EXCLUDED.salary_min,
			salary_max = EXCLUDED.salary_max,
			currency = EXCLUDED.currency,
			salary_period = EXCLUDED.salary_period,
			posted_at = EXCLUDED.posted_at,
			company_key = EXCLUDED.company_key,
			title_key = EXCLUDED.title_key,
			skills_extracted_at = CASE
				WHEN EXCLUDED.raw_job_id = jobs.raw_job_id THEN jobs.skills_extracted_at
				ELSE NULL
			END,
			scored_at = CASE
				WHEN EXCLUDED.raw_job_id = jobs.raw_job_id THEN jobs.scored_at
				ELSE NULL
			END RETURNING id;`
	var jobID int
	var date *time.Time
	if !job.Date.IsZero() {
		date = &job.Date
	}
	err := conn.QueryRow(ctx, query, rawJobID, sourceID, externalID, job.Title, job.Company, job.URL, job.Description, job.SalaryMin, job.SalaryMax, job.Currency, job.SalaryPeriod, date, companyKey, titleKey).Scan(&jobID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert job: %w", err)
	}
	return jobID, nil
}

// updateDuplicate points the job just written (jobID, from source1) at the oldest
// live job that isDuplicate accepts, or clears duplicate_of when none does: an edit
// can turn a copy back into a job of its own. Only older jobs are candidates, so a
// job never points at itself, and only live ones, so copies never chain.
func updateDuplicate(ctx context.Context, conn querier, jobID, source1 int, job1 sources.Job) error {
	companyKey, titleKey := normalize(job1.Company, job1.Title)
	const query = `
		SELECT id, source_id, company, title, description
		FROM jobs
		WHERE company_key = $1 AND title_key = $2 AND duplicate_of IS NULL AND id < $3
		ORDER BY id;`
	rows, err := conn.Query(ctx, query, companyKey, titleKey, jobID)
	if err != nil {
		return fmt.Errorf("failed to query rows: %w", err)
	}
	defer rows.Close()

	// Stays nil when no candidate matches; pgx writes a nil pointer as NULL.
	var duplicatedID *int
	for rows.Next() {
		var (
			id      int
			source2 int
			job2    sources.Job
		)
		err = rows.Scan(&id, &source2, &job2.Company, &job2.Title, &job2.Description)
		if err != nil {
			return fmt.Errorf("failed to scan rows: %w", err)
		}
		if isDuplicate(job1, job2, source1, source2) {
			duplicatedID = &id
			break
		}
	}
	// Closed before the UPDATE: a pgx.Conn runs one query at a time, and the break
	// above leaves rows open (the deferred Close only runs at return). pgx reads
	// rows.Err only after Close.
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to get duplicates: %w", err)
	}

	_, err = conn.Exec(ctx, "UPDATE jobs SET duplicate_of = $1 WHERE id = $2;", duplicatedID, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job duplicated: %w", err)
	}
	return nil
}

// sourceLooksBroken reports whether each of the last k runs of source failed, with
// an error or with zero items fetched. A source with fewer than k runs is not
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

// storedJob is a job read back from the jobs table: its row id and source id, which
// sources.Job does not carry, plus the fields the dedupe rule compares.
type storedJob struct {
	id       int
	sourceID int
	job      sources.Job
}

// backfillDedupe gives dedupe keys and duplicate marks to the jobs written before the
// keys existed, the ones whose company_key is still NULL. It reads them batchSize at a
// time in id order, so every older job is already keyed when a newer one looks for its
// original. Each job is marked first and keyed last: if marking fails, the key stays
// NULL and the next run retries it. Setting company_key back to NULL is how to make a
// run re-evaluate jobs. It returns how many jobs it finished, also on error.
func backfillDedupe(ctx context.Context, conn querier, batchSize int) (int, error) {
	backfilled := 0
	for {
		batch := []storedJob{}
		rows, err := conn.Query(ctx, "SELECT id, source_id, title, company, description FROM jobs WHERE company_key IS NULL ORDER BY id LIMIT $1;", batchSize)
		if err != nil {
			return backfilled, fmt.Errorf("failed to query keyless jobs: %w", err)
		}
		for rows.Next() {
			j := storedJob{}
			err = rows.Scan(&j.id, &j.sourceID, &j.job.Title, &j.job.Company, &j.job.Description)
			if err != nil {
				rows.Close()
				return backfilled, fmt.Errorf("failed to scan keyless job: %w", err)
			}
			batch = append(batch, j)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return backfilled, fmt.Errorf("failed to read batch: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		for _, j := range batch {
			err = updateDuplicate(ctx, conn, j.id, j.sourceID, j.job)
			if err != nil {
				return backfilled, fmt.Errorf("failed to mark job %d: %w", j.id, err)
			}
			companyKey, titleKey := normalize(j.job.Company, j.job.Title)
			_, err = conn.Exec(ctx, "UPDATE jobs SET company_key = $1, title_key = $2 WHERE id = $3;", companyKey, titleKey, j.id)
			if err != nil {
				return backfilled, fmt.Errorf("failed to write keys of job %d: %w", j.id, err)
			}
			backfilled++
		}
	}
	return backfilled, nil
}

func normalizeRawJobs(ctx context.Context, conn querier, batchSize int) (int, error) {
	var lastSeenID int
	backfilledTotal := 0
	for {
		rows, err := conn.Query(ctx, `
			WITH ranked as (
				SELECT id, source_id, external_id, payload, 
				ROW_NUMBER() OVER (
					PARTITION BY source_id, external_id
					ORDER BY fetched_at DESC
				) as rn
			FROM raw_jobs
			) 
			SELECT r.id, r.source_id, r.external_id, r.payload, s.name 
			FROM ranked r JOIN sources s ON s.id = r.source_id
			WHERE r.rn = 1 AND r.id > $1
			ORDER BY r.id
			LIMIT $2;
		`, lastSeenID, batchSize)
		if err != nil {
			return backfilledTotal, fmt.Errorf("failed to query newest raw_jobs: %w", err)
		}

		// Read the whole batch first and close rows: pgx allows one active query per
		// connection, so insertJob cannot run while rows still holds it (conn busy).
		type rawRow struct {
			rawJobID   int
			sourceID   int
			externalID string
			sourceName string
			payload    json.RawMessage
		}
		var batch []rawRow
		for rows.Next() {
			var r rawRow
			if err := rows.Scan(&r.rawJobID, &r.sourceID, &r.externalID, &r.payload, &r.sourceName); err != nil {
				rows.Close()
				return backfilledTotal, fmt.Errorf("failed to read row: %w", err)
			}
			batch = append(batch, r)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return backfilledTotal, fmt.Errorf("failed to read batch: %w", err)
		}
		rows.Close()

		if len(batch) == 0 {
			break
		}

		for _, r := range batch {
			source, err := newSource(r.sourceName, nil)
			if err != nil {
				return backfilledTotal, fmt.Errorf("failed to get source from name: %w", err)
			}
			job, err := source.ToJob(r.payload)
			if err != nil {
				return backfilledTotal, fmt.Errorf("failed to convert source to job: %w", err)
			}
			if _, err := insertJob(ctx, conn, r.rawJobID, r.sourceID, r.externalID, job); err != nil {
				return backfilledTotal, fmt.Errorf("failed to insert job: %w", err)
			}
			lastSeenID = r.rawJobID
			backfilledTotal++
		}
	}
	return backfilledTotal, nil
}
