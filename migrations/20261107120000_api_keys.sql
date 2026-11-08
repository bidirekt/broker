CREATE TABLE api_keys (
  id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id         bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key_hash        bytea NOT NULL UNIQUE,  -- SHA-256 of the whole key
  display_prefix  text NOT NULL,
  name            text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
  permission      text NOT NULL CHECK (permission IN ('read', 'write')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  last_used_at    timestamptz NULL,
  expires_at      timestamptz NULL  -- NULL = never
);

CREATE INDEX api_keys_user_id_idx ON api_keys (user_id);
