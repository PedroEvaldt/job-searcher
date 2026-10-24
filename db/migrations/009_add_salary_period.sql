-- +goose Up
ALTER TABLE jobs
ADD COLUMN salary_period text;

-- +goose Down
ALTER TABLE jobs
DROP COLUMN IF EXISTS salary_period;
