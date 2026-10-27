import json
import os
import sys

import psycopg
from sklearn.metrics import roc_auc_score

from freeze_test_set import OUTPUT_PATH

MODEL_VERSION = "tfidf-v1"


def evaluate_baseline() -> int:
    data = json.loads(OUTPUT_PATH.read_text(encoding="utf-8"))
    jobs_labels: dict[int, bool] = {
        d["job_id"]: d["would_apply"] for d in data["test_set"]
    }
    ids = list(jobs_labels)

    db_url = os.getenv("DATABASE_URL")
    if db_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1
    try:
        with psycopg.connect(db_url) as conn:
            rows = conn.execute(
                "SELECT job_id, score FROM scores "
                "WHERE model_version = %s AND job_id = ANY(%s);",
                (MODEL_VERSION, ids),
            ).fetchall()
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to connect to the database: {detail}", file=sys.stderr)
        return 1

    jobs_scores: dict[int, float] = {job_id: score for job_id, score in rows}
    missing = [job_id for job_id in ids if job_id not in jobs_scores]
    if missing:
        print(f"no {MODEL_VERSION} score for jobs: {missing}", file=sys.stderr)
        return 1

    labels = [jobs_labels[job_id] for job_id in ids]
    scores = [jobs_scores[job_id] for job_id in ids]
    metric = roc_auc_score(labels, scores)
    print(
        f"{MODEL_VERSION} ROC AUC={metric:.4f} "
        f"n={data['n_test']} pos={data['n_test_positive']}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(evaluate_baseline())
