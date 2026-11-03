CREATE TABLE users (
  id             BIGSERIAL PRIMARY KEY,
  email          text NOT NULL,
  permission     text NOT NULL CHECK (permission IN ('read', 'write', 'admin')),
  password_hash  text NULL,  -- NULL = invited, password not set yet
  active         boolean NOT NULL DEFAULT true,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_unique ON users (lower(email));
CREATE UNIQUE INDEX users_one_admin ON users (permission) WHERE permission = 'admin';

CREATE TABLE sessions (
  id_hash       bytea PRIMARY KEY,  -- SHA-256 of the cookie value
  user_id       BIGINT NOT NULL REFERENCES users(id),
  csrf_token    text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  last_seen_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE one_time_links (
  token_hash  bytea PRIMARY KEY,  -- SHA-256 of the token in the link
  user_id     BIGINT NOT NULL REFERENCES users(id),
  purpose     text NOT NULL CHECK (purpose IN ('invite', 'reset')),
  expires_at  timestamptz NOT NULL,
  used_at     timestamptz NULL
);

CREATE UNIQUE INDEX one_time_links_one_per_user ON one_time_links (user_id) WHERE used_at IS NULL;
