# %%
import sys
from pathlib import Path

import pandas as pd
from db import sql_to_dataframe

# text.py lives in analyzer/, one level above eda/ (works with cwd in either one)
sys.path.insert(0, str(Path.cwd().parent if Path.cwd().name == "eda" else Path.cwd()))
from jobradar.boilerplate import (
    BOILERPLATE_MIN_DESCRIPTIONS,
    cut_boilerplate,
    paragraphs_counter,
)
from jobradar.text import html_to_paragraphs

pd.set_option("display.width", 120)

# "Repeated in k+ jobs" thresholds the cut in 2.3.3 can choose from
THRESHOLDS = (2, 5, 10, 50, 100)


def section(title: str) -> None:
    print(f"\n=== {title} ===")


jobs = sql_to_dataframe("""
    SELECT j.id AS job_id, j.company, s.name AS source_name, j.description
    FROM jobs j JOIN sources s ON s.id = j.source_id
    WHERE j.duplicate_of IS NULL;
""")

# %%
# One row per paragraph of each live job
jobs["paragraphs"] = jobs["description"].apply(html_to_paragraphs)
all_paragraphs = (
    jobs.explode("paragraphs", ignore_index=True)
    .rename(columns={"paragraphs": "paragraph"})
    .drop(columns=["description"])
)
all_paragraphs["chars"] = all_paragraphs["paragraph"].str.len()

# Distinct jobs and companies with the same text, on every row. nunique, not size: a job
# that repeats a paragraph (two "Requirements") is not boilerplate across jobs
same_text = all_paragraphs.groupby("paragraph")
all_paragraphs["jobs"] = same_text["job_id"].transform("nunique")
all_paragraphs["companies"] = same_text["company"].transform("nunique")

section("paragraphs")
print(
    f"{len(all_paragraphs):,} paragraphs in {len(jobs):,} live jobs, "
    f"{all_paragraphs['paragraph'].nunique():,} distinct"
)

# %%
# A
total_chars = all_paragraphs["chars"].sum()
section("share of chars in paragraphs found in k+ jobs")
for k in THRESHOLDS:
    partial_chars = all_paragraphs.loc[all_paragraphs["jobs"] >= k, "chars"].sum()
    print(f"k = {k:>3}: {partial_chars / total_chars:.3f}")

# A third of the text repeats in 2+ jobs, but the share falls fast with k (0.112 at 10,
# 0.035 at 50): most of it repeats among a few jobs, not across the whole base.

# %%
# B
# The chars of a paragraph found in 2+ jobs, 0 for the others: summing it per source
# gives the repeated text of each source in one pass, whatever sources exist
all_paragraphs["repeated_chars"] = all_paragraphs["chars"].where(
    all_paragraphs["jobs"] >= 2, 0
)
chars_by_source = all_paragraphs.groupby("source_name")[
    ["repeated_chars", "chars"]
].sum()
repeated_share_by_source = chars_by_source["repeated_chars"] / chars_by_source["chars"]
section("share of chars in paragraphs found in 2+ jobs, by source")
print(repeated_share_by_source.round(3).to_string())

# Himalayas (0.334) holds 99% of the chars, so it sets the overall share. Remote OK
# repeats little (0.095); Remotive's 0.263 comes from only 19 jobs.

# %%
# C
one_company_chars = all_paragraphs.loc[
    (all_paragraphs["jobs"] >= 2) & (all_paragraphs["companies"] == 1), "chars"
].sum()
# 2+ companies already means 2+ jobs
across_companies_chars = all_paragraphs.loc[
    all_paragraphs["companies"] >= 2, "chars"
].sum()
section("share of chars in paragraphs found in 2+ jobs, by who repeats them")
print(f"one company only: {one_company_chars / total_chars:.3f}")
print(f"across companies: {across_companies_chars / total_chars:.3f}")

# Of the 0.332 repeated, 0.269 stays inside one company (its "about us", EEO and team
# intros, pasted in each of its jobs) and only 0.063 crosses companies (headings and
# "Originally posted on Himalayas").

# %%
# D
# One row per text first: otherwise the top 10 is the same footer ten times
top_paragraphs = (
    all_paragraphs.drop_duplicates("paragraph")
    .nlargest(10, "jobs")[["paragraph", "jobs", "companies"]]
    .reset_index(drop=True)
)
top_paragraphs.index += 1
section("10 most repeated paragraphs")
print(top_paragraphs.to_string())

# The Himalayas footer and section headings: short texts found across hundreds of
# companies. Company texts ("about us", EEO) repeat in fewer jobs each and never reach
# the top, though together they are most of the repeated text (C).

# %%
# E
section("jobs left with no paragraph if the cut removes those found in k+ jobs")
for k in THRESHOLDS:
    # A paragraph is kept by the cut when it is found in fewer than k jobs
    all_paragraphs["kept"] = all_paragraphs["jobs"] < k
    job_keeps_text = all_paragraphs.groupby("job_id")["kept"].any()
    print(f"k = {k:>3}: {(~job_keeps_text).sum():>4}")

# Even k = 10 empties 141 jobs: a company that posts the same job many times (Speechify,
# one copy per city) has every paragraph repeated. The cut in 2.3.3 must never leave a
# job empty.

# %%
# F: the cut of 2.3.3 applied to every live job, with the default threshold
# Counted once over every live description; each job is then cut against it
counter = paragraphs_counter(jobs["description"])
jobs["paragraphs_cut"] = jobs["paragraphs"].apply(lambda p: cut_boilerplate(p, counter))
jobs["chars_before"] = jobs["paragraphs"].apply(lambda p: sum(len(w) for w in p))
jobs["chars_after"] = jobs["paragraphs_cut"].apply(lambda p: sum(len(w) for w in p))

chars_by_source = jobs.groupby("source_name")[["chars_before", "chars_after"]].sum()
cut_share_by_source = (
    chars_by_source["chars_before"] - chars_by_source["chars_after"]
) / chars_by_source["chars_before"]
total_before = jobs["chars_before"].sum()
cut_share = (total_before - jobs["chars_after"].sum()) / total_before

# Jobs where every paragraph is boilerplate: the cut gives them back whole. Counted from
# the counts, not by comparing before and after: a job with no boilerplate also comes
# back unchanged
kept_whole = jobs["paragraphs"].apply(
    lambda p: all(counter[w] >= BOILERPLATE_MIN_DESCRIPTIONS for w in p)
)

section(
    f"share of chars cut with k = {BOILERPLATE_MIN_DESCRIPTIONS} distinct descriptions"
)
print(cut_share_by_source.round(3).to_string())
print(f"total        {cut_share:.3f}")
print(f"jobs kept whole (every paragraph is boilerplate): {kept_whole.sum()}")

# The cut takes a tenth of the text, nearly all of it from Himalayas (headings, company
# texts and the footer). Remote OK and Remotive lose under 2%.
