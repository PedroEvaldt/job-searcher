"""One pass of the worker, then exit: bring every live job up to date for MODEL_VERSION.

    uv run --group jev --env-file ../.env python -m scripts.update_jobs

Language, skills, the model's Jev answers and its score, each only for the jobs still
missing it. Needs JEV_API_KEY.
"""

import sys

from jobradar.db import DatabaseError, connect
from jobradar.jev.client import jev_client
from jobradar.model.combined import MODEL_VERSION, SPECS, load, model_path
from jobradar.pipeline import update_jobs


def main() -> int:
    jev = jev_client()
    if jev is None:
        print("no JEV_API_KEY set, nothing to run", file=sys.stderr)
        return 1
    path = model_path(MODEL_VERSION)
    if not path.exists():
        print(f"no model at {path}", file=sys.stderr)
        return 1
    pipeline = load(path)
    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1

    with conn:
        report = update_jobs(conn, jev, SPECS[MODEL_VERSION], pipeline)
    print(f"done: {report} ({MODEL_VERSION})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
