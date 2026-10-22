# EDA — defects in the collected data

Numbers come from a local copy of the Pi database (dump of 2026-10-19): 16,800 jobs, **16,204
live** (`duplicate_of IS NULL`), of which Himalayas 16,049, Remote OK 136 and Remotive 19. Every
share below is over live jobs of the source named, unless it says otherwise.

To reproduce, from `analyzer/` with only `postgres` up:

```bash
uv run --group eda --env-file ../.env python eda/02_text_defects.py   # defects 1-3
uv run --group eda --env-file ../.env python eda/03_payload_fields.py # defects 4-6
```

Defect 7 comes from the `runs` table; the query is in its section.

| #   | Defect                       | Number                                        | Owner          | Decision      |
| --- | ---------------------------- | --------------------------------------------- | -------------- | ------------- |
| 1   | HTML in every description    | 16,204 of 16,204 (100%)                       | Python · 2.3   | fix in Python |
| 2   | HTML entities in titles      | 86 of 16,049 Himalayas titles (0.5%)          | Go · D10 · 3a  | fix in Go     |
| 3   | Mojibake                     | 102 of 136 Remote OK descriptions (75%)       | Go · D10 · 3a  | fix in Go     |
| 4   | Fields the collector drops   | 5 `jobs` columns, 0 of 16,204 filled          | Go · D33, D32  | fix in Go     |
| 5   | Sentinels for "not informed" | `salary_min: 0` in 112 of 136 Remote OK (82%) | Go · D33, D10  | fix in Go     |
| 6   | Salary in many units         | 4 periods, 25 currencies, 0.59 to 168M        | Python · 3a    | fix in Python |
| 7   | Himalayas page cap           | 37 of 44 full runs with ≥370 of 400 new       | Go · D42 · backlog | fix in Go |

---

## 1. HTML in every description

**What:** `jobs.description` is the source's HTML, not text.

**Number:** 16,204 of 16,204 live jobs (100%), in all three sources.

**Example:** job 1 (Remote OK): `<h1>Einleitung </h1><p>Wir suchen …`.

**Owner:** Python, Task 2.3.

**Decision:** fix in Python. `jobs` keeps the HTML: the tags mark the paragraphs that the
boilerplate cut works on. The analyzer turns it into paragraphs of plain text.

## 2. HTML entities in titles

**What:** titles are plain text, but some arrive with characters still escaped as HTML.

**Number:** 86 of 16,049 Himalayas titles (0.5%); none in Remote OK or Remotive. `&#x2f;` (`/`) in
45 titles, `&#x28;`/`&#x29;` (`(`/`)`) in 32, `&ndash;` in 23.

**Example:** job 5612 (Himalayas): `Full Stack Developer - .NET&#x2f;Angular - Full Remote Employee`.

