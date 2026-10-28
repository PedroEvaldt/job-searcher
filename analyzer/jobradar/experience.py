import re


def extract_years(text: str) -> int | None:
    m = re.search(
        r"(\d+)\+?(?:-\d+)?\s+(years|yrs)\s+(?:of\s+)?experience", text, re.IGNORECASE
    )
    if m:
        experience = int(m.group(1))
        if experience >= 20:
            return None
        return experience
    return None
