CREATE TABLE IF NOT EXISTS outbox_skorozvon (
	uuid VARCHAR PRIMARY KEY,
	uuid_lead VARCHAR NOT NULL REFERENCES lead_row (uuid),
	locked_until TIMESTAMPTZ,
	count_retry SMALLINT NOT NULL DEFAULT 0,
	last_retry TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS outbox_skorozvon_uuid_lead_uidx ON outbox_skorozvon (uuid_lead);
CREATE INDEX IF NOT EXISTS outbox_skorozvon_claim_idx ON outbox_skorozvon (uuid)
	WHERE count_retry < 6;

COMMENT ON TABLE outbox_skorozvon IS 'очередь отправки в Скорозвон. Пишем после обогащения. Claim отдельной горутиной';
COMMENT ON COLUMN outbox_skorozvon.uuid IS 'id задачи в очереди';
COMMENT ON COLUMN outbox_skorozvon.uuid_lead IS 'lead_row.uuid';
COMMENT ON COLUMN outbox_skorozvon.locked_until IS 'пока > now() строка занята воркером; NULL или прошлое = можно брать';
COMMENT ON COLUMN outbox_skorozvon.count_retry IS 'сколько раз уже пробовали отправить';
COMMENT ON COLUMN outbox_skorozvon.last_retry IS 'когда последний раз пробовали отправить';
