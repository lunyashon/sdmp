-- Старая проверка Зари (crm.duplicate.findbycomm). Чистый / дубль.
INSERT INTO dedup_provider (name, is_active)
SELECT 'Заря старая', true
WHERE NOT EXISTS (
	SELECT 1 FROM dedup_provider WHERE lower(name) IN ('заря старая', 'zarya_old')
);

INSERT INTO dedup_value (id, provider_id, name)
SELECT v.id, p.id, v.name
FROM dedup_provider p
CROSS JOIN (VALUES
	(4::smallint, 'Чистый'),
	(5::smallint, 'дубль')
) AS v(id, name)
WHERE lower(p.name) IN ('заря старая', 'zarya_old')
  AND NOT EXISTS (SELECT 1 FROM dedup_value d WHERE d.id = v.id);

INSERT INTO bitrix_lead_map (source_key, bitrix_code, fixed_value) VALUES
	('zarya_old', '', '')
ON CONFLICT (source_key) DO NOTHING;
