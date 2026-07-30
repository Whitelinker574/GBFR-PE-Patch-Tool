<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { LoadoutEditContext, LoadoutOptimizerEvidence, LoadoutOptimizerInventorySnapshot, LoadoutSimulateBuild, LoadoutStatContext } from '../../wailsjs/go/backend/App'
import { characterIdentityByHash } from '../characterRoster.js'
import { traitAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'
import { characterBuildRoutes, LOADOUT_CHARACTER_ROUTE_VERSION, routeTraitTargets } from '../loadoutCharacterRoutes.js'
import { graduationRouteBranches, LOADOUT_ROUTE_BRANCH_VERSION } from '../loadoutRouteBranches.js'
import { buildCatalogCandidates, buildInventoryCandidates, buildTableExactCandidates, synthesizeOwnedFirstSuggestion } from '../loadoutOptimizer'
import { characterLoadoutProfile, LOADOUT_ACTION_TYPES, LOADOUT_CHARACTER_PROFILE_VERSION, LOADOUT_DIRECTIONS, LOADOUT_SCENARIO_VERSION } from '../loadoutScenarioConfig.js'
import { sigilAtlasStore } from '../sigilAtlasStore'
import { createOptimizerWorkerMessage } from '../utils/optimizerWorkerPayload.js'
import CatalogSelect from './CatalogSelect.vue'

const props = defineProps({
  savePath: { type: String, default: '' },
  charaHash: { type: String, default: '' },
  charaName: { type: String, default: '' },
  baseLoadout: { type: Object, default: null },
  embedded: { type: Boolean, default: false },
  pendingTarget: { type: Object, default: null },
})
const emit = defineEmits(['status', 'apply'])
const atlas = ref({ sigils: [], traits: [], dataVersion: '' })
const inventory = ref([])
const inventorySnapshot = ref(null)
const ownerCode = ref('')
const loadedInventoryKey = ref('')
const evidence = ref({ traits: [], dataVersion: '', formulaVersion: '' })
const statContext = ref(null)
const fixedSimulation = ref(null)
const loading = ref(false)
const solving = ref(false)
const domain = ref('all')
const profile = ref('custom')
const selectedRouteId = ref('')
const selectedRouteBranchId = ref('graduation')
const selected = ref([])
const targetLevels = ref({})
const customSelected = ref([])
const customTargetLevels = ref({})
const pendingTraitId = ref('')
const pendingTraitLevel = ref(15)
const results = ref([])
const expandedResultKeys = ref(new Set())
const solved = ref(false)
const solveError = ref('')
const resultRegion = ref(null)
const coverage = ref(100)
const coverageLow = ref(85)
const coverageHigh = ref(100)
const currentHp = ref(100)
const odStage = ref(0)
const berserk = ref(false)
const actionRate = ref(1)
const minimumHp = ref(0)
const minimumDefense = ref(0)
const incomingDamage = ref(100000)
const surviveHits = ref(0)
let solveWorker = null
let rejectSolve = null
let solveGeneration = 0

const tx = (zh, en) => language.value === 'en' ? en : zh
const domainLabel = domain => ({ all: tx('背包优先 · 缺失制造', 'Owned First · Create Gaps'), 'owned-first': tx('背包优先方案', 'Owned-First Plan'), inventory: tx('当前存档可部署', 'Deployable Save'), catalog: tx('工具目录合法构造', 'Tool Catalog'), table: tx('游戏表基线', 'Game Table Baseline') }[domain] || domain)
const profiles = {
  character: { zh: '当前角色 · 本配装', en: 'Character & Current Preset', traitIds: [] },
  ...Object.fromEntries(Object.entries(LOADOUT_DIRECTIONS).map(([key, item]) => [key, { zh: item.zh, en: item.en, traitIds: item.traitIds }])),
  custom: { zh: '自定义', en: 'Custom', traitIds: [] },
}
const characterProfile = computed(() => characterLoadoutProfile(props.charaHash))
const characterIdentity = computed(() => characterIdentityByHash(props.charaHash))
const characterRoutes = computed(() => characterBuildRoutes(props.charaHash))
const selectedRoute = computed(() => characterRoutes.value.find(route => route.id === selectedRouteId.value) || characterRoutes.value[0] || null)
const routeBranches = computed(() => graduationRouteBranches(selectedRoute.value, atlas.value))
const selectedRouteBranch = computed(() => routeBranches.value.find(item => item.branchId === selectedRouteBranchId.value) || routeBranches.value[0] || null)
const selectedRouteTargets = computed(() => selectedRouteBranch.value ? routeTraitTargets(selectedRouteBranch.value, atlas.value) : [])
const selectedRouteRequiredTargets = computed(() => selectedRouteTargets.value.filter(item => item.required))
const resolvedDirection = computed(() => selectedRouteBranch.value?.actionType || selectedRoute.value?.actionType || (profile.value === 'character' ? characterProfile.value.defaultDirection : profile.value))
const directionProfile = computed(() => LOADOUT_DIRECTIONS[resolvedDirection.value] || LOADOUT_DIRECTIONS.normal)
const combatMode = computed(() => profile.value !== 'custom' && evidence.value.traits?.length > 0 && !!fixedSimulation.value?.finalStats)
const optimizerIntent = computed(() => profile.value === 'custom' ? 'skills' : 'auto')
const selectedSkillNames = computed(() => (props.baseLoadout?.skills || [])
  .map(skill => String(skill?.name || skill?.displayName || '').trim())
  .filter(name => name && !/^(?:未记录|未收录|unknown)/i.test(name)))
const capEvidenceSummary = computed(() => {
  const reference = fixedSimulation.value?.combatReference || {}
  return {
    normalRows: Array.isArray(reference.normalCurve) ? reference.normalCurve.length : 0,
    artsRows: Array.isArray(reference.artsCurve) ? reference.artsCurve.length : 0,
  }
})
function characterProfileNames() {
  const counts = new Map()
  const add = (name, weight = 1) => {
    const value = String(name || '').trim()
    if (value) counts.set(value, (counts.get(value) || 0) + weight)
  }
  for (const sigil of props.baseLoadout?.sigils || []) {
    add(sigil.primaryTraitName, 2)
    add(sigil.secondaryTraitName, 1)
  }
  for (const skill of props.baseLoadout?.weapon?.skills || []) add(skill.name, 1)
  for (const trait of props.baseLoadout?.weapon?.wrightstone?.traits || []) add(trait.name, 1)
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], language.value === 'en' ? 'en' : 'zh-Hans-CN'))
    .map(([name]) => name)
}
const ownerAllowedSigils = computed(() => (atlas.value.sigils || []).filter(entry => {
  if (entry?.category !== 'character_sigil' && !(entry?.allowedOwnerCodes || []).length) return true
  const owner = String(ownerCode.value || '').trim().toUpperCase()
  return !!owner && (entry.allowedOwnerCodes || []).some(item => String(item || '').trim().toUpperCase() === owner)
}))
const reachableTraitIds = computed(() => {
  const ids = new Set()
  for (const entry of ownerAllowedSigils.value) {
    if (entry.primaryTraitId) ids.add(entry.primaryTraitId)
    for (const trait of entry.secondaryTraits || []) if (trait?.internalId) ids.add(trait.internalId)
  }
  for (const item of inventory.value) {
    if (item?.primaryTraitId) ids.add(item.primaryTraitId)
    if (item?.secondaryTraitId) ids.add(item.secondaryTraitId)
  }
  return ids
})
const conditionalTraitIds = computed(() => [...new Set(ownerAllowedSigils.value
  .filter(entry => entry?.category === 'character_sigil')
  .map(entry => entry.primaryTraitId)
  .filter(Boolean))])
const availableTraits = computed(() => (atlas.value.traits || []).filter(trait => reachableTraitIds.value.has(trait.internalId)).map(trait => ({
  ...trait,
  displayName: trait.displayName || trait.internalId,
  levelHint: `Lv${trait.maxLevel || 65}`,
})))
const pendingTrait = computed(() => atlas.value.traits.find(item => item.internalId === pendingTraitId.value) || null)
const chosen = computed(() => selected.value.map((id, index) => {
  const trait = atlas.value.traits.find(item => item.internalId === id)
  if (!trait) return null
  const naturalMax = Math.max(1, Number(trait.maxLevel || 65))
  const requested = Math.max(1, Math.min(naturalMax, Number(targetLevels.value[id] || naturalMax)))
  return { ...trait, priority: index + 1, weight: 1, cap: requested, targetLevel: requested }
}).filter(Boolean))
const coverageInputsValid = computed(() => {
  if (!combatMode.value) return true
  const low = Number(coverageLow.value)
  const high = Number(coverageHigh.value)
  return Number.isFinite(low) && Number.isFinite(high) && low >= 0 && high <= 100 && low <= high
})
const canSolve = computed(() => coverageInputsValid.value
  && (combatMode.value || chosen.value.length > 0)
  && (optimizerIntent.value !== 'auto' || !!selectedRoute.value)
  && (domain.value !== 'inventory' || (props.savePath && props.charaHash)))
function isManufacturedResult(result) {
  return resultConstructionCount(result) > 0 || ['owned-first', 'catalog', 'table'].includes(String(result?.domain || ''))
}
const suppressedManufacturedCount = computed(() => {
  if (!combatMode.value || domain.value !== 'all') return 0
  const bestInventoryScore = Math.max(...results.value.filter(item => item.domain === 'inventory').map(item => Number(item?.score || 0)), -Infinity)
  if (!Number.isFinite(bestInventoryScore)) return 0
  return results.value.filter(item => isManufacturedResult(item) && Number(item?.score || 0) <= bestInventoryScore).length
})
const displayResults = computed(() => {
  const sorted = [...results.value].sort(compareDisplayResults)
  if (optimizerIntent.value === 'auto' && selectedRoute.value) return sorted.slice(0, 1)
  if (!combatMode.value || domain.value !== 'all') return sorted.slice(0, 10)
  const bestInventoryScore = Math.max(...sorted.filter(item => item.domain === 'inventory').map(item => Number(item?.score || 0)), -Infinity)
  const useful = Number.isFinite(bestInventoryScore)
    ? sorted.filter(item => !isManufacturedResult(item) || Number(item?.score || 0) > bestInventoryScore)
    : sorted
  const plan = [
    ...useful.filter(item => item.domain === 'inventory').slice(0, 7),
    ...useful.filter(item => item.domain === 'owned-first').slice(0, 1),
    ...useful.filter(item => item.domain === 'catalog').slice(0, 1),
    ...useful.filter(item => item.domain === 'table').slice(0, 1),
  ]
  const seen = new Set(plan.map(resultSignature))
  for (const item of useful) {
    if (plan.length >= 10) break
    const signature = resultSignature(item)
    if (seen.has(signature)) continue
    seen.add(signature)
    plan.push(item)
  }
  return plan.slice(0, 10)
})
const primaryResult = computed(() => {
  return displayResults.value[0] || null
})

