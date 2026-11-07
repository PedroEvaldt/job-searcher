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
- junior or mid level, the entry level (junior, intern, trainee) preferred;
- written in English, Portuguese or Spanish (a rule);
- no hard requirement of a completed degree: "or equivalent experience", an in-progress degree
  or current students are fine (a rule).

## Done in v1.0.0: features and models

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
- [x] `qs-v4` (a remote job that names no country is open) and `sklearn-combined-v8`: test AP
      0.600, audit 8/10, a near tie; v7 stays in the digest.
- [x] A stack read from the title too, with COBOL and the other missing languages
      (`jobradar/stack.py`): `sklearn-combined-v9`, test AP 0.554, audit 9/10, a tie; v7 stays.
- [x] `junior_title` (the title says junior, intern or entry level), for more weight on the
      entry level: `sklearn-combined-v10`, audit **10/10**, test AP 0.498 (no test positive is
      entry level). The digest switches to v10.

## Done in v1.0.0: the digest on the Pi

- [x] The worker keeps every new job complete for the digest's model: language, skills, the
      model's Jev answers, its score (`jobradar/pipeline.py`).
- [x] `digest.model_version: sklearn-combined-v10`, `min_score: 0.20` (≈4 jobs a day),
      `max_age: 336h`. Model files are versioned, so the images carry them.
- [x] Deploy to the Pi (2026-11-07): the desktop's database restored into a fresh volume, the
      new images, the worker on v10. Its first cycles took 438 new jobs through the whole
      pipeline for $0.03; 7 passed the digest's filters.

## Next

In order:

- More labels drawn **at random** from the live jobs, to grow the test set past its 3 positives:
  v7 to v10 differ only in where two test jobs land, and none of its positives is entry level.
  Labels picked by a model's ranking go to training only.
- Label what the digest sends (training only), then a version that learns the junior title
  together with the stack: in v10 a junior title outweighs another stack, and the first digest's
  top job was a junior C++ role.
- Ask the Jev again when a posting's text changes (58 live jobs on 2026-11-07; MODELING.md).
- Teach `fully_remote` (see MODELING.md). The sources' location field was tried: no gain.
- Pin the Jev model (`jev-latest` floats) and record the CV version in the model versions.
- Refit `tfidf-v1`'s vocabulary on the whole live set, as a new version.
