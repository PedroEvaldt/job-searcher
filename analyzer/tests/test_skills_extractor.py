import pytest

from jobradar.skills import skills_extractor


@pytest.mark.parametrize(
    ("text", "skills", "expected"),
    [
        # two written forms of the same skill collapse to one canonical, counts summed
        (
            "I use JS and JavaScript",
            {"js": "JavaScript", "javascript": "JavaScript"},
            {"JavaScript": 2},
        ),
        # skill not mentioned -> absent from the result
        ("a plain sentence", {"go": "Go"}, {}),
        # a skill must not match inside a larger word
        ("I searched it on Google", {"go": "Go"}, {}),
        # a name ending in a special char is still found
        ("I know C++", {"c++": "C++"}, {"C++": 1}),
        # the alias is written in a different case than the dict key
        ("I code in golang", {"golang": "Go"}, {"Go": 1}),
        # confidence grows with the number of mentions
        ("Go here, Go there, Go everywhere", {"go": "Go"}, {"Go": 3}),
        # distinct skills stay distinct
        ("Python and SQL", {"python": "Python", "sql": "SQL"}, {"Python": 1, "SQL": 1}),
    ],
)
def test_skills_extractor(
    text: str, skills: dict[str, str], expected: dict[str, float]
) -> None:
    assert skills_extractor(text, skills) == expected
