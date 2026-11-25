CREATE TABLE graphql_graph_members (
  contract_id  BIGINT PRIMARY KEY REFERENCES contracts(id),
  graph_name   text NOT NULL,
  federation1  boolean NOT NULL DEFAULT false
);

CREATE INDEX ON graphql_graph_members (graph_name);
