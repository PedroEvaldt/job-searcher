# Modelling: ranking jobs against the résumé

The goal is a ranking whose top is jobs I would actually apply to, because the top is all the
digest sends. This file is the record of how models are measured, what each one scores, and
what is known to be wrong. Every number comes with the `model_version` that produced it.

**Now:** the digest ranks by `sklearn-combined-v10`. All 10 of the live jobs it ranks first are
good ones; the first learned model, `sklearn-combined-v2`, managed 1 of 10. On the frozen test
set its average precision is 0.498 (random: 0.020; v2: 0.121), below v7's 0.767: v10 weights
up the entry level, a preference stated after the test set was labelled (see v10 below).

## How a model is measured

- **Labels.** Jobs labelled by hand with `radar label`: `would_apply` (yes/no) is the target,
  judged against the criteria in [ROADMAP.md](ROADMAP.md). The optional `hires_latam`,
  `seniority` and `note` are notes, not targets.
- **Frozen test set** (`analyzer/data/frozen.json`, `test-set-v3`): 150 labelled jobs that
  never take part in training, hyperparameter search, question writing or rule thresholds.
- **Deciding metric: average precision (AP)** on the test set. It rewards putting the good jobs
  first. A random order scores the positive rate; a perfect one scores 1.
- **Next to it:** the cross-validated AP on the training jobs, ROC AUC as a sanity check, and
  P@10, the share of good jobs among the 10 best-ranked test jobs.
- **The audit**, the check that matters most: label the 10 live jobs the model would send
  today (a shuffled queue in `analyzer/data/queues/`, so the order does not reveal the
  ranking) and count the good ones.

The test set has only 3 positives, so one job moves its AP by a lot. Feature sets are chosen by
nested cross-validation on the training jobs (C picked inside each outer fold, 5×5 repeats), the
test set is read once per version, and the audit decides between versions that tie. A
preference stated after the test set was labelled (more weight on the entry level, v10) is
judged by the audit: the test labels cannot reflect it.

## Labels

636 labelled jobs, 36 of them "would apply" (2026-11-07). The training pool is every label
outside the test set: 486 jobs, 33 positive.

| Batch | When | Jobs | How they were chosen |
| --- | --- | --- | --- |
| first rounds | Oct 2026 | 250 | live jobs in id order; `test-set-v1` = a stratified 75 of them |
| round 1 | 2026-11-03 | 200 | random live jobs: 75 completed `test-set-v2`, 125 went to training |
| round 2 | 2026-11-03 | 150 | training only, from the top of `skills-gate-v1` |
| positives review | 2026-11-06 | (41) | every "yes" re-checked under a stricter criterion: 41 became 27 |
| criteria review | 2026-11-07 | (13) | training "yes" that broke the stated criteria, re-checked from their descriptions: 11 became "no" |
| test review | 2026-11-07 | (4) | the 4 test "yes" re-checked against the stated criteria: 1 became "no" (`test-set-v3`) |
| audits | 2026-11-06/07 | 80 | the top 10 of v2 and v4 to v10; 36 of them new labels |

Round 2 and the audits were selected by a model's ranking, so they over-represent what those
models like. That is fine for training, but they never grow the test set, and they make the
cross-validated AP of the later versions optimistic (their positives are the jobs this kind of
model already ranks first).

Test labels do not change once a model has been measured on them, with one documented
exception: when the criteria were stated, the test positives were re-checked from their
descriptions, without any model score, and the set became `test-set-v3`. The criteria only
restrict, so only positives could change.

## The Jev's questions

All asked in one call per job, about $0.07 per 1000 jobs; answers are probabilities.

| Set | Questions | Answered for |
| --- | --- | --- |
| `qs-v1` | `open_to_latam`, `fully_remote`, `is_senior_role` | all live jobs (the worker until 2026-11-07) |
| `qs-v2` | + `open_to_brazil` (Brazil can take it; a single other country cannot), `is_target_role` (hands-on backend, infra or data work) | all live jobs ($1.06) |
| `qs-v3` | + `requires_degree` (a completed degree with no alternative) | all live jobs ($1.14) |
| `qs-v4` | `open_to_brazil` rewritten: a remote job that names no country counts as open | all live jobs ($1.15); the worker since 2026-11-07 (v10) |

