import math

import pytest

from jobradar.model.features import FEATURES, Job, feature_matrix, features_for_job

CV = {"Go", "Python", "Docker"}
ANSWERS = {"open_to_latam": 0.9, "fully_remote": 0.8, "is_senior_role": 0.1}


def make_job(description: str = "", skills: list[str] | None = None) -> Job:
    return Job(job_id=1, description=description, jev_answers=ANSWERS, skills=skills)


def test_features_for_job_has_every_feature_in_order() -> None:
    features = features_for_job(make_job(skills=["Go"]), CV)
    assert tuple(features) == FEATURES


def test_features_for_job_values() -> None:
    job = make_job("We need 3+ years of experience with Go.", ["Go", "Python", "Java"])
    assert features_for_job(job, CV) == {
        "open_to_latam": 0.9,
        "fully_remote": 0.8,
        "is_senior_role": 0.1,
        "years_required": 3,
        "skill_overlap": 2,
        "skill_coverage": pytest.approx(2 / 3),
        "skill_missing": 1,
        "n_job_skills": 3,
    }


def test_features_for_job_without_skills() -> None:
    # the LEFT JOIN gives None for a job with no extracted skill
    features = features_for_job(make_job(skills=None), CV)
    assert features["n_job_skills"] == 0
    assert features["skill_coverage"] == 0.0


def test_feature_matrix_turns_none_into_nan() -> None:
    rows = [features_for_job(make_job("no years here", ["Go"]), CV)]
    matrix = feature_matrix(rows)
    assert matrix.shape == (1, len(FEATURES))
    assert math.isnan(matrix[0, FEATURES.index("years_required")])
