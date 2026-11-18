ALTER TABLE google_sheet
	ADD COLUMN IF NOT EXISTS zarya_results TEXT[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN google_sheet.zarya_results IS 'Что принимает от Зари: dup, unique. Как у покупателя (upk)';

CREATE TABLE IF NOT EXISTS google_sheet_region (
	sheet_id INT NOT NULL REFERENCES google_sheet(id) ON DELETE CASCADE,
	region_id INT NOT NULL REFERENCES region(id),
	PRIMARY KEY (sheet_id, region_id)
);

COMMENT ON TABLE google_sheet_region IS 'Разрешённые регионы инстанса Google Sheets. Пусто = регион не проверяется';

CREATE TABLE IF NOT EXISTS google_sheet_stop_source (
	sheet_id INT NOT NULL REFERENCES google_sheet(id) ON DELETE CASCADE,
	source_id INT NOT NULL REFERENCES source(id),
	PRIMARY KEY (sheet_id, source_id)
);

COMMENT ON TABLE google_sheet_stop_source IS 'Стоп-источники инстанса Google Sheets. Строки этих источников не уходят в таблицу';
