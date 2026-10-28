import json
import os
import sys
from pathlib import Path

import psycopg
from sklearn.model_selection import train_test_split

RANDOM_STATE = 7
TEST_SIZE = 0.3
OUTPUT_PATH = Path(__file__).parents[1] / "data" / "frozen.json"


def freeze_test_set() -> int:
    db_url = os.getenv("DATABASE_URL")
    if db_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1
    try:
        with psycopg.connect(db_url) as conn:
            rows = conn.execute(
                "SELECT job_id, would_apply FROM labels ORDER BY job_id;"
            ).fetchall()
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to connect to the database: {detail}", file=sys.stderr)
        return 1

    jobs_labels: dict[int, bool] = {job_id: would_apply for job_id, would_apply in rows}
    job_ids = list(jobs_labels.keys())
    labels = list(jobs_labels.values())

    # stratify keeps the positive/negative ratio in both sides; the frozen set is
    # the test half only (train is derived later as "everything not in here").
    _train_ids, test_ids = train_test_split(
        job_ids,
        test_size=TEST_SIZE,
        stratify=labels,
        random_state=RANDOM_STATE,
    )

    test_ids = sorted(test_ids)
    n_test_positive = sum(1 for job_id in test_ids if jobs_labels[job_id])
    record = {
        "version": "test-set-v1",
        "random_state": RANDOM_STATE,
        "test_size": TEST_SIZE,
        "n_total": len(job_ids),
        "n_test": len(test_ids),
        "n_test_positive": n_test_positive,
        "n_test_negative": len(test_ids) - n_test_positive,
        "test_set": [
            {"job_id": job_id, "would_apply": jobs_labels[job_id]}
            for job_id in test_ids
        ],
    }
    with OUTPUT_PATH.open("w") as f:
        json.dump(record, f, indent=2)
    return 0


if __name__ == "__main__":
    sys.exit(freeze_test_set())
