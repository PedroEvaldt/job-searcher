-- +goose Up
ALTER TABLE jobs
ADD COLUMN company_key text,
ADD COLUMN title_key text;

CREATE INDEX idx_jobs_dedupe_key ON jobs (company_key, title_key);

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_dedupe_key;

ALTER TABLE jobs
DROP COLUMN IF EXISTS company_key,
DROP COLUMN IF EXISTS title_key;
