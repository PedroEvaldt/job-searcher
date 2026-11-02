import os
import sys

import psycopg
from psycopg.types.json import Json

from jobradar.boilerplate import clean_descriptions
from jobradar.jev.client import JevAnswers, JevClient, jev_client
from jobradar.jev.questions import QUESTION_SET_VERSION, QUESTIONS
from scripts.score_jev import MODEL_VERSION, get_score

BATCH_SIZE = 100

# Jev bills input tokens only, ~$0.042 per million (output is free).
INPUT_PRICE_PER_MILLION = 0.042


def score_jev_batch() -> int:
    """One-shot: drain the pending jobs once and exit. The daemon version is
    scripts.jev_worker; both share process_pending_jobs."""
    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1

    jev_c = jev_client()
    if jev_c is None:
        print("no JEV_API_KEY set — nothing to run", file=sys.stderr)
        return 1

    try:
        # autocommit=True: process_pending_jobs relies on it, so each batch's
        # conn.transaction() is a real commit instead of a savepoint (3a.2.3 gotcha).
        conn = psycopg.connect(database_url, autocommit=True)
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        print(f"failed to connect to the database: {detail}", file=sys.stderr)
        return 1

    with conn:
        total_jobs, total_cost = process_pending_jobs(conn, jev_c)
    print(f"done: {total_jobs} jobs scored, ${total_cost:.6f} total ({MODEL_VERSION})")
    return 0


def process_pending_jobs(
    conn: psycopg.Connection, jev_c: JevClient
) -> tuple[int, float]:
    """Score every live job still missing a Jev answer, in batches with a commit per
    batch — if interrupted, the next call picks up where this one left off, since a job
    with a jev_answers row is no longer pending. Expects an autocommit connection.

    Returns how many jobs were scored and what they cost, in USD."""
    with conn.cursor() as cur:
        # The whole live set: clean_descriptions builds the boilerplate statistic over
        # it once, so the cut does not change batch to batch.
        corpus = [
            row[0]
            for row in cur.execute(
                "SELECT description FROM jobs WHERE duplicate_of IS NULL;"
            ).fetchall()
        ]

    total_jobs = 0
    total_cost = 0.0
    batch_num = 0
    while True:
        with conn.cursor() as cur:
            rows = cur.execute(
                """SELECT j.id, j.title, j.description FROM jobs j
                LEFT JOIN jev_answers je
                    ON je.job_id = j.id AND je.question_set_version = %s
                WHERE j.duplicate_of IS NULL AND je.job_id IS NULL
                ORDER BY j.id
                LIMIT %s;""",
                (QUESTION_SET_VERSION, BATCH_SIZE),
            ).fetchall()
        if not rows:
            break
        batch_num += 1

        job_ids = [job_id for job_id, _, _ in rows]
        descriptions = clean_descriptions(
            corpus, [description for _, _, description in rows]
        )
        titles = [title for _, title, _ in rows]
        jev_states = [
            f"Title: {title}\n\n" + desc for title, desc in zip(titles, descriptions)
        ]

        answers: list[JevAnswers] = [
            jev_c.answer(state, QUESTIONS) for state in jev_states
        ]
        scores = [get_score(answer.answers) for answer in answers]

        batch_tokens = sum(a.input_tokens for a in answers)
        batch_cost = batch_tokens / 1_000_000 * INPUT_PRICE_PER_MILLION
        total_cost += batch_cost
        total_jobs += len(rows)

        with conn.cursor() as cur, conn.transaction():
            cur.executemany(
                """INSERT INTO jev_answers (job_id, question_set_version, answers)
                VALUES (%s, %s, %s)
                ON CONFLICT (job_id, question_set_version) DO UPDATE
                SET answers = EXCLUDED.answers, computed_at = now();""",
                [
                    (job_id, QUESTION_SET_VERSION, Json(answer.answers))
                    for job_id, answer in zip(job_ids, answers)
                ],
            )
            cur.executemany(
                """INSERT INTO scores (job_id, model_version, score)
                VALUES (%s, %s, %s)
                ON CONFLICT (job_id, model_version) DO UPDATE
                SET score = EXCLUDED.score, computed_at = now();""",
                [
                    (job_id, MODEL_VERSION, score)
                    for job_id, score in zip(job_ids, scores)
                ],
            )

        print(
            f"batch {batch_num}: {len(rows)} jobs ({total_jobs} total), "
            f"${batch_cost:.6f} this batch, ${total_cost:.6f} total"
        )

    return (total_jobs, total_cost)


if __name__ == "__main__":
    sys.exit(score_jev_batch())
