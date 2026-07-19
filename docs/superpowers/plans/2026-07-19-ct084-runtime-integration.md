# CT 0.8.4 Runtime Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Safely integrate every locally verified reversible NidasBot CT 0.8.4 direct patch, plus the highest-value 0.8.4 runtime slices, into the existing Wails/Vue parchment UI.

**Architecture:** A generated, embedded feature catalog describes nibble-wildcard AOB sites and patch slices; a single transactional runtime owns scan, enable, readback, rollback, conflict, process-instance, and detach behavior. Three Vue modes consume the same API and component, while existing save editors remain the canonical persistent-data workflows.

**Tech Stack:** Go 1.24, `golang.org/x/sys/windows`, Wails v2, Vue 3, Node test runner, Vite, PowerShell generator.

---

## File map

- Create `data/ct084_patches.json`: generated production catalog, no Lua or full CT payload.
- Create `tools/generate_ct084_patches.ps1`: deterministic CT XML → JSON generator.
- Create `ct084_catalog.go`: embedded schema, validation, lookup, public catalog API.
- Create `ct084_pattern.go`: nibble-mask parser and local/remote unique scanner.
- Create `ct084_runtime.go`: owned multi-site transactions and detach restore.
- Create `ct084_catalog_test.go`, `ct084_pattern_test.go`, `ct084_runtime_test.go`, `ct084_local_exe_test.go`.
- Create `runtime_quest_mods.go`, `runtime_action_speed.go`, `runtime_party_monitor.go`, `runtime_inventory_item.go` and focused tests.
- Create `frontend/src/components/CT084Features.vue` and `frontend/src/ct084FeaturesUi.test.js`.
- Modify `app.go`: App state, RP definition, lifecycle restore.
- Modify `frontend/src/components/PatchTool.vue`, `HomeJournal.vue`, `MiscTools.vue`, `frontend/src/i18n-ui.js`.
- Modify responsive and navigation contract tests.

### Task 1: Lock the CT-derived catalog contract

**Files:**
- Create: `ct084_catalog_test.go`
- Create: `tools/generate_ct084_patches.ps1`
- Create: `data/ct084_patches.json`

- [ ] **Step 1: Write the failing catalog test**

```go
func TestCT084ProductionCatalogHasEverySafeDirectPatch(t *testing.T) {
    catalog, err := loadCT084Catalog()
    if err != nil { t.Fatal(err) }
    if len(catalog.Features) != 60 { t.Fatalf("features=%d, want 60", len(catalog.Features)) }
    excluded := map[int]bool{31935:true, 33086:true, 31060:true, 31456:true}
    seen := map[int]bool{}
    for _, feature := range catalog.Features {
        if excluded[feature.CTID] { t.Fatalf("unsafe/duplicate CT ID %d shipped", feature.CTID) }
        if seen[feature.CTID] { t.Fatalf("duplicate CT ID %d", feature.CTID) }
        seen[feature.CTID] = true
        if len(feature.Sites) == 0 { t.Fatalf("%s has no sites", feature.ID) }
    }
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `go test -count=1 -run TestCT084ProductionCatalogHasEverySafeDirectPatch .`

Expected: FAIL because `loadCT084Catalog` is undefined.

- [ ] **Step 3: Implement the deterministic generator**

The script must parse `CheatEntry[AssemblerScript]`, accept AOB scripts with no `alloc(mem...)`, merge `NAME+hexOffset` blocks, expand `nop N`, and exclude exactly `31935,33086,31060,31456`. Its public command is:

```powershell
pwsh -NoProfile -File tools/generate_ct084_patches.ps1 `
  -InputCT 'D:\gbf\GFR_v0.8.4_CHS_祝福石修改hotfix(Uno.ct' `
  -Output 'data\ct084_patches.json'
