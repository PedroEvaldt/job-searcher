"""Round 1 of the label expansion (5.4): draw 200 random live jobs, split 75 to the
test set and 125 to train, and freeze both lists BEFORE any of them is labelled.

Writes two files under data/:
- test_set_v2.json: the 75 test-set-v1 jobs plus the 75 new ones. Ids only — the
  labels live in the labels table, read at evaluation time.
- label_queue_round1.json: the 200 new ids, in the order `radar label` shows them.
  Test and train are shuffled together, so while labelling you cannot tell which
  is which.

Refuses to run twice: a test set is frozen once, or it stops being a test set.
"""

import json
import os
import random
import sys
from pathlib import Path

import psycopg

RANDOM_STATE = 20261102
N_SAMPLE = 200
N_NEW_TEST = 75

DATA_DIR = Path(__file__).parents[1] / "data"
FROZEN_V1_PATH = DATA_DIR / "frozen.json"
TEST_V2_PATH = DATA_DIR / "test_set_v2.json"
QUEUE_PATH = DATA_DIR / "label_queue_round1.json"


def sample_label_round() -> int:
    if TEST_V2_PATH.exists() or QUEUE_PATH.exists():
        print(
            f"{TEST_V2_PATH.name} or {QUEUE_PATH.name} already exists — the round is "
            "frozen; delete them by hand only if no job in them was labelled yet",
            file=sys.stderr,
        )
        return 1

    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1
    try:
        with psycopg.connect(database_url) as conn:
            rows = conn.execute(
                """SELECT j.id FROM jobs j
                LEFT JOIN labels l ON l.job_id = j.id
                WHERE j.duplicate_of IS NULL AND l.job_id IS NULL
                ORDER BY j.id;"""
            ).fetchall()
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to read candidate jobs: {detail}", file=sys.stderr)
        return 1

    candidates = [job_id for (job_id,) in rows]
    if len(candidates) < N_SAMPLE:
        print(f"only {len(candidates)} unlabelled live jobs", file=sys.stderr)
        return 1

    # ORDER BY id above + a fixed seed = the same draw on every machine with the
    # same data. sample() returns them already in random order.
    rng = random.Random(RANDOM_STATE)
    drawn = rng.sample(candidates, N_SAMPLE)
    new_test = sorted(drawn[:N_NEW_TEST])
    new_train = sorted(drawn[N_NEW_TEST:])

    v1 = json.loads(FROZEN_V1_PATH.read_text())
    v1_test = sorted(d["job_id"] for d in v1["test_set"])

    test_ids = sorted(v1_test + new_test)
    test_v2 = {
        "version": "test-set-v2",
        "random_state": RANDOM_STATE,
        "from_test_set_v1": v1_test,
        "new": new_test,
        "job_ids": test_ids,
    }
    queue = {
        "round": 1,
        "random_state": RANDOM_STATE,
        "n_test": len(new_test),
        "n_train": len(new_train),
        "job_ids": drawn,  # shuffled: test and train mixed
    }
    TEST_V2_PATH.write_text(json.dumps(test_v2, indent=2) + "\n")
    QUEUE_PATH.write_text(json.dumps(queue, indent=2) + "\n")

    print(
        f"drew {N_SAMPLE} of {len(candidates)} unlabelled live jobs: "
        f"{len(new_test)} test + {len(new_train)} train; "
        f"test-set-v2 has {len(test_ids)} jobs"
    )
    return 0


if __name__ == "__main__":
    sys.exit(sample_label_round())
