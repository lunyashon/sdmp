#!/usr/bin/env python3
"""Prepare dictionaries and INSERT e_dmp_tmp_leads_2 → lead_row."""

from __future__ import annotations

import os
import subprocess
import sys

PSQL = [
    "sudo", "-u", "postgres", "psql", "-d", "sdmp", "-v", "ON_ERROR_STOP=1",
]

REGIONS = [
    "Республика Адыгея",
    "Алтайский край",
    "Амурская область",
    "Архангельская область",
    "Астраханская область",
    "Республика Башкортостан",
    "Белгородская область",
    "Брянская область",
    "Республика Бурятия",
    "Владимирская область",
    "Волгоградская область",
    "Вологодская область",
    "Воронежская область",
    "Республика Дагестан",
    "Еврейская АО",
    "Забайкальский край",
    "Ивановская область",
    "Республика Ингушетия",
    "Иркутская область",
    "Кабардино-Балкарская Республика",
    "Калининградская область",
    "Республика Калмыкия",
    "Калужская область",
    "Камчатский край",
    "Карачаево-Черкесия",
    "Республика Карелия",
    "Кемеровская область",
    "Кировская область",
    "Республика Коми",
    "Костромская область",
    "Краснодарский край",
    "Красноярский край",
    "Курганская область",
    "Курская область",
    "Ленинградская область",
    "Липецкая область",
    "Магаданская область",
    "Республика Марий Эл",
    "Республика Мордовия",
    "Московская область",
    "Мурманская область",
    "Ненецкий автономный округ",
    "Нижегородская область",
    "Новгородская область",
    "Новосибирская область",
    "Омская область",
    "Оренбургская область",
    "Орловская область",
    "Пензенская область",
    "Пермский край",
    "Приморский край",
    "Псковская область",
    "Ростовская область",
    "Рязанская область",
    "Самарская область",
    "Саратовская область",
    "Сахалинская область",
    "Свердловская область",
    "Северная Осетия",
    "Смоленская область",
    "Ставропольский край",
    "Тамбовская область",
    "Республика Татарстан",
    "Тверская область",
    "Томская область",
    "Тульская область",
    "Тыва республика",
    "Тюменская область",
    "Удмуртия",
    "Ульяновская область",
    "Хабаровский край",
    "Республика Хакасия",
    "Ханты-Мансийский автономный округ",
    "Челябинская область",
    "Чеченская республика",
    "Чувашская Республика",
    "Чукотский автономный округ",
    "Республика Саха",
    "Ямало-Ненецкий автономный округ",
    "Ярославская область",
    "Республика крым",
]

# dump alias → canonical name
ALIASES = {
    "г. Москва": "Московская область",
    "Москва": "Московская область",
    "Москва и Московская область": "Московская область",
    "г. Москва и Московская область": "Московская область",
    "г. Москва * Московская область": "Московская область",
    "Московская область, г. Москва": "Московская область",
    "г. Санкт-Петербург": "Ленинградская область",
    "Санкт-Петербург": "Ленинградская область",
    "г. Санкт-Петербург и Ленинградская область": "Ленинградская область",
    "г. Санкт-Петербург * Ленинградская область": "Ленинградская область",
    "Ленинградская область, г. Санкт-Петербург": "Ленинградская область",
    "Санкт-Петербург и Ленинградская область": "Ленинградская область",
    "Республика Крым и г. Севастополь": "Республика крым",
    "Крым и Севастополь": "Республика крым",
    "Севастополь": "Республика крым",
    "Севастопль": "Республика крым",
    "Республика Алтай": "Алтайский край",
    "Республика Крым": "Республика крым",
    "Архангельская область и Ненецкий автономный округ": "Архангельская область",
    "Архангельская область * Ненецкий автономный округ": "Архангельская область",
    "Архангельская область и Ненецкий АО": "Архангельская область",
    "Кемеровская область - Кузбасс": "Кемеровская область",
    "Кемеровская область - Кузбасс обл.": "Кемеровская область",
    "Кемеровская область - Кузбасс область": "Кемеровская область",
    "Республика Саха /Якутия/": "Республика Саха",
    "Республика Саха (Якутия)": "Республика Саха",
    "Республика Северная Осетия - Алания": "Северная Осетия",
    "Республика Северная Осетия — Алания": "Северная Осетия",
    "Ханты-Мансийский автономный округ - Югра": "Ханты-Мансийский автономный округ",
    "Ханты-Мансийский автономный округ — Югра": "Ханты-Мансийский автономный округ",
    "Ханты-Мансийский автономный округ-Югра": "Ханты-Мансийский автономный округ",
    "Ханты-Мансийский Автономный округ - Югра АО": "Ханты-Мансийский автономный округ",
    "Ханты-Мансийский АО - Югра": "Ханты-Мансийский автономный округ",
    "Ханты - Мансийский - Югра АО": "Ханты-Мансийский автономный округ",
    "Ямало-Ненецкий АО": "Ямало-Ненецкий автономный округ",
    "Ненецкий АО": "Ненецкий автономный округ",
    "Чукотский АО": "Чукотский автономный округ",
    "Еврейская автономная область": "Еврейская АО",
    "Еврейская автономная обл.": "Еврейская АО",
    "Республика Удмуртия": "Удмуртия",
    "Республика Удмуртская": "Удмуртия",
    "Удмуртская Республика": "Удмуртия",
    "Республика Тыва": "Тыва республика",
    "Карачаево-Черкесская Республика": "Карачаево-Черкесия",
    "Республика Карачаево-Черкесская": "Карачаево-Черкесия",
    "Кабардино-Балкарская республика": "Кабардино-Балкарская Республика",
    "Республика Кабардино-Балкария": "Кабардино-Балкарская Республика",
    "Республика Кабардино-Балкарская": "Кабардино-Балкарская Республика",
    "Чувашская республика": "Чувашская Республика",
    "Чувашская Республика - Чувашия": "Чувашская Республика",
    "Республика Чувашия": "Чувашская Республика",
    "Чеченская Республика": "Чеченская республика",
    "Республика Чеченская": "Чеченская республика",
    "Республика Чечня": "Чеченская республика",
    "Донецкая область": "Ростовская область",
    "Донецкая Народная Республика": "Ростовская область",
    "Республика Донецкая Народная": "Ростовская область",
    "Калужская": "Калужская область",
    "Республика Татарстан (Татарстан)": "Республика Татарстан",
}


