# Modelling: ranking jobs against the résumé

The goal is a ranking whose top is jobs I would actually apply to, because the top is all the
digest sends. This file is the record of how models are measured, what each one scores, and
what is known to be wrong. Every number comes with the `model_version` that produced it.

## How a model is measured

- **Labels.** Jobs labelled by hand with `radar label`: `would_apply` (yes/no) is the target.
  The optional `hires_latam`, `seniority` and `note` are notes, not targets.
- **Frozen test set** (`analyzer/data/frozen.json`, `test-set-v2`): 150 labelled jobs that never
  take part in training, hyperparameter search or question writing. 4 of them are positive.
- **Deciding metric: average precision (AP)** on the test set. It rewards putting the good jobs
  first. A random order scores the positive rate (0.027 here); a perfect one scores 1.
- **Next to it:** the cross-validated AP on the training jobs (more positives, so steadier),
  ROC AUC as a sanity check, and P@10, the share of good jobs among the top 10.
- **Reality check:** label the 10 live jobs the model would send today (a shuffled queue for
  `radar label`, so the order does not reveal the ranking) and count how many are good.

With 4 positives, one job moves the test AP a lot: on 2026-11-06, almost all of
`sklearn-combined-v2`'s lead came from a single job ranked 4th instead of ~35th. Read the test
AP together with the cross-validated AP, never alone.

## Labels

605 labelled jobs, 28 of them "would apply" (as of 2026-11-07):

| Batch | When | Jobs | How they were chosen |
| --- | --- | --- | --- |
| first rounds | Oct 2026 | 250 | live jobs in id order; `test-set-v1` = a stratified 75 of them |
| round 1 | 2026-11-03 | 200 | random live jobs: 75 completed `test-set-v2`, 125 went to training |
| round 2 | 2026-11-03 | 150 | training only, from the top of `skills-gate-v1` (where positives were expected) |
| review | 2026-11-06 | (41) | every "yes" re-checked under a stricter criterion: 41 became 27 |
| audit | 2026-11-06 | 10 | the top 10 of `sklearn-combined-v2`; 5 were already labelled |

The training pool is every label outside the test set: 455 jobs. Round 2 and the audit were
selected by a model's ranking, so they over-represent what those models like; that is fine for
training, but they must never be used to grow the test set.

## Results on `test-set-v2`

n=150, 4 positive, random AP = 0.0267. Measured 2026-11-07 with `scripts.evaluate`.

| model_version | What it is | AP | ROC AUC | P@10 |
| --- | --- | --- | --- | --- |
| `tfidf-v1` | TF-IDF cosine between the CV and the description | 0.0514 | 0.527 | 0.0 |
| `minilm-l6-v2` | cosine between `all-MiniLM-L6-v2` embeddings of the CV and the description | 0.0548 | 0.497 | 0.1 |
| `skills-gate-v1` | Jev answers as a gate (remote, LatAm, not senior), then skill coverage | 0.0267 | 0.497 | 0.2 |
| `jev-manual-qs-v1` | 0.7·remote + 0.6·LatAm − 0.7·senior, weights set by hand | 0.0678 | 0.649 | 0.1 |
| `sklearn-combined-v2` | logistic regression, see below | **0.1002** | **0.670** | 0.1 |

Raw similarity to the CV (`tfidf-v1`, `minilm-l6-v2`) has no ranking power: the jobs I want are
not the ones that read most like my résumé. Everything above chance goes through the Jev's
answers.

`jev-manual-qs-v1` read 0.0618 until 2026-11-07. That day, 73 of its stored scores (68 in the
test set) turned out not to match the formula over the answers in `jev_answers`: the answers had
been rewritten after the scores were computed. `scripts.score_jev` recomputed every score from
the stored answers, which is what the worker does for new jobs, and the AP became 0.0678.

### `sklearn-combined-v2`

`LogisticRegression` (imputer → scaler → model) over 8 features, computed by
`jobradar/model/features.py`:

