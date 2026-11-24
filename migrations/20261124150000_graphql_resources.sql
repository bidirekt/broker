CREATE TABLE graphql_resources (
  resource_id  BIGINT PRIMARY KEY,
  protocol     text NOT NULL CHECK (protocol = 'graphql'),
  operation    text NOT NULL CHECK (operation IN ('query', 'mutation')),
  field        text NOT NULL,
  interaction  text NOT NULL CHECK (interaction IN ('arguments', 'returns')),

  FOREIGN KEY (resource_id, protocol) REFERENCES resources (id, protocol)
);
