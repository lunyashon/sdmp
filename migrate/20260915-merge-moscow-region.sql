-- One canonical Moscow region. Phone-search / Bitrix glued names stay as aliases.
WITH keep AS (
	SELECT id FROM region WHERE name = 'Московская область' LIMIT 1
), drop AS (
	SELECT id FROM region WHERE name IN ('Москва', 'Москва и Московская область')
)
UPDATE lead_row lr
SET region_id = keep.id
FROM keep, drop
WHERE lr.region_id = drop.id;

WITH keep AS (
	SELECT id FROM region WHERE name = 'Московская область' LIMIT 1
), drop AS (
	SELECT id FROM region WHERE name IN ('Москва', 'Москва и Московская область')
)
UPDATE region_alias a
SET region_id = keep.id
FROM keep, drop
WHERE a.region_id = drop.id;

INSERT INTO region_alias (alias, region_id)
SELECT v.alias, k.id
FROM region k
CROSS JOIN (VALUES
	('Москва'),
	('г. Москва'),
	('Москва и Московская область'),
	('Москва и Московская обл.'),
	('г. Москва и Московская область'),
	('г. Москва * Московская область'),
	('Московская область, г. Москва')
) AS v(alias)
WHERE k.name = 'Московская область'
ON CONFLICT (alias) DO UPDATE SET region_id = EXCLUDED.region_id;

INSERT INTO upk_region (upk_id, region_id)
SELECT DISTINCT u.upk_id, k.id
FROM upk_region u
JOIN region d ON d.id = u.region_id AND d.name IN ('Москва', 'Москва и Московская область')
JOIN region k ON k.name = 'Московская область'
ON CONFLICT DO NOTHING;

DELETE FROM upk_region u
USING region d
WHERE d.name IN ('Москва', 'Москва и Московская область')
  AND u.region_id = d.id;

INSERT INTO skorozvon_link_region (link_id, region_id)
SELECT DISTINCT u.link_id, k.id
FROM skorozvon_link_region u
JOIN region d ON d.id = u.region_id AND d.name IN ('Москва', 'Москва и Московская область')
JOIN region k ON k.name = 'Московская область'
ON CONFLICT DO NOTHING;

DELETE FROM skorozvon_link_region u
USING region d
WHERE d.name IN ('Москва', 'Москва и Московская область')
  AND u.region_id = d.id;

DELETE FROM region
WHERE name IN ('Москва', 'Москва и Московская область');
