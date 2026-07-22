<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { LoadoutApplyWithResources, LoadoutCheckCompliance, LoadoutEditContext, LoadoutExport, LoadoutImport, LoadoutRuntimePanelStats, LoadoutSimulateBuild, LoadoutStatContext, MasteryNodePool, MasterySummarize, SummonGetOptions } from '../../wailsjs/go/backend/App'
import { GetSigilList, GetTraitList, GetCompatibleSecondaryTraits } from '../../wailsjs/go/backend/SigilGen'
import { groupMasteryNodes, inferMasteryDirection, limitMasteryHashesByRankCaps, resolveMasteryHashes } from '../loadoutMastery'
import { buildFactorWritePayload, clearFactorSlot, createFactorSlots, factorSlotCount, putBagFactor, putConstructedFactor } from '../loadoutFactorSlots'
import { formatFinalStat, formatWeaponSkillLevel, groupEffectTotals, summarizeTraitLevels } from '../loadoutFinalStats'
import { resolveVirtualGridWindow } from '../loadoutVirtualGrid'
import { buildConstructCatalog, collectBagTraitOptions, filterAndSortBagSigils, filterConstructCatalog, resolveConstructSelection } from '../loadoutCatalogFilters'
import { characterAssetIcon, summonAssetIcon, traitAssetIcon, weaponAssetIcon } from '../gameAssetIcons'
import skillIconFiles from '../loadoutSkillIcons.json'
import CatalogSelect from './CatalogSelect.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const props = defineProps({
  savePath: { type: String, default: '' },
  charaHash: { type: String, default: '' },
  charaName: { type: String, default: '' },
  loadouts: { type: Array, default: () => [] },
})
const emit = defineEmits(['status', 'reload'])

const confirmDialog = ref(null)
const ctx = ref(null)
const loading = ref(false)
const applying = ref(false)
const sharing = ref(false)
const importMissing = ref([])
const importApplyPayload = ref(null)

const targetSlot = ref(0)          // 目标预设槽 unitId
const op = ref('write')            // write | clone | clear
const form = ref({ name: '', weaponSlotId: 0, skillHashes: [], masterySource: 0 })
const factorSlots = ref(createFactorSlots())
const activeFactorIndex = ref(0)
const cloneFrom = ref(0)
const sigilSearch = ref('')
const bagStateFilter = ref('all')
const bagTraitFilter = ref('')
const bagSort = ref('slot-asc')
const bagViewport = ref(null)
const bagScrollTop = ref(0)
const bagViewportWidth = ref(900)
const bagViewportHeight = ref(420)
let bagResizeObserver = null
const factorMode = ref('construct')
const masteryExpanded = ref(false)
const pendingSkillHash = ref('')
const constructCatalog = ref([])
const constructTraits = ref([])
const constructCompatibleTraits = ref([])
const constructSearch = ref('')
const constructSigilId = ref('')
const constructPrimaryId = ref('')
const constructSecondaryId = ref('')
const constructSigilLevel = ref(0)
const constructPrimaryLevel = ref(0)
const constructSecondaryLevel = ref(0)
const constructLoading = ref(false)
const statContext = ref({ summons: [], equippedSummonSlotIds: [], equippedSummons: [], summonSystemAvailable: false, summonSystemState: 'unknown', overLimit: [], warnings: [] })
const summonSlotIds = ref([0, 0, 0, 0])
const writeGlobalSummons = ref(false)
const summonCatalog = ref({ traits: [], subParams: [] })
const summonInlineEnabled = ref(false)
const summonDrafts = ref({})
const weaponInlineEnabled = ref(false)
const weaponSkillDrafts = ref([])
const finalStats = ref(null)
const simulationError = ref('')
const weaponSkills = ref([])
const selectedWeaponContext = ref(null)
const runtimePanelStats = ref(null)
const runtimePanelLoading = ref(false)
const runtimePanelError = ref('')
const statPanelMode = ref('estimate')
const displayedPanelStats = computed(() => statPanelMode.value === 'runtime' && runtimePanelStats.value
  ? { ...runtimePanelStats.value, damageCap: finalStats.value?.damageCap }
  : (finalStats.value || {}))
const calculationFormulaVerified = computed(() => Boolean(finalStats.value?.formulaVerified)
  && (!selectedWeaponContext.value || Boolean(selectedWeaponContext.value?.formulaVerified)))
const calculationWarnings = computed(() => {
  const warnings = [
    ...(statContext.value?.warnings || []),
    ...(finalStats.value?.warnings || []),
    ...(selectedWeaponContext.value?.warnings || []),
  ]
  if (selectedWeaponContext.value && !selectedWeaponContext.value.formulaVerified) {
    warnings.push('当前武器仍有未完全解析的属性或技能效果。')
  }
  return [...new Set(warnings.filter(Boolean))]
})
const runtimeStatComparisons = computed(() => {
  if (!runtimePanelStats.value || !finalStats.value) return []
  return [
    ['HP', 'hp', ''],
    ['攻击力', 'attack', ''],
    ['暴击率', 'critRate', 'pct'],
    ['昏厥值', 'stunPower', ''],
  ].map(([label, field, unit]) => {
    const estimate = Number(finalStats.value?.[field] || 0)
    const runtime = Number(runtimePanelStats.value?.[field] || 0)
    return { label, field, unit, estimate, runtime, delta: estimate - runtime }
  })
})

// 名称字节数（后端上限 63）
function utf8Bytes(s) { return new TextEncoder().encode(s || '').length }
const nameBytes = computed(() => utf8Bytes(form.value.name))
const nameTooLong = computed(() => nameBytes.value > 63)

const slots = computed(() => ctx.value?.slots || [])
const occupiedSlots = computed(() => slots.value.filter(s => s.occupied))
const masterySources = computed(() => ctx.value?.masterySources || [])
const configuredFactorCount = computed(() => factorSlotCount(factorSlots.value))
const factorSlotCards = computed(() => factorSlots.value.map((entry, index) => {
  if (!entry) return { index, empty: true }
  if (entry.kind === 'construct') {
    return { index, kind: 'construct', ...entry.preview, slotId: 0 }
  }
  const sigil = (ctx.value?.sigils || []).find(item => item.slotId === entry.slotId)
  return {
    index,
    kind: 'bag',
    slotId: entry.slotId,
    hash: sigil?.hash || '',
    name: sigil?.name || '未收录因子',
    level: sigil?.level || 0,
    primaryTraitHash: sigil?.primaryTraitHash || '',
    primaryTraitName: sigil?.primaryTraitName || '',
    primaryTraitLevel: sigil?.primaryTraitLevel || 0,
    secondaryTraitHash: sigil?.secondaryTraitHash || '',
    secondaryTraitName: sigil?.secondaryTraitName || '',
    secondaryTraitLevel: sigil?.secondaryTraitLevel || 0,
  }
}))
const activeFactorCard = computed(() => factorSlotCards.value[activeFactorIndex.value] || { index: activeFactorIndex.value, empty: true })
const usedBagSlotIds = computed(() => new Set(factorSlots.value
  .filter(entry => entry?.kind === 'bag')
  .map(entry => Number(entry.slotId))))
const bagTraitOptions = computed(() => collectBagTraitOptions(ctx.value?.sigils || []))
const filteredSigils = computed(() => filterAndSortBagSigils(ctx.value?.sigils || [], {
  query: sigilSearch.value,
  state: bagStateFilter.value,
  trait: bagTraitFilter.value,
  sort: bagSort.value,
  usedSlotIds: usedBagSlotIds.value,
}))
const bagWindow = computed(() => resolveVirtualGridWindow({
  itemCount: filteredSigils.value.length,
  viewportWidth: bagViewportWidth.value,
  viewportHeight: bagViewportHeight.value,
  scrollTop: bagScrollTop.value,
}))
const visibleSigils = computed(() => filteredSigils.value.slice(bagWindow.value.startIndex, bagWindow.value.endIndex))
const bagSpacerStyle = computed(() => ({ height: `${bagWindow.value.totalHeight}px` }))
const bagWindowStyle = computed(() => ({
  '--bag-columns': String(bagWindow.value.columns),
  transform: `translateY(${bagWindow.value.offsetTop}px)`,
}))

function resetBagScroll() {
  bagScrollTop.value = 0
  nextTick(() => {
    if (bagViewport.value) bagViewport.value.scrollTop = 0
  })
}

function onBagScroll(event) {
  bagScrollTop.value = event.currentTarget.scrollTop
}
function assetPath(folder, file) {
  if (!file) return ''
  return `/loadout-icons/${folder}/${String(file).split('/').map(part => encodeURIComponent(part).replace(/'/g, '%27')).join('/')}`
}
function skillIcon(skill) {
  const verifiedFile = skillIconFiles[skill?.key || ''] || ''
  return assetPath('skills', verifiedFile || 'Plain_Skill_Frame.png')
}
function traitIcon(name, hash = '', internalId = '') { return traitAssetIcon({ name, hash, internalId }) }
function traitIconForOption(trait) { return traitAssetIcon({ name: trait?.displayName, hash: trait?.hash, internalId: trait?.internalId }) }

function normalizedHash(value) { return String(value || '').replace(/^0x/i, '').toUpperCase() }
const characterAvatar = computed(() => characterAssetIcon(props.charaHash))
const selectedWeaponPick = computed(() => ctx.value?.weapons?.find(item => Number(item.slotId) === Number(form.value.weaponSlotId)) || null)
const selectedWeaponIcon = computed(() => weaponAssetIcon(selectedWeaponContext.value || selectedWeaponPick.value || {}))
const importedSummonsByIndex = computed(() => new Map((importApplyPayload.value?.constructedSummons || [])
  .map(item => [Number(item.index), item])))
const selectedSummons = computed(() => summonSlotIds.value.map((slotId, index) => {
  const existing = statContext.value.summons.find(item => item.slotId === slotId)
  if (existing) return existing
  const pending = importedSummonsByIndex.value.get(index)
  if (!pending) return null
  return {
    slotId: 0, unitId: 0, typeHash: hashHex(pending.state?.typeHash), name: pending.name || '导入召唤石（将自动生成）',
    mainTraitHash: hashHex(pending.state?.mainTraitHash), mainTraitName: '导入主加护', mainTraitLevel: Number(pending.state?.mainTraitLevel || 0),
    subParamHash: hashHex(pending.state?.subParamHash), subParamName: '导入副参数', subParamLevel: Number(pending.state?.subParamLevel || 0),
    subParamValue: 0, subParamUnit: '', rank: Number(pending.state?.rank || 0), pendingImport: true,
  }
}))
const runtimeWeaponPick = computed(() => ctx.value?.weapons?.find(item => Number(item.slotId) === Number(runtimePanelStats.value?.currentWeaponSlotId)) || null)
function factorIdentity(factor) {
  const identityHash = value => {
    const hash = normalizedHash(value)
    return !hash || /^0+$/.test(hash) || hash === 'FFFFFFFF' ? '' : hash
  }
  return [
    identityHash(factor?.hash || factor?.itemHash),
    identityHash(factor?.primaryTraitHash), Number(factor?.primaryTraitLevel || 0),
    identityHash(factor?.secondaryTraitHash), Number(factor?.secondaryTraitLevel || 0),
  ].join(':')
}
function runtimeFactorPick(factor) {
  const inventory = ctx.value?.sigils || []
  return inventory.find(item => Number(item.slotId) === Number(factor?.runtimeSlotId))
    || inventory.find(item => factorIdentity(item) === factorIdentity(factor))
    || null
}
const runtimeFactorCards = computed(() => (runtimePanelStats.value?.currentFactors || []).map(factor => ({
  ...factor,
  name: runtimeFactorPick(factor)?.name || factor.itemHash || '未识别因子',
})))
const factorComparison = computed(() => {
  const runtime = runtimeFactorCards.value
  if (!runtime.length || !runtimePanelStats.value?.currentFactorStableReads) {
    return { available: false, match: false, detail: '运行时因子指纹尚未读取' }
  }
  const draft = factorSlotCards.value.filter(item => !item.empty)
  const count = Math.max(runtime.length, draft.length)
  for (let index = 0; index < count; index += 1) {
    if (!runtime[index] || !draft[index] || factorIdentity(runtime[index]) !== factorIdentity(draft[index])) {
      const draftName = draft[index]?.name || '空槽'
      const runtimeName = runtime[index]?.name || '空槽'
      return { available: true, match: false, index, detail: `第 ${index + 1} 槽：草稿 ${draftName}，游戏 ${runtimeName}` }
    }
  }
  return { available: true, match: true, detail: `${runtime.length} 枚因子逐槽一致` }
})
const runtimeComparisonRelation = computed(() => {
  if (!runtimePanelStats.value || !finalStats.value) return { kind: 'unavailable', comparable: false, exact: false }
  const exact = runtimeStatComparisons.value.every(row => Math.abs(row.delta) < 0.0001)
  const runtimeWeaponSlot = Number(runtimePanelStats.value.currentWeaponSlotId || 0)
  const draftWeaponSlot = Number(form.value.weaponSlotId || 0)
  if (runtimeWeaponSlot && draftWeaponSlot && runtimeWeaponSlot !== draftWeaponSlot) {
    return {
      kind: 'different-weapon', comparable: false, exact: false,
      title: '当前是两套不同配装，不是校准失败',
      detail: `草稿武器槽 ${draftWeaponSlot}，游戏当前武器槽 ${runtimeWeaponSlot}`,
    }
  }
  if (factorComparison.value.available && !factorComparison.value.match) {
    return {
      kind: 'different-factors', comparable: false, exact: false,
      title: '当前游戏与草稿的因子不同，不是同一套配装',
      detail: factorComparison.value.detail,
    }
  }
  if (exact) return { kind: 'exact', comparable: true, exact: true, title: '草稿与当前游戏四项一致', detail: '四项最终值逐项相同' }
  return {
    kind: 'same-weapon-different-loadout', comparable: false, exact: false,
    title: factorComparison.value.match ? '武器和因子相同，但专精或其他来源不同' : '武器相同，但因子、专精或其他来源尚未证明相同',
    detail: factorComparison.value.match ? '因子已逐槽核对；差值继续定位到专精、强化或全局效果' : '差值用于定位来源，不标记为公式错误',
  }
})
const draftSourceSummary = computed(() => ({
  weapon: selectedWeaponContext.value?.name || selectedWeaponPick.value?.name || '未选择武器',
  weaponSlot: Number(form.value.weaponSlotId || 0),
  factors: factorSlots.value.filter(Boolean).length,
  mastery: selectedMasteryHashes.value.length,
  summons: selectedSummons.value.filter(Boolean).length,
}))
const editableWeaponSkillSlots = computed(() => (selectedWeaponContext.value?.skillSlots || []).filter(slot => slot.editable))
const weaponInlineAvailable = computed(() => editableWeaponSkillSlots.value.length > 0)
const summonSelectionValid = computed(() => {
  const ids = summonSlotIds.value.map(Number)
  if (ids.length !== 4) return false
  const existing = ids.filter(id => id > 0)
  if (new Set(existing).size !== existing.length) return false
  return ids.every((id, index) => (id > 0 && statContext.value.summons.some(item => item.slotId === id)) || importedSummonsByIndex.value.has(index))
})
function summonUsedElsewhere(slotId, currentIndex) {
  return summonSlotIds.value.some((value, index) => index !== currentIndex && Number(value) === Number(slotId))
}
function hashHex(value) {
  if (typeof value === 'string') return normalizedHash(value).padStart(8, '0')
  return Number(value || 0).toString(16).toUpperCase().padStart(8, '0')
}
function summonSnapshotKey(summon) {
  if (!summon) return '0'
  return [summon.slotId, summon.unitId, summon.typeHash, summon.mainTraitHash, summon.mainTraitLevel,
    summon.subParamHash, summon.subParamLevel, summon.rank].join(':')
}
function makeSummonDraft(summon) {
  return {
    mainTraitHash: hashHex(summon.mainTraitHash),
    mainTraitLevel: Number(summon.mainTraitLevel),
    subParamHash: hashHex(summon.subParamHash),
    subParamLevel: Number(summon.subParamLevel),
    rank: Number(summon.rank),
  }
}
function summonMainOption(hash) {
  const normalized = normalizedHash(hash)
  return summonCatalog.value.traits.find(option => hashHex(option.hash) === normalized) || null
}
function summonSubOption(hash) {
  const normalized = normalizedHash(hash)
  return summonCatalog.value.subParams.find(option => hashHex(option.hash) === normalized) || null
}
function summonMainLevelLimit(draft) {
  return Math.min(15, Math.max(0, Number(summonMainOption(draft?.mainTraitHash)?.maxLevel || 0)))
}
function summonSubLevelLimit(draft) {
  return Math.min(9, Math.max(0, Number(summonSubOption(draft?.subParamHash)?.maxLevel || 0)))
}
function clampSummonDraft(draft) {
  if (!draft) return
  draft.mainTraitLevel = Math.min(Math.max(0, Math.trunc(Number(draft.mainTraitLevel) || 0)), 0xFFFFFFFF)
  draft.subParamLevel = Math.min(Math.max(0, Math.trunc(Number(draft.subParamLevel) || 0)), 0xFFFFFFFF)
  draft.rank = Math.min(Math.max(0, Math.trunc(Number(draft.rank) || 0)), 0xFFFFFFFF)
}
function summonDraftChanged(summon, draft) {
  return Boolean(summon && draft) && (
    normalizedHash(draft.mainTraitHash) !== normalizedHash(summon.mainTraitHash)
    || Number(draft.mainTraitLevel) !== Number(summon.mainTraitLevel)
    || normalizedHash(draft.subParamHash) !== normalizedHash(summon.subParamHash)
    || Number(draft.subParamLevel) !== Number(summon.subParamLevel)
    || Number(draft.rank) !== Number(summon.rank)
  )
}
function buildWeaponInlineEdits() {
  const weapon = selectedWeaponContext.value
  const current = (weapon?.skillSlots || []).map(slot => normalizedHash(slot.currentHash))
  const draft = weaponSkillDrafts.value.map(normalizedHash)
  if (op.value !== 'write' || !weaponInlineEnabled.value || !weaponInlineAvailable.value
    || current.length !== 5 || draft.length !== 5 || current.every((hash, index) => hash === draft[index])) return []
  return [{
    slotId: Number(weapon.slotId),
    expectUnitId: Number(weapon.unitId),
    expectStoredHash: weapon.storedHash,
    expectTranscendence: Number(weapon.transcendence),
    expectSkillHashes: current,
    skillHashes: draft,
  }]
}
function buildSummonInlineEdits() {
  if (op.value !== 'write' || !summonInlineEnabled.value || !summonSelectionValid.value) return []
  return selectedSummons.value.flatMap((summon) => {
    const draft = summonDrafts.value[summon?.slotId]
    if (!summonDraftChanged(summon, draft)) return []
    return [{
      slotId: Number(summon.slotId),
      expectUnitId: Number(summon.unitId),
      expectTypeHash: summon.typeHash,
      expectMainTraitHash: summon.mainTraitHash,
      expectMainTraitLevel: Number(summon.mainTraitLevel),
      expectSubParamHash: summon.subParamHash,
      expectSubParamLevel: Number(summon.subParamLevel),
      expectRank: Number(summon.rank),
      mainTraitHash: draft.mainTraitHash,
      mainTraitLevel: Number(draft.mainTraitLevel),
      subParamHash: draft.subParamHash,
      subParamLevel: Number(draft.subParamLevel),
      rank: Number(draft.rank),
    }]
  })
}

