#!/usr/bin/env python3
"""Load cold/base labels and child parent_id from «Источники Битрикс.xlsx»."""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import zipfile
import xml.etree.ElementTree as ET
from pathlib import Path

NS = {"m": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}


def col_row(ref: str) -> tuple[int, int]:
    m = re.match(r"([A-Z]+)(\d+)", ref)
    if not m:
        raise ValueError(ref)
    n = 0
    for ch in m.group(1):
        n = n * 26 + (ord(ch) - 64)
    return n - 1, int(m.group(2)) - 1


def bitrix_code(raw: str) -> str:
    raw = (raw or "").strip()
    if not raw:
        return ""
    first = raw.split()[0]
    if "|" in first:
        first = first.split("|", 1)[0]
    return first.strip()


def load_sheet(z: zipfile.ZipFile, strings: list[str], name: str) -> list[list[str]]:
    xml = ET.fromstring(z.read(name))
    grid: dict[tuple[int, int], str] = {}
    max_r = max_c = 0
    for c in xml.findall(".//m:c", NS):
        ref = c.get("r")
        if not ref:
            continue
        ci, ri = col_row(ref)
        t = c.get("t")
        v = c.find("m:v", NS)
        is_el = c.find("m:is", NS)
        val = ""
        if t == "s" and v is not None and v.text is not None:
            val = strings[int(v.text)]
        elif t == "inlineStr" and is_el is not None:
            val = "".join(x.text or "" for x in is_el.findall(".//m:t", NS))
        elif v is not None and v.text is not None:
            val = v.text
        grid[(ri, ci)] = val
        max_r = max(max_r, ri)
        max_c = max(max_c, ci)
    return [[grid.get((r, c), "") for c in range(max_c + 1)] for r in range(max_r + 1)]


def parse_xlsx(path: Path) -> tuple[list[tuple[str, str, str]], list[tuple[str, str]]]:
    with zipfile.ZipFile(path) as z:
        root = ET.fromstring(z.read("xl/sharedStrings.xml"))
        strings = ["".join(t.text or "" for t in si.findall(".//m:t", NS)) for si in root.findall("m:si", NS)]
        labels_sheet = load_sheet(z, strings, "xl/worksheets/sheet1.xml")
        map_sheet = load_sheet(z, strings, "xl/worksheets/sheet2.xml")

    labels: list[tuple[str, str, str]] = []
    for row in labels_sheet[1:]:
        code = bitrix_code(row[0] if row else "")
        if not code:
            continue
        cold = (row[2] if len(row) > 2 else "").strip()
        base = (row[3] if len(row) > 3 else "").strip()
        if cold or base:
            labels.append((code, cold, base))

    children: list[tuple[str, str]] = []
    parent = ""
    for row in map_sheet[1:]:
        src = bitrix_code(row[1] if len(row) > 1 else "")
        child = bitrix_code(row[2] if len(row) > 2 else "")
        if src:
            parent = src
        if parent and child:
            children.append((parent, child))
    return labels, children


def sql_literal(s: str) -> str:
    return "'" + s.replace("'", "''") + "'"


def build_sql(labels: list[tuple[str, str, str]], children: list[tuple[str, str]]) -> str:
    parts = [
        "-- Метки и иерархия из «Источники Битрикс.xlsx».",
        "-- Холодная база / Какая база → cold_label / base_label по bitrix_code.",
        "-- Маппинг: колонка «Источник» = родитель, «Подисточник» = child.parent_id.",
        "",
    ]
    for code, cold, base in labels:
        parts.append(
            "UPDATE source\n"
            f"SET cold_label = {sql_literal(cold) if cold else 'NULL'},\n"
            f"    base_label = {sql_literal(base) if base else 'NULL'}\n"
            f"WHERE bitrix_code = {sql_literal(code)}\n"
            f"  AND (\n"
            f"    COALESCE(cold_label, '') IS DISTINCT FROM {sql_literal(cold)}\n"
            f"    OR COALESCE(base_label, '') IS DISTINCT FROM {sql_literal(base)}\n"
            f"  );"
        )
    parts.append("")
    for parent, child in children:
        parts.append(
            "UPDATE source AS child\n"
            "SET parent_id = parent.id\n"
            "FROM source AS parent\n"
            f"WHERE child.bitrix_code = {sql_literal(child)}\n"
            f"  AND parent.bitrix_code = {sql_literal(parent)}\n"
            "  AND child.id <> parent.id\n"
            "  AND child.parent_id IS DISTINCT FROM parent.id;"
        )
    parts.append("")
    return "\n".join(parts) + "\n"


def load_env(path: Path) -> dict[str, str]:
    env = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        k, v = line.split("=", 1)
        env[k.strip()] = v.strip().strip('"').strip("'")
    return env


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--xlsx", default="/root/projects/Источники Битрикс.xlsx")
    ap.add_argument("--sql-out", default="/root/projects/sdmp/migrate/20260903-source-xlsx-labels.sql")
    ap.add_argument("--apply", action="store_true")
    args = ap.parse_args()

    labels, children = parse_xlsx(Path(args.xlsx))
    sql = build_sql(labels, children)
    out = Path(args.sql_out)
    out.write_text(sql)
    print(f"labels={len(labels)} children={len(children)} wrote {out}")

    if not args.apply:
        return

    env = load_env(Path("/root/projects/sdmp/config/.env"))
    pg_env = os.environ.copy()
    pg_env["PGPASSWORD"] = env.get("DB_PASS", "")
    r = subprocess.run(
        [
            "psql",
            "-h", env.get("DB_HOST", "127.0.0.1"),
            "-p", env.get("DB_PORT", "5432"),
            "-U", env.get("DB_USER", "admin"),
            "-d", env.get("DB_NAME", "sdmp"),
            "-v", "ON_ERROR_STOP=1",
            "-f", str(out),
        ],
        env=pg_env,
        check=False,
        capture_output=True,
        text=True,
    )
    print(r.stdout)
    if r.returncode != 0:
        raise SystemExit(r.stderr or f"psql exit {r.returncode}")


if __name__ == "__main__":
    main()
