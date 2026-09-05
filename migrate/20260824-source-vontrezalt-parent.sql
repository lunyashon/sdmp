UPDATE source
SET parent_id = (SELECT id FROM source WHERE bitrix_code = 'UC_P8EDM3')
WHERE bitrix_code IN (
	'UC_MD8A2D',
	'UC_HQ2ZO9',
	'UC_5Z44LG',
	'UC_S92S42',
	'UC_BSR2NP',
	'UC_ZTILOY'
);
