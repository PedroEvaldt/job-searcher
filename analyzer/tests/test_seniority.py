import pytest

from jobradar.seniority import is_junior_title


@pytest.mark.parametrize(
    ("title", "junior"),
    [
        ("Junior Backend Developer", True),
        ("Remote Infrastructure Engineer Junior", True),
        ("Analista Designer UX / Produto – Júnior", True),
        ("Jr. Python Developer", True),
        ("Full-Stack Engineering Intern", True),
        ("Estagio em Desenvolvimento Delphi - Remote Work", True),
        ("Estagiário(a) Financeiro", True),
        ("Practicante de Datos", True),
        ("Devops Trainee", True),
        ("Entry-Level Software Engineer", True),
        ("New Grad Software Engineer", True),
        # not the entry level
        ("Senior Backend Engineer", False),
        ("Software Engineer II Golang", False),
        ("JR-213496 QA Engineer Middle", False),  # a requisition id
        ("Internal Tools Engineer", False),
        ("International Sales Manager", False),
    ],
)
def test_is_junior_title(title: str, junior: bool) -> None:
    assert is_junior_title(title) is junior
