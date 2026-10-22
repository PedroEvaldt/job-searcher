# %%
import pandas as pd
from db import sql_to_dataframe

pd.set_option("display.width", 120)


def section(title: str) -> None:
    print(f"\n=== {title} ===")


# Location is not in `jobs` (D33); it lives in the raw payload of each source, under a
# different key and shape per source. Pull the live jobs with their payload.
jobs = sql_to_dataframe("""
    SELECT j.id, s.name AS source_name, r.payload
    FROM jobs j
    JOIN raw_jobs r ON j.raw_job_id = r.id
    JOIN sources s ON j.source_id = s.id
    WHERE j.duplicate_of IS NULL
    ORDER BY j.id;
""")

# %%
# --- Policy: can someone living in Brazil (UTC-3) take this job? ---
# accept = eligible for sure · reject = excluded for sure · unknown = the field can't tell
BRAZIL_TZ = -3

# Remotive free-text tokens that include a Brazil-based candidate. Matched whole, after
# splitting on commas, so "northern america" (North America, excludes Brazil) does not
# match "americas" (the Americas, includes South America).
ACCEPT_TOKENS = {
    "worldwide",
    "anywhere",
    "americas",
    "latam",
    "latin america",
    "south america",
    "brazil",
    "brasil",
}


def classify_himalayas(payload: dict) -> str:
    # Two restriction lists; use both. Open to Brazil if it lists Brazil as a country
    # OR includes the -3 offset among its accepted timezones.
    countries = {c.lower() for c in (payload.get("locationRestrictions") or [])}
    zones = payload.get("timezoneRestrictions") or []
    if not countries and not zones:
        return "unknown"  # nothing stated: can't tell (not the same as "open to all")
    if "brazil" in countries or BRAZIL_TZ in zones:
        return "accept"
    return "reject"


def classify_remotive(payload: dict) -> str:
    raw = (payload.get("candidate_required_location") or "").lower()
    tokens = {t.strip() for t in raw.split(",") if t.strip()}
    if not tokens:
        return "unknown"
    if tokens & ACCEPT_TOKENS:
        return "accept"
    return "reject"


def classify_remoteok(payload: dict) -> str:
    # `location` is the city the job was POSTED from, not where the candidate may live.
    # It does not answer the question, so every Remote OK job is a doubt.
    return "unknown"


CLASSIFIERS = {
    "himalayas": classify_himalayas,
    "remotive": classify_remotive,
    "remoteok": classify_remoteok,
}

jobs["bucket"] = jobs.apply(
    lambda row: CLASSIFIERS[row["source_name"]](row["payload"]), axis=1
)

# %%
section("jobs by source x location policy (Brazil, UTC-3)")
counts = pd.crosstab(jobs["source_name"], jobs["bucket"], margins=True)
print(counts.to_string())

section("share of each bucket within the source")
share = pd.crosstab(jobs["source_name"], jobs["bucket"], normalize="index")
print(share.round(3).to_string())

# %%
# Spot-check: a few real jobs per bucket, with the field the rule read
section("sample jobs per bucket")
FIELD = {
    "himalayas": "locationRestrictions",
    "remotive": "candidate_required_location",
    "remoteok": "location",
}
jobs["location_field"] = jobs.apply(
    lambda r: r["payload"].get(FIELD[r["source_name"]]), axis=1
)
for bucket in ("accept", "reject", "unknown"):
    print(f"\n-- {bucket} --")
    sample = jobs.loc[jobs["bucket"] == bucket, ["id", "source_name", "location_field"]]
    print(sample.head(6).to_string(index=False))

# %%
# Analysis: on Himalayas, do the two signals (country list vs -3 timezone) agree?
section("himalayas: brazil by country signal vs by timezone signal")
him = jobs[jobs["source_name"] == "himalayas"].copy()
him["by_country"] = him["payload"].apply(
    lambda p: "brazil" in {c.lower() for c in (p.get("locationRestrictions") or [])}
)
him["by_tz"] = him["payload"].apply(
    lambda p: BRAZIL_TZ in (p.get("timezoneRestrictions") or [])
)
print(pd.crosstab(him["by_country"], him["by_tz"]).to_string())
