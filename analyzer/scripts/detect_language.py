"""Tag the language of every live job that has none.

    uv run --env-file ../.env python -m scripts.detect_language

The worker does this for new jobs every cycle; this is the one-shot version.
"""

import sys

from jobradar.db import DatabaseError, connect
from jobradar.enrich import detect_pending_languages


def main() -> int:
    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1
    with conn:
        tagged = detect_pending_languages(conn)
    print(f"tagged {tagged} jobs")
    return 0


if __name__ == "__main__":
    sys.exit(main())
