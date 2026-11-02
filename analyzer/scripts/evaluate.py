import json
import os
import sys

import psycopg
from sklearn.metrics import average_precision_score, roc_auc_score

from jobradar.frozen import FROZEN_PATH

MODELS = ["jev-manual-qs-v1"]


def evaluate() -> int:
    """Report each model_version on the frozen test set, side by side.

    Average Precision is the deciding metric (5.4): it weighs the hits at the top,
    which is all the digest shows. ROC AUC sits next to it only to compare with the
    3b–4c numbers. Model versions come from the command line; with none given,
    `MODELS` is measured. Every model is scored on the same labelled frozen ids.
    """
    data = json.loads(FROZEN_PATH.read_text(encoding="utf-8"))
    frozen_ids: list[int] = [d["job_id"] for d in data["test_set"]]

    db_url = os.getenv("DATABASE_URL")
    if db_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1

    models = sys.argv[1:] or MODELS
    results: list[tuple[str, float, float]] = []
    try:
        with psycopg.connect(db_url) as conn:
            labels_by_job = fetch_labels(conn, frozen_ids)
            unlabelled = len(frozen_ids) - len(labels_by_job)
            if unlabelled:
                # Measuring on a partial test set is allowed but must be visible:
                # the numbers change once the rest is labelled.
                print(
                    f"warning: {unlabelled} of {len(frozen_ids)} test jobs not "
                    "labelled yet — measuring on the labelled ones only",
                    file=sys.stderr,
                )
            ids = sorted(labels_by_job)
            labels = [labels_by_job[job_id] for job_id in ids]
            if not any(labels) or all(labels):
                print("the labelled test jobs are all one class", file=sys.stderr)
                return 1

            for model in models:
                scores_by_job = fetch_scores(conn, model, ids)
                missing = [job_id for job_id in ids if job_id not in scores_by_job]
                if missing:
                    print(f"no {model} score for jobs: {missing}", file=sys.stderr)
                    return 1
                scores = [scores_by_job[job_id] for job_id in ids]
                results.append(
                    (
                        model,
                        float(average_precision_score(labels, scores)),
                        float(roc_auc_score(labels, scores)),
                    )
                )
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to connect to the database: {detail}", file=sys.stderr)
        return 1

    n_pos = sum(labels)
    # AP of a random ranking equals the positive rate: the floor to beat.
    print(
        f"{data['version']}: n={len(ids)} pos={n_pos} (random AP={n_pos / len(ids):.4f})"
    )
    width = max(len(model) for model, _, _ in results)
    for model, ap, auc in results:
        print(f"{model:<{width}}  AP={ap:.4f}  ROC AUC={auc:.4f}")
    return 0


def fetch_labels(conn: psycopg.Connection, ids: list[int]) -> dict[int, bool]:
    """would_apply of the frozen jobs that already have a label, as {job_id: label}."""
    rows = conn.execute(
        "SELECT job_id, would_apply FROM labels WHERE job_id = ANY(%s);", (ids,)
    ).fetchall()
    return {job_id: would_apply for job_id, would_apply in rows}


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
