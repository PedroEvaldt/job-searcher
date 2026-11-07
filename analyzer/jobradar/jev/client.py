import os
from dataclasses import dataclass
from typing import Protocol

from typesafe_sdk import (
    Noul,
    NoulAnswer,
    NoulCriteria,
    SystemOneResponse,
    TypeSafeClient,
)

from jobradar.jev.questions import Question

# "jev-latest" floats across releases, and the answers are not versioned by Jev model: if
# the provider updates it, new answers come from a different model under the same
# question_set_version. TODO: pin a concrete version (see client.models()).
JEV_MODEL = "jev-latest"


@dataclass(frozen=True)
class JevAnswers:
    answers: dict[str, float]  # question id -> probability the statement is true (0..1)
    input_tokens: int  # Jev bills input only; output is free


class JevClient(Protocol):
    def answer(self, state: str, questions: list[Question]) -> JevAnswers: ...


class TypeSafeAdapter:
    def __init__(self, api_key: str, model: str) -> None:
        self.api_key = api_key
        self.model = model

    def answer(self, state: str, questions: list[Question]) -> JevAnswers:
        # One call carries every question, so the job text (the bulk of the input
        # tokens Jev charges for) is sent once, not once per question.
        with TypeSafeClient(api_key=self.api_key, model=self.model) as client:
            resp = client.system_one(
                state=state,
                questions={
                    q.id: Noul(
                        instructions=q.instructions,
                        criteria=NoulCriteria(
                            true=q.criteria_true, false=q.criteria_false
                        ),
                    )
                    for q in questions
                },
            )
        return _parse(resp, questions)


def _parse(resp: SystemOneResponse, questions: list[Question]) -> JevAnswers:
    """Pure: pull the noul probability of each question out of a response. No network,
    so the test feeds it a recorded response straight from testdata/."""
    answers: dict[str, float] = {}
    for q in questions:
        answer = resp.answers[q.id]
        if isinstance(answer, NoulAnswer):
            answers[q.id] = answer.noul
    return JevAnswers(answers=answers, input_tokens=resp.usage.input_tokens or 0)


def jev_client() -> JevClient | None:
    """The configured Jev client, or None when no key is set — the caller then scores
    with the cosine baseline instead (project runs without a key)."""
    api_key = os.getenv("JEV_API_KEY")
    if not api_key:
        return None
    return TypeSafeAdapter(api_key, JEV_MODEL)


# --- Future adapter (not implemented) -------------------------------------------------
# A second provider can implement JevClient without touching the scorer: defapi serves the
# same Jev model over its own "Decisions" API (not OpenAI-compatible, not TypeSafe-native).
#   POST https://api.defapi.org/api/v1/decisions      (header: Authorization: Bearer <key>)
#   body:     {"model": "typesafe/jev-1.13", "state": <text>,
#              "questions": {id: {"type": "noul", "instructions": ...,
#                                 "criteria": {"true": ..., "false": ...}}}}
#   response: {"answers": {id: {"type": "noul", "noul": 0.55}},
#              "usage": {"input_tokens": ..., "output_tokens": ..., "cost": ...}}
# Unlike TypeSafe, defapi returns usage.cost directly. A DefapiAdapter would POST with httpx,
# read answers[id]["noul"], and return the same JevAnswers — the scorer would not change.
