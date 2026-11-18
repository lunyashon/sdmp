#!/usr/bin/env python3
"""Заливка отчёта BigData-IT в crm.lead.add.

По умолчанию только отчёт (--dry-run): читает CSV, выкидывает уже принятые
на источнике токена (phone или web_id), печатает статистику и пример тела.

Реальная отправка: --apply
Отфильтрованный CSV: --write-pending path.csv
"""

from __future__ import annotations

import argparse
import csv
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from collections import Counter
from pathlib import Path

DEFAULT_CSV = Path("/root/projects/1790235708_report_739367_974997.csv")
DEFAULT_TOKEN = "4524vby182ytwhs9"
DEFAULT_BASE = "https://cdp-dmp.ru"
DEFAULT_TAG = "vr_confirmed_kc"
DEFAULT_SOURCE_DESCRIPTION = "test"
TAG_MAP = {
    "подтвержден кц": "vr_confirmed_kc",
    "подтверждён кц": "vr_confirmed_kc",
    "vr_confirmed_kc": "vr_confirmed_kc",
}
WEB_ID_FIELD = "UF_CRM_1752577998838"
TAG_FIELD = "UF_CRM_1766149282"

PHONE_ALIASES = (
    "phone", "телефон", "тел", "mobile", "msisdn", "номер", "number",
)
WEB_ALIASES = (
    "id", "visit_id", "visitid", "web_id", "webid", "ident_id", "id визита",
    "идентификатор", "id идентификации", "identification_id", "visit",
)
NAME_ALIASES = ("name", "имя", "fio", "фио")
TITLE_ALIASES = ("title", "название", "заголовок")
EMAIL_ALIASES = ("email", "почта", "e-mail")
DOMAIN_ALIASES = ("domain", "сайт", "с сайта", "project", "проект", "host")
TIME_ALIASES = ("time", "datetime", "date", "дата", "время", "created", "создано")
IP_ALIASES = ("ip", "ip-адрес", "ip_address")
CITY_ALIASES = ("city", "город", "местоположение", "geo")
URL_ALIASES = ("url", "адрес страницы", "page", "page_url")
UTM_SOURCE_ALIASES = ("utm_source",)
UTM_MEDIUM_ALIASES = ("utm_medium",)
UTM_CAMPAIGN_ALIASES = ("utm_campaign",)
UTM_CONTENT_ALIASES = ("utm_content",)
UTM_TERM_ALIASES = ("utm_term",)
COMMENTS_ALIASES = ("comments", "комментарий", "comment")
SOURCE_DESC_ALIASES = ("source_description", "дополнительно об источнике")
TAG_ALIASES = ("tag", "тег", "тег проекта")


def norm_key(name: str) -> str:
    return " ".join(name.strip().lower().replace("_", " ").replace("-", " ").split())


def pick(headers: list[str], aliases: tuple[str, ...]) -> str | None:
    keyed = {norm_key(h): h for h in headers}
    for alias in aliases:
        if alias in keyed:
            return keyed[alias]
    for h in headers:
        hn = norm_key(h)
        for alias in aliases:
            if alias and (hn == alias or alias in hn or hn in alias):
                return h
    return None


def normalize_phone(raw: str) -> str:
    digits = "".join(ch for ch in raw if ch.isdigit())
    if len(digits) == 11 and digits[0] == "8":
        digits = "7" + digits[1:]
    if len(digits) == 10:
        digits = "7" + digits
    return digits


def cell(row: dict[str, str], col: str | None) -> str:
    if not col:
        return ""
    return (row.get(col) or "").strip()


def detect_dialect(sample: str) -> csv.Dialect:
    try:
        return csv.Sniffer().sniff(sample, delimiters=",;\t")
    except csv.Error:
        class Fallback(csv.Dialect):
            delimiter = ";" if sample.count(";") > sample.count(",") else ","
            quotechar = '"'
            doublequote = True
            skipinitialspace = True
            lineterminator = "\n"
            quoting = csv.QUOTE_MINIMAL

        return Fallback()


def load_csv(path: Path) -> tuple[list[str], list[dict[str, str]]]:
    raw = path.read_bytes()
    for enc in ("utf-8-sig", "utf-8", "cp1251"):
        try:
            text = raw.decode(enc)
            break
        except UnicodeDecodeError:
            text = ""
    else:
        raise SystemExit(f"cannot decode {path}")
    dialect = detect_dialect(text[:8192])
    reader = csv.DictReader(text.splitlines(), dialect=dialect)
    if not reader.fieldnames:
        raise SystemExit("csv has no header")
    headers = [h.strip() for h in reader.fieldnames if h and h.strip()]
    rows = []
    for row in reader:
        rows.append({(k or "").strip(): (v or "").strip() for k, v in row.items() if k})
    return headers, rows


