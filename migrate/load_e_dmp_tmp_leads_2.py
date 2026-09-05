#!/usr/bin/env python3
"""Stream MySQL dump INSERTs into Postgres COPY. Does not load the 6GB file into memory."""

from __future__ import annotations

import argparse
import csv
import io
import os
import subprocess
import sys

DUMP = "/root/projects/sdmp/migrate/e_dmp_tmp_leads_2.sql"
NCOLS = 28
TS_COLS = {7, 12, 17}  # date_create, call_last_at, date_modify
ESC = {
    "n": "\n",
    "r": "\r",
    "t": "\t",
    "0": "\0",
    "b": "\b",
    "Z": "\x1a",
    "\\": "\\",
    "'": "'",
    '"': '"',
}

COPY_SQL = """COPY e_dmp_tmp_leads_2 (
  phone, source_id, region, operator, city, name, owner_id, date_create,
  web_id, tag, call_id, call_url, call_last_at, project_num, call_form,
  dedup_last_result, status_id, date_modify, operator_id_raw, city_alt,
  cold_label, base_label, upk, partner_sent_at, title, comments, address,
  legacy_fields
) FROM STDIN WITH (FORMAT csv, NULL '\\N', ENCODING 'UTF8')"""


def refill(f, buf: str, i: int, keep: int = 1_000_000) -> tuple[str, int]:
    if i < len(buf) - 256_000:
        return buf, i
    buf = buf[i:]
    chunk = f.read(8_000_000)
    if chunk:
        buf += chunk
    return buf, 0


def parse_string(buf: str, i: int) -> tuple[str, int]:
    # buf[i] is the opening quote
    i += 1
    parts: list[str] = []
    start = i
    while i < len(buf):
        c = buf[i]
        if c == "\\":
            parts.append(buf[start:i])
            i += 1
            if i >= len(buf):
                raise ValueError("truncated escape")
            parts.append(ESC.get(buf[i], buf[i]))
            i += 1
            start = i
            continue
        if c == "'":
            if i + 1 < len(buf) and buf[i + 1] == "'":
                parts.append(buf[start:i])
                parts.append("'")
                i += 2
                start = i
                continue
            parts.append(buf[start:i])
            return "".join(parts), i + 1
        i += 1
    raise ValueError("unterminated string")


def parse_row(buf: str, i: int) -> tuple[list[str | None], int]:
    if buf[i] != "(":
        raise ValueError(f"expected '(' at {i}: {buf[i:i+40]!r}")
    i += 1
    fields: list[str | None] = []
    while True:
        while i < len(buf) and buf[i] in " \t\n\r":
            i += 1
        if i >= len(buf):
            raise ValueError("truncated row")
        if buf.startswith("NULL", i) and (i + 4 == len(buf) or not buf[i + 4].isalnum()):
            fields.append(None)
            i += 4
        elif buf[i] == "'":
            val, i = parse_string(buf, i)
            fields.append(val)
        else:
            start = i
            while i < len(buf) and buf[i] not in ",)":
                i += 1
            token = buf[start:i].strip()
            fields.append(None if token == "NULL" else token)
        while i < len(buf) and buf[i] in " \t\n\r":
            i += 1
        if i >= len(buf):
            raise ValueError("truncated row after field")
        if buf[i] == ",":
            i += 1
            continue
        if buf[i] == ")":
            i += 1
            break
        raise ValueError(f"expected ',' or ')' got {buf[i:i+20]!r}")
    if len(fields) != NCOLS:
        raise ValueError(f"expected {NCOLS} cols, got {len(fields)}")
    return fields, i


def empty_ts(val: str | None) -> str | None:
    if val is None:
        return None
    if val.strip() in ("", "0000-00-00 00:00:00", "0000-00-00"):
        return None
    return val


def write_row(w: csv.writer, fields: list[str | None]) -> None:
    out: list[str] = []
    for idx, val in enumerate(fields):
        if idx in TS_COLS:
            val = empty_ts(val)
        out.append("\\N" if val is None else val)
    w.writerow(out)


def open_copy() -> subprocess.Popen:
    env = os.environ.copy()
    env["PGPASSWORD"] = os.environ.get("PGPASSWORD", "admin_tmp_password")
    return subprocess.Popen(
        [
            "psql",
            "-h", os.environ.get("PGHOST", "127.0.0.1"),
            "-U", os.environ.get("PGUSER", "admin"),
            "-d", os.environ.get("PGDATABASE", "sdmp"),
            "-v", "ON_ERROR_STOP=1",
            "-c", COPY_SQL,
        ],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=env,
    )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dump", default=DUMP)
    ap.add_argument("--limit", type=int, default=0)
    args = ap.parse_args()

    proc = open_copy()
    assert proc.stdin is not None
    text = io.TextIOWrapper(proc.stdin, encoding="utf-8", newline="")
    writer = csv.writer(text, lineterminator="\n")

    n = 0
    with open(args.dump, "r", encoding="utf-8", errors="replace") as f:
        buf = f.read(8_000_000)
        i = 0
        while True:
            buf, i = refill(f, buf, i)
            if i >= len(buf):
                break
            if buf[i] != "(":
                pos = buf.find("VALUES", i)
                if pos == -1:
                    nxt = buf.find("(", i)
                    if nxt == -1:
                        if len(buf) - i < 16 and not f.read(1):
                            break
                        # skip junk until next INSERT or EOF
                        more = f.read(8_000_000)
                        if not more:
                            break
                        buf = buf[i:] + more
                        i = 0
                        continue
                    # only treat '(' as row start after VALUES
                    values = buf.rfind("VALUES", 0, nxt)
                    if values == -1:
                        i = nxt + 1
                        continue
                    i = nxt
                    continue
                i = pos + 6
                while i < len(buf) and buf[i] in " \t\n\r":
                    i += 1
                continue
            try:
                fields, i = parse_row(buf, i)
            except ValueError:
                buf, i = refill(f, buf, i)
                fields, i = parse_row(buf, i)
            write_row(writer, fields)
            n += 1
            if n % 200_000 == 0:
                print(f"copied {n}", file=sys.stderr, flush=True)
            if args.limit and n >= args.limit:
                break
            while i < len(buf) and buf[i] in " \t\n\r,":
                i += 1
            if i < len(buf) and buf[i] == ";":
                i += 1

    text.flush()
    text.detach()
    stdout, stderr = proc.communicate()
    if proc.returncode != 0:
        sys.stderr.write(stderr.decode("utf-8", "replace"))
        sys.stderr.write(stdout.decode("utf-8", "replace"))
        return proc.returncode
    print(f"ok rows={n}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
