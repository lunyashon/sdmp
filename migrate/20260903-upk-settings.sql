ALTER TABLE upk
	ADD COLUMN IF NOT EXISTS zarya_results TEXT[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN upk.zarya_results IS 'Что принимает от Зари: dup, unique. Ошибка ответа Зари никуда не уходит';

CREATE TABLE IF NOT EXISTS upk_region (
	upk_id INT NOT NULL REFERENCES upk(id) ON DELETE CASCADE,
	region_id INT NOT NULL REFERENCES region(id),
	PRIMARY KEY (upk_id, region_id)
);

COMMENT ON TABLE upk_region IS 'Разрешённые регионы покупателя. Пусто = регион не проверяется';

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_constraint WHERE conname = 'source_upk_pkey'
	) THEN
		ALTER TABLE source_upk ADD CONSTRAINT source_upk_pkey PRIMARY KEY (source_id, upk_id);
	END IF;
END $$;