let inlineWeaponUnitID = 0
watch(() => [selectedWeaponContext.value?.unitId || 0, JSON.stringify(selectedWeaponContext.value?.skillSlots || [])], ([unitId]) => {
  if (Number(unitId) !== inlineWeaponUnitID) weaponInlineEnabled.value = false
  inlineWeaponUnitID = Number(unitId)
  weaponSkillDrafts.value = (selectedWeaponContext.value?.skillSlots || []).map(slot => normalizedHash(slot.currentHash))
})
watch(() => selectedSummons.value.map(summonSnapshotKey).join('|'), () => {
  const next = {}
  for (const summon of selectedSummons.value) {
    if (summon) next[summon.slotId] = makeSummonDraft(summon)
  }
  summonDrafts.value = next
}, { immediate: true })
watch(summonInlineEnabled, (enabled) => {
  if (enabled) writeGlobalSummons.value = true
})
watch(writeGlobalSummons, (enabled) => {
  if (!enabled) summonInlineEnabled.value = false
})
function formatStatNumber(value) {
  return Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}
function formatSignedValue(value, unit = '') {
  const numeric = Number(value || 0)
  const sign = numeric > 0 ? '+' : numeric < 0 ? '−' : ''
  return `${sign}${formatStatNumber(Math.abs(numeric))}${unit === 'pct' ? '%' : ''}`
}
function formatPanelStat(field, unit = '') {
  const value = displayedPanelStats.value?.[field]
  if (value === null || value === undefined) return '—'
  const formatted = formatFinalStat(value, unit)
  const isRuntimeExact = statPanelMode.value === 'runtime' && Boolean(runtimePanelStats.value) && field !== 'damageCap'
  if (isRuntimeExact) return formatted
  return `≈${formatted}`
}
function formatComparisonStat(value, unit = '') { return formatFinalStat(value, unit) }
function formatComparisonDelta(value, unit = '') {
  const numeric = Number(value || 0)
  if (Math.abs(numeric) < 0.0001) return '一致'
  return `草稿${numeric > 0 ? '+' : '−'}${formatFinalStat(Math.abs(numeric), unit)}`
}
function defenseEvidenceLabel(value) {
  return ({
    '2.0.2-table + Io runtime +5%': '解包表 + 伊欧受击实测',
    'reference-candidate': '参考候选 · 待本机复测',
    '2.0.2-table + reference-zone': '2.0.2 解包表 + 分区参考',
    '2.0.2-table; runtime-curve-open': '2.0.2 解包表 · 曲线未闭环',
    'battle-state-unavailable': '需要战斗状态',
  })[value] || value
}
function mergedTraitBonus(trait) {
  const id = String(trait?.traitId || '').toUpperCase()
  const hash = normalizedHash(trait?.hash)
  return bonuses.value.find(item => String(item?.traitId || '').toUpperCase() === id || normalizedHash(item?.traitId) === hash) || null
}

async function readRuntimePanel(silent = false) {
  if (!props.charaHash || runtimePanelLoading.value) return
  runtimePanelLoading.value = true
  runtimePanelError.value = ''
  try {
    runtimePanelStats.value = await LoadoutRuntimePanelStats(props.charaHash)
    statPanelMode.value = 'runtime'
  } catch (err) {
    runtimePanelStats.value = null
    statPanelMode.value = 'estimate'
    if (!silent) runtimePanelError.value = String(err)
  } finally {
    runtimePanelLoading.value = false
  }
}
function summonOptionLabel(summon) {
  const main = summon.mainTraitName ? `${summon.mainTraitName} Lv${summon.mainTraitLevel}` : '无主词条'
  const sub = summon.subParamName ? `${summon.subParamName} ${formatSignedValue(summon.subParamValue, summon.subParamUnit)}` : '无副参数'
  return `${summon.name} · ${main} · ${sub}`
}
const fullConstructCatalog = computed(() => buildConstructCatalog(constructCatalog.value, ctx.value?.sigils || []))
const filteredConstructCatalog = computed(() => {
  return filterConstructCatalog(fullConstructCatalog.value, constructSearch.value)
})
const selectedConstructSigil = computed(() => fullConstructCatalog.value.find(item => item.internalId === constructSigilId.value) || null)
const selectedConstructPrimary = computed(() => constructTraits.value.find(item => item.internalId === constructPrimaryId.value) || null)
const constructSecondaryOptions = computed(() => constructTraits.value)
const selectedConstructSecondary = computed(() => constructSecondaryOptions.value.find(item => item.internalId === constructSecondaryId.value) || null)
function highestAllowed(levels, fallback = 0) {
  return (levels || []).reduce((max, value) => value <= 15 && value > max ? value : max, Math.min(fallback, 15))
}
function constructTraitWritableMax(trait) { return Math.min(50, Math.max(15, Number(trait?.maxLevel || 0))) }
function constructLevelLimit() { return 0x7FFFFFFF }
function clampConstructLevel(value, max = 50) {
  const number = Number.isFinite(Number(value)) ? Math.trunc(Number(value)) : 0
  return Math.min(max, Math.max(0, number))
}
function onConstructSecondaryPick(trait) {
  constructSecondaryLevel.value = trait ? Math.min(15, constructTraitWritableMax(trait)) : 0
}

async function loadConstructCatalog() {
  if (constructCatalog.value.length || constructLoading.value) return
  constructLoading.value = true
  try {
    ;[constructCatalog.value, constructTraits.value] = await Promise.all([GetSigilList(), GetTraitList()])
    const first = fullConstructCatalog.value.find(item => item.allowedSigilLevels?.length && item.allowedFirstTraitLevels?.length)
    if (first) constructSigilId.value = first.internalId
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    constructLoading.value = false
  }
}

watch(factorMode, value => { if (value === 'construct') loadConstructCatalog() }, { immediate: true })
watch(fullConstructCatalog, items => {
  if (items.some(item => item.internalId === constructSigilId.value)) return
  const first = items.find(item => item.allowedSigilLevels?.length && item.allowedFirstTraitLevels?.length)
  constructSigilId.value = first?.internalId || ''
}, { immediate: true })
watch(filteredConstructCatalog, matches => {
  constructSigilId.value = resolveConstructSelection(matches, constructSigilId.value, constructSearch.value)
})
let pendingConstructRestore = null
let constructCompatibilityTicket = 0
watch(constructSigilId, async value => {
  const ticket = ++constructCompatibilityTicket
  const sigil = fullConstructCatalog.value.find(item => item.internalId === value)
  constructSigilLevel.value = highestAllowed(sigil?.allowedSigilLevels, sigil?.defaultSigilLevel || 0)
  constructPrimaryLevel.value = highestAllowed(sigil?.allowedFirstTraitLevels, sigil?.firstTraitMaxLevel || 0)
  constructPrimaryId.value = sigil?.primaryTraitId || ''
  constructSecondaryId.value = ''
  constructSecondaryLevel.value = 0
  try {
    const compatible = value && sigil?.supportsSecondaryTrait ? await GetCompatibleSecondaryTraits(value) : []
    if (ticket !== constructCompatibilityTicket) return
    constructCompatibleTraits.value = compatible || []
  } catch (err) {
    if (ticket !== constructCompatibilityTicket) return
    constructCompatibleTraits.value = []
    emit('status', `读取因子副词条池失败：${String(err)}`, 'error')
  }
  const restore = pendingConstructRestore?.sigilId === value ? pendingConstructRestore : null
  constructSigilLevel.value = restore?.level || constructSigilLevel.value
  constructPrimaryLevel.value = restore?.primaryLevel || constructPrimaryLevel.value
  if (restore && constructSecondaryOptions.value.some(item => item.internalId === restore.secondaryTraitId)) {
    constructSecondaryId.value = restore.secondaryTraitId
    constructSecondaryLevel.value = restore.secondaryLevel || 0
  }
  if (restore) pendingConstructRestore = null
})

// 专精：复制现有 or 自由配置（4 档 10/10/10/20）
const masteryMode = ref('free')     // copy | free
const masteryPool = ref([])         // [{rank,label,cap,nodes}]
const masteryPick = ref({})         // { R1:[hash...], R2:[], R3:[], EX:[] }
const masteryRankTab = ref('R1')
const CAT_ABBR = { SB_ATK: '攻', SB_DEF: '防', SB_LIMIT: '界' }
function catAbbr(cat) { return CAT_ABBR[cat] || '基' }
const activeRankPool = computed(() => masteryPool.value.find(p => p.rank === masteryRankTab.value) || null)
const activeRankGroups = computed(() => groupMasteryNodes(activeRankPool.value?.nodes || []))
function rankPicked(rank) { return (masteryPick.value[rank] || []).length }
const masteryRanks = ['R1', 'R2', 'R3', 'EX']
const masteryStructuralRankCap = rank => {
  return Number(masteryPool.value.find(pool => pool.rank === rank)?.cap || 0)
}
const masteryCapacity = computed(() => masteryRanks.reduce((total, rank) => total + masteryStructuralRankCap(rank), 0))
const masteryUnlockAmbiguous = computed(() => Boolean(statContext.value?.permanentGrowth)
  && Number(statContext.value.permanentGrowth.masterTotalMsp || 0) === 0)
const masteryRankCaps = computed(() => masteryUnlockAmbiguous.value
  ? { R1: 0, R2: 0, R3: 0, EX: 0 }
  : (statContext.value?.permanentGrowth?.masteryRankCaps || {}))
const masteryUnlockedRankCap = rank => Math.max(0, Number(masteryRankCaps.value?.[rank] || 0))
const masteryUnlockedCapacity = computed(() => masteryRanks.reduce((total, rank) => total + masteryUnlockedRankCap(rank), 0))
function toggleNode(rank, hash, cap) {
  const arr = masteryPick.value[rank] || (masteryPick.value[rank] = [])
  const i = arr.indexOf(hash)
  if (i >= 0) arr.splice(i, 1)
  else {
    if (arr.length < cap) arr.push(hash)
  }
}

function masteryNodeDisabled(rank, node) {
  const selected = (masteryPick.value[rank] || []).includes(node.hash)
  return !selected && rankPicked(rank) >= masteryStructuralRankCap(rank)
}

async function loadMasteryPool() {
  masteryPool.value = []
  masteryPick.value = { R1: [], R2: [], R3: [], EX: [] }
  if (!ctx.value?.ownerCode) return
  try { masteryPool.value = (await MasteryNodePool(ctx.value.ownerCode)) || [] }
  catch (err) { emit('status', String(err), 'error') }
}

const selectedSlot = computed(() => slots.value.find(s => s.unitId === targetSlot.value) || null)
const selectedLoadout = computed(() => props.loadouts.find(item => item.unitId === targetSlot.value) || null)
const selectedMasteryHashes = computed(() => resolveMasteryHashes({
  mode: masteryMode.value,
  picks: masteryPick.value,
  sourceId: form.value.masterySource,
  sources: masterySources.value,
}))
const masteryNodeByHash = computed(() => {
  const result = new Map()
  for (const pool of masteryPool.value) for (const node of pool.nodes || []) result.set(node.hash, { ...node, rank: pool.rank, rankLabel: pool.label })
  return result
})
const effectiveMasteryHashes = computed(() => limitMasteryHashesByRankCaps(
  selectedMasteryHashes.value,
  masteryNodeByHash.value,
  masteryRankCaps.value,
))
const effectiveMasteryPick = computed(() => {
  const result = { R1: [], R2: [], R3: [], EX: [] }
  for (const hash of effectiveMasteryHashes.value) {
    const rank = masteryNodeByHash.value.get(hash)?.rank
    if (rank && result[rank]) result[rank].push(hash)
  }
  return result
})
const masteryDraftOverflow = computed(() => Math.max(0, selectedMasteryHashes.value.length - effectiveMasteryHashes.value.length))
const masteryDirection = computed(() => inferMasteryDirection(masteryPick.value, masteryNodeByHash.value))
const effectiveMasteryDirection = computed(() => inferMasteryDirection(effectiveMasteryPick.value, masteryNodeByHash.value))
function masteryCategoryPicked(rank, cat) {
  return (effectiveMasteryPick.value[rank] || []).filter(hash => masteryNodeByHash.value.get(hash)?.cat === cat).length
}
function masteryStageSkillPicked(rank, cat) {
  return (effectiveMasteryPick.value[rank] || []).some(hash => {
    const node = masteryNodeByHash.value.get(hash)
    return node?.cat === cat && node.specialization
  })
}
const displayedMasteryDirection = computed(() => masterySummary.value?.primaryCat || '')
const selectedMasteryDetails = computed(() => effectiveMasteryHashes.value.map(hash => masteryNodeByHash.value.get(hash)).filter(Boolean))
const selectedSkills = computed(() => form.value.skillHashes.map(hash => ctx.value?.skills?.find(skill => skill.hash === hash)).filter(Boolean))
const masterySummary = ref(null)
const summarizingMastery = ref(false)
const masteryDirectionCards = computed(() => {
  const rankOne = masteryPool.value.find(pool => pool.rank === 'R1')
  return groupMasteryNodes(rankOne?.nodes || []).map(group => {
    const rows = ['R1', 'R2', 'R3'].map(rank => {
      const rankSummary = masterySummary.value?.ranks?.find(item => item.rank === rank)
      const category = rankSummary?.categories?.find(item => item.cat === group.cat)
      const threshold = category?.threshold || (rank === 'R1' ? 3 : 6)
      if (masteryMode.value === 'free') {
        const count = masteryCategoryPicked(rank, group.cat)
        const rankCap = masteryUnlockedRankCap(rank)
        const hasStageSkill = masteryStageSkillPicked(rank, group.cat)
        const directionMatches = rank === 'R1' || effectiveMasteryDirection.value === group.cat
        let reason = ''
        if (rankCap === 0) reason = `角色强化 Lv${statContext.value?.permanentGrowth?.masterLevel || 1} 尚未解锁`
        else if (rankCap < threshold) reason = `本阶已解锁 ${rankCap}/${threshold}，尚不能激活效果`
        else if (rank !== 'R1' && !effectiveMasteryDirection.value) reason = '当前生效的2阶节点尚未形成唯一主方向'
        else if (rank !== 'R1' && !directionMatches) reason = '非推导主方向，专精技能通常不生效'
        else if (!hasStageSkill) reason = `未选择${rank === 'R1' ? '1阶' : rank === 'R2' ? '2阶' : '3阶'}专精技能`
        else if (count < threshold) reason = `需 ${threshold} 项，当前 ${count} 项`
        return {
          rank,
          label: rank === 'R1' ? '1阶' : rank === 'R2' ? '2阶' : '3阶',
          count,
          threshold,
          active: rankCap >= threshold && directionMatches && hasStageSkill && count >= threshold,
          reason,
          effect: category?.effect || masteryPool.value.find(pool => pool.rank === rank)?.nodes?.find(node => node.cat === group.cat && node.specialization)?.desc || '',
        }
      }
      return {
        rank,
        label: rank === 'R1' ? '1阶' : rank === 'R2' ? '2阶' : '3阶',
        count: category?.count || 0,
        threshold,
        active: !!category?.active,
        reason: category?.reason || (rank === 'R1' ? '三个方向均可激活' : rank === 'R2' ? '达到门槛后成为唯一主方向' : '必须沿用2阶主方向'),
        effect: category?.effect || masteryPool.value.find(pool => pool.rank === rank)?.nodes?.find(node => node.cat === group.cat && node.specialization)?.desc || '',
      }
    })
    const summaryCategory = masterySummary.value?.ranks?.find(item => item.rank === 'R1')?.categories?.find(item => item.cat === group.cat)
    return {
      cat: group.cat,
      label: group.label,
      specialization: summaryCategory?.specialization || group.nodes.find(node => node.name)?.name || group.label,
      rows,
    }
  })
})

function masteryNodeTitle(rank, node) {
  if (node.name) return node.name
  if (!node.specialization) return ''
  const direction = masteryDirectionCards.value.find(item => item.cat === node.cat)
  return `${rank === 'R2' ? '2阶' : '3阶'} · ${direction?.specialization || node.catLabel}`
}

// ── 配装模拟器：随所选因子实时算「词条加成汇总」 ──
const bonuses = ref([])
const totals = ref([])
const displayTotals = computed(() => groupEffectTotals(totals.value))
const traitLevelSummary = computed(() => summarizeTraitLevels(bonuses.value))
const simulating = ref(false)
let simTimer = null
let masteryTimer = null
let simRequestId = 0
function clearSimulationResult() {
  bonuses.value = []
  totals.value = []
  finalStats.value = null
  weaponSkills.value = []
  selectedWeaponContext.value = null
}
function refreshSim() {
  clearTimeout(simTimer)
  simTimer = setTimeout(async () => {
    const requestId = ++simRequestId
    const payload = buildFactorWritePayload(factorSlots.value)
    if (!props.savePath) {
      simulationError.value = ''
      clearSimulationResult()
      return
    }
    simulating.value = true
    simulationError.value = ''
    try {
      const result = await LoadoutSimulateBuild(
        props.savePath,
        props.charaHash,
        form.value.weaponSlotId,
        payload.sigilSlotIds,
        payload.constructedSigils,
        selectedMasteryHashes.value.slice(),
        [...summonSlotIds.value],
      )
      if (requestId !== simRequestId) return
      bonuses.value = result?.bonuses || []
      totals.value = result?.totals || []
      finalStats.value = result?.finalStats || null
      weaponSkills.value = result?.weaponSkills || []
      selectedWeaponContext.value = result?.weapon || null
    } catch (error) {
      if (requestId !== simRequestId) return
      clearSimulationResult()
      simulationError.value = `配装计算失败：${String(error)}`
    }
    finally { if (requestId === simRequestId) simulating.value = false }
  }, 180)
}
watch(factorSlots, refreshSim, { deep: true })
watch(summonSlotIds, refreshSim, { deep: true })
watch(() => form.value.weaponSlotId, refreshSim)
watch(() => selectedMasteryHashes.value.slice(), refreshSim, { deep: true })
const catClass = (label) => ({ '攻击类': 'atk', '基础能力': 'base', '防御类': 'def', '支援类': 'sup' }[label] || 'misc')
function formatEffectTotal(total) {
  const value = Number(total?.value || 0)
  const sign = value > 0 ? '+' : value < 0 ? '−' : ''
  const magnitude = Math.abs(value).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
  return `${sign}${magnitude}${total?.unit === 'pct' ? '%' : ''}`
}
const effectUnitLabel = unit => unit === 'pct' ? '比例' : '固定'

function refreshMasterySummary() {
  clearTimeout(masteryTimer)
  masteryTimer = setTimeout(async () => {
    const hashes = effectiveMasteryHashes.value
    if (!ctx.value?.ownerCode || !hashes.length) { masterySummary.value = null; return }
    summarizingMastery.value = true
    try { masterySummary.value = await MasterySummarize(ctx.value.ownerCode, hashes) }
    catch { masterySummary.value = null }
    finally { summarizingMastery.value = false }
  }, 100)
}
watch(() => effectiveMasteryHashes.value.slice(), refreshMasterySummary, { deep: true })

function setMasteryHashes(hashes) {
  masteryPick.value = { R1: [], R2: [], R3: [], EX: [] }
  for (const value of hashes || []) {
    const hash = typeof value === 'string' ? value : value.hash
    const rank = (typeof value === 'object' && value.rank) || masteryNodeByHash.value.get(hash)?.rank
    if (rank && masteryPick.value[rank]) masteryPick.value[rank].push(hash)
  }
}

function hydrateFromTarget() {
  importMissing.value = []
  importApplyPayload.value = null
  const loadout = selectedLoadout.value
  pendingSkillHash.value = ''
  form.value = {
    name: loadout?.name || '',
    weaponSlotId: loadout?.weaponSlotId || 0,
    skillHashes: (loadout?.skills || []).map(item => item.hash).filter(Boolean),
    masterySource: loadout?.unitId || 0,
  }
  factorSlots.value = createFactorSlots(loadout?.sigils || [])
  activeFactorIndex.value = Math.min(activeFactorIndex.value, 11)
  setMasteryHashes(loadout?.mastery || [])
  const fullestRank = ['R1', 'R2', 'R3', 'EX'].find(rank => rankPicked(rank) < (masteryPool.value.find(pool => pool.rank === rank)?.cap || 0))
  masteryRankTab.value = fullestRank || 'EX'
}

function selectTarget(unitId) {
  if (targetSlot.value === unitId) return
  targetSlot.value = unitId
  hydrateFromTarget()
}

