# job-radar

A self-hosted aggregator of international remote job postings. It collects jobs from public
job boards into Postgres, extracts what matters from each one (skills, seniority, location
policy, salary), ranks them against your résumé, and sends the best ones to Discord.

Single-user by design: you run it on your own machine (mine is a Raspberry Pi), and your
data never leaves it.

```
Remote OK ─┐                                   ┌── analyzer (Python)   skills, TF-IDF
Remotive  ─┼─▶ collector (Go) ─▶ PostgreSQL ◀──┼── jev-worker (Python) LLM answers, model scores
Himalayas ─┘   every 6h                        ├── radar (Go CLI)      label jobs by hand
                                               └── digest (Go)  ─▶ Discord
```

Go collects, Python analyzes, and the two never talk directly: the Postgres schema is the
contract. More in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Quick start

You need [Docker](https://docs.docker.com/get-docker/) with Compose v2.

```
git clone https://github.com/PedroEvaldt/job-searcher.git
cd job-searcher
cp .env.example .env          # the example values work; change the password to keep the data
cp cv.example.md cv.md        # then write your own résumé in it
docker compose up -d --build
docker compose logs -f app
```

What happens:

1. `postgres` becomes healthy, then `migrate` applies the schema and exits.
2. `app` (the collector) fetches every source (`source collected source=remoteok
   fetched=... failed=0`, then `remotive`, then `himalayas`) and repeats every 6 hours.
3. `analyzer` extracts the skills of the new jobs, scores them with TF-IDF and exits. Run it
   again after a collection with `docker compose run --rm analyzer`.
4. `digest` sends the best unsent jobs to Discord every 6 hours. With no
   `DISCORD_WEBHOOK_URL` it prints them to its log instead.

`jev-worker` keeps every new job ready for the ranking (language, skills, the LLM's answers,
the model's score). It is off by default: set `JEV_API_KEY` and `COMPOSE_PROFILES=jev` in
`.env` to turn it on. Without a key everything else runs.

> `docker compose down` keeps the data; `docker compose down -v` deletes the database.
> After pulling new code, always rebuild (`docker compose up -d --build`).

## Configuration

| What                                        | Where                                       |
| ------------------------------------------- | ------------------------------------------- |
| Credentials, `DATABASE_URL`, API keys       | `.env` (never committed; see `.env.example`) |
| Sources, collection interval, digest model  | `config.yaml`                               |
| Your résumé                                 | `cv.md` (never committed; see `cv.example.md`) |

`config.yaml` is read at start-up and strictly validated: an unknown key or a missing value
on an enabled source stops the process with an error. After editing it, `docker compose
restart app digest`.

| Key                    | Meaning                                                                    |
| ---------------------- | -------------------------------------------------------------------------- |
| `collect_every`        | time between collections (`6h`; Remotive asks for at most 4 requests a day) |
| `sources[].name`       | the source; must match a row in the `sources` table                        |
| `sources[].enabled`    | `false` skips the source without removing it                               |
| `sources[].url`        | the endpoint to fetch                                                      |
| `sources[].timeout`    | budget for all of a source's work in one collection, retries included     |
| `sources[].retries`    | attempts per request before the source fails                              |
| `sources[].initial_wait` | wait before the first retry; doubles on each new one                     |
| `sources[].interval`   | minimum time between two requests to the same source (rate limit)         |
| `digest.model_version` | which `scores.model_version` the digest ranks by                          |
| `digest.min_score`     | lowest score sent, on that model's own scale                              |
| `digest.every`         | time between digests                                                       |
| `digest.max_age`       | only jobs posted within this (`336h` = 14 days); `0` = any age             |

Durations use Go's format (`500ms`, `30s`, `6h`). Postgres is published on the host at port
**5433**, to stay clear of a local Postgres on 5432.

## Labelling jobs: `radar`

The models are measured against your own judgement, so the project needs labels: jobs you
marked as "I would apply" or not. `radar` is the CLI for that. It never shows a model score
while you label.

```
go run ./cmd/radar status              # how many live jobs are labelled
go run ./cmd/radar label               # label the next unlabelled jobs, one at a time
go run ./cmd/radar label queue.json    # label the jobs of a queue file, in its order
go run ./cmd/radar review              # re-check every job labelled "would apply"
go run ./cmd/radar digest              # send (or print) one digest now
```

`radar` reads `DATABASE_URL` from `.env` or the environment. How labels become a frozen test
set and what the models score on it is in [docs/MODELING.md](docs/MODELING.md).

## Development

Requirements: [Go](https://go.dev/dl/) 1.26+, [uv](https://docs.astral.sh/uv/) (it installs
Python 3.13), Docker (the Go integration tests start a throwaway Postgres), and optionally
[golangci-lint](https://golangci-lint.run/) and [goose](https://github.com/pressly/goose).

```
docker compose up -d postgres          # just the database
go run ./cmd/collector -once           # one collection, then exit
cd analyzer && uv run --env-file ../.env python -m scripts.main   # the analyzer, once

make test     # Go (unit + integration) and Python tests
make lint     # gofmt, go vet, golangci-lint, ruff, mypy --strict
make help     # every target
```

The Python side has its own guide: [analyzer/README.md](analyzer/README.md).

### Layout

```
cmd/collector/      collector binary: flags and wiring only
cmd/radar/          radar CLI: label, review, status, digest, digest-loop
internal/collector/ one collection pass: sources → raw_jobs → jobs, dedupe, migrations
internal/config/    config.yaml schema, shared by both binaries
internal/sources/   one file per job board, behind a common Source interface
internal/client/    HTTP client with retries, backoff and rate limiting
db/migrations/      goose SQL migrations, embedded into the collector binary
db/queries/         saved SQL for checking data health by hand
analyzer/           Python: text cleaning, skills, Jev client, models, evaluation
docs/               architecture, modelling notes and results, operations
archive/            finished experiments and the project's learning log, kept for reference
```

### Checking the data

```
docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < db/queries/live_duplicates.sql
```

| File                  | What it answers                                                                 |
| --------------------- | ------------------------------------------------------------------------------- |
| `health.sql`          | runs and errors per source per day, and the longest gap between runs (`-v start=YYYY-MM-DD`) |
| `live_duplicates.sql` | duplicate groups left among the live jobs. Expected: `0 \| 0`                   |
| `duplicates.sql`      | duplicate groups across all jobs, ignoring the marks                            |

## Where it stands

- **Collection** runs unattended on a Raspberry Pi: three sources, retries and rate limits,
  cross-source deduplication. It ran 7 days straight through 3 power cuts with no
  intervention. About 16,000 live jobs collected.
- **Extraction** in code: skills, salary in annual USD, required years, job age, language.
  An LLM (Jev) answers six literal questions about each job (can someone in Brazil take it?
  fully remote? senior? backend, infra or data work? does it require a completed degree?) for
  about $0.07 per 1000 jobs.
- **Ranking**: a logistic regression over those answers and the skill match, with hard rules
  on top (language, degree). All 10 of the live jobs it ranks first are ones I would apply to
  (the first learned model: 1 of 10); on a frozen test set of 150 hand-labelled jobs its
  average precision is 0.50 (a random order: 0.02; the first learned model: 0.12).
  How it is measured and what still goes wrong: [docs/MODELING.md](docs/MODELING.md).
- **Digest**: about 4 jobs a day on Discord, from the last 14 days of postings, ranked by
  `sklearn-combined-v10`.

Running it on the Pi, backups and upgrades: [docs/OPERATIONS.md](docs/OPERATIONS.md).

## License

[MIT](LICENSE)
