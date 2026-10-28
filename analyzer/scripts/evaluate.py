import json
import os
import sys

import psycopg
from sklearn.metrics import roc_auc_score

from scripts.freeze_test_set import OUTPUT_PATH

MODELS = ["tfidf-v1", "minilm-l6-v2"]


def evaluate() -> int:
    """Report the ROC AUC of each model_version on the frozen test set, side by side.

    Model versions come from the command line; with none given, both `MODELS` are
    measured. Every model is scored on the same frozen job ids, so the numbers compare.
    """
    data = json.loads(OUTPUT_PATH.read_text(encoding="utf-8"))
    labels_by_job: dict[int, bool] = {
        d["job_id"]: d["would_apply"] for d in data["test_set"]
    }
    ids = list(labels_by_job)
    labels = [labels_by_job[job_id] for job_id in ids]

    db_url = os.getenv("DATABASE_URL")
    if db_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1

    models = sys.argv[1:] or MODELS
    results: list[tuple[str, float]] = []
    try:
        with psycopg.connect(db_url) as conn:
            for model in models:
                scores_by_job = fetch_scores(conn, model, ids)
                missing = [job_id for job_id in ids if job_id not in scores_by_job]
                if missing:
                    print(f"no {model} score for jobs: {missing}", file=sys.stderr)
                    return 1
                scores = [scores_by_job[job_id] for job_id in ids]
                results.append((model, float(roc_auc_score(labels, scores))))
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to connect to the database: {detail}", file=sys.stderr)
        return 1

    # Pad the model name to the widest one so the columns line up
    width = max(len(model) for model, _ in results)
    for model, auc in results:
        print(
            f"{model:<{width}}  ROC AUC={auc:.4f}  "
            f"n={data['n_test']}  pos={data['n_test_positive']}"
        )
    return 0


def fetch_scores(
    conn: psycopg.Connection, model_version: str, ids: list[int]
) -> dict[int, float]:
    """The frozen jobs' scores for one model_version, as {job_id: score}."""
    rows = conn.execute(
        "SELECT job_id, score FROM scores "
        "WHERE model_version = %s AND job_id = ANY(%s);",
        (model_version, ids),
    ).fetchall()
    return {job_id: score for job_id, score in rows}


if __name__ == "__main__":
    sys.exit(evaluate())
