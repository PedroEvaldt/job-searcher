# %%

import matplotlib.pyplot as plt
import pandas as pd
from db import sql_to_dataframe

# Wide tables (describe) print whole instead of being cut with "..."
pd.set_option("display.max_columns", None)
pd.set_option("display.width", 120)


def section(title: str) -> None:
    print(f"\n=== {title} ===")


# %%
jobs_alive = sql_to_dataframe(
    """
        SELECT j.*, s.name source_name
        FROM jobs j JOIN sources s ON j.source_id = s.id 
        WHERE j.duplicate_of IS NULL 
        ORDER BY  j.id;
    """
)
job_versions = sql_to_dataframe(
    "SELECT source_id, external_id, fetched_at FROM raw_jobs;"
)


first_fetch = job_versions.groupby(["source_id", "external_id"], as_index=False)[
    "fetched_at"
].min()
alive_with_date = jobs_alive.merge(
    first_fetch, on=["source_id", "external_id"], how="inner", validate="one_to_one"
)


# %%
section("Overview")
lines, columns = jobs_alive.shape
print(f"rows: {lines} · columns: {columns}\n")
print(jobs_alive.dtypes, "\n")
print(jobs_alive["source_name"].value_counts())


# %%
# Medians are close across sources (~4-5k chars), but the tail is long (max ~33-36k)
# and pulls the means up: Remotive's mean (9.8k) is twice its median (5k), on 19 jobs.
# Sizes count HTML tags too: every Himalayas description is HTML.
section("Description size (chars) by source")
jobs_alive["description_size"] = jobs_alive["description"].str.len()
print(jobs_alive.groupby("source_name")["description_size"].describe())

plt.figure()
jobs_alive["description_size"].plot.hist(bins=60)
plt.title("Description size, live jobs")
plt.xlabel("characters")
plt.ylabel("jobs")
plt.show()


# %%
# Two companies hold ~230 jobs each, and several of the top 15 look like intermediaries
# (consultancies, talent marketplaces) rather than the hiring company. Many jobs from
# one company means repeated text: what the boilerplate cut (2.3) will measure.
section("Top 15 companies (live jobs)")
print(jobs_alive["company"].value_counts().head(15))


# %%
# ~1.1-1.6k new jobs per full day, almost flat. The flatness is the collector, not the
# market: Himalayas stops at 20 pages (400 jobs) per cycle, and in most cycles 370-400
# of those 400 are new, so the cap is binding and some jobs are never seen.
# 10-08 (first day) and 10-19 (dump day) are partial.
section("New jobs per day (first fetch, UTC)")
print(
    alive_with_date["fetched_at"]
    .dt.strftime("%m-%d")
    .value_counts()
    .sort_index(ascending=False)
)
