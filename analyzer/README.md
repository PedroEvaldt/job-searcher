# analyzer

The Python side of [job-radar](../README.md). It reads the Postgres that the Go `collector`
fills, extracts meaning from each job, and ranks it against the résumé. As of Phase 3a it
extracts skills, normalizes salary and required experience, derives job age, and produces a
first TF-IDF ranking of every job against the CV. Phase 4a adds a second ranking — every job and
the CV embedded with a sentence-transformer and scored by cosine in `pgvector` — plus an
evaluation harness that measures any model against a frozen test set.

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
| `embed` | embeddings (sentence-transformers → torch) | `uv run --group embed …` |

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

## Embeddings (Phase 4a)

A second, model-heavy ranking kept out of the always-on `main.py`: loading the
sentence-transformer pulls in torch, so it lives in its own `uv` group (`embed`) and its own
scripts.

- `jobradar/embed.py` — `embed(texts)`: encodes a list of texts with `all-MiniLM-L6-v2` into
  384-dimensional vectors.
- `scripts/embed_jobs.py` — embeds every live job with no `minilm-l6-v2` vector yet (after the
  same boilerplate cut the scorer uses) and upserts it into `job_embeddings (job_id,
  model_version, embedding vector(384))`, a `pgvector` table symmetric to `scores`. Resumable and
  idempotent: a second run embeds only what is still missing.
- `scripts/score_embeddings.py` — embeds the CV, then lets `pgvector` take the cosine
  `1 - (embedding <=> cv)` of every job against it in one `INSERT … SELECT`, upserting into
  `scores` under `model_version = 'minilm-l6-v2'`.

```bash
uv run --group embed --env-file ../.env python -m scripts.embed_jobs        # fill job_embeddings
uv run --group embed --env-file ../.env python -m scripts.score_embeddings  # write minilm-l6-v2 scores
```

Postgres needs the `vector` extension; the compose stack uses the `pgvector/pgvector:pg16` image
for it. A `scores` row carries the embedding model's version, not the CV's — change `cv.md` and
re-run and the row is overwritten under the same `model_version` (debt D49 in
[`../docs/PROGRESS.md`](../docs/PROGRESS.md)).

## Evaluation: the frozen test set

A score is only trustworthy on jobs the model never saw. Once the jobs were hand-labelled
with the `radar` CLI (see the [root README](../README.md)), the labels were split once and the
test half was frozen to disk — it never enters training, tuning, or question rewriting, in any
phase.

- `scripts/freeze_test_set.py` → `data/frozen.json`. Reads every row of `labels`, makes a stratified split
  (`random_state = 7`, 30% held out) so the positive/negative ratio is kept on both sides, and
  writes the test half to `data/frozen.json`: a versioned record (`test-set-v1`) with the counts and
  the `(job_id, would_apply)` of each held-out job. Only the test set is stored — the train set
  is "every label not in here" — so adding more labels later never disturbs it. Re-running is
  deterministic: it overwrites the file identically.
- `scripts/evaluate.py`. Loads `data/frozen.json` and, for each `model_version` given on the
  command line (default: `tfidf-v1` and `minilm-l6-v2`), fetches that model's score of exactly
  those jobs and prints the ROC AUC with the `model_version` and the set size — one line per
  model, every model on the same frozen jobs so the numbers compare. ROC AUC fits here because
  the label is binary and rare while the score is a continuous ranking: 0.5 is no ranking power,
  1.0 is perfect.

```bash
uv run --env-file ../.env python -m scripts.freeze_test_set           # writes data/frozen.json
uv run --env-file ../.env python -m scripts.evaluate                  # every model, side by side
uv run --env-file ../.env python -m scripts.evaluate minilm-l6-v2     # just one model
```

Current result on the frozen set:

```
tfidf-v1          ROC AUC=0.5086  n=75  pos=5
minilm-l6-v2      ROC AUC=0.4600  n=75  pos=5
jev-manual-qs-v1  ROC AUC=0.7200  n=75  pos=5
```

Neither raw cosine against the CV ranks the jobs you would apply to: TF-IDF sits at chance, the
MiniLM embedding below it. With only 5 positives in 75 the gap is within noise — the honest
reading is "no signal", not "TF-IDF beats embeddings". That is why Phase 4b turns to features
extracted from the job text (Jev) instead of raw similarity — and `jev-manual-qs-v1` (see below)
clears both by a wide margin, though still on a thin 5-positive sample.

## Jev (Phase 4b)

An LLM used as a **feature extractor**, never as the match judge: it answers a fixed set of
atomic, literal yes/no questions about the job posting, never about the job and the CV together.

- `jobradar/jev/questions.py` — the question set (`QUESTION_SET_VERSION = "qs-v1"`), each
  `Question` carrying `instructions` plus a `criteria_true`/`criteria_false` pair that is the
  annotation contract between a human and the Jev. v1 asks `open_to_latam`, `fully_remote`,
  `is_senior_role`.
- `jobradar/jev/client.py` — `Protocol JevClient` behind the base-URL-swappable provider; adapter
  #1 is `TypeSafeAdapter` (native SDK `system_one`). `jev_client()` returns `None` when
  `JEV_API_KEY` is unset, so the scorer falls back to the cosine score — a test proves this.
- `db/migrations/011_add_jev_answers.sql` — `jev_answers (job_id, question_set_version, answers
  jsonb)`, PK `(job_id, question_set_version)`.
- `scripts/jev_pilot.py` — runs the Jev on the frozen test set's jobs (boilerplate-cut text,
  `Title: ...\n\n...`), upserts the answers into `jev_answers`, and prints the real cost per 1000
  jobs (Jev bills input tokens only).
