# Maintainer tools

This directory contains the maintainer scripts that reproduce checked-in data from auditable inputs, plus one automated test that verifies the icon script stays deterministic. They are not required to run the packaged application.

| File | Input | Checked result | When to run |
| --- | --- | --- | --- |
| `audit_sigil_tables.py` | Extracted DLC 2.0.2 sigil tables plus `internal/backend/data/sigils.json` and adjacent `traits.json` | Prints or writes the audit JSON; `--fix-catalog` deterministically rebuilds both catalogs and drops sigils absent from `gem.tbl` | After a game-table extraction or sigil-catalog change |
| `generate_ap_tree_panel_growth.py` | The local extracted table database, table directory, and explicit dataset-version label | Rebuilds `internal/backend/data/ap_tree_panel_growth.json` with source checksums | After mastery, Fate, weapon-tree, or permanent-growth data changes |
| `sync_reference_icons.ps1` | Extracted game assets and catalogs under `internal/backend/data/` | Rebuilds official UI icon mappings without translated-filename guesses | After catalog or bundled icon changes |
| `sync_reference_icons.repro.test.js` | The icon script and current mapping catalogs | Proves full and skills-only runs are deterministic and remove stale generated keys | Before accepting changes to the icon script |

Each script exposes its own command-line parameters and fails when a required source is missing. Source files are supplied locally; extracted game data and generated scratch workbooks are not committed.

The icon reproducibility test reads archive locations only from `GBFR_REFERENCE_ZIP` and `GBFR_GAME_TABLE_ZIP`. It skips when those local inputs are not configured and never contains a machine-specific fallback path.

Automated `*_test.go` and `*.test.js` files are release verification, not disposable test scripts, and remain in the repository. The runtime patch catalog is checked in as game-version evidence and is not regenerated from a third-party runtime table. Temporary maintainer files belong under the ignored `tools/local/` directory.
