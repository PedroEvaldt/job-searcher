"""Rewrite the jev-manual score of every live job from its stored Jev answers.

    uv run --env-file ../.env python -m scripts.score_jev

No Jev call and no cost: it only recombines the answers already in jev_answers. The
worker (scripts.jev_worker) writes this same score as it answers new jobs, so this is
only needed to backfill or after restoring a database.
"""

import sys

from jobradar.db import DatabaseError, connect
from jobradar.jev.manual import MODEL_VERSION, manual_score
from jobradar.jev.questions import QUESTION_SET_VERSION


def main() -> int:
    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1

    with conn, conn.cursor() as cur:
        rows = cur.execute(
            """
            SELECT je.job_id, je.answers
            FROM jev_answers je
            JOIN jobs j ON j.id = je.job_id
            WHERE je.question_set_version = %s AND j.duplicate_of IS NULL
            ORDER BY je.job_id;
            """,
            (QUESTION_SET_VERSION,),
        ).fetchall()
        if not rows:
            print("no Jev answers to score")
            return 0
        with conn.transaction():
            cur.executemany(
                """
                INSERT INTO scores (job_id, model_version, score)
                VALUES (%s, %s, %s)
                ON CONFLICT (job_id, model_version) DO UPDATE
                SET score = EXCLUDED.score, computed_at = now();
                """,
                [
                    (job_id, MODEL_VERSION, manual_score(answers))
                    for job_id, answers in rows
                ],
            )
    print(f"wrote {len(rows)} rows to scores ({MODEL_VERSION})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
