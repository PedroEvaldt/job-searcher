"""Score every embedded job by its cosine similarity to the CV: the minilm-l6-v2 baseline.

    uv run --env-file ../.env --group embed python -m scripts.score_embeddings

Run after scripts.embed_jobs: only jobs that have a vector get a score.
"""

import sys

from jobradar.db import DatabaseError, connect
from jobradar.embed import embed
from jobradar.match import CV_PATH
from scripts.embed_jobs import MODEL_VERSION


def score_embeddings() -> int:
    cv_text = CV_PATH.read_text(encoding="utf-8")
    cv_vector = embed([cv_text])

    cv_pgvector = "[" + ",".join(map(str, cv_vector[0])) + "]"

    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1
    with conn, conn.cursor() as cur:
        cur.execute(
            """
        INSERT INTO scores (job_id, model_version, score)
        SELECT job_id, model_version, 1 - (embedding <=> %s::vector) AS score FROM job_embeddings WHERE model_version = %s
        ON CONFLICT (job_id, model_version) DO UPDATE SET score=EXCLUDED.score, computed_at=now();
        """,
            (cv_pgvector, MODEL_VERSION),
        )
        if not cur.rowcount:
            print("no embeddings to score")
            return 0
        print(f"wrote {cur.rowcount} rows to scores ({MODEL_VERSION})")
    return 0


if __name__ == "__main__":
    sys.exit(score_embeddings())
