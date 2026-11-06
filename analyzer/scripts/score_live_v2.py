"""Score every live job with sklearn-combined-v2, so the digest can rank them all.

score_sklearn_v2 only scores the frozen test set (to measure the model); this one
scores everything the digest can send. The test jobs are live too, so their scores
are rewritten here: if scripts.evaluate gives a different AP afterwards, the features
were computed differently from training.
"""

import os
import sys

import joblib
import numpy as np
import psycopg

from jobradar.jev.questions import QUESTION_SET_VERSION
from jobradar.match import CV_PATH, cv_skills
from scripts.build_features_v2 import features_for_job
from scripts.score_sklearn_v2 import MODEL_VERSION
from scripts.train_model_v2 import COLS, MODEL_PATH

# COLS ends with the label (would_apply), which an unlabelled job does not have.
FEATURE_COLS = COLS[:-1]


def score_live() -> int:
    try:
        pipeline = joblib.load(MODEL_PATH)
    except OSError as e:
        print(f"could not open {MODEL_PATH}: {e}", file=sys.stderr)
        return 1

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
        # Same query as build_features_v2, minus the labels: every live job, labelled
        # or not. A job the Jev has not answered yet has no features, so no score.
        rows = cur.execute(
            """
                SELECT j.id, j.description, je.answers AS jev_score, sk.skills
                FROM jobs j
                JOIN jev_answers je ON je.job_id = j.id
                    AND je.question_set_version = %s
                LEFT JOIN (
                    SELECT job_id, array_agg(skill) AS skills
                    FROM job_skills
                    GROUP BY job_id
                ) sk ON sk.job_id = j.id
                WHERE j.duplicate_of IS NULL
                ORDER BY j.id;
                """,
            (QUESTION_SET_VERSION,),
        ).fetchall()
        if not rows:
            print("no live jobs to score")
            return 0

        cv_skill = cv_skills(CV_PATH)
        job_ids = [job_id for job_id, _, _, _ in rows]
        features = [
            features_for_job(desc, jev_score, skills, cv_skill)
            for _, desc, jev_score, skills in rows
        ]
        # Same column order as training; None (no years stated) becomes NaN, which
        # the pipeline's imputer fills exactly as it did in training.
        matrix = np.array([[f[c] for c in FEATURE_COLS] for f in features], dtype=float)
        probs = pipeline.predict_proba(matrix)[:, 1]

        # One transaction: either every score is updated or none is, so the digest
        # never ranks a mix of old and new scores.
        with conn.transaction():
            cur.executemany(
                """
                    INSERT INTO scores (job_id, model_version, score)
                    VALUES (%s, %s, %s)
                    ON CONFLICT (job_id, model_version) DO UPDATE
                    SET score = EXCLUDED.score, computed_at = now();
                    """,
                [
                    (job_id, MODEL_VERSION, float(prob))
                    for job_id, prob in zip(job_ids, probs)
                ],
            )
    print(f"wrote {len(job_ids)} rows to scores ({MODEL_VERSION})")
    return 0


if __name__ == "__main__":
    sys.exit(score_live())