```

Each JSON feature must contain `id`, `ctId`, `name`, `mode`, `group`, `character`, `conflicts`, and `sites`; each site contains the original AOB string and `{offset,bytes}` slices.

- [ ] **Step 4: Generate the JSON and inspect exclusions/count**

Run the command above, then:

```powershell
$doc = Get-Content data\ct084_patches.json -Raw | ConvertFrom-Json
$doc.features.Count
$doc.features | Where-Object ctId -in 31935,33086,31060,31456
```

Expected: `60`, then no rows. The earlier static pre-audit count of 59 omitted CT 32556 because its `AssemblerScript` node carries `Async="1"`; the production generator and content-lock tests include it.

- [ ] **Step 5: Commit the generator/data/test slice**

```powershell
git add tools/generate_ct084_patches.ps1 data/ct084_patches.json ct084_catalog_test.go
git commit -m "test: lock CT 0.8.4 safe patch catalog"
```

### Task 2: Parse and scan nibble-wildcard AOB patterns

**Files:**
- Create: `ct084_pattern.go`
- Create: `ct084_pattern_test.go`

- [ ] **Step 1: Write RED tests for exact, byte wildcard, nibble wildcard, chunk boundary, zero, and duplicate matches**

```go
func TestParseCT084PatternSupportsNibbleWildcards(t *testing.T) {
    got, err := parseCT084Pattern("88xxx8 4? ?F")
    if err != nil { t.Fatal(err) }
    wantMask := []byte{0xFF, 0x00, 0x0F, 0xF0, 0x0F}
    if !bytes.Equal(got.Mask, wantMask) { t.Fatalf("mask=% X", got.Mask) }
}
```

Also assert a match can start in the final `patternLen-1` bytes of one 64 KiB chunk and continue into the next.

- [ ] **Step 2: Verify RED**

Run: `go test -count=1 -run 'Test(Parse|Find)CT084Pattern' .`

Expected: FAIL because parser/scanner functions do not exist.

- [ ] **Step 3: Implement the pure parser and slice scanner**

Use one byte of mask per pattern byte: `0xFF` exact, `0xF0` high nibble, `0x0F` low nibble, `0x00` wildcard. Reject odd nibble count, empty patterns, and non-hex/non-wildcard characters.

- [ ] **Step 4: Add a remote-module scanner using the existing process lease**

`scanCT084PatternUnique` must mirror `scanPatternUnique` chunk behavior, preserve overlap, ignore unreadable chunks, and return explicit zero/multiple-match errors.

- [ ] **Step 5: Verify GREEN and commit**

Run: `go test -count=1 -run 'Test(Parse|Find|Scan)CT084Pattern' .`

Expected: PASS.

```powershell
git add ct084_pattern.go ct084_pattern_test.go
git commit -m "feat: scan CT nibble wildcard signatures"
```

### Task 3: Load and validate the embedded feature catalog

**Files:**
- Create: `ct084_catalog.go`
- Modify: `ct084_catalog_test.go`

- [ ] **Step 1: Extend RED tests**

Assert unique string IDs/CT IDs, only `combat|characters|quest` modes, patch slice bounds within pattern length, non-overlapping slices per site, known conflict targets, and valid nibble patterns.

- [ ] **Step 2: Verify RED**

Run: `go test -count=1 -run TestCT084ProductionCatalog .`

Expected: FAIL until the loader exists.

- [ ] **Step 3: Implement embedded loader and public DTO**

```go
//go:embed data/ct084_patches.json
var ct084CatalogJSON []byte

type CT084Feature struct {
    ID string `json:"id"`; CTID int `json:"ctId"`; Name string `json:"name"`
    Mode string `json:"mode"`; Group string `json:"group"`; Character string `json:"character"`
    Conflicts []string `json:"conflicts"`; Sites []CT084PatchSite `json:"sites"`
}

func (a *App) CT084GetCatalog() ([]CT084Feature, error)
```

Return a defensive copy so frontend callers cannot mutate the process-global catalog.

- [ ] **Step 4: Verify GREEN and commit**

Run: `go test -count=1 -run TestCT084ProductionCatalog .`

```powershell
git add ct084_catalog.go ct084_catalog_test.go
git commit -m "feat: embed validated CT 0.8.4 feature catalog"
```

### Task 4: Implement atomic multi-site patch ownership

**Files:**
- Create: `ct084_runtime.go`
- Create: `ct084_runtime_test.go`
- Modify: `app.go`

- [ ] **Step 1: Write transaction RED tests against a fake memory image**

Cover: one-site enable/disable, three-site enable, second-site write failure rollback, write-read mismatch rollback, conflicting feature rejection, stale owner rejection, process-instance change, foreign bytes during disable, and `restoreAllCT084PatchesLocked` reverse-order restoration.

- [ ] **Step 2: Verify RED**

Run: `go test -count=1 -run TestCT084Patch .`

Expected: FAIL because runtime methods are undefined.

- [ ] **Step 3: Add App state and DTOs**

```go
type CT084FeatureStatus struct {
    ID string `json:"id"`; Enabled bool `json:"enabled"`; Available bool `json:"available"`
    RVAs []uint64 `json:"rvas"`; CurrentBytes []string `json:"currentBytes"`; Error string `json:"error"`
}