**Owner:** Go, normalization; D10 (recorded for Remote OK, applies to every source).
Rescheduled 2026-10-22 to Phase 3a (fixed in the same pass that cleans the extractor's text).

**Decision:** fix in Go: unescape the title when normalizing. Inside the description, entities
are valid HTML and go away in the HTML → text step (defect 1).

## 3. Mojibake

**What:** UTF-8 text decoded as Latin-1 before it reaches the collector. It leaves C1 control
characters (U+0080–U+009F) and pairs like `Ã¼` where `ü` should be.

**Number:** 102 of 136 Remote OK descriptions (75%); 4 of 16,049 Himalayas (0.02%); none in
Remotive.

**Example:** job 1 (Remote OK): `FÃ¼r unsere â\x80\x8bTierhalterbesitzer:innen`, which should read
`Für unsere Tierhalterbesitzer:innen` (with a zero-width space before the `T`).

**Owner:** Go, normalization; D10. Rescheduled 2026-10-22 to Phase 3a.

**Decision:** fix in Go for Remote OK. All 102 come back whole with `encode("latin-1")` +
`decode("utf-8")`, so the normalizer can reverse it when C1 characters are present and keep the
text as is when the round trip fails. Accept the 4 Himalayas: they mix real Unicode with
mojibake, and the whole-string round trip fails on them.

## 4. Fields the collector drops

**What:** the payloads carry date, location, salary and tags, but `jobs` only stores title,
company, URL and description.

**Number:** `posted_at`, `remote_scope`, `salary_min`, `salary_max` and `currency` are filled in 0
of 16,204 live jobs. In the payloads, truly informed (not null and not a sentinel):

| Field    | Himalayas | Remote OK | Remotive |
| -------- | --------- | --------- | -------- |
| date     | 100%      | 100%      | 100%     |
| location | 97.3%     | 57.4%     | 100%     |
| salary   | 41.2%     | 17.6%     | 57.9%    |
| tags     | 100%      | 98.5%     | 94.7%    |

**Example:** job 119 (Himalayas) has `pubDate: 1791491837` (2026-10-08 20:37 UTC) in the payload
and `posted_at` NULL. Job 5 (Remote OK) has `salary_min: 170000, salary_max: 350000` in the payload
and `salary_min` NULL.

**Owner:** Go: D33 (`insertJob` writes the fields) and D32 (re-normalize the history). The
consumers are Task 2.4 (location) and Phase 3a (job age, salary).

**Decision:** fix in Go. `raw_jobs` keeps every payload, so the jobs already collected are filled
by re-normalizing, without re-collecting.

## 5. Sentinels for "not informed"

**What:** the key is there with a placeholder value instead of null, so a null check counts it as
informed.

**Number:**

- Remote OK: `salary_min`/`salary_max` `0` in 112 of 136 (82%); `location` `""` in 58 of 136 (43%).
- Remotive: `salary` `""` in 8 of 19.
- Himalayas: `salaryPeriod` comes in every job, `annual` in 8,911 of the 9,432 with no amount;
  `currency` comes without an amount in 1,235.
- Himalayas: `locationRestrictions` `[]` in 432 of 16,049 (2.7%). This may mean "worldwide" rather
  than "unknown"; Task 2.4 decides.

**Example:** job 1 (Remote OK): `salary_min: 0, salary_max: 0`. Job 3 (Remote OK): `location: ""`.
Job 191 (Himalayas): no `minSalary`, but `salaryPeriod: "annual"` and `currency: "USD"`.

**Owner:** Go, in the same code that will write the columns (D33). D10 already records
`salary_min: 0`.

**Decision:** fix in Go: when the columns start being written, sentinels become NULL, so `IS NULL`
means "not informed" in every source. Himalayas period and currency are written only with an
amount.

## 6. Salary in many units

**What:** the salaries that are informed cannot be compared as they come.

**Number:**

- Himalayas, 6,617 jobs with an amount:
  - periods: 5,186 annual, 1,001 hourly, 425 monthly, 5 weekly;
  - 25 currencies, USD in 83.7%;
  - amounts from 0.59 (hourly, USD) to 168,000,000 (annual, IDR).
- Remotive: free text in 11 of 19, in several shapes (`$90k - $105k`, `$90 - $150 /hour`,
  `$35,3k- $52k`, `$10K-$20K`, `OTE $25k - $35k`).
- Remote OK: numbers, with no currency or period key.

**Example:**

- Job 10352 (Himalayas): 0.59–0.59 USD, hourly.
- Job 10901 (Himalayas): 120,000,000–168,000,000 IDR, annual.
- Job 116 (Remotive): `$35,3k- $52k`.

**Owner:** Python, Phase 3a (salary extraction; numbers are compared in code, never by the LLM).

**Decision:** fix in Python: bring every amount to one period, keep the currency next to it and
parse Remotive's text.

## 7. Himalayas page cap

**What:** the collector reads at most 20 pages (400 jobs) of Himalayas per 6 h cycle, and the cap
is binding. Most of what it reads is new, so jobs published past page 20 are never seen. The flat
~1.1–1.6k new jobs per day in `eda/01_overview.py` is this cap, not the market.

**Number:** 47 finished Himalayas runs (2026-10-08 → 2026-10-19). 44 read the full 400; in 37 of
those, at least 370 of the 400 were new. The median is 398 new.

**Example:** run 165 (2026-10-19 02:00 UTC): 400 read, 400 new. Run 159 got 0 new, because it was
a restart about 40 minutes after the previous run.

```sql
WITH first_seen AS (
  SELECT source_id, external_id, min(fetched_at) AS first_at FROM raw_jobs GROUP BY 1, 2
)
SELECT r.id, r.started_at, r.fetched,
       (SELECT count(*) FROM first_seen f
         WHERE f.source_id = r.source_id
           AND f.first_at BETWEEN r.started_at AND r.finished_at) AS new_jobs
FROM runs r JOIN sources s ON s.id = r.source_id
WHERE s.name = 'himalayas' AND r.finished_at IS NOT NULL
ORDER BY r.id;
```

**Owner:** Go, D42 (`main.go:28`, `himalayasMaxPages = 20`). Rescheduled 2026-10-22 to the
backlog (revisit if coverage limits Phase 3a).

**Decision:** fix in Go. Either raise the cap, or keep paging until a page brings only known jobs,
inside the source's time budget and rate limit.

---

# Policies

Which jobs the ranking and the digest work on, and which are set aside. Numbers here come from a
fresher copy (Pi dump of 2026-10-22, with the duplicate marks applied): 20,817 jobs, **20,029 live**
(Himalayas 19,870, Remote OK 140, Remotive 19). They are larger than the defects above because the
Pi kept collecting; the shape holds.

To reproduce, from `analyzer/` with only `postgres` up:

```bash
uv run --group eda --env-file ../.env python eda/05_location.py   # location
uv run --group eda --env-file ../.env python eda/06_language.py   # language
```

## A. Location — can someone living in Brazil take this job?

**What:** remote does not mean open to everyone — many jobs restrict who may apply, by country or by
timezone. The eligibility fields never reach `jobs` (defect 4, D33); they live in the `raw_jobs`
payload, named and shaped differently per source. The rule reads them and buckets each live job as
**accept** / **reject** / **unknown** for a candidate in Brazil (UTC−3):

- **Himalayas** — accept if `locationRestrictions` lists Brazil **or** `timezoneRestrictions`
  includes the −3 offset; both empty → unknown.
- **Remotive** — accept if `candidate_required_location` (comma-split into whole tokens) contains
  one of `worldwide, anywhere, americas, latam, latin america, south america, brazil, brasil`.
  Whole-token match on purpose: `northern america` (excludes Brazil) must not match `americas`.
- **Remote OK** — `location` is the city the job was posted *from*, not candidate eligibility, so it
  cannot answer the question: every Remote OK job is **unknown**.

**Number:** 20,029 live → **1,173 accept · 18,716 reject · 140 unknown** (100% covered). By source:
Himalayas 1,162 / 18,708 / 0; Remotive 11 / 8 / 0; Remote OK 0 / 0 / 140. On Himalayas the timezone
signal **subsumes** the country one: every job that lists Brazil as a country (406) also includes
−3, and 756 more include −3 without naming Brazil, so the timezone alone catches all 1,162.

**Example:** accept — job 100 (Remotive), `candidate_required_location: "Worldwide"`. Reject — the
bulk of Himalayas, `locationRestrictions: ["United States"]` with `timezoneRestrictions:
[-10,-9,-8,-7,-6,-5,14]` (no −3). Unknown — job 3 (Remote OK), `location: ""`.

**Decision:** the ranking (3a) and the digest (5) operate on the **accept** bucket only. 94% of live
jobs exclude a Brazil resident; keeping them would make the digest mostly unreachable jobs. Open
edge: the rule is strict on −3, so the 698 jobs with `[-8,-7,-6,-5,-4,-3.5]` (a North-American
Atlantic window with no −3) are rejected — the most debatable boundary. Widening to the Brazilian
window (−2 to −5) is a one-line change; the measurement supports deciding later.

## B. Language — which jobs are not in English

**What:** the matching pipeline (embeddings, Jev questions) is English, so a non-English description
scored by it gives garbage; those jobs are handled in a separate batch (SPEC §11).
`eda/06_language.py` guesses each live description's language by counting frequent function words of
Portuguese vs English (by hand, no library; empty text and ties → unknown).

**Number:** over all live jobs, only **552 of 20,029 (2.8%)** are non-English — a number that would
suggest dropping the batch. But over the **accept** bucket, the jobs that actually matter, it is
**214 of 1,173 (18%)**: jobs open to Brazil skew to regional employers posting in Portuguese or
Spanish.

**Example:** job 66 (Remote OK), `Sobre a Stone: Somos muito mais do que uma empresa de
maquininhas…` (PT). Job 207 (Himalayas), `GetMyInvoices recopila automáticamente facturas de más
de…` (ES — labelled "pt", see below).

**Decision:** **keep** the separate batch. The decision is driven by the 18% inside `accept`, not by
the 2.8% overall; dropping it would have been wrong. Two caveats, both deferred to **D45**:

- The detector conflates Portuguese and Spanish (shared function words) and also catches French, so
  the batch is really **non-English**, not PT-BR — reframe the SPEC §11 batch accordingly. Of the
  214 accept/non-English, disambiguating by distinctive words gives ~92 PT, ~100 ES, 22 tie.
- ~20 genuine Brazilian-Portuguese jobs are marked **reject** by the location rule (e.g. a clearly
  Brazilian posting tagged `locationRestrictions: ["United States"]`): a location false-negative
  where PT-BR is a stronger eligibility signal than the mis-filled country. A finer PT-BR vs ES
  detector and using language as a weak eligibility hint are both deferred to D45.

Mojibake (defect 3, D10) corrupts accents in some descriptions (`A Stone Ã© um`), weakening the
accented PT markers; the function-word counts still separate the languages.
