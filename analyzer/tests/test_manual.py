import pytest

from jobradar.jev.manual import WEIGHTS, manual_score
from jobradar.jev.questions import QUESTIONS


def test_every_question_has_a_weight() -> None:
    assert {q.id for q in QUESTIONS} == set(WEIGHTS)


def test_manual_score_is_the_weighted_sum() -> None:
    answers = {"fully_remote": 1.0, "open_to_latam": 1.0, "is_senior_role": 0.0}
    assert manual_score(answers) == pytest.approx(0.7 + 0.6)


def test_senior_role_lowers_the_score() -> None:
    junior = {"fully_remote": 1.0, "open_to_latam": 1.0, "is_senior_role": 0.0}
    senior = {"fully_remote": 1.0, "open_to_latam": 1.0, "is_senior_role": 1.0}
    assert manual_score(senior) < manual_score(junior)


def test_unknown_question_raises() -> None:
    with pytest.raises(KeyError):
        manual_score({"new_question": 1.0})
