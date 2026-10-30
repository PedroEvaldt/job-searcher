import json
import os
import sys

import psycopg
from psycopg.types.json import Json

from jobradar.boilerplate import clean_descriptions
from jobradar.jev.client import JevAnswers, jev_client
from jobradar.jev.questions import QUESTION_SET_VERSION, QUESTIONS
from scripts.freeze_test_set import OUTPUT_PATH

# Jev bills input tokens only, ~$0.042 per million (output is free).
INPUT_PRICE_PER_MILLION = 0.042


def jev_pilot(mode: str) -> int:
    """Run Jev on a set of jobs picked by `mode`, store the answers, and print the cost per 1000.

    "test" is the frozen test set's jobs; "train" is the labelled jobs outside of it;
    "all" is every live job outside of it.
    """

    mode_query: dict[str, str] = {
        "train": """SELECT j.id, j.title, j.description FROM jobs j
                JOIN labels l ON l.job_id = j.id
                WHERE j.duplicate_of IS NULL AND j.id != ALL(%s)
                ORDER BY j.id;""",
        "test": """SELECT id, title, description FROM jobs
                WHERE duplicate_of IS NULL AND id = ANY(%s)
                ORDER BY id;""",
        "all": """SELECT id, title, description FROM jobs
                WHERE duplicate_of IS NULL AND id != ALL(%s)
                ORDER BY id;""",
    }
    query = mode_query.get(mode, None)
    if query is None:
        print("Invalid mode for jev")
        return 2

    data = json.loads(OUTPUT_PATH.read_text(encoding="utf-8"))
    ids = [d["job_id"] for d in data["test_set"]]

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

    conn.autocommit = True
    with conn, conn.cursor() as cur:
        # The whole live set: clean_descriptions builds the boilerplate statistic over
        # it, so the cut does not change with the sampled subset.
        corpus = [
            row[0]
            for row in cur.execute(
                "SELECT description FROM jobs WHERE duplicate_of IS NULL;"
            ).fetchall()
        ]
        # The frozen test set's jobs — a fixed, deterministic sample.
        rows = cur.execute(
            query,
            (ids,),
        ).fetchall()
        if not rows:
            print("no jobs to score")
            return 0

        job_ids = [job_id for job_id, _, _ in rows]
        descriptions = clean_descriptions(
            corpus, [description for _, _, description in rows]
        )
        titles = [title for _, title, _ in rows]
        jev_states = [
            f"Title: {title}\n\n" + desc for title, desc in zip(titles, descriptions)
        ]

        jev_c = jev_client()
        if jev_c is None:
            print("no JEV_API_KEY set — nothing to run", file=sys.stderr)
            return 1
        answers: list[JevAnswers] = [
            jev_c.answer(state, QUESTIONS) for state in jev_states
        ]

        total_input_tokens = sum(a.input_tokens for a in answers)
        cost = total_input_tokens / 1_000_000 * INPUT_PRICE_PER_MILLION
        per_1000 = cost / len(answers) * 1000
        print(
            f"{len(answers)} jobs: {total_input_tokens} input tokens, "
            f"${cost:.6f} total -> ~${per_1000:.4f} per 1000 jobs"
        )

        with conn.transaction():
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
    print(f"wrote {len(answers)} rows to jev_answers ({QUESTION_SET_VERSION})")
    return 0


if __name__ == "__main__":
    sys.exit(jev_pilot("all"))
