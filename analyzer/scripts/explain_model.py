import sys

import joblib

from scripts.train_model import COLS, MODEL_PATH


def explain_model() -> int:
    """Print the 3 largest-magnitude coefficients of the saved combined model."""
    pipeline = joblib.load(MODEL_PATH)
    model = pipeline.named_steps["model"]
    names = COLS[:-1]
    pairs = sorted(zip(names, model.coef_[0]), key=lambda p: abs(p[1]), reverse=True)

    for name, coef in pairs:
        sign = "+" if coef >= 0 else "-"
        print(f"{name}: {coef:.4f} ({sign})")
    return 0


if __name__ == "__main__":
    sys.exit(explain_model())
