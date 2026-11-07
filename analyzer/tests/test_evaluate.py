from scripts.evaluate import precision_at_k


def test_precision_at_k() -> None:
    labels = [True, False, True, False]
    scores = [0.9, 0.8, 0.1, 0.7]
    assert precision_at_k(labels, scores, 2) == 0.5
    assert precision_at_k(labels, scores, 1) == 1.0


def test_precision_at_k_shorter_than_k() -> None:
    assert precision_at_k([True, False], [0.2, 0.1], 10) == 0.5
