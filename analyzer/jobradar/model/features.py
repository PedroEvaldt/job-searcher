"""The combined model's features, computed one way for training and for scoring.

Training and scoring both go through features_for_job, so a job's features can never
be computed one way for the model and another way for the digest. A feature that
changes here changes the model: give it a new MODEL_VERSION (jobradar/model/combined.py).
"""

from dataclasses import dataclass

import numpy as np
import psycopg

from jobradar.experience import extract_years
from jobradar.jev.questions import QUESTION_SET_VERSION
from jobradar.match import skill_match

# Column order of the feature matrix. The saved model expects exactly this order.
FEATURES = (
    "open_to_latam",
    "fully_remote",
    "is_senior_role",
    "years_required",
    "skill_overlap",
    "skill_coverage",
    "skill_missing",
    "n_job_skills",
)

Features = dict[str, int | float | None]


@dataclass(frozen=True)
class Job:
    """What the features are computed from: one job, its Jev answers and its skills."""

    job_id: int
    description: str
    jev_answers: dict[str, float]
    skills: list[str] | None  # None: no skill extracted (the LEFT JOIN found nothing)


def features_for_job(job: Job, cv_skills: set[str]) -> Features:
    match = skill_match(set(job.skills or ()), cv_skills)
    return {
        "open_to_latam": job.jev_answers["open_to_latam"],
        "fully_remote": job.jev_answers["fully_remote"],
        "is_senior_role": job.jev_answers["is_senior_role"],
        "years_required": extract_years(job.description),
        "skill_overlap": match.overlap,
        "skill_coverage": match.coverage,
        "skill_missing": match.missing,
        "n_job_skills": match.n_job_skills,
    }


def feature_matrix(rows: list[Features]) -> np.ndarray:
    """One row per job, columns in FEATURES order. A missing value (None, e.g. no years
    stated) becomes NaN, which the model's imputer fills."""
    return np.array([[row[name] for name in FEATURES] for row in rows], dtype=float)


# Both queries below only return jobs the Jev answered with the current question set:
# without those answers there are no features. Filtering on the version also keeps a job
# from coming back twice once a second question set exists.


def fetch_live_jobs(conn: psycopg.Connection) -> list[Job]:
    """Every live (non-duplicate) job the model can score, by id."""
    rows = conn.execute(
        """
        SELECT j.id, j.description, je.answers, sk.skills
        FROM jobs j
        JOIN jev_answers je ON je.job_id = j.id AND je.question_set_version = %s
        LEFT JOIN (
            SELECT job_id, array_agg(skill) AS skills
            FROM job_skills
            GROUP BY job_id
        ) sk ON sk.job_id = j.id
        WHERE j.duplicate_of IS NULL
        ORDER BY j.id;
        """,
        (QUESTION_SET_VERSION,),
    ).fetchall()
    return [Job(*row) for row in rows]


def fetch_labelled_jobs(conn: psycopg.Connection) -> list[tuple[Job, bool]]:
    """Every labelled job with its would_apply label, by id. No duplicate_of filter: a
    labelled job keeps its label even if it is later marked as a duplicate."""
    rows = conn.execute(
        """
        SELECT j.id, j.description, je.answers, sk.skills, l.would_apply
        FROM labels l
        JOIN jobs j ON j.id = l.job_id
        JOIN jev_answers je ON je.job_id = j.id AND je.question_set_version = %s
        LEFT JOIN (
            SELECT job_id, array_agg(skill) AS skills
            FROM job_skills
            GROUP BY job_id
        ) sk ON sk.job_id = j.id
        ORDER BY j.id;
        """,
        (QUESTION_SET_VERSION,),
    ).fetchall()
    return [(Job(*row[:4]), bool(row[4])) for row in rows]
