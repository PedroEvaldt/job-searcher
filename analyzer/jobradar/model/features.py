"""The combined models' features, computed one way for training and for scoring.

Training and scoring both go through features_for_job, so a job's features can never
be computed one way for the model and another way for the digest. Each model version
picks its columns by name (jobradar/model/combined.py); changing how a feature is
computed changes every model that uses it, so it needs new versions.
"""

from dataclasses import dataclass

import numpy as np
import psycopg

from jobradar.experience import extract_years
from jobradar.match import skill_match
from jobradar.skills import LANGUAGES
from jobradar.stack import STACK_LANGUAGES, description_languages, title_languages

# The features computed in code; every Jev answer of the job's question set is a
# feature too, under its question id.
CODE_FEATURES = (
    "years_required",
    "skill_overlap",
    "skill_coverage",
    "skill_missing",
    "n_job_skills",
    "other_stack",
    "main_other_stack",
    "title_cv_stack",
)

Features = dict[str, int | float | None]


@dataclass(frozen=True)
class Job:
    """What the features are computed from: one job, its Jev answers and its skills."""

    job_id: int
    title: str
    description: str
    jev_answers: dict[str, float]
    skills: list[str] | None  # None: no skill extracted (the LEFT JOIN found nothing)
    language: str | None  # jobs.language (langdetect); None: not detected


def features_for_job(job: Job, cv_skills: set[str]) -> Features:
    job_skills = set(job.skills or ())
    match = skill_match(job_skills, cv_skills)
    job_languages = job_skills & LANGUAGES
    cv_languages = cv_skills & STACK_LANGUAGES
    title_langs = title_languages(job.title)
    main_langs = title_langs or description_languages(job.description)
    return {
        **job.jev_answers,
        "years_required": extract_years(job.description),
        "skill_overlap": match.overlap,
        "skill_coverage": match.coverage,
        "skill_missing": match.missing,
        "n_job_skills": match.n_job_skills,
        # 1 when the job names programming languages and none of them is in the CV: a
        # Java or Node.js backend, for a Go and Python CV.
        "other_stack": int(bool(job_languages) and not job_languages & cv_skills),
        # other_stack read with jobradar/stack.py, which also knows COBOL or Delphi, and
        # from the job's main languages: the title's when it names any, since a "Java
        # Developer" that mentions Python is still a Java job; else the description's.
        "main_other_stack": int(bool(main_langs) and not main_langs & cv_languages),
        # 1 when the title names one of the CV's languages: "Golang Developer".
        "title_cv_stack": int(bool(title_langs & cv_languages)),
    }


def feature_matrix(rows: list[Features], names: tuple[str, ...]) -> np.ndarray:
    """One row per job, one column per name, in that order. A missing value (None, e.g.
    no years stated) becomes NaN, which the model's imputer fills."""
    return np.array([[row[name] for name in names] for row in rows], dtype=float)


# Both queries only return jobs the Jev answered with the given question set: without
# those answers there are no features. Filtering on the version also keeps a job from
# coming back once per question set.


def fetch_live_jobs(conn: psycopg.Connection, question_set: str) -> list[Job]:
    """Every live (non-duplicate) job answered with question_set, by id."""
    rows = conn.execute(
        """
        SELECT j.id, j.title, j.description, je.answers, sk.skills, j.language
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
        (question_set,),
    ).fetchall()
    return [Job(*row) for row in rows]


def fetch_labelled_jobs(
    conn: psycopg.Connection, question_set: str
) -> list[tuple[Job, bool]]:
    """Every labelled job answered with question_set, with its would_apply label, by id.
    No duplicate_of filter: a labelled job keeps its label even if it is later marked
    as a duplicate."""
    rows = conn.execute(
        """
        SELECT j.id, j.title, j.description, je.answers, sk.skills, j.language, l.would_apply
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
        (question_set,),
    ).fetchall()
    return [(Job(*row[:6]), bool(row[6])) for row in rows]


def fetch_unscored_jobs(
    conn: psycopg.Connection, question_set: str, model_version: str
) -> list[Job]:
    """The live jobs whose model_version score is missing or older than what it was
    computed from (the job's answers or its skills). Only jobs whose skills were already
    extracted: scored before that, they would be scored without their skills."""
    rows = conn.execute(
        """
        SELECT j.id, j.title, j.description, je.answers, sk.skills, j.language
        FROM jobs j
        JOIN jev_answers je ON je.job_id = j.id AND je.question_set_version = %s
        LEFT JOIN (
            SELECT job_id, array_agg(skill) AS skills
            FROM job_skills
            GROUP BY job_id
        ) sk ON sk.job_id = j.id
        LEFT JOIN scores s ON s.job_id = j.id AND s.model_version = %s
        WHERE j.duplicate_of IS NULL
          AND j.skills_extracted_at IS NOT NULL
          AND (
              s.job_id IS NULL
              OR s.computed_at < je.computed_at
              OR s.computed_at < j.skills_extracted_at
          )
        ORDER BY j.id;
        """,
        (question_set, model_version),
    ).fetchall()
    return [Job(*row) for row in rows]
