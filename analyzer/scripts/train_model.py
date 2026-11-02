import json
import sys
from pathlib import Path

import joblib
import numpy as np
from sklearn.impute import SimpleImputer
from sklearn.linear_model import LogisticRegression
from sklearn.model_selection import GridSearchCV
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from jobradar.frozen import FROZEN_PATH as FREEZE_PATH
from scripts.build_features import OUTPUT_PATH

MODEL_PATH = Path(__file__).parents[1] / "data" / "jev_sklearn_model.pkl"

COLS = [
    "open_to_latam",
    "fully_remote",
    "is_senior_role",
    "cosine_minilm",
    "years_required",
    "salary_usd_year",
    "job_age_days",
    "skills_matched",
    "has_salary",
    "would_apply",
]


def train_model() -> int:
    try:
        with open(OUTPUT_PATH, encoding="utf-8") as f:
            full_set = json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        print(f"could not open {OUTPUT_PATH}: {e}")
        return 1
    try:
        with open(FREEZE_PATH, encoding="utf-8") as f:
            data = json.load(f)
        test_ids = [d["job_id"] for d in data["test_set"]]
    except (OSError, json.JSONDecodeError, KeyError) as e:
        print(f"could not open {FREEZE_PATH}: {e}")
        return 1

    train_rows = [job for job in full_set if job["job_id"] not in test_ids]
    for row in train_rows:
        row["has_salary"] = row["salary_usd_year"] is not None

    train_matrix = np.array([[row[c] for c in COLS] for row in train_rows], dtype=float)
    x_train_set = train_matrix[:, :-1]
    y_train_set = train_matrix[:, -1]

    pipeline = Pipeline(
        [
            ("imputer", SimpleImputer(strategy="median")),
            ("scaler", StandardScaler()),
            ("model", LogisticRegression(random_state=7)),
        ]
    )
    C = [0.01, 0.1, 1, 10, 100, 500, 1000]

    search = GridSearchCV(pipeline, param_grid={"model__C": C}, cv=5, scoring="roc_auc")
    search.fit(x_train_set, y_train_set)

    print("Best C:", search.best_params_["model__C"])
    joblib.dump(search.best_estimator_, MODEL_PATH)

    return 0


if __name__ == "__main__":
    sys.exit(train_model())
