WITH source AS (
	SELECT
		id,
		regexp_replace(phone, '[^0-9]', '', 'g') AS digits
	FROM lead_row
),
normalized AS (
	SELECT
		id,
		CASE
			WHEN length(digits) = 10 THEN '7' || digits
			WHEN length(digits) = 11 THEN '7' || substr(digits, 2)
		END AS phone
	FROM source
	WHERE length(digits) IN (10, 11)
)
UPDATE lead_row lr
SET phone = n.phone,
	phone_valid = true
FROM normalized n
WHERE lr.id = n.id
  AND (lr.phone IS DISTINCT FROM n.phone OR NOT lr.phone_valid);

UPDATE lead_row
SET phone_valid = false
WHERE phone_valid
  AND phone !~ '^7[0-9]{10}$';
