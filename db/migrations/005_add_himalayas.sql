-- +goose Up
INSERT INTO
  sources (name)
VALUES
  ('himalayas')
ON CONFLICT (name) DO NOTHING;

-- +goose Down
DELETE FROM sources
WHERE
  name = 'himalayas';
