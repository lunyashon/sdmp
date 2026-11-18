ALTER TABLE outbox_bx24
	ADD COLUMN IF NOT EXISTS count_retry SMALLINT NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS last_retry TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS outbox_bx24_claim_idx ON outbox_bx24 (uuid)
	WHERE count_retry < 6;

COMMENT ON COLUMN outbox_bx24.count_retry IS 'сколько раз уже пробовали отправить';
COMMENT ON COLUMN outbox_bx24.last_retry IS 'когда последний раз пробовали отправить';