function icon(trait) { return traitAssetIcon({ internalId: trait?.internalId, hash: trait?.hash, name: trait?.displayName }) }
function routeTraitCatalog(item) {
  return atlas.value.traits.find(trait => trait.internalId === item?.traitId) || null
}
function routeTraitName(item) {
  return routeTraitCatalog(item)?.displayName || item?.traitId || ''
}
function routeTraitIcon(item) {
  const catalog = routeTraitCatalog(item)
  return traitAssetIcon({ internalId: item?.traitId, hash: catalog?.hash, name: catalog?.displayName })
}
function routeConditionState(item) {
  const stats = fixedSimulation.value?.finalStats || {}
  if (item?.condition === 'base-attack-25000') {
    const value = Number(stats.attack || 0)
    return { conditional: true, met: value >= 25000, label: tx(`当前攻击 ${Math.round(value).toLocaleString()} / 25,000`, `Current ATK ${Math.round(value).toLocaleString()} / 25,000`) }
  }
  if (item?.condition === 'base-hp-80000') {
    const value = Number(stats.hp || 0)
    return { conditional: true, met: value >= 80000, label: tx(`当前 HP ${Math.round(value).toLocaleString()} / 80,000`, `Current HP ${Math.round(value).toLocaleString()} / 80,000`) }
  }
  if (item?.condition === 'high-hp-75') {
    const value = Number(currentHp.value || 0)
    return { conditional: true, met: value >= 75, label: tx(`当前 HP ${value}% · 需要至少 75%`, `Current HP ${value}% · requires at least 75%`) }
  }
  if (item?.condition === 'low-hp-25') {
    const value = Number(currentHp.value || 0)
    return { conditional: true, met: value <= 25, label: tx(`当前 HP ${value}% · 需要不高于 25%`, `Current HP ${value}% · requires 25% or lower`) }
  }
  if (item?.condition === 'high-hp') {
    const value = Number(currentHp.value || 0)
    return { conditional: true, met: value >= 75, label: tx(`当前 HP ${value}% · 高血时效果更强`, `Current HP ${value}% · stronger at high HP`) }
  }
  if (item?.condition === 'low-hp') {
    const value = Number(currentHp.value || 0)
    return { conditional: true, met: value <= 50, label: tx(`当前 HP ${value}% · 失血后效果提高`, `Current HP ${value}% · improves after losing HP`) }
  }
  if (item?.condition === 'endgame-quest') {
    return { conditional: true, met: true, label: tx('仅在对应毕业任务中生效', 'Only active in the matching endgame quest') }
  }
  if (item?.condition === 'stout-heart') {
    return { conditional: true, met: true, label: tx('需要霸体状态', 'Requires Stout Heart') }
  }
  return { conditional: false, met: true, label: '' }
}
function routeFinalCheckState(item) {
  const bonus = (fixedSimulation.value?.bonuses || []).find(entry => entry?.traitId === item?.traitId)
  const actual = Number(bonus?.level || 0)
  const target = Number(item?.targetLevel || 0)
  const condition = routeConditionState(item)
  return {
    actual,
    target,
    met: actual >= target && condition.met,
    sources: Array.isArray(bonus?.sources) ? bonus.sources.filter(Boolean) : [],
    condition,
  }
}
function onPendingTraitPick(trait) {
  const max = Math.max(1, Number(trait?.maxLevel || 65))
  pendingTraitLevel.value = Math.min(15, max)
}
function clampPendingTraitLevel() {
  const max = Math.max(1, Number(pendingTrait.value?.maxLevel || 65))
  pendingTraitLevel.value = Math.max(1, Math.min(max, Number(pendingTraitLevel.value) || 1))
}
function addPendingTrait() {
  if (!pendingTraitId.value) return
  clampPendingTraitLevel()
  const id = pendingTraitId.value
  if (selected.value.includes(id)) {
    setTargetLevel(id, pendingTraitLevel.value)
  } else {
    selected.value = [...selected.value, id]
    targetLevels.value = { ...targetLevels.value, [id]: pendingTraitLevel.value }
    cancelSolve()
    solved.value = false
  }
  rememberCustomTargets()
  pendingTraitId.value = ''
  pendingTraitLevel.value = 15
}
function chooseTrait(id) {
  cancelSolve()
  if (selected.value.includes(id)) {
    selected.value = selected.value.filter(item => item !== id)
    const next = { ...targetLevels.value }
    delete next[id]
    targetLevels.value = next
  } else {
    const trait = atlas.value.traits.find(item => item.internalId === id)
    selected.value = [...selected.value, id]
    targetLevels.value = { ...targetLevels.value, [id]: Math.max(1, Number(trait?.maxLevel || 15)) }
  }
  rememberCustomTargets()
  solved.value = false
}
function moveTrait(index, offset) {
  const target = index + offset
  if (target < 0 || target >= selected.value.length) return
  cancelSolve()
  const next = selected.value.slice()
  ;[next[index], next[target]] = [next[target], next[index]]
  selected.value = next
  rememberCustomTargets()
  solved.value = false
}
function rememberCustomTargets() {
  if (profile.value !== 'custom') return
  customSelected.value = selected.value.slice()
  customTargetLevels.value = { ...targetLevels.value }
}
function resultConstructionCount(result) {
  if (Number.isFinite(Number(result?.constructedCount))) return Number(result.constructedCount)
  return (result?.picked || []).filter(item => item?.source !== 'inventory').length
}
function resultOverflow(result) {
  return (result?.totals || []).reduce((sum, item) => sum + Math.max(0, Number(item.level || 0) - Number(item.effective || 0)), 0)
}
function resultCoverage(result) {
  if (combatMode.value && selectedRoute.value) {
    const missing = result?.combat?.metrics?.missingRequiredTraits || []
    const missingById = new Map(missing.map(item => [item.traitId, Number(item.missingLevel || 0)]))
    const requested = selectedRouteRequiredTargets.value.reduce((sum, item) => sum + Number(item.targetLevel || 0), 0)
    if (!requested) return 1
    const missingLevels = selectedRouteRequiredTargets.value.reduce((sum, item) => sum + Number(missingById.get(item.traitId) || 0), 0)
    return Math.max(0, Math.min(1, (requested - missingLevels) / requested))
  }
  if (combatMode.value) return 1
  const rows = targetFulfilment(result).rows
  const requested = rows.reduce((sum, item) => sum + Number(item.target || 0), 0)
  if (!requested) return 0
  return rows.reduce((sum, item) => sum + Math.min(Number(item.actual || 0), Number(item.target || 0)), 0) / requested
}
function resultGroup(result) {
  const complete = combatMode.value ? result?.combat?.valid !== false : targetFulfilment(result).complete
  if (!complete) return 2
  return resultConstructionCount(result) > 0 ? 1 : 0
}
function resultSignature(result) {
  return (result?.picked || []).map(item => `${item?.source || ''}:${item?.slotId || 0}:${item?.id || item?.sigilId || item?.name || ''}`).join('|')
}
function combatResultPriority(result) {
  return ({ inventory: 0, 'owned-first': 1, catalog: 2, table: 3 })[String(result?.domain || '')] ?? 9
}
function compareDisplayResults(left, right) {
  if (combatMode.value) {
    const validity = Number(right?.combat?.valid === true) - Number(left?.combat?.valid === true)
    if (validity) return validity
    return resultGroup(left) - resultGroup(right)
      || combatResultPriority(left) - combatResultPriority(right)
      || resultCoverage(right) - resultCoverage(left)
      || Number(right?.score || 0) - Number(left?.score || 0)
      || resultConstructionCount(left) - resultConstructionCount(right)
      || resultSignature(left).localeCompare(resultSignature(right), 'en')
  }
  const leftPriority = targetFulfilment(left)
  const rightPriority = targetFulfilment(right)
  return resultGroup(left) - resultGroup(right)
    || rightPriority.completedPrefix - leftPriority.completedPrefix
    || leftPriority.rows.reduce((order, row, index) => order || Number(rightPriority.rows[index]?.actual || 0) - Number(row.actual || 0), 0)
    || resultCoverage(right) - resultCoverage(left)
    || resultConstructionCount(left) - resultConstructionCount(right)
    || Number(left?.picked?.length || 0) - Number(right?.picked?.length || 0)
    || resultOverflow(left) - resultOverflow(right)
    || Number(right?.score || 0) - Number(left?.score || 0)
    || resultSignature(left).localeCompare(resultSignature(right), 'en')
}
function resultRankReason(result) {
  const count = resultConstructionCount(result)
  const slots = Number(result?.picked?.length || 0)
  if (combatMode.value) {
    const missing = result?.combat?.metrics?.missingRequiredTraits || []
    if (missing.length) {
      return tx(
        `固定路线尚缺 ${missing.length} 项 · 已覆盖 ${Math.round(resultCoverage(result) * 100)}% · 不会把这套标成毕业方案`,
        `${missing.length} locked route requirements are still missing · ${Math.round(resultCoverage(result) * 100)}% covered · this is not labeled a completed build`,
      )
    }
    const cap = combatCapProof(result)
    const source = count > 0
      ? tx(`复用背包并制造 ${count} 个缺口`, `reuses inventory and creates ${count} gaps`)
      : tx('只用背包已有实例', 'owned inventory only')
    return tx(
      `${cap.hitsCap ? '已触及动作上限，继续比较上限与追击收益' : '现有动作上限尚未吃满，优先补实际伤害'} · ${source} · 12 个主因子槽按固定路线排满`,
      `${cap.hitsCap ? 'reaches the action cap; compares cap and supplemental gains' : 'current action cap is not yet saturated; prioritizes real damage'} · ${source} · all 12 primary sigil slots follow the fixed route`,
    )
  }
  if (!combatMode.value && !targetFulfilment(result).complete) {
    const fulfilment = targetFulfilment(result)
    return tx(
      `已按顺序完成前 ${fulfilment.completedPrefix} 项 · 第 ${fulfilment.completedPrefix + 1} 项“${fulfilment.firstUnmet?.name || '目标'}”还缺 Lv${fulfilment.firstUnmet?.missing || 0} · 后续 ${fulfilment.waitingCount} 项不抢占前序槽位`,
      `Completed the first ${fulfilment.completedPrefix} target(s) in order · #${fulfilment.completedPrefix + 1} “${fulfilment.firstUnmet?.name || 'target'}” is short by Lv${fulfilment.firstUnmet?.missing || 0} · ${fulfilment.waitingCount} later target(s) do not displace earlier ones`,
    )
  }
  if (count > 0) {
    return tx(`全部满足 · 背包缺少 ${count} 个实例 · 确认保存时才制造`, `Complete · ${count} inventory gaps · created only after save confirmation`)
  }
  return tx(`全部满足 · 只用背包已有实例 · 使用 ${slots} 槽`, `Complete · owned inventory only · ${slots} slots`)
}
function resultGroupLabel(result) {
  return resultGroup(result) === 0
    ? tx('纯背包可达', 'Owned and Ready')
    : resultGroup(result) === 1
      ? tx('需要制造', 'Creation Required')
      : tx('部分满足', 'Partial Match')
}
function resultKey(result, index) {
  return `${result?.domain || ''}:${result?.domainRank || index + 1}:${resultSignature(result)}`
}
function isResultExpanded(result, index) {
  return index === 0 || expandedResultKeys.value.has(resultKey(result, index))
}
function toggleResult(result, index) {
  const next = new Set(expandedResultKeys.value)
  const key = resultKey(result, index)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedResultKeys.value = next
}
function previewTrait(item, index) {
  const trait = item?.traits?.[index]
  if (trait) return { name: trait.name || '', level: Number(trait.level || 0), internalId: trait.id || '' }
  if (index === 0) return {
    name: item?.primaryTraitName || '',
    level: Number(item?.primaryTraitLevel || item?.primaryLevel || 0),
    internalId: item?.primaryTraitId || '',
  }
  return {
    name: item?.secondaryTraitName || '',
    level: Number(item?.secondaryTraitLevel || item?.secondaryLevel || 0),
    internalId: item?.secondaryTraitId || '',
  }
}
function resultPreviewSlots(result) {
  const picked = (result?.picked || []).slice(0, 12)
  const usedInventory = new Set(picked.map(item => Number(item?.slotId || 0)).filter(Boolean))
  const rows = picked.map(item => ({
    item,
    retained: false,
    source: item?.source === 'inventory' && Number(item?.slotId || 0) > 0 ? `背包 #${item.slotId}` : '待制造',
  }))
  for (const base of props.baseLoadout?.sigils || []) {
    if (rows.length >= 12) break
    const slotId = Number(base?.slotId || 0)
    if (slotId && usedInventory.has(slotId)) continue
    rows.push({ item: base, retained: true, source: slotId ? `背包 #${slotId}` : '保留当前槽' })
    if (slotId) usedInventory.add(slotId)
  }
  while (rows.length < 12) rows.push({ item: null, retained: false, source: '空槽' })
  return rows
}
function previewIcon(row) {
  const trait = previewTrait(row?.item, 0)
  return traitAssetIcon({ internalId: trait.internalId, name: trait.name })
}
function setTargetLevel(id, value) {
  const trait = atlas.value.traits.find(item => item.internalId === id)
  const max = Math.max(1, Number(trait?.maxLevel || 65))
  targetLevels.value = { ...targetLevels.value, [id]: Math.max(1, Math.min(max, Number(value) || 1)) }
  rememberCustomTargets()
  cancelSolve()
  solved.value = false
}
function selectOptimizerIntent(intent) {
  if (intent === 'skills') {
    applyProfile('custom')
    selected.value = customSelected.value.slice()
    targetLevels.value = { ...customTargetLevels.value }
    solved.value = false
    return
  }
  rememberCustomTargets()
  applyProfile('character')
  if (characterRoutes.value.length && !selectedRouteId.value) selectedRouteId.value = characterRoutes.value[0].id
}
function selectCharacterRoute(route) {
  if (!route?.id) return
  cancelSolve()
  selectedRouteId.value = route.id
  selectedRouteBranchId.value = 'graduation'
  profile.value = 'character'
  coverage.value = 100
  coverageLow.value = 85
  coverageHigh.value = 100
  solved.value = false
}
function selectRouteBranch(branch) {
  if (!branch?.branchId) return
  cancelSolve()
  selectedRouteBranchId.value = branch.branchId
  profile.value = 'character'
  solved.value = false
}
function applyProfile(value) {
  cancelSolve()
  profile.value = value
  if (value === 'custom') return
  const configured = value === 'character' ? directionProfile.value.traitIds : profiles[value].traitIds
  const inherited = value === 'character' ? characterProfileNames().map(name => atlas.value.traits.find(item => item.displayName === name || item.displayName.includes(name))?.internalId).filter(Boolean) : []
  const ids = [...configured, ...inherited].filter(id => atlas.value.traits.some(item => item.internalId === id))
  selected.value = [...new Set(ids)]
  coverage.value = Math.round(Number(directionProfile.value.coverage || 1) * 100)
  coverageLow.value = Math.max(0, coverage.value - 15)
  coverageHigh.value = Math.min(100, coverage.value + 15)
  actionRate.value = Number(directionProfile.value.actionRate || 1)
  solved.value = false
}
function applyPendingTarget(target) {
  const ids = (target?.traitIds || []).filter(id => atlas.value.traits.some(item => item.internalId === id))
  if (!ids.length) return
  cancelSolve()
  profile.value = 'custom'
  selected.value = [...new Set(ids)]
  targetLevels.value = Object.fromEntries(selected.value.map(id => {
    const trait = atlas.value.traits.find(item => item.internalId === id)
    return [id, Math.max(1, Number(trait?.maxLevel || 15))]
  }))
  customSelected.value = selected.value.slice()
  customTargetLevels.value = { ...targetLevels.value }
  solved.value = false
}
function applyResult(result) {
  if (!result?.picked?.length) return
  emit('apply', {
    result: JSON.parse(JSON.stringify(result)),
    domain: result.domain || domain.value,
    targetUnitId: Number(props.baseLoadout?.unitId || 0),
    profile: profile.value,
    characterRouteId: selectedRoute.value?.id || '',
    characterRouteBranchId: selectedRouteBranch.value?.branchId || '',
    characterRouteBranchVersion: selectedRouteBranch.value ? LOADOUT_ROUTE_BRANCH_VERSION : '',
    requestId: Date.now(),
  })
}
function targetFulfilment(result) {
  const totals = new Map((result?.totals || []).map(item => [item.name, item]))
  let blocked = false
  const rows = chosen.value.map(item => {
    const actual = Number(totals.get(item.displayName)?.effective || 0)
    const met = actual >= item.targetLevel
    const state = blocked ? 'waiting' : met ? 'met' : 'blocked'
    if (!met) blocked = true
    return {
      name: item.displayName,
      actual,
      target: item.targetLevel,
      missing: Math.max(0, item.targetLevel - actual),
      met,
      state,
    }
  })
  const firstGap = rows.findIndex(item => !item.met)
  const completedPrefix = firstGap < 0 ? rows.length : firstGap
  return {
    rows,
    complete: rows.length > 0 && rows.every(item => item.met),
    completedPrefix,
    firstUnmet: rows[completedPrefix] || null,
    waitingCount: rows.slice(completedPrefix + 1).length,
  }
}
function routeFulfilment(result) {
  if (!selectedRoute.value) return { rows: [], complete: true }
  const totals = new Map((result?.totals || []).filter(item => item?.traitId).map(item => [item.traitId, item]))
  const rows = selectedRouteRequiredTargets.value.map(item => {
    const actual = Number(totals.get(item.traitId)?.effective || 0)
    const target = Number(item.targetLevel || 0)
    return {
      ...item,
      name: routeTraitName(item),
      actual,
      target,
      met: actual >= target,
      condition: routeConditionState(item),
    }
  })
  return { rows, complete: rows.length > 0 && rows.every(item => item.met) }
}
function localizedResultTotals(result) {
  return (result?.totals || []).map(item => {
    const catalogTrait = atlas.value.traits.find(trait => trait.internalId === item?.traitId)
    const displayName = String(catalogTrait?.displayName || item?.name || '').trim()
    if (!displayName || /^(?:MEMORY|SKILL|TRAIT|SIGIL|ABILITY|WEAPON|SUMMON)_[A-Z0-9_]+$/i.test(displayName)) return null
    return { ...item, displayName }
  }).filter(Boolean)
}
function visibleResultTotals(result) {
  return localizedResultTotals(result)
    .filter(item => Number(item?.effective || 0) > 0)
    .sort((left, right) => Number(right.effective || 0) - Number(left.effective || 0) || left.displayName.localeCompare(right.displayName, language.value === 'en' ? 'en' : 'zh-Hans-CN'))
    .slice(0, combatMode.value ? 8 : 4)
}
function combatCapProof(result) {
  const metrics = result?.combat?.metrics || {}
  const effectiveCap = Math.max(0, Number(metrics.effectiveCap || 0))
  const uncappedDamage = Math.max(0, Number(metrics.uncappedDamage || 0))
  const utilization = effectiveCap > 0 ? Math.min(100, uncappedDamage / effectiveCap * 100) : 0
  const action = String(result?.explanation?.formulaEvidence?.actionCapEvidence || result?.explanation?.evidenceSource || '')
  return {
    bonus: Number(metrics.actionCapBonus || 0),
    effectiveCap,
    uncappedDamage,
    offenseGap: Math.max(0, effectiveCap - uncappedDamage),
    utilization,
    hitsCap: effectiveCap > 0 && uncappedDamage >= effectiveCap,
    evidence: action.includes('table-exact') ? tx('动作表精确上限', 'Exact action-table cap') : tx('全局基线上限估算', 'Global-baseline cap estimate'),
  }
}
function resultCharacterTitle(index) {
  const name = language.value === 'en'
    ? (characterIdentity.value?.nameEn || props.charaName || tx('Current Character', 'Current Character'))
    : (characterIdentity.value?.nameZh || props.charaName || tx('当前角色', 'Current Character'))
  const direction = language.value === 'en' ? directionProfile.value.en : directionProfile.value.zh
  const routeName = selectedRouteBranch.value ? (language.value === 'en' ? selectedRouteBranch.value.nameEn : selectedRouteBranch.value.nameZh) : ''
  return routeName
    ? `${name} · ${routeName}`
    : tx(`${name} · ${direction}向 · 方案 ${index + 1}`, `${name} · ${direction} · Plan ${index + 1}`)
}
function combatDefenseProof(result) {
  const metrics = result?.combat?.metrics || {}
  const hp = Math.max(0, Number(metrics.hp || 0))
  const defense = Math.max(0, Number(metrics.defense || 0))
  const reducedHit = Math.max(0, Number(metrics.incomingDamage || 0))
  const survivableHits = reducedHit > 0 ? Math.floor(hp / reducedHit) : 0
  const requiredHp = Math.max(0, Number(minimumHp.value || 0))
  const requiredDefense = Math.max(0, Number(minimumDefense.value || 0))
  const requiredHits = Math.max(0, Number(surviveHits.value || 0))
  const checks = [
    { active: requiredHp > 0, met: hp >= requiredHp, zh: `HP 至少 ${requiredHp.toLocaleString()}`, en: `HP at least ${requiredHp.toLocaleString()}` },
    { active: requiredDefense > 0, met: defense >= requiredDefense, zh: `减伤至少 ${requiredDefense}%`, en: `Defense at least ${requiredDefense}%` },
    { active: requiredHits > 0, met: survivableHits >= requiredHits, zh: `承受至少 ${requiredHits} 次`, en: `Survive at least ${requiredHits} hits` },
  ].filter(check => check.active)
  return {
    hp,
    defense,
    baseHit: Math.max(0, Number(incomingDamage.value || 0)),
    reducedHit,
    survivableHits,
    checks,
    valid: checks.every(check => check.met),
  }
}
function currentActionLabel() {
  const action = LOADOUT_ACTION_TYPES[directionProfile.value.actionType || 'normal']
  return action ? (language.value === 'en' ? action.en : action.zh) : ''
}
async function revealResults() {
  await nextTick()
  resultRegion.value?.scrollIntoView?.({ block: 'nearest' })
}
function cancelSolve() {
  solveGeneration++
  solveWorker?.terminate()
  solveWorker = null
  rejectSolve?.(new Error('optimizer.cancelled'))
  rejectSolve = null
  solving.value = false
}
async function loadInventory() {
  const key = `${props.savePath}|${props.charaHash}|${Number(props.baseLoadout?.unitId || 0)}`
  if (key === loadedInventoryKey.value && inventorySnapshot.value) return
  inventory.value = []
  inventorySnapshot.value = null
  ownerCode.value = ''
  loadedInventoryKey.value = ''
  if (!props.savePath || !props.charaHash) return
  const [context, snapshot] = await Promise.all([
    LoadoutEditContext(props.savePath, props.charaHash),
    LoadoutOptimizerInventorySnapshot(props.savePath, props.charaHash, Number(props.baseLoadout?.unitId || 0)),
  ])
  inventory.value = context?.sigils || []
  inventorySnapshot.value = snapshot || null
  ownerCode.value = String(context?.ownerCode || '')
  loadedInventoryKey.value = key
}
function baseLoadoutInputs() {
  return {
    weaponSlotId: Number(props.baseLoadout?.weaponSlotId || props.baseLoadout?.weaponSlotID || 0),
    mastery: (props.baseLoadout?.mastery || []).map(item => String(item?.hash || item || '')).filter(Boolean),
  }
}
async function loadCombatContext() {
  statContext.value = null
  fixedSimulation.value = null
  if (!props.savePath || !props.charaHash || !props.baseLoadout) return
  const context = await LoadoutStatContext(props.savePath, props.charaHash)
  const input = baseLoadoutInputs()
  const draftSummons = Array.isArray(props.baseLoadout?.summonSlotIds) ? props.baseLoadout.summonSlotIds.map(Number).filter(Boolean).slice(0, 4) : []
  const simulation = await LoadoutSimulateBuild(props.savePath, props.charaHash, input.weaponSlotId, [], [], input.mastery, draftSummons.length === 4 ? draftSummons : (context?.equippedSummonSlotIds || []))
  statContext.value = context
  fixedSimulation.value = simulation
}
async function load() {
  if (loading.value) return
  loading.value = true
  try {
    const [loadedAtlas, loadedEvidence] = await Promise.all([sigilAtlasStore.load(language.value), LoadoutOptimizerEvidence()])
    atlas.value = loadedAtlas
    evidence.value = loadedEvidence || { traits: [] }
    applyProfile(profile.value)
    applyPendingTarget(props.pendingTarget)
    await Promise.all([loadInventory(), loadCombatContext()])
  } catch (error) {
    emit('status', String(error), 'error')
  } finally {
    loading.value = false
  }
}
function exactCurveCap(curve, rate) {
  const rows = Array.isArray(curve) ? curve : []
  const exact = rows.find(item => Math.abs(Number(item.attackRate) - rate) <= 1e-9)
  return exact ? Number(exact.damageCap || 0) : 0
}
function fixedNonPanelTotals(simulation) {
  return (simulation?.dynamicTotals || []).filter(total => {
    const label = String(total?.label || '')
    return label.includes('造成的伤害') || label.includes('冷却时间')
  })
}
function combatCandidateTargets() {
  const atlasById = new Map((atlas.value.traits || []).map(item => [item.internalId, item]))
  const targets = new Map()
  for (const curve of evidence.value.traits || []) {
    if (!reachableTraitIds.value.has(curve.traitId)) continue
    const trait = atlasById.get(curve.traitId)
    if (trait) targets.set(curve.traitId, { traitId: curve.traitId, name: trait.displayName, weight: 1, cap: curve.maxLevel || trait.maxLevel || 65 })
  }
  for (const target of selectedRouteTargets.value) {
    if (!reachableTraitIds.value.has(target.traitId)) continue
    targets.set(target.traitId, target)
  }
  return [...targets.values()]
}
function combatScenario() {
  const simulation = fixedSimulation.value
  const final = simulation?.finalStats || {}
  const reference = simulation?.combatReference || {}
  const action = directionProfile.value.actionType || 'normal'
  const rate = Math.max(0.1, Number(actionRate.value || 1))
  const curve = action === 'ability' ? reference.artsCurve : reference.normalCurve
  const rawCap = action === 'chain'
    ? Number(reference.damageCalculate?.chainBurstDamageLimit || 9999999)
    : action === 'sba'
      ? Number(reference.damageCalculate?.atkTypeDamageLimit_SpArts || 19999)
      : exactCurveCap(curve, rate) || Number(reference.damageCalculate?.[action === 'ability' ? 'atkTypeDamageLimit_Ability' : 'atkTypeDamageLimit_Normal'] || 9999)
  const exactActionCap = (action === 'normal' || action === 'ability') && exactCurveCap(curve, rate) > 0
  const capPercent = action === 'ability' ? Number(final.abilityDamageCap || 0) : action === 'sba' ? Number(final.skyboundDamageCap || 0) : action === 'chain' ? Number(final.chainDamageCap || 0) : Number(final.normalDamageCap || 0)
  return {
    mode: 'combat', evidence: evidence.value, direction: resolvedDirection.value, actionType: action,
    directionProfile: directionProfile.value, actionRate: rate, coverage: Number(coverage.value) / 100,
    conditionalTraitIds: conditionalTraitIds.value,
    coverageRange: [Math.min(Number(coverageLow.value), Number(coverageHigh.value)) / 100, Math.max(Number(coverageLow.value), Number(coverageHigh.value)) / 100],
    conditionalCurves: reference.conditionalCurves || {}, actionCapEvidence: exactActionCap ? 'table-exact' : 'global-baseline',
    currentHpRatio: Number(currentHp.value) / 100, odStage: Number(odStage.value), berserk: berserk.value,
    disableAttackDefenseInOD: true,
    minimumHp: Number(minimumHp.value || 0),
    minimumDefense: Number(minimumDefense.value || 0),
    incomingDamage: Number(incomingDamage.value || 100000),
    surviveHits: Number(surviveHits.value || 0),
    baseStats: { attack: Number(final.attack || 1), hp: Number(final.hp || 1), critRate: Number(final.critRate || 0) },
    baseDamageCap: rawCap * (1 + capPercent / 100), comparedCapPercent: capPercent,
    baseUncappedDamage: Math.max(1, Number(final.attack || 1) * rate),
    criticalDamageBonus: Number(reference.damageCalculate?.criticalDamageUpperRate || 20),
    fixedTotals: fixedNonPanelTotals(simulation),
    fixedBonuses: (simulation?.bonuses || []).map(item => ({ traitId: item.traitId, level: item.level })),
    fixedDefenseZones: final.defenseModel?.zones || [],
    ownerCode: ownerCode.value || characterProfile.value.ownerCode,
    selectedSkills: selectedSkillNames.value,
    characterResearch: characterProfile.value.research,
    formulaDataVersion: evidence.value.dataVersion, characterProfileVersion: LOADOUT_CHARACTER_PROFILE_VERSION,
  }
}
function inventoryCombatScenario() {
  const scenario = combatScenario()
  const base = inventorySnapshot.value?.baseStats || {}
  const capPercent = scenario.actionType === 'ability' ? Number(base.abilityDamageCap || 0)
    : scenario.actionType === 'sba' ? Number(base.skyboundDamageCap || 0)
      : scenario.actionType === 'chain' ? Number(base.chainDamageCap || 0)
        : Number(base.normalDamageCap || 0)
  const rawCap = Number(scenario.baseDamageCap || 1) / Math.max(1, 1 + Number(scenario.comparedCapPercent || 0) / 100)
  return {
    ...scenario,
    baseStats: { attack: Number(base.attack || 1), hp: Number(base.hp || 1), critRate: Number(base.critRate || 0) },
    baseDamageCap: rawCap * (1 + capPercent / 100),
    baseUncappedDamage: Math.max(1, Number(base.attack || 1) * Number(scenario.actionRate || 1)),
    fixedTotals: inventorySnapshot.value?.baseFixedTotals || [],
    fixedBonuses: inventorySnapshot.value?.baseFixedBonuses || [],
    fixedDefenseZones: inventorySnapshot.value?.baseDefenseZones || [],
  }
}
async function solve() {
  if (!canSolve.value || loading.value || solving.value) return
  solveError.value = ''
  solved.value = false
  results.value = []
  solving.value = true
  const generation = ++solveGeneration
  try {
    if (combatMode.value || domain.value === 'inventory' || domain.value === 'all') await loadInventory()
    if (generation !== solveGeneration) return
    const routeMode = optimizerIntent.value === 'auto' && !!selectedRoute.value
    const targets = routeMode
      ? selectedRouteTargets.value
      : combatMode.value
        ? combatCandidateTargets()
      : chosen.value.map(item => ({ name: item.displayName, weight: item.weight, cap: item.targetLevel }))
    const inventoryCandidates = buildInventoryCandidates(inventory.value, targets, atlas.value)
    const catalogCandidates = buildCatalogCandidates(atlas.value, targets, ownerCode.value)
    const tableCandidates = buildTableExactCandidates(atlas.value, targets, ownerCode.value, catalogCandidates)
    const equippedSlotIds = new Set((props.baseLoadout?.sigils || []).map(item => Number(item?.slotId || 0)).filter(Boolean))
    const retainedCandidates = inventoryCandidates
      .filter(item => equippedSlotIds.has(Number(item?.slotId || 0)))
      .map(item => ({ ...item, retained: true }))
    const retainedBySlot = new Map(retainedCandidates.map(item => [Number(item.slotId), item]))
    const baseSigils = (props.baseLoadout?.sigils || []).map(item => retainedBySlot.get(Number(item?.slotId || 0)) || {
      id: item.hash || item.name,
      slotId: Number(item.slotId || 0),
      source: 'inventory',
      name: item.name || '因子',
      traits: [],
      retained: true,
    })
    const includeRetained = rows => {
      const seen = new Set()
      return [...retainedCandidates, ...(rows || [])].filter(item => {
        const key = String(item?.id || `${item?.source || ''}:${item?.slotId || 0}:${item?.hash || ''}`)
        if (seen.has(key)) return false
        seen.add(key)
        return true
      })
    }
    const catalogSolveCandidates = includeRetained(catalogCandidates)
    const tableSolveCandidates = includeRetained(tableCandidates)
    const candidates = domain.value === 'inventory' ? inventoryCandidates : domain.value === 'catalog' ? catalogSolveCandidates : domain.value === 'table' ? tableSolveCandidates : null
    solveWorker?.terminate()
    const workers = []
    solveWorker = {
      terminate() {
        for (const worker of workers.splice(0)) worker.terminate()
      },
    }
    let resolvedScenario = null
    const scenario = {
      character: props.charaName,
      direction: resolvedDirection.value,
      directionProfile: directionProfile.value,
      domain: domain.value,
      scenarioVersion: LOADOUT_SCENARIO_VERSION,
      baseSigils,
      characterRouteId: selectedRoute.value?.id || '',
      characterRouteVersion: selectedRoute.value ? LOADOUT_CHARACTER_ROUTE_VERSION : '',
      characterRouteBranchId: selectedRouteBranch.value?.branchId || '',
      characterRouteBranchVersion: selectedRouteBranch.value ? LOADOUT_ROUTE_BRANCH_VERSION : '',
      requiredTraitTargets: routeMode ? selectedRouteRequiredTargets.value.map(item => ({ traitId: item.traitId, targetLevel: item.targetLevel })) : [],
      ...(combatMode.value ? combatScenario() : {}),
    }
    const scoringScenario = combatMode.value && inventorySnapshot.value
      ? { ...scenario, ...inventoryCombatScenario(), targets, domain: domain.value }
      : scenario
    resolvedScenario = scoringScenario
    const runWorker = (payload, modes = {}) => new Promise((resolve, reject) => {
      const worker = new Worker(new URL('../loadoutOptimizer.worker.js', import.meta.url), { type: 'module' })
      workers.push(worker)
      worker.addEventListener('message', event => {
        if (event.data?.id !== generation) return
        if (event.data?.error) reject(new Error(event.data.error))
        else resolve(event.data?.results || [])
      }, { once: true })
      worker.addEventListener('error', event => {
        reject(event.error || new Error(event.message || 'optimizer worker failed'))
      }, { once: true })
      worker.postMessage(createOptimizerWorkerMessage(generation, payload, false, modes))
    })
    const resolvedResults = await new Promise((resolve, reject) => {
      rejectSolve = reject
      if (routeMode) {
        runWorker({
          inventoryCandidates,
          catalogCandidates: domain.value === 'inventory' ? [] : catalogCandidates,
          targets: selectedRouteRequiredTargets.value,
          slotCount: 12,
          scenario: { ...scoringScenario, domain: 'owned-first' },
        }, { solveFixedRoute: true }).then(resolve, reject)
        return
      }
      if (domain.value !== 'all') {
        runWorker({ candidates, targets, slotCount: 12, limit: routeMode ? 1 : 10, scenario: scoringScenario }).then(resolve, reject)
        return
      }
      const domains = {
        ...(combatMode.value ? {} : { 'owned-first': [...inventoryCandidates, ...catalogCandidates] }),
        table: tableSolveCandidates,
        catalog: catalogSolveCandidates,
        inventory: inventoryCandidates,
      }
      Promise.all(Object.entries(domains).map(([domainKey, domainCandidates]) => runWorker({
        candidates: domainCandidates,
        targets,
        slotCount: 12,
        limit: routeMode ? 1 : 6,
        scenario: { ...scoringScenario, domain: domainKey },
      }).then(rows => rows.map((row, index) => ({ ...row, domain: domainKey, domainRank: index + 1 }))))).then(
        batches => resolve(batches.flat()),
        reject,
      )
    })
    rejectSolve = null
    if (generation !== solveGeneration) return
    if (routeMode) {
      results.value = resolvedResults
    } else if (domain.value === 'all' && combatMode.value) {
      const theoretical = resolvedResults.find(item => item.domain === 'catalog' && Number(item.domainRank || 1) === 1)
        || resolvedResults.find(item => item.domain === 'table' && Number(item.domainRank || 1) === 1)
      const ownedFirst = synthesizeOwnedFirstSuggestion({
        desired: theoretical,
        inventoryCandidates,
        targets,
        scenario: resolvedScenario,
      })
      results.value = ownedFirst
        ? [ownedFirst, ...resolvedResults.filter(item => item.domain !== 'owned-first')]
        : resolvedResults
    } else {
      results.value = resolvedResults
    }
    expandedResultKeys.value = new Set()
    solved.value = true
    await revealResults()
  } catch (error) {
    if (error?.message !== 'optimizer.cancelled') {
      solveError.value = tx('方案没有生成成功，请重试；如果仍然失败，请切换“只用当前背包”后再计算。', 'The plans could not be generated. Try again, or switch to “Current Save Only” if the problem continues.')
      solved.value = true
      await revealResults()
      emit('status', solveError.value, 'error')
    }
  } finally {
    if (generation === solveGeneration) {
      solveWorker?.terminate()
      solveWorker = null
      rejectSolve = null
      solving.value = false
    }
  }
}
watch(() => [props.savePath, props.charaHash, props.baseLoadout?.unitId], () => {
  loadedInventoryKey.value = ''
  return Promise.all([loadInventory(), loadCombatContext()])
})
watch(() => [props.savePath, props.charaHash], cancelSolve)
watch(() => props.charaHash, () => {
  selectedRouteId.value = ''
  selectedRouteBranchId.value = 'graduation'
  solved.value = false
})
watch(() => props.baseLoadout?.unitId, () => { applyProfile('character'); cancelSolve(); solved.value = false })
watch(() => props.pendingTarget?.requestId, () => applyPendingTarget(props.pendingTarget))
watch(domain, () => { cancelSolve(); solved.value = false })
onMounted(load)
onBeforeUnmount(cancelSolve)
</script>

