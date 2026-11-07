import sys

from langdetect import DetectorFactory, LangDetectException, detect_langs

from jobradar.db import DatabaseError, connect
from jobradar.text import html_to_paragraphs

# langdetect is non-deterministic unless the seed is fixed: same text -> same language.
DetectorFactory.seed = 0

LANGUAGES = {
    "af",
    "ar",
    "bg",
    "bn",
    "ca",
    "cs",
    "cy",
    "da",
    "de",
    "el",
    "en",
    "es",
    "et",
    "fa",
    "fi",
    "fr",
    "gu",
    "he",
    "hi",
    "hr",
    "hu",
    "id",
    "it",
    "ja",
    "kn",
    "ko",
    "lt",
    "lv",
    "mk",
    "ml",
    "mr",
    "ne",
    "nl",
    "no",
    "pa",
    "pl",
    "pt",
    "ro",
    "ru",
    "sk",
    "sl",
    "so",
    "sq",
    "sv",
    "sw",
    "ta",
    "te",
    "th",
    "tl",
    "tr",
    "uk",
    "ur",
    "vi",
    "zh-cn",
    "zh-tw",
}


def detect_language(text: str) -> str | None:
    """The first of the languages we track that langdetect finds, else None."""
    try:
        for guess in detect_langs(text):
            if guess.lang in LANGUAGES:
                return str(guess.lang)
        return None
    except LangDetectException:
        return None


def languages_pipeline() -> int:
    try:
        conn = connect()
    except DatabaseError as e:
        print(e, file=sys.stderr)
        return 1
    with conn, conn.cursor() as cur:
        # Resumable: only the live jobs not tagged yet.
        rows = cur.execute(
            "SELECT id, description FROM jobs "
            "WHERE duplicate_of IS NULL AND language IS NULL "
            "ORDER BY id;",
        ).fetchall()
        if not rows:
            print("no jobs to detect")
            return 0

        languages: dict[int, str] = {}
        for job_id, description in rows:
            language = detect_language("\n".join(html_to_paragraphs(description)))
            if language is not None:
                languages[job_id] = language

        with conn.transaction():
            cur.executemany(
                "UPDATE jobs SET language = %s WHERE id = %s;",
                [(language, job_id) for job_id, language in languages.items()],
            )
    print(f"tagged {len(languages)} of {len(rows)} jobs")
    return 0


if __name__ == "__main__":
    sys.exit(languages_pipeline())
