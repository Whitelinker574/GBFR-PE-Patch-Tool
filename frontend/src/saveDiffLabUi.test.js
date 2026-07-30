import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/SaveDiffLab.vue', import.meta.url), 'utf8')
const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('save comparison is an offline save page with an in-page copy transaction', () => {
  assert.match(shell, /saveDiff:\s*\(\) => import\('\.\/SaveDiffLab\.vue'\)/)
  assert.match(shell, /items:\s*\['loadoutPresets', 'sigil', 'wrightstone', 'summonSave', 'progression', 'chara', 'save', 'saveDiff'\]/)
  assert.match(shell, /saveDiff:\s*'offline'/)
  assert.match(source, /ApplySaveDiffTransfers/)
  assert.match(source, /自动备份、原子替换并重新打开回读/)
  assert.doesNotMatch(source, /@open-tool="selectTool"/)
})

test('save comparison stages either copy direction without leaving the page', () => {
  assert.match(source, /defineEmits\(\['status'\]\)/)
  assert.match(source, /写入左侧/)
  assert.match(source, /写入右侧/)
  assert.match(source, /beginTransferDrag/)
  assert.match(source, /dropTransfer/)
  assert.match(source, /用右侧替换左侧/)
  assert.match(source, /用左侧替换右侧/)
  assert.match(source, /第三步 · 核对变更单/)
  assert.doesNotMatch(source, /emit\('open-tool'/)
})

test('save diff uses bounded pagination and preserves unknown record structure', () => {
  assert.match(source, /SaveDiffPage\([\s\S]*?80,/)
  assert.match(source, /const canCompare = computed\(\(\) => !!leftPath\.value && !!rightPath\.value && !loading\.value\)/)
  assert.match(source, /if \(!summary\.value \|\| loading\.value\) return/)
  for (const field of ['entry.valueType', 'entry.idType', 'entry.unitId', 'entry.leftOccurrence', 'entry.rightOccurrence', 'entry.leftCount', 'entry.rightCount', 'entry.leftHash', 'entry.rightHash']) {
    assert.match(source, new RegExp(field.replace('.', '\\.')))
  }
})

test('save diff explains field semantics and exposes category, copyability, and confidence filters', () => {
  for (const state of ['category', 'copyability', 'semanticConfidence']) {
    assert.match(source, new RegExp(`const ${state} = ref\\('all'\\)`))
  }
  assert.match(source, /SaveDiffPage\([\s\S]*?category\.value,\s*copyability\.value,\s*semanticConfidence\.value,\s*\)/)
  for (const field of ['categoryNameZh', 'categoryNameEn', 'semanticNameZh', 'semanticNameEn', 'semanticPurposeZh', 'semanticPurposeEn', 'semanticConfidence']) {
    assert.match(source, new RegExp(field))
  }
  assert.match(source, /未知字段/)
  assert.match(source, /Unknown Field/)
  assert.match(source, /是否可复制/)
  assert.match(source, /语义置信度/)
  assert.match(source, /中文\/英文语义名、ID 或 Hash/)
})

test('save laboratory exposes detected slots and releases the backend diff session on exit', () => {
  assert.match(source, /FindSaveFiles/)
  assert.match(source, /左侧存档快捷选择/)
  assert.match(source, /右侧存档快捷选择/)
  assert.match(source, /已识别存档位/)
  assert.match(source, /onBeforeUnmount\(\(\) => \{ void CloseSaveDiff\(\)\.catch/)
})

test('sanitized export copy states exactly what leaves the machine', () => {
  assert.match(source, /导出文件仍不含源路径、文件名或原始字段值/)
  assert.match(source, /ExportSaveDiffJSON/)
})

test('unsafe structural changes stay visible but cannot be staged as raw writes', () => {
  assert.match(source, /entry\.copySupported/)
  assert.match(source, /entry\.copyBlockReason/)
  assert.match(source, /不能直接复制/)
  assert.match(source, /单侧新增、删除或长度不同/)
})

test('save laboratory reflows without document-level horizontal overflow', () => {
  assert.match(source, /container:save-diff \/ inline-size/)
  assert.match(source, /@container save-diff \(max-width:620px\)/)
  assert.match(source, /\.diff-row \{ grid-template-columns:minmax\(0,1fr\)/)
  assert.match(source, /\.compare-mark \{ pointer-events:none;/)
})

test('fate fields can be selected and written directly with explicit verification boundaries', () => {
  assert.match(source, /FateEpisodeEditableInspect/)
  assert.match(source, /WriteFateEpisodeFields/)
  assert.match(source, /ExportFateEpisodeEvidence/)
  assert.match(source, /实验直写 · 选择后设为完成/)
  assert.match(source, /设为完成/)
  assert.match(source, /补完.*未完成篇章/)
  assert.match(source, /加入全部未完成记录/)
  assert.match(source, /writeSelectedFate/)
  assert.match(source, /expectedRevision:\s*fateSnapshot\.value\.revision/)
  assert.match(source, /只保证字段值，不保证奖励领取或游戏内效果/)
  assert.match(source, /fateStatus\.characters/)
  assert.match(source, /selectedFateCharacter\.episodes/)
  assert.match(source, /episode\?\.titleZh/)
  assert.match(source, /episode\?\.titleEn/)
  assert.match(source, /episode\.requiredLevel/)
  assert.match(source, /episode\.missionQuestId/)
  assert.match(source, /completedStaticHp/)
  assert.match(source, /chara_status_fate\.tbl/)
  assert.doesNotMatch(source, /CompleteAllFateEpisodes|completeAllFate/)
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
