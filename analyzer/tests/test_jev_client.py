from pathlib import Path

import pytest
from typesafe_sdk import SystemOneResponse

from jobradar.jev.client import _parse, jev_client
from jobradar.jev.questions import QUESTIONS

TESTDATA = Path(__file__).parent / "testdata" / "jev_systemone.json"


def test_parse_maps_each_question_to_its_noul_probability() -> None:
    # A real Jev response, recorded once — no network.
    resp = SystemOneResponse.model_validate_json(TESTDATA.read_text(encoding="utf-8"))
    out = _parse(resp, QUESTIONS)
    assert out.answers == {
        "open_to_latam": 0.98,
        "fully_remote": 0.94,
        "is_senior_role": 0.99,
    }
    assert out.input_tokens == 533


def test_jev_client_is_none_without_a_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("JEV_API_KEY", raising=False)
    assert jev_client() is None