async function loadCtx() {
	simRequestId++
	clearTimeout(simTimer)
	clearSimulationResult()
	simulating.value = false
  runtimePanelStats.value = null
  runtimePanelError.value = ''
  statPanelMode.value = 'estimate'
  if (!props.savePath || !props.charaHash) return
  loading.value = true
  ctx.value = null
  try {
    const [editContext, loadedStatContext, loadedSummonCatalog] = await Promise.all([
      LoadoutEditContext(props.savePath, props.charaHash),
      LoadoutStatContext(props.savePath, props.charaHash),
      SummonGetOptions(),
    ])
    ctx.value = editContext
    statContext.value = loadedStatContext || { summons: [], equippedSummonSlotIds: [], equippedSummons: [], summonSystemAvailable: false, summonSystemState: 'unknown', overLimit: [], warnings: [] }
    summonCatalog.value = loadedSummonCatalog || { traits: [], subParams: [] }
    summonSlotIds.value = [...(statContext.value.equippedSummonSlotIds || [])].slice(0, 4)
    while (summonSlotIds.value.length < 4) summonSlotIds.value.push(0)
    writeGlobalSummons.value = false
    importApplyPayload.value = null
    summonInlineEnabled.value = false
    await loadMasteryPool()
    // 默认打开该角色内容最完整的一套，真实存档可直接看到 12 因子、4 技能与 50 专精。
    const richest = [...props.loadouts].filter(item => !item.isParty).sort((a, b) =>
      (b.mastery?.length || 0) - (a.mastery?.length || 0) || (b.sigils?.length || 0) - (a.sigils?.length || 0)
    )[0]
    const empty = ctx.value.slots.find(s => !s.occupied)
    targetSlot.value = richest?.unitId || (empty || ctx.value.slots[0])?.unitId || 0
    if (occupiedSlots.value.length) cloneFrom.value = occupiedSlots.value[0].unitId
    if (masterySources.value.length) form.value.masterySource = masterySources.value[0].unitId
    hydrateFromTarget()
    void readRuntimePanel(true)
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    loading.value = false
  }
}

watch(() => [props.savePath, props.charaHash], loadCtx, { immediate: true })
watch(() => props.loadouts, (next, previous) => {
  if (next !== previous && props.savePath && props.charaHash) loadCtx()
})
watch(sigilSearch, resetBagScroll)
watch([bagStateFilter, bagTraitFilter, bagSort], resetBagScroll)
watch(() => ctx.value?.sigils, resetBagScroll)
watch(bagViewport, (nextViewport, previousViewport) => {
  if (previousViewport) bagResizeObserver?.unobserve(previousViewport)
  if (nextViewport) bagResizeObserver?.observe(nextViewport)
})

onMounted(() => {
  if (typeof ResizeObserver === 'undefined') return
  bagResizeObserver = new ResizeObserver(entries => {
    const rect = entries[0]?.contentRect
    if (!rect) return
    bagViewportWidth.value = rect.width || 900
    bagViewportHeight.value = rect.height || 420
  })
  if (bagViewport.value) bagResizeObserver.observe(bagViewport.value)
})

function selectFactorSlot(index) {
  activeFactorIndex.value = index
  const entry = factorSlots.value[index]
  if (entry?.kind !== 'construct') return
  pendingConstructRestore = { ...entry.item }
  if (constructSigilId.value === entry.item.sigilId) {
    constructSigilLevel.value = entry.item.level
    constructPrimaryId.value = entry.item.primaryTraitId || ''
    constructPrimaryLevel.value = entry.item.primaryLevel
    constructSecondaryId.value = entry.item.secondaryTraitId || ''
    constructSecondaryLevel.value = entry.item.secondaryLevel || 0
    pendingConstructRestore = null
  } else {
    constructSigilId.value = entry.item.sigilId
  }
}
function chooseBagFactor(slotId) {
  factorSlots.value = putBagFactor(factorSlots.value, activeFactorIndex.value, slotId)
}
function clearActiveFactor() {
  factorSlots.value = clearFactorSlot(factorSlots.value, activeFactorIndex.value)
}
function bagFactorSlotNumber(slotId) {
  const index = factorSlots.value.findIndex(entry => entry?.kind === 'bag' && entry.slotId === slotId)
  return index >= 0 ? index + 1 : 0
}
function toggleSkill(hash) {
  const arr = form.value.skillHashes
  const i = arr.indexOf(hash)
  if (i >= 0) {
    arr.splice(i, 1)
    if (pendingSkillHash.value && !arr.includes(pendingSkillHash.value)) arr.push(pendingSkillHash.value)
    pendingSkillHash.value = ''
  } else if (arr.length < 4) {
    arr.push(hash)
    pendingSkillHash.value = ''
  } else {
    pendingSkillHash.value = hash
  }
}
function skillOrder(hash) { return form.value.skillHashes.indexOf(hash) + 1 }
function replaceSkillAt(index) {
  if (!pendingSkillHash.value || index < 0 || index >= form.value.skillHashes.length) return
  const pending = pendingSkillHash.value
  if (form.value.skillHashes.includes(pending)) {
    pendingSkillHash.value = ''
    return
  }
  if (form.value.skillHashes.length < 4) form.value.skillHashes.push(pending)
  else form.value.skillHashes.splice(index, 1, pending)
  pendingSkillHash.value = ''
}
watch(op, (nextOp) => {
  pendingSkillHash.value = ''
  if (nextOp !== 'write') {
    weaponInlineEnabled.value = false
    summonInlineEnabled.value = false
  }
})

function stageConstructedFactor() {
  const sigil = selectedConstructSigil.value
  const primary = selectedConstructPrimary.value
  const secondary = selectedConstructSecondary.value
  if (!sigil || !primary) return
  const item = {
    sigilId: sigil.internalId,
    sigilName: sigil.displayName,
    level: constructSigilLevel.value,
    primaryTraitId: primary.internalId,
    primaryTraitName: primary.displayName,
    primaryLevel: constructPrimaryLevel.value,
    secondaryTraitId: secondary?.internalId || '',
    secondaryTraitName: secondary?.displayName || '',
    secondaryLevel: secondary ? constructSecondaryLevel.value : 0,
    quantity: 1,
  }
  factorSlots.value = putConstructedFactor(factorSlots.value, activeFactorIndex.value, item, {
    name: sigil.displayName,
    level: constructSigilLevel.value,
    primaryTraitName: primary.displayName,
    primaryTraitId: primary.internalId,
    primaryTraitLevel: constructPrimaryLevel.value,
    secondaryTraitName: secondary?.displayName || '',
    secondaryTraitId: secondary?.internalId || '',
    secondaryTraitLevel: secondary ? constructSecondaryLevel.value : 0,
  })
}

function buildWriteRequest() {
  const factorPayload = buildFactorWritePayload(factorSlots.value)
  const w = { unitId: targetSlot.value, expectCharaHash: props.charaHash, op: op.value }
  if (op.value === 'write') {
    w.name = form.value.name
    w.weaponSlotId = form.value.weaponSlotId || 0
    w.sigilSlotIds = factorPayload.sigilSlotIds
    w.constructedSigils = factorPayload.constructedSigils
    if (writeGlobalSummons.value) w.summonSlotIds = [...summonSlotIds.value]
    w.skillHashes = [...form.value.skillHashes]
    if (masteryMode.value === 'free') {
      w.masteryHashes = [...selectedMasteryHashes.value]
    } else {
      const source = masterySources.value.find(item => item.unitId === form.value.masterySource)
      w.masteryHashes = source ? [...source.nodeHashes] : []
    }
  } else if (op.value === 'clone') {
    w.cloneFromUnitId = cloneFrom.value
  }
  return w
}

const writeInvalid = computed(() => {
  if (op.value === 'clear') return false
  if (op.value === 'clone') return !cloneFrom.value || cloneFrom.value === targetSlot.value
  return !form.value.name.trim()
    || nameTooLong.value
    || importMissing.value.length > 0
    || (writeGlobalSummons.value && !summonSelectionValid.value)
})

onBeforeUnmount(() => { simRequestId++; bagResizeObserver?.disconnect(); clearTimeout(simTimer); clearTimeout(masteryTimer) })

function opLabel() {
  return op.value === 'write' ? '写入' : op.value === 'clone' ? '克隆' : '清空'
}

function saveButtonLabel() {
  const slot = String(selectedSlot.value?.slot ?? 0).padStart(2, '0')
  if (applying.value) return '保存中…'
  if (op.value === 'clone') return `克隆配装到槽 ${slot}`
  if (op.value === 'clear') return `清空槽 ${slot}`
  return `保存配装到槽 ${slot}`
}

async function exportCurrentLoadout() {
  if (!selectedLoadout.value || selectedLoadout.value.isParty) return
  sharing.value = true
  try {
    const output = await LoadoutExport(props.savePath, selectedLoadout.value.unitId)
    if (output) emit('status', `已导出单套配装：${output}`, 'success')
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    sharing.value = false
  }
}

async function importLoadout() {
  sharing.value = true
  try {
    const draft = await LoadoutImport(props.savePath, props.charaHash)
    if (!draft) return
    importApplyPayload.value = draft.applyPayload || null
    form.value.name = draft.name || form.value.name
    form.value.weaponSlotId = draft.weaponSlotId || 0
    factorSlots.value = createFactorSlots(draft.sigilSlotIds || [])
    for (const constructed of draft.constructedSigils || []) {
      const item = constructed?.item || {}
      factorSlots.value = putConstructedFactor(factorSlots.value, Number(constructed.index), item, {
        name: item.sigilName || '导入因子',
        level: Number(item.level || 0),
        primaryTraitName: item.primaryTraitName || '',
        primaryTraitLevel: Number(item.primaryLevel || 0),
        secondaryTraitName: item.secondaryTraitName || '',
        secondaryTraitLevel: Number(item.secondaryLevel || 0),
      })
    }
    if (Array.isArray(draft.summonSlotIds) && draft.summonSlotIds.length === 4) {
      summonSlotIds.value = [...draft.summonSlotIds]
      const generated = new Set((draft.applyPayload?.constructedSummons || []).map(item => Number(item.index)))
      writeGlobalSummons.value = draft.summonSlotIds.every((slotId, index) => Number(slotId) > 0 || generated.has(index))
    }
    activeFactorIndex.value = 0
    form.value.skillHashes = [...(draft.skillHashes || [])]
    pendingSkillHash.value = ''
    masteryMode.value = 'free'
    setMasteryHashes(draft.masteryHashes || [])
    masteryRankTab.value = 'EX'
    importMissing.value = [...(draft.missing || [])]
    if (importMissing.value.length) {
      emit('status', `已载入草稿，但当前存档缺少 ${importMissing.value.length} 项不能自动生成的资源：${importMissing.value.join('；')}；为避免部分配装落盘，已禁止保存。补齐资源后请重新导入`, 'error')
    } else {
      const count = draft.constructedSigils?.length || 0
      const summonCount = draft.applyPayload?.constructedSummons?.length || 0
      emit('status', `单套配装已完整载入；保存时将生成 ${count} 枚独立因子${summonCount ? `、补建 ${summonCount} 个缺失召唤石` : ''}，并同步专精等级、角色强化及武器强化/祝福`, 'success')
    }
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    sharing.value = false
  }
}

