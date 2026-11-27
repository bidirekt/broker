CREATE TABLE graphql_type_info (
  contract_id  BIGINT NOT NULL REFERENCES contracts(id),
  coordinate   text NOT NULL,
  kind         text NOT NULL CHECK (kind IN ('object', 'input', 'interface', 'union', 'enum', 'scalar', 'field', 'argument')),
  named_type   text NOT NULL DEFAULT '',
  PRIMARY KEY (contract_id, coordinate)
);

CREATE TABLE graphql_root_types (
  contract_id  BIGINT NOT NULL REFERENCES contracts(id),
  operation    text NOT NULL CHECK (operation IN ('query', 'mutation')),
  type_name    text NOT NULL,
  PRIMARY KEY (contract_id, operation)
);

CREATE TABLE graphql_federation_directives (
  contract_id  BIGINT NOT NULL REFERENCES contracts(id),
  coordinate   text NOT NULL,
  directive    text NOT NULL CHECK (directive IN ('key', 'shareable', 'inaccessible', 'external', 'override', 'provides')),
  fields       text NOT NULL DEFAULT '',
  override_from text NOT NULL DEFAULT '',
  resolvable   boolean NOT NULL DEFAULT true,
  PRIMARY KEY (contract_id, coordinate, directive, fields)
);
