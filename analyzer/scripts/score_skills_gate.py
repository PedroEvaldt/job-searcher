"""5.4.3: write the skills-gate-v1 score of every live job to scores.

Needs, per job, the Jev answers (jev_answers, qs-v1) and the extracted skills
(job_skills). A live job without Jev answers yet is skipped — its gate cannot be
evaluated — and gets scored on the next run, after jev-worker reaches it. A job with
Jev answers but no skills is scored normally: coverage 0, so score 0 — which is why
it warns when some live jobs never went through skill extraction.

Idempotent: re-running overwrites the same (job_id, model_version) rows.
"""

import os
import sys

import psycopg

from jobradar.jev.questions import QUESTION_SET_VERSION
from jobradar.match import (
    SKILLS_GATE_VERSION,
    cv_skills,
    skill_match,
    skills_gate_score,
)


def score_skills_gate() -> int:
    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1

    cv = cv_skills()
    try:
        with psycopg.connect(database_url) as conn:
            rows = conn.execute(
                """SELECT j.id, je.answers,
                       COALESCE(array_agg(js.skill) FILTER (WHERE js.skill IS NOT NULL),
                                '{}') AS skills
                FROM jobs j
                JOIN jev_answers je
                    ON je.job_id = j.id AND je.question_set_version = %s
                LEFT JOIN job_skills js ON js.job_id = j.id
                WHERE j.duplicate_of IS NULL
                GROUP BY j.id, je.answers;""",
                (QUESTION_SET_VERSION,),
            ).fetchall()
            n_live_without_jev = conn.execute(
                """SELECT count(*) FROM jobs j
                LEFT JOIN jev_answers je
                    ON je.job_id = j.id AND je.question_set_version = %s
                WHERE j.duplicate_of IS NULL AND je.job_id IS NULL;""",
                (QUESTION_SET_VERSION,),
            ).fetchone()
            # A job whose skills were never extracted has an empty job_skills and would
            # silently score 0 — run `python -m scripts.main` first to drain that queue.
            n_without_skills = conn.execute(
                """SELECT count(*) FROM jobs
                WHERE duplicate_of IS NULL AND skills_extracted_at IS NULL;"""
            ).fetchone()
            if n_without_skills and n_without_skills[0]:
                print(
                    f"warning: {n_without_skills[0]} live jobs have no extracted skills "
                    "yet and will score 0 — run `python -m scripts.main` first",
                    file=sys.stderr,
                )

            scored = [
                (
                    job_id,
                    SKILLS_GATE_VERSION,
                    skills_gate_score(answers, skill_match(set(skills), cv)),
                )
                for job_id, answers, skills in rows
            ]
            # One transaction: the model_version is either fully rewritten or untouched.
            with conn.transaction(), conn.cursor() as cur:
                cur.executemany(
                    """INSERT INTO scores (job_id, model_version, score)
                    VALUES (%s, %s, %s)
                    ON CONFLICT (job_id, model_version) DO UPDATE
                    SET score = EXCLUDED.score, computed_at = now();""",
                    scored,
                )
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to score {SKILLS_GATE_VERSION}: {detail}", file=sys.stderr)
        return 1

    in_gate = sum(1 for _, _, score in scored if score > 0)
    skipped = n_live_without_jev[0] if n_live_without_jev else 0
    print(
        f"{SKILLS_GATE_VERSION}: scored {len(scored)} live jobs, {in_gate} with score > 0; "
        f"skipped {skipped} without Jev answers (cv: {len(cv)} skills)"
    )
    return 0


if __name__ == "__main__":
    sys.exit(score_skills_gate())
