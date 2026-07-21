#!/usr/bin/env python3
"""Inspect the locally decoded GBFR 2.0.2 tables without mutating them."""

from __future__ import annotations

import argparse
import sqlite3
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("database", type=Path)
    parser.add_argument("--schema", action="store_true")
    parser.add_argument("--sample", type=int, default=0)
    args = parser.parse_args()

    connection = sqlite3.connect(f"file:{args.database.resolve()}?mode=ro", uri=True)
    names = [
        row[0]
        for row in connection.execute(
            """
            SELECT name
            FROM sqlite_master
            WHERE type = 'table'
              AND (
                lower(name) LIKE '%gem%'
                OR lower(name) LIKE '%skill%'
                OR lower(name) LIKE '%lot%'
              )
            ORDER BY name
            """
        )
    ]
    for name in names:
        count = connection.execute(f'SELECT COUNT(*) FROM "{name}"').fetchone()[0]
        print(f"[{name}] rows={count}")
        if args.schema:
            columns = connection.execute(f'PRAGMA table_info("{name}")').fetchall()
            print("  columns=" + ", ".join(f"{column[1]}:{column[2]}" for column in columns))
        if args.sample:
            for row in connection.execute(f'SELECT * FROM "{name}" LIMIT ?', (args.sample,)):
                print(f"  {row!r}")


if __name__ == "__main__":
    main()
