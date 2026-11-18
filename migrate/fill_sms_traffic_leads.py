#!/usr/bin/env python3
"""Import Bitrix SMS-traffic leads into lead_row as source 20, status 5.

Duplicates bitrix_lead_id if the lead already exists. No outbox / SK / partners.
"""

from __future__ import annotations

import csv
import json
import os
import subprocess
import sys

MYSQL = ["mysql", "-N", "--batch", "--raw", "--default-character-set=utf8mb4", "bitrix_dump"]
CSV_PATH = "/tmp/sms_traffic_leads.csv"
TMP_IDS = "tmp_sms_traffic_ids"
SEP = "\x1f"
SOURCE_ID = 20
STATUS_ID = 5

SOURCE_IDS = [
    "UC_4W0KQX",
    "UC_9B4E4W",
    "UC_SXCREC",
    "UC_3CW53Q",
    "sms.127gzrussia1.ru",
    "sms1.1minute-bfl.ru",
    "UC_N8LO9Y",
    "sms.gosbankrostvofl.ru",
    "UC_4AY1VJ",
    "UC_25IB0A",
    "UC_3159RF",
    "UC_0KGQBO",
    "UC_KO7S95",
    "UC_TJAUM9",
    "UC_3BGO3X",
    "UC_3NEC9F",
    "UC_8P2CQI",
    "UC_WNCJWV",
    "UC_1R83EG",
    "UC_1GPKOX",
]

ZARYA_MAP = {
    "Чистый": "Чистый",
    "ДЮ": "Дубль",
    "ДС менее 90": "Дубль",
    "ДС более 90": "Дубль",
}

ZARYA_AD_MAP = {
    "90": "да",
    "91": "не указано",
    "361": "Да_GDMP",
    "362": "нет",
}

UTM_KEYS = {
    "UTM_CAMPAIGN": "utm_campaign",
    "UTM_CONTENT": "utm_content",
    "UTM_MEDIUM": "utm_medium",
    "UTM_SOURCE": "utm_source",
    "UTM_TERM": "utm_term",
}

LEAD_FIELDS = [
    "id",
    "date_create",
    "date_modify",
    "title",
    "name",
    "comments",
    "source_description",
    "status_id",
    "address",
    "region",
    "city",
    "city_alt",
    "gdmp_operator",
    "operator_raw",
    "web_id",
    "tag",
    "call_url",
    "call_last_at",
    "project_num",
    "zarya",
    "cold_label",
    "base_label",
    "partner_sent",
    "dup_b24_soda",
    "duplicate_lead_id",
    "macroregion",
    "reason_no_meeting",
    "reason_no_contract",
    "zarya_nkch",
    "debt_qw2",
    "meeting_dt",
    "transfer_dt",
    "meeting_sched",
    "bailiffs",
    "trash_dt",
    "dmp_flow",
    "debt_type",
    "income",
    "other_info",
    "property",
    "debt_amount",
    "z_flag",
    "zarya_ad_dup",
    "ds90",
    "type_dup",
    "call_id",
]


def mysql(sql: str) -> str:
    proc = subprocess.run(MYSQL, input=sql, text=True, capture_output=True)
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)
    return proc.stdout


