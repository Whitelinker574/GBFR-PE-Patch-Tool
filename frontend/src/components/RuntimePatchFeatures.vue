<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  CharaAcquire,
  CharaRelease,
  CombatTuningGetStatusOwned,
  CombatTuningSetActionSpeedOwned,
  CombatTuningSetChargeOwned,
  CombatTuningSetCooldownOwned,
  ConfluxTimerGetStatusOwned,
  ConfluxTimerSetEnabledOwned,
  ConfluxTimerVerifyStatusOwned,
  RuntimePatchGetCatalog,
  RuntimePatchGetStatusesOwned,
  RuntimePatchReleaseOwned,
  RuntimePatchSetEnabledOwned,
  SummonDurationGetStatusOwned,
  SummonDurationSetOwned,
  TaskRulesGetStatusOwned,
  TaskRulesSetScoreMultiplierOwned,
  TaskRulesSetSideQuestAutoCompleteOwned,
} from '../../wailsjs/go/backend/App'
import {
  buildActionSpeedRequest,
  buildChargeRequest,
  buildCooldownRequest,
  combatTuningStatusMatchesRequest,
  COMBAT_TUNING_CHARGE_ID,
  COMBAT_TUNING_COOLDOWN_ID,
  COMBAT_TUNING_ACTION_SPEED_ID,
  emptyCombatTuningStatus,
  normalizeCombatTuningStatus,
  parseCombatTuningMultiplier,
} from '../combatTuningUi.js'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'
import { language } from '../i18n.js'
import {
  buildRuntimePatchGroups,
  buildRuntimePatchStatusIndex,
  filterRuntimePatchGroups,
  findActiveRuntimePatchConflict,
  replaceRuntimePatchFeatureIDs,
  validateRuntimePatchStatusSet,
} from '../runtimePatchFeatureView.js'
import { createRuntimePatchOperationGate } from '../runtimePatchOperationGate.js'
import {
  translateRuntimePatchFeatureName,
  translateRuntimePatchFeatureSummary,
  translateRuntimePatchGroupName,
  translateRuntimePatchText,
} from '../runtimePatchTranslations.js'

const props = defineProps({
  mode: {
    type: String,
    required: true,
    validator: value => ['combat', 'characters', 'quest'].includes(value),
  },
})
const emit = defineEmits(['status', 'session-change'])

const RUNTIME_LEASE_SCOPE = 'runtime-patch-features'
const OFFLINE_CONFIRMATION_KEY = 'gbfr.runtimePatch.offline-only-confirmed'
const EMPTY_STATUS = Object.freeze({ enabled: false, available: false, rvas: [], currentBytes: [], error: '' })
const EMPTY_CONFLUX_STATUS = Object.freeze({ verified: false, available: false, enabled: false, owned: false, mode: 0, initialSeconds: 0, currentSeconds: 0, error: '' })
const CONFLUX_FEATURE = Object.freeze({ id: 'conflux-fast-wait', name: '极沌空域快速等待' })
const COOLDOWN_FEATURE = Object.freeze({ id: COMBAT_TUNING_COOLDOWN_ID, name: '能力冷却调整', kind: 'cooldown' })
const CHARGE_FEATURE = Object.freeze({ id: COMBAT_TUNING_CHARGE_ID, name: '三角色共享蓄力调整', kind: 'charge' })
const ACTION_SPEED_FEATURE = Object.freeze({ id: COMBAT_TUNING_ACTION_SPEED_ID, name: '人物动作速度', kind: 'actionSpeed' })
const TASK_SCORE_FEATURE = Object.freeze({ id: 'task-score-multiplier', name: '任务分数倍率' })
const TASK_SIDE_QUEST_FEATURE = Object.freeze({ id: 'task-side-quest-auto-complete', name: '自动补齐支线目标进度' })
const SUMMON_DURATION_FEATURE = Object.freeze({ id: 'summon-duration', name: '召唤持续时间' })
const CHARGE_CONFLICT_PATCH_ID = 'runtime-patch-017'
const EMPTY_TASK_RULE_FEATURE = Object.freeze({ available: false, enabled: false, multiplier: 0, rvas: [], currentBytes: [], evidenceNote: '', error: '' })
const EMPTY_TASK_RULES = Object.freeze({ scoreMultiplier: { ...EMPTY_TASK_RULE_FEATURE, multiplier: 2 }, sideQuestAutoComplete: { ...EMPTY_TASK_RULE_FEATURE } })
const EMPTY_SUMMON_DURATION = Object.freeze({ available: false, enabled: false, infinite: false, durationMultiplier: 2, rva: 0, currentBytes: '', evidenceNote: '', error: '' })

const catalog = ref([])
const statuses = ref([])
const confluxStatus = ref({ ...EMPTY_CONFLUX_STATUS })
const combatTuningStatus = ref(emptyCombatTuningStatus())
const taskRulesStatus = ref({ scoreMultiplier: { ...EMPTY_TASK_RULES.scoreMultiplier }, sideQuestAutoComplete: { ...EMPTY_TASK_RULES.sideQuestAutoComplete } })
const summonDurationStatus = ref({ ...EMPTY_SUMMON_DURATION })
const summonDurationMode = ref('multiplier')
const summonDurationMultiplier = ref('2')
const taskScoreMultiplier = ref('2')
const cooldownMode = ref('multiplier')
const cooldownMultiplier = ref('2')
const cooldownScope = ref('self')
const chargeMode = ref('multiplier')
const chargeMultiplier = ref('2')
const actionSpeedMultiplier = ref('1.5')
const actionSpeedScope = ref('self')
const searchQuery = ref('')
const browserScope = ref('all')
const catalogLoading = ref(true)
const activeOperation = ref(null)
const releasePending = ref(false)
const connected = ref(false)
const processInfo = ref({ pid: 0, moduleBase: 0 })
const liveMessage = ref(tr('正在读取实时功能目录…'))
const liveTone = ref('info')
const pendingConfirmationFeature = ref(null)
const confirmationCancelButton = ref(null)
const confirmationButton = ref(null)
const offlineConfirmationGranted = ref(false)
let disposed = false
let lifecycleEpoch = 0
let connectionOwnerToken = ''
let confirmationReturnTarget = null
const operationGate = createRuntimePatchOperationGate((operation) => {
  activeOperation.value = operation
})

function tr(value) {
  return translateRuntimePatchText(value, language.value)
}

const modeCopy = computed(() => {
  const copy = ({
    combat: {
      label: '战斗规则',
      summary: '闪避、格挡、Link、召唤限制与部位破坏等通用战斗规则。',
    },
    characters: {
      label: '角色机制',
      summary: '按真实角色分组的专属机制；搜索后只显示匹配角色与功能。',
    },
    quest: {
      label: '任务与便利',
      summary: '任务倒计时、宝箱、结算、支线奖励与养成便利。',
    },
  })[props.mode]
  return { label: tr(copy.label), summary: tr(copy.summary) }
})
const statusIndex = computed(() => buildRuntimePatchStatusIndex(statuses.value))
const groups = computed(() => buildRuntimePatchGroups(catalog.value, props.mode, searchQuery.value, {
  featureLabel: feature => translateRuntimePatchFeatureName(feature, language.value),
  groupLabel: group => translateRuntimePatchGroupName(group, language.value),
}))
const displayedGroups = computed(() => filterRuntimePatchGroups(
  groups.value,
  searchQuery.value ? 'all' : browserScope.value,
  statusIndex.value,
))
const modeExtraFeatureCount = computed(() => ({ combat: 2, characters: 2, quest: 3 })[props.mode] || 0)
const visibleFeatureCount = computed(() => displayedGroups.value.reduce((total, group) => total + group.features.length, 0) + modeExtraFeatureCount.value)
const activeFeatureCount = computed(() => statuses.value.filter(status => status.enabled).length
  + (confluxStatus.value.enabled && confluxStatus.value.owned ? 1 : 0)
  + Number(combatTuningStatus.value.cooldown.enabled)
  + Number(combatTuningStatus.value.charge.enabled)
  + Number(combatTuningStatus.value.actionSpeed.enabled)
  + Number(summonDurationStatus.value.enabled)
  + Number(taskRulesStatus.value.scoreMultiplier.enabled)
  + Number(taskRulesStatus.value.sideQuestAutoComplete.enabled))
const activeFeatureNames = computed(() => {
  const names = statuses.value
    .filter(status => status.enabled)
    .map(status => catalog.value.find(feature => feature.id === status.id))
    .filter(Boolean)
    .map(feature => translateRuntimePatchFeatureName(feature, language.value))
  if (confluxStatus.value.enabled && confluxStatus.value.owned) names.push(tr(CONFLUX_FEATURE.name))
  if (combatTuningStatus.value.cooldown.enabled) names.push(tr(COOLDOWN_FEATURE.name))
  if (combatTuningStatus.value.charge.enabled) names.push(tr(CHARGE_FEATURE.name))
  if (combatTuningStatus.value.actionSpeed.enabled) names.push(tr(ACTION_SPEED_FEATURE.name))
  if (summonDurationStatus.value.enabled) names.push(tr(SUMMON_DURATION_FEATURE.name))
  if (taskRulesStatus.value.scoreMultiplier.enabled) names.push(tr(TASK_SCORE_FEATURE.name))
  if (taskRulesStatus.value.sideQuestAutoComplete.enabled) names.push(tr(TASK_SIDE_QUEST_FEATURE.name))
  return names
})
const activeFeatureRoute = computed(() => {
  const activeFeature = statuses.value
    .filter(status => status.enabled)
    .map(status => catalog.value.find(feature => feature.id === status.id))
    .find(Boolean)
  const mode = activeFeature?.mode
    || (confluxStatus.value.enabled && confluxStatus.value.owned ? 'quest' : '')
    || (combatTuningStatus.value.cooldown.enabled ? 'combat' : '')
    || (summonDurationStatus.value.enabled ? 'combat' : '')
    || (combatTuningStatus.value.charge.enabled ? 'characters' : '')
    || (combatTuningStatus.value.actionSpeed.enabled ? 'characters' : '')
    || (taskRulesStatus.value.scoreMultiplier.enabled || taskRulesStatus.value.sideQuestAutoComplete.enabled ? 'quest' : '')
  return ({ combat: 'patchCombat', characters: 'patchCharacters', quest: 'patchQuest' })[mode] || 'patchCombat'
})
const recoveryFeatureCount = computed(() => statuses.value.filter(status => !status.enabled && status.rvas.length > 0).length
  + (confluxStatus.value.owned && !confluxStatus.value.enabled ? 1 : 0)
  + Number(!taskRulesStatus.value.scoreMultiplier.enabled && taskRulesStatus.value.scoreMultiplier.rvas.length > 0)
  + Number(!taskRulesStatus.value.sideQuestAutoComplete.enabled && taskRulesStatus.value.sideQuestAutoComplete.rvas.length > 0)
  + Number(!summonDurationStatus.value.enabled && Boolean(summonDurationStatus.value.currentBytes)))
const browserOwnedFeatureCount = computed(() => catalog.value
  .filter(feature => feature.mode === props.mode)
  .reduce((total, feature) => total + Number(ownsFeature(feature)), 0))
const operationBusy = computed(() => activeOperation.value !== null)
const interactionLocked = computed(() => operationBusy.value || releasePending.value)
const connectionLoading = computed(() => ['connect', 'disconnect'].includes(activeOperation.value?.kind))
const statusLoading = computed(() => activeOperation.value?.kind === 'refresh')
const busyFeatureID = computed(() => activeOperation.value?.kind === 'feature' ? activeOperation.value.featureID : '')
const cooldownMultiplierInvalid = computed(() => cooldownMode.value === 'multiplier' && !validMultiplier(cooldownMultiplier.value))
const chargeMultiplierInvalid = computed(() => chargeMode.value === 'multiplier' && !validMultiplier(chargeMultiplier.value))
const actionSpeedMultiplierInvalid = computed(() => {
  const numeric = Number(String(actionSpeedMultiplier.value).trim())
  return !Number.isFinite(numeric) || numeric < 0.1 || numeric > 5
})
const taskScoreMultiplierInvalid = computed(() => {
  const numeric = Number(String(taskScoreMultiplier.value).trim())
  return !Number.isFinite(numeric) || numeric < 0.1 || numeric > 16
})
const summonDurationMultiplierInvalid = computed(() => {
  const numeric = Number(String(summonDurationMultiplier.value).trim())
  return summonDurationMode.value === 'multiplier' && (!Number.isFinite(numeric) || numeric < 0.1 || numeric > 16)
})
const chargeCatalogConflict = computed(() => {
  const status = statusIndex.value.get(CHARGE_CONFLICT_PATCH_ID)
  return !!status && (status.enabled || status.rvas.length > 0)
})

