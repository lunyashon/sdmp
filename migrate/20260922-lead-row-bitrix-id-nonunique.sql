-- Исторический импорт СМС-трафика дублирует bitrix_lead_id, если строка уже была.
DROP INDEX IF EXISTS lead_row_bitrix_lead_id_uidx;

CREATE INDEX IF NOT EXISTS lead_row_bitrix_lead_id_idx
  ON lead_row (bitrix_lead_id)
  WHERE bitrix_lead_id IS NOT NULL;
