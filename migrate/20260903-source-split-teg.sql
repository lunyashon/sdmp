ALTER TABLE source
	ADD COLUMN IF NOT EXISTS teg_active BOOLEAN NOT NULL DEFAULT false,
	ADD COLUMN IF NOT EXISTS teg_expected_value VARCHAR;

COMMENT ON COLUMN source.teg_active IS 'Проверка teg после дробления по оператору. Только у корня';
COMMENT ON COLUMN source.teg_expected_value IS 'Ожидаемое значение teg (например подтверждён). Не совпало — тестовый под-источник правила';

ALTER TABLE source_split
	ADD COLUMN IF NOT EXISTS test_child_source_id INT REFERENCES source(id);

COMMENT ON COLUMN source_split.test_child_source_id IS 'Куда уходит строка, если teg не совпал. Nullable если проверка выключена';

CREATE UNIQUE INDEX IF NOT EXISTS source_split_source_operator_uidx
	ON source_split (source_id, operator_id);