Each new question was piloted on the labelled training jobs first. `is_target_role` alone ranks
the training jobs at AP 0.147 (the best `qs-v1` answer: 0.084); `open_to_brazil` drops the
Mexico-only and Colombia-only jobs from 0.89–0.97 (`open_to_latam`) to 0.02–0.03;
`requires_degree` reads "Bachelor's required" high and "or equivalent experience" low, but misses
a few real requirements.

## Results

`test-set-v3` (n=150, 3 positive, random AP = 0.020), measured 2026-11-07 with `scripts.evaluate`;
audit = good jobs among the 10 live jobs the version ranks first.

| model_version | What it is | AP | ROC AUC | P@10 | Audit |
| --- | --- | --- | --- | --- | --- |
| `tfidf-v1` | TF-IDF cosine, CV vs description | 0.058 | 0.640 | 0.0 | |
| `minilm-l6-v2` | embedding cosine, CV vs description | 0.064 | 0.642 | 0.1 | |
| `skills-gate-v1` | Jev gate (remote, LatAm, not senior), then skill coverage | 0.020 | 0.497 | 0.2 | |
| `jev-manual-qs-v1` | 0.7·remote + 0.6·LatAm − 0.7·senior, by hand | 0.070 | 0.652 | 0.1 | |
| `sklearn-combined-v2` | logistic regression, `qs-v1` + 4 skill features + years | 0.121 | 0.794 | 0.1 | 1/10 |
| `sklearn-combined-v4` | `qs-v2` + skill coverage and overlap | 0.600 | 0.980 | 0.3 | 7/10 |
| `sklearn-combined-v5` | v4's features on `qs-v3`, plus the language and degree rules | 0.514 | 0.984 | 0.3 | 7/10 |
| `sklearn-combined-v6` | v5 plus a rule dropping `open_to_brazil` < 0.15 | 0.307 | 0.776 | 0.2 | 6/10 |
| `sklearn-combined-v7` | v5 plus `other_stack` | 0.767 | 0.984 | 0.3 | 9/10 |
| `sklearn-combined-v8` | v7 on `qs-v4` | 0.600 | 0.980 | 0.3 | 8/10 |
| `sklearn-combined-v9` | v8 with the stack read from the title: `main_other_stack`, `title_cv_stack` | 0.554 | 0.977 | 0.3 | 9/10 |
| **`sklearn-combined-v10`** | **v9 plus `junior_title`** | **0.498** | **0.966** | **0.2** | **10/10** |

Raw similarity to the CV (`tfidf-v1`, `minilm-l6-v2`) has no ranking power: the jobs I want are
not the ones that read most like my résumé. What works is literal facts about the job (can I
take it from Brazil, is it the kind of work I do, how senior), plus the skill match.

How the versions got there:

- **v2** had only the `qs-v1` answers. Its audit failed on location: jobs open to Latin America
  but to a single country (Mexico-only), which `open_to_latam` cannot tell apart, and DevOps
  jobs restricted to India or the Philippines that got in on skills alone. Hence `qs-v2`.
- **v4** (`qs-v2`) jumped to 7/10. Before training it, 10 training "yes" that broke the newly
  stated criteria (QA, security, writing, on-site, US-only roles) became "no".
- **v5** added the two stated rules (below) and `requires_degree`: same audit count, but its 7
  best-ranked jobs were all good. Its 3 misses were jobs not open to Brazil.
- **v6** tried a location rule for those misses and was rejected: it also drops a job labelled
  good (open to Southeast Asia, `open_to_brazil` 0.10), and its audit fell to 6/10. The
  misses it removed were replaced by Java, Rust and Node.js jobs.
- **v7** added `other_stack` for those: 1 when the job names programming languages and none of
  them is in the CV (Go, Python). Among labelled technical jobs, only 1 of 24 with another stack
  and no Go or Python is good. Nested CV AP 0.508 against v5's 0.483 (a small step, within
  noise), but test AP 0.767 (good test jobs at 1, 2 and 10) and audit 9/10.

