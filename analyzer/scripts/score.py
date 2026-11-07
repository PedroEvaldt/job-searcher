"""Score every live job with sklearn-combined and write the scores.

    uv run --env-file ../.env python -m scripts.score

Idempotent: a second run rewrites the same scores. The frozen test jobs are live too,
so this also refreshes the scores scripts.evaluate reads.
"""

import sys

from jobradar.db import DatabaseError, connect
from jobradar.match import cv_skills
from jobradar.model.combined import MODEL_VERSION, load, model_path, predict
from jobradar.model.features import feature_matrix, features_for_job, fetch_live_jobs


def main() -> int:
    path = model_path()
    if not path.exists():
        print(f"no model at {path}: run scripts.train first", file=sys.stderr)
        return 1
    pipeline = load(path)

    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1
    with conn:
        jobs = fetch_live_jobs(conn)
        if not jobs:
            print("no live jobs to score")
            return 0

        cv = cv_skills()
        probs = predict(
            pipeline, feature_matrix([features_for_job(j, cv) for j in jobs])
        )

        # One transaction: the digest never ranks a mix of old and new scores.
        with conn.transaction(), conn.cursor() as cur:
            cur.executemany(
                """
                INSERT INTO scores (job_id, model_version, score)
                VALUES (%s, %s, %s)
                ON CONFLICT (job_id, model_version) DO UPDATE
                SET score = EXCLUDED.score, computed_at = now();
                """,
                [
                    (job.job_id, MODEL_VERSION, float(prob))
                    for job, prob in zip(jobs, probs, strict=True)
                ],
            )
    print(f"wrote {len(jobs)} rows to scores ({MODEL_VERSION})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
