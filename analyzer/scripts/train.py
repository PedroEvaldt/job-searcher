"""Train a sklearn-combined version on every labelled job outside the frozen test set.

    uv run --env-file ../.env python -m scripts.train [model_version]

Defaults to MODEL_VERSION (jobradar/model/combined.py). Prints the cross-validated
average precision (the honest number with few labels) and the coefficients, then saves
the model under data/models/. Refuses to overwrite a model that already exists:
retraining changes the scores, so it needs a new version in SPECS.
"""

import sys

import numpy as np

from jobradar.db import DatabaseError, connect
from jobradar.frozen import frozen_ids
from jobradar.match import cv_skills
from jobradar.model.combined import (
    MODEL_VERSION,
    SPECS,
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
    version = sys.argv[1] if len(sys.argv) > 1 else MODEL_VERSION
    if version not in SPECS:
        print(
            f"unknown model version {version!r}; known: {sorted(SPECS)}",
            file=sys.stderr,
        )
        return 1
    spec = SPECS[version]
    path = model_path(version)
    if path.exists():
        print(
            f"{path} already exists. Retraining {version} would change its scores "
            "under the same name: add a new version to SPECS in "
            "jobradar/model/combined.py instead.",
            file=sys.stderr,
        )
        return 1

    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1
    with conn:
        labelled = fetch_labelled_jobs(conn, spec.question_set)

    # The frozen test set never takes part in training, not even in choosing C.
    test_ids = set(frozen_ids())
    training = [(job, label) for job, label in labelled if job.job_id not in test_ids]
    if not training:
        print("no labelled jobs to train on", file=sys.stderr)
        return 1

    cv = cv_skills()
    x = feature_matrix(
        [features_for_job(job, cv) for job, _ in training], spec.features
    )
    y = np.array([label for _, label in training], dtype=float)
    result = train(x, y)

    print(f"{version}: trained on {result.n} jobs, {result.n_pos} positive")
    print(f"best C: {result.best_c}")
    print("CV AP per fold:", " ".join(f"{ap:.4f}" for ap in result.fold_ap))
    print(f"CV AP: {np.mean(result.fold_ap):.4f} ± {np.std(result.fold_ap):.4f}")
    print(f"random AP (prevalence): {result.prevalence:.4f}")
    print("coefficients (scaled features):")
    for name, weight in coefficients(result.pipeline, spec.features):
        print(f"  {name:<16} {weight:+.3f}")

    save(result.pipeline, path)
    print(f"saved {path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
