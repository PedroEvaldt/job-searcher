import os
import sys

import psycopg


def main() -> int:
    """Report how many jobs are still waiting for a score."""
    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1
    try:
        conn = psycopg.connect(database_url)
    except psycopg.OperationalError as e:
        detail = " ".join(str(e).split())
        print(f"failed to connect to the database: {detail}", file=sys.stderr)
        return 1

    with conn, conn.cursor() as cur:
        cur.execute(
            """
            SELECT COUNT(*) AS total,
                   COUNT(*) FILTER (WHERE scored_at IS NULL) AS unscored
            FROM jobs;
            """
        )
        row = cur.fetchone()
    assert row is not None
    total, unscored = row
    print(f"jobs: {unscored} of {total} not scored yet")
    return 0


if __name__ == "__main__":
    sys.exit(main())
