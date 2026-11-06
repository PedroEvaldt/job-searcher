import json
import os
import sys
from pathlib import Path

import psycopg

from jobradar.experience import extract_years
from jobradar.jev.questions import QUESTION_SET_VERSION
from jobradar.match import CV_PATH, cv_skills, skill_match

OUTPUT_PATH = Path(__file__).parents[1] / "data" / "features_v2.json"


def build_features() -> int:
    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1
    try:
        conn = psycopg.connect(database_url)
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to connect to the database: {detail}", file=sys.stderr)
        return 1

    jobs_features = []
    conn.autocommit = True
    with conn, conn.cursor() as cur:
        rows = cur.execute(
            """
                    SELECT l.job_id, l.would_apply, j.description, 
                    je.answers AS jev_score, sk.skills
                    FROM labels l 
                    JOIN jobs j ON l.job_id = j.id
                    -- One question set only: with two, every job would come back twice.
                    JOIN jev_answers je ON l.job_id = je.job_id
                        AND je.question_set_version = %s
                    LEFT JOIN (
                        SELECT job_id, array_agg(skill) AS skills
                        FROM job_skills
                        GROUP BY job_id
                    ) sk ON sk.job_id = l.job_id
                    ORDER BY l.job_id;
                    """,
            (QUESTION_SET_VERSION,),
        ).fetchall()
        if not rows:
            print("no jobs to build features")
            return 0
        cv_skill = cv_skills(CV_PATH)

        for (
            job_id,
            would_apply,
            desc,
            jev_score,
            skills,
        ) in rows:
            jobs_features.append(
                {
                    "job_id": job_id,
                    "would_apply": would_apply,
                    **features_for_job(desc, jev_score, skills, cv_skill),
                }
            )

    with OUTPUT_PATH.open("w") as f:
        json.dump(jobs_features, f, indent=2)
    return 0


def features_for_job(
    desc: str,
    jev_score: dict[str, float],
    skills: list[str] | None,
    cv_skill: set[str],
) -> dict[str, int | float | None]:
    """The model's features for one job. Training (above) and scoring
    (score_live_v2) both call this, so a job's features can never be computed one
    way for the model and another way for the digest. The label stays with the caller.
    """
    # A job with no extracted skills comes back from the LEFT JOIN as NULL.
    job_skills = set(skills) if skills is not None else set()
    skills_match = skill_match(job_skills, cv_skill)

    return {
        "open_to_latam": jev_score["open_to_latam"],
        "fully_remote": jev_score["fully_remote"],
        "is_senior_role": jev_score["is_senior_role"],
        "years_required": extract_years(desc),
        "skill_overlap": skills_match.overlap,
        "skill_coverage": skills_match.coverage,
        "skill_missing": skills_match.missing,
        "n_job_skills": skills_match.n_job_skills,
    }


if __name__ == "__main__":
    sys.exit(build_features())
