ALTER TABLE compatibility_checks
  ADD COLUMN report           jsonb NULL,
  ADD COLUMN environment_name text  NULL;

ALTER TABLE compatibility_check_results
  ADD COLUMN role text NULL CHECK (role IN ('provider','consumer','both'));