- **v8** read `qs-v4`, written after a stated clarification: a remote job that names no country
  is open to anyone. On the training jobs the good jobs' median `open_to_brazil` rose from 0.63
  to 0.87, the two location weights stopped cancelling (+3.15 / −1.53) and the nested CV AP went
  from 0.594 to 0.645. But on the test set its second good job fell from 2nd to 4th (AP 0.600),
  and its audit gave 8/10: it surfaced two good jobs v7 ranked 61st and 132nd, and let in a
  COBOL and a Java job. COBOL is not in the skill dictionary, and the Java job names Java only
  in its title, which skill extraction does not read. v7 stays: a near tie is no reason to
  switch.
- **v9** read the stack with its own dictionary (`jobradar/stack.py`), which knows COBOL,
  Delphi or ABAP, maps frameworks to their language (Django → Python) and skips English words
  ("go beyond", "react to"). `main_other_stack` replaces `other_stack`: the title's languages
  when it names any, else the description's, and none in the CV. `title_cv_stack` marks a
  title that names Go or Python (8 training jobs, 5 good). Nested CV AP 0.691 against v8's
  features 0.640 (5×10 folds, better in 33 of 50), C = 1, the two features at −0.35 and +0.35.
  The COBOL and Java jobs left the top 10, which is now Go and Python titles, and its audit
  gave 9/10, with 5 new good jobs that v7 ranked 46th to 226th. On the test set the good jobs
  landed at 1, 7 and 8 (AP 0.554), behind a senior job with Python in its title at 2nd:
  `title_cv_stack` outweighs `is_senior_role` there. That job is a test job, so it cannot
  shape a next version's features. A tie with v7 on the audit and a lower test AP: v7 stays.
