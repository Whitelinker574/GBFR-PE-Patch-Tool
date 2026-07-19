# Functional Page Art Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce, validate, and integrate six function-specific portraits and five matching guide stickers without repeated page mappings or edge-clipped source art.

**Architecture:** Six validated page briefs drive the Portable two-stage portrait and sticker jobs. Accepted transparent PNGs are visually reviewed, mechanically validated, converted to production WebP, then mapped explicitly in `PatchTool.vue` with per-character crop tokens and source-contract tests.

**Tech Stack:** Portable GBFR portrait skill, QNAIGC GPT Image 2 runner, Python validators supplied by the workflow, FFmpeg WebP encoder, Vue 3, Node test runner.

---

## File map

- Create `tools/portrait-briefs/*.json`: six reproducible page briefs without secrets.
- Ignore `tools/portrait-jobs/`: manifests, line art, full PNGs, metadata, and API responses.
- Create/replace 11 production files under `frontend/src/assets/gbfr/cutouts/` and `stickers/`.
- Modify `frontend/src/components/PatchTool.vue`: imports, explicit mappings, speaker copy, crop tokens.
- Create `frontend/src/pageArtMapping.test.js` and extend responsive tests.
- Create `docs/CT084_ART_PROVENANCE_2026-07-19.md` with prompts, references, validation, and hashes.

### Task 1: Lock page-to-art mapping before generation

**Files:**
- Create: `frontend/src/pageArtMapping.test.js`
- Modify: `.gitignore`

- [ ] **Step 1: Write the failing mapping test**

```js
test('every tool page has explicit portrait and sticker mappings', () => {
  const source = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
  for (const id of ['progression','sigil','sigilMemory','loadout','loadoutPresets','wrightstone','wrightstoneMemory','summon','overlimit','runtime','chara','save','compatibility','legacyRuntime','monster','patch','language','ctCombat','ctCharacters','ctQuest']) {
    assert.match(source, new RegExp(`\\b${id}:`))
  }
  assert.doesNotMatch(source, /functionArt\[activeTab\.value\]\s*\|\|\s*progressionArt/)
  assert.doesNotMatch(source, /functionStickers\[activeTab\.value\]\s*\|\|\s*progressionSticker/)
})
```

- [ ] **Step 2: Verify RED**

Run: `node --test src/pageArtMapping.test.js` from `frontend`.

Expected: FAIL for missing CT mappings and fallback expressions.

- [ ] **Step 3: Ignore only generated job payloads**

Add `/tools/portrait-jobs/` to `.gitignore`; do not ignore `tools/portrait-briefs/` or production assets.

- [ ] **Step 4: Commit the guard**

```powershell
git add .gitignore frontend/src/pageArtMapping.test.js
git commit -m "test: require explicit function page art mappings"
```

### Task 2: Author and validate all six page briefs

**Files:**
- Create: `tools/portrait-briefs/loadout-live.json`
- Create: `tools/portrait-briefs/loadout-presets.json`
- Create: `tools/portrait-briefs/wrightstone-memory.json`
- Create: `tools/portrait-briefs/ct-combat.json`
- Create: `tools/portrait-briefs/ct-characters.json`
- Create: `tools/portrait-briefs/ct-quest.json`

- [ ] **Step 1: Read each real component and fill the complete schema**

Use the exact characters/actions fixed by the design: Fraux/12-slot live recording, Gran/offline full loadout, Maglielle/three-slot live wrightstone, Vane/combat-rule training, Djeeta/character-mechanic roster, Yodarha/quest map and treasure.

- [ ] **Step 2: Retrieve only exact gameplay references**

Use `scripts/search_gameplay_assets.py` against the local semantic catalog by visible name/ID. Attach zero to three images per portrait and state a narrow use such as `sigil emblem only` or `exact weapon geometry only`.

- [ ] **Step 3: Validate every brief**

```powershell
$skill='D:\gbf\.tmp\portrait-skill-v3-audit-20260719\GBFR-Relink-Portrait-Skill-Portable-20260718\generate-gbfr-relink-portraits'
Get-ChildItem tools\portrait-briefs\*.json | ForEach-Object {
  python "$skill\scripts\validate_page_brief.py" $_.FullName
  if ($LASTEXITCODE -ne 0) { throw "brief validation failed: $($_.Name)" }
}
```

Expected: six successful validations.

- [ ] **Step 4: Commit reproducible briefs**

```powershell
git add tools/portrait-briefs/*.json
git commit -m "docs: define function-specific portrait briefs"
```

### Task 3: Generate and approve the six portraits

