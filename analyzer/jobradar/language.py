"""Which language a job description is written in, with langdetect."""

from langdetect import DetectorFactory, LangDetectException, detect_langs

# langdetect is non-deterministic unless the seed is fixed: same text -> same language.
DetectorFactory.seed = 0

# The languages langdetect knows; anything else it guesses is ignored.
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
    """The first of LANGUAGES that langdetect finds in text, else None."""
    try:
        for guess in detect_langs(text):
            if guess.lang in LANGUAGES:
                return str(guess.lang)
        return None
    except LangDetectException:
        return None
