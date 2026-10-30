import json
import os
import sys
from datetime import UTC, datetime
from pathlib import Path

import psycopg

from jobradar.experience import extract_years
from jobradar.salary import normalize_salary

OUTPUT_PATH = Path(__file__).parents[1] / "data" / "features.json"


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
                    SELECT l.job_id, l.would_apply, j.description, j.salary_min,
                    j.salary_max, j.currency, j.salary_period, j.posted_at,
                    je.answers AS jev_score, s.score AS cosine_minilm,
                    COALESCE(sk.skills_matched, 0) AS skills_matched
                    FROM labels l JOIN jobs j ON l.job_id = j.id
                    JOIN jev_answers je ON l.job_id = je.job_id
                    JOIN (SELECT job_id, score FROM scores WHERE model_version = 'minilm-l6-v2') s ON s.job_id = l.job_id
                    LEFT JOIN (SELECT job_id, SUM(confidence) AS skills_matched FROM job_skills GROUP BY (job_id)) sk ON sk.job_id = l.job_id
                    ORDER BY l.job_id;
                    """
        ).fetchall()
        if not rows:
            print("no jobs to build features")
            return 0

        for (
            job_id,
            would_apply,
            desc,
            salary_min,
            salary_max,
            currency,
            salary_period,
            posted_at,
            jev_score,
            cosine_minilm,
            skills_matched,
        ) in rows:
            years_required = extract_years(desc)
            salary = (
                float(salary_min + salary_max) / 2
                if salary_min is not None and salary_max is not None
                else None
            )
            salary_usd_year = normalize_salary(salary, currency, salary_period)
            job_age_days = (
                (datetime.now(UTC) - posted_at).days
                if posted_at is not None
                else None
            )

            job_features: dict[str, int | bool | float | None] = {
                "job_id": job_id,
                "would_apply": would_apply,
                "open_to_latam": jev_score["open_to_latam"],
                "fully_remote": jev_score["fully_remote"],
                "is_senior_role": jev_score["is_senior_role"],
                "cosine_minilm": cosine_minilm,
                "years_required": years_required,
                "salary_usd_year": salary_usd_year,
                "job_age_days": job_age_days,
                "skills_matched": skills_matched,
            }
            jobs_features.append(job_features)

    with OUTPUT_PATH.open("w") as f:
        json.dump(jobs_features, f, indent=2)
    return 0


if __name__ == "__main__":
    sys.exit(build_features())