function validMultiplier(value) {
  try {
    parseCombatTuningMultiplier(value, '倍率')
    return true
  } catch {
    return false
  }
}

function publishSession() {
  emit('session-change', Object.freeze({
    connected: connected.value,
    releasePending: releasePending.value,
    activeCount: activeFeatureCount.value,
    activeFeatures: [...activeFeatureNames.value],
    route: activeFeatureRoute.value,
    recoveryCount: recoveryFeatureCount.value,
    pid: connected.value ? processInfo.value.pid : 0,
  }))
}

watch(() => props.mode, () => {
  browserScope.value = 'all'
  searchQuery.value = ''
})
watch([connected, releasePending, activeFeatureCount, activeFeatureNames, activeFeatureRoute, recoveryFeatureCount, () => processInfo.value.pid], publishSession, { immediate: true })
watch(pendingConfirmationFeature, async (feature) => {
  if (!feature) return
  await nextTick()
  confirmationButton.value?.focus()
})

function normalizeCatalog(value) {
  if (!Array.isArray(value)) throw new Error('实时功能目录格式无效')
  return value.map((feature) => ({
    ...feature,
    groupPath: Array.isArray(feature?.groupPath) ? feature.groupPath : [],
    conflicts: Array.isArray(feature?.conflicts) ? feature.conflicts : [],
    sites: Array.isArray(feature?.sites) ? feature.sites : [],
  }))
}

function errorMessage(error) {
  const message = error instanceof Error ? error.message : String(error || '未知错误')
  return tr(replaceRuntimePatchFeatureIDs(message.replace(/^Error:\s*/i, ''), catalog.value))
}

function announce(message, tone = 'info') {
  const translatedMessage = tr(message)
  liveMessage.value = translatedMessage
  liveTone.value = tone
  if (translatedMessage) emit('status', translatedMessage, tone === 'danger' ? 'error' : tone === 'ok' ? 'success' : tone)
}

function applyStatuses(nextStatuses) {
  statuses.value = nextStatuses
}

function normalizeConfluxStatus(value) {
  return {
    verified: value?.verified === true,
    available: value?.available === true,
    enabled: value?.enabled === true,
    owned: value?.owned === true,
    mode: Number.isSafeInteger(Number(value?.mode)) ? Number(value.mode) : 0,
    initialSeconds: Number.isFinite(Number(value?.initialSeconds)) ? Number(value.initialSeconds) : 0,
    currentSeconds: Number.isFinite(Number(value?.currentSeconds)) ? Number(value.currentSeconds) : 0,
    error: String(value?.error || ''),
  }
}

function normalizeTaskRuleFeature(value, fallbackMultiplier = 0) {
  return {
    available: value?.available === true,
    enabled: value?.enabled === true,
    multiplier: Number.isFinite(Number(value?.multiplier)) ? Number(value.multiplier) : fallbackMultiplier,
    rvas: Array.isArray(value?.rvas) ? value.rvas.map(Number).filter(Number.isFinite) : [],
    currentBytes: Array.isArray(value?.currentBytes) ? value.currentBytes.map(String) : [],
    evidenceNote: String(value?.evidenceNote || ''),
    error: String(value?.error || ''),
  }
}

function normalizeTaskRulesStatus(value) {
  return {
    scoreMultiplier: normalizeTaskRuleFeature(value?.scoreMultiplier, 2),
    sideQuestAutoComplete: normalizeTaskRuleFeature(value?.sideQuestAutoComplete),
  }
}

function normalizeSummonDurationStatus(value) {
  const multiplier = Number(value?.durationMultiplier ?? 2)
  const rva = Number(value?.rva ?? 0)
  if (!Number.isFinite(multiplier) || multiplier < 0.1 || multiplier > 16) throw new Error(tr('召唤持续时间倍率回读超出 0.1 到 16.0'))
  if (!Number.isSafeInteger(rva) || rva < 0) throw new Error(tr('召唤持续时间入口回读无效'))
  return Object.freeze({
    available: value?.available === true,
    enabled: value?.enabled === true,
    infinite: value?.infinite === true,
    durationMultiplier: multiplier,
    rva,
    currentBytes: String(value?.currentBytes || ''),
    evidenceNote: String(value?.evidenceNote || ''),
    error: String(value?.error || ''),
  })
}

function syncCombatTuningDrafts(status) {
  cooldownMode.value = status.cooldown.noCooldown ? 'instant' : 'multiplier'
  cooldownMultiplier.value = String(status.cooldown.speedMultiplier)
  cooldownScope.value = status.cooldown.applyWholeParty ? 'party' : 'self'
  chargeMode.value = status.charge.instant ? 'instant' : 'multiplier'
  chargeMultiplier.value = String(status.charge.speedMultiplier)
  actionSpeedMultiplier.value = String(status.actionSpeed.speedMultiplier)
  actionSpeedScope.value = status.actionSpeed.applyWholeParty ? 'party' : 'self'
}

function applyVerifiedSession(session, syncTuningDraft = false) {
  applyStatuses(session.statuses)
  confluxStatus.value = session.conflux
  combatTuningStatus.value = session.combatTuning
  if (session.taskRules) taskRulesStatus.value = session.taskRules
  if (session.summonDuration) summonDurationStatus.value = session.summonDuration
  if (syncTuningDraft) syncCombatTuningDrafts(session.combatTuning)
  if (syncTuningDraft && session.taskRules?.scoreMultiplier.enabled) taskScoreMultiplier.value = String(session.taskRules.scoreMultiplier.multiplier)
  if (syncTuningDraft && session.summonDuration?.enabled) {
    summonDurationMode.value = session.summonDuration.infinite ? 'infinite' : 'multiplier'
    summonDurationMultiplier.value = String(session.summonDuration.durationMultiplier)
  }
}

function beginOperation(kind, featureID = '') {
  if (disposed) return null
  return operationGate.begin(kind, featureID)
}

function operationIsCurrent(token, epoch) {
  return !disposed && lifecycleEpoch === epoch && operationGate.isCurrent(token)
}

function finishOperation(token) {
  operationGate.finish(token)
}

async function loadCatalog(notify = false) {
  const epoch = lifecycleEpoch
  catalogLoading.value = true
  try {
    const nextCatalog = normalizeCatalog(await RuntimePatchGetCatalog())
    if (disposed || epoch !== lifecycleEpoch) return
    catalog.value = nextCatalog
    if (notify) announce(`已读取 ${catalog.value.length} 项已验证补丁`, 'ok')
    else liveMessage.value = tr('功能目录已就绪；连接游戏后可读取实时状态。')
  } catch (error) {
    if (!disposed && epoch === lifecycleEpoch) announce(`读取实时功能目录失败：${errorMessage(error)}`, 'danger')
  } finally {
    if (!disposed) catalogLoading.value = false
  }
}

async function releaseRuntimePatchPageOwner(ownerToken) {
  await RuntimePatchReleaseOwned(ownerToken)
  await CharaRelease(ownerToken)
}

function clearConnectionState() {
  operationGate.reset()
  releasePending.value = false
  connected.value = false
  connectionOwnerToken = ''
  processInfo.value = { pid: 0, moduleBase: 0 }
  statuses.value = []
  confluxStatus.value = { ...EMPTY_CONFLUX_STATUS }
  combatTuningStatus.value = emptyCombatTuningStatus()
  taskRulesStatus.value = normalizeTaskRulesStatus(EMPTY_TASK_RULES)
  summonDurationStatus.value = normalizeSummonDurationStatus(EMPTY_SUMMON_DURATION)
}

function completeRuntimeRelease(expectedOwnerToken, expectedEpoch, notification) {
  if (
    disposed
    || lifecycleEpoch !== expectedEpoch
    || connectionOwnerToken !== expectedOwnerToken
    || notification?.ownerToken !== expectedOwnerToken
  ) return
  clearConnectionState()
  announce('全部实时补丁已恢复，并已断开游戏进程', 'ok')
}

