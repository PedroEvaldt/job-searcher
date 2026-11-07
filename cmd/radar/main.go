package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/PedroEvaldt/job-searcher/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

const usage = "usage: radar [status | label [queue.json] | digest | digest-loop | review]"

// main is the only place that logs a failure and picks the exit code; everything
// below it returns errors with context and leaves that decision here.
func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("radar failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Only label takes an optional argument: the queue file to label from.
	if len(args) == 0 || len(args) > 2 || (len(args) == 2 && args[0] != "label") {
		return errors.New(usage)
	}

	// Same as the collector: a local .env is picked up, compose sets the variables itself.
	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is not set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// digest-loop is a daemon: only an invalid config stops it, at start-up, since no
	// cycle could ever succeed with one. Every cycle error is logged and retried on the
	// next tick: Postgres or Discord down heal on their own, and a model with no scores
	// yet is normal right after jev-worker is switched on. Hence it is handled before
	// the shared connection below — each cycle opens its own, so a Postgres restart
	// does not leave the loop holding a dead connection.
	if args[0] == "digest-loop" {
		cfg, err := loadDigest()
		if err != nil {
			return err
		}
		cycle := func() error {
			conn, err := pgx.Connect(ctx, dbURL)
			if err != nil {
				return fmt.Errorf("connect to db: %w", err)
			}
			defer conn.Close(ctx) //nolint:errcheck // nothing left to do if closing fails on the way out
			return digest(ctx, conn, cfg)
		}
		digestor(ctx, slog.Default(), cfg.Every, cycle)
		return nil
	}
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
		var queue []int64
		if len(args) == 2 {
			if queue, err = loadLabelQueue(args[1]); err != nil {
				return err
			}
		}
		return label(ctx, conn, 100, queue)
	case "digest":
		cfg, err := loadDigest()
		if err != nil {
			return err
		}
		return digest(ctx, conn, cfg)
	case "review":
		return review(ctx, conn)
	default:
		return fmt.Errorf("unknown command %q; %s", mode, usage)
	}
}

// loadDigest reads config.yaml and checks its digest: section.
func loadDigest() (config.Digest, error) {
	cfg, err := config.Load(config.Path)
	if err != nil {
		return config.Digest{}, err
	}
	if err := cfg.Digest.Validate(); err != nil {
		return config.Digest{}, err
	}
	return cfg.Digest, nil
}
