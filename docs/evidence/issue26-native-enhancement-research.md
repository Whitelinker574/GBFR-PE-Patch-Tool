# Issue #26: native 2.0.6 enhancement encoding

Read-only investigation, 2026-10-19, against repository master `e60e663`.
All executable addresses below are RVAs, not absolute process addresses.
The executable image base is `0x140000000`. The native analysis did not read or
change a game process. Separate operational acceptance is recorded at the end.

## Primary sources and scope

- Installed game executable:
  `E:/app/steamapps/common/Granblue Fantasy Relink/granblue_fantasy_relink.exe`,
  SHA-256 `4B3ACE0FD03DF5D3D08C6453E12DF2279C6259ABF7D2FB3622921FCBEA775145`.
- Extracted same-version tables:
  `D:/gbf/GBFR-Codex-Field-Lab-20260720/field-extracted-206-audit/tables-206.sqlite`;
  relevant tables are `ap_tree_atk`, `ap_tree_def`, `ap_tree_wep`,
  `ap_tree_rebuild`, `limit_bonus`, `limit_bonus_param`, and `chara_invite`.
- Issue author's [four original save files](https://github.com/user-attachments/files/33641542/SaveData20261019.zip),
  parsed by the main investigation: A/A0 before and B/B0 after purchasing one
  Maglielle attack-tree sigil-slot node for 200 MSP. Their character UnitID is
  `10038`; character code is `PL2500`; level is 90.

Function boundaries were checked against PE exception/unwind records;
inferred function purposes below are not debug symbols. Raw saves and local
investigation scripts are not distributed with the repository.

## Proven node representation

`0xB592A0` registers two consecutive uint32 fields: node hash at pointer `p`
as IDType 1601 (`0xB5931D`), and flags at `p+4` as IDType 1602
(`0xB593CA`). The registration key is `(IDType << 32) | UnitID`.
`0x2B7B45..0x2B7B79` registers 400 node records per character, advancing the
native record pointer by `0x38` each time. Save node UnitIDs occupy the
character's 1000-wide region; for character UnitID `u`, region start is `u*1000`
(equivalently `10000000 + (u-10000)*1000`). Only 400 entries are registered
in this 2.0.6 path; the remaining 600 identifiers must not be invented.

The 1601 hash is **`limit_bonus.Key`**, not `ap_tree_*.Key`. A single
`limit_bonus` record groups several visible AP-tree nodes. For the matching
character/tree, `ap_tree_*.LimitBonusId` identifies that group, and
`LimitBonusParamIndex` identifies its purchased bit. Do not equate the save
record index with table row order or UI grid location.

The native add primitive at `0x2EAB80` makes this explicit:

```text
i = ap_tree row's LimitBonusParamIndex
if 0 <= i <= 7: flags |= 1 << i                 # 0x2EAD8C..0x2EADA8
if grant_origin && 0 <= i <= 7:
    flags |= 0x100 << i                        # 0x2EADAD..0x2EADD4
rebuild_node_effects()                         # 0x2EADDF -> 0x28F810
```

The `grant_origin` name is an inference for its `r9b` argument: the exact
observed behavior is mirroring acquired low-byte bits into bits 8..15.
Bulk-grant callers `0x2F39DD` and `0x2F3B1D` pass true; the generic caller
`0x2EA632` passes its explicit boolean. The paired remove primitive at
`0x2EB140`, especially `0x2EB34C..0x2EB36A`, clears only bit `i <= 7` using
`BTR` and leaves bits 8..15 intact.

`0x28F810` resolves the group and rebuilds up to three parameter effects.
At `0x28F8AC/0x28F8C4/0x28F8DD` it forwards the flags with an argument
instructing `0x28F900` to truncate to uint8 (`0x28F9B1..0x28F9BE`). Thus
the high byte is **not a second set of active mastery bonuses**. The generic
parameter routine can consume ten bits when its other calling mode is used;
that must not be confused with the eight-bit AP-node acquisition mask.

## Exact 200-MSP purchase join

All four saves retain 400 node records for Maglielle; A/A0 have 117 nonzero
flags records, with matching low/high bytes. B/B0 introduce exactly one
asymmetry:

| Record | Before | After |
| --- | --- | --- |
| 1601 / 10038029 | `4FEE00FA` | unchanged |
| 1602 / 10038029 | `0x0303` | `0x0307` |
| 1503 / 10038 | `[-1,-1]` | `[312,-1]` |
| 1321 / 10038 | `0` | unchanged |

`limit_bonus.Key=4FEE00FA` has `ParamId1=7B727910`, sigil-slot title/description
IDs ending `CMN_PARAM_GNS`. The parameter row has `Lv1Value..Lv8Value=1`
and `Lv9Value=Lv10Value=0`. The PL2500 attack-tree join is:

