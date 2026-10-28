-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE job_embeddings (
  job_id bigint references jobs (id),
  model_version text,
  embedding vector (384) NOT NULL,
  computed_at timestamptz DEFAULT now(),
  PRIMARY KEY (job_id, model_version)
);

-- +goose Down
DROP TABLE IF EXISTS job_embeddings;

DROP EXTENSION IF EXISTS vector;
