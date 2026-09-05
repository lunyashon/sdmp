CREATE TABLE IF NOT EXISTS skorozvon_project (
	uuid VARCHAR PRIMARY KEY,
	project_id BIGINT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	last_date_request TIMESTAMP,
	last_request BOOLEAN
);

COMMENT ON TABLE skorozvon_project IS 'Реестр проектов Скорозвона. project_id = call_project_id';
COMMENT ON COLUMN skorozvon_project.uuid IS 'внутренний ключ';
COMMENT ON COLUMN skorozvon_project.project_id IS 'id проекта в Скорозвоне (call_project_id)';
COMMENT ON COLUMN skorozvon_project.last_date_request IS 'когда последний раз проверяли, что проект ещё есть';
COMMENT ON COLUMN skorozvon_project.last_request IS 'true = Скорозвон ответил 200';
