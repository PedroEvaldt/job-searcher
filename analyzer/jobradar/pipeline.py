"""One pass over the live jobs: bring each one up to date for a model's score.

Each step only touches the jobs still missing it, so a pass with nothing new is cheap:
language, skills, the model's Jev answers, then the score. The worker runs it every
cycle (scripts.jev_worker); scripts.update_jobs runs it once.
"""

from dataclasses import dataclass

import psycopg
from sklearn.pipeline import Pipeline

from jobradar.enrich import detect_pending_languages, extract_pending_skills
from jobradar.jev.client import JevClient
from jobradar.jev.pending import answer_pending_jobs
from jobradar.match import cv_skills
from jobradar.model.combined import ModelSpec, score_pending


@dataclass(frozen=True)
class PassReport:
    tagged: int  # languages detected
    extracted: int  # jobs whose skills were extracted
    answered: int  # jobs the Jev answered
    cost: float  # USD
    scored: int  # scores written

    def __str__(self) -> str:
        return (
            f"{self.tagged} languages, {self.extracted} skill extractions, "
            f"{self.answered} jobs answered (${self.cost:.6f}), {self.scored} scored"
        )


def update_jobs(
    conn: psycopg.Connection, jev: JevClient, spec: ModelSpec, pipeline: Pipeline
) -> PassReport:
    """Expects an autocommit connection: every step commits batch by batch."""
    tagged = detect_pending_languages(conn)
    extracted = extract_pending_skills(conn)
    answered, cost = answer_pending_jobs(conn, jev, spec.question_set)
    # cv.md is read on every pass, so an edited CV reaches the next scores.
    scored = score_pending(conn, spec, pipeline, cv_skills())
    return PassReport(tagged, extracted, answered, cost, scored)