def mysql_lines(sql: str):
    proc = subprocess.Popen(
        MYSQL + ["-e", sql],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    assert proc.stdout is not None
    for line in proc.stdout:
        yield line.rstrip("\n")
    err = proc.stderr.read() if proc.stderr else ""
    if proc.wait() != 0:
        sys.stderr.write(err)
        raise SystemExit(proc.returncode)


def clean(val: str | None) -> str:
    if val is None:
        return ""
    return val.replace("\r", " ").replace("\n", " ").replace("\t", " ").strip()


def nonempty(val: str | None) -> str:
    v = clean(val)
    if v in ("", "-", "0000-00-00", "0000-00-00 00:00:00", "null", "[]", "{}"):
        return ""
    return v


def compact(data: dict[str, str]) -> str:
    out = {k: v for k, v in data.items() if v}
    if not out:
        return ""
    return json.dumps(out, ensure_ascii=False, separators=(",", ":"))


def pg_env() -> dict[str, str]:
    env = os.environ.copy()
    env.setdefault("PGHOST", "127.0.0.1")
    env.setdefault("PGPORT", "5432")
    env.setdefault("PGUSER", "admin")
    env.setdefault("PGDATABASE", "sdmp")
    env.setdefault("PGPASSWORD", "admin_tmp_password")
    return env


def psql(sql: str) -> None:
    proc = subprocess.run(
        ["sudo", "-u", "postgres", "psql", "-d", "sdmp", "-v", "ON_ERROR_STOP=1"],
        input=sql,
        text=True,
        capture_output=True,
    )
    sys.stdout.write(proc.stdout)
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)


def wrap_txt(expr: str) -> str:
    return (
        "REPLACE(REPLACE(REPLACE(IFNULL("
        + expr
        + ", ''), '\\n', ' '), '\\t', ' '), '\\r', ' ')"
    )


def wrap_dt(expr: str) -> str:
    return wrap_txt(f"DATE_FORMAT({expr}, '%Y-%m-%d %H:%i:%s')")


def collect_ids() -> int:
    ids_sql = ",".join("'" + s.replace("'", "''") + "'" for s in SOURCE_IDS)
    print("mysql collect ids…", flush=True)
    mysql(f"""
DROP TABLE IF EXISTS {TMP_IDS};
CREATE TABLE {TMP_IDS} (id INT PRIMARY KEY);
INSERT INTO {TMP_IDS}
SELECT ID FROM b_crm_lead
WHERE SOURCE_ID IN ({ids_sql});
""")
    n = int(mysql(f"SELECT COUNT(*) FROM {TMP_IDS}").strip())
    print(f"mysql ids={n}", flush=True)
    return n


def load_phones() -> dict[str, str]:
    print("mysql phones…", flush=True)
    out: dict[str, str] = {}
    n = 0
    for line in mysql_lines(f"""
SELECT m.ELEMENT_ID, m.VALUE
FROM b_crm_field_multi m
INNER JOIN {TMP_IDS} t ON t.id = m.ELEMENT_ID
WHERE m.ENTITY_ID = 'LEAD' AND m.TYPE_ID = 'PHONE'
ORDER BY m.ID
"""):
        parts = line.split("\t", 1)
        if len(parts) != 2:
            continue
        lead_id, value = parts
        if lead_id in out:
            continue
        value = nonempty(value)
        if value:
            out[lead_id] = value
            n += 1
    print(f"mysql phones={n}", flush=True)
    return out


def load_addr() -> dict[str, str]:
    print("mysql addr…", flush=True)
    out: dict[str, str] = {}
    for line in mysql_lines(f"""
SELECT a.ENTITY_ID,
       CONCAT_WS(', ',
         NULLIF(TRIM(a.ADDRESS_1), ''),
         NULLIF(TRIM(a.ADDRESS_2), ''),
         NULLIF(TRIM(a.CITY), ''),
         NULLIF(TRIM(a.REGION), ''),
         NULLIF(TRIM(a.PROVINCE), ''),
         NULLIF(TRIM(a.POSTAL_CODE), ''),
         NULLIF(TRIM(a.COUNTRY), '')
       )
FROM b_crm_addr a
INNER JOIN {TMP_IDS} t ON t.id = a.ENTITY_ID
WHERE a.ENTITY_TYPE_ID = 1
"""):
        parts = line.split("\t", 1)
        if len(parts) != 2:
            continue
        lead_id, value = parts
        value = nonempty(value)
        if value and lead_id not in out:
            out[lead_id] = value
    print(f"mysql addr={len(out)}", flush=True)
    return out


