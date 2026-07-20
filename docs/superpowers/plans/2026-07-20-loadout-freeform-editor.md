# Loadout Freeform Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make factor construction freeform, derive mastery direction from selected nodes, restore visible mastery effects, and eliminate narrow-column overflow without weakening structural save safety.

**Architecture:** Separate encodability from natural-game legality. The frontend owns free selection and warning presentation; Go prepares any known writable factor while compliance classifies non-natural combinations as writable warnings. Mastery selection stays lossless, while summary code derives direction and activation effects from the selected hashes.

**Tech Stack:** Vue 3, Wails, Go, Node test runner, Go test, Playwright/WebView2 QA.

---

### Task 1: Lock the desired UI contracts

**Files:**
- Modify: `frontend/src/loadoutCatalogFilters.test.js`
- Modify: `frontend/src/loadoutMastery.test.js`
- Modify: `frontend/src/loadoutEditorUi.test.js`
- Modify: `frontend/src/loadoutComplianceUi.test.js`

- [ ] Add failing assertions that the constructor excludes bag templates, both traits are searchable/editable, direction buttons and filtering are absent, mastery effects are visible, compliance warnings do not disable save, and weapon controls use one column.
- [ ] Run `npm test -- --test-name-pattern="constructor|mastery|compliance|weapon"` from `frontend` and confirm the new assertions fail for the current implementation.

### Task 2: Lock freeform backend encoding

**Files:**
- Modify: `loadout_write_test.go`
- Modify: `loadout_mastery_test.go`

- [ ] Add a test that a known factor accepts a known non-default primary trait, arbitrary known secondary trait, and writable non-natural levels.
- [ ] Add a test that compliance reports such a combination as writable warning rather than impossible.
- [ ] Change mastery tests to require mixed-direction and partial known-node vectors to remain writable while duplicate, foreign, unknown, and over-cap nodes still fail.
- [ ] Run the focused Go tests and confirm they fail for the current strict implementation.

### Task 3: Implement the freeform factor constructor

**Files:**
- Modify: `frontend/src/components/LoadoutEditor.vue`
- Modify: `frontend/src/loadoutCatalogFilters.js`
- Modify: `loadout_write.go`
- Modify: `loadout_compliance.go`

- [ ] Load the complete trait catalog once, expose searchable primary and secondary selectors, preserve independent levels, and stop cloning bag templates in construction mode.
- [ ] Prepare any catalog factor with any known trait IDs in the writable level range, retaining only structural validation.
- [ ] Classify deviations from the natural factor definition as `forced`/warning with `writable: true`; reserve `impossible` for structural failures.
- [ ] Remove status suffixes from selector option labels and keep the warning detail in the compliance panel.
- [ ] Run the focused frontend and Go tests until green.

### Task 4: Implement lossless mastery inference and effects

**Files:**
- Modify: `frontend/src/loadoutMastery.js`
- Modify: `frontend/src/components/LoadoutEditor.vue`
- Modify: `loadout_mastery.go`

- [ ] Make node selectability independent of a manually selected direction and make direction application lossless.
- [ ] Derive the displayed direction from selected second-rank nodes after every edit and hydration; remove direction buttons and all direction-based disabling/deletion.
- [ ] Return the selected specialization root effect for each category and render it directly in the three-direction summary.
- [ ] Permit partial and mixed-direction vectors while retaining duplicate, owner, known-node, and per-rank capacity checks.
- [ ] Run focused frontend and Go mastery tests until green.

### Task 5: Fix responsive overflow and warning UX

**Files:**
- Modify: `frontend/src/components/LoadoutEditor.vue`

- [ ] Make weapon skill edit rows single-column inside the narrow setup rail with `min-width: 0`, full-width selectors, and wrapped labels.
- [ ] Rename the compliance presentation from a hard gate to a write warning/check and ensure semantic warnings never disable the persistent save action.
- [ ] Run UI source-contract tests and the full frontend test suite.

### Task 6: End-to-end verification

**Files:**
- Update only generated build output under `build/bin` if the repository convention tracks it.

- [ ] Run `go test ./...` and `npm test` plus `npm run build`.
- [ ] Start the actual Wails application with the real test save, verify factor construction and save/readback, and inspect at 1280×720, 1600×900, and maximized dimensions.
- [ ] Confirm no horizontal overflow, no deleted mastery selections, visible specialization effects, clean Chinese/English isolation, and writable warnings.
- [ ] Build the release executable and record path, size, and SHA-256.
