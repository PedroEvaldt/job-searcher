gup:
	goose -dir /mnt/ssd_m2/Programas/Portifolio/job-searcher/db/migrations/ postgres "host=localhost port=5433 user=postgres password=postgres dbname=jobs sslmode=disable" up
gdown:
	goose -dir /mnt/ssd_m2/Programas/Portifolio/job-searcher/db/migrations/ postgres "host=localhost port=5433 user=postgres password=postgres dbname=jobs sslmode=disable" down
gstatus:
	goose -dir /mnt/ssd_m2/Programas/Portifolio/job-searcher/db/migrations/ postgres "host=localhost port=5433 user=postgres password=postgres dbname=jobs sslmode=disable" status
.PHONY: gup gdown gstatus
