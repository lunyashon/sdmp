CREATE TABLE IF NOT EXISTS outbox_bx24 (
	uuid VARCHAR PRIMARY KEY,
	uuid_lead VARCHAR NOT NULL REFERENCES lead_row (uuid),
	locked_until TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS outbox_bx24_uuid_lead_uidx ON outbox_bx24 (uuid_lead);

COMMENT ON TABLE outbox_bx24 IS 'очередь отправки в Б24. Пишем в той же TX, что status_id=4. Claim позже, как outbox';
COMMENT ON COLUMN outbox_bx24.uuid IS 'id задачи в очереди';
COMMENT ON COLUMN outbox_bx24.uuid_lead IS 'lead_row.uuid';
COMMENT ON COLUMN outbox_bx24.locked_until IS 'пока > now() строка занята воркером; NULL или прошлое = можно брать';
