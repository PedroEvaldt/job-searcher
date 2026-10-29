-- +goose Up
ALTER TABLE jobs
ADD COLUMN language text;

-- +goose Down
ALTER TABLE jobs
DROP COLUMN IF EXISTS language;
