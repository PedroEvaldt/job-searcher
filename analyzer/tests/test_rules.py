import pytest

from jobradar.model.combined import SPECS, Rules
from jobradar.model.features import Job


def job(
    language: str | None = "en",
    requires_degree: float = 0.0,
    open_to_brazil: float = 0.9,
) -> Job:
    answers = {"requires_degree": requires_degree, "open_to_brazil": open_to_brazil}
    return Job(
        job_id=1,
        title="",
        description="",
        jev_answers=answers,
        skills=None,
        language=language,
    )


RULES = Rules(
    languages=frozenset({"en", "pt", "es"}),
    max_requires_degree=0.5,
    min_open_to_brazil=0.15,
)


@pytest.mark.parametrize(
    ("candidate", "passes"),
    [
        (job("en"), True),
        (job("pt"), True),
        (job("es"), True),
        (job("de"), False),
        (job("fr"), False),
        # not detected: the rule only drops what it is sure about
        (job(None), True),
        (job("en", requires_degree=0.49), True),
        (job("en", requires_degree=0.5), False),
        (job("en", requires_degree=0.97), False),
        (job("en", open_to_brazil=0.15), True),
        (job("en", open_to_brazil=0.14), False),
        (job("en", open_to_brazil=0.03), False),
    ],
)
def test_rules(candidate: Job, passes: bool) -> None:
    assert RULES.passes(candidate) is passes


def test_no_rules_pass_everything() -> None:
    # without a degree rule the answer is never read, so qs-v1/qs-v2 jobs work too
    assert Rules().passes(Job(1, "", "", {}, None, "zh-cn"))


def test_answer_rules_only_on_a_set_that_asks_them() -> None:
    from jobradar.jev.questions import QUESTION_SETS

    for spec in SPECS.values():
        asked = {q.id for q in QUESTION_SETS[spec.question_set]}
        if spec.rules.max_requires_degree is not None:
            assert "requires_degree" in asked, spec.version
        if spec.rules.min_open_to_brazil is not None:
            assert "open_to_brazil" in asked, spec.version
