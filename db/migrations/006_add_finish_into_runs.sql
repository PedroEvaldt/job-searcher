-- +goose Up
ALTER TABLE runs
ADD COLUMN finished_at timestamptz;

-- +goose Down
ALTER TABLE runs
DROP COLUMN IF EXISTS finished_at;
