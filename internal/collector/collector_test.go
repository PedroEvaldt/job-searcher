package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/config"
	"github.com/PedroEvaldt/job-searcher/internal/sources"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	rjCount     = 137
	jCount      = 137
	sourceCount = 3

	// testInitialWait keeps retry waits short so a source answering 5xx does not slow the suite.
	testInitialWait = 10 * time.Millisecond
	// testTimeout is the per-source budget run gives each source in the tests.
	testTimeout = 60 * time.Second
	testRetries = 3
)

var (
	db  *sql.DB
	dsn string
)

func TestMain(m *testing.M) {
	var (
		err      error
		teardown func() error
	)
	db, teardown, err = setupPostgres()
	if err != nil {
		if teardown != nil {
			err := teardown()
			if err != nil {
				log.Fatalf("failed to teardown: %v", err)
			}
		}
		log.Fatal(err)
	}
	code := m.Run()
	err = teardown()
	if err != nil {
		log.Fatalf("failed to teardown: %v", err)
	}
	os.Exit(code)
}

func TestSources(t *testing.T) {
	expected := sourceCount
	err := pingDB(db)
	if err != nil {
		t.Fatalf("database not ready for testing sources: %v", err)
	}
	var sourcesCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM sources;").Scan(&sourcesCount); err != nil {
		t.Fatalf("failed to query sources count: %v", err)
	}
	if sourcesCount != expected {
		t.Errorf("expected %d sources; got %d", expected, sourcesCount)
	}
}

func TestMigrations(t *testing.T) {
	expected := []string{
		"sources",
		"raw_jobs",
		"jobs",
		"runs",
		"labels",
		"scores",
		"job_skills",
		"job_embeddings",
		"jev_answers",
		"jobs_sent",
	}
	err := pingDB(db)
	if err != nil {
		t.Fatalf("database not ready for testing migrations: %v", err)
	}
	rows, err := db.Query(`
	SELECT table_name 
	FROM information_schema.tables 
	WHERE table_schema = 'public' 
	AND table_name != 'goose_db_version';
	`)
	if err != nil {
		t.Fatalf("failed to query tables names: %v", err)
	}
	defer func() { _ = rows.Close() }()

	actual := map[string]bool{}
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			t.Fatalf("failed to scan table row: %v", err)
		}
		actual[tableName] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to iterate over rows: %v", err)
	}
	for _, expectedTableName := range expected {
		if !actual[expectedTableName] {
			t.Errorf("table %s does not exist", expectedTableName)
		}
		delete(actual, expectedTableName)
	}
	for remaining := range actual {
		t.Errorf("unexpected table %s", remaining)
	}
}

