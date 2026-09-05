CREATE UNIQUE INDEX IF NOT EXISTS source_token_uidx
	ON source (token)
	WHERE token IS NOT NULL AND token <> '';
