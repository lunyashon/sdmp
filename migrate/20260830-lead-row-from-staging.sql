-- Исторические строки: s3_key может быть пустым, сырьё пока в legacy_fields.
ALTER TABLE lead_row ALTER COLUMN s3_key DROP NOT NULL;
ALTER TABLE lead_row ADD COLUMN IF NOT EXISTS legacy_fields TEXT;
ALTER TABLE lead_row ADD COLUMN IF NOT EXISTS bitrix_lead_id VARCHAR;

COMMENT ON COLUMN lead_row.s3_key IS 'ключ объекта в S3. NULL у мигрированных исторических строк до выгрузки в S3';
COMMENT ON COLUMN lead_row.legacy_fields IS 'json-слепок легаси-полей Bitrix при миграции';
COMMENT ON COLUMN lead_row.bitrix_lead_id IS 'ID лида в Bitrix (owner_id из выгрузки)';

UPDATE source
SET parent_id = (SELECT id FROM source WHERE bitrix_code = '141')
WHERE bitrix_code IN ('UC_VKLEXV', 'UC_CJQR7Z', 'UC_LQNNKH')
  AND parent_id IS NULL;

INSERT INTO operator (name, bitrix_name)
SELECT v.name, v.bitrix_name
FROM (VALUES
	('БИЛАЙН', 'Билайн / Вымпелком'),
	('МТС', 'МТС'),
	('МЕГАФОН', 'Мегафон / Йота'),
	('ТЕЛЕ-2', 'Теле2 / Т2'),
	('ДРУГИЕ', NULL)
) AS v(name, bitrix_name)
WHERE NOT EXISTS (SELECT 1 FROM operator o WHERE o.name = v.name);
