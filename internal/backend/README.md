# Backend map

`internal/backend` is one Go package because the Wails application shares a single process/session owner and many safety checks deliberately use package-private state. Files are grouped by stable prefixes instead of being scattered across the repository root. A matching `*_test.go` file is an automated regression test for the adjacent feature family, not an application script.

| Feature family | Production files | Responsibility |
| --- | --- | --- |
| Application shell | `run.go`, `app.go`, `locale.go` | Wails startup, bindings, configuration, process ownership, shutdown and language state |
| Save transactions | `save_*.go`, `progression_editor.go`, `badge_store.go` | Save discovery, parsing, backup, checksum-safe mutation, quest/title/progression records |
| Loadouts and formulas | `loadout*.go`, `weapon_awakening_stages.go` | Presets, weapons, skills, mastery, permanent growth, estimates, sharing and atomic writes |
| Sigils | `sigil_*.go` | Shared catalog, offline creation, live capture/write, names and safety checks |
| Wrightstones | `wrightstone_*.go` | Blessing catalog, offline creation, live editing and write verification |
| Summons | `summon_*.go` | Summon catalogs, advisory natural rules, save editing and live editing |
| Runtime foundation | `readonly_game_process.go`, `code_hook_safety.go` | Process identity, bounded reads/writes, target ownership, address validation and rollback evidence; regression coverage lives in the colocated `process_*_test.go` files |
| Runtime patches | `runtime_patch_*.go`, `monster_enhance_safety.go`, `overlimit.go`, `runtime_currency.go`, `runtime_inventory_item.go` | Version-guarded patch catalog, conflict handling, persistent sessions and exact restoration |
| Runtime monitoring | `runtime_party_monitor.go`, `runtime_character_panel*.go`, `damage_overlay_windows.go` | Party snapshots, selected-object reads, final character panel location and damage overlay lifecycle |
| Formula evidence | `formula_*.go` | Stable observation, A/B/A/B state machine, candidate scans and redacted evidence export; field calibration is covered by the colocated `field_runtime_calibration_test.go` |
| Embedded data | `data/` | Versioned 2.0.2 catalogs, layouts, formulas and machine-readable evidence |
| Native resource | `resources/patch_core.dll` | Audited embedded helper built from `src_dll/patch_core` |

## Test naming

- `*_test.go` next to a production family covers its normal behavior and failure boundaries.
- `*_safety_test.go`, `*_lease_test.go`, `*_atomic_test.go` and `*_detach_test.go` protect memory ownership, rollback and cleanup behavior.
- `*_truth_test.go`, `*_evidence_test.go` and `*_local_exe_test.go` compare checked-in catalogs or layouts with versioned evidence; local-game tests skip when their explicit input is unavailable.
- Frontend behavior lives under `frontend/src`; its `*.test.js` files verify UI contracts, generated bindings, catalog parity and responsive behavior.

Maintainer-only data scripts are documented separately in [`tools/README.md`](../../tools/README.md). One-off diagnostics and credentials belong in ignored local directories and are never part of a release.
