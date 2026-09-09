ALTER TABLE lead_row
	ADD COLUMN IF NOT EXISTS operator_raw VARCHAR;

COMMENT ON COLUMN lead_row.operator_raw IS 'Сырое значение оператора от поставщика или определителя до верхнего маппинга. Пусто — не было чего резолвить.';

CREATE INDEX IF NOT EXISTS lead_row_operator_raw_idx
	ON lead_row (operator_id, operator_raw)
	WHERE operator_raw IS NOT NULL AND operator_raw <> '';