def map_columns(headers: list[str]) -> dict[str, str | None]:
    return {
        "phone": pick(headers, PHONE_ALIASES),
        "web_id": pick(headers, WEB_ALIASES),
        "name": pick(headers, NAME_ALIASES),
        "title": pick(headers, TITLE_ALIASES),
        "email": pick(headers, EMAIL_ALIASES),
        "domain": pick(headers, DOMAIN_ALIASES),
        "time": pick(headers, TIME_ALIASES),
        "ip": pick(headers, IP_ALIASES),
        "city": pick(headers, CITY_ALIASES),
        "url": pick(headers, URL_ALIASES),
        "utm_source": pick(headers, UTM_SOURCE_ALIASES),
        "utm_medium": pick(headers, UTM_MEDIUM_ALIASES),
        "utm_campaign": pick(headers, UTM_CAMPAIGN_ALIASES),
        "utm_content": pick(headers, UTM_CONTENT_ALIASES),
        "utm_term": pick(headers, UTM_TERM_ALIASES),
        "comments": pick(headers, COMMENTS_ALIASES),
        "source_description": pick(headers, SOURCE_DESC_ALIASES),
        "tag": pick(headers, TAG_ALIASES),
    }


def build_comments(row: dict[str, str], cols: dict[str, str | None], phone: str) -> str:
    ready = cell(row, cols["comments"])
    if ready:
        return ready.replace("\n", "<br>")
    parts = [
        ("Имя", cell(row, cols["name"]) or "{name}"),
        ("Телефон", phone),
        ("Email", cell(row, cols["email"])),
        ("С сайта", cell(row, cols["domain"])),
        ("Время", cell(row, cols["time"])),
        ("IP-адрес", cell(row, cols["ip"])),
        ("Местоположение клиента", cell(row, cols["city"])),
        ("Адрес страницы", cell(row, cols["url"]) or cell(row, cols["domain"])),
        ("utm_source", cell(row, cols["utm_source"])),
        ("utm_medium", cell(row, cols["utm_medium"])),
        ("utm_campaign", cell(row, cols["utm_campaign"])),
        ("utm_content", cell(row, cols["utm_content"])),
        ("utm_term", cell(row, cols["utm_term"])),
    ]
    return "<br>".join(f"{k}: {v}" for k, v in parts)


def build_title(row: dict[str, str], cols: dict[str, str | None]) -> str:
    title = cell(row, cols["title"])
    if title:
        return title
    name = cell(row, cols["name"])
    if name:
        return name
    domain = cell(row, cols["domain"])
    if domain:
        return f"ГЦК. Сделка по проекту {domain}"
    return "сервис идентификаций"


def build_payload(row: dict[str, str], cols: dict[str, str | None], args: argparse.Namespace) -> dict | None:
    phone = normalize_phone(cell(row, cols["phone"]))
    if len(phone) != 11 or not phone.startswith("7"):
        return None
    web_id = cell(row, cols["web_id"])
    title = build_title(row, cols)
    fields: dict[str, object] = {
        "ASSIGNED_BY_ID": "1",
        "TITLE": title,
        "NAME": title,
        "PHONE": phone,
        "COMMENTS": build_comments(row, cols, phone),
        "SOURCE_DESCRIPTION": cell(row, cols["source_description"]) or args.source_description,
    }
    if web_id:
        fields[WEB_ID_FIELD] = web_id
    raw_tag = cell(row, cols["tag"]) or args.tag
    tag = TAG_MAP.get(raw_tag.lower(), raw_tag) if raw_tag else ""
    if tag:
        fields[TAG_FIELD] = tag
    return {"phone": phone, "web_id": web_id, "fields": fields}


def psql(query: str) -> list[list[str]]:
    env = os.environ.copy()
    env.setdefault("PGPASSWORD", env.get("DB_PASS", "admin_tmp_password"))
    cmd = [
        "psql",
        "-h", env.get("DB_HOST", "127.0.0.1"),
        "-p", env.get("DB_PORT", "5432"),
        "-U", env.get("DB_USER", "admin"),
        "-d", env.get("DB_NAME", "sdmp"),
        "-v", "ON_ERROR_STOP=1",
        "-At",
        "-F", "\t",
        "-c", query,
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True, env=env)
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)
    rows = []
    for line in proc.stdout.splitlines():
        if line:
            rows.append(line.split("\t"))
    return rows


def source_id_by_token(token: str) -> int:
    if not token.isalnum():
        raise SystemExit("token must be alphanumeric")
    rows = psql(f"SELECT id FROM source WHERE token = '{token}'")
    if not rows:
        raise SystemExit(f"source not found for token {token}")
    return int(rows[0][0])


def load_existing(source_id: int) -> tuple[set[str], set[str]]:
    rows = psql(
        f"""
        SELECT phone, COALESCE(web_id, '')
        FROM lead_row
        WHERE source_id = {int(source_id)}
          AND created_at >= date_trunc('day', now())
        """
    )
    phones: set[str] = set()
    webs: set[str] = set()
    for phone, web_id in rows:
        if phone:
            phones.add(phone)
        if web_id:
            webs.add(web_id)
    return phones, webs