- the 3 Jev answers: `open_to_latam`, `fully_remote`, `is_senior_role` (probabilities);
- 4 skill features against the CV: `skill_overlap`, `skill_coverage`, `skill_missing`,
  `n_job_skills`;
- `years_required`, by regex (missing in 80% of jobs, filled with the median).

Trained on 450 jobs (23 positive). `C` chosen by 5-fold cross-validated AP: **C = 0.01**, the
strongest regularization in the grid (smaller values plateau at about the same AP).
**CV AP = 0.1465 ± 0.0569** (folds 0.105, 0.133, 0.259, 0.125, 0.111) against a random 0.051.

| Feature (scaled) | Coefficient |
| --- | --- |
| `skill_coverage` | +0.104 |
| `is_senior_role` | −0.101 |
| `skill_overlap` | +0.090 |
| `open_to_latam` | +0.076 |
| `fully_remote` | +0.045 |
| `n_job_skills` | +0.028 |
| `skill_missing` | −0.008 |
| `years_required` | −0.003 |

`skill_missing` = `n_job_skills` − `skill_overlap`, so those three share their effect.

### The audit: 1 out of 10

On 2026-11-06 the 10 live jobs `sklearn-combined-v2` would send were labelled: **P@10 = 1/10**.
Five of them were already training negatives, which the model still put at the top, so it is not
memorizing: its features cannot tell those jobs apart from good ones. Of the 9 "no":

- 4 have a low `open_to_latam` (0.02 to 0.26: Philippines-only, India-only…) and got in on
  DevOps skills alone. The skill features weigh more than location.
- 5 have a high `open_to_latam` (0.86 to 0.97), but several are restricted to a single Latin
  American country (Mexico-only, Colombia-only). "Open to Latin America" is not "open to
  someone in Brazil", and the question does not separate the two.

Because of this, the digest still ranks by `jev-manual-qs-v1`.

### History: `test-set-v1`

The first test set (75 jobs, 5 positive, ROC AUC only) measured `tfidf-v1` 0.51, `minilm-l6-v2`
0.46, `jev-manual-qs-v1` 0.72 and `sklearn-combined-v1` 0.65. `sklearn-combined-v1` added the
`minilm-l6-v2` cosine and salary/age features, was trained on 175 jobs with C = 100, and gave
its largest weight to the cosine, a feature with no signal on its own: overfitting. v2 dropped
the cosine and the salary/age columns. Its scripts are in `archive/`.

## Known issues

- **Too few positives.** 4 in the test set, 23 in training. Any comparison between models within
  a few hundredths of AP is noise.
- **Location is the main error** (see the audit). The labels already hold a sharper signal:
  `hires_latam` is filled for 139 jobs, and `note` for 250.
- **`tfidf-v1` is not comparable across runs.** Its vocabulary is fit on the CV plus the jobs
  unscored in that run, so a job's score depends on which other jobs came in with it.
- **Jev answers do not record which model answered.** `JEV_MODEL` is `jev-latest`, which floats
  with the provider; answers written months apart may come from different models under the same
  `question_set_version`.
- **Scores do not record which CV produced them.** Changing `cv.md` and re-scoring overwrites
  `tfidf-v1`, `minilm-l6-v2` and `sklearn-combined-v2` under the same version.

## Adding a model version

1. Change the features (`jobradar/model/features.py`) or the method
   (`jobradar/model/combined.py`), and bump `MODEL_VERSION`. `scripts.train` refuses to
   overwrite an existing model file, so the old version stays reproducible.
2. `python -m scripts.train`: read the CV AP and the coefficients.
3. `python -m scripts.score`: writes the new version's score for every live job.
4. `python -m scripts.evaluate`: compare on the test set, next to the current models.
5. Audit its top 10 before pointing `digest.model_version` at it, and set `digest.min_score` on
   its scale (the probabilities of `sklearn-combined-v2` range from 0.03 to 0.10, so the current
   0.75 would let nothing through).

A new Jev question means a new `QUESTION_SET_VERSION` (`jobradar/jev/questions.py`), new answers
for every live job (about $0.05 per 1000), and a new version of every model that reads them.
