ALTER TABLE lead_row
	ADD COLUMN IF NOT EXISTS source_description TEXT;

COMMENT ON COLUMN lead_row.source_description IS 'SOURCE_DESCRIPTION из Bitrix. Исторические строки — миграция по bitrix_lead_id';
