"""The frozen test set: the jobs every model is measured on and none is trained on.

frozen.json holds test-set-v3: 150 job ids (their labels live in the labels table), the same
jobs as test-set-v2 with the positives re-checked against the criteria stated on 2026-11-07.
One place, so every script reads the same set.
"""

import json
from pathlib import Path

DATA_DIR = Path(__file__).resolve().parents[1] / "data"
FROZEN_PATH = DATA_DIR / "frozen.json"


def frozen_ids(path: Path = FROZEN_PATH) -> list[int]:
    """The test set's job ids, in file order."""
    data = json.loads(path.read_text(encoding="utf-8"))
    return [entry["job_id"] for entry in data["test_set"]]


def frozen_version(path: Path = FROZEN_PATH) -> str:
    """The test set's name (e.g. "test-set-v2"), printed next to every metric."""
    return str(json.loads(path.read_text(encoding="utf-8"))["version"])
