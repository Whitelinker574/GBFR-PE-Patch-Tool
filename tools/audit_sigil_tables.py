#!/usr/bin/env python3
"""Compare the sigil catalog with freshly decoded GBFR game tables.

The audit treats gem.tbl as the authority for a sigil's fixed primary trait and
for whether its secondary trait is fixed, random-lot-backed, or absent.  Random
pools are resolved through skill_type_lot.tbl and skill_lot.tbl.  The tool is
read-only unless --output or --fix-catalog is supplied.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sqlite3
from collections import defaultdict
from pathlib import Path


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest().upper()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("database", type=Path)
    parser.add_argument("catalog", type=Path)
    parser.add_argument("--output", type=Path, help="write the audit JSON instead of printing it")
    parser.add_argument(
        "--raw-table-dir",
        type=Path,
        help="include SHA-256 values for the four decoded source tables",
    )
    parser.add_argument(
        "--fix-catalog",
        action="store_true",
        help="rebuild sigils.json and the adjacent traits.json from the decoded tables",
    )
    args = parser.parse_args()

    connection = sqlite3.connect(f"file:{args.database.resolve()}?mode=ro", uri=True)
    connection.row_factory = sqlite3.Row

    skill_lots: dict[str, set[str]] = defaultdict(set)
    for row in connection.execute("SELECT Key, SkillId FROM skill_lot"):
        if row["Key"] and row["SkillId"]:
            skill_lots[row["Key"]].add(row["SkillId"])

    type_lots: dict[int, set[str]] = {}
    for row in connection.execute("SELECT * FROM skill_type_lot"):
        pool: set[str] = set()
        for index in range(1, 7):
            lot = row[f"SkillLotId{index}"]
            chance = row[f"ChancePercent{index}"]
            if lot and chance > 0:
                pool.update(skill_lots.get(lot, set()))
        type_lots[row["Key"]] = pool

    gems = {row["Key"]: row for row in connection.execute("SELECT * FROM gem")}
    payload = json.loads(args.catalog.read_text(encoding="utf-8-sig"))
    traits_path = args.catalog.with_name("traits.json")
    traits_payload = json.loads(traits_path.read_text(encoding="utf-8-sig"))
    trait_names = {
        row["internalId"]: row["displayName"]
        for row in traits_payload["traits"]
    }

    primary_traits = {row["SkillId1"] for row in gems.values() if row["SkillId1"]}
    secondary_traits = set().union(*type_lots.values())
    secondary_traits.update(row["SkillId2"] for row in gems.values() if row["SkillId2"])
    status_max = {
        row["Key"]: row["MaxLevel"]
        for row in connection.execute(
            "SELECT Key, MAX(Level) AS MaxLevel FROM skill_status GROUP BY Key"
        )
    }

    trait_rows = []
    for trait in traits_payload["traits"]:
        internal_id = trait["internalId"]
        hash_key = trait["hash"].removeprefix("0x").upper()
        game_max = status_max.get(internal_id, status_max.get(hash_key))
        issues = []
        if game_max is None:
            issues.append("missing-from-skill_status.tbl")
        if trait.get("maxLevel") != game_max:
            issues.append("effect-curve-max-mismatch")
        trait_rows.append(
            {
                "internalId": internal_id,
                "hash": trait["hash"],
                "displayName": trait["displayName"],
                "status": "verified" if not issues else "mismatch",
                "issues": issues,
                "catalogMaxLevel": trait.get("maxLevel"),
                "gameEffectCurveMaxLevel": game_max,
                "appearsAsPrimaryInGemTable": internal_id in primary_traits,
                "appearsAsSecondaryInGemLotTables": internal_id in secondary_traits,
            }
        )

    if args.fix_catalog:
        fixed_sigils = []
        for sigil in payload["sigils"]:
            game = gems.get(sigil["internalId"])
            if game is None:
                continue
            if game["SkillId2"]:
                mode = "fixed"
                pool = {game["SkillId2"]}
            elif game["SkillTypeLotIdForRandom2ndSkill"] >= 0:
                mode = "random"
                pool = type_lots[game["SkillTypeLotIdForRandom2ndSkill"]]
            else:
                mode = "none"
                pool = set()
            sigil["primaryTraitId"] = game["SkillId1"]
            sigil["supportsSecondaryTrait"] = bool(pool)
            sigil["allowedSecondaryTraitIds"] = sorted(pool)
            if sigil["internalId"] != "GEEN_142_02":
                sigil["allowedSigilLevels"] = [15]
                sigil["defaultSigilLevel"] = 15
                sigil["maxSigilLevel"] = 15
                sigil["allowedFirstTraitLevels"] = [15]
            if mode == "fixed":
                fixed_id = next(iter(pool))
                sigil["defaultSecondaryTraitId"] = fixed_id
                sigil["defaultSecondaryTraitName"] = trait_names.get(fixed_id)
            elif sigil.get("defaultSecondaryTraitId") not in pool:
                sigil["defaultSecondaryTraitId"] = None
                sigil["defaultSecondaryTraitName"] = None
            lot = game["SkillTypeLotIdForRandom2ndSkill"]
            if mode == "fixed":
                evidence = f"SkillId2 is fixed to {game['SkillId2']}; no random secondary lot."
            elif mode == "random":
                evidence = f"secondary lot {lot} resolves to {len(pool)} exact traits through skill_type_lot.tbl and skill_lot.tbl."
            else:
                evidence = "no fixed SkillId2 and no random secondary lot."
            sigil["notes"] = (
                f"Fresh local 2.0.2 gem.tbl verifies SkillId1 {game['SkillId1']}; "
                + evidence
            )
            sigil["source"] = (
                "fresh local 2.0.2 gem.tbl from data.i; local 2.0.2 "
                "skill_type_lot.tbl; local 2.0.2 skill_lot.tbl"
            )
            sigil["confidence"] = "high"
            fixed_sigils.append(sigil)
        payload["sigils"] = fixed_sigils
        payload["description"] = (
            "GBFR 2.0.2 sigil catalog rebuilt from a fresh local data.i extraction. "
            "gem.tbl is authoritative for item/primary/fixed-secondary identity; "
            "skill_type_lot.tbl and skill_lot.tbl are authoritative for random secondary pools. "
            "Three legacy IDs absent from gem.tbl are retained only by read-only name fallbacks, not this constructible catalog."
        )
        payload["sources"] = [
            {
                "name": "Fresh local GBFR 2.0.2 data.i extraction",
                "path": "system/table/gem.tbl",
                "notes": "Authoritative item, primary trait, fixed secondary trait, and secondary lot selector.",
            },
            {
                "name": "Fresh local GBFR 2.0.2 secondary-lot tables",
                "path": "system/table/skill_type_lot.tbl + system/table/skill_lot.tbl",
                "notes": "Authoritative random secondary-trait pools.",
            },
        ]
        args.catalog.write_text(
            json.dumps(payload, ensure_ascii=False, indent=4) + "\n", encoding="utf-8"
        )
        for trait in traits_payload["traits"]:
            internal_id = trait["internalId"]
            hash_key = trait["hash"].removeprefix("0x").upper()
            game_max = status_max.get(internal_id, status_max.get(hash_key))
            if game_max is None:
                continue
            trait["maxLevel"] = game_max
            trait["canAppearAsPrimary"] = internal_id in primary_traits
            trait["canAppearAsSecondary"] = internal_id in secondary_traits
            trait["bannedAsSecondaryOnPlusSigils"] = False
            trait["notes"] = (
                f"Fresh local 2.0.2 skill_status.tbl verifies the effect curve through level {game_max}; "
                "gem.tbl and the joined lot tables verify factor-slot reachability."
            )
            trait["source"] = (
                "fresh local 2.0.2 skill_status.tbl; local 2.0.2 gem.tbl; "
                "local 2.0.2 skill_type_lot.tbl; local 2.0.2 skill_lot.tbl"
            )
            trait["confidence"] = "high"
        traits_payload["description"] = (
            "GBFR 2.0.2 trait catalog audited against a fresh local data.i extraction. "
            "maxLevel is the skill_status.tbl effect-curve cap; a single factor record remains limited separately to its natural 1..15 range."
        )
        traits_payload["sources"] = [
            {
                "name": "Fresh local GBFR 2.0.2 skill and factor tables",
                "path": "system/table/skill_status.tbl + gem.tbl + skill_type_lot.tbl + skill_lot.tbl",
                "notes": "Authoritative effect-curve caps and factor primary/secondary reachability.",
            }
        ]
        traits_path.write_text(
            json.dumps(traits_payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )
        for row in trait_rows:
            if row["gameEffectCurveMaxLevel"] is not None:
                row["catalogMaxLevel"] = row["gameEffectCurveMaxLevel"]
                row["issues"] = []
                row["status"] = "verified"

    rows = []
    summary = defaultdict(int)
    for sigil in payload["sigils"]:
        internal_id = sigil["internalId"]
        game = gems.get(internal_id)
        issues: list[str] = []
        game_pool: set[str] = set()
        game_mode = "missing"
        if game is None:
            issues.append("missing-from-gem.tbl")
        else:
            if game["SkillId2"]:
                game_mode = "fixed"
                game_pool = {game["SkillId2"]}
            elif game["SkillTypeLotIdForRandom2ndSkill"] >= 0:
                game_mode = "random"
                lot_key = game["SkillTypeLotIdForRandom2ndSkill"]
                if lot_key not in type_lots:
                    issues.append(f"missing-skill-type-lot:{lot_key}")
                game_pool = type_lots.get(lot_key, set())
            else:
                game_mode = "none"

            if sigil.get("primaryTraitId") != game["SkillId1"]:
                issues.append("primary-trait-mismatch")
            catalog_pool = set(sigil.get("allowedSecondaryTraitIds") or [])
            if catalog_pool != game_pool:
                issues.append("secondary-pool-mismatch")
            if bool(sigil.get("supportsSecondaryTrait")) != bool(game_pool):
                issues.append("secondary-support-mismatch")

        status = "verified" if not issues else "mismatch"
        summary[status] += 1
        for issue in issues:
            summary[issue.split(":", 1)[0]] += 1
        rows.append(
            {
                "internalId": internal_id,
                "hash": sigil.get("hash"),
                "displayName": sigil.get("displayName"),
                "status": status,
                "issues": issues,
                "catalogPrimaryTraitId": sigil.get("primaryTraitId"),
                "gamePrimaryTraitId": game["SkillId1"] if game else None,
                "gameSecondaryMode": game_mode,
                "gameSkillTypeLotId": game["SkillTypeLotIdForRandom2ndSkill"] if game else None,
                "catalogSecondaryTraitIds": sorted(sigil.get("allowedSecondaryTraitIds") or []),
                "gameSecondaryTraitIds": sorted(game_pool),
            }
        )

    result = {
        "schemaVersion": 1,
        "authority": "fresh local GBFR 2.0.2 data.i extraction",
        "database": args.database.name,
        "databaseSha256": sha256(args.database),
        "catalog": args.catalog.name,
        "catalogSha256": sha256(args.catalog),
        "tableCounts": {
            "gem": len(gems),
            "skill_status": connection.execute("SELECT COUNT(*) FROM skill_status").fetchone()[0],
            "skill_lot": connection.execute("SELECT COUNT(*) FROM skill_lot").fetchone()[0],
            "skill_type_lot": connection.execute("SELECT COUNT(*) FROM skill_type_lot").fetchone()[0],
        },
        "summary": dict(sorted(summary.items())),
        "rows": rows,
        "traitSummary": {
            "verified": sum(row["status"] == "verified" for row in trait_rows),
            "mismatch": sum(row["status"] != "verified" for row in trait_rows),
        },
        "traitRows": trait_rows,
    }
    if args.raw_table_dir:
        result["rawTableSha256"] = {
            name: sha256(args.raw_table_dir / name)
            for name in ("gem.tbl", "skill_status.tbl", "skill_lot.tbl", "skill_type_lot.tbl")
        }
    rendered = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered)


if __name__ == "__main__":
    main()
