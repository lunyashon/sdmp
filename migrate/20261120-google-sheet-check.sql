ALTER TABLE google_sheet
	ADD COLUMN IF NOT EXISTS last_checked_at timestamptz,
	ADD COLUMN IF NOT EXISTS last_ok boolean,
	ADD COLUMN IF NOT EXISTS last_http_status int,
	ADD COLUMN IF NOT EXISTS last_response text NOT NULL DEFAULT '';
