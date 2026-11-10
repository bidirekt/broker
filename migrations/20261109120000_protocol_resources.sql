ALTER TABLE resources
  ADD COLUMN protocol  text,
  ADD COLUMN body_role text;

UPDATE resources
SET protocol  = 'rest',
    body_role = CASE WHEN interaction = 'rest_request' THEN 'input' ELSE 'output' END;

ALTER TABLE resources
  ALTER COLUMN protocol SET NOT NULL,
  ALTER COLUMN body_role SET NOT NULL,
  ADD CHECK (body_role IN ('input', 'output')),
  ADD CHECK (direction IN ('provides', 'consumes')),
  ADD UNIQUE (id, protocol);

CREATE TABLE rest_resources (
  resource_id           BIGINT PRIMARY KEY,
  protocol              text NOT NULL CHECK (protocol = 'rest'),
  endpoint              text NOT NULL,
  method                text NOT NULL,
  response_status_code  text,

  FOREIGN KEY (resource_id, protocol) REFERENCES resources (id, protocol)
);

INSERT INTO rest_resources (resource_id, protocol, endpoint, method, response_status_code)
SELECT id, protocol, endpoint, method, response_status_code
FROM resources;

UPDATE compatibility_verdicts
SET breaks = COALESCE(
  (
    SELECT jsonb_agg(
      jsonb_build_object(
        'rest', jsonb_build_object('endpoint', b->'endpoint', 'method', b->'method', 'interaction', b->'interaction'),
        'reason', b->'reason',
        'details', b->'details'
      ) ORDER BY i
    )
    FROM jsonb_array_elements(breaks) WITH ORDINALITY t(b, i)
  ),
  '[]'
);

ALTER TABLE resources
  DROP COLUMN interaction,
  DROP COLUMN endpoint,
  DROP COLUMN method,
  DROP COLUMN response_status_code;
