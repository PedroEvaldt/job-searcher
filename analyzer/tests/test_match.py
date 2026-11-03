import pytest

from jobradar.match import SkillMatch, cv_skills, skill_match, skills_gate_score

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


IN_GATE = {"fully_remote": 0.9, "open_to_latam": 0.8, "is_senior_role": 0.1}
FULL = SkillMatch(overlap=4, coverage=1.0, missing=0, n_job_skills=4)


@pytest.mark.parametrize(
    ("answers", "passes"),
    [
        (IN_GATE, True),
        ({**IN_GATE, "fully_remote": 0.4}, False),  # not remote
        ({**IN_GATE, "open_to_latam": 0.2}, False),  # not open to LATAM
        ({**IN_GATE, "is_senior_role": 0.7}, False),  # senior
        ({**IN_GATE, "is_senior_role": 0.5}, False),  # the cutoff itself is senior
        ({**IN_GATE, "fully_remote": 0.5}, True),  # …and remote at exactly 0.5
        ({"fully_remote": 0.9, "open_to_latam": 0.9}, False),  # missing answer fails
    ],
)
def test_skills_gate_filter(answers: dict[str, float], passes: bool) -> None:
    assert (skills_gate_score(answers, FULL) > 0) is passes


def test_skills_gate_orders_by_coverage_then_overlap() -> None:
    high_cov = SkillMatch(overlap=1, coverage=1.0, missing=0, n_job_skills=1)
    lower_cov_more_overlap = SkillMatch(
        overlap=11, coverage=0.917, missing=1, n_job_skills=12
    )
    same_cov_more_overlap = SkillMatch(
        overlap=4, coverage=1.0, missing=0, n_job_skills=4
    )
    s = {
        name: skills_gate_score(IN_GATE, m)
        for name, m in [
            ("high_cov", high_cov),
            ("lower_cov", lower_cov_more_overlap),
            ("same_cov", same_cov_more_overlap),
        ]
    }
    # overlap never beats a higher coverage…
    assert s["high_cov"] > s["lower_cov"]
    # …but breaks the tie between equal coverages
    assert s["same_cov"] > s["high_cov"]


def test_skills_gate_non_technical_job_scores_zero() -> None:
    nothing = SkillMatch(overlap=0, coverage=0.0, missing=0, n_job_skills=0)
    assert skills_gate_score(IN_GATE, nothing) == 0.0
