CREATE TABLE graphql_interface_implementations (
  contract_id     BIGINT NOT NULL REFERENCES contracts(id),
  interface_name  text NOT NULL,
  implements      text NOT NULL,
  PRIMARY KEY (contract_id, interface_name, implements)
);
