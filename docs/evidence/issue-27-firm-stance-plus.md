# Issue #27: missing Firm Stance V+ catalog entry

Investigated against master `fbe833e` on 2026-10-13 (Asia/Shanghai).
Issue: https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/issues/27

## Cause and evidence

The editor catalog contains only `GEEN_087_04` (Firm Stance V). Its single-slot
metadata is correct. The two-slot `GEEN_087_24` (Firm Stance V+) exists in the
bundled game table and runtime name map but was omitted from `sigils.json`.
Thus the picker offers only V; its false `supportsSecondaryTrait` propagates
through the UI, compatible-trait query and backend impossible-write check.
There is no disagreement between the frontend and backend.

The checked-in `natural_drop_tables_202.zstd.b64` contains the original 2.0.2
`gem.tbl` (66,184 bytes; SHA-256
`F6B39C0FF9A190B3DD44FEFEAC84180D5FDA8254636DEEA1A8181336B1EA2C99`).
Each row is 64 bytes after the 8-byte header:

| Record | Zero-based row | Item hash (+8) | Primary (+0) | Fixed secondary (+4) | Random secondary selector (+36, int32) |
| --- | --- | --- | --- | --- | --- |
| `GEEN_087_04` / Firm Stance V | 246 | `80DC0FB8` | `B6E31F76` | `887AE0B0` (empty) | -1 |
| `GEEN_087_24` / Firm Stance V+ | 861 | `B1CCC211` | `B6E31F76` | `887AE0B0` (empty) | 7 |
| `GEEN_063_24` / Improved Dodge+ | audited reference | `E89224A1` | `8B3BF60C` | `887AE0B0` (empty) | 7 |

The V+ pool therefore uses the same 16-trait lot-7 join already recorded for
Improved Dodge+ in `sigil-table-audit-202.json`. The new test checks both raw
selectors and the exact shared pool. The historical audit stays unchanged:
it describes the 189 records present when it was generated. This recovery
adds one independently tested record, bringing the unified catalog to 224.

The Issue attachment shows a **synthesis prediction**, with Firm Stance first
and Celestial Aqua (天星之雪) second. It does not show a completed, persisted
result or a save dump. Celestial Aqua's trait hash `A898E283`, and Firm Stance's
`B6E31F76`, are both in the existing 2.0.3 `synthesis-traits.json`. A public
modding ID reference independently maps V+ to `GEEN_087_24` / `B1CCC211`:
https://nenkai.github.io/relink-modding/resources/item_ids/

## Minimal correction

- Add the missing V+ record and Chinese label, using its fixed primary and
  exact existing lot-7 pool. Keep V unchanged and distinguish the two items.
- Use the existing synthesis-secondary selection and validation code. No
  new wildcard pool, primary replacement, slot bypass or write bypass.
- Preserve the existing advisory policy: a lot-7 combination is `legal`;
  the screenshot's out-of-drop-pool synthesis combination is `forced` and
  writable with a warning. This status describes the drop-pool diagnostic;
  it does not establish that the screenshot combination is unobtainable.
  Reclassifying every synthesis combination requires destination-specific
  synthesis rules beyond the checked-in trait-membership list.
- Leave backup, stale-source checks, bounds, checksum repair, rollback and
  post-write verification intact.

## Regression and remaining verification

`sigil_firm_stance_test.go` checks the raw game rows, fixed primary, exact
natural pool, frontend-facing metadata/choices, generator queue, runtime
validator, loadout constructor and two-trait record encoding. It also rejects
secondaries on V and a true single-slot quest factor, an unknown trait, and a
known quest-only trait excluded from the synthesis pool.

`TestFirmStancePlusSaveCopyBacksUpAndReadsBothTraits` uses
`GBFR_TEST_LOADOUT_SAVE` when supplied. It modifies a temporary copy, verifies
the untouched backup, then reopens the written file and checks item hash,
both trait hashes/levels and normal flags. It skips when no sample is supplied.
Offline records store item hash at IDType 2703, item level at 2704, flags at
2707, and the two trait records at 1701/1702. Runtime encoding is a 0x1C-byte
record with primary at +0/+4, secondary at +8/+0xC, item hash at +0x10 and item
level at +0x18; reserved +0x14 bytes are preserved.

No game installation or completed synthesis save was available for this
investigation. Before release, execute the shown synthesis in the supported
game version, save/reload, and capture its item hash, both trait records,
levels and flags. Also create V+ with the editor on a backed-up save copy and
confirm that both traits survive an in-game reload. Table evidence is pinned
to 2.0.2; synthesis membership is pinned to 2.0.3. The current 2.0.6 runtime
layout support alone does not prove unchanged game data or synthesis rules.
