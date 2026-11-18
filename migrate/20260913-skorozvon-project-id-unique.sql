DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'skorozvon_project'::regclass
      AND contype = 'u'
      AND conname = 'skorozvon_project_project_id_key'
  ) THEN
    ALTER TABLE skorozvon_project
      ADD CONSTRAINT skorozvon_project_project_id_key UNIQUE (project_id);
  END IF;
END $$;
