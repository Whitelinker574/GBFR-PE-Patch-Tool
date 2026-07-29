import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')
const archive = readFileSync(new URL('./components/LogsBattleArchive.vue', import.meta.url), 'utf8')
const atlas = readFileSync(new URL('./components/SigilAtlas.vue', import.meta.url), 'utf8')
const optimizer = readFileSync(new URL('./components/LoadoutOptimizer.vue', import.meta.url), 'utf8')
const optimizerWorker = readFileSync(new URL('./loadoutOptimizer.worker.js', import.meta.url), 'utf8')
const optimizerConfig = readFileSync(new URL('./loadoutScenarioConfig.js', import.meta.url), 'utf8')
const workshop = readFileSync(new URL('./components/LoadoutShareWorkshop.vue', import.meta.url), 'utf8')
const editor = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')

test('loadout workspace keeps only cross-loadout tools in the top navigation', () => {
  for (const id of ['view', 'atlas', 'battles']) {
    assert.match(viewer, new RegExp(`id: '${id}'`))
  }
  assert.doesNotMatch(viewer, /id: 'optimizer'|id: 'share'/)
  assert.match(viewer, /grid-template-columns:repeat\(3,minmax\(0,1fr\)\)/)
  assert.match(viewer, /@container loadout-viewer \(max-width:560px\)[\s\S]*?\.loadout-subnav \{ grid-template-columns:1fr; \}/)
})

test('character-specific optimization and share image live inside expanded loadout details', () => {
  assert.match(viewer, /class="loadout-detail-actions"/)
  assert.match(viewer, /编辑此配装/)
  assert.match(viewer, /优化这套因子/)
  assert.match(viewer, /生成分享图/)
  assert.match(viewer, /<LoadoutOptimizer embedded[\s\S]*?:base-loadout="lo"[\s\S]*?@apply="applyOptimizerPlan"/)
  assert.match(viewer, /<LoadoutShareWorkshop embedded :group="shareGroup\(lo\)"/)
  assert.match(viewer, /@container loadout-viewer \(max-width:560px\)[\s\S]*?\.loadout-detail-actions \{ grid-template-columns:1fr; \}/)
})

test('save and character selectors state their downstream ownership and omit manual refresh', () => {
  assert.match(viewer, /第一步 · 选择来源存档/)
  assert.match(viewer, /后续角色、库存优化和写入目标都绑定这里选择的存档/)
  assert.match(viewer, /第二步 · 选择角色/)
  assert.match(viewer, /优化和分享都会使用当前角色数据/)
  assert.doesNotMatch(viewer, />刷新<\/button>/)
})

test('large workspace tools stay outside the initial page bundle', () => {
  for (const component of ['LoadoutEditor', 'SigilAtlas', 'LoadoutOptimizer', 'LoadoutShareWorkshop', 'LogsBattleArchive']) {
    assert.match(viewer, new RegExp(`const ${component} = defineAsyncComponent`))
  }
  assert.doesNotMatch(viewer, /import LoadoutEditor from '\.\/LoadoutEditor\.vue'/u)
})

test('catalog-heavy atlas stays mounted while contextual tools stay with their loadout', () => {
  assert.match(viewer, /<KeepAlive v-else-if="mode === 'atlas'">/)
  assert.match(viewer, /<SigilAtlas class="loadout-tool-subpage"/)
  assert.match(viewer, /cardToolOpen\(lo, 'optimizer'\)/)
  assert.match(viewer, /cardToolOpen\(lo, 'share'\)/)
})

