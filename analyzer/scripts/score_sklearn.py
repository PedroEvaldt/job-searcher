import json
import os
import sys

import joblib
import numpy as np
import psycopg

from scripts.build_features import OUTPUT_PATH
from scripts.freeze_test_set import OUTPUT_PATH as FREEZE_PATH
from scripts.train_model import COLS, MODEL_PATH


def score_sklearn() -> int:
    pipeline = joblib.load(MODEL_PATH)
    try:
        with open(OUTPUT_PATH, encoding="utf-8") as f:
            full_set = json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        print(f"could not open {OUTPUT_PATH}: {e}")
        return 1
    try:
        with open(FREEZE_PATH, encoding="utf-8") as f:
            data = json.load(f)
        test_ids = [d["job_id"] for d in data["test_set"]]
    except (OSError, json.JSONDecodeError, KeyError) as e:
        print(f"could not open {FREEZE_PATH}: {e}")
        return 1

    test_rows = [job for job in full_set if job["job_id"] in test_ids]
    for row in test_rows:
        row["has_salary"] = row["salary_usd_year"] is not None

    test_matrix = np.array([[row[c] for c in COLS] for row in test_rows], dtype=float)
    x_test_set = test_matrix[:, :-1]

    probs = pipeline.predict_proba(x_test_set)[:, 1]
    test_ids = [test_row["job_id"] for test_row in test_rows]

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
        cur.executemany(
            """
                INSERT INTO scores (job_id, model_version, score)
                VALUES (%s, %s, %s)
                ON CONFLICT (job_id, model_version) DO UPDATE
                SET score = EXCLUDED.score, computed_at = now();
                """,
            [
                (job_id, "sklearn-combined-v1", float(score))
                for job_id, score in zip(test_ids, probs)
            ],
        )
    print(f"wrote {len(test_ids)} rows to scores (sklearn-combined-v1)")
    return 0


if __name__ == "__main__":
    sys.exit(score_sklearn())
