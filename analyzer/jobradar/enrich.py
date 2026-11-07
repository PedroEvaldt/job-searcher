"""What every new live job needs before it can be scored: its language and its skills.

Both steps only touch the jobs still missing them, so they are cheap to run often: the
worker runs them every cycle, and the analyzer service once per start.
"""

from collections import Counter

import psycopg

from jobradar.boilerplate import cut_boilerplate, paragraphs_counter
from jobradar.language import detect_language
from jobradar.skills import ALIASES, skills_extractor
from jobradar.text import html_to_paragraphs

SKILLS_BATCH_SIZE = 100


def detect_pending_languages(conn: psycopg.Connection) -> int:
    """Tag jobs.language on the live jobs that have none. Returns how many were tagged;
    a text langdetect cannot read stays NULL (and is retried next time)."""
    with conn.cursor() as cur:
        rows = cur.execute(
            "SELECT id, description FROM jobs "
            "WHERE duplicate_of IS NULL AND language IS NULL ORDER BY id;"
        ).fetchall()
        languages = {
            job_id: language
            for job_id, description in rows
            if (language := detect_language("\n".join(html_to_paragraphs(description))))
        }
        if languages:
            with conn.transaction():
                cur.executemany(
                    "UPDATE jobs SET language = %s WHERE id = %s;",
                    [(language, job_id) for job_id, language in languages.items()],
                )
    return len(languages)


def extract_pending_skills(conn: psycopg.Connection) -> int:
    """Write job_skills for the live jobs not extracted yet (skills_extracted_at IS
    NULL; the collector resets it when a job's text changes). Batches are claimed with
    FOR UPDATE SKIP LOCKED, so two processes never extract the same job, and each one
    commits on its own: expects an autocommit connection. Returns how many jobs."""
    with conn.cursor() as cur:
        # The boilerplate statistic is built once over the whole live set.
        counter = paragraphs_counter(
            [
                row[0]
                for row in cur.execute(
                    "SELECT description FROM jobs WHERE duplicate_of IS NULL;"
                ).fetchall()
            ]
        )
        total = 0
        while True:
            with conn.transaction():
                jobs = cur.execute(
                    "SELECT id, description FROM jobs "
                    "WHERE duplicate_of IS NULL AND skills_extracted_at IS NULL "
                    "ORDER BY id LIMIT %s FOR UPDATE SKIP LOCKED;",
                    (SKILLS_BATCH_SIZE,),
                ).fetchall()
                if not jobs:
                    return total
                for job_id, skills in skills_per_job(jobs, counter).items():
                    # Delete first, so a skill that left the text leaves the table too.
                    cur.execute("DELETE FROM job_skills WHERE job_id = %s;", (job_id,))
                    cur.executemany(
                        "INSERT INTO job_skills (job_id, skill, confidence) "
                        "VALUES (%s, %s, %s);",
                        [(job_id, skill, conf) for skill, conf in skills.items()],
                    )
                    cur.execute(
                        "UPDATE jobs SET skills_extracted_at = now() WHERE id = %s;",
                        (job_id,),
                    )
                total += len(jobs)


def skills_per_job(
    jobs: list[tuple[int, str]], counter: Counter[str]
) -> dict[int, dict[str, float]]:
    """Canonical skills found in each job, after cutting shared boilerplate."""
    return {
        job_id: skills_extractor(
            "\n".join(cut_boilerplate(html_to_paragraphs(description), counter)),
            ALIASES,
        )
        for job_id, description in jobs
    }
