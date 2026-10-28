# %%
import re
import sys
from collections import Counter
from pathlib import Path

import pandas as pd
from db import sql_to_dataframe

# text.py lives in analyzer/, one level above eda/ (works with cwd in either one)
sys.path.insert(0, str(Path.cwd().parent if Path.cwd().name == "eda" else Path.cwd()))
from jobradar.text import html_to_paragraphs

pd.set_option("display.width", 120)


def section(title: str) -> None:
    print(f"\n=== {title} ===")


# Function words frequent in one language and rare in the other. A full description has
# dozens of them, so a rough list separates PT from EN well enough to COUNT — this is a
# measurement, not a production detector. Ambiguous overlaps are left out on purpose
# ("do"/"no"/"as" are also common English words).
PT_WORDS = {
    "que", "não", "para", "com", "uma", "você", "são", "também", "como", "mais",
    "dos", "das", "da", "por", "ser", "sua", "seu", "está", "é", "ou", "até",
    "sem", "sobre", "já", "nós",
}
EN_WORDS = {
    "the", "and", "for", "with", "you", "are", "this", "that", "will", "your",
    "our", "have", "from", "we", "is", "of", "to", "in", "on",
}


def language(text: str) -> str:
    counts = Counter(re.findall(r"\w+", text.lower()))
    pt = sum(counts[w] for w in PT_WORDS)
    en = sum(counts[w] for w in EN_WORDS)
    if pt == en:  # covers 0 vs 0: no signal / empty text
        return "unknown"
    return "pt" if pt > en else "en"


# The accept test from 2.4.1, trimmed (duplicated on purpose: these are exploration
# scripts; 3a consolidates the location policy into one place).
BRAZIL_TZ = -3
ACCEPT_TOKENS = {
    "worldwide", "anywhere", "americas", "latam",
    "latin america", "south america", "brazil", "brasil",
}


def accepts_brazil(source: str, payload: dict) -> bool:
    if source == "himalayas":
        countries = {c.lower() for c in (payload.get("locationRestrictions") or [])}
        zones = payload.get("timezoneRestrictions") or []
        return "brazil" in countries or BRAZIL_TZ in zones
    if source == "remotive":
        raw = (payload.get("candidate_required_location") or "").lower()
        tokens = {t.strip() for t in raw.split(",") if t.strip()}
        return bool(tokens & ACCEPT_TOKENS)
    return False  # remote ok: eligibility unknown, never a sure accept


jobs = sql_to_dataframe("""
    SELECT j.id, s.name AS source_name, j.description, r.payload
    FROM jobs j
    JOIN raw_jobs r ON j.raw_job_id = r.id
    JOIN sources s ON j.source_id = s.id
    WHERE j.duplicate_of IS NULL
    ORDER BY j.id;
""")

# %%
# Plain text from the HTML description (reuse 2.3), then the language guess
jobs["text"] = jobs["description"].apply(lambda h: " ".join(html_to_paragraphs(h)))
jobs["language"] = jobs["text"].apply(language)

# %%
section("language by source (all live jobs)")
print(pd.crosstab(jobs["source_name"], jobs["language"], margins=True).to_string())

# %%
# What actually decides the PT-BR batch: PT among the jobs you could take (2.4.1 accept)
jobs["accepts"] = jobs.apply(
    lambda r: accepts_brazil(r["source_name"], r["payload"]), axis=1
)
accepted = jobs[jobs["accepts"]]
section("language within the accept bucket (jobs open to Brazil)")
print(pd.crosstab(accepted["source_name"], accepted["language"], margins=True).to_string())

# %%
section("sample of jobs guessed as PT (first 80 chars)")
pt = jobs.loc[jobs["language"] == "pt", ["id", "source_name", "text"]]
for _, row in pt.head(10).iterrows():
    print(f"{row['id']:>6} {row['source_name']:<10} {row['text'][:80]!r}")
