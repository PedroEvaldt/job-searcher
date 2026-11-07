"""The analyzer service: the skills of every new live job, then the tfidf-v1 baseline.

    uv run --env-file ../.env python -m scripts.main

Runs once per `docker compose up` (the "analyzer" service) and exits. tfidf-v1 is kept as a
baseline to compare models with (docs/MODELING.md); the digest does not rank by it.
"""

import sys
from collections import Counter

import psycopg
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity

from jobradar.boilerplate import (
    cut_boilerplate,
    paragraphs_counter,
)
from jobradar.db import DatabaseError, connect
from jobradar.enrich import extract_pending_skills
from jobradar.match import CV_PATH
from jobradar.text import html_to_paragraphs

TFIDF_VERSION = "tfidf-v1"


def main() -> int:
    """The analyzer service: extract the skills of every new live job, score the
    unscored ones with tfidf-v1, then report how many jobs are still unscored."""
    # Autocommit (jobradar.db): each skills batch commits on its own, so the FOR UPDATE
    # locks are released batch by batch instead of at the very end.
    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1
    with conn, conn.cursor() as cur:
        try:
            extract_pending_skills(conn)
            insert_scores(conn)
        except psycopg.OperationalError as e:
            detail = " ".join(str(e).split())
            print(f"failed to insert skills: {detail}", file=sys.stderr)
            return 1

        cur.execute(
            """
            SELECT COUNT(*) AS total,
                   COUNT(*) FILTER (WHERE scored_at IS NULL AND duplicate_of IS NULL) AS unscored
            FROM jobs;
            """
        )
        row = cur.fetchone()
    assert row is not None
    total, unscored = row
    print(f"jobs: {unscored} of {total} not scored yet")
    return 0


def insert_scores(conn: psycopg.Connection) -> None:
    with conn.cursor() as cur:
        # Build the boilerplate statistic once over the whole corpus, then cut per job
        descriptions = [
            d[0]
            for d in cur.execute(
                "SELECT description FROM jobs WHERE duplicate_of IS NULL;"
            ).fetchall()
        ]
        counter = paragraphs_counter(descriptions)

        with conn.transaction():
            jobs = cur.execute(
                "SELECT id, description FROM jobs WHERE duplicate_of IS NULL AND scored_at IS NULL ORDER BY id;",
            ).fetchall()
            # A run with no new job is a normal day: nothing to fit TF-IDF on.
            if not jobs:
                return

            jobs_scores = scorer(jobs, counter)

            cur.executemany(
                "INSERT INTO scores (job_id, model_version, score, computed_at) VALUES(%s, %s, %s, now()) ON CONFLICT (job_id, model_version) DO UPDATE set score=EXCLUDED.score, computed_at=EXCLUDED.computed_at;",
                [
                    (job_id, TFIDF_VERSION, score)
                    for job_id, score in jobs_scores.items()
                ],
            )
            cur.executemany(
                "UPDATE jobs SET scored_at = now() WHERE id = %s;",
                [(job_id,) for job_id in jobs_scores],
            )


def scorer(jobs: list[tuple[int, str]], counter: Counter[str]) -> dict[int, float]:
    paragraphs: dict[int, list[str]] = {}
    descriptions: dict[int, str] = {}
    scores: dict[int, float] = {}
    for job_id, description in jobs:
        paragraphs[job_id] = html_to_paragraphs(description)
    for job_id, job_paragraphs in paragraphs.items():
        kept = cut_boilerplate(job_paragraphs, counter)
        descriptions[job_id] = "\n".join(kept)

    cv_text = CV_PATH.read_text(encoding="utf-8")
    corpus = [cv_text, *descriptions.values()]
    matrix = TfidfVectorizer(stop_words="english").fit_transform(corpus)
    grades = cosine_similarity(matrix[0], matrix[1:])[0]
    for i, job_id in enumerate(descriptions):
        scores[job_id] = float(grades[i])
    return scores


if __name__ == "__main__":
    sys.exit(main())
