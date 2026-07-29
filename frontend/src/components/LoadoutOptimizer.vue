<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { LoadoutEditContext, LoadoutOptimizerEvidence, LoadoutOptimizerInventorySnapshot, LoadoutSimulateBuild, LoadoutStatContext } from '../../wailsjs/go/backend/App'
import { traitAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'
import { buildCatalogCandidates, buildInventoryCandidates, buildTableExactCandidates } from '../loadoutOptimizer'
import { characterLoadoutProfile, LOADOUT_CHARACTER_PROFILE_VERSION, LOADOUT_DIRECTIONS, LOADOUT_SCENARIO_VERSION } from '../loadoutScenarioConfig.js'
import { sigilAtlasStore } from '../sigilAtlasStore'
import { matchText } from '../utils/matchText'

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
const query = ref('')
const domain = ref('all')
const profile = ref('character')
const selected = ref([])
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
const domainLabel = domain => ({ all: tx('三域并算', 'All Domains'), inventory: tx('当前存档可部署', 'Deployable Save'), catalog: tx('工具目录合法构造', 'Tool Catalog'), table: tx('游戏表基线', 'Game Table Baseline') }[domain] || domain)
const equipmentStageLabel = stage => ({ weapon: tx('武器', 'Weapon'), wrightstone: tx('绑定祝福', 'Bound Wrightstone'), summons: tx('召唤石', 'Summons'), mastery: tx('专精', 'Mastery') }[stage] || stage)
const alternativeGroupLabel = group => ({
  primary: tx('本域首选', 'Domain Primary'),
  'least-change': tx('最少改动', 'Least Changes'),
  'preserve-equipment': tx('保留当前装备', 'Keep Current Gear'),
  'robust-coverage': tx('覆盖率稳健', 'Uptime Robust'),
}[group] || group)
const profiles = {
  character: { zh: '当前角色 · 本配装', en: 'Character & Current Preset', traitIds: [] },
  ...Object.fromEntries(Object.entries(LOADOUT_DIRECTIONS).map(([key, item]) => [key, { zh: item.zh, en: item.en, traitIds: item.traitIds }])),
  custom: { zh: '自定义', en: 'Custom', traitIds: [] },
}
const characterProfile = computed(() => characterLoadoutProfile(props.charaHash))
const resolvedDirection = computed(() => profile.value === 'character' ? characterProfile.value.defaultDirection : profile.value)
const directionProfile = computed(() => LOADOUT_DIRECTIONS[resolvedDirection.value] || LOADOUT_DIRECTIONS.normal)
const combatMode = computed(() => profile.value !== 'custom' && evidence.value.traits?.length > 0 && !!fixedSimulation.value?.finalStats)
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
const availableTraits = computed(() => (atlas.value.traits || []).filter(trait => matchText(`${trait.displayName} ${trait.internalId} ${trait.hash}`, query.value)).slice(0, 120))
const chosen = computed(() => selected.value.map((id, index) => {
  const trait = atlas.value.traits.find(item => item.internalId === id)
  return trait ? { ...trait, weight: Math.max(1, 4 - index), cap: trait.maxLevel || 65 } : null
}).filter(Boolean))
const coverageInputsValid = computed(() => {
  if (!combatMode.value) return true
  const low = Number(coverageLow.value)
  const high = Number(coverageHigh.value)
  return Number.isFinite(low) && Number.isFinite(high) && low >= 0 && high <= 100 && low <= high
})
const canSolve = computed(() => coverageInputsValid.value && (combatMode.value || chosen.value.length > 0) && (domain.value !== 'inventory' || (props.savePath && props.charaHash)))
const primaryResult = computed(() => {
  if (!results.value.length) return null
  if (domain.value !== 'all') return results.value[0]
  return results.value.find(item => item.domain === 'inventory' && Number(item.domainRank || 1) === 1)
    || results.value.find(item => item.domain === 'catalog' && Number(item.domainRank || 1) === 1)
    || results.value.find(item => item.domain === 'table' && Number(item.domainRank || 1) === 1)
    || results.value[0]
})

function icon(trait) { return traitAssetIcon({ internalId: trait?.internalId, hash: trait?.hash, name: trait?.displayName }) }
function chooseTrait(id) {
  cancelSolve()
  if (selected.value.includes(id)) selected.value = selected.value.filter(item => item !== id)
  else if (selected.value.length < 4) selected.value = [...selected.value, id]
  solved.value = false
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
  solved.value = false
}
function applyResult(result) {
  if (!result?.picked?.length && !result?.equipment?.length) return
  emit('apply', {
    result: JSON.parse(JSON.stringify(result)),
    domain: result.domain || domain.value,
    targetUnitId: Number(props.baseLoadout?.unitId || 0),
    profile: profile.value,
    requestId: Date.now(),
  })
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
  const simulation = await LoadoutSimulateBuild(props.savePath, props.charaHash, input.weaponSlotId, [], [], input.mastery, context?.equippedSummonSlotIds || [])
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
    const targets = combatMode.value ? combatCandidateTargets() : chosen.value.map(item => ({ name: item.displayName, weight: item.weight, cap: item.maxLevel || 65 }))
    const candidates = domain.value === 'inventory' ? buildInventoryCandidates(inventory.value, targets, atlas.value) : domain.value === 'catalog' ? buildCatalogCandidates(atlas.value, targets) : domain.value === 'table' ? buildTableExactCandidates(atlas.value, targets) : null
    solveWorker?.terminate()
    const worker = new Worker(new URL('../loadoutOptimizer.worker.js', import.meta.url), { type: 'module' })
    solveWorker = worker
      results.value = await new Promise((resolve, reject) => {
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
      if (domain.value === 'inventory' && combatMode.value) {
        worker.postMessage({
          id: generation,
          solveEquipmentAware: true,
          payload: {
            snapshot: inventorySnapshot.value,
            sigilCandidates: candidates,
            sigilSlotCount: 12,
            limit: 10,
            scenario: { ...scenario, ...inventoryCombatScenario(), targets, domain: 'inventory' },
          },
        })
      } else if (domain.value === 'all' && combatMode.value) {
        worker.postMessage({
          id: generation,
          solveMixedDomains: true,
          payload: {
            inventorySnapshot: inventorySnapshot.value,
            domains: { table: buildTableExactCandidates(atlas.value, targets), catalog: buildCatalogCandidates(atlas.value, targets) },
            sigilCandidatesByDomain: { inventory: buildInventoryCandidates(inventory.value, targets, atlas.value) },
            targets,
            sigilSlotCount: 12,
            limit: 10,
            scenario,
            inventoryScenario: inventoryCombatScenario(),
          },
        })
      } else {
        const payload = domain.value === 'all'
          ? { domains: { table: buildTableExactCandidates(atlas.value, targets), catalog: buildCatalogCandidates(atlas.value, targets), inventory: buildInventoryCandidates(inventory.value, targets, atlas.value) }, targets, slotCount: 12, limit: 10, scenario }
          : { candidates, targets, slotCount: 12, limit: 10, scenario }
        worker.postMessage({ id: generation, payload, solveAllDomains: domain.value === 'all' })
      }
    })
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
    <header class="optimizer-heading">
      <div><small>{{ evidence.dataVersion || atlas.dataVersion || 'GBFR 2.0.2' }} · {{ tx('只读分析', 'Read-Only Analysis') }}</small><h2>{{ tx('配装优化建议', 'Loadout Optimization Suggestions') }}</h2><p>{{ tx('角色方向使用审计公式、独立伤害上限、覆盖率、OD/狂暴和生存约束计算 Top 10；自定义模式仍保留精确技能覆盖求解。', 'Character directions calculate a Top 10 with audited formulas, independent caps, uptime, OD/berserk, and survival constraints. Custom mode keeps the exact trait-coverage solver.') }}</p></div>
      <span class="optimizer-character">{{ charaName || tx('未选择角色', 'No Character Selected') }}<small v-if="baseLoadout">{{ baseLoadout.name || tx('当前配装', 'Current Loadout') }}</small></span>
    </header>

    <div class="optimizer-setup">
      <section class="optimizer-controls ui-card is-flat">
        <div class="optimizer-control-group"><small>{{ tx('数据范围', 'Data Domain') }}</small><div class="ui-seg"><button type="button" class="ui-seg-btn" :class="{ 'is-on': domain === 'all' }" :disabled="!savePath" @click="domain = 'all'">{{ tx('三域并算', 'All Three Domains') }}</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': domain === 'inventory' }" :disabled="!savePath" @click="domain = 'inventory'">{{ tx('当前存档', 'Current Save') }}</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': domain === 'catalog' }" @click="domain = 'catalog'">{{ tx('工具目录', 'Tool Catalog') }}</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': domain === 'table' }" @click="domain = 'table'">{{ tx('游戏表基线', 'Game Table Baseline') }}</button></div><p>{{ domain === 'all' ? tx('同时计算游戏表基线、工具目录和当前存档可部署三种结果；表域只使用明确标记的解包表行。', 'Computes the game-table baseline, tool catalog, and deployable-save domains together; the table domain only uses explicitly marked unpacked rows.') : domain === 'inventory' ? tx(`读取当前存档可识别实例 ${inventory.length} 个，同一方案不会复用同一个槽位；因子可能也被其他预设引用，载入后请在目标槽确认。`, `Uses ${inventory.length} recognized instances from the current save without reusing a slot. A sigil may also be referenced by another preset; verify the target slot after loading.`) : domain === 'table' ? tx('只使用明确标记为表域精确行的数据；没有证据时显示为空。', 'Uses only rows explicitly marked as table-exact; an empty result is intentional when evidence is missing.') : tx('允许重复使用目录中的合法构造组合，不声称代表实际库存。', 'Allows repeated legal catalog combinations and does not claim actual ownership.') }}</p></div>
        <div class="optimizer-control-group"><small>{{ tx('角色方向', 'Character Direction') }} · {{ LOADOUT_CHARACTER_PROFILE_VERSION }}</small><div class="profile-row"><button v-for="(item, key) in profiles" :key="key" type="button" class="ui-chip" :class="{ 'is-on': profile === key }" @click="applyProfile(key)">{{ language === 'en' ? item.en : item.zh }}</button></div><p v-if="profile === 'character'">{{ tx(`当前角色默认方向：${directionProfile.zh}。这是按角色定位整理的启发式配置，不是实战标定结论。当前存档域会联合比较现有武器、绑定祝福、召唤石、已保存专精和 12 个因子槽；目录域与表域只比较因子，固定使用当前装备。`, `Current default: ${directionProfile.en}. This is a role-level heuristic profile, not a field-calibrated result. The deployable-save domain jointly compares owned weapons, bound wrightstones, summons, saved mastery presets, and 12 sigil slots. Catalog and table domains compare sigils only with current equipment fixed.`) }} <small>{{ characterProfile.provenance }} · {{ characterProfile.completeness }}</small></p></div>
        <div v-if="combatMode" class="combat-scenario">
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
        <div v-if="!combatMode" class="chosen-traits"><small>{{ tx('目标技能（最多 4 项，顺序代表优先级）', 'Target Traits (up to 4, ordered by priority)') }}</small><div><button v-for="(trait, index) in chosen" :key="trait.internalId" type="button" class="chosen-trait" @click="chooseTrait(trait.internalId)"><img v-if="icon(trait)" :src="icon(trait)" alt="" /><b>{{ index + 1 }}</b><span>{{ trait.displayName }}</span><em>×{{ trait.weight }}</em></button><span v-if="!chosen.length" class="empty-choice">{{ tx('从下方目录选择目标技能', 'Choose target traits below') }}</span></div></div>
        <label v-if="!combatMode" class="trait-search"><span>{{ tx('查找技能', 'Find Trait') }}</span><input v-model="query" class="ui-input" :placeholder="tx('名称、拼音、ID 或 Hash', 'Name, pinyin, ID, or hash')" /></label>
        <div v-if="!combatMode" class="trait-choice-grid"><button v-for="trait in availableTraits" :key="trait.internalId" type="button" class="trait-choice" :class="{ selected: selected.includes(trait.internalId) }" :disabled="!selected.includes(trait.internalId) && selected.length >= 4" @click="chooseTrait(trait.internalId)"><img v-if="icon(trait)" :src="icon(trait)" alt="" loading="lazy" /><span>{{ trait.displayName }}</span><small>Lv{{ trait.maxLevel || '—' }}</small></button></div>
        <button type="button" class="solve-button ui-btn is-primary" :disabled="!canSolve || loading || solving" @click="solve">{{ solving ? tx('正在计算…', 'Calculating…') : combatMode ? tx('计算三域 Top 10', 'Calculate Three-Domain Top 10') : tx('生成替代方案', 'Generate Suggestions') }}</button>
      </section>

      <section class="optimizer-results">
        <div class="result-scope ui-notice" :class="combatMode ? 'is-info' : primaryResult?.exact ? 'is-info' : 'is-warning'"><strong>{{ combatMode ? tx('当前结果：各数据域公式模型 Top 10', 'Current Result: Formula Top 10 per Domain') : primaryResult?.exact ? tx('当前结果：精确技能覆盖方案', 'Current Result: Exact Trait-Coverage Plans') : tx('当前结果：技能覆盖启发式建议', 'Current Result: Heuristic Trait-Coverage Suggestions') }}</strong><span>{{ combatMode ? tx('当前存档域联合计算可部署装备与因子；目录域和表域只比较因子。每个数据域独立排名，未命中表节点或缺少实测概率的效果会排除并披露，因此结果是可审计模型而非实战保证。', 'The deployable-save domain jointly evaluates equipment and sigils; catalog and table domains compare sigils only. Domains rank independently, and effects missing table nodes or measured proc rates are excluded and disclosed, so results are an auditable model rather than a field guarantee.') : primaryResult?.exact ? tx('各数据域分别穷尽当前目标可达状态，并精确遵守槽位、库存实例、目录组合和技能等级上限。', 'Each data domain independently exhausts reachable states while enforcing slots, inventory instances, catalog combinations, and trait caps.') : tx('精确状态超过安全预算后已明确降级。', 'The exact state space exceeded the safety budget and explicitly fell back to a heuristic.') }}</span><button v-if="baseLoadout && (primaryResult?.picked?.length || primaryResult?.equipment?.length)" type="button" class="ui-btn is-primary" @click="applyResult(primaryResult)">{{ tx('一键载入首选方案', 'Load Primary Suggestion') }}</button></div>
        <p v-if="!solved" class="optimizer-empty ui-empty">{{ tx('选择目标后生成方案。结果会列出每个因子和各技能达到的有效等级。', 'Choose targets and generate suggestions. Results list every sigil and each effective trait level.') }}</p>
        <p v-else-if="!results.length || (!results[0].picked.length && !results[0].equipment?.length)" class="optimizer-empty ui-empty">{{ tx('当前范围找不到可部署的装备或因子组合。', 'No deployable equipment or sigil combination was found in this domain.') }}</p>
        <article v-for="(result, index) in results" v-else :key="`${result.domain}-${result.domainRank || index + 1}-${result.score}`" class="optimizer-result ui-card is-flat">
          <header><div><small>{{ Number(result.domainRank || index + 1) === 1 ? tx(`${domainLabel(result.domain)}首选`, `${domainLabel(result.domain)} Primary`) : tx(`${domainLabel(result.domain)}替代方案 ${result.domainRank || index}`, `${domainLabel(result.domain)} Alternative ${result.domainRank || index}`) }} · {{ result.method === 'combat-beam' ? tx(`评估 ${result.exploredStates} 个公式状态`, `${result.exploredStates} formula states evaluated`) : result.exact ? tx(`精确检查 ${result.exploredStates} 个状态`, `${result.exploredStates} exact states checked`) : tx('启发式降级', 'Heuristic fallback') }}</small><strong>{{ result.picked.length }} / 12 {{ tx('个因子', 'sigils') }}</strong><span v-if="result.alternativeGroups?.length" class="result-tags"><em v-for="group in result.alternativeGroups" :key="group">{{ alternativeGroupLabel(group) }}</em></span></div><b>{{ result.combat ? Math.round(result.combat.metrics.finalDamage).toLocaleString() : result.score }}</b></header>
          <p v-if="result.explanation" class="result-explanation"><b>{{ domainLabel(result.domain) }}</b> · {{ result.explanation.summary }}{{ language === 'en' ? `; ${result.explanation.limitationEn}` : `；${result.explanation.limitation}` }}<br><small>{{ result.explanation.comparisonBasis === 'domain-primary' ? tx(`相对本域第一名（分差 ${result.explanation.scoreDelta.toFixed(0)}）：`, `Versus this domain's primary (score delta ${result.explanation.scoreDelta.toFixed(0)}): `) : tx('相对当前配装：', 'Versus current loadout: ') }}{{ (language === 'en' ? result.explanation.slotChangesEn : result.explanation.slotChanges) || tx('无', 'None') }}</small><br><small v-if="result.coverageSensitivity">{{ tx(`覆盖率 ${(result.coverageSensitivity.low * 100).toFixed(0)}%–${(result.coverageSensitivity.high * 100).toFixed(0)}%：第 ${result.coverageSensitivity.lowRank} / ${result.coverageSensitivity.highRank} 名，${result.coverageSensitivity.stable ? '排名稳定' : '对覆盖率敏感'}`, `Uptime ${(result.coverageSensitivity.low * 100).toFixed(0)}%–${(result.coverageSensitivity.high * 100).toFixed(0)}%: rank ${result.coverageSensitivity.lowRank} / ${result.coverageSensitivity.highRank}; ${result.coverageSensitivity.stable ? 'stable ranking' : 'uptime-sensitive'}`) }}</small><br><small v-if="result.explanation.metricChanges?.length">{{ tx('乘区变化：', 'Bucket changes: ') }}{{ result.explanation.metricChanges.map(item => `${item.label} ${item.delta > 0 ? '+' : ''}${item.delta.toFixed(2)}`).join(' · ') }}</small><br v-if="result.explanation.metricChanges?.length"><small>{{ language === 'en' ? result.explanation.inventoryReasonEn : result.explanation.inventoryReason }} · {{ result.explanation.scenarioSummary }}</small><br><small v-if="result.explanation.formulaEvidence">{{ tx('公式证据', 'Formula evidence') }} {{ result.explanation.evidenceSource }} · {{ tx('场景倍率', 'scenario multiplier') }} ×{{ result.explanation.formulaEvidence.stateMultiplier.toFixed(3) }} · {{ tx('稳定触顶余量', 'stable-cap margin') }} {{ (result.explanation.formulaEvidence.stableCapMargin * 100).toFixed(3) }}%</small></p>
          <div v-if="result.equipment?.length" class="result-equipment"><span v-for="item in result.equipment" :key="`${item.stage}-${item.id}`"><small>{{ equipmentStageLabel(item.stage) }}</small><b>{{ item.label || item.id }}</b></span><em v-if="result.unresolvedAtoms?.length">{{ tx(`有 ${result.unresolvedAtoms.length} 项效果缺少审计证据，未计入评分`, `${result.unresolvedAtoms.length} effects lack audited evidence and were excluded from scoring`) }}</em></div>
          <div v-if="baseLoadout" class="optimizer-apply-row"><span>{{ result.equipment?.length ? tx('将所列库存武器、其绑定祝福、四个召唤石、专精与因子一并载入草稿；固定角色强化不会被覆盖。请核对目标槽后保存。', 'Loads the listed owned weapon, its bound wrightstone, four summons, mastery, and sigils into the draft. Permanent character growth is not overwritten. Review the target slot before saving.') : tx('建议因子放入前部；不足 12 个时沿用这套配装未重复的剩余因子。载入后仍需在编辑器核对目标槽并保存。', 'Suggested sigils are placed first; when fewer than 12 are suggested, non-duplicate remaining sigils stay from this preset. Review the target slot and save in the editor.') }}</span><button type="button" class="ui-btn is-primary" @click="applyResult(result)">{{ tx(`载入到 ${charaName || '当前角色'} · ${baseLoadout.name || '当前配装'}`, `Load into ${charaName || 'character'} · ${baseLoadout.name || 'current preset'}`) }}</button></div>
          <div class="result-levels"><span v-for="total in result.totals" :key="total.name"><small>{{ total.name }}</small><b>Lv{{ total.effective }}</b><em v-if="total.level > total.effective">+{{ total.level - total.effective }} {{ tx('溢出', 'overflow') }}</em></span></div>
          <ol class="result-sigils"><li v-for="(item, slot) in result.picked" :key="`${item.id}-${slot}`"><b>{{ String(slot + 1).padStart(2, '0') }}</b><span><strong>{{ item.name }}</strong><small>{{ item.traits.map(trait => `${trait.name} Lv${trait.level}`).join(' · ') }}</small></span><em v-if="item.slotId">#{{ item.slotId }}</em></li></ol>
        </article>
      </section>
    </div>
  </section>
</template>

<style scoped>
.optimizer-page { min-width:0; display:grid; gap:var(--space-4); container:optimizer / inline-size; }
.optimizer-heading { min-width:0; display:flex; align-items:end; justify-content:space-between; gap:var(--space-4); padding:2px var(--space-2); border-bottom:1px solid var(--border-soft); }
.optimizer-heading > div { min-width:0; }
.optimizer-heading small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.optimizer-heading h2 { margin:2px 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); letter-spacing:0; }
.optimizer-heading p { max-width:80ch; margin:0 0 var(--space-3); color:var(--text-muted); font-size:var(--fs-sm); }
.optimizer-character { flex:0 0 auto; display:grid; gap:2px; padding:var(--space-2) var(--space-3); border-left:2px solid var(--accent); color:var(--text-secondary); font-size:var(--fs-sm); }
.optimizer-character small { max-width:220px; overflow:hidden; text-overflow:ellipsis; color:var(--text-muted); font-size:var(--fs-2xs); white-space:nowrap; }
.optimizer-page.is-embedded .optimizer-heading { padding-top:var(--space-3); }
.optimizer-setup { min-width:0; display:grid; grid-template-columns:minmax(340px,.82fr) minmax(420px,1.18fr); gap:var(--space-4); align-items:start; }
.optimizer-controls { min-width:0; display:grid; gap:var(--space-4); padding:var(--space-4); }
.optimizer-control-group,.chosen-traits,.trait-search { min-width:0; display:grid; gap:6px; }
.optimizer-control-group > small,.chosen-traits > small,.trait-search > span { color:var(--text-muted); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.optimizer-control-group p { margin:0; color:var(--text-muted); font-size:var(--fs-2xs); line-height:1.45; }
.combat-scenario { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); padding:var(--space-3); border:1px solid var(--accent-border); background:var(--accent-soft); }
.combat-scenario label { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) minmax(88px,.8fr) auto; gap:var(--space-2); align-items:center; color:var(--text-secondary); font-size:var(--fs-xs); }
.combat-scenario label > span { overflow-wrap:anywhere; }
.combat-scenario label > b { min-width:3rem; color:var(--accent-hover); font-family:var(--font-data); text-align:right; }
.combat-scenario label.check { grid-template-columns:auto minmax(0,1fr); }
.combat-scenario input[type="range"] { min-width:0; width:100%; accent-color:var(--accent); }
.combat-scenario .coverage-error { grid-column:1 / -1; margin:0; color:var(--danger-ink); font-size:var(--fs-2xs); }
.profile-row { display:flex; flex-wrap:wrap; gap:6px; }
.chosen-traits > div { min-width:0; display:flex; flex-wrap:wrap; gap:6px; }
.chosen-trait { min-width:0; display:grid; grid-template-columns:24px 18px minmax(0,1fr) auto; align-items:center; gap:5px; padding:3px 7px 3px 3px; border:1px solid var(--accent-border); border-radius:var(--radius-sm); background:var(--accent-soft); color:var(--text-secondary); cursor:pointer; }
.chosen-trait img { width:24px; height:24px; border-radius:4px; object-fit:cover; }
.chosen-trait b,.chosen-trait em { color:var(--accent); font-size:var(--fs-2xs); font-style:normal; }
.chosen-trait span { overflow-wrap:anywhere; font-size:var(--fs-xs); }
.empty-choice { color:var(--text-muted); font-size:var(--fs-xs); }
.trait-choice-grid { max-height:280px; min-width:0; display:grid; grid-template-columns:repeat(auto-fill,minmax(126px,1fr)); gap:5px; overflow:auto; padding:2px; scrollbar-gutter:stable; }
.trait-choice { min-width:0; display:grid; grid-template-columns:26px minmax(0,1fr) auto; align-items:center; gap:5px; min-height:34px; padding:3px 6px 3px 3px; border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); color:var(--text-secondary); text-align:left; cursor:pointer; }
.trait-choice img { width:26px; height:26px; border-radius:4px; object-fit:cover; }
.trait-choice span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:var(--fs-xs); }
.trait-choice small { color:var(--text-muted); font-size:var(--fs-2xs); }
.trait-choice.selected { border-color:var(--accent); background:var(--accent-soft); }
.solve-button { width:100%; }
.optimizer-results { min-width:0; display:grid; gap:var(--space-3); }
.result-scope { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:3px var(--space-3); align-items:center; }.result-scope > strong,.result-scope > span { grid-column:1; }.result-scope > .ui-btn { grid-column:2; grid-row:1 / 3; white-space:nowrap; }
.optimizer-empty { min-height:180px; }
.optimizer-result { min-width:0; display:grid; gap:var(--space-3); padding:var(--space-4); border-left:3px solid var(--accent); }
.optimizer-result > header { min-width:0; display:flex; justify-content:space-between; gap:var(--space-3); align-items:center; }
.optimizer-result > header div { min-width:0; display:grid; gap:1px; }
.optimizer-result > header small { color:var(--accent); font-size:var(--fs-2xs); font-weight:var(--fw-bold); }
.optimizer-result > header strong { color:var(--text-primary); font-size:var(--fs-md); }
.optimizer-result > header > b { color:var(--accent-hover); font-family:var(--font-data); font-size:var(--fs-xl); }
.result-tags { display:flex; flex-wrap:wrap; gap:4px; margin-top:3px; }
.result-tags em { padding:2px 6px; border:1px solid var(--accent-border); border-radius:var(--radius-pill); color:var(--accent-hover); background:var(--accent-soft); font-size:var(--fs-2xs); font-style:normal; font-weight:var(--fw-bold); }
.result-explanation { margin:0; padding:var(--space-2) var(--space-3); border-left:2px solid var(--border-soft); color:var(--text-muted); font-size:var(--fs-xs); line-height:1.45; }
.optimizer-apply-row { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-3); align-items:center; padding:var(--space-3); border:1px solid var(--accent-border); background:var(--accent-soft); }
.optimizer-apply-row span { color:var(--text-secondary); font-size:var(--fs-xs); line-height:1.45; }
.result-levels { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(110px,1fr)); gap:5px; }
.result-levels span { min-width:0; display:grid; padding:var(--space-2); background:var(--surface-sunken); }
.result-levels small { color:var(--text-muted); font-size:var(--fs-2xs); }
.result-levels b { color:var(--text-primary); font-size:var(--fs-sm); }
.result-levels em { color:var(--warning-ink); font-size:var(--fs-2xs); font-style:normal; }
.result-equipment { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(130px,1fr)); gap:5px; }
.result-equipment > span { min-width:0; display:grid; gap:2px; padding:var(--space-2); border:1px solid var(--accent-border); background:var(--accent-soft); }
.result-equipment small { color:var(--text-muted); font-size:var(--fs-2xs); }
.result-equipment b { min-width:0; overflow-wrap:anywhere; color:var(--text-primary); font-size:var(--fs-xs); }
.result-equipment > em { grid-column:1 / -1; color:var(--warning-ink); font-size:var(--fs-2xs); font-style:normal; }
.result-sigils { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:5px; margin:0; padding:0; list-style:none; }
.result-sigils li { min-width:0; display:grid; grid-template-columns:24px minmax(0,1fr) auto; gap:var(--space-2); align-items:center; padding:6px; border-bottom:1px solid var(--border-soft); }
.result-sigils li > b { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-2xs); }
.result-sigils li > span { min-width:0; display:grid; gap:1px; }
.result-sigils strong,.result-sigils small { min-width:0; overflow-wrap:anywhere; }
.result-sigils strong { color:var(--text-primary); font-size:var(--fs-xs); }
.result-sigils small { color:var(--text-muted); font-size:var(--fs-2xs); }
.result-sigils em { color:var(--accent); font-size:var(--fs-2xs); font-style:normal; }
@container optimizer (max-width:900px) { .optimizer-setup { grid-template-columns:minmax(0,1fr); } }
@container optimizer (max-width:520px) { .optimizer-heading { align-items:start; } .combat-scenario { grid-template-columns:minmax(0,1fr); }.combat-scenario label { grid-template-columns:minmax(0,1fr); }.combat-scenario label > b { text-align:left; }.result-scope,.optimizer-apply-row { grid-template-columns:minmax(0,1fr); } .result-scope > .ui-btn { grid-column:1; grid-row:auto; width:100%; } .result-sigils { grid-template-columns:1fr; } .trait-choice-grid { grid-template-columns:repeat(2,minmax(0,1fr)); } }
</style>
