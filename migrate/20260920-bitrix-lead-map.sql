CREATE TABLE IF NOT EXISTS bitrix_lead_field (
	code VARCHAR PRIMARY KEY,
	title VARCHAR NOT NULL DEFAULT '',
	type VARCHAR NOT NULL DEFAULT '',
	writable BOOLEAN NOT NULL DEFAULT true,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE bitrix_lead_field IS 'Кэш crm.lead.fields с вебхука SODATECH_BX24_URL';

CREATE TABLE IF NOT EXISTS bitrix_status (
	entity_id VARCHAR NOT NULL,
	status_id VARCHAR NOT NULL,
	name VARCHAR NOT NULL DEFAULT '',
	sort INT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (entity_id, status_id)
);

COMMENT ON TABLE bitrix_status IS 'Кэш crm.status.list (STATUS) и crm.source.list (SOURCE)';

CREATE TABLE IF NOT EXISTS bitrix_lead_map (
	source_key VARCHAR PRIMARY KEY,
	bitrix_code VARCHAR NOT NULL DEFAULT '',
	fixed_value VARCHAR NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE bitrix_lead_map IS 'Связка полей CDP → Битрикс для crm.lead.add. source_key: phone, city, uuid…';
COMMENT ON COLUMN bitrix_lead_map.fixed_value IS 'Константа, если поле не из лида. Для status_id — STATUS_ID стадии Б24.';

INSERT INTO bitrix_lead_map (source_key, bitrix_code, fixed_value) VALUES
	('phone', 'PHONE', ''),
	('city', 'UF_CRM_1638446549', ''),
	('uuid', 'UF_CRM_1788705605', ''),
	('comments', 'COMMENTS', ''),
	('status_id', 'STATUS_ID', '14'),
	('base_label', 'UF_CRM_1698755876042', ''),
	('cold_label', 'UF_CRM_1697524451888', ''),
	('zarya', 'UF_CRM_1730362141931', '')
ON CONFLICT (source_key) DO NOTHING;
