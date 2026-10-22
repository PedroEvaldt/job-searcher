# analyzer

The Python side of [job-radar](../README.md). It reads the Postgres that the Go `collector`
fills and, from Phase 3a on, will score each job against the résumé. Today it holds the
exploratory data analysis (EDA) of the collected jobs and the first production helpers
(`text.py`, `boilerplate.py`).

Go and Python never share memory or files — only the Postgres schema. See the root README
for the whole picture.

## Requirements

- [uv](https://docs.astral.sh/uv/)
- A running Postgres with the collected data. In the compose stack it is published on
  `localhost:5433`; point `DATABASE_URL` at it.

## Setup

```bash
uv sync          # runtime deps only (psycopg)
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
uv run --env-file ../.env python main.py
```

Today it prints how many jobs are still waiting to be scored (`scored_at IS NULL`). Real
scoring arrives in Phase 3a.

## The EDA bench

The exploration is a set of scripts under `eda/`, written as `# %%` cells and meant for an
IPython REPL (`uv run --group eda ipython`, then `%run eda/01_overview.py`). Each one also
runs straight through:

```bash
uv run --group eda --env-file ../.env python eda/01_overview.py
```

| Script                 | What it shows                                                  |
| ---------------------- | -------------------------------------------------------------- |
| `db.py`                | `sql_to_dataframe(sql)` — the shared SQL → DataFrame loader     |
| `01_overview.py`       | shape, dtypes, jobs per source, description sizes, jobs per day |
| `02_text_defects.py`   | HTML, HTML entities and mojibake per source (defects 1–3)       |
| `03_payload_fields.py` | fields the collector drops; sentinels (defects 4–6)            |
| `04_boilerplate.py`    | paragraphs repeated across jobs, and how much text they take    |
| `05_location.py`       | each live job as accept/reject/unknown for a Brazil resident    |
| `06_language.py`       | language guess per description; the non-English share           |

The findings are written up in [`../docs/eda.md`](../docs/eda.md).

## Production helpers

- `text.py` — turns a description's HTML into a list of plain-text paragraphs.
- `boilerplate.py` — counts paragraphs across descriptions and cuts the boilerplate.

These have type hints and tests; the `eda/` scripts are throwaway exploration and do not.

## Checks

```bash
uv run --group dev pytest                                     # tests (tests/)
uv run ruff check .                                           # lint
uv run --group dev mypy --strict text.py boilerplate.py tests/
```

`mypy --strict` covers the production modules. The `eda/` scripts import pandas and
matplotlib, which ship no type stubs, so they are not kept under strict typing.
