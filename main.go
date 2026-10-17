package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/client"
	"github.com/PedroEvaldt/job-searcher/internal/sources"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

const (
	// failureThreshold is the share of a source's items that may fail before the
	// whole source is reported as failed.
	failureThreshold = 0.5
	// himalayasMaxPages caps how many cursor pages a single Himalayas run fetches.
	himalayasMaxPages = 20
	// httpTimeout bounds one HTTP request; the source budget (timeout in
	// config.yaml) bounds all the work done for a source.
	httpTimeout = 30 * time.Second
	// brokenAfterRuns is how many runs in a row a source must fail (error or zero
	// items) before the cycle warns about it. At collect_every 6h, 2 runs is 12h
	// without data.
	brokenAfterRuns = 2
)

// Source is what the collector needs from a job source: fetch the raw items, read
// each item's id, and map an item to the common Job model.
type Source interface {
	Fetch(context.Context, string) ([]json.RawMessage, error)
	ExternalID(json.RawMessage) (string, error)
	ToJob(json.RawMessage) (sources.Job, error)
}

// newSource returns the Source registered under name, wired to the given client.
func newSource(name string, c *client.Client) (Source, error) {
	switch name {
	case "remoteok":
		return sources.RemoteOK{C: c}, nil
	case "remotive":
		return sources.Remotive{C: c}, nil
	case "himalayas":
		return sources.Himalayas{C: c, MaxPages: himalayasMaxPages}, nil
	default:
		return nil, fmt.Errorf("unknown source: %s", name)
	}
}

// main wires the process together: load .env and DATABASE_URL, load config.yaml,
// then collect every collect_every until SIGINT or SIGTERM, or run a single
// collection pass with -once. It is the only place that exits the process.
func main() {
	logger := slog.Default()

	err := godotenv.Load()
	if err != nil {
		logger.Debug("failed to load env configs", "error", err)
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	cfg, err := loadConfig("./config.yaml")
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	once := flag.Bool("once", false, "run one time only")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	err = waitForDB(dbCtx, logger, dbURL, 100*time.Millisecond)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error("could not connect to database in less than 2 minutes", "error", err)
			os.Exit(1)
		}
		logger.Info("shutting down")
		return
	}

	cycle := func() error { return run(ctx, logger, dbURL, cfg) }
	if *once {
		if err := cycle(); err != nil {
			logger.Error("failed to run", "error", err)
			os.Exit(1)
		}
	} else {
		collector(ctx, logger, cfg.CollectEvery, cycle)
	}
}

func waitForDB(ctx context.Context, logger *slog.Logger, dbURL string, initialDBWait time.Duration) error {
	counter := 1
	for {
		conn, err := openConn(ctx, dbURL)
		if err != nil {
			logger.Warn("failed to connect to database", "error", err, "attempt", counter)
			counter++
			err := client.Sleep(ctx, initialDBWait)
			if err != nil {
				return err
			}
			initialDBWait *= 2
		} else {
			_ = conn.Close(ctx) // only probing; run opens its own connection
			return nil
		}
	}
}

// collector runs cycle once right away, then again on every tick of interval, and
// returns when ctx is cancelled. A failed cycle is logged and does not stop the
// loop: the next tick tries again.
func collector(ctx context.Context, logger *slog.Logger, interval time.Duration, cycle func() error) {
	if err := cycle(); err != nil {
		logger.Error("failed to run", "error", err)
	}

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if err := cycle(); err != nil {
				logger.Error("failed to run", "error", err)
			}
		case <-ctx.Done():
			logger.Info("shutting down")
			return
		}
	}
}

// run performs one full collection pass: open the Postgres connection, apply
// migrations, then process each enabled source in turn. A failure in one source is
// logged and skipped, not fatal to the others; run fails only when no source succeeds.
func run(ctx context.Context, logger *slog.Logger, dbURL string, cfg *Config) (err error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := openConn(dbCtx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to open db conn: %w", err)
	}
	defer func() {
		err := conn.Close(dbCtx)
		if err != nil {
			logger.Error("failed to close connection", "error", err)
		}
	}()
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			logger.Error("failed to close database", "error", err)
		}
	}()

	err = migrate(dbCtx, db, logger)
	if err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}
	var runErrs []error
	var success bool
	for _, sourceConfig := range cfg.SourcesConfig {
		if errors.Is(ctx.Err(), context.Canceled) {
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		}
		if !sourceConfig.Enabled {
			continue
		}
		c := client.NewClient(sourceConfig.Retries, httpTimeout, sourceConfig.InitialWait, sourceConfig.Interval)
		source, err := newSource(sourceConfig.Name, c)
		if err != nil {
			runErrs = append(runErrs, err)
			logger.Warn("config name does not match a known source", "error", err)
			continue
		}

		err = processSource(ctx, conn, logger, source, sourceConfig, sourceConfig.Timeout)
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("failed to process %s: %w", sourceConfig.Name, err))
			logger.Warn("failed to process source", "source", sourceConfig.Name, "error", err)
			continue
		}
		success = true
	}

	for _, sourceConfig := range cfg.SourcesConfig {
		if errors.Is(ctx.Err(), context.Canceled) {
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		}
		if !sourceConfig.Enabled {
			continue
		}
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		isBroken, err := sourceLooksBroken(checkCtx, conn, brokenAfterRuns, sourceConfig.Name)
		if err != nil {
			logger.Error("failed to check if source is broken", "source", sourceConfig.Name, "error", err)
			continue
		}
		if isBroken {
			logger.Warn("source is broken", "source", sourceConfig.Name, "after", brokenAfterRuns)
		}
	}

	if !success {
		finalErr := errors.Join(runErrs...)
		if finalErr == nil {
			return errors.New("no source succeeded: no source is enabled")
		}
		return fmt.Errorf("no source succeeded: %w", finalErr)
	}
	return nil
}