<template>
  <section class="optimizer-page" :class="{ 'is-embedded': embedded }" :aria-label="tx('配装优化建议', 'Loadout Optimization Suggestions')">
    <header v-if="!embedded" class="optimizer-heading">
      <div><small>{{ evidence.dataVersion || atlas.dataVersion || 'GBFR 2.0.2' }} · {{ tx('只读分析', 'Read-Only Analysis') }}</small><h2>{{ tx('配装优化建议', 'Loadout Optimization Suggestions') }}</h2><p>{{ tx('角色模式先选择逐帧核对的固定路线，再结合当前背包给出一套最佳可达方案；自定义模式按你填写的技能等级精确补齐。', 'Character mode starts from a frame-verified fixed route and returns its single best reachable plan from the current inventory. Custom mode fills the exact skill levels you enter.') }}</p></div>
      <span class="optimizer-character">{{ charaName || tx('未选择角色', 'No Character Selected') }}<small v-if="baseLoadout">{{ baseLoadout.name || tx('当前配装', 'Current Loadout') }}</small></span>
    </header>

    <div class="optimizer-setup">
      <section :class="['optimizer-controls', { 'ui-card': !embedded, 'is-flat': !embedded }]">
        <div class="optimizer-intent" role="tablist" :aria-label="tx('智能配装方式', 'Smart Loadout Method')">
          <button type="button" role="tab" :aria-selected="optimizerIntent === 'auto'" :class="{ active: optimizerIntent === 'auto' }" @click="selectOptimizerIntent('auto')">
            {{ tx('自动推荐', 'Auto Recommend') }}
          </button>
          <button type="button" role="tab" :aria-selected="optimizerIntent === 'skills'" :class="{ active: optimizerIntent === 'skills' }" @click="selectOptimizerIntent('skills')">
            {{ tx('按技能目标配装', 'Build by Skill Targets') }}
          </button>
        </div>

        <div v-if="optimizerIntent === 'skills'" class="skill-target-workflow">
          <div class="skill-target-entry" aria-label="添加技能目标">
            <label class="target-skill-field"><span>{{ tx('目标技能', 'Target Skill') }}</span><CatalogSelect v-model="pendingTraitId" :options="availableTraits" :icon-resolver="icon" detail-key="levelHint" :placeholder="tx('搜索并选择技能', 'Search and choose a skill')" :search-placeholder="tx('输入技能名称、拼音或 ID', 'Enter a skill name or ID')" @pick="onPendingTraitPick" /></label>
            <label class="target-level-field"><span>{{ tx('目标等级', 'Target Level') }}</span><input v-model.number="pendingTraitLevel" class="ui-input" type="number" min="1" :max="pendingTrait?.maxLevel || 65" @change="clampPendingTraitLevel" /></label>
            <button type="button" class="add-target-button ui-btn is-primary" :disabled="!pendingTraitId" @click="addPendingTrait">{{ selected.includes(pendingTraitId) ? tx('更新目标', 'Update Target') : tx('加入目标', 'Add Target') }}</button>
          </div>
          <div class="chosen-traits">
            <div class="chosen-order-copy"><small>{{ tx('可添加任意数量；从 #1 开始依次满足。12 槽放不下时会停在首个缺口，并说明后续目标。', 'Add as many as needed. Targets are fulfilled from #1 onward; if 12 slots are insufficient, the first gap and all later targets are explained.') }}</small><b>{{ chosen.length }} {{ tx('项目标', 'targets') }}</b></div>
            <div>
              <article v-for="(trait, index) in chosen" :key="trait.internalId" class="chosen-trait">
                <img v-if="icon(trait)" :src="icon(trait)" alt="" />
                <b>#{{ index + 1 }}</b>
                <span>{{ trait.displayName }}</span>
                <label><small>Lv</small><input :value="trait.targetLevel" class="ui-input" type="number" min="1" :max="trait.maxLevel || 65" @change="setTargetLevel(trait.internalId, $event.target.value)" /></label>
                <span class="chosen-order-actions"><button type="button" :disabled="index === 0" :aria-label="tx(`提高${trait.displayName}的优先级`, `Raise priority for ${trait.displayName}`)" @click="moveTrait(index, -1)">↑</button><button type="button" :disabled="index === chosen.length - 1" :aria-label="tx(`降低${trait.displayName}的优先级`, `Lower priority for ${trait.displayName}`)" @click="moveTrait(index, 1)">↓</button></span>
                <button type="button" :aria-label="tx(`移除${trait.displayName}`, `Remove ${trait.displayName}`)" @click="chooseTrait(trait.internalId)">×</button>
              </article>
              <span v-if="!chosen.length" class="empty-choice">{{ tx('先选择一个技能和目标等级，再加入目标。', 'Choose a skill and target level, then add it here.') }}</span>
            </div>
          </div>
        </div>

        <div v-else class="auto-recommend-workflow">
          <section v-if="characterRoutes.length && selectedRoute" class="character-route-planner" aria-label="角色固定配装路线">
            <header>
              <div><strong>{{ tx('先选毕业母路线，再选实战方向', 'Choose a Graduation Route, Then a Direction') }}</strong><small>{{ tx('角色专属核心保持不动，只替换这条毕业路线里的弹性槽；每次都会写清换入、换出和触发代价。', 'Character-specific cores stay locked while only flexible graduation slots are replaced, with every swap and condition shown.') }}</small></div>
              <b>{{ characterRoutes.length }} {{ tx('条已核对路线', 'reviewed route(s)') }}</b>
            </header>
            <div class="character-route-tabs ui-seg">
              <button v-for="route in characterRoutes" :key="route.id" type="button" class="ui-seg-btn" :class="{ 'is-on': selectedRoute.id === route.id }" @click="selectCharacterRoute(route)">{{ language === 'en' ? route.nameEn : route.nameZh }}</button>
            </div>
            <div class="route-branch-picker">
              <div class="route-branch-heading">
                <span><strong>{{ tx('路线方向', 'Route Direction') }}</strong><small>{{ tx('都从上面的毕业配装发散，不是另一套通用模板', 'Every direction branches from the graduation build above, not a separate generic template') }}</small></span>
                <b v-if="selectedRouteBranch">{{ tx(`保留 ${selectedRouteBranch.preservedSlots} 槽 · 替换 ${selectedRouteBranch.replacedSlots} 槽`, `${selectedRouteBranch.preservedSlots} kept · ${selectedRouteBranch.replacedSlots} replaced`) }}</b>
              </div>
              <div class="character-route-tabs route-branch-tabs ui-seg" role="tablist" :aria-label="tx('毕业配装方向', 'Graduation build directions')">
                <button v-for="branch in routeBranches" :key="branch.branchId" type="button" class="ui-seg-btn" :class="{ 'is-on': selectedRouteBranch?.branchId === branch.branchId }" role="tab" :aria-selected="selectedRouteBranch?.branchId === branch.branchId" @click="selectRouteBranch(branch)">{{ language === 'en' ? branch.branchNameEn : branch.branchNameZh }}</button>
              </div>
            </div>
            <article class="selected-route-card">
              <div class="selected-route-copy">
                <strong>{{ language === 'en' ? selectedRouteBranch?.nameEn : selectedRouteBranch?.nameZh }}</strong>
                <p>{{ language === 'en' ? selectedRouteBranch?.branchSummaryEn : selectedRouteBranch?.branchSummaryZh }}</p>
                <small v-if="selectedRouteBranch?.derived">{{ tx('这是依据内置 2.0.2 词条效果从已核对毕业路线推导出的方向分支，不冒充视频原装配。', 'This direction is derived from the reviewed graduation route using built-in 2.0.2 trait effects; it is not presented as the video’s exact build.') }}</small>
                <div v-if="selectedRouteBranch?.derived" class="route-base-summary">
                  <b>{{ tx('毕业母路线', 'Graduation Base') }}</b>
                  <span>{{ language === 'en' ? selectedRoute.summaryEn : selectedRoute.summaryZh }}</span>
                </div>
              </div>
              <div v-if="selectedRouteBranch?.replacements?.length" class="route-swap-summary">
                <strong>{{ tx('本方向改了什么', 'What This Direction Changes') }}</strong>
                <div>
                  <span v-for="item in selectedRouteBranch.replacements" :key="`${item.traitId}:${item.removed}:${item.added}`" :class="{ added: item.added, removed: item.removed }">
                    <b>{{ routeTraitName(item) }}</b>
                    <small v-if="item.removed">−{{ item.removed }} {{ tx('槽', 'slot(s)') }}</small>
                    <small v-if="item.added">+{{ item.added }} {{ tx('槽', 'slot(s)') }}</small>
                  </span>
                </div>
              </div>
              <div class="route-requirements">
                <span v-for="item in selectedRouteBranch?.required" :key="`${item.traitId}:${item.sigilId || ''}`" class="route-trait" :class="{ 'is-branch-added': item.origin === 'branch', 'condition-unmet': routeConditionState(item).conditional && !routeConditionState(item).met }">
                  <img v-if="routeTraitIcon(item)" :src="routeTraitIcon(item)" alt="" />
                  <span><b>{{ routeTraitName(item) }} <em v-if="item.slotCount > 1">×{{ item.slotCount }}</em><i v-if="item.origin === 'branch'">{{ tx('方向换入', 'Branch') }}</i></b><small>Lv{{ item.targetLevel }} · {{ language === 'en' ? item.reasonEn : item.reasonZh }}</small><em v-if="routeConditionState(item).conditional">{{ routeConditionState(item).label }}</em></span>
                </span>
              </div>
              <details v-if="selectedRouteBranch?.finalChecks?.length" class="route-alternatives route-final-checks">
                <summary>{{ tx('固定装备与最终技能检查', 'Fixed-Source & Final Skill Checks') }}</summary>
                <p>{{ tx('这些技能可以来自当前武器、祝福、召唤石或专精；只核对最终等级，不会为了补它们擅自改动固定装备，也不会强塞进 12 个因子槽。', 'These skills may come from the current weapon, wrightstone, summons, or mastery. They are checked as final totals and are never forced into the 12 sigil slots or used to change fixed equipment.') }}</p>
                <span v-for="item in selectedRouteBranch.finalChecks" :key="item.traitId" :class="{ 'condition-unmet': !routeFinalCheckState(item).met, 'condition-met': routeFinalCheckState(item).met }">
                  <b>{{ routeTraitName(item) }} <em>{{ tx(`当前固定来源 Lv${routeFinalCheckState(item).actual} / 目标 Lv${routeFinalCheckState(item).target}`, `Current fixed sources Lv${routeFinalCheckState(item).actual} / target Lv${routeFinalCheckState(item).target}`) }}</em></b>
                  <small>{{ language === 'en' ? item.reasonEn : item.reasonZh }}</small>
                  <small v-if="routeFinalCheckState(item).sources.length">{{ tx('来源：', 'Sources: ') }}{{ routeFinalCheckState(item).sources.join('、') }}</small>
                  <em>{{ routeFinalCheckState(item).met ? tx('当前固定装备已满足', 'Met by current fixed equipment') : tx('当前固定来源不足；不会把这项伪装成因子槽', 'Current fixed sources are short; this is not disguised as a sigil-slot requirement') }}</em>
                  <em v-if="routeFinalCheckState(item).condition.conditional">{{ routeFinalCheckState(item).condition.label }}</em>
                </span>
              </details>
              <details v-if="selectedRouteBranch?.optional?.length" class="route-alternatives">
                <summary>{{ tx('可用副词条与替代项', 'Optional Secondaries & Alternatives') }}</summary>
                <span v-for="item in selectedRouteBranch.optional" :key="item.traitId"><b>{{ routeTraitName(item) }} Lv{{ item.targetLevel }}</b><small>{{ language === 'en' ? item.reasonEn : item.reasonZh }}</small></span>
              </details>
              <footer>
                <span>{{ tx('路线证据', 'Route Evidence') }}</span>
                <a v-for="item in selectedRoute.sources" :key="item.url" :href="item.url" target="_blank" rel="noreferrer">{{ language === 'en' ? item.titleEn : item.titleZh }}</a>
                <small>{{ tx('社区资料只确定玩法路线；因子壳、等级和可制造性仍由内置 2.0.2 表校验。', 'Community sources define the play route; sigil shells, levels, and constructibility are still validated by the built-in 2.0.2 tables.') }}</small>
              </footer>
            </article>
          </section>
          <div v-else class="optimizer-control-group">
            <small>{{ tx('当前角色路线还在核对', 'This Character Route Is Still Under Review') }}</small>
            <p class="optimizer-context-warning">{{ tx('暂时只开放按技能目标配装；没有当前版本社区与本地表共同证据时，不生成“毕业方案”。', 'Only target-skill planning is available for now. A “graduation build” is not generated without matching current-version community and local-table evidence.') }}</p>
          </div>
          <section class="character-research-card" aria-label="当前角色配装证据">
            <header>
              <div><strong>{{ language === 'en' ? (characterIdentity?.nameEn || charaName) : (characterIdentity?.nameZh || charaName) }} · {{ ownerCode || characterProfile.ownerCode }}</strong><small>{{ tx('当前角色独立档案', 'Character-Specific Profile') }}</small></div>
              <b>{{ tx(`${currentActionLabel()}方向`, `${currentActionLabel()} Direction`) }}</b>
            </header>
            <div class="character-research-grid">
              <span><small>{{ tx('当前选用技能', 'Selected Skills') }}</small><b>{{ selectedSkillNames.length ? selectedSkillNames.join('、') : tx('当前配装未记录', 'Not recorded in this build') }}</b></span>
              <span><small>{{ tx('角色伤害上限表', 'Character Cap Curves') }}</small><b>{{ tx(`普通 ${capEvidenceSummary.normalRows} 行 · Arts ${capEvidenceSummary.artsRows} 行`, `${capEvidenceSummary.normalRows} normal · ${capEvidenceSummary.artsRows} Arts rows`) }}</b></span>
              <span><small>{{ tx('防御端计算', 'Defense Model') }}</small><b>{{ tx('HP、减伤、单次承伤与存活次数', 'HP, defense, hit damage, and surviving hits') }}</b></span>
            </div>
            <p>{{ tx('角色身份、当前技能和 2.0.2 独立上限曲线会进入本次方案证据；尚未逐动作校准的倍率、角色机制与完整轮转不会冒充精确结论。', 'Character identity, selected skills, and the 2.0.2 character cap curves are attached to this run. Uncalibrated per-action coefficients, mechanics, and full rotations are not presented as exact results.') }}</p>
          </section>
          <details v-if="combatMode" class="optimizer-advanced">
            <summary><span><strong>{{ tx('高级战斗条件', 'Advanced Combat Conditions') }}</strong><small>{{ tx('不确定时保持默认即可', 'Keep the defaults if unsure') }}</small></span><em><span class="when-closed">{{ tx('展开设置', 'Show') }}</span><span class="when-open">{{ tx('收起', 'Hide') }}</span></em></summary>
            <div class="battle-condition-grid">
              <label class="condition-field is-wide"><span>{{ tx('触发覆盖率', 'Effect Uptime') }}</span><span class="condition-control"><input v-model.number="coverage" class="condition-range" type="range" min="0" max="100" step="5" /><output>{{ coverage }}%</output></span></label>
              <label class="condition-field"><span>{{ tx('覆盖率下限', 'Uptime Lower Bound') }}</span><span class="condition-number"><input v-model.number="coverageLow" class="ui-input" type="number" min="0" max="100" step="5" /><span class="condition-suffix">%</span></span></label>
              <label class="condition-field"><span>{{ tx('覆盖率上限', 'Uptime Upper Bound') }}</span><span class="condition-number"><input v-model.number="coverageHigh" class="ui-input" type="number" min="0" max="100" step="5" /><span class="condition-suffix">%</span></span></label>
              <p v-if="!coverageInputsValid" class="coverage-error" role="alert">{{ tx('请填写 0 到 100 之间的覆盖率，且下限不能高于上限。', 'Enter uptime bounds from 0 to 100, with the lower bound not exceeding the upper bound.') }}</p>
              <label class="condition-field is-wide"><span>{{ tx('当前 HP', 'Current HP') }}</span><span class="condition-control"><input v-model.number="currentHp" class="condition-range" type="range" min="1" max="100" step="1" /><output>{{ currentHp }}%</output></span></label>
              <label class="condition-field"><span>{{ tx('动作倍率', 'Action Rate') }}</span><span class="condition-number"><input v-model.number="actionRate" class="ui-input" type="number" min="0.1" max="9999" step="0.1" /><span class="condition-suffix">×</span></span></label>
              <label class="condition-field"><span>{{ tx('OD 阶段', 'OD Stage') }}</span><select v-model.number="odStage" class="ui-select"><option :value="0">{{ tx('关闭', 'Off') }}</option><option :value="1">OD 1</option><option :value="2">OD 2+</option></select></label>
              <label class="condition-check"><input v-model="berserk" type="checkbox" /><span>{{ tx('按狂暴状态计算', 'Calculate as Berserk') }}</span></label>
              <label class="condition-field"><span>{{ tx('最低 HP 约束', 'Minimum HP') }}</span><span class="condition-number"><input v-model.number="minimumHp" class="ui-input" type="number" min="0" step="1000" /><span class="condition-suffix">HP</span></span></label>
              <label class="condition-field"><span>{{ tx('最低减伤约束', 'Minimum Defense') }}</span><span class="condition-number"><input v-model.number="minimumDefense" class="ui-input" type="number" min="0" max="100" step="1" /><span class="condition-suffix">%</span></span></label>
              <label class="condition-field"><span>{{ tx('基准单次伤害', 'Base Incoming Hit') }}</span><span class="condition-number"><input v-model.number="incomingDamage" class="ui-input" type="number" min="1" step="1000" /><span class="condition-suffix">{{ tx('伤害', 'DMG') }}</span></span></label>
              <label class="condition-field"><span>{{ tx('至少承受次数', 'Required Surviving Hits') }}</span><span class="condition-number"><input v-model.number="surviveHits" class="ui-input" type="number" min="0" max="20" step="1" /><span class="condition-suffix">{{ tx('次', 'hits') }}</span></span></label>
            </div>
          </details>
        </div>

        <div class="optimizer-resource-row">
          <span>{{ tx('使用哪些因子', 'Sigil Source') }}</span>
          <div class="resource-tabs ui-seg">
            <button type="button" class="ui-seg-btn" :class="{ 'is-on': domain === 'all' }" :disabled="!savePath" @click="domain = 'all'">{{ tx('背包优先，缺少时制造', 'Owned First, Create Gaps') }}</button>
            <button type="button" class="ui-seg-btn" :class="{ 'is-on': domain === 'inventory' }" :disabled="!savePath" @click="domain = 'inventory'">{{ tx('只用当前背包', 'Current Save Only') }}</button>
          </div>
          <small>{{ domain === 'inventory' ? tx(`从背包 ${inventory.length} 个可识别实例中组合；每个 SlotID 最多使用一次，不足时只显示缺口。背包实例可能也被其他预设引用，应用后请逐槽核对。`, `Combines ${inventory.length} recognized inventory instances. Each SlotID is used once at most; gaps remain explicit. An instance may also be referenced by another preset, so review every slot after applying.`) : tx('先使用互不重复的背包实例；缺少的因子会标成“待制造”，应用结果时仍不会写存档。', 'Uses distinct owned instances first. Missing sigils are marked “creation required”; applying a result still does not write the save.') }}</small>
          <details class="optimizer-source-details">
            <summary>{{ tx('只查看理论目录', 'View Theoretical Sources') }}</summary>
            <div class="ui-seg"><button type="button" class="ui-seg-btn" :class="{ 'is-on': domain === 'catalog' }" @click="domain = 'catalog'">{{ tx('合法因子目录', 'Legal Catalog') }}</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': domain === 'table' }" @click="domain = 'table'">{{ tx('游戏表精确行', 'Exact Game-Table Rows') }}</button></div>
          </details>
        </div>

        <button type="button" class="solve-button ui-btn is-primary" :disabled="!canSolve || loading || solving" @click="solve">{{ solving ? tx('正在组合 12 个因子槽…', 'Building 12 sigil slots…') : optimizerIntent === 'auto' ? tx('生成这条路线', 'Build This Route') : tx('按目标生成方案', 'Generate Target Builds') }}</button>
      </section>

      <section ref="resultRegion" class="optimizer-results" data-testid="optimizer-output" tabindex="-1">
        <header class="optimizer-output-heading"><div><strong>{{ tx('方案结果', 'Plan Results') }}</strong><span>{{ tx('计算结果会在这里显示；应用后回填当前草稿并切回手动配装，不会直接写入存档。', 'Results appear here. Applying fills the current draft and returns to manual mode; it does not write the save.') }}</span></div></header>
        <p v-if="solving" class="optimizer-progress" role="status">{{ tx('正在组合 12 个因子槽，请稍候…', 'Building 12 sigil slots…') }}</p>
        <p v-else-if="solveError" class="optimizer-error" role="alert">{{ solveError }}</p>
          <div v-if="solved && primaryResult" class="result-scope ui-notice is-info"><strong>{{ selectedRouteBranch ? tx(`只显示“${selectedRouteBranch.nameZh}”的最佳可达方案`, `Showing the best reachable plan for “${selectedRouteBranch.nameEn}”`) : combatMode ? tx('先显示无需制造的纯背包方案，再给出缺口制造与游戏表方案作对照', 'Owned-only plans come first, followed by created-gap and game-table comparisons') : tx('结果先按是否满足目标，再按是否需要制造排序', 'Results prioritize target completion, then whether creation is needed') }}</strong><span>{{ tx('保持当前武器、祝福、召唤石与专精不变，只重新安排 12 个因子槽；永久成长也不会改动。', 'Keeps the current weapon, wrightstone, summons, and mastery unchanged and only rearranges the 12 sigil slots; permanent growth also stays unchanged.') }}</span></div>
        <p v-if="solved && suppressedManufacturedCount" class="optimizer-empty is-success">{{ tx(`当前背包方案已经不低于 ${suppressedManufacturedCount} 个制造候选，这些更差的制造方案已隐藏；不会为了“能制造”而推荐更差配装。`, `The current inventory plan already matches or beats ${suppressedManufacturedCount} manufactured candidates, so those inferior plans are hidden.`) }}</p>
        <p v-if="!solving && !solveError && !solved" class="optimizer-empty">{{ tx('点击计算后，方案会显示在这里，并可直接预览 12 个因子槽。', 'After calculation, plans appear here with a direct preview of all 12 sigil slots.') }}</p>
        <p v-else-if="!solving && !solveError && (!results.length || !results[0].picked.length)" class="optimizer-empty is-warning">{{ tx('这组条件暂时配不出来。可降低目标等级、放宽高级条件，或改为允许制造缺少的因子。', 'No plan matches these conditions yet. Lower target levels, relax advanced conditions, or allow missing sigils to be created.') }}</p>
        <template v-for="(result, index) in displayResults" v-else :key="`${result.domain}-${result.domainRank || index + 1}-${result.score}`">
          <h3 v-if="index === 0 || resultGroup(displayResults[index - 1]) !== resultGroup(result)" class="result-group-heading"><span>{{ resultGroupLabel(result) }}</span><small>{{ resultGroup(result) === 0 ? tx('无需制造，直接回填背包实例', 'No creation; fills owned instances directly') : resultGroup(result) === 1 ? tx('目标满足，但背包存在缺口', 'Targets met, with inventory gaps') : tx('当前来源无法完全满足目标', 'Selected source cannot fully meet every target') }}</small></h3>
          <article class="optimizer-result ui-card is-flat" :class="`result-group-${resultGroup(result)}`">
          <header><div><small>{{ domainLabel(result.domain) }}</small><strong>{{ combatMode ? resultCharacterTitle(index) : resultGroupLabel(result) }}</strong><span>{{ resultRankReason(result) }}</span></div><span class="result-score"><small>{{ result.combat ? tx('当前动作最终估算', 'Estimated Final Action Damage') : tx('目标覆盖得分', 'Target Coverage Score') }}</small><b>{{ result.combat ? Math.round(result.combat.metrics.finalDamage).toLocaleString() : result.score }}</b></span><button v-if="index > 0" type="button" class="result-expand-button" :aria-expanded="isResultExpanded(result, index)" @click="toggleResult(result, index)">{{ isResultExpanded(result, index) ? tx('收起详情', 'Collapse') : tx('展开 12 槽', 'Show 12 Slots') }}</button></header>
          <template v-if="isResultExpanded(result, index)">
          <section v-if="result.combat" class="result-cap-proof" :class="{ capped: combatCapProof(result).hitsCap }">
            <header><div><strong>{{ tx('伤害上限核对', 'Damage Cap Check') }}</strong><small>{{ combatCapProof(result).evidence }}</small></div><b>{{ combatCapProof(result).hitsCap ? tx('已触及当前动作上限', 'Reaches Current Action Cap') : tx('尚未触及当前动作上限', 'Below Current Action Cap') }}</b></header>
            <div>
              <span><small>{{ tx('因子追加上限', 'Sigil Cap Bonus') }}</small><b>+{{ combatCapProof(result).bonus.toFixed(1) }}%</b></span>
              <span><small>{{ tx('上限前伤害', 'Uncapped Damage') }}</small><b>{{ Math.round(combatCapProof(result).uncappedDamage).toLocaleString() }}</b></span>
              <span><small>{{ tx('当前动作上限', 'Current Action Cap') }}</small><b>{{ Math.round(combatCapProof(result).effectiveCap).toLocaleString() }}</b></span>
              <span><small>{{ tx('上限利用率', 'Cap Utilization') }}</small><b>{{ combatCapProof(result).utilization.toFixed(1) }}%</b></span>
            </div>
            <p>{{ combatCapProof(result).hitsCap ? tx('当前伤害已经碰到动作上限，方案会继续比较伤害上限、追击、属性克制、斯巴达与狂战士的实际边际收益。顶部最终估算还会计入封顶后生效的增伤。', 'Damage reaches the action cap, so the plan compares the marginal value of cap, supplemental damage, elemental conversion, Spartan, and Berserker. The final estimate also includes post-cap damage bonuses.') : tx(`距离当前动作上限还差约 ${Math.round(combatCapProof(result).offenseGap).toLocaleString()}，方案优先补真正能提高上限前伤害的因子，不会盲目堆“伤害上限”。`, `About ${Math.round(combatCapProof(result).offenseGap).toLocaleString()} damage remains before this action reaches its cap, so the plan prioritizes real pre-cap damage instead of blindly stacking cap.`) }}</p>
          </section>
          <section v-if="result.combat" class="result-defense-proof" :class="{ valid: combatDefenseProof(result).valid }">
            <header><strong>{{ tx('防御端核对', 'Defense Check') }}</strong><b>{{ combatDefenseProof(result).valid ? tx('已满足当前生存约束', 'Current Survival Constraints Met') : tx('仍有生存缺口', 'Survival Gap Remains') }}</b></header>
            <div>
              <span><small>HP</small><b>{{ Math.round(combatDefenseProof(result).hp).toLocaleString() }}</b></span>
              <span><small>{{ tx('减伤', 'Defense') }}</small><b>{{ combatDefenseProof(result).defense.toFixed(1) }}%</b></span>
              <span><small>{{ tx('基准伤害 → 实际承伤', 'Base Hit → Damage Taken') }}</small><b>{{ Math.round(combatDefenseProof(result).baseHit).toLocaleString() }} → {{ Math.round(combatDefenseProof(result).reducedHit).toLocaleString() }}</b></span>
              <span><small>{{ tx('预计可承受', 'Estimated Hits Survived') }}</small><b>{{ combatDefenseProof(result).survivableHits }} {{ tx('次', 'hits') }}</b></span>
            </div>
            <p v-if="combatDefenseProof(result).checks.length"><span v-for="check in combatDefenseProof(result).checks" :key="check.zh" :class="{ met: check.met }">{{ check.met ? '✓' : '×' }} {{ language === 'en' ? check.en : check.zh }}</span></p>
            <p v-else>{{ tx('当前没有设置硬性生存门槛；如需兼顾高难承伤，请在“高级战斗条件”填写基准单次伤害、最低 HP、最低减伤或存活次数。', 'No hard survival threshold is set. For difficult encounters, enter a base incoming hit, minimum HP, minimum defense, or required surviving hits under Advanced Combat Conditions.') }}</p>
          </section>
          <section class="result-final-levels">
            <header><strong>{{ selectedRoute ? tx('固定路线达成情况', 'Fixed Route Completion') : !combatMode ? tx('目标技能达成', 'Target Skill Completion') : tx('关键技能最终等级', 'Final Key Skill Levels') }}</strong><span>{{ tx('最终技能等级', 'Final Skill Levels') }}</span></header>
            <div v-if="selectedRoute && routeFulfilment(result).rows.length" class="target-fulfilment" :class="{ complete: routeFulfilment(result).complete }"><strong>{{ routeFulfilment(result).complete ? tx('12 槽路线要求全部达到', 'All 12-slot route requirements met') : tx('仍有路线等级缺口', 'Route levels are still short') }}</strong><span v-for="item in routeFulfilment(result).rows" :key="item.traitId" :class="{ met: item.met }">{{ item.name }} <b>Lv{{ item.actual }}</b><em>/ Lv{{ item.target }}</em></span></div>
            <div v-else-if="!combatMode && targetFulfilment(result).rows.length" class="target-fulfilment" :class="{ complete: targetFulfilment(result).complete }"><strong>{{ targetFulfilment(result).complete ? tx('已按顺序达到全部目标', 'All targets met in order') : tx(`按顺序完成前 ${targetFulfilment(result).completedPrefix} 项；第 ${targetFulfilment(result).completedPrefix + 1} 项还缺等级`, `First ${targetFulfilment(result).completedPrefix} met; target #${targetFulfilment(result).completedPrefix + 1} is short`) }}</strong><span v-for="(item, targetIndex) in targetFulfilment(result).rows" :key="item.name" :class="[item.state, { met: item.met }]"><i>#{{ targetIndex + 1 }}</i>{{ item.name }} <b>Lv{{ item.actual }}</b><em>/ Lv{{ item.target }}</em><small v-if="item.state === 'blocked'">{{ tx(`缺 Lv${item.missing}`, `Short Lv${item.missing}`) }}</small><small v-else-if="item.state === 'waiting'">{{ tx('等待前序', 'After earlier targets') }}</small></span></div>
            <div v-else class="result-level-preview"><span v-for="total in visibleResultTotals(result)" :key="total.traitId || total.displayName"><small>{{ total.displayName }}</small><b>Lv{{ total.effective }}</b></span></div>
          </section>
          <h4 class="slot-preview-title"><span>{{ tx('12 个因子槽预览', '12-Slot Sigil Preview') }}</span><small>{{ tx('缺少的实例会标为“待制造”；没有替换的槽保留当前因子。', 'Missing instances are marked “Create on Save”; unchanged slots keep their current sigils.') }}</small></h4>
          <div class="result-sigil-grid solution-slot-grid" :aria-label="tx('十二个因子槽预览', 'Twelve-slot sigil preview')">
            <article v-for="(row, slot) in resultPreviewSlots(result)" :key="`${slot}-${row.item?.slotId || row.item?.id || row.item?.hash || 'empty'}`" class="result-sigil-slot" :class="{ empty: !row.item, constructed: row.item && row.source === '待制造', retained: row.retained }">
              <i>{{ String(slot + 1).padStart(2, '0') }}</i>
              <template v-if="row.item">
                <span class="result-sigil-icon"><img v-if="previewIcon(row)" :src="previewIcon(row)" alt="" /><b v-else>◆</b></span>
                <span class="result-sigil-copy"><strong>{{ row.item.name || row.item.sigilName || tx('因子', 'Sigil') }}</strong><small v-if="previewTrait(row.item, 0).name">{{ tx('主', 'P') }} · {{ previewTrait(row.item, 0).name }} Lv{{ previewTrait(row.item, 0).level }}</small><small v-if="previewTrait(row.item, 1).name">{{ tx('副', 'S') }} · {{ previewTrait(row.item, 1).name }} Lv{{ previewTrait(row.item, 1).level }}</small></span>
                <em>{{ row.item?.source === 'inventory' && row.item?.slotId ? tx(`背包 #${row.item.slotId}`, `Owned #${row.item.slotId}`) : row.retained ? (row.item?.slotId ? tx(`背包 #${row.item.slotId}`, `Owned #${row.item.slotId}`) : tx('保留当前槽', 'Keep Current Slot')) : tx('待制造', 'Create on Save') }}</em>
              </template>
              <template v-else><span class="empty-slot-mark">＋</span><strong>{{ tx('空槽', 'Empty Slot') }}</strong></template>
            </article>
          </div>
          <details class="result-details">
            <summary>{{ tx('查看技能汇总与排序依据', 'View Skill Totals and Ranking Details') }}</summary>
            <div class="result-levels"><span v-for="total in localizedResultTotals(result)" :key="total.traitId || total.displayName"><small>{{ total.displayName }}</small><b>Lv{{ total.effective }}</b><em v-if="total.level > total.effective">+{{ total.level - total.effective }} {{ tx('溢出', 'overflow') }}</em></span></div>
            <p v-if="result.explanation" class="result-explanation"><b>{{ domainLabel(result.domain) }}</b> · {{ result.explanation.summary }}{{ language === 'en' ? `; ${result.explanation.limitationEn}` : `；${result.explanation.limitation}` }}<br><small>{{ language === 'en' ? result.explanation.inventoryReasonEn : result.explanation.inventoryReason }}</small></p>
          </details>
          <div v-if="baseLoadout" class="optimizer-apply-row"><span>{{ tx('回填后会自动切回手动因子槽，右侧重新计算实际技能与加成；只有之后点击保存才会写入存档。', 'After filling, manual slots reopen and the right side recalculates actual skills and bonuses. The save changes only after you click Save.') }}</span><button type="button" class="ui-btn is-primary" @click="applyResult(result)">{{ tx('回填到当前配装草稿', 'Fill Current Draft') }}</button></div>
          </template>
          </article>
        </template>
      </section>
    </div>
  </section>
