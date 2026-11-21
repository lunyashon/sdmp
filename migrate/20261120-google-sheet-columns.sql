ALTER TABLE google_sheet
	ADD COLUMN IF NOT EXISTS columns jsonb NOT NULL DEFAULT '[]'::jsonb;
