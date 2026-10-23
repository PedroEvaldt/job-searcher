-- +goose Up
ALTER TABLE jobs
ADD COLUMN skills_extracted_at timestamptz;

-- +goose Down
ALTER TABLE jobs
DROP COLUMN IF EXISTS skills_extracted_at;
