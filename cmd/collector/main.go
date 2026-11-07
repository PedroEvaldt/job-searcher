// Command collector pulls remote jobs from every enabled source into Postgres, every
// collect_every (config.yaml), until SIGINT or SIGTERM.
//
//	collector               collect forever
//	collector -once         one collection pass, then exit
//	collector -migrate      apply pending migrations, then exit (the compose "migrate" service)
//	collector -renormalize  re-parse every stored raw payload into jobs, then exit
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/collector"
	"github.com/PedroEvaldt/job-searcher/internal/config"
	"github.com/joho/godotenv"
)

// dbWaitTimeout is how long start-up waits for Postgres before giving up. On the Pi,
// the database container may still be starting when the collector comes up.
const dbWaitTimeout = 2 * time.Minute

// main wires the process together and is the only place that exits it.
func main() {
	logger := slog.Default()

	once := flag.Bool("once", false, "run one collection pass and exit")
	renormalize := flag.Bool("renormalize", false, "re-parse every stored raw payload into jobs and exit")
	migrateOnly := flag.Bool("migrate", false, "apply pending migrations and exit")
	flag.Parse()

	// A missing .env is normal in the containers, where compose sets the variables.
	if err := godotenv.Load(); err != nil {
		logger.Debug("no .env loaded", "error", err)
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	cfg, err := config.Load(config.Path)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbCtx, cancel := context.WithTimeout(ctx, dbWaitTimeout)
	defer cancel()
	if err := collector.WaitForDB(dbCtx, logger, dbURL, 100*time.Millisecond); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error("could not connect to the database", "waited", dbWaitTimeout, "error", err)
			os.Exit(1)
		}
		logger.Info("shutting down")
		return
	}

	switch {
	case *migrateOnly:
		// Every other compose service waits for this one to exit 0
		// (service_completed_successfully), so none starts before the schema exists.
		// Run still migrates at the start of each pass: a no-op by then.
		if err := collector.Migrate(dbCtx, logger, dbURL); err != nil {
			logger.Error("failed to migrate", "error", err)
			os.Exit(1)
		}
	case *renormalize:
		n, err := collector.Renormalize(ctx, dbURL)
		if err != nil {
			logger.Error("failed to renormalize jobs", "error", err)
			os.Exit(1)
		}
		logger.Info("renormalized jobs", "total", n)
	case *once:
		if err := collector.Run(ctx, logger, dbURL, cfg); err != nil {
			logger.Error("failed to run", "error", err)
			os.Exit(1)
		}
	default:
		collector.Loop(ctx, logger, dbURL, cfg)
	}
}
