import pytest

from jobradar.experience import extract_years


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        ("5 years of experience", 5),
        ("5+ years experience", 5),
        ("5 years experience", 5),
        ("3-5 years experience", 3),  # range -> mínimo
        ("minimum 5 years of experience required", 5),
        ("we value curiosity", None),
    ],
)
def test_extract_years(text: str, expected: int | None) -> None:
    assert extract_years(text) == expected