async function apply() {
  if (!props.savePath || !targetSlot.value) return
  const w = buildWriteRequest()
  let preflight
  try {
    preflight = await LoadoutCheckCompliance(props.savePath, w)
  } catch (err) {
    emit('status', `写入预检失败：${String(err)}`, 'error')
    return
  }
  if (!preflight?.writable) {
    emit('status', `当前配装不可写：${preflight?.message || '合规预检未通过'}`, 'error')
    return
  }
  const slotNo = selectedSlot.value?.slot ?? '?'
  const occupied = selectedSlot.value?.occupied
  const draftCount = w.constructedSigils?.length || 0
  const weaponEdits = buildWeaponInlineEdits()
  const summonEdits = buildSummonInlineEdits()
  const ownedEditCount = weaponEdits.length + summonEdits.length
  const verb = op.value === 'clear' ? '清空' : (occupied ? '覆盖' : '写入')
  const detail = op.value === 'clear'
    ? `将清空【${props.charaName}·槽${String(slotNo).padStart(2, '0')}】的配装。`
    : `将${verb}【${props.charaName}·槽${String(slotNo).padStart(2, '0')}】的配装。${draftCount ? `其中 ${draftCount} 个构造草稿会在本次写入中自动生成并绑定到对应槽位。` : ''}${writeGlobalSummons.value ? '同时更新存档共用的全局四槽召唤石。' : ''}${ownedEditCount ? `本次还会编辑 ${ownedEditCount} 个背包实例；所有引用它的配装都会看到新值。配装、构造因子和实例改动在同一事务写入并回读。` : ''}`
  const confirmed = await confirmDialog.value?.ask({
    title: '写入存档前确认',
    detail,
    tone: 'warning',
    confirmLabel: '备份并写入',
  })
  if (!confirmed) return

  applying.value = true
  try {
    const res = await LoadoutApplyWithResources(props.savePath, '', {
      changes: [w],
      weaponEdits,
      summonEdits,
      importPayload: importApplyPayload.value,
    })
    emit('status', `已${opLabel()}并验证 ${res.verifiedFields} 项${res.createdCount ? `，生成 ${res.createdCount} 个独立因子` : ''}${res.createdSummonCount ? `，补建 ${res.createdSummonCount} 个召唤石` : ''}`, 'success')
    emit('reload')
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <div class="loadout-editor ui-page is-fluid">
    <div v-if="loading" class="hint ui-empty">正在读取该角色可用资源…</div>
    <template v-else-if="ctx">
      <div class="editor-layout">
      <aside class="editor-column setup-column">
      <section class="character-profile ui-card ui-panel is-compact">
        <div class="character-portrait">
          <img v-if="characterAvatar" :src="characterAvatar" :alt="charaName" />
          <span v-else>{{ (charaName || '?').slice(0, 1) }}</span>
          <b>Lv{{ statContext.level || 0 }}</b>
        </div>
        <div class="character-profile-main">
          <div class="ed-head">
            <strong>{{ charaName }}</strong>
            <span v-if="ctx.ownerCode" class="owner">{{ ctx.ownerCode }}</span>
            <span v-else class="owner warn">未能确定角色码</span>
          </div>
        </div>
        <div class="profile-stat-card">
          <div class="profile-stat-heading">
            <strong>人物属性</strong>
            <div class="profile-stat-source-tabs" role="tablist" aria-label="人物属性数据来源">
              <button type="button" role="tab" :aria-selected="statPanelMode === 'estimate'" :class="{ on: statPanelMode === 'estimate' }" @click="statPanelMode = 'estimate'">配装草稿估算</button>
              <button type="button" role="tab" :aria-selected="statPanelMode === 'runtime'" :class="{ on: statPanelMode === 'runtime' }" :disabled="!runtimePanelStats" @click="statPanelMode = 'runtime'">游戏真实回读</button>
            </div>
          </div>
          <div class="runtime-read-row">
            <small v-if="statPanelMode === 'runtime' && runtimePanelStats">2.0.2 游戏已计算对象 · 当前游戏内已应用配装</small>
            <small v-else>离线草稿近似值仅用于比较，写入游戏后可读取真实面板</small>
            <button type="button" :disabled="runtimePanelLoading" @click="readRuntimePanel(false)">{{ runtimePanelLoading ? '读取中…' : (runtimePanelStats ? '刷新游戏回读' : '从游戏读取') }}</button>
          </div>
          <small v-if="runtimePanelError" class="runtime-read-error" role="alert">{{ runtimePanelError }}</small>
          <dl class="profile-stats" aria-label="人物属性面板">
            <div class="profile-stat">
              <dt class="profile-stat-label">HP</dt>
              <dd class="profile-stat-value">{{ formatPanelStat('hp') }}</dd>
            </div>
            <div class="profile-stat">
              <dt class="profile-stat-label">攻击力</dt>
              <dd class="profile-stat-value">{{ formatPanelStat('attack') }}</dd>
            </div>
            <div class="profile-stat">
              <dt class="profile-stat-label">暴击率</dt>
              <dd class="profile-stat-value">{{ formatPanelStat('critRate', 'pct') }}</dd>
            </div>
            <div class="profile-stat">
              <dt class="profile-stat-label">昏厥值</dt>
              <dd class="profile-stat-value">{{ formatPanelStat('stunPower') }}</dd>
            </div>
            <div class="profile-stat">
              <dt class="profile-stat-label">防御力加成</dt>
              <dd class="profile-stat-value">{{ formatFinalStat(finalStats?.defenseBonus, 'signedPct') }}</dd>
              <small class="profile-stat-evidence">预计受击倍率 ≈{{ formatFinalStat(finalStats?.damageTakenRate, 'pct') }}</small>
            </div>
            <div class="profile-stat profile-stat-cap">
              <dt class="profile-stat-label">伤害上限</dt>
              <dd class="profile-stat-value">{{ formatPanelStat('damageCap', 'signedPct') }}</dd>
            </div>
          </dl>
          <div v-if="runtimeStatComparisons.length" class="runtime-comparison" :class="`relation-${runtimeComparisonRelation.kind}`" aria-label="配装草稿与游戏实读逐项对照">
            <div class="runtime-comparison-head"><b>{{ runtimeComparisonRelation.title }}</b><small>{{ runtimeComparisonRelation.detail }}</small></div>
            <div class="runtime-source-ledger">
              <span><small>草稿来源</small><b>{{ draftSourceSummary.weapon }} · 槽 {{ draftSourceSummary.weaponSlot }}</b><em>{{ draftSourceSummary.factors }} 因子 · {{ draftSourceSummary.mastery }} 专精 · {{ draftSourceSummary.summons }} 召唤石</em></span>
              <span><small>游戏当前</small><b>{{ runtimeWeaponPick?.name || runtimePanelStats.currentWeaponHash || '武器未识别' }} · 槽 {{ runtimePanelStats.currentWeaponSlotId || '未知' }}</b><em>{{ runtimeFactorCards.length }} 因子（{{ runtimePanelStats.currentFactorStableReads || 0 }} 次稳定） · 武炼结晶 {{ runtimePanelStats.currentWrightstoneHash || '未读取' }}</em></span>
            </div>
            <div v-for="row in runtimeStatComparisons" :key="row.field" class="runtime-comparison-row" :class="{ exact: runtimeComparisonRelation.comparable && Math.abs(row.delta) < 0.0001, unrelated: !runtimeComparisonRelation.comparable }">
              <b>{{ row.label }}</b>
              <span><small>草稿</small>{{ formatComparisonStat(row.estimate, row.unit) }}</span>
              <span><small>实读</small>{{ formatComparisonStat(row.runtime, row.unit) }}</span>
              <em>{{ runtimeComparisonRelation.comparable ? formatComparisonDelta(row.delta, row.unit) : `来源差 ${formatComparisonDelta(row.delta, row.unit)}` }}</em>
            </div>
          </div>
        </div>
        <p v-if="simulationError" class="simulation-error ui-notice is-danger" role="alert">{{ simulationError }}</p>
        <span class="source-badge">真实存档 · {{ selectedLoadout ? '槽' + String(selectedLoadout.slot).padStart(2, '0') : '新配装' }}</span>
        <details class="final-stat-detail-disclosure ui-disclosure">
          <summary>查看属性计算明细</summary>
          <p v-if="statContext.permanentGrowth?.runtimeObserved" class="runtime-growth-evidence">角色基础、命运篇章、角色强化固定成长：2.0.2 运行时状态对象，连续 {{ statContext.permanentGrowth?.stableReads }} 次稳定读取（角色独立）</p>
          <p v-else-if="!statContext.permanentGrowth?.masterSystemAvailable || !statContext.permanentGrowth?.legacySystemAvailable" class="runtime-growth-evidence is-candidate">该存档尚未建立完整 DLC 角色强化/专精字段；缺失层按未开启处理，不把结构容量或零值伪装成已解锁效果。</p>
          <div class="panel-base-grid">
            <span><small>角色基础 HP</small><b>{{ formatStatNumber(statContext.baseHp) }}</b></span>
            <span><small>角色基础攻击</small><b>{{ formatStatNumber(statContext.baseAtk) }}</b></span>
            <span><small>命运篇章 HP</small><b>{{ formatSignedValue(statContext.permanentGrowth?.fateHp) }}</b></span>
            <span><small>命运篇章攻击</small><b>{{ formatSignedValue(statContext.permanentGrowth?.fateAtk) }}</b></span>
            <span><small>角色强化 Lv{{ statContext.permanentGrowth?.masterLevel || 1 }} HP</small><b>{{ formatSignedValue(statContext.permanentGrowth?.masterHp) }}</b></span>
            <span><small>角色强化 Lv{{ statContext.permanentGrowth?.masterLevel || 1 }} 攻击</small><b>{{ formatSignedValue(statContext.permanentGrowth?.masterAtk) }}</b></span>
            <span class="baseline-total"><small>固定基准 HP</small><b>{{ formatStatNumber(statContext.baselineHp) }}</b></span>
            <span class="baseline-total"><small>固定基准攻击</small><b>{{ formatStatNumber(statContext.baselineAtk) }}</b></span>
            <span><small>固定基准暴击率</small><b>{{ formatFinalStat(statContext.baselineCritRate, 'pct') }}</b></span>
            <span><small>固定基准昏厥值</small><b>{{ formatFinalStat(statContext.baselineStun) }}</b></span>
            <span><small>角色强化伤害上限</small><b>{{ formatFinalStat(statContext.baselineDamageCap, 'signedPct') }}</b></span>
            <span v-if="selectedWeaponContext"><small>武器 HP</small><b>{{ formatFinalStat(selectedWeaponContext.total?.hp) }}</b></span>
            <span v-if="selectedWeaponContext"><small>武器攻击</small><b>{{ formatFinalStat(selectedWeaponContext.total?.attack) }}</b></span>
          </div>
          <div class="legacy-mastery-audit" aria-label="角色强化四页固定属性">
            <div class="legacy-mastery-audit-head">
              <b>角色强化四页</b>
              <small v-if="statContext.permanentGrowth?.legacyMastery?.runtimeObserved">2.0.2 运行时聚合 · 连续 {{ statContext.permanentGrowth?.legacyMastery?.stableReads }} 次稳定读取</small>
              <small v-else-if="statContext.permanentGrowth?.legacyMastery?.complete">2.0.2 解包表 + 存档完成进度</small>
              <small v-else>进度无法逐节点还原 · 不假算</small>
            </div>
            <div class="legacy-mastery-tabs">
              <article v-for="tab in [
                ['攻击强化', statContext.permanentGrowth?.legacyMastery?.attackTab],
                ['HP・抗性', statContext.permanentGrowth?.legacyMastery?.defenseTab],
                ['武器收集加成', statContext.permanentGrowth?.legacyMastery?.collectionTab],
                ['上限突破', statContext.permanentGrowth?.legacyMastery?.transcendenceTab],
              ]" :key="tab[0]">
                <strong>{{ tab[0] }}</strong>
                <span>HP <b>{{ formatSignedValue(tab[1]?.hp) }}</b></span>
                <span>攻击 <b>{{ formatSignedValue(tab[1]?.attack) }}</b></span>
                <span>暴击 <b>{{ formatSignedValue(tab[1]?.critRate, 'pct') }}</b></span>
                <span>昏厥 <b>{{ formatSignedValue(tab[1]?.stunPanel) }}</b></span>
                <small>昏厥原始 f32 {{ tab[1]?.stunRaw || 0 }} ×10 面板</small>
              </article>
            </div>
          </div>
          <div class="cap-detail-grid" aria-label="三类伤害上限明细">
            <span><small>普通伤害上限</small><b>{{ formatFinalStat(finalStats?.normalDamageCap, 'signedPct') }}</b></span>
            <span><small>能力伤害上限</small><b>{{ formatFinalStat(finalStats?.abilityDamageCap, 'signedPct') }}</b></span>
            <span><small>奥义伤害上限</small><b>{{ formatFinalStat(finalStats?.skyboundDamageCap, 'signedPct') }}</b></span>
          </div>
          <section v-if="finalStats?.defenseModel?.zones" class="defense-model" aria-label="防御分区计算">
            <header><b>防御分区</b><span>{{ finalStats.defenseModel.formula }} · 满血静态参考</span></header>
            <div class="defense-zone-grid">
              <article v-for="zone in finalStats.defenseModel.zones" :key="zone.key" :class="{ included: zone.included }">
                <b>{{ zone.label }}</b>
                <strong>{{ zone.included ? `−${formatFinalStat(zone.reduction, 'pct')}` : '未计入' }}</strong>
                <small>{{ zone.condition }}</small>
                <em>{{ defenseEvidenceLabel(zone.evidence) }}</em>
              </article>
            </div>
          </section>
          <p class="defense-scope-note"><b>配装防御加成</b>伊欧 +5% 实测将同一攻击从 36,938 降至 35,091，重复两次一致。当前满血参考按“同区相加，跨区相乘”展示；攻击 DOWN、战斗 Buff、坚守低血曲线、格挡和无敌没有当前状态时不强行计入。</p>
          <div class="formula-audit-row" :class="{ verified: calculationFormulaVerified }">
            <b>{{ calculationFormulaVerified ? '草稿公式证据已闭环' : '草稿公式未完全验证' }}</b>
            <span>带“≈”的离线值只用于草稿比较；只有游戏运行时回读不带近似标记。</span>
          </div>
          <div v-if="calculationWarnings.length" class="stat-warnings">
            <span v-for="warning in calculationWarnings" :key="warning">{{ warning }}</span>
          </div>
        </details>
        <details v-if="statContext.overLimit?.length" class="overlimit-disclosure ui-disclosure">
          <summary>极限加成来源（{{ statContext.overLimit.length }} 槽）</summary>
          <div class="overlimit-list">
            <span v-for="bonus in statContext.overLimit" :key="bonus.index">
              <b>{{ bonus.name }}</b><em>Lv{{ bonus.level }}</em><strong>{{ formatSignedValue(bonus.value, bonus.unit) }}</strong>
            </span>
          </div>
        </details>
      </section>

      <!-- 目标槽 -->
      <div class="ed-field">
        <label>目标槽位</label>
        <div class="slot-grid">
          <button v-for="s in slots" :key="s.unitId" class="slot-btn ui-btn is-sm" :class="{ on: targetSlot === s.unitId, occ: s.occupied }"
            @click="selectTarget(s.unitId)" :title="s.occupied ? s.name : '空槽'">
            {{ String(s.slot).padStart(2, '0') }}
          </button>
        </div>
        <small v-if="selectedSlot?.occupied" class="occ-warn">该槽已有配装「{{ selectedSlot.name || '(未命名)' }}」，写入将覆盖它</small>
      </div>

      <!-- 操作 -->
      <div class="ed-field">
        <label>操作</label>
        <div class="op-row ui-seg">
          <button class="op-btn ui-seg-btn" :class="{ 'is-on': op === 'write' }" @click="op = 'write'">自定义写入</button>
          <button class="op-btn ui-seg-btn" :class="{ 'is-on': op === 'clone' }" @click="op = 'clone'" :disabled="!occupiedSlots.length">克隆现有</button>
          <button class="op-btn ui-seg-btn" :class="{ 'is-on': op === 'clear' }" @click="op = 'clear'">清空</button>
        </div>
      </div>

      <!-- 克隆源 -->
      <div v-if="op === 'clone'" class="ed-field">
        <label>克隆来源</label>
        <select v-model.number="cloneFrom" class="ed-select ui-select">
          <option v-for="s in occupiedSlots" :key="s.unitId" :value="s.unitId" :disabled="s.unitId === targetSlot">
            槽{{ String(s.slot).padStart(2, '0') }} · {{ s.name || '(未命名)' }}
          </option>
        </select>
      </div>

      <!-- 自定义写入表单 -->
      <template v-if="op === 'write'">
        <div class="ed-field">
          <label>配装名称 <em :class="{ over: nameTooLong }">{{ nameBytes }}/63 字节</em></label>
          <input v-model="form.name" class="ed-input ui-input" maxlength="30" placeholder="给这套配装起个名字" />
        </div>

        <div class="ed-field">
          <label>武器（{{ ctx.weapons.length }} 可选）</label>
          <select v-model.number="form.weaponSlotId" class="ed-select ui-select">
            <option :value="0">— 不设置武器 —</option>
            <option v-for="w in ctx.weapons" :key="w.slotId" :value="w.slotId">
              {{ w.name }}{{ w.ownerCode ? '' : '（通用）' }}
            </option>
          </select>
          <div v-if="selectedWeaponContext" class="weapon-context-strip">
            <img v-if="selectedWeaponIcon" class="weapon-context-icon" :src="selectedWeaponIcon" alt="" />
            <span><b>{{ selectedWeaponContext.name }}</b><small>Lv{{ selectedWeaponContext.level }} · 觉醒 {{ selectedWeaponContext.awakening }} · 超凡 {{ selectedWeaponContext.transcendence }}</small></span>
            <em>HP {{ formatFinalStat(selectedWeaponContext.total?.hp) }} · 攻击 {{ formatFinalStat(selectedWeaponContext.total?.attack) }}</em>
          </div>
          <div v-if="weaponInlineAvailable" class="inline-resource-panel weapon-inline-panel">
            <label class="inline-resource-toggle ui-check">
              <input v-model="weaponInlineEnabled" type="checkbox" />
              <span><b>同时编辑该武器实例</b><small>候选来自该武器五个技能槽的解包表；固定槽保持只读</small></span>
            </label>
            <div v-if="weaponInlineEnabled" class="weapon-skill-edit-list">
              <label v-for="slot in editableWeaponSkillSlots" :key="slot.index" class="weapon-skill-edit-row">
                <span><b>技能槽 {{ slot.label }}</b><small>当前阶段 Lv{{ slot.currentLevel }}</small></span>
                <select v-model="weaponSkillDrafts[slot.index]" class="ed-select ui-select">
                  <option v-for="option in slot.options" :key="option.hash" :value="option.hash">{{ option.name }} · Lv{{ option.level }}</option>
                </select>
              </label>
            </div>
            <small>编辑的是背包中的武器与召唤石实例，会影响所有引用它们的配装；武器写入前会核对完整五槽快照。</small>
          </div>
          <small v-else-if="selectedWeaponContext && Number(selectedWeaponContext.transcendence) > 0" class="hint">当前阶段的武器技能均为该武器固定项，没有可切换槽。</small>
        </div>

        <div class="ed-field summon-field">
          <label>全局已装备召唤石（独立于单套配装）</label>
          <p v-if="!statContext.summonSystemAvailable" class="summon-system-unavailable ui-notice is-info">该存档尚未进入或初始化召唤石系统。预览按无召唤石效果继续，空槽不会报错；进入对应 DLC 并由游戏建立数据后再开放编辑。</p>
          <template v-else>
          <label class="summon-write-toggle ui-check">
            <input v-model="writeGlobalSummons" type="checkbox" />
            <span>
              <b>写入时同步更新全局四槽</b>
              <small>关闭时仅参与右侧数值预览；背包可选 {{ statContext.summons.length }} 个</small>
            </span>
          </label>
          <label class="summon-write-toggle ui-check" :class="{ disabled: !summonSelectionValid || !!importApplyPayload }">
            <input v-model="summonInlineEnabled" type="checkbox" :disabled="!summonSelectionValid || !!importApplyPayload" />
            <span>
              <b>同时编辑当前四个召唤石实例</b>
              <small>启用后会一并选定并写入这四个全局槽；实例属性与配装在同一事务回读验证</small>
            </span>
          </label>
          <small v-if="summonInlineEnabled" class="ui-hint">召唤石天然词池与等级只作提醒；所选可编码值不会被拦截。</small>
          <div class="summon-slot-list">
            <article v-for="index in 4" :key="index" class="summon-slot-card">
              <span class="summon-slot-index">{{ String(index).padStart(2, '0') }}</span>
              <select v-model.number="summonSlotIds[index - 1]" class="ed-select ui-select">
                <option :value="0" disabled>— 选择召唤石 —</option>
                <option v-if="importedSummonsByIndex.has(index - 1)" :value="0">{{ importedSummonsByIndex.get(index - 1).name || '导入召唤石' }} · 保存时自动生成</option>
                <option v-for="summon in statContext.summons" :key="summon.slotId" :value="summon.slotId"
                  :disabled="summonUsedElsewhere(summon.slotId, index - 1)">
                  {{ summonOptionLabel(summon) }}
                </option>
              </select>
              <img v-if="selectedSummons[index - 1] && summonAssetIcon(selectedSummons[index - 1])" class="summon-icon" :src="summonAssetIcon(selectedSummons[index - 1])" alt="" />
              <span v-if="selectedSummons[index - 1]" class="summon-source-lines">
                <b>{{ selectedSummons[index - 1].name }}</b>
                <small><i>主</i>{{ selectedSummons[index - 1].mainTraitName }} Lv{{ selectedSummons[index - 1].mainTraitLevel }}</small>
                <small><i>副</i>{{ selectedSummons[index - 1].subParamName }} {{ formatSignedValue(selectedSummons[index - 1].subParamValue, selectedSummons[index - 1].subParamUnit) }}</small>
              </span>
              <div v-if="summonInlineEnabled && selectedSummons[index - 1] && summonDrafts[selectedSummons[index - 1].slotId]" class="summon-inline-grid">
                <label class="summon-inline-wide"><span>主加护</span>
                  <select v-model="summonDrafts[selectedSummons[index - 1].slotId].mainTraitHash" class="ed-select ui-select"
                    @change="clampSummonDraft(summonDrafts[selectedSummons[index - 1].slotId])">
                    <option v-if="!summonMainOption(selectedSummons[index - 1].mainTraitHash)" :value="hashHex(selectedSummons[index - 1].mainTraitHash)">未收录主加护（仅原样保留）</option>
                    <option v-for="option in summonCatalog.traits" :key="option.hash" :value="hashHex(option.hash)">{{ option.name }}</option>
                  </select>
                </label>
                <label><span>主等级</span><input v-model.number="summonDrafts[selectedSummons[index - 1].slotId].mainTraitLevel" class="ed-input ui-input" type="number" min="0"
                  max="4294967295" @change="clampSummonDraft(summonDrafts[selectedSummons[index - 1].slotId])" /></label>
                <label class="summon-inline-wide"><span>副参数</span>
                  <select v-model="summonDrafts[selectedSummons[index - 1].slotId].subParamHash" class="ed-select ui-select" @change="clampSummonDraft(summonDrafts[selectedSummons[index - 1].slotId])">
                    <option v-for="option in summonCatalog.subParams" :key="option.hash" :value="hashHex(option.hash)">{{ option.name }}</option>
                  </select>
                </label>
                <label><span>副等级</span><input v-model.number="summonDrafts[selectedSummons[index - 1].slotId].subParamLevel" class="ed-input ui-input" type="number" min="0"
                  max="4294967295" @change="clampSummonDraft(summonDrafts[selectedSummons[index - 1].slotId])" /></label>
                <label title="独立存档字段 1460，不是召唤石稀有度；默认保留原值"><span>原始状态</span><input v-model.number="summonDrafts[selectedSummons[index - 1].slotId].rank" class="ed-input ui-input" type="number" min="0" max="4294967295" @change="clampSummonDraft(summonDrafts[selectedSummons[index - 1].slotId])" /></label>
              </div>
            </article>
          </div>
          <small v-if="writeGlobalSummons && !summonSelectionValid" class="summon-invalid">要同步全局四槽，请选择四个不重复、确实存在于当前存档背包中的召唤石。</small>
          <small v-else class="hint">四个召唤石始终参与右侧来源汇总；只有开启上方选项时才会改动存档的全局装备槽。</small>
          </template>
        </div>

        <div class="ed-field skill-field">
          <label>技能顺序（{{ form.skillHashes.length }}/4 · 完整技能池 {{ ctx.skills.length }}）</label>
          <div class="skill-grid ui-card-grid">
            <button v-for="s in ctx.skills" :key="s.hash" class="skill-pick"
              :class="{ on: form.skillHashes.includes(s.hash), pending: pendingSkillHash === s.hash }"
              @click="toggleSkill(s.hash)" :title="form.skillHashes.includes(s.hash) ? `当前第 ${skillOrder(s.hash)} 个技能` : s.name">
              <img class="skill-icon" :src="skillIcon(s)" alt="" />
              <span>{{ s.name || '未收录技能' }}</span>
              <b v-if="skillOrder(s.hash)" class="skill-order">{{ skillOrder(s.hash) }}</b>
            </button>
            <span v-if="!ctx.skills.length" class="empty">解包技能表中没有该角色的可选技能。</span>
          </div>
          <div v-if="pendingSkillHash" class="replace-strip">
            <span>4 个技能已满，替换哪一位？</span>
            <button v-for="(hash, index) in form.skillHashes" :key="hash" @click="replaceSkillAt(index)">
              <b>{{ index + 1 }}</b>{{ ctx.skills.find(s => s.hash === hash)?.name || '未收录技能' }}
            </button>
            <button class="replace-cancel" @click="pendingSkillHash = ''">取消</button>
          </div>
        </div>
      </template>
      </aside>

      <main class="editor-column build-column">
      <div class="editor-save-bar">
        <span><b>{{ op === 'write' ? '配装草稿' : op === 'clone' ? '克隆操作' : '清空操作' }}</b><small>目标槽 {{ String(selectedSlot?.slot ?? 0).padStart(2, '0') }}</small></span>
        <div class="editor-persistent-actions" aria-label="配装保存与单套导入导出">
          <small class="single-loadout-label">单套配装</small>
          <button class="ui-btn is-ghost single-loadout-action" :disabled="sharing || !selectedLoadout || selectedLoadout.isParty" title="导出当前单套配装，不包含存档" @click="exportCurrentLoadout">导出单套</button>
          <button class="ui-btn is-ghost single-loadout-action" :disabled="sharing" title="导入单套配装并载入为草稿" @click="importLoadout">导入单套</button>
          <button class="editor-save-button apply-btn ui-btn is-primary" :disabled="applying || writeInvalid" @click="apply">
            {{ saveButtonLabel() }}
          </button>
        </div>
      </div>
      <p class="single-loadout-scope">单套文件会复制武器及其强化、觉醒/超凡与祝福，12 个独立因子、4 个技能、专精选择与等级、角色强化进度及全局召唤石；目标存档缺少的召唤石会自动补建。系统开放状态和角色上限突破保持目标存档原值，缺少对应武器时不做部分写入。</p>
      <p v-if="op === 'write' && importMissing.length" class="import-blocker" role="alert">导入草稿缺少资源，为避免只写入部分配装，保存已锁定：{{ importMissing.join('；') }}。补齐后请重新导入。</p>
      <template v-if="op === 'write'">

        <div class="ed-field factor-field">
          <label>因子配置（{{ configuredFactorCount }}/12 · 背包 {{ ctx.sigils.length }}）</label>

          <div class="factor-slot-grid ui-card-grid" aria-label="当前配装的十二个因子槽">
            <button v-for="card in factorSlotCards" :key="card.index" class="factor-slot-card"
              :class="{ active: activeFactorIndex === card.index, empty: card.empty, draft: card.kind === 'construct' }"
              @click="selectFactorSlot(card.index)">
              <i class="factor-slot-number">{{ String(card.index + 1).padStart(2, '0') }}</i>
              <template v-if="card.empty">
                <span class="empty-factor-mark">＋</span><strong>空槽</strong><small>点此配置</small>
              </template>
              <template v-else>
                <span class="sigil-icon-frame">
                  <img v-if="traitIcon(card.primaryTraitName, card.primaryTraitHash, card.primaryTraitId)" :src="traitIcon(card.primaryTraitName, card.primaryTraitHash, card.primaryTraitId)" alt="" />
                  <i v-else>◆</i>
                </span>
                <span class="factor-slot-copy">
                  <b>{{ card.name }}</b>
                  <small v-if="card.primaryTraitName" class="trait-line"><i>主</i><span>{{ card.primaryTraitName }}</span><em>Lv{{ card.primaryTraitLevel }}</em></small>
                  <small v-if="card.secondaryTraitName" class="trait-line"><i>副</i><span>{{ card.secondaryTraitName }}</span><em>Lv{{ card.secondaryTraitLevel }}</em></small>
                </span>
                <em class="factor-source">{{ card.kind === 'construct' ? '待生成' : `背包 #${card.slotId}` }}</em>
              </template>
            </button>
          </div>

          <div class="factor-selection-bar">
            <span><b>当前槽 {{ String(activeFactorIndex + 1).padStart(2, '0') }}</b>{{ activeFactorCard.empty ? '空槽' : ` · ${activeFactorCard.name}` }}</span>
            <button v-if="!activeFactorCard.empty" @click="clearActiveFactor">清空此槽</button>
          </div>

          <div class="factor-mode-tabs ui-seg" role="tablist" aria-label="因子替换来源">
            <button class="ui-seg-btn" role="tab" :aria-selected="factorMode === 'construct'" :class="{ 'is-on': factorMode === 'construct' }" @click="factorMode = 'construct'">
              <b>构造模式</b><span>写入配装时自动生成</span>
            </button>
            <button class="ui-seg-btn" role="tab" :aria-selected="factorMode === 'bag'" :class="{ 'is-on': factorMode === 'bag' }" @click="factorMode = 'bag'">
              <b>从背包选择</b><span>替换当前选中槽</span>
            </button>
          </div>

          <div v-if="factorMode === 'bag'" class="bag-picker-shell">
            <div class="bag-toolbar ui-toolbar">
              <strong>替换槽 {{ String(activeFactorIndex + 1).padStart(2, '0') }}</strong>
              <input v-model="sigilSearch" class="ui-input" placeholder="搜索因子、主词条或副词条" />
              <span>{{ filteredSigils.length }} 件</span>
            </div>
            <div class="bag-filter-row" aria-label="背包因子筛选和排序">
              <select v-model="bagStateFilter" class="ui-select">
                <option value="all">全部因子</option>
                <option value="unused">未装入当前草稿</option>
                <option value="used">已装入当前草稿</option>
                <option value="dual">仅双词条</option>
                <option value="single">仅单词条</option>
              </select>
              <select v-model="bagTraitFilter" class="ui-select">
                <option value="">全部词条</option>
                <option v-for="name in bagTraitOptions" :key="name" :value="name">{{ name }}</option>
              </select>
              <select v-model="bagSort" class="ui-select">
                <option value="slot-asc">背包槽位从小到大</option>
                <option value="slot-desc">背包槽位从大到小</option>
                <option value="name">因子名称</option>
                <option value="primary-desc">主词条等级从高到低</option>
                <option value="secondary-desc">副词条等级从高到低</option>
              </select>
            </div>
            <div ref="bagViewport" class="bag-virtual-viewport ui-scroll-region" aria-label="背包因子" @scroll="onBagScroll">
              <div class="bag-virtual-spacer" :style="bagSpacerStyle">
                <div class="pick-grid sigils bag-virtual-window" :style="bagWindowStyle">
                  <button v-for="(s, visibleIndex) in visibleSigils" :key="s.slotId" class="pick sigil-pick"
                    :data-virtual-index="bagWindow.startIndex + visibleIndex"
                    :class="{ on: factorSlots[activeFactorIndex]?.kind === 'bag' && factorSlots[activeFactorIndex]?.slotId === s.slotId, used: bagFactorSlotNumber(s.slotId) }"
                    @click="chooseBagFactor(s.slotId)" :title="`${s.name}｜主：${s.primaryTraitName} Lv${s.primaryTraitLevel}${s.secondaryTraitName ? `｜副：${s.secondaryTraitName} Lv${s.secondaryTraitLevel}` : ''}`">
                    <span class="sigil-icon-frame">
                      <img v-if="traitIcon(s.primaryTraitName, s.primaryTraitHash)" :src="traitIcon(s.primaryTraitName, s.primaryTraitHash)" alt="" />
                      <i v-else>{{ s.generic ? '◇' : '◆' }}</i>
                    </span>
                    <span class="sigil-copy">
                      <b>{{ s.name }}</b>
                      <small v-if="s.primaryTraitName" class="trait-line"><i>主</i><span>{{ s.primaryTraitName }}</span><em>Lv{{ s.primaryTraitLevel }}</em></small>
                      <small v-if="s.secondaryTraitName" class="trait-line"><i>副</i><span>{{ s.secondaryTraitName }}</span><em>Lv{{ s.secondaryTraitLevel }}</em></small>
                    </span>
                    <span class="bag-factor-meta"><b v-if="bagFactorSlotNumber(s.slotId)">槽 {{ String(bagFactorSlotNumber(s.slotId)).padStart(2, '0') }}</b><i v-if="s.level">Lv{{ s.level }}</i></span>
                  </button>
                </div>
              </div>
            </div>
            <div v-if="!filteredSigils.length" class="catalog-empty ui-empty">没有符合当前筛选条件的背包因子。</div>
          </div>

          <div v-else class="constructor-shell">
            <small class="ui-hint">天然因子组合与等级只作提醒；所选可编码值不会被拦截。</small>
            <div class="constructor-note">
              <span class="constructor-mark">{{ String(activeFactorIndex + 1).padStart(2, '0') }}</span>
              <div><b>因子构造器</b><small>配置会先留在当前槽；点击整套写入时再生成真实因子并绑定。</small></div>
            </div>
            <div class="constructor-grid">
              <label class="constructor-search ui-field"><span class="ui-field-label">搜索目录</span><input v-model="constructSearch" class="ui-input" placeholder="按因子名或主词条搜索" /></label>
              <label class="constructor-wide"><span>因子</span>
                <select v-model="constructSigilId" class="ui-select" :disabled="constructLoading">
                  <option v-for="item in filteredConstructCatalog" :key="item.internalId" :value="item.internalId">{{ item.displayName }} · {{ item.primaryTraitName }}</option>
                </select>
                <small v-if="constructSearch && !filteredConstructCatalog.length" class="catalog-empty">构造目录无匹配结果。</small>
              </label>
              <label><span>因子等级</span>
                <input v-model.number="constructSigilLevel" type="number" min="0" :max="constructLevelLimit(highestAllowed(selectedConstructSigil?.allowedSigilLevels, 15))" class="ui-input" @change="constructSigilLevel = clampConstructLevel(constructSigilLevel, constructLevelLimit(highestAllowed(selectedConstructSigil?.allowedSigilLevels, 15)))" />
              </label>
              <label class="constructor-wide"><span>主词条</span>
                <CatalogSelect v-model="constructPrimaryId" :options="constructTraits" :icon-resolver="traitIconForOption" placeholder="选择主词条" search-placeholder="搜索全部词条" />
              </label>
              <label><span>主词条等级</span>
                <input v-model.number="constructPrimaryLevel" type="number" min="0" :max="constructLevelLimit(selectedConstructSigil?.firstTraitMaxLevel || 15)" class="ui-input" @change="constructPrimaryLevel = clampConstructLevel(constructPrimaryLevel, constructLevelLimit(selectedConstructSigil?.firstTraitMaxLevel || 15))" />
              </label>
              <label class="constructor-wide"><span>副词条 · 全部已知词条</span>
                <CatalogSelect v-model="constructSecondaryId" :options="constructSecondaryOptions" :icon-resolver="traitIconForOption" optional placeholder="不设置副词条" search-placeholder="搜索副词条" @pick="onConstructSecondaryPick" />
              </label>
              <label v-if="selectedConstructSecondary"><span>副词条等级</span><input v-model.number="constructSecondaryLevel" type="number" min="0" :max="constructLevelLimit(constructTraitWritableMax(selectedConstructSecondary))" class="ui-input" @change="constructSecondaryLevel = clampConstructLevel(constructSecondaryLevel, constructLevelLimit(constructTraitWritableMax(selectedConstructSecondary)))" /></label>
            </div>
            <div v-if="selectedConstructSigil" class="constructor-preview">
              <span class="sigil-icon-frame large"><img v-if="traitIcon(selectedConstructPrimary?.displayName, selectedConstructPrimary?.hash, selectedConstructPrimary?.internalId)" :src="traitIcon(selectedConstructPrimary?.displayName, selectedConstructPrimary?.hash, selectedConstructPrimary?.internalId)" alt="" /><i v-else>◆</i></span>
              <div><b>{{ selectedConstructSigil.displayName }}</b><span>主 · {{ selectedConstructPrimary?.displayName || '未设置' }} Lv{{ constructPrimaryLevel }}</span><span v-if="selectedConstructSecondary">副 · {{ selectedConstructSecondary.displayName }} Lv{{ constructSecondaryLevel }}</span></div>
              <button class="ui-btn is-primary" :disabled="!constructSigilId || !constructPrimaryId" @click="stageConstructedFactor">替换槽 {{ String(activeFactorIndex + 1).padStart(2, '0') }}</button>
            </div>
          </div>
        </div>

        <div class="ed-field mastery-field">
          <button class="mastery-toggle" :aria-expanded="masteryExpanded" @click="masteryExpanded = !masteryExpanded">
            <span><b>专精配置</b><small>位于因子配置下方 · 三方向 1–3 阶 + EX</small></span>
            <em>草稿 {{ selectedMasteryHashes.length }}/{{ masteryCapacity }} · 解锁内估算 {{ effectiveMasteryHashes.length }}/{{ masteryUnlockedCapacity }}</em>
            <i>{{ masteryExpanded ? '收起' : '展开' }}</i>
          </button>
          <div class="mastery-direction-map" aria-label="三个专精方向与阶段效果">
            <article v-for="direction in masteryDirectionCards" :key="direction.cat" :class="['direction-card', 'cat-' + catAbbr(direction.cat), { 'is-primary-direction': displayedMasteryDirection === direction.cat }]">
              <header><span class="cat-mark">{{ catAbbr(direction.cat) }}</span><div><small>{{ direction.label }}</small><b>{{ direction.specialization }}</b></div><em v-if="displayedMasteryDirection === direction.cat">当前主方向</em></header>
              <div v-for="row in direction.rows" :key="row.rank" class="direction-stage" :class="{ active: row.active }">
                <b>{{ row.label }}</b><span>{{ row.count }}/{{ row.threshold }}</span><small>{{ row.active ? '专精效果生效' : row.reason }}</small>
                <p v-if="row.effect" class="direction-effect">{{ row.effect }}</p>
              </div>
            </article>
          </div>

          <small v-if="masteryUnlockAmbiguous" class="hint mastery-unlock-warning">存档总 MSP 为 0：现有字段无法区分“专精系统尚未开放”和“已开放但尚未获得 MSP”，离线属性按 0 个专精节点保守估算。</small>
          <small v-if="masteryDraftOverflow" class="hint mastery-unlock-warning">越过当前角色强化解锁范围的节点仍保留在可写草稿中；离线属性暂按各阶存档顺序截取到当前容量，非法超量在游戏内的实际生效顺序待实机验证。</small>

          <div v-if="masteryExpanded" class="mastery-panel">
          <div class="op-row ui-seg">
            <button class="op-btn ui-seg-btn" :class="{ 'is-on': masteryMode === 'copy' }" @click="masteryMode = 'copy'">复制现有</button>
            <button class="op-btn ui-seg-btn" :class="{ 'is-on': masteryMode === 'free' }" @click="masteryMode = 'free'" :disabled="!ctx.ownerCode">自由配置</button>
          </div>

          <template v-if="masteryMode === 'copy'">
            <select v-model.number="form.masterySource" class="ed-select ui-select">
              <option :value="0">— 不设置专精 —</option>
              <option v-for="m in masterySources" :key="m.unitId" :value="m.unitId">
                复制自 槽{{ String(m.slot).padStart(2, '0') }}「{{ m.name || '未命名' }}」（{{ m.nodeCount }} 节点）
              </option>
            </select>
            <small class="hint">整套复制自该角色已有配装；若来源超过目标角色当前解锁容量，会保留草稿并明确提示。</small>
          </template>

          <template v-else>
            <div class="mastery-auto-direction">
              <strong>主方向由已点节点自动推导</strong>
              <small>{{ masteryDirection ? `草稿推导：${masteryDirectionCards.find(item => item.cat === masteryDirection)?.label || masteryDirection}` : '继续配置2阶节点；未形成或存在冲突时只提示，不会删除选择。' }}</small>
            </div>
            <div class="rank-tabs">
              <button v-for="p in masteryPool" :key="p.rank" class="rank-tab" :class="{ on: masteryRankTab === p.rank }" :disabled="masteryStructuralRankCap(p.rank) === 0" @click="masteryRankTab = p.rank">
                {{ p.label }}<i :class="{ full: rankPicked(p.rank) >= masteryStructuralRankCap(p.rank) }">草稿 {{ rankPicked(p.rank) }}/{{ masteryStructuralRankCap(p.rank) }} · 解锁容量 {{ masteryUnlockedRankCap(p.rank) }}</i>
              </button>
            </div>
            <div v-if="['R2', 'R3'].includes(masteryRankTab)" class="mastery-rule direction-rule">
              <b>方向与激活状态实时计算</b>
              <span>可自由选择任意已收录节点；偏离游戏通常的2/3阶方向规则时只给提示，原选择保持不变。</span>
            </div>
            <div v-if="masteryRankTab === 'R1'" class="mastery-rule">
              1阶三个方向互不排斥：每组达到 3 项即可激活该方向的1阶专精技能。
            </div>
            <div v-else-if="masteryRankTab === 'EX'" class="mastery-rule ex-rule">
              <b>官方名称：EX阶专精技能</b><span>MLv30→50 解锁 · 本阶仍按真谛、觉醒、秘义整理，不是第四种专精类型。</span>
            </div>
            <div v-if="activeRankPool" class="mastery-groups">
              <section v-for="grp in activeRankGroups" :key="grp.cat" class="mastery-group" :class="'cat-' + catAbbr(grp.cat)">
                <header>
                  <span class="cat-mark">{{ catAbbr(grp.cat) }}</span>
                  <strong>{{ grp.label }}</strong>
                  <em>{{ grp.nodes.filter(n => (masteryPick[activeRankPool.rank] || []).includes(n.hash)).length }}/{{ grp.nodes.length }}</em>
                  <i v-if="['R2', 'R3'].includes(activeRankPool.rank) && masteryDirection === grp.cat">主方向</i>
                </header>
                <div class="node-list">
                  <button v-for="n in grp.nodes" :key="n.hash" class="node"
                    :class="{ on: (masteryPick[activeRankPool.rank] || []).includes(n.hash), named: n.name || n.specialization, disabled: masteryNodeDisabled(activeRankPool.rank, n) }"
                    :disabled="masteryNodeDisabled(activeRankPool.rank, n)"
                    @click="toggleNode(activeRankPool.rank, n.hash, masteryStructuralRankCap(activeRankPool.rank))">
                    <span class="node-check">{{ (masteryPick[activeRankPool.rank] || []).includes(n.hash) ? '✓' : '' }}</span>
                    <span><b v-if="n.name || n.specialization">{{ masteryNodeTitle(activeRankPool.rank, n) }}</b><small>{{ n.desc }}</small></span>
                    <em v-if="n.name || n.specialization">专精技能</em>
                  </button>
                </div>
              </section>
            </div>
            <small class="hint">存档结构容量 {{ masteryStructuralRankCap('R1') }} / {{ masteryStructuralRankCap('R2') }} / {{ masteryStructuralRankCap('R3') }} / {{ masteryStructuralRankCap('EX') }}；角色强化 Lv{{ statContext.permanentGrowth?.masterLevel || 1 }} HP/攻击等固定成长已从存档读取。</small>
          </template>
          </div>
        </div>
      </template>

      </main>

      <aside class="editor-column result-sidebar">
        <section class="result-card result-overview ui-card ui-panel is-compact">
          <header><strong>角色效果总计</strong><span>随当前槽实时更新</span></header>
          <div class="result-metrics">
            <div><b>{{ configuredFactorCount }}</b><span>/12 因子</span></div>
            <div><b>{{ form.skillHashes.length }}</b><span>/4 技能</span></div>
            <div><b>{{ effectiveMasteryHashes.length }}</b><span>/{{ masteryUnlockedCapacity }} 解锁内估算</span></div>
            <div><b>{{ selectedSummons.filter(Boolean).length }}</b><span>/4 召唤</span></div>
          </div>
        </section>

        <section class="result-card skill-summary-card ui-card ui-panel is-compact">
          <header><strong>技能效果</strong><span>{{ selectedSkills.length }} 主动 · {{ weaponSkills.length }} 武器 · {{ selectedMasteryDetails.length }} 专精</span></header>
          <div class="skill-chips">
            <span v-for="(skill, index) in selectedSkills" :key="skill.hash"><b>{{ index + 1 }}</b><img :src="skillIcon(skill)" alt="" />{{ skill.name || '未收录技能' }}</span>
            <i v-if="!selectedSkills.length">未配置主动技能</i>
          </div>
          <div class="weapon-skill-block">
            <div class="weapon-skill-title"><b>武器技能</b><span>{{ selectedWeaponContext?.name || '未选择武器' }}</span></div>
            <div v-if="selectedWeaponContext?.wrightstone" class="wrightstone-audit" aria-label="武炼结晶有效词条">
              <div class="wrightstone-audit-head">
                <span><b>武炼结晶 · {{ selectedWeaponContext.wrightstone.name || '未收录名称' }}</b><small>{{ selectedWeaponContext.wrightstone.hash }}</small></span>
                <em v-if="selectedWeaponContext.wrightstone.runtimeObserved">游戏运行时 · {{ selectedWeaponContext.wrightstone.stableReads }} 次稳定读取</em>
                <em v-else>存档候选 · 写入或重载后需游戏复核</em>
              </div>
              <article v-for="trait in selectedWeaponContext.wrightstone.traits" :key="`${trait.index}-${trait.hash}`">
                <span><b>{{ trait.name || '未收录词条' }}</b><small>{{ trait.hash }}</small></span>
                <strong>结晶 Lv{{ trait.level }}</strong>
                <template v-if="mergedTraitBonus(trait)">
                  <em>全来源合并 Lv{{ mergedTraitBonus(trait).rawLevel }}</em>
                  <p>{{ mergedTraitBonus(trait).effect }}</p>
                </template>
              </article>
            </div>
            <small v-else-if="selectedWeaponContext" class="hint">当前武器没有可验证的武炼结晶；不会凭武器类型补假数据。</small>
            <div v-if="weaponSkills.length" class="weapon-skill-list">
              <article v-for="skill in weaponSkills" :key="`${skill.slot}-${skill.traitHash}`">
                <span><b>{{ skill.name || '未收录武器技能' }}</b><em>{{ formatWeaponSkillLevel(skill) }}</em></span>
                <p class="weapon-skill-effect">{{ skill.effect || '暂无可验证效果说明' }}</p>
                <small v-if="skill.runtimeObserved">有效等级 · 游戏运行时 {{ skill.stableReads }} 次稳定读取；静态表 Lv{{ skill.staticLevel }}</small>
                <small v-if="skill.unlockCondition">解锁阶段 · {{ skill.unlockCondition }}</small>
                <small>来源 · {{ skill.sourceWeapon || '未收录武器' }}</small>
              </article>
            </div>
            <small v-else class="hint">当前武器没有可解析的武器技能。</small>
          </div>
        </section>

        <section class="result-card bonus-summary-card ui-card ui-panel is-compact">
          <header><strong>总计加成</strong><span>{{ simulating ? '计算中…' : displayTotals.length + ' 类' }}</span></header>
          <p class="calculation-scope-note">人物属性以存档中的角色基础值、命运篇章与角色强化为固定基准；加成明细默认只汇总可随时更换的武器（含武器技能）、因子、专精、角色上限突破与召唤石，不含任务、队伍、临时状态及战斗内条件加成。</p>
          <div v-if="bonuses.length" class="trait-level-ledger">
            <span><small>有效</small><b>{{ traitLevelSummary.effective }}</b></span>
            <span><small>投入</small><b>{{ traitLevelSummary.invested }}</b></span>
            <span v-if="traitLevelSummary.overflow > 0" class="overflow"><small>溢出</small><b>+{{ traitLevelSummary.overflow }}</b><em>{{ traitLevelSummary.cappedCount }} 项</em></span>
          </div>
          <div v-if="displayTotals.length" class="effect-total-list">
            <div v-for="total in displayTotals" :key="total.key" class="effect-total-row" :class="catClass(total.catLabel)">
              <span><b>{{ total.label }}</b><small>{{ total.sources.join(' · ') }}</small></span>
              <strong class="effect-total-values">
                <span v-for="part in total.parts" :key="part.unit">
                  <small v-if="total.parts.length > 1">{{ effectUnitLabel(part.unit) }}</small>
                  <b>{{ formatEffectTotal(part) }}</b>
                </span>
              </strong>
            </div>
          </div>
          <small v-else-if="!simulating" class="hint">当前配置没有可合并的数值加成。</small>
          <details v-if="bonuses.length" class="trait-detail-disclosure ui-disclosure">
            <summary>查看词条等级与完整效果（{{ bonuses.length }}）</summary>
            <div class="sim-list">
              <div v-for="b in bonuses" :key="b.traitId" class="sim-row" :class="catClass(b.catLabel)">
                <span class="sim-name">{{ b.name }}<i v-if="b.capped" class="sim-cap" title="已达词条上限">封顶</i></span>
                <span class="sim-lv">Lv{{ b.level }}<small v-if="b.rawLevel !== b.level">/{{ b.rawLevel }}</small></span>
                <span class="sim-eff">{{ b.effect }}</span>
              </div>
            </div>
          </details>
        </section>

        <section class="result-card mastery-summary-card ui-card ui-panel is-compact">
          <header><strong>专精解锁内估算</strong><span>{{ summarizingMastery ? '解析中…' : (masterySummary?.primaryLabel || '未形成2阶主方向') }}</span></header>
          <div v-if="masterySummary" class="mastery-result">
            <article v-for="rank in masterySummary.ranks" :key="rank.rank" class="mastery-result-rank">
              <div class="rank-line"><b>{{ rank.label }}</b><em>{{ rank.count }}/{{ masteryUnlockedRankCap(rank.rank) }}</em></div>
              <template v-if="rank.rank !== 'EX'">
                <div v-for="cat in rank.categories" :key="cat.cat" class="mastery-result-cat" :class="{ active: cat.active }">
                  <span>{{ cat.specialization || cat.label }}</span>
                  <b>{{ cat.count }}/{{ cat.threshold }}</b>
                  <i>{{ cat.active ? '生效' : cat.reason }}</i>
                  <p v-if="cat.effect">{{ cat.effect }}</p>
                </div>
              </template>
              <div v-else class="ex-result">
                EX阶专精技能 {{ rank.count }}/{{ masteryUnlockedRankCap(rank.rank) }} · 按三方向归类，不构成第四专精
              </div>
            </article>
          </div>
          <small v-else class="hint">当前没有可解析的专精节点。</small>
          <details v-if="selectedMasteryDetails.length" class="mastery-detail-disclosure ui-disclosure">
            <summary>查看已选专精子词条（{{ selectedMasteryDetails.length }}）</summary>
            <div class="mastery-effect-list">
              <div v-for="node in selectedMasteryDetails" :key="node.hash" class="mastery-effect">
                <span>{{ node.rankLabel }} · {{ node.catLabel }}</span>
                <b v-if="node.name">{{ node.name }}</b>
                <p>{{ node.desc }}</p>
                <small v-if="node.rawDesc">解包原始文本：{{ node.rawDesc }} · 显示尺度 ×{{ node.displayScale }} · {{ node.evidence }}</small>
              </div>
            </div>
          </details>
        </section>

      </aside>
      </div>
    </template>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.loadout-editor { width:100%; min-width:0; height:100%; min-height:0; --editor-scale:1; color:var(--text-secondary); font-family:var(--font-ui,"Microsoft YaHei UI","Microsoft YaHei",sans-serif); font-size:var(--fs-base); font-weight:var(--fw-normal); line-height:var(--lh-normal); container:loadout-editor / inline-size; }
.editor-layout { width:100%; height:100%; min-height:0; display:grid; grid-template-columns:clamp(250px,20vw,360px) minmax(540px,1fr) clamp(280px,22vw,400px); justify-content:stretch; gap:var(--space-4); align-items:stretch; }
.editor-column { min-width:0; min-height:0; overflow:auto; scrollbar-gutter:stable; padding:0; border:1px solid var(--line); border-radius:10px; background:rgba(255,253,247,.94); box-shadow:0 4px 14px rgba(61,47,29,.06); }
.setup-column, .build-column { display:flex; flex-direction:column; gap:0; }
.setup-column > * + *,
.build-column > * + * { border-top:1px solid var(--line-soft); }
.build-column > * { flex:0 0 auto; }
.hint { font-size:var(--fs-xs); color:var(--text-muted); }
.character-profile { display:grid; grid-template-columns:62px minmax(0,1fr); gap:9px 11px; padding:12px 14px; background:rgba(139,103,55,.035); }
.character-portrait { position:relative; width:62px; height:62px; display:grid; place-items:center; overflow:hidden; border:1px solid #c5ad7f; border-radius:9px; background:#f3e8d2; color:#765126; font-size:calc(23px * var(--editor-scale)); font-weight:700; }
.character-portrait img { width:100%; height:100%; object-fit:cover; }
.character-portrait > b { position:absolute; right:4px; bottom:4px; padding:1px 5px; border-radius:8px; background:rgba(47,39,27,.8); color:#fffdf7; font-size:var(--fs-xs); line-height:1.5; }
.character-profile-main { min-width:0; display:flex; flex-direction:column; justify-content:center; }
.profile-stat-card { grid-column:1 / -1; padding:8px; border:1px solid var(--line-soft); border-radius:8px; background:rgba(255,255,255,.5); }
.profile-stat-heading { min-width:0; display:flex; flex-wrap:wrap; justify-content:space-between; align-items:center; gap:5px 8px; margin-bottom:6px; padding:0 1px; }
.profile-stat-heading strong { color:var(--text-primary); font-size:calc(12px * var(--editor-scale)); font-weight:700; }
.profile-stat-source-tabs { min-width:0; display:flex; gap:3px; padding:2px; border:1px solid var(--line-soft); border-radius:7px; background:rgba(139,103,55,.06); }
.profile-stat-source-tabs button { min-height:26px; padding:2px 7px; border:0; border-radius:5px; background:transparent; color:var(--text-muted); font-size:calc(12px * var(--editor-scale)); cursor:pointer; }
.profile-stat-source-tabs button.on { background:#8b6737; color:#fff9e9; }
.profile-stat-source-tabs button:disabled { opacity:.42; cursor:not-allowed; }
.runtime-read-row { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:6px; align-items:center; margin-bottom:6px; padding:5px 6px; border-left:2px solid #b5925b; background:rgba(139,103,55,.045); }
.runtime-read-row small { min-width:0; color:var(--text-muted); font-size:calc(12px * var(--editor-scale)); line-height:1.35; white-space:normal; overflow-wrap:anywhere; }
.runtime-read-row button { min-height:26px; padding:2px 7px; border:1px solid var(--line-gold); border-radius:5px; background:rgba(255,255,255,.64); color:#765126; font-size:calc(12px * var(--editor-scale)); cursor:pointer; white-space:nowrap; }
.runtime-read-row button:disabled { opacity:.5; cursor:wait; }
.runtime-read-error { display:block; margin:-1px 0 6px; color:var(--red); font-size:calc(12px * var(--editor-scale)); line-height:1.4; overflow-wrap:anywhere; }
.profile-stats { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:5px; margin:0; }
.profile-stat { min-width:0; display:flex; flex-direction:column; padding:5px 7px; border-left:2px solid #b5925b; background:rgba(139,103,55,.06); }
.profile-stat:nth-child(2) { border-left-color:#88704e; }
.profile-stat:nth-child(3) { border-left-color:#a34b50; }
.profile-stat:nth-child(4) { border-left-color:#81704f; }
.profile-stat:nth-child(5) { border-left-color:#5f8067; }
.profile-stat-cap .profile-stat-value { color:#a23f65; }
.profile-stat-label { color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); }
.profile-stat-value { margin:0; white-space:nowrap; color:var(--text-primary); font-size:calc(15px * var(--editor-scale)); font-weight:700; font-variant-numeric:tabular-nums; }
.profile-stat-evidence { margin-top:1px; color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); line-height:1.3; }
.runtime-comparison { display:flex; flex-direction:column; gap:3px; margin-top:7px; padding-top:7px; border-top:1px dashed var(--line-soft); }
.runtime-comparison-head { display:flex; flex-wrap:wrap; justify-content:space-between; gap:3px 8px; color:var(--text-primary); font-size:calc(11px * var(--editor-scale)); }
.runtime-comparison-head small { color:var(--text-muted); font-weight:400; }
.runtime-source-ledger { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:5px; }
.runtime-source-ledger > span { min-width:0; display:flex; flex-direction:column; gap:1px; padding:5px 6px; border:1px solid var(--line-soft); border-radius:5px; background:rgba(255,255,255,.38); }
.runtime-source-ledger small,.runtime-source-ledger em { color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); font-style:normal; font-weight:400; overflow-wrap:anywhere; }
.runtime-source-ledger b { color:var(--text-primary); font-size:calc(11px * var(--editor-scale)); overflow-wrap:anywhere; }
.runtime-comparison-row { display:grid; grid-template-columns:minmax(48px,.8fr) repeat(2,minmax(62px,1fr)) minmax(64px,auto); gap:5px; align-items:center; padding:4px 6px; border-left:2px solid #b36a55; background:rgba(179,106,85,.06); font-size:calc(11px * var(--editor-scale)); font-variant-numeric:tabular-nums; }
.runtime-comparison-row.exact { border-left-color:#4f8061; background:rgba(79,128,97,.07); }
.runtime-comparison-row.unrelated { border-left-color:#9a7a46; background:rgba(154,122,70,.07); }
.runtime-comparison-row > span { display:flex; flex-direction:column; color:var(--text-primary); font-weight:600; }
.runtime-comparison-row small { color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); font-weight:400; }
.runtime-comparison-row em { justify-self:end; color:#9b4f42; font-style:normal; font-weight:700; white-space:nowrap; }
.runtime-comparison-row.exact em { color:#3f7653; }
.runtime-comparison-row.unrelated em { color:#765f35; }
.legacy-mastery-audit { margin-top:8px; padding-top:8px; border-top:1px solid var(--line-soft); }
.legacy-mastery-audit-head { display:flex; flex-wrap:wrap; justify-content:space-between; gap:5px 10px; margin-bottom:6px; }
.legacy-mastery-audit-head small { color:var(--text-muted); }
.legacy-mastery-tabs { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:6px; }
.legacy-mastery-tabs article { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:3px 7px; padding:7px; border:1px solid var(--line-soft); border-radius:7px; background:rgba(139,103,55,.045); }
.legacy-mastery-tabs article > strong, .legacy-mastery-tabs article > small { grid-column:1 / -1; }
.legacy-mastery-tabs article span { display:flex; justify-content:space-between; gap:5px; color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); }
.legacy-mastery-tabs article b { color:var(--text-primary); font-variant-numeric:tabular-nums; }
.legacy-mastery-tabs article small { color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); }
.character-profile > .source-badge,
.final-stat-detail-disclosure,
.overlimit-disclosure,
.stat-warnings { grid-column:1/-1; }
.final-stat-detail-disclosure { border-top:1px solid var(--line-soft); }
.final-stat-detail-disclosure summary { padding-top:5px; color:#765126; font-size:calc(12px * var(--editor-scale)); font-weight:600; cursor:pointer; }
.final-stat-detail-disclosure[open] summary { margin-bottom:6px; }
.panel-base-grid { display:grid; grid-template-columns:1fr 1fr; gap:4px; margin-top:6px; }
.panel-base-grid > span { min-width:0; display:flex; flex-direction:column; padding:5px 6px; border:1px solid var(--line-soft); border-radius:5px; background:rgba(255,255,255,.46); }
.panel-base-grid > .baseline-total { border-color:rgba(143,97,39,.34); background:rgba(220,188,122,.14); }
.panel-base-grid small { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-muted); font-size:var(--fs-xs); }
.panel-base-grid b { color:var(--text-primary); font-size:calc(12px * var(--editor-scale)); font-variant-numeric:tabular-nums; }
.cap-detail-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:4px; margin-top:4px; }
.cap-detail-grid > span { min-width:0; display:flex; flex-direction:column; justify-content:space-between; gap:2px; padding:5px 4px; border:1px solid rgba(162,63,101,.16); border-radius:5px; background:rgba(162,63,101,.045); text-align:center; }
.cap-detail-grid small { color:var(--text-muted); font-size:var(--fs-xs); line-height:1.3; }
.cap-detail-grid b { color:#a23f65; font-size:calc(11px * var(--editor-scale)); font-variant-numeric:tabular-nums; white-space:nowrap; }
.defense-scope-note { margin:5px 0 0; padding:5px 7px; border-left:2px solid #5f8067; background:rgba(95,128,103,.06); color:var(--text-muted); font-size:var(--fs-xs); line-height:1.45; }
.defense-scope-note b { margin-right:.4em; color:#466a51; font-weight:700; }
.defense-model { display:grid; gap:5px; margin-top:6px; }
.defense-model > header { display:flex; justify-content:space-between; gap:8px; align-items:baseline; color:#466a51; font-size:var(--fs-xs); }
.defense-model > header span { color:var(--text-muted); text-align:right; }
.defense-zone-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(128px,1fr)); gap:4px; }
.defense-zone-grid article { min-width:0; display:grid; grid-template-columns:1fr auto; gap:2px 6px; padding:5px 6px; border:1px solid var(--line-soft); border-radius:5px; background:var(--surface-soft); }
.defense-zone-grid article.included { border-color:rgba(70,106,81,.38); background:rgba(95,128,103,.07); }
.defense-zone-grid b,.defense-zone-grid strong { font-size:var(--fs-xs); }
.defense-zone-grid strong { color:#466a51; font-variant-numeric:tabular-nums; }
.defense-zone-grid small,.defense-zone-grid em { grid-column:1/-1; color:var(--text-muted); font-size:var(--fs-xs); line-height:1.3; overflow-wrap:anywhere; }
.defense-zone-grid em { color:#786a52; font-style:normal; }
.formula-audit-row { display:grid; gap:2px; margin-top:6px; padding:6px 8px; border:1px solid var(--warning); border-radius:var(--radius-sm); background:var(--warning-bg); color:var(--warning-ink); font-size:var(--fs-xs); line-height:1.4; }
.formula-audit-row.verified { border-color:var(--success); background:var(--success-bg); color:var(--success-ink); }
.formula-audit-row span { color:inherit; }
.overlimit-disclosure { padding-top:2px; border-top:1px solid var(--line-soft); }
.overlimit-disclosure summary { padding-top:5px; color:#765126; font-size:calc(12px * var(--editor-scale)); font-weight:600; cursor:pointer; }
.overlimit-list { display:flex; flex-direction:column; gap:3px; margin-top:6px; }
.overlimit-list > span { display:grid; grid-template-columns:minmax(0,1fr) auto auto; gap:6px; padding:3px 5px; border-radius:4px; background:rgba(139,103,55,.05); font-size:calc(11px * var(--editor-scale)); }
.overlimit-list b { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-secondary); font-weight:600; }
.overlimit-list em { color:var(--text-muted); font-style:normal; }
.overlimit-list strong { color:#765126; font-variant-numeric:tabular-nums; }
.stat-warnings { display:flex; flex-direction:column; gap:3px; color:var(--red); font-size:calc(11px * var(--editor-scale)); }
.ed-head { display:grid; grid-template-columns:minmax(0,1fr) auto; align-items:center; gap:7px 9px; }
.ed-head strong { min-width:0; overflow:hidden; text-overflow:ellipsis; font-size:.9rem; color:var(--text-primary); white-space:nowrap; }
.ed-head .owner { font-size:var(--fs-xs); color:var(--gold); padding:1px 7px; border:1px solid var(--line-soft); border-radius:10px; }
.ed-head .owner.warn { color:var(--red); }
.source-badge { grid-column:1/-1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:var(--fs-xs); font-weight:700; color:var(--text-muted); padding:3px 8px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel-solid); }
.ed-field { display:flex; flex-direction:column; gap:7px; padding:13px 14px; background:transparent; }
.ed-field > label { font-size:var(--fs-sm); font-weight:700; color:var(--text-secondary); }
.ed-field > label em { font-style:normal; font-weight:600; color:var(--text-muted); margin-left:6px; }
.ed-field > label em.over { color:var(--red); }
.slot-grid { display:grid; grid-template-columns:repeat(auto-fill, minmax(38px, 1fr)); gap:5px; }
.slot-btn { min-height:30px; border:1px solid var(--line-soft); border-radius:5px; background:var(--sky-900); color:var(--text-muted); font-size:var(--fs-xs); cursor:pointer; user-select:none; }
.slot-btn.occ { color:var(--text-primary); border-color:var(--line-gold); }
.slot-btn.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.occ-warn { font-size:var(--fs-xs); color:var(--amber); }
.op-row { display:flex; flex-wrap:wrap; gap:7px; align-items:center; }
.editor-save-bar { position:sticky; z-index:12; top:0; min-height:48px; display:flex; align-items:center; justify-content:space-between; gap:10px; padding:7px 12px; border-bottom:1px solid var(--line-gold); background:rgba(250,244,226,.97); box-shadow:0 3px 10px rgba(70,51,26,.09); backdrop-filter:blur(8px); }
.editor-save-bar > span { min-width:0; display:flex; flex-direction:column; }
.editor-save-bar > span b { color:var(--text-primary); font-size:var(--fs-sm); }
.editor-save-bar > span small { color:var(--text-muted); font-size:var(--fs-xs); }
.editor-persistent-actions { flex:0 0 auto; display:flex; align-items:center; gap:6px; }
.editor-persistent-actions .ui-btn { min-height:34px; }
.single-loadout-label { padding:0 3px 0 1px; color:var(--text-muted); font-size:var(--fs-xs); font-weight:700; white-space:nowrap; }
.single-loadout-action { border-color:var(--line-gold); background:rgba(255,255,255,.5); color:#765126; }
.single-loadout-scope { margin:0; padding:7px 12px; border-bottom:1px solid var(--line-soft); background:rgba(255,255,255,.38); color:var(--text-muted); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.import-blocker { position:sticky; z-index:11; top:48px; margin:0; padding:8px 12px; border-bottom:1px solid var(--danger); background:var(--danger-bg); color:var(--danger); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.editor-save-button { flex:0 0 auto; min-width:142px; }
.op-btn { min-height:30px; padding:0 13px; border:1px solid var(--line-gold); border-radius:6px; background:var(--sky-900); color:var(--text-primary); font-size:var(--fs-sm); cursor:pointer; user-select:none; }
.op-btn.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.op-btn:disabled { opacity:.4; cursor:not-allowed; }
.ed-input, .ed-select { min-height:32px; padding:0 10px; border:1px solid var(--line-gold); border-radius:6px; background:var(--panel-solid); color:var(--text-primary); font-size:var(--fs-sm); }
.ed-input:focus, .ed-select:focus { outline:2px solid rgba(154,116,64,.5); outline-offset:1px; }
.weapon-context-strip { min-width:0; display:grid; grid-template-columns:58px minmax(0,1fr); gap:4px 8px; align-items:center; padding:7px 9px; border:1px solid var(--line-soft); border-left:3px solid #8b6737; border-radius:6px; background:rgba(139,103,55,.055); }
.weapon-context-icon { width:58px; height:42px; object-fit:contain; border-radius:5px; background:rgba(255,255,255,.58); }
.weapon-context-strip > span { min-width:0; display:flex; flex-direction:column; }
.weapon-context-strip b { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-primary); font-size:calc(12px * var(--editor-scale)); }
.weapon-context-strip small { color:var(--text-muted); font-size:var(--fs-xs); }
.weapon-context-strip em { grid-column:2 / -1; min-width:0; color:#765126; font-size:var(--fs-xs); font-style:normal; font-variant-numeric:tabular-nums; white-space:normal; overflow-wrap:anywhere; }
.inline-resource-panel { display:grid; gap:7px; padding:8px; border:1px solid var(--line-soft); border-radius:7px; background:rgba(139,103,55,.055); }
.inline-resource-panel > small { color:var(--text-muted); font-size:var(--fs-xs); line-height:1.4; }
.inline-resource-toggle { min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr); gap:8px; align-items:center; cursor:pointer; }
.inline-resource-toggle input { width:16px; height:16px; margin:0; accent-color:#765126; }
.inline-resource-toggle span { min-width:0; display:flex; flex-direction:column; gap:1px; }
.inline-resource-toggle b { color:var(--text-primary); }
.inline-resource-toggle small { color:var(--text-muted); }
.weapon-skill-edit-list { display:flex; flex-direction:column; gap:6px; }
.weapon-skill-edit-row { min-width:0; display:grid; grid-template-columns:minmax(0,1fr); gap:7px; align-items:stretch; padding:6px 7px; border:1px solid var(--line-soft); border-radius:6px; background:rgba(255,255,255,.5); }
.weapon-skill-edit-row > span { min-width:0; display:flex; flex-direction:column; }
.weapon-skill-edit-row b { color:var(--text-primary); font-size:var(--fs-xs); }
.weapon-skill-edit-row small { overflow-wrap:anywhere; color:var(--text-muted); font-size:var(--fs-xs); }
.weapon-skill-edit-row .ui-select { width:100%; min-width:0; }
.summon-slot-list { display:flex; flex-direction:column; gap:8px; }
.summon-write-toggle { display:grid; grid-template-columns:auto minmax(0,1fr); gap:8px; align-items:center; padding:7px 8px; border:1px solid var(--line-soft); border-radius:7px; background:rgba(139,103,55,.055); cursor:pointer; }
.summon-write-toggle.disabled { opacity:.55; cursor:not-allowed; }
.summon-write-toggle input { width:16px; height:16px; margin:0; accent-color:#765126; }
.summon-write-toggle span { min-width:0; display:flex; flex-direction:column; gap:1px; }
.summon-write-toggle b { color:var(--text-primary); font-size:calc(12px * var(--editor-scale)); }
.summon-write-toggle small { color:var(--text-muted); font-size:var(--fs-xs); line-height:1.35; }
.summon-slot-card { position:relative; min-width:0; display:flex; flex-direction:column; gap:5px; padding:8px 7px 7px 31px; border-top:1px solid var(--line-soft); }
.summon-slot-card:first-child { border-top:0; }
.summon-slot-index { position:absolute; top:12px; left:2px; width:22px; height:22px; display:grid; place-items:center; border-radius:50%; background:#8b6737; color:#fff9e9; font-size:var(--fs-xs); font-weight:700; font-variant-numeric:tabular-nums; }
.summon-slot-card .ed-select { width:100%; min-width:0; }
.summon-icon { position:absolute; top:48px; right:7px; width:46px; height:46px; object-fit:contain; border:1px solid var(--line-soft); border-radius:7px; background:rgba(255,255,255,.7); }
.summon-source-lines { min-width:0; display:flex; flex-direction:column; gap:2px; padding:0 54px 0 2px; }
.summon-source-lines > b { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-primary); font-size:calc(12px * var(--editor-scale)); font-weight:600; }
.summon-source-lines > small { display:grid; grid-template-columns:19px minmax(0,1fr); gap:4px; color:var(--text-muted); line-height:1.4; }
.summon-source-lines i { height:17px; display:grid; place-items:center; border:1px solid var(--line-soft); border-radius:3px; color:#765126; font-style:normal; }
.summon-inline-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:6px; margin-top:4px; padding:8px; border:1px solid rgba(139,103,55,.2); border-radius:7px; background:rgba(255,255,255,.44); }
.summon-inline-grid label { min-width:0; display:flex; flex-direction:column; gap:3px; }
.summon-inline-grid label > span { color:var(--text-muted); font-size:var(--fs-xs); font-weight:600; }
.summon-inline-grid input,.summon-inline-grid select { width:100%; min-width:0; }
.summon-inline-wide { grid-column:span 2; }
.summon-invalid { color:var(--red); font-size:calc(12px * var(--editor-scale)); }
.pick-grid { display:flex; flex-wrap:wrap; gap:6px; padding:2px; }
.pick-grid.sigils { display:grid; grid-template-columns:repeat(auto-fit,minmax(260px,1fr)); grid-auto-rows:minmax(86px,auto); align-items:stretch; gap:9px; padding:3px 5px 3px 3px; }
.pick { display:inline-flex; align-items:center; gap:5px; padding:3px 9px; border:1px solid var(--line-soft); border-radius:12px; background:var(--panel-solid); color:var(--text-secondary); font-size:var(--fs-xs); cursor:pointer; user-select:none; }
.pick:hover { border-color:var(--line-gold); }
.pick.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.pick i { font-style:normal; margin-left:5px; opacity:.7; font-size:var(--fs-xs); }
.skill-icon { width:22px; height:22px; flex:0 0 22px; object-fit:cover; border-radius:4px; box-shadow:0 0 0 1px rgba(118,81,38,.28); }
.bag-toolbar { display:grid; grid-template-columns:auto minmax(150px,1fr) auto; gap:8px; align-items:center; padding:7px 8px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel-solid); }
.bag-toolbar strong { font-size:var(--fs-xs); color:var(--text-primary); }
.bag-toolbar input { min-height:28px; padding:0 8px; border:1px solid var(--line-soft); border-radius:5px; background:var(--panel); color:var(--text-primary); font-size:var(--fs-xs); }
.bag-toolbar input:focus { outline:1px solid var(--line-gold); }
.bag-toolbar span { font-size:var(--fs-xs); color:var(--text-muted); }
.bag-filter-row { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:7px; }
.bag-filter-row .ui-select { width:100%; min-width:0; min-height:30px; color:var(--text-secondary); font-size:var(--fs-xs); }
.catalog-empty { margin:0; padding:10px; border:1px dashed var(--line-soft); border-radius:6px; color:var(--text-muted); font-size:var(--fs-xs); text-align:center; }
.constructor-wide .catalog-empty { display:block; margin-top:5px; padding:6px; }
.sigil-pick { min-width:0; min-height:86px; height:100%; display:grid; grid-template-columns:36px minmax(0,1fr) auto; gap:8px; align-items:center; padding:8px 9px; border-radius:7px; text-align:left; }
.sigil-glyph { width:23px; height:23px; display:grid; place-items:center; border:1px solid var(--line-gold); border-radius:50%; background:rgba(139,103,55,.08); color:var(--gold); font-size:var(--fs-sm); }
.sigil-copy { min-width:0; display:flex; flex-direction:column; gap:3px; }
.sigil-copy b { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-primary); font-size:var(--fs-sm); line-height:1.25; }
.sigil-copy .trait-line { min-width:0; display:grid; grid-template-columns:18px minmax(0,1fr) auto; gap:5px; align-items:center; overflow:visible; color:var(--text-muted); font-size:var(--fs-xs); line-height:1.25; white-space:normal; }
.trait-line > i { width:18px; height:16px; display:grid; place-items:center; margin:0; border:1px solid rgba(139,103,55,.3); border-radius:3px; color:#765126; font-size:var(--fs-xs); font-style:normal; font-weight:700; opacity:1; }
.trait-line > span { min-width:0; overflow-wrap:anywhere; }
.trait-line > em { color:var(--gold); font-size:var(--fs-xs); font-style:normal; font-weight:600; white-space:nowrap; }
.sigil-pick.on .sigil-copy b, .sigil-pick.on .sigil-copy small, .sigil-pick.on .sigil-glyph { color:inherit; }
.sigil-pick.used:not(.on) { border-color:var(--line-gold); box-shadow:inset 0 0 0 1px rgba(139,103,55,.08); }
.bag-factor-meta { align-self:stretch; display:flex; flex-direction:column; align-items:flex-end; justify-content:space-between; gap:4px; color:var(--text-muted); }
.bag-factor-meta b { padding:2px 5px; border-radius:8px; background:rgba(139,103,55,.13); color:#765126; font-size:var(--fs-xs); white-space:nowrap; }
.bag-factor-meta i { margin:0; }
.empty { font-size:var(--fs-xs); color:var(--text-muted); }
.apply-btn { min-height:34px; padding:0 18px; border:1px solid #765126; border-radius:6px; background:#8b6737; color:#fff9e9; font-size:.8rem; font-weight:700; cursor:pointer; }
.apply-btn:hover:not(:disabled) { background:#76552d; }
.apply-btn:disabled { opacity:.45; cursor:not-allowed; }
/* 右侧角色总计：始终保留因子加成、技能与专精效果 */
.result-sidebar { display:flex; flex-direction:column; gap:0; }
.result-overview { order:0; }
.skill-summary-card { order:1; }
.bonus-summary-card { order:2; }
.mastery-summary-card { order:3; }
.result-card { min-width:0; padding:14px; border:0; border-radius:0; background:transparent; box-shadow:none; }
.result-card + .result-card { border-top:1px solid var(--line-soft); }
.result-card > header { display:flex; align-items:baseline; justify-content:space-between; gap:8px; margin-bottom:8px; }
.result-card > header strong { font-size:var(--fs-sm); color:var(--text-primary); }
.result-card > header span { font-size:calc(11px * var(--editor-scale)); color:var(--text-muted); text-align:right; }
.result-overview { background:rgba(139,103,55,.055); }
.result-metrics { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:6px; }
.result-metrics div { display:flex; align-items:baseline; justify-content:center; gap:2px; padding:7px 4px; border:1px solid var(--line-soft); border-radius:6px; background:var(--panel); }
.result-metrics b { font-size:1rem; color:var(--gold); font-variant-numeric:tabular-nums; }
.result-metrics span { font-size:var(--fs-xs); color:var(--text-muted); }
.trait-level-ledger { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:5px; margin:0 0 8px; }
.trait-level-ledger > span { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; align-items:baseline; gap:4px; padding:5px 7px; border:1px solid var(--line-soft); border-radius:5px; background:rgba(255,255,255,.42); }
.trait-level-ledger small { color:var(--text-muted); font-size:var(--fs-xs); }
.trait-level-ledger b { color:#765126; font-size:calc(12px * var(--editor-scale)); font-variant-numeric:tabular-nums; }
.trait-level-ledger .overflow { grid-template-columns:minmax(0,1fr) auto; border-color:rgba(174,94,58,.25); background:rgba(174,94,58,.055); }
.trait-level-ledger .overflow b { color:#a45735; }
.trait-level-ledger .overflow em { grid-column:1/-1; color:var(--text-muted); font-size:var(--fs-xs); font-style:normal; line-height:1.25; }
.effect-total-list { display:flex; flex-direction:column; gap:5px; }
.effect-total-row { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:9px; align-items:center; padding:7px 8px; border:1px solid var(--border-soft); border-left:3px solid var(--border-soft); border-radius:6px; background:var(--surface-card-pop); }
.effect-total-row.atk { border-left-color:var(--danger); }
.effect-total-row.base { border-left-color:var(--accent); }
.effect-total-row.def { border-left-color:var(--info); }
.effect-total-row.sup { border-left-color:var(--success); }
.effect-total-row > span { min-width:0; display:flex; flex-direction:column; gap:1px; }
.effect-total-row > span b { white-space:normal; overflow-wrap:anywhere; color:var(--text-primary); font-size:calc(12px * var(--editor-scale)); }
.effect-total-row > span small { white-space:normal; overflow-wrap:anywhere; color:var(--text-muted); line-height:1.4; }
.effect-total-values { display:flex; flex-direction:column; align-items:flex-end; gap:2px; color:#765126; font-size:calc(15px * var(--editor-scale)); font-variant-numeric:tabular-nums; white-space:nowrap; }
.effect-total-values > span { display:flex; align-items:baseline; justify-content:flex-end; gap:5px; }
.effect-total-values small { color:var(--text-muted); font-size:var(--fs-xs); font-weight:500; }
.effect-total-values b { color:#765126; font-size:inherit; }
.trait-detail-disclosure { margin-top:8px; border-top:1px solid var(--line-soft); padding-top:7px; }
.trait-detail-disclosure summary { color:#765126; font-size:calc(12px * var(--editor-scale)); font-weight:600; cursor:pointer; }
.trait-detail-disclosure[open] summary { margin-bottom:7px; }
.sim-list { display:flex; flex-direction:column; gap:2px; border:1px solid var(--border-soft); border-radius:var(--radius-md); background:var(--surface-card-pop); padding:4px; }
.sim-row { display:grid; grid-template-columns:minmax(5.5em,.8fr) 3.5em minmax(0,1.5fr); gap:7px; align-items:baseline; padding:4px 7px; border-radius:var(--radius-sm); font-size:calc(12px * var(--editor-scale)); border-left:3px solid var(--border-soft); }
.sim-row.atk { border-left-color:var(--danger); }
.sim-row.base { border-left-color:var(--accent); }
.sim-row.def { border-left-color:var(--info); }
.sim-row.sup { border-left-color:var(--success); }
.sim-row:nth-child(even) { background:var(--surface-row); }
.sim-name { min-width:0; font-weight:600; color:var(--text-primary); white-space:normal; overflow-wrap:anywhere; }
.sim-cap { font-style:normal; margin-left:4px; font-size:var(--fs-xs); color:var(--warning-ink); }
.sim-lv { color:var(--text-secondary); font-variant-numeric:tabular-nums; white-space:nowrap; }
.sim-lv small { color:var(--text-muted); }
.sim-eff { color:var(--text-secondary); white-space:pre-line; line-height:1.4; }
.skill-chips { display:flex; flex-wrap:wrap; gap:5px; margin-bottom:8px; }
.skill-chips span { padding:3px 8px; border:1px solid var(--line-gold); border-radius:12px; background:rgba(139,103,55,.1); color:var(--text-primary); font-size:var(--fs-xs); }
.skill-chips i { font-style:normal; color:var(--text-muted); font-size:var(--fs-xs); }
.weapon-skill-block { display:flex; flex-direction:column; gap:6px; padding-top:8px; border-top:1px solid var(--line-soft); }
.weapon-skill-title { display:flex; justify-content:space-between; align-items:baseline; gap:8px; }
.weapon-skill-title b { color:var(--text-primary); font-size:calc(12px * var(--editor-scale)); }
.weapon-skill-title span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-muted); font-size:var(--fs-xs); }
.weapon-skill-list { display:flex; flex-direction:column; gap:5px; }
.weapon-skill-list article { padding:7px 8px; border:1px solid var(--line-soft); border-left:3px solid #8b6737; border-radius:6px; background:rgba(139,103,55,.045); }
.weapon-skill-list article > span { display:flex; align-items:baseline; justify-content:space-between; gap:7px; }
.weapon-skill-list article b { min-width:0; overflow-wrap:anywhere; color:var(--text-primary); font-size:calc(12px * var(--editor-scale)); }
.weapon-skill-list article em { color:#765126; font-size:calc(11px * var(--editor-scale)); font-style:normal; font-weight:700; white-space:nowrap; }
.weapon-skill-list article p { margin:3px 0 0; color:var(--text-secondary); font-size:calc(11px * var(--editor-scale)); line-height:1.45; white-space:pre-line; }
.weapon-skill-list article small { display:block; margin-top:3px; color:var(--text-muted); font-size:var(--fs-xs); }
.wrightstone-audit { display:flex; flex-direction:column; gap:4px; padding:7px; border:1px solid rgba(100,68,137,.22); border-radius:7px; background:rgba(100,68,137,.045); }
.wrightstone-audit-head { display:flex; flex-wrap:wrap; justify-content:space-between; gap:4px 8px; padding-bottom:4px; border-bottom:1px solid rgba(100,68,137,.14); }
.wrightstone-audit-head > span { min-width:0; display:flex; flex-direction:column; }
.wrightstone-audit-head b { color:var(--text-primary); font-size:calc(12px * var(--editor-scale)); overflow-wrap:anywhere; }
.wrightstone-audit-head small { color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); font-family:ui-monospace,monospace; }
.wrightstone-audit-head em { color:#684489; font-size:calc(11px * var(--editor-scale)); font-style:normal; font-weight:700; }
.wrightstone-audit article { display:grid; grid-template-columns:minmax(0,1fr) auto auto; gap:2px 7px; align-items:center; padding:5px 6px; border-left:3px solid #8061a0; border-radius:5px; background:rgba(255,255,255,.52); }
.wrightstone-audit article > span { min-width:0; display:flex; flex-direction:column; }
.wrightstone-audit article b { overflow-wrap:anywhere; color:var(--text-primary); font-size:calc(11px * var(--editor-scale)); }
.wrightstone-audit article small { color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); font-family:ui-monospace,monospace; }
.wrightstone-audit article strong, .wrightstone-audit article em { white-space:nowrap; color:#684489; font-size:calc(11px * var(--editor-scale)); font-style:normal; }
.wrightstone-audit article p { grid-column:1/-1; margin:2px 0 0; color:var(--text-secondary); font-size:calc(11px * var(--editor-scale)); line-height:1.4; white-space:pre-line; }
.calculation-scope-note { margin:0 0 9px; padding:8px 9px; border:1px solid rgba(139,103,55,.18); border-radius:6px; background:rgba(139,103,55,.05); color:var(--text-muted); font-size:var(--fs-xs); line-height:1.55; }
.mastery-effect-list { display:flex; flex-direction:column; gap:4px; padding-right:2px; }
.mastery-effect { padding:6px 7px; border-left:3px solid var(--line-gold); border-radius:4px; background:var(--surface-row); }
.mastery-effect > span { display:block; font-size:calc(11px * var(--editor-scale)); color:var(--text-muted); }
.mastery-effect > b { display:block; margin-top:2px; font-size:var(--fs-xs); color:var(--gold); }
.mastery-effect > p { margin:2px 0 0; font-size:var(--fs-xs); line-height:1.4; color:var(--text-secondary); }
.mastery-effect > small { display:block; margin-top:3px; color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); line-height:1.4; }
.mastery-result { display:flex; flex-direction:column; gap:7px; }
.mastery-result-rank { padding-top:6px; border-top:1px dashed var(--line-soft); }
.mastery-result-rank:first-child { padding-top:0; border-top:0; }
.rank-line { display:flex; justify-content:space-between; align-items:center; margin-bottom:4px; }
.rank-line b { font-size:calc(12px * var(--editor-scale)); color:var(--text-primary); }
.rank-line em { font-style:normal; font-size:calc(11px * var(--editor-scale)); color:var(--gold); font-variant-numeric:tabular-nums; }
.mastery-result-cat { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:1px 7px; padding:3px 5px; border-radius:4px; opacity:.68; }
.mastery-result-cat.active { opacity:1; background:rgba(63,125,92,.1); }
.mastery-result-cat span { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:var(--fs-xs); color:var(--text-secondary); }
.mastery-result-cat b { font-size:calc(11px * var(--editor-scale)); color:var(--text-muted); }
.mastery-result-cat i { grid-column:1/-1; font-style:normal; font-size:calc(11px * var(--editor-scale)); color:var(--text-muted); }
.mastery-result-cat.active i { color:#3f7d5c; font-weight:600; }
.ex-result { padding:5px 6px; border-radius:4px; background:rgba(139,103,55,.09); font-size:calc(11px * var(--editor-scale)); line-height:1.45; color:var(--text-secondary); }
.mastery-detail-disclosure { margin-top:8px; border-top:1px solid var(--line-soft); padding-top:7px; }
.mastery-detail-disclosure summary { color:#765126; font-size:calc(12px * var(--editor-scale)); font-weight:600; cursor:pointer; }
.mastery-detail-disclosure[open] summary { margin-bottom:7px; }
/* 专精自由配置 */
.rank-tabs { display:flex; flex-wrap:wrap; gap:6px; }
.rank-tab { min-height:28px; padding:0 12px; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-card-pop); color:var(--text-secondary); font-size:var(--fs-sm); cursor:pointer; user-select:none; }
.rank-tab.on { background:var(--selected-bg); border-color:var(--selected-border); color:var(--selected-fg); }
.rank-tab i { font-style:normal; margin-left:5px; font-size:var(--fs-xs); opacity:.8; }
.rank-tab i.full { color:var(--success); font-weight:600; }
.rank-tab.on i.full { color:#bfe6cd; }
.mastery-unlock-warning { display:block; margin:6px 10px; padding:7px 9px; border-left:3px solid var(--warning); border-radius:var(--radius-sm); background:var(--warning-bg); color:var(--warning-ink); font-weight:var(--fw-bold); line-height:var(--lh-normal); overflow-wrap:anywhere; }
.mastery-auto-direction { display:flex; flex-wrap:wrap; align-items:baseline; justify-content:space-between; gap:4px 10px; padding:8px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel-solid); }
.mastery-auto-direction strong { font-size:var(--fs-xs); color:var(--text-primary); }
.mastery-auto-direction small { min-width:0; color:var(--text-muted); font-size:var(--fs-xs); overflow-wrap:anywhere; }
.mastery-rule { display:flex; gap:6px; align-items:center; padding:7px 9px; border:1px solid var(--line-soft); border-radius:6px; background:rgba(63,125,92,.07); color:var(--text-secondary); font-size:calc(11px * var(--editor-scale)); line-height:1.45; }
.mastery-rule.direction-rule { flex-direction:column; align-items:flex-start; }
.mastery-rule.ex-rule { flex-direction:column; align-items:flex-start; border-color:var(--line-gold); background:rgba(139,103,55,.08); }
.mastery-rule b { color:var(--gold); }
.mastery-groups { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:7px; padding:2px; }
.mastery-group { min-width:0; padding:7px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel-solid); }
.mastery-group > header { display:flex; align-items:center; gap:5px; margin-bottom:6px; }
.mastery-group > header .cat-mark { width:19px; height:19px; display:grid; place-items:center; border-radius:4px; color:#fff; font-size:var(--fs-xs); font-weight:700; }
.mastery-group.cat-攻 .cat-mark { background:var(--danger); }
.mastery-group.cat-防 .cat-mark { background:var(--info); }
.mastery-group.cat-界 .cat-mark { background:var(--accent); }
.mastery-group > header strong { font-size:var(--fs-sm); color:var(--text-primary); }
.mastery-group > header em { margin-left:auto; font-style:normal; font-size:var(--fs-xs); color:var(--text-muted); }
.mastery-group > header i { font-style:normal; padding:1px 5px; border-radius:8px; background:#3f7d5c; color:#fff; font-size:var(--fs-xs); }
.node-list { display:flex; flex-direction:column; gap:4px; }
.node { width:100%; display:grid; grid-template-columns:16px minmax(0,1fr) auto; gap:6px; align-items:start; padding:6px; border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card-pop); color:var(--text-secondary); text-align:left; cursor:pointer; user-select:none; }
.node-check { width:15px; height:15px; display:grid; place-items:center; margin-top:1px; border:1px solid var(--line-soft); border-radius:3px; color:#fff; font-size:var(--fs-xs); }
.node > span:nth-child(2) { min-width:0; display:flex; flex-direction:column; gap:2px; }
.node b { font-size:var(--fs-xs); color:var(--gold); }
.node small { font-size:var(--fs-xs); line-height:1.38; color:var(--text-secondary); }
.node > em { font-style:normal; font-size:var(--fs-xs); color:var(--gold); white-space:nowrap; }
.node.on { border-color:var(--selected-border); background:var(--selected-bg); color:var(--selected-fg); }
.node.on .node-check { border-color:rgba(255,255,255,.55); background:rgba(255,255,255,.22); }
.node.on b, .node.on small, .node.on > em { color:inherit; }
.node.disabled { opacity:.42; cursor:not-allowed; }
.ed-field .hint { font-size:var(--fs-xs); color:var(--text-muted); }

/* Keep one stable type hierarchy inside the dedicated editor:
   16px headings, 14px labels/body controls, 12px supporting copy. */
.loadout-editor .ed-head strong,
.loadout-editor .mastery-toggle b { font-size:calc(16px * var(--editor-scale)); line-height:1.35; }
.loadout-editor .ed-field > label,
.loadout-editor .result-card > header strong,
.loadout-editor .bag-toolbar strong,
.loadout-editor .mastery-rule b { font-size:calc(14px * var(--editor-scale)); }
.loadout-editor button,
.loadout-editor input,
.loadout-editor select,
.loadout-editor .sigil-copy b,
.loadout-editor .mastery-group > header strong { font-family:inherit; font-size:calc(13px * var(--editor-scale)); }
.loadout-editor small,
.loadout-editor .hint,
.loadout-editor .source-badge,
.loadout-editor .owner,
.loadout-editor .result-card > header span,
.loadout-editor .trait-line,
.loadout-editor .sim-row,
.loadout-editor .mastery-result-cat span,
.loadout-editor .mastery-effect > p { font-size:calc(12px * var(--editor-scale)); }
.loadout-editor button:focus-visible,
.loadout-editor input:focus-visible,
.loadout-editor select:focus-visible { outline:2px solid #a66b20; outline-offset:2px; }

.skill-field { flex:1 0 auto; }
.skill-grid { --ui-grid-min:116px; grid-template-columns:repeat(auto-fit, minmax(116px, 1fr)); gap:var(--space-3); }
.skill-pick { position:relative; min-width:0; min-height:50px; display:grid; grid-template-columns:34px minmax(0,1fr); gap:7px; align-items:center; padding:6px 9px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel-solid); color:var(--text-secondary); text-align:left; cursor:pointer; }
.skill-pick:hover { border-color:var(--line-gold); transform:translateY(-1px); }
.skill-pick.on { border-color:#765126; background:linear-gradient(145deg, #9a7543, #765126); color:#fff9e9; }
.skill-pick.pending { border-color:#a33c2d; box-shadow:0 0 0 2px rgba(163,60,45,.12); }
.skill-pick span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-weight:700; }
.loadout-editor .skill-icon { width:32px; height:32px; flex:0 0 32px; object-fit:cover; border-radius:6px; box-shadow:0 0 0 1px rgba(118,81,38,.3); }
.skill-order { position:absolute; top:3px; right:4px; width:17px; height:17px; display:grid; place-items:center; border-radius:50%; background:#f8ead1; color:#765126; font-size:var(--fs-xs); font-variant-numeric:tabular-nums; box-shadow:0 1px 3px rgba(0,0,0,.16); }
.replace-strip { display:grid; grid-template-columns:1fr; gap:5px; padding:8px; border:1px solid rgba(163,60,45,.35); border-radius:7px; background:rgba(163,60,45,.06); }
.replace-strip > span { color:var(--red); font-weight:600; font-size:calc(12px * var(--editor-scale)); }
.replace-strip button { min-height:28px; display:flex; align-items:center; gap:6px; padding:3px 7px; border:1px solid var(--line-soft); border-radius:5px; background:var(--panel); color:var(--text-primary); text-align:left; cursor:pointer; }
.replace-strip button b { width:18px; height:18px; display:grid; place-items:center; border-radius:50%; background:#8b6737; color:white; }
.replace-strip .replace-cancel { justify-content:center; color:var(--text-muted); }

.factor-slot-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(190px,1fr)); gap:8px; }
.factor-slot-card { position:relative; min-width:0; min-height:92px; height:auto; display:grid; grid-template-columns:36px minmax(0,1fr); grid-template-rows:minmax(0,1fr) auto; gap:5px 8px; align-items:start; padding:8px 8px 6px 44px; border:1px solid #d1bf98; border-radius:8px; background:#fffdf7; color:var(--text-secondary); text-align:left; cursor:pointer; overflow:hidden; }
.factor-slot-card:hover { border-color:#9e7a45; transform:translateY(-1px); }
.factor-slot-card.active { border-color:#765126; box-shadow:0 0 0 2px rgba(118,81,38,.16); }
.factor-slot-card.draft { background:linear-gradient(145deg, #fff9e8, #f2e3c8); }
.factor-slot-card.empty { display:flex; flex-direction:column; align-items:center; justify-content:center; gap:1px; padding:9px; border-style:dashed; background:repeating-linear-gradient(-45deg, rgba(255,255,255,.4), rgba(255,255,255,.4) 7px, rgba(222,208,176,.15) 7px, rgba(222,208,176,.15) 14px); color:var(--text-muted); text-align:center; }
.factor-slot-card.empty.active { border-style:solid; background:#f6ecd7; }
.factor-slot-number { position:absolute; left:8px; bottom:6px; margin:0; color:#9b8c72; font:700 calc(11px * var(--editor-scale))/1 ui-monospace, monospace; font-style:normal; }
.factor-slot-card > .sigil-icon-frame { position:absolute; left:7px; top:8px; }
.factor-slot-copy { min-width:0; grid-column:1/-1; display:flex; flex-direction:column; gap:3px; }
.factor-slot-copy > b { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-primary); }
.factor-slot-copy .trait-line { min-width:0; display:grid; grid-template-columns:18px minmax(0,1fr) auto; gap:4px; align-items:center; color:var(--text-muted); }
.factor-source { grid-column:1/-1; justify-self:end; align-self:end; margin:0; color:#8a6a3f; font-size:var(--fs-xs); font-style:normal; white-space:nowrap; }
.empty-factor-mark { font-size:calc(22px * var(--editor-scale)); line-height:1; color:#8b6737; }
.factor-slot-card.empty strong { color:var(--text-secondary); }
.factor-selection-bar { min-height:34px; display:flex; align-items:center; justify-content:space-between; gap:8px; padding:5px 8px; border:1px solid var(--line-soft); border-radius:6px; background:var(--panel-solid); }
.factor-selection-bar span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-muted); }
.factor-selection-bar b { margin-right:4px; color:#765126; }
.factor-selection-bar button { min-height:var(--control-height-sm); padding:0 8px; border:1px solid rgba(163,60,45,.28); border-radius:5px; background:rgba(163,60,45,.06); color:var(--red); cursor:pointer; white-space:nowrap; }
.factor-mode-tabs { display:grid; grid-template-columns:1fr 1fr; gap:7px; }
.factor-mode-tabs button { min-height:48px; display:flex; flex-direction:column; align-items:flex-start; justify-content:center; gap:1px; padding:6px 10px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel); color:var(--text-secondary); cursor:pointer; }
.factor-mode-tabs button b { color:var(--text-primary); }
.factor-mode-tabs button span { font-size:var(--fs-xs); color:var(--text-muted); }
.factor-mode-tabs button.on { border-color:#765126; background:linear-gradient(145deg, rgba(154,117,67,.18), rgba(139,103,55,.08)); box-shadow:inset 0 0 0 1px rgba(118,81,38,.12); }
.factor-mode-tabs button.on b { color:#765126; }
.sigil-icon-frame { width:34px; height:34px; display:grid; place-items:center; overflow:hidden; border:1px solid var(--line-gold); border-radius:8px; background:linear-gradient(145deg, #fbf6eb, #e9dcc5); color:var(--gold); }
.sigil-icon-frame img { width:100%; height:100%; object-fit:cover; }
.sigil-icon-frame > i { margin:0; font-style:normal; opacity:1; }
.sigil-icon-frame.large { width:48px; height:48px; border-radius:10px; }
.loadout-editor .sigil-pick { grid-template-columns:36px minmax(0,1fr) auto; min-height:86px; height:100%; }

.bag-picker-shell { display:flex; flex-direction:column; gap:8px; padding:9px; border:1px solid var(--line-soft); border-radius:9px; background:var(--panel-solid); }
.bag-virtual-viewport { position:relative; height:clamp(300px,46dvh,560px); min-height:0; overflow-x:hidden; overflow-y:auto; overscroll-behavior:contain; scrollbar-gutter:stable; contain:layout paint; }
.bag-virtual-spacer { position:relative; width:100%; min-width:0; }
.pick-grid.sigils.bag-virtual-window { position:absolute; top:0; right:0; left:0; grid-template-columns:repeat(var(--bag-columns),minmax(0,1fr)); grid-auto-rows:86px; will-change:transform; }
.bag-virtual-window .sigil-pick { min-height:86px; height:86px; overflow:hidden; }
.bag-virtual-window .sigil-copy > b,
.bag-virtual-window .trait-line > span { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.bag-virtual-window .sigil-copy .trait-line { overflow:hidden; white-space:nowrap; }
.constructor-shell { display:flex; flex-direction:column; gap:9px; padding:10px; border:1px solid var(--line-gold); border-radius:9px; background:rgba(249,242,225,.78); }
.constructor-note { display:flex; align-items:center; gap:9px; }
.constructor-mark { width:30px; height:30px; display:grid; place-items:center; flex:0 0 30px; border-radius:50%; background:#8b6737; color:#fff; font-size:calc(12px * var(--editor-scale)); font-weight:700; line-height:1; }
.constructor-note > div { min-width:0; display:flex; flex-direction:column; gap:1px; }
.constructor-note b { color:var(--text-primary); }
.constructor-note small { color:var(--text-muted); line-height:1.45; }
.constructor-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:8px; }
.constructor-grid label { min-width:0; display:flex; flex-direction:column; gap:3px; }
.constructor-grid label > span { font-size:calc(12px * var(--editor-scale)); font-weight:600; color:var(--text-secondary); }
.constructor-grid input,
.constructor-grid select { width:100%; min-height:32px; padding:0 8px; border:1px solid var(--line-soft); border-radius:6px; background:var(--panel); color:var(--text-primary); }
.constructor-wide,
.constructor-search { grid-column:1/-1; }
.constructor-preview { display:grid; grid-template-columns:auto minmax(0,1fr) auto; gap:10px; align-items:center; padding:9px; border:1px solid rgba(118,81,38,.25); border-radius:8px; background:rgba(255,255,255,.58); }
.constructor-preview > div { min-width:0; display:flex; flex-direction:column; }
.constructor-preview > div b { color:var(--text-primary); }
.constructor-preview > div span { color:var(--text-muted); font-size:calc(11px * var(--editor-scale)); }
.constructor-preview > button { min-height:34px; padding:0 13px; border:1px solid #765126; border-radius:6px; background:#8b6737; color:#fff9e9; cursor:pointer; }
.constructor-preview > button:disabled { opacity:.48; cursor:not-allowed; }

.mastery-field { padding:0; overflow:hidden; }
.mastery-toggle { width:100%; min-height:54px; display:grid; grid-template-columns:minmax(0,1fr) auto auto; gap:9px; align-items:center; padding:9px 11px; border:0; background:rgba(139,103,55,.1); color:var(--text-primary); text-align:left; cursor:pointer; }
.mastery-direction-map { margin:0 10px 10px; }
.mastery-toggle > span { display:flex; flex-direction:column; }
.mastery-toggle small { color:var(--text-muted); font-weight:400; }
.mastery-toggle em { font-style:normal; color:var(--gold); font-weight:700; font-variant-numeric:tabular-nums; }
.mastery-toggle i { min-width:42px; font-style:normal; color:#765126; text-align:right; }
.mastery-panel { display:flex; flex-direction:column; gap:9px; padding:10px; border-top:1px solid var(--line-soft); }
.mastery-panel .op-row { flex:0 0 auto; }
.mastery-direction-map { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:7px; }
.direction-card { min-width:0; padding:8px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel-solid); }
.direction-card.is-primary-direction { border-color:#3f7d5c; box-shadow:inset 0 0 0 1px rgba(63,125,92,.14); }
.direction-card > header { min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr); gap:6px; align-items:center; margin-bottom:6px; }
.direction-card .cat-mark { width:22px; height:22px; display:grid; place-items:center; border-radius:5px; color:#fff; font-weight:700; }
.direction-card.cat-攻 .cat-mark { background:var(--danger); }
.direction-card.cat-防 .cat-mark { background:var(--info); }
.direction-card.cat-界 .cat-mark { background:var(--accent); }
.direction-card > header div { min-width:0; display:flex; flex-direction:column; }
.direction-card > header small { color:var(--text-muted); }
.direction-card > header b { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-primary); }
.direction-card > header em { grid-column:1/-1; padding:2px 5px; border-radius:4px; background:rgba(63,125,92,.1); color:#3f7d5c; font-size:calc(11px * var(--editor-scale)); font-style:normal; font-weight:600; }
.direction-stage { display:grid; grid-template-columns:auto auto; gap:1px 5px; padding:4px 0; border-top:1px dashed var(--line-soft); opacity:.72; }
.direction-stage.active { opacity:1; }
.direction-stage > b { color:var(--text-primary); font-size:calc(11px * var(--editor-scale)); }
.direction-stage > span { margin-left:auto; color:var(--gold); font-size:var(--fs-xs); font-variant-numeric:tabular-nums; }
.direction-stage > small { grid-column:1/-1; min-height:2.8em; color:var(--text-muted); line-height:1.35; }
.direction-stage.active > small { color:#3f7d5c; font-weight:600; }
.direction-effect { grid-column:1/-1; margin:2px 0 0; padding-top:4px; border-top:1px dotted var(--line-soft); color:var(--text-secondary); font-size:var(--fs-xs); line-height:1.45; overflow-wrap:anywhere; }
.mastery-result-cat p { grid-column:1/-1; margin:2px 0 0; color:var(--text-secondary); font-size:var(--fs-xs); line-height:1.45; overflow-wrap:anywhere; }

.loadout-editor .skill-chips span { display:flex; align-items:center; gap:5px; padding:4px 7px; }
.loadout-editor .skill-chips span b { width:16px; height:16px; display:grid; place-items:center; border-radius:50%; background:#8b6737; color:white; font-size:var(--fs-xs); }
.loadout-editor .skill-chips img { width:24px; height:24px; border-radius:4px; object-fit:cover; }

@container loadout-editor (max-width:1199px) {
  .loadout-editor, .editor-layout { height:auto; min-height:100%; }
  .editor-layout { grid-template-columns:minmax(270px,300px) minmax(0,1fr); align-items:start; }
  .editor-column { overflow:visible; }
  .result-sidebar { position:relative; grid-column:1/-1; display:grid; grid-template-columns:repeat(auto-fit,minmax(280px,1fr)); overflow:visible; }
}
@container loadout-editor (max-width:900px) {
  .editor-layout { grid-template-columns:1fr; }
  .result-sidebar { grid-column:auto; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); }
}
@container loadout-editor (max-width:760px) {
  .mastery-groups, .result-sidebar { grid-template-columns:1fr; }
  .bag-filter-row { grid-template-columns:1fr; }
  .weapon-skill-edit-row { grid-template-columns:1fr; }
  .factor-slot-grid, .mastery-direction-map, .constructor-grid { grid-template-columns:1fr; }
  .constructor-wide, .constructor-search { grid-column:auto; }
  .source-badge { width:100%; margin-left:0; }
  .ed-head { grid-template-columns:minmax(0,1fr); }
  .ed-head .owner { justify-self:start; }
  .bag-toolbar { grid-template-columns:1fr auto; }
  .bag-toolbar strong { grid-column:1/-1; }
  .bag-virtual-viewport { height:clamp(260px,42dvh,440px); }
  .summon-inline-grid { grid-template-columns:1fr; }
  .summon-inline-wide { grid-column:auto; }
  .editor-save-bar { align-items:stretch; flex-direction:column; }
  .editor-persistent-actions { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); }
  .editor-persistent-actions .single-loadout-label { grid-column:1/-1; padding:0; }
  .editor-persistent-actions .single-loadout-action { width:100%; }
  .editor-persistent-actions .editor-save-button { grid-column:1/-1; width:100%; }
}
</style>
