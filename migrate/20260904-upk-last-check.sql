ALTER TABLE upk ADD COLUMN IF NOT EXISTS last_date_request TIMESTAMP;
ALTER TABLE upk ADD COLUMN IF NOT EXISTS last_request BOOLEAN;
ALTER TABLE upk ADD COLUMN IF NOT EXISTS last_http_status INT;
ALTER TABLE upk ADD COLUMN IF NOT EXISTS last_response TEXT;

COMMENT ON COLUMN upk.last_date_request IS 'время последней проверки webhook';
COMMENT ON COLUMN upk.last_request IS 'последняя проверка: HTTP 2xx';
COMMENT ON COLUMN upk.last_http_status IS 'HTTP-статус последней проверки';
COMMENT ON COLUMN upk.last_response IS 'тело ответа последней проверки';
