# analyzer

The Python side of [job-radar](../README.md): text cleaning, skill extraction, the Jev client,
the ranking models and their evaluation. It only talks to the rest of the project through
Postgres.

## Setup

Requirements: [uv](https://docs.astral.sh/uv/), a migrated Postgres (the collector migrates it;
compose publishes it on `localhost:5433`), and `cv.md` at the repository root (or `CV_PATH`).

```bash
uv sync                      # runtime dependencies
uv run --group dev pytest    # tests
```

Dependencies are split in groups so the image and the tests only pull what they need:

| Group   | For                                   | Use                    |
| ------- | ------------------------------------- | ---------------------- |
| (base)  | everything below unless noted         | `uv run …`             |
| `dev`   | pytest, ruff, mypy                    | `uv run --group dev …` |
| `jev`   | the Jev SDK                           | `uv run --group jev …` |
| `embed` | sentence-transformers (pulls torch)   | `uv run --group embed …` |

Scripts read `DATABASE_URL` from the environment: pass `--env-file ../.env` to `uv run`.

## Layout

```
jobradar/                 the library: pure functions plus a few queries, all tested
  db.py                   connect(): DATABASE_URL, autocommit, one-line errors
  pipeline.py             one worker pass: enrich, ask the Jev, score
  enrich.py               language and skills of the jobs still missing them
  language.py             langdetect, with a fixed seed
  text.py                 HTML → paragraphs of plain text (mojibake repaired)
  boilerplate.py          paragraphs shared by many descriptions, and cutting them
  skills.py               dictionary of skills and their aliases, and the matcher
  stack.py                the languages a job's title and description name (v9's stack)
  match.py                a job's skills against the CV's; the skills-gate baseline
  experience.py           required years, by regex
  salary.py               salary to annual USD
  embed.py                all-MiniLM-L6-v2 embeddings (group embed)
  frozen.py               the frozen test set
  jev/questions.py        the versioned question sets (qs-v1 to qs-v4)
  jev/pending.py          answers a question set for the jobs that lack it, in batches
  jev/client.py           the Jev client, behind a Protocol; None without JEV_API_KEY
  jev/manual.py           jev-manual: the answers with hand-picked weights
  model/features.py       the combined model's features, one function for train and score
  model/combined.py       sklearn-combined: SPECS (versions, features, rules), training,
                          scoring, MODEL_VERSION (the digest's model)
scripts/                  thin command-line entry points: python -m scripts.<name>
tests/
data/frozen.json          test-set-v2
data/models/              trained models, one file per model_version (versioned: the
                          worker image needs them, and a version never changes)
data/queues/              label queues for radar label (audits, rounds)
```

## Scripts

Always-on (compose services):

| Script | Does |
| --- | --- |
| `scripts.main` | the `analyzer` service: skills of new jobs into `job_skills`, then `tfidf-v1` scores |
| `scripts.jev_worker` | the `jev-worker` service: every 15 min, one `jobradar.pipeline` pass for `MODEL_VERSION`: language, skills, the model's Jev answers and its score, each only for the jobs still missing it |

By hand (`uv run --env-file ../.env python -m scripts.<name>`):

| Script | Does |
| --- | --- |
| `train` | trains `sklearn-combined` on the labels outside the test set; prints CV AP and coefficients; saves `data/models/<version>.joblib` |
| `score` | scores every live job with the saved `sklearn-combined` |
| `evaluate [version …]` | AP, ROC AUC and P@10 of each model version on the test set |
| `explain [version]` | the coefficients of a saved model |
| `score_jev` | recomputes `jev-manual-qs-v1` from the stored answers, no Jev call |
| `update_jobs` | one pass of the worker, then exit (group `jev`) |
| `answer_jev <set> [--training-labels]` | answers a question set for the jobs that lack it, without scoring; for trying a new set (group `jev`) |
| `score_skills_gate` | writes the `skills-gate-v1` baseline |
| `embed_jobs`, `score_embeddings` | embeds new jobs into `job_embeddings`, then scores `minilm-l6-v2` (group `embed`) |
| `detect_language` | tags `jobs.language` on the jobs that have none (the worker does it too) |

The measurement protocol, the results of every model and the known problems are in
[docs/MODELING.md](../docs/MODELING.md).

## A new model version

```bash
# 1. change jobradar/model/features.py or combined.py, and bump MODEL_VERSION
uv run --env-file ../.env python -m scripts.train      # refuses to overwrite an existing version
uv run --env-file ../.env python -m scripts.score
uv run --env-file ../.env python -m scripts.evaluate
```

## Checks

```bash
uv run ruff check .
uv run ruff format --check .
uv run --group dev mypy          # strict, over jobradar/, scripts/ and tests/
uv run --group dev pytest
```

Or `make lint test-py` from the repository root.
