import pytest

from salary import normalize_salary


@pytest.mark.parametrize(
    ("salary", "currency", "period", "expected"),
    [
        # period -> annual (hourly would be 14_000 with the old 280 bug)
        (50.0, "USD", "hourly", 104_000.0),
        (5_000.0, "USD", "monthly", 60_000.0),
        (1_000.0, "USD", "weekly", 52_000.0),
        (120_000.0, "USD", "annual", 120_000.0),
        # currency -> USD (expected written as the same float op the function does,
        # so it matches exactly without approx)
        (1_000.0, "EUR", "annual", 1_000.0 * 1.08),
        # missing currency/period -> assume USD annual (the Remote OK case)
        (1_000.0, None, "annual", 1_000.0),
        (1_000.0, "USD", None, 1_000.0),
        # no amount -> None
        (None, "USD", "annual", None),
        # unknown currency or period -> None (never guess)
        (1_000.0, "XYZ", "annual", None),
        (1_000.0, "USD", "fortnightly", None),
    ],
)
def test_normalize_salary(
    salary: float | None,
    currency: str | None,
    period: str | None,
    expected: float | None,
) -> None:
    assert normalize_salary(salary, currency, period) == expected
