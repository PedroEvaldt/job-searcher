import json
import sys
from pathlib import Path

import joblib
import numpy as np
from sklearn.impute import SimpleImputer
from sklearn.linear_model import LogisticRegression
from sklearn.model_selection import GridSearchCV, StratifiedKFold
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from jobradar.frozen import FROZEN_PATH as FREEZE_PATH
from scripts.build_features_v2 import OUTPUT_PATH

MODEL_PATH = Path(__file__).parents[1] / "data" / "jev_sklearn_model_v2.pkl"

COLS = [
    "open_to_latam",
    "fully_remote",
    "is_senior_role",
    "years_required",
    "skill_overlap",
    "skill_coverage",
    "skill_missing",
    "n_job_skills",
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

    # Shuffled: features_v2.json is ordered by job_id, and cv=5 alone would not shuffle.
    folds = StratifiedKFold(n_splits=5, shuffle=True, random_state=7)
    search = GridSearchCV(
        pipeline, param_grid={"model__C": C}, cv=folds, scoring="average_precision"
    )
    search.fit(x_train_set, y_train_set)

    print("Best C:", search.best_params_["model__C"])

    # Few positives per fold make one number unreliable: show the spread, and the
    # prevalence, which is the AP a random ranking would get.
    results = search.cv_results_
    best = search.best_index_
    fold_scores = [
        results[f"split{k}_test_score"][best] for k in range(folds.get_n_splits())
    ]
    print("CV AP per fold:", " ".join(f"{s:.4f}" for s in fold_scores))
    print(f"CV AP: {np.mean(fold_scores):.4f} ± {np.std(fold_scores):.4f}")
    print(
        f"prevalence (random AP): {y_train_set.mean():.4f} "
        f"({int(y_train_set.sum())}/{len(y_train_set)})"
    )
    joblib.dump(search.best_estimator_, MODEL_PATH)

    return 0


if __name__ == "__main__":
    sys.exit(train_model())
