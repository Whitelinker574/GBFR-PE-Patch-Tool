<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { LoadoutEditContext, LoadoutOptimizerEvidence, LoadoutOptimizerInventorySnapshot, LoadoutSimulateBuild, LoadoutStatContext } from '../../wailsjs/go/backend/App'
import { traitAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'
import { buildCatalogCandidates, buildInventoryCandidates, buildTableExactCandidates, synthesizeOwnedFirstSuggestion } from '../loadoutOptimizer'
import { characterLoadoutProfile, LOADOUT_CHARACTER_PROFILE_VERSION, LOADOUT_DIRECTIONS, LOADOUT_SCENARIO_VERSION } from '../loadoutScenarioConfig.js'
import { sigilAtlasStore } from '../sigilAtlasStore'
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
const evidence = ref({ traits: [], dataVersion: '', formulaVersion: '' })
const statContext = ref(null)
const fixedSimulation = ref(null)
const loading = ref(false)
const solving = ref(false)
const domain = ref('all')
const profile = ref('custom')
const selected = ref([])
const targetLevels = ref({})
const customSelected = ref([])
const customTargetLevels = ref({})
const pendingTraitId = ref('')
const pendingTraitLevel = ref(15)
const results = ref([])
const solved = ref(false)
const coverage = ref(100)
const coverageLow = ref(85)
const coverageHigh = ref(100)
const currentHp = ref(100)
const odStage = ref(0)
const berserk = ref(false)
const actionRate = ref(1)
const minimumHp = ref(0)
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
const resolvedDirection = computed(() => profile.value === 'character' ? characterProfile.value.defaultDirection : profile.value)
const directionProfile = computed(() => LOADOUT_DIRECTIONS[resolvedDirection.value] || LOADOUT_DIRECTIONS.normal)
const combatMode = computed(() => profile.value !== 'custom' && evidence.value.traits?.length > 0 && !!fixedSimulation.value?.finalStats)
const optimizerIntent = computed(() => profile.value === 'custom' ? 'skills' : 'auto')
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
const availableTraits = computed(() => (atlas.value.traits || []).map(trait => ({
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
  return { ...trait, weight: Math.max(1, 4 - index), cap: requested, targetLevel: requested }
}).filter(Boolean))
const coverageInputsValid = computed(() => {
  if (!combatMode.value) return true
  const low = Number(coverageLow.value)
  const high = Number(coverageHigh.value)
  return Number.isFinite(low) && Number.isFinite(high) && low >= 0 && high <= 100 && low <= high
})
const canSolve = computed(() => coverageInputsValid.value && (combatMode.value || chosen.value.length > 0) && (domain.value !== 'inventory' || (props.savePath && props.charaHash)))
const displayResults = computed(() => [...results.value]
  .sort(compareDisplayResults)
  .slice(0, 10))
const primaryResult = computed(() => {
  return displayResults.value[0] || null
})

function icon(trait) { return traitAssetIcon({ internalId: trait?.internalId, hash: trait?.hash, name: trait?.displayName }) }
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
  } else if (selected.value.length < 4) {
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
  } else if (selected.value.length < 4) {
    const trait = atlas.value.traits.find(item => item.internalId === id)
    selected.value = [...selected.value, id]
    targetLevels.value = { ...targetLevels.value, [id]: Math.max(1, Number(trait?.maxLevel || 15)) }
  }
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
  if (combatMode.value) return 1
  const rows = targetFulfilment(result).rows
  const requested = rows.reduce((sum, item) => sum + Number(item.target || 0), 0)
  if (!requested) return 0
  return rows.reduce((sum, item) => sum + Math.min(Number(item.actual || 0), Number(item.target || 0)), 0) / requested
}
function resultGroup(result) {
  const complete = combatMode.value || targetFulfilment(result).complete
  if (!complete) return 2
  return resultConstructionCount(result) > 0 ? 1 : 0
}
function resultSignature(result) {
  return (result?.picked || []).map(item => `${item?.source || ''}:${item?.slotId || 0}:${item?.id || item?.sigilId || item?.name || ''}`).join('|')
}
function compareDisplayResults(left, right) {
  return resultGroup(left) - resultGroup(right)
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
  if (!combatMode.value && !targetFulfilment(result).complete) {
    return tx(`部分满足 · 目标覆盖 ${Math.round(resultCoverage(result) * 100)}% · 使用 ${slots} 槽`, `Partial match · ${Math.round(resultCoverage(result) * 100)}% target coverage · ${slots} slots`)
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
}
function applyProfile(value) {
  cancelSolve()
  profile.value = value
  if (value === 'custom') return
  const configured = value === 'character' ? directionProfile.value.traitIds : profiles[value].traitIds
  const inherited = value === 'character' ? characterProfileNames().map(name => atlas.value.traits.find(item => item.displayName === name || item.displayName.includes(name))?.internalId).filter(Boolean) : []
  const ids = [...configured, ...inherited].filter(id => atlas.value.traits.some(item => item.internalId === id))
  selected.value = [...new Set(ids)].slice(0, 4)
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
  selected.value = [...new Set(ids)].slice(0, 4)
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
    requestId: Date.now(),
  })
}
function targetFulfilment(result) {
  const totals = new Map((result?.totals || []).map(item => [item.name, item]))
  const rows = chosen.value.map(item => {
    const actual = Number(totals.get(item.displayName)?.effective || 0)
    return { name: item.displayName, actual, target: item.targetLevel, met: actual >= item.targetLevel }
  })
  return { rows, complete: rows.length > 0 && rows.every(item => item.met) }
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
  inventory.value = []
  inventorySnapshot.value = null
  if (!props.savePath || !props.charaHash) return
  const [context, snapshot] = await Promise.all([
    LoadoutEditContext(props.savePath, props.charaHash),
    LoadoutOptimizerInventorySnapshot(props.savePath, props.charaHash, Number(props.baseLoadout?.unitId || 0)),
  ])
  inventory.value = context?.sigils || []
  inventorySnapshot.value = snapshot || null
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
  return (evidence.value.traits || []).map(curve => {
    const trait = atlasById.get(curve.traitId)
    return trait ? { name: trait.displayName, weight: 1, cap: curve.maxLevel || trait.maxLevel || 65 } : null
  }).filter(Boolean)
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
    coverageRange: [Math.min(Number(coverageLow.value), Number(coverageHigh.value)) / 100, Math.max(Number(coverageLow.value), Number(coverageHigh.value)) / 100],
    conditionalCurves: reference.conditionalCurves || {}, actionCapEvidence: exactActionCap ? 'table-exact' : 'global-baseline',
    currentHpRatio: Number(currentHp.value) / 100, odStage: Number(odStage.value), berserk: berserk.value,
    disableAttackDefenseInOD: true, minimumHp: Number(minimumHp.value || 0), surviveHits: Number(surviveHits.value || 0),
    baseStats: { attack: Number(final.attack || 1), hp: Number(final.hp || 1), critRate: Number(final.critRate || 0) },
    baseDamageCap: rawCap * (1 + capPercent / 100), comparedCapPercent: capPercent,
    baseUncappedDamage: Math.max(1, Number(final.attack || 1) * rate),
    criticalDamageBonus: Number(reference.damageCalculate?.criticalDamageUpperRate || 20),
    fixedTotals: fixedNonPanelTotals(simulation),
    fixedBonuses: (simulation?.bonuses || []).map(item => ({ traitId: item.traitId, level: item.level })),
    fixedDefenseZones: final.defenseModel?.zones || [],
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
  solving.value = true
  const generation = ++solveGeneration
  try {
    if (domain.value === 'inventory' || domain.value === 'all') await loadInventory()
    if (generation !== solveGeneration) return
    const targets = combatMode.value
      ? combatCandidateTargets()
      : chosen.value.map(item => ({ name: item.displayName, weight: item.weight, cap: item.targetLevel }))
    const candidates = domain.value === 'inventory' ? buildInventoryCandidates(inventory.value, targets, atlas.value) : domain.value === 'catalog' ? buildCatalogCandidates(atlas.value, targets) : domain.value === 'table' ? buildTableExactCandidates(atlas.value, targets) : null
    solveWorker?.terminate()
    const worker = new Worker(new URL('../loadoutOptimizer.worker.js', import.meta.url), { type: 'module' })
    solveWorker = worker
      const resolvedResults = await new Promise((resolve, reject) => {
      rejectSolve = reject
      worker.addEventListener('message', event => {
        if (event.data?.id !== generation) return
        rejectSolve = null
        if (event.data?.error) reject(new Error(event.data.error))
        else resolve(event.data?.results || [])
      }, { once: true })
      worker.addEventListener('error', event => {
        rejectSolve = null
        reject(event.error || new Error(event.message || 'optimizer worker failed'))
      }, { once: true })
      const scenario = {
        character: props.charaName,
        direction: resolvedDirection.value,
        directionProfile: directionProfile.value,
        domain: domain.value,
        scenarioVersion: LOADOUT_SCENARIO_VERSION,
        baseSigils: (props.baseLoadout?.sigils || []).map(item => ({ id: item.hash || item.name, slotId: Number(item.slotId || 0), name: item.name || '因子' })),
        ...(combatMode.value ? combatScenario() : {}),
      }
      const inventoryCandidates = buildInventoryCandidates(inventory.value, targets, atlas.value)
      const catalogCandidates = buildCatalogCandidates(atlas.value, targets)
      const tableCandidates = buildTableExactCandidates(atlas.value, targets)
      const payload = domain.value === 'all'
        ? {
            domains: {
              'owned-first': [...inventoryCandidates, ...catalogCandidates],
              table: tableCandidates,
              catalog: catalogCandidates,
              inventory: inventoryCandidates,
            },
            targets,
            slotCount: 12,
            limit: 10,
            scenario,
          }
        : {
            candidates,
            targets,
            slotCount: 12,
            limit: 10,
            scenario: domain.value === 'inventory' && combatMode.value ? { ...scenario, ...inventoryCombatScenario(), targets, domain: 'inventory' } : scenario,
          }
      worker.postMessage({ id: generation, payload, solveAllDomains: domain.value === 'all' })
    })
    if (domain.value === 'all' && combatMode.value) {
      const theoretical = resolvedResults.find(item => item.domain === 'catalog' && Number(item.domainRank || 1) === 1)
        || resolvedResults.find(item => item.domain === 'table' && Number(item.domainRank || 1) === 1)
      const ownedFirst = synthesizeOwnedFirstSuggestion({
        desired: theoretical,
        inventoryCandidates: buildInventoryCandidates(inventory.value, targets, atlas.value),
        targets,
      })
      results.value = ownedFirst
        ? [ownedFirst, ...resolvedResults.filter(item => item.domain !== 'owned-first')]
        : resolvedResults
    } else {
      results.value = resolvedResults
    }
    if (generation !== solveGeneration) return
    solved.value = true
  } catch (error) {
    if (error?.message !== 'optimizer.cancelled') emit('status', String(error), 'error')
  } finally {
    if (generation === solveGeneration) {
      solveWorker?.terminate()
      solveWorker = null
      rejectSolve = null
      solving.value = false
    }
  }
}
watch(() => [props.savePath, props.charaHash], () => Promise.all([loadInventory(), loadCombatContext()]))
watch(() => [props.savePath, props.charaHash], cancelSolve)
watch(() => props.baseLoadout?.unitId, async () => { await loadCombatContext(); applyProfile('character'); cancelSolve(); solved.value = false })
watch(() => props.pendingTarget?.requestId, () => applyPendingTarget(props.pendingTarget))
watch(domain, () => { cancelSolve(); solved.value = false })
onMounted(load)
onBeforeUnmount(cancelSolve)
</script>

