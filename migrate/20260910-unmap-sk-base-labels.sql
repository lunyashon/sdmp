-- Какая база / Холодная база пишутся из lead_row (дробление), не из Скорозвона.
UPDATE skorozvon_maps
SET bitrix_code = NULL
WHERE bitrix_code IN ('UF_CRM_1698755876042', 'UF_CRM_1697524451888');
