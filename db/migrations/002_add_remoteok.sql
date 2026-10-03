-- +goose Up
INSERT INTO
  sources (name)
VALUES
  ('remoteok')
ON CONFLICT (name) DO NOTHING;

-- +goose Down
DELETE FROM sources
WHERE
  name = 'remoteok';