<template>
  <section class="optimizer-page" :class="{ 'is-embedded': embedded }" :aria-label="tx('配装优化建议', 'Loadout Optimization Suggestions')">
    <header v-if="!embedded" class="optimizer-heading">
      <div><small>{{ evidence.dataVersion || atlas.dataVersion || 'GBFR 2.0.2' }} · {{ tx('只读分析', 'Read-Only Analysis') }}</small><h2>{{ tx('配装优化建议', 'Loadout Optimization Suggestions') }}</h2><p>{{ tx('角色方向使用审计公式、独立伤害上限、覆盖率、OD/狂暴和生存约束计算 Top 10；自定义模式仍保留精确技能覆盖求解。', 'Character directions calculate a Top 10 with audited formulas, independent caps, uptime, OD/berserk, and survival constraints. Custom mode keeps the exact trait-coverage solver.') }}</p></div>
      <span class="optimizer-character">{{ charaName || tx('未选择角色', 'No Character Selected') }}<small v-if="baseLoadout">{{ baseLoadout.name || tx('当前配装', 'Current Loadout') }}</small></span>
    </header>

    <div class="optimizer-setup">
      <section :class="['optimizer-controls', { 'ui-card': !embedded, 'is-flat': !embedded }]">
        <div class="optimizer-intent" role="tablist" :aria-label="tx('智能配装方式', 'Smart Loadout Method')">
          <button type="button" role="tab" :aria-selected="optimizerIntent === 'auto'" :class="{ active: optimizerIntent === 'auto' }" @click="selectOptimizerIntent('auto')">
            {{ tx('自动推荐最好一套', 'Recommend Best Set') }}
          </button>
          <button type="button" role="tab" :aria-selected="optimizerIntent === 'skills'" :class="{ active: optimizerIntent === 'skills' }" @click="selectOptimizerIntent('skills')">
            {{ tx('指定技能与等级', 'Choose Skills and Levels') }}
          </button>
        </div>

        <div v-if="optimizerIntent === 'skills'" class="skill-target-workflow">
          <div class="skill-target-entry" aria-label="添加技能目标">
            <label class="target-skill-field"><span>{{ tx('目标技能', 'Target Skill') }}</span><CatalogSelect v-model="pendingTraitId" :options="availableTraits" :icon-resolver="icon" detail-key="levelHint" :placeholder="tx('搜索并选择技能', 'Search and choose a skill')" :search-placeholder="tx('输入技能名称、拼音或 ID', 'Enter a skill name or ID')" @pick="onPendingTraitPick" /></label>
            <label class="target-level-field"><span>{{ tx('目标等级', 'Target Level') }}</span><input v-model.number="pendingTraitLevel" class="ui-input" type="number" min="1" :max="pendingTrait?.maxLevel || 65" @change="clampPendingTraitLevel" /></label>
            <button type="button" class="add-target-button ui-btn is-primary" :disabled="!pendingTraitId || (!selected.includes(pendingTraitId) && selected.length >= 4)" @click="addPendingTrait">{{ selected.includes(pendingTraitId) ? tx('更新目标', 'Update Target') : tx('加入目标', 'Add Target') }}</button>
          </div>
          <div class="chosen-traits">
            <small>{{ tx('已选目标（最多 4 项，拖动不参与排序；显示顺序就是优先级）', 'Selected targets (up to 4; display order is priority)') }}</small>
            <div>
              <article v-for="(trait, index) in chosen" :key="trait.internalId" class="chosen-trait">
                <img v-if="icon(trait)" :src="icon(trait)" alt="" />
                <b>{{ index + 1 }}</b>
                <span>{{ trait.displayName }}</span>
                <label><small>Lv</small><input :value="trait.targetLevel" class="ui-input" type="number" min="1" :max="trait.maxLevel || 65" @change="setTargetLevel(trait.internalId, $event.target.value)" /></label>
                <button type="button" :aria-label="tx(`移除${trait.displayName}`, `Remove ${trait.displayName}`)" @click="chooseTrait(trait.internalId)">×</button>
              </article>
              <span v-if="!chosen.length" class="empty-choice">{{ tx('先选择一个技能和目标等级，再加入目标。', 'Choose a skill and target level, then add it here.') }}</span>
            </div>
          </div>
        </div>

        <div v-else class="auto-recommend-workflow">
          <div class="optimizer-control-group">
            <small>{{ tx('这套配装更看重什么', 'What This Build Prioritizes') }}</small>
            <div class="profile-row"><button v-for="(item, key) in profiles" v-show="key !== 'custom'" :key="key" type="button" class="ui-chip" :class="{ 'is-on': profile === key }" @click="applyProfile(key)">{{ language === 'en' ? item.en : item.zh }}</button></div>
            <p>{{ tx(`默认按${directionProfile.zh}方向，保持当前武器、祝福、召唤石与专精不变，只重新安排 12 个因子槽。`, `Defaults to ${directionProfile.en}; weapon, wrightstone, summons, and mastery stay unchanged while only the 12 sigil slots are planned.`) }}</p>
            <p v-if="!combatMode" class="optimizer-context-warning">{{ tx('当前配装的战斗上下文尚未加载，先按这个方向的技能目标给出覆盖方案；不会把它标成完整伤害最优。', 'Combat context for this build is not loaded, so this ranks target-skill coverage only and does not claim a complete damage optimum.') }}</p>
          </div>
          <details v-if="combatMode" class="optimizer-advanced">
            <summary>{{ tx('高级战斗条件', 'Advanced Combat Conditions') }}</summary>
            <div class="combat-scenario">
              <label><span>{{ tx('触发覆盖率', 'Effect Uptime') }}</span><input v-model.number="coverage" type="range" min="0" max="100" step="5" /><b>{{ coverage }}%</b></label>
              <label><span>{{ tx('覆盖率下限', 'Uptime Lower Bound') }}</span><input v-model.number="coverageLow" class="ui-input" type="number" min="0" max="100" step="5" /><b>%</b></label>
              <label><span>{{ tx('覆盖率上限', 'Uptime Upper Bound') }}</span><input v-model.number="coverageHigh" class="ui-input" type="number" min="0" max="100" step="5" /><b>%</b></label>
              <p v-if="!coverageInputsValid" class="coverage-error" role="alert">{{ tx('请填写 0 到 100 之间的覆盖率，且下限不能高于上限。', 'Enter uptime bounds from 0 to 100, with the lower bound not exceeding the upper bound.') }}</p>
              <label><span>{{ tx('当前 HP', 'Current HP') }}</span><input v-model.number="currentHp" type="range" min="1" max="100" step="1" /><b>{{ currentHp }}%</b></label>
              <label><span>{{ tx('动作倍率', 'Action Rate') }}</span><input v-model.number="actionRate" class="ui-input" type="number" min="0.1" max="9999" step="0.1" /></label>
              <label><span>{{ tx('OD 阶段', 'OD Stage') }}</span><select v-model.number="odStage" class="ui-select"><option :value="0">{{ tx('关闭', 'Off') }}</option><option :value="1">OD 1</option><option :value="2">OD 2+</option></select></label>
              <label class="check"><input v-model="berserk" type="checkbox" /><span>{{ tx('狂暴状态', 'Berserk State') }}</span></label>
              <label><span>{{ tx('最低 HP 约束', 'Minimum HP') }}</span><input v-model.number="minimumHp" class="ui-input" type="number" min="0" step="1000" /></label>
              <label><span>{{ tx('至少承受次数', 'Required Surviving Hits') }}</span><input v-model.number="surviveHits" class="ui-input" type="number" min="0" max="20" step="1" /></label>
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

        <button type="button" class="solve-button ui-btn is-primary" :disabled="!canSolve || loading || solving" @click="solve">{{ solving ? tx('正在计算…', 'Calculating…') : optimizerIntent === 'auto' ? tx('计算最好的一套因子', 'Find the Best Sigil Set') : tx('按目标等级生成方案', 'Build Plans for These Levels') }}</button>
      </section>

      <section class="optimizer-results">
        <div v-if="solved && primaryResult" class="result-scope ui-notice is-info"><strong>{{ tx('结果先按是否满足目标，再按是否需要制造排序', 'Results prioritize target completion, then whether creation is needed') }}</strong><span>{{ tx('这里只预览和回填当前配装的 12 个因子槽；武器、祝福、召唤石、专精和永久成长不会跟着改变。', 'This only previews and fills the current build’s 12 sigil slots. Weapon, wrightstone, summons, mastery, and permanent growth stay unchanged.') }}</span></div>
        <p v-if="!solved" class="optimizer-empty ui-empty">{{ optimizerIntent === 'skills' ? tx('把需要的技能和等级加入目标，然后点击“按目标等级生成方案”。', 'Add the skills and levels you need, then build plans for those targets.') : tx('选择配装方向后点击“计算最好的一套因子”。', 'Choose a build direction, then find the best sigil set.') }}</p>
        <p v-else-if="!results.length || !results[0].picked.length" class="optimizer-empty ui-empty">{{ tx('当前范围找不到可用于这 12 个槽的因子组合。', 'No sigil plan is available for these 12 slots in the selected source.') }}</p>
        <template v-for="(result, index) in displayResults" v-else :key="`${result.domain}-${result.domainRank || index + 1}-${result.score}`">
          <h3 v-if="index === 0 || resultGroup(displayResults[index - 1]) !== resultGroup(result)" class="result-group-heading"><span>{{ resultGroupLabel(result) }}</span><small>{{ resultGroup(result) === 0 ? tx('无需制造，直接回填背包实例', 'No creation; fills owned instances directly') : resultGroup(result) === 1 ? tx('目标满足，但背包存在缺口', 'Targets met, with inventory gaps') : tx('当前来源无法完全满足目标', 'Selected source cannot fully meet every target') }}</small></h3>
          <article class="optimizer-result ui-card is-flat" :class="`result-group-${resultGroup(result)}`">
          <header><div><small>{{ tx(`方案 ${index + 1}`, `Plan ${index + 1}`) }} · {{ domainLabel(result.domain) }}</small><strong>{{ resultGroupLabel(result) }}</strong><span>{{ resultRankReason(result) }}</span></div><span class="result-score"><small>{{ result.combat ? tx('预估伤害', 'Estimated Damage') : tx('目标覆盖得分', 'Target Coverage Score') }}</small><b>{{ result.combat ? Math.round(result.combat.metrics.finalDamage).toLocaleString() : result.score }}</b></span></header>
          <div v-if="!combatMode && targetFulfilment(result).rows.length" class="target-fulfilment" :class="{ complete: targetFulfilment(result).complete }"><strong>{{ targetFulfilment(result).complete ? tx('目标技能已全部满足', 'All Skill Targets Met') : tx('仍有技能等级缺口', 'Some Skill Targets Are Short') }}</strong><span v-for="item in targetFulfilment(result).rows" :key="item.name" :class="{ met: item.met }">{{ item.name }} <b>Lv{{ item.actual }}</b><em>/ {{ item.target }}</em></span></div>
          <div class="result-sigil-grid" :aria-label="tx('十二个因子槽预览', 'Twelve-slot sigil preview')">
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
            <div class="result-levels"><span v-for="total in result.totals" :key="total.name"><small>{{ total.name }}</small><b>Lv{{ total.effective }}</b><em v-if="total.level > total.effective">+{{ total.level - total.effective }} {{ tx('溢出', 'overflow') }}</em></span></div>
            <p v-if="result.explanation" class="result-explanation"><b>{{ domainLabel(result.domain) }}</b> · {{ result.explanation.summary }}{{ language === 'en' ? `; ${result.explanation.limitationEn}` : `；${result.explanation.limitation}` }}<br><small>{{ language === 'en' ? result.explanation.inventoryReasonEn : result.explanation.inventoryReason }}</small></p>
          </details>
          <div v-if="baseLoadout" class="optimizer-apply-row"><span>{{ tx('点击后只回填当前草稿的 12 个因子槽，并自动切回手动页面给你逐槽核对；此时不会改存档。', 'This fills only the draft’s 12 sigil slots and returns to the manual view for slot-by-slot review. The save is not changed yet.') }}</span><button type="button" class="ui-btn is-primary" @click="applyResult(result)">{{ tx('应用到当前配装草稿', 'Apply to Current Draft') }}</button></div>
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
.optimizer-control-group p { margin:0; color:var(--text-muted); font-size:13px; line-height:1.5; }
.optimizer-control-group .optimizer-context-warning { padding:7px 9px; border-left:3px solid var(--warning); background:var(--warning-bg); color:var(--warning-ink); font-size:12px; }
.skill-target-workflow,.auto-recommend-workflow { min-width:0; display:grid; gap:10px; }
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
.optimizer-source-details summary,.optimizer-advanced summary,.result-details summary { width:max-content; color:#765126; font-size:12px; font-weight:700; cursor:pointer; }
.optimizer-source-details > .ui-seg { margin-top:8px; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:6px; }
.optimizer-advanced { min-width:0; padding:8px 10px; border:1px solid var(--line-soft); border-radius:7px; background:rgba(139,103,55,.04); }
.optimizer-advanced[open] summary { margin-bottom:8px; }
.combat-scenario { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); padding:var(--space-3); border:1px solid var(--accent-border); background:var(--accent-soft); }
.combat-scenario label { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) minmax(88px,.8fr) auto; gap:var(--space-2); align-items:center; color:var(--text-secondary); font-size:var(--fs-xs); }
.combat-scenario label > span { overflow-wrap:anywhere; }
.combat-scenario label > b { min-width:3rem; color:var(--accent-hover); font-family:var(--font-data); text-align:right; }
.combat-scenario label.check { grid-template-columns:auto minmax(0,1fr); }
.combat-scenario input[type="range"] { min-width:0; width:100%; accent-color:var(--accent); }
.combat-scenario .coverage-error { grid-column:1 / -1; margin:0; color:var(--danger-ink); font-size:var(--fs-2xs); }
.profile-row { display:flex; flex-wrap:wrap; gap:6px; }
.profile-row .ui-chip { min-height:34px; padding:0 10px; font-size:13px; }
.chosen-traits > div { min-width:0; display:grid; gap:6px; }
.chosen-trait { min-width:0; min-height:44px; display:grid; grid-template-columns:32px 20px minmax(92px,1fr) minmax(86px,112px) 32px; align-items:center; gap:8px; padding:5px 8px; border:1px solid var(--accent-border); border-left:3px solid var(--accent); border-radius:7px; background:linear-gradient(90deg,var(--accent-soft),rgba(255,253,247,.72)); color:var(--text-secondary); }
.chosen-trait img { width:32px; height:32px; border-radius:6px; object-fit:cover; }
.chosen-trait > b { color:var(--accent); font-family:var(--font-data); font-size:12px; }
.chosen-trait span { overflow-wrap:anywhere; color:var(--text-primary); font-size:14px; font-weight:700; }
.chosen-trait label { min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr); gap:4px; align-items:center; }
.chosen-trait label small { color:var(--accent-hover); font-size:12px; font-weight:var(--fw-bold); }
.chosen-trait label input { width:100%; min-height:34px; padding:2px 8px; font-family:var(--font-data); font-weight:var(--fw-bold); }
.chosen-trait > button { width:32px; height:32px; border:0; border-radius:50%; color:var(--text-muted); background:transparent; cursor:pointer; }
.chosen-trait > button:hover,.chosen-trait > button:focus-visible { color:var(--danger-ink); background:var(--danger-bg); }
.empty-choice { padding:10px; border:1px dashed var(--line-soft); border-radius:7px; color:var(--text-muted); font-size:13px; text-align:center; }
.solve-button { width:100%; min-height:38px; font-size:14px; }
.optimizer-results { min-width:0; display:grid; gap:var(--space-3); }
.result-group-heading { min-width:0; display:flex; align-items:baseline; gap:8px; margin:4px 0 -2px; padding:0 2px 6px; border-bottom:1px solid var(--line-soft); }
.result-group-heading span { color:var(--text-primary); font-size:16px; }
.result-group-heading small { color:var(--text-muted); font-size:12px; font-weight:500; }
.result-scope { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:3px var(--space-3); align-items:center; }.result-scope > strong,.result-scope > span { grid-column:1; }.result-scope > .ui-btn { grid-column:2; grid-row:1 / 3; white-space:nowrap; }
.optimizer-empty { min-height:180px; }
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
.result-explanation { margin:0; padding:var(--space-2) var(--space-3); border-left:2px solid var(--border-soft); color:var(--text-muted); font-size:var(--fs-xs); line-height:1.45; }
.optimizer-apply-row { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:10px; align-items:center; padding:9px 10px; border:1px solid var(--accent-border); border-radius:7px; background:var(--accent-soft); }
.optimizer-apply-row span { color:var(--text-secondary); font-size:13px; line-height:1.45; }
.optimizer-apply-row .ui-btn { min-height:36px; white-space:nowrap; }
.target-fulfilment { min-width:0; display:flex; flex-wrap:wrap; gap:5px; align-items:center; padding:var(--space-2) var(--space-3); border:1px solid rgba(151,91,37,.3); background:var(--warning-bg); }
.target-fulfilment > strong { margin-right:auto; color:var(--warning-ink); font-size:var(--fs-xs); }
.target-fulfilment > span { display:inline-flex; align-items:baseline; gap:3px; padding:3px 7px; border:1px solid rgba(151,91,37,.2); border-radius:var(--radius-pill); color:var(--warning-ink); background:rgba(255,255,255,.38); font-size:var(--fs-2xs); }
.target-fulfilment > span b { font-family:var(--font-data); }
.target-fulfilment > span em { color:var(--text-muted); font-style:normal; }
.target-fulfilment.complete { border-color:rgba(47,108,70,.3); background:var(--success-bg); }
.target-fulfilment.complete > strong,.target-fulfilment > span.met { color:var(--success-ink); }
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
@container optimizer (max-width:560px) { .optimizer-heading { align-items:start; }.skill-target-entry { grid-template-columns:minmax(0,1fr) 112px; }.add-target-button { grid-column:1/-1; }.optimizer-resource-row { grid-template-columns:minmax(0,1fr); }.optimizer-resource-row > small,.optimizer-source-details { grid-column:1; }.combat-scenario { grid-template-columns:minmax(0,1fr); }.combat-scenario label { grid-template-columns:minmax(0,1fr); }.combat-scenario label > b { text-align:left; }.chosen-trait { grid-template-columns:32px 20px minmax(0,1fr) 32px; }.chosen-trait label { grid-column:3; }.chosen-trait > button { grid-column:4; grid-row:1; }.result-scope,.optimizer-apply-row { grid-template-columns:minmax(0,1fr); }.optimizer-apply-row .ui-btn { width:100%; } }
@container optimizer (max-width:380px) { .skill-target-entry { grid-template-columns:minmax(0,1fr); }.target-level-field,.add-target-button { grid-column:1; }.result-sigil-grid { grid-template-columns:minmax(0,1fr); }.result-group-heading { align-items:flex-start; flex-direction:column; gap:2px; } }
</style>
