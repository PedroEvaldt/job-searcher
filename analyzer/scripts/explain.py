"""Print the coefficients of a saved sklearn-combined model.

    uv run python -m scripts.explain [model_version]

Defaults to the current MODEL_VERSION. Coefficients are on the scaled features, so
their sizes compare directly.
"""

import sys

from jobradar.model.combined import MODEL_VERSION, coefficients, load, model_path


def main() -> int:
    version = sys.argv[1] if len(sys.argv) > 1 else MODEL_VERSION
    path = model_path(version)
    if not path.exists():
        print(f"no model at {path}", file=sys.stderr)
        return 1
    pipeline = load(path)
    print(f"{version} (C={pipeline.named_steps['model'].C})")
    for name, weight in coefficients(pipeline):
        print(f"  {name:<16} {weight:+.3f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
