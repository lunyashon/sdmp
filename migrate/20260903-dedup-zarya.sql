INSERT INTO dedup_provider (name, is_active)
SELECT 'Заря', true
WHERE NOT EXISTS (
	SELECT 1 FROM dedup_provider WHERE lower(name) IN ('заря', 'zarya')
);

INSERT INTO dedup_value (id, provider_id, name)
SELECT v.id, p.id, v.name
FROM dedup_provider p
CROSS JOIN (
	VALUES
		(1::smallint, 'Чистый'),
		(2::smallint, 'Дубль'),
		(3::smallint, 'Ошибка проверки')
) AS v(id, name)
WHERE lower(p.name) IN ('заря', 'zarya')
  AND NOT EXISTS (SELECT 1 FROM dedup_value d WHERE d.id = v.id);