func TestFetch(t *testing.T) {
	err := pingDB(db)
	if err != nil {
		t.Fatalf("database not ready for testing fetching: %v", err)
	}
	cfg := config.Config{
		Sources: []sources.SourceConfig{
			{
				Name:        "remoteok",
				Enabled:     true,
				Timeout:     testTimeout,
				Retries:     testRetries,
				InitialWait: testInitialWait,
			},
			{
				Name:        "remotive",
				Enabled:     true,
				Timeout:     testTimeout,
				Retries:     testRetries,
				InitialWait: testInitialWait,
			},
			{
				Name:        "himalayas",
				Enabled:     true,
				Timeout:     testTimeout,
				Retries:     testRetries,
				InitialWait: testInitialWait,
			},
		},
	}
	for i, source := range cfg.Sources {
		body, err := os.ReadFile(fmt.Sprintf("testdata/%s.json", source.Name))
		if err != nil {
			t.Fatalf("failed to read json file: %v", err)
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(body); err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		}))
		t.Cleanup(func() { srv.Close() })
		cfg.Sources[i].URL = srv.URL
	}

	_, err = db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}

	err = Run(context.Background(), slog.New(slog.DiscardHandler), dsn, &cfg)
	if err != nil {
		t.Errorf("expected error nil, got %v", err)
		return
	}
	var rawJobsCount int
	err = db.QueryRow("SELECT COUNT(*) FROM raw_jobs;").Scan(&rawJobsCount)
	if err != nil {
		t.Fatalf("failed to query raw_jobs rows: %v", err)
	}
	var jobsCount int
	err = db.QueryRow("SELECT COUNT(*) FROM jobs;").Scan(&jobsCount)
	if err != nil {
		t.Fatalf("failed to query jobs rows: %v", err)
	}
	if rawJobsCount != rjCount {
		t.Errorf("expected %d; got %d", rjCount, rawJobsCount)
		return
	}
	if jobsCount != jCount {
		t.Errorf("expected %d; got %d", jCount, jobsCount)
		return
	}

	err = Run(context.Background(), slog.New(slog.DiscardHandler), dsn, &cfg)
	if err != nil {
		t.Errorf("expected error nil, got %v", err)
		return
	}
	var rawJobsSecondRun int
	err = db.QueryRow("SELECT COUNT(*) FROM raw_jobs;").Scan(&rawJobsSecondRun)
	if err != nil {
		t.Fatalf("failed to query raw_jobs rows: %v", err)
	}
	var jobsSecondRun int
	err = db.QueryRow("SELECT COUNT(*) FROM jobs;").Scan(&jobsSecondRun)
	if err != nil {
		t.Fatalf("failed to query jobs rows: %v", err)
	}

	if rawJobsCount != rawJobsSecondRun {
		t.Errorf("expected %d; got %d", rawJobsCount, rawJobsSecondRun)
		return
	}
	if jobsCount != jobsSecondRun {
		t.Errorf("expected %d; got %d", jobsCount, jobsSecondRun)
		return
	}

	var runsCount int
	err = db.QueryRow("SELECT COUNT(*) FROM runs;").Scan(&runsCount)
	if err != nil {
		t.Fatalf("failed to query runs rows: %v", err)
	}
	if runsCount != (2 * sourceCount) {
		t.Errorf("expected %d; got %d", (2 * sourceCount), runsCount)
		return
	}

	var runsErrorsCount int
	err = db.QueryRow("SELECT COUNT(*) FROM runs WHERE error IS NOT NULL;").Scan(&runsErrorsCount)
	if err != nil {
		t.Fatalf("failed to query runs errors rows: %v", err)
	}
	if runsErrorsCount != 0 {
		t.Errorf("expected 0; got %d", runsErrorsCount)
		return
	}
}

func TestPartialFailure(t *testing.T) {
	err := pingDB(db)
	if err != nil {
		t.Fatalf("database not ready for testing partial failure: %v", err)
	}

	body, err := os.ReadFile("testdata/remotive.json")
	if err != nil {
		t.Fatalf("failed to read json file: %v", err)
	}
	goodSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(body); err != nil {
			t.Errorf("failed to write body: %v", err)
		}
	}))
	defer goodSrv.Close()

	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badSrv.Close()

	cfg := config.Config{
		Sources: []sources.SourceConfig{
			{
				Name:        "remoteok",
				Enabled:     true,
				URL:         badSrv.URL,
				Timeout:     testTimeout,
				Retries:     testRetries,
				InitialWait: testInitialWait,
			},
			{
				Name:        "remotive",
				Enabled:     true,
				URL:         goodSrv.URL,
				Timeout:     testTimeout,
				Retries:     testRetries,
				InitialWait: testInitialWait,
			},
		},
	}
	_, err = db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}
	err = Run(context.Background(), slog.New(slog.DiscardHandler), dsn, &cfg)
	if err != nil {
		t.Fatalf("expected err nil; got %v", err)
	}

	// One run per source, and only the failing source records an error.
	var runsCount int
	err = db.QueryRow("SELECT COUNT(*) FROM runs;").Scan(&runsCount)
	if err != nil {
		t.Fatalf("failed to query runs rows: %v", err)
	}

	var runsErrCount int
	err = db.QueryRow("SELECT COUNT(*) FROM runs WHERE error IS NOT NULL;").Scan(&runsErrCount)
	if err != nil {
		t.Fatalf("failed to query runs rows: %v", err)
	}

	if runsCount != 2 {
		t.Errorf("expected 2 runs; got %d", runsCount)
	}
	if runsErrCount != 1 {
		t.Errorf("expected 1 error in runs; got %d", runsErrCount)
	}
}

