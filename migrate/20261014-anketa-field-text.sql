ALTER TABLE skorozvon_anketa_fields
	ADD COLUMN IF NOT EXISTS value_text VARCHAR;

COMMENT ON COLUMN skorozvon_anketa_fields.value_text IS 'текст custom_field СК, если это не select (ФИО из lead.name как FIELD_50000007333)';
