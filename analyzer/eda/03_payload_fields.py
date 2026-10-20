# %%
import pandas as pd
from db import sql_to_dataframe

pd.set_option("display.width", 120)

# Payload keys measured per source, in the order: date, location, salary, tags
FIELDS = {
    "himalayas": [
        "pubDate",
        "locationRestrictions",
        "minSalary",
        "maxSalary",
        "currency",
        "salaryPeriod",
        "categories",
    ],
    "remoteok": ["date", "location", "salary_min", "salary_max", "tags"],
    "remotive": ["publication_date", "candidate_required_location", "salary", "tags"],
}
# Values a source sends in place of "not informed": the key is there, the data is not
SENTINELS = ("", 0)


def section(title: str) -> None:
    print(f"\n=== {title} ===")


def source_payloads(jobs: pd.DataFrame, source_name: str) -> pd.DataFrame:
    """One row per job of `source_name`, one column per payload key."""
    payloads = jobs.loc[jobs["source_name"] == source_name, "payload"]
    return pd.json_normalize(payloads)


def is_informed(value: object) -> bool:
    """False for null, an empty list or a sentinel; True for anything else."""
    # Lists first: pd.isna on a list returns an array, and `if` cannot judge an array
    if isinstance(value, list):
        return len(value) > 0
    if pd.isna(value):
        return False
    return value not in SENTINELS


def fill_rates(payloads: pd.DataFrame, columns: list[str]) -> pd.DataFrame:
    """Share of jobs where each column is present (not null) and informed (not empty)."""
    fields = payloads[columns]
    rates = pd.DataFrame(
        {"present": fields.notna().mean(), "informed": fields.map(is_informed).mean()}
    )
    return rates.round(3)


# %%
jobs_alive = sql_to_dataframe(
    """
    SELECT j.id, s.name AS source_name, r.payload
    FROM jobs j
    JOIN raw_jobs r ON j.raw_job_id = r.id
    JOIN sources s ON j.source_id = s.id
    WHERE j.duplicate_of IS NULL
    ORDER BY j.id;
    """
)


# %%
# Date: informed in every job of every source, in three formats: Himalayas sends epoch
# seconds (pubDate), Remote OK ISO 8601 with offset (date, plus epoch) and Remotive ISO
# without timezone (publication_date). Job age (3a) is computable for all of them.
#
# Location: Himalayas sends a list of countries (97% non-empty, 432 [] jobs; "United
# States" alone in 8.9k). Remote OK is free text, "" in 58 of 136, mixing cities,
# "Remote" and "Remoto". Remotive is free text of regions ("Worldwide", "USA, UK, ...").
#
# Salary: Himalayas has min/max in 41% (null when absent, no sentinel). salaryPeriod is
# always set ("annual" even with no amount) and currency comes alone in 1.2k jobs, so
# only the amounts count; they go from 0.59 to 120M, so period and currency must be
# normalized before comparing. Remote OK has amounts in 24 of 136 (0 = not informed, no
# currency key). Remotive is free text in 11 of 19 ("$90 - $150 /hour", "$20k -$35k",
# "OTE $25k - $35k"): it needs parsing.
#
# Tags: nearly always there (Himalayas 1.000, Remote OK 0.985, Remotive 0.947), but they
# mean different things: Himalayas sends role slugs ("Software-Engineer"), Remote OK
# mixes broad labels and skills ("exec", "golang"), Remotive sends skills ("python").
for source_name, columns in FIELDS.items():
    payloads = source_payloads(jobs_alive, source_name)
    section(f"{source_name} · {len(payloads)} live jobs")
    print(fill_rates(payloads, columns))

# %%
# posted_at, remote_scope and the salary columns of `jobs` are empty in every source
# (D33): the payloads above carry the date in 100%, the location in 57-100% and the
# salary in 18-58%, but the collector never writes them. The data is in raw_jobs, so
# filling these columns is a re-normalization, not a re-collection.
jobs = sql_to_dataframe(
    """
    SELECT s.name AS source_name, j.posted_at, j.remote_scope, j.salary_min, j.salary_max, j.currency
    FROM jobs j
    JOIN sources s ON j.source_id = s.id
    WHERE j.duplicate_of IS NULL
    ORDER BY j.id;
    """
)
section("jobs columns filled, by source")
print(
    jobs[["posted_at", "remote_scope", "salary_min", "salary_max", "currency"]]
    .notna()
    .groupby(jobs["source_name"])
    .mean()
)
