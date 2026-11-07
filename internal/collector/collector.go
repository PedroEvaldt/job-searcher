// Package collector pulls jobs from every enabled source into Postgres: the raw
// payload into raw_jobs, the normalized job into jobs, with duplicates across sources
// marked as they arrive. cmd/collector is its only entry point.
package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/client"
	"github.com/PedroEvaldt/job-searcher/internal/config"
	"github.com/PedroEvaldt/job-searcher/internal/sources"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for database/sql (goose)
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
	// backfillTimeout bounds the backfill at the start of a run. The first pass after
	// the deploy covers every job written before dedupe keys existed; if it runs out
	// of time, the next run picks up where it stopped.
	backfillTimeout = 5 * time.Minute
	// backfillBatchSize is how many jobs the backfill reads at a time. Descriptions
	// average about 5 KB, so a batch holds a few MB.
	backfillBatchSize = 1000
	// cycleRetries is how many more times the collector tries a cycle that failed whole
	// before it waits for the next tick. With cycleRetryWait it covers about 15 min,
	// enough for the router to come back after a power cut (the Pi boots first).
	cycleRetries = 3
	// cycleRetryWait is how long the collector waits before each of those tries.
	cycleRetryWait = 5 * time.Minute
	// renormalizeBatchSize is how many raw_jobs Renormalize reads at a time.
	renormalizeBatchSize = 500
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

// WaitForDB blocks until Postgres accepts a connection, doubling the wait between
// attempts from initialDBWait. It returns ctx's error when ctx ends first.
func WaitForDB(ctx context.Context, logger *slog.Logger, dbURL string, initialDBWait time.Duration) error {
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
			_ = conn.Close(ctx) // only probing; Run opens its own connection
			return nil
		}
	}
}

// runWithRetries calls cycle and, while it fails, calls it again after retryWait, up to
// retries more times. It returns when a call succeeds, when the retries run out, or
// when ctx is cancelled.
func runWithRetries(ctx context.Context, logger *slog.Logger, cycle func() error, retries int, retryWait time.Duration) {
	for attempt := 1; ; attempt++ {
		err := cycle()
		// A cycle cut short by shutdown is not worth a warning or a retry.
		if err == nil || ctx.Err() != nil {
			return
		}
		if attempt > retries {
			logger.Error("failed to run, waiting for the next tick", "attempts", attempt, "error", err)
			return
		}
		logger.Warn("failed to run, trying again", "attempt", attempt, "retry_in", retryWait, "error", err)
		if err := client.Sleep(ctx, retryWait); err != nil {
			return
		}
	}
}

// Loop runs a collection pass right away and then every interval, until ctx is
// cancelled. A pass that fails whole is retried cycleRetries times, cycleRetryWait
// apart, before the loop goes back to waiting for the next tick.
func Loop(ctx context.Context, logger *slog.Logger, dbURL string, cfg *config.Config) {
	pass := func() error { return Run(ctx, logger, dbURL, cfg) }
	loop(ctx, logger, cfg.CollectEvery, cycleRetries, cycleRetryWait, pass)
}

// loop runs cycle right away, then again on every tick of interval, and returns
// when ctx is cancelled. A cycle that fails whole is tried again up to retries more
// times, retryWait apart, before the loop goes back to waiting for the tick: a cycle
// lost to a network that is still coming up costs minutes, not a whole interval.
func loop(ctx context.Context, logger *slog.Logger, interval time.Duration, retries int, retryWait time.Duration, cycle func() error) {
	runWithRetries(ctx, logger, cycle, retries, retryWait)

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			runWithRetries(ctx, logger, cycle, retries, retryWait)
		case <-ctx.Done():
			logger.Info("shutting down")
			return
		}
	}
}

// Run performs one full collection pass: open the Postgres connection, apply
// migrations, backfill dedupe keys for older jobs, process each enabled source in turn,
// then warn about sources that look broken. A failure in one source is logged and
// skipped, not fatal to the others; Run fails only when no source succeeds.
func Run(ctx context.Context, logger *slog.Logger, dbURL string, cfg *config.Config) (err error) {
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

	err = applyMigrations(dbCtx, db, logger)
	if err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	// Before the sources: a new job only finds its duplicates among jobs that
	// already have keys.
	backfillCtx, cancelBackfill := context.WithTimeout(ctx, backfillTimeout)
	backfillStart := time.Now()
	backfilled, err := backfillDedupe(backfillCtx, conn, backfillBatchSize)
	cancelBackfill()
	backfillDuration := time.Since(backfillStart).Round(time.Millisecond)
	if err != nil {
		logger.Error("failed to backfill dedupe keys", "backfilled", backfilled, "duration", backfillDuration, "error", err)
	} else if backfilled > 0 {
		logger.Info("backfilled dedupe keys and duplicate marks", "jobs", backfilled, "duration", backfillDuration)
	}

	var runErrs []error
	var success bool
	for _, sourceConfig := range cfg.Sources {
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

	for _, sourceConfig := range cfg.Sources {
		if errors.Is(ctx.Err(), context.Canceled) {
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		}
		if !sourceConfig.Enabled {
			continue
		}
		checkCtx, cancelCheck := context.WithTimeout(ctx, 10*time.Second)
		isBroken, err := sourceLooksBroken(checkCtx, conn, brokenAfterRuns, sourceConfig.Name)
		cancelCheck()
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

// Renormalize re-parses every stored raw payload with its source's current mapping and
// rewrites the matching jobs row, so a parser fix reaches old jobs without re-fetching
// anything. It returns how many jobs it rewrote.
func Renormalize(ctx context.Context, dbURL string) (n int, err error) {
	conn, err := openConn(ctx, dbURL)
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := conn.Close(ctx); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close connection: %w", closeErr)
		}
	}()
	return normalizeRawJobs(ctx, conn, renormalizeBatchSize)
}
