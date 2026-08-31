-- +goose Up
ALTER TABLE jobs
ADD COLUMN source_id int NOT NULL,
ADD COLUMN external_id text NOT NULL,
ADD CONSTRAINT fk_source_id FOREIGN KEY (source_id) REFERENCES sources (id),
ADD CONSTRAINT uq_source_id_external_id UNIQUE (source_id, external_id);

-- +goose Down
ALTER TABLE jobs
DROP CONSTRAINT fk_source_id,
DROP CONSTRAINT uq_source_id_external_id,
DROP COLUMN source_id,
DROP COLUMN external_id;
