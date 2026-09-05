#!/usr/bin/env python3
"""Copy nonempty b_crm_lead.SOURCE_DESCRIPTION into lead_row.source_description by bitrix_lead_id."""

from __future__ import annotations

import csv
import os
import subprocess
import sys

MYSQL = ["mysql", "-N", "--batch", "--raw", "--default-character-set=utf8mb4", "bitrix_dump"]
CSV_PATH = "/tmp/source_description.csv"


def main() -> None:
    print("export mysql…", flush=True)
    proc = subprocess.Popen(
        MYSQL + ["-e", """
SELECT ID,
       REPLACE(REPLACE(REPLACE(SOURCE_DESCRIPTION, '\\r', ' '), '\\n', ' '), '\\t', ' ')
FROM b_crm_lead
WHERE SOURCE_DESCRIPTION IS NOT NULL
  AND TRIM(SOURCE_DESCRIPTION) <> '';
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
            value = value.strip()
            if not lead_id or not value:
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
    env.setdefault("PGHOST", "127.0.0.1")
    env.setdefault("PGUSER", "admin")
    env.setdefault("PGDATABASE", "sdmp")
    env.setdefault("PGPASSWORD", "admin_tmp_password")

    print("add column…", flush=True)
    alter = subprocess.run(
        ["sudo", "-u", "postgres", "psql", "-d", "sdmp", "-v", "ON_ERROR_STOP=1", "-f",
         os.path.join(os.path.dirname(__file__), "20260904-lead-row-source-description.sql")],
        capture_output=True,
        text=True,
    )
    sys.stdout.write(alter.stdout)
    if alter.returncode != 0:
        sys.stderr.write(alter.stderr)
        raise SystemExit(alter.returncode)

    sql = f"""
CREATE TEMP TABLE tmp_source_desc (
  bitrix_lead_id TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
\\copy tmp_source_desc (bitrix_lead_id, value) FROM '{CSV_PATH}' WITH (FORMAT csv)

SELECT
  (SELECT count(*) FROM tmp_source_desc) AS mysql_nonempty,
  (SELECT count(*) FROM lead_row lr JOIN tmp_source_desc t ON t.bitrix_lead_id = lr.bitrix_lead_id) AS matched;

UPDATE lead_row lr
SET source_description = t.value
FROM tmp_source_desc t
WHERE lr.bitrix_lead_id = t.bitrix_lead_id
  AND COALESCE(lr.source_description, '') = '';

SELECT count(*) AS filled
FROM lead_row
WHERE source_description IS NOT NULL AND source_description <> '';
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
