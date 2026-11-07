import numpy as np

from jobradar.model.combined import (
    CV_FOLDS,
    MODEL_VERSION,
    coefficients,
    model_path,
    predict,
    train,
)
from jobradar.model.features import FEATURES


def synthetic(n: int = 200, seed: int = 0) -> tuple[np.ndarray, np.ndarray]:
    """y depends on the first feature only, with some NaNs in years_required."""
    rng = np.random.default_rng(seed)
    x = rng.normal(size=(n, len(FEATURES)))
    x[rng.random(n) < 0.5, FEATURES.index("years_required")] = np.nan
    y = (x[:, 0] + rng.normal(scale=0.5, size=n) > 1).astype(float)
    return x, y


def test_train_reports_every_fold() -> None:
    x, y = synthetic()
    result = train(x, y)
    assert len(result.fold_ap) == CV_FOLDS
    assert result.n == len(y)
    assert result.n_pos == int(y.sum())
    assert 0 < result.prevalence < 1


def test_train_is_deterministic() -> None:
    x, y = synthetic()
    first, second = train(x, y), train(x, y)
    assert first.best_c == second.best_c
    assert first.fold_ap == second.fold_ap


def test_coefficients_find_the_signal() -> None:
    x, y = synthetic()
    result = train(x, y)
    name, weight = coefficients(result.pipeline)[0]
    assert name == FEATURES[0]
    assert weight > 0


def test_predict_returns_probabilities() -> None:
    x, y = synthetic()
    probs = predict(train(x, y).pipeline, x)
    assert probs.shape == (len(y),)
    assert ((probs >= 0) & (probs <= 1)).all()


def test_model_path_is_named_by_version() -> None:
    assert model_path().name == f"{MODEL_VERSION}.joblib"
