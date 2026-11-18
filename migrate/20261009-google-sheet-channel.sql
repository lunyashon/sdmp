INSERT INTO channel (id, name) VALUES (4, 'Google Sheets')
ON CONFLICT (id) DO NOTHING;

UPDATE google_sheet
SET zarya_results = ARRAY['dup', 'unique']
WHERE COALESCE(cardinality(zarya_results), 0) = 0;
