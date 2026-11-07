# Roadmap

Current goal: **the digest sends jobs I would apply to.** Measured by the audit (how many of the
top 10 live jobs are good) and by average precision on the frozen test set; see
[MODELING.md](MODELING.md).

## What a good job is

Stated on 2026-11-07, and what the questions, the rules and the models aim at:

- someone living in Brazil can take it: worldwide, all of Latin America, or Brazil listed. A job
  open to a single other country (Mexico-only, Colombia-only) does not count;
- fully remote; hybrid or on-site never;
- hands-on backend / software engineering (Go, Python), DevOps / infrastructure / SRE, or
  data / AI / ML. Not QA, security analysis, support, frontend-only or non-technical roles;
- junior or mid level;
- written in English, Portuguese or Spanish (a rule);
- no hard requirement of a completed degree: "or equivalent experience", an in-progress degree
  or current students are fine (a rule).

## Now: better features, then a model that uses them

- [x] Question set `qs-v2`: adds `open_to_brazil` and `is_target_role` to the three `qs-v1`
      questions. `qs-v1` stays as it is: the worker and the digest keep using it until the switch.
- [x] Pilot: `qs-v2` on the 455 labelled training jobs ($0.03): the training CV AP went from 0.17
      to 0.44.
- [x] `qs-v2` answered for all 16,204 live jobs ($1.03).
- [x] Criteria review: 12 training "yes" that broke the stated criteria re-checked; 10 became "no".
- [x] `sklearn-combined-v4`: test AP 0.483 (v2: 0.100), ROC AUC 0.940, CV AP 0.368.
- [x] Audit of v4's top 10 live jobs: **7/10** (v2: 1/10).
- [x] Two more stated criteria as rules: language (en, pt, es) and no hard degree requirement
      (question `requires_degree`, set `qs-v3`). `sklearn-combined-v5` = v4 on `qs-v3` + rules:
      audit **7/10** with the 7 best all good.
- [x] Location rule on `open_to_brazil` (v6): rejected, it drops a good job and audits 6/10.
- [x] Test positives re-checked against the criteria: `test-set-v3` (one job became "no").
- [x] `other_stack` (no Go or Python among the job's languages): `sklearn-combined-v7`, test AP
      0.767, audit **9/10**.

## Next: switch the digest

- [x] The worker keeps every new job complete for the digest's model: language, skills, the
      model's Jev answers, its score (`jobradar/pipeline.py`).
- [x] `digest.model_version: sklearn-combined-v7`, `min_score: 0.20` (≈5 jobs a day),
      `max_age: 336h`. Model files are versioned, so the images carry them.
- [ ] Deploy to the Pi: database move with the reindex (OPERATIONS.md), then the new images.

## Later

- Teach `fully_remote` (see MODELING.md). The sources' location field was tried: no gain.
- More labels drawn **at random** from the live jobs, to grow the test set past 4 positives.
  Labels picked by a model's ranking go to training only.
- Pin the Jev model (`jev-latest` floats) and record the CV version in the model versions.
- Refit `tfidf-v1`'s vocabulary on the whole live set, as a new version.
