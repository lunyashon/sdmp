CREATE TABLE IF NOT EXISTS crmcalls_settings (
	id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
	api_key VARCHAR NOT NULL DEFAULT '',
	base_url VARCHAR NOT NULL DEFAULT 'https://cc.crmcalls.ru',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE crmcalls_settings IS 'Подключение CRMCalls с вкладки «Исходящий трафик». Ключ для GET /api/v1/projects и POST /api/v1/contacts';

INSERT INTO crmcalls_settings (id, api_key, base_url)
VALUES (1, '', 'https://cc.crmcalls.ru')
ON CONFLICT (id) DO NOTHING;
