# Operations

job-radar runs with `docker compose` on a Raspberry Pi (arm64; Docker builds the images natively
there). Nothing in the code or the compose file is Pi-specific.

All commands run from the repository root. `psql` inside the database container:

```
docker compose exec postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

## First deploy

```
git clone https://github.com/PedroEvaldt/job-searcher.git
cd job-searcher
cp .env.example .env            # set a real POSTGRES_PASSWORD
cp cv.example.md cv.md          # your résumé
docker compose up -d --build
```

`postgres` always restarts and `app`, `jev-worker` and `digest` restart unless stopped by hand,
so the stack comes back on its own after a reboot or a power cut. Logs rotate at 10 MB × 3.

Optional services, both configured in `.env`:

- **Jev worker:** `JEV_API_KEY=...` and `COMPOSE_PROFILES=jev`. Without the profile the service
  does not exist; without the key it would exit 1 in a restart loop, hence the profile.
- **Discord:** `DISCORD_WEBHOOK_URL=...`. Without it the digest prints to its log.

## Updating

```
git pull
docker compose up -d --build
docker compose run --rm analyzer        # catch up skills and TF-IDF on the new jobs
```

Always `--build`: the images have fixed tags, so without it Compose runs the old binary.

Since 2026-11-07 the database container no longer has a fixed name (it was `postgres_db`). The
first `up` after that change recreates it as `job-searcher-postgres-1` on the **same** volume;
use `docker compose exec postgres` instead of the old name.

## Moving a database from `postgres:16-alpine` to `pgvector/pgvector:pg16`

The stack uses the pgvector image (Debian, glibc) since the embeddings were added. A data volume
created by the old `postgres:16-alpine` image (musl) can be mounted by the new one, but the two C
libraries sort text differently: every index on a text column is silently wrong, lookups miss
rows that are there, and the collector starts writing jobs twice **without any error** (its
`ON CONFLICT` no longer finds the original). Before the new image serves any write:

```
docker compose stop app digest jev-worker      # nothing writes while indexes are wrong
docker compose up -d postgres                  # now on pgvector/pgvector:pg16
docker compose exec postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "REINDEX DATABASE \"$POSTGRES_DB\";" \
  -c "ALTER DATABASE \"$POSTGRES_DB\" REFRESH COLLATION VERSION;"'
```

Then check that no job identity is duplicated, reading the table without indexes:

```
docker compose exec postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "SET enable_indexscan = off; SET enable_bitmapscan = off;" \
  -c "SELECT source_id, external_id, count(*) FROM jobs GROUP BY 1, 2 HAVING count(*) > 1;"'
```

Zero rows: start everything with `docker compose up -d --build`. The cleaner alternative is a
dump of the old database restored into a fresh volume (below): the indexes are then built under
glibc from the start.

## Backups

The labels are the one thing that cannot be recollected: they are hours of human judgement.
Dumps go to `db_dump/`, which git ignores.

```
# the whole database (custom format)
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > db_dump/jobs-$(date +%F).dump

# just the labels, as plain SQL
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --data-only -t labels' \
  > db_dump/labels-$(date +%F).sql
```

Restore a full dump into the running database (replaces what is there):

```
docker compose exec -T postgres sh -c 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner' \
  < db_dump/jobs-YYYY-MM-DD.dump
```

## Health

- `docker compose logs -f app`: one `source collected` line per source per pass; a `source is
  broken` warning after 2 failed passes in a row.
- `db/queries/health.sql`: runs and errors per source per day and the longest gap between runs.
- `go run ./cmd/radar status`: how many live jobs are labelled.

## Maintenance commands

| Command | When |
| --- | --- |
| `docker compose run --rm app ./collector -renormalize` | a source parser changed: rebuild every `jobs` row from `raw_jobs` |
| `make gstatus` / `make gup` / `make gdown` | inspect or roll back migrations by hand (the collector applies them on its own) |
| `cd analyzer && uv run --env-file ../.env python -m scripts.detect_language` | tag the language of new jobs |
| `cd analyzer && uv run --env-file ../.env python -m scripts.score_jev` | recompute `jev-manual-qs-v1` from the stored answers (no Jev call) |