func TestClientTimeout(t *testing.T) {
	err := pingDB(db)
	if err != nil {
		t.Fatalf("database not ready for testing client timeout: %v", err)
	}
	// Outlasts the source budget, but returns as soon as the client gives up.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Config{
		Sources: []sources.SourceConfig{
			{
				Name:    "remoteok",
				Enabled: true,
				URL:     srv.URL,
				// 100ms budget against a 500ms handler: the source budget must fire first.
				Timeout:     100 * time.Millisecond,
				Retries:     testRetries,
				InitialWait: testInitialWait,
			},
		},
	}

	// The container is shared with TestFetch, so start from empty tables.
	_, err = db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}

	err = Run(context.Background(), slog.New(slog.DiscardHandler), dsn, &cfg)

	switch {
	case err == nil:
		t.Fatalf("expected error got nil")
	case !errors.Is(err, context.DeadlineExceeded):
		t.Fatalf("expected DeadlineExceeded; got %v", err)
	}

	var (
		runsCount int
		fetched   int
		runErr    string
	)
	err = db.QueryRow("SELECT fetched, error, COUNT(*) OVER() FROM runs;").Scan(&fetched, &runErr, &runsCount)
	if err != nil {
		t.Fatalf("failed to query runs: %v", err)
	}

	if runsCount != 1 {
		t.Errorf("expected 1 run; got %d", runsCount)
	}

	if fetched != 0 {
		t.Errorf("expected 0 fetched; got %d", fetched)
	}

	if !strings.Contains(runErr, "context deadline exceeded") {
		t.Errorf("expected error to contain 'context deadline exceeded'; got %s", runErr)
	}
}

func TestFailureThreshold(t *testing.T) {
	tests := []struct {
		name            string
		body            string
		wantErr         bool
		fetchedExpected int
		failedExpected  int
	}{
		{"all items invalid", `{"jobs":[{"id":"x"},{"id":"y"}]}`, true, 2, 2},
		{"one in four invalid", `{"jobs":[{"id":1},{"id":2},{"id":3},{"id":"x"}]}`, false, 4, 1},
		{"empty feed", `{"jobs":[]}`, false, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := w.Write([]byte(tt.body))
				if err != nil {
					t.Errorf("failed writing body: %v", err)
				}
			}))
			defer srv.Close()
			cfg := config.Config{
				Sources: []sources.SourceConfig{
					{
						Name:        "remotive",
						Enabled:     true,
						URL:         srv.URL,
						Timeout:     testTimeout,
						Retries:     testRetries,
						InitialWait: testInitialWait,
					},
				},
			}

			_, err := db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
			if err != nil {
				t.Fatalf("failed to clean tables: %v", err)
			}

			err = Run(context.Background(), slog.New(slog.DiscardHandler), dsn, &cfg)

			var (
				runsErrCount int
				fetched      int
				failed       int
			)
			queryErr := db.QueryRow("SELECT fetched, failed FROM runs;").Scan(&fetched, &failed)
			if queryErr != nil {
				t.Fatalf("failed to query runs: %v", queryErr)
			}
			queryErr = db.QueryRow("SELECT COUNT(*) FROM runs WHERE error IS NOT NULL;").Scan(&runsErrCount)
			if queryErr != nil {
				t.Fatalf("failed to query runs: %v", queryErr)
			}

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected err too many items failed; got nil")
				}
				if !strings.Contains(err.Error(), "too many items failed") {
					t.Errorf("expected err too many items failed; got %v", err)
				}
				if runsErrCount != 1 {
					t.Errorf("expected 1 run with error; got %d", runsErrCount)
				}
			} else {
				if err != nil {
					t.Errorf("expected err nil; got %v", err)
				}
				if runsErrCount > 0 {
					t.Errorf("expected 0 runs error; got %d", runsErrCount)
				}
			}

			if fetched != tt.fetchedExpected {
				t.Errorf("expected %d; got %d", tt.fetchedExpected, fetched)
			}
			if failed != tt.failedExpected {
				t.Errorf("expected %d; got %d", tt.failedExpected, failed)
			}
		})
	}
}

// dummy is a Source whose Fetch panics, to prove a panic is recorded as a failed run.
type dummy struct{}

func (d dummy) Fetch(context.Context, string) ([]json.RawMessage, error) {
	panic("dummy panic")
}

func (d dummy) ExternalID(json.RawMessage) (string, error) {
	return "", nil
}

func (d dummy) ToJob(json.RawMessage) (sources.Job, error) {
	return sources.Job{}, nil
}