- `scripts/score_jev.py` — combines the 3 answers with manual weights (`WEIGHT`, chosen before
  ever looking at the frozen set's score — tuning them against it would be the same invariant
  violation as training on it) into one score per job, upserted into `scores` under
  `model_version = 'jev-manual-qs-v1'`.

```bash
uv run --group jev --env-file ../.env python -m scripts.jev_pilot  # fill jev_answers
uv run --env-file ../.env python -m scripts.score_jev              # write jev-manual-qs-v1 scores
```

Measured cost: **~$0.0524 per 1000 jobs** (75 jobs, 93539 input tokens). The question set's
answers aren't cross-checked against a full hand-labelled agreement measurement (visual spot
check accepted instead — see `docs/PROGRESS.md`, 2026-10-30); the `jev-manual-qs-v1` score is a
disposable diagnostic, not a `model_version` meant to survive into production — Phase 4c decides
whether and how the Jev features earn a permanent place. `jev_answers` doesn't record which
concrete Jev model answered (`JEV_MODEL` floats on `jev-latest`, debt D50).

## sklearn: the combined model (Phase 4c)

Three standalone signals exist by Phase 4b: `tfidf-v1`, `minilm-l6-v2` (both at or below chance —
Phase 4a) and `jev-manual-qs-v1` (0.72, manual weights). Phase 4c asks whether *learning* the
weights, over all the features at once (Jev answers + cosine + the code-extracted features from
Phase 3a), beats picking 3 numbers by hand.

- `scripts/build_features.py` — one feature row per labelled job (250), joining `jev_answers`,
  `scores` (`minilm-l6-v2`), and `job_skills` (aggregated to `skills_matched = SUM(confidence)` —
  one column, not one per skill; invariant §9, few labels mean few columns). `years_required` and
  `salary_usd_year` are computed on the fly (`jobradar/experience.py`, `jobradar/salary.py` — never
  persisted as columns before this). Writes `data/features.json`. The train/test split is decided
  once, by `data/frozen.json` — this file holds all 250 so that decision is never duplicated.
- `scripts/train_model.py` — the 175 training jobs (everything in `features.json` not in the
  frozen set) through a `Pipeline(SimpleImputer → StandardScaler → LogisticRegression)`.
  `GridSearchCV(scoring="roc_auc", cv=5)` picks `C` by cross-validation on the training fold only
  — the frozen set never influences this choice. Saves the whole fitted pipeline (imputer, scaler
  and model together, so scoring later reuses exactly what training learned) to
  `data/jev_sklearn_model.pkl`.
- `scripts/score_sklearn.py` — loads that pipeline, scores the 75 frozen jobs, writes `scores`
  under `model_version = 'sklearn-combined-v1'`.
- `scripts/explain_model.py` — the 3 largest-magnitude coefficients of the trained model.

```bash
uv run --env-file ../.env python -m scripts.build_features   # writes data/features.json
uv run --env-file ../.env python -m scripts.train_model      # writes data/jev_sklearn_model.pkl
uv run --env-file ../.env python -m scripts.score_sklearn     # writes sklearn-combined-v1 scores
uv run --env-file ../.env python -m scripts.explain_model     # top-3 coefficients
```

Result on the frozen set, all four approaches side by side:

```
sklearn-combined-v1  ROC AUC=0.6486  n=75  pos=5
jev-manual-qs-v1      ROC AUC=0.7200  n=75  pos=5
tfidf-v1               ROC AUC=0.5086  n=75  pos=5
minilm-l6-v2           ROC AUC=0.4600  n=75  pos=5
```

An honest, unexpected result: the learned combination scores **below** the 3 hand-picked Jev
weights. The 3 largest coefficients explain why — `open_to_latam` (+0.98) and `is_senior_role`
(−0.89) agree in direction with the manual weights (real signal), but the single largest,
`cosine_minilm` (+1.04), belongs to a feature with no standalone ranking power at all
(`minilm-l6-v2` alone scored 0.46, below chance, in Phase 4a). With 175 training jobs and a
handful of positives, `GridSearchCV` picked a weakly-regularized `C=100` — likely letting a noisy
feature pick up a coefficient it hasn't earned. The takeaway isn't "sklearn is worse than
hand-picked weights" in general; it's that this particular combination, at this sample size,
overfits a feature that was already known to carry no signal on its own.

## Production helpers

Each has type hints and tests (`tests/`):

- `jobradar/text.py` — turns a description's HTML into a list of plain-text paragraphs (repairing
  mojibake first).
- `jobradar/boilerplate.py` — counts paragraphs across descriptions and cuts the shared boilerplate.
- `jobradar/skills.py` — `skills_extractor(text, aliases)`: finds the dictionary's skills in a text,
  maps aliases to a canonical name, and returns a confidence per skill.
- `jobradar/salary.py` — `normalize_salary(...)`: converts an amount + currency + period to annual USD
  for comparison; returns `None` when the input is missing or the unit is unknown.
- `jobradar/experience.py` — `extract_years(text)`: estimates the required years of experience by
  regex on the cleaned description.
- `jobradar/embed.py` — `embed(texts)`: encodes texts into 384-dim vectors with `all-MiniLM-L6-v2`
  (Phase 4a; needs the `embed` group).

The always-on pipeline (skills + TF-IDF) lives in `scripts/main.py`; the embedding pipeline in
`scripts/embed_jobs.py` and `scripts/score_embeddings.py`.

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
