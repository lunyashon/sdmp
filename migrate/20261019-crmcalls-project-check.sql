ALTER TABLE crmcalls_project
	ADD COLUMN IF NOT EXISTS last_date_request TIMESTAMPTZ,
	ADD COLUMN IF NOT EXISTS last_request BOOLEAN;

COMMENT ON COLUMN crmcalls_project.last_date_request IS 'когда последний раз проверяли, что проект есть и активен';
COMMENT ON COLUMN crmcalls_project.last_request IS 'true = проект найден в CRMCalls и status=active';
