CREATE TABLE IF NOT EXISTS lead_skorozvon (
	skorozvon_lead_id TEXT PRIMARY KEY,
	lead_row_id BIGINT NOT NULL REFERENCES lead_row(id),
	project_id TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS lead_skorozvon_lead_row_idx ON lead_skorozvon (lead_row_id);
CREATE INDEX IF NOT EXISTS lead_skorozvon_lead_project_idx ON lead_skorozvon (lead_row_id, project_id);

COMMENT ON TABLE lead_skorozvon IS 'Все id лидов Скорозвона на одну lead_row. Вебхук ищет по skorozvon_lead_id сюда, не только в lead_row';
COMMENT ON COLUMN lead_skorozvon.skorozvon_lead_id IS 'lead.id из Скорозвона';
COMMENT ON COLUMN lead_skorozvon.lead_row_id IS 'наша заявка';
COMMENT ON COLUMN lead_skorozvon.project_id IS 'call_project_id, если известен';

INSERT INTO lead_skorozvon (skorozvon_lead_id, lead_row_id, project_id, created_at)
SELECT btrim(lr.skorozvon_lead_id), lr.id, NULLIF(btrim(lr.project_num), ''), COALESCE(lr.updated_at, lr.created_at)
FROM lead_row lr
WHERE lr.skorozvon_lead_id IS NOT NULL
  AND btrim(lr.skorozvon_lead_id) <> ''
ON CONFLICT (skorozvon_lead_id) DO NOTHING;

INSERT INTO lead_skorozvon (skorozvon_lead_id, lead_row_id, project_id, created_at)
SELECT btrim(d.external_id), d.lead_row_id, NULLIF(btrim(d.skorozvon_project_id), ''), d.sent_at
FROM dispatch d
WHERE d.channel_id = 1
  AND d.sent_ok
  AND d.external_id IS NOT NULL
  AND btrim(d.external_id) <> ''
ON CONFLICT (skorozvon_lead_id) DO NOTHING;
