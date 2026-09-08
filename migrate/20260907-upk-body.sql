ALTER TABLE upk
	ADD COLUMN IF NOT EXISTS body JSONB NOT NULL DEFAULT '{}'::jsonb,
	ADD COLUMN IF NOT EXISTS content_type VARCHAR NOT NULL DEFAULT 'application/x-www-form-urlencoded';

COMMENT ON COLUMN upk.body IS 'Параметры запроса: уходят и в query (GET), и в тело POST. Плейсхолдеры {{phone}}, {{phone_valid}}, {{region}}, {{city}}';
COMMENT ON COLUMN upk.content_type IS 'Content-Type POST-тела';

UPDATE upk SET
	body = '{"token":"rwi1p2vmymu8qqnoy7ojyydfpckw","name":"Без имени","phone":"{{phone}}"}'::jsonb,
	content_type = 'application/x-www-form-urlencoded'
WHERE name = 'РТ';

UPDATE upk SET
	body = '{"auth":"WEEkB6l8TPHsdWw?msfR?SRcAXmLLHpyxPbbKWvCVBPlmLuc!Xs8!DksvY?DvTWi","phone":"{{phone}}","tag":"{{region}}"}'::jsonb,
	content_type = 'application/x-www-form-urlencoded'
WHERE name = 'КР';

UPDATE upk SET
	body = '{"auth":"d8f89e47226b8f3b9a11c82736b4e672","phone":"{{phone}}","region":"{{region}}"}'::jsonb,
	content_type = 'application/x-www-form-urlencoded'
WHERE name = 'ББ';
