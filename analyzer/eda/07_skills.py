# %%
import sys
from collections import Counter
from pathlib import Path

from db import sql_to_dataframe

# boilerplate.py, skills.py and text.py live in analyzer/, one level above eda/
sys.path.insert(0, str(Path.cwd().parent if Path.cwd().name == "eda" else Path.cwd()))
from boilerplate import (
    BOILERPLATE_MIN_DESCRIPTIONS,
    cut_boilerplate,
    paragraphs_counter,
)
from skills import ALIASES, skills_extractor
from text import html_to_paragraphs


def skills_per_job() -> dict[int, dict[str, float]]:
    """Canonical skills found in every live job, after cutting shared boilerplate."""
    jobs = sql_to_dataframe(
        "SELECT id, description FROM jobs WHERE duplicate_of IS NULL ORDER BY id;"
    )

    paragraphs: dict[int, list[str]] = {}
    descriptions: list[str] = []
    for job_id, description in jobs.itertuples(index=False, name=None):
        paragraphs[job_id] = html_to_paragraphs(description)
        descriptions.append(description)

    # Build the boilerplate statistic once over the whole corpus, then cut per job
    counter = paragraphs_counter(descriptions)

    skills: dict[int, dict[str, float]] = {}
    for job_id, job_paragraphs in paragraphs.items():
        kept = cut_boilerplate(job_paragraphs, counter, BOILERPLATE_MIN_DESCRIPTIONS)
        skills[job_id] = skills_extractor("\n".join(kept), ALIASES)
    return skills


if __name__ == "__main__":
    job_skills = skills_per_job()

    # How many live jobs mention each skill (presence, not total mentions)
    jobs_per_skill: Counter[str] = Counter()
    for found in job_skills.values():
        jobs_per_skill.update(found.keys())

    print(f"skills across {len(job_skills)} live jobs (jobs mentioning each):")
    for skill, count in jobs_per_skill.most_common():
        print(f"{skill}: {count}")
