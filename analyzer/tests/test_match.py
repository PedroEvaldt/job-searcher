import pytest

from jobradar.match import SkillMatch, cv_skills, skill_match

CV = {"Go", "Python", "Docker", "Kubernetes", "PostgreSQL"}


@pytest.mark.parametrize(
    ("job", "expected"),
    [
        # the job asks only for things the CV has -> full coverage
        (
            {"Go", "Docker", "Kubernetes", "PostgreSQL"},
            SkillMatch(overlap=4, coverage=1.0, missing=0, n_job_skills=4),
        ),
        # a broad job: 2 of 8 known -> low coverage even though 2 skills match
        (
            {"Go", "Python", "AWS", "Terraform", "Kafka", "Java", "React", "GraphQL"},
            SkillMatch(overlap=2, coverage=0.25, missing=6, n_job_skills=8),
        ),
        # no skills at all (nursing, sales): coverage 0, not a ZeroDivisionError
        (set(), SkillMatch(overlap=0, coverage=0.0, missing=0, n_job_skills=0)),
        # technical, but nothing in common
        (
            {"Java", "Spring", "Oracle"},
            SkillMatch(overlap=0, coverage=0.0, missing=3, n_job_skills=3),
        ),
    ],
)
def test_skill_match(job: set[str], expected: SkillMatch) -> None:
    assert skill_match(job, CV) == expected


def test_coverage_beats_overlap() -> None:
    # Same overlap (4), very different fit: coverage is what tells them apart.
    narrow = skill_match({"Go", "Docker", "Kubernetes", "PostgreSQL"}, CV)
    broad = skill_match(
        {"Go", "Docker", "Kubernetes", "PostgreSQL"} | {f"Other{i}" for i in range(16)},
        CV,
    )
    assert narrow.overlap == broad.overlap == 4
    assert narrow.coverage > broad.coverage


def test_cv_skills_reads_the_real_cv() -> None:
    skills = cv_skills()
    # The two skills the whole project is built around (cv.md, confirmed 2026-11-02).
    assert {"Go", "Python"} <= skills
