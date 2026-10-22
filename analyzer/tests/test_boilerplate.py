from pathlib import Path

import pytest

from boilerplate import cut_boilerplate, paragraphs_counter
from text import html_to_paragraphs


@pytest.mark.parametrize(
    ("descriptions", "paragraph", "expected"),
    [
        (["<p>Remote</p><p>Python</p>", "<p>Remote</p><p>Python</p>"], "Python", 1),
        (["<p>Remote</p><p>Go</p><p>Remote</p>"], "Remote", 1),
        (["<p>Teste</p>"], "Rust", 0),
    ],
)
def test_paragraphs_counter(
    descriptions: list[str], paragraph: str, expected: int
) -> None:
    paragraphs = paragraphs_counter(descriptions)
    assert paragraphs[paragraph] == expected


@pytest.mark.parametrize(
    ("paragraphs", "counts", "k", "expected"),
    [
        (["Go", "Python"], {"Go": 1, "Python": 1}, 2, ["Go", "Python"]),
        (["Go", "Python"], {"Go": 1, "Python": 2}, 2, ["Go"]),
        (["Python", "Go"], {"Go": 1, "Python": 1}, 2, ["Python", "Go"]),
        (
            ["Kubernetes", "Bash"],
            {"Bash": 2, "Kubernetes": 2},
            2,
            ["Kubernetes", "Bash"],
        ),
        (["Kubernetes", "Bash"], {"Kubernetes": 1}, 2, ["Kubernetes", "Bash"]),
    ],
)
def test_cut_boilerplate(
    paragraphs: list[str], counts: dict[str, int], k: int, expected: list[str]
) -> None:
    assert cut_boilerplate(paragraphs, counts, k) == expected


# Real descriptions of two Dell jobs (2026-10-19 dump): they share the team intro, a
# heading, the closing line and the Himalayas footer, and differ in the title and role
TESTDATA = Path(__file__).parent / "testdata"


def test_cut_boilerplate_dell() -> None:
    descriptions = [
        (TESTDATA / "dell_3232.html").read_text(),
        (TESTDATA / "dell_3234.html").read_text(),
    ]
    counts = paragraphs_counter(descriptions)
    # k = 2: with only two descriptions, nothing reaches the default threshold
    kept = cut_boilerplate(html_to_paragraphs(descriptions[0]), counts, k=2)
    assert kept == [
        "Account Executive - Large Enterprise",
        (
            "As an Account Executive, you will drive new-business hunting and targeted "
            "expansion in a defined territory. You will run disciplined prospecting "
            "cadences, perform discovery, and progress opportunities through the funnel. "
            "Your results will be measured by pipeline generation, conversion, and "
            "customer satisfaction."
        ),
    ]
