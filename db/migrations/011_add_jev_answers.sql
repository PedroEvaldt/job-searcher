-- +goose Up
CREATE TABLE jev_answers (
  job_id bigint NOT NULL REFERENCES jobs (id),
  question_set_version text NOT NULL,
  answers jsonb NOT NULL,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (job_id, question_set_version)
);

-- +goose Down
DROP TABLE IF EXISTS jev_answers;
