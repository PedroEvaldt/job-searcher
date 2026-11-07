"""Measure model versions on the frozen test set, side by side.

    uv run --env-file ../.env python -m scripts.evaluate [model_version ...]

With no argument, measures the digest's current model and the combined model. Every
model is scored on the same labelled frozen jobs, so the numbers compare directly.

Average precision decides: it weighs the hits at the top of the ranking, which is all
the digest shows. ROC AUC is there as a sanity check, and P@10 is the share of good
jobs among the 10 best-ranked test jobs. With only a handful of positives in the test
set, one job moves these numbers a lot: read them next to the cross-validated AP that
scripts.train prints.
"""

import sys

import psycopg
from sklearn.metrics import average_precision_score, roc_auc_score

from jobradar.db import DatabaseError, connect
from jobradar.frozen import frozen_ids, frozen_version
from jobradar.jev.manual import MODEL_VERSION as JEV_MANUAL
from jobradar.model.combined import MODEL_VERSION as COMBINED

DEFAULT_MODELS = [JEV_MANUAL, COMBINED]
TOP_K = 10


def main() -> int:
    models = sys.argv[1:] or DEFAULT_MODELS
    test_ids = frozen_ids()

    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1

    results: list[tuple[str, float, float, float]] = []
    with conn:
        labels_by_job = fetch_labels(conn, test_ids)
        unlabelled = len(test_ids) - len(labels_by_job)
        if unlabelled:
            # Measuring on a partial test set is allowed but must be visible: the
            # numbers change once the rest is labelled.
            print(
                f"warning: {unlabelled} of {len(test_ids)} test jobs not labelled "
                "yet, measuring on the labelled ones only",
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
                    precision_at_k(labels, scores, TOP_K),
                )
            )

    n_pos = sum(labels)
    # The AP of a random ranking equals the positive rate: the floor to beat.
    print(
        f"{frozen_version()}: n={len(ids)} pos={n_pos} "
        f"(random AP={n_pos / len(ids):.4f})"
    )
    width = max(len(model) for model, *_ in results)
    for model, ap, auc, p_at_k in results:
        print(
            f"{model:<{width}}  AP={ap:.4f}  ROC AUC={auc:.4f}  P@{TOP_K}={p_at_k:.2f}"
        )
    return 0


def precision_at_k(labels: list[bool], scores: list[float], k: int) -> float:
    """Share of positives among the k best-scored jobs (ties broken by input order)."""
    ranked = sorted(range(len(scores)), key=lambda i: -scores[i])[:k]
    return sum(labels[i] for i in ranked) / len(ranked)


def fetch_labels(conn: psycopg.Connection, ids: list[int]) -> dict[int, bool]:
    """would_apply of the given jobs that already have a label, as {job_id: label}."""
    rows = conn.execute(
        "SELECT job_id, would_apply FROM labels WHERE job_id = ANY(%s);", (ids,)
    ).fetchall()
    return {job_id: would_apply for job_id, would_apply in rows}


def fetch_scores(
    conn: psycopg.Connection, model_version: str, ids: list[int]
) -> dict[int, float]:
    """The given jobs' scores for one model_version, as {job_id: score}."""
    rows = conn.execute(
        "SELECT job_id, score FROM scores WHERE model_version = %s AND job_id = ANY(%s);",
        (model_version, ids),
    ).fetchall()
    return {job_id: score for job_id, score in rows}


if __name__ == "__main__":
    sys.exit(main())
