#!/usr/bin/env python3
"""Copy UF_CRM_1748968339326 into dedup_result (Заря) by bitrix_lead_id.

Чистый → Чистый (uniq). ДЮ / ДС менее 90 / ДС более 90 → Дубль (dup).
"""

from __future__ import annotations

import csv
import os
import subprocess
import sys

MYSQL = ["mysql", "-N", "--batch", "--raw", "--default-character-set=utf8mb4", "bitrix_dump"]
CSV_PATH = "/tmp/uf_zarya.csv"
MAP = {
    "Чистый": "Чистый",
    "ДЮ": "Дубль",
    "ДС менее 90": "Дубль",
    "ДС более 90": "Дубль",
}


def main() -> None:
    print("export mysql…", flush=True)
    proc = subprocess.Popen(
        MYSQL + ["-e", """
SELECT VALUE_ID, UF_CRM_1748968339326
FROM b_uts_crm_lead
WHERE UF_CRM_1748968339326 IN ('Чистый', 'ДЮ', 'ДС менее 90', 'ДС более 90');
"""],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    n = 0
    skipped = 0
    with open(CSV_PATH, "w", encoding="utf-8", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        assert proc.stdout is not None
        for line in proc.stdout:
            parts = line.rstrip("\n").split("\t", 1)
            if len(parts) != 2:
                skipped += 1
                continue
            lead_id, raw = parts
            mapped = MAP.get(raw.strip())
            if not lead_id or not mapped:
                skipped += 1
                continue
            w.writerow([lead_id, mapped, raw.strip()])
            n += 1
            if n % 1_000_000 == 0:
                print(f"  exported {n}", flush=True)
    err = proc.stderr.read() if proc.stderr else ""
    if proc.wait() != 0:
        sys.stderr.write(err)
        raise SystemExit(proc.returncode)
    print(f"mysql nonempty={n} skipped={skipped}", flush=True)
    if n == 0:
        print("nothing to copy")
        return

    env = os.environ.copy()
    env.setdefault("PGHOST", "127.0.0.1")
    env.setdefault("PGUSER", "admin")
    env.setdefault("PGDATABASE", "sdmp")
    env.setdefault("PGPASSWORD", "admin_tmp_password")
    sql = f"""
CREATE TEMP TABLE tmp_zarya (
  bitrix_lead_id TEXT PRIMARY KEY,
  mapped TEXT NOT NULL,
  raw TEXT NOT NULL
);
\\copy tmp_zarya (bitrix_lead_id, mapped, raw) FROM '{CSV_PATH}' WITH (FORMAT csv)

SELECT
  (SELECT count(*) FROM tmp_zarya) AS mysql_nonempty,
  (SELECT count(*) FROM lead_row lr JOIN tmp_zarya t ON t.bitrix_lead_id = lr.bitrix_lead_id) AS matched,
  (SELECT count(*) FROM tmp_zarya t WHERE t.mapped = 'Чистый') AS mysql_uniq,
  (SELECT count(*) FROM tmp_zarya t WHERE t.mapped = 'Дубль') AS mysql_dup;

INSERT INTO dedup_result (lead_row_id, provider_id, value_id, checked_at, raw_response)
SELECT lr.id, p.id, v.id, COALESCE(lr.created_at, now()), t.raw
FROM tmp_zarya t
JOIN lead_row lr ON lr.bitrix_lead_id = t.bitrix_lead_id
JOIN dedup_provider p ON lower(p.name) IN ('заря', 'zarya')
JOIN dedup_value v ON v.provider_id = p.id AND v.name = t.mapped
WHERE NOT EXISTS (
  SELECT 1 FROM dedup_result dr
  WHERE dr.lead_row_id = lr.id AND dr.provider_id = p.id
);

SELECT
  (SELECT count(*) FROM dedup_result dr
   JOIN dedup_provider p ON p.id = dr.provider_id
   WHERE lower(p.name) IN ('заря', 'zarya')) AS zarya_total;
SELECT v.name, count(*)
FROM dedup_result dr
JOIN dedup_provider p ON p.id = dr.provider_id
JOIN dedup_value v ON v.id = dr.value_id
WHERE lower(p.name) IN ('заря', 'zarya')
GROUP BY v.name
ORDER BY count(*) DESC;
"""
    print("insert dedup_result…", flush=True)
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
