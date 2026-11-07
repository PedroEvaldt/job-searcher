"""Find and cut boilerplate: paragraphs repeated across many job descriptions."""

from collections import Counter
from collections.abc import Iterable, Mapping

from jobradar.text import html_to_paragraphs

# A paragraph found in this many distinct descriptions or more is boilerplate. Measured
# on the live jobs (2026-10-21, see archive/docs/PROGRESS.md): it cuts 10.2% of the text and empties
# only 2 jobs; at 5, 362 jobs would keep less than 10% of their text.
BOILERPLATE_MIN_DESCRIPTIONS = 10


def paragraphs_counter(htmls: Iterable[str]) -> Counter[str]:
    """In how many distinct descriptions each paragraph appears."""
    counts: Counter[str] = Counter()
    # Identical descriptions count once: the same job posted many times is not
    # boilerplate across jobs
    for html in set(htmls):
        # A paragraph repeated inside one description counts once too
        counts.update(set(html_to_paragraphs(html)))
    return counts


def cut_boilerplate(
    paragraphs: list[str],
    counts: Mapping[str, int],
    k: int = BOILERPLATE_MIN_DESCRIPTIONS,
) -> list[str]:
    """The paragraphs of one job found in fewer than `k` descriptions, in order.

    A job left with no paragraph comes back whole: no text reaches 3a empty.
    """
    # A paragraph missing from `counts` is new text (a job counted later), not boilerplate
    remaining = [p for p in paragraphs if counts.get(p, 0) < k]
    if len(remaining) == 0:
        return paragraphs
    return remaining


def clean_descriptions(corpus: list[str], descriptions: list[str]) -> list[str]:
    """Each of `descriptions` as one clean string: mojibake fixed and HTML stripped
    (via html_to_paragraphs) and shared boilerplate cut. The boilerplate statistic is
    built over `corpus` (the whole live set), so the cut does not change with the subset
    being cleaned. Lives here, not in text.py, because text.py is the lower layer."""
    counts = paragraphs_counter(corpus)
    cleaned: list[str] = []
    for description in descriptions:
        kept = cut_boilerplate(html_to_paragraphs(description), counts)
        cleaned.append("\n".join(kept))
    return cleaned
