package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.yaml.in/yaml/v3"
)

var (
	brTag           = regexp.MustCompile(`(?i)<br\s*/?>`)
	blockClose      = regexp.MustCompile(`(?i)</(p|div|li|ul|ol|h[1-6])>`)
	anyTag          = regexp.MustCompile(`<[^>]+>`)
	blankRuns       = regexp.MustCompile(`\n{3,}`)
	maxJobsPerRound = 10
)

// cleanDescription makes a job description readable on screen while labelling:
// it turns line/block tags into newlines, drops every other tag, decodes HTML
// entities, and collapses blank-line runs. It is a DISPLAY helper only — the
// description stored in the database is never touched.
func cleanDescription(desc string) string {
	desc = brTag.ReplaceAllString(desc, "\n")
	desc = blockClose.ReplaceAllString(desc, "\n")
	desc = anyTag.ReplaceAllString(desc, "")
	// Unescape AFTER stripping, so literal "&lt;b&gt;" in the text is not turned
	// into a real tag and then removed.
	desc = html.UnescapeString(desc)
	desc = blankRuns.ReplaceAllString(desc, "\n")
	return strings.TrimSpace(desc)
}

func main() {
	logger := slog.Default()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		logger.Error("failed to start db connection", "error", err)
		os.Exit(1)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close(ctx) // already failing; the ping error is the one that matters
		logger.Error("failed to ping db", "error", err)
		os.Exit(1)
	}
	if len(os.Args) != 2 {
		logger.Warn("wrong usage for radar: ./radar [status, label]")
		os.Exit(2)
	}
	switch mode := os.Args[1]; mode {
	case "status":
		status(ctx, logger, conn)
	case "label":
		label(ctx, logger, conn, 100)
	case "digest":
		digest(ctx, logger, conn, "./config.yaml")
	default:
		logger.Warn("wrong usage for radar: ./radar [status, label]")
		os.Exit(2)
	}
}

func digest(ctx context.Context, logger *slog.Logger, conn *pgx.Conn, path string) error {
	type Digest struct {
		ModelVersion string  `yaml:"model_version"`
		MinScore     float64 `yaml:"min_score"`
	}

	type Config struct {
		Digest Digest `yaml:"digest"`
	}

	data, err := os.ReadFile(path)
	if err != nil {
		logger.Error("failed to read config", "path", path, "error", err)
		return err
	}

	var cfg Config
	// Not KnownFields(true): radar only cares about the digest: key, but config.yaml's
	// other top-level keys (collect_every, sources) belong to the collector.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&cfg); err != nil {
		logger.Error("failed to parse digest config", "error", err)
		return err
	}

	var models []string
	rows, err := conn.Query(ctx, "SELECT DISTINCT model_version FROM scores;")
	if err != nil {
		logger.Error("failed to query model versions", "error", err)
		return err
	}
	for rows.Next() {
		var model string
		err = rows.Scan(&model)
		if err != nil {
			logger.Error("failed to scan model version", "error", err)
			return err
		}
		models = append(models, model)
	}

	if cfg.Digest.MinScore < 0 || cfg.Digest.MinScore > 1 {
		logger.Warn("digest.min_score out of range, expected 0-1", "min_score", cfg.Digest.MinScore)
		return nil
	}

	if !slices.Contains(models, cfg.Digest.ModelVersion) {
		logger.Warn("digest.model_version has no scores yet", "model_version", cfg.Digest.ModelVersion)
		return nil
	}

	rows, err = conn.Query(ctx, "SELECT title FROM jobs JOIN scores ON jobs.id = scores.job_id WHERE model_version = $1 AND score >= $2 ORDER BY score DESC LIMIT $3;", cfg.Digest.ModelVersion, cfg.Digest.MinScore, maxJobsPerRound)
	if err != nil {
		logger.Error("failed to query jobs for digest", "error", err)
		return err
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title string
		err = rows.Scan(&title)
		if err != nil {
			logger.Error("failed to scan job title", "error", err)
			return err
		}
		titles = append(titles, title)
	}
	for i, t := range titles {
		fmt.Printf("%d: %s\n", (i + 1), t)
	}
	return nil
}

