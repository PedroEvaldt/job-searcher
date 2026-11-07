"""sklearn-combined: logistic regression over the Jev answers and the code's features.

A trained model is an artifact of one MODEL_VERSION and is never retrained in place:
new labels or a changed feature mean a new version, so every number in scores (and in
the docs) still points at the exact model that produced it.
"""

from dataclasses import dataclass
from pathlib import Path

import joblib
import numpy as np
from sklearn.impute import SimpleImputer
from sklearn.linear_model import LogisticRegression
from sklearn.model_selection import GridSearchCV, StratifiedKFold
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from jobradar.frozen import DATA_DIR
from jobradar.model.features import FEATURES

MODEL_VERSION = "sklearn-combined-v2"

MODELS_DIR = DATA_DIR / "models"

RANDOM_STATE = 7
CV_FOLDS = 5
# Regularization strength, chosen by cross-validated average precision. With ~20
# positives, the search tends to pick strong regularization (small C).
C_GRID = (0.01, 0.1, 1, 10, 100, 500, 1000)


def model_path(version: str = MODEL_VERSION) -> Path:
    return MODELS_DIR / f"{version}.joblib"


def build_pipeline() -> Pipeline:
    # The imputer fills a missing years_required with the training median; scaling puts
    # every coefficient on the same footing, so their sizes can be compared.
    return Pipeline(
        [
            ("imputer", SimpleImputer(strategy="median")),
            ("scaler", StandardScaler()),
            ("model", LogisticRegression(random_state=RANDOM_STATE)),
        ]
    )


@dataclass(frozen=True)
class TrainResult:
    pipeline: Pipeline
    best_c: float
    fold_ap: list[float]  # average precision of the best C on each validation fold
    n: int
    n_pos: int

    @property
    def prevalence(self) -> float:
        """The positive rate: the average precision a random ranking would get."""
        return self.n_pos / self.n


def train(x: np.ndarray, y: np.ndarray) -> TrainResult:
    """Pick C by cross-validated average precision, then refit on all of x."""
    # Shuffled folds: the rows come ordered by job id, and job ids follow collection
    # order, so unshuffled folds would each be one slice of time.
    folds = StratifiedKFold(n_splits=CV_FOLDS, shuffle=True, random_state=RANDOM_STATE)
    search = GridSearchCV(
        build_pipeline(),
        param_grid={"model__C": list(C_GRID)},
        cv=folds,
        scoring="average_precision",
    )
    search.fit(x, y)
    results = search.cv_results_
    best = search.best_index_
    return TrainResult(
        pipeline=search.best_estimator_,
        best_c=float(search.best_params_["model__C"]),
        fold_ap=[float(results[f"split{k}_test_score"][best]) for k in range(CV_FOLDS)],
        n=len(y),
        n_pos=int(y.sum()),
    )


def coefficients(pipeline: Pipeline) -> list[tuple[str, float]]:
    """Each feature's coefficient on the scaled inputs, largest effect first. Correlated
    features (skill_missing = n_job_skills - skill_overlap) share their effect, so read
    them together, not one by one."""
    weights = pipeline.named_steps["model"].coef_[0]
    pairs = [(name, float(w)) for name, w in zip(FEATURES, weights, strict=True)]
    return sorted(pairs, key=lambda pair: -abs(pair[1]))


def predict(pipeline: Pipeline, x: np.ndarray) -> np.ndarray:
    """Probability that you would apply, one per row of x."""
    return np.asarray(pipeline.predict_proba(x)[:, 1])


def save(pipeline: Pipeline, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    joblib.dump(pipeline, path)


def load(path: Path) -> Pipeline:
    pipeline: Pipeline = joblib.load(path)
    return pipeline
