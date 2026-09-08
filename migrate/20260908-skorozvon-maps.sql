CREATE TABLE IF NOT EXISTS skorozvon_maps (
	field_id BIGINT NOT NULL,
	option_id BIGINT NOT NULL DEFAULT 0,
	name TEXT NOT NULL,
	field_type VARCHAR,
	lead_type VARCHAR,
	updated_at TIMESTAMP NOT NULL DEFAULT now(),
	PRIMARY KEY (field_id, option_id)
);

COMMENT ON TABLE skorozvon_maps IS 'справочник настраиваемых полей Скорозвона (GET /api/v2/custom_fields). Пишет worker в 12:00 и 19:00 Europe/Moscow';
COMMENT ON COLUMN skorozvon_maps.field_id IS 'id поля в СК. В webhook ключ FIELD_{field_id}';
COMMENT ON COLUMN skorozvon_maps.option_id IS '0 = само поле (name = название поля). Иначе id опции select/multiselect (name = label опции)';
COMMENT ON COLUMN skorozvon_maps.name IS 'название поля или подпись опции';
COMMENT ON COLUMN skorozvon_maps.field_type IS 'date, multiselect, select, link, text, textarea, number, comment';
COMMENT ON COLUMN skorozvon_maps.lead_type IS 'lead или organization';
COMMENT ON COLUMN skorozvon_maps.updated_at IS 'когда worker последний раз обновил строку';
