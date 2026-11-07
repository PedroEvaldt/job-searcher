"""jev-manual: the Jev answers combined with hand-picked weights.

This is the score the digest ranks by today. The weights are part of the method, so
changing one means a new model version, not a silent overwrite of the old scores.
"""

WEIGHTS = {"fully_remote": 0.7, "open_to_latam": 0.6, "is_senior_role": -0.7}

# The question set whose answers it combines.
QUESTION_SET = "qs-v1"

MODEL_VERSION = f"jev-manual-{QUESTION_SET}"


def manual_score(answers: dict[str, float]) -> float:
    """Weighted sum of the answers; a question with no weight raises KeyError, so a new
    question can never be scored without someone deciding its weight."""
    return sum(value * WEIGHTS[question] for question, value in answers.items())
