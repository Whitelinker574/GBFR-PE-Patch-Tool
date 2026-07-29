#!/usr/bin/env python3
"""Build the bilingual Endless/Infinity rule catalog from GBFR 2.0.2 data."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sqlite3
from pathlib import Path

try:
    import msgpack
except ImportError as exc:  # pragma: no cover
    raise SystemExit("msgpack is required: python -m pip install msgpack") from exc


PLACEHOLDER = re.compile(r"\{(\d+)(?::([^}]+))?\}")
MARKUP = re.compile(r"<[^>]*>")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def messages(path: Path) -> dict[str, str]:
    payload = msgpack.unpackb(path.read_bytes(), raw=False, strict_map_key=False)
    result = {}
    for row in payload.get("rows_", []):
        column = row.get("column_", {})
        key = str(column.get("id_hash_", ""))
        if key:
            result[key] = str(column.get("text_", ""))
    return result


def format_number(value: float, one_decimal: bool = False) -> str:
    if one_decimal:
        return f"{value:.1f}"
    if value.is_integer():
        return str(int(value))
    return f"{value:g}"


def render(text: str, values: dict[int, float]) -> str:
    def replace(match: re.Match[str]) -> str:
        index = int(match.group(1))
        if index not in values:
            return match.group(0)
        spec = (match.group(2) or "").strip()
        value = values[index]
        one_decimal = ".1f" in spec
        if spec and not any(char in spec for char in ".fFeEgG"):
            try:
                scale = float(spec)
                if scale > 0:
                    value *= scale
            except ValueError:
                pass
        return format_number(value, one_decimal)

    return MARKUP.sub("", PLACEHOLDER.sub(replace, text)).strip()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--sqlite", required=True, type=Path)
    parser.add_argument("--tables-dir", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    sqlite_path = args.sqlite.resolve()
    tables_dir = args.tables_dir.resolve()
    sources = {
        "ruleTable": tables_dir / "infinity_rule.tbl",
        "effectTable": tables_dir / "infinity_rule_effect.tbl",
        "difficultyTable": tables_dir / "endlessmode_difficulty.tbl",
        "textZh": tables_dir / "text" / "cs" / "text_stage.msg",
        "textEn": tables_dir / "text" / "en" / "text_stage.msg",
    }
    missing = [str(path) for path in [sqlite_path, *sources.values()] if not path.is_file()]
    if missing:
        raise SystemExit("missing inputs:\n" + "\n".join(missing))

    zh = messages(sources["textZh"])
    en = messages(sources["textEn"])
    connection = sqlite3.connect(f"file:{sqlite_path.as_posix()}?mode=ro", uri=True)
    connection.row_factory = sqlite3.Row
    try:
        effects = {
            str(row["Key"]): {"id": int(row["Id"]), "value": float(row["Value"])}
            for row in connection.execute("SELECT Key, Id, Value FROM infinity_rule_effect")
        }
        rules = []
        for row in connection.execute("SELECT * FROM infinity_rule ORDER BY QuestId, EffectName"):
            refs = []
            values: dict[int, float] = {}
            for index in range(1, 11):
                key = str(row[f"InfinityRuleEffectId{index}"] or "")
                if not key:
                    continue
                effect = effects.get(key)
                if effect is None:
                    raise SystemExit(f"missing infinity effect {key}")
                refs.append({"key": key, **effect})
                # The effect Id is a global effect-kind enum. Localized text
                # placeholders follow the populated EffectId1..10 slot order.
                values[len(refs) - 1] = float(effect["value"])
            name_key = str(row["EffectName"])
            description_key = str(row["EffectDescription"])
            rules.append(
                {
                    "questId": str(row["QuestId"]),
                    "nameKey": name_key,
                    "descriptionKey": description_key,
                    "nameZh": zh.get(name_key, name_key),
                    "nameEn": en.get(name_key, name_key),
                    "descriptionZh": render(zh.get(description_key, description_key), values),
                    "descriptionEn": render(en.get(description_key, description_key), values),
                    "effects": refs,
                }
            )
        difficulties = [dict(row) for row in connection.execute("SELECT * FROM endlessmode_difficulty ORDER BY SortOrder")]
    finally:
        connection.close()

    if len(rules) != 25 or len(difficulties) != 5:
        raise SystemExit(f"unexpected infinity catalog shape: rules={len(rules)} difficulties={len(difficulties)}")
    unresolved = [row["nameKey"] for row in rules if row["nameZh"] == row["nameKey"] or row["nameEn"] == row["nameKey"]]
    if unresolved:
        raise SystemExit("missing localized infinity names: " + ", ".join(unresolved))

    payload = {
        "schemaVersion": 1,
        "dataVersion": args.version,
        "sources": {
            "sqlite": {"file": sqlite_path.name, "sha256": sha256(sqlite_path)},
            **{
                name: {"file": path.relative_to(tables_dir).as_posix(), "sha256": sha256(path)}
                for name, path in sources.items()
            },
        },
        "rules": rules,
        "difficulties": difficulties,
        "interpretation": "Effect id/value pairs are raw table parameters. Only localized placeholders are substituted; unknown field semantics remain unnamed.",
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {len(rules)} rules and {len(difficulties)} difficulty rows to {args.output}")


if __name__ == "__main__":
    main()
