#!/usr/bin/env python3
"""Copy nonempty b_crm_utm into lead_row.legacy_fields utm_* keys."""

from __future__ import annotations

import csv
import os
import subprocess
import sys

MYSQL = ["mysql", "-N", "--batch", "--raw", "--default-character-set=utf8mb4", "bitrix_dump"]
CSV_PATH = "/tmp/b_crm_utm.csv"
KEY_MAP = {
    "UTM_CAMPAIGN": "utm_campaign",
    "UTM_CONTENT": "utm_content",
    "UTM_MEDIUM": "utm_medium",
    "UTM_SOURCE": "utm_source",
    "UTM_TERM": "utm_term",
}


def mysql(sql: str) -> str:
    proc = subprocess.run(MYSQL, input=sql, text=True, capture_output=True)
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)
    return proc.stdout


def main() -> None:
    print("export mysql…", flush=True)
    proc = subprocess.Popen(
        MYSQL + ["-e", """
SELECT ENTITY_ID,
       CODE,
       REPLACE(REPLACE(VALUE, '\\n', ' '), '\\t', ' ')
FROM b_crm_utm
WHERE VALUE IS NOT NULL AND TRIM(VALUE) <> ''
  AND CODE IN ('UTM_CAMPAIGN','UTM_CONTENT','UTM_MEDIUM','UTM_SOURCE','UTM_TERM');
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
            parts = line.rstrip("\n").split("\t")
            if len(parts) != 3:
                continue
            lead_id, code, value = parts
            key = KEY_MAP.get(code)
            if not key:
                continue
            w.writerow([lead_id, key, value])
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
CREATE TEMP TABLE tmp_utm_row (
  bitrix_lead_id TEXT NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL
);
\\copy tmp_utm_row (bitrix_lead_id, key, value) FROM '{CSV_PATH}' WITH (FORMAT csv)
CREATE TEMP TABLE tmp_utm_obj AS
SELECT bitrix_lead_id, jsonb_object_agg(key, value) AS obj
FROM tmp_utm_row
GROUP BY bitrix_lead_id;
CREATE INDEX ON tmp_utm_obj (bitrix_lead_id);

SELECT
  (SELECT count(*) FROM tmp_utm_row) AS utm_rows,
  (SELECT count(*) FROM tmp_utm_obj) AS utm_leads,
  (SELECT count(*) FROM lead_row lr JOIN tmp_utm_obj t ON t.bitrix_lead_id = lr.bitrix_lead_id) AS matched_leads;

UPDATE lead_row lr
SET legacy_fields = (COALESCE(lr.legacy_fields::jsonb, '{{}}'::jsonb) || t.obj)::text
FROM tmp_utm_obj t
WHERE lr.bitrix_lead_id = t.bitrix_lead_id;

SELECT
  count(*) FILTER (WHERE legacy_fields::jsonb ? 'utm_campaign') AS utm_campaign,
  count(*) FILTER (WHERE legacy_fields::jsonb ? 'utm_content') AS utm_content,
  count(*) FILTER (WHERE legacy_fields::jsonb ? 'utm_medium') AS utm_medium,
  count(*) FILTER (WHERE legacy_fields::jsonb ? 'utm_source') AS utm_source,
  count(*) FILTER (WHERE legacy_fields::jsonb ? 'utm_term') AS utm_term
FROM lead_row
WHERE bitrix_lead_id IS NOT NULL;
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
