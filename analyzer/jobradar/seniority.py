"""Whether a job title says junior, intern or entry level.

Read from the title only: a description says "junior" in senior jobs too ("you will mentor
junior engineers"). Literal words, no Jev: the Jev's is_senior_role tells senior from the
rest, this tells the entry level apart from mid.
"""

import re

_JUNIOR_TITLE = re.compile(
    r"(?<!\w)(?:"
    r"junior|júnior|jr(?!-\d)\.?"  # "JR-213496" is a requisition id
    r"|intern|interns|internship"
    r"|estágio|estagio|estagiári[oa]|estagiari[oa]|aprendiz"
    r"|pasante|pasantía|pasantia|practicante|prácticas|becari[oa]"
    r"|trainee|apprentice|apprenticeship|entry[\s-]level|new grad|graduate"
    r")(?!\w)",
    re.IGNORECASE,
)


def is_junior_title(title: str) -> bool:
    return _JUNIOR_TITLE.search(title) is not None
