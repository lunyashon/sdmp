-- Метки и иерархия из «Источники Битрикс.xlsx».
-- Холодная база / Какая база → cold_label / base_label по bitrix_code.
-- Маппинг: колонка «Источник» = родитель, «Подисточник» = child.parent_id.

UPDATE source
SET cold_label = 'МФО5_дата',
    base_label = 'МФО5_дата'
WHERE bitrix_code = 'UC_F8SXH3'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'МФО5_дата'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'МФО5_дата'
  );
UPDATE source
SET cold_label = 'МФО1_дата',
    base_label = 'МФО1_дата'
WHERE bitrix_code = 'UC_69H7XP'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'МФО1_дата'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'МФО1_дата'
  );
UPDATE source
SET cold_label = 'BZDM',
    base_label = 'BZDM'
WHERE bitrix_code = 'UC_2WMMZY'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'BZDM'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'BZDM'
  );
UPDATE source
SET cold_label = 'BZDM_2',
    base_label = 'BZDM_2'
WHERE bitrix_code = 'UC_DYWJ1H'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'BZDM_2'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'BZDM_2'
  );
UPDATE source
SET cold_label = 'RT ROWS',
    base_label = 'RT ROWS'
WHERE bitrix_code = 'UC_IVOMHK'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'RT ROWS'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'RT ROWS'
  );
UPDATE source
SET cold_label = 'KRM ROWS',
    base_label = 'KRM ROWS'
WHERE bitrix_code = 'UC_ZWIQ5R'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'KRM ROWS'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'KRM ROWS'
  );
UPDATE source
SET cold_label = 'ВР',
    base_label = 'ВР'
WHERE bitrix_code = 'UC_P8EDM3'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР'
  );
UPDATE source
SET cold_label = 'ВР Пиксель',
    base_label = 'ВР Пиксель'
WHERE bitrix_code = 'UC_X735N0'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР Пиксель'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР Пиксель'
  );
UPDATE source
SET cold_label = 'ВР2',
    base_label = 'ВР2'
WHERE bitrix_code = 'UC_TQ0AO3'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР2'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР2'
  );
UPDATE source
SET cold_label = 'ВР ХОСТ',
    base_label = 'ВР ХОСТ'
WHERE bitrix_code = 'UC_8TALV7'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР ХОСТ'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР ХОСТ'
  );
UPDATE source
SET cold_label = 'НХ',
    base_label = 'НХ'
WHERE bitrix_code = 'UC_6VTLIR'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'НХ'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'НХ'
  );
UPDATE source
SET cold_label = 'AZ',
    base_label = 'AZ'
WHERE bitrix_code = 'UC_YN1PP3'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'AZ'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'AZ'
  );
UPDATE source
SET cold_label = 'Лидстех',
    base_label = 'Лидстех'
WHERE bitrix_code = '141'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'Лидстех'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'Лидстех'
  );
UPDATE source
SET cold_label = 'VG',
    base_label = 'VG'
WHERE bitrix_code = '143'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'VG'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'VG'
  );
UPDATE source
SET cold_label = 'CLG TEST',
    base_label = 'CLG TEST'
WHERE bitrix_code = '144'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'CLG TEST'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'CLG TEST'
  );
UPDATE source
SET cold_label = 'CLG TEST',
    base_label = 'CLG TEST'
WHERE bitrix_code = 'UC_2373KW'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'CLG TEST'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'CLG TEST'
  );
UPDATE source
SET cold_label = 'BL SODA',
    base_label = 'VG Р/У'
WHERE bitrix_code = 'UC_ZE0NES'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'BL SODA'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'VG Р/У'
  );
UPDATE source
SET cold_label = 'HX',
    base_label = 'HX'
WHERE bitrix_code = 'UC_NF10L4'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'HX'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'HX'
  );
UPDATE source
SET cold_label = 'VG',
    base_label = 'VG'
WHERE bitrix_code = '169'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'VG'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'VG'
  );
UPDATE source
SET cold_label = 'ВР Билайн',
    base_label = 'ВР Билайн'
WHERE bitrix_code = 'UC_MD8A2D'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР Билайн'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР Билайн'
  );
UPDATE source
SET cold_label = 'ВР МТС',
    base_label = 'ВР МТС'
WHERE bitrix_code = 'UC_HQ2ZO9'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР МТС'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР МТС'
  );
