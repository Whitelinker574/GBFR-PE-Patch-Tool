# Save / memory table parity (DLC 2.0.2)

## Result

| Family | Offline save path | Runtime memory path | Result |
| --- | --- | --- | --- |
| Summon types, main traits, sub parameters | `SummonSaveGen.GetOptions` | `App.SummonGetOptions` | Exact: both call the same function (189 / 82 / 22) |
| Summon natural pools and random levels | `validateSummonTraitChange` | `validateSummonTraitChange` | Exact: both use the same 189-rule validator |
| Wrightstone traits and levels | `WrightstoneGen.GetTraitList` | `App.WrightstoneMemoryGetOptions` | Exact: same catalog and level resolver |
| Sigil catalog | `SigilGen` and loadout constructor | `App.SigilMemoryGetOptions` | Exact: same hashes, names and natural level sets |

The parity contract is executable in `catalog_channel_parity_test.go`. Historical
runtime-only hash names remain available solely to label an old value already
found in memory; they are not selectable or writable through any editor.

## Summon rule evidence

- The local summon type catalog and the referenced probability dataset contain
  the same 189 unique type hashes.
- All 82 referenced main-trait names map to local hashes. Nine rows require
  translation aliases only; no hash is inferred from a translated name.
- All 22 sub-parameter names/hashes match directly.
- 151 random templates have allowed main/sub pools and level sets.
- 38 fixed templates prove their fixed main/sub hashes but the referenced page
  omits their fixed level values. Existing levels can be preserved; creating a
  fixed template or changing its levels remains fail-closed.
- The saved `Rank` field is not rarity. A 102-record real-save read produced the
  tier-index/rank matrix `0→2:11, 1→2:47, 2→0:3, 2→2:41`, so the tool does not
  derive Rank from tier.

The checked-in normalized evidence is `data/summon_natural_rules_202.json`.
Its source role is secondary cross-check; local game-table hashes remain the
primary identity evidence.

## Spreadsheet drop boundary

`D:\gbf\11` is audited by content, not filename. The review workbook is
`docs/evidence/ct085-table-audit.xlsx`; its `Channel Parity` sheet contains the
same result in tabular form. Misnamed, degraded, or partial spreadsheets remain
reference-only and do not overwrite production catalogs.
