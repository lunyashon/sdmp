CREATE INDEX IF NOT EXISTS lead_row_via_us_false_idx
	ON lead_row (id DESC)
	WHERE NOT via_us;
