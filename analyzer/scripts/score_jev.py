"""Rewrite the jev-manual score of every live job from its stored Jev answers.

    uv run --env-file ../.env python -m scripts.score_jev

No Jev call and no cost: it only recombines the qs-v1 answers already in jev_answers. The
worker answers the digest model's question set, not qs-v1, so this baseline covers only the
jobs that have qs-v1 answers, and is kept to compare models with (docs/MODELING.md).
"""

import sys

from jobradar.db import DatabaseError, connect
from jobradar.jev.manual import MODEL_VERSION, QUESTION_SET, manual_score


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
            (QUESTION_SET,),
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
