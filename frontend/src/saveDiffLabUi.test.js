import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/SaveDiffLab.vue', import.meta.url), 'utf8')
const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('save laboratory is a lazy tools page and keeps the comparison mode strictly read-only', () => {
  assert.match(shell, /saveDiff:\s*\(\) => import\('\.\/SaveDiffLab\.vue'\)/)
  assert.match(shell, /items:\s*\['saveDiff', 'naturalDrop', 'audioMixer', 'camera', 'virtualSigils', 'compatibility', 'language', 'patch'\]/)
  assert.match(source, /严格只读/)
  assert.match(source, /不会备份、修改或写回/)
  assert.doesNotMatch(source, /UpdateSave|WriteSave|ApplySave|SaveChanges/)
})

test('save diff uses bounded pagination and preserves unknown record structure', () => {
  assert.match(source, /SaveDiffPage\([^,]+, 80,/)
  assert.match(source, /const canCompare = computed\(\(\) => !!leftPath\.value && !!rightPath\.value && !loading\.value\)/)
  assert.match(source, /if \(!summary\.value \|\| loading\.value\) return/)
  for (const field of ['entry.valueType', 'entry.idType', 'entry.unitId', 'entry.leftOccurrence', 'entry.rightOccurrence', 'entry.leftCount', 'entry.rightCount', 'entry.leftHash', 'entry.rightHash']) {
    assert.match(source, new RegExp(field.replace('.', '\\.')))
  }
})

test('save laboratory exposes detected slots and releases the backend diff session on exit', () => {
  assert.match(source, /FindSaveFiles/)
  assert.match(source, /基准存档快捷选择/)
  assert.match(source, /对照存档快捷选择/)
  assert.match(source, /已识别存档位/)
  assert.match(source, /onBeforeUnmount\(\(\) => \{ void CloseSaveDiff\(\)\.catch/)
})

test('sanitized export copy states exactly what leaves the machine', () => {
  assert.match(source, /导出文件不含源路径、文件名或原始字段值/)
  assert.match(source, /ExportSaveDiffJSON/)
})

test('save laboratory reflows without document-level horizontal overflow', () => {
  assert.match(source, /container:save-diff \/ inline-size/)
  assert.match(source, /@container save-diff \(max-width:620px\)/)
  assert.match(source, /\.diff-row \{ grid-template-columns:minmax\(0,1fr\)/)
  assert.match(source, /\.compare-mark \{ pointer-events:none;/)
})

test('fate research stays read-only until its field-evidence gate is complete', () => {
  assert.match(source, /FateEpisodeInspect/)
  assert.match(source, /ExportFateEpisodeEvidence/)
  assert.match(source, /导出只读证据/)
  assert.match(source, /命运篇章保持只读研究/)
  assert.match(source, /本构建不会写入命运篇章/)
  assert.doesNotMatch(source, /CompleteAllFateEpisodes|completeAllFate|确认写入/)
  assert.match(source, /fateStatus\.characters/)
  assert.match(source, /selectedFateCharacter\.episodes/)
  assert.match(source, /episode\?\.titleZh/)
  assert.match(source, /episode\?\.titleEn/)
  assert.match(source, /episode\.requiredLevel/)
  assert.match(source, /episode\.missionQuestId/)
  assert.match(source, /completedStaticHp/)
  assert.match(source, /chara_status_fate\.tbl/)
})

test('new laboratory controls use the shared themed segmented control', () => {
  const optimizer = readFileSync(new URL('./components/LoadoutOptimizer.vue', import.meta.url), 'utf8')
  const workshop = readFileSync(new URL('./components/LoadoutShareWorkshop.vue', import.meta.url), 'utf8')
  for (const component of [source, optimizer, workshop]) {
    assert.doesNotMatch(component, /ui-segmented/)
    assert.match(component, /ui-seg/)
    assert.match(component, /ui-seg-btn/)
  }
})

test('fate layout remains usable at the compact desktop width', () => {
  assert.match(source, /\.fate-character-grid \{ min-width:0; display:grid; grid-template-columns:repeat\(auto-fill,minmax\(160px,1fr\)\)/)
  assert.match(source, /\.fate-source \{ grid-template-columns:minmax\(0,1fr\); \}/)
  assert.match(source, /\.fate-actions \{ align-items:stretch; flex-direction:column; \}/)
  assert.match(source, /\.fate-episode-list \{ grid-template-columns:minmax\(0,1fr\); \}/)
})

test('Endless rule atlas stays read-only, bilingual and preserves raw unknown IDs', () => {
  assert.match(source, /InfinityRuleCatalog/)
  assert.match(source, /infinityName\(rule\)/)
  assert.match(source, /infinityDescription\(rule\)/)
  assert.match(source, /effect\.id/)
  assert.match(source, /未知效果 Id 只保留原值，不猜测语义/)
  assert.doesNotMatch(source, /InfinityRuleWrite|InfinityRuleApply/)
  assert.match(source, /\.infinity-quest \{ min-width:0;/)
  assert.match(source, /\.infinity-quest \{ grid-template-columns:minmax\(0,1fr\); \}/)
})
