ALTER TABLE lead_row
	ADD COLUMN IF NOT EXISTS cold_label VARCHAR,
	ADD COLUMN IF NOT EXISTS base_label VARCHAR;

COMMENT ON COLUMN lead_row.cold_label IS 'Холодная база после дробления. Пишет Worker из source_split';
COMMENT ON COLUMN lead_row.base_label IS 'Какая база после дробления. Пишет Worker из source_split';
