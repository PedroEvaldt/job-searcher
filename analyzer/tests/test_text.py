import pytest

from text import html_to_paragraphs

# Real descriptions (2026-10-19 dump), cut right after a closing tag

# <strong> opening a paragraph, and a paragraph that is only <strong>
REMOTEOK_JOB_5 = (
    "<p>Leverage Live Local prepares and manages Florida Live Local Act property tax "
    "exemptions for multifamily owners. We represent more than 90 properties and "
    "about 30,000 units. Demand is not our constraint. Operations is. This seat owns "
    "the operating system that lets us grow without breaking.</p><p><strong>What's in "
    "it for you.</strong> A base of 180,000 dollars with upside tied to the numbers "
    "you own, full compensation likely 250,000 to 350,000 dollars when the role is "
    "performing. Direct weekly access to the founder. Twenty days of paid time off "
    "plus federal holidays. Fully remote.</p><p><strong>What you own</strong></p>"
)
REMOTEOK_JOB_5_PARAGRAPHS = [
    (
        "Leverage Live Local prepares and manages Florida Live Local Act property tax "
        "exemptions for multifamily owners. We represent more than 90 properties and "
        "about 30,000 units. Demand is not our constraint. Operations is. This seat "
        "owns the operating system that lets us grow without breaking."
    ),
    (
        "What's in it for you. A base of 180,000 dollars with upside tied to the "
        "numbers you own, full compensation likely 250,000 to 350,000 dollars when the "
        "role is performing. Direct weekly access to the founder. Twenty days of paid "
        "time off plus federal holidays. Fully remote."
    ),
    "What you own",
]

# class/style attributes, and a paragraph holding only a non-breaking space
REMOTIVE_JOB_100 = (
    '<p class="MsoNormal" style="margin: 0in 0in 8pt; line-height: 15.6933px; '
    'color: #000000;">Unió Digital is an Arizona-based managed service provider (MSP) '
    "delivering Managed IT Services, Low Voltage Cabling, Access Control, Video "
    "Surveillance, and Intrusion Services. We believe technology should be "
    "intuitive, not intimidating.</p>\n<p>\xa0</p>\n"
    '<p class="MsoNormal" style="margin: 0in 0in 8pt; line-height: 15.6933px; '
    'color: #000000;">We are looking for an experienced Tier 3 Service Desk '
    "Technician with exceptional problem-solving skills. You will serve as the "
    "escalation point within the team, resolving complex technical issues while also "
    "leading client onboarding and project initiatives. You are expected to be "
    "involved in direct end-user support when required. If you have a passion for "
    "ensuring top-quality user support and thrive in a collaborative environment, "
    "this role is for you.</p>"
)
REMOTIVE_JOB_100_PARAGRAPHS = [
    (
        "Unió Digital is an Arizona-based managed service provider (MSP) delivering "
        "Managed IT Services, Low Voltage Cabling, Access Control, Video Surveillance, "
        "and Intrusion Services. We believe technology should be intuitive, not "
        "intimidating."
    ),
    (
        "We are looking for an experienced Tier 3 Service Desk Technician with "
        "exceptional problem-solving skills. You will serve as the escalation point "
        "within the team, resolving complex technical issues while also leading client "
        "onboarding and project initiatives. You are expected to be involved in direct "
        "end-user support when required. If you have a passion for ensuring "
        "top-quality user support and thrive in a collaborative environment, this role "
        "is for you."
    ),
]

# A line break between <ul> and <li>, which must not become a paragraph
HIMALAYAS_JOB_120 = (
    "<p>Join Our Team at Magical Destinations Travel Your Gateway to a Rewarding "
    "Career in Travel!</p><p>Are you passionate about your clients and delivering "
    "outstanding customer service? Magical Destinations Travel is on the lookout for "
    "a Travel Advisor to enhance our dedicated team. We pride ourselves on creating a "
    "professional, supportive, and collaborative environment, where you can grow, "
    "build lasting relationships, and continuously learn.</p><h3>What We Offer:</h3>"
    "<ul>\n<li>Multiple Training Opportunities: We believe in nurturing our teams "
    "potential with extensive training support.</li>"
)
HIMALAYAS_JOB_120_PARAGRAPHS = [
    (
        "Join Our Team at Magical Destinations Travel Your Gateway to a Rewarding "
        "Career in Travel!"
    ),
    (
        "Are you passionate about your clients and delivering outstanding customer "
        "service? Magical Destinations Travel is on the lookout for a Travel Advisor "
        "to enhance our dedicated team. We pride ourselves on creating a professional, "
        "supportive, and collaborative environment, where you can grow, build lasting "
        "relationships, and continuously learn."
    ),
    "What We Offer:",
    (
        "Multiple Training Opportunities: We believe in nurturing our teams potential "
        "with extensive training support."
    ),
]


@pytest.mark.parametrize(
    ("html", "expected"),
    [
        ("a<br>b<br><br>c&nbsp; d", ["a", "b", "c d"]),
        ("<p>Go <strong>and</strong> Python &amp; SQL</p>", ["Go and Python & SQL"]),
        ("<ul><li>one</li><li>two</li></ul>", ["one", "two"]),
        ("<p>foo<strong>bar</strong></p>", ["foobar"]),
        ("<p>R<sup>2</sup> score</p>", ["R2 score"]),
        ("Intro<h3>Benefits</h3>Remote", ["Intro", "Benefits", "Remote"]),
        # Invisible characters (D43): a paragraph of only them is empty, and a soft
        # hyphen inside a word goes away
        ("<p>\u200b</p><p>Remote</p>", ["Remote"]),
        ("<p>Engi\u00adneer</p>", ["Engineer"]),
        # Short ids: the default one would print the whole description on failure
        pytest.param(REMOTEOK_JOB_5, REMOTEOK_JOB_5_PARAGRAPHS, id="remoteok-5"),
        pytest.param(REMOTIVE_JOB_100, REMOTIVE_JOB_100_PARAGRAPHS, id="remotive-100"),
        pytest.param(
            HIMALAYAS_JOB_120, HIMALAYAS_JOB_120_PARAGRAPHS, id="himalayas-120"
        ),
    ],
)
def test_html_to_paragraphs(html: str, expected: list[str]) -> None:
    assert html_to_paragraphs(html) == expected
