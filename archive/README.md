# archive

Work that is finished and no longer runs, kept because the results it produced are still
referenced (in [docs/MODELING.md](../docs/MODELING.md)) and because it shows how the project
got here. Nothing here is maintained: the scripts import modules that have since moved or
changed, and they are not part of the tests or the lint.

| Path | What it was |
| --- | --- |
| `analyzer/eda/` | the exploratory data analysis of the collected jobs (`# %%` cells, pandas): data defects, boilerplate, location and language policies, skills, salary, experience, similarity |
| `analyzer/scripts/build_features.py`, `train_model.py`, `score_sklearn.py`, `explain_model.py` | `sklearn-combined-v1`, the first learned model (Jev answers + embedding cosine + salary and age features), measured on `test-set-v1` |
| `analyzer/scripts/jev_pilot.py` | the first Jev run on the test set, which measured the cost (~$0.05 per 1000 jobs) |
| `analyzer/scripts/freeze_test_set.py`, `sample_label_round.py`, `build_label_queue_round2.py` | how `test-set-v1`, `test-set-v2` and the label rounds were drawn |
| `analyzer/data/frozen_v1.json` | `test-set-v1`: 75 jobs, a subset of today's `test-set-v2` |
| `analyzer/data/label_queue_*.json` | the label queues of rounds 1 and 2 and of the top-10 audit of `sklearn-combined-v2` |
| `docs/SPEC.md`, `ROADMAP.md`, `PROGRESS.md` | the original specification, phase plan and session log of the project (in Portuguese) |
| `docs/eda.md` | the written findings of the EDA |
| `docs/rotulagem-5.4.md` | the labelling guide used for rounds 1 and 2 (in Portuguese) |

`analyzer/data/local/` (not in git) holds intermediate files from those scripts on the machine
that ran them.
