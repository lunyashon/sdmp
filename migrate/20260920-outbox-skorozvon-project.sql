ALTER TABLE outbox_skorozvon
	ADD COLUMN IF NOT EXISTS project_id BIGINT;

COMMENT ON COLUMN outbox_skorozvon.project_id IS 'call_project_id для импорта; NULL = обычный роутинг по связкам';

DROP TABLE IF EXISTS outbox_import;
