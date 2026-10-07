package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/sources"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
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
	}
	err := pingDB(db)
	if err != nil {
		t.Fatalf("database not ready for testing Migrations: %v", err)
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
	defer rows.Close()

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
	cfg := Config{
		SourcesConfig: []sources.SourceConfig{
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
	for i, source := range cfg.SourcesConfig {
		body, err := os.ReadFile(fmt.Sprintf("testdata/%s.json", source.Name))
		if err != nil {
			t.Fatalf("failed to read json file: %v", err)
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		t.Cleanup(func() { srv.Close() })
		cfg.SourcesConfig[i].URL = srv.URL
	}

	_, err = db.Exec("TRUNCATE raw_jobs, jobs, runs RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean tables: %v", err)
	}

	err = run(slog.New(slog.DiscardHandler), dsn, &cfg)
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

	err = run(slog.New(slog.DiscardHandler), dsn, &cfg)
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
		t.Fatalf("database not ready for testing client timeout: %v", err)
	}

	body, err := os.ReadFile("testdata/remotive.json")
	if err != nil {
		t.Fatalf("failed to read json file: %v", err)
	}
	goodSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer goodSrv.Close()

	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badSrv.Close()

	cfg := Config{
		SourcesConfig: []sources.SourceConfig{
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
	err = run(slog.New(slog.DiscardHandler), dsn, &cfg)
	if err != nil {
		t.Fatalf("expected err nil; got %v", err)
	}

	// 7. Leia de runs o total de linhas e quantas têm error preenchido. Afirme 2 e 1.
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

	cfg := Config{
		SourcesConfig: []sources.SourceConfig{
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

	err = run(slog.New(slog.DiscardHandler), dsn, &cfg)

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
		t.Fatalf("failed to query jobs rows: %v", err)
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

func TestLimiarValidation(t *testing.T) {
	tests := []struct {
		name            string
		body            string
		wantErr         bool
		fetchedExpected int
		failedExpected  int
	}{
		{"t1", `{"jobs":[{"id":"x"},{"id":"y"}]}`, true, 2, 2},
		{"t2", `{"jobs":[{"id":1},{"id":2},{"id":3},{"id":"x"}]}`, false, 4, 1},
		{"t3", `{"jobs":[]}`, false, 0, 0},
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
			cfg := Config{
				SourcesConfig: []sources.SourceConfig{
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

			err = run(slog.New(slog.DiscardHandler), dsn, &cfg)

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
					t.Fatalf("expected err too many itens failed; got nil")
				}
				if !strings.Contains(err.Error(), "too many items failed") {
					t.Errorf("expected err too many itens failed; got %v", err)
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
	err = processSource(conn, slog.New(slog.DiscardHandler), dummy{}, cfg, testTimeout)
	if err == nil {
		t.Fatalf("expected err; got nil")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("expected error to containg panic; got %v", err)
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
		t.Errorf("expected error to containg panic; got %v", err)
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
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

func pingDB(db *sql.DB) error {
	return db.Ping()
}

func setupPostgres() (*sql.DB, func() error, error) {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("jobs_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start postgres: %v", err)
	}
	teardown := func() error {
		err := testcontainers.TerminateContainer(ctr)
		if err != nil {
			return fmt.Errorf("failed to clean container: %v", err)
		}
		return nil
	}

	dsn, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, teardown, fmt.Errorf("failed to get dsn string: %v", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, teardown, fmt.Errorf("failed to open database: %v", err)
	}
	teardown = func() error {
		var errs []error
		errClose := db.Close()
		if errClose != nil {
			errs = append(errs, fmt.Errorf("failed to close db: %v", errClose))
		}
		errTerminate := testcontainers.TerminateContainer(ctr)
		if errTerminate != nil {
			errs = append(errs, fmt.Errorf("failed to terminate container: %v", errTerminate))
		}
		return errors.Join(errs...)
	}

	err = db.PingContext(ctx)
	if err != nil {
		return nil, teardown, fmt.Errorf("failed to ping db: %v", err)
	}

	migrationsFS, err := fs.Sub(migrationsFS, "db/migrations")
	if err != nil {
		return nil, teardown, fmt.Errorf("failed to rmeove FS prefix: %v", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationsFS)
	if err != nil {
		return nil, teardown, fmt.Errorf("failed to get goose provider: %v", err)
	}
	_, err = provider.Up(ctx)
	if err != nil {
		return nil, teardown, fmt.Errorf("failed to do goose up: %v", err)
	}
	return db, teardown, nil
}

func TestLoad(t *testing.T) {
	// YAML forbids tabs for indentation, so the documents below start at column 0.
	tests := []struct {
		name        string
		yaml        string
		want        sources.SourceConfig
		wantErr     bool
		errContains []string
	}{
		{
			name: "valid",
			yaml: `
sources:
  - name: remotive
    enabled: true
    url: https://remotive.com/api/remote-jobs
    timeout: 30s
    retries: 3
    initial_wait: 5s
`,
			want: sources.SourceConfig{
				Name:        "remotive",
				Enabled:     true,
				URL:         "https://remotive.com/api/remote-jobs",
				Timeout:     30 * time.Second,
				Retries:     3,
				InitialWait: 5 * time.Second,
			},
		},
		{
			name: "enabled source without retries",
			yaml: `
sources:
  - name: remotive
    enabled: true
    url: https://remotive.com/api/remote-jobs
    timeout: 30s
    initial_wait: 5s
`,
			wantErr:     true,
			errContains: []string{"remotive", "retries"},
		},
		{
			name: "disabled source without retry fields",
			yaml: `
sources:
  - name: remotive
    enabled: false
    url: https://remotive.com/api/remote-jobs
`,
			want: sources.SourceConfig{
				Name: "remotive",
				URL:  "https://remotive.com/api/remote-jobs",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := Load(path)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected err; got nil")
				}
				for _, s := range tt.errContains {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("expected err to contain %q; got %v", s, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("expected err nil; got %v", err)
			}
			if len(cfg.SourcesConfig) != 1 {
				t.Fatalf("expected 1 source; got %d", len(cfg.SourcesConfig))
			}
			if got := cfg.SourcesConfig[0]; got != tt.want {
				t.Errorf("expected %+v; got %+v", tt.want, got)
			}
		})
	}
}
