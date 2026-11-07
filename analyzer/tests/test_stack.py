import pytest

from jobradar.stack import description_languages, title_languages


@pytest.mark.parametrize(
    ("title", "expected"),
    [
        ("Golang Developer", {"Go"}),
        ("Desenvolvedor GO Pleno", {"Go"}),
        ("Programador - COBOL", {"COBOL"}),
        ("Remote Backend Django Engineer", {"Python"}),
        ("Senior C# / .NET Engineer", {"C#"}),
        ("C++ Developer", {"C++"}),
        ("Node.js Backend Developer", {"JavaScript"}),
        ("React Native Developer", {"JavaScript"}),
        ("Full Stack Developer (Golang-Angular)", {"Go", "JavaScript"}),
        # everyday words and look-alikes are not languages
        ("Go-To-Market Engineer", set()),
        ("Go Live Specialist", set()),
        ("Google Cloud Engineer", set()),
        ("Data Analyst", set()),
    ],
)
def test_title_languages(title: str, expected: set[str]) -> None:
    assert title_languages(title) == expected


@pytest.mark.parametrize(
    ("html", "expected"),
    [
        ("<p>You're productive in Go, PostgreSQL and Kubernetes</p>", {"Go"}),
        ("<p>Experiencia en COBOL y JCL.</p>", {"COBOL"}),
        ("<ul><li>Linguagem de Programação Delphi;</li></ul>", {"Delphi"}),
        # in a description the English words only count capitalised
        ("<p>Projects that go beyond your role, ready to go</p>", set()),
        ("<p>You react to alerts and fix them</p>", set()),
        ("<p>Build UIs in React and TypeScript</p>", {"JavaScript", "TypeScript"}),
        # a file name is not the language
        ("<p>Edit main.go and deploy</p>", set()),
        ("<p>Java/J2EE and Spring Boot</p>", {"Java"}),
    ],
)
def test_description_languages(html: str, expected: set[str]) -> None:
    assert description_languages(html) == expected