func TestPanic(t *testing.T) {
	_, err := db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}
	conn := newConn(t)
	cfg := sources.SourceConfig{
		Name:    "remotive",
		Enabled: true,
		URL:     "https://test.url",
	}
	err = processSource(context.Background(), conn, slog.New(slog.DiscardHandler), dummy{}, cfg, testTimeout)
	if err == nil {
		t.Fatalf("expected err; got nil")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("expected error to contain panic; got %v", err)
	}

	var (
		runsErr           string
		finishedAtNotNull bool
	)
	err = db.QueryRow("SELECT error, finished_at IS NOT NULL FROM runs;").Scan(&runsErr, &finishedAtNotNull)
	if err != nil {
		t.Fatalf("failed to query runs: %v", err)
	}
	if !strings.Contains(runsErr, "panic") {
		t.Errorf("expected runs.error to contain panic; got %v", runsErr)
	}
	if !finishedAtNotNull {
		t.Errorf("expected finished_at NOT NULL true; got %v", finishedAtNotNull)
	}
}

func newConn(t *testing.T) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatalf("failed opening connection: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func pingDB(db *sql.DB) error {
	return db.Ping()
}

func setupPostgres() (*sql.DB, func() error, error) {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "pgvector/pgvector:pg16",
		postgres.WithDatabase("jobs_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start postgres: %w", err)
	}
	teardown := func() error {
		err := testcontainers.TerminateContainer(ctr)
		if err != nil {
			return fmt.Errorf("failed to clean container: %w", err)
		}
		return nil
	}

	dsn, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, teardown, fmt.Errorf("failed to get dsn string: %w", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, teardown, fmt.Errorf("failed to open database: %w", err)
	}
	teardown = func() error {
		var errs []error
		errClose := db.Close()
		if errClose != nil {
			errs = append(errs, fmt.Errorf("failed to close db: %w", errClose))
		}
		errTerminate := testcontainers.TerminateContainer(ctr)
		if errTerminate != nil {
			errs = append(errs, fmt.Errorf("failed to terminate container: %w", errTerminate))
		}
		return errors.Join(errs...)
	}

	err = db.PingContext(ctx)
	if err != nil {
		return nil, teardown, fmt.Errorf("failed to ping db: %w", err)
	}

	// The same code path the collector runs at start-up, against the embedded files.
	if err := applyMigrations(ctx, db, slog.New(slog.DiscardHandler)); err != nil {
		return nil, teardown, fmt.Errorf("failed to migrate: %w", err)
	}
	return db, teardown, nil
}

func TestCollector(t *testing.T) {
	tests := []struct {
		name        string
		interval    time.Duration
		toCancel    int
		wantedCalls int
		alwaysFail  bool
	}{
		{"runs until cancelled", 10 * time.Millisecond, 3, 3, false},
		{"keeps going after failed cycles", 10 * time.Millisecond, 3, 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			counter := 0
			cycle := func() error {
				counter++
				// Cancel before failing, so a failing cycle still ends the loop.
				if counter == tt.toCancel {
					cancel()
				}
				if tt.alwaysFail {
					return errors.New("bidibi bodobo")
				}
				return nil
			}
			// No retries: a failed cycle waits for the next tick, as before them.
			loop(ctx, slog.New(slog.DiscardHandler), tt.interval, 0, 0, cycle)

			if counter != tt.wantedCalls {
				t.Errorf("expected %d; got %d", tt.wantedCalls, counter)
			}
		})
	}
}

func TestFirstCycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	counter := 0
	cycle := func() error {
		counter++
		cancel()
		return nil
	}
	begin := time.Now()
	// Interval and retry wait both 1s: the first call must wait for neither.
	loop(ctx, slog.New(slog.DiscardHandler), time.Second, 3, time.Second, cycle)
	after := time.Since(begin)
	if after > 500*time.Millisecond {
		t.Errorf("expected time < 500ms; got %v", after)
	}
	if counter != 1 {
		t.Errorf("expected 1 cycle; got %d", counter)
	}
}

// TestCollectorRetry checks the retries after a cycle that fails whole. The tick is an
// hour away, so every call after the first is a retry, and each case ends when ctx
// times out with the collector waiting: for the tick, or in the middle of a retry wait.
func TestCollectorRetry(t *testing.T) {
	const retries = 3
	tests := []struct {
		name      string
		fails     int // how many calls fail before one succeeds
		retryWait time.Duration
		wantCalls int
	}{
		{"recovers after two failures", 2, 10 * time.Millisecond, 3},
		{"gives up after the retries", 100, 10 * time.Millisecond, 1 + retries},
		{"cancelled while waiting to retry", 100, time.Hour, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			calls := 0
			cycle := func() error {
				calls++
				if calls <= tt.fails {
					return errors.New("network is down")
				}
				return nil
			}
			begin := time.Now()
			loop(ctx, slog.New(slog.DiscardHandler), time.Hour, retries, tt.retryWait, cycle)

			if calls != tt.wantCalls {
				t.Errorf("expected %d calls; got %d", tt.wantCalls, calls)
			}
			if elapsed := time.Since(begin); elapsed > time.Second {
				t.Errorf("expected the collector to return soon after ctx timed out; got %v", elapsed)
			}
		})
	}
}

