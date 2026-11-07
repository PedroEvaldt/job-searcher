"""sklearn-combined: logistic regression over the Jev answers and the code's features.

A trained model is an artifact of one MODEL_VERSION and is never retrained in place:
new labels or a changed feature mean a new version, so every number in scores (and in
the docs) still points at the exact model that produced it.
"""

from dataclasses import dataclass
from pathlib import Path

import joblib
import numpy as np
import psycopg
from sklearn.impute import SimpleImputer
from sklearn.linear_model import LogisticRegression
from sklearn.model_selection import GridSearchCV, StratifiedKFold
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from jobradar.frozen import DATA_DIR
from jobradar.model.features import (
    Job,
    feature_matrix,
    features_for_job,
    fetch_unscored_jobs,
)


@dataclass(frozen=True)
class Rules:
    """Hard rules from the stated criteria, applied on top of the model: a job that
    breaks one scores 0 whatever the model says. They are not learned, because the
    labels were not written with them in mind."""

    # Job languages allowed; None allows any. A job whose language was not detected
    # passes: the rule only drops what it is sure about.
    languages: frozenset[str] | None = None
    # A job whose requires_degree answer reaches this is dropped; None keeps all.
    max_requires_degree: float | None = None
    # A job whose open_to_brazil answer is below this is dropped; None keeps all.
    min_open_to_brazil: float | None = None

    def passes(self, job: Job) -> bool:
        wrong_language = (
            self.languages is not None
            and job.language is not None
            and job.language not in self.languages
        )
        needs_degree = (
            self.max_requires_degree is not None
            and job.jev_answers["requires_degree"] >= self.max_requires_degree
        )
        closed_to_brazil = (
            self.min_open_to_brazil is not None
            and job.jev_answers["open_to_brazil"] < self.min_open_to_brazil
        )
        return not (wrong_language or needs_degree or closed_to_brazil)


@dataclass(frozen=True)
class ModelSpec:
    """One version of the combined model: which Jev answers it reads, which features in
    which column order (a saved model expects exactly this order), and the rules
    applied on top. Changing any of them means a new version."""

    version: str
    question_set: str
    features: tuple[str, ...]
    rules: Rules = Rules()


SPECS = {
    spec.version: spec
    for spec in [
        ModelSpec(
            "sklearn-combined-v2",
            question_set="qs-v1",
            features=(
                "open_to_latam",
                "fully_remote",
                "is_senior_role",
                "years_required",
                "skill_overlap",
                "skill_coverage",
                "skill_missing",
                "n_job_skills",
            ),
        ),
        # qs-v2 adds open_to_brazil and is_target_role. Of the code features only
        # skill_coverage kept improving the nested cross-validated AP (2026-11-07).
        # Trained before 10 training labels were revised: superseded by v4.
        ModelSpec(
            "sklearn-combined-v3",
            question_set="qs-v2",
            features=(
                "open_to_latam",
                "open_to_brazil",
                "fully_remote",
                "is_senior_role",
                "is_target_role",
                "skill_coverage",
            ),
        ),
        # v3's features plus skill_overlap, the best nested cross-validated AP once 10
        # training positives that broke the stated criteria became negatives (2026-11-07).
        ModelSpec(
            "sklearn-combined-v4",
            question_set="qs-v2",
            features=(
                "open_to_latam",
                "open_to_brazil",
                "fully_remote",
                "is_senior_role",
                "is_target_role",
                "skill_coverage",
                "skill_overlap",
            ),
        ),
        # v4's model retrained on the qs-v3 answers, plus the two rules stated on
        # 2026-11-07: only jobs in English, Portuguese or Spanish, and none that
        # requires a completed degree with no alternative.
        ModelSpec(
            "sklearn-combined-v5",
            question_set="qs-v3",
            features=(
                "open_to_latam",
                "open_to_brazil",
                "fully_remote",
                "is_senior_role",
                "is_target_role",
                "skill_coverage",
                "skill_overlap",
            ),
            rules=Rules(
                languages=frozenset({"en", "pt", "es"}),
                max_requires_degree=0.5,
            ),
        ),
        # v5 retrained after job 5904 became a "no" and v5's audit added 4 labels, plus a
        # location rule: v5's audit misses were jobs the Jev is sure Brazil cannot take
        # (open_to_brazil 0.03-0.13), which the model ranked high anyway. Every training
        # "yes" has open_to_brazil >= 0.27; 0.15 keeps a margin below that. Rejected: on
        # test-set-v3 the rule drops a job labelled good, and its audit gave 6/10.
        ModelSpec(
            "sklearn-combined-v6",
            question_set="qs-v3",
            features=(
                "open_to_latam",
                "open_to_brazil",
                "fully_remote",
                "is_senior_role",
                "is_target_role",
                "skill_coverage",
                "skill_overlap",
            ),
            rules=Rules(
                languages=frozenset({"en", "pt", "es"}),
                max_requires_degree=0.5,
                min_open_to_brazil=0.15,
            ),
        ),
        # v5 plus other_stack (the job's languages miss Go and Python): 3 of v6's 4 audit
        # misses were Java, Rust or Node.js jobs. Trained on the labels as of v6's audit
        # and the test-set-v3 review; v5's rules, no location rule.
        ModelSpec(
            "sklearn-combined-v7",
            question_set="qs-v3",
            features=(
                "open_to_latam",
                "open_to_brazil",
                "fully_remote",
                "is_senior_role",
                "is_target_role",
                "skill_coverage",
                "skill_overlap",
                "other_stack",
            ),
            rules=Rules(
                languages=frozenset({"en", "pt", "es"}),
                max_requires_degree=0.5,
            ),
        ),
        # v7 on qs-v4, whose open_to_brazil counts a remote job that names no country as
        # open (2026-11-07). Same features and rules.
        ModelSpec(
            "sklearn-combined-v8",
            question_set="qs-v4",
            features=(
                "open_to_latam",
                "open_to_brazil",
                "fully_remote",
                "is_senior_role",
                "is_target_role",
                "skill_coverage",
                "skill_overlap",
                "other_stack",
            ),
            rules=Rules(
                languages=frozenset({"en", "pt", "es"}),
                max_requires_degree=0.5,
            ),
        ),
    ]
}

