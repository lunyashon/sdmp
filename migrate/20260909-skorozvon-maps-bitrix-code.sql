ALTER TABLE skorozvon_maps
	ADD COLUMN IF NOT EXISTS bitrix_code VARCHAR;

COMMENT ON COLUMN skorozvon_maps.bitrix_code IS 'Код поля Битрикса для этого названия из Скорозвона. Пусто — поле в Б24 не мапится.';

UPDATE skorozvon_maps
SET bitrix_code = CASE lower(trim(name))
	WHEN 'название' THEN 'TITLE'
	WHEN 'регион клиента' THEN 'UF_CRM_1747216350206'
	WHEN 'телефон' THEN 'PHONE[0][VALUE]'
	WHEN 'телефоны' THEN 'PHONE[0][VALUE]'
	WHEN 'номер проекта' THEN 'UF_CRM_1699340770806'
	WHEN 'база' THEN 'UF_CRM_1698755876042'
	WHEN 'сумма долга' THEN 'UF_CRM_1730630573577'
	WHEN 'оператор' THEN 'UF_CRM_1676626557'
	WHEN 'город клиента' THEN 'UF_CRM_1638446549'
	WHEN 'вид долга' THEN 'UF_CRM_1730630661182'
	WHEN 'другая информация' THEN 'UF_CRM_1730631041590'
	WHEN 'имущество' THEN 'UF_CRM_1730630726419'
	WHEN 'дубликат' THEN 'UF_CRM_1730362141931'
	WHEN 'одм' THEN 'UF_CRM_1737099727557'
	WHEN 'доход' THEN 'UF_CRM_1742819602566'
	WHEN 'теги' THEN 'UF_CRM_1729062300505'
	WHEN 'gdmp_z' THEN 'UF_CRM_1747907679528'
	WHEN 'приставы' THEN 'UF_CRM_1758104136848'
END
WHERE option_id = 0
  AND CASE lower(trim(name))
	WHEN 'название' THEN true
	WHEN 'регион клиента' THEN true
	WHEN 'телефон' THEN true
	WHEN 'телефоны' THEN true
	WHEN 'номер проекта' THEN true
	WHEN 'база' THEN true
	WHEN 'сумма долга' THEN true
	WHEN 'оператор' THEN true
	WHEN 'город клиента' THEN true
	WHEN 'вид долга' THEN true
	WHEN 'другая информация' THEN true
	WHEN 'имущество' THEN true
	WHEN 'дубликат' THEN true
	WHEN 'одм' THEN true
	WHEN 'доход' THEN true
	WHEN 'теги' THEN true
	WHEN 'gdmp_z' THEN true
	WHEN 'приставы' THEN true
	ELSE false
END;
