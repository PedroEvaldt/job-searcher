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


SKILLS_GATE_VERSION = "skills-gate-v1"

# Fixed before looking at the test set (5.4, 2026-11-02). Tuning it on test-set-v2
# would turn the test set into training data.
GATE_CUTOFF = 0.5

# Coverage is rounded to 3 decimals and overlap added at 1e-5 per skill (≤ 12 CV skills
# → < 0.0002), so overlap can only break ties between equal coverages, never reorder
# two different ones.
_COVERAGE_DECIMALS = 3
_OVERLAP_STEP = 1e-5


def skills_gate_score(answers: dict[str, float], match: SkillMatch) -> float:
    """skills-gate-v1: the Jev answers decide WHO gets in, the skills decide the ORDER.

    A job passes the gate when Jev thinks it is fully remote, open to LATAM and not a
    senior role (each probability against GATE_CUTOFF); a missing answer fails it.
    Outside the gate the score is 0. Inside, it is the coverage, with overlap as the
    tie-break.
    """
    passes = (
        answers.get("fully_remote", 0.0) >= GATE_CUTOFF
        and answers.get("open_to_latam", 0.0) >= GATE_CUTOFF
        and answers.get("is_senior_role", 1.0) < GATE_CUTOFF
    )
    if not passes:
        return 0.0
    return round(match.coverage, _COVERAGE_DECIMALS) + match.overlap * _OVERLAP_STEP


def cv_skills(path: Path = CV_PATH) -> set[str]:
    """The canonical skills found in the CV, as a set."""
    return set(skills_extractor(path.read_text(encoding="utf-8"), ALIASES))