def post_lead(base: str, token: str, fields: dict, timeout: float, retries: int = 5) -> tuple[int, str]:
    url = f"{base.rstrip('/')}/rest/1/{token}/crm.lead.add.json"
    body = json.dumps({"fields": fields}, ensure_ascii=False).encode("utf-8")
    last_err = ""
    for attempt in range(1, retries + 1):
        req = urllib.request.Request(
            url,
            data=body,
            method="POST",
            headers={"Content-Type": "application/json", "Accept": "application/json"},
        )
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return resp.status, resp.read().decode("utf-8", "replace")
        except urllib.error.HTTPError as exc:
            return exc.code, exc.read().decode("utf-8", "replace")
        except urllib.error.URLError as exc:
            last_err = str(exc)
            time.sleep(min(2 * attempt, 10))
    return 0, last_err


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Import BigData-IT CSV via crm.lead.add")
    p.add_argument("--csv", type=Path, default=DEFAULT_CSV, help="Путь к отчёту BigData")
    p.add_argument("--token", default=os.environ.get("BIGDATA_WEBHOOK_TOKEN", DEFAULT_TOKEN))
    p.add_argument("--base-url", default=os.environ.get("SDMP_BASE_URL", DEFAULT_BASE))
    p.add_argument("--tag", default=DEFAULT_TAG)
    p.add_argument("--source-description", default=DEFAULT_SOURCE_DESCRIPTION)
    p.add_argument("--sleep", type=float, default=0.05, help="Пауза между POST, сек")
    p.add_argument("--timeout", type=float, default=30)
    p.add_argument("--limit", type=int, default=0, help="Максимум новых строк (0 = все)")
    p.add_argument("--write-pending", type=Path, help="Записать CSV без уже принятых")
    p.add_argument("--apply", action="store_true", help="Реально слать на API. Без флага — только отчёт")
    return p.parse_args()


def main() -> None:
    args = parse_args()
    if not args.csv.is_file():
        raise SystemExit(f"csv not found: {args.csv}")

    headers, rows = load_csv(args.csv)
    cols = map_columns(headers)
    print("csv", args.csv, "rows", len(rows))
    print("headers", headers)
    print("mapped", {k: v for k, v in cols.items() if v})
    if not cols["phone"]:
        raise SystemExit("phone column not found — поправь алиасы в скрипте")

    source_id = source_id_by_token(args.token)
    existing_phones, existing_webs = load_existing(source_id)
    print("source_id", source_id, "today phones", len(existing_phones), "today web_id", len(existing_webs))

    pending_rows: list[dict[str, str]] = []
    payloads: list[dict] = []
    stats: Counter[str] = Counter()
    seen_phone: set[str] = set()
    seen_web: set[str] = set()

    for row in rows:
        stats["csv"] += 1
        built = build_payload(row, cols, args)
        if built is None:
            stats["bad_phone"] += 1
            continue
        phone = built["phone"]
        web_id = built["web_id"]
        if web_id and (web_id in existing_webs or web_id in seen_web):
            stats["already_web"] += 1
            continue
        if phone in existing_phones or phone in seen_phone:
            stats["already_phone"] += 1
            continue
        seen_phone.add(phone)
        if web_id:
            seen_web.add(web_id)
        pending_rows.append(row)
        payloads.append(built)
        if args.limit and len(payloads) >= args.limit:
            break

    print(
        "pending", len(payloads),
        "already_phone", stats["already_phone"],
        "already_web", stats["already_web"],
        "bad_phone", stats["bad_phone"],
    )
    if payloads:
        print("sample payload")
        print(json.dumps({"fields": payloads[0]["fields"]}, ensure_ascii=False, indent=2))

    if args.write_pending:
        args.write_pending.parent.mkdir(parents=True, exist_ok=True)
        with args.write_pending.open("w", encoding="utf-8", newline="") as f:
            w = csv.DictWriter(f, fieldnames=headers, extrasaction="ignore")
            w.writeheader()
            w.writerows(pending_rows)
        print("wrote pending", args.write_pending, "rows", len(pending_rows))

    if not args.apply:
        print("dry-run, pass --apply to POST")
        return

    ok = fail = 0
    for i, item in enumerate(payloads, 1):
        status, body = post_lead(args.base_url, args.token, item["fields"], args.timeout)
        if 200 <= status < 300:
            ok += 1
        else:
            fail += 1
            print(f"FAIL {i}/{len(payloads)} phone={item['phone']} status={status} body={body[:300]}", file=sys.stderr)
        if i % 50 == 0:
            print(f"sent {i}/{len(payloads)} ok={ok} fail={fail}", flush=True)
        if args.sleep > 0:
            time.sleep(args.sleep)
    print(f"done ok={ok} fail={fail}")
    if fail:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