func label(ctx context.Context, logger *slog.Logger, conn *pgx.Conn, limit int) {
	reader := bufio.NewReader(os.Stdin)
	var (
		jobID       int
		title       string
		company     string
		description string
	)
	counter := 0
	for {
		if counter >= limit {
			return
		}
		err := conn.QueryRow(ctx, "SELECT j.id, j.title, j.company, j.description FROM jobs j LEFT JOIN labels l ON j.id = l.job_id WHERE l.labeled_at IS NULL AND j.duplicate_of IS NULL ORDER BY j.id LIMIT 1;").Scan(&jobID, &title, &company, &description)
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("nothing left to label")
			return
		}
		if err != nil {
			logger.Error("failed to query database for labeling", "error", err)
			return
		}
		clearScreen()
		fmt.Printf("Title: %s\nCompany: %s\n\n%s\n", title, company, cleanDescription(description))

		fmt.Print("Would apply (y/n): ")
		var (
			wouldApply bool = false
			hiresLatam *bool
			seniority  *string
			note       *string
		)
	outer:
		for {
			wouldApplyStr, err := reader.ReadString('\n')
			if errors.Is(err, io.EOF) {
				fmt.Println("shutting down")
				return
			}
			if err != nil {
				logger.Error("failed to read user input", "error", err)
				return
			}
			wouldApplyStr = strings.TrimSpace(wouldApplyStr)
			switch {
			case wouldApplyStr == "y":
				wouldApply = true
				break outer
			case wouldApplyStr == "n":
				wouldApply = false
				break outer
			default:
				fmt.Printf("answer must be y or n!\n")
			}
		}
		fmt.Printf("Hires Latam (y|n): ")
		hiresLatamStr, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			fmt.Println("shutting down")
			return
		}
		if err != nil {
			logger.Error("failed to read user input", "error", err)
			return
		}
		if strings.TrimSpace(hiresLatamStr) == "y" {
			v := true
			hiresLatam = &v
		}
		fmt.Printf("Seniority (junior|mid-level|senior): ")
		seniorityStr, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			fmt.Println("shutting down")
			return
		}
		if err != nil {
			logger.Error("failed to read user input", "error", err)
			return
		}
		if s := strings.TrimSpace(seniorityStr); s != "" {
			seniority = &s
		}
		fmt.Printf("Note: ")
		noteStr, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			fmt.Println("shutting down")
			return
		}
		if err != nil {
			logger.Error("failed to read user input", "error", err)
			return
		}
		if s := strings.TrimSpace(noteStr); s != "" {
			note = &s
		}
		_, err = conn.Exec(ctx, "INSERT INTO labels (job_id, would_apply, hires_latam, seniority, note, labeled_at) VALUES ($1, $2, $3, $4, $5, now());", jobID, wouldApply, hiresLatam, seniority, note)
		if err != nil {
			logger.Error("failed to insert labels", "job id", jobID, "error", err)
			return
		}
		counter++
	}
}

func status(ctx context.Context, logger *slog.Logger, conn *pgx.Conn) {
	var (
		toLabel int
		labeled int
	)
	err := conn.QueryRow(ctx, "SELECT COUNT(*) FILTER (WHERE l.labeled_at IS NULL) AS to_label, COUNT(*) FILTER (WHERE l.labeled_at IS NOT NULL) AS labeled FROM jobs j LEFT JOIN labels l ON j.id = l.job_id WHERE j.duplicate_of IS NULL;").Scan(&toLabel, &labeled)
	if err != nil {
		logger.Error("failed to collect label statistic", "error", err)
		return
	}
	fmt.Printf("LABELED %d/%d\n", labeled, toLabel)
}

func clearScreen() {
	fmt.Print("\033[H\033[2J") // cursor to home, then clear the whole screen
}
