"""Answer the jobs the Jev has not seen yet, in batches, one commit per batch.

Used by every pass of jobradar.pipeline (the worker) and by scripts.answer_jev, which
runs a new question set over the existing jobs.
"""

from collections.abc import Callable
from concurrent.futures import ThreadPoolExecutor

import psycopg
from psycopg.types.json import Json

from jobradar.boilerplate import clean_descriptions
from jobradar.jev.client import JevClient
from jobradar.jev.questions import QUESTION_SETS

BATCH_SIZE = 100

# Jev bills input tokens only, ~$0.042 per million (output is free).
INPUT_PRICE_PER_MILLION = 0.042


def answer_pending_jobs(
    conn: psycopg.Connection,
    jev: JevClient,
    version: str,
    *,
    only_ids: list[int] | None = None,
    workers: int = 1,
    log: Callable[[str], None] | None = None,
) -> tuple[int, float]:
    """Answer question set `version` for every live job that has no answers to it yet
    (only the jobs in only_ids, when given). Resumable: a job with answers is no longer
    pending, so an interrupted run picks up where it stopped. Expects an autocommit
    connection, so each batch's transaction is a real commit. With workers > 1 the Jev
    calls of a batch run in parallel threads: they only wait on the network.

    Returns how many jobs were answered and what they cost, in USD."""
    questions = QUESTION_SETS[version]
    with conn.cursor() as cur:
        # The whole live set: the boilerplate statistic is built over it once, so the
        # cut does not change from batch to batch.
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
                """
                SELECT j.id, j.title, j.description FROM jobs j
                LEFT JOIN jev_answers je
                    ON je.job_id = j.id AND je.question_set_version = %s
                WHERE j.duplicate_of IS NULL AND je.job_id IS NULL
                    AND (%s::bigint[] IS NULL OR j.id = ANY(%s::bigint[]))
                ORDER BY j.id
                LIMIT %s;
                """,
                (version, only_ids, only_ids, BATCH_SIZE),
            ).fetchall()
        if not rows:
            break
        batch_num += 1

        job_ids = [job_id for job_id, _, _ in rows]
        descriptions = clean_descriptions(corpus, [desc for _, _, desc in rows])
        states = [
            f"Title: {title}\n\n{desc}"
            for (_, title, _), desc in zip(rows, descriptions, strict=True)
        ]
        if workers > 1:
            with ThreadPoolExecutor(max_workers=workers) as pool:
                answers = list(
                    pool.map(lambda state: jev.answer(state, questions), states)
                )
        else:
            answers = [jev.answer(state, questions) for state in states]

        batch_cost = (
            sum(a.input_tokens for a in answers) / 1_000_000 * INPUT_PRICE_PER_MILLION
        )
        total_cost += batch_cost
        total_jobs += len(rows)

        with conn.transaction(), conn.cursor() as cur:
            cur.executemany(
                """
                INSERT INTO jev_answers (job_id, question_set_version, answers)
                VALUES (%s, %s, %s)
                ON CONFLICT (job_id, question_set_version) DO UPDATE
                SET answers = EXCLUDED.answers, computed_at = now();
                """,
                [
                    (job_id, version, Json(answer.answers))
                    for job_id, answer in zip(job_ids, answers, strict=True)
                ],
            )

        (log or _print)(
            f"batch {batch_num}: {len(rows)} jobs ({total_jobs} total), "
            f"${batch_cost:.6f} this batch, ${total_cost:.6f} total ({version})"
        )

    return total_jobs, total_cost


def _print(line: str) -> None:
    # flushed: in a container, stdout is a pipe and would otherwise hold the lines back
    print(line, flush=True)
