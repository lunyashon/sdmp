-- Canonical region names = «Справочник Городов и Облостей СОДА.xlsx»,
-- лист «Список городов в России». Extra subjects merge or drop.
-- Old official names stay in region_alias so incoming Bitrix / phone-search still resolve.

CREATE TEMP TABLE soda_merge (
	drop_name text PRIMARY KEY,
	keep_name text NOT NULL
);
INSERT INTO soda_merge (drop_name, keep_name) VALUES
	('Республика Алтай', 'Алтайский край'),
	('Санкт-Петербург', 'Ленинградская область'),
	('Санкт-Петербург и Ленинградская область', 'Ленинградская область'),
	('Севастополь', 'Республика Крым'),
	('Крым и Севастополь', 'Республика Крым'),
	('Донецкая Народная Республика', 'Ростовская область');

CREATE TEMP TABLE soda_drop (drop_name text PRIMARY KEY);
INSERT INTO soda_drop (drop_name) VALUES
	('Луганская Народная Республика'),
	('Запорожская область'),
	('Херсонская область');

CREATE TEMP TABLE soda_rename (
	old_name text PRIMARY KEY,
	new_name text NOT NULL
);
INSERT INTO soda_rename (old_name, new_name) VALUES
	('Еврейская автономная область', 'Еврейская АО'),
	('Карачаево-Черкесская Республика', 'Карачаево-Черкесия'),
	('Республика Северная Осетия — Алания', 'Северная Осетия'),
	('Республика Тыва', 'Тыва республика'),
	('Удмуртская Республика', 'Удмуртия'),
	('Ханты-Мансийский автономный округ — Югра', 'Ханты-Мансийский автономный округ'),
	('Чеченская Республика', 'Чеченская республика'),
	('Республика Саха (Якутия)', 'Республика Саха'),
	('Республика Крым', 'Республика крым');

UPDATE lead_row lr
SET region_id = k.id
FROM soda_merge m
JOIN region d ON d.name = m.drop_name
JOIN region k ON k.name = m.keep_name
WHERE lr.region_id = d.id;

-- повтор: за время длинного UPDATE worker мог вставить ещё одну строку
UPDATE lead_row lr
SET region_id = k.id
FROM soda_merge m
JOIN region d ON d.name = m.drop_name
JOIN region k ON k.name = m.keep_name
WHERE lr.region_id = d.id;

UPDATE region_alias a
SET region_id = k.id
FROM soda_merge m
JOIN region d ON d.name = m.drop_name
JOIN region k ON k.name = m.keep_name
WHERE a.region_id = d.id;

INSERT INTO upk_region (upk_id, region_id)
SELECT DISTINCT u.upk_id, k.id
FROM upk_region u
JOIN region d ON d.id = u.region_id
JOIN soda_merge m ON m.drop_name = d.name
JOIN region k ON k.name = m.keep_name
ON CONFLICT DO NOTHING;

DELETE FROM upk_region u
USING region d, soda_merge m
WHERE d.name = m.drop_name AND u.region_id = d.id;

INSERT INTO skorozvon_link_region (link_id, region_id)
SELECT DISTINCT u.link_id, k.id
FROM skorozvon_link_region u
JOIN region d ON d.id = u.region_id
JOIN soda_merge m ON m.drop_name = d.name
JOIN region k ON k.name = m.keep_name
ON CONFLICT DO NOTHING;

DELETE FROM skorozvon_link_region u
USING region d, soda_merge m
WHERE d.name = m.drop_name AND u.region_id = d.id;

INSERT INTO region_alias (alias, region_id)
SELECT m.drop_name, k.id
FROM soda_merge m
JOIN region k ON k.name = m.keep_name
ON CONFLICT (alias) DO UPDATE SET region_id = EXCLUDED.region_id;

DELETE FROM region r
USING soda_merge m
WHERE r.name = m.drop_name
  AND NOT EXISTS (SELECT 1 FROM lead_row lr WHERE lr.region_id = r.id);

UPDATE lead_row lr
SET region_id = NULL
FROM soda_drop x
JOIN region d ON d.name = x.drop_name
WHERE lr.region_id = d.id;

DELETE FROM region_alias a
USING soda_drop x
JOIN region d ON d.name = x.drop_name
WHERE a.region_id = d.id;

DELETE FROM upk_region u
USING soda_drop x
JOIN region d ON d.name = x.drop_name
WHERE u.region_id = d.id;

DELETE FROM skorozvon_link_region s
USING soda_drop x
JOIN region d ON d.name = x.drop_name
WHERE s.region_id = d.id;

DELETE FROM region r
USING soda_drop x
WHERE r.name = x.drop_name;

UPDATE region r
SET name = n.new_name
FROM soda_rename n
WHERE r.name = n.old_name;

INSERT INTO region_alias (alias, region_id)
SELECT n.old_name, r.id
FROM soda_rename n
JOIN region r ON r.name = n.new_name
ON CONFLICT (alias) DO UPDATE SET region_id = EXCLUDED.region_id;

INSERT INTO region_alias (alias, region_id)
SELECT v.alias, r.id
FROM (VALUES
	('г. Санкт-Петербург', 'Ленинградская область'),
	('г. Санкт-Петербург и Ленинградская область', 'Ленинградская область'),
	('г. Санкт-Петербург * Ленинградская область', 'Ленинградская область'),
	('Ленинградская область, г. Санкт-Петербург', 'Ленинградская область'),
	('Санкт-Петербург и Ленинградская обл.', 'Ленинградская область'),
	('Республика Крым и г. Севастополь', 'Республика крым'),
	('Севастопль', 'Республика крым'),
	('Донецкая область', 'Ростовская область'),
	('Республика Донецкая Народная', 'Ростовская область'),
	('Еврейская автономная обл.', 'Еврейская АО'),
	('Республика Саха /Якутия/', 'Республика Саха'),
	('Республика Северная Осетия - Алания', 'Северная Осетия'),
	('Республика Северная Осетия — Алания', 'Северная Осетия'),
	('Ханты-Мансийский автономный округ - Югра', 'Ханты-Мансийский автономный округ'),
	('Ханты-Мансийский автономный округ — Югра', 'Ханты-Мансийский автономный округ'),
	('Ханты-Мансийский автономный округ-Югра', 'Ханты-Мансийский автономный округ'),
	('Ханты-Мансийский Автономный округ - Югра АО', 'Ханты-Мансийский автономный округ'),
	('Ханты-Мансийский АО - Югра', 'Ханты-Мансийский автономный округ'),
	('Ханты - Мансийский - Югра АО', 'Ханты-Мансийский автономный округ'),
	('Республика Удмуртия', 'Удмуртия'),
	('Республика Удмуртская', 'Удмуртия'),
	('Удмуртская Республика', 'Удмуртия'),
	('Республика Тыва', 'Тыва республика'),
	('Карачаево-Черкесская Республика', 'Карачаево-Черкесия'),
	('Республика Карачаево-Черкесская', 'Карачаево-Черкесия'),
	('Чеченская Республика', 'Чеченская республика'),
	('Республика Чеченская', 'Чеченская республика'),
	('Республика Чечня', 'Чеченская республика')
) AS v(alias, canon)
JOIN region r ON r.name = v.canon
ON CONFLICT (alias) DO UPDATE SET region_id = EXCLUDED.region_id;

INSERT INTO region_alias (alias, region_id)
SELECT r.name, r.id FROM region r
ON CONFLICT (alias) DO NOTHING;
