"""Daemon mode of the Jev scorer: drain every pending job, sleep, repeat.

The one-shot version is scripts.score_jev_batch; both share process_pending_jobs.
"""

import os
import signal
import sys
import threading

import psycopg
from typesafe_sdk import (
    TypeSafeAPIConnectionError,
    TypeSafeInternalServerError,
    TypeSafeRateLimitError,
)

from jobradar.jev.client import JevClient, jev_client
from jobradar.jev.manual import MODEL_VERSION
from scripts.score_jev_batch import process_pending_jobs

DEFAULT_INTERVAL_SECONDS = 15 * 60

# Failures that heal on their own: log one line and try again next cycle. Everything
# else still crashes the worker, loudly — a bug (TypeError…), a bad key (401) or a
# request Jev rejects (400) does not get better by waiting.
TRANSIENT_ERRORS = (
    psycopg.OperationalError,  # Postgres down or restarting
    TypeSafeAPIConnectionError,  # Jev unreachable; includes timeouts
    TypeSafeRateLimitError,  # 429
    TypeSafeInternalServerError,  # 5xx
)


def interval_from_env() -> int:
    """Seconds to sleep between cycles, from JEV_WORKER_INTERVAL_SECONDS. A bad value
    is an error, not a silent fallback: a typo should not quietly change the cadence."""
    raw = os.getenv("JEV_WORKER_INTERVAL_SECONDS")
    # Empty counts as unset: compose passes ${VAR} through as "" when .env leaves it blank.
    if not raw:
        return DEFAULT_INTERVAL_SECONDS
    try:
        seconds = int(raw)
    except ValueError:
        raise ValueError(
            f"JEV_WORKER_INTERVAL_SECONDS must be a whole number of seconds, got {raw!r}"
        ) from None
    if seconds <= 0:
        raise ValueError(f"JEV_WORKER_INTERVAL_SECONDS must be positive, got {seconds}")
    return seconds


def main() -> int:
    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        print("DATABASE_URL is not set", file=sys.stderr, flush=True)
        return 1

    try:
        interval = interval_from_env()
    except ValueError as e:
        print(e, file=sys.stderr, flush=True)
        return 1

    jev_c = jev_client()
    if jev_c is None:
        print("no JEV_API_KEY set — nothing to run", file=sys.stderr, flush=True)
        return 1

    stop = threading.Event()

    def handle_stop(signum: int, frame: object) -> None:
        # Only flip the flag. The loop notices it after the current cycle; doing more
        # here (closing the connection, sys.exit) would cut a batch in half.
        print(f"got signal {signum}, stopping after the current cycle", flush=True)
        stop.set()

    signal.signal(signal.SIGTERM, handle_stop)
    signal.signal(signal.SIGINT, handle_stop)

    print(f"jev worker started, every {interval}s ({MODEL_VERSION})", flush=True)
    while not stop.is_set():
        try:
            run_cycle(database_url, jev_c)
        except TRANSIENT_ERRORS as e:
            detail = " ".join(str(e).split())
            print(
                f"cycle failed ({type(e).__name__}), retrying in {interval}s: {detail}",
                file=sys.stderr,
                flush=True,
            )
        stop.wait(timeout=interval)

    print("jev worker stopped", flush=True)
    return 0


def run_cycle(database_url: str, jev_c: JevClient) -> None:
    # A fresh connection every cycle: a connection opened before a Postgres restart is
    # dead for good, and opening one every few minutes costs nothing. The with closes
    # it at the end of the cycle, on success or on error.
    with psycopg.connect(database_url, autocommit=True) as conn:
        total_jobs, total_cost = process_pending_jobs(conn, jev_c)
    print(f"cycle: {total_jobs} jobs scored, ${total_cost:.6f}", flush=True)


if __name__ == "__main__":
    sys.exit(main())
