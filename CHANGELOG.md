# Changelog

## v1.0.0 (2026-11-07)

The first version that runs end to end: jobs are collected, ranked against the résumé and sent
to Discord, on a Raspberry Pi, unattended.

- **Collection** (Go): Remote OK, Remotive and Himalayas every 6 hours, with retries, backoff
  and rate limits; raw payloads kept apart from the normalized jobs (`collector -renormalize`
  rebuilds them without collecting again); cross-source deduplication; goose migrations
  embedded in the binary.
- **Extraction** (Python): text cleaning and boilerplate cut, skills, salary in annual USD,
  required years, language, and the Jev's answers to the `qs-v4` question set (Latin America,
  Brazil, fully remote, senior, target role, degree), about $0.07 per 1000 jobs.
- **Ranking**: `sklearn-combined-v10`, a logistic regression over the Jev's answers, the skill
  match, the job's stack and an entry-level title, with the language and degree rules on top.
  All 10 of the live jobs it ranked first were good ones (the first learned model: 1 of 10).
- **Measurement**: a frozen test set (`test-set-v3`), nested cross-validation for feature sets,
  audits of the live top 10, and every result recorded with its `model_version`
  (docs/MODELING.md).
- **Labelling and digest** (Go CLI `radar`): `label`, `review`, `status`, and a Discord digest of
  about 4 jobs a day from the last 14 days.
- **Operations**: `docker compose` on the Pi, with the database published on localhost only and
  reached from other machines through an SSH tunnel over Tailscale, and a daily backup to the
  desktop (docs/OPERATIONS.md).
