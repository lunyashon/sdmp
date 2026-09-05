-- Lease for worker: claim commits quickly, HTTP runs without row lock.
ALTER TABLE outbox
	ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP,
	ADD COLUMN IF NOT EXISTS locked_by UUID;

COMMENT ON COLUMN outbox.locked_until IS 'пока > now() строка занята воркером; NULL или прошлое = можно брать';
COMMENT ON COLUMN outbox.locked_by IS 'токен текущего claim; DELETE/UNLOCK только с этим токеном';

CREATE INDEX IF NOT EXISTS outbox_unlocked_idx ON outbox (id) WHERE locked_until IS NULL;
CREATE INDEX IF NOT EXISTS outbox_locked_until_idx ON outbox (locked_until) WHERE locked_until IS NOT NULL;
