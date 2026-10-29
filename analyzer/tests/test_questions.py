from jobradar.jev.questions import QUESTION_SET_VERSION, QUESTIONS


def test_question_set() -> None:
    assert QUESTION_SET_VERSION == "qs-v1"
    ids = [q.id for q in QUESTIONS]
    assert len(ids) == len(set(ids))
    for q in QUESTIONS:
        assert q.instructions != ""
        assert q.criteria_true != ""
        assert q.criteria_false != ""
