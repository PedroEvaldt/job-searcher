"""The questions the Jev answers about each job, grouped in versioned sets.

Answers are stored per set (jev_answers.question_set_version), so a new set never
overwrites the old one's answers. Changing any question means a new set.
"""

from dataclasses import dataclass


@dataclass(frozen=True)
class Question:
    id: str
    instructions: str
    criteria_true: str
    criteria_false: str


OPEN_TO_LATAM = Question(
    "open_to_latam",
    "Decide whether the posting makes applicants located in Latin America or Brazil eligible.",
    criteria_true="Open worldwide / anywhere / no country restriction, or explicitly includes Latin America or Brazil.",
    criteria_false="Restricted to a region that excludes Latin America (e.g. US-only, EU-based), or no location stated.",
)
FULLY_REMOTE = Question(
    "fully_remote",
    "Decide whether the role is fully remote with no requirement to work on-site.",
    criteria_true="Fully remote, with no requirement to ever work on-site.",
    criteria_false="Hybrid, occasional on-site required, or the posting does not say it is remote.",
)
IS_SENIOR_ROLE = Question(
    "is_senior_role",
    "Decide whether the posting frames the role as senior level or above, judging only from how it describes the role, not from the years of experience required.",
    criteria_true="Uses a label like senior/staff/principal/lead, or describes owning large scope or leading/mentoring others.",
    criteria_false="Junior or mid level, or no level is stated.",
)
# qs-v2: open_to_latam says yes to a job open to one Latin American country only
# (Mexico-only, Colombia-only), which someone living in Brazil cannot take.
OPEN_TO_BRAZIL = Question(
    "open_to_brazil",
    "Decide whether someone who lives in Brazil can apply for this job and do it from Brazil.",
    criteria_true="Open worldwide / anywhere / with no country restriction, open to all of Latin America or the Americas, or explicitly lists Brazil among the allowed countries.",
    criteria_false="Restricted to countries or regions that do not include Brazil (e.g. US-only, EU-only, Mexico-only, Colombia-only, India-only), requires living in or relocating to another country, or no location is stated.",
)
# qs-v2: most jobs are not engineering at all (sales, healthcare, support…), and skills
# found in the text only hint at that.
IS_TARGET_ROLE = Question(
    "is_target_role",
    "Decide whether the job is a hands-on technical role in backend or software engineering, DevOps / infrastructure / SRE / platform engineering, or data / AI / machine learning.",
    criteria_true="The day-to-day work is writing backend or full-stack code, running infrastructure or platforms, or building data pipelines, analyses or ML/AI systems.",
    criteria_false="Sales, marketing, recruiting, customer support, QA or testing, security analysis, design, content writing, management without hands-on technical work, frontend-only or mobile-only development, or any non-technical role.",
)

# qs-v3: a hard requirement of a completed degree rules a job out (stated 2026-11-07);
# "or equivalent experience", an in-progress degree or current students do not.
REQUIRES_DEGREE = Question(
    "requires_degree",
    "Decide whether the posting makes a completed university degree (bachelor's or higher) a hard requirement, with no alternative accepted.",
    criteria_true="Lists a completed bachelor's, master's or other university degree as required, and accepts no alternative to it.",
    criteria_false="No degree is mentioned; a degree is only preferred or a plus; equivalent experience or any other alternative is accepted; or current students, an in-progress degree or self-taught candidates are accepted.",
)

# qs-v4: OPEN_TO_BRAZIL read a remote job that names no country as closed ("no location
# is stated" was a false criterion), but such a job is open to anyone (stated 2026-11-07).
OPEN_TO_BRAZIL_V2 = Question(
    "open_to_brazil",
    "Decide whether someone who lives in Brazil can apply for this job and do it from Brazil.",
    criteria_true="Open worldwide / anywhere, open to all of Latin America or the Americas, explicitly lists Brazil, or the job is remote and the posting states no country or region restriction at all.",
    criteria_false="Restricted to countries or regions that do not include Brazil (e.g. US-only, EU-only, Mexico-only, Colombia-only, India-only), requires living in, being authorized to work in or relocating to another country, or the job is on-site or hybrid somewhere outside Brazil.",
)

QUESTION_SETS: dict[str, list[Question]] = {
    "qs-v1": [OPEN_TO_LATAM, FULLY_REMOTE, IS_SENIOR_ROLE],
    "qs-v2": [
        OPEN_TO_LATAM,
        OPEN_TO_BRAZIL,
        FULLY_REMOTE,
        IS_SENIOR_ROLE,
        IS_TARGET_ROLE,
    ],
    "qs-v3": [
        OPEN_TO_LATAM,
        OPEN_TO_BRAZIL,
        FULLY_REMOTE,
        IS_SENIOR_ROLE,
        IS_TARGET_ROLE,
        REQUIRES_DEGREE,
    ],
    "qs-v4": [
        OPEN_TO_LATAM,
        OPEN_TO_BRAZIL_V2,
        FULLY_REMOTE,
        IS_SENIOR_ROLE,
        IS_TARGET_ROLE,
        REQUIRES_DEGREE,
    ],
}
