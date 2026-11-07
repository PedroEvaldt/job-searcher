"""Train sklearn-combined on every labelled job outside the frozen test set.

    uv run --env-file ../.env python -m scripts.train

Prints the cross-validated average precision (the honest number with few labels) and
the coefficients, then saves the model under data/models/. Refuses to overwrite a
model that already exists: retraining changes the scores, so it needs a new
MODEL_VERSION in jobradar/model/combined.py.
"""

import sys

import numpy as np

from jobradar.db import DatabaseError, connect
from jobradar.frozen import frozen_ids
from jobradar.match import cv_skills
from jobradar.model.combined import (
    MODEL_VERSION,
    coefficients,
    model_path,
    save,
    train,
)
from jobradar.model.features import (
    feature_matrix,
    features_for_job,
    fetch_labelled_jobs,
)


def main() -> int:
    path = model_path()
    if path.exists():
        print(
            f"{path} already exists. Retraining {MODEL_VERSION} would change its "
            "scores under the same name: bump MODEL_VERSION in "
            "jobradar/model/combined.py first.",
            file=sys.stderr,
        )
        return 1

    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1
    with conn:
        labelled = fetch_labelled_jobs(conn)

    # The frozen test set never takes part in training, not even in choosing C.
    test_ids = set(frozen_ids())
    training = [(job, label) for job, label in labelled if job.job_id not in test_ids]
    if not training:
        print("no labelled jobs to train on", file=sys.stderr)
        return 1

    cv = cv_skills()
    x = feature_matrix([features_for_job(job, cv) for job, _ in training])
    y = np.array([label for _, label in training], dtype=float)
    result = train(x, y)

    print(f"{MODEL_VERSION}: trained on {result.n} jobs, {result.n_pos} positive")
    print(f"best C: {result.best_c}")
    print("CV AP per fold:", " ".join(f"{ap:.4f}" for ap in result.fold_ap))
    print(f"CV AP: {np.mean(result.fold_ap):.4f} ± {np.std(result.fold_ap):.4f}")
    print(f"random AP (prevalence): {result.prevalence:.4f}")
    print("coefficients (scaled features):")
    for name, weight in coefficients(result.pipeline):
        print(f"  {name:<16} {weight:+.3f}")

    save(result.pipeline, path)
    print(f"saved {path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
