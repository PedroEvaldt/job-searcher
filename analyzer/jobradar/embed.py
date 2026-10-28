from typing import cast

from sentence_transformers import SentenceTransformer


def embed(descriptions: list[str]) -> list[list[float]]:
    transformer = SentenceTransformer("all-MiniLM-L6-v2")
    vectors = transformer.encode(descriptions)
    return cast(list[list[float]], vectors.tolist())