def sql_str(s: str) -> str:
    return "'" + s.replace("'", "''") + "'"


def psql(sql: str) -> None:
    proc = subprocess.run(PSQL, input=sql, text=True, capture_output=True)
    if proc.returncode != 0:
        sys.stderr.write(proc.stdout)
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)
    if proc.stdout.strip():
        print(proc.stdout, end="")


def main() -> None:
    schema = open(
        os.path.join(os.path.dirname(__file__), "20260830-lead-row-from-staging.sql"),
        encoding="utf-8",
    ).read()
    print("schema…", flush=True)
    psql(schema)

    print("regions…", flush=True)
    rows = ",\n".join(f"({sql_str(n)})" for n in REGIONS)
    psql(f"""
CREATE TABLE IF NOT EXISTS region_alias (
  alias TEXT PRIMARY KEY,
  region_id INT NOT NULL REFERENCES region(id)
);
INSERT INTO region (name)
SELECT n FROM (VALUES {rows}) AS v(n)
WHERE NOT EXISTS (SELECT 1 FROM region r WHERE r.name = v.n);

INSERT INTO region_alias (alias, region_id)
SELECT a.alias, r.id
FROM (VALUES
{chr(10).join(f"  ({sql_str(a)}, {sql_str(c)})," for a, c in ALIASES.items()).rstrip(",")}
) AS a(alias, canon)
JOIN region r ON r.name = a.canon
ON CONFLICT (alias) DO UPDATE SET region_id = EXCLUDED.region_id;

-- короткие формы «Х обл.» → «Х область»
INSERT INTO region_alias (alias, region_id)
SELECT replace(r.name, ' область', ' обл.'), r.id
FROM region r
WHERE r.name LIKE '% область'
ON CONFLICT (alias) DO NOTHING;

INSERT INTO region_alias (alias, region_id)
SELECT r.name, r.id FROM region r
ON CONFLICT (alias) DO NOTHING;
""")

    print("insert lead_row…", flush=True)
    psql(r"""
SET synchronous_commit = off;

INSERT INTO lead_row (
  uuid, phone, phone_valid, source_id, child_source_id,
  region_id, region_source, city, operator_id, operator_source,
  web_id, tag, call_id, call_url, call_last_at, project_num, call_form,
  status_id, s3_key, archived_at, created_at, updated_at,
  legacy_fields, bitrix_lead_id
)
SELECT
  format(
    '%s-%s-%s-%s-%s',
    substr(md5('sdmp-lead-' || t.owner_id), 1, 8),
    substr(md5('sdmp-lead-' || t.owner_id), 9, 4),
    substr(md5('sdmp-lead-' || t.owner_id), 13, 4),
    substr(md5('sdmp-lead-' || t.owner_id), 17, 4),
    substr(md5('sdmp-lead-' || t.owner_id), 21, 12)
  ),
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
  ) ~ '^7[0-9]{10}$',
  COALESCE(s.parent_id, s.id),
  CASE WHEN s.parent_id IS NOT NULL THEN s.id END,
  ra.region_id,
  CASE
    WHEN t.region IS NULL OR btrim(t.region) IN ('', '-') THEN NULL
    WHEN ra.region_id IS NOT NULL THEN 1
    ELSE 0
  END,
  NULLIF(btrim(COALESCE(NULLIF(btrim(t.city), ''), t.city_alt)), ''),
  op.id,
  CASE
    WHEN NULLIF(btrim(t.operator_id_raw), '') IS NULL THEN NULL
    WHEN op.id IS NOT NULL THEN 1
    ELSE 0
  END,
  NULLIF(btrim(t.web_id), ''),
  NULLIF(btrim(t.tag), ''),
  NULLIF(btrim(t.call_id), ''),
  NULLIF(btrim(t.call_url), ''),
  t.call_last_at,
  NULLIF(btrim(t.project_num), ''),
  CASE
    WHEN t.call_form IS NULL OR btrim(t.call_form) IN ('', '[]', '{}', 'null') THEN NULL
    ELSE t.call_form
  END,
  CASE
    WHEN t.source_id IN ('UC_L26K09', 'UC_VEAX7V') THEN 5
    WHEN t.status_id IN ('UC_SPNNRS', 'CONVERTED', '5') THEN 5
    WHEN t.status_id = 'JUNK' THEN 6
    WHEN t.call_form ~* '(summa_dolga|vid_dolga|dohod|zalog|imuschestvo|bankrotstvo)' THEN 4
    ELSE 2
  END,
  NULL,
  NULL,
  t.date_create,
  t.date_modify,
  NULLIF(btrim(t.legacy_fields), ''),
  t.owner_id
FROM e_dmp_tmp_leads_2 t
JOIN source s ON s.bitrix_code = t.source_id
LEFT JOIN region_alias ra
  ON ra.alias = CASE
    WHEN t.region IS NULL THEN NULL
    WHEN position('|' IN t.region) > 0 THEN btrim(split_part(t.region, '|', 2))
    ELSE btrim(t.region)
  END
LEFT JOIN LATERAL (
  SELECT CASE
    WHEN t.operator_id_raw IS NULL OR btrim(t.operator_id_raw) = '' THEN NULL
    WHEN upper(t.operator_id_raw) IN ('BEELINE')
      OR upper(t.operator_id_raw) LIKE '%БИЛАЙН%'
      OR upper(t.operator_id_raw) LIKE '%ВЫМПЕЛКОМ%' THEN 'БИЛАЙН'
    WHEN upper(t.operator_id_raw) IN ('MTS')
      OR upper(t.operator_id_raw) LIKE '%МТС%'
      OR upper(t.operator_id_raw) LIKE '%МГТС%' THEN 'МТС'
    WHEN upper(t.operator_id_raw) IN ('MEGAFON')
      OR upper(t.operator_id_raw) LIKE '%МЕГАФОН%'
      OR upper(t.operator_id_raw) LIKE '%ЙОТА%'
      OR upper(t.operator_id_raw) LIKE '%YOTA%'
      OR upper(t.operator_id_raw) LIKE '%СКАРТЕЛ%' THEN 'МЕГАФОН'
    WHEN upper(t.operator_id_raw) IN ('TELE2', 'T2')
      OR upper(t.operator_id_raw) LIKE '%ТЕЛЕ2%'
      OR upper(t.operator_id_raw) LIKE '%ТЕЛЕ-2%'
      OR upper(t.operator_id_raw) = 'Т2 МОБАЙЛ'
      OR upper(t.operator_id_raw) LIKE '%Т2 МОБАЙЛ%' THEN 'ТЕЛЕ-2'
    ELSE 'ДРУГИЕ'
  END AS grp
) og ON TRUE
LEFT JOIN operator op ON op.name = og.grp
CROSS JOIN LATERAL (
  SELECT regexp_replace(COALESCE(t.phone, ''), '[^0-9]', '', 'g') AS digits
) d
WHERE NOT EXISTS (
  SELECT 1 FROM lead_row lr WHERE lr.bitrix_lead_id = t.owner_id
);
""")

    print("indexes…", flush=True)
    psql("""
CREATE UNIQUE INDEX IF NOT EXISTS lead_row_bitrix_lead_id_uidx
  ON lead_row (bitrix_lead_id) WHERE bitrix_lead_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS lead_row_phone_idx ON lead_row (phone);
CREATE INDEX IF NOT EXISTS lead_row_source_created_idx ON lead_row (source_id, created_at);
CREATE INDEX IF NOT EXISTS lead_row_status_idx ON lead_row (status_id);
ANALYZE lead_row;

SELECT
  (SELECT count(*) FROM e_dmp_tmp_leads_2) AS staging,
  (SELECT count(*) FROM lead_row WHERE bitrix_lead_id IS NOT NULL) AS migrated,
  (SELECT count(*) FROM lead_row) AS lead_row_total;

SELECT status_id, count(*) FROM lead_row GROUP BY 1 ORDER BY 1;
SELECT count(*) FILTER (WHERE phone_valid) AS valid_phone,
       count(*) FILTER (WHERE NOT phone_valid) AS invalid_phone,
       count(*) FILTER (WHERE region_id IS NOT NULL) AS with_region,
       count(*) FILTER (WHERE operator_id IS NOT NULL) AS with_operator,
       count(*) FILTER (WHERE child_source_id IS NOT NULL) AS with_child
FROM lead_row
WHERE bitrix_lead_id IS NOT NULL;
""")


if __name__ == "__main__":
    main()