def load_utm() -> dict[str, dict[str, str]]:
    print("mysql utm…", flush=True)
    out: dict[str, dict[str, str]] = {}
    n = 0
    for line in mysql_lines(f"""
SELECT u.ENTITY_ID, u.CODE, u.VALUE
FROM b_crm_utm u
INNER JOIN {TMP_IDS} t ON t.id = u.ENTITY_ID
WHERE u.ENTITY_TYPE_ID = 1
  AND u.CODE IN ('UTM_CAMPAIGN','UTM_CONTENT','UTM_MEDIUM','UTM_SOURCE','UTM_TERM')
  AND u.VALUE IS NOT NULL AND TRIM(u.VALUE) <> ''
"""):
        parts = line.split("\t")
        if len(parts) != 3:
            continue
        lead_id, code, value = parts
        key = UTM_KEYS.get(code)
        value = nonempty(value)
        if not key or not value:
            continue
        out.setdefault(lead_id, {})[key] = value
        n += 1
    print(f"mysql utm_rows={n} leads={len(out)}", flush=True)
    return out


def load_enums() -> tuple[dict[str, str], dict[str, str]]:
    raw = mysql("""
SELECT ID, USER_FIELD_ID, VALUE
FROM b_user_field_enum
WHERE USER_FIELD_ID IN (1201, 406)
""")
    type_dup: dict[str, str] = {}
    zarya_ad: dict[str, str] = {}
    for line in raw.splitlines():
        parts = line.split("\t")
        if len(parts) != 3:
            continue
        enum_id, field_id, value = parts
        value = nonempty(value)
        if not value:
            continue
        if field_id == "1201":
            type_dup[enum_id] = value
        elif field_id == "406":
            zarya_ad[enum_id] = value
    return type_dup, zarya_ad


