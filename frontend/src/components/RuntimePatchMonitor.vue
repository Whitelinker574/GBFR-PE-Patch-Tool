<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  CharaAcquire,
  CharaRelease,
	RuntimeEmergencyStop,
  RuntimePatchPartyMonitorOwned,
  RuntimePatchSelectedItemReadOwned,
  RuntimePatchSelectedItemsDisableOwned,
  RuntimePatchSelectedItemsEnableOwned,
  RuntimePatchSelectedItemsStatusOwned,
  RuntimeSpatialGravitySetEnabledOwned,
  RuntimeSpatialGravityStatusOwned,
  RuntimeSpatialMoveOwned,
  RuntimeSpatialTeleportOwned,
} from '../../wailsjs/go/backend/App'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { language } from '../i18n.js'
import { createOperationGate } from '../runtimeOperationGate.js'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'
import {
  consumeRuntimePatchSelectedCapture,
  formatRuntimeAddress,
  formatRuntimeCoordinate,
  formatRuntimeInteger,
  normalizeRuntimePatchPartySnapshot,
  normalizeRuntimePatchSelectedRecord,
  normalizeRuntimePatchSelectedStatus,
  normalizeRuntimeSpatialGravityStatus,
  normalizeRuntimeSpatialTeleport,
  runtimeMonitorRoleName,
  runtimeMonitorText,
  selectedCapturePhase,
} from '../runtimePatchMonitorView.js'
import RuntimeLoadoutDetector from './RuntimeLoadoutDetector.vue'

const emit = defineEmits(['status', 'deploy-loadout'])
const props = defineProps({
  pageActive: { type: Boolean, default: true },
  mode: {
    type: String,
    default: 'party',
    validator: value => ['party', 'spatial', 'items'].includes(value),
  },
})
const RUNTIME_LEASE_SCOPE = 'runtime-patch-selected-item-monitor'
const ITEM_KINDS = Object.freeze(['material', 'keyItem'])

const activeTab = ref(props.mode)
const activeOperation = ref(null)
const releasePending = ref(false)
const connected = ref(false)
const processInfo = ref({ pid: 0, moduleBase: 0 })
const selectedStatus = ref(null)
const spatialSnapshot = ref(null)
const lastTeleport = ref(null)
const flightDirection = ref('')
const flightPending = ref(false)
const FLIGHT_FRAME_MS = 45
const flightSpeed = ref(8)
const gravityStatus = ref(null)
const gravityError = ref('')
const spatialOrigin = ref(null)
const spatialBookmarkName = ref('')
const spatialBookmarks = ref([])
const spatialBookmarkStorageKey = 'gbfr-codex-spatial-bookmarks-v1'
const teleportTarget = reactive({ x: '', y: '', z: '' })
const selectedRecords = reactive({ material: null, keyItem: null })
const consumedSelections = reactive({ material: false, keyItem: false })
const liveMessage = ref(t('statusDisconnected'))
const liveTone = ref('info')
const operationGate = createOperationGate()
let disposed = false
let lifecycleEpoch = 0
let connectionOwnerToken = ''
let flightSession = 0
let stopEmergencyEvents = null

const operationBusy = computed(() => activeOperation.value !== null)
const interactionLocked = computed(() => operationBusy.value || releasePending.value || flightPending.value || Boolean(flightDirection.value))
const connectionStateLabel = computed(() => {
  if (releasePending.value) return t('releasing')
  return connected.value ? t('connected') : t('notConnected')
})
const connectionStateClass = computed(() => releasePending.value ? 'is-warn' : connected.value ? 'is-ok' : 'is-info')
const captureRefreshing = computed(() => activeOperation.value?.kind === 'capture-refresh')
const captureChanging = computed(() => ['capture-enable', 'capture-disable'].includes(activeOperation.value?.kind))
const gravityChanging = computed(() => activeOperation.value?.kind === 'gravity-change')
const gravityRefreshing = computed(() => activeOperation.value?.kind === 'gravity-refresh')
const gravityDetail = computed(() => {
  if (gravityError.value) return gravityError.value
  if (gravityStatus.value?.error) return gravityStatus.value.error
  if (gravityStatus.value?.recoveryPending) return t('spatialGravityRecovery')
  if (gravityStatus.value?.enabled) return t('spatialGravityEnabled')
  if (gravityStatus.value?.available) return t('spatialGravityReady')
  return t('spatialGravityUnavailable')
})

function t(key, parameters) {
  return runtimeMonitorText(key, language.value, parameters)
}

function errorMessage(error) {
  return (error instanceof Error ? error.message : String(error || '')).replace(/^Error:\s*/i, '')
}

function announce(message, tone = 'info') {
  liveMessage.value = message
  liveTone.value = tone
  emit('status', message, tone === 'danger' ? 'error' : tone === 'ok' ? 'success' : tone)
}

function beginOperation(kind) {
  if (disposed) return null
  const token = operationGate.begin(kind)
  if (!token) return null
  activeOperation.value = Object.freeze({ token, kind })
  return token
}

function operationIsCurrent(token, epoch, ownerToken = connectionOwnerToken) {
  return !disposed
    && lifecycleEpoch === epoch
    && operationGate.isCurrent(token)
    && (!ownerToken || ownerToken === connectionOwnerToken)
}

function finishOperation(token) {
  if (!operationGate.isCurrent(token)) return
  operationGate.finish(token)
  activeOperation.value = null
}

function resetSelectedItems() {
  selectedStatus.value = null
  selectedRecords.material = null
  selectedRecords.keyItem = null
  consumedSelections.material = false
  consumedSelections.keyItem = false
}

function clearRuntimeState() {
  stopFlight()
  operationGate.invalidate()
  activeOperation.value = null
  releasePending.value = false
  connected.value = false
  connectionOwnerToken = ''
  processInfo.value = { pid: 0, moduleBase: 0 }
  resetSelectedItems()
  spatialSnapshot.value = null
  lastTeleport.value = null
  gravityStatus.value = null
  gravityError.value = ''
  spatialOrigin.value = null
  teleportTarget.x = ''
  teleportTarget.y = ''
  teleportTarget.z = ''
}