async function connect() {
  if (connected.value || releasePending.value) return
  const operationToken = beginOperation('connect')
  if (!operationToken) return
  const epoch = ++lifecycleEpoch
  let acquiredOwnerToken = ''
  try {
    if (!catalog.value.length) catalog.value = normalizeCatalog(await RuntimePatchGetCatalog())
    if (!operationIsCurrent(operationToken, epoch)) return
    const info = await CharaAcquire(nextRuntimeAcquireRequestID())
    acquiredOwnerToken = String(info?.ownerToken || '')
    if (!acquiredOwnerToken) throw new Error('后端未返回连接所有权令牌')
    if (!operationIsCurrent(operationToken, epoch)) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, releaseRuntimePatchPageOwner)
      return
    }
    const verifiedSession = await fetchVerifiedSession(acquiredOwnerToken)
    if (!operationIsCurrent(operationToken, epoch)) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, releaseRuntimePatchPageOwner)
      return
    }
    connectionOwnerToken = acquiredOwnerToken
    connected.value = true
    processInfo.value = { pid: Number(info?.pid || 0), moduleBase: Number(info?.moduleBase || 0) }
    applyVerifiedSession(verifiedSession, true)
    announce(`已连接游戏进程 PID ${processInfo.value.pid}`, 'ok')
  } catch (error) {
    let cleanupError = null
    if (acquiredOwnerToken) {
      try {
        await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, releaseRuntimePatchPageOwner)
      } catch (nextError) {
        cleanupError = nextError
      }
    }
    if (!cleanupError) clearConnectionState()
    if (!disposed && epoch === lifecycleEpoch) {
      const suffix = cleanupError ? `；释放连接也失败：${errorMessage(cleanupError)}` : ''
      announce(`${errorMessage(error)}${suffix}`, 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

async function disconnect() {
  const ownerToken = connectionOwnerToken
  if (!ownerToken) return
  const operationToken = beginOperation('disconnect')
  if (!operationToken) return
  const epoch = ++lifecycleEpoch
  releasePending.value = true
  try {
    await releaseRuntimeLease(
      RUNTIME_LEASE_SCOPE,
      ownerToken,
      releaseRuntimePatchPageOwner,
      notification => completeRuntimeRelease(ownerToken, epoch, notification),
    )
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch)) {
      releasePending.value = true
      announce(`安全断开暂未完成，正在后台重试恢复：${errorMessage(error)}`, 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

async function fetchVerifiedStatuses(ownerToken) {
  return validateRuntimePatchStatusSet(catalog.value, await RuntimePatchGetStatusesOwned(ownerToken))
}

async function fetchVerifiedSession(ownerToken) {
  const [nextStatuses, nextConfluxStatus, nextCombatTuningStatus, nextTaskRulesStatus, nextSummonDurationStatus] = await Promise.all([
    fetchVerifiedStatuses(ownerToken),
    ConfluxTimerGetStatusOwned(ownerToken),
    CombatTuningGetStatusOwned(ownerToken),
    TaskRulesGetStatusOwned(ownerToken),
    SummonDurationGetStatusOwned(ownerToken),
  ])
  return {
    statuses: nextStatuses,
    conflux: normalizeConfluxStatus(nextConfluxStatus),
    combatTuning: normalizeCombatTuningStatus(nextCombatTuningStatus),
    taskRules: normalizeTaskRulesStatus(nextTaskRulesStatus),
    summonDuration: normalizeSummonDurationStatus(nextSummonDurationStatus),
  }
}

async function refreshStatuses() {
  const ownerToken = connectionOwnerToken
  if (!ownerToken || releasePending.value) return
  const operationToken = beginOperation('refresh')
  if (!operationToken) return
  const epoch = lifecycleEpoch
  try {
    const verifiedSession = await fetchVerifiedSession(ownerToken)
    if (!operationIsCurrent(operationToken, epoch) || ownerToken !== connectionOwnerToken) return
    applyVerifiedSession(verifiedSession, true)
    announce('实时补丁状态已回读', 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch)) announce(`刷新状态失败：${errorMessage(error)}`, 'danger')
  } finally {
    finishOperation(operationToken)
  }
}

async function setFeatureEnabled(feature, enabled) {
  if (!feature || releasePending.value) return
  const operationToken = beginOperation('feature', feature.id)
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken) throw new Error('当前页面不再持有连接所有权')
    await RuntimePatchSetEnabledOwned(ownerToken, feature.id, enabled)
    const verifiedSession = await fetchVerifiedSession(ownerToken)
    if (!operationIsCurrent(operationToken, epoch) || ownerToken !== connectionOwnerToken) return
    const verifiedStatus = verifiedSession.statuses.find(status => status.id === feature.id)
    if (!verifiedStatus || verifiedStatus.enabled !== enabled) throw new Error(`${feature.name} 写后回读状态不一致`)
    applyVerifiedSession(verifiedSession)
    announce(`${displayFeatureName(feature)}已${enabled ? '开启' : '恢复默认'}`, 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) {
      try {
        const recoveredSession = await fetchVerifiedSession(ownerToken)
        if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) applyVerifiedSession(recoveredSession)
      } catch {
        // Keep the last verified UI state. Disconnect remains available so the
        // backend can retry restoration using its retained recovery lease.
      }
      announce(`${displayFeatureName(feature)}操作失败：${errorMessage(error)}`, 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

async function setConfluxEnabled(enabled) {
  if (releasePending.value) return
  const operationToken = beginOperation('feature', CONFLUX_FEATURE.id)
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken) throw new Error('当前页面不再持有连接所有权')
    await ConfluxTimerSetEnabledOwned(ownerToken, enabled)
    const verifiedSession = await fetchVerifiedSession(ownerToken)
    if (!operationIsCurrent(operationToken, epoch) || ownerToken !== connectionOwnerToken) return
    if (verifiedSession.conflux.enabled !== enabled || (enabled && !verifiedSession.conflux.owned)) {
      throw new Error('极沌空域快速等待写后回读状态不一致')
    }
    applyVerifiedSession(verifiedSession)
    announce(`极沌空域快速等待已${enabled ? '开启' : '恢复默认'}`, 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) {
      try {
        const recoveredSession = await fetchVerifiedSession(ownerToken)
        if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) applyVerifiedSession(recoveredSession)
      } catch {
        // Retain the last verified state so the global safe-disconnect path can retry.
      }
      announce(`极沌空域快速等待操作失败：${errorMessage(error)}`, 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

function tuningFeature(kind) {
  if (kind === 'cooldown') return COOLDOWN_FEATURE
  if (kind === 'actionSpeed') return ACTION_SPEED_FEATURE
  return CHARGE_FEATURE
}

function tuningRequest(kind, enabled) {
  if (kind === 'cooldown') {
    return buildCooldownRequest({
      enabled,
      mode: cooldownMode.value,
      multiplier: cooldownMultiplier.value,
      scope: cooldownScope.value,
    })
  }
  if (kind === 'actionSpeed') {
    return buildActionSpeedRequest({
      enabled,
      multiplier: actionSpeedMultiplier.value,
      scope: actionSpeedScope.value,
    })
  }
  return buildChargeRequest({
    enabled,
    mode: chargeMode.value,
    multiplier: chargeMultiplier.value,
  })
}

async function setCombatTuningEnabled(kind, enabled) {
  if (releasePending.value) return
  const feature = tuningFeature(kind)
  let request
  try {
    if (kind === 'charge' && enabled && chargeCatalogConflict.value) {
      throw new Error('先恢复冈达葛萨「瞬间直冲拳」，再应用共享蓄力调整')
    }
    request = tuningRequest(kind, enabled)
  } catch (error) {
    announce(`${feature.name}操作失败：${errorMessage(error)}`, 'danger')
    return
  }
  const operationToken = beginOperation('feature', feature.id)
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken) throw new Error('当前页面不再持有连接所有权')
    if (kind === 'cooldown') await CombatTuningSetCooldownOwned(ownerToken, request)
    else if (kind === 'actionSpeed') await CombatTuningSetActionSpeedOwned(ownerToken, request)
    else await CombatTuningSetChargeOwned(ownerToken, request)
    const verifiedSession = await fetchVerifiedSession(ownerToken)
    if (!operationIsCurrent(operationToken, epoch) || ownerToken !== connectionOwnerToken) return
    const verifiedStatus = verifiedSession.combatTuning[kind]
    if (!combatTuningStatusMatchesRequest(verifiedStatus, request, kind)) {
      throw new Error(`${feature.name}写后回读状态不一致`)
    }
    applyVerifiedSession(verifiedSession, true)
    announce(`${feature.name}已${enabled ? '应用并回读' : '恢复默认'}`, 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) {
      try {
        const recoveredSession = await fetchVerifiedSession(ownerToken)
        if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) {
          applyVerifiedSession(recoveredSession, true)
        }
      } catch {
        // Retain the last verified state. The shared disconnect path still
        // owns restoration for both parameterized hooks.
      }
      announce(`${feature.name}操作失败：${errorMessage(error)}`, 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

function taskRuleFeature(kind) {
  return kind === 'score' ? TASK_SCORE_FEATURE : TASK_SIDE_QUEST_FEATURE
}

function taskRuleStatus(kind) {
  return kind === 'score' ? taskRulesStatus.value.scoreMultiplier : taskRulesStatus.value.sideQuestAutoComplete
}

async function setTaskRuleEnabled(kind, enabled) {
  if (releasePending.value) return
  const feature = taskRuleFeature(kind)
  const operationToken = beginOperation('feature', feature.id)
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken) throw new Error('当前页面不再持有连接所有权')
    if (kind === 'score') {
      const multiplier = Number(String(taskScoreMultiplier.value).trim())
      if (enabled && taskScoreMultiplierInvalid.value) throw new Error('任务分数倍率请输入 0.1 到 16.0')
      await TaskRulesSetScoreMultiplierOwned(ownerToken, { enabled, multiplier: enabled ? multiplier : 2 })
    } else {
      await TaskRulesSetSideQuestAutoCompleteOwned(ownerToken, enabled)
    }
    const verifiedSession = await fetchVerifiedSession(ownerToken)
    if (!operationIsCurrent(operationToken, epoch) || ownerToken !== connectionOwnerToken) return
    const verified = kind === 'score' ? verifiedSession.taskRules.scoreMultiplier : verifiedSession.taskRules.sideQuestAutoComplete
    if (verified.enabled !== enabled || (enabled && kind === 'score' && verified.multiplier !== Number(taskScoreMultiplier.value))) {
      throw new Error(`${feature.name}写后回读状态不一致`)
    }
    applyVerifiedSession(verifiedSession, true)
    announce(`${feature.name}已${enabled ? '应用并回读' : '恢复默认'}`, 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) {
      try {
        const recoveredSession = await fetchVerifiedSession(ownerToken)
        if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) applyVerifiedSession(recoveredSession, true)
      } catch {
        // Retain the last verified state so disconnect can retry restoration.
      }
      announce(`${feature.name}操作失败：${errorMessage(error)}`, 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

function requestTaskRuleApply(kind) {
  if (interactionLocked.value) return
  const feature = taskRuleFeature(kind)
  if (!offlineUseConfirmed()) {
    confirmationReturnTarget = document.activeElement
    pendingConfirmationFeature.value = feature
    return
  }
  void setTaskRuleEnabled(kind, true)
}

function requestTaskRuleRestore(kind) {
  if (!interactionLocked.value) void setTaskRuleEnabled(kind, false)
}

async function setSummonDurationEnabled(enabled) {
  if (releasePending.value) return
  const operationToken = beginOperation('feature', SUMMON_DURATION_FEATURE.id)
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken) throw new Error('当前页面不再持有连接所有权')
    if (enabled && summonDurationMultiplierInvalid.value) throw new Error(tr('召唤持续时间倍率请输入 0.1 到 16.0'))
    const request = {
      enabled,
      infinite: enabled && summonDurationMode.value === 'infinite',
      durationMultiplier: enabled ? Number(summonDurationMultiplier.value) : 2,
    }
    await SummonDurationSetOwned(ownerToken, request)
    const verifiedSession = await fetchVerifiedSession(ownerToken)
    if (!operationIsCurrent(operationToken, epoch) || ownerToken !== connectionOwnerToken) return
    const verified = verifiedSession.summonDuration
    if (verified.enabled !== enabled || (enabled && (verified.infinite !== request.infinite || verified.durationMultiplier !== request.durationMultiplier))) {
      throw new Error(tr('召唤持续时间写后回读状态不一致'))
    }
    applyVerifiedSession(verifiedSession, true)
    announce(tr(enabled ? '召唤持续时间已应用并回读' : '召唤持续时间已恢复默认'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) {
      try {
        const recoveredSession = await fetchVerifiedSession(ownerToken)
        if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) applyVerifiedSession(recoveredSession, true)
      } catch {
        // Keep the last verified state so safe disconnect can retry restoration.
      }
      announce(tr(`召唤持续时间操作失败：${errorMessage(error)}`), 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

function requestSummonDurationApply() {
  if (interactionLocked.value) return
  if (!offlineUseConfirmed()) {
    confirmationReturnTarget = document.activeElement
    pendingConfirmationFeature.value = SUMMON_DURATION_FEATURE
    return
  }
  void setSummonDurationEnabled(true)
}

function requestSummonDurationRestore() {
  if (!interactionLocked.value) void setSummonDurationEnabled(false)
}

async function verifyConfluxStatus() {
  if (releasePending.value) return
  const operationToken = beginOperation('feature', CONFLUX_FEATURE.id)
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken) throw new Error('当前页面不再持有连接所有权')
    const [nextStatuses, nextConfluxStatus] = await Promise.all([
      fetchVerifiedStatuses(ownerToken),
      ConfluxTimerVerifyStatusOwned(ownerToken),
    ])
    if (!operationIsCurrent(operationToken, epoch) || ownerToken !== connectionOwnerToken) return
    const normalized = normalizeConfluxStatus(nextConfluxStatus)
    applyVerifiedSession({ statuses: nextStatuses, conflux: normalized, combatTuning: combatTuningStatus.value })
    if (!normalized.verified) throw new Error(normalized.error || '游戏版本校验失败')
    announce(normalized.error ? `游戏版本已校验；${normalized.error}` : '游戏版本已校验，极沌空域计时器状态已回读', normalized.error ? 'info' : 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) {
      announce(`极沌空域版本校验失败：${errorMessage(error)}`, 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

function offlineUseConfirmed() {
  try {
    return offlineConfirmationGranted.value || window.sessionStorage.getItem(OFFLINE_CONFIRMATION_KEY) === '1'
  } catch {
    return offlineConfirmationGranted.value
  }
}

function requestFeatureChange(feature) {
  if (interactionLocked.value) return
  const enable = !ownsFeature(feature)
  if (enable && !offlineUseConfirmed()) {
    confirmationReturnTarget = document.activeElement
    pendingConfirmationFeature.value = feature
    return
  }
  void setFeatureEnabled(feature, enable)
}

function requestConfluxChange() {
  if (interactionLocked.value) return
  if (!confluxStatus.value.verified) {
    void verifyConfluxStatus()
    return
  }
  const enable = !confluxStatus.value.owned
  if (enable && !offlineUseConfirmed()) {
    confirmationReturnTarget = document.activeElement
    pendingConfirmationFeature.value = CONFLUX_FEATURE
    return
  }
  void setConfluxEnabled(enable)
}

function requestCombatTuningApply(kind) {
  if (interactionLocked.value) return
  const feature = tuningFeature(kind)
  if (!offlineUseConfirmed()) {
    confirmationReturnTarget = document.activeElement
    pendingConfirmationFeature.value = feature
    return
  }
  void setCombatTuningEnabled(kind, true)
}

function requestCombatTuningRestore(kind) {
  if (interactionLocked.value) return
  void setCombatTuningEnabled(kind, false)
}

function cancelOfflineConfirmation() {
  pendingConfirmationFeature.value = null
  const returnTarget = confirmationReturnTarget
  confirmationReturnTarget = null
  void nextTick(() => returnTarget?.focus?.())
}

async function confirmOfflineUse() {
  const feature = pendingConfirmationFeature.value
  if (!feature) return
  const returnTarget = confirmationReturnTarget
  confirmationReturnTarget = null
  offlineConfirmationGranted.value = true
  try {
    window.sessionStorage.setItem(OFFLINE_CONFIRMATION_KEY, '1')
  } catch {
    // The confirmation remains valid for this mounted page if storage is
    // unavailable; no patch state is changed before this explicit action.
  }
  pendingConfirmationFeature.value = null
  if (feature.id === CONFLUX_FEATURE.id) await setConfluxEnabled(true)
  else if (feature.id === COOLDOWN_FEATURE.id) await setCombatTuningEnabled('cooldown', true)
  else if (feature.id === CHARGE_FEATURE.id) await setCombatTuningEnabled('charge', true)
  else if (feature.id === ACTION_SPEED_FEATURE.id) await setCombatTuningEnabled('actionSpeed', true)
  else if (feature.id === TASK_SCORE_FEATURE.id) await setTaskRuleEnabled('score', true)
  else if (feature.id === TASK_SIDE_QUEST_FEATURE.id) await setTaskRuleEnabled('sideQuest', true)
  else if (feature.id === SUMMON_DURATION_FEATURE.id) await setSummonDurationEnabled(true)
  else await setFeatureEnabled(feature, true)
  await nextTick()
  returnTarget?.focus?.()
}

function trapConfirmationFocus(event) {
  const first = confirmationCancelButton.value
  const last = confirmationButton.value
  if (!first || !last) return
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

function selectGroup(key) {
  browserScope.value = key
}

function selectBrowserScope(scope) {
  browserScope.value = scope
}

function statusFor(feature) {
  return statusIndex.value.get(feature?.id) || EMPTY_STATUS
}

function ownsFeature(feature) {
  const status = statusFor(feature)
  return status.enabled || status.rvas.length > 0
}

function activeConflictFor(feature) {
  if (feature?.id === CHARGE_CONFLICT_PATCH_ID && combatTuningStatus.value.charge.enabled) return CHARGE_FEATURE
  return findActiveRuntimePatchConflict(feature, statusIndex.value, catalog.value)
}

function displayFeatureName(feature) {
  if ([COOLDOWN_FEATURE.id, CHARGE_FEATURE.id, ACTION_SPEED_FEATURE.id, TASK_SCORE_FEATURE.id, TASK_SIDE_QUEST_FEATURE.id, SUMMON_DURATION_FEATURE.id].includes(feature?.id)) return tr(feature.name)
  return translateRuntimePatchFeatureName(feature, language.value)
}

function displayFeatureSummary(feature) {
  return translateRuntimePatchFeatureSummary(feature, language.value)
}

function displayGroupName(group) {
  return translateRuntimePatchGroupName(group, language.value)
}

function displayFeatureError(feature) {
  if (activeConflictFor(feature)) return ''
  return tr(replaceRuntimePatchFeatureIDs(statusFor(feature).error, catalog.value))
}

function featureDisabled(feature) {
  const status = statusFor(feature)
  if (!connected.value || interactionLocked.value) return true
  if (ownsFeature(feature)) return false
  return !status.available || !!activeConflictFor(feature)
}

function tuningStatusFor(kind) {
  return combatTuningStatus.value[kind]
}

function tuningStateLabel(kind) {
  const feature = tuningFeature(kind)
  if (busyFeatureID.value === feature.id) return tr('回读中')
  const status = tuningStatusFor(kind)
  if (status.enabled) return tr('已开启')
  if (!connected.value) return tr('未连接')
  if (!status.available) return tr('不可用')
  return tr('默认')
}

function tuningDisabled(kind, action = 'apply') {
  if (!connected.value || interactionLocked.value) return true
  const status = tuningStatusFor(kind)
  if (action === 'restore') return !status.enabled
  if (!status.available) return true
  if (kind === 'cooldown') return cooldownMultiplierInvalid.value
  if (kind === 'actionSpeed') return actionSpeedMultiplierInvalid.value
  return chargeMultiplierInvalid.value || chargeCatalogConflict.value
}

function tuningProof(kind) {
  const status = tuningStatusFor(kind)
  if (!connected.value) return tr('连接后定位候选入口并读取原字节')
  if (status.rvas.length) return tr(`已回读 ${status.rvas.length} 个候选入口`)
  return tr('尚未定位到候选入口')
}

function tuningCurrentSetting(kind) {
  const status = tuningStatusFor(kind)
  if (!status.enabled) return tr('当前保持游戏默认')
  if (kind === 'cooldown') {
    const mode = status.noCooldown ? tr('无冷却') : `${status.speedMultiplier}×`
    const scope = status.applyWholeParty ? tr('全队实验范围') : tr('仅自己')
    return `${mode} · ${scope}`
  }
  if (kind === 'actionSpeed') {
    const scope = status.applyWholeParty ? tr('全队实验范围') : tr('仅自己')
    return `${status.speedMultiplier}× · ${scope}`
  }
  return status.instant ? tr('瞬间蓄力') : `${status.speedMultiplier}×`
}

function taskRuleStateLabel(kind) {
  const feature = taskRuleFeature(kind)
  if (busyFeatureID.value === feature.id) return tr('回读中')
  const status = taskRuleStatus(kind)
  if (status.enabled) return tr('已开启')
  if (status.rvas.length) return tr('需要恢复')
  if (!connected.value) return tr('未连接')
  if (!status.available) return tr('不可用')
  return tr('默认')
}

function taskRuleDisabled(kind, action = 'apply') {
  if (!connected.value || interactionLocked.value) return true
  const status = taskRuleStatus(kind)
  if (action === 'restore') return !status.enabled && !status.rvas.length
  if (!status.available) return true
  return kind === 'score' && taskScoreMultiplierInvalid.value
}

function taskRuleProof(kind) {
  const status = taskRuleStatus(kind)
  if (!connected.value) return tr('连接后校验 2.0.3 / 2.0.4 / 2.0.5 入口与原字节')
  if (status.rvas.length) return tr(`已回读 ${status.rvas.length} 个任务入口`)
  return tr('尚未写入，游戏保持默认')
}

function summonDurationStateLabel() {
  if (busyFeatureID.value === SUMMON_DURATION_FEATURE.id) return tr('回读中')
  if (summonDurationStatus.value.enabled) return tr('已开启')
  if (summonDurationStatus.value.currentBytes) return tr('需要恢复')
  if (!connected.value) return tr('未连接')
  if (!summonDurationStatus.value.available) return tr('不可用')
  return tr('默认')
}

function summonDurationDisabled(action = 'apply') {
  if (!connected.value || interactionLocked.value) return true
  if (action === 'restore') return !summonDurationStatus.value.enabled && !summonDurationStatus.value.currentBytes
  return !summonDurationStatus.value.available || summonDurationMultiplierInvalid.value || summonDurationStatus.value.enabled
}

function featureStateLabel(feature) {
  if (busyFeatureID.value === feature.id) return tr('回读中')
  const status = statusFor(feature)
  if (status.enabled) return tr('已开启')
  if (ownsFeature(feature)) return tr('需要恢复')
  if (!connected.value) return tr('未连接')
  if (activeConflictFor(feature)) return tr('互斥占用')
  if (!status.available) return tr('不可用')
  return tr('默认')
}

function formatRVA(value) {
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric >= 0 ? `0x${numeric.toString(16).toUpperCase()}` : '—'
}

function confluxStateLabel() {
  if (busyFeatureID.value === CONFLUX_FEATURE.id) return tr('回读中')
  if (confluxStatus.value.enabled && confluxStatus.value.owned) return tr('已开启')
  if (confluxStatus.value.owned) return tr('需要恢复')
  if (!connected.value) return tr('未连接')
  if (!confluxStatus.value.verified) return tr('待验证')
  if (!confluxStatus.value.available) return tr('不可用')
  return tr('默认')
}

function confluxDisabled() {
  if (!connected.value || interactionLocked.value) return true
  if (!confluxStatus.value.verified) return false
  return !confluxStatus.value.owned && !confluxStatus.value.available
}

function formatConfluxSeconds(value) {
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric >= 0 ? `${numeric.toFixed(numeric < 10 ? 1 : 0)} s` : '—'
}

onMounted(() => {
  void loadCatalog()
})

onBeforeUnmount(() => {
  disposed = true
  lifecycleEpoch += 1
  operationGate.reset()
  pendingConfirmationFeature.value = null
  confirmationReturnTarget = null
  const ownerToken = connectionOwnerToken
  connectionOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, releaseRuntimePatchPageOwner)
})
</script>

<template>
  <section class="runtime-patch-page ui-page is-wide ui-page-stack" :data-mode="mode">
    <section class="patch-connection ui-card ui-panel is-compact">
      <div class="patch-connection-main">
        <span class="connection-emblem" aria-hidden="true"><i></i></span>
        <div class="connection-copy">
          <strong>{{ tr(releasePending ? '正在安全恢复并断开' : connected ? '游戏进程已连接' : '连接游戏后读取实时状态') }}</strong>
          <span v-if="connected">PID {{ processInfo.pid }} · {{ tr('已开启') }} {{ activeFeatureCount }} {{ tr('项') }}</span>
          <span v-else>{{ modeCopy.summary }}</span>
          <span>{{ tr('三个补丁页共用此连接；切换页面时保持常驻。') }}</span>
        </div>
        <span class="ui-tag" :class="releasePending ? 'is-warn' : connected ? 'is-ok' : 'is-info'">{{ tr(releasePending ? '等待恢复' : connected ? '已验证连接' : '未连接') }}</span>
      </div>
      <div class="patch-connection-actions ui-actions">
        <button v-if="connected" type="button" class="ui-btn is-ghost is-sm" :disabled="interactionLocked" @click="refreshStatuses">
          {{ tr(statusLoading ? '回读中…' : '刷新状态') }}
        </button>
        <button type="button" class="ui-btn is-sm" :class="connected ? 'is-danger' : 'is-primary'" :disabled="operationBusy" @click="connected ? disconnect() : connect()">
          {{ tr(connectionLoading ? '处理中…' : releasePending ? '重试安全恢复' : connected ? '恢复全部并断开' : '连接游戏进程') }}
        </button>
      </div>
    </section>

    <div class="patch-live-region ui-notice" :class="`is-${liveTone}`" aria-live="polite" aria-atomic="true">
      <span class="live-mark" aria-hidden="true"></span>
      <span>{{ liveMessage }}</span>
    </div>

    <section
      v-if="mode === 'combat'"
      class="tuning-priority ui-card ui-panel"
      :class="{ 'is-on': combatTuningStatus.cooldown.enabled }"
      :aria-label="tr('能力冷却调整')"
    >
      <header class="tuning-priority-head">
        <div>
          <p>{{ tr('靠前常用功能') }}</p>
          <h2>{{ tr('能力冷却调整') }}</h2>
          <span>{{ tr('先选调整方式和作用范围，再应用；不会因为切换补丁页而停止。') }}</span>
        </div>
        <div class="tuning-tags">
          <span class="ui-tag is-warn">{{ tr('实验候选') }}</span>
          <span class="ui-tag" :class="combatTuningStatus.cooldown.enabled ? 'is-ok' : combatTuningStatus.cooldown.available ? 'is-info' : 'is-danger'">
            {{ tuningStateLabel('cooldown') }}
          </span>
        </div>
      </header>

      <form class="tuning-form" @submit.prevent="requestCombatTuningApply('cooldown')">
        <div class="tuning-control-grid">
          <fieldset class="tuning-control">
            <legend>{{ tr('调整方式') }}</legend>
            <div class="tuning-segment ui-seg" role="group" :aria-label="tr('冷却调整方式')">
              <button
                type="button"
                class="ui-seg-btn"
                :class="{ 'is-on': cooldownMode === 'multiplier' }"
                :aria-pressed="cooldownMode === 'multiplier'"
                :disabled="interactionLocked"
                @click="cooldownMode = 'multiplier'"
              >{{ tr('速度倍率') }}</button>
              <button
                type="button"
                class="ui-seg-btn"
                :class="{ 'is-on': cooldownMode === 'instant' }"
                :aria-pressed="cooldownMode === 'instant'"
                :disabled="interactionLocked"
                @click="cooldownMode = 'instant'"
              >{{ tr('无冷却') }}</button>
            </div>
          </fieldset>

          <label v-if="cooldownMode === 'multiplier'" class="tuning-control ui-field">
            <span class="ui-field-label">{{ tr('冷却速度倍率') }}</span>
            <span class="tuning-number">
              <input
                v-model.trim="cooldownMultiplier"
                class="ui-input"
                type="number"
                min="0.1"
                max="100"
                step="0.1"
                inputmode="decimal"
                :aria-invalid="cooldownMultiplierInvalid"
                :disabled="interactionLocked"
              >
              <b aria-hidden="true">×</b>
            </span>
            <small :class="{ 'is-error': cooldownMultiplierInvalid }">
              {{ tr(cooldownMultiplierInvalid ? '请输入 0.1 到 100。' : '倍率越大，候选路径推进越快；实际冷却效果待任务实测。') }}
            </small>
          </label>

          <fieldset class="tuning-control">
            <legend>{{ tr('作用范围') }}</legend>
            <div class="tuning-segment ui-seg" role="group" :aria-label="tr('冷却作用范围')">
              <button
                type="button"
                class="ui-seg-btn"
                :class="{ 'is-on': cooldownScope === 'self' }"
                :aria-pressed="cooldownScope === 'self'"
                :disabled="interactionLocked"
                @click="cooldownScope = 'self'"
              >{{ tr('仅自己（默认）') }}</button>
              <button
                type="button"
                class="ui-seg-btn"
                :class="{ 'is-on': cooldownScope === 'party' }"
                :aria-pressed="cooldownScope === 'party'"
                :disabled="interactionLocked"
                @click="cooldownScope = 'party'"
              >{{ tr('应用全队（实验）') }}</button>
            </div>
          </fieldset>
        </div>

        <p v-if="cooldownScope === 'party'" class="tuning-warning ui-notice is-warn">
          {{ tr('全队识别仍待实机：只在离线任务中测试，并确认没有影响敌人或召唤物。') }}
        </p>
        <p v-if="combatTuningStatus.cooldown.error" class="feature-error ui-notice is-danger">{{ tr(combatTuningStatus.cooldown.error) }}</p>

        <div class="tuning-evidence">
          <span>{{ tr(combatTuningStatus.cooldown.evidenceNote || '三个候选入口会在连接后逐一校验；未通过时拒绝写入。') }}</span>
          <strong>{{ tuningCurrentSetting('cooldown') }}</strong>
        </div>

        <footer class="tuning-actions">
          <span class="feature-proof">{{ tuningProof('cooldown') }}</span>
          <div class="ui-actions">
            <button
              v-if="combatTuningStatus.cooldown.enabled"
              type="button"
              class="ui-btn is-ghost is-sm"
              :disabled="tuningDisabled('cooldown', 'restore')"
              @click="requestCombatTuningRestore('cooldown')"
            >{{ tr('恢复默认') }}</button>
            <button type="submit" class="ui-btn is-primary is-sm" :disabled="tuningDisabled('cooldown')">
              {{ tr(busyFeatureID === COOLDOWN_FEATURE.id ? '回读中…' : combatTuningStatus.cooldown.enabled ? '更新并回读' : '应用设置') }}
            </button>
          </div>
        </footer>
      </form>
    </section>

    <section
      v-if="mode === 'combat'"
      class="tuning-priority ui-card ui-panel"
      :class="{ 'is-on': summonDurationStatus.enabled }"
      :aria-label="tr('召唤持续时间')"
    >
      <header class="tuning-priority-head">
        <div>
          <p>{{ tr('召唤战斗规则') }}</p>
          <h2>{{ tr('召唤持续时间') }}</h2>
          <span>{{ tr('延长召唤效果在场时间，或设为无限持续；默认关闭；切换页面后继续生效。') }}</span>
        </div>
        <div class="tuning-tags">
          <span class="ui-tag is-warn">{{ tr('2.0.3 / 2.0.4 / 2.0.5 · 实验') }}</span>
          <span class="ui-tag" :class="summonDurationStatus.enabled ? 'is-ok' : summonDurationStatus.available ? 'is-info' : 'is-danger'">{{ summonDurationStateLabel() }}</span>
        </div>
      </header>

      <form class="tuning-form" @submit.prevent="requestSummonDurationApply">
        <div class="tuning-control-grid is-charge">
          <fieldset class="tuning-control">
            <legend>{{ tr('调整方式') }}</legend>
            <div class="tuning-segment ui-seg" role="group" :aria-label="tr('召唤持续时间调整方式')">
              <button type="button" class="ui-seg-btn" :class="{ 'is-on': summonDurationMode === 'multiplier' }" :aria-pressed="summonDurationMode === 'multiplier'" :disabled="interactionLocked || summonDurationStatus.enabled" @click="summonDurationMode = 'multiplier'">{{ tr('持续时间倍率') }}</button>
              <button type="button" class="ui-seg-btn" :class="{ 'is-on': summonDurationMode === 'infinite' }" :aria-pressed="summonDurationMode === 'infinite'" :disabled="interactionLocked || summonDurationStatus.enabled" @click="summonDurationMode = 'infinite'">{{ tr('无限持续') }}</button>
            </div>
          </fieldset>

          <label v-if="summonDurationMode === 'multiplier'" class="tuning-control ui-field">
            <span class="ui-field-label">{{ tr('持续时间倍率') }}</span>
            <span class="tuning-number"><input v-model.trim="summonDurationMultiplier" class="ui-input" type="number" min="0.1" max="16" step="0.1" inputmode="decimal" :aria-invalid="summonDurationMultiplierInvalid" :disabled="interactionLocked || summonDurationStatus.enabled"><b aria-hidden="true">×</b></span>
            <small :class="{ 'is-error': summonDurationMultiplierInvalid }">{{ tr(summonDurationMultiplierInvalid ? '请输入 0.1 到 16.0。' : '1× 为游戏默认；建议从 2× 或 4× 开始。') }}</small>
          </label>
        </div>

        <p v-if="summonDurationStatus.error" class="feature-error ui-notice is-danger">{{ tr(summonDurationStatus.error) }}</p>
        <div class="tuning-evidence"><span>{{ tr(summonDurationStatus.evidenceNote || '连接后校验 2.0.3 / 2.0.4 / 2.0.5 召唤持续时间入口和原字节。') }}</span><strong>{{ summonDurationStatus.enabled ? tr(summonDurationStatus.infinite ? '无限持续' : `${summonDurationStatus.durationMultiplier}×`) : tr('当前保持游戏默认') }}</strong></div>
        <footer class="tuning-actions">
          <span class="feature-proof">{{ connected && summonDurationStatus.rva ? `RVA ${formatRVA(summonDurationStatus.rva)}` : tr('尚未写入，游戏保持默认') }}</span>
          <div class="ui-actions">
            <button v-if="summonDurationStatus.enabled || summonDurationStatus.currentBytes" type="button" class="ui-btn is-ghost is-sm" :disabled="summonDurationDisabled('restore')" @click="requestSummonDurationRestore">{{ tr('恢复默认') }}</button>
            <button type="submit" class="ui-btn is-primary is-sm" :disabled="summonDurationDisabled()">{{ tr(busyFeatureID === SUMMON_DURATION_FEATURE.id ? '回读中…' : '应用设置') }}</button>
          </div>
        </footer>
      </form>
    </section>

    <section
      v-if="mode === 'characters'"
      class="tuning-priority ui-card ui-panel"
      :class="{ 'is-on': combatTuningStatus.actionSpeed.enabled }"
      :aria-label="tr('人物动作速度')"
    >
      <header class="tuning-priority-head">
        <div>
          <p>{{ tr('通用角色功能') }}</p>
          <h2>{{ tr('人物动作速度') }}</h2>
          <span>{{ tr('只调整角色动作节奏，不改变整个游戏的时间速度；默认只作用于自己。') }}</span>
        </div>
        <div class="tuning-tags">
          <span class="ui-tag is-warn">{{ tr('实验候选') }}</span>
          <span class="ui-tag" :class="combatTuningStatus.actionSpeed.enabled ? 'is-ok' : combatTuningStatus.actionSpeed.available ? 'is-info' : 'is-danger'">
            {{ tuningStateLabel('actionSpeed') }}
          </span>
        </div>
      </header>

      <form class="tuning-form" @submit.prevent="requestCombatTuningApply('actionSpeed')">
        <div class="tuning-control-grid is-charge">
          <label class="tuning-control ui-field">
            <span class="ui-field-label">{{ tr('动作速度倍率') }}</span>
            <span class="tuning-number">
              <input
                v-model.trim="actionSpeedMultiplier"
                class="ui-input"
                type="number"
                min="0.1"
                max="5"
                step="0.1"
                inputmode="decimal"
                :aria-invalid="actionSpeedMultiplierInvalid"
                :disabled="interactionLocked"
              >
              <b aria-hidden="true">×</b>
            </span>
            <small :class="{ 'is-error': actionSpeedMultiplierInvalid }">
              {{ tr(actionSpeedMultiplierInvalid ? '请输入 0.1 到 5.0。' : '1× 为游戏默认；建议从 1.25× 或 1.5× 开始。') }}
            </small>
          </label>

          <fieldset class="tuning-control">
            <legend>{{ tr('作用范围') }}</legend>
            <div class="tuning-segment ui-seg" role="group" :aria-label="tr('人物动作速度作用范围')">
              <button type="button" class="ui-seg-btn" :class="{ 'is-on': actionSpeedScope === 'self' }" :aria-pressed="actionSpeedScope === 'self'" :disabled="interactionLocked" @click="actionSpeedScope = 'self'">{{ tr('仅自己（默认）') }}</button>
              <button type="button" class="ui-seg-btn" :class="{ 'is-on': actionSpeedScope === 'party' }" :aria-pressed="actionSpeedScope === 'party'" :disabled="interactionLocked" @click="actionSpeedScope = 'party'">{{ tr('应用全队（实验）') }}</button>
            </div>
          </fieldset>
        </div>

        <p v-if="actionSpeedScope === 'party'" class="tuning-warning ui-notice is-warn">{{ tr('全队范围会处理同一角色上下文中的队友；请先在离线队伍中确认动作、判定和技能释放。') }}</p>
        <p v-if="combatTuningStatus.actionSpeed.error" class="feature-error ui-notice is-danger">{{ tr(combatTuningStatus.actionSpeed.error) }}</p>

        <div class="tuning-evidence">
          <span>{{ tr(combatTuningStatus.actionSpeed.evidenceNote || '连接后校验 2.0.3 / 2.0.4 / 2.0.5 的人物动作字段入口；任何原字节不一致都会拒绝写入。') }}</span>
          <strong>{{ tuningCurrentSetting('actionSpeed') }}</strong>
        </div>

        <footer class="tuning-actions">
          <span class="feature-proof">{{ tuningProof('actionSpeed') }}</span>
          <div class="ui-actions">
            <button v-if="combatTuningStatus.actionSpeed.enabled" type="button" class="ui-btn is-ghost is-sm" :disabled="tuningDisabled('actionSpeed', 'restore')" @click="requestCombatTuningRestore('actionSpeed')">{{ tr('恢复默认') }}</button>
            <button type="submit" class="ui-btn is-primary is-sm" :disabled="tuningDisabled('actionSpeed')">{{ tr(busyFeatureID === ACTION_SPEED_FEATURE.id ? '回读中…' : combatTuningStatus.actionSpeed.enabled ? '更新并回读' : '应用设置') }}</button>
          </div>
        </footer>
      </form>
    </section>

    <section
      v-if="mode === 'characters'"
      class="tuning-priority ui-card ui-panel"
      :class="{ 'is-on': combatTuningStatus.charge.enabled }"
      :aria-label="tr('三角色共享蓄力调整')"
    >
      <header class="tuning-priority-head">
        <div>
          <p>{{ tr('靠前常用功能') }}</p>
          <h2>{{ tr('伊欧 / 巴萨拉卡 / 冈达葛萨：蓄力调整') }}</h2>
          <span>{{ tr('这是一个共享候选入口：先选瞬间蓄力或速度倍率，再统一应用。') }}</span>
        </div>
        <div class="tuning-tags">
          <span class="ui-tag is-warn">{{ tr('实验候选') }}</span>
          <span class="ui-tag" :class="combatTuningStatus.charge.enabled ? 'is-ok' : combatTuningStatus.charge.available ? 'is-info' : 'is-danger'">
            {{ tuningStateLabel('charge') }}
          </span>
        </div>
      </header>

      <form class="tuning-form" @submit.prevent="requestCombatTuningApply('charge')">
        <div class="tuning-control-grid is-charge">
          <fieldset class="tuning-control">
            <legend>{{ tr('调整方式') }}</legend>
            <div class="tuning-segment ui-seg" role="group" :aria-label="tr('蓄力调整方式')">
              <button
                type="button"
                class="ui-seg-btn"
                :class="{ 'is-on': chargeMode === 'multiplier' }"
                :aria-pressed="chargeMode === 'multiplier'"
                :disabled="interactionLocked"
                @click="chargeMode = 'multiplier'"
              >{{ tr('速度倍率') }}</button>
              <button
                type="button"
                class="ui-seg-btn"
                :class="{ 'is-on': chargeMode === 'instant' }"
                :aria-pressed="chargeMode === 'instant'"
                :disabled="interactionLocked"
                @click="chargeMode = 'instant'"
              >{{ tr('瞬间蓄力') }}</button>
            </div>
          </fieldset>

          <label v-if="chargeMode === 'multiplier'" class="tuning-control ui-field">
            <span class="ui-field-label">{{ tr('蓄力速度倍率') }}</span>
            <span class="tuning-number">
              <input
                v-model.trim="chargeMultiplier"
                class="ui-input"
                type="number"
                min="0.1"
                max="100"
                step="0.1"
                inputmode="decimal"
                :aria-invalid="chargeMultiplierInvalid"
                :disabled="interactionLocked"
              >
              <b aria-hidden="true">×</b>
            </span>
            <small :class="{ 'is-error': chargeMultiplierInvalid }">
              {{ tr(chargeMultiplierInvalid ? '请输入 0.1 到 100。' : '倍率越大，候选蓄力推进越快；实际动作范围待逐角色实测。') }}
            </small>
          </label>
        </div>

        <p v-if="chargeCatalogConflict" class="tuning-warning ui-notice is-warn">
          {{ tr('冈达葛萨「瞬间直冲拳」正在占用相关机制；先恢复它，再应用共享蓄力调整。') }}
        </p>
        <p v-if="combatTuningStatus.charge.error" class="feature-error ui-notice is-danger">{{ tr(combatTuningStatus.charge.error) }}</p>

        <div class="tuning-evidence">
          <span>{{ tr(combatTuningStatus.charge.evidenceNote || '共享候选入口会在连接后校验；未证明作用范围前不会标为正式功能。') }}</span>
          <strong>{{ tuningCurrentSetting('charge') }}</strong>
        </div>

        <footer class="tuning-actions">
          <span class="feature-proof">{{ tuningProof('charge') }}</span>
          <div class="ui-actions">
            <button
              v-if="combatTuningStatus.charge.enabled"
              type="button"
              class="ui-btn is-ghost is-sm"
              :disabled="tuningDisabled('charge', 'restore')"
              @click="requestCombatTuningRestore('charge')"
            >{{ tr('恢复默认') }}</button>
            <button type="submit" class="ui-btn is-primary is-sm" :disabled="tuningDisabled('charge')">
              {{ tr(busyFeatureID === CHARGE_FEATURE.id ? '回读中…' : combatTuningStatus.charge.enabled ? '更新并回读' : '应用设置') }}
            </button>
          </div>
        </footer>
      </form>
    </section>

    <section class="patch-browser ui-card ui-panel">
      <header class="patch-browser-head">
        <div>
          <h2 class="ui-section-title">{{ modeCopy.label }} {{ tr('目录') }} <small>{{ visibleFeatureCount }} {{ tr('项') }}</small></h2>
        </div>
        <label class="patch-search ui-field">
          <span class="ui-field-label">{{ tr('搜索名称、角色或分组') }}</span>
          <span class="search-field">
            <span class="search-glyph" aria-hidden="true"></span>
            <input v-model.trim="searchQuery" class="ui-input" type="search" autocomplete="off" :placeholder="tr('输入关键词筛选')">
          </span>
        </label>
      </header>

      <section v-if="mode === 'quest'" class="task-rule-grid" :aria-label="tr('任务规则快捷设置')">
        <article class="task-rule-card patch-feature-card" :class="{ 'is-on': taskRulesStatus.scoreMultiplier.enabled, 'needs-recovery': taskRulesStatus.scoreMultiplier.rvas.length && !taskRulesStatus.scoreMultiplier.enabled }" :aria-busy="busyFeatureID === TASK_SCORE_FEATURE.id">
          <div class="patch-feature-summary">
            <div class="feature-title-block">
              <div class="feature-kicker"><span>{{ tr('任务结算') }}</span><span>{{ tr('分数') }}</span></div>
              <h4>{{ tr('任务分数倍率') }}</h4>
              <small class="feature-evidence">{{ tr('只放大任务结算分数，不改变奖励物品数量，也不与掉落倍率叠加。') }}</small>
            </div>
            <span class="ui-tag" :class="taskRulesStatus.scoreMultiplier.enabled ? 'is-ok' : taskRulesStatus.scoreMultiplier.rvas.length ? 'is-warn' : taskRulesStatus.scoreMultiplier.available ? 'is-info' : 'is-danger'">{{ taskRuleStateLabel('score') }}</span>
          </div>
          <label class="task-rule-field ui-field">
            <span class="ui-field-label">{{ tr('分数倍率') }}</span>
            <span class="tuning-number"><input v-model.trim="taskScoreMultiplier" class="ui-input" type="number" min="0.1" max="16" step="0.1"><b>×</b></span>
            <small :class="{ 'is-error': taskScoreMultiplierInvalid }">{{ tr('可填 0.1–16；默认建议 2×。') }}</small>
          </label>
          <p v-if="taskRulesStatus.scoreMultiplier.error" class="feature-error ui-notice is-danger">{{ tr(taskRulesStatus.scoreMultiplier.error) }}</p>
          <div class="patch-feature-actions">
            <span class="feature-proof">{{ taskRuleProof('score') }}</span>
            <div class="ui-actions">
              <button v-if="taskRulesStatus.scoreMultiplier.enabled || taskRulesStatus.scoreMultiplier.rvas.length" type="button" class="ui-btn is-ghost is-sm" :disabled="taskRuleDisabled('score', 'restore')" @click="requestTaskRuleRestore('score')">{{ tr('恢复默认') }}</button>
              <button type="button" class="ui-btn is-primary is-sm" :disabled="taskRuleDisabled('score')" @click="requestTaskRuleApply('score')">{{ tr(busyFeatureID === TASK_SCORE_FEATURE.id ? '回读中…' : taskRulesStatus.scoreMultiplier.enabled ? '更新并回读' : '应用设置') }}</button>
            </div>
          </div>
        </article>

        <article class="task-rule-card patch-feature-card" :class="{ 'is-on': taskRulesStatus.sideQuestAutoComplete.enabled, 'needs-recovery': taskRulesStatus.sideQuestAutoComplete.rvas.length && !taskRulesStatus.sideQuestAutoComplete.enabled }" :aria-busy="busyFeatureID === TASK_SIDE_QUEST_FEATURE.id">
          <div class="patch-feature-summary">
            <div class="feature-title-block">
              <div class="feature-kicker"><span>{{ tr('任务进度') }}</span><span>{{ tr('支线目标') }}</span></div>
              <h4>{{ tr('自动补齐支线目标进度') }}</h4>
              <small class="feature-evidence">{{ tr('当支线目标更新时把当前计数补到要求值；它不直接发奖，奖励仍由任务结算流程处理。') }}</small>
            </div>
            <span class="ui-tag" :class="taskRulesStatus.sideQuestAutoComplete.enabled ? 'is-ok' : taskRulesStatus.sideQuestAutoComplete.rvas.length ? 'is-warn' : taskRulesStatus.sideQuestAutoComplete.available ? 'is-info' : 'is-danger'">{{ taskRuleStateLabel('sideQuest') }}</span>
          </div>
          <p v-if="taskRulesStatus.sideQuestAutoComplete.error" class="feature-error ui-notice is-danger">{{ tr(taskRulesStatus.sideQuestAutoComplete.error) }}</p>
          <div class="task-rule-note ui-notice is-info">{{ tr('适合不想反复核对支线计数的任务；默认关闭，开启后常驻到你主动恢复或安全断开。') }}</div>
          <div class="patch-feature-actions">
            <span class="feature-proof">{{ taskRuleProof('sideQuest') }}</span>
            <div class="ui-actions">
              <button v-if="taskRulesStatus.sideQuestAutoComplete.enabled || taskRulesStatus.sideQuestAutoComplete.rvas.length" type="button" class="ui-btn is-ghost is-sm" :disabled="taskRuleDisabled('sideQuest', 'restore')" @click="requestTaskRuleRestore('sideQuest')">{{ tr('恢复默认') }}</button>
              <button type="button" class="ui-btn is-primary is-sm" :disabled="taskRuleDisabled('sideQuest')" @click="requestTaskRuleApply('sideQuest')">{{ tr(busyFeatureID === TASK_SIDE_QUEST_FEATURE.id ? '回读中…' : taskRulesStatus.sideQuestAutoComplete.enabled ? '保持开启' : '开启') }}</button>
            </div>
          </div>
        </article>
      </section>

      <article
        v-if="mode === 'quest'"
        class="conflux-feature patch-feature-card"
        :class="{ 'is-on': confluxStatus.enabled, 'needs-recovery': confluxStatus.owned && !confluxStatus.enabled }"
        :aria-busy="busyFeatureID === CONFLUX_FEATURE.id"
      >
        <div class="patch-feature-summary">
          <div class="feature-title-block">
            <div class="feature-kicker"><span>{{ tr('极沌空域') }}</span><span>{{ tr('等待计时器') }}</span></div>
            <h4>{{ tr('极沌空域快速等待') }}</h4>
            <small class="feature-evidence">{{ tr('只缩短已验证的等待计时器；不自动领奖，也不自动重新进入任务。') }}</small>
          </div>
          <span class="ui-tag" :class="confluxStatus.enabled && confluxStatus.owned ? 'is-ok' : confluxStatus.owned ? 'is-warn' : confluxStatus.available ? 'is-info' : 'is-danger'">
            {{ confluxStateLabel() }}
          </span>
        </div>

        <p v-if="confluxStatus.error" class="feature-error ui-notice" :class="confluxStatus.owned ? 'is-danger' : 'is-info'">{{ tr(confluxStatus.error) }}</p>

        <div class="conflux-readback" aria-label="极沌空域计时器回读">
          <span><small>{{ tr('运行模式') }}</small><strong>{{ confluxStatus.mode === 1 ? tr('Endless') : tr('未进入') }}</strong></span>
          <span><small>{{ tr('本轮初始') }}</small><strong>{{ formatConfluxSeconds(confluxStatus.initialSeconds) }}</strong></span>
          <span><small>{{ tr('当前等待') }}</small><strong>{{ formatConfluxSeconds(confluxStatus.currentSeconds) }}</strong></span>
        </div>

        <div class="patch-feature-actions">
          <span class="feature-proof">{{ tr(confluxStatus.owned ? '已保存本工具写入前的 12 项原始配置' : connected && !confluxStatus.verified ? '先校验游戏版本，再读取任务计时器' : connected ? '进入极沌空域任务后刷新读取' : '连接后读取状态') }}</span>
          <button
            type="button"
            role="switch"
            class="feature-switch ui-btn is-sm"
            :class="{ 'is-primary': !confluxStatus.owned, 'is-danger': confluxStatus.owned }"
            :aria-checked="confluxStatus.enabled"
            :aria-label="`${tr('极沌空域快速等待')}: ${tr(!confluxStatus.verified ? '验证并读取' : confluxStatus.owned ? '恢复默认' : '开启')}`"
            :disabled="confluxDisabled()"
            @click="requestConfluxChange"
          >
            <span class="switch-track" :class="{ 'is-on': confluxStatus.enabled }" aria-hidden="true"><i></i></span>
            <span>{{ tr(busyFeatureID === CONFLUX_FEATURE.id ? '回读中…' : !confluxStatus.verified ? '验证并读取' : confluxStatus.owned ? '恢复默认' : '开启') }}</span>
          </button>
        </div>
      </article>

      <div v-if="catalogLoading" class="patch-empty ui-empty">{{ tr('正在读取功能目录…') }}</div>
      <div v-else-if="!groups.length" class="patch-empty ui-empty">
        <strong>{{ tr('没有匹配的功能') }}</strong>
        <span>{{ tr('换一个角色名、功能名或分组关键词。') }}</span>
      </div>
      <div v-else class="patch-feature-workspace">
        <aside class="patch-group-pane">
          <label class="patch-group-select ui-field">
            <span class="ui-field-label">{{ tr('显示范围') }}</span>
            <select class="ui-select" :value="browserScope" @change="selectBrowserScope($event.target.value)">
              <option value="all">{{ tr('全部功能') }}</option>
              <option value="active">{{ tr('已开启与待恢复') }}</option>
              <option v-for="group in groups" :key="group.key" :value="group.key">{{ group.label }} ({{ group.features.length }})</option>
            </select>
          </label>
          <nav class="patch-group-disclosure" :aria-label="`${modeCopy.label} ${tr('分组')}`">
            <button
              type="button"
              class="patch-group-button is-overview"
              :class="{ 'is-on': browserScope === 'all' }"
              :aria-pressed="browserScope === 'all'"
              @click="selectBrowserScope('all')"
            >
              <span>{{ tr('全部功能') }}</span>
              <b>{{ groups.reduce((total, group) => total + group.features.length, 0) }}</b>
            </button>
            <button
              type="button"
              class="patch-group-button is-overview"
              :class="{ 'is-on': browserScope === 'active' }"
              :aria-pressed="browserScope === 'active'"
              @click="selectBrowserScope('active')"
            >
              <span>{{ tr('已开启与待恢复') }}</span>
              <b>{{ browserOwnedFeatureCount }}</b>
            </button>
            <button
              v-for="group in groups"
              :key="group.key"
              type="button"
              class="patch-group-button"
              :class="{ 'is-on': browserScope === group.key }"
              :aria-expanded="browserScope === group.key"
              :aria-controls="`patch-group-${mode}-${group.key}`"
              @click="selectGroup(group.key)"
            >
              <span>{{ group.label }}</span>
              <b>{{ group.features.length }}</b>
            </button>
          </nav>
        </aside>

        <div v-if="displayedGroups.length" class="patch-feature-sections">
        <section v-for="group in displayedGroups" :id="`patch-group-${mode}-${group.key}`" :key="group.key" class="patch-feature-column" :aria-label="`${group.label} ${tr('功能')}`">
          <header class="patch-group-heading">
            <div>
              <span>{{ modeCopy.label }}</span>
              <h3>{{ group.label }}</h3>
            </div>
            <small>{{ group.features.length }} {{ tr('项已验证补丁') }}</small>
          </header>

          <div class="patch-feature-list">
            <article
              v-for="feature in group.features"
              :key="feature.id"
              class="patch-feature-card ui-card"
              :class="{ 'is-on': statusFor(feature).enabled, 'needs-recovery': ownsFeature(feature) && !statusFor(feature).enabled }"
              :aria-busy="busyFeatureID === feature.id"
            >
              <div class="patch-feature-summary">
                <div class="feature-title-block">
                  <div class="feature-kicker">
                    <span>{{ displayGroupName(feature.character || feature.group) }}</span>
                    <span>{{ tr('补丁') }} {{ feature.catalogId }}</span>
                  </div>
                  <h4>{{ displayFeatureName(feature) }}</h4>
                  <small v-if="displayFeatureSummary(feature)" class="feature-description">{{ displayFeatureSummary(feature) }}</small>
                  <small
                    v-if="feature.evidenceNote"
                    class="feature-evidence"
                    :class="{ 'is-candidate': String(feature.evidenceLevel || '').startsWith('candidate_') }"
                  >{{ tr(feature.evidenceNote) }}</small>
                </div>
                <span class="ui-tag" :class="statusFor(feature).enabled ? 'is-ok' : ownsFeature(feature) ? 'is-warn' : activeConflictFor(feature) ? 'is-danger' : 'is-info'">
                  {{ featureStateLabel(feature) }}
                </span>
              </div>

              <p v-if="activeConflictFor(feature)" class="feature-conflict ui-notice is-warn">
                {{ tr('与「') }}{{ displayFeatureName(activeConflictFor(feature)) }}{{ tr('」互斥；先恢复该功能后才能启用。') }}
              </p>
              <p v-else-if="displayFeatureError(feature)" class="feature-error ui-notice is-danger">{{ displayFeatureError(feature) }}</p>

              <div class="patch-feature-actions">
                <span class="feature-proof">
                  {{ tr(statusFor(feature).rvas.length ? `已回读 ${statusFor(feature).rvas.length} 个写入点` : connected ? '首次启用时定位并保存原字节' : '连接后读取状态') }}
                </span>
                <button
                  type="button"
                  role="switch"
                  class="feature-switch ui-btn is-sm"
                  :class="{ 'is-primary': !ownsFeature(feature), 'is-danger': ownsFeature(feature) }"
                  :aria-checked="statusFor(feature).enabled"
                  :aria-label="`${displayFeatureName(feature)}: ${tr(ownsFeature(feature) ? '恢复默认' : '开启')}`"
                  :disabled="featureDisabled(feature)"
                  @click="requestFeatureChange(feature)"
                >
                  <span class="switch-track" :class="{ 'is-on': statusFor(feature).enabled }" aria-hidden="true"><i></i></span>
                  <span>{{ tr(busyFeatureID === feature.id ? '回读中…' : ownsFeature(feature) ? '恢复默认' : '开启') }}</span>
                </button>
              </div>

              <details class="patch-technical ui-disclosure">
                <summary>{{ tr('技术详情') }}</summary>
                <dl>
                  <div><dt>{{ tr('目录 ID') }}</dt><dd><code>{{ feature.id }}</code></dd></div>
                  <div><dt>{{ tr('写入点') }}</dt><dd>{{ feature.sites.length }}</dd></div>
                  <div v-if="feature.conflictGroup"><dt>{{ tr('冲突组') }}</dt><dd>{{ feature.conflictGroup }}</dd></div>
                </dl>
                <ol class="site-list">
                  <li v-for="(site, index) in feature.sites" :key="`${feature.id}-${index}`">
                    <div><b>{{ site.symbol }}</b><span>RVA {{ formatRVA(statusFor(feature).rvas[index]) }}</span></div>
                    <code>{{ site.aob }}</code>
                    <small>{{ tr('偏移') }} {{ site.offset }} · {{ tr('当前字节') }} {{ statusFor(feature).currentBytes[index] || tr('未读取') }}</small>
                  </li>
                </ol>
              </details>
            </article>
          </div>
        </section>
        </div>
        <div v-else class="patch-empty ui-empty is-compact">
          <strong>{{ tr('当前没有已开启或待恢复的功能') }}</strong>
          <span>{{ tr('开启功能后，它会在这里集中显示。') }}</span>
        </div>
      </div>
    </section>

    <div v-if="pendingConfirmationFeature" class="patch-confirm-backdrop" @click.self="cancelOfflineConfirmation">
      <section class="patch-confirm-dialog ui-card" role="dialog" aria-modal="true" aria-labelledby="patch-offline-title" aria-describedby="patch-offline-copy" @keydown.esc="cancelOfflineConfirmation" @keydown.tab="trapConfirmationFocus">
        <span class="confirm-emblem" aria-hidden="true"><i></i></span>
        <div>
          <p class="confirm-kicker">{{ tr('首次启用确认') }}</p>
          <h2 id="patch-offline-title">{{ tr('仅离线/单机使用') }}</h2>
          <p id="patch-offline-copy">{{ tr('这些功能会直接修改游戏运行时规则。请确认当前不在联机房间，并只在离线或单机内容中使用。本次打开应用只确认一次。') }}</p>
        </div>
        <div class="confirm-feature"><span>{{ tr('即将开启') }}</span><strong>{{ displayFeatureName(pendingConfirmationFeature) }}</strong></div>
        <div class="confirm-actions ui-actions is-end">
          <button ref="confirmationCancelButton" type="button" class="ui-btn is-ghost" @click="cancelOfflineConfirmation">{{ tr('取消') }}</button>
          <button ref="confirmationButton" type="button" class="ui-btn is-primary" @click="confirmOfflineUse">{{ tr('确认仅在单机使用并开启') }}</button>
        </div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.runtime-patch-page {
  width:100%;
  max-width:none;
  padding-bottom:var(--space-8);
  gap:var(--space-4);
}

.patch-connection {
  display:flex;
  min-height:72px;
  flex-direction:row;
  align-items:center;
  justify-content:space-between;
  border-color:var(--border-strong);
  background:color-mix(in srgb,var(--surface-card-pop) 90%,transparent);
}

.patch-connection-main,
.patch-connection-actions,
.patch-browser-head,
.patch-feature-summary,
.patch-feature-actions,
.patch-group-heading {
  min-width:0;
  display:flex;
  align-items:center;
  gap:var(--space-3);
}

.patch-connection-main { flex:1 1 auto; }
.patch-connection-actions { flex:0 0 auto; }
.connection-copy { min-width:0; flex:1 1 auto; }
.connection-copy strong,.connection-copy span { display:block; }
.connection-copy strong { color:var(--text-primary); font-size:var(--fs-md); }
.connection-copy span { margin-top:2px; color:var(--text-muted); font-size:var(--fs-sm); line-height:var(--lh-normal); }

.connection-emblem,
.confirm-emblem {
  position:relative;
  width:34px;
  height:34px;
  flex:0 0 34px;
  display:grid;
  place-items:center;
  border:1px solid var(--accent-border);
  border-radius:var(--radius-pill);
  background:var(--accent-soft);
}
.connection-emblem::before,.connection-emblem::after {
  content:"";
  position:absolute;
  width:7px;
  height:11px;
  border:2px solid var(--accent-hover);
}
.connection-emblem::before { left:8px; border-right:0; border-radius:7px 0 0 7px; }
.connection-emblem::after { right:8px; border-left:0; border-radius:0 7px 7px 0; }
.connection-emblem i { width:8px; height:2px; background:var(--accent-hover); }

.patch-live-region {
  min-height:42px;
  display:flex;
  align-items:center;
  gap:var(--space-3);
}
.live-mark { width:7px; height:7px; flex:0 0 7px; border-radius:50%; background:currentColor; }

.tuning-priority {
  position:relative;
  gap:var(--space-4);
  border-color:var(--accent-border);
  background:color-mix(in srgb,var(--surface-card-pop) 94%,var(--accent-soft));
  box-shadow:3px 0 0 var(--accent-border) inset;
}
.tuning-priority.is-on {
  border-color:var(--success);
  background:color-mix(in srgb,var(--success-bg) 28%,var(--surface-card-pop));
  box-shadow:3px 0 0 var(--success) inset;
}
.tuning-priority-head {
  min-width:0;
  display:flex;
  align-items:flex-start;
  justify-content:space-between;
  gap:var(--space-4);
  padding-bottom:var(--space-3);
  border-bottom:1px solid var(--border-soft);
}
.tuning-priority-head > div:first-child { min-width:0; }
.tuning-priority-head p {
  margin:0;
  color:var(--accent);
  font-size:var(--fs-xs);
  font-weight:var(--fw-bold);
  letter-spacing:.08em;
}
.tuning-priority-head h2 {
  margin:var(--space-1) 0 0;
  color:var(--text-primary);
  font-family:var(--font-display);
  font-size:var(--fs-lg);
  line-height:var(--lh-tight);
}
.tuning-priority-head > div:first-child > span {
  display:block;
  margin-top:var(--space-2);
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}
.tuning-tags {
  flex:0 0 auto;
  display:flex;
  flex-wrap:wrap;
  justify-content:flex-end;
  gap:var(--space-2);
}
.tuning-form { min-width:0; }
.tuning-control-grid {
  min-width:0;
  display:grid;
  grid-template-columns:minmax(0,1fr) minmax(180px,.72fr) minmax(0,1fr);
  gap:var(--space-4);
  align-items:start;
}
.tuning-control-grid.is-charge { grid-template-columns:minmax(0,1fr) minmax(200px,1fr); }
.tuning-control {
  min-width:0;
  margin:0;
  padding:0;
  border:0;
}
.tuning-control legend,
.tuning-control > .ui-field-label {
  display:block;
  margin:0 0 var(--space-2);
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  font-weight:var(--fw-semibold);
}
.tuning-segment.ui-seg {
  width:100%;
  display:flex;
  gap:0;
  padding:0;
  border:0;
  border-bottom:1px solid var(--border-default);
  border-radius:0;
  background:transparent;
}
.tuning-segment .ui-seg-btn {
  min-width:0;
  min-height:var(--control-height-sm);
  flex:1 1 0;
  padding:0 var(--space-3);
  border:0;
  border-bottom:2px solid transparent;
  border-radius:0;
  color:var(--text-muted);
  background:transparent;
  box-shadow:none;
  white-space:normal;
}
.tuning-segment .ui-seg-btn:hover,.tuning-segment .ui-seg-btn:focus-visible { background:var(--state-hover); }
.tuning-segment .ui-seg-btn.is-on {
  border-bottom-color:var(--selected-bar);
  color:var(--selected-fg);
  background:color-mix(in srgb,var(--selected-bg) 54%,transparent);
  box-shadow:none;
}
.tuning-number { position:relative; display:block; }
.tuning-number .ui-input { width:100%; padding-right:36px; font-family:var(--font-data); }
.tuning-number b {
  position:absolute;
  top:50%;
  right:var(--space-3);
  color:var(--text-muted);
  font-family:var(--font-data);
  transform:translateY(-50%);
  pointer-events:none;
}
.tuning-control > small {
  display:block;
  margin-top:var(--space-2);
  color:var(--text-muted);
  font-size:var(--fs-xs);
  line-height:var(--lh-normal);
}
.tuning-control > small.is-error { color:var(--danger); }
.tuning-warning { margin:var(--space-3) 0 0; }
.tuning-evidence {
  min-width:0;
  display:flex;
  align-items:flex-start;
  justify-content:space-between;
  gap:var(--space-4);
  margin-top:var(--space-4);
  padding:var(--space-3);
  border:1px solid var(--border-soft);
  border-radius:var(--radius-sm);
  background:var(--surface-sunken);
}
.tuning-evidence span {
  min-width:0;
  color:var(--warning-ink);
  font-size:var(--fs-xs);
  line-height:var(--lh-normal);
}
.tuning-evidence strong {
  flex:0 0 auto;
  color:var(--text-primary);
  font-family:var(--font-data);
  font-size:var(--fs-sm);
  white-space:nowrap;
}
.tuning-actions {
  min-width:0;
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:var(--space-4);
  margin-top:var(--space-3);
}

.patch-browser {
  gap:var(--space-4);
  border-color:var(--border-default);
  background:color-mix(in srgb,var(--surface-card) 91%,transparent);
}
.patch-browser-head { align-items:flex-end; justify-content:space-between; }
.patch-browser-head > div { min-width:0; flex:1 1 auto; }
.patch-search { width:min(330px,44%); flex:0 1 330px; }
.search-field { position:relative; display:block; }
.search-field .ui-input { padding-left:38px; }
.search-glyph {
  position:absolute;
  z-index:1;
  top:50%;
  left:14px;
  width:12px;
  height:12px;
  border:2px solid var(--text-muted);
  border-radius:50%;
  transform:translateY(-58%);
  pointer-events:none;
}
.search-glyph::after { content:""; position:absolute; right:-5px; bottom:-3px; width:6px; height:2px; background:var(--text-muted); transform:rotate(45deg); }

.patch-feature-workspace { min-width:0; display:grid; grid-template-columns:minmax(0,1fr); gap:var(--space-5); align-items:start; }
.patch-group-pane { min-width:0; }
.patch-group-select { display:flex; }
.patch-group-disclosure { display:none; min-width:0; }
.patch-group-button {
  width:100%;
  min-height:40px;
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:var(--space-2);
  padding:0 var(--space-3);
  border:1px solid transparent;
  border-radius:var(--radius-sm);
  color:var(--text-secondary);
  background:transparent;
  font:inherit;
  font-size:var(--fs-sm);
  text-align:left;
  cursor:pointer;
  transition:var(--transition-control);
}
.patch-group-button:hover,.patch-group-button:focus-visible { border-color:var(--border-default); background:var(--state-hover); color:var(--text-primary); }
.patch-group-button.is-on { border-color:var(--selected-border); background:var(--selected-bg); color:var(--selected-fg); box-shadow:3px 0 0 var(--selected-bar) inset; }
.patch-group-button.is-overview { color:var(--text-primary); font-weight:var(--fw-semibold); }
.patch-group-button.is-overview + .patch-group-button.is-overview { margin-bottom:var(--space-2); }
.patch-group-button span { min-width:0; overflow-wrap:anywhere; }
.patch-group-button b { min-width:24px; padding:1px var(--space-2); border-radius:var(--radius-pill); background:var(--surface-card-pop); color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); text-align:center; }

.patch-feature-sections { min-width:0; display:flex; flex-direction:column; gap:var(--space-6); }
.patch-feature-column { min-width:0; scroll-margin-top:96px; }
.patch-group-heading { justify-content:space-between; margin-bottom:var(--space-3); padding:0 var(--space-1) var(--space-3); border-bottom:1px solid var(--border-soft); }
.patch-group-heading > div span { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.08em; }
.patch-group-heading h3 { margin:2px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); line-height:var(--lh-tight); }
.patch-group-heading small { color:var(--text-muted); font-size:var(--fs-xs); }
.patch-feature-list { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,330px),1fr)); gap:var(--space-3); align-items:stretch; }

.patch-feature-card {
  min-width:0;
  min-height:214px;
  display:flex;
  flex-direction:column;
  padding:var(--space-4);
  border-color:var(--border-default);
  background:var(--surface-card-pop);
  box-shadow:none;
  transition:border-color var(--dur-base) var(--ease-out),background-color var(--dur-base) var(--ease-out),box-shadow var(--dur-base) var(--ease-out);
}
.patch-feature-card.is-on { border-color:var(--success); background:color-mix(in srgb,var(--success-bg) 36%,var(--surface-card-pop)); box-shadow:3px 0 0 var(--success) inset; }
.patch-feature-card.needs-recovery { border-color:var(--warning); box-shadow:3px 0 0 var(--warning) inset; }
.task-rule-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); margin-bottom:var(--space-3); }
.task-rule-card { border:1px solid var(--border-default); border-left:3px solid var(--accent-border); min-width:0; }
.task-rule-field { max-width:260px; margin-top:var(--space-3); }
.task-rule-field > small { color:var(--text-muted); font-size:var(--fs-xs); }
.task-rule-field > small.is-error { color:var(--danger); }
.task-rule-note { margin-top:var(--space-3); }
.conflux-feature { border:1px solid var(--border-default); border-left:3px solid var(--accent-border); }
.conflux-readback {
  min-width:0;
  display:grid;
  grid-template-columns:repeat(3,minmax(0,1fr));
  gap:var(--space-2);
  margin-top:var(--space-3);
}
.conflux-readback > span {
  min-width:0;
  display:flex;
  align-items:baseline;
  justify-content:space-between;
  gap:var(--space-2);
  padding:var(--space-2) var(--space-3);
  border:1px solid var(--border-soft);
  border-radius:var(--radius-sm);
  background:var(--surface-field);
}
.conflux-readback small { color:var(--text-muted); font-size:var(--fs-xs); }
.conflux-readback strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-sm); white-space:nowrap; }
.patch-feature-summary { align-items:flex-start; justify-content:space-between; }
.feature-title-block { min-width:0; }
.feature-kicker { display:flex; flex-wrap:wrap; gap:var(--space-2); color:var(--text-muted); font-size:var(--fs-xs); }
.feature-kicker span + span::before { content:"·"; margin-right:var(--space-2); }
.feature-title-block h4 { margin:var(--space-1) 0 0; color:var(--text-primary); font-size:var(--fs-base); line-height:var(--lh-tight); overflow-wrap:anywhere; }
.feature-description { display:block; margin-top:var(--space-2); color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.feature-evidence { display:block; margin-top:var(--space-2); color:var(--text-muted); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.feature-evidence.is-candidate { color:var(--warning-ink); }
.feature-conflict,.feature-error { margin:var(--space-3) 0 0; }
.patch-feature-actions { justify-content:space-between; margin-top:auto; padding-top:var(--space-3); border-top:1px solid var(--border-soft); }
.feature-proof { min-width:0; color:var(--text-muted); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.feature-switch { min-width:124px; flex:0 0 auto; }
.switch-track { width:27px; height:16px; flex:0 0 27px; padding:2px; border-radius:var(--radius-pill); background:color-mix(in srgb,var(--text-muted) 44%,transparent); transition:background-color var(--dur-base) var(--ease-out); }
.switch-track i { display:block; width:12px; height:12px; border-radius:50%; background:var(--surface-card-pop); box-shadow:0 1px 2px rgba(55,39,19,.24); transition:transform var(--dur-base) var(--ease-out); }
.switch-track.is-on { background:var(--success); }
.switch-track.is-on i { transform:translateX(11px); }

.patch-technical { margin-top:var(--space-3); box-shadow:none; }
.patch-technical > summary { min-height:var(--control-height-sm); padding-block:var(--space-2); color:var(--text-muted); font-size:var(--fs-xs); }
.patch-technical dl { display:flex; flex-wrap:wrap; gap:var(--space-3) var(--space-5); }
.patch-technical dl > div { min-width:0; }
.patch-technical dt { color:var(--text-muted); font-size:var(--fs-xs); }
.patch-technical dd { margin:2px 0 0; color:var(--text-secondary); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.site-list { display:flex; min-width:0; flex-direction:column; gap:var(--space-2); padding:0; list-style:none; }
.site-list li { min-width:0; padding:var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.site-list li > div { display:flex; min-width:0; justify-content:space-between; gap:var(--space-3); color:var(--text-secondary); font-size:var(--fs-xs); }
.site-list code,.site-list small { display:block; margin-top:var(--space-2); color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); overflow-wrap:anywhere; word-break:break-word; }
.site-list code { color:var(--text-secondary); }

.patch-empty { min-height:180px; display:grid; place-content:center; gap:var(--space-2); text-align:center; }
.patch-empty strong { color:var(--text-primary); font-size:var(--fs-md); }
.patch-empty span { color:var(--text-muted); font-size:var(--fs-sm); }

.patch-confirm-backdrop {
  position:fixed;
  z-index:60;
  inset:0;
  display:grid;
  place-items:center;
  padding:var(--space-5);
  background:rgba(50,37,20,.44);
  backdrop-filter:blur(3px);
}
.patch-confirm-dialog { width:min(520px,100%); max-height:calc(100dvh - 32px); overflow:auto; display:grid; grid-template-columns:auto minmax(0,1fr); gap:var(--space-4); padding:var(--space-7); border-color:var(--border-strong); background:var(--surface-card-pop); box-shadow:var(--shadow-3); }
.confirm-emblem::before,.confirm-emblem::after { content:""; position:absolute; background:var(--accent-hover); }
.confirm-emblem::before { width:2px; height:13px; top:7px; }
.confirm-emblem::after { width:2px; height:2px; bottom:7px; border-radius:50%; }
.confirm-kicker { margin:0; color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.08em; }
.patch-confirm-dialog h2 { margin:var(--space-1) 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); }
.patch-confirm-dialog p:not(.confirm-kicker) { margin:var(--space-3) 0 0; color:var(--text-secondary); font-size:var(--fs-md); line-height:var(--lh-relaxed); }
.confirm-feature { grid-column:1 / -1; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); padding:var(--space-3) var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--accent-soft); }
.confirm-feature span { color:var(--text-muted); font-size:var(--fs-sm); }
.confirm-feature strong { color:var(--text-primary); text-align:right; }
.confirm-actions { grid-column:1 / -1; }

@container tool-panel (min-width:680px) {
  .patch-feature-workspace { grid-template-columns:minmax(178px,224px) minmax(0,1fr); gap:var(--space-5); }
  .patch-group-select { display:none; }
  .patch-group-disclosure { display:flex; flex-direction:column; gap:var(--space-1); }
  .patch-group-pane { position:sticky; top:var(--space-3); }
}

@container tool-panel (min-width:1180px) {
  .patch-feature-workspace { grid-template-columns:232px minmax(0,1fr); gap:var(--space-6); }
  .patch-feature-list { grid-template-columns:repeat(auto-fit,minmax(340px,1fr)); }
}

@container tool-panel (max-width:679px) {
  .task-rule-grid { grid-template-columns:minmax(0,1fr); }
  .patch-browser-head { align-items:stretch; flex-direction:column; }
  .patch-search { width:100%; flex-basis:auto; }
  .patch-feature-workspace { grid-template-columns:minmax(0,1fr); }
  .patch-feature-sections { gap:var(--space-5); }
  .patch-connection { align-items:stretch; flex-direction:column; }
  .patch-connection-actions { width:100%; }
  .patch-connection-actions .ui-btn { flex:1 1 150px; }
  .tuning-control-grid,.tuning-control-grid.is-charge { grid-template-columns:minmax(0,1fr) minmax(0,1fr); }
  .tuning-control-grid .tuning-control:last-child:nth-child(3) { grid-column:1 / -1; }
}

@container tool-panel (max-width:520px) {
  .runtime-patch-page { gap:var(--space-3); padding-bottom:var(--space-5); }
  .patch-browser { padding:var(--space-4); }
  .patch-feature-actions { align-items:stretch; flex-direction:column; }
  .feature-switch { width:100%; }
  .patch-group-heading { align-items:flex-start; flex-direction:column; }
  .conflux-readback { grid-template-columns:minmax(0,1fr); }
  .tuning-priority { padding:var(--space-4); }
  .tuning-priority-head,.tuning-actions,.tuning-evidence { align-items:stretch; flex-direction:column; }
  .tuning-tags { justify-content:flex-start; }
  .tuning-control-grid,.tuning-control-grid.is-charge { grid-template-columns:minmax(0,1fr); }
  .tuning-control-grid .tuning-control:last-child:nth-child(3) { grid-column:auto; }
  .tuning-actions .ui-actions { width:100%; }
  .tuning-actions .ui-btn { flex:1 1 130px; }
  .tuning-evidence strong { white-space:normal; }
}

@container tool-panel (max-width:340px) {
  .patch-browser-head .ui-section-copy { display:none; }
  .patch-connection-main { align-items:flex-start; flex-wrap:wrap; }
  .patch-connection-main > .ui-tag { margin-left:46px; }
  .patch-feature-card { padding:var(--space-3); }
  .patch-feature-card { min-height:0; }
  .patch-feature-summary { gap:var(--space-2); }
  .patch-feature-summary > .ui-tag { flex:0 0 auto; }
  .patch-confirm-dialog { grid-template-columns:minmax(0,1fr); padding:var(--space-5); }
  .confirm-emblem { display:none; }
  .confirm-feature,.confirm-actions { grid-column:1; }
  .confirm-actions .ui-btn { width:100%; }
}

@media (prefers-reduced-motion:reduce) {
  .patch-feature-card,.patch-group-button,.switch-track,.switch-track i,.tuning-segment .ui-seg-btn { transition:none; }
}
</style>
