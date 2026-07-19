# Loadout icon sources and mapping contract

## Game data icons

`tools/sync_reference_icons.ps1` is the reproducible source of
`frontend/src/gameAssetIcons.json` and the generated compatibility map
`frontend/src/loadoutTraitIcons.json`. It reads the local
`GBFR-UI-Reference-Library-2.0.2.zip` release-v2 semantic catalog, then checks
the joins against the unpacked 2.0.2 `skill.tbl`, `weapon.tbl`, and `item.tbl`.
It bundles the matched game PNG files under this directory, so runtime pages
do not need a network connection.

Generated filenames are normalized to lower case and tested with an
ordinal, case-sensitive directory lookup. This is required even on Windows:
Wails serves the bundle from Go `embed.FS`, where `Cmn_*.png` and
`cmn_*.png` are different paths.

The generator only accepts an authoritative entity ID/hash join (or a
documented exact table alias) and copies the game's internal asset unchanged:

- traits: `cmn_icskill_*.png`
- weapons: `cmn_imgequ_wp*.png`
- summons: `cmn_icitmsmn02_*.png`
- items: `cmn_icitm_*.png`
- character chips: `cmn_mini_s_plXXXX.png`

Character-trait names use the official per-character `cmn_icskill_05_plXXXX`
emblem. Character chips use the official compact `cmn_mini_s_plXXXX` image.
Neither mapping uses community `*_Avatar.png` portraits. The compatibility
trait-name map is generated from the same semantic catalog; it is not a
separate source of guessed artwork.

The current application-catalog coverage is:

- traits: 182/184
- weapons: 159/163
- summons: 189/189
- items: 301/312
- characters: 29/29

Unresolved records intentionally have no icon mapping and keep the neutral UI
fallback. The 2.0.2 semantic catalog has no authoritative join for:

- `SKILL_112_00` / `0xD0A1C6E5` — Window of Opportunity
- `SKILL_023_00` / `0xCAC6AFF2` — Potent Greens; the semantic catalog only
  joins the same display name to the distinct `SKILL_156_00` /
  `0xCD18A77D` identity, so the UI may use its name fallback but must not
  invent an ID/hash mapping
- `WEP_PL2100_03` / `0x2C4CAADD` — World Ender (`weapon.tbl` requires
  `cmn_imgequ_wp2102`)
- `WEP_PL2100_06` / `0xDFBB5727` — Efes (`weapon.tbl` requires
  `cmn_imgequ_wp2105`)
- `WEP_PL2200_03` / `0x73D34F1B` — Gateway-Star Sword (`weapon.tbl` requires
  `cmn_imgequ_wp2202`)
- `WEP_PL2300_03` / `0xDA807CA2` — Desolation-Crown Bow (`weapon.tbl` requires
  `cmn_imgequ_wp2302`)
- eleven special items: hashes `CB39E0FC`, `29BBA035`, `E600BE75`, `7F695B76`,
  `E8C461CA`, `F384E322`, `CD6AF550`, `EAC2D7AB`, `131A4636`, `9FC6585E`, and
  `CE0B379E`; `item.tbl` requires `cmn_icitm_50_0000..0010`

An exact filename scan of every ZIP supplied in `D:\gbf` on 2026-07-19
(including the 2.0.2 UI Reference Library, Portrait/Skill Portable pack and
both DLC extract packs) found none of the files above. Similar `wp2106`,
`wp2206`, and `wp2306` images are deliberately not reused. The eight Sky
Memory records are fully mapped: `item.tbl` proves that all eight reuse the
official `cmn_icitm_50_0100.png` image.

## Active-skill icons

Runtime active-skill mappings are generated from the unpacked 2.0.2
`ability.tbl` and the local UI Reference Library. All 243 unique mapped PNGs
match the corresponding reference-archive bytes; legacy community files can
remain on disk, but no current mapping references them. The playable-character
catalog contains 262 skills and 261 exact mappings.
`AB_PL2400_09` / `0xBC2EF1D8` (狼牙斩) explicitly requires
`cmn_icablt_pl2400_09` in `ability.tbl`; the archive does not contain that PNG
and provides no reuse field, so it deliberately keeps the neutral frame. The
other 16 unmapped rows belong to NPC owners `NP0000` and `NP0300`.
