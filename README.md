# job-radar

A self-hosted aggregator of international remote job postings. It collects jobs from
public job boards, stores them in Postgres, and (in later phases) ranks each one against
your résumé and sends you a daily digest on Discord.

Single-user by design: you clone it, run it on your own machine, and nothing leaves it.

## Architecture

```
Remote OK ─┐
Remotive  ─┼─▶ collector (Go) ──writes──▶ PostgreSQL ◀──reads── analyzer (Python)
Himalayas ─┘   every collect_every
```

- **collector** (Go): a long-running process. It collects right away on startup, then
  again every `collect_every` (6h by default). Each cycle:
  1. applies any pending database migration;
  2. gives dedupe keys to jobs written before they existed (a no-op once done);
  3. fetches every enabled source in `config.yaml`, one after the other;
  4. warns about any source that has failed or returned nothing for 2 cycles in a row.
- **analyzer** (Python): reads from the same database, extracts skills, salary and required
  experience from each job, and ranks every job against your résumé with TF-IDF. See
  [`analyzer/README.md`](analyzer/README.md).

The two sides never talk to each other directly; the database schema is the contract.

### Sources

| Source    | How it is fetched                                   |
| --------- | --------------------------------------------------- |
| Remote OK | one request; the first element is a legal notice    |
| Remotive  | one request                                         |
| Himalayas | cursor pagination, up to 20 pages per cycle         |

Each source sits behind the same Go interface (`Source`: fetch, read an item's id, map
an item to a job). Adding one takes a new file in `internal/sources/`, a case in
`newSource` (`main.go`), and a migration that adds its row to the `sources` table.

### What a failure looks like

- **One request fails:** it is retried with exponential backoff (`retries`,
  `initial_wait` in `config.yaml`).
- **One item fails** (bad payload, insert error): it is logged and skipped. A source
  counts as failed if more than half of its items fail.
- **One source fails:** the others still run. Every attempt is a row in `runs`, with the
  error if there was one.
- **The whole cycle fails** (typically no network yet after a power cut): it is tried
  again 3 times, 5 minutes apart, before waiting for the next tick.
- **The database is not up yet at startup:** the collector waits up to 2 minutes for it.

### Tables

| Table      | What it holds                                                                           |
| ---------- | --------------------------------------------------------------------------------------- |
| `raw_jobs` | every payload exactly as the source sent it, one row per distinct version               |
| `jobs`     | one row per job and source, normalized; points at its latest `raw_jobs` row             |
| `runs`     | one row per source per cycle: start, finish, items fetched, items failed, error         |
| `sources`  | the known sources                                                                       |

The raw payloads are never rewritten, so `jobs` can be rebuilt from them without
collecting again.

### Deduplication

The same job often shows up more than once: a company posts it several times on one
board, or it appears on two boards. Two jobs are the same when:

- their **company** and **title** match after lowercasing and dropping everything but
  `a-z` and `0-9` ("Full-stack" = "Full stack"), **and**
- they come from **different sources**, or from the same source with an **identical
  description**.

A copy keeps its row but points at the original through `jobs.duplicate_of` (the oldest
matching job). Same company, title and source with different descriptions are kept apart
on purpose: they are often the same role in different locations. A title or company that
has no `a-z0-9` at all (e.g. written in Chinese) is never treated as a duplicate.

To read each job once, filter on `duplicate_of IS NULL`.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) with Compose v2
- Git

To run outside Docker (optional):

