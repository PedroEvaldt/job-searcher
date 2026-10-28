TO_ANNUAL = {"annual": 1, "monthly": 12, "weekly": 52, "hourly": 2080}

# USD per 1 unit of each currency — a rough static snapshot, enough to rank salaries
# across currencies (not for accounting). Covers every currency seen in the data.
TO_USD = {
    "USD": 1.0,
    "EUR": 1.08,
    "GBP": 1.27,
    "CHF": 1.11,
    "CAD": 0.73,
    "AUD": 0.66,
    "SGD": 0.74,
    "ILS": 0.27,
    "PLN": 0.25,
    "BRL": 0.18,
    "CNY": 0.14,
    "DKK": 0.145,
    "HKD": 0.128,
    "SEK": 0.095,
    "MXN": 0.058,
    "ZAR": 0.055,
    "CZK": 0.043,
    "TWD": 0.031,
    "TRY": 0.031,
    "THB": 0.028,
    "PHP": 0.017,
    "INR": 0.012,
    "JPY": 0.0067,
    "HUF": 0.0028,
    "CLP": 0.0011,
    "IDR": 0.000063,
}


def normalize_salary(
    salary: float | None, currency: str | None, period: str | None
) -> float | None:
    if salary is None:
        return None
    if currency is None:
        currency = "USD"
    if period is None:
        period = "annual"
    currency_factor = TO_USD.get(currency)
    if currency_factor is None:
        return None
    period_factor = TO_ANNUAL.get(period)
    if period_factor is None:
        return None
    return salary * currency_factor * period_factor
