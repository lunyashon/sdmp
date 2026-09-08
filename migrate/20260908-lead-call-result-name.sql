ALTER TABLE lead_row
	ALTER COLUMN call_result TYPE BIGINT;

ALTER TABLE lead_row
	ADD COLUMN IF NOT EXISTS call_result_name VARCHAR;

COMMENT ON COLUMN lead_row.call_result IS 'id результата сценария Скорозвона. Nullable';
COMMENT ON COLUMN lead_row.call_result_name IS 'название результата сценария Скорозвона (call_result.result_name). Nullable';