| Param index / bit | AP-tree key | MSP cost | NodeGridLocation |
| --- | --- | --- | --- |
| 0 | `B05662A3` | 10 | 127 |
| 1 | `64C21B75` | 50 | 213 |
| 2 | `0D1E9832` | 200 | 312 |
| 3 | `03EBBDBA` | 300 | 367 |
| 4 | `6F74FC3D` | 700 | 431 |

Therefore `0x0303 -> 0x0307` adds precisely bit 2 of the active mask while
leaving the two mirrored baseline bits unchanged. A/A0 are an automatically
granted baseline, not an all-unlearned reference. The same database's
`chara_invite` contains progression-dependent nonzero attack/defense grants
(e.g. row 15: level 90 and attack/defense grant cutoff 120); this independently
rules out treating zero as every recruited character's ordinary default.

## Rebuilds, cursor state, and fields to preserve

The character state used by these functions contains nodes at `state+0x18`
with stride `0x38`. `0x2D78C0` is a state reconstruction/recalculation path:
it visits all 400 nodes at `0x2D7AA5..0x2D7B55`, calls `0x28F810` for each,
then calls `0xB596A0` at `0x2D7BFA`. The latter clears and recomputes the
derived parameter aggregate at `state+0x57D8` from the per-node effects.
Consequently effects can be derived from low-byte node masks, without
rewriting serialized base HP/ATK or guessing an aggregate cache.

Field 1503 is a two-int value at `state+8`, registered at `0x2B79C3` and
`0x2B7A01`. Its observed attack entry equals the bought node's
`NodeGridLocation` (312), not a bonus or progress count. Treating this as a
UI selection/cursor is a strongly supported inference. `[-1,-1]` is observed
before opening/selecting the edited node, not a fabricated zero sentinel.
Whether resetting this cursor is necessary has not been established; preserve
it for an initially minimal gameplay test.

Fields 1309/1310 register `state+0x5A28`/`state+0x5A2C` at
`0x2B7D05`/`0x2B7DDA`. The initial reconstruction call to `0x2B5AD0`
recalculates level-derived base values and writes those locations at
`0x2B5DB6` and `0x2B5EEC`; it does not grant AP nodes. They are a separate
base-stat layer and must not be zeroed to erase enhancement.

Field 1321 registers `state+0x5A1C` at `0x2B76A2`. Its incrementing native
path at `0x2B4455..0x2B44E8` is conditional on `state+0x5A24 >= 100`
(the level field), adds an accumulated amount, divides old/new totals by a
constant, and handles crossed reward thresholds. This is evidence of a
post-level-cap accumulation/reward counter, **not purchased enhancement
progress**. Exact reward semantics are not named here without further proof.
It stays zero across the real level-90 purchase. Preserve 1321 in a mastery
reset; existing editor terminology alone is not contrary native evidence.

## Minimal candidate transformations and remaining acceptance gate

For a supported 1602 record `v`, preserve bits outside the active low byte:

```text
restore automatically granted baseline: (v & ~0xFF) | ((v >> 8) & 0xFF)
unlearn every currently active node:     v & ~0xFF
```

The second mode matches the Issue's desired all-unlearned state, including
automatically granted nodes. Neither transformation deletes the mirrored
grant provenance or changes 1601 hashes, other characters, level/base stats,
1321/1323, selected loadout mastery, weapons, sigils, badges, or global MSP.
No refund amount should be invented from this single purchase.

These are **native-encoding-supported candidates**, not an observed built-in
game reset policy. This analysis did not identify an exhaustive high-byte
consumer or prove that every future progression/recruitment event will refrain
from granting nodes again. A copied-save game test must establish loadability,
displayed tree/HP changes, ability to buy a node, and save/reload persistence.
Native byte reasoning and checksummed export alone are not that acceptance.
The main investigation owns this operational test and any release decision.

## Implemented transaction and on-machine results

`character_enhancement_reset.go` implements both transformations as a separate,
explicit offline transaction. Existing non-regressive loadout imports are not
changed. Revision checks bind the preview to the source file, and a failed
post-write readback restores the exact pre-write bytes when no concurrent edit
has intervened. Node hashes, all bits outside the low byte, and every other save
record are preserved. A zero-change operation does not write or create a backup.

The author's B sample was copied before testing. Baseline restoration changes
one of 400 groups, and all-unlearn changes 117. Regression checks compare every
other serialized record, check exact backup bytes, exercise stale revisions,
repeat operations, and inject a readback failure to verify rollback.

A separate copy of a local save was installed temporarily in an already backed-up
slot for the 2.0.6 game test. Io's attack/defense trees changed from 150% to 0%,
and both collection indicators changed from 100% to 0%. Her displayed HP changed
from 134750 to 11645; the other three party members retained their displayed
values. Level and equipment were retained. The first attack node could be
purchased again for 2 MSP: the attack tree showed 1%, and a normal game save
contained exactly one acquired bit across the character's 400 groups. This is
an actual game write, not an editor-generated round trip. After completely
closing and restarting the game, loading that saved copy retained the 1% attack
tree and its learned node; defense and collection indicators remained at 0%.
The original save group was restored after the test and compared to its backup.
