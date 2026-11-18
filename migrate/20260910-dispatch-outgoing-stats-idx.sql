CREATE INDEX IF NOT EXISTS dispatch_upk_sent_idx
	ON dispatch (upk_id, sent_at)
	WHERE upk_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS dispatch_skorozvon_sent_idx
	ON dispatch (skorozvon_project_id, sent_at)
	WHERE skorozvon_project_id IS NOT NULL;
