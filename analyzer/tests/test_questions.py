import pytest

from jobradar.jev.questions import QUESTION_SETS


@pytest.mark.parametrize("version", sorted(QUESTION_SETS))
def test_question_set_is_well_formed(version: str) -> None:
    questions = QUESTION_SETS[version]
    ids = [q.id for q in questions]
    assert len(ids) == len(set(ids))
    for q in questions:
        assert q.instructions != ""
        assert q.criteria_true != ""
        assert q.criteria_false != ""


def test_qs_v1_never_changes() -> None:
    # its answers are stored under "qs-v1": editing a question would mislabel them
    assert [q.id for q in QUESTION_SETS["qs-v1"]] == [
        "open_to_latam",
        "fully_remote",
        "is_senior_role",
    ]
