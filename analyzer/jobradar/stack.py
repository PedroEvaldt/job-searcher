"""What a job is written in: the programming languages named in its title and its text.

Kept apart from skills.ALIASES on purpose. job_skills feeds every model version's
features, so changing that dictionary would change the features of versions already
measured; this one only feeds the features that read it (main_other_stack,
title_cv_stack). It also knows what the skills dictionary misses: COBOL, Delphi, ABAP,
and frameworks that name their language ("Django Developer" is a Python job).

The canonical names match skills.ALIASES where both know a language, so the CV's
languages are its skills (match.cv_skills) that are in STACK_LANGUAGES.
"""

import re

from jobradar.text import html_to_paragraphs

# canonical language -> its written forms, regular expressions matched ignoring case.
LANGUAGE_FORMS: dict[str, tuple[str, ...]] = {
    "Python": ("python", "django", "flask", "fastapi"),
    "Go": ("golang",),
    "Java": ("java", "j2ee", "spring boot"),
    "JavaScript": (
        "javascript",
        "js",
        r"node\.?js",
        "node js",
        r"react\.?js",
        "react native",
        "angular",
        r"vue(?:\.?js)?",
        r"next\.?js",
    ),
    "TypeScript": ("typescript",),
    "C#": ("c#", "csharp", r"\.net", "dotnet", r"asp\.net"),
    "C++": (r"c\+\+", "cpp"),
    "Ruby": ("ruby", "rails"),
    "PHP": ("php", "laravel", "symfony"),
    "Rust": ("rust",),
    "Kotlin": ("kotlin",),
    "Swift": ("swift",),
    "Scala": ("scala",),
    "Elixir": ("elixir",),
    "Erlang": ("erlang",),
    "Haskell": ("haskell",),
    "Clojure": ("clojure",),
    "F#": ("f#",),
    "Dart": ("dart", "flutter"),
    "Objective-C": ("objective-c",),
    "Perl": ("perl",),
    "Lua": ("lua",),
    "Groovy": ("groovy",),
    "Fortran": ("fortran",),
    "MATLAB": ("matlab",),
    "COBOL": ("cobol",),
    "Delphi": ("delphi",),
    "ABAP": ("abap",),
    "Visual Basic": (r"vb\.net", "vba", "visual basic"),
    "PL/SQL": ("pl/sql", "plsql"),
    "Solidity": ("solidity",),
}

# Forms that are also everyday English ("go beyond", "react to an alert"). A title
# rarely means the word, so there they match in any case; a description often does,
# so there they only count capitalised ("Go", "React"). "Go-to-market" and the like
# never count.
WORD_FORMS: dict[str, tuple[str, ...]] = {
    "Go": (r"go(?![\s-]+(?:to|live|getter|beyond|above|ahead|back|for)\b)",),
    "JavaScript": ("react",),
}

STACK_LANGUAGES = frozenset(LANGUAGE_FORMS) | frozenset(WORD_FORMS)


def _compile(forms: tuple[str, ...]) -> re.Pattern[str]:
    # A form must not touch a word character, nor the "+", "#" or "." that languages
    # are spelled with: "golang" is not "go", "main.go" is a file, not the language.
    return re.compile(rf"(?<![\w+#.])(?:{'|'.join(forms)})(?![\w+#])", re.IGNORECASE)


_LANGUAGE_PATTERNS = {lang: _compile(forms) for lang, forms in LANGUAGE_FORMS.items()}
_WORD_PATTERNS = {lang: _compile(forms) for lang, forms in WORD_FORMS.items()}


def _named(text: str) -> set[str]:
    return {
        lang for lang, pattern in _LANGUAGE_PATTERNS.items() if pattern.search(text)
    }


def title_languages(title: str) -> set[str]:
    """The languages a job title names."""
    words = {lang for lang, pattern in _WORD_PATTERNS.items() if pattern.search(title)}
    return _named(title) | words


def description_languages(html: str) -> set[str]:
    """The languages a job description (HTML) names. Read without the boilerplate cut,
    which needs the whole live set: a language in a company's shared paragraph is
    still that company's stack."""
    text = "\n".join(html_to_paragraphs(html))
    words = {
        lang
        for lang, pattern in _WORD_PATTERNS.items()
        if any(m.group()[0].isupper() for m in pattern.finditer(text))
    }
    return _named(text) | words