</template>

<style scoped>
.optimizer-page { min-width:0; display:grid; gap:12px; container:optimizer / inline-size; color:var(--text-secondary); font-size:13px; }
.optimizer-heading { min-width:0; display:flex; align-items:end; justify-content:space-between; gap:var(--space-4); padding:2px var(--space-2); border-bottom:1px solid var(--border-soft); }
.optimizer-heading > div { min-width:0; }
.optimizer-heading small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.optimizer-heading h2 { margin:2px 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); letter-spacing:0; }
.optimizer-heading p { max-width:80ch; margin:0 0 var(--space-3); color:var(--text-muted); font-size:var(--fs-sm); }
.optimizer-character { flex:0 0 auto; display:grid; gap:2px; padding:var(--space-2) var(--space-3); border-left:2px solid var(--accent); color:var(--text-secondary); font-size:var(--fs-sm); }
.optimizer-character small { max-width:220px; overflow:hidden; text-overflow:ellipsis; color:var(--text-muted); font-size:var(--fs-2xs); white-space:nowrap; }
.optimizer-setup { min-width:0; display:grid; grid-template-columns:minmax(0,1fr); gap:12px; align-items:start; }
.optimizer-controls { min-width:0; display:grid; gap:12px; padding:12px; }
.optimizer-page.is-embedded .optimizer-controls { padding:0; }
.optimizer-intent { min-width:0; display:flex; flex-wrap:nowrap; gap:16px; overflow-x:auto; overflow-y:hidden; padding:0; border-bottom:1px solid rgba(133,94,43,.3); background:transparent; scrollbar-width:thin; scrollbar-color:var(--border-strong) transparent; }
.optimizer-intent button { min-width:max-content; min-height:36px; flex:0 0 auto; padding:0 2px; border:0; border-bottom:2px solid transparent; border-radius:0; color:var(--text-muted); background:transparent; box-shadow:none; font-family:inherit; font-size:14px; font-weight:700; line-height:1; text-align:center; cursor:pointer; }
.optimizer-intent button.active { border-bottom-color:#765126; color:#765126; background:transparent; box-shadow:none; }
.optimizer-control-group,.chosen-traits { min-width:0; display:grid; gap:8px; }
.optimizer-control-group > small,.chosen-traits > small { color:var(--text-muted); font-size:12px; font-weight:var(--fw-semibold); }
.chosen-order-copy { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:12px; }
.chosen-order-copy small { color:var(--text-muted); font-size:12px; font-weight:var(--fw-semibold); line-height:1.45; }
.chosen-order-copy b { flex:0 0 auto; color:var(--accent-hover); font-size:12px; }
.optimizer-control-group p { margin:0; color:var(--text-muted); font-size:13px; line-height:1.5; }
.optimizer-control-group .optimizer-context-warning { padding:7px 9px; border-left:3px solid var(--warning); background:var(--warning-bg); color:var(--warning-ink); font-size:12px; }
.character-research-card { min-width:0; display:grid; gap:8px; padding:10px; border:1px solid var(--line-soft); border-left:3px solid var(--accent); border-radius:8px; background:rgba(255,253,247,.62); }
.character-research-card > header { min-width:0; display:flex; justify-content:space-between; align-items:center; gap:10px; }
.character-research-card > header div { min-width:0; display:grid; gap:1px; }
.character-research-card > header strong { color:var(--text-primary); font-size:15px; }
.character-research-card > header small { color:var(--text-muted); font-size:12px; }
.character-research-card > header > b { flex:0 0 auto; color:var(--accent-hover); font-size:12px; }
.character-research-grid { min-width:0; display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:6px; }
.character-research-grid > span { min-width:0; display:grid; gap:2px; padding:7px 8px; border:1px solid rgba(177,145,94,.24); border-radius:6px; background:rgba(249,242,225,.56); }
.character-research-grid small { color:var(--text-muted); font-size:11px; }
.character-research-grid b { overflow-wrap:anywhere; color:var(--text-secondary); font-size:12px; line-height:1.4; }
.character-research-card > p { margin:0; color:var(--text-muted); font-size:12px; line-height:1.5; }
.skill-target-workflow,.auto-recommend-workflow { min-width:0; display:grid; gap:10px; }
.character-route-planner { min-width:0; display:grid; gap:10px; }
.character-route-planner > header { min-width:0; display:flex; align-items:end; justify-content:space-between; gap:12px; }
.character-route-planner > header > div { min-width:0; display:grid; gap:2px; }
.character-route-planner > header strong { color:var(--text-primary); font-size:16px; }
.character-route-planner > header small { color:var(--text-muted); font-size:13px; line-height:1.45; }
.character-route-planner > header > b { flex:0 0 auto; color:var(--accent-hover); font-size:12px; }
.character-route-tabs { min-width:0; display:flex; gap:16px; overflow-x:auto; overflow-y:hidden; padding:0; border-bottom:1px solid rgba(133,94,43,.3); background:transparent; scrollbar-width:thin; scrollbar-color:var(--border-strong) transparent; }
.character-route-tabs .ui-seg-btn { min-width:max-content; min-height:36px; flex:0 0 auto; padding:0 2px; border:0; border-bottom:2px solid transparent; border-radius:0; color:var(--text-muted); background:transparent; box-shadow:none; font-size:14px; }
.character-route-tabs .ui-seg-btn.is-on { border-bottom-color:#765126; color:#765126; background:transparent; box-shadow:none; }
.route-branch-picker { min-width:0; display:grid; gap:6px; padding:9px 10px 0; border:1px solid rgba(133,94,43,.22); border-radius:8px 8px 0 0; background:rgba(249,242,225,.42); }
.route-branch-heading { min-width:0; display:flex; align-items:end; justify-content:space-between; gap:12px; }
.route-branch-heading > span { min-width:0; display:grid; gap:1px; }
.route-branch-heading strong { color:var(--text-primary); font-size:14px; }
.route-branch-heading small { color:var(--text-muted); font-size:12px; line-height:1.4; }
.route-branch-heading > b { flex:0 0 auto; color:var(--accent-hover); font-family:var(--font-data); font-size:12px; }
.route-branch-tabs { gap:14px; }
.route-branch-tabs .ui-seg-btn { min-height:34px; font-size:13px; }
.selected-route-card { min-width:0; display:grid; gap:10px; padding:12px; border:1px solid var(--line-soft); border-left:3px solid var(--accent); border-radius:8px; background:rgba(255,253,247,.66); }
.selected-route-copy { min-width:0; display:grid; gap:3px; }
.selected-route-copy strong { color:var(--text-primary); font-size:16px; }
.selected-route-copy p { margin:0; color:var(--text-secondary); font-size:13px; line-height:1.55; }
.selected-route-copy > small { color:var(--warning-ink); font-size:11px; line-height:1.45; }
.route-base-summary { min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr); gap:7px; align-items:start; margin-top:3px; padding:7px 8px; border-left:2px solid var(--border-strong); background:rgba(249,242,225,.45); }
.route-base-summary b { color:var(--accent-hover); font-size:11px; white-space:nowrap; }
.route-base-summary span { color:var(--text-muted); font-size:11px; line-height:1.45; }
.route-swap-summary { min-width:0; display:grid; gap:6px; padding:8px 9px; border:1px solid rgba(177,145,94,.24); border-radius:7px; background:rgba(255,255,255,.3); }
.route-swap-summary > strong { color:var(--text-secondary); font-size:12px; }
.route-swap-summary > div { min-width:0; display:flex; flex-wrap:wrap; gap:5px; }
.route-swap-summary span { min-width:0; display:flex; align-items:center; gap:5px; padding:4px 7px; border:1px solid var(--line-soft); border-radius:999px; background:rgba(249,242,225,.72); }
.route-swap-summary span.added { border-color:rgba(49,116,84,.28); background:var(--success-bg); }
.route-swap-summary span.removed:not(.added) { border-color:rgba(151,91,37,.25); background:var(--warning-bg); }
.route-swap-summary b { color:var(--text-secondary); font-size:11px; }
.route-swap-summary small { color:var(--text-muted); font-family:var(--font-data); font-size:10px; font-weight:700; }
.route-requirements { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(250px,1fr)); gap:7px; }
.route-trait { min-width:0; display:grid; grid-template-columns:32px minmax(0,1fr); gap:9px; align-items:center; padding:8px 9px; border:1px solid rgba(177,145,94,.28); border-radius:7px; background:rgba(249,242,225,.56); }
.route-trait.is-branch-added { border-left:3px solid var(--accent); background:linear-gradient(90deg,var(--accent-soft),rgba(249,242,225,.5)); }
.route-trait > img { width:32px; height:32px; border-radius:6px; object-fit:cover; }
.route-trait > span { min-width:0; display:grid; gap:2px; }
.route-trait b { color:var(--text-primary); font-size:14px; }
.route-trait b > em { color:var(--accent-hover); font-family:var(--font-data); font-size:12px; font-style:normal; }
.route-trait b > i { margin-left:6px; padding:2px 5px; border-radius:999px; color:var(--accent-hover); background:var(--accent-soft); font-size:10px; font-style:normal; font-weight:700; }
.route-trait small { color:var(--text-muted); font-size:12px; line-height:1.45; }
.route-trait > span > em,.route-alternatives > span > em { color:var(--warning-ink); font-size:11px; font-style:normal; font-weight:700; }
.route-trait.condition-unmet,.route-alternatives > span.condition-unmet { border-color:rgba(151,91,37,.32); background:var(--warning-bg); }
.route-alternatives { min-width:0; padding:8px 9px; border:1px solid rgba(177,145,94,.25); border-radius:7px; background:rgba(249,242,225,.42); }
.route-alternatives summary { color:#765126; font-size:13px; font-weight:700; cursor:pointer; }
.route-alternatives > p { margin:8px 0 0; color:var(--text-muted); font-size:12px; line-height:1.5; }
.route-alternatives > span { min-width:0; display:grid; gap:2px; margin-top:7px; padding:7px 8px; border-left:2px solid var(--border-soft); background:rgba(255,255,255,.32); }
.route-alternatives > span b { color:var(--text-primary); font-size:13px; }
.route-final-checks > span b { display:flex; flex-wrap:wrap; gap:4px 8px; align-items:baseline; }
.route-final-checks > span b > em { color:var(--text-muted); font-family:var(--font-data); font-size:12px; font-style:normal; }
.route-final-checks > span.condition-met { border-left-color:var(--success-ink); background:var(--success-bg); }
.route-alternatives > span small { color:var(--text-muted); font-size:12px; line-height:1.45; }
.selected-route-card > footer { min-width:0; display:flex; flex-wrap:wrap; gap:6px 10px; align-items:center; padding-top:8px; border-top:1px solid var(--line-soft); }
.selected-route-card > footer > span { color:var(--text-secondary); font-size:12px; font-weight:700; }
.selected-route-card > footer > a { color:var(--accent-hover); font-size:12px; font-weight:700; text-decoration:none; }
.selected-route-card > footer > a:hover { text-decoration:underline; }
.selected-route-card > footer > small { flex:1 0 100%; color:var(--text-muted); font-size:11px; line-height:1.45; }
.skill-target-entry { min-width:0; display:grid; grid-template-columns:minmax(220px,1fr) minmax(104px,128px) auto; gap:8px; align-items:end; padding:10px; border:1px solid var(--line-gold); border-radius:8px; background:rgba(249,242,225,.78); }
.skill-target-entry label { min-width:0; display:grid; gap:5px; }
.skill-target-entry label > span,.optimizer-resource-row > span { color:var(--text-secondary); font-size:12px; font-weight:700; }
.skill-target-entry .ui-input,.add-target-button { min-height:36px; }
.add-target-button { padding-inline:14px; white-space:nowrap; }
.optimizer-page :deep(.catalog-trigger) { min-height:36px; padding:2px 10px; font-size:13px; }
.optimizer-page :deep(.catalog-icon) { width:32px; height:32px; flex-basis:32px; }
.optimizer-resource-row { min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr); gap:6px 10px; align-items:center; padding:10px; border:1px solid var(--line-soft); border-radius:8px; background:var(--panel-solid); }
.optimizer-resource-row > small { grid-column:2; color:var(--text-muted); font-size:12px; line-height:1.45; }
.resource-tabs { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:6px; }
.resource-tabs .ui-seg-btn,.optimizer-source-details .ui-seg-btn { min-height:36px; font-size:13px; }
.optimizer-source-details { grid-column:2; color:var(--text-muted); font-size:12px; }
.optimizer-source-details summary,.result-details summary { width:max-content; color:#765126; font-size:12px; font-weight:700; cursor:pointer; }
.optimizer-source-details > .ui-seg { margin-top:8px; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:6px; }
.optimizer-advanced { min-width:0; border:1px solid var(--line-soft); border-radius:8px; background:rgba(249,242,225,.44); overflow:hidden; }
.optimizer-advanced summary { min-height:42px; display:flex; align-items:center; justify-content:space-between; gap:12px; padding:8px 11px; color:#765126; cursor:pointer; list-style:none; }
.optimizer-advanced summary::-webkit-details-marker { display:none; }
.optimizer-advanced summary > span { min-width:0; display:flex; align-items:baseline; gap:8px; }
.optimizer-advanced summary strong { color:var(--text-primary); font-size:14px; }
.optimizer-advanced summary small { color:var(--text-muted); font-size:12px; font-weight:500; }
.optimizer-advanced summary > em { flex:0 0 auto; color:var(--accent); font-size:12px; font-style:normal; font-weight:700; }
.optimizer-advanced summary .when-open { display:none; }
.optimizer-advanced[open] summary .when-closed { display:none; }
.optimizer-advanced[open] summary .when-open { display:inline; }
.battle-condition-grid { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:8px 10px; padding:10px; border-top:1px solid var(--line-soft); background:rgba(255,253,247,.62); }
.condition-field { min-width:0; min-height:38px; display:grid; grid-template-columns:minmax(108px,.75fr) minmax(112px,1fr); gap:8px; align-items:center; padding:4px 7px; border:1px solid rgba(177,145,94,.24); border-radius:6px; background:rgba(255,255,255,.35); color:var(--text-secondary); font-size:13px; }
.condition-field.is-wide { grid-column:1 / -1; grid-template-columns:minmax(108px,.36fr) minmax(220px,1fr); }
.condition-control { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) 54px; gap:8px; align-items:center; }
.condition-control output { color:var(--accent-hover); font-family:var(--font-data); font-size:13px; font-weight:700; text-align:right; }
.condition-range { min-width:0; width:100%; height:22px; margin:0; accent-color:var(--accent); cursor:pointer; }
.condition-number { position:relative; min-width:0; display:block; }
.condition-number .ui-input { width:100%; min-height:34px; padding-right:38px; font-family:var(--font-data); font-weight:700; }
.condition-number .condition-suffix { position:absolute; top:50%; right:10px; transform:translateY(-50%); color:var(--text-muted); font-size:12px; pointer-events:none; }
.condition-field .ui-select { width:100%; min-height:34px; }
.condition-check { min-width:0; min-height:38px; display:flex; align-items:center; gap:9px; padding:4px 9px; border:1px solid rgba(177,145,94,.24); border-radius:6px; background:rgba(255,255,255,.35); color:var(--text-secondary); font-size:13px; cursor:pointer; }
.condition-check input { width:17px; height:17px; margin:0; accent-color:var(--accent); }
.battle-condition-grid .coverage-error { grid-column:1 / -1; margin:0; padding:7px 9px; border-left:3px solid var(--danger); background:var(--danger-bg); color:var(--danger-ink); font-size:12px; }
.profile-row { display:flex; flex-wrap:wrap; gap:6px; }
.profile-row .ui-chip { min-height:34px; padding:0 10px; font-size:13px; }
.chosen-traits > div { min-width:0; display:grid; gap:6px; }
.chosen-trait { min-width:0; min-height:44px; display:grid; grid-template-columns:32px 28px minmax(92px,1fr) minmax(86px,112px) 58px 32px; align-items:center; gap:8px; padding:5px 8px; border:1px solid var(--accent-border); border-left:3px solid var(--accent); border-radius:7px; background:linear-gradient(90deg,var(--accent-soft),rgba(255,253,247,.72)); color:var(--text-secondary); }
.chosen-trait img { width:32px; height:32px; border-radius:6px; object-fit:cover; }
.chosen-trait > b { color:var(--accent); font-family:var(--font-data); font-size:12px; }
.chosen-trait span { overflow-wrap:anywhere; color:var(--text-primary); font-size:14px; font-weight:700; }
.chosen-trait label { min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr); gap:4px; align-items:center; }
.chosen-trait label small { color:var(--accent-hover); font-size:12px; font-weight:var(--fw-bold); }
.chosen-trait label input { width:100%; min-height:34px; padding:2px 8px; font-family:var(--font-data); font-weight:var(--fw-bold); }
.chosen-order-actions { display:flex; align-items:center; gap:2px; }
.chosen-order-actions button { width:28px; height:28px; padding:0; border:1px solid var(--border-soft); border-radius:5px; color:var(--accent-hover); background:rgba(255,255,255,.48); cursor:pointer; }
.chosen-order-actions button:disabled { color:var(--text-muted); opacity:.38; cursor:default; }
.chosen-trait > button { width:32px; height:32px; border:0; border-radius:50%; color:var(--text-muted); background:transparent; cursor:pointer; }
.chosen-trait > button:hover,.chosen-trait > button:focus-visible { color:var(--danger-ink); background:var(--danger-bg); }
.empty-choice { padding:10px; border:1px dashed var(--line-soft); border-radius:7px; color:var(--text-muted); font-size:13px; text-align:center; }
.solve-button { width:100%; min-height:38px; font-size:14px; }
.optimizer-results { min-width:0; display:grid; gap:var(--space-3); scroll-margin-block:12px; outline:none; }
.optimizer-output-heading { min-width:0; padding:10px 12px; border:1px solid var(--line-soft); border-radius:8px; background:rgba(249,242,225,.54); }
.optimizer-output-heading > div { min-width:0; display:grid; gap:2px; }
.optimizer-output-heading strong { color:var(--text-primary); font-size:15px; }
.optimizer-output-heading span { color:var(--text-muted); font-size:12px; line-height:1.45; }
.optimizer-progress,.optimizer-error,.optimizer-empty { min-height:0; margin:0; padding:12px 14px; border:1px dashed var(--line-soft); border-radius:7px; color:var(--text-muted); background:rgba(255,253,247,.6); font-size:13px; line-height:1.45; text-align:left; }
.optimizer-progress { border-style:solid; border-color:var(--accent-border); color:var(--accent-hover); background:var(--accent-soft); }
.optimizer-error,.optimizer-empty.is-warning { border-style:solid; border-color:rgba(151,91,37,.3); color:var(--warning-ink); background:var(--warning-bg); }
.optimizer-empty.is-success { border-style:solid; border-color:rgba(47,108,70,.28); color:var(--success-ink); background:var(--success-bg); }
.result-group-heading { min-width:0; display:flex; align-items:baseline; gap:8px; margin:4px 0 -2px; padding:0 2px 6px; border-bottom:1px solid var(--line-soft); }
.result-group-heading span { color:var(--text-primary); font-size:16px; }
.result-group-heading small { color:var(--text-muted); font-size:12px; font-weight:500; }
.result-scope { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:3px var(--space-3); align-items:center; }.result-scope > strong,.result-scope > span { grid-column:1; }.result-scope > .ui-btn { grid-column:2; grid-row:1 / 3; white-space:nowrap; }
.optimizer-result { min-width:0; display:grid; gap:10px; padding:12px; border-left:3px solid var(--accent); }
.optimizer-result.result-group-0 { border-left-color:var(--success); }
.optimizer-result.result-group-1 { border-left-color:var(--accent); }
.optimizer-result.result-group-2 { border-left-color:var(--warning); }
.optimizer-result > header { min-width:0; display:flex; justify-content:space-between; gap:var(--space-3); align-items:center; }
.optimizer-result > header div { min-width:0; display:grid; gap:1px; }
.optimizer-result > header small { color:var(--accent); font-size:12px; font-weight:var(--fw-bold); }
.optimizer-result > header strong { color:var(--text-primary); font-size:16px; }
.optimizer-result > header span { color:var(--text-muted); font-size:13px; }
.optimizer-result > header .result-score { flex:0 0 auto; display:grid; gap:1px; text-align:right; }
.optimizer-result > header .result-score small { color:var(--text-muted); font-size:12px; font-weight:500; }
.optimizer-result > header .result-score b { color:var(--accent-hover); font-family:var(--font-data); font-size:16px; }
.result-expand-button { flex:0 0 auto; min-height:32px; padding:0 10px; border:1px solid var(--border-soft); border-radius:6px; background:transparent; color:var(--accent); font-size:13px; font-weight:700; cursor:pointer; }
.result-expand-button:hover { border-color:var(--border-strong); background:rgba(139,103,55,.06); }
.result-cap-proof { min-width:0; display:grid; gap:8px; padding:10px; border:1px solid rgba(151,91,37,.28); border-radius:7px; background:rgba(247,232,194,.46); }
.result-cap-proof > header { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:10px; }
.result-cap-proof > header div { min-width:0; display:grid; gap:1px; }
.result-cap-proof > header strong { color:var(--text-primary); font-size:14px; }
.result-cap-proof > header small { color:var(--text-muted); font-size:12px; }
.result-cap-proof > header > b { flex:0 0 auto; color:var(--warning-ink); font-size:12px; }
.result-cap-proof.capped { border-color:rgba(47,108,70,.28); background:rgba(224,239,218,.48); }
.result-cap-proof.capped > header > b { color:var(--success-ink); }
.result-cap-proof > div { min-width:0; display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:6px; }
.result-cap-proof > div > span { min-width:0; display:grid; gap:2px; padding:7px 8px; border:1px solid rgba(177,145,94,.24); border-radius:6px; background:rgba(255,253,247,.56); }
.result-defense-proof { min-width:0; display:grid; gap:8px; padding:10px; border:1px solid rgba(151,91,37,.28); border-radius:7px; background:rgba(247,232,194,.38); }
.result-defense-proof.valid { border-color:rgba(47,108,70,.28); background:rgba(224,239,218,.42); }
.result-defense-proof > header { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:10px; }
.result-defense-proof > header strong { color:var(--text-primary); font-size:14px; }
.result-defense-proof > header b { color:var(--warning-ink); font-size:12px; }
.result-defense-proof.valid > header b { color:var(--success-ink); }
.result-defense-proof > div { min-width:0; display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:6px; }
.result-defense-proof > div > span { min-width:0; display:grid; gap:2px; padding:7px 8px; border:1px solid rgba(177,145,94,.24); border-radius:6px; background:rgba(255,253,247,.56); }
.result-defense-proof small { color:var(--text-muted); font-size:11px; }
.result-defense-proof b { color:var(--text-primary); font-family:var(--font-data); font-size:13px; }
.result-defense-proof > p { display:flex; flex-wrap:wrap; gap:6px; margin:0; color:var(--text-muted); font-size:12px; line-height:1.45; }
.result-defense-proof > p span { padding:3px 6px; border-radius:999px; color:var(--warning-ink); background:var(--warning-bg); }
.result-defense-proof > p span.met { color:var(--success-ink); background:var(--success-bg); }
.result-cap-proof > div small { color:var(--text-muted); font-size:11px; }
.result-cap-proof > div b { overflow:hidden; color:var(--accent-hover); font-family:var(--font-data); font-size:14px; text-overflow:ellipsis; white-space:nowrap; }
.result-cap-proof > p { margin:0; color:var(--text-secondary); font-size:12px; line-height:1.5; }
.result-explanation { margin:0; padding:var(--space-2) var(--space-3); border-left:2px solid var(--border-soft); color:var(--text-muted); font-size:var(--fs-xs); line-height:1.45; }
.optimizer-apply-row { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:10px; align-items:center; padding:9px 10px; border:1px solid var(--accent-border); border-radius:7px; background:var(--accent-soft); }
.optimizer-apply-row span { color:var(--text-secondary); font-size:13px; line-height:1.45; }
.optimizer-apply-row .ui-btn { min-height:36px; white-space:nowrap; }
.result-final-levels { min-width:0; display:grid; gap:7px; padding:9px 10px; border:1px solid var(--line-soft); border-radius:7px; background:rgba(249,242,225,.42); }
.result-final-levels > header { min-width:0; display:flex; align-items:baseline; justify-content:space-between; gap:8px; }
.result-final-levels > header strong { color:var(--text-primary); font-size:14px; }
.result-final-levels > header span { color:var(--text-muted); font-size:12px; }
.target-fulfilment { min-width:0; display:flex; flex-wrap:wrap; gap:5px; align-items:center; padding:7px 8px; border:1px solid rgba(151,91,37,.3); border-radius:6px; background:var(--warning-bg); }
.target-fulfilment > strong { margin-right:auto; color:var(--warning-ink); font-size:var(--fs-xs); }
.target-fulfilment > span { display:inline-flex; align-items:baseline; gap:3px; padding:3px 7px; border:1px solid rgba(151,91,37,.2); border-radius:var(--radius-pill); color:var(--warning-ink); background:rgba(255,255,255,.38); font-size:var(--fs-2xs); }
.target-fulfilment > span i { color:var(--text-muted); font-family:var(--font-data); font-size:10px; font-style:normal; }
.target-fulfilment > span small { margin-left:3px; color:var(--warning-ink); font-size:10px; font-weight:700; }
.target-fulfilment > span.waiting { border-style:dashed; color:var(--text-muted); background:rgba(255,255,255,.24); }
.target-fulfilment > span b { font-family:var(--font-data); }
.target-fulfilment > span em { color:var(--text-muted); font-style:normal; }
.target-fulfilment.complete { border-color:rgba(47,108,70,.3); background:var(--success-bg); }
.target-fulfilment.complete > strong,.target-fulfilment > span.met { color:var(--success-ink); }
.result-level-preview { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(118px,1fr)); gap:6px; }
.result-level-preview > span { min-width:0; display:flex; align-items:baseline; justify-content:space-between; gap:6px; padding:6px 8px; border:1px solid rgba(177,145,94,.22); border-radius:6px; background:rgba(255,255,255,.48); }
.result-level-preview small { min-width:0; overflow:hidden; color:var(--text-secondary); font-size:12px; text-overflow:ellipsis; white-space:nowrap; }
.result-level-preview b { flex:0 0 auto; color:var(--accent-hover); font-family:var(--font-data); font-size:13px; }
.slot-preview-title { min-width:0; display:flex; align-items:baseline; justify-content:space-between; gap:10px; margin:0; padding:2px 1px; }
.slot-preview-title span { color:var(--text-primary); font-size:14px; }
.slot-preview-title small { color:var(--text-muted); font-size:12px; font-weight:500; text-align:right; }
.result-levels { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(110px,1fr)); gap:5px; }
.result-levels span { min-width:0; display:grid; padding:var(--space-2); background:var(--surface-sunken); }
.result-levels small { color:var(--text-muted); font-size:var(--fs-2xs); }
.result-levels b { color:var(--text-primary); font-size:var(--fs-sm); }
.result-levels em { color:var(--warning-ink); font-size:var(--fs-2xs); font-style:normal; }
.result-sigil-grid { min-width:0; display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:8px; }
.result-sigil-slot { position:relative; min-width:0; min-height:88px; display:grid; grid-template-columns:32px minmax(0,1fr); grid-template-rows:minmax(0,1fr) auto; gap:5px 8px; align-items:start; padding:8px 8px 6px 44px; border:1px solid #d1bf98; border-radius:8px; background:#fffdf7; color:var(--text-secondary); overflow:hidden; }
.result-sigil-slot.constructed { border-color:#b48241; background:linear-gradient(145deg,#fff9e8,#f2e3c8); }
.result-sigil-slot.retained { border-style:dashed; background:rgba(255,253,247,.7); }
.result-sigil-slot > i { position:absolute; left:8px; bottom:7px; color:#9b8c72; font:700 12px/1 ui-monospace,monospace; font-style:normal; }
.result-sigil-icon { position:absolute; top:8px; left:7px; width:32px; height:32px; display:grid; place-items:center; overflow:hidden; border:1px solid var(--line-gold); border-radius:7px; background:linear-gradient(145deg,#fbf6eb,#e9dcc5); color:var(--gold); }
.result-sigil-icon img { width:100%; height:100%; object-fit:cover; }
.result-sigil-copy { min-width:0; grid-column:1/-1; display:grid; gap:2px; }
.result-sigil-copy strong { min-width:0; overflow:hidden; color:var(--text-primary); font-size:13px; text-overflow:ellipsis; white-space:nowrap; }
.result-sigil-copy small { min-width:0; overflow:hidden; color:var(--text-muted); font-size:12px; text-overflow:ellipsis; white-space:nowrap; }
.result-sigil-slot > em { grid-column:1/-1; justify-self:end; color:#8a6a3f; font-size:12px; font-style:normal; white-space:nowrap; }
.result-sigil-slot.empty { display:flex; flex-direction:column; align-items:center; justify-content:center; gap:2px; padding:8px; border-style:dashed; background:repeating-linear-gradient(-45deg,rgba(255,255,255,.4),rgba(255,255,255,.4) 7px,rgba(222,208,176,.15) 7px,rgba(222,208,176,.15) 14px); color:var(--text-muted); text-align:center; }
.result-sigil-slot.empty > i { display:block; }
.empty-slot-mark { color:#8b6737; font-size:22px; line-height:1; }
.result-details { min-width:0; padding:7px 9px; border:1px solid var(--line-soft); border-radius:7px; background:rgba(139,103,55,.035); }
.result-details[open] summary { margin-bottom:8px; }
@container optimizer (max-width:760px) { .result-sigil-grid { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@container optimizer (max-width:660px) { .battle-condition-grid { grid-template-columns:minmax(0,1fr); }.condition-field.is-wide { grid-column:1; grid-template-columns:minmax(108px,.75fr) minmax(112px,1fr); }.character-research-grid { grid-template-columns:minmax(0,1fr); } }
@container optimizer (max-width:560px) { .optimizer-heading { align-items:start; }.skill-target-entry { grid-template-columns:minmax(0,1fr) 112px; }.add-target-button { grid-column:1/-1; }.optimizer-resource-row { grid-template-columns:minmax(0,1fr); }.optimizer-resource-row > small,.optimizer-source-details { grid-column:1; }.chosen-order-copy { align-items:flex-start; flex-direction:column; gap:3px; }.chosen-trait { grid-template-columns:32px 28px minmax(0,1fr) 32px; }.chosen-trait label { grid-column:3; }.chosen-order-actions { grid-column:3; }.chosen-trait > button { grid-column:4; grid-row:1; }.result-scope,.optimizer-apply-row { grid-template-columns:minmax(0,1fr); }.optimizer-apply-row .ui-btn { width:100%; }.slot-preview-title { align-items:flex-start; flex-direction:column; gap:2px; }.slot-preview-title small { text-align:left; }.result-cap-proof > div,.result-defense-proof > div { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@container optimizer (max-width:380px) { .skill-target-entry { grid-template-columns:minmax(0,1fr); }.target-level-field,.add-target-button { grid-column:1; }.optimizer-advanced summary > span { align-items:flex-start; flex-direction:column; gap:2px; }.condition-field { grid-template-columns:minmax(0,1fr); }.condition-control { grid-template-columns:minmax(0,1fr) 48px; }.result-sigil-grid { grid-template-columns:minmax(0,1fr); }.result-group-heading,.result-cap-proof > header,.result-defense-proof > header,.character-research-card > header { align-items:flex-start; flex-direction:column; gap:2px; }.result-final-levels > header { align-items:flex-start; flex-direction:column; gap:2px; } }
</style>
