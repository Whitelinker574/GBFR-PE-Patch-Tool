"""Generate the 2.0.2 permanent panel-stat catalog from unpacked TBL SQLite."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sqlite3
from collections import defaultdict
from pathlib import Path


PANEL_TYPES = {0: "attack", 1: "hp", 2: "critRate", 3: "stunRaw"}
WEAPON_ID = re.compile(r"^(WEP_PL\d{4}_\d{2})")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest().upper()


def effect_rows(connection: sqlite3.Connection, table: str, owner: str):
    query = f"""
        SELECT a.WeaponId, a.ReqWepLevel, a.ReqWepUncapLevel,
               a.ReqWepAwakeLevel, a.ReqWepTranscensionLevel,
               a.MspCost, a.LimitBonusParamIndex,
               p.Key, p.Lv1Value, p.Lv2Value, p.Lv3Value, p.Lv4Value,
               p.Lv5Value, p.Lv6Value, p.Lv7Value, p.Lv8Value,
               p.Lv9Value, p.Lv10Value, p.NameFormat,
               p.DisplayNumberMultiplier
          FROM {table} a
          JOIN limit_bonus b ON b.Key = a.LimitBonusId
          JOIN limit_bonus_param p ON p.Key IN (b.ParamId1, b.ParamId2, b.ParamId3)
         WHERE a.CharaId = ?
    """
    return connection.execute(query, (owner,)).fetchall()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--db", type=Path, required=True)
    parser.add_argument("--tbl-dir", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--dataset-version", required=True)
    args = parser.parse_args()

    connection = sqlite3.connect(f"file:{args.db.resolve()}?mode=ro", uri=True)
    owners = [row[0] for row in connection.execute("SELECT DISTINCT CharaId FROM ap_tree_atk ORDER BY CharaId")]
    characters: dict[str, object] = {}
    for owner in owners:
        fixed: dict[str, object] = {}
        for table, label in (("ap_tree_atk", "attack"), ("ap_tree_def", "defense")):
            totals: dict[str, float] = defaultdict(float)
            for row in effect_rows(connection, table, owner):
                param_index = int(row[6])
                if not 0 <= param_index <= 9:
                    raise ValueError(f"invalid LimitBonusParamIndex {param_index} for {owner}/{table}")
                # In the unpacked 2.0.2 limit_bonus_param table this field is
                # the 0..3 panel-stat kind consumed by PANEL_TYPES.
                param_type = int(row[19])
                if param_type not in PANEL_TYPES:
                    continue
                totals[PANEL_TYPES[param_type]] += float(row[8 + param_index])
            full_msp = int(connection.execute(
                f"SELECT COALESCE(SUM(MspCost), 0) FROM {table} WHERE CharaId = ?", (owner,)
            ).fetchone()[0])
            fixed[label] = {"fullMsp": full_msp, **dict(sorted(totals.items()))}

        nodes: list[dict[str, object]] = []
        for table, source in (("ap_tree_wep", "collection"), ("ap_tree_rebuild", "transcendence")):
            for row in effect_rows(connection, table, owner):
                param_index = int(row[6])
                if not 0 <= param_index <= 9:
                    raise ValueError(f"invalid LimitBonusParamIndex {param_index} for {owner}/{table}")
                param_type = int(row[19])
                if param_type not in PANEL_TYPES:
                    continue
                match = WEAPON_ID.match(str(row[0]))
                if match is None:
                    continue
                value = float(row[8 + param_index])
                # MAX_PARAM rows expose the resulting total in Lv1 and the
                # newly gained amount in Lv10. Runtime crit 108 proves that
                # only the delta enters the permanent panel aggregation.
                if str(row[18]).startswith("TXT_LBP_INFO_MAX_"):
                    value = float(row[17])
                nodes.append({
                    "source": source,
                    "weaponId": match.group(1),
                    "effect": PANEL_TYPES[param_type],
                    "value": value,
                    "level": int(row[1]),
                    "uncap": int(row[2]),
                    "awakening": int(row[3]),
                    "transcendence": int(row[4]),
                    "mspCost": int(row[5]),
                    "paramKey": str(row[7]),
                })
        nodes.sort(key=lambda node: (
            node["source"], node["weaponId"], node["transcendence"],
            node["level"], node["effect"], node["paramKey"],
        ))
        characters[owner] = {**fixed, "weaponNodes": nodes}

    source_files = [
        "ap_tree_atk.tbl", "ap_tree_def.tbl", "ap_tree_wep.tbl",
        "ap_tree_rebuild.tbl", "limit_bonus.tbl", "limit_bonus_param.tbl",
    ]
    payload = {
        "version": args.dataset_version,
        "gameVersion": "2.0.2",
        "units": {
            "stunRawToPanel": 10,
            "0": "flat attack", "1": "flat HP", "2": "panel crit percent", "3": "raw stun",
        },
        "sources": {name: sha256(args.tbl_dir / name) for name in source_files},
        "characters": characters,
    }
    args.output.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
