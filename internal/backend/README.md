# Backend map

`internal/backend` is one Go package because the Wails application shares a single process/session owner and many safety checks deliberately use package-private state. Files are grouped by stable prefixes instead of being scattered across the repository root. A matching `*_test.go` file is an automated regression test for the adjacent feature family, not an application script.

| Feature family | Production files | Responsibility |
| --- | --- | --- |
| Application shell | `run.go`, `app.go`, `locale.go` | Wails startup, bindings, configuration, process ownership, shutdown and language state |
| Save transactions | `save_*.go`, `progression_editor.go`, `badge_store.go` | Save discovery, parsing, backup, checksum-safe mutation, quest/title/progression records |
| Loadouts and formulas | `loadout*.go`, `weapon_awakening_stages.go` | Presets, weapons, skills, mastery, permanent growth, estimates, sharing and atomic writes |
| Optimizer evidence | `loadout_optimizer_evidence.go`, `infinity_rule_catalog.go` | Versioned optimizer inputs, Infinity catalogs, evidence provenance and replay checks |
| Logs battle archive | `logs_battle_archive.go`, `logs_loadout_import.go` | Read-only Logs database pagination, encounter details and external loadout conversion |
| Combat reference | `runtime_combat_reference.go` | Versioned 2.0.2 damage-cap, guard and conditional-curve evidence exposed without invented interpolation |
| Sigils | `sigil_*.go` | Shared catalog, offline creation, live capture/write, names and safety checks |
| Wrightstones | `wrightstone_*.go` | Blessing catalog, offline creation, live editing and write verification |
| Summons | `summon_*.go` | Summon catalogs, advisory natural rules, save editing and live editing |
| Advisory legality | `legality.go` | Shared warning-level legality results; encodable user choices remain writable subject to structural safety checks |
| Runtime foundation | `readonly_game_process.go`, `runtime_executable_identity.go`, `code_hook_safety.go` | Process identity, executable version gates, bounded reads/writes, target ownership, address validation and rollback evidence; regression coverage lives in the colocated `process_*_test.go` files |
| Runtime patches | `runtime_patch_*.go`, `monster_enhance_safety.go`, `overlimit.go`, `runtime_currency.go`, `runtime_inventory_item.go` | Version-guarded patch catalog, task-result quantity multiplier, conflict handling, persistent sessions and exact restoration |
| Runtime monitoring | `runtime_party_monitor.go`, `runtime_character_panel*.go` | Party snapshots, selected-object reads, and final character panel location |
| Runtime companions | `runtime_companion.go`, `runtime_qol.go`, `runtime_damage_capture.go`, `runtime_emergency_stop.go` | Application-owned native runtime lifecycle, complete process identity, shared-state ownership and unified emergency restoration |
| Audio control | `audio_mixer_mod.go` | Character voice and audited UI sound routing, configuration and runtime lifecycle |
| Camera and spatial tools | `camera_mod.go`, `runtime_spatial.go` | Town camera controls, coordinate diagnostics, bounded movement and restoration |
| Conflux timer | `conflux_timer.go` | Version-guarded Endless Conflux timer patch, ownership and rollback |
| Natural drop deployment | `natural_drop_mod.go`, `natural_wrightstone_mod.go` | Transactional external table generation, native `data.i` registration, backup and exact restoration |
| Save comparison | `save_diff.go`, `fate_episode_batch.go` | Redacted save diff evidence and version-guarded Fate Episode transactions |
| Virtual sigils | `virtual_sigil_mod.go` | Per-character virtual slot presets, runtime encoding, inventory identity checks and restoration |
| Formula evidence | `formula_*.go` | Stable observation, A/B/A/B state machine, candidate scans and redacted evidence export; field calibration is covered by the colocated `field_runtime_calibration_test.go` |
| GBFR data index | `gbfr_data_index.go`, `gbfr_hash32.go` | Native `data.i` parsing/rebuilding and canonical game-path hashing used by transactional external-table deployment |
| Embedded data | `data/` | Versioned 2.0.2 catalogs, layouts, formulas and machine-readable evidence |
| Native resource | `resources/patch_core.dll` | Audited embedded helper built from `src_dll/patch_core` |

Package documentation lives in `doc.go`. Cross-family catalog consistency is covered by `catalog_*_test.go` files in addition to the feature-prefixed tests below.

## Test naming

- `*_test.go` next to a production family covers its normal behavior and failure boundaries.
- `*_safety_test.go`, `*_lease_test.go`, `*_atomic_test.go` and `*_detach_test.go` protect memory ownership, rollback and cleanup behavior.
- `*_truth_test.go`, `*_evidence_test.go` and `*_local_exe_test.go` compare checked-in catalogs or layouts with versioned evidence; local-game tests skip when their explicit input is unavailable.
- Frontend behavior lives under `frontend/src`; its `*.test.js` files verify UI contracts, generated bindings, catalog parity and responsive behavior.

Maintainer-only data scripts are documented separately in [`tools/README.md`](../../tools/README.md). One-off diagnostics and credentials belong in ignored local directories and are never part of a release.
