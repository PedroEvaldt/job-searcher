"""5.4.4: the round 2 label queue — 150 training jobs chosen by skills-gate-v1.

Takes the live jobs with the highest skills-gate-v1 score that are:
- not labelled yet;
- not in the frozen test set (a test job never becomes training data);
- not in the round 1 queue (round 1 may still be half-labelled).

The top is where most positives are, and also the costliest mistakes (jobs the rule
loves and you would not apply to): both teach the sklearn model of 5.4.6.

The queue is shuffled with a fixed seed before writing, so the labelling order does
not tell you which jobs the rule ranked first. Refuses to run twice.
"""

import json
import os
import random
import sys

import psycopg

from jobradar.frozen import DATA_DIR, FROZEN_PATH
from jobradar.match import SKILLS_GATE_VERSION

N_ROUND2 = 150
RANDOM_STATE = 20261103
ROUND1_PATH = DATA_DIR / "label_queue_round1.json"
QUEUE_PATH = DATA_DIR / "label_queue_round2.json"


def build_label_queue_round2() -> int:
    if QUEUE_PATH.exists():
        print(f"{QUEUE_PATH.name} already exists — round 2 is frozen", file=sys.stderr)
        return 1

    frozen_ids = [d["job_id"] for d in json.loads(FROZEN_PATH.read_text())["test_set"]]
    round1_ids = json.loads(ROUND1_PATH.read_text())["job_ids"]
    excluded = sorted(set(frozen_ids) | set(round1_ids))

    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1
    try:
        with psycopg.connect(database_url) as conn:
            rows = conn.execute(
                """SELECT j.id, s.score FROM jobs j
                JOIN scores s ON s.job_id = j.id AND s.model_version = %s
                LEFT JOIN labels l ON l.job_id = j.id
                WHERE j.duplicate_of IS NULL
                  AND l.job_id IS NULL
                  AND NOT (j.id = ANY(%s))
                ORDER BY s.score DESC, j.id
                LIMIT %s;""",
                (SKILLS_GATE_VERSION, excluded, N_ROUND2),
            ).fetchall()
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to read {SKILLS_GATE_VERSION} scores: {detail}", file=sys.stderr)
        return 1

    if len(rows) < N_ROUND2:
        print(
            f"only {len(rows)} candidates — did score_skills_gate run?", file=sys.stderr
        )
        return 1
    in_gate = sum(1 for _, score in rows if score > 0)
    if in_gate < N_ROUND2:
        # Not fatal: the tail is then score-0 jobs, ordered by id. Worth knowing.
        print(
            f"warning: only {in_gate} of {N_ROUND2} have score > 0; the rest are "
            "score-0 jobs (outside the gate or no skill in common)",
            file=sys.stderr,
        )

    job_ids = [job_id for job_id, _ in rows]
    rng = random.Random(RANDOM_STATE)
    rng.shuffle(job_ids)
    queue = {
        "round": 2,
        "model_version": SKILLS_GATE_VERSION,
        "random_state": RANDOM_STATE,
        "score_range": [rows[-1][1], rows[0][1]],
        "job_ids": job_ids,
    }
    QUEUE_PATH.write_text(json.dumps(queue, indent=2) + "\n")
    print(
        f"round 2: {len(job_ids)} jobs from the top of {SKILLS_GATE_VERSION} "
        f"(score {rows[-1][1]:.3f}–{rows[0][1]:.3f}, {in_gate} in the gate)"
    )
    return 0


if __name__ == "__main__":
    sys.exit(build_label_queue_round2())