func TestCtxCancel(t *testing.T) {
	err := pingDB(db)
	if err != nil {
		t.Fatalf("database not ready for testing cancellation: %v", err)
	}
	// Never answers on its own: it returns only once the request is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Config{
		Sources: []sources.SourceConfig{
			{
				Name:    "remoteok",
				Enabled: true,
				URL:     srv.URL,
				// A 2s budget against a cancel at 100ms: the cancel must fire first.
				Timeout:     2 * time.Second,
				Retries:     testRetries,
				InitialWait: testInitialWait,
			},
			{
				Name:    "remotive",
				Enabled: true,
				URL:     srv.URL,
				// A 2s budget against a cancel at 100ms: the cancel must fire first.
				Timeout:     2 * time.Second,
				Retries:     testRetries,
				InitialWait: testInitialWait,
			},
		},
	}
	// The container is shared with TestFetch, so start from empty tables.
	_, err = db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	time.AfterFunc(100*time.Millisecond, cancel)
	begin := time.Now()
	err = Run(ctx, slog.New(slog.DiscardHandler), dsn, &cfg)
	after := time.Since(begin)
	if err == nil {
		t.Fatalf("expected err; got nil")
	}
	if after > time.Second {
		t.Errorf("expected duration less than 1s; got %v", after)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context canceled; got %v", err)
	}
	// Cancelled during remoteok, so the loop must stop before remotive is tried.
	if strings.Contains(err.Error(), "remotive") {
		t.Errorf("expected run to stop before remotive; got %v", err)
	}

	var (
		runsCount int
		fetched   int
		runErr    string
		finished  bool
	)
	err = db.QueryRow("SELECT fetched, error, finished_at IS NOT NULL, COUNT(*) OVER() FROM runs;").Scan(&fetched, &runErr, &finished, &runsCount)
	if err != nil {
		t.Fatalf("failed to query runs: %v", err)
	}

	if !finished {
		t.Errorf("expected finished_at to be set")
	}

	if runsCount != 1 {
		t.Errorf("expected 1 run; got %d", runsCount)
	}

	if fetched != 0 {
		t.Errorf("expected 0 fetched; got %d", fetched)
	}

	if !strings.Contains(runErr, "context canceled") {
		t.Errorf("expected error to contain 'context canceled'; got %s", runErr)
	}
}

func TestWaitForDB(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
		timeout time.Duration
	}{
		{"database up", dsn, false, 5 * time.Second},
		{"nobody listening", "postgres://postgres:postgres@localhost:1/postgres?sslmode=disable", true, 300 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()
			err := WaitForDB(ctx, slog.New(slog.DiscardHandler), tt.dsn, testInitialWait)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error got nil")
				}
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("expected error deadline exceeded; got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("failed to wait for db: %v", err)
				}
			}
		})
	}
}

func TestSourceLooksBroken(t *testing.T) {
	err := pingDB(db)
	if err != nil {
		t.Fatalf("database not ready for testing source health: %v", err)
	}
	ctx := context.Background()
	conn, err := openConn(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to open conn: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	// An empty runErr is stored as NULL, like a run that finished without error.
	type pastRun struct {
		fetched int
		runErr  string
	}
	const k = 2
	tests := []struct {
		name string
		runs []pastRun // oldest first: the newest run has the highest id
		want bool
	}{
		// failureThreshold can close a run with an error and items fetched.
		{"last k failed", []pastRun{{100, "too many items failed"}, {0, ""}}, true},
		{"newest ok", []pastRun{{0, "boom"}, {0, "boom"}, {100, ""}}, false},
		{"oldest ok", []pastRun{{100, ""}, {0, "boom"}, {0, ""}}, true},
		{"fewer than k runs", []pastRun{{0, "boom"}}, false},
		{"no runs", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := db.Exec("TRUNCATE runs;")
			if err != nil {
				t.Fatalf("failed to clean runs: %v", err)
			}
			for _, r := range tt.runs {
				_, err := db.Exec(`
					INSERT INTO runs (source_id, started_at, finished_at, fetched, error)
					VALUES ((SELECT id FROM sources WHERE name = 'remoteok'), now(), now(), $1, NULLIF($2, ''));`,
					r.fetched, r.runErr)
				if err != nil {
					t.Fatalf("failed to insert run: %v", err)
				}
			}

			got, err := sourceLooksBroken(ctx, conn, k, "remoteok")
			if err != nil {
				t.Fatalf("failed to check source: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected broken = %v; got %v", tt.want, got)
			}
		})
	}
}

