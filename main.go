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
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.Default()
	if err := run(logger); err != nil {
		logger.Error("failed to run", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	err := godotenv.Load()
	if err != nil {
		logger.Warn("failed to load env configs", "error", err)
	}
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("failed start db connection: %w", err)
	}
	err = conn.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed ping db: %w", err)
	}
	logger.Info("connected to db successfully")

	err = insertRawJobs(ctx, conn, logger)
	if err != nil {
		return fmt.Errorf("failed to insert raw jobs: %w", err)
	}
	jobsMap, err := normalizeRawJobs(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to normalize raw jobs: %w", err)
	}
	for externals, job := range jobsMap {
		err = insertJob(ctx, conn, externals.RawJobID, externals.SourceID, externals.ExternalID, job)
		if err != nil {
			logger.Error("failed to insert job", "job_id", job.ID, "job_company", job.Company, "error", err)
			continue
		}
	}
	return nil
}

func insertJob(ctx context.Context, conn *pgx.Conn, rawjobID, sourceID int, externalID string, job Job) error {
	sql := "INSERT into jobs (raw_job_id, source_id, external_id, title, company, url, description) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT ON CONSTRAINT uq_source_id_external_id DO UPDATE SET title = EXCLUDED.title, company = EXCLUDED.company, url = EXCLUDED.url, description = EXCLUDED.description, raw_job_id = EXCLUDED.raw_job_id;"
	_, err := conn.Exec(ctx, sql, rawjobID, sourceID, externalID, job.Position, job.Company, job.URL, job.Description)
	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}
	return nil
}

func insertRawJobs(ctx context.Context, conn *pgx.Conn, logger *slog.Logger) error {
	sql := "SELECT id from sources WHERE name = $1"

	var id int
	err := conn.QueryRow(ctx, sql, "remoteok").Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to query db: %w", err)
	}
	resp, err := http.Get("https://remoteok.com/api")
	if err != nil {
		return fmt.Errorf("failed to fetch remoteok api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to fetch remoteok api, statusCode %v", resp.StatusCode)
	}
	var items []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return fmt.Errorf("failed to decode request body: %w", err)
	}
	for _, item := range items {
		var jobID JobID
		if err := json.Unmarshal(item, &jobID); err != nil {
			logger.Error("failed to unmarshal raw json to job", "error", err)
			continue
		}
		if jobID.ID == "" {
			continue
		}
		var compactedBuffer bytes.Buffer
		err = json.Compact(&compactedBuffer, item)
		if err != nil {
			return fmt.Errorf("failed to compact raw json: %w", err)
		}
		h := sha256.Sum256(compactedBuffer.Bytes())
		hash := hex.EncodeToString(h[:])

		var rawJobID int
		sql := "INSERT INTO raw_jobs (source_id, external_id, payload, content_hash) VALUES ($1, $2, $3, $4) ON CONFLICT (source_id, external_id, content_hash) DO NOTHING RETURNING id;"
		err := conn.QueryRow(ctx, sql, id, jobID.ID, item, hash).Scan(&rawJobID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				logger.Info("item already exists, jumping", "source_id", id, "external_id", jobID.ID)
				continue
			}
			return fmt.Errorf("failed to insert item into raw_jobs: %w", err)
		}
		logger.Info("new raw_job inserted", "id", rawJobID)
	}
	return nil
}

func normalizeRawJobs(ctx context.Context, conn *pgx.Conn) (map[JobExternals]Job, error) {
	sql := "SELECT id, source_id, external_id, payload FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY source_id, external_id ORDER BY fetched_at DESC) AS rn FROM raw_jobs)t WHERE rn = 1;"
	rows, err := conn.Query(ctx, sql)
	defer rows.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to query raw jobs: %w", err)
	}
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
