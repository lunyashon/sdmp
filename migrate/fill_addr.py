#!/usr/bin/env python3
"""Copy nonempty b_crm_addr into lead_row.legacy_fields.address."""

from __future__ import annotations

import csv
import os
import subprocess
import sys

MYSQL = ["mysql", "-N", "--batch", "--raw", "--default-character-set=utf8mb4", "bitrix_dump"]
CSV_PATH = "/tmp/b_crm_addr.csv"


def main() -> None:
    print("export mysql…", flush=True)
    proc = subprocess.Popen(
        MYSQL + ["-e", """
SELECT ENTITY_ID,
       REPLACE(REPLACE(addr, '\\n', ' '), '\\t', ' ')
FROM (
  SELECT ENTITY_ID,
         CONCAT_WS(', ',
           NULLIF(TRIM(ADDRESS_1), ''),
           NULLIF(TRIM(ADDRESS_2), ''),
           NULLIF(TRIM(CITY), ''),
           NULLIF(TRIM(REGION), ''),
           NULLIF(TRIM(PROVINCE), ''),
           NULLIF(TRIM(POSTAL_CODE), ''),
           NULLIF(TRIM(COUNTRY), '')
         ) AS addr
  FROM b_crm_addr
  WHERE ENTITY_TYPE_ID = 1
) t
WHERE addr IS NOT NULL AND addr <> '';
"""],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    n = 0
    with open(CSV_PATH, "w", encoding="utf-8", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        assert proc.stdout is not None
        for line in proc.stdout:
            parts = line.rstrip("\n").split("\t", 1)
            if len(parts) != 2:
                continue
            lead_id, value = parts
            if not value:
                continue
            w.writerow([lead_id, value])
            n += 1
            if n % 1_000_000 == 0:
                print(f"  exported {n}", flush=True)
    err = proc.stderr.read() if proc.stderr else ""
    if proc.wait() != 0:
        sys.stderr.write(err)
        raise SystemExit(proc.returncode)
    print(f"mysql nonempty={n}", flush=True)
    if n == 0:
        print("nothing to copy")
        return

    env = os.environ.copy()
    env.setdefault("PGPASSWORD", "admin_tmp_password")
    sql = f"""
CREATE TEMP TABLE tmp_addr (
  bitrix_lead_id TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
\\copy tmp_addr (bitrix_lead_id, value) FROM '{CSV_PATH}' WITH (FORMAT csv)

SELECT
  (SELECT count(*) FROM tmp_addr) AS mysql_nonempty,
  (SELECT count(*) FROM lead_row lr JOIN tmp_addr t ON t.bitrix_lead_id = lr.bitrix_lead_id) AS matched;

UPDATE lead_row lr
SET legacy_fields = (
  COALESCE(lr.legacy_fields::jsonb, '{{}}'::jsonb)
  || jsonb_build_object('address', t.value)
)::text
FROM tmp_addr t
WHERE lr.bitrix_lead_id = t.bitrix_lead_id
  AND COALESCE(lr.legacy_fields::jsonb->>'address', '') = '';

SELECT count(*) AS with_address
FROM lead_row
WHERE legacy_fields::jsonb ? 'address';
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