def export_csv(phones: dict[str, str], addr: dict[str, str], utm: dict[str, dict[str, str]]) -> int:
    type_dup, zarya_ad_enum = load_enums()
    cols_sql = ",\n  ".join(
        [
            "l.ID",
            wrap_dt("l.DATE_CREATE"),
            wrap_dt("l.DATE_MODIFY"),
            wrap_txt("l.TITLE"),
            wrap_txt("l.NAME"),
            wrap_txt("l.COMMENTS"),
            wrap_txt("l.SOURCE_DESCRIPTION"),
            wrap_txt("l.STATUS_ID"),
            wrap_txt("l.ADDRESS"),
            wrap_txt("u.UF_CRM_1747216350206"),
            wrap_txt("u.UF_CRM_1638446549"),
            wrap_txt("u.UF_CRM_1714979251"),
            wrap_txt("u.UF_CRM_1676626557"),
            wrap_txt("u.UF_CRM_1734523005552"),
            wrap_txt("u.UF_CRM_1752577998838"),
            wrap_txt("u.UF_CRM_1729062300505"),
            wrap_txt("u.UF_CRM_1747171451142"),
            wrap_txt("u.UF_CRM_1710321354659"),
            wrap_txt("u.UF_CRM_1699340770806"),
            wrap_txt("u.UF_CRM_1748968339326"),
            wrap_txt("u.UF_CRM_1697524451888"),
            wrap_txt("u.UF_CRM_1698755876042"),
            wrap_txt("u.UF_CRM_1750147327906"),
            wrap_txt("u.UF_CRM_1723637118439"),
            wrap_txt("u.UF_CRM_1723546478556"),
            wrap_txt("u.UF_CRM_1747338118253"),
            wrap_txt("u.UF_CRM_1747338192"),
            wrap_txt("u.UF_CRM_1747339138"),
            wrap_txt("u.UF_CRM_1731580098189"),
            wrap_txt("u.UF_CRM_1650444099345"),
            wrap_dt("u.UF_CRM_1745925814962"),
            wrap_dt("u.UF_CRM_1650437905198"),
            wrap_txt("u.UF_CRM_1723813359156"),
            wrap_txt("u.UF_CRM_1758104136848"),
            wrap_dt("u.UF_CRM_1755681342073"),
            wrap_txt("u.UF_CRM_1775028479"),
            wrap_txt("u.UF_CRM_1730630661182"),
            wrap_txt("u.UF_CRM_1742819602566"),
            wrap_txt("u.UF_CRM_1730631041590"),
            wrap_txt("u.UF_CRM_1730630726419"),
            wrap_txt("u.UF_CRM_1730630573577"),
            wrap_txt("u.UF_CRM_1747907679528"),
            wrap_txt("u.UF_CRM_1722831843010"),
            wrap_txt("u.UF_CRM_1752052705219"),
            wrap_txt("u.UF_CRM_1766132352291"),
            wrap_txt("u.UF_CRM_MGO_CC_ENTRY_ID"),
        ]
    )
    sql = f"""
SELECT CONCAT_WS(CHAR(31),
  {cols_sql}
)
FROM {TMP_IDS} t
JOIN b_crm_lead l ON l.ID = t.id
LEFT JOIN b_uts_crm_lead u ON u.VALUE_ID = t.id
"""
    print("mysql leads+uts…", flush=True)
    n = 0
    bad = 0
    with open(CSV_PATH, "w", encoding="utf-8", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        for line in mysql_lines(sql):
            parts = line.split(SEP)
            if len(parts) != len(LEAD_FIELDS):
                bad += 1
                continue
            row = {k: nonempty(v) for k, v in zip(LEAD_FIELDS, parts)}
            lead_id = row["id"]
            if not lead_id:
                bad += 1
                continue
            city = row["city"] or row["city_alt"]
            phone = phones.get(lead_id, "")
            address = addr.get(lead_id, "") or row["address"]
            zarya_ad = (
                zarya_ad_enum.get(row["zarya_ad_dup"])
                or ZARYA_AD_MAP.get(row["zarya_ad_dup"])
                or row["zarya_ad_dup"]
            )
            type_dup_val = type_dup.get(row["type_dup"], "")
            legacy = compact(
                {
                    "address": address,
                    "dup_b24_soda": row["dup_b24_soda"],
                    "duplicate_lead_id": row["duplicate_lead_id"],
                    "macroregion": row["macroregion"],
                    "reason_no_meeting": row["reason_no_meeting"],
                    "reason_no_contract": row["reason_no_contract"],
                    "zarya_nkch_reason": row["zarya_nkch"],
                    "debt_amount_qw2": row["debt_qw2"],
                    "gdmp_meeting_datetime": row["meeting_dt"],
                    "gdmp_transfer_date": row["transfer_dt"],
                    "gdmp_meeting_scheduled": row["meeting_sched"],
                    "gdmp_operator": row["gdmp_operator"],
                    "gdmp_bailiffs": row["bailiffs"],
                    "trash_stage_date": row["trash_dt"],
                    "is_duplicate_stage": "Y" if row["status_id"] == "UC_NAJASU" else "",
                    "source_description": row["source_description"],
                    "partner_sent_flag": row["partner_sent"],
                    "client_name": row["name"],
                    "comment": row["comments"],
                    "lead_title": row["title"],
                    "operator_raw": row["operator_raw"],
                    "dmp_flow": row["dmp_flow"],
                    "call_link_legacy": row["call_url"],
                    "project_number": row["project_num"],
                    **utm.get(lead_id, {}),
                    "gdmp_debt_type": row["debt_type"],
                    "gdmp_income": row["income"],
                    "gdmp_other_info": row["other_info"],
                    "gdmp_property": row["property"],
                    "gdmp_debt_amount": row["debt_amount"],
                    "gdmp_tags_raw": row["tag"],
                    "gdmp_z_flag": row["z_flag"],
                    "lead_type_dup_250_300": type_dup_val,
                    "lead_zarya_ad_dup": zarya_ad,
                    "lead_ds90_dup_flag": row["ds90"],
                }
            )
            call_form = compact(
                {
                    "imya": row["name"],
                    "summa_dolga": row["debt_amount"],
                    "vid_dolga": row["debt_type"],
                    "dohod": row["income"],
                    "imuschestvo": row["property"],
                    "gorod": city,
                }
            )
            w.writerow(
                [
                    lead_id,
                    row["date_create"] or r"\N",
                    row["date_modify"] or r"\N",
                    phone,
                    row["region"] or r"\N",
                    city or r"\N",
                    row["operator_raw"] or r"\N",
                    row["web_id"] or r"\N",
                    row["tag"] or r"\N",
                    row["project_num"] or r"\N",
                    row["call_id"] or r"\N",
                    row["call_url"] or r"\N",
                    row["call_last_at"] or r"\N",
                    row["zarya"] or r"\N",
                    row["source_description"] or r"\N",
                    row["cold_label"] or r"\N",
                    row["base_label"] or r"\N",
                    legacy or r"\N",
                    call_form or r"\N",
                ]
            )
            n += 1
            if n % 10000 == 0:
                print(f"  exported {n}", flush=True)
    print(f"csv rows={n} skipped={bad}", flush=True)
    return n


def insert_pg() -> None:
    print("index…", flush=True)
    schema = open(
        os.path.join(os.path.dirname(__file__), "20260922-lead-row-bitrix-id-nonunique.sql"),
        encoding="utf-8",
    ).read()
    psql(schema)

    sql = f"""
SET synchronous_commit = off;

CREATE TEMP TABLE tmp_sms_in (
  bitrix_lead_id TEXT NOT NULL,
  date_create TIMESTAMP,
  date_modify TIMESTAMP,
  phone TEXT,
  region_raw TEXT,
  city TEXT,
  operator_raw TEXT,
  web_id TEXT,
  tag TEXT,
  project_num TEXT,
  call_id TEXT,
  call_url TEXT,
  call_last_at TEXT,
  zarya_raw TEXT,
  source_description TEXT,
  cold_label TEXT,
  base_label TEXT,
  legacy_fields TEXT,
  call_form TEXT
);
\\copy tmp_sms_in FROM '{CSV_PATH}' WITH (FORMAT csv, NULL '\\N')

SELECT count(*) AS csv_rows FROM tmp_sms_in;

INSERT INTO lead_row (
  uuid, phone, phone_valid, via_us, source_id, child_source_id,
  region_id, region_source, city, operator_id, operator_source, operator_raw,
  web_id, tag, call_id, call_url, call_last_at, project_num, call_form,
  status_id, s3_key, archived_at, created_at, updated_at,
  legacy_fields, bitrix_lead_id, source_description, cold_label, base_label
)
SELECT
  gen_random_uuid()::text,
  CASE
    WHEN length(d.digits) = 11 AND d.digits LIKE '8%' THEN '7' || substr(d.digits, 2)
    WHEN length(d.digits) = 10 THEN '7' || d.digits
    WHEN length(d.digits) = 11 AND d.digits LIKE '7%' THEN d.digits
    ELSE COALESCE(d.digits, '')
  END,
  (
    CASE
      WHEN length(d.digits) = 11 AND d.digits LIKE '8%' THEN '7' || substr(d.digits, 2)
      WHEN length(d.digits) = 10 THEN '7' || d.digits
      WHEN length(d.digits) = 11 AND d.digits LIKE '7%' THEN d.digits
      ELSE COALESCE(d.digits, '')
    END
  ) ~ '^7[0-9]{{10}}$',
  true,
  {SOURCE_ID},
  NULL,
  ra.region_id,
  CASE
    WHEN t.region_raw IS NULL OR btrim(t.region_raw) IN ('', '-') THEN NULL
    WHEN ra.region_id IS NOT NULL THEN 1
    ELSE 0
  END,
  NULLIF(btrim(t.city), ''),
  op.id,
  CASE
    WHEN NULLIF(btrim(t.operator_raw), '') IS NULL THEN NULL
    WHEN op.id IS NOT NULL THEN 1
    ELSE 0
  END,
  NULLIF(btrim(t.operator_raw), ''),
  NULLIF(btrim(t.web_id), ''),
  NULLIF(btrim(t.tag), ''),
  NULLIF(btrim(t.call_id), ''),
  NULLIF(btrim(t.call_url), ''),
  CASE
    WHEN t.call_last_at IS NULL OR btrim(t.call_last_at) = '' THEN NULL
    WHEN t.call_last_at ~ '^[0-9]{{4}}-' THEN t.call_last_at::timestamp
    ELSE NULL
  END,
  NULLIF(btrim(t.project_num), ''),
  NULLIF(btrim(t.call_form), ''),
  {STATUS_ID},
  NULL,
  NULL,
  COALESCE(t.date_create, now()),
  t.date_modify,
  NULLIF(btrim(t.legacy_fields), ''),
  t.bitrix_lead_id,
  NULLIF(btrim(t.source_description), ''),
  NULLIF(btrim(t.cold_label), ''),
  NULLIF(btrim(t.base_label), '')
FROM tmp_sms_in t
LEFT JOIN region_alias ra
  ON ra.alias = CASE
    WHEN t.region_raw IS NULL THEN NULL
    WHEN position('|' IN t.region_raw) > 0 THEN btrim(split_part(t.region_raw, '|', 2))
    ELSE btrim(t.region_raw)
  END
LEFT JOIN LATERAL (
  SELECT CASE
    WHEN t.operator_raw IS NULL OR btrim(t.operator_raw) = '' THEN NULL
    WHEN upper(t.operator_raw) IN ('BEELINE')
      OR upper(t.operator_raw) LIKE '%БИЛАЙН%'
      OR upper(t.operator_raw) LIKE '%ВЫМПЕЛКОМ%' THEN 'БИЛАЙН'
    WHEN upper(t.operator_raw) IN ('MTS')
      OR upper(t.operator_raw) LIKE '%МТС%'
      OR upper(t.operator_raw) LIKE '%МГТС%' THEN 'МТС'
    WHEN upper(t.operator_raw) IN ('MEGAFON')
      OR upper(t.operator_raw) LIKE '%МЕГАФОН%'
      OR upper(t.operator_raw) LIKE '%ЙОТА%'
      OR upper(t.operator_raw) LIKE '%YOTA%'
      OR upper(t.operator_raw) LIKE '%СКАРТЕЛ%' THEN 'МЕГАФОН'
    WHEN upper(t.operator_raw) IN ('TELE2', 'T2')
      OR upper(t.operator_raw) LIKE '%ТЕЛЕ2%'
      OR upper(t.operator_raw) LIKE '%ТЕЛЕ-2%'
      OR upper(t.operator_raw) = 'Т2 МОБАЙЛ'
      OR upper(t.operator_raw) LIKE '%Т2 МОБАЙЛ%' THEN 'ТЕЛЕ-2'
    ELSE 'ДРУГИЕ'
  END AS grp
) og ON TRUE
LEFT JOIN operator op ON op.name = og.grp
CROSS JOIN LATERAL (
  SELECT regexp_replace(COALESCE(t.phone, ''), '[^0-9]', '', 'g') AS digits
) d;

CREATE TEMP TABLE tmp_sms_zarya AS
SELECT bitrix_lead_id, zarya_raw
FROM tmp_sms_in
WHERE zarya_raw IN ('Чистый', 'ДЮ', 'ДС менее 90', 'ДС более 90');
CREATE INDEX ON tmp_sms_zarya (bitrix_lead_id);

INSERT INTO dedup_result (lead_row_id, provider_id, value_id, checked_at, raw_response)
SELECT lr.id, p.id, v.id, COALESCE(lr.created_at, now()), t.zarya_raw
FROM lead_row lr
JOIN tmp_sms_zarya t ON t.bitrix_lead_id = lr.bitrix_lead_id
JOIN dedup_provider p ON lower(p.name) IN ('заря', 'zarya')
JOIN dedup_value v ON v.provider_id = p.id AND v.name = CASE t.zarya_raw
  WHEN 'Чистый' THEN 'Чистый'
  WHEN 'ДЮ' THEN 'Дубль'
  WHEN 'ДС менее 90' THEN 'Дубль'
  WHEN 'ДС более 90' THEN 'Дубль'
END
WHERE lr.source_id = {SOURCE_ID}
  AND NOT EXISTS (
    SELECT 1 FROM dedup_result dr
    WHERE dr.lead_row_id = lr.id AND dr.provider_id = p.id
  );

SELECT
  (SELECT count(*) FROM lead_row WHERE source_id = {SOURCE_ID}) AS source20,
  (SELECT count(*) FROM lead_row WHERE source_id = {SOURCE_ID} AND status_id = {STATUS_ID}) AS status5,
  (SELECT count(*) FROM lead_row WHERE source_id = {SOURCE_ID} AND phone_valid) AS valid_phone,
  (SELECT count(*) FROM lead_row WHERE source_id = {SOURCE_ID} AND region_id IS NOT NULL) AS with_region,
  (SELECT count(*) FROM lead_row WHERE source_id = {SOURCE_ID} AND operator_id IS NOT NULL) AS with_operator,
  (SELECT count(*) FROM lead_row WHERE source_id = {SOURCE_ID} AND legacy_fields IS NOT NULL) AS with_legacy,
  (SELECT count(*) FROM lead_row lr
     JOIN dedup_result dr ON dr.lead_row_id = lr.id
     JOIN dedup_provider p ON p.id = dr.provider_id
    WHERE lr.source_id = {SOURCE_ID} AND lower(p.name) IN ('заря', 'zarya')) AS zarya,
  (SELECT count(*) FROM outbox o
     JOIN lead_row lr ON lr.id = o.lead_row_id
    WHERE lr.source_id = {SOURCE_ID}) AS outbox_rows;

SELECT count(*) AS already_had_bitrix_id
FROM tmp_sms_in t
WHERE EXISTS (
  SELECT 1 FROM lead_row lr
  WHERE lr.bitrix_lead_id = t.bitrix_lead_id
    AND lr.source_id IS DISTINCT FROM {SOURCE_ID}
);
"""
    print("insert lead_row…", flush=True)
    psql(sql)


def main() -> None:
    import argparse

    global CSV_PATH, TMP_IDS, SOURCE_ID, SOURCE_IDS
    p = argparse.ArgumentParser()
    p.add_argument("--source-id", type=int, default=SOURCE_ID)
    p.add_argument("--bitrix", action="append", default=None)
    p.add_argument("--csv", default=CSV_PATH)
    p.add_argument("--tmp-table", default=TMP_IDS)
    args = p.parse_args()
    SOURCE_ID = args.source_id
    CSV_PATH = args.csv
    TMP_IDS = args.tmp_table
    if args.bitrix:
        SOURCE_IDS = args.bitrix
    print(f"source_id={SOURCE_ID} bitrix={SOURCE_IDS} csv={CSV_PATH}", flush=True)

    n = collect_ids()
    if n == 0:
        print("no leads")
        return
    phones = load_phones()
    addr = load_addr()
    utm = load_utm()
    exported = export_csv(phones, addr, utm)
    if exported == 0:
        print("nothing to insert")
        return
    insert_pg()
    mysql(f"DROP TABLE IF EXISTS {TMP_IDS};")
    print("done", flush=True)


if __name__ == "__main__":
    main()
