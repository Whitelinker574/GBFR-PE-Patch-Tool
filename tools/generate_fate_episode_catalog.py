#!/usr/bin/env python3
"""Build the checked-in Fate Episode catalog from local GBFR 2.0.2 data."""

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


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_messages(path: Path) -> dict[str, str]:
    payload = msgpack.unpackb(path.read_bytes(), raw=False, strict_map_key=False)
    result: dict[str, str] = {}
    for row in payload.get("rows_", []):
        column = row.get("column_", {})
        key = str(column.get("id_hash_", "")).strip()
        if key:
            result[key] = str(column.get("text_", "")).strip()
    return result


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
        "fateEpisode": tables_dir / "fate_episode.tbl",
        "characterFateStatus": tables_dir / "chara_status_fate.tbl",
        "textZh": tables_dir / "text" / "cs" / "text_fate_episode.msg",
        "textEn": tables_dir / "text" / "en" / "text_fate_episode.msg",
    }
    missing = [str(path) for path in [sqlite_path, *sources.values()] if not path.is_file()]
    if missing:
        raise SystemExit("missing inputs:\n" + "\n".join(missing))

    zh = load_messages(sources["textZh"])
    en = load_messages(sources["textEn"])
    connection = sqlite3.connect(f"file:{sqlite_path.as_posix()}?mode=ro", uri=True)
    connection.row_factory = sqlite3.Row
    try:
        rewards: dict[tuple[str, int], dict[str, float]] = {}
        for row in connection.execute(
            "SELECT Key, Unk6, Hp, Attack FROM chara_status_fate ORDER BY Key, Unk6"
        ):
            # chara_status_fate uses one-based Fate status IDs, while the
            # episode key suffix and UI index are zero-based.
            rewards[(row["Key"], int(row["Unk6"]) - 1)] = {
                "hp": float(row["Hp"]),
                "attack": float(row["Attack"]),
            }

        episodes = []
        for row in connection.execute(
            """
            SELECT Key, CharaId, SortOrder, ReqLevel, ReqQuestId, MissionQuestId,
                   PartyUnlockStatus, UnlockByDefaultMaybe, FinalFateMaybe,
                   FateMissionTitle, SummaryOfPreviousFate
            FROM fate_episode
            WHERE Key LIKE 'FATE_PL%'
            ORDER BY CharaId, SortOrder, Key
            """
        ):
            key = str(row["Key"])
            index = int(key.rsplit("_", 1)[1])
            title_key = str(row["FateMissionTitle"] or "")
            summary_key = str(row["SummaryOfPreviousFate"] or "")
            entry = {
                "key": key,
                "characterCode": str(row["CharaId"]),
                "index": index,
                "sortOrder": int(row["SortOrder"]),
                "requiredLevel": int(row["ReqLevel"]),
                "requiredQuestId": str(row["ReqQuestId"] or ""),
                "missionQuestId": str(row["MissionQuestId"] or ""),
                "partyUnlockStatus": int(row["PartyUnlockStatus"]),
                "unlockedByDefault": bool(row["UnlockByDefaultMaybe"]),
                "finalEpisode": bool(row["FinalFateMaybe"]),
                "titleZh": zh.get(title_key, ""),
                "titleEn": en.get(title_key, ""),
                "summaryZh": zh.get(summary_key, ""),
                "summaryEn": en.get(summary_key, ""),
            }
            reward = rewards.get((entry["characterCode"], index))
            if reward is not None:
                entry["staticBonus"] = reward
            episodes.append(entry)
    finally:
        connection.close()

    characters = sorted({row["characterCode"] for row in episodes})
    if len(episodes) != 319 or len(characters) != 29:
        raise SystemExit(
            f"unexpected Fate catalog shape: {len(episodes)} episodes / {len(characters)} characters"
        )
    if any(sum(1 for row in episodes if row["characterCode"] == code) != 11 for code in characters):
        raise SystemExit("every playable character must have exactly 11 Fate episodes")

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
        "episodes": episodes,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {len(episodes)} Fate episodes to {args.output}")


if __name__ == "__main__":
    main()
