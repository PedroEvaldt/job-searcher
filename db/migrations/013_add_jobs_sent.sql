-- +goose Up
CREATE TABLE jobs_sent (
  job_id bigint NOT NULL REFERENCES jobs (id),
  model_version text NOT NULL,
  sent_at timestamptz DEFAULT now(),
  PRIMARY KEY (job_id, model_version)
);

-- +goose Down
DROP TABLE IF EXISTS jobs_sent;
