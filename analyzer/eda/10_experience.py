# %%
import sys
from pathlib import Path

import pandas as pd
from db import sql_to_dataframe

pd.set_option("display.width", 120)
sys.path.insert(0, str(Path.cwd().parent if Path.cwd().name == "eda" else Path.cwd()))
from boilerplate import cut_boilerplate, paragraphs_counter
from experience import extract_years
from text import html_to_paragraphs


def section(title: str) -> None:
    print(f"\n=== {title} ===")


# %%
# Required years of experience, estimated by regex on the cleaned description (html ->
# paragraphs -> boilerplate cut, the same pipeline as skills). The point is to see how
# often the rule fires (its recall is a guess, this is the ceiling) and what it extracts.
jobs = sql_to_dataframe(
    """
    SELECT j.id, s.name AS source_name, j.description
    FROM jobs j
    JOIN sources s ON j.source_id = s.id
    WHERE j.duplicate_of IS NULL
    ORDER BY j.id;
    """
)

# Boilerplate statistic once over the whole corpus, then clean each description and apply
# the regex. The cut also drops "about the company" blocks, cutting false positives like
# "founded 10 years ago".
counter = paragraphs_counter(jobs["description"])
jobs["years"] = [
    extract_years("\n".join(cut_boilerplate(html_to_paragraphs(desc), counter)))
    for desc in jobs["description"]
]

# %%
total = len(jobs)
found = int(jobs["years"].notna().sum())
section("years-of-experience detection rate")
print(f"regex found a number in {found} of {total} live jobs ({found / total:.1%})")
print("hit rate by source:")
print(jobs["years"].notna().groupby(jobs["source_name"]).mean().round(3))

# %%
section("distribution of required years (where found)")
print(jobs["years"].describe().round(1))
print(jobs["years"].value_counts().sort_index())
