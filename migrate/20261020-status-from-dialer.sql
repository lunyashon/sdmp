UPDATE status SET name = 'из звонилки' WHERE id = 4;
UPDATE event_type SET name = 'из звонилки' WHERE id = 4;

COMMENT ON TABLE status IS '1=принята 2=обогащена 3=в СК 4=из звонилки 5=в Б24 6=отклонена 7=ожидание';
COMMENT ON TABLE event_type IS '1=принята 2=обогащена 3=в СК 4=из звонилки 5=партнёру 6=в Б24 7=отклонена 8=архивирована';
COMMENT ON COLUMN lead_row.status_id IS '1=принята 2=обогащена 3=в СК 4=из звонилки 5=в Б24 6=отклонена 7=ожидание';
