# DLC 2.0.2 implementation status

This page separates implemented behavior from open calibration work. “Implemented” means the code path and automated verification exist; “field-verified” additionally requires a repeated observation against the running game.

## Implemented and covered

| Area | Current state |
| --- | --- |
| Multiple save slots | Save discovery is shared by offline editors and displays each detected slot independently. |
| Offline transaction safety | Backup, checksum repair, temporary output, atomic replacement and readback are implemented. |
| Sigil catalogs | Save, live-memory and loadout constructors share the same 2.0.2 catalog; DLC entries remain searchable. |
| Wrightstones | Offline and live editors share catalog choices; tier index and the independent stored state/rank field are labelled separately. |
| Summons | Offline creation/update, live editing and loadout editing share catalog options. Natural/legality rules are advisory; encodable values remain writable. |
| Loadout workspace | Weapons, twelve sigils, four active skills, mastery, permanent growth, Over Mastery and summons are editable; single-loadout import/export stays in the persistent action bar. |
| Pre-DLC saves | Editors remain usable when mastery/summon records are absent and do not claim that writing a record unlocks the DLC system. |
| Persistent patch pages | Combat, character and quest tabs share one owned connection; navigation does not unmount or silently restore enabled patches. |
| Automatic perfect guard | The two-site combo-continuation patch has its own original-byte guards and was re-verified in the DLC 2.0.2 field session. |
| Runtime monitoring | Solo party snapshots, selected-object monitoring and stable HP readback are implemented. |
| UI/catalog icons | Character, item, weapon, sigil, summon and 261/262 playable-skill mappings use bundled official assets; `AB_PL2400_09` has no matching 2.0.2 PNG and uses the neutral fallback. |

## Field evidence captured

- Io final HP was read repeatedly from the running game, including the stable `126,645` snapshot used to validate the monitor lifecycle.
- The Io EX node `1F52146F` is stored as float32 `0.4` but produces `+4` on the displayed stun scale; the versioned calibration record keeps the raw and displayed units separate.
- A repeated training-area comparison observed the same incoming attack with and without Defense +5%; the result is consistent with percentage damage reduction for that sample.
- Automatic perfect guard was retested after adding the second guarded site used during combo continuation.

## Still open or only partially closed

| Request | Honest boundary |
| --- | --- |
| Complete SDK/game dump | Not performed and not required by the released application. Checked-in data comes from selected extracted tables, guarded executable sites, save layouts and field samples—not a claim of complete engine metadata. |
| Every character’s exact final formula | The four final panel fields can be read when a supported runtime object is available, but a universal offline formula covering every character, conditional skill and battle state is not closed. |
| Mastery unlock boundaries for every save state | Structural rank pools and save-backed caps are implemented. The full matrix of unopened, zero-MSP, low-rank and max-rank saves has not been field-sampled across multiple saves. |
| All permanent character growth | Extracted AP-tree/panel contributions are included where mapped. Irreversible growth and character-specific exceptions still require paired saves or cross-character evidence. |
| Defense and support interactions | The +5% sample is field-verified; the broader set of defense masteries, buffs, reductions and support interactions has not been generalized from that single sample. |
| Damage cap and conditional skills | Represented as separate sources/candidates, but battle/dummy samples are still required for exact effective caps and conditions. |
| Every runtime patch in every scene | Catalog guards and automated lifecycle tests exist for all published entries; only named critical paths have repeated live-game evidence. |
| Unlocking absent DLC systems | The tool can edit encodable records but intentionally does not promise to set every progression flag needed to unlock systems the save has never opened. |

The application must continue to label unresolved values as estimates, candidates, negative observations or open evidence. A green unit test proves a software contract, not a previously unobserved game formula.
