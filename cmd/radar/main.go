package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
)

const usage = "usage: radar [status | label | digest]"

// main is the only place that logs a failure and picks the exit code; everything
// below it returns errors with context and leaves that decision here.
func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("radar failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New(usage)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is not set")
	}

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect to db: %w", err)
	}
	defer conn.Close(ctx) //nolint:errcheck // nothing left to do if closing fails on the way out
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	switch mode := args[0]; mode {
	case "status":
		return status(ctx, conn)
	case "label":
		return label(ctx, conn, 100)
	case "digest":
		return digest(ctx, conn, "./config.yaml")
	default:
		return fmt.Errorf("unknown command %q; %s", mode, usage)
	}
}
