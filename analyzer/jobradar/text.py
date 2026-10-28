"""Turn job descriptions from HTML into paragraphs of plain text."""

from html.parser import HTMLParser

# Tags that end the current paragraph, opening or closing (measured in the descriptions
# of the three sources). Any other tag (strong, a, span...) is inline: only its text
# counts.
BLOCK_TAGS = {
    "p",
    "div",
    "blockquote",
    "ul",
    "ol",
    "li",
    "h1",
    "h2",
    "h3",
    "h4",
    "h5",
    "h6",
    "br",
    "hr",
}

# Invisible characters found in the descriptions (Unicode category Cf). str.split() does
# not treat them as spaces, so a paragraph made only of them looked non-empty (D43).
# A translate table that deletes them.
REMOVE_INVISIBLE = str.maketrans(
    "",
    "",
    (
        "\u200b"  # zero-width space
        "\u200c"  # zero-width non-joiner
        "\u200d"  # zero-width joiner
        "\u200e"  # left-to-right mark
        "\u2060"  # word joiner
        "\ufeff"  # byte order mark
        "\u00ad"  # soft hyphen
    ),
)


class _ParagraphCollector(HTMLParser):
    """Collects the text between block tags, one paragraph at a time."""

    def __init__(self) -> None:
        super().__init__()
        self.paragraphs: list[str] = []
        # Text of the paragraph being read, in the pieces the parser hands over
        self._pieces: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag in BLOCK_TAGS:
            self._end_paragraph()

    def handle_data(self, data: str) -> None:
        self._pieces.append(data)

    def handle_endtag(self, tag: str) -> None:
        if tag in BLOCK_TAGS:
            self._end_paragraph()

    def close(self) -> None:
        super().close()
        # Text after the last block tag is a paragraph too ("a<br>b")
        self._end_paragraph()

    def _end_paragraph(self) -> None:
        # Join first, then normalize: an inline tag can split a word ("foo<b>bar</b>")
        text = " ".join("".join(self._pieces).translate(REMOVE_INVISIBLE).split())
        if text:
            self.paragraphs.append(text)
        self._pieces = []


def html_to_paragraphs(html: str) -> list[str]:
    """The paragraphs of `html` as plain text, in order and without empty ones."""
    collector = _ParagraphCollector()
    collector.feed(html)
    collector.close()
    return collector.paragraphs
