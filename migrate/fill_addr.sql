SET synchronous_commit = off;
SET enable_hashjoin = off;
SET enable_mergejoin = off;

CREATE UNLOGGED TABLE tmp_addr (
  bitrix_lead_id TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

\copy tmp_addr (bitrix_lead_id, value) FROM '/tmp/b_crm_addr.csv' WITH (FORMAT csv)
ANALYZE tmp_addr;

CREATE UNLOGGED TABLE tmp_addr_hit AS
SELECT lr.id, t.value
FROM tmp_addr t
JOIN lead_row lr ON lr.bitrix_lead_id = t.bitrix_lead_id;

ANALYZE tmp_addr_hit;

SELECT
  (SELECT count(*) FROM tmp_addr) AS mysql_nonempty,
  (SELECT count(*) FROM tmp_addr_hit) AS matched;

UPDATE lead_row lr
SET legacy_fields = (
  COALESCE(lr.legacy_fields::jsonb, '{}'::jsonb)
  || jsonb_build_object('address', h.value)
)::text
FROM tmp_addr_hit h
WHERE lr.id = h.id;

SELECT count(*) AS with_address FROM tmp_addr_hit;

DROP TABLE tmp_addr;
DROP TABLE tmp_addr_hit;
