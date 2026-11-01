package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"go.yaml.in/yaml/v3"
)

const (
	maxJobsPerRound = 10
	// discordMaxContent is Discord's cap on a webhook message's content, counted in
	// characters (runes), not bytes.
	discordMaxContent = 2000
	// maxTitleRunes keeps a single entry far below discordMaxContent, so one absurd
	// title can never stop the digest from sending anything at all.
	maxTitleRunes = 200
	digestHeader  = "**Vagas de hoje**\n\n"
)

type digestConfig struct {
	ModelVersion string  `yaml:"model_version"`
	MinScore     float64 `yaml:"min_score"`
}

type digestJob struct {
	id      int64
	title   string
	company string
	url     string
	score   float64
}

// digest sends today's best unsent jobs to Discord, or prints them when no webhook
// is configured. Finding nothing new is a normal day, not an error.
func digest(ctx context.Context, conn *pgx.Conn, path string) error {
	cfg, err := loadDigestConfig(path)
	if err != nil {
		return err
	}
	if err := checkModelHasScores(ctx, conn, cfg.ModelVersion); err != nil {
		return err
	}

	jobs, err := pendingJobs(ctx, conn, cfg)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		slog.Info("no new jobs for the digest", "model_version", cfg.ModelVersion)
		return nil
	}

	msg, included := formatDigest(jobs, discordMaxContent)

	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		slog.Warn("DISCORD_WEBHOOK_URL is not set, printing the digest instead")
		fmt.Print(msg)
		return nil
	}

	// At-least-once: send first, mark after. A crash between the two re-sends these
	// jobs on the next run instead of losing them for good.
	if err := sendNotification(ctx, msg, webhookURL); err != nil {
		return fmt.Errorf("send discord notification: %w", err)
	}
	return markSent(ctx, conn, included, cfg.ModelVersion)
}

func loadDigestConfig(path string) (digestConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return digestConfig{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var file struct {
		Digest digestConfig `yaml:"digest"`
	}
	// Not KnownFields(true): radar only cares about the digest: key, but config.yaml's
	// other top-level keys (collect_every, sources) belong to the collector.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&file); err != nil {
		return digestConfig{}, fmt.Errorf("parse digest config: %w", err)
	}

	// An error, not a warning: with min_score wrong the digest would send nothing
	// forever while the scheduler sees a clean exit.
	cfg := file.Digest
	if cfg.MinScore < 0 || cfg.MinScore > 1 {
		return digestConfig{}, fmt.Errorf("digest.min_score is %v, expected 0-1", cfg.MinScore)
	}
	return cfg, nil
}

// checkModelHasScores catches a typo in digest.model_version, which would otherwise
// look exactly like "no new jobs" every single day.
func checkModelHasScores(ctx context.Context, conn *pgx.Conn, modelVersion string) error {
	var exists bool
	err := conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM scores WHERE model_version = $1);",
		modelVersion).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check scores for %q: %w", modelVersion, err)
	}
	if !exists {
		return fmt.Errorf("digest.model_version %q has no scores yet", modelVersion)
	}
	return nil
}

// pendingJobs returns the best-scored live jobs this model has not sent yet. "Sent"
// is per model_version: switching models may resend a job, by design (jobs_sent PK).
func pendingJobs(ctx context.Context, conn *pgx.Conn, cfg digestConfig) ([]digestJob, error) {
	rows, err := conn.Query(ctx, `
		SELECT j.id, j.title, j.company, j.url, s.score
		FROM jobs j
		JOIN scores s ON s.job_id = j.id
		WHERE s.model_version = $1
		  AND s.score >= $2
		  AND j.duplicate_of IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM jobs_sent js
		      WHERE js.job_id = j.id AND js.model_version = s.model_version
		  )
		ORDER BY s.score DESC, j.id
		LIMIT $3;`,
		cfg.ModelVersion, cfg.MinScore, maxJobsPerRound)
	if err != nil {
		return nil, fmt.Errorf("query jobs for digest: %w", err)
	}
	defer rows.Close()

	var jobs []digestJob
	for rows.Next() {
		var j digestJob
		if err := rows.Scan(&j.id, &j.title, &j.company, &j.url, &j.score); err != nil {
			return nil, fmt.Errorf("scan digest job: %w", err)
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fetch digest jobs: %w", err)
	}
	return jobs, nil
}

// formatDigest renders jobs into one message of at most limit runes. It never cuts an
// entry in half: jobs that do not fit are left out, and only the ones returned in
// included may be marked as sent — the rest stay pending for the next run.
func formatDigest(jobs []digestJob, limit int) (msg string, included []digestJob) {
	var b strings.Builder
	b.WriteString(digestHeader)
	size := utf8.RuneCountInString(digestHeader)

	for i, j := range jobs {
		// <url> keeps Discord from expanding a preview card under every job.
		entry := fmt.Sprintf("**%s** — %s\nScore: %.0f%%\n<%s>\n\n",
			truncateRunes(j.title, maxTitleRunes), j.company, j.score*100, j.url)
		n := utf8.RuneCountInString(entry)
		if size+n > limit {
			return b.String(), jobs[:i]
		}
		b.WriteString(entry)
		size += n
	}
	return b.String(), jobs
}

// truncateRunes cuts s to at most n runes, marking the cut with an ellipsis.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// markSent records every job in one transaction: either the whole digest is marked or
// none of it is, so a failure halfway never leaves a partial record.
func markSent(ctx context.Context, conn *pgx.Conn, jobs []digestJob, modelVersion string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin marking sent jobs: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	for _, j := range jobs {
		_, err := tx.Exec(ctx,
			"INSERT INTO jobs_sent (job_id, model_version) VALUES ($1, $2) ON CONFLICT DO NOTHING;",
			j.id, modelVersion)
		if err != nil {
			return fmt.Errorf("mark job %d as sent: %w", j.id, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sent jobs: %w", err)
	}
	return nil
}
