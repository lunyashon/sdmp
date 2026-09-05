#!/usr/bin/env python3
"""Copy non-empty UF_CRM_1766132352291 into lead_row.legacy_fields.lead_type_dup_250_300."""

from __future__ import annotations

import csv
import os
import subprocess
import sys

MYSQL = ["mysql", "-N", "--batch", "--default-character-set=utf8mb4", "bitrix_dump"]
CSV_PATH = "/tmp/lead_type_dup.csv"


def mysql(sql: str) -> str:
    proc = subprocess.run(MYSQL, input=sql, text=True, capture_output=True)
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)
    return proc.stdout


def main() -> None:
    print("export mysql…", flush=True)
    raw = mysql("""
SELECT u.VALUE_ID, e.VALUE
FROM b_uts_crm_lead u
JOIN b_user_field_enum e ON e.ID = u.UF_CRM_1766132352291
WHERE u.UF_CRM_1766132352291 IS NOT NULL
  AND e.USER_FIELD_ID = 1201
  AND e.VALUE IS NOT NULL
  AND e.VALUE <> '';
""")
    rows = [line.split("\t", 1) for line in raw.splitlines() if line.strip()]
    print(f"mysql nonempty={len(rows)}", flush=True)
    if not rows:
        print("nothing to copy")
        return

    with open(CSV_PATH, "w", encoding="utf-8", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        for lead_id, value in rows:
            w.writerow([lead_id, value])

    env = os.environ.copy()
    env.setdefault("PGPASSWORD", "admin_tmp_password")
    sql = f"""
CREATE TEMP TABLE tmp_lead_type_dup (
  bitrix_lead_id TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
\\copy tmp_lead_type_dup (bitrix_lead_id, value) FROM '{CSV_PATH}' WITH (FORMAT csv)
UPDATE lead_row lr
SET legacy_fields = (
  COALESCE(lr.legacy_fields::jsonb, '{{}}'::jsonb)
  || jsonb_build_object('lead_type_dup_250_300', t.value)
)::text
FROM tmp_lead_type_dup t
WHERE lr.bitrix_lead_id = t.bitrix_lead_id
  AND COALESCE(lr.legacy_fields::jsonb->>'lead_type_dup_250_300', '') = '';
SELECT
  (SELECT count(*) FROM tmp_lead_type_dup) AS mysql_nonempty,
  (SELECT count(*) FROM lead_row lr JOIN tmp_lead_type_dup t ON t.bitrix_lead_id = lr.bitrix_lead_id) AS matched,
  (SELECT count(*) FROM lead_row WHERE legacy_fields::jsonb ? 'lead_type_dup_250_300') AS with_key;
SELECT lr.legacy_fields::jsonb->>'lead_type_dup_250_300' AS value, count(*)
FROM lead_row lr
WHERE lr.legacy_fields::jsonb ? 'lead_type_dup_250_300'
GROUP BY 1
ORDER BY 2 DESC;
"""
    print("update lead_row…", flush=True)
    proc = subprocess.run(
        [
            "psql",
            "-h", env.get("PGHOST", "127.0.0.1"),
            "-U", env.get("PGUSER", "admin"),
            "-d", env.get("PGDATABASE", "sdmp"),
            "-v", "ON_ERROR_STOP=1",
        ],
        input=sql,
        text=True,
        capture_output=True,
        env=env,
    )
    sys.stdout.write(proc.stdout)
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)


if __name__ == "__main__":
    main()
