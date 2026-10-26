# %%
import sys
from pathlib import Path

import pandas as pd
from db import sql_to_dataframe

pd.set_option("display.width", 120)
sys.path.insert(0, str(Path.cwd().parent if Path.cwd().name == "eda" else Path.cwd()))
from salary import normalize_salary


def section(title: str) -> None:
    print(f"\n=== {title} ===")


# %%
# Salary after the Go extraction (3a.3.3). Each source fills different columns, so the
# coverage below shows what is still messy: Himalayas brings amount + currency + period,
# Remote OK only amounts (no currency/period), Remotive nothing (its salary is free text,
# left NULL on purpose — defect 6, normalized later / in Python).
jobs = sql_to_dataframe(
    """
    SELECT s.name AS source_name, j.salary_min, j.salary_max, j.currency, j.salary_period
    FROM jobs j
    JOIN sources s ON j.source_id = s.id
    WHERE j.duplicate_of IS NULL
    ORDER BY j.id;
    """
)

cols = ["salary_min", "salary_max", "currency", "salary_period"]
section("salary column coverage by source (share non-null)")
print(jobs[cols].notna().groupby(jobs["source_name"]).mean().round(3))

# %%
# The units that have to be reconciled: currencies and periods among the salaried jobs.
salaried = jobs[jobs["salary_min"].notna()]
section(f"currencies present ({len(salaried)} salaried jobs)")
print(salaried["currency"].value_counts(dropna=False))
section("periods present")
print(salaried["salary_period"].value_counts(dropna=False))

# %%
# normalize_salary (3a.4.2) turns every (amount, currency, period) into annual USD, so
# salaries from different sources/units become comparable. This is the payoff: one scale.
def _annual_usd(row: pd.Series) -> float | None:
    # The DB numeric comes back as decimal.Decimal and NULL currency/period as NaN, but
    # normalize_salary expects float and None — sanitize here, at the pandas boundary.
    currency = row["currency"] if pd.notna(row["currency"]) else None
    period = row["salary_period"] if pd.notna(row["salary_period"]) else None
    return normalize_salary(float(row["salary_min"]), currency, period)


annual_usd = salaried.apply(_annual_usd, axis=1)
section("annual USD salary floor, after normalization")
print(annual_usd.describe().round(0))
