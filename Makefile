# Day-to-day commands. Database targets read DATABASE_URL from .env, like the apps do.
LOAD_ENV = set -a && . ./.env && set +a
GOOSE = $(LOAD_ENV) && goose -dir ./db/migrations postgres "$$DATABASE_URL"
ANALYZER = cd analyzer && uv run

.PHONY: help build test test-go test-py lint lint-go lint-py up down gup gdown gstatus

help: ## list the targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-10s %s\n", $$1, $$2}'

build: ## build the collector and radar binaries into ./bin
	go build -o bin/collector ./cmd/collector
	go build -o bin/radar ./cmd/radar

test: test-go test-py ## run every test (the Go integration tests need Docker)

test-go:
	go test ./...

test-py:
	$(ANALYZER) --group dev pytest -q

lint: lint-go lint-py ## gofmt, go vet, golangci-lint, ruff, mypy --strict

lint-go:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	go vet ./...
	golangci-lint run ./...

lint-py:
	$(ANALYZER) ruff check .
	$(ANALYZER) ruff format --check .
	$(ANALYZER) --group dev mypy

up: ## on the Pi: build the images one at a time, then start the whole stack
	COMPOSE_PARALLEL_LIMIT=1 docker compose build
	docker compose up -d

down: ## stop the stack, keeping the data volume
	docker compose down

# goose by hand: the collector already migrates on its own at start-up; these are for
# checking the state or rolling back.
gup:
	@$(GOOSE) up
gdown:
	@$(GOOSE) down
gstatus:
	@$(GOOSE) status