// jobSource is a Source whose items are sources.Job values encoded as JSON, so a test
// can feed collectAndStoreData any job it wants. Fetch is never called there.
type jobSource struct{}

func (jobSource) Fetch(context.Context, string) ([]json.RawMessage, error) {
	return nil, nil
}

func (s jobSource) ExternalID(item json.RawMessage) (string, error) {
	job, err := s.ToJob(item)
	return job.ExternalID, err
}

func (jobSource) ToJob(item json.RawMessage) (sources.Job, error) {
	var job sources.Job
	err := json.Unmarshal(item, &job)
	return job, err
}

// jobItem encodes one job in the format jobSource reads.
func jobItem(t *testing.T, externalID, company, title, description string) json.RawMessage {
	t.Helper()
	item, err := json.Marshal(sources.Job{ExternalID: externalID, Company: company, Title: title, Description: description})
	if err != nil {
		t.Fatalf("failed to encode job %s: %v", externalID, err)
	}
	return item
}

// writeDuplicateScenario empties the tables and writes the real duplicate cases (see dedup_test.go)
// through collectAndStoreData, in order: the himalayas jobs first, then remoteok.
func writeDuplicateScenario(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	_, err := db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}
	ctx := t.Context()
	logger := slog.New(slog.DiscardHandler)

	himalayasID, err := getSourceID(ctx, conn, "himalayas")
	if err != nil {
		t.Fatalf("failed to get himalayas id: %v", err)
	}
	remoteokID, err := getSourceID(ctx, conn, "remoteok")
	if err != nil {
		t.Fatalf("failed to get remoteok id: %v", err)
	}

	const (
		editor = "Social Media Video Editor & Designer"
		sdr    = "Junior Sales Development Representative"
	)
	himalayas := []json.RawMessage{
		jobItem(t, "h1", "CapsLock", editor, "Edit short videos."),
		jobItem(t, "h2", "CapsLock", editor, "Edit short videos."),
		jobItem(t, "h3", "CapsLock", editor, "Edit short videos."),
		jobItem(t, "h4", "Bjak", "Engineering Manager", "Hybrid role, 3 days in the office."),
		jobItem(t, "h5", "Bjak", "Engineering Manager", "Remote role, we hire globally."),
		jobItem(t, "h6", "Huzzle", sdr+" - IT Industry", "<p>Sell IT services.</p>"),
		jobItem(t, "h7", "Gate", "内容风控专家", "Content risk."),
	}
	remoteok := []json.RawMessage{
		jobItem(t, "r1", "Huzzle", sdr+" IT Industry", "Sell IT services."),
		jobItem(t, "r2", "Bjak", "Engineering Manager", "Bjak engineering manager."),
		jobItem(t, "r3", "Gate", "测试负责人", "Test lead."),
	}
	if failed := collectAndStoreData(ctx, conn, logger, himalayas, jobSource{}, himalayasID); failed != 0 {
		t.Fatalf("expected 0 failed himalayas items; got %d", failed)
	}
	if failed := collectAndStoreData(ctx, conn, logger, remoteok, jobSource{}, remoteokID); failed != 0 {
		t.Fatalf("expected 0 failed remoteok items; got %d", failed)
	}
}

