package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5"
)

func main() {
	logger := slog.Default()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	var (
		toLabel int
		labeled int
	)
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FILTER (WHERE l.labeled_at IS NULL) AS to_label, COUNT(*) FILTER (WHERE l.labeled_at IS NOT NULL) AS labeled FROM jobs j LEFT JOIN labels l ON j.id = l.job_id WHERE j.duplicate_of IS NULL;").Scan(&toLabel, &labeled)
	if err != nil {
		logger.Error("failed to collect label statistic", "error", err)
		os.Exit(1)
	}
	fmt.Printf("LABELED %d/%d\n", labeled, toLabel)
}
