# Maintainer tools

This directory contains the maintainer scripts that reproduce checked-in data from auditable inputs, plus one automated test that verifies the icon script stays deterministic. They are not required to run the packaged application.

| File | Input | Checked result | When to run |
| --- | --- | --- | --- |
| `audit_sigil_tables.py` | Extracted DLC 2.0.2 sigil tables plus `internal/backend/data/sigils.json` and adjacent `traits.json` | Prints or writes the audit JSON; `--fix-catalog` deterministically rebuilds both catalogs and drops sigils absent from `gem.tbl` | After a game-table extraction or sigil-catalog change |
| `generate_ap_tree_panel_growth.py` | The local extracted table database, table directory, and explicit dataset-version label | Rebuilds `internal/backend/data/ap_tree_panel_growth.json` with source checksums | After mastery, Fate, weapon-tree, or permanent-growth data changes |
| `generate_fate_episode_catalog.py` | The local extracted table database plus `fate_episode.tbl`, `chara_status_fate.tbl`, and Simplified Chinese/English Fate MessagePack files | Rebuilds the bilingual 319-episode read-only catalog with source checksums | After Fate tables or localized text change; requires the Python `msgpack` package |
| `generate_infinity_rule_catalog.py` | The local 2.0.2 table database, Infinity rule/effect/difficulty tables, and Simplified Chinese/English stage MessagePack files | Rebuilds 25 localized Endless rules with substituted official parameters and preserves unknown raw effect IDs | After Endless/Infinity tables or stage text change; requires the Python `msgpack` package |
| `generate_runtime_combat_reference.py` | The local 2.0.2 table database, `damagecalcparam.msg`, `guardparam.msg`, seven battle curves, and both character damage-limit tables | Rebuilds the versioned combat-reference catalog with every raw character curve and source checksum; it does not approximate unknown interpolation | After combat configuration or damage-limit tables change; requires the Python `msgpack` package |
| `sync_reference_icons.ps1` | Extracted game assets and catalogs under `internal/backend/data/` | Rebuilds official UI icon mappings without translated-filename guesses | After catalog or bundled icon changes |
| `sync_reference_icons.repro.test.js` | The icon script and current mapping catalogs | Proves full and skills-only runs are deterministic and remove stale generated keys | Before accepting changes to the icon script |
| `measure_frontend_bundle.mjs` | Vite build manifest and `performance-budget.json` | Reports the exact initial JS/CSS gzip graph and fails CI when a budget is exceeded | Every production frontend build |
| `performance_budget.test.js` | Production frontend build and performance budget | Verifies the budget evaluator and current entry graph | CI after the frontend build |
| `qa_runtime_stress.mjs` | A debuggable Wails WebView2 target, or the Vite UI with a Wails API stub | Switches representative heavy pages, records cold/warm timing percentiles, retained/peak JS heap, DOM/document growth, long tasks, and minimized-window CPU | CI for the browser shell; packaged WebView2 before a release that changes routing, KeepAlive, large lists, or runtime polling |
| `build_patch_core.ps1` | Visual Studio Build Tools with the C++ desktop workload and `src_dll/thirdparty/libmem/` | Rebuilds the application-owned x64 runtime DLL and verifies the embedded copy used by Go | Every Windows release build; `build-windows.bat` runs it automatically |

`frontend/scripts/generate_function_assets.mjs` is the only frontend build-time asset generator. It creates the content-hashed `display` assets consumed by the application and a versioned function-art manifest before development or production builds. Unused thumb/full duplicates are not packaged; generated output is not committed.

Each script exposes its own command-line parameters and fails when a required source is missing. Source files are supplied locally; extracted game data and generated scratch workbooks are not committed.

The icon reproducibility test reads archive locations only from `GBFR_REFERENCE_ZIP` and `GBFR_GAME_TABLE_ZIP`. It skips when those local inputs are not configured and never contains a machine-specific fallback path.

Automated `*_test.go` and `*.test.js` files are release verification, not disposable test scripts, and remain in the repository. The runtime patch catalog is checked in as game-version evidence and is not regenerated from a third-party runtime table. Temporary maintainer files belong under the ignored `tools/_local/` directory; the underscore also keeps Go's `./...` package scan out of local research code.

For the runtime stress check, start the application or a Chromium page with CDP on port `9223`, then run `npm run check:runtime-stress` in `frontend/`. `--window-size 960x640` fixes the packaged-window profile, `--hidden-seconds 60` runs the release-length idle gate, and `--detector-active` keeps the persistent role-loadout detector active only for a packaged Wails target while restoring its previous state afterward. To test the Vite shell instead, pass `--url http://127.0.0.1:4174/`; that mode installs a read-only Wails API stub and is reported separately in the JSON result.