// processSource handles a single source end to end and records the attempt in the
// runs table. It opens the run row first (closed via a deferred closeRun over the
// named return) so even a failure before fetching leaves a trace, then resolves the
// source id, fetches its items, and stores them. A panic inside becomes the error.
func processSource(ctx context.Context, conn *pgx.Conn, logger *slog.Logger, source Source, sourceConfig sources.SourceConfig, timeout time.Duration) (err error) {
	var (
		fetched     int
		failed      int
		sourceID    int
		sourceIDPtr *int
	)

	runCtx, cancelRun := context.WithTimeout(ctx, 10*time.Second)
	defer cancelRun()
	runID, err := openRun(runCtx, conn)
	if err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}

	defer func() {
		closeErr := closeRun(conn, fetched, failed, runID, sourceIDPtr, err)
		if closeErr != nil {
			logger.Error("failed to close run", "error", closeErr)
		}
	}()

	// Registered after the closeRun defer, so it runs first (LIFO) and fills err
	// before closeRun reads it.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	collectionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sourceID, err = getSourceID(collectionCtx, conn, sourceConfig.Name)
	if err != nil {
		return fmt.Errorf("failed to get source id: %w", err)
	}
	sourceIDPtr = &sourceID

	items, err := source.Fetch(collectionCtx, sourceConfig.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
	}

	fetched = len(items)
	failed = collectAndStoreData(collectionCtx, conn, logger, items, source, sourceID)

	// Checked before dividing: an empty feed is a success, not 0/0.
	if fetched != 0 {
		failedRatio := float64(failed) / float64(fetched)
		if failedRatio > failureThreshold {
			return fmt.Errorf("too many items failed: %d of %d (%.2f%%) (threshold %.2f%%)",
				failed, fetched, failedRatio*100, failureThreshold*100)
		}
	}
	logger.Info("source collected", "source", sourceConfig.Name, "fetched", fetched, "failed", failed)
	return nil
}

// collectAndStoreData stores each item's raw payload in raw_jobs and, when the
// payload is new, maps it to the common model and upserts it into jobs. A payload
// already in raw_jobs is skipped and is not a failure. Every item stands alone: a bad
// one is logged and counted, and the rest carry on. Returns how many items failed.
func collectAndStoreData(ctx context.Context, conn *pgx.Conn, logger *slog.Logger, items []json.RawMessage, source Source, sourceID int) int {
	failed := 0
	for _, item := range items {
		externalID, err := source.ExternalID(item)
		if err != nil {
			logger.Warn("failed to get external id", "error", err)
			failed++
			continue
		}
		hash, err := contentHash(item)
		if err != nil {
			logger.Warn("failed to hash payload", "external_id", externalID, "error", err)
			failed++
			continue
		}
		rawJobID, existed, err := insertRawJob(ctx, conn, logger, item, sourceID, externalID, hash)
		if existed {
			continue
		}
		if err != nil {
			// insertRawJob already logged the detail.
			failed++
			continue
		}

		job, err := source.ToJob(item)
		if err != nil {
			logger.Warn("failed to map item to job", "external_id", externalID, "error", err)
			failed++
			continue
		}
		jobID, err := insertJob(ctx, conn, rawJobID, sourceID, externalID, job)
		if err != nil {
			logger.Warn("failed to insert job", "external_id", externalID, "error", err)
			failed++
			continue
		}
		err = updateDuplicate(ctx, conn, jobID, sourceID, job)
		if err != nil {
			logger.Warn("failed to update jobs duplicate_of field", "external_id", externalID, "error", err)
			failed++
			continue
		}
	}
	return failed
}