test('optimizer apply enters the requested preset and stages a draft before save', () => {
  assert.match(optimizer, /emit\('apply'/)
  assert.match(optimizer, /一键载入首选方案/)
  assert.match(optimizer, /applyResult\(primaryResult\)/)
  assert.match(optimizer, /载入到 \$\{charaName/)
  assert.match(viewer, /pendingOptimizerPlan\.value = \{ \.\.\.payload, requestId: Date\.now\(\) \}/)
  assert.match(viewer, /:preferred-unit-id="editorTargetUnitId"/)
  assert.match(editor, /function stageOptimizerPlan\(payload\)/)
  assert.match(editor, /optimizerEquipmentDraft\(payload\?\.result\)/)
  assert.match(editor, /if \(!appliedCandidates\) return equipmentApplied/)
  assert.match(editor, /请核对武器、祝福、召唤石、专精、因子和目标槽后保存/)
})

test('optimizer handoff preserves exact catalog hashes and keeps remaining preset slots', () => {
  assert.match(editor, /exactSigilHash:\s*candidate\.exactSigilHash/)
  assert.match(editor, /exactPrimaryTraitHash:\s*candidate\.exactPrimaryTraitHash/)
  assert.match(editor, /exactSecondaryTraitHash:\s*candidate\.exactSecondaryTraitHash/)
  assert.match(editor, /for \(const entry of baseSlots\)/)
  assert.match(optimizer, /不足 12 个时沿用这套配装未重复的剩余因子/)
})

test('function artwork pipeline packages only the high-quality displayed images', () => {
  const generator = readFileSync(new URL('../scripts/generate_function_assets.mjs', import.meta.url), 'utf8')
  assert.doesNotMatch(generator, /thumb:\s*\{/)
  assert.match(generator, /display:\s*\{/)
  assert.doesNotMatch(generator, /variants\.full|copyFile\(sourcePath/)
})

test('missing page artwork never renders a broken mascot image', () => {
  assert.match(viewer, /defineAsyncComponent/)
  const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
  assert.match(shell, /<img v-if="currentSticker" class="sidebar-mascot-img"/)
})

test('battle archive states its read-only contract and exposes explicit disconnect', () => {
  assert.match(archive, /本机只读/)
  assert.match(archive, /不会注入游戏，也不会修改数据库/)
  assert.match(archive, /CloseLogsBattleArchive/)
  assert.match(archive, /await CloseLogsBattleArchive\(\)/)
  assert.doesNotMatch(archive, /onBeforeUnmount\([^)]*CloseLogsBattleArchive/)
  assert.doesNotMatch(archive, /UPDATE\s+encounters|DELETE\s+FROM|INSERT\s+INTO/i)
  assert.match(archive, /非 v1 协议记录/)
  assert.match(archive, /cursorTime: nextCursorTime\.value, cursorId: nextCursorID\.value/)
})

test('battle archive owns a persistent current-session damage capture mode', () => {
  assert.match(archive, /RuntimeDamageCaptureStart/)
  assert.match(archive, /RuntimeDamageCaptureSnapshot\(1024\)/)
  assert.match(archive, /window\.setTimeout\(refreshLiveCapture, 500\)/)
  assert.match(archive, /await RuntimeDamageCaptureStop\(\)/)
  assert.match(archive, /停止并恢复 Hook/)
  assert.match(archive, /sourceMode = 'live'; scheduleLivePolling\(\)/)
  assert.match(archive, /sourceMode = 'logs'; stopLivePolling\(\)/)
  assert.match(archive, /liveSourceOrdinals/)
  assert.match(archive, /当前窗口观测伤害/)
  assert.match(archive, /当前不冒充个人 DPS/)
	assert.match(archive, /onBeforeUnmount\(deactivateLivePolling\)/)
  assert.doesNotMatch(archive, /onBeforeUnmount\([^)]*RuntimeDamageCaptureStop/)
  assert.match(archive, /@container battle \(max-width:520px\) \{ \.archive-heading \{ align-items:stretch; flex-direction:column; \}/)
})

test('Logs entry describes the bounded recent-record scan without claiming the whole database', () => {
  assert.match(viewer, /扫描最近的兼容记录/)
  assert.match(viewer, /Scan recent supported records/)
  assert.doesNotMatch(viewer, /Parse every character in the database/)
})

test('optimizer distinguishes formula-ranked directions from exact custom trait coverage', () => {
  assert.match(optimizer, /精确技能覆盖方案/)
  assert.match(optimizer, /启发式降级/)
  assert.match(optimizer, /公式模型 Top 10/)
  assert.match(optimizer, /可审计模型而非实战保证/)
  assert.match(optimizer, /profile\.value !== 'custom'/)
})

test('optimizer presets use versioned locale-independent character profiles and honest inventory scope', () => {
  for (const id of ['SKILL_020_00', 'SKILL_001_00', 'SKILL_069_00']) assert.match(optimizerConfig, new RegExp(id))
  assert.match(optimizerConfig, /LOADOUT_CHARACTER_PROFILE_VERSION/)
  assert.match(optimizerConfig, /LOADOUT_CHARACTER_PROFILES/)
  assert.match(optimizer, /LoadoutOptimizerEvidence/)
  assert.match(optimizer, /LoadoutOptimizerInventorySnapshot/)
  assert.match(optimizer, /LoadoutSimulateBuild/)
  assert.match(optimizer, /当前存档可识别实例/)
  assert.match(optimizer, /可能也被其他预设引用/)
})

test('optimizer runs exact search off the UI thread and cancels stale jobs', () => {
  assert.match(optimizer, /new Worker\(new URL\('\.\.\/loadoutOptimizer\.worker\.js'/u)
  assert.match(optimizer, /solveWorker\?\.terminate\(\)/u)
  assert.match(optimizer, /generation !== solveGeneration/u)
  assert.match(optimizerWorker, /solveLoadoutSuggestions/u)
  assert.match(optimizerWorker, /solveEquipmentAwareSuggestions/u)
  assert.match(optimizerWorker, /solveMixedOptimizerDomains/u)
})

test('sigil atlas localizes every catalog category instead of exposing internal enum names', () => {
  for (const category of ['character_sigil', 'support_sigil', 'special_sigil', 'opus_sigil']) {
    assert.match(atlas, new RegExp(`${category}: \\[`))
  }
  assert.match(atlas, /categoryLabel\(entry\.category\)/)
})

test('sigil atlas prefills the existing loadout constructor without writing a save', () => {
  assert.match(atlas, /emit\('construct'/)
  assert.match(atlas, /送入配装构造器/)
  assert.match(viewer, /@construct="constructFromAtlas"/)
  assert.match(viewer, /:pending-atlas-construct="pendingAtlasConstruct"/)
  assert.match(editor, /consumePendingAtlasConstruct/)
  assert.match(editor, /已从因子图鉴预填构造器，请确认槽位与等级后保存/)
  assert.doesNotMatch(atlas, /LoadoutApplyWithResources|WriteSave|SaveChanges/)
})

test('sigil atlas can hand a legal trait direction to optimizer and share description', () => {
  assert.match(atlas, /emit\('optimize'/)
  assert.match(atlas, /emit\('share-note'/)
  assert.match(atlas, /送入优化目标/)
  assert.match(atlas, /加入分享图说明/)
  assert.match(viewer, /@optimize="payload => openAtlasTool\('optimizer', payload\)"/)
  assert.match(viewer, /@share-note="payload => openAtlasTool\('share', payload\)"/)
  assert.match(optimizer, /pendingTarget/)
  assert.match(workshop, /suggestedDescription/)
})

test('share image export waits for image decoding before capture', () => {
  assert.match(workshop, /waitForImages/)
  assert.match(workshop, /\.decode\?\.\(\)/)
  assert.match(workshop, /html-to-image/)
  assert.match(workshop, /if \(image\.complete\)[\s\S]*?if \(!image\.naturalWidth\) throw new Error/)
  assert.match(workshop, /window\.setTimeout\([\s\S]*?素材加载超时/)
})

test('share image reuses capped combined skills from the application preview', () => {
  assert.match(workshop, /preview\.value\?\.combinedSkills/)
  assert.match(workshop, /trait\.rawLevel > trait\.level/)
  assert.match(workshop, /原始等级 · 未含召唤石/)
})