// checkDuplicateMarks reads where each job of the scenario points and compares it with
// the expected marks: copies point at the oldest live match, never at a copy, and jobs
// the rule keeps apart stay live.
func checkDuplicateMarks(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	// Each job's external id, and the external id of the job it points at ("" = live).
	rows, err := conn.Query(t.Context(), `
		SELECT j.external_id, COALESCE(o.external_id, '')
		FROM jobs j
		LEFT JOIN jobs o ON o.id = j.duplicate_of;`)
	if err != nil {
		t.Fatalf("failed to query jobs: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var job, original string
		if err := rows.Scan(&job, &original); err != nil {
			t.Fatalf("failed to scan job: %v", err)
		}
		got[job] = original
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to read jobs: %v", err)
	}

	want := map[string]string{
		"h1": "",
		"h2": "h1",
		"h3": "h1", // not h2: a copy is never a candidate, so copies do not chain
		"h4": "",
		"h5": "", // same source, different description: kept apart from h4
		"h6": "",
		"h7": "",
		"r1": "h6",
		"r2": "h4", // matches h4 and h5 (other source); the oldest wins
		"r3": "",   // title normalizes to "": never a duplicate
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d jobs; got %d", len(want), len(got))
	}
	for job, original := range want {
		if got[job] != original {
			t.Errorf("%s: expected duplicate_of %q; got %q", job, original, got[job])
		}
	}
}

// TestMarkDuplicates checks the marks set as each job is written.
func TestMarkDuplicates(t *testing.T) {
	conn := newConn(t)
	writeDuplicateScenario(t, conn)
	checkDuplicateMarks(t, conn)
}

func TestNilPostedAt(t *testing.T) {
	_, err := db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}
	conn := newConn(t)
	ctx := t.Context()
	logger := slog.New(slog.DiscardHandler)

	himalayasID, err := getSourceID(ctx, conn, "himalayas")
	if err != nil {
		t.Fatalf("failed to get himalayas id: %v", err)
	}

	tests := []struct {
		name       string
		externalID string
		date       time.Time
	}{
		{"normal date", "d1", time.Now()},
		{"zero date", "d2", time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := json.Marshal(sources.Job{
				ExternalID:  tt.externalID,
				Company:     "CapsLock",
				Title:       "Social Media Video Editor & Designer",
				Description: "Edit short videos.",
				Date:        tt.date,
			})
			if err != nil {
				t.Fatalf("failed to encode job: %v", err)
			}
			items := []json.RawMessage{item}
			if failed := collectAndStoreData(ctx, conn, logger, items, jobSource{}, himalayasID); failed != 0 {
				t.Fatalf("expected 0 failed items; got %d", failed)
			}

			var postedAt *time.Time
			err = conn.QueryRow(ctx,
				"SELECT posted_at FROM jobs WHERE external_id = $1", tt.externalID,
			).Scan(&postedAt)
			if err != nil {
				t.Fatalf("failed to query posted_at: %v", err)
			}

			if tt.date.IsZero() && postedAt != nil {
				t.Errorf("zero date: expected NULL posted_at; got %v", *postedAt)
			}
			if !tt.date.IsZero() && postedAt == nil {
				t.Errorf("real date: expected posted_at set; got NULL")
			}
		})
	}
}

// TestBackfillDedupe turns the scenario into jobs written before dedupe keys existed
// and checks that the backfill rebuilds the same marks, then finds nothing left to do.
func TestBackfillDedupe(t *testing.T) {
	ctx := t.Context()
	conn := newConn(t)
	writeDuplicateScenario(t, conn)

	// Marks are cleared too, so only the backfill can put them back.
	_, err := conn.Exec(ctx, "UPDATE jobs SET company_key = NULL, title_key = NULL, duplicate_of = NULL;")
	if err != nil {
		t.Fatalf("failed to clear keys and marks: %v", err)
	}

	// 3 per batch: the 10 jobs take four batches. h2 and h3 find h1 inside their own
	// batch; r1 and r2 find h6 and h4 in an earlier one.
	const batchSize = 3
	backfilled, err := backfillDedupe(ctx, conn, batchSize)
	if err != nil {
		t.Fatalf("failed to backfill: %v", err)
	}
	if backfilled != 10 {
		t.Errorf("expected 10 backfilled jobs; got %d", backfilled)
	}
	checkDuplicateMarks(t, conn)

	backfilled, err = backfillDedupe(ctx, conn, batchSize)
	if err != nil {
		t.Fatalf("failed to backfill a second time: %v", err)
	}
	if backfilled != 0 {
		t.Errorf("expected 0 backfilled jobs on the second pass; got %d", backfilled)
	}
}