**Files:**
- Generated, ignored: `tools/portrait-jobs/loadout-live`, `loadout-presets`, `wrightstone-memory`, `ct-combat`, `ct-characters`, and `ct-quest`.
- Production: six WebP cutouts.

- [ ] **Step 1: Prepare all portrait jobs**

Prepare every brief with the exact page slug:

```powershell
$pages=@('loadout-live','loadout-presets','wrightstone-memory','ct-combat','ct-characters','ct-quest')
foreach($page in $pages) {
  python "$skill\scripts\prepare_portrait_job.py" --brief "tools\portrait-briefs\$page.json" --out "tools\portrait-jobs\$page"
  if($LASTEXITCODE -ne 0) { throw "portrait job preparation failed: $page" }
}
```

- [ ] **Step 2: Generate line art, inspect at original resolution, and reject defects**

Run the following one page at a time, then use `view_image` on the produced `*-lineart.png` before advancing that page. Reject unrelated actions, copied official pose, ambiguous props, broken hands, weapons, or edge contact.

```powershell
foreach($page in $pages) {
  python "$skill\scripts\run_qnaigc.py" --manifest "tools\portrait-jobs\$page\01-lineart.json"
  if($LASTEXITCODE -ne 0) { throw "line art generation failed: $page" }
}
```

- [ ] **Step 3: Generate the final transparent source**

Run the final manifest only after its line art is accepted, then use `view_image` at original detail and verify identity, anatomy, prop count, connections, gaze, weapon continuity, and margins.

```powershell
foreach($page in $pages) {
  python "$skill\scripts\run_qnaigc.py" --manifest "tools\portrait-jobs\$page\02-final.json"
  if($LASTEXITCODE -ne 0) { throw "portrait generation failed: $page" }
}
```

- [ ] **Step 4: Validate and convert accepted portraits**

```powershell
$portraitMap=@{
  'loadout-live'=@('fraux','frontend\src\assets\gbfr\cutouts\loadout-live-official-edge-safe.webp')
  'loadout-presets'=@('gran','frontend\src\assets\gbfr\cutouts\loadout-presets-official-edge-safe.webp')
  'wrightstone-memory'=@('maglielle','frontend\src\assets\gbfr\cutouts\wrightstone-memory-official-edge-safe.webp')
  'ct-combat'=@('vane','frontend\src\assets\gbfr\cutouts\ct-combat-official-edge-safe.webp')
  'ct-characters'=@('djeeta','frontend\src\assets\gbfr\cutouts\ct-characters-official-edge-safe.webp')
  'ct-quest'=@('yodarha','frontend\src\assets\gbfr\cutouts\ct-quest-official-edge-safe.webp')
}
foreach($page in $pages) {
  $slug=$portraitMap[$page][0]
  $source="tools\portrait-jobs\$page\$slug-portrait.png"
  $target=$portraitMap[$page][1]
  python "$skill\scripts\validate_portrait.py" $source
  if($LASTEXITCODE -ne 0) { throw "portrait validation failed: $page" }
  ffmpeg -y -i $source -frames:v 1 -c:v libwebp -quality 92 -compression_level 6 $target
  if($LASTEXITCODE -ne 0) { throw "WebP conversion failed: $page" }
}
```

Production targets:

- `cutouts/loadout-live-official-edge-safe.webp`
- `cutouts/loadout-presets-official-edge-safe.webp`
- `cutouts/wrightstone-memory-official-edge-safe.webp`
- `cutouts/ct-combat-official-edge-safe.webp`
- `cutouts/ct-characters-official-edge-safe.webp`
- `cutouts/ct-quest-official-edge-safe.webp`

- [ ] **Step 5: Re-run the validator on decoded production files and commit**

Expected: all six pass.

```powershell
git add frontend/src/assets/gbfr/cutouts/*.webp
git commit -m "feat: add function-specific edge-safe portraits"
```

### Task 4: Generate and approve five matching guide stickers

**Files:**
- Generated, ignored: sticker manifests/PNGs.
- Production: five WebP stickers; retain existing `stickers/loadout.webp` for Fraux live recording.

- [ ] **Step 1: Prepare and generate stickers**

Run the exact five sticker jobs:

```powershell
$stickerPages=@('loadout-presets','wrightstone-memory','ct-combat','ct-characters','ct-quest')
foreach($page in $stickerPages) {
  python "$skill\scripts\prepare_sticker_job.py" --brief "tools\portrait-briefs\$page.json" --out "tools\portrait-jobs\$page"
  if($LASTEXITCODE -ne 0) { throw "sticker job preparation failed: $page" }
  python "$skill\scripts\run_qnaigc.py" --manifest "tools\portrait-jobs\$page\03-sticker.json"
  if($LASTEXITCODE -ne 0) { throw "sticker generation failed: $page" }
}
```

