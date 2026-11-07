"""Answer a question set for the live jobs that do not have its answers yet.

    uv run --group jev --env-file ../.env python -m scripts.answer_jev qs-v2
    uv run --group jev --env-file ../.env python -m scripts.answer_jev qs-v2 --training-labels

For trying a new question set before the worker switches to it. With
--training-labels it only answers the labelled jobs outside the frozen test set: a
question is written and checked on those, never on the test set. Writes jev_answers
only, no score.
"""

import argparse
import sys

from jobradar.db import DatabaseError, connect
from jobradar.frozen import frozen_ids
from jobradar.jev.client import jev_client
from jobradar.jev.pending import answer_pending_jobs
from jobradar.jev.questions import QUESTION_SETS


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("version", choices=sorted(QUESTION_SETS))
    parser.add_argument(
        "--training-labels",
        action="store_true",
        help="only the labelled jobs outside the frozen test set",
    )
    parser.add_argument(
        "--workers", type=int, default=1, help="parallel Jev calls (default 1)"
    )
    args = parser.parse_args()

    jev = jev_client()
    if jev is None:
        print("no JEV_API_KEY set, nothing to run", file=sys.stderr)
        return 1
    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1

    with conn:
        only_ids = None
        if args.training_labels:
            test_ids = set(frozen_ids())
            labelled = conn.execute("SELECT job_id FROM labels;").fetchall()
            only_ids = [job_id for (job_id,) in labelled if job_id not in test_ids]
        total_jobs, total_cost = answer_pending_jobs(
            conn, jev, args.version, only_ids=only_ids, workers=args.workers
        )
    print(f"done: {total_jobs} jobs answered, ${total_cost:.6f} total ({args.version})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
