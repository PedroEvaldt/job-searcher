-- +goose Up
create table sources (
  id serial primary key,
  name text unique not null,
  enabled boolean not null default true,
  last_run_at timestamptz
);

create table raw_jobs (
  id bigserial primary key,
  source_id int not null references sources (id),
  external_id text not null,
  payload jsonb not null,
  content_hash text not null,
  fetched_at timestamptz not null default now(),
  unique (source_id, external_id, content_hash)
);

create table jobs (
  id bigserial primary key,
  raw_job_id bigint not null references raw_jobs (id),
  title text not null,
  company text not null,
  url text not null,
  description text not null,
  remote_scope text,
  salary_min numeric,
  salary_max numeric,
  currency text,
  posted_at timestamptz,
  scored_at timestamptz,
  duplicate_of bigint references jobs (id)
);

create table job_skills (
  job_id bigint not null references jobs (id),
  skill text not null,
  confidence real not null,
  primary key (job_id, skill)
);

create table labels (
  job_id bigint primary key references jobs (id),
  would_apply boolean not null,
  hires_latam boolean,
  seniority text,
  note text,
  labeled_at timestamptz not null default now()
);

create table scores (
  job_id bigint not null references jobs (id),
  model_version text not null,
  score real not null,
  computed_at timestamptz not null default now(),
  primary key (job_id, model_version)
);

create table runs (
  id bigserial primary key,
  source_id int references sources (id),
  started_at timestamptz not null,
  fetched int,
  failed int,
  error text
);

create index on raw_jobs (source_id, fetched_at desc);

create index on jobs (posted_at desc);

create index on jobs (scored_at)
where
  scored_at is null;

create index on job_skills (skill);

-- +goose Down
drop table scores;

drop table labels;

drop table job_skills;

drop table jobs;

drop table raw_jobs;

drop table runs;

drop table sources;