# The digest's model: what the worker scores new jobs with, and the version the scripts
# use when none is given. config.yaml's digest.model_version must name the same one.
MODEL_VERSION = "sklearn-combined-v7"

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


def coefficients(
    pipeline: Pipeline, features: tuple[str, ...]
) -> list[tuple[str, float]]:
    """Each feature's coefficient on the scaled inputs, largest effect first. Correlated
    features (skill_missing = n_job_skills - skill_overlap) share their effect, so read
    them together, not one by one."""
    weights = pipeline.named_steps["model"].coef_[0]
    pairs = [(name, float(w)) for name, w in zip(features, weights, strict=True)]
    return sorted(pairs, key=lambda pair: -abs(pair[1]))


def predict(pipeline: Pipeline, x: np.ndarray) -> np.ndarray:
    """Probability that you would apply, one per row of x."""
    return np.asarray(pipeline.predict_proba(x)[:, 1])


def score_jobs(
    spec: ModelSpec, pipeline: Pipeline, jobs: list[Job], cv_skills: set[str]
) -> list[float]:
    """The final score of each job: the model's probability, or 0 when the job breaks
    one of the version's rules."""
    x = feature_matrix(
        [features_for_job(job, cv_skills) for job in jobs], spec.features
    )
    probs = predict(pipeline, x)
    return [
        float(prob) if spec.rules.passes(job) else 0.0
        for job, prob in zip(jobs, probs, strict=True)
    ]


def save(pipeline: Pipeline, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    joblib.dump(pipeline, path)


def load(path: Path) -> Pipeline:
    pipeline: Pipeline = joblib.load(path)
    return pipeline


def write_scores(
    conn: psycopg.Connection, version: str, jobs: list[Job], scores: list[float]
) -> None:
    """Upsert the scores of one version in a single transaction, so the digest never
    ranks a mix of old and new scores."""
    with conn.transaction(), conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO scores (job_id, model_version, score)
            VALUES (%s, %s, %s)
            ON CONFLICT (job_id, model_version) DO UPDATE
            SET score = EXCLUDED.score, computed_at = now();
            """,
            [
                (job.job_id, version, score)
                for job, score in zip(jobs, scores, strict=True)
            ],
        )


def score_pending(
    conn: psycopg.Connection, spec: ModelSpec, pipeline: Pipeline, cv_skills: set[str]
) -> int:
    """Score the live jobs that have no up-to-date spec.version score. Returns how many."""
    jobs = fetch_unscored_jobs(conn, spec.question_set, spec.version)
    if jobs:
        write_scores(
            conn, spec.version, jobs, score_jobs(spec, pipeline, jobs, cv_skills)
        )
    return len(jobs)
