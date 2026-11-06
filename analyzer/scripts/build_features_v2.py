import json
import os
import sys
from pathlib import Path

import psycopg

from jobradar.experience import extract_years
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
                    JOIN jev_answers je ON l.job_id = je.job_id
                    LEFT JOIN (
                        SELECT job_id, array_agg(skill) AS skills
                        FROM job_skills
                        GROUP BY job_id
                    ) sk ON sk.job_id = l.job_id
                    ORDER BY l.job_id;
                    """
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
            years_required = extract_years(desc)
            skills = set(skills) if skills is not None else set()
            skills_match = skill_match(skills, cv_skill)

            job_features: dict[str, int | bool | float | None] = {
                "job_id": job_id,
                "would_apply": would_apply,
                "open_to_latam": jev_score["open_to_latam"],
                "fully_remote": jev_score["fully_remote"],
                "is_senior_role": jev_score["is_senior_role"],
                "years_required": years_required,
                "skill_overlap": skills_match.overlap,
                "skill_coverage": skills_match.coverage,
                "skill_missing": skills_match.missing,
                "n_job_skills": skills_match.n_job_skills,
            }
            jobs_features.append(job_features)

    with OUTPUT_PATH.open("w") as f:
        json.dump(jobs_features, f, indent=2)
    return 0


if __name__ == "__main__":
    sys.exit(build_features())
