<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ApplySaveDiffTransfers, CloseSaveDiff, ExportFateEpisodeEvidence, ExportSaveDiffCSV, ExportSaveDiffJSON, FateEpisodeEditableInspect, FindSaveFiles, InfinityRuleCatalog, OpenSaveDiff, SaveDiffPage, SelectSaveDiffFile, WriteFateEpisodeFields } from '../../wailsjs/go/backend/App'
import { characterNamePairByPLID } from '../characterRoster.js'
import { language } from '../i18n.js'

const emit = defineEmits(['status'])
const leftPath = ref('')
const rightPath = ref('')
const summary = ref(null)
const items = ref([])
const search = ref('')
const status = ref('different')
const category = ref('all')
const copyability = ref('all')
const semanticConfidence = ref('all')
const loading = ref(false)
const exporting = ref(false)
const nextCursor = ref(0)
const hasMore = ref(false)
const totalFiltered = ref(0)
const targetSide = ref('left')
const stagedByKey = ref(new Map())
const applying = ref(false)
const writeConfirmed = ref(false)
const draggedTransfer = ref(null)
const mode = ref('diff')
const fatePath = ref('')
const fateStatus = ref(null)
const fateSnapshot = ref(null)
const fateLoading = ref(false)
const fateExporting = ref(false)
const fateWriting = ref(false)
const fateWriteConfirmed = ref(false)
const fateSelectedFields = ref(new Set())
const fateSelectedCode = ref('')
const infinityCatalog = ref(null)
const infinityLoading = ref(false)
const saveSlots = ref([])

const tx = (zh, en) => language.value === 'en' ? en : zh
const canCompare = computed(() => !!leftPath.value && !!rightPath.value && !loading.value)
const stagedEntries = computed(() => [...stagedByKey.value.values()])
const stagedCount = computed(() => stagedByKey.value.size)
const stagedTargetName = computed(() => fileName(targetSide.value === 'left' ? leftPath.value : rightPath.value))
const stagedSourceName = computed(() => fileName(targetSide.value === 'left' ? rightPath.value : leftPath.value))
const loadedCopyable = computed(() => items.value.filter(item => item.copySupported))
const statusOptions = computed(() => [
  { value: 'different', label: tx('仅差异', 'Differences Only') },
  { value: 'changed', label: tx('已修改', 'Changed') },
  { value: 'added', label: tx('右侧新增', 'Added on Right') },
  { value: 'removed', label: tx('右侧缺少', 'Missing on Right') },
  { value: 'all', label: tx('全部记录', 'All Records') },
])
const categoryOptions = computed(() => [
  { value: 'all', label: tx('全部类别', 'All Categories') },
  { value: 'system', label: tx('存档结构', 'Save Structure') },
  { value: 'currency', label: tx('货币与点数', 'Currency & Points') },
  { value: 'character', label: tx('角色成长', 'Character Progress') },
  { value: 'inventory', label: tx('物品背包', 'Inventory') },
  { value: 'quest', label: tx('任务进度', 'Quest Progress') },
  { value: 'trait', label: tx('装备词条', 'Equipment Traits') },
  { value: 'sigil', label: tx('因子', 'Sigils') },
  { value: 'wrightstone', label: tx('祝福石', 'Wrightstones') },
  { value: 'summon', label: tx('召唤石', 'Summons') },
  { value: 'weapon', label: tx('武器', 'Weapons') },
  { value: 'loadout', label: tx('配装预设', 'Loadout Presets') },
  { value: 'title', label: tx('称号与收藏', 'Titles & Collection') },
  { value: 'unlock', label: tx('开放状态', 'Unlock State') },
  { value: 'unknown', label: tx('未识别', 'Unidentified') },
])
const copyabilityOptions = computed(() => [
  { value: 'all', label: tx('全部记录', 'All Records') },
  { value: 'copyable', label: tx('可以复制', 'Copyable') },
  { value: 'blocked', label: tx('不能直接复制', 'Not Directly Copyable') },
])
const semanticConfidenceOptions = computed(() => [
  { value: 'all', label: tx('全部置信级别', 'All Confidence Levels') },
  { value: 'known', label: tx('已确认', 'Known') },
  { value: 'inferred', label: tx('推断', 'Inferred') },
  { value: 'unknown', label: tx('未知', 'Unknown') },
])
const fateComplete = computed(() => fateStatus.value?.completed === fateStatus.value?.total && fateStatus.value?.missionCompleted === fateStatus.value?.missionTotal)
const selectedFateCharacter = computed(() => {
  const characters = fateStatus.value?.characters || []
  return characters.find(item => item.code === fateSelectedCode.value) || characters[0] || null
})
const fateEditableFields = computed(() => fateSnapshot.value?.fields || [])
const fateEpisodeFieldByKey = computed(() => new Map(
  fateEditableFields.value
    .filter(field => field.field === 'episodeState')
    .map(field => [String(field.episodeKey || '').toUpperCase(), field]),
))
const fateMissionFieldByCode = computed(() => new Map(
  fateEditableFields.value
    .filter(field => field.field === 'missionState')
    .map(field => [String(field.missionCode || '').toUpperCase().padStart(8, '0'), field]),
))
const fateSelectedChanges = computed(() => fateEditableFields.value
  .filter(field => fateSelectedFields.value.has(fateFieldIdentity(field)))
  .map(field => ({
    field: field.field,
    episodeKey: field.episodeKey || '',
    missionId: Number(field.missionId || 0) >>> 0,
    expectedValue: Number(field.currentValue || 0) >>> 0,
    targetValue: Number(field.allowedTargetValues?.[0] ?? field.currentValue ?? 0) >>> 0,
  })))
const fateSelectedEpisodeCount = computed(() => fateSelectedChanges.value.filter(change => change.field === 'episodeState').length)
const fateSelectedMissionCount = computed(() => fateSelectedChanges.value.filter(change => change.field === 'missionState').length)
const infinityRulesByQuest = computed(() => {
  const groups = new Map()
  for (const rule of infinityCatalog.value?.rules || []) {
    if (!groups.has(rule.questId)) groups.set(rule.questId, [])
    groups.get(rule.questId).push(rule)
  }
  return [...groups].map(([questId, rules]) => ({ questId, rules }))
})

