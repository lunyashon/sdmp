ALTER TABLE source_split
	ADD COLUMN IF NOT EXISTS cold_label VARCHAR,
	ADD COLUMN IF NOT EXISTS base_label VARCHAR;

COMMENT ON COLUMN source_split.cold_label IS 'Холодная база этого правила. Не общая на под-источник';
COMMENT ON COLUMN source_split.base_label IS 'Какая база этого правила. Не общая на под-источник';

UPDATE source_split r
SET
	cold_label = COALESCE(r.cold_label, c.cold_label),
	base_label = COALESCE(r.base_label, c.base_label)
FROM source c
WHERE c.id = r.child_source_id
  AND (r.cold_label IS NULL OR r.base_label IS NULL);
