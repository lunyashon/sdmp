ALTER TABLE skorozvon_maps
	ADD COLUMN IF NOT EXISTS source_key VARCHAR;

COMMENT ON COLUMN skorozvon_maps.source_key IS 'Ключ CDP для исходящей отправки в СК (uuid, cold_label, zarya, lead_id, source_description). Пусто = поле в СК не заполняем.';

UPDATE skorozvon_maps
SET source_key = 'uuid'
WHERE option_id = 0
  AND lead_type = 'lead'
  AND lower(name) = 'uuid'
  AND COALESCE(source_key, '') = '';

UPDATE skorozvon_maps
SET source_key = 'cold_label'
WHERE option_id = 0
  AND lead_type = 'lead'
  AND lower(name) = 'база'
  AND COALESCE(source_key, '') = '';

UPDATE skorozvon_maps
SET source_key = 'zarya'
WHERE option_id = 0
  AND lead_type = 'lead'
  AND lower(name) = 'дубликат'
  AND COALESCE(source_key, '') = '';

UPDATE skorozvon_maps
SET source_key = 'lead_id'
WHERE option_id = 0
  AND lead_type = 'lead'
  AND lower(name) = 'id лида б24'
  AND COALESCE(source_key, '') = '';

UPDATE skorozvon_maps
SET source_key = 'source_description'
WHERE option_id = 0
  AND lead_type = 'lead'
  AND lower(name) = 'номер проекта'
  AND COALESCE(source_key, '') = '';
