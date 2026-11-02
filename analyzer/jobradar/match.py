"""How well a job's skills fit the CV's skills (5.4.2).

Both sides are sets — a skill is there or not — because mention counts measure how a
text was written, not what anyone knows (D47). Pure functions, no database: the
caller brings the job's skills (from job_skills) and the CV's (from cv_skills()).
"""

from dataclasses import dataclass
from pathlib import Path

from jobradar.skills import ALIASES, skills_extractor

CV_PATH = Path(__file__).resolve().parents[2] / "cv.md"


@dataclass(frozen=True)
class SkillMatch:
    overlap: int  # |J ∩ C|: CV skills the job uses; breaks ties between equal coverages
    coverage: float  # |J ∩ C| / |J|: of what the job asks, how much the CV has (main)
    missing: int  # |J − C|: what the job asks that the CV lacks
    n_job_skills: int  # |J|: 0 means a non-technical job (nursing, sales…)


def skill_match(job_skills: set[str], cv_skills: set[str]) -> SkillMatch:
    """Compare a job's skills with the CV's. A job with no skills gets coverage 0, not
    0/0: if it asks for nothing technical, it is not a job for this CV."""
    common = job_skills & cv_skills
    n_job = len(job_skills)
    return SkillMatch(
        overlap=len(common),
        coverage=len(common) / n_job if n_job else 0.0,
        missing=len(job_skills - cv_skills),
        n_job_skills=n_job,
    )


def cv_skills(path: Path = CV_PATH) -> set[str]:
    """The canonical skills found in the CV, as a set."""
    return set(skills_extractor(path.read_text(encoding="utf-8"), ALIASES))
