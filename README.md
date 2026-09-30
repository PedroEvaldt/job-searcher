# job-radar

A self-hosted aggregator of international remote job postings. It collects jobs from
public job boards, stores them in Postgres, and (in later phases) ranks each one against
your résumé and sends you a weekly digest on Discord.

Single-user by design: you clone it, run it on your own machine, and nothing leaves it.

## Architecture

```
collector (Go) ──writes──▶ PostgreSQL ◀──reads── analyzer (Python)
```

- **collector** (Go): fetches the sources listed in `config.yaml`, stores the raw payload
  in `raw_jobs`, normalizes it into `jobs`, and records every execution in `runs`. It
  applies the database migrations itself on startup.
- **analyzer** (Python): reads from the same database. Today it only reports how many
  jobs are still waiting to be scored.

The two sides never talk to each other directly; the database schema is the contract.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) with Compose v2 (tested with Compose 5.5)
- Git

To run outside Docker (optional):

- [Go](https://go.dev/dl/) 1.26+
- [uv](https://docs.astral.sh/uv/getting-started/installation/) (it installs Python 3.13 for you)

## Quick start

```
git clone https://github.com/PedroEvaldt/job-searcher.git
cd job-searcher
cp .env.example .env
docker compose up --build
```

The example values in `.env.example` work as they are. Change the password if you plan
to keep the database around.

What you should see, in this order:

1. `postgres` becomes healthy.
2. `app` (the collector) applies the migrations on the first run
   (`migration applied version=1 ...`, up to version 3), then collects:
   `source collected source=remoteok fetched=... failed=0`.
3. `app` exits with code 0.
4. `analyzer` prints how many jobs are pending a score, and exits.

Postgres keeps running after that. Press `Ctrl+C` to stop, or run
`docker compose down` from another terminal. Your data stays in a Docker volume; to wipe
it, use `docker compose down -v`.

> Always pass `--build` after pulling or changing code. Without it, Compose reuses an
> existing image with the same tag and runs the old binary.

## Configuration

| What                                    | Where                                              |
| --------------------------------------- | -------------------------------------------------- |
| Database credentials and `DATABASE_URL` | `.env` (never committed; copy from `.env.example`) |
| Sources to collect, enabled or not      | `config.yaml`                                      |

`config.yaml` is baked into the collector image, so changing it requires
`docker compose up --build`.

Postgres is exposed on the host at port **5433** (not 5432), to avoid clashing with a
local Postgres you may already have.

## Running locally (without Docker for the apps)

Start only the database:

```
docker compose up -d postgres
```

Collector, from the repository root (reads `.env` from the current directory):

```
go run .
```

Analyzer:

```
cd analyzer
uv run --env-file ../.env main.py
```

## Tests

```
go test ./... -count=1
```

The tests need Docker running: they start a throwaway Postgres with
[testcontainers](https://golang.testcontainers.org/), apply the migrations, and feed the
collector with the fixtures in `testdata/`. No internet access is needed, and your
development database is never touched.

Python checks:

```
cd analyzer
uv run ruff check .
uv run mypy --strict main.py
```

## Project status

- ✅ **Phase 0:** one source (Remote OK) end to end: collector, Postgres, analyzer,
  integration test, `docker compose up` on a clean machine.
- 🔜 **Phase 1:** more sources behind a common interface, deduplication, rate limiting,
  and running unattended for 7 days.

Later phases add data analysis, skill extraction, ranking against your résumé, a weekly
Discord digest, and a Kubernetes deployment.

## License

[MIT](LICENSE)
