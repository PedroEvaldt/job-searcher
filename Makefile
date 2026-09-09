gup:
	goose -dir ./db/migrations/ postgres "host=localhost port=5433 user=postgres password=postgres dbname=jobs sslmode=disable" up
gdown:
	goose -dir ./db/migrations/ postgres "host=localhost port=5433 user=postgres password=postgres dbname=jobs sslmode=disable" down
gstatus:
	goose -dir ./db/migrations/ postgres "host=localhost port=5433 user=postgres password=postgres dbname=jobs sslmode=disable" status
.PHONY: gup gdown gstatus