function loadSpatialBookmarks() {
  try {
    const value = JSON.parse(localStorage.getItem(spatialBookmarkStorageKey) || '[]')
    spatialBookmarks.value = Array.isArray(value)
      ? value.filter(item => item?.id && item?.name && ['x', 'y', 'z'].every(axis => Number.isFinite(Number(item.position?.[axis])))).slice(0, 20)
      : []
  } catch { spatialBookmarks.value = [] }
}

function persistSpatialBookmarks() {
  localStorage.setItem(spatialBookmarkStorageKey, JSON.stringify(spatialBookmarks.value.slice(0, 20)))
}

function currentPlayerPosition() {
  const player = spatialSnapshot.value?.entities?.[0]
  return player?.present ? { x: Number(player.position.x), y: Number(player.position.y), z: Number(player.position.z) } : null
}

function fillTeleportTarget(position) {
  if (!position) return
  for (const axis of ['x', 'y', 'z']) teleportTarget[axis] = String(position[axis])
}

function saveSpatialBookmark() {
  const position = currentPlayerPosition()
  const name = spatialBookmarkName.value.trim()
  if (!position || !name) return
  const existing = spatialBookmarks.value.find(item => item.name.toLocaleLowerCase() === name.toLocaleLowerCase())
  if (existing) {
    existing.position = position
    existing.updatedAt = new Date().toISOString()
  } else {
    spatialBookmarks.value.unshift({ id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, name, position, updatedAt: new Date().toISOString() })
  }
  persistSpatialBookmarks()
  spatialBookmarkName.value = ''
  announce(t('spatialBookmarkSaved', { name }), 'ok')
}

function deleteSpatialBookmark(id) {
  spatialBookmarks.value = spatialBookmarks.value.filter(item => item.id !== id)
  persistSpatialBookmarks()
}

const flightDirections = Object.freeze({
  'x-': { x: -1, y: 0, z: 0 },
  'x+': { x: 1, y: 0, z: 0 },
  'y+': { x: 0, y: 1, z: 0 },
  'y-': { x: 0, y: -1, z: 0 },
  'z-': { x: 0, y: 0, z: -1 },
  'z+': { x: 0, y: 0, z: 1 },
})

function flightDelta(direction) {
  const unit = flightDirections[direction]
  const speed = Number(flightSpeed.value)
  if (!unit || !Number.isFinite(speed) || speed < 0.1 || speed > 1000) throw new Error(t('spatialFlightInvalidStep'))
  const distance = speed * FLIGHT_FRAME_MS / 1000
  return { x: unit.x * distance, y: unit.y * distance, z: unit.z * distance }
}

function stopFlight() {
  flightSession += 1
  flightDirection.value = ''
}

function applyEmergencyStopResult(result) {
	stopFlight()
	clearRuntimeState()
	if (result?.restored) announce(t('statusEmergencyStopped'), 'ok')
	else announce(t('statusEmergencyFailed', { error: String(result?.detail || 'unknown restoration error') }), 'danger')
}

async function emergencyStop() {
	stopFlight()
	try {
		applyEmergencyStopResult(await RuntimeEmergencyStop())
	} catch (error) {
		clearRuntimeState()
		announce(t('statusEmergencyFailed', { error: errorMessage(error) }), 'danger')
	}
}

function onEmergencyKeydown(event) {
	if (event.key !== 'Escape' && event.key !== 'F12') return
	event.preventDefault()
	if (event.key === 'F12') {
		stopFlight()
		return
	}
	void emergencyStop()
}

function waitFlightFrame(milliseconds = FLIGHT_FRAME_MS) {
  return new Promise(resolve => window.setTimeout(resolve, milliseconds))
}

async function runFlight(direction, session, ownerToken, epoch) {
  while (!disposed && flightDirection.value === direction && flightSession === session && connected.value && connectionOwnerToken === ownerToken && lifecycleEpoch === epoch) {
    try {
      flightPending.value = true
      const result = normalizeRuntimeSpatialTeleport(await RuntimeSpatialMoveOwned(ownerToken, flightDelta(direction)), ownerToken, processInfo.value.pid)
      if (disposed || flightSession !== session || connectionOwnerToken !== ownerToken || lifecycleEpoch !== epoch) return
      lastTeleport.value = result
      teleportTarget.x = String(result.observed.x)
      teleportTarget.y = String(result.observed.y)
      teleportTarget.z = String(result.observed.z)
      if (spatialSnapshot.value?.entities?.[0]) {
        const current = spatialSnapshot.value
        const entities = current.entities.map((entity, index) => index === 0
          ? Object.freeze({ ...entity, position: Object.freeze({ ...result.observed }) })
          : entity)
        spatialSnapshot.value = Object.freeze({ ...current, entities: Object.freeze(entities) })
      }
    } catch (error) {
      if (!disposed && flightSession === session) announce(t('statusSpatialFlightStopped', { error: errorMessage(error) }), 'danger')
      stopFlight()
      return
    } finally {
      flightPending.value = false
    }
    await waitFlightFrame()
  }
}

function startFlight(direction, event) {
  if (!connected.value || !spatialSnapshot.value || operationBusy.value || releasePending.value || flightDirection.value) return
  event?.currentTarget?.setPointerCapture?.(event.pointerId)
  const session = ++flightSession
  flightDirection.value = direction
  announce(t('statusSpatialFlightActive'), 'warn')
  void runFlight(direction, session, connectionOwnerToken, lifecycleEpoch)
}

function normalizedProcessInfo(value) {
  const ownerToken = String(value?.ownerToken ?? '').trim()
  const pid = Number(value?.pid ?? Number.NaN)
  const moduleBase = Number(value?.moduleBase ?? Number.NaN)
  if (!ownerToken) throw new Error(t('errorMissingOwner'))
  if (!Number.isSafeInteger(pid) || pid <= 0) throw new Error(t('errorInvalidPid'))
  if (!Number.isSafeInteger(moduleBase) || moduleBase <= 0) throw new Error(t('errorInvalidModule'))
  return { ownerToken, pid, moduleBase }
}

