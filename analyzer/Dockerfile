# Python side of job-radar: one image for the "analyzer" (skills + tfidf, then exit) and
# "jev-worker" (Jev answers, long-running) compose services.
FROM python:3.13-slim

COPY --from=ghcr.io/astral-sh/uv:0.11.7 /uv /uvx /bin/

WORKDIR /app

# On the Pi, with the other images building at the same time, a bytecode-compile worker
# can take longer than uv's default 60 s to start, and the whole sync fails.
ENV UV_COMPILE_BYTECODE=1 UV_COMPILE_BYTECODE_TIMEOUT=600 UV_LINK_MODE=copy

# Dependencies first, in their own layer: code changes do not reinstall them.
COPY pyproject.toml uv.lock ./
RUN uv sync --locked --no-dev --group jev --no-install-project

COPY . .
RUN uv sync --locked --no-dev --group jev

CMD ["uv", "run", "--no-dev", "python", "-m", "scripts.main"]
