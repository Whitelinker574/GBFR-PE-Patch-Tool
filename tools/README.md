# Maintainer tools

Only reproducible maintenance tools belong here:

- `audit_sigil_tables.py` verifies checked-in sigil evidence against extracted 2.0.2 tables.
- `generate_ap_tree_panel_growth.py` rebuilds the character progression dataset.
- `generate_ct084_patches.ps1` and `ct084_originals/` rebuild and verify the audited CT patch catalog.
- `sync_reference_icons.ps1` and its regression test rebuild authoritative UI icon mappings.
- `qa_formula_sampler.mjs` and `qa_page_matrix_current.mjs` run manual WebView UI checks against a local development build.

Machine-specific field scripts, screenshots, generated workbooks, image-generation prompts and API credentials must stay outside Git. Put temporary maintainer files under an ignored `tools/local/` directory.
