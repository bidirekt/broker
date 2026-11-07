CREATE TABLE current_deployments (
  participant_id  BIGINT NOT NULL REFERENCES participants(id),
  environment_id  BIGINT NOT NULL REFERENCES environments(id),
  version         text NOT NULL,
  contract_id     BIGINT NOT NULL REFERENCES contracts(id),
  deployed_at     timestamptz NOT NULL,
  PRIMARY KEY (participant_id, environment_id)
);

INSERT INTO current_deployments
  (participant_id, environment_id, version, contract_id, deployed_at)
SELECT
  latest.participant_id,
  latest.environment_id,
  latest.version,
  (SELECT cv.contract_id FROM contract_versions cv
    WHERE cv.participant_id = latest.participant_id AND cv.version = latest.version),
  latest.deployed_at
FROM (
  SELECT DISTINCT ON (participant_id, environment_id)
    participant_id, environment_id, version, deployed_at
  FROM deployments
  ORDER BY participant_id, environment_id, deployed_at DESC, id DESC
) latest;

CREATE INDEX ON contract_versions (participant_id, id DESC);

CREATE VIEW latest_contract_versions AS
SELECT p.id AS participant_id, cv.version, cv.contract_id, cv.created_at
FROM participants p
CROSS JOIN LATERAL (
  SELECT version, contract_id, created_at
  FROM contract_versions
  WHERE participant_id = p.id
  ORDER BY id DESC
  LIMIT 1
) cv;

ALTER TABLE compatibility_checks
  ADD COLUMN report           jsonb NULL,
  ADD COLUMN environment_name text  NULL;

ALTER TABLE compatibility_check_results
  ADD COLUMN role text NULL CHECK (role IN ('provider','consumer','both'));

ALTER TABLE sessions
  ADD COLUMN messages jsonb NULL CHECK (jsonb_typeof(messages) = 'array');

CREATE INDEX ON compatibility_checks (participant_id, created_at DESC);

CREATE INDEX ON resources (consumed_provider) WHERE direction = 'consumes';

CREATE INDEX ON deployments (environment_id, deployed_at DESC);

CREATE INDEX ON compatibility_checks (environment_id, created_at DESC);

CREATE INDEX ON compatibility_checks (participant_id, environment_id, created_at DESC, id DESC);
CREATE INDEX ON compatibility_checks (created_at DESC);
CREATE INDEX ON deployments (deployed_at DESC);
