#!/usr/bin/env python3
"""Build the checked-in GBFR 2.0.2 combat-reference catalog."""

from __future__ import annotations

import argparse
import hashlib
import json
import sqlite3
from pathlib import Path

try:
    import msgpack
except ImportError as exc:  # pragma: no cover - maintainer dependency guard
    raise SystemExit("msgpack is required: python -m pip install msgpack") from exc


CURVE_FILES = (
    "enmity.f-curve.msg",
    "enmity2.f-curve.msg",
    "garrison.f-curve.msg",
    "linktime.f-curve.msg",
    "stamina.f-curve.msg",
    "stamina2.f-curve.msg",
    "sturdy.f-curve.msg",
)

DAMAGE_FIELDS = (
    "criticalDamageUpperRate_",
    "superArmorDamageRate_",
    "atkTypeDamageLimit_Normal_",
    "atkTypeDamageLimit_Ability_",
    "atkTypeDamageLimit_SpArts_",
    "chainBurstDamageLimit_",
    "weakElementAddDamageRate_",
    "addDamageLimitBonusStatusRate_",
    "autoReviveMaxCount_",
    "autoReviveHpRate_",
    "autoReviveCoolTime_",
    "gutsMaxCount_",
    "gutsInvinsibleSec_",
    "gutsCoolTime_",
)

GUARD_FIELDS = (
    "GuardGageMax",
    "GuardStopHealSec",
    "DamageStopHealSec",
    "GuardPointStopHealSec",
    "HelpGuardStopHealSec",
    "GuardBreakSec",
    "GuardAutoHealValue",
    "GuardDamageCutRate",
    "GuardBreakDamageCutRate",
    "HelpGuardBreakGageRate",
    "GuardPointBreakGageRate",
    "JustGuardAcceptFrame",
    "ChargeParryAcceptFrame",
    "ChargeParryInvinsbleTime",
    "JustGuardAttackBreakRate",
    "guardFailedPenaltyTime",
)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def unpack(path: Path) -> dict:
    return msgpack.unpackb(path.read_bytes(), raw=False, strict_map_key=False)


def number(value: object) -> int | float:
    parsed = float(str(value))
    return int(parsed) if parsed.is_integer() else parsed


def source(path: Path, root: Path) -> dict[str, str]:
    try:
        name = path.relative_to(root).as_posix()
    except ValueError:
        name = path.name
    return {"file": name, "sha256": sha256(path)}


def curve_rows(path: Path) -> list[dict[str, object]]:
    payload = unpack(path)
    rows = payload.get("Array", [])
    expected = int(payload.get("Count", len(rows)))
    if len(rows) != expected:
        raise SystemExit(f"curve row count mismatch: {path}: {len(rows)} != {expected}")
    result = []
    for row in rows:
        if len(row) != 5:
            raise SystemExit(f"unexpected curve row: {path}: {row!r}")
        result.append(
            {
                "interpolation": str(row[0]),
                "x": number(row[1]),
                "y": number(row[2]),
                "leftTangent": number(row[3]),
                "rightTangent": number(row[4]),
            }
        )
    return result


def load_damage_curves(connection: sqlite3.Connection, table: str) -> dict[str, list[dict[str, int | float]]]:
    result: dict[str, list[dict[str, int | float]]] = {}
    for character, attack_rate, damage_cap in connection.execute(
        f"SELECT Character, AttackRate, DamageCap FROM {table} ORDER BY Character, AttackRate"
    ):
        result.setdefault(str(character), []).append(
            {"attackRate": number(attack_rate), "damageCap": number(damage_cap)}
        )
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--sqlite", required=True, type=Path)
    parser.add_argument("--runtime-dir", required=True, type=Path)
    parser.add_argument("--tables-dir", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    sqlite_path = args.sqlite.resolve()
    runtime_dir = args.runtime_dir.resolve()
    tables_dir = args.tables_dir.resolve()
    damage_path = runtime_dir / "system" / "player" / "damagecalcparam.msg"
    guard_path = runtime_dir / "system" / "player" / "guardparam.msg"
    curve_paths = {name.removesuffix(".f-curve.msg"): runtime_dir / "curve" / "battle" / name for name in CURVE_FILES}
    table_paths = {
        "characterDamageLimitTable": tables_dir / "chara_damage_limit.tbl",
        "characterArtsDamageLimitTable": tables_dir / "chara_arts_damage_limit.tbl",
    }
    inputs = [sqlite_path, damage_path, guard_path, *curve_paths.values(), *table_paths.values()]
    missing = [str(path) for path in inputs if not path.is_file()]
    if missing:
        raise SystemExit("missing inputs:\n" + "\n".join(missing))

    raw_damage = unpack(damage_path).get("DamageCalculateParam", {})
    raw_guard = unpack(guard_path).get("GuardParam", {})
    damage = {key.removesuffix("_"): number(raw_damage[key]) for key in DAMAGE_FIELDS}
    guard = {key: number(raw_guard[key]) for key in GUARD_FIELDS}

    connection = sqlite3.connect(f"file:{sqlite_path.as_posix()}?mode=ro", uri=True)
    try:
        normal = load_damage_curves(connection, "chara_damage_limit")
        arts = load_damage_curves(connection, "chara_arts_damage_limit")
    finally:
        connection.close()
    normal_rows = sum(len(rows) for rows in normal.values())
    arts_rows = sum(len(rows) for rows in arts.values())
    if normal_rows != 975 or arts_rows != 930:
        raise SystemExit(f"unexpected damage-limit shape: normal={normal_rows}, arts={arts_rows}")

    payload = {
        "schemaVersion": 1,
        "dataVersion": args.version,
        "sources": {
            "sqlite": source(sqlite_path, tables_dir),
            "damageCalculateParam": source(damage_path, runtime_dir),
            "guardParam": source(guard_path, runtime_dir),
            **{f"curve_{name}": source(path, runtime_dir) for name, path in curve_paths.items()},
            **{name: source(path, tables_dir) for name, path in table_paths.items()},
        },
        "damageCalculate": damage,
        "guard": guard,
        "conditionalCurves": {name: curve_rows(path) for name, path in curve_paths.items()},
        "damageLimits": {
            "normalRowCount": normal_rows,
            "artsRowCount": arts_rows,
            "normal": normal,
            "arts": arts,
        },
        "interpretation": {
            "globalAttackTypeLimits": "Raw global baseline constants; not final per-action caps.",
            "characterCurves": "Raw table nodes keyed by game character identifier.",
            "conditionalCurves": "Raw control points. Smooth and SmoothSide interpolation is intentionally not approximated.",
        },
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(
        f"wrote {normal_rows} normal rows, {arts_rows} arts rows and "
        f"{len(curve_paths)} conditional curves to {args.output}"
    )


if __name__ == "__main__":
    main()
