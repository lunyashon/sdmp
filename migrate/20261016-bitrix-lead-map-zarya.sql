-- CDP → Битрикс: результат проверки по Заре (как «Дубликат» в Скорозвоне).
INSERT INTO bitrix_lead_map (source_key, bitrix_code, fixed_value) VALUES
	('zarya', 'UF_CRM_1730362141931', '')
ON CONFLICT (source_key) DO NOTHING;

COMMENT ON COLUMN bitrix_lead_map.source_key IS 'Ключ CDP: phone, city, uuid, zarya, cold_label…';
