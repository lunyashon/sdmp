ALTER TABLE lead_row
	ADD COLUMN IF NOT EXISTS via_us BOOLEAN NOT NULL DEFAULT true;

COMMENT ON COLUMN lead_row.via_us IS 'false = строка создана из webhook Скорозвона, лид не проходил наш приём. true = приняли мы';
