# goose against the database in .env (the same DATABASE_URL the collector reads). The
# collector also applies pending migrations on its own at startup; these targets are
# for checking the state or rolling back by hand.
GOOSE = set -a && . ./.env && set +a && goose -dir ./db/migrations postgres "$$DATABASE_URL"

gup:
	@$(GOOSE) up
gdown:
	@$(GOOSE) down
gstatus:
	@$(GOOSE) status
.PHONY: gup gdown gstatus
