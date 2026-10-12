CREATE TABLE participants (
  id          BIGSERIAL PRIMARY KEY,
  name        text NOT NULL UNIQUE,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE environments (
  id          BIGSERIAL PRIMARY KEY,
  name        text NOT NULL UNIQUE,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE contracts (
  id              BIGSERIAL PRIMARY KEY,
  participant_id  BIGINT NOT NULL REFERENCES participants(id),
  checksum        text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (participant_id, checksum)
);

CREATE TABLE contract_versions (
  id                BIGSERIAL PRIMARY KEY,
  participant_id    BIGINT NOT NULL REFERENCES participants(id),
  version           text NOT NULL,
  contract_id       BIGINT NOT NULL REFERENCES contracts(id),
  contract_content  text NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (participant_id, version)
);

CREATE INDEX ON contract_versions (contract_id);

CREATE TABLE resources (
  id                    BIGSERIAL PRIMARY KEY,
  participant_id        BIGINT NOT NULL REFERENCES participants(id),
  direction             text NOT NULL,
  interaction           text NOT NULL,
  consumed_provider     text,
  endpoint              text NOT NULL,
  method                text NOT NULL,
  response_status_code  text,
  provider_hash         text NOT NULL,
  consumer_hash         text,
  created_at            timestamptz NOT NULL DEFAULT now(),

  CHECK ((direction = 'provides') = (consumer_hash IS NULL))
);

CREATE INDEX ON resources (participant_id);
CREATE UNIQUE INDEX ON resources (provider_hash) WHERE direction = 'provides';
CREATE UNIQUE INDEX ON resources (consumer_hash) WHERE direction = 'consumes';
CREATE INDEX ON resources (provider_hash) WHERE direction = 'consumes';

CREATE TABLE resource_versions (
  id           BIGSERIAL PRIMARY KEY,
  resource_id  BIGINT NOT NULL REFERENCES resources(id),
  contract_id  BIGINT NOT NULL REFERENCES contracts(id),
  change_type  text NOT NULL CHECK (change_type IN ('added', 'removed')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (resource_id, contract_id)
);

CREATE INDEX ON resource_versions (resource_id, contract_id);
CREATE INDEX ON resource_versions (contract_id);

CREATE TABLE schemas (
  id              BIGSERIAL PRIMARY KEY,
  participant_id  BIGINT NOT NULL REFERENCES participants(id),
  name            text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (participant_id, name)
);

CREATE TABLE schema_versions (
  id           BIGSERIAL PRIMARY KEY,
  schema_id    BIGINT NOT NULL REFERENCES schemas(id),
  contract_id  BIGINT NOT NULL REFERENCES contracts(id),
  change_type  text NOT NULL CHECK (change_type IN ('added', 'removed')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (schema_id, contract_id)
);

CREATE INDEX ON schema_versions (contract_id);

CREATE TABLE fields (
  id          BIGSERIAL PRIMARY KEY,
  schema_id   BIGINT NOT NULL REFERENCES schemas(id),
  path        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (schema_id, path)
);

CREATE TABLE field_versions (
  id           BIGSERIAL PRIMARY KEY,
  field_id     BIGINT NOT NULL REFERENCES fields(id),
  contract_id  BIGINT NOT NULL REFERENCES contracts(id),
  type         text,
  optional     boolean NOT NULL,
  nullable     boolean NOT NULL,
  ref          text,
  change_type  text NOT NULL CHECK (change_type IN ('added', 'modified', 'removed')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (field_id, contract_id)
);

CREATE INDEX ON field_versions (contract_id);

CREATE TABLE resource_bodies (
  id           BIGSERIAL PRIMARY KEY,
  resource_id  BIGINT NOT NULL REFERENCES resources(id),
  contract_id  BIGINT NOT NULL REFERENCES contracts(id),
  source       text NOT NULL,
  schema_id    BIGINT REFERENCES schemas(id),
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON resource_bodies (contract_id, resource_id);

CREATE TABLE deployments (
  id              BIGSERIAL PRIMARY KEY,
  participant_id  BIGINT NOT NULL REFERENCES participants(id),
  version         text NOT NULL,
  environment_id  BIGINT NOT NULL REFERENCES environments(id),
  rollback        boolean NOT NULL,
  deployed_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON deployments (participant_id, environment_id, deployed_at DESC);

CREATE TABLE compatibility_verdicts (
  contract_id_one bigint      NOT NULL REFERENCES contracts(id),
  contract_id_two bigint      NOT NULL REFERENCES contracts(id),
  breaks          jsonb       NOT NULL DEFAULT '[]',
  deployable      boolean     GENERATED ALWAYS AS (jsonb_array_length(breaks) = 0) STORED,
  created_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (contract_id_one, contract_id_two),
  CHECK (contract_id_one <= contract_id_two)
);

CREATE TABLE compatibility_checks (
  id             BIGSERIAL   PRIMARY KEY,
  participant_id bigint      NOT NULL REFERENCES participants(id),
  contract_id    bigint      NOT NULL REFERENCES contracts(id),
  version        text        NOT NULL,
  environment_id bigint      NOT NULL REFERENCES environments(id),
  deployable     boolean     NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON compatibility_checks (participant_id, version, created_at DESC);

CREATE TABLE compatibility_check_results (
  id                         BIGSERIAL PRIMARY KEY,
  check_id                   bigint  NOT NULL REFERENCES compatibility_checks(id),
  counterpart_name           text    NOT NULL,
  counterpart_participant_id bigint  REFERENCES participants(id),
  counterpart_version        text,
  verdict_contract_id_one    bigint,
  verdict_contract_id_two    bigint,
  deployable                 boolean NOT NULL,
  FOREIGN KEY (verdict_contract_id_one, verdict_contract_id_two)
    REFERENCES compatibility_verdicts (contract_id_one, contract_id_two)
);

CREATE INDEX ON compatibility_check_results (check_id);
