# Maintainer tools

This directory contains only scripts that can reproduce checked-in data from auditable inputs. They are not required to run the packaged application.

| Script | Input | Checked result | When to run |
| --- | --- | --- | --- |
| `audit_sigil_tables.py` | Extracted DLC 2.0.2 sigil tables plus the application catalog | Updates or verifies `docs/evidence/sigil-table-audit-202.json` | After a game-table extraction or sigil-catalog change |
| `generate_ap_tree_panel_growth.py` | The local extracted table database and table directory | Rebuilds the character progression dataset with source checksums | After mastery, Fate, weapon-tree, or permanent-growth data changes |
| `sync_reference_icons.ps1` | Extracted game assets and the checked-in mapping catalogs | Rebuilds official UI icon mappings without translated-filename guesses | After catalog or bundled icon changes |
| `sync_reference_icons.repro.test.js` | The icon script and current mapping catalogs | Proves full and skills-only runs are deterministic and remove stale generated keys | Before accepting changes to the icon script |

Each script exposes its own command-line parameters and fails when a required source is missing. Source files are supplied locally; extracted game data and generated scratch workbooks are not committed.

Automated `*_test.go` and `*.test.js` files are release verification, not disposable test scripts, and remain in the repository. The runtime patch catalog is checked in as game-version evidence and is not regenerated from a third-party runtime table. One-off WebView drivers, importers, machine-specific field scripts, screenshots, image-generation prompts, credentials, and API responses must stay outside Git. Put temporary maintainer files under the ignored `tools/local/` directory.
