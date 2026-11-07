"""Embed every live job that has no minilm-l6-v2 vector yet into job_embeddings.

    uv run --env-file ../.env --group embed python -m scripts.embed_jobs

A baseline (docs/MODELING.md): the digest does not use embeddings. The embed group pulls in
torch, so it stays out of the images.
"""

import sys

from jobradar.boilerplate import clean_descriptions
from jobradar.db import DatabaseError, connect
from jobradar.embed import embed

MODEL_VERSION = "minilm-l6-v2"


def embed_jobs() -> int:
    """Embed every live job that has no `MODEL_VERSION` vector yet and store it.

    Kept out of the always-on `main.py` on purpose: loading the model pulls in torch.
    Resumable and idempotent — a second run embeds only what is still missing, and the
    ON CONFLICT upsert means re-embedding a job overwrites its row, never duplicates it.
    """
    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1
    with conn, conn.cursor() as cur:
        # The whole live set: clean_descriptions builds the boilerplate statistic over
        # it, so the cut does not change with the pending subset.
        corpus = [
            row[0]
            for row in cur.execute(
                "SELECT description FROM jobs WHERE duplicate_of IS NULL;"
            ).fetchall()
        ]
        # Live jobs with no vector for this model yet: anti-join on the pair that is
        # job_embeddings' primary key (LEFT JOIN ... WHERE the right side is missing).
        rows = cur.execute(
            "SELECT j.id, j.description FROM jobs j "
            "LEFT JOIN job_embeddings e "
            "ON e.job_id = j.id AND e.model_version = %s "
            "WHERE j.duplicate_of IS NULL AND e.job_id IS NULL "
            "ORDER BY j.id;",
            (MODEL_VERSION,),
        ).fetchall()
        if not rows:
            print("no jobs to embed")
            return 0

        job_ids = [job_id for job_id, _ in rows]
        descriptions = [description for _, description in rows]
        vectors = embed(clean_descriptions(corpus, descriptions))

        with conn.transaction():
            cur.executemany(
                "INSERT INTO job_embeddings (job_id, model_version, embedding) "
                "VALUES (%s, %s, %s::vector) "
                "ON CONFLICT (job_id, model_version) DO UPDATE "
                "SET embedding = EXCLUDED.embedding, computed_at = now();",
                [
                    (job_id, MODEL_VERSION, "[" + ",".join(map(str, vector)) + "]")
                    for job_id, vector in zip(job_ids, vectors)
                ],
            )
    print(f"embedded {len(job_ids)} jobs as {MODEL_VERSION}")
    return 0


if __name__ == "__main__":
    sys.exit(embed_jobs())
