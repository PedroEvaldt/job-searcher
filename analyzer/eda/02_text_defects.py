# %%
import re

import pandas as pd
from db import sql_to_dataframe

pd.set_option("display.width", 120)

# One HTML tag: opening, closing or self-closing, with or without attributes
HTML_TAG = r"<[^>]+>"
# Named (&ndash;), decimal (&#47;) or hex (&#x2f;) HTML entity
HTML_ENTITY = r"&(?:#\d{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{1,31});"
# C1 control characters: never in real text, left behind when UTF-8 is read as Latin-1
MOJIBAKE = r"[\x80-\x9f]"


def section(title: str) -> None:
    print(f"\n=== {title} ===")


def measure_defect(jobs: pd.DataFrame, column: str, pattern: str) -> pd.Series:
    """Print, by source, how many jobs have `pattern` in `column`, plus the first one."""
    mask = jobs[column].str.contains(pattern, regex=True, na=False)

    by_source = jobs.assign(defect=mask).groupby("source_name")["defect"]
    print(by_source.agg(jobs="sum", share="mean").round(4))

    first = jobs.loc[mask].iloc[0]
    match = re.search(pattern, first[column])
    assert match is not None
    snippet = first[column][max(0, match.start() - 30) : match.end() + 30]
    print(f"\nexample · job {first['id']} ({first['source_name']}): {snippet!r}")
    return mask


# %%
jobs_alive = sql_to_dataframe(
    """
    SELECT j.id, j.title, j.description, s.name AS source_name
    FROM jobs j JOIN sources s ON j.source_id = s.id
    WHERE j.duplicate_of IS NULL
    ORDER BY j.id;
    """
)


# %%
# Every description from every source is HTML: before anything reads the text, it has
# to become plain paragraphs (the boilerplate cut in 2.3 starts there).
section("HTML tag in the description")
html_in_description = measure_defect(jobs_alive, "description", HTML_TAG)


# %%
# Inside HTML, &amp; is just how '&' is written, so only the title (plain text) counts.
# Only Himalayas has it: 86 titles (0.5%), mostly &#x2f; '/', &#x28; '(' and &ndash;.
section("HTML entity in the title")
entity_in_title = measure_defect(jobs_alive, "title", HTML_ENTITY)


# %%
# 3 in 4 Remote OK descriptions arrive as mojibake (D10): UTF-8 read as Latin-1 before
# the payload reaches the collector. All 102 come back with encode("latin-1") +
# decode("utf-8"). Himalayas has 4 (same company text, mixed with real Unicode, so the
# whole-string round trip fails); Remotive none.
section("Mojibake in the description")
mojibake_in_description = measure_defect(jobs_alive, "description", MOJIBAKE)
