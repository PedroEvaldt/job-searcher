from dataclasses import dataclass

QUESTION_SET_VERSION = "qs-v1"


@dataclass(frozen=True)
class Question:
    id: str
    instructions: str
    criteria_true: str
    criteria_false: str


QUESTIONS = [
    Question(
        "open_to_latam",
        "Decide whether the posting makes applicants located in Latin America or Brazil eligible.",
        criteria_true="Open worldwide / anywhere / no country restriction, or explicitly includes Latin America or Brazil.",
        criteria_false="Restricted to a region that excludes Latin America (e.g. US-only, EU-based), or no location stated.",
    ),
    Question(
        "fully_remote",
        "Decide whether the role is fully remote with no requirement to work on-site.",
        criteria_true="Fully remote, with no requirement to ever work on-site.",
        criteria_false="Hybrid, occasional on-site required, or the posting does not say it is remote.",
    ),
    Question(
        "is_senior_role",
        "Decide whether the posting frames the role as senior level or above, judging only from how it describes the role, not from the years of experience required.",
        criteria_true="Uses a label like senior/staff/principal/lead, or describes owning large scope or leading/mentoring others.",
        criteria_false="Junior or mid level, or no level is stated.",
    ),
]
