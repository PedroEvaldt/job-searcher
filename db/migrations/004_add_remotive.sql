-- +goose Up
INSERT INTO
  sources (name)
VALUES
  ('remotive')
ON CONFLICT (name) DO NOTHING;

-- +goose Down
DELETE FROM sources
WHERE
  name = 'remotive';