async function connect() {
  if (connected.value || releasePending.value) return
  const operationToken = beginOperation('connect')
  if (!operationToken) return
  const epoch = ++lifecycleEpoch
  let acquiredOwnerToken = ''
  try {
    const acquireResult = await CharaAcquire(nextRuntimeAcquireRequestID())
    acquiredOwnerToken = String(acquireResult?.ownerToken ?? '').trim()
    const acquired = normalizedProcessInfo(acquireResult)
    if (!operationIsCurrent(operationToken, epoch, '')) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, CharaRelease)
      return
    }
    connectionOwnerToken = acquiredOwnerToken
    processInfo.value = { pid: acquired.pid, moduleBase: acquired.moduleBase }
    connected.value = true
    try {
      gravityStatus.value = normalizeRuntimeSpatialGravityStatus(await RuntimeSpatialGravityStatusOwned(acquiredOwnerToken), acquiredOwnerToken, acquired.pid)
      gravityError.value = ''
    } catch (error) {
      gravityStatus.value = null
      gravityError.value = errorMessage(error)
    }
    announce(t('statusConnected', { pid: formatRuntimeInteger(acquired.pid, language.value) }), 'ok')
  } catch (error) {
    let cleanupError = null
    if (acquiredOwnerToken) {
      try {
        await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, CharaRelease)
      } catch (nextError) {
        cleanupError = nextError
      }
    }
    if (!disposed && epoch === lifecycleEpoch) {
      const detail = cleanupError ? `${errorMessage(error)}; ${errorMessage(cleanupError)}` : errorMessage(error)
      announce(t('statusActionFailed', { error: detail }), 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

function completeRuntimeRelease(expectedOwnerToken, expectedEpoch, notification) {
  if (disposed || lifecycleEpoch !== expectedEpoch || connectionOwnerToken !== expectedOwnerToken || notification?.ownerToken !== expectedOwnerToken) return
  clearRuntimeState()
  announce(t('statusReleaseComplete'), 'ok')
}

async function disconnect() {
  stopFlight()
  const ownerToken = connectionOwnerToken
  if (!ownerToken) return
  const operationToken = beginOperation('disconnect')
  if (!operationToken) return
  const epoch = ++lifecycleEpoch
  releasePending.value = true
  try {
    await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease, notification => completeRuntimeRelease(ownerToken, epoch, notification))
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) {
      releasePending.value = true
      announce(t('statusReleaseFailed', { error: errorMessage(error) }), 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

function applySelectedStatus(status) {
  selectedStatus.value = status
  for (const kind of ITEM_KINDS) {
    if (status[kind].captured) consumedSelections[kind] = false
  }
}

async function enableCapture() {
  const operationToken = beginOperation('capture-enable')
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken || !connected.value) throw new Error(t('statusConnect'))
    const status = normalizeRuntimePatchSelectedStatus(await RuntimePatchSelectedItemsEnableOwned(ownerToken), ownerToken, processInfo.value.pid)
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    if (!status.enabled) throw new Error(t('errorCaptureEnableVerification'))
    applySelectedStatus(status)
    announce(t('statusCaptureEnabled'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
  } finally {
    finishOperation(operationToken)
  }
}

async function disableCapture() {
  const operationToken = beginOperation('capture-disable')
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken || !connected.value) throw new Error(t('statusConnect'))
    const status = normalizeRuntimePatchSelectedStatus(await RuntimePatchSelectedItemsDisableOwned(ownerToken), ownerToken, processInfo.value.pid)
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    if (status.enabled) throw new Error(t('errorCaptureDisableVerification'))
    applySelectedStatus(status)
    resetSelectedItems()
    selectedStatus.value = status
    announce(t('statusCaptureDisabled'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
  } finally {
    finishOperation(operationToken)
  }
}

async function refreshCaptureStatus() {
  const operationToken = beginOperation('capture-refresh')
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken || !connected.value) throw new Error(t('statusConnect'))
    const status = normalizeRuntimePatchSelectedStatus(await RuntimePatchSelectedItemsStatusOwned(ownerToken), ownerToken, processInfo.value.pid)
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    applySelectedStatus(status)
    announce(t('statusCaptureRefreshed'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
  } finally {
    finishOperation(operationToken)
  }
}

async function readSelectedItem(kind) {
  const operationToken = beginOperation('item-read')
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  const capture = selectedStatus.value?.[kind]
  const expectedSelectedAddr = Number(capture?.selectedAddr ?? Number.NaN)
  try {
    if (!ownerToken || !connected.value) throw new Error(t('statusConnect'))
    if (!capture?.captured || !Number.isSafeInteger(expectedSelectedAddr) || expectedSelectedAddr <= 0) throw new Error(t('stepSelect'))
    const record = normalizeRuntimePatchSelectedRecord(await RuntimePatchSelectedItemReadOwned(ownerToken, { kind, expectedSelectedAddr }), kind, expectedSelectedAddr)
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    selectedRecords[kind] = Object.freeze({ ...record })
    consumedSelections[kind] = true
    selectedStatus.value = consumeRuntimePatchSelectedCapture(selectedStatus.value, kind)
    try {
      const status = normalizeRuntimePatchSelectedStatus(await RuntimePatchSelectedItemsStatusOwned(ownerToken), ownerToken, processInfo.value.pid)
      if (operationIsCurrent(operationToken, epoch, ownerToken)) applySelectedStatus(status)
    } catch (error) {
      if (operationIsCurrent(operationToken, epoch, ownerToken)) announce(t('statusReadRefreshFailed', { error: errorMessage(error) }), 'warn')
      return
    }
    if (operationIsCurrent(operationToken, epoch, ownerToken)) announce(t('statusItemRead', { name: record.name }), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
  } finally {
    finishOperation(operationToken)
  }
}

function applySpatialSnapshot(value) {
  spatialSnapshot.value = value
  const player = value.entities[0]
  if (player?.present) {
    if (!spatialOrigin.value) spatialOrigin.value = Object.freeze({ ...player.position })
    teleportTarget.x = String(player.position.x)
    teleportTarget.y = String(player.position.y)
    teleportTarget.z = String(player.position.z)
  }
}

async function readSpatialSnapshot() {
  const operationToken = beginOperation('spatial-read')
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken || !connected.value) throw new Error(t('statusConnect'))
    const snapshot = normalizeRuntimePatchPartySnapshot(await RuntimePatchPartyMonitorOwned(ownerToken), ownerToken, processInfo.value.pid)
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    applySpatialSnapshot(snapshot)
    lastTeleport.value = null
    announce(t('statusSpatialRead'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
  } finally {
    finishOperation(operationToken)
  }
}

function teleportVector() {
  const target = { x: Number(teleportTarget.x), y: Number(teleportTarget.y), z: Number(teleportTarget.z) }
  for (const value of Object.values(target)) {
    if (!Number.isFinite(value) || Math.abs(value) > 10_000_000) throw new Error(language.value === 'en' ? 'Coordinates must be finite and within world bounds.' : '坐标必须是有限数值，并且不能超出世界边界。')
  }
  return target
}

async function teleportPlayer() {
  const operationToken = beginOperation('spatial-teleport')
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken || !connected.value) throw new Error(t('statusConnect'))
    const result = normalizeRuntimeSpatialTeleport(await RuntimeSpatialTeleportOwned(ownerToken, teleportVector()), ownerToken, processInfo.value.pid)
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    lastTeleport.value = result
    teleportTarget.x = String(result.observed.x)
    teleportTarget.y = String(result.observed.y)
    teleportTarget.z = String(result.observed.z)
    announce(t('statusSpatialTeleport'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
  } finally {
    finishOperation(operationToken)
  }
}

async function refreshGravityStatus() {
  const operationToken = beginOperation('gravity-refresh')
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken || !connected.value) throw new Error(t('statusConnect'))
    gravityStatus.value = normalizeRuntimeSpatialGravityStatus(await RuntimeSpatialGravityStatusOwned(ownerToken), ownerToken, processInfo.value.pid)
    gravityError.value = ''
    if (gravityStatus.value.error) throw new Error(gravityStatus.value.error)
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) {
      gravityError.value = errorMessage(error)
      announce(t('statusActionFailed', { error: gravityError.value }), 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

async function setGravityEnabled(enabled) {
  const operationToken = beginOperation('gravity-change')
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken || !connected.value) throw new Error(t('statusConnect'))
    const status = normalizeRuntimeSpatialGravityStatus(await RuntimeSpatialGravitySetEnabledOwned(ownerToken, enabled), ownerToken, processInfo.value.pid)
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    gravityStatus.value = status
    gravityError.value = ''
    if (status.error) throw new Error(status.error)
    if (status.enabled !== enabled) throw new Error(t('spatialGravityUnavailable'))
    announce(t(enabled ? 'statusSpatialGravityEnabled' : 'statusSpatialGravityDisabled'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) {
      gravityError.value = errorMessage(error)
      try {
        gravityStatus.value = normalizeRuntimeSpatialGravityStatus(await RuntimeSpatialGravityStatusOwned(ownerToken), ownerToken, processInfo.value.pid)
        if (gravityStatus.value.recoveryPending) gravityError.value = ''
      } catch {
        // Keep the original operation error; disconnect will retry backend recovery.
      }
      announce(t('statusActionFailed', { error: gravityError.value }), 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

function capturePhase(kind) {
  return selectedCapturePhase(selectedStatus.value, kind, consumedSelections[kind])
}

function capturePhaseText(kind) {
  const phase = capturePhase(kind)
  if (phase === 'reselect') return t('needsReselection')
  return t(({ disabled: 'captureDisabled', awaiting: 'captureAwaiting', ready: 'captureReady' })[phase])
}

function capturePhaseClass(kind) {
  return ({ disabled: '', awaiting: 'is-info', ready: 'is-ok', reselect: 'is-warn' })[capturePhase(kind)]
}

function categoryText(record) {
  return record.category || t('unknownCategory')
}

onBeforeUnmount(() => {
  stopFlight()
  disposed = true
  lifecycleEpoch += 1
  operationGate.invalidate()
  activeOperation.value = null
  const ownerToken = connectionOwnerToken
  connectionOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease)
  window.removeEventListener('blur', stopFlight)
  document.removeEventListener('visibilitychange', onVisibilityChange)
	window.removeEventListener('keydown', onEmergencyKeydown)
	stopEmergencyEvents?.()
	stopEmergencyEvents = null
})

function onVisibilityChange() {
  if (document.hidden) stopFlight()
}

onMounted(() => {
  loadSpatialBookmarks()
  window.addEventListener('blur', stopFlight)
  document.addEventListener('visibilitychange', onVisibilityChange)
	window.addEventListener('keydown', onEmergencyKeydown)
	stopEmergencyEvents = EventsOn('runtime-emergency-stop', applyEmergencyStopResult)
})

watch(activeTab, value => { if (value !== 'spatial') stopFlight() })
watch(() => props.mode, value => { activeTab.value = value }, { immediate: true })
watch(() => props.pageActive, value => { if (!value) stopFlight() })
</script>

<template>
  <section class="runtime-monitor-page ui-page ui-page-stack is-fluid" data-page="runtime-patch-runtime-monitor" :aria-busy="operationBusy">
    <RuntimeLoadoutDetector v-show="activeTab === 'party'" id="runtime-monitor-panel-party" @status="(message, tone) => emit('status', message, tone)" @deploy-loadout="payload => emit('deploy-loadout', payload)" />

    <template v-if="activeTab !== 'party'">
      <section class="monitor-connection ui-card ui-panel is-compact" :aria-label="`${t('memoryMonitoring')} · ${t('readOnly')}`">
        <div class="connection-summary"><span class="connection-emblem" :class="{ 'is-on': connected }" aria-hidden="true"><i></i></span><div><strong>{{ connectionStateLabel }}</strong><small v-if="connected">PID {{ formatRuntimeInteger(processInfo.pid, language) }} · {{ t('gameVersion') }} 2.0.2</small><small v-else>{{ t('statusConnect') }}</small></div><span class="ui-tag" :class="connectionStateClass">{{ connectionStateLabel }}</span></div>
        <div class="ui-actions connection-actions"><button v-if="!connected" type="button" class="ui-btn is-primary" :disabled="interactionLocked" @click="connect">{{ activeOperation?.kind === 'connect' ? t('working') : t('connect') }}</button><button v-else type="button" class="ui-btn is-ghost" :disabled="operationBusy" @click="disconnect">{{ releasePending ? t('retryDisconnect') : t('disconnect') }}</button><button type="button" class="ui-btn is-danger" @click="emergencyStop">{{ t('emergencyStop') }}</button></div>
      </section>
      <p class="emergency-hint">{{ t('emergencyStopHint') }}</p>
      <div class="monitor-live ui-notice" :class="`is-${liveTone}`" aria-live="polite" aria-atomic="true"><span class="live-mark" aria-hidden="true"></span><span>{{ liveMessage }}</span></div>
    </template>

    <section v-if="activeTab === 'spatial'" id="runtime-monitor-panel-spatial" class="spatial-panel ui-card ui-panel" data-monitor-panel="spatial">
      <header class="panel-heading"><div><h3 class="ui-section-title">{{ t('spatialTitle') }}</h3><p class="ui-section-copy">{{ t('spatialSummary') }}</p></div><button type="button" class="ui-btn is-primary is-sm" :disabled="interactionLocked || !connected" @click="readSpatialSnapshot">{{ activeOperation?.kind === 'spatial-read' ? t('spatialReading') : t('spatialRead') }}</button></header>
      <div v-if="!spatialSnapshot" class="monitor-empty ui-empty"><strong>{{ t('spatialCurrent') }}</strong><span>{{ t('spatialEmpty') }}</span></div>
      <div v-else class="spatial-entity-grid">
        <article v-for="entity in spatialSnapshot.entities" :key="entity.role" class="spatial-entity ui-card is-flat" :class="{ 'is-empty': !entity.present }">
          <header><strong>{{ runtimeMonitorRoleName(entity.role, language) }}</strong><span class="ui-tag" :class="entity.present ? 'is-ok' : ''">{{ entity.present ? t('verifiedSnapshot') : t('notInParty') }}</span></header>
          <div v-if="entity.present" class="coordinate-row"><span v-for="axis in ['x', 'y', 'z']" :key="axis"><small>{{ axis.toUpperCase() }}</small><b>{{ formatRuntimeCoordinate(entity.position[axis]) }}</b></span></div>
          <p v-else>{{ t('emptySlotCopy') }}</p>
        </article>
      </div>
      <section class="teleport-lab ui-card is-flat">
        <header class="panel-heading"><div><h4>{{ t('spatialTeleportTitle') }}</h4><p>{{ t('spatialTeleportSummary') }}</p></div><span class="ui-tag is-warn">{{ t('spatialExperimental') }}</span></header>
        <div class="teleport-controls">
          <label v-for="axis in ['x', 'y', 'z']" :key="axis"><span>{{ axis.toUpperCase() }}</span><input v-model="teleportTarget[axis]" class="ui-input" type="number" step="0.1" inputmode="decimal" :disabled="interactionLocked || !connected || !spatialSnapshot" /></label>
          <button type="button" class="ui-btn is-primary" :disabled="interactionLocked || !connected || !spatialSnapshot" @click="teleportPlayer">{{ activeOperation?.kind === 'spatial-teleport' ? t('spatialTeleporting') : t('spatialTeleport') }}</button>
        </div>
        <div v-if="lastTeleport" class="teleport-result"><span><small>{{ t('spatialBefore') }}</small><b>{{ ['x','y','z'].map(axis => formatRuntimeCoordinate(lastTeleport.before[axis])).join(' / ') }}</b></span><span><small>{{ t('spatialObserved') }}</small><b>{{ ['x','y','z'].map(axis => formatRuntimeCoordinate(lastTeleport.observed[axis])).join(' / ') }}</b></span></div>
        <section class="spatial-bookmarks">
          <header><div><b>{{ t('spatialBookmarks') }}</b><small>{{ t('spatialBookmarkLoad') }} · {{ t('spatialTeleport') }}</small></div><button v-if="spatialOrigin" type="button" class="ui-btn is-sm" :disabled="interactionLocked" @click="fillTeleportTarget(spatialOrigin)">{{ t('spatialSessionOrigin') }}</button></header>
          <div class="bookmark-compose"><input v-model="spatialBookmarkName" class="ui-input" maxlength="36" :placeholder="t('spatialBookmarkName')" :disabled="!spatialSnapshot" @keyup.enter="saveSpatialBookmark" /><button type="button" class="ui-btn is-sm" :disabled="!currentPlayerPosition() || !spatialBookmarkName.trim()" @click="saveSpatialBookmark">{{ t('spatialBookmarkSave') }}</button></div>
          <div v-if="spatialBookmarks.length" class="bookmark-list"><article v-for="bookmark in spatialBookmarks" :key="bookmark.id"><span><b>{{ bookmark.name }}</b><small>{{ ['x','y','z'].map(axis => formatRuntimeCoordinate(bookmark.position[axis])).join(' / ') }}</small></span><button type="button" class="ui-btn is-sm" @click="fillTeleportTarget(bookmark.position)">{{ t('spatialBookmarkLoad') }}</button><button type="button" class="bookmark-delete" :aria-label="`${t('spatialBookmarkDelete')} ${bookmark.name}`" @click="deleteSpatialBookmark(bookmark.id)">×</button></article></div>
          <p v-else>{{ t('spatialBookmarkEmpty') }}</p>
        </section>
        <p class="spatial-boundary">{{ t('spatialUnsupported') }}</p>
      </section>
      <section class="flight-lab ui-card is-flat">
        <header class="panel-heading"><div><h4>{{ t('spatialFlightTitle') }}</h4><p>{{ t('spatialFlightSummary') }}</p></div><span class="ui-tag is-warn">{{ flightDirection ? t('spatialFlightMoving') : t('spatialExperimental') }}</span></header>
        <div class="flight-console">
          <div class="flight-pad" :aria-label="t('spatialFlightDirections')">
            <button type="button" class="ui-btn flight-axis" :disabled="!connected || !spatialSnapshot || operationBusy || releasePending" @pointerdown.prevent="startFlight('x-', $event)" @pointerup="stopFlight" @pointercancel="stopFlight" @lostpointercapture="stopFlight">−X</button>
            <button type="button" class="ui-btn flight-axis" :disabled="!connected || !spatialSnapshot || operationBusy || releasePending" @pointerdown.prevent="startFlight('y+', $event)" @pointerup="stopFlight" @pointercancel="stopFlight" @lostpointercapture="stopFlight">{{ t('spatialFlightUp') }} +Y</button>
            <button type="button" class="ui-btn flight-axis" :disabled="!connected || !spatialSnapshot || operationBusy || releasePending" @pointerdown.prevent="startFlight('x+', $event)" @pointerup="stopFlight" @pointercancel="stopFlight" @lostpointercapture="stopFlight">+X</button>
            <button type="button" class="ui-btn flight-axis" :disabled="!connected || !spatialSnapshot || operationBusy || releasePending" @pointerdown.prevent="startFlight('z-', $event)" @pointerup="stopFlight" @pointercancel="stopFlight" @lostpointercapture="stopFlight">−Z</button>
            <button type="button" class="ui-btn flight-stop" :disabled="!flightDirection" @click="stopFlight">{{ t('spatialFlightStop') }}</button>
            <button type="button" class="ui-btn flight-axis" :disabled="!connected || !spatialSnapshot || operationBusy || releasePending" @pointerdown.prevent="startFlight('z+', $event)" @pointerup="stopFlight" @pointercancel="stopFlight" @lostpointercapture="stopFlight">+Z</button>
            <span></span>
            <button type="button" class="ui-btn flight-axis" :disabled="!connected || !spatialSnapshot || operationBusy || releasePending" @pointerdown.prevent="startFlight('y-', $event)" @pointerup="stopFlight" @pointercancel="stopFlight" @lostpointercapture="stopFlight">{{ t('spatialFlightDown') }} −Y</button>
            <span></span>
          </div>
          <div class="flight-settings">
            <label><span>{{ t('spatialFlightStep') }}</span><input v-model.number="flightSpeed" class="ui-input" type="number" min="0.1" max="1000" step="0.1" inputmode="decimal" :disabled="Boolean(flightDirection)" /></label>
            <div class="flight-capability" :class="{ 'is-active': gravityStatus?.enabled, 'has-error': gravityError || gravityStatus?.error }"><span><b>{{ t('spatialGravity') }}</b><small>{{ gravityDetail }}</small></span><button v-if="gravityStatus?.available" type="button" class="ui-btn is-sm" :class="gravityStatus.enabled ? 'is-ghost' : 'is-primary'" :disabled="interactionLocked || !connected" @click="setGravityEnabled(!gravityStatus.enabled)">{{ gravityChanging ? t('spatialGravityChanging') : t(gravityStatus.enabled ? 'spatialGravityDisable' : 'spatialGravityEnable') }}</button><button v-else type="button" class="ui-btn is-sm" :disabled="interactionLocked || !connected" @click="refreshGravityStatus">{{ gravityRefreshing ? t('refreshing') : t('refresh') }}</button></div>
            <div class="flight-capability"><span><b>{{ t('spatialNoclip') }}</b><small>{{ t('spatialNotLocated') }}</small></span><button type="button" class="ui-btn is-sm" disabled>{{ t('spatialUnavailable') }}</button></div>
          </div>
        </div>
        <p class="spatial-boundary">{{ t('spatialFlightBoundary') }}</p>
      </section>
    </section>

    <template v-if="activeTab === 'items'">
      <section id="runtime-monitor-panel-items" class="items-panel ui-card ui-panel" data-monitor-panel="selected-items">
        <header class="panel-heading"><div><h3 class="ui-section-title">{{ t('selectedTitle') }}</h3><p class="ui-section-copy">{{ t('selectedSummary') }}</p></div><span class="ui-tag is-ok">{{ t('readOnlyChip') }}</span></header>
        <section class="read-only-banner ui-notice is-ok"><span class="shield-mark" aria-hidden="true"><i></i></span><div><strong>{{ t('readOnlyBanner') }}</strong><p>{{ t('neverWritesSave') }}</p><small>{{ t('hookTechnical') }}</small></div></section>
        <ol class="capture-steps"><li><span>1</span>{{ t('stepConnect') }}</li><li><span>2</span>{{ t('stepEnable') }}</li><li><span>3</span>{{ t('stepSelect') }}</li><li><span>4</span>{{ t('stepRead') }}</li></ol>
        <div class="capture-toolbar ui-toolbar"><div><strong>{{ selectedStatus?.enabled ? t('captureReady') : t('captureDisabled') }}</strong><small>{{ t('readOnlyBanner') }}</small></div><div class="ui-actions"><button v-if="!selectedStatus?.enabled" type="button" class="ui-btn is-primary is-sm" :disabled="interactionLocked || !connected" @click="enableCapture">{{ captureChanging ? t('working') : t('enableCapture') }}</button><button v-else type="button" class="ui-btn is-ghost is-sm" :disabled="interactionLocked" @click="disableCapture">{{ captureChanging ? t('working') : t('disableCapture') }}</button><button type="button" class="ui-btn is-sm" :disabled="interactionLocked || !connected || !selectedStatus?.enabled" @click="refreshCaptureStatus">{{ captureRefreshing ? t('refreshing') : t('refreshCapture') }}</button></div></div>
        <div v-if="!connected" class="monitor-empty ui-empty"><strong>{{ t('notConnected') }}</strong><span>{{ t('statusConnect') }}</span></div>
        <div v-else class="capture-grid">
          <article v-for="kind in ITEM_KINDS" :key="kind" class="capture-card ui-card is-flat" :class="`phase-${capturePhase(kind)}`">
            <header class="capture-card-heading"><span class="capture-glyph" :class="`is-${kind}`" aria-hidden="true"><i></i></span><div><small>{{ t('readOnly') }}</small><h4>{{ t(kind) }}</h4></div><span class="ui-tag" :class="capturePhaseClass(kind)">{{ capturePhaseText(kind) }}</span></header>
            <div class="capture-address"><span>{{ t('selectedAddress') }}</span><code v-if="selectedStatus?.[kind]?.captured">{{ formatRuntimeAddress(selectedStatus[kind].selectedAddr) }}</code><strong v-else>{{ capturePhase(kind) === 'reselect' ? t('selectAgain') : capturePhaseText(kind) }}</strong></div>
            <button type="button" class="ui-btn is-primary capture-read" :disabled="interactionLocked || capturePhase(kind) !== 'ready'" @click="readSelectedItem(kind)">{{ activeOperation?.kind === 'item-read' ? t('reading') : t('readOnce') }}</button>
            <section v-if="selectedRecords[kind]" class="record-card" :aria-label="t('lastRead')"><div class="record-title"><span>{{ t('lastRead') }}</span><strong>{{ selectedRecords[kind].name }}</strong></div><dl><div><dt>{{ t('catalogName') }}</dt><dd>{{ selectedRecords[kind].name }}</dd></div><div><dt>{{ t('category') }}</dt><dd>{{ categoryText(selectedRecords[kind]) }}</dd></div><div><dt>{{ t('hash') }}</dt><dd><code>0x{{ selectedRecords[kind].hashHex }}</code></dd></div><div><dt>{{ t('quantity') }}</dt><dd>{{ formatRuntimeInteger(selectedRecords[kind].quantity, language) }}</dd></div><div><dt>{{ t('flags') }}</dt><dd><code>0x{{ selectedRecords[kind].flagsHex }}</code></dd></div><div><dt>{{ t('selectedAddress') }}</dt><dd><code>{{ formatRuntimeAddress(selectedRecords[kind].selectedAddr) }}</code></dd></div></dl></section>
            <div v-else class="record-empty">{{ t('noRecord') }}</div>
          </article>
        </div>
      </section>
    </template>
  </section>
</template>

<style scoped>
.runtime-monitor-page { width:100%; max-width:none; min-width:0; padding-bottom:var(--space-8); container:runtime-monitor / inline-size; }
.monitor-connection,.panel-heading,.connection-summary,.capture-card-heading { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); }
.monitor-connection { flex-direction:row; }
.connection-summary { flex:1 1 auto; justify-content:flex-start; }
.connection-summary > div { min-width:0; }
.connection-summary strong,.connection-summary small { display:block; }
.connection-summary small { margin-top:2px; color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); }
.connection-summary > .ui-tag { margin-left:auto; flex:none; min-width:4.5em; justify-content:center; }
.connection-emblem { position:relative; width:36px; height:36px; flex:0 0 36px; border:1px solid var(--border-strong); border-radius:50%; background:var(--surface-sunken); }
.connection-emblem::before,.connection-emblem::after { content:""; position:absolute; top:10px; width:7px; height:12px; border:2px solid var(--text-muted); }
.connection-emblem::before { left:7px; border-right:0; border-radius:7px 0 0 7px; }
.connection-emblem::after { right:7px; border-left:0; border-radius:0 7px 7px 0; }
.connection-emblem i { position:absolute; top:17px; left:13px; width:10px; height:2px; background:var(--text-muted); }
.connection-emblem.is-on { border-color:var(--success); background:var(--success-bg); }
.connection-emblem.is-on::before,.connection-emblem.is-on::after { border-color:var(--success-ink); }
.connection-emblem.is-on i { background:var(--success-ink); }
.monitor-live { min-height:42px; display:flex; align-items:center; gap:var(--space-3); }
.emergency-hint { margin:calc(var(--space-3) * -1) 0 0; color:var(--text-muted); font-size:var(--fs-xs); text-align:right; }
.live-mark { width:7px; height:7px; flex:0 0 7px; border-radius:50%; background:currentColor; }
.items-panel { min-width:0; }
.read-only-banner { display:flex; align-items:flex-start; gap:var(--space-4); border-left-width:4px; }
.read-only-banner > div { min-width:0; }
.read-only-banner strong { display:block; font-family:var(--font-display); font-size:var(--fs-md); }
.read-only-banner p { margin:var(--space-1) 0 0; }
.read-only-banner small { display:block; margin-top:var(--space-2); opacity:.78; }
.shield-mark { position:relative; width:34px; height:38px; flex:0 0 34px; border:2px solid currentColor; border-radius:9px 9px 14px 14px; clip-path:polygon(50% 0,100% 16%,91% 76%,50% 100%,9% 76%,0 16%); }
.capture-steps { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:var(--space-2); margin:var(--space-4) 0 0; padding:0; list-style:none; }
.capture-steps li { min-width:0; display:flex; align-items:center; gap:var(--space-2); padding:var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card-pop); color:var(--text-secondary); font-size:var(--fs-xs); }
.capture-steps li span { width:23px; height:23px; flex:0 0 23px; display:grid; place-items:center; border:1px solid var(--accent-border); border-radius:50%; color:var(--accent-hover); }
.capture-toolbar { margin-top:var(--space-4); align-items:center; justify-content:space-between; }
.capture-toolbar strong,.capture-toolbar small { display:block; }
.capture-toolbar small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.monitor-empty { min-height:180px; margin-top:var(--space-4); }
.capture-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,320px),1fr)); gap:var(--space-4); margin-top:var(--space-4); }
.capture-card { min-width:0; padding:var(--space-4); }
.capture-card.phase-ready { border-color:var(--success); box-shadow:3px 0 0 var(--success) inset; }
.capture-card.phase-reselect { border-color:var(--warning); box-shadow:3px 0 0 var(--warning) inset; }
.capture-card-heading { justify-content:flex-start; }
.capture-card-heading > div { min-width:0; flex:1 1 auto; }
.capture-card-heading small { display:block; color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.capture-card-heading h4 { margin:2px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-md); }
.capture-glyph { width:38px; height:38px; flex:0 0 38px; border:1px solid var(--accent-border); border-radius:var(--radius-sm); background:var(--accent-soft); }
.capture-address { min-height:68px; display:flex; flex-direction:column; justify-content:center; gap:var(--space-1); margin-top:var(--space-4); padding:var(--space-3) var(--space-4); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.capture-address span { color:var(--text-muted); font-size:var(--fs-xs); }
.capture-address code { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-md); overflow-wrap:anywhere; }
.capture-read { width:100%; margin-top:var(--space-3); }
.record-card { margin-top:var(--space-4); padding:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:color-mix(in srgb,var(--success-bg) 24%,var(--surface-card)); }
.record-title span,.record-title strong { display:block; }
.record-title span { color:var(--success-ink); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.record-card dl { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); margin:var(--space-3) 0 0; }
.record-card dl > div { min-width:0; padding:var(--space-2); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card-pop); }
.record-card dt { color:var(--text-muted); font-size:var(--fs-xs); }
.record-card dd { margin:2px 0 0; color:var(--text-primary); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.record-empty { margin-top:var(--space-4); padding:var(--space-5); border:1px dashed var(--border-default); border-radius:var(--radius-sm); color:var(--text-muted); text-align:center; }
.spatial-panel { display:grid; gap:var(--space-4); min-width:0; }
.spatial-entity-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,210px),1fr)); gap:var(--space-3); }
.spatial-entity { min-width:0; padding:var(--space-3); }
.spatial-entity header { display:flex; align-items:center; justify-content:space-between; gap:var(--space-2); }
.spatial-entity.is-empty { color:var(--text-muted); }
.spatial-entity p { margin:var(--space-3) 0 0; color:var(--text-muted); }
.coordinate-row { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:var(--space-2); margin-top:var(--space-3); }
.coordinate-row span,.teleport-result span { min-width:0; padding:var(--space-2); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.coordinate-row small,.coordinate-row b,.teleport-result small,.teleport-result b { display:block; overflow-wrap:anywhere; }
.coordinate-row small,.teleport-result small { color:var(--text-muted); }
.teleport-lab { display:grid; gap:var(--space-3); padding:var(--space-4); }
.teleport-lab h4,.teleport-lab p { margin:0; }
.teleport-controls { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)) minmax(170px,auto); gap:var(--space-3); align-items:end; }
.teleport-controls label { display:grid; gap:var(--space-1); min-width:0; }
.teleport-controls .ui-input { width:100%; min-width:0; }
.teleport-result { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); }
.spatial-boundary { color:var(--text-muted); }
.spatial-bookmarks { min-width:0; display:grid; gap:var(--space-3); padding:var(--space-3); border:1px solid var(--border-soft); background:var(--surface-sunken); }
.spatial-bookmarks > header { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); }.spatial-bookmarks > header > div { min-width:0; display:grid; gap:2px; }.spatial-bookmarks > header b { color:var(--text-primary); font-size:var(--fs-sm); }.spatial-bookmarks > header small,.spatial-bookmarks > p { color:var(--text-muted); font-size:var(--fs-xs); }.spatial-bookmarks > p { margin:0; }
.bookmark-compose { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-2); }.bookmark-list { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); }.bookmark-list article { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto 28px; gap:var(--space-2); align-items:center; padding:var(--space-2); border:1px solid var(--border-default); background:var(--surface-card); }.bookmark-list article > span { min-width:0; display:grid; gap:1px; }.bookmark-list b,.bookmark-list small { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.bookmark-list b { color:var(--text-primary); font-size:var(--fs-xs); }.bookmark-list small { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-2xs); }.bookmark-delete { width:28px; height:28px; border:0; background:transparent; color:var(--text-muted); cursor:pointer; }.bookmark-delete:hover { color:var(--danger-ink); background:var(--danger-bg); }
.flight-lab { display:grid; gap:var(--space-3); padding:var(--space-4); }
.flight-lab h4,.flight-lab p { margin:0; }
.flight-console { display:grid; grid-template-columns:minmax(280px,1.05fr) minmax(240px,.95fr); gap:var(--space-4); align-items:start; }
.flight-pad { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:var(--space-2); padding:var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-md); background:var(--surface-sunken); }
.flight-pad .ui-btn { min-width:0; min-height:42px; padding-inline:var(--space-2); touch-action:none; user-select:none; }
.flight-axis:active:not(:disabled) { border-color:var(--accent-border); background:var(--accent); color:var(--text-on-accent); }
.flight-stop { border-color:var(--warning); color:var(--warning-ink); background:var(--warning-bg); }
.flight-settings { display:grid; gap:var(--space-2); }
.flight-settings > label { display:grid; grid-template-columns:minmax(0,1fr) 110px; align-items:center; gap:var(--space-3); min-height:42px; }
.flight-settings > label span { color:var(--text-secondary); font-size:var(--fs-sm); font-weight:var(--fw-semibold); }
.flight-capability { display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); min-height:48px; padding:var(--space-2) var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card-pop); }
.flight-capability.is-active { border-color:var(--success); box-shadow:3px 0 0 var(--success) inset; background:color-mix(in srgb,var(--success-bg) 42%,var(--surface-card-pop)); }
.flight-capability.has-error { border-color:var(--warning); box-shadow:3px 0 0 var(--warning) inset; }
.flight-capability > span { min-width:0; }
.flight-capability .ui-btn { min-width:7.5rem; flex:0 0 auto; }
.flight-capability b,.flight-capability small { display:block; }
.flight-capability b { color:var(--text-primary); font-size:var(--fs-sm); }
.flight-capability small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); overflow-wrap:anywhere; }
@container runtime-monitor (max-width:720px) { .monitor-connection,.panel-heading,.capture-toolbar { align-items:stretch; flex-direction:column; }.connection-actions,.connection-actions .ui-btn { width:100%; }.capture-steps,.bookmark-list { grid-template-columns:repeat(2,minmax(0,1fr)); }.teleport-controls { grid-template-columns:repeat(3,minmax(0,1fr)); }.teleport-controls .ui-btn { grid-column:1 / -1; }.teleport-result { grid-template-columns:minmax(0,1fr); } }
@container runtime-monitor (max-width:720px) { .flight-console { grid-template-columns:minmax(0,1fr); } }
@container runtime-monitor (max-width:460px) { .capture-steps,.record-card dl,.coordinate-row,.teleport-controls,.bookmark-compose,.bookmark-list { grid-template-columns:minmax(0,1fr); }.teleport-controls .ui-btn { grid-column:auto; }.spatial-bookmarks > header { align-items:stretch; flex-direction:column; }.spatial-bookmarks > header .ui-btn,.bookmark-compose .ui-btn { width:100%; }.flight-settings > label { grid-template-columns:minmax(0,1fr); }.flight-settings > label .ui-input { width:100%; }.flight-capability { align-items:stretch; flex-direction:column; }.flight-capability .ui-btn { width:100%; } }
</style>