function fileName(path) {
  return String(path || '').split(/[\\/]/).pop() || tx('尚未选择', 'Not Selected')
}
function saveSlotLabel(slot) {
  const index = Number(slot?.index)
  return Number.isFinite(index) && index > 0 ? tx(`存档位 ${index}`, `Save Slot ${index}`) : fileName(slot?.path || slot?.name)
}
function clearComparisonResult() {
  summary.value = null
  items.value = []
  nextCursor.value = 0
  hasMore.value = false
  totalFiltered.value = 0
  clearStaged()
}
function setComparePath(side, path) {
  if (side === 'left') leftPath.value = path
  else rightPath.value = path
  clearComparisonResult()
}
async function scanSaveSlots() {
  try { saveSlots.value = await FindSaveFiles() || [] }
  catch { saveSlots.value = [] }
}
function formatID(value) {
  const number = Number(value) >>> 0
  return `${number} · 0x${number.toString(16).toUpperCase().padStart(8, '0')}`
}
function statusLabel(value) {
  return {
    added: tx('右侧新增', 'Added on Right'),
    removed: tx('右侧缺少', 'Missing on Right'),
    changed: tx('内容变化', 'Changed'),
    unchanged: tx('相同', 'Unchanged'),
  }[value] || value
}
function semanticName(entry) {
  return language.value === 'en'
    ? (entry?.semanticNameEn || entry?.semanticName || 'Unknown Field')
    : (entry?.semanticNameZh || '未知字段')
}
function semanticCategoryName(entry) {
  return language.value === 'en'
    ? (entry?.categoryNameEn || 'Unidentified')
    : (entry?.categoryNameZh || '未识别')
}
function semanticPurpose(entry) {
  return language.value === 'en'
    ? (entry?.semanticPurposeEn || 'No repeatable evidence currently explains this field; no effect is inferred.')
    : (entry?.semanticPurposeZh || '尚无可重复证据说明该字段用途，不猜测效果。')
}
function semanticConfidenceLabel(value) {
  return {
    known: tx('用途已确认', 'Purpose Confirmed'),
    inferred: tx('用途待验证', 'Purpose Needs Verification'),
    unknown: tx('用途未知', 'Purpose Unknown'),
  }[value] || tx('用途未知', 'Purpose Unknown')
}
function entityName(entry, side) {
  const entity = side === 'right' ? entry?.rightEntity : entry?.leftEntity
  return language.value === 'en'
    ? (entity?.nameEn || tx('未识别记录', 'Unidentified Record'))
    : (entity?.nameZh || tx('未识别记录', 'Unidentified Record'))
}
function entityDetail(entry, side) {
  const entity = side === 'right' ? entry?.rightEntity : entry?.leftEntity
  return language.value === 'en' ? (entity?.detailEn || '') : (entity?.detailZh || '')
}
function readableValue(entry, side) {
  if (side === 'right') return language.value === 'en' ? (entry?.rightDisplayEn || entry?.rightPreview || '—') : (entry?.rightDisplayZh || entry?.rightPreview || '—')
  return language.value === 'en' ? (entry?.leftDisplayEn || entry?.leftPreview || '—') : (entry?.leftDisplayZh || entry?.leftPreview || '—')
}
function riskLabel(entry) {
  return {
    low: tx('可直接复制', 'Ready to Copy'),
    review: tx('只读 · 待验证', 'Read Only · Needs Verification'),
    blocked: tx('不能直接复制', 'Cannot Copy Directly'),
  }[entry?.riskLevel] || tx('需要核对', 'Review Required')
}
function riskReason(entry) {
  return language.value === 'en'
    ? (entry?.riskReasonEn || entry?.copyBlockReasonEn || entry?.copyBlockReason || '')
    : (entry?.riskReasonZh || entry?.copyBlockReasonZh || entry?.copyBlockReason || '')
}
async function choose(side) {
  try {
    const path = await SelectSaveDiffFile()
    if (!path) return
    setComparePath(side, path)
  } catch (error) { emit('status', String(error), 'error') }
}
async function fetchPage(reset = false) {
  if (!summary.value || loading.value) return
  loading.value = true
  try {
    const page = await SaveDiffPage(
      reset ? 0 : nextCursor.value,
      80,
      search.value.trim(),
      status.value,
      category.value,
      copyability.value,
      semanticConfidence.value,
    )
    items.value = reset ? (page.items || []) : [...items.value, ...(page.items || [])]
    nextCursor.value = page.nextCursor || 0
    hasMore.value = !!page.hasMore
    totalFiltered.value = page.totalFiltered || 0
  } catch (error) { emit('status', String(error), 'error') } finally { loading.value = false }
}
async function compare() {
  if (!canCompare.value) return
  loading.value = true
  try {
    summary.value = await OpenSaveDiff(leftPath.value, rightPath.value)
    items.value = []
    nextCursor.value = 0
    hasMore.value = false
    totalFiltered.value = 0
    loading.value = false
    await fetchPage(true)
    emit('status', summary.value.different
      ? tx(`发现 ${summary.value.different} 条差异`, `${summary.value.different} differences found`)
      : tx('两份存档的逻辑记录完全一致', 'The two saves contain identical logical records'), 'success')
  } catch (error) {
    summary.value = null
    emit('status', String(error), 'error')
  } finally { loading.value = false }
}
function clearStaged() {
  stagedByKey.value = new Map()
  writeConfirmed.value = false
}
function setTargetSide(side) {
  if (side === targetSide.value) return
  targetSide.value = side
  clearStaged()
}
function stageEntry(entry) {
  if (!entry?.copySupported) return
  const next = new Map(stagedByKey.value)
  if (next.has(entry.key)) next.delete(entry.key)
  else next.set(entry.key, entry)
  stagedByKey.value = next
  writeConfirmed.value = false
}
function stageLoaded() {
  const next = new Map(stagedByKey.value)
  for (const entry of loadedCopyable.value) next.set(entry.key, entry)
  stagedByKey.value = next
  writeConfirmed.value = false
}
function beginTransferDrag(event, entry, sourceSide) {
  if (!entry?.copySupported) {
    event.preventDefault()
    return
  }
  draggedTransfer.value = { entry, sourceSide }
  event.dataTransfer.effectAllowed = 'copy'
  event.dataTransfer.setData('text/plain', entry.key)
}
function finishTransferDrag() {
  draggedTransfer.value = null
}
function dropTransfer(event, entry, destinationSide) {
  event.preventDefault()
  const transfer = draggedTransfer.value
  if (!transfer || transfer.entry.key !== entry.key || transfer.sourceSide === destinationSide) return
  setTargetSide(destinationSide)
  const next = new Map(stagedByKey.value)
  next.set(entry.key, entry)
  stagedByKey.value = next
  writeConfirmed.value = false
  draggedTransfer.value = null
}
function transferDirectionLabel(short = false) {
  if (targetSide.value === 'left') return short ? tx('右 → 左', 'Right → Left') : tx('用右侧替换左侧', 'Replace Left with Right')
  return short ? tx('左 → 右', 'Left → Right') : tx('用左侧替换右侧', 'Replace Right with Left')
}
async function applyTransfers() {
  if (!stagedCount.value || !writeConfirmed.value || applying.value) return
  applying.value = true
  try {
    const result = await ApplySaveDiffTransfers({ targetSide: targetSide.value, keys: stagedEntries.value.map(entry => entry.key) })
    summary.value = result.summary
    items.value = []
    nextCursor.value = 0
    hasMore.value = false
    totalFiltered.value = 0
    clearStaged()
    await fetchPage(true)
    const backup = result.backupPath ? tx(`；备份：${result.backupPath}`, `; backup: ${result.backupPath}`) : ''
    emit('status', tx(`已复制并回读确认 ${result.verified} 条差异到 ${result.targetName}${backup}`, `Copied and verified ${result.verified} differences in ${result.targetName}${backup}`), 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally {
    applying.value = false
  }
}
async function reset() {
  await CloseSaveDiff()
  clearComparisonResult()
}
async function exportDiff() {
  if (!summary.value || exporting.value) return
  exporting.value = true
  try {
    const path = await ExportSaveDiffJSON()
    if (path) emit('status', tx(`脱敏差分已导出到：${path}`, `Sanitized diff exported to: ${path}`), 'success')
  } catch (error) { emit('status', String(error), 'error') } finally { exporting.value = false }
}
async function exportCSV() {
  if (!summary.value || exporting.value) return
  exporting.value = true
  try {
    const path = await ExportSaveDiffCSV()
    if (path) emit('status', tx(`脱敏 CSV 已导出到：${path}`, `Sanitized CSV exported to: ${path}`), 'success')
  } catch (error) { emit('status', String(error), 'error') } finally { exporting.value = false }
}
function applyFilter() { fetchPage(true) }
function fateCharacterName(code) {
  const names = characterNamePairByPLID(code)
  return names ? names[language.value === 'en' ? 1 : 0] : code
}
function fateEpisodeTitle(episode) {
  return language.value === 'en' ? (episode?.titleEn || episode?.key) : (episode?.titleZh || episode?.key)
}
function fateRequirement(episode) {
  const parts = []
  if (episode?.requiredLevel > 1) parts.push(tx(`角色 Lv${episode.requiredLevel}`, `Character Lv${episode.requiredLevel}`))
  if (episode?.requiredQuestId && episode.requiredQuestId !== '00000000') parts.push(tx(`前置任务 ${episode.requiredQuestId}`, `Prerequisite ${episode.requiredQuestId}`))
  if (episode?.missionQuestId && episode.missionQuestId !== '00000000') parts.push(tx(`战斗任务 ${episode.missionQuestId}`, `Battle Mission ${episode.missionQuestId}`))
  return parts.length ? parts.join(' · ') : tx('无额外等级或任务要求', 'No additional level or quest requirement')
}
function fateFieldIdentity(field) {
  return field?.field === 'missionState'
    ? `mission:${Number(field.missionId || 0) >>> 0}`
    : `episode:${String(field?.episodeKey || '').toUpperCase()}`
}
function clearFateSelection() {
  fateSelectedFields.value = new Set()
  fateWriteConfirmed.value = false
}
function fateFieldNeedsWrite(field) {
  const target = Number(field?.allowedTargetValues?.[0] ?? field?.currentValue ?? 0) >>> 0
  return Number(field?.currentValue || 0) >>> 0 !== target
}
function fateEpisodeFields(episode) {
  const fields = []
  const episodeField = fateEpisodeFieldByKey.value.get(String(episode?.key || '').toUpperCase())
  if (episodeField && fateFieldNeedsWrite(episodeField)) fields.push(episodeField)
  const missionCode = String(episode?.missionQuestId || '').toUpperCase().padStart(8, '0')
  const missionField = fateMissionFieldByCode.value.get(missionCode)
  if (missionField && fateFieldNeedsWrite(missionField)) fields.push(missionField)
  return fields
}
function fateEpisodeSelected(episode) {
  const fields = fateEpisodeFields(episode)
  return fields.length > 0 && fields.every(field => fateSelectedFields.value.has(fateFieldIdentity(field)))
}
function toggleFateEpisode(episode) {
  const fields = fateEpisodeFields(episode)
  if (!fields.length) return
  const next = new Set(fateSelectedFields.value)
  const remove = fields.every(field => next.has(fateFieldIdentity(field)))
  for (const field of fields) {
    const identity = fateFieldIdentity(field)
    if (remove) next.delete(identity)
    else next.add(identity)
  }
  fateSelectedFields.value = next
  fateWriteConfirmed.value = false
}
function addFateFields(fields) {
  const next = new Set(fateSelectedFields.value)
  for (const field of fields) {
    if (fateFieldNeedsWrite(field)) next.add(fateFieldIdentity(field))
  }
  fateSelectedFields.value = next
  fateWriteConfirmed.value = false
}
function selectCurrentCharacterFate() {
  const episodes = selectedFateCharacter.value?.episodes || []
  addFateFields(episodes.flatMap(fateEpisodeFields))
}
function selectAllIncompleteFate() {
  addFateFields(fateEditableFields.value)
}
async function chooseFateSave() {
  try {
    const path = await SelectSaveDiffFile()
    if (!path) return
    fatePath.value = path
    await inspectFate()
  } catch (error) { emit('status', String(error), 'error') }
}
async function selectFateSave(path) {
  if (!path || fateLoading.value) return
  fatePath.value = path
  await inspectFate()
}
async function inspectFate() {
  if (!fatePath.value || fateLoading.value) return
  fateLoading.value = true
  try {
    fateSnapshot.value = await FateEpisodeEditableInspect(fatePath.value)
    fateStatus.value = fateSnapshot.value?.status || null
    clearFateSelection()
    const characters = fateStatus.value?.characters || []
    if (!characters.some(item => item.code === fateSelectedCode.value)) {
      fateSelectedCode.value = (characters.find(item => item.completed < item.total) || characters[0])?.code || ''
    }
    emit('status', tx(`已读取 ${fateStatus.value.completed}/${fateStatus.value.total} 篇命运篇章`, `Read ${fateStatus.value.completed}/${fateStatus.value.total} Fate Episodes`), 'success')
  } catch (error) {
    fateStatus.value = null
    fateSnapshot.value = null
    clearFateSelection()
    emit('status', String(error), 'error')
  } finally { fateLoading.value = false }
}
async function writeSelectedFate() {
  if (!fateSelectedChanges.value.length || !fateWriteConfirmed.value || !fateSnapshot.value?.revision || fateWriting.value) return
  fateWriting.value = true
  try {
    const result = await WriteFateEpisodeFields({
      path: fatePath.value,
      expectedRevision: fateSnapshot.value.revision,
      changes: fateSelectedChanges.value,
    })
    const backup = result.backupPath ? tx(`；备份：${result.backupPath}`, `; backup: ${result.backupPath}`) : ''
    emit('status', tx(
      `已写入并逐字段回读 ${result.verified} 项命运篇章记录${backup}`,
      `Wrote and verified ${result.verified} Fate fields${backup}`,
    ), 'success')
    await inspectFate()
  } catch (error) {
    emit('status', String(error), 'error')
  } finally {
    fateWriting.value = false
  }
}
async function exportFateEvidence() {
  if (!fatePath.value || !fateStatus.value || fateExporting.value) return
  fateExporting.value = true
  try {
    const path = await ExportFateEpisodeEvidence(fatePath.value)
    if (path) emit('status', tx(`命运篇章只读证据已导出到：${path}`, `Read-only Fate evidence exported to: ${path}`), 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally { fateExporting.value = false }
}
async function openInfinityRules() {
  mode.value = 'infinity'
  if (infinityCatalog.value || infinityLoading.value) return
  infinityLoading.value = true
  try {
    infinityCatalog.value = await InfinityRuleCatalog()
    emit('status', tx('已读取 25 条无尽模式官方规则', 'Loaded 25 official Endless rules'), 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally { infinityLoading.value = false }
}
function infinityName(rule) { return language.value === 'en' ? rule.nameEn : rule.nameZh }
function infinityDescription(rule) { return language.value === 'en' ? rule.descriptionEn : rule.descriptionZh }
onMounted(scanSaveSlots)
onBeforeUnmount(() => { void CloseSaveDiff().catch(() => {}) })
</script>

<template>
  <section class="save-diff-lab ui-page-stack" :aria-label="tx('存档实验室', 'Save Laboratory')">
    <nav class="lab-modes ui-seg" :aria-label="tx('存档实验室功能', 'Save Laboratory Modes')">
      <button type="button" class="ui-seg-btn" :class="{ 'is-on': mode === 'diff' }" @click="mode = 'diff'">{{ tx('比较两份存档', 'Compare Two Saves') }}</button>
      <button type="button" class="ui-seg-btn" :class="{ 'is-on': mode === 'fate' }" @click="mode = 'fate'">{{ tx('命运篇章写入', 'Fate Episode Write') }}</button>
      <button type="button" class="ui-seg-btn" :class="{ 'is-on': mode === 'infinity' }" @click="openInfinityRules">{{ tx('查看无尽规则', 'Browse Endless Rules') }}</button>
    </nav>

    <div v-if="mode === 'diff'" class="lab-boundary ui-notice is-info">
      <strong>{{ tx('存档差异与定向复制', 'Save Differences and Targeted Copying') }}</strong>
      <span>{{ tx('找出两份存档中不同的角色、装备、物品和进度，把需要的差异从一侧拖到另一侧，或逐条加入变更单；不会整份覆盖目标存档。每条差异会先说明具体对象、字段、改前改后和风险；确认后自动备份、原子替换并重新打开回读。', 'Find changed characters, equipment, items, and progress across two saves, then drag the differences you need from one side to the other or add them one by one. The target save is never replaced wholesale. Each difference identifies the entity, field, before/after value, and risk before backup, atomic replacement, and readback verification.') }}</span>
    </div>

    <section v-if="mode === 'diff'" class="source-grid" :aria-label="tx('选择要比较的两份存档', 'Choose Two Saves to Compare')">
      <button type="button" class="source-file ui-card is-flat" :class="{ ready: leftPath }" :disabled="loading" @click="choose('left')">
        <small>{{ targetSide === 'left' ? tx('左侧 · 当前写入目标', 'Left · Current Write Target') : tx('左侧 · 复制来源', 'Left · Copy Source') }}</small><strong>{{ fileName(leftPath) }}</strong><span>{{ tx('点击更换左侧存档', 'Choose the left save') }}</span>
      </button>
      <div class="compare-mark" aria-hidden="true">⇄</div>
      <button type="button" class="source-file ui-card is-flat" :class="{ ready: rightPath }" :disabled="loading" @click="choose('right')">
        <small>{{ targetSide === 'right' ? tx('右侧 · 当前写入目标', 'Right · Current Write Target') : tx('右侧 · 复制来源', 'Right · Copy Source') }}</small><strong>{{ fileName(rightPath) }}</strong><span>{{ tx('点击更换右侧存档', 'Choose the right save') }}</span>
      </button>
      <button type="button" class="ui-btn is-primary compare-button" :disabled="!canCompare" @click="compare">{{ loading ? tx('正在解析…', 'Parsing…') : tx('比较并开始复制', 'Compare and Start Copying') }}</button>
    </section>
    <section v-if="mode === 'diff' && saveSlots.length" class="known-save-pickers ui-card is-flat" :aria-label="tx('快速选择已识别存档位', 'Quick Select Detected Save Slots')">
      <div><span>{{ tx('左侧存档快捷选择', 'Left Save Quick Select') }}</span><div><button v-for="slot in saveSlots" :key="`left-${slot.path}`" type="button" class="ui-btn is-sm" :class="{ 'is-primary': leftPath === slot.path }" :disabled="loading" @click="setComparePath('left', slot.path)">{{ saveSlotLabel(slot) }}</button></div></div>
      <div><span>{{ tx('右侧存档快捷选择', 'Right Save Quick Select') }}</span><div><button v-for="slot in saveSlots" :key="`right-${slot.path}`" type="button" class="ui-btn is-sm" :class="{ 'is-primary': rightPath === slot.path }" :disabled="loading" @click="setComparePath('right', slot.path)">{{ saveSlotLabel(slot) }}</button></div></div>
    </section>

    <template v-if="mode === 'diff' && summary">
      <section class="diff-summary ui-stat-grid">
        <article class="ui-card ui-stat"><small>{{ tx('差异总数', 'Differences') }}</small><strong>{{ summary.different }}</strong><span>{{ summary.leftRecords }} / {{ summary.rightRecords }} {{ tx('条记录', 'records') }}</span></article>
        <article class="ui-card ui-stat is-changed"><small>{{ tx('内容变化', 'Changed') }}</small><strong>{{ summary.changed }}</strong><span>{{ tx('同一位置，值或长度变化', 'Same location, changed values or length') }}</span></article>
        <article class="ui-card ui-stat is-added"><small>{{ tx('右侧新增', 'Added on Right') }}</small><strong>{{ summary.added }}</strong><span>{{ tx('只存在于对照存档', 'Only in comparison save') }}</span></article>
        <article class="ui-card ui-stat is-removed"><small>{{ tx('右侧缺少', 'Missing on Right') }}</small><strong>{{ summary.removed }}</strong><span>{{ tx('只存在于基准存档', 'Only in baseline save') }}</span></article>
      </section>

      <section class="transfer-workbench ui-card is-flat" :aria-label="tx('页内差异复制', 'In-Page Difference Copying')">
        <header>
          <div><small>{{ tx('第一步 · 选写入目标', 'Step 1 · Choose the Write Target') }}</small><strong>{{ tx('另一侧只作为复制来源', 'The Other Side Remains the Copy Source') }}</strong></div>
          <div class="ui-seg transfer-target">
            <button type="button" class="ui-seg-btn" :class="{ 'is-on': targetSide === 'left' }" @click="setTargetSide('left')">{{ tx('写入左侧', 'Write Left') }}</button>
            <button type="button" class="ui-seg-btn" :class="{ 'is-on': targetSide === 'right' }" @click="setTargetSide('right')">{{ tx('写入右侧', 'Write Right') }}</button>
          </div>
          <p>{{ tx(`当前方向：${stagedSourceName} → ${stagedTargetName}。默认只处理你加入变更单的记录，不会整份覆盖。`, `Current direction: ${stagedSourceName} → ${stagedTargetName}. Only records added to the change list are copied; the whole save is never overwritten.`) }}</p>
        </header>
        <div class="transfer-actions">
          <span><b>{{ tx('第二步 · 挑选差异', 'Step 2 · Select Differences') }}</b><small>{{ tx('可以拖拽左右值，也可以逐条点击替换', 'Drag values between sides or use each row’s replace button') }}</small></span>
          <button type="button" class="ui-btn" :disabled="!loadedCopyable.length" @click="stageLoaded">{{ tx(`加入当前已加载的 ${loadedCopyable.length} 条`, `Add ${loadedCopyable.length} Loaded Records`) }}</button>
          <button type="button" class="ui-btn is-ghost" :disabled="!stagedCount" @click="clearStaged">{{ tx('清空变更单', 'Clear Change List') }}</button>
        </div>
        <p class="transfer-structure-note">{{ tx('单侧新增、删除或长度不同的记录仍会显示差异，但不会提供直接复制，避免破坏存档结构。', 'One-sided, removed, or different-length records remain visible but cannot be copied directly, protecting the save structure.') }}</p>
      </section>

      <div class="diff-toolbar ui-card is-flat">
        <label class="diff-search"><span>{{ tx('搜索角色、物品、装备、字段名或 ID', 'Search characters, items, equipment, fields, or IDs') }}</span><input v-model="search" class="ui-input" :placeholder="tx('中文/英文语义名、ID 或 Hash，例如：伊欧 / 圆石 / 1308', 'Chinese/English semantic name, ID, or hash, for example: Io / Cobblestone / 1308')" @keyup.enter="applyFilter" /></label>
        <label><span>{{ tx('变化类型', 'Change Type') }}</span><select v-model="status" class="ui-select" @change="applyFilter"><option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
        <label><span>{{ tx('内容类型', 'Content Type') }}</span><select v-model="category" class="ui-select" @change="applyFilter"><option v-for="option in categoryOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
        <label><span>{{ tx('是否可复制', 'Copyability') }}</span><select v-model="copyability" class="ui-select" @change="applyFilter"><option v-for="option in copyabilityOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
        <label><span>{{ tx('语义置信度', 'Semantic Confidence') }}</span><select v-model="semanticConfidence" class="ui-select" @change="applyFilter"><option v-for="option in semanticConfidenceOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
        <div class="diff-toolbar-actions">
          <button type="button" class="ui-btn" :disabled="loading" @click="applyFilter">{{ tx('筛选', 'Filter') }}</button>
          <button type="button" class="ui-btn" :disabled="exporting" @click="exportDiff">{{ exporting ? tx('正在导出…', 'Exporting…') : tx('导出脱敏 JSON', 'Export Sanitized JSON') }}</button>
          <button type="button" class="ui-btn" :disabled="exporting" @click="exportCSV">{{ tx('导出脱敏 CSV', 'Export Sanitized CSV') }}</button>
          <button type="button" class="ui-btn is-ghost" :disabled="loading" @click="reset">{{ tx('清空比较', 'Clear Comparison') }}</button>
        </div>
      </div>
      <p class="export-boundary">{{ tx('脱敏导出只包含分组、语义名、ID、Hash 与摘要；导出文件仍不含源路径、文件名或原始字段值。', 'Sanitized exports contain groups, semantic names, IDs, hashes, and previews only; source paths, file names, and raw field values remain excluded.') }}</p>

      <div class="diff-count"><span>{{ tx(`当前筛选 ${totalFiltered} 条`, `${totalFiltered} records in current filter`) }}</span><small>{{ tx(`SlotData 版本 ${summary.leftVersion} → ${summary.rightVersion}`, `SlotData version ${summary.leftVersion} → ${summary.rightVersion}`) }}</small></div>

      <section class="diff-list" aria-live="polite">
        <article v-for="entry in items" :key="entry.key" class="diff-row ui-card is-flat" :class="[`is-${entry.status}`, { 'is-staged': stagedByKey.has(entry.key) }]">
          <div class="record-identity">
            <div class="semantic-badges">
              <span class="status-badge">{{ statusLabel(entry.status) }}</span>
              <span class="category-badge">{{ semanticCategoryName(entry) }}</span>
              <span class="confidence-badge" :class="`is-${entry.semanticConfidence || 'unknown'}`">{{ semanticConfidenceLabel(entry.semanticConfidence) }}</span>
            </div>
            <strong>{{ entityName(entry, targetSide) }}</strong>
            <h4>{{ semanticName(entry) }}</h4>
            <p>{{ semanticPurpose(entry) }}</p>
            <span class="risk-line" :class="`is-${entry.riskLevel || 'review'}`"><b>{{ riskLabel(entry) }}</b><small>{{ riskReason(entry) }}</small></span>
          </div>
          <div class="transfer-lane">
            <div class="value-side" :class="{ 'is-target': targetSide === 'left' }" :draggable="entry.copySupported && targetSide === 'right'" @dragstart="beginTransferDrag($event, entry, 'left')" @dragend="finishTransferDrag" @dragover.prevent @drop="dropTransfer($event, entry, 'left')">
              <small>{{ tx('左侧', 'Left') }} · {{ entityName(entry, 'left') }}</small><strong>{{ readableValue(entry, 'left') }}</strong><span v-if="entityDetail(entry, 'left')">{{ entityDetail(entry, 'left') }}</span>
            </div>
            <button v-if="entry.copySupported" type="button" class="transfer-one" :class="{ selected: stagedByKey.has(entry.key) }" @click="stageEntry(entry)">
              <b>{{ stagedByKey.has(entry.key) ? tx('已加入', 'Added') : transferDirectionLabel(true) }}</b>
              <small>{{ stagedByKey.has(entry.key) ? tx('点击移除', 'Click to Remove') : transferDirectionLabel() }}</small>
            </button>
            <div v-else class="transfer-blocked"><b>{{ tx('不能直接复制', 'Cannot Copy Directly') }}</b><small>{{ entry.copyBlockReason || tx('左右记录结构不同', 'The two record structures differ') }}</small></div>
            <div class="value-side" :class="{ 'is-target': targetSide === 'right' }" :draggable="entry.copySupported && targetSide === 'left'" @dragstart="beginTransferDrag($event, entry, 'right')" @dragend="finishTransferDrag" @dragover.prevent @drop="dropTransfer($event, entry, 'right')">
              <small>{{ tx('右侧', 'Right') }} · {{ entityName(entry, 'right') }}</small><strong>{{ readableValue(entry, 'right') }}</strong><span v-if="entityDetail(entry, 'right')">{{ entityDetail(entry, 'right') }}</span>
            </div>
          </div>
          <details class="technical-details">
            <summary>{{ tx('技术信息', 'Technical Details') }}</summary>
            <dl><div><dt>IDType</dt><dd>{{ formatID(entry.idType) }}</dd></div><div><dt>UnitID</dt><dd>{{ formatID(entry.unitId) }}</dd></div><div><dt>{{ tx('位置', 'Location') }}</dt><dd>{{ entry.section }} · {{ entry.valueType }} · L#{{ entry.leftOccurrence >= 0 ? entry.leftOccurrence + 1 : '—' }} / R#{{ entry.rightOccurrence >= 0 ? entry.rightOccurrence + 1 : '—' }}</dd></div><div><dt>{{ tx('长度', 'Count') }}</dt><dd>{{ entry.leftCount }} → {{ entry.rightCount }}</dd></div><div><dt>{{ tx('摘要 Hash', 'Preview Hash') }}</dt><dd>{{ entry.leftHash || '—' }} → {{ entry.rightHash || '—' }}</dd></div><div><dt>{{ tx('原始摘要', 'Raw Preview') }}</dt><dd>{{ entry.leftPreview || '—' }} → {{ entry.rightPreview || '—' }}</dd></div></dl>
          </details>
        </article>
        <p v-if="!items.length && !loading" class="ui-empty">{{ tx('当前筛选没有记录。', 'No records match the current filter.') }}</p>
      </section>
      <button v-if="hasMore" type="button" class="load-more ui-btn" :disabled="loading" @click="fetchPage(false)">{{ loading ? tx('正在读取…', 'Loading…') : tx('加载更多记录', 'Load More Records') }}</button>

      <section v-if="stagedCount" class="transfer-review ui-card" aria-live="polite">
        <header><div><small>{{ tx('第三步 · 核对变更单', 'Step 3 · Review the Change List') }}</small><strong>{{ transferDirectionLabel() }} · {{ stagedCount }} {{ tx('条记录', 'records') }}</strong></div><button type="button" class="ui-btn is-ghost" @click="clearStaged">{{ tx('全部撤销', 'Remove All') }}</button></header>
        <div class="transfer-review-list">
          <button v-for="entry in stagedEntries" :key="`staged-${entry.key}`" type="button" @click="stageEntry(entry)"><span><b>{{ entityName(entry, targetSide) }} · {{ semanticName(entry) }}</b><small>{{ readableValue(entry, targetSide) }} → {{ readableValue(entry, targetSide === 'left' ? 'right' : 'left') }} · {{ riskLabel(entry) }}</small></span><em>{{ targetSide === 'left' ? readableValue(entry, 'right') : readableValue(entry, 'left') }}</em><i>×</i></button>
        </div>
        <label class="transfer-confirm"><input v-model="writeConfirmed" type="checkbox" /><span><b>{{ tx(`我确认只把以上 ${stagedCount} 条记录写入 ${stagedTargetName}`, `I confirm writing only these ${stagedCount} records to ${stagedTargetName}`) }}</b><small>{{ tx('默认存档写入要求游戏完全退出；应用会先备份，再原子替换并逐条回读。', 'The game must be fully closed before writing a managed save. The app backs up first, replaces atomically, then verifies every record.') }}</small></span></label>
        <button type="button" class="ui-btn is-primary transfer-apply" :disabled="!writeConfirmed || applying" @click="applyTransfers">{{ applying ? tx('正在备份、写入并回读…', 'Backing Up, Writing, and Verifying…') : tx(`确认写入 ${stagedCount} 条差异`, `Write ${stagedCount} Differences`) }}</button>
      </section>
    </template>

    <template v-if="mode === 'fate'">
      <div class="lab-boundary ui-notice is-info"><strong>{{ tx('实验直写 · 选择后设为完成', 'Experimental Direct Write · Mark Selected Records Complete') }}</strong><span>{{ tx('可直接写入 DLC 2.0.2 已定位的 319 条篇章状态和 56 条任务完成状态。应用只保证存档字段、备份与回读正确；不会把奖励领取或游戏内实际效果说成已验证。', 'Directly writes the 319 located episode states and 56 mission completion states from DLC 2.0.2. The app verifies save fields, backup, and readback only; reward claiming and in-game effects are not presented as verified.') }}</span></div>
      <section class="fate-source ui-card is-flat">
        <button type="button" class="source-file" :class="{ ready: fatePath }" :disabled="fateLoading" @click="chooseFateSave"><small>{{ tx('目标存档', 'Target Save') }}</small><strong>{{ fileName(fatePath) }}</strong><span>{{ tx('选择后立即检查结构', 'Select and validate immediately') }}</span></button>
        <button type="button" class="ui-btn" :disabled="!fatePath || fateLoading" @click="inspectFate">{{ fateLoading ? tx('正在检查…', 'Checking…') : tx('重新检查', 'Check Again') }}</button>
        <div v-if="saveSlots.length" class="fate-save-slots"><span>{{ tx('已识别存档位', 'Detected Save Slots') }}</span><div><button v-for="slot in saveSlots" :key="`fate-${slot.path}`" type="button" class="ui-btn is-sm" :class="{ 'is-primary': fatePath === slot.path }" :disabled="fateLoading" @click="selectFateSave(slot.path)">{{ saveSlotLabel(slot) }}</button></div></div>
      </section>
      <template v-if="fateStatus">
        <section class="fate-summary ui-stat-grid">
          <article class="ui-card ui-stat"><small>{{ tx('已完成篇章', 'Completed Episodes') }}</small><strong>{{ fateStatus.completed }} / {{ fateStatus.total }}</strong><span>{{ tx('按 29 名角色逐篇校验', 'Verified across 29 characters') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('战斗篇章任务', 'Battle Missions') }}</small><strong>{{ fateStatus.missionCompleted }} / {{ fateStatus.missionTotal }}</strong><span>{{ tx('任务 ID 保持原值，只补完成状态', 'Mission IDs stay unchanged; only completion is raised') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('待完成', 'Remaining') }}</small><strong>{{ fateStatus.total - fateStatus.completed }}</strong><span>{{ fateComplete ? tx('当前记录已全部完成', 'All records are complete') : tx('可逐篇、按角色或全部补为完成', 'Complete individually, by character, or all at once') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('辅助记录', 'Auxiliary Rows') }}</small><strong>{{ fateStatus.auxiliaryPreserved }} / 5</strong><span>REMI · {{ tx('仅验证存在性', 'presence is validated only') }}</span></article>
        </section>
        <section class="fate-character-grid" :aria-label="tx('各角色命运篇章进度', 'Fate progress by character')">
          <button v-for="character in fateStatus.characters" :key="character.code" type="button" class="fate-character ui-card is-flat" :class="{ complete: character.completed === character.total, selected: character.code === selectedFateCharacter?.code }" @click="fateSelectedCode = character.code"><span><strong>{{ fateCharacterName(character.code) }}</strong><small>{{ character.code }}</small></span><b>{{ character.completed }}/{{ character.total }}</b></button>
        </section>
        <section v-if="selectedFateCharacter" class="fate-detail" :aria-label="tx(`${fateCharacterName(selectedFateCharacter.code)}命运篇章明细`, `${fateCharacterName(selectedFateCharacter.code)} Fate Episode Details`)">
          <header><div><small>{{ fateStatus.dataVersion }} · {{ tx('解包表逐篇校验', 'Per-Episode Unpacked-Table Validation') }}</small><h3>{{ fateCharacterName(selectedFateCharacter.code) }}</h3></div><span><b>HP +{{ selectedFateCharacter.completedStaticHp }}</b><b>{{ tx('攻击', 'ATK') }} +{{ selectedFateCharacter.completedStaticAttack }}</b><small>{{ tx('仅汇总已完成篇章在 chara_status_fate.tbl 中的静态奖励', 'Only completed static rewards from chara_status_fate.tbl are totalled') }}</small></span></header>
          <div class="fate-episode-list">
            <article v-for="episode in selectedFateCharacter.episodes" :key="episode.key" class="fate-episode" :class="{ complete: episode.completed, selected: fateEpisodeSelected(episode) }">
              <i>{{ String(episode.index + 1).padStart(2, '0') }}</i><span><strong>{{ fateEpisodeTitle(episode) }}</strong><small>{{ fateRequirement(episode) }}</small><code>{{ episode.key }} · 0x{{ episode.hash }}</code></span><em v-if="episode.hasStaticBonus">HP +{{ episode.staticHp }} · {{ tx('攻击', 'ATK') }} +{{ episode.staticAttack }}</em>
              <b v-if="episode.completed">{{ tx('已完成', 'Completed') }}</b>
              <button v-else type="button" class="fate-select-one" :class="{ selected: fateEpisodeSelected(episode) }" :disabled="!fateEpisodeFields(episode).length" @click="toggleFateEpisode(episode)">{{ fateEpisodeSelected(episode) ? tx('已加入写入', 'Added to Write') : tx('设为完成', 'Mark Complete') }}</button>
            </article>
          </div>
        </section>
        <section class="fate-write-panel ui-card">
          <header><span><small>{{ tx('选择写入范围', 'Choose Write Scope') }}</small><strong>{{ tx('只把选中的未完成状态补为完成', 'Only Selected Incomplete States Are Marked Complete') }}</strong></span><div class="fate-action-buttons"><button type="button" class="ui-btn" :disabled="!selectedFateCharacter || fateWriting" @click="selectCurrentCharacterFate">{{ tx(`补完${fateCharacterName(selectedFateCharacter?.code)}未完成篇章`, `Complete ${fateCharacterName(selectedFateCharacter?.code)}'s Remaining Episodes`) }}</button><button type="button" class="ui-btn" :disabled="fateComplete || fateWriting" @click="selectAllIncompleteFate">{{ tx('加入全部未完成记录', 'Add All Incomplete Records') }}</button><button type="button" class="ui-btn is-ghost" :disabled="!fateSelectedChanges.length || fateWriting" @click="clearFateSelection">{{ tx('清空选择', 'Clear Selection') }}</button></div></header>
          <div class="fate-write-summary"><span><small>{{ tx('篇章状态', 'Episode States') }}</small><b>{{ fateSelectedEpisodeCount }}</b></span><span><small>{{ tx('关联任务状态', 'Related Mission States') }}</small><b>{{ fateSelectedMissionCount }}</b></span><p>{{ tx('点单篇“设为完成”时，会一并加入该篇已确认的任务完成字段；不会改 REMI、占位记录、角色等级或奖励领取字段。', 'Marking one episode complete also adds its confirmed mission-completion field. REMI, placeholders, character levels, and reward-claim fields are not changed.') }}</p></div>
          <label v-if="fateSelectedChanges.length" class="transfer-confirm"><input v-model="fateWriteConfirmed" type="checkbox" /><span><b>{{ tx(`我确认把以上 ${fateSelectedChanges.length} 个字段写入 ${fileName(fatePath)}`, `I confirm writing these ${fateSelectedChanges.length} fields to ${fileName(fatePath)}`) }}</b><small>{{ tx('写入默认存档前必须完全退出游戏。应用会先备份、原子替换，再重新打开并逐字段回读；只保证字段值，不保证奖励领取或游戏内效果。', 'The game must be fully closed before writing a managed save. The app backs up, replaces atomically, reopens, and verifies every field. Only field values are guaranteed, not reward claiming or in-game effects.') }}</small></span></label>
          <div class="fate-write-actions"><button type="button" class="ui-btn" :disabled="fateExporting" @click="exportFateEvidence">{{ fateExporting ? tx('正在导出…', 'Exporting…') : tx('导出当前字段证据', 'Export Current Field Evidence') }}</button><button type="button" class="ui-btn is-primary" :disabled="!fateSelectedChanges.length || !fateWriteConfirmed || fateWriting" @click="writeSelectedFate">{{ fateWriting ? tx('正在备份、写入并回读…', 'Backing Up, Writing, and Verifying…') : tx(`确认写入 ${fateSelectedChanges.length} 个字段`, `Write ${fateSelectedChanges.length} Fields`) }}</button></div>
        </section>
      </template>
    </template>

    <template v-if="mode === 'infinity'">
      <div class="lab-boundary ui-notice is-info"><strong>{{ tx('2.0.2 官方规则图鉴', 'Official 2.0.2 Rule Atlas') }}</strong><span>{{ tx('名称、说明和参数来自 infinity_rule / infinity_rule_effect 与官方文本表。未知效果 Id 只保留原值，不猜测语义。', 'Names, descriptions, and parameters come from infinity_rule / infinity_rule_effect and official text tables. Unknown effect IDs retain raw values without invented semantics.') }}</span></div>
      <p v-if="infinityLoading" class="ui-empty">{{ tx('正在读取无尽模式规则…', 'Loading Endless rules…') }}</p>
      <template v-else-if="infinityCatalog">
        <section class="infinity-summary ui-stat-grid">
          <article class="ui-card ui-stat"><small>{{ tx('任务组', 'Quest Groups') }}</small><strong>{{ infinityRulesByQuest.length }}</strong><span>{{ tx('按任务 ID 分组', 'Grouped by quest ID') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('规则', 'Rules') }}</small><strong>{{ infinityCatalog.rules.length }}</strong><span>{{ tx('官方中英文本', 'Official Chinese and English text') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('难度阶梯', 'Difficulty Tiers') }}</small><strong>{{ infinityCatalog.difficulties.length }}</strong><span>{{ tx('原始等级与战力范围', 'Raw level and power ranges') }}</span></article>
        </section>
        <section class="infinity-difficulties" :aria-label="tx('无尽模式难度表', 'Endless Difficulty Table')">
          <article v-for="tier in infinityCatalog.difficulties" :key="tier.SortOrder"><small>{{ tx('阶梯', 'Tier') }} {{ tier.SortOrder + 1 }}</small><strong>Lv{{ tier.EnemyMinLevel }}–{{ tier.EnemyMaxLevel }}</strong><span>PWR {{ tier.Power }}</span></article>
        </section>
        <section class="infinity-quest-list">
          <article v-for="group in infinityRulesByQuest" :key="group.questId" class="infinity-quest">
            <header><span><small>{{ tx('任务 ID', 'Quest ID') }}</small><strong>{{ group.questId }}</strong></span><b>{{ group.rules.length }} {{ tx('条规则', 'rules') }}</b></header>
            <div class="infinity-rule-list">
              <details v-for="rule in group.rules" :key="rule.nameKey" class="infinity-rule">
                <summary><strong>{{ infinityName(rule) }}</strong><small>{{ rule.nameKey }}</small></summary>
                <p>{{ infinityDescription(rule) }}</p>
                <div v-if="rule.effects?.length" class="infinity-effect-row"><code v-for="effect in rule.effects" :key="effect.key">{{ effect.key }} · Id {{ effect.id }} · {{ effect.value }}</code></div>
                <small v-else class="infinity-no-effect">{{ tx('该条规则没有独立效果参数行', 'This rule has no separate effect parameter row') }}</small>
              </details>
            </div>
          </article>
        </section>
      </template>
    </template>
  </section>
</template>

<style scoped>
.save-diff-lab { min-width:0; container:save-diff / inline-size; }
.lab-modes { justify-self:start; }
.lab-boundary { align-items:start; }
.source-grid { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) 36px minmax(0,1fr) auto; gap:var(--space-3); align-items:stretch; }
.source-file { min-width:0; display:grid; gap:3px; padding:var(--space-4); border-left:3px solid var(--border-strong); color:inherit; text-align:left; cursor:pointer; }
.source-file:hover,.source-file.ready { border-left-color:var(--accent); background:var(--accent-soft); }
.source-file small,.source-file span { color:var(--text-muted); font-size:var(--fs-xs); }
.source-file strong { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); font-size:var(--fs-md); white-space:nowrap; }
.compare-mark { pointer-events:none; display:grid; place-items:center; color:var(--accent); font-size:var(--fs-xl); }
.compare-button { align-self:center; }
.known-save-pickers { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); padding:var(--space-3); }.known-save-pickers > div,.fate-save-slots { min-width:0; display:grid; gap:6px; }.known-save-pickers > div > span,.fate-save-slots > span { color:var(--text-muted); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }.known-save-pickers > div > div,.fate-save-slots > div { min-width:0; display:flex; flex-wrap:wrap; gap:6px; }
.diff-summary { grid-template-columns:repeat(4,minmax(0,1fr)); }
.diff-summary .ui-stat { min-width:0; padding:var(--space-3); border-top:2px solid var(--border-strong); }
.diff-summary .ui-stat.is-changed { border-top-color:var(--warning); }
.diff-summary .ui-stat.is-added { border-top-color:var(--success); }
.diff-summary .ui-stat.is-removed { border-top-color:var(--danger); }
.diff-summary small,.diff-summary span { color:var(--text-muted); font-size:var(--fs-2xs); }
.diff-summary strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xl); }
.transfer-workbench { min-width:0; display:grid; gap:var(--space-3); padding:var(--space-3); border-left:3px solid var(--accent); }
.transfer-workbench > header { min-width:0; display:grid; grid-template-columns:minmax(210px,.65fr) auto minmax(260px,1fr); gap:var(--space-4); align-items:center; }
.transfer-workbench > header > div:first-child { min-width:0; display:grid; gap:2px; }
.transfer-workbench > header small { color:var(--text-muted); font-size:var(--fs-2xs); }
.transfer-workbench > header strong { color:var(--text-primary); font-size:var(--fs-md); }
.transfer-workbench > header p { min-width:0; margin:0; color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.transfer-target { justify-self:center; }
.transfer-actions { min-width:0; display:flex; align-items:center; gap:var(--space-2); padding-top:var(--space-2); border-top:1px solid var(--border-soft); }
.transfer-actions > span { min-width:0; display:grid; gap:2px; margin-right:auto; }
.transfer-actions b { color:var(--text-primary); font-size:var(--fs-sm); }
.transfer-actions small { color:var(--text-muted); font-size:var(--fs-2xs); }
.transfer-structure-note { margin:0; color:var(--text-muted); font-size:var(--fs-2xs); line-height:var(--lh-normal); }
.diff-toolbar { min-width:0; display:grid; grid-template-columns:minmax(260px,1.35fr) repeat(4,minmax(132px,.55fr)); gap:var(--space-2); align-items:end; padding:var(--space-3); }
.diff-toolbar label { min-width:0; display:grid; gap:4px; color:var(--text-muted); font-size:var(--fs-xs); }
.diff-toolbar-actions { grid-column:1/-1; min-width:0; display:flex; flex-wrap:wrap; justify-content:flex-end; gap:var(--space-2); padding-top:var(--space-1); border-top:1px solid var(--border-soft); }
.export-boundary { margin:-4px 0 0; padding:0 2px; color:var(--text-muted); font-size:12px; line-height:1.45; }
.diff-count { display:flex; justify-content:space-between; gap:var(--space-3); color:var(--text-secondary); font-size:var(--fs-xs); }
.diff-count small { color:var(--text-muted); }
.diff-list { min-width:0; display:grid; gap:6px; }
.diff-row { min-width:0; display:grid; grid-template-columns:minmax(190px,1fr) minmax(230px,auto); gap:var(--space-2) var(--space-4); align-items:center; padding:var(--space-3); border-left:3px solid var(--border-strong); transition:border-color .16s ease,background .16s ease; }
.diff-row.is-changed { border-left-color:var(--warning); }
.diff-row.is-added { border-left-color:var(--success); }
.diff-row.is-removed { border-left-color:var(--danger); }
.diff-row.is-staged { outline:1px solid var(--accent-border); outline-offset:-1px; background:color-mix(in srgb,var(--accent-soft) 54%,var(--surface-card)); }
.record-identity,.value-side { min-width:0; display:grid; gap:3px; }
.record-identity { grid-column:1/-1; }
.semantic-badges { min-width:0; display:flex; flex-wrap:wrap; gap:4px; align-items:center; }
.record-identity strong { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); white-space:nowrap; }
.record-identity h4 { margin:0; color:var(--accent); font-size:var(--fs-sm); }
.record-identity p { margin:1px 0 2px; color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); overflow-wrap:anywhere; }
.record-identity small,.value-side small,.value-side span { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-muted); font-size:var(--fs-2xs); white-space:nowrap; }
.risk-line { min-width:0; display:flex; align-items:flex-start; gap:var(--space-2); margin-top:3px; padding:6px 8px; border-left:3px solid var(--border-strong); background:var(--surface-sunken); }
.risk-line b { flex:0 0 auto; color:var(--text-primary); font-size:var(--fs-xs); }
.risk-line small { white-space:normal; line-height:var(--lh-normal); }
.risk-line.is-low { border-left-color:var(--success); background:var(--success-bg); }
.risk-line.is-review { border-left-color:var(--warning); background:var(--warning-bg); }
.risk-line.is-blocked { border-left-color:var(--danger); background:var(--danger-bg); }
.status-badge,.category-badge,.confidence-badge { display:inline-flex; align-items:center; min-height:20px; padding:2px 6px; border:1px solid var(--border-soft); color:var(--accent); background:var(--surface-card); font-size:var(--fs-2xs); font-weight:var(--fw-bold); }
.category-badge { color:var(--text-secondary); background:var(--surface-sunken); }
.confidence-badge.is-known { border-color:var(--success-border); color:var(--success-ink); background:var(--success-bg); }
.confidence-badge.is-inferred { border-color:var(--warning); color:var(--warning-ink); background:var(--warning-bg); }
.confidence-badge.is-unknown { border-style:dashed; color:var(--text-muted); background:var(--surface-sunken); }
.diff-row dl { min-width:0; display:grid; gap:4px; margin:0; }
.diff-row dl div { min-width:0; display:grid; grid-template-columns:52px minmax(0,1fr); gap:var(--space-2); }
.diff-row dt { color:var(--text-muted); font-size:var(--fs-2xs); }
.diff-row dd { min-width:0; margin:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-2xs); white-space:nowrap; }
.transfer-lane { grid-column:1/-1; min-width:0; display:grid; grid-template-columns:minmax(0,1fr) minmax(132px,.22fr) minmax(0,1fr); gap:var(--space-2); align-items:stretch; }
.value-side { min-height:64px; align-content:center; padding:var(--space-2) var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.value-side[draggable="true"] { cursor:grab; }
.value-side[draggable="true"]:active { cursor:grabbing; }
.value-side.is-target { border-color:var(--accent-border); background:var(--accent-soft); }
.value-side > strong { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); font-size:var(--fs-md); white-space:nowrap; }
.technical-details { grid-column:1/-1; color:var(--text-muted); font-size:var(--fs-2xs); }
.technical-details summary { width:max-content; color:var(--accent); font-weight:var(--fw-semibold); cursor:pointer; }
.technical-details dl { margin-top:6px; padding:8px; border:1px solid var(--border-soft); background:var(--surface-sunken); }
.transfer-one,.transfer-blocked { min-width:0; min-height:64px; display:grid; place-content:center; gap:3px; padding:var(--space-2); border:1px solid var(--accent-border); border-radius:var(--radius-sm); text-align:center; }
.transfer-one { color:var(--accent); background:var(--surface-card); cursor:pointer; }
.transfer-one:hover,.transfer-one:focus-visible,.transfer-one.selected { background:var(--accent-soft); }
.transfer-one b,.transfer-blocked b { color:var(--text-primary); font-size:var(--fs-xs); }
.transfer-one small,.transfer-blocked small { color:var(--text-muted); font-size:var(--fs-2xs); line-height:1.35; }
.transfer-blocked { border-style:dashed; border-color:var(--border-soft); background:var(--surface-sunken); }
.transfer-review { min-width:0; display:grid; gap:var(--space-3); padding:var(--space-4); border-top:3px solid var(--accent); }
.transfer-review > header { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); }
.transfer-review > header > div { min-width:0; display:grid; gap:2px; }
.transfer-review > header small { color:var(--accent); font-size:var(--fs-2xs); font-weight:var(--fw-bold); }
.transfer-review > header strong { color:var(--text-primary); font-size:var(--fs-md); }
.transfer-review-list { min-width:0; max-height:240px; overflow:auto; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:6px; padding-right:4px; }
.transfer-review-list button { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) minmax(100px,.55fr) 20px; gap:var(--space-2); align-items:center; padding:var(--space-2) var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); color:inherit; background:var(--surface-sunken); text-align:left; cursor:pointer; }
.transfer-review-list button:hover { border-color:var(--danger); }
.transfer-review-list span { min-width:0; display:grid; gap:2px; }
.transfer-review-list b,.transfer-review-list small,.transfer-review-list em { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.transfer-review-list b { color:var(--text-primary); font-size:var(--fs-xs); }
.transfer-review-list small { color:var(--text-muted); font-size:var(--fs-2xs); }
.transfer-review-list em { color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-2xs); font-style:normal; }
.transfer-review-list i { color:var(--danger); font-size:var(--fs-md); font-style:normal; text-align:right; }
.transfer-confirm { min-width:0; display:flex; align-items:flex-start; gap:var(--space-2); padding:var(--space-3); border:1px solid var(--accent-border); background:var(--accent-soft); cursor:pointer; }
.transfer-confirm input { flex:0 0 auto; margin-top:3px; accent-color:var(--accent); }
.transfer-confirm span { min-width:0; display:grid; gap:2px; }
.transfer-confirm b { color:var(--text-primary); font-size:var(--fs-sm); }
.transfer-confirm small { color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.transfer-apply { justify-self:end; min-width:240px; }
.load-more { justify-self:center; }
.fate-source { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-3); align-items:center; padding:var(--space-3); }
.fate-save-slots { grid-column:1 / -1; padding-top:var(--space-2); border-top:1px solid var(--border-soft); }
.fate-source .source-file { border:0; border-left:3px solid var(--border-strong); background:transparent; }
.fate-summary { grid-template-columns:repeat(4,minmax(0,1fr)); }
.fate-summary .ui-stat { min-width:0; padding:var(--space-3); border-top:2px solid var(--border-strong); }
.fate-summary small,.fate-summary span { color:var(--text-muted); font-size:var(--fs-2xs); }
.fate-summary strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xl); }
.fate-character-grid { min-width:0; display:grid; grid-template-columns:repeat(auto-fill,minmax(160px,1fr)); gap:6px; }
.fate-character { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-2); padding:var(--space-2) var(--space-3); border-left:3px solid var(--warning); color:inherit; text-align:left; cursor:pointer; }
.fate-character.complete { border-left-color:var(--success); }
.fate-character.selected { outline:2px solid var(--accent-border); outline-offset:-2px; background:var(--accent-soft); }
.fate-character span { min-width:0; display:grid; }
.fate-character strong { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); font-size:var(--fs-sm); white-space:nowrap; }
.fate-character small { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-2xs); }
.fate-character b { flex:0 0 auto; color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-xs); }
.fate-detail { min-width:0; display:grid; gap:var(--space-3); padding-top:var(--space-3); border-top:1px solid var(--border-soft); }
.fate-detail > header { min-width:0; display:flex; align-items:end; justify-content:space-between; gap:var(--space-4); }
.fate-detail > header div,.fate-detail > header span { min-width:0; display:grid; gap:2px; }
.fate-detail > header small { color:var(--text-muted); font-size:var(--fs-2xs); }
.fate-detail > header h3 { margin:0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); letter-spacing:0; }
.fate-detail > header span { grid-template-columns:repeat(2,auto); justify-content:end; text-align:right; }
.fate-detail > header span b { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); }
.fate-detail > header span small { grid-column:1/-1; max-width:64ch; }
.fate-episode-list { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:6px var(--space-3); }
.fate-episode { min-width:0; display:grid; grid-template-columns:28px minmax(0,1fr) auto; gap:var(--space-2); align-items:center; padding:8px 0; border-bottom:1px solid var(--border-soft); }
.fate-episode > i { grid-row:1/3; display:grid; place-items:center; width:28px; height:28px; border:1px solid var(--border-soft); color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-2xs); font-style:normal; }
.fate-episode > span { min-width:0; display:grid; gap:2px; }
.fate-episode strong,.fate-episode small,.fate-episode code { min-width:0; overflow-wrap:anywhere; }
.fate-episode strong { color:var(--text-primary); font-size:var(--fs-xs); }
.fate-episode small,.fate-episode code { color:var(--text-muted); font-size:var(--fs-2xs); }
.fate-episode > em { grid-column:2; color:var(--text-secondary); font-size:var(--fs-2xs); font-style:normal; }
.fate-episode > b { grid-column:3; grid-row:1/3; color:var(--warning-ink); font-size:var(--fs-2xs); }
.fate-episode.complete > i { border-color:var(--success-border); color:var(--success); }
.fate-episode.complete > b { color:var(--success); }
.fate-episode.selected { margin-inline:-6px; padding-inline:6px; outline:1px solid var(--accent-border); outline-offset:-1px; background:var(--accent-soft); }
.fate-select-one { grid-column:3; grid-row:1/3; align-self:center; min-width:92px; min-height:32px; padding:5px 8px; border:1px solid var(--accent-border); border-radius:var(--radius-sm); color:var(--accent); background:var(--surface-card); font-size:var(--fs-2xs); font-weight:var(--fw-bold); cursor:pointer; }
.fate-select-one:hover,.fate-select-one:focus-visible,.fate-select-one.selected { color:var(--text-on-accent); background:var(--accent); }
.fate-select-one:disabled { cursor:not-allowed; opacity:.55; }
.fate-actions { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); padding:var(--space-4); border-left:3px solid var(--accent); }
.fate-actions span { min-width:0; display:grid; gap:2px; }
.fate-actions strong { color:var(--text-primary); }
.fate-actions small { color:var(--text-muted); font-size:var(--fs-xs); }
.fate-action-buttons { min-width:0; display:flex; flex-wrap:wrap; justify-content:flex-end; gap:var(--space-2); }
.fate-write-panel { min-width:0; display:grid; gap:var(--space-3); padding:var(--space-4); border-top:3px solid var(--accent); }
.fate-write-panel > header { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); }
.fate-write-panel > header > span { min-width:0; display:grid; gap:2px; }
.fate-write-panel > header small { color:var(--accent); font-size:var(--fs-2xs); font-weight:var(--fw-bold); }
.fate-write-panel > header strong { color:var(--text-primary); font-size:var(--fs-md); }
.fate-write-summary { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(140px,.25fr)) minmax(260px,1fr); gap:var(--space-2); align-items:stretch; }
.fate-write-summary > span { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); padding:var(--space-3); border:1px solid var(--border-soft); background:var(--surface-sunken); }
.fate-write-summary small { color:var(--text-muted); font-size:var(--fs-xs); }
.fate-write-summary b { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-lg); }
.fate-write-summary p { min-width:0; margin:0; padding:var(--space-3); border-left:2px solid var(--warning); color:var(--text-secondary); background:var(--warning-bg); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.fate-write-actions { min-width:0; display:flex; justify-content:flex-end; gap:var(--space-2); }
.infinity-summary { grid-template-columns:repeat(3,minmax(0,1fr)); }
.infinity-summary .ui-stat { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:2px var(--space-3); align-items:center; min-height:58px; padding:var(--space-3); border-top:2px solid var(--accent); }
.infinity-summary .ui-stat small,.infinity-summary .ui-stat span { grid-column:1; min-width:0; }
.infinity-summary .ui-stat strong { grid-column:2; grid-row:1/3; }
.infinity-summary small,.infinity-summary span { color:var(--text-muted); font-size:var(--fs-2xs); }
.infinity-summary strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xl); }
.infinity-difficulties { min-width:0; display:grid; grid-template-columns:repeat(5,minmax(0,1fr)); gap:1px; background:var(--border-soft); }
.infinity-difficulties article { min-width:0; display:grid; gap:2px; padding:var(--space-3); background:var(--surface-soft); }
.infinity-difficulties small { color:var(--text-muted); font-size:var(--fs-2xs); }
.infinity-difficulties strong { color:var(--text-primary); font-family:var(--font-data); }
.infinity-difficulties span { color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-xs); }
.infinity-quest-list { min-width:0; display:grid; gap:var(--space-4); }
.infinity-quest { min-width:0; display:grid; grid-template-columns:minmax(140px,.24fr) minmax(0,1fr); gap:var(--space-4); padding-top:var(--space-3); border-top:1px solid var(--border-soft); }
.infinity-quest > header { min-width:0; display:flex; align-items:start; justify-content:space-between; gap:var(--space-2); }
.infinity-quest > header span { min-width:0; display:grid; gap:2px; }
.infinity-quest > header small { color:var(--text-muted); font-size:var(--fs-2xs); }
.infinity-quest > header strong { color:var(--text-primary); font-family:var(--font-data); overflow-wrap:anywhere; }
.infinity-quest > header b { flex:0 0 auto; color:var(--accent); font-size:var(--fs-xs); }
.infinity-rule-list { min-width:0; display:grid; gap:4px; }
.infinity-rule { min-width:0; border-bottom:1px solid var(--border-soft); }
.infinity-rule summary { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-2); padding:7px 0; cursor:pointer; }
.infinity-rule summary strong { min-width:0; color:var(--text-primary); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.infinity-rule summary small { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-2xs); overflow-wrap:anywhere; }
.infinity-rule p { margin:0 0 var(--space-2); color:var(--text-secondary); font-size:var(--fs-xs); line-height:1.55; white-space:pre-line; }
.infinity-effect-row { min-width:0; display:flex; flex-wrap:wrap; gap:4px; padding-bottom:var(--space-2); }
.infinity-effect-row code { padding:2px 4px; background:var(--surface-soft); color:var(--text-muted); font-size:var(--fs-2xs); overflow-wrap:anywhere; }
.infinity-no-effect { display:block; padding-bottom:var(--space-2); color:var(--text-muted); font-size:var(--fs-2xs); }
@container save-diff (max-width:980px) {
  .source-grid { grid-template-columns:minmax(0,1fr) 28px minmax(0,1fr); }
  .compare-button { grid-column:1/-1; justify-self:center; }
  .transfer-workbench > header { grid-template-columns:minmax(190px,.7fr) auto; }
  .transfer-workbench > header p { grid-column:1/-1; }
  .diff-toolbar { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .diff-toolbar .diff-search,.diff-toolbar-actions { grid-column:1/-1; }
  .diff-row { grid-template-columns:minmax(140px,.6fr) minmax(220px,1fr); }
  .transfer-review-list { grid-template-columns:minmax(0,1fr); }
}
@container save-diff (max-width:620px) {
  .source-grid { grid-template-columns:minmax(0,1fr); }
  .known-save-pickers { grid-template-columns:minmax(0,1fr); }
  .compare-mark { min-height:24px; transform:rotate(90deg); }
  .compare-button { grid-column:1; width:100%; }
  .diff-summary { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .transfer-workbench > header { grid-template-columns:minmax(0,1fr); align-items:start; }
  .transfer-target { justify-self:start; width:100%; }
  .transfer-target .ui-seg-btn { flex:1; }
  .transfer-workbench > header p { grid-column:1; }
  .transfer-actions { align-items:stretch; flex-direction:column; }
  .transfer-actions > span { margin-right:0; }
  .transfer-actions .ui-btn { width:100%; }
  .fate-summary { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .infinity-summary,.infinity-difficulties { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .diff-toolbar { grid-template-columns:minmax(0,1fr); }
  .diff-toolbar label,.diff-toolbar .diff-search,.diff-toolbar-actions { grid-column:1; }
  .diff-toolbar-actions { display:grid; grid-template-columns:minmax(0,1fr); }
  .diff-toolbar-actions .ui-btn { width:100%; }
  .diff-count { display:grid; }
  .diff-row { grid-template-columns:minmax(0,1fr); align-items:start; }
  .diff-row dl { grid-column:1; }
  .transfer-lane { grid-template-columns:minmax(0,1fr); }
  .transfer-one,.transfer-blocked { min-height:50px; }
  .record-identity strong,.record-identity small,.value-side small,.value-side span,.value-side code { white-space:normal; overflow-wrap:anywhere; }
  .transfer-review > header { align-items:stretch; flex-direction:column; }
  .transfer-review > header .ui-btn,.transfer-apply { width:100%; min-width:0; }
  .transfer-review-list button { grid-template-columns:minmax(0,1fr) 20px; }
  .transfer-review-list em { grid-column:1; }
  .transfer-review-list i { grid-column:2; grid-row:1/3; }
  .fate-source { grid-template-columns:minmax(0,1fr); }
  .fate-source .ui-btn { width:100%; }
  .fate-actions { align-items:stretch; flex-direction:column; }
  .fate-actions .ui-btn { width:100%; }
  .fate-action-buttons { width:100%; display:grid; grid-template-columns:minmax(0,1fr); }
  .fate-write-panel > header { align-items:stretch; flex-direction:column; }
  .fate-write-summary { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .fate-write-summary p { grid-column:1/-1; }
  .fate-write-actions { display:grid; grid-template-columns:minmax(0,1fr); }
  .fate-write-actions .ui-btn { width:100%; }
  .fate-detail > header { align-items:start; flex-direction:column; }
  .fate-detail > header span { justify-content:start; text-align:left; }
  .fate-episode-list { grid-template-columns:minmax(0,1fr); }
  .infinity-quest { grid-template-columns:minmax(0,1fr); }
  .infinity-rule summary { grid-template-columns:minmax(0,1fr); }
}
</style>
