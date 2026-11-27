ALTER TABLE schemas ADD COLUMN namespace text NOT NULL DEFAULT 'yaml';
ALTER TABLE schemas DROP CONSTRAINT schemas_participant_id_name_key;
ALTER TABLE schemas ADD CONSTRAINT schemas_participant_id_namespace_name_key UNIQUE (participant_id, namespace, name);
