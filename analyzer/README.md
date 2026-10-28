# analyzer

The Python side of [job-radar](../README.md). It reads the Postgres that the Go `collector`
fills, extracts meaning from each job, and ranks it against the résumé. As of Phase 3a it
extracts skills, normalizes salary and required experience, derives job age, and produces a
first TF-IDF ranking of every job against the CV.

Go and Python never share memory or files — only the Postgres schema. See the root README
for the whole picture.

## Requirements

- [uv](https://docs.astral.sh/uv/)
- A running Postgres with the collected data, migrated to the latest schema (the Go
  `collector` applies the migrations). In the compose stack it is published on
  `localhost:5433`; point `DATABASE_URL` at it.
- `cv.md` at the repo root — your résumé in Markdown. The scorer reads it as the document to
  match every job against.

## Setup

```bash
uv sync          # runtime deps only (psycopg, scikit-learn)
```

Dependencies live in groups, so neither the image nor the tests pull what they do not need:

| Group  | What for                       | How to use                 |
| ------ | ------------------------------ | -------------------------- |
| (base) | the analyzer itself            | `uv sync`                  |
| `dev`  | tests, lint, types             | `uv run --group dev …`     |
| `eda`  | exploration (pandas, IPython…) | `uv run --group eda …`     |

`DATABASE_URL` is read straight from the environment (no dotenv loading). Export it, or pass
`--env-file ../.env` to `uv run`.

## Running the analyzer

```bash
uv run --env-file ../.env python -m scripts.main
```

One pass over the database, in order:

1. **Skills** — for every live job still unextracted (`skills_extracted_at IS NULL`), it runs
   the text pipeline (`jobradar/text.py` → `jobradar/boilerplate.py`) and the dictionary matcher (`jobradar/skills.py`),
   then writes the canonical skills to `job_skills (job_id, skill, confidence)`. Jobs are
   claimed in batches with `FOR UPDATE SKIP LOCKED` so two instances never process the same
   one, and a job's old rows are deleted before reinsert, so a skill dropped from the text
   disappears. A job whose text changed is pushed back here automatically (the collector
   resets `skills_extracted_at` on a content change, the same way it resets `scored_at`).
2. **Score** — it builds one TF-IDF vocabulary over the CV and every live job (the CV is
   document 0), takes the cosine of the CV against each job, and upserts into
   `scores (job_id, model_version, score)` under `model_version = 'tfidf-v1'`, marking
   `scored_at`. Idempotent: a second run selects nothing.
3. Prints how many jobs are still unscored.

So the analyzer fills `job_skills`, `scores`, and — via the Go side — the `jobs` columns
`posted_at`, `salary_min`, `salary_max`, `currency`, `salary_period`.

## Evaluation: the frozen test set

A score is only trustworthy on jobs the model never saw. Once ~100 jobs were hand-labelled
with the `radar` CLI (see the [root README](../README.md)), the labels were split once and the
test half was frozen to disk — it never enters training, tuning, or question rewriting, in any
phase.

- `scripts/freeze_test_set.py` → `data/frozen.json`. Reads every row of `labels`, makes a stratified split
  (`random_state = 7`, 30% held out) so the positive/negative ratio is kept on both sides, and
  writes the test half to `data/frozen.json`: a versioned record (`test-set-v1`) with the counts and
  the `(job_id, would_apply)` of each held-out job. Only the test set is stored — the train set
  is "every label not in here" — so adding more labels later never disturbs it. Re-running is
  deterministic: it overwrites the file identically.
- `scripts/evaluate_baseline.py`. Loads `data/frozen.json`, fetches the `tfidf-v1` score of exactly those
  jobs, and prints the ROC AUC with its `model_version` and the set size. ROC AUC fits here
  because the label is binary and rare while the score is a continuous ranking: 0.5 is no
  ranking power, 1.0 is perfect.

```bash
uv run --env-file ../.env python -m scripts.freeze_test_set     # writes data/frozen.json
uv run --env-file ../.env python -m scripts.evaluate_baseline   # prints the metric
```

First result: `tfidf-v1 ROC AUC=0.5000 n=37 pos=5` — on the frozen set the Phase 3a ranking is
no better than chance. That is the honest baseline the embedding and Jev phases have to beat.

## Production helpers

Each has type hints and tests (`tests/`):

- `jobradar/text.py` — turns a description's HTML into a list of plain-text paragraphs.
- `jobradar/boilerplate.py` — counts paragraphs across descriptions and cuts the shared boilerplate.
- `jobradar/skills.py` — `skills_extractor(text, aliases)`: finds the dictionary's skills in a text,
  maps aliases to a canonical name, and returns a confidence per skill.
- `jobradar/salary.py` — `normalize_salary(...)`: converts an amount + currency + period to annual USD
  for comparison; returns `None` when the input is missing or the unit is unknown.
- `jobradar/experience.py` — `extract_years(text)`: estimates the required years of experience by
  regex on the cleaned description.

The scoring pipeline itself (skills + TF-IDF) lives in `scripts/main.py`.

## The EDA bench

The exploration is a set of scripts under `eda/`, written as `# %%` cells and meant for an
IPython REPL (`uv run --group eda ipython`, then `%run eda/01_overview.py`). Each one also
runs straight through:

```bash
uv run --group eda --env-file ../.env python eda/01_overview.py
```

| Script                 | What it shows                                                   |
| ---------------------- | --------------------------------------------------------------- |
| `db.py`                | `sql_to_dataframe(sql)` — the shared SQL → DataFrame loader      |
| `01_overview.py`       | shape, dtypes, jobs per source, description sizes, jobs per day  |
| `02_text_defects.py`   | HTML, HTML entities and mojibake per source (defects 1–3)        |
| `03_payload_fields.py` | fields the collector drops; sentinels (defects 4–6)             |
| `04_boilerplate.py`    | paragraphs repeated across jobs, and how much text they take     |
| `05_location.py`       | each live job as accept/reject/unknown for a Brazil resident     |
| `06_language.py`       | language guess per description; the non-English share            |
| `07_skills.py`         | skill presence across the live jobs, after the text pipeline     |
| `08_date.py`           | job age (`now − posted_at`) distribution                         |
| `09_salary.py`         | salary coverage per source and the normalized annual-USD spread  |
| `10_experience.py`     | required years of experience extracted from descriptions         |
| `11_similarity.py`     | TF-IDF cosine of the CV against every job; the top/bottom ranking |

The findings are written up in [`../docs/eda.md`](../docs/eda.md).

## Checks

```bash
uv run --group dev pytest                                     # tests (tests/)
uv run ruff check .                                           # lint
uv run --group dev mypy --strict jobradar/ scripts/ tests/
```

`mypy --strict` covers the production modules. The `eda/` scripts import pandas and
matplotlib, which ship no type stubs, so they are not kept under strict typing; scikit-learn
ships none either, so it is ignored for the production modules in `pyproject.toml`.