- **v10** added `junior_title` for a preference stated after v9: its jobs, with more weight on
  the entry level. 1 when the title says junior, intern, trainee or entry level
  (`jobradar/seniority.py`; the title only, since senior jobs' descriptions say "mentor junior
  engineers"). 23 training titles say so, 5 of them good: 22%, against 6% overall. The model
  gives it +0.51, and the nested CV AP goes from 0.762 to 0.791 on the labels as of v9's audit
  (30 of 50 folds better). The labelled good entry-level jobs moved up from v9's 43rd–187th to
  11th–36th, and the audit gave 10/10, a Golang trainee and two remote internships among them.
  On the test set, whose good jobs are not entry level, they landed at 1, 9 and 11 (AP 0.498).
  The digest switched to v10 on the audit and the stated preference.

`sklearn-combined-v3` (`qs-v2` + skill coverage, trained before the criteria review) was
superseded by v4 without being measured on the test set.

### `sklearn-combined-v10`, the digest's model

Logistic regression (median imputer → scaler → model) over 10 features
(`jobradar/model/features.py`) on the `qs-v4` answers, trained on 483 jobs (30 positive), C = 1
chosen by 5-fold cross-validated AP (0.81 ± 0.15, optimistic for the reason above).

| Feature (scaled) | Coefficient |
| --- | --- |
| `is_target_role` | +2.452 |
| `open_to_brazil` | +1.467 |
| `is_senior_role` | −0.843 |
| `open_to_latam` | −0.709 |
| `skill_overlap` | −0.634 |
| `junior_title` | +0.514 |
| `title_cv_stack` | +0.488 |
| `main_other_stack` | −0.434 |
| `fully_remote` | +0.214 |
| `skill_coverage` | +0.002 |

The two location answers move together and share their effect: together they reward "open to
Brazil" and punish "open to Latin America but not to Brazil"; with v5's features, dropping
`open_to_latam` lowered the nested CV AP from 0.48 to 0.36. `skill_overlap` and
`skill_coverage` share theirs too.

**Rules** on top (`Rules` in `jobradar/model/combined.py`; a job that breaks one scores 0). They
are stated criteria the labels were not written with, so the model cannot learn them:

- written in English, Portuguese or Spanish (`jobs.language`, langdetect): drops 590 live jobs,
  mostly German and French; no good job is in another language;
- no hard degree requirement (`requires_degree` < 0.5): drops 3,051 live jobs.

Together 3,568 of the 16,204 live jobs score 0.

**Threshold.** Share of good jobs above a v10 score, on the training and audit labels (rough:
those labels concentrate at the top, and v10 was trained on most of them), with how many of
the ~382 jobs collected a day pass it:

| `min_score` | good / labelled above | new jobs a day above |
| --- | --- | --- |
| 0.10 | 44% | ~9 |
| 0.15 | 55% | ~6 |
| **0.20** | **68%** | **~4** |
| 0.30 | 82% | ~2 |
| 0.40 | 86% | ~1.4 |

The digest runs with `min_score: 0.20` and `max_age: 336h` (postings from the last 14 days).

## Known issues

- **Few positives.** 3 in the test set, 33 in training. Differences within a few hundredths of
  AP are noise; the audit is the tiebreaker. v7 to v10 rank the first test positive 1st and
  differ only in where the other two land: the test set can no longer tell them apart, and it
  has no entry-level positive to measure v10's preference with.
- **"Fully remote" is weakly learned.** "Never hybrid" is a stated criterion, but 3 good jobs
  read as not remote to the Jev (one, at Stone, is marked "Remoto" by its source), so no rule,
  and the model gives `fully_remote` little weight.
- **The sources' own location field is not a feature.** Himalayas lists countries for 97% of
  its jobs, but they look like the company's country, not eligibility: a job whose text says
  "Remote · LATAM" is listed as "United States". Adding it lowered the nested CV AP (0.48 →
  0.45).
- **Skills miss the title and some languages.** `job_skills` reads the description only, with
  no COBOL or Delphi, so `skill_coverage`, `skill_overlap` and `other_stack` miss a job that
  names its stack in the title. Only the stack features of v9 and v10 read the title
  (`jobradar/stack.py`, a dictionary of its own, so `job_skills` and the earlier versions stay
  as measured).
- **The degree rule sits on a noisy answer.** 35 live jobs cross `requires_degree` 0.5 between
  `qs-v3` and `qs-v4`. One is a good job ("Bachelors Degree in IT…" and, two lines below, "or
  equivalent related experience"): 0.45 in `qs-v3`, 0.52 in `qs-v4`, so v8 to v10 drop it.
- **Jev answers are not refreshed when a posting changes.** A new text resets the job's skills
  and its score follows, but its answers stay those of the first text: the worker only asks
  about jobs with no answers for the set. 58 of the 16,642 live jobs (2026-11-07).
- **`tfidf-v1` is not comparable across runs.** Its vocabulary is fit on the CV plus the jobs
  unscored in that run.
- **Jev answers do not record which model answered.** `JEV_MODEL` is `jev-latest`, which floats
  with the provider.
- **Scores do not record which CV produced them.** Changing `cv.md` and re-scoring overwrites a
  version's scores under the same name (the worker reads the CV every cycle).

## Adding a model version

1. Add a `ModelSpec` to `SPECS` (`jobradar/model/combined.py`): its question set, its features
   (`jobradar/model/features.py` computes them) and its rules. `scripts.train` refuses to
   overwrite an existing version's model file, so every published number stays reproducible.
2. Compare feature sets by nested cross-validation on the training jobs, never on the test set.
3. `python -m scripts.train <version>`: the CV AP and the coefficients.
4. `python -m scripts.score <version>`: scores every live job.
5. `python -m scripts.evaluate`: once, next to the current models.
6. Audit its top 10. To make it the digest's model, see docs/OPERATIONS.md.

A new Jev question means a new question set (`QUESTION_SETS` in `jobradar/jev/questions.py`),
answers for every live job (`python -m scripts.answer_jev <set> --workers 6`, about $1.10 for
16,000 jobs; pilot it first with `--training-labels`), and a new model version that reads it.

## History: `test-set-v1` and `test-set-v2`

`test-set-v1` (75 jobs, 5 positive, ROC AUC only) measured `tfidf-v1` 0.51, `minilm-l6-v2` 0.46,
`jev-manual-qs-v1` 0.72 and `sklearn-combined-v1` 0.65. `sklearn-combined-v1` added the embedding
cosine and salary and age features, was trained on 175 jobs with C = 100, and gave its largest
weight to the cosine, a feature with no signal on its own: overfitting. Its scripts are in
`archive/`.

`test-set-v2` (the same 150 jobs as v3, 4 positive) measured, by AP: `jev-manual-qs-v1` 0.068
(0.062 until 73 stale scores were recomputed on 2026-11-07), `sklearn-combined-v2` 0.100,
v4 0.483, v5 0.419, v6 0.276.
