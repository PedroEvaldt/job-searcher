# %%
import sys
from pathlib import Path

import pandas as pd
from db import sql_to_dataframe
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity

pd.set_option("display.width", 120)
# __file__ is not defined when the cell runs in a REPL, so resolve paths from the cwd
# (analyzer/ or analyzer/eda/), the same way the other eda scripts do.
ANALYZER_ROOT = Path.cwd().parent if Path.cwd().name == "eda" else Path.cwd()
sys.path.insert(0, str(ANALYZER_ROOT))
from jobradar.boilerplate import cut_boilerplate, paragraphs_counter
from jobradar.text import html_to_paragraphs

# cv.md lives at the repo root, one level above analyzer/
CV_PATH = ANALYZER_ROOT.parent / "cv.md"


def section(title: str) -> None:
    print(f"\n=== {title} ===")


# %%
# Baseline ranking: TF-IDF cosine between the CV and every live job. The CV and the jobs
# go through the same cleaning pipeline and the same vectorizer vocabulary, so the scores
# are comparable. Jobs that share the CV's distinctive words rank high.
jobs = sql_to_dataframe(
    """
    SELECT j.id, j.title, j.company, s.name AS source_name, j.description
    FROM jobs j
    JOIN sources s ON j.source_id = s.id
    WHERE j.duplicate_of IS NULL
    ORDER BY j.id;
    """
)

counter = paragraphs_counter(jobs["description"])
jobs["description"] = jobs["description"].apply(
    lambda desc: "\n".join(cut_boilerplate(html_to_paragraphs(desc), counter))
)

# %%
# CV as document 0, then every job in id order — fit together so they share one vocabulary.
cv_text = CV_PATH.read_text(encoding="utf-8")
corpus = [cv_text, *jobs["description"].tolist()]
matrix = TfidfVectorizer(stop_words="english").fit_transform(corpus)
jobs["similarity"] = cosine_similarity(matrix[0], matrix[1:])[0]

section(f"corpus: 1 CV + {len(jobs)} live jobs")
print(f"similarity range: {jobs['similarity'].min():.3f} .. {jobs['similarity'].max():.3f}")

# %%
ranked = jobs.sort_values("similarity", ascending=False)
section("top 15 jobs by TF-IDF cosine with the CV")
print(
    ranked[["similarity", "title", "company", "source_name"]].head(15).to_string(index=False)
)

section("bottom 5 (sanity: should be unrelated to the CV)")
print(ranked[["similarity", "title", "company"]].tail(5).to_string(index=False))
