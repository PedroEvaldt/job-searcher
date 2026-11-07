"""Score every live job with a saved sklearn-combined version and write the scores.

    uv run --env-file ../.env python -m scripts.score [model_version]

Defaults to MODEL_VERSION. Idempotent: a second run rewrites the same scores. The frozen
test jobs are live too, so this also refreshes the scores scripts.evaluate reads.
"""

import sys

from jobradar.db import DatabaseError, connect
from jobradar.match import cv_skills
from jobradar.model.combined import (
    MODEL_VERSION,
    SPECS,
    load,
    model_path,
    score_jobs,
    write_scores,
)
from jobradar.model.features import fetch_live_jobs


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
    if not path.exists():
        print(f"no model at {path}: run scripts.train {version} first", file=sys.stderr)
        return 1
    pipeline = load(path)

    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1
    with conn:
        jobs = fetch_live_jobs(conn, spec.question_set)
        if not jobs:
            print(f"no live jobs answered with {spec.question_set}")
            return 0

        scores = score_jobs(spec, pipeline, jobs, cv_skills())

        write_scores(conn, version, jobs, scores)
    dropped = sum(score == 0.0 for score in scores)
    print(f"wrote {len(jobs)} rows to scores ({version}), {dropped} at 0 by its rules")
    return 0


if __name__ == "__main__":
    sys.exit(main())