- [Go](https://go.dev/dl/) 1.26+
- [uv](https://docs.astral.sh/uv/getting-started/installation/) (it installs Python 3.13 for you)
- [goose](https://github.com/pressly/goose), only for the `make` targets

## Quick start

```
git clone https://github.com/PedroEvaldt/job-searcher.git
cd job-searcher
cp .env.example .env
docker compose up --build
```

The example values in `.env.example` work as they are. Change the password if you plan
to keep the database around.

What you should see:

1. `postgres` becomes healthy.
2. `app` (the collector) applies the migrations (`migration applied version=1`, up to
   `version=7`), then collects each source:
   `source collected source=remoteok fetched=... failed=0`, then `remotive`, then
   `himalayas` (the slowest, about half a minute).
3. `analyzer` extracts skills, scores every live job against the CV, and exits, printing
   how many jobs are still unscored. It does not wait for the collector: on the very first
   start it may run before anything was collected, or even before the tables exist. Run it
   again with `docker compose run --rm analyzer`.
4. `app` keeps running and collects again every `collect_every`.

To stop, press `Ctrl+C` (or `docker compose stop` from another terminal). Your data
stays in a Docker volume.

> - `docker compose down` also removes the containers, and their logs with them.
> - `docker compose down -v` also deletes the volume: the whole database.
> - Always pass `--build` after pulling or changing code. Without it, Compose reuses an
>   existing image with the same tag and runs the old binary.

## Configuration

| What                                    | Where                                              |
| --------------------------------------- | -------------------------------------------------- |
| Database credentials and `DATABASE_URL` | `.env` (never committed; copy from `.env.example`) |
| What to collect, and how often          | `config.yaml`                                      |

`config.yaml` is mounted read-only into the collector and read once at startup, so after
editing it run `docker compose restart app` (no rebuild needed). An unknown key or a
missing value on an enabled source stops the collector at startup with an error.

| Key             | Meaning                                                                         |
| --------------- | ------------------------------------------------------------------------------- |
| `collect_every` | time between cycles (`6h`). Remotive asks for at most 4 requests a day          |
| `name`          | the source; must match a row in the `sources` table                             |
| `enabled`       | `false` skips the source without removing it                                    |
| `url`           | the endpoint to fetch                                                           |
| `timeout`       | budget for all of the source's work in one cycle, retries and pages included    |
| `retries`       | how many times one request is tried before the source fails                     |
| `initial_wait`  | wait before the first retry; it doubles on each new one                         |
| `interval`      | minimum time between two requests to the same source (rate limit)              |

Durations use Go's format: `500ms`, `30s`, `6h`.

Postgres is exposed on the host at port **5433** (not 5432), to avoid clashing with a
local Postgres you may already have.

## Running locally (without Docker for the apps)

Start only the database:

```
docker compose up -d postgres
```

Collector, from the repository root (reads `.env` from the current directory):

```
go run .          # collects now, then every collect_every, until Ctrl+C
go run . -once    # one cycle, then exits (code 1 if every source failed)
```

Analyzer:

```
cd analyzer
uv run --env-file ../.env python -m scripts.main
```

Migrations by hand (the collector applies them on its own; these are for checking or
rolling back). They read `DATABASE_URL` from `.env`:

```
make gstatus   # applied and pending migrations
make gup       # apply all pending
make gdown     # roll back the last one
```

## Checking the data

Saved queries live in `db/queries/`. Run them inside the Postgres container, from the
repository root:

```
docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < db/queries/live_duplicates.sql
```

| File                  | What it answers                                                                                                                                                |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `health.sql`          | runs and errors per source per day (São Paulo time), and the longest gap between two runs of each source. Takes a start date: add `-v start=2026-10-08` to `psql` |
| `live_duplicates.sql` | duplicate groups left among the jobs you read (`duplicate_of IS NULL`). Expected: `0 \| 0`                                                                     |
| `duplicates.sql`      | duplicate groups across all jobs, ignoring the marks: how much the dedupe has to do                                                                            |

## Labelling jobs (`radar`)

`radar` is the single-user CLI: it is how you tell the project which jobs you would actually
apply to, so later phases have a human ground truth to measure models against. It reads the
same database and never shows any model score while you label.

Build it and run it against a database (it reads `DATABASE_URL`):

```
go build -o cmd/radar/radar ./cmd/radar
./cmd/radar/radar status      # LABELED <done>/<remaining> among the jobs you read
./cmd/radar/radar label       # label the next unlabelled jobs, one at a time
```

`label` shows one job at a time — title, company, and a cleaned-up description — and asks
whether you would apply (`y`/`n`), plus three optional fields (hires LatAm, seniority, a free
note). Each answer is written to the `labels` table right away, so a later run resumes where
you left off and never re-shows a job you already labelled. Press Ctrl+D to stop.

The `labels` table is human ground truth: only `would_apply` is the label that trains and
evaluates models. The optional fields are notes, never evaluation targets. How those labels
become a frozen test set, and how the TF-IDF ranking scores against it, is in
[`analyzer/README.md`](analyzer/README.md).

## Running on a Raspberry Pi

The collector runs unattended on a Raspberry Pi (arm64). Docker builds both images
natively there; nothing changes in the code or in the compose file.

```
git clone https://github.com/PedroEvaldt/job-searcher.git
cd job-searcher
cp .env.example .env          # set a real password
docker compose up -d --build
```

- `app` restarts unless you stop it by hand, and `postgres` always restarts, so both come
  back after a reboot or a power cut on their own.
- Logs: `docker compose logs -f app`. They rotate at 10 MB × 3 files.
- To update: `git pull && docker compose up -d --build app`.
- To stop without losing logs or data: `docker compose stop`.

## Tests

```
go test ./... -count=1 -race
```

The tests need Docker running: they start a throwaway Postgres with
[testcontainers](https://golang.testcontainers.org/), apply the migrations, and feed the
collector with the fixtures in `testdata/` and fake sources. No internet access is
needed, and your development database is never touched.

Python checks:

```
cd analyzer
uv run ruff check .
uv run --group dev mypy --strict jobradar/ scripts/ tests/
```

## Project status

- ✅ **Phase 0:** one source end to end: collector, Postgres, analyzer, integration test,
  `docker compose up` on a clean machine.
- ✅ **Phase 1:** three sources behind a common interface, retries and rate limiting,
  deduplication, and running unattended: 7 days on a Raspberry Pi, through 3 power cuts,
  with no intervention.
- ✅ **Phase 2:** exploring the collected data in Python: EDA bench, measured data defects,
  a boilerplate cutter, and location/language policies (see [`docs/eda.md`](docs/eda.md)).
- ✅ **Phase 3a:** extraction in code — skills (`job_skills`), salary, required years and job
  age (`jobs` columns), and a first TF-IDF ranking of every job against the résumé
  (`scores`, `model_version='tfidf-v1'`).
- ✅ **Phase 3b:** the `radar` CLI and the first honest baseline — jobs hand-labelled into
  `labels` (250 so far), a stratified test set frozen to `analyzer/data/frozen.json` (`test-set-v1`,
  75 held out, 5 positive), and the Phase 3a ranking measured on it: `tfidf-v1 ROC AUC ≈ 0.51` — no
  ranking power yet. That is the number the next phases have to beat.
- ✅ **Phase 4a:** embeddings vs. the baseline — every live job and the CV embedded with
  `all-MiniLM-L6-v2` into a `pgvector` table (`job_embeddings`) and scored by cosine into `scores`
  (`model_version='minilm-l6-v2'`), then compared on the frozen set: `minilm-l6-v2 ROC AUC = 0.46`
  vs. `tfidf-v1 0.51`. Raw cosine against the CV — TF-IDF or embedding — does not rank the jobs
  worth applying to; that sends the next phase to richer, Jev-extracted features.
- ✅ **Phase 4b:** an LLM (Jev) as a feature extractor, never the match judge — a 3-question set
  (`open_to_latam`, `fully_remote`, `is_senior_role`), a swappable client that falls back to the
  cosine score with no API key, and job language detected with `langdetect` (`jobs.language`).
  Cost measured at **~$0.05 per 1000 jobs**. The 3 answers combined with manual weights chosen
  before looking at the frozen set score **`jev-manual-qs-v1` ROC AUC = 0.72** on the same
  75-job set — well above both Phase 4a baselines, on a thin 5-positive sample. See
  [`analyzer/README.md`](analyzer/README.md#jev-phase-4b).
- ✅ **Phase 4c:** a `sklearn` `LogisticRegression` learning weights over every feature at once
  (Jev answers + `minilm-l6-v2` cosine + the Phase 3a code-extracted features), trained on the
  175 non-frozen labelled jobs, `C` chosen by cross-validation on that fold only. Honest result:
  **`sklearn-combined-v1` ROC AUC = 0.65**, *below* the hand-picked `jev-manual-qs-v1` (0.72) —
  the top coefficient belongs to `cosine_minilm`, a feature with no standalone signal (Phase 4a),
  likely overfit at this sample size. See [`analyzer/README.md`](analyzer/README.md#sklearn-the-combined-model-phase-4c).

A later phase adds the daily Discord digest. There is no Kubernetes deployment planned —
`docker compose up` on a Raspberry Pi is the only deploy target.

## License

[MIT](LICENSE)
