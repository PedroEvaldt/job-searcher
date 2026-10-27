package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

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
	default:
		logger.Warn("wrong usage for radar: ./radar [status, label]")
		os.Exit(2)
	}
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
		fmt.Printf("Title: %s \nCompany: %s\nDescription: %s\n", title, company, description)

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
		if err != nil {
			logger.Error("failed to read user input", "error", err)
			return
		}
		if s := strings.TrimSpace(seniorityStr); s != "" {
			seniority = &s
		}
		fmt.Printf("Note: ")
		noteStr, err := reader.ReadString('\n')
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
