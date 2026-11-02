"""Where the frozen test set lives. One place, so every script reads the same one.

frozen.json is the current test set (test-set-v2: 150 job ids, labels in the labels
table). frozen_v1.json is test-set-v1 (75 jobs, labels inline), kept only to
reproduce the 3b–4c numbers; it is a subset of v2.
"""

from pathlib import Path

DATA_DIR = Path(__file__).resolve().parents[1] / "data"
FROZEN_PATH = DATA_DIR / "frozen.json"
FROZEN_V1_PATH = DATA_DIR / "frozen_v1.json"
