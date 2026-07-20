# Offline Closeout and Formula Sampler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close every currently actionable offline gap and add a strictly read-only A/B/A/B formula sampler for later GBFR 2.0.2 sessions.

**Architecture:** Extend the existing loadout transaction with prepared inline resource mutations, add a dependency-free virtual grid, and build the sampler on a new read-only process adapter rather than the CT monitor lifecycle. Persist no raw memory or personal paths; export a deterministic redacted bundle.

**Tech Stack:** Go 1.24, Wails v2, Vue 3, Node test runner, Windows process APIs, JSON/NDJSON/ZIP.

---

### Task 1: Virtualize the real 1269-item factor bag

**Files:**
- Create: `frontend/src/loadoutVirtualGrid.js`
- Create: `frontend/src/loadoutVirtualGrid.test.js`
- Modify: `frontend/src/components/LoadoutEditor.vue`
- Modify: `frontend/src/loadoutResponsiveUi.test.js`
- Modify: `.qa-webview2/cdp-loadout-qa.mjs`

- [ ] Write failing pure-function tests for column resolution, aligned start indexes, bottom reachability, empty results, one-column reflow and a 1269-item render ceiling of 60.
- [ ] Run `node --test frontend/src/loadoutVirtualGrid.test.js frontend/src/loadoutResponsiveUi.test.js` and confirm failure because the helper and virtual viewport do not exist.
- [ ] Implement `resolveColumnCount` and `resolveVirtualWindow` with clamped inputs, row-aligned slices and two-row overscan.
- [ ] Replace `v-for="s in filteredSigils"` with a ResizeObserver-driven viewport, spacer and `visibleSigils` slice; reset scroll on query/data changes and disconnect the observer on unmount.
- [ ] Add fixed-height parchment scroll-region CSS, stable scrollbar gutter, fixed card row height and single-line trait truncation with full `title` text.
- [ ] Extend native QA to assert total 1269, DOM cards 1..60, scrollable range, last-card selection, search reset and zero overlap/overflow.
- [ ] Run the focused Node tests and commit only these files.

### Task 2: Add single-transaction inline weapon and summon mutations

**Files:**
- Create: `summon_traits.go`
- Create: `summon_traits_test.go`
- Create: `loadout_inline_resources.go`
- Create: `loadout_inline_resources_test.go`
- Modify: `loadout_write.go`
- Modify: `progression_editor.go`
- Modify: `loadout_stats.go`
- Modify: `loadout_write_test.go`
- Modify: `loadout_stats_test.go`

- [ ] Write failing tests for stale SlotID/UnitID/hash snapshots, the exact four legal seventh-stage hashes, summon natural-pool validation, legacy-main preservation, conflicting duplicate edits and no-buffer-mutation on preflight failure.
- [ ] Run the focused Go tests and confirm failures are caused by missing request DTOs and validators.
- [ ] Add `LoadoutApplyRequest`, `LoadoutWeaponInlineEdit` and `LoadoutSummonInlineEdit`; keep legacy `LoadoutApply` as a compatibility adapter.
- [ ] Extract summon state validation so live-memory and save adapters share the same natural/legacy rules.
- [ ] Resolve inline edits by stable SlotID to one exact UnitID, verify expected snapshots, prepare mutations, and only then alter the shared SaveData buffer.
- [ ] Reuse the proven seventh-stage map and only patch `2818[4]`; assert all other weapon bytes remain unchanged.
- [ ] Patch only target summon `1458/1459/1460` fields, preserve other instances, perform one save write, then reopen and verify every dirty field.
- [ ] Run focused Go tests and commit only backend transaction files.

### Task 3: Add copy-on-write inline editors to the loadout UI

**Files:**
- Create: `frontend/src/loadoutInlineDrafts.js`
- Create: `frontend/src/loadoutInlineDrafts.test.js`
- Modify: `frontend/src/components/LoadoutEditor.vue`
- Modify: `frontend/src/loadoutEditorUi.test.js`
- Modify: `frontend/src/loadoutResponsiveUi.test.js`
- Modify: `frontend/src/loadoutBindingSync.test.js`
- Regenerate: `frontend/wailsjs/go/main/App.js`
- Regenerate: `frontend/wailsjs/go/main/App.d.ts`
- Regenerate: `frontend/wailsjs/go/models.ts`

- [ ] Write failing tests proving contexts are immutable, dirty drafts serialize once, reset restores snapshots, only four weapon options render, and legacy summon mains stay locked.
- [ ] Run focused Node tests and confirm missing helpers/markup fail.
- [ ] Enrich loadout context with weapon UnitID/progression fields and summon catalog options.
- [ ] Add collapsible weapon and summon edit sections below their selectors, with legality state, global-instance impact text and responsive wide/narrow layouts.
- [ ] Submit only dirty drafts through `LoadoutApplyRequest`; after parent reload, rehydrate from the reopened save.
- [ ] Regenerate Wails bindings through the project build, run focused tests, and commit UI/bindings.

### Task 4: Close offline naming, sizing, long-name and portrait gaps

**Files:**
- Modify: `data/summon_skills.json`
- Modify: `README.md`
- Modify: `README_EN.md`
- Modify: `frontend/src/components/LoadoutViewer.vue`
- Modify: `frontend/src/components/LoadoutEditor.vue`
- Modify: `frontend/src/components/PatchTool.vue`
- Modify: `frontend/src/responsiveShell.test.js`
- Modify: `frontend/src/loadoutResponsiveUi.test.js`
- Modify: `frontend/src/gameAssetIcons.test.js`