UPDATE source
SET cold_label = 'ВР Другие',
    base_label = 'ВР Другие'
WHERE bitrix_code = 'UC_5Z44LG'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР Другие'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР Другие'
  );
UPDATE source
SET cold_label = 'Лидстех МТС',
    base_label = 'Лидстех МТС'
WHERE bitrix_code = 'UC_VKLEXV'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'Лидстех МТС'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'Лидстех МТС'
  );
UPDATE source
SET cold_label = 'Лидстех Билайн',
    base_label = 'Лидстех Билайн'
WHERE bitrix_code = 'UC_CJQR7Z'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'Лидстех Билайн'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'Лидстех Билайн'
  );
UPDATE source
SET cold_label = 'Лидстех Другие',
    base_label = 'Лидстех Другие'
WHERE bitrix_code = 'UC_LQNNKH'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'Лидстех Другие'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'Лидстех Другие'
  );
UPDATE source
SET cold_label = 'ВР Билайн тест',
    base_label = 'ВР Билайн тест'
WHERE bitrix_code = 'UC_S92S42'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР Билайн тест'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР Билайн тест'
  );
UPDATE source
SET cold_label = 'ВР МТС тест',
    base_label = 'ВР МТС тест'
WHERE bitrix_code = 'UC_BSR2NP'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР МТС тест'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР МТС тест'
  );
UPDATE source
SET cold_label = 'ВР Другие тест',
    base_label = 'ВР Другие тест'
WHERE bitrix_code = 'UC_ZTILOY'
  AND (
    COALESCE(cold_label, '') IS DISTINCT FROM 'ВР Другие тест'
    OR COALESCE(base_label, '') IS DISTINCT FROM 'ВР Другие тест'
  );

UPDATE source AS child
SET parent_id = parent.id
FROM source AS parent
WHERE child.bitrix_code = 'UC_MD8A2D'
  AND parent.bitrix_code = 'UC_P8EDM3'
  AND child.id <> parent.id
  AND child.parent_id IS DISTINCT FROM parent.id;
UPDATE source AS child
SET parent_id = parent.id
FROM source AS parent
WHERE child.bitrix_code = 'UC_HQ2ZO9'
  AND parent.bitrix_code = 'UC_P8EDM3'
  AND child.id <> parent.id
  AND child.parent_id IS DISTINCT FROM parent.id;
UPDATE source AS child
SET parent_id = parent.id
FROM source AS parent
WHERE child.bitrix_code = 'UC_5Z44LG'
  AND parent.bitrix_code = 'UC_P8EDM3'
  AND child.id <> parent.id
  AND child.parent_id IS DISTINCT FROM parent.id;
UPDATE source AS child
SET parent_id = parent.id
FROM source AS parent
WHERE child.bitrix_code = 'UC_S92S42'
  AND parent.bitrix_code = 'UC_P8EDM3'
  AND child.id <> parent.id
  AND child.parent_id IS DISTINCT FROM parent.id;
UPDATE source AS child
SET parent_id = parent.id
FROM source AS parent
WHERE child.bitrix_code = 'UC_BSR2NP'
  AND parent.bitrix_code = 'UC_P8EDM3'
  AND child.id <> parent.id
  AND child.parent_id IS DISTINCT FROM parent.id;
UPDATE source AS child
SET parent_id = parent.id
FROM source AS parent
WHERE child.bitrix_code = 'UC_ZTILOY'
  AND parent.bitrix_code = 'UC_P8EDM3'
  AND child.id <> parent.id
  AND child.parent_id IS DISTINCT FROM parent.id;
UPDATE source AS child
SET parent_id = parent.id
FROM source AS parent
WHERE child.bitrix_code = 'UC_VKLEXV'
  AND parent.bitrix_code = '141'
  AND child.id <> parent.id
  AND child.parent_id IS DISTINCT FROM parent.id;
UPDATE source AS child
SET parent_id = parent.id
FROM source AS parent
WHERE child.bitrix_code = 'UC_CJQR7Z'
  AND parent.bitrix_code = '141'
  AND child.id <> parent.id
  AND child.parent_id IS DISTINCT FROM parent.id;
UPDATE source AS child
SET parent_id = parent.id
FROM source AS parent
WHERE child.bitrix_code = 'UC_LQNNKH'
  AND parent.bitrix_code = '141'
  AND child.id <> parent.id
  AND child.parent_id IS DISTINCT FROM parent.id;

