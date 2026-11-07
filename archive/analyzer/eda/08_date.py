# %%
from datetime import UTC, datetime

import matplotlib.pyplot as plt
import pandas as pd
from db import sql_to_dataframe

pd.set_option("display.width", 120)


def section(title: str) -> None:
    print(f"\n=== {title} ===")


# %%
# Job age = now - posted_at. posted_at was empty in `jobs` before 3a.3 (D33); the Go
# re-normalization (3a.3.2) backfilled it from the payloads in raw_jobs. This confirms
# the backfill reached every live job and shows how fresh the board is.
jobs = sql_to_dataframe(
    """
    SELECT j.id, s.name AS source_name, j.posted_at
    FROM jobs j
    JOIN sources s ON j.source_id = s.id
    WHERE j.duplicate_of IS NULL
    ORDER BY j.id;
    """
)

total = len(jobs)
with_date = int(jobs["posted_at"].notna().sum())
section("posting date coverage")
print(f"{with_date} of {total} live jobs have posted_at ({with_date / total:.1%})")
print("share with a date, by source:")
print(jobs["posted_at"].notna().groupby(jobs["source_name"]).mean().round(3))

# %%
# Age in days. Every live job has a date, so there is nothing to drop; the range is weeks,
# not years — no 1970 sentinel, because Himalayas always sent a valid pubDate in this data
# (the time.Unix(0,0) risk flagged in 3a.3 never materializes here).
jobs["age_days"] = (datetime.now(UTC) - jobs["posted_at"]).dt.days
section("job age in days (all live jobs)")
print(jobs["age_days"].describe().round(1))

plt.figure()
jobs["age_days"].plot.hist(bins=60)
plt.title("Live job age")
plt.xlabel("age (days)")
plt.ylabel("number of jobs")
plt.show()