func TestNormalizeRawJobs(t *testing.T) {
	_, err := db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}
	conn := newConn(t)
	ctx := t.Context()
	logger := slog.New(slog.DiscardHandler)

	// normalizeRawJobs re-parses each payload with the real source (newSource), so the
	// raw payloads must be in that source's format. Remote OK reads an RFC3339 "date";
	// an absent date is a zero time.Time, which becomes NULL.
	remoteokID, err := getSourceID(ctx, conn, "remoteok")
	if err != nil {
		t.Fatalf("failed to get remoteok id: %v", err)
	}
	src, err := newSource("remoteok", nil)
	if err != nil {
		t.Fatalf("failed to build remoteok source: %v", err)
	}

	tests := []struct {
		name       string
		externalID string
		hasDate    bool
	}{
		{"with date", "d1", true},
		{"without date", "d2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]any{
				"id":          tt.externalID,
				"position":    "Social Media Video Editor",
				"company":     "CapsLock",
				"url":         "https://example.com/" + tt.externalID,
				"description": "Edit short videos.",
			}
			if tt.hasDate {
				payload["date"] = "2024-01-02T03:04:05Z"
			}
			item, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("failed to encode item: %v", err)
			}
			if failed := collectAndStoreData(ctx, conn, logger, []json.RawMessage{item}, src, remoteokID); failed != 0 {
				t.Fatalf("expected 0 failed items; got %d", failed)
			}

			// Simulate a job stored before posted_at was extracted, and already scored.
			_, err = db.Exec("UPDATE jobs SET posted_at = NULL, scored_at = now() WHERE external_id = $1;", tt.externalID)
			if err != nil {
				t.Fatalf("failed to reset job: %v", err)
			}

			if _, err := normalizeRawJobs(ctx, conn, 5); err != nil {
				t.Fatalf("failed to normalize jobs: %v", err)
			}

			var (
				nullPostedAt bool
				nullScoredAt bool
			)
			err = db.QueryRow("SELECT posted_at IS NULL, scored_at IS NULL FROM jobs WHERE external_id = $1;", tt.externalID).Scan(&nullPostedAt, &nullScoredAt)
			if err != nil {
				t.Fatalf("failed to query job: %v", err)
			}
			if tt.hasDate && nullPostedAt {
				t.Errorf("expected posted_at set; got NULL")
			}
			if !tt.hasDate && !nullPostedAt {
				t.Errorf("expected posted_at NULL; got a time")
			}
			if nullScoredAt {
				t.Errorf("expected scored_at preserved; got NULL")
			}
		})
	}
}

func TestNilSkillsExtractedAt(t *testing.T) {
	_, err := db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}
	conn := newConn(t)
	ctx := t.Context()
	logger := slog.New(slog.DiscardHandler)

	himalayasID, err := getSourceID(ctx, conn, "himalayas")
	if err != nil {
		t.Fatalf("failed to get himalayas id: %v", err)
	}

	item1, err := json.Marshal(sources.Job{
		ExternalID:  "d1",
		Company:     "CapsLock",
		Title:       "Social Media Video Editor & Designer",
		Description: "first description",
	})
	if err != nil {
		t.Fatalf("failed to encode job: %v", err)
	}
	items := []json.RawMessage{item1}
	if failed := collectAndStoreData(ctx, conn, logger, items, jobSource{}, himalayasID); failed != 0 {
		t.Fatalf("expected 0 failed items; got %d", failed)
	}
	_, err = conn.Exec(ctx, "UPDATE jobs SET skills_extracted_at = now() WHERE external_id = 'd1'")
	if err != nil {
		t.Fatalf("failed to simulad analyzer: %v", err)
	}
	item2, err := json.Marshal(sources.Job{
		ExternalID:  "d1",
		Company:     "CapsLock",
		Title:       "Social Media Video Editor & Designer",
		Description: "second description",
	})
	if err != nil {
		t.Fatalf("failed to encode job: %v", err)
	}
	items = []json.RawMessage{item2}
	if failed := collectAndStoreData(ctx, conn, logger, items, jobSource{}, himalayasID); failed != 0 {
		t.Fatalf("expected 0 failed items; got %d", failed)
	}

	var got string
	err = conn.QueryRow(ctx, "SELECT external_id FROM jobs WHERE skills_extracted_at IS NULL;").Scan(&got)
	if err != nil {
		t.Fatalf("failed to extract from skills_extracted_at NULL: %v", err)
	}
	if got != "d1" {
		t.Errorf("expected d1 as external_id; got %v", got)
	}
}
