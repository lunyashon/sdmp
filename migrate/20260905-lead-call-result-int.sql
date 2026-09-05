ALTER TABLE lead_row
	ALTER COLUMN call_result TYPE INTEGER;

COMMENT ON COLUMN lead_row.call_result IS 'id результата сценария Скорозвона. Nullable';

CREATE INDEX IF NOT EXISTS dispatch_skorozvon_external_id_idx
	ON dispatch (external_id)
	WHERE channel_id = 1 AND external_id IS NOT NULL;