func (a *App) CT084GetStatusesOwned(token string) ([]CT084FeatureStatus, error)
func (a *App) CT084SetEnabledOwned(token, id string, enabled bool) (CT084FeatureStatus, error)
func (a *App) CT084ReleaseOwned(token string) error
```

Store each active lease with `{PID,Created}`, exact addresses, originals, patches, and owner token. All writes hold `liveMemoryWriteMu`, `procMu`, and `runtimePatchMu` in that order.

- [ ] **Step 4: Implement scan/apply/readback/rollback/restore**

Use `installCodeHookAtomic` for each site and an outer reverse rollback for previously completed sites. If any restore is unproven, call `poisonCurrentLiveMemoryWrites` and retain the lease.

- [ ] **Step 5: Wire detach/shutdown ownership**

Add CT leases to `hasActiveRuntimeHookLeaseLocked`; call `restoreAllCT084PatchesLocked` from `charaDetachLocked` before closing the process. `CT084ReleaseOwned` must restore only features owned by the presented current token.

- [ ] **Step 6: Verify GREEN and commit**

Run: `go test -count=1 -run 'TestCT084Patch|TestCharaDetach.*CT084' .`

```powershell
git add ct084_runtime.go ct084_runtime_test.go app.go
git commit -m "feat: add owned atomic CT patch runtime"
```

### Task 5: Verify every signature against the supplied 2.0.2 EXE

**Files:**
- Create: `ct084_local_exe_test.go`
- Modify if needed: `data/ct084_patches.json`

- [ ] **Step 1: Write the environment-gated truth test**

Open the PE sections and assert every catalog site matches exactly once in the unmodified executable. Report `feature ID / CT ID / site index` on failure.

- [ ] **Step 2: Run against the real binary**

Run:

```powershell
$env:GBFR_GAME_EXE_TEST='D:\gbf\granblue_fantasy_relink.exe'
go test -count=1 -run TestCT084CatalogMatchesLocalGame202 .
```

Expected: PASS for every shipped site.

- [ ] **Step 3: Remove any non-unique/non-matching feature rather than weakening the scanner**

Regenerate with an explicit `unsafeOrUnverified` exclusion list and update the catalog count test to the proven count. Document every removal in the final report.

- [ ] **Step 4: Commit local-truth coverage**

```powershell
git add ct084_local_exe_test.go data/ct084_patches.json ct084_catalog_test.go tools/generate_ct084_patches.ps1
git commit -m "test: verify CT signatures against game 2.0.2"
```

### Task 6: Correct CP to Resonance Points / RP

**Files:**
- Modify: `app.go`
- Modify: `runtime_currency_test.go`
- Modify: `frontend/src/components/MiscTools.vue`
- Modify: `frontend/src/runtimePagesUi.test.js`

- [ ] **Step 1: Add RED assertions**

Assert offset `0x9C` uses stable ID `rp`, Chinese name `共鸣点数/RP`, and the UI contains no claim that this field is CP. Keep `cp` only as an accepted legacy write alias inside lookup.

- [ ] **Step 2: Verify RED**

Run: `go test -count=1 -run TestRuntimeCurrencyRP .` and `node --test src/runtimePagesUi.test.js` from `frontend`.

- [ ] **Step 3: Implement rename and alias migration**

Change the catalog item to `{ID:"rp", Name:"共鸣点数/RP", Offset:0x9C}` and normalize incoming legacy `cp` to `rp` before lookup.

- [ ] **Step 4: Verify GREEN and commit**

```powershell
git add app.go runtime_currency_test.go frontend/src/components/MiscTools.vue frontend/src/runtimePagesUi.test.js
git commit -m "fix: label resonance points from CT 0.8.4 truth"
```

### Task 7: Implement 0.8.4 parameterized and read-only vertical slices

**Files:**
- Create: `runtime_quest_mods.go`, `runtime_quest_mods_test.go`
- Create: `runtime_action_speed.go`, `runtime_action_speed_test.go`
- Create: `runtime_party_monitor.go`, `runtime_party_monitor_test.go`
- Create: `runtime_inventory_item.go`, `runtime_inventory_item_safety_test.go`
- Modify: `app.go`

- [ ] **Step 1: Write RED tests for quest score and action speed caves**

Assert exact cave layout, preserved original bytes, finite ranges (`score 0.1–20`, `speed 0.25–3`), owner token, enable readback, parameter update readback, and detach restoration.

- [ ] **Step 2: Implement quest score and action speed**

Translate behavior independently from the visible CT assembly. Use near allocations, RIP-relative internal data, `makeRelJump`, `installRemoteCodeHook`, and the existing poison/rollback discipline.

- [ ] **Step 3: Write RED tests for party monitor stable snapshots**

Assert three identical snapshots are required; reject changing pointers, NaN/Inf, HP outside `[0,max]`, impossible max HP, and invalid coordinate values.

- [ ] **Step 4: Implement read-only party monitor**

Expose `CT084PartyMonitorOwned(token)` returning player + three members + companion. Do not expose setters.

- [ ] **Step 5: Write RED tests for selected material/key-item transactions**

Assert known catalog ID, quantity `0–999999`, valid status, pointer revalidation, full record write/readback, save call, deterministic rollback, timeout quarantine, and write-success pointer invalidation.

- [ ] **Step 6: Implement the selected-item editor**

Reuse the existing remote-save and snapshot primitives used by sigil/wrightstone/summon. Keep material and key-item record layouts separate.

- [ ] **Step 7: Run focused and race tests, then commit**

Run: `go test -count=1 -run 'Test(QuestScore|ActionSpeed|PartyMonitor|InventoryItem)' .`

```powershell
git add runtime_quest_mods.go runtime_quest_mods_test.go runtime_action_speed.go runtime_action_speed_test.go runtime_party_monitor.go runtime_party_monitor_test.go runtime_inventory_item.go runtime_inventory_item_safety_test.go app.go
git commit -m "feat: add CT 0.8.4 runtime vertical slices"
```

### Task 8: Add the three categorized Vue pages

**Files:**
- Create: `frontend/src/components/CT084Features.vue`
- Create: `frontend/src/ct084FeaturesUi.test.js`
- Modify: `frontend/src/components/PatchTool.vue`
- Modify: `frontend/src/components/HomeJournal.vue`
- Modify: `frontend/src/i18n-ui.js`

- [ ] **Step 1: Write RED source-contract tests**

Assert routes `ctCombat`, `ctCharacters`, `ctQuest`; explicit art/sticker mappings; no production fallback to progression art; one shared component with `mode`; search; character group disclosure; conflict label; offline-only confirmation; owner acquire/release; no optimistic switch; and `aria-live` status.

- [ ] **Step 2: Verify RED**

Run: `node --test src/ct084FeaturesUi.test.js` from `frontend`.

Expected: FAIL because page/component bindings are absent.

- [ ] **Step 3: Implement shared component and Wails bindings**

The component accepts `mode: 'combat'|'characters'|'quest'`, calls `CharaAcquire`, `CT084GetCatalog`, `CT084GetStatusesOwned`, `CT084SetEnabledOwned`, and `CT084ReleaseOwned`, and renders only the chosen mode. Put monitor cards above combat, score controls in quest, and action speed in characters.

- [ ] **Step 4: Add navigation/meta without changing the three top-level groups**

Insert the new pages under `memory`; keep all save pages in their current group. Update tool descriptions/speakers and do not introduce a dark CT theme.

- [ ] **Step 5: Verify GREEN and commit**

Run: `node --test src/ct084FeaturesUi.test.js src/responsiveShell.test.js src/shellToolsAtomicUi.test.js`.

```powershell
git add frontend/src/components/CT084Features.vue frontend/src/ct084FeaturesUi.test.js frontend/src/components/PatchTool.vue frontend/src/components/HomeJournal.vue frontend/src/i18n-ui.js
git commit -m "feat: add categorized CT runtime pages"
```

### Task 9: Responsive, lifecycle, and full regression verification

**Files:**
- Modify: `frontend/src/ct084FeaturesUi.test.js`
- Modify: `frontend/src/responsiveShell.test.js`
- Modify: generated Wails bindings only through the project build command.

- [ ] **Step 1: Add responsive RED assertions**

Assert 375/768 single-column behavior, 1024/1440 dual-pane behavior, one main scroll container, sticky action reachability, no fixed-width card overflow, keyboard focus, and reduced-motion handling.

- [ ] **Step 2: Implement minimal CSS in the shared component/design tokens**

Reuse `ui-card`, `ui-panel`, `ui-seg`, `ui-btn`, `ui-input`, and project spacing/color tokens; add only CT-page layout selectors.

- [ ] **Step 3: Run the complete gates**

```powershell
gofmt -w ct084_*.go runtime_quest_mods*.go runtime_action_speed*.go runtime_party_monitor*.go runtime_inventory_item*.go app.go
go test -mod=mod -count=1 -timeout=10m ./...
Set-Location frontend
node --test src/*.test.js
npm run build
Set-Location ..
wails build -platform windows/amd64 -clean
```

Expected: all tests pass, Vite and Wails exit 0.

- [ ] **Step 4: Commit the verified integration**

```powershell
git add -u
git add ct084_*.go runtime_quest_mods*.go runtime_action_speed*.go runtime_party_monitor*.go runtime_inventory_item*.go data/ct084_patches.json tools/generate_ct084_patches.ps1 frontend/src/components/CT084Features.vue frontend/src/ct084FeaturesUi.test.js
git commit -m "feat: complete CT 0.8.4 runtime integration"
```
