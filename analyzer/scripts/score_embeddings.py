import os
import sys
from pathlib import Path

import psycopg

from jobradar.embed import embed
from scripts.embed_jobs import MODEL_VERSION


def score_embeddings() -> int:
    cv_path = Path(__file__).resolve().parents[2] / "cv.md"
    cv_text = cv_path.read_text(encoding="utf-8")
    cv_vector = embed([cv_text])

    cv_pgvector = "[" + ",".join(map(str, cv_vector[0])) + "]"

    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1
    try:
        conn = psycopg.connect(database_url)
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to connect to the database: {detail}", file=sys.stderr)
        return 1

    conn.autocommit = True
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
        print(f"Inseri {cur.rowcount} linhas no scores")
    return 0


if __name__ == "__main__":
    sys.exit(score_embeddings())
