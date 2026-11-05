CREATE INDEX ON compatibility_checks (participant_id, environment_id, created_at DESC);
CREATE INDEX ON compatibility_checks (created_at DESC);
CREATE INDEX ON deployments (deployed_at DESC);
