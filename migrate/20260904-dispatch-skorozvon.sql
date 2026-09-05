CREATE UNIQUE INDEX IF NOT EXISTS dispatch_lead_channel_skorozvon_uidx
	ON dispatch (lead_row_id, channel_id, skorozvon_project_id)
	WHERE skorozvon_project_id IS NOT NULL;
