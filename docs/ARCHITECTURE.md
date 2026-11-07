# Architecture

job-radar is a handful of small processes around one Postgres database. Go does the I/O-heavy,
long-running work (collecting, sending); Python does the text and the models. They never call
each other: each reads and writes tables, and the schema in `db/migrations/` is the contract.
Everything runs on a Raspberry Pi, the database included; labelling and modelling from another
machine reach that one database through an SSH tunnel ([OPERATIONS.md](OPERATIONS.md)).

```
                       ┌──────────────────────── PostgreSQL ─────────────────────────┐
Remote OK ─┐           │ sources · runs · raw_jobs · jobs                           │
Remotive  ─┼─ collector┼▶ job_skills · job_embeddings · jev_answers · scores        │
Himalayas ─┘   (Go)    │ labels · jobs_sent                                         │
                       └──▲────────────▲──────────────▲───────────────▲─────────────┘
                          │            │              │               │
                  analyzer (Py)   jev-worker (Py)  radar label   digest (Go) ──▶ Discord
                  skills, tfidf   language, skills, (you, by hand) top unsent
                                  Jev answers, score               recent jobs
```

## Processes

| Compose service | Binary / module                  | Runs                 | Does |
| --------------- | -------------------------------- | -------------------- | ---- |
| `migrate`       | `collector -migrate`             | once, then exits 0   | applies pending migrations; every other service waits for it |
| `app`           | `collector`                      | forever, every 6h    | fetches each source, stores raw payloads and normalized jobs, marks duplicates |
| `analyzer`      | `python -m scripts.main`         | once per `up`        | extracts skills from new jobs, scores them with the TF-IDF baseline (`tfidf-v1`) |
| `jev-worker`    | `python -m scripts.jev_worker`   | forever, every 15min | brings each new job up to date for the digest's model: language, skills, the Jev's answers to the model's question set, the score |
| `digest`        | `radar digest-loop`              | forever, every 6h    | sends the best unsent jobs of `digest.model_version` to a Discord webhook |
| (none)          | `radar label / review / status`  | by hand              | the human side: labels that every model is measured against |

The scripts that train and measure models (`scripts.train`, `scripts.score`,
`scripts.evaluate`, the embedding scripts) are run by hand: see [MODELING.md](MODELING.md).

## Collection

One collection pass (`internal/collector.Run`):

1. apply pending migrations (a no-op once `migrate` ran);
2. give dedupe keys to jobs stored before keys existed (a no-op once done);
3. for each enabled source in `config.yaml`: open a `runs` row, fetch, and for each item store
   the raw payload in `raw_jobs`, map it to the common `Job` model, upsert it into `jobs` and
   mark it if it duplicates a job from another source; close the `runs` row with the counts
   and the error, if any;
4. warn about any source that failed or returned nothing for 2 passes in a row.

Failure handling, from smallest to largest:

- **one request fails:** retried with exponential backoff (`retries`, `initial_wait`);
- **one item fails** (bad payload, insert error): logged and skipped; the source fails only if
  more than half of its items fail;
- **one source fails:** the others still run; its `runs` row keeps the error;
- **the whole pass fails** (typically no network yet after a power cut): retried 3 times, 5 min
  apart, before waiting for the next tick;
- **Postgres is not up at start-up:** the collector waits up to 2 minutes for it.

Each source sits behind the same interface (`collector.Source`: fetch, read an item's id, map an
item to a `Job`). Adding one takes a file in `internal/sources/`, a case in `newSource`
(`internal/collector/collector.go`) and a migration that adds its `sources` row.

### Deduplication

The same job often appears twice: posted several times on one board, or on two boards. Two jobs
are the same when their company and title match after lowercasing and keeping only `a-z0-9`,
**and** they come from different sources or have an identical description. (Same company, title
and source with different descriptions are kept apart on purpose: usually the same role in
different locations.) A copy keeps its row and points at the oldest original through
`jobs.duplicate_of`; "live jobs" means `duplicate_of IS NULL` everywhere in the code.

## Data

| Table            | Key                              | Holds |
| ---------------- | -------------------------------- | ----- |
| `sources`        | `id`                             | the known job boards |
| `runs`           | `id`                             | one row per source per pass: start, finish, fetched, failed, error |
| `raw_jobs`       | `(source_id, external_id, content_hash)` | every payload exactly as received, one row per distinct version |
| `jobs`           | `(source_id, external_id)`       | the job as it is now, normalized, pointing at its latest raw payload |
| `job_skills`     | `(job_id, skill)`                | canonical skills found in the description |
| `job_embeddings` | `(job_id, model_version)`        | description embeddings (`pgvector`) |
| `jev_answers`    | `(job_id, question_set_version)` | the Jev's answers (probabilities) to one question set |
| `scores`         | `(job_id, model_version)`        | every model's score for every job, side by side |
| `labels`         | `job_id`                         | your judgement: `would_apply`, plus optional `hires_latam`, `seniority`, `note` |
| `jobs_sent`      | `(job_id, model_version)`        | what the digest already sent, per model |

Decisions behind the shape:

- **Raw payloads are never rewritten.** When a parser improves, `collector -renormalize` rebuilds
  `jobs` from `raw_jobs` without collecting again. A source editing a posting produces a new
  `raw_jobs` row (new hash), and the `jobs` row is updated in place.
- **`jobs` is the job as it is now, not a snapshot.** When the content changes, `scored_at` and
  `skills_extracted_at` go back to `NULL`, so the analyzer picks the job up again: a score
  computed on old text does not stand for the new text.
- **Scores are versioned.** `scores` is keyed by `model_version`, so every model ever measured
  coexists on the same jobs and is compared on data, not from memory. A changed method gets a
  new version; it never silently overwrites the old scores.
- **Jev answers are versioned by question set.** Changing any question means a new
  `question_set_version`, and that version is part of every `model_version` built on it.

## The LLM (Jev)

The Jev is used as a **feature extractor, never as the judge of the match**:

- the questions are in English, atomic, with literal criteria, and always about the job, never
  about the job and the CV together (`analyzer/jobradar/jev/questions.py`);
- numbers, dates and salary are extracted and compared in code, never asked;
- the text goes through the boilerplate cut before it is sent;
- it is optional: with no `JEV_API_KEY` the worker does not start and everything else runs (a test
  covers the client returning `None`);
- the provider is swappable by base URL; nothing outside `jobradar/jev/client.py` knows which
  one is used.

## Rules the code keeps

- Secrets (`DATABASE_URL`, `DISCORD_WEBHOOK_URL`, `JEV_API_KEY`) only come from the environment.
  The résumé (`cv.md`) is personal data and is not versioned either.
- Every collection attempt leaves a `runs` row, failures included.
- Migrations always have an `up` and a `down`.
- The frozen test set never takes part in training, hyperparameter search or question writing.
- `radar label` never shows any model's score while you label.
- Every reported number comes with the `model_version` that produced it.

## Out of scope, on purpose

Multiple users or authentication; a web UI or HTTP API (the digest is the product); applying
to jobs automatically; scraping sites without an API (LinkedIn, Indeed); managed cloud or any
recurring cost besides the Jev; real-time notifications (the digest is batched); Kubernetes:
`docker compose up` on a Raspberry Pi is the only deployment target (decided 2026-10-30, since
there is no cluster to run it on and compose already does the job).
