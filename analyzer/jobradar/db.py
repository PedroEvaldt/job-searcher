"""The one way scripts open Postgres: DATABASE_URL from the environment, autocommit on.

Autocommit is the default on purpose. Every script that writes does it inside
`with conn.transaction():`, and with autocommit on that block is a real transaction.
With it off, the first read opens an implicit transaction and every later
`conn.transaction()` silently becomes a savepoint, holding locks until the very end.
"""

import os

import psycopg


class DatabaseError(Exception):
    """DATABASE_URL is missing or Postgres refused the connection. The message is one
    line, ready to print."""


def connect() -> psycopg.Connection:
    database_url = os.getenv("DATABASE_URL")
    if not database_url:
        raise DatabaseError("DATABASE_URL is not set")
    try:
        return psycopg.connect(database_url, autocommit=True)
    except psycopg.Error as e:
        detail = " ".join(str(e).split())
        raise DatabaseError(f"failed to connect to the database: {detail}") from e
