import os

import pandas as pd
import psycopg


def sql_to_dataframe(sql: str) -> pd.DataFrame:
    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        raise RuntimeError("DATABASE_URL is not set")

    try:
        conn = psycopg.connect(database_url)
    except psycopg.OperationalError as e:
        detail = " ".join(str(e).split())
        raise RuntimeError(f"failed to connect to the database: {detail}") from e

    with conn, conn.cursor() as cur:
        cur.execute(sql)
        df = pd.DataFrame(cur.fetchall(), columns=[c.name for c in cur.description])
    return df
