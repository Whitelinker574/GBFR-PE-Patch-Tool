# Save / memory table parity (DLC 2.0.2)

## Result

| Family | Offline save path | Runtime memory path | Result |
| --- | --- | --- | --- |
| Summon types, main traits, sub parameters | `SummonSaveGen.GetOptions` | `App.SummonGetOptions` | Exact: both call the same function (189 / 82 / 22) |
| Summon natural pools and random levels | `validateSummonTraitChange` | `validateSummonTraitChange` | Exact: both use the same 189-rule validator |
| Wrightstone traits and levels | `WrightstoneGen.GetTraitList` | `App.WrightstoneMemoryGetOptions` | Exact: same catalog and level resolver |
| Sigil catalog | `SigilGen` and loadout constructor | `App.SigilMemoryGetOptions` | Exact: same 184 table-backed items, primary traits, secondary pools and natural level sets |

The parity contract is executable in `catalog_channel_parity_test.go`. Historical
runtime-only hash names remain labelled when found in memory. The editors may
preserve or write a raw encodable value without presenting it as natural.

## Advisory legality policy

- Sigil, wrightstone and summon natural pools, combinations, duplicate rules,
  observed levels and DLC availability are diagnostics, not authority over the
  user's requested bytes.
- The exact unpacked 2.0.2 tables provide defaults and compact warnings, but
  non-natural combinations remain directly selectable and the chosen encodable
  values are written through offline save, runtime memory and loadout-resource
  transactions without a separate force-mode switch.
- Advisory legality does not bypass target ownership, stale-record comparison,
  container bounds, automatic backup, atomic rollback, checksum repair or
  post-write readback. Those checks prevent a wrong-target or partial write and
  are therefore structural/transactional safety, not game-rule legality.

## Sigil table evidence

- The four tables were freshly extracted from the installed 2.0.2 `data.i`:
  `gem.tbl` has 1,034 rows, `skill_status.tbl` 6,320,
  `skill_lot.tbl` 439, and `skill_type_lot.tbl` 21.
- All 184 selectable catalog items now match `gem.tbl` for `SkillId1` and
  fixed/random/no-secondary mode. Random pools are exact joins through the two
  lot tables; the old 137-trait fallback is gone.
- All 184 catalog traits now match `skill_status.tbl` for their effect-curve
  cap. The editor keeps that aggregate cap separate from a single factor's
  natural Lv1–15 write range, so two legal Lv15 records can reach a Lv30 curve
  without permitting an impossible single Lv30 record.
- `GEEN_100_04`, `GEEN_112_04`, and `GEEN_113_04` are absent from the 2.0.2
  `gem.tbl`. They remain read-only display aliases for old memory/save values,
  but cannot be selected for new construction.
- The reproducible audit and raw table checksums are in
  `docs/evidence/sigil-table-audit-202.json`. The executable regression is
  `TestSigilCatalogMatchesFreshLocal202TableEvidence`.

## Summon rule evidence

- The local summon type catalog and the referenced probability dataset contain
  the same 189 unique type hashes.
- All 82 referenced main-trait names map to local hashes. Nine rows require
  translation aliases only; no hash is inferred from a translated name.
- All 22 sub-parameter names/hashes match directly.
- 151 random templates have allowed main/sub pools and level sets.
- 38 fixed templates prove their fixed main/sub hashes but the referenced page
  omits their fixed level values. The editor permits creation or changes while
  labelling them non-natural/unverified.
- The saved `Rank` field is not rarity. A 102-record real-save read produced the
  tier-index/rank matrix `0→2:11, 1→2:47, 2→0:3, 2→2:41`, so the tool does not
  derive Rank from tier.

The checked-in normalized evidence is `internal/backend/data/summon_natural_rules_202.json`.
Its source role is secondary cross-check; local game-table hashes remain the
primary identity evidence.

## Evidence boundary

Production catalogs are derived from audited local 2.0.2 game tables and are
checked by executable regression tests. External spreadsheets, third-party
runtime tables, generated workbooks, and local field-capture reports are review
inputs only. They are intentionally not kept in this repository and never
replace checked-in catalog data by name alone.