- [ ] Write failing tests for the six canonical summon names, long-name nowrap/ellipsis/title, bounded controls and bottom/right portrait anchoring.
- [ ] Confirm the focused tests fail against the current top-anchored and overwide implementation.
- [ ] Apply canonical names and update the outdated 82-main-trait README statement.
- [ ] Add long-name wrappers and max-width form primitives without changing the parchment product shell.
- [ ] Replace top portrait anchoring with semantic right/bottom variables while retaining the art-free dedicated loadout editor.
- [ ] Audit remaining icon gaps against local authoritative ZIP/table assets; add mappings only when exact bytes/IDs prove identity, otherwise retain the neutral fallback and update exact counts.
- [ ] Run focused tests and commit.

### Task 5: Build the strictly read-only formula sampler core

**Files:**
- Create: `readonly_game_process.go`
- Create: `readonly_game_process_test.go`
- Create: `formula_sampler.go`
- Create: `formula_sampler_test.go`
- Create: `formula_sampler_scan.go`
- Create: `formula_sampler_scan_test.go`
- Modify: `runtime_character_panel.go`
- Modify: `runtime_character_panel_test.go`

- [ ] Write failing tests that the access mask is exactly query+read, all failure paths close handles, process identity/version changes abort, snapshots are bit-exact, and sampler source contains none of the banned write/network symbols.
- [ ] Write failing fake-memory tests for the known probe fields, status-object i32/f32 scanning, A/B/A/B reversible candidate scoring, control filtering, scan budgets and cancellation.
- [ ] Run focused Go tests and confirm missing types/functions fail.
- [ ] Extract a reusable character-status locator from the current panel reader without widening process permissions.
- [ ] Implement the read-only adapter, short-lived identity guards, session state machine, known probes and bounded status-object scan.
- [ ] Keep full-process scanning disabled by default and fail closed on unreadable/guard regions or budget exhaustion.
- [ ] Run focused Go tests and commit the sampler core.

### Task 6: Add deterministic redacted export and the sampler page

**Files:**
- Create: `formula_sample_bundle.go`
- Create: `formula_sample_bundle_test.go`
- Create: `frontend/src/formulaSamplerView.js`
- Create: `frontend/src/formulaSamplerView.test.js`
- Create: `frontend/src/components/FormulaSampler.vue`
- Create: `frontend/src/formulaSamplerNavigation.test.js`
- Modify: `app.go`
- Modify: `frontend/src/components/PatchTool.vue`
- Modify: `frontend/src/components/HomeJournal.vue`
- Modify: `frontend/src/i18n-ui.js`
- Modify: `frontend/src/loadoutBindingSync.test.js`
- Regenerate: `frontend/wailsjs/go/main/App.js`
- Regenerate: `frontend/wailsjs/go/main/App.d.ts`
- Regenerate: `frontend/wailsjs/go/models.ts`

- [ ] Write failing Go golden tests for deterministic member order, SHA256SUMS and removal of PID, absolute paths, pointers, local absolute time, free notes and raw memory.
- [ ] Write failing Node tests for strict DTO normalization, A1/B1/A2/B2 state transitions, no capture before connection, no export before a reversible pair, and guaranteed stop on unmount.
- [ ] Implement `.gbfr-formula-sample.zip` export with manifest/events/observations/candidates/model/redaction report/README/checksums and no network path.
- [ ] Implement an independent parchment sampler page under memory monitoring; do not import `CharaAcquire`, selected-item hooks or CT monitor lifecycle methods.
- [ ] Add live four-value preview, experiment type, A/B/A/B timeline, residual table, advanced scan disclosure and local export.
- [ ] Regenerate Wails bindings, run focused tests, and commit.

### Task 7: Make formula evidence granular and document the later game workflow

**Files:**
- Modify: `loadout_final_stats.go`
- Modify: `loadout_final_stats_test.go`
- Modify: `loadout_sim.go`
- Modify: `docs/FORMULAS_2.0.2.md`
- Modify: `docs/FINAL_AUDIT_2026-07-19.md`
- Create: `docs/FORMULA_SAMPLER_2.0.2.md`

- [ ] Write failing tests for per-field evidence, structured exclusion reasons and float32 trace steps.
- [ ] Replace the single verification interpretation with field-level evidence while retaining backward-compatible summary output.
- [ ] Move the hard-coded Io weapon-skill curve into versioned data and expose excluded conditional effects structurally.
- [ ] Document the exact A1/B1/A2/B2 operation sequence and mark runtime panel offsets as candidates until the exported real-process bundle validates them.
- [ ] Run focused tests and commit.

### Task 8: Full offline verification

**Files:**
- Modify only if a failing verification exposes an in-scope defect.

- [ ] Run `node --test frontend/src/*.test.js` and record pass/fail totals.
- [ ] Run `go test -timeout 10m ./...`, `go vet ./...`, `npm --prefix frontend run build`, and the Wails production build.
- [ ] Run the real-save copy tests and confirm the original test save hash is unchanged.
- [ ] Run loadout WebView2 QA with the 1269-item bag, including 1199px and 1200px breakpoints.
- [ ] Run the 21-page multi-size application matrix and inspect failures from the application capture path only.
- [ ] Re-read this plan and the design requirement-by-requirement; report any remaining item that genuinely requires the user's later game session.

