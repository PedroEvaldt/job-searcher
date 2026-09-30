package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	rjCount = 100
	jCount  = 100
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
	expected := 1
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
		Sources: []Source{
			{
				Name:    "remoteok",
				Enabled: true,
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
			w.Write(body)
		}))
		t.Cleanup(func() { srv.Close() })
		cfg.Sources[i].URL = srv.URL
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
	if runsCount != 2 {
		t.Errorf("expected 2; got %d", runsCount)
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
