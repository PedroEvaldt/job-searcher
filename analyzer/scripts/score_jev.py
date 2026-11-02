import json
import os
import sys

import psycopg

from jobradar.frozen import FROZEN_PATH as OUTPUT_PATH
from jobradar.jev.questions import QUESTION_SET_VERSION

WEIGHT = {"fully_remote": 0.7, "open_to_latam": 0.6, "is_senior_role": -0.7}

MODEL_VERSION = f"jev-manual-{QUESTION_SET_VERSION}"


def get_score(jev_norm: dict[str, float]) -> float:
    weight: float = 0
    for cat, value in jev_norm.items():
        weight += value * WEIGHT[cat]
    return weight


def score_jev() -> int:
    """Combine the Jev answers with manual weights and write the result to scores."""
    data = json.loads(OUTPUT_PATH.read_text(encoding="utf-8"))
    ids = [d["job_id"] for d in data["test_set"]]

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
        rows = cur.execute(
            "SELECT job_id, answers FROM jev_answers "
            "WHERE job_id = ANY(%s) AND question_set_version = %s "
            "ORDER BY job_id;",
            (ids, QUESTION_SET_VERSION),
        ).fetchall()
        if not rows:
            print("no jev answers to score")
            return 0
        job_ids = [job_id for job_id, _ in rows]
        scores = [get_score(answer) for _, answer in rows]
        with conn.transaction():
            cur.executemany(
                """INSERT INTO scores (job_id, model_version, score)
                VALUES (%s, %s, %s)
                ON CONFLICT (job_id, model_version) DO UPDATE
                SET score = EXCLUDED.score, computed_at = now();""",
                [
                    (job_id, MODEL_VERSION, score)
                    for job_id, score in zip(job_ids, scores)
                ],
            )
        print(f"wrote {len(rows)} rows to scores ({MODEL_VERSION})")
    return 0


if __name__ == "__main__":
    sys.exit(score_jev())
