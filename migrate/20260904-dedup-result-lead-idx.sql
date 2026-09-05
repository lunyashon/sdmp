-- Latest Zarya (and other providers) per lead: List/Get LATERAL looks up
-- dedup_result by lead_row_id + provider_id + checked_at DESC.
CREATE INDEX IF NOT EXISTS dedup_result_lead_provider_checked_idx
	ON dedup_result (lead_row_id, provider_id, checked_at DESC);
