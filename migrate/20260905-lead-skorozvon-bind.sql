ALTER TABLE lead_row
	ADD COLUMN IF NOT EXISTS skorozvon_lead_id VARCHAR;

COMMENT ON COLUMN lead_row.skorozvon_lead_id IS 'id лида в Скорозвоне после успешной отправки. Ищем call_result по нему. Nullable';
COMMENT ON COLUMN lead_row.project_num IS 'id проекта Скорозвона после отправки; при приёме может быть номер проекта поставщика. Nullable';

UPDATE lead_row lr
SET skorozvon_lead_id = d.external_id,
    project_num = COALESCE(NULLIF(lr.project_num, ''), d.skorozvon_project_id)
FROM (
	SELECT DISTINCT ON (lead_row_id)
		lead_row_id,
		external_id,
		skorozvon_project_id
	FROM dispatch
	WHERE channel_id = 1
	  AND sent_ok
	  AND external_id IS NOT NULL
	  AND external_id <> ''
	ORDER BY lead_row_id, sent_at DESC
) d
WHERE lr.id = d.lead_row_id
  AND lr.skorozvon_lead_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS lead_row_skorozvon_lead_id_uidx
	ON lead_row (skorozvon_lead_id)
	WHERE skorozvon_lead_id IS NOT NULL;
