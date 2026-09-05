ALTER TABLE dispatch
	ADD COLUMN IF NOT EXISTS http_status SMALLINT,
	ADD COLUMN IF NOT EXISTS raw_response TEXT;

COMMENT ON COLUMN dispatch.http_status IS 'HTTP код ответа партнёра / СК. NULL если сеть упала до ответа';
COMMENT ON COLUMN dispatch.raw_response IS 'Сырое тело ответа. Nullable';

CREATE UNIQUE INDEX IF NOT EXISTS dispatch_lead_channel_upk_uidx
	ON dispatch (lead_row_id, channel_id, upk_id)
	WHERE upk_id IS NOT NULL;