- [ ] **Step 2: Inspect original and 150×160 preview**

Reject text, letters, speech bubbles, extra limbs, wrong character traits, wrong prop count, edge contact, or an emotion that disappears when reduced.

- [ ] **Step 3: Validate alpha and convert to WebP**

Encode to:

- `stickers/loadout-presets.webp`
- `stickers/wrightstone-memory.webp`
- `stickers/ct-combat.webp`
- `stickers/ct-characters.webp`
- `stickers/ct-quest.webp`

```powershell
$stickerMap=@{
  'loadout-presets'=@('gran','frontend\src\assets\gbfr\stickers\loadout-presets.webp')
  'wrightstone-memory'=@('maglielle','frontend\src\assets\gbfr\stickers\wrightstone-memory.webp')
  'ct-combat'=@('vane','frontend\src\assets\gbfr\stickers\ct-combat.webp')
  'ct-characters'=@('djeeta','frontend\src\assets\gbfr\stickers\ct-characters.webp')
  'ct-quest'=@('yodarha','frontend\src\assets\gbfr\stickers\ct-quest.webp')
}
foreach($page in $stickerPages) {
  $slug=$stickerMap[$page][0]
  $source="tools\portrait-jobs\$page\$slug-sticker.png"
  $target=$stickerMap[$page][1]
  ffmpeg -y -i $source -frames:v 1 -c:v libwebp -quality 92 -compression_level 6 $target
  if($LASTEXITCODE -ne 0) { throw "sticker WebP conversion failed: $page" }
}
```

- [ ] **Step 4: Commit accepted stickers**

```powershell
git add frontend/src/assets/gbfr/stickers/*.webp
git commit -m "feat: add function-specific guide stickers"
```

### Task 5: Integrate assets and tune page-specific crops

**Files:**
- Modify: `frontend/src/components/PatchTool.vue`
- Modify: `frontend/src/i18n-ui.js`
- Modify: `frontend/src/pageArtMapping.test.js`
- Modify: `frontend/src/responsiveShell.test.js`

- [ ] **Step 1: Add imports and explicit mappings**

Map `loadout` to the new Fraux portrait + existing Fraux sticker; `loadoutPresets` to Gran; `wrightstoneMemory` to Maglielle; and each CT page to its own pair. Keep offline `wrightstone` on Ferry.

- [ ] **Step 2: Update speakers and HTML/CSS bubble copy**

Use the sticker character as the visible speaker. Do not bake text into image assets.

- [ ] **Step 3: Add per-page crop tokens**

Set CSS custom properties on the art stage, for example `--art-scale`, `--art-x`, `--art-y`; tune each new page independently without shrinking all portraits.

- [ ] **Step 4: Verify mapping and responsive tests GREEN**

Run: `node --test src/pageArtMapping.test.js src/responsiveShell.test.js`.

- [ ] **Step 5: Commit integration**

```powershell
git add frontend/src/components/PatchTool.vue frontend/src/i18n-ui.js frontend/src/pageArtMapping.test.js frontend/src/responsiveShell.test.js
git commit -m "feat: map unique art to every function page"
```

### Task 6: Visual QA, provenance, and full build

**Files:**
- Create: `docs/CT084_ART_PROVENANCE_2026-07-19.md`
- Modify only if QA finds defects: component CSS or the single affected asset.

- [ ] **Step 1: Capture application-internal screenshots**

Use the Wails/WebView2 QA path, not desktop-region screenshots. Capture each changed page at 375, 768, 1024, 1440 widths, minimum height, full-screen, and right-art-collapsed states.

- [ ] **Step 2: Audit layout and image uniqueness**

Check left/right coordination, speaker bubble, controls, portrait crop, transparent edge, and sticker size. Hash production images and assert no same-byte duplicate; verify every new page has its own mapping.

- [ ] **Step 3: Write provenance report**

Record each page brief path, character, prompt roles, gameplay references, generation mode, accepted output, validation result, production SHA-256, and any repair iteration. Never include the API key.

- [ ] **Step 4: Run full frontend and Windows build**

```powershell
Set-Location frontend
node --test src/*.test.js
npm run build
Set-Location ..
wails build -platform windows/amd64 -clean
```

Expected: tests pass and both builds exit 0.

- [ ] **Step 5: Commit verified art delivery**

```powershell
git add docs/CT084_ART_PROVENANCE_2026-07-19.md
git add -u frontend/src
git commit -m "docs: verify CT page art provenance and QA"
```
