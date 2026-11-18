ALTER TABLE google_sheet
	ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

COMMENT ON COLUMN google_sheet.is_active IS 'false = инстанс выключен, запись в таблицу не должна идти';
