<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import {
  CharaAcquire,
  CharaRelease,
  RuntimePatchPartyMonitorOwned,
  RuntimePatchSelectedItemReadOwned,
  RuntimePatchSelectedItemsDisableOwned,
  RuntimePatchSelectedItemsEnableOwned,
  RuntimePatchSelectedItemsStatusOwned,
} from '../../wailsjs/go/main/App'
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
  partyOptionalMetric,
  runtimeMonitorRoleName,
  runtimeMonitorText,
  selectedCapturePhase,
} from '../runtimePatchMonitorView.js'

const emit = defineEmits(['status'])
const RUNTIME_LEASE_SCOPE = 'runtime-patch-runtime-monitor'
const ITEM_KINDS = Object.freeze(['material', 'keyItem'])

const activeTab = ref('party')
const tabElements = ref([])
const activeOperation = ref(null)
const releasePending = ref(false)
const connected = ref(false)
const processInfo = ref({ pid: 0, moduleBase: 0 })
const partySnapshot = ref(null)
const selectedStatus = ref(null)
const selectedRecords = reactive({ material: null, keyItem: null })
const consumedSelections = reactive({ material: false, keyItem: false })
const liveMessage = ref(t('statusDisconnected'))
const liveTone = ref('info')
const operationGate = createOperationGate()
let disposed = false
let lifecycleEpoch = 0
let connectionOwnerToken = ''

const tabs = computed(() => [
  { id: 'party', label: t('tabParty') },
  { id: 'items', label: t('tabItems') },
])
const operationBusy = computed(() => activeOperation.value !== null)
const interactionLocked = computed(() => operationBusy.value || releasePending.value)
const connectionStateLabel = computed(() => {
  if (releasePending.value) return t('releasing')
  return connected.value ? t('connected') : t('notConnected')
})
const connectionStateClass = computed(() => releasePending.value ? 'is-warn' : connected.value ? 'is-ok' : 'is-info')
const partyRefreshing = computed(() => activeOperation.value?.kind === 'party')
const captureRefreshing = computed(() => activeOperation.value?.kind === 'capture-refresh')
const captureChanging = computed(() => ['capture-enable', 'capture-disable'].includes(activeOperation.value?.kind))

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

function resetMonitorData() {
  partySnapshot.value = null
  selectedStatus.value = null
  selectedRecords.material = null
  selectedRecords.keyItem = null
  consumedSelections.material = false
  consumedSelections.keyItem = false
}

function clearRuntimeState() {
  operationGate.invalidate()
  activeOperation.value = null
  releasePending.value = false
  connected.value = false
  connectionOwnerToken = ''
  processInfo.value = { pid: 0, moduleBase: 0 }
  resetMonitorData()
}

function completeRuntimeRelease(expectedOwnerToken, expectedEpoch, notification) {
  if (
    disposed
    || lifecycleEpoch !== expectedEpoch
    || connectionOwnerToken !== expectedOwnerToken
    || notification?.ownerToken !== expectedOwnerToken
  ) return
  clearRuntimeState()
  announce(t('statusReleaseComplete'), 'ok')
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
      const detail = cleanupError
        ? `${errorMessage(error)}; ${errorMessage(cleanupError)}`
        : errorMessage(error)
      announce(t('statusActionFailed', { error: detail }), 'danger')
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
      CharaRelease,
      notification => completeRuntimeRelease(ownerToken, epoch, notification),
    )
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) {
      releasePending.value = true
      announce(t('statusReleaseFailed', { error: errorMessage(error) }), 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

async function refreshParty() {
  const operationToken = beginOperation('party')
  if (!operationToken) return
  const ownerToken = connectionOwnerToken
  const epoch = lifecycleEpoch
  try {
    if (!ownerToken || !connected.value) throw new Error(t('statusConnect'))
    const snapshot = normalizeRuntimePatchPartySnapshot(
      await RuntimePatchPartyMonitorOwned(ownerToken),
      ownerToken,
      processInfo.value.pid,
    )
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    partySnapshot.value = snapshot
    announce(t('statusPartyRead'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) {
      announce(t('statusPartyFailed', { error: errorMessage(error) }), 'danger')
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
    const status = normalizeRuntimePatchSelectedStatus(
      await RuntimePatchSelectedItemsEnableOwned(ownerToken),
      ownerToken,
      processInfo.value.pid,
    )
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    if (!status.enabled) throw new Error(t('errorCaptureEnableVerification'))
    applySelectedStatus(status)
    announce(t('statusCaptureEnabled'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) {
      announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
    }
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
    const status = normalizeRuntimePatchSelectedStatus(
      await RuntimePatchSelectedItemsDisableOwned(ownerToken),
      ownerToken,
      processInfo.value.pid,
    )
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    if (status.enabled) throw new Error(t('errorCaptureDisableVerification'))
    applySelectedStatus(status)
    selectedRecords.material = null
    selectedRecords.keyItem = null
    consumedSelections.material = false
    consumedSelections.keyItem = false
    announce(t('statusCaptureDisabled'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) {
      announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
    }
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
    const status = normalizeRuntimePatchSelectedStatus(
      await RuntimePatchSelectedItemsStatusOwned(ownerToken),
      ownerToken,
      processInfo.value.pid,
    )
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    applySelectedStatus(status)
    announce(t('statusCaptureRefreshed'), 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) {
      announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
    }
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
    if (!capture?.captured || !Number.isSafeInteger(expectedSelectedAddr) || expectedSelectedAddr <= 0) {
      throw new Error(t('stepSelect'))
    }
    const record = normalizeRuntimePatchSelectedRecord(
      await RuntimePatchSelectedItemReadOwned(ownerToken, { kind, expectedSelectedAddr }),
      kind,
      expectedSelectedAddr,
    )
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    selectedRecords[kind] = Object.freeze({
      ...record,
      hashHex: record.hashHex,
      name: record.name,
      quantity: record.quantity,
      flagsHex: record.flagsHex,
    })
    consumedSelections[kind] = true
    selectedStatus.value = consumeRuntimePatchSelectedCapture(selectedStatus.value, kind)

    let refreshError = null
    try {
      const status = normalizeRuntimePatchSelectedStatus(
        await RuntimePatchSelectedItemsStatusOwned(ownerToken),
        ownerToken,
        processInfo.value.pid,
      )
      if (operationIsCurrent(operationToken, epoch, ownerToken)) applySelectedStatus(status)
    } catch (error) {
      refreshError = error
    }
    if (!operationIsCurrent(operationToken, epoch, ownerToken)) return
    if (refreshError) {
      announce(t('statusReadRefreshFailed', { error: errorMessage(refreshError) }), 'warn')
    } else {
      announce(t('statusItemRead', { name: record.name }), 'ok')
    }
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch, ownerToken)) {
      announce(t('statusActionFailed', { error: errorMessage(error) }), 'danger')
    }
  } finally {
    finishOperation(operationToken)
  }
}

function setTabElement(element, index) {
  if (element) tabElements.value[index] = element
}

function activateTab(tab, index) {
  activeTab.value = tab.id
  tabElements.value[index]?.focus?.()
}

function onTabKeydown(event, index) {
  let nextIndex = index
  if (event.key === 'ArrowRight') nextIndex = (index + 1) % tabs.value.length
  else if (event.key === 'ArrowLeft') nextIndex = (index - 1 + tabs.value.length) % tabs.value.length
  else if (event.key === 'Home') nextIndex = 0
  else if (event.key === 'End') nextIndex = tabs.value.length - 1
  else return
  event.preventDefault()
  activateTab(tabs.value[nextIndex], nextIndex)
}

function formatPosition(position) {
  return `X ${formatRuntimeCoordinate(position.x)} · Y ${formatRuntimeCoordinate(position.y)} · Z ${formatRuntimeCoordinate(position.z)}`
}

function hpProgress(entity) {
  return `${Math.max(0, Math.min(100, (entity.hp / entity.maxHp) * 100))}%`
}

function hpText(entity) {
  return `${formatRuntimeInteger(entity.hp, language.value)} / ${formatRuntimeInteger(entity.maxHp, language.value)}`
}

function roleName(entity) {
  return runtimeMonitorRoleName(entity.role, language.value)
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

function itemKindName(kind) {
  return t(kind)
}

function categoryText(record) {
  return record.category || t('unknownCategory')
}

onBeforeUnmount(() => {
  disposed = true
  lifecycleEpoch += 1
  operationGate.invalidate()
  activeOperation.value = null
  const ownerToken = connectionOwnerToken
  connectionOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease)
})
</script>

<template>
  <section class="runtime-patch-monitor-page ui-page ui-page-stack is-fluid" data-page="runtime-patch-runtime-monitor" :aria-busy="operationBusy">
    <section class="monitor-connection ui-card ui-panel is-compact" :aria-label="`${t('memoryMonitoring')} · ${t('readOnly')}`" :aria-busy="operationBusy">
      <div class="connection-summary">
        <span class="connection-emblem" :class="{ 'is-on': connected }" aria-hidden="true"><i></i></span>
        <div>
          <strong>{{ connectionStateLabel }}</strong>
          <small v-if="connected">PID {{ formatRuntimeInteger(processInfo.pid, language) }} · {{ t('gameVersion') }} 2.0.2</small>
          <small v-else>{{ t('statusConnect') }}</small>
        </div>
        <span class="ui-tag" :class="connectionStateClass">{{ connectionStateLabel }}</span>
      </div>
      <div class="ui-actions connection-actions">
        <button v-if="!connected" type="button" class="ui-btn is-primary" :disabled="interactionLocked" @click="connect">
          {{ activeOperation?.kind === 'connect' ? t('working') : t('connect') }}
        </button>
        <button v-else type="button" class="ui-btn is-ghost" :disabled="operationBusy" @click="disconnect">
          {{ releasePending ? t('retryDisconnect') : t('disconnect') }}
        </button>
      </div>
    </section>

    <div class="monitor-live ui-notice" :class="`is-${liveTone}`" aria-live="polite" aria-atomic="true">
      <span class="live-mark" aria-hidden="true"></span><span>{{ liveMessage }}</span>
    </div>

    <nav class="monitor-tabs ui-tabs" role="tablist" :aria-label="t('pageTitle')">
      <button
        v-for="(tab, index) in tabs"
        :id="`runtime-monitor-tab-${tab.id}`"
        :key="tab.id"
        :ref="element => setTabElement(element, index)"
        type="button"
        class="ui-tab"
        :class="{ 'is-on': activeTab === tab.id }"
        role="tab"
        :aria-selected="activeTab === tab.id"
        :aria-controls="`runtime-monitor-panel-${tab.id}`"
        :tabindex="activeTab === tab.id ? 0 : -1"
        @click="activeTab = tab.id"
        @keydown="onTabKeydown($event, index)"
      >
        <span class="tab-glyph" :class="`is-${tab.id}`" aria-hidden="true"><i></i></span>
        {{ tab.label }}
      </button>
    </nav>

    <section
      v-show="activeTab === 'party'"
      id="runtime-monitor-panel-party"
      class="monitor-panel ui-card ui-panel"
      role="tabpanel"
      aria-labelledby="runtime-monitor-tab-party"
      data-monitor-panel="party"
    >
      <header class="panel-heading">
        <div>
          <h3 class="ui-section-title">{{ t('partyTitle') }}</h3>
          <p class="ui-section-copy">{{ t('partySummary') }}</p>
        </div>
        <button type="button" class="ui-btn is-primary is-sm" :disabled="interactionLocked || !connected" @click="refreshParty">
          {{ partyRefreshing ? t('refreshing') : t('refresh') }}
        </button>
      </header>

      <div v-if="!partySnapshot" class="monitor-empty ui-empty">
        <span class="empty-orbit" aria-hidden="true"><i></i></span>
        <strong>{{ t('partyEmptyTitle') }}</strong>
        <span>{{ t('partyEmptyCopy') }}</span>
      </div>

      <template v-else>
        <div class="verification-ribbon ui-notice is-ok">
          <span class="verification-seal" aria-hidden="true"><i></i></span>
          <div><strong>{{ t('partyReadyTitle') }}</strong><span>{{ t('partyReadyCopy') }}</span></div>
          <dl>
            <div><dt>{{ t('snapshotCount') }}</dt><dd>{{ partySnapshot.snapshotCount }}</dd></div>
            <div><dt>{{ t('gameVersion') }}</dt><dd>{{ partySnapshot.gameVersion }}</dd></div>
            <div><dt>{{ t('processId') }}</dt><dd>{{ partySnapshot.pid }}</dd></div>
          </dl>
        </div>

        <div class="party-grid">
          <article v-for="entity in partySnapshot.entities" :key="entity.role" class="party-card ui-card is-flat" :class="{ 'is-absent': !entity.present }">
            <header class="entity-heading">
              <span class="role-crest" :data-role="entity.role" aria-hidden="true"><i></i></span>
              <div><small>{{ t('verifiedSnapshot') }}</small><h4>{{ roleName(entity) }}</h4></div>
              <span class="ui-tag" :class="entity.present ? 'is-ok' : 'is-info'">{{ entity.present ? t('validData') : t('notInParty') }}</span>
            </header>

            <div v-if="!entity.present" class="empty-party-slot">{{ t('emptySlotCopy') }}</div>

            <section v-if="entity.present" class="hp-block" :aria-label="`${roleName(entity)} ${t('hp')} ${hpText(entity)}`">
              <div><span>{{ t('hp') }}</span><strong>{{ hpText(entity) }}</strong></div>
              <span class="hp-track" aria-hidden="true"><i :style="{ width: hpProgress(entity) }"></i></span>
            </section>

            <dl v-if="entity.present" class="entity-metrics">
              <div :class="{ 'is-unavailable': !partyOptionalMetric(entity, 'sba', language).available }">
                <dt>{{ t('sba') }}</dt><dd>{{ partyOptionalMetric(entity, 'sba', language).text }}</dd>
              </div>
              <div :class="{ 'is-unavailable': !partyOptionalMetric(entity, 'dodge', language).available }">
                <dt>{{ t('dodge') }}</dt><dd>{{ partyOptionalMetric(entity, 'dodge', language).text }}</dd>
              </div>
              <div class="is-wide"><dt>{{ t('position') }}</dt><dd>{{ formatPosition(entity.position) }}</dd></div>
              <div class="is-wide" :class="{ 'is-unavailable': !entity.capabilities.directPosition }">
                <dt>{{ t('directPosition') }}</dt>
                <dd>{{ entity.capabilities.directPosition ? formatPosition(entity.directPosition) : t('fieldUnavailable') }}</dd>
              </div>
            </dl>

            <details v-if="entity.present" class="entity-technical ui-disclosure">
              <summary>{{ t('entityAddress') }}</summary>
              <code>{{ formatRuntimeAddress(entity.address) }}</code>
            </details>
          </article>
        </div>
      </template>
    </section>

    <section
      v-show="activeTab === 'items'"
      id="runtime-monitor-panel-items"
      class="monitor-panel ui-card ui-panel"
      role="tabpanel"
      aria-labelledby="runtime-monitor-tab-items"
      data-monitor-panel="selected-items"
    >
      <header class="panel-heading">
        <div>
          <h3 class="ui-section-title">{{ t('selectedTitle') }}</h3>
          <p class="ui-section-copy">{{ t('selectedSummary') }}</p>
        </div>
        <span class="ui-tag is-ok">{{ t('readOnlyChip') }}</span>
      </header>

      <section class="read-only-banner ui-notice is-ok">
        <span class="shield-mark" aria-hidden="true"><i></i></span>
        <div><strong>{{ t('readOnlyBanner') }}</strong><p>{{ t('neverWritesSave') }}</p><small>{{ t('hookTechnical') }}</small></div>
      </section>

      <ol class="capture-steps">
        <li><span>1</span>{{ t('stepConnect') }}</li>
        <li><span>2</span>{{ t('stepEnable') }}</li>
        <li><span>3</span>{{ t('stepSelect') }}</li>
        <li><span>4</span>{{ t('stepRead') }}</li>
      </ol>

      <div class="capture-toolbar ui-toolbar">
        <div>
          <strong>{{ selectedStatus?.enabled ? t('captureReady') : t('captureDisabled') }}</strong>
          <small>{{ t('readOnlyBanner') }}</small>
        </div>
        <div class="ui-actions">
          <button
            v-if="!selectedStatus?.enabled"
            type="button"
            class="ui-btn is-primary is-sm"
            :disabled="interactionLocked || !connected"
            @click="enableCapture"
          >{{ captureChanging ? t('working') : t('enableCapture') }}</button>
          <button
            v-else
            type="button"
            class="ui-btn is-ghost is-sm"
            :disabled="interactionLocked"
            @click="disableCapture"
          >{{ captureChanging ? t('working') : t('disableCapture') }}</button>
          <button
            type="button"
            class="ui-btn is-sm"
            :disabled="interactionLocked || !connected || !selectedStatus?.enabled"
            @click="refreshCaptureStatus"
          >{{ captureRefreshing ? t('refreshing') : t('refreshCapture') }}</button>
        </div>
      </div>

      <div v-if="!connected" class="monitor-empty ui-empty">
        <span class="empty-orbit" aria-hidden="true"><i></i></span>
        <strong>{{ t('notConnected') }}</strong><span>{{ t('statusConnect') }}</span>
      </div>

      <div v-else class="capture-grid">
        <article v-for="kind in ITEM_KINDS" :key="kind" class="capture-card ui-card is-flat" :class="`phase-${capturePhase(kind)}`">
          <header class="capture-card-heading">
            <span class="capture-glyph" :class="`is-${kind}`" aria-hidden="true"><i></i></span>
            <div><small>{{ t('readOnly') }}</small><h4>{{ itemKindName(kind) }}</h4></div>
            <span class="ui-tag" :class="capturePhaseClass(kind)">{{ capturePhaseText(kind) }}</span>
          </header>

          <div class="capture-address">
            <span>{{ t('selectedAddress') }}</span>
            <code v-if="selectedStatus?.[kind]?.captured">{{ formatRuntimeAddress(selectedStatus[kind].selectedAddr) }}</code>
            <strong v-else>{{ capturePhase(kind) === 'reselect' ? t('selectAgain') : capturePhaseText(kind) }}</strong>
          </div>

          <button
            type="button"
            class="ui-btn is-primary capture-read"
            :disabled="interactionLocked || capturePhase(kind) !== 'ready'"
            @click="readSelectedItem(kind)"
          >{{ activeOperation?.kind === 'item-read' ? t('reading') : t('readOnce') }}</button>

          <section v-if="selectedRecords[kind]" class="record-card" :aria-label="t('lastRead')">
            <div class="record-title"><span>{{ t('lastRead') }}</span><strong>{{ selectedRecords[kind].name }}</strong></div>
            <dl>
              <div><dt>{{ t('catalogName') }}</dt><dd>{{ selectedRecords[kind].name }}</dd></div>
              <div><dt>{{ t('category') }}</dt><dd>{{ categoryText(selectedRecords[kind]) }}</dd></div>
              <div><dt>{{ t('hash') }}</dt><dd><code>0x{{ selectedRecords[kind].hashHex }}</code></dd></div>
              <div><dt>{{ t('quantity') }}</dt><dd>{{ formatRuntimeInteger(selectedRecords[kind].quantity, language) }}</dd></div>
              <div><dt>{{ t('flags') }}</dt><dd><code>0x{{ selectedRecords[kind].flagsHex }}</code></dd></div>
              <div><dt>{{ t('selectedAddress') }}</dt><dd><code>{{ formatRuntimeAddress(selectedRecords[kind].selectedAddr) }}</code></dd></div>
            </dl>
          </section>
          <div v-else class="record-empty">{{ t('noRecord') }}</div>

          <details v-if="selectedStatus?.[kind]" class="capture-technical ui-disclosure">
            <summary>{{ t('hookTechnical') }}</summary>
            <dl>
              <div><dt>{{ t('captureAddress') }}</dt><dd><code>{{ formatRuntimeAddress(selectedStatus[kind].address) }}</code></dd></div>
              <div><dt>{{ t('hookRva') }}</dt><dd><code>{{ formatRuntimeAddress(selectedStatus[kind].rva) }}</code></dd></div>
            </dl>
          </details>
        </article>
      </div>
    </section>
  </section>
</template>

<style scoped>
.runtime-patch-monitor-page {
  width:100%;
  max-width:none;
  min-width:0;
  padding-bottom:var(--space-8);
  container-name:runtime-monitor;
  container-type:inline-size;
}

.monitor-connection,
.panel-heading,
.connection-summary,
.capture-card-heading,
.entity-heading,
.record-title {
  display:flex;
  min-width:0;
  align-items:center;
  justify-content:space-between;
  gap:var(--space-4);
}

.monitor-connection { flex-direction:row; }
.connection-summary { flex:1 1 auto; justify-content:flex-start; }
.connection-summary > div { min-width:0; }
.connection-summary strong,.connection-summary small { display:block; }
.connection-summary strong { color:var(--text-primary); }
.connection-summary small { margin-top:2px; color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); }
.connection-summary > .ui-tag { margin-left:auto; flex:none; min-width:4.5em; justify-content:center; white-space:nowrap; }
.connection-actions { flex:0 0 auto; }
.connection-emblem { position:relative; width:36px; height:36px; flex:0 0 36px; border:1px solid var(--border-strong); border-radius:50%; background:var(--surface-sunken); }
.connection-emblem::before,.connection-emblem::after { content:""; position:absolute; top:10px; width:7px; height:12px; border:2px solid var(--text-muted); }
.connection-emblem::before { left:7px; border-right:0; border-radius:7px 0 0 7px; }
.connection-emblem::after { right:7px; border-left:0; border-radius:0 7px 7px 0; }
.connection-emblem i { position:absolute; top:17px; left:13px; width:10px; height:2px; background:var(--text-muted); }
.connection-emblem.is-on { border-color:var(--success); background:var(--success-bg); }
.connection-emblem.is-on::before,.connection-emblem.is-on::after { border-color:var(--success-ink); }
.connection-emblem.is-on i { background:var(--success-ink); }

.monitor-live { min-height:42px; display:flex; align-items:center; gap:var(--space-3); }
.live-mark { width:7px; height:7px; flex:0 0 7px; border-radius:50%; background:currentColor; }
.monitor-tabs { padding-inline:var(--space-2); background:color-mix(in srgb,var(--surface-card-pop) 72%,transparent); }
.monitor-tabs .ui-tab { display:flex; align-items:center; gap:var(--space-2); }
.tab-glyph { position:relative; width:18px; height:18px; flex:0 0 18px; }
.tab-glyph::before,.tab-glyph::after,.tab-glyph i { content:""; position:absolute; border:1px solid currentColor; }
.tab-glyph.is-party::before { inset:2px 6px 8px; border-radius:50%; }
.tab-glyph.is-party::after { inset:10px 3px 1px; border-radius:8px 8px 3px 3px; }
.tab-glyph.is-party i { inset:6px 1px 4px 12px; border-left:0; border-radius:0 5px 5px 0; }
.tab-glyph.is-items::before { inset:2px; transform:rotate(45deg); border-radius:4px; }
.tab-glyph.is-items::after { inset:6px; transform:rotate(45deg); border-color:var(--accent-border); }
.tab-glyph.is-items i { display:none; }

.monitor-panel { min-width:0; border-color:var(--border-default); background:color-mix(in srgb,var(--surface-card) 93%,transparent); }
.panel-heading { align-items:flex-start; }
.panel-heading > div { min-width:0; }
.panel-heading .ui-section-copy { max-width:72ch; margin-top:var(--space-2); }
.monitor-empty { min-height:210px; display:grid; place-content:center; justify-items:center; gap:var(--space-2); }
.monitor-empty strong { color:var(--text-primary); font-size:var(--fs-md); }
.empty-orbit { position:relative; width:42px; height:42px; margin-bottom:var(--space-2); border:1px solid var(--border-strong); border-radius:50%; }
.empty-orbit::before { content:""; position:absolute; inset:7px -5px; border:1px solid var(--accent-border); border-radius:50%; transform:rotate(-24deg); }
.empty-orbit i { position:absolute; top:17px; left:17px; width:7px; height:7px; border-radius:50%; background:var(--accent); }

.verification-ribbon { display:flex; align-items:center; gap:var(--space-4); }
.verification-ribbon > div { min-width:0; flex:1 1 auto; }
.verification-ribbon strong,.verification-ribbon span { display:block; }
.verification-ribbon span { margin-top:2px; font-size:var(--fs-xs); }
.verification-ribbon dl { display:flex; flex:0 0 auto; gap:var(--space-5); margin:0; }
.verification-ribbon dt { color:inherit; font-size:var(--fs-xs); opacity:.72; }
.verification-ribbon dd { margin:2px 0 0; font-family:var(--font-data); font-weight:var(--fw-bold); }
.verification-seal { position:relative; width:32px; height:32px; flex:0 0 32px; border:1px solid currentColor; border-radius:50%; }
.verification-seal::before,.verification-seal::after { content:""; position:absolute; background:currentColor; transform-origin:left center; }
.verification-seal::before { width:7px; height:2px; left:8px; top:16px; transform:rotate(45deg); }
.verification-seal::after { width:13px; height:2px; left:13px; top:20px; transform:rotate(-48deg); }

.party-grid { display:grid; min-width:0; grid-template-columns:repeat(auto-fit,minmax(min(100%,260px),1fr)); gap:var(--space-4); align-items:stretch; }
.party-card { padding:var(--space-4); border-color:var(--border-default); background:var(--surface-card-pop); }
.party-card.is-absent { background:var(--surface-sunken); }
.empty-party-slot { margin-top:var(--space-4); padding:var(--space-5); border:1px dashed var(--border-default); border-radius:var(--radius-sm); color:var(--text-muted); font-size:var(--fs-sm); text-align:center; }
.entity-heading { justify-content:flex-start; }
.entity-heading > div { min-width:0; flex:1 1 auto; }
.entity-heading small { display:block; color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.entity-heading h4 { margin:2px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-md); }
.role-crest { position:relative; width:38px; height:38px; flex:0 0 38px; border:1px solid var(--accent-border); border-radius:50% 50% 44% 44%; background:var(--accent-soft); }
.role-crest::before { content:""; position:absolute; inset:8px 10px 13px; border:2px solid var(--accent-hover); border-radius:50%; }
.role-crest::after { content:""; position:absolute; inset:23px 7px 5px; border:2px solid var(--accent-hover); border-bottom:0; border-radius:12px 12px 0 0; }
.role-crest[data-role="companion"] { border-radius:48% 52% 40% 46%; transform:rotate(-3deg); }
.role-crest[data-role="companion"] i::before,.role-crest[data-role="companion"] i::after { content:""; position:absolute; top:5px; width:8px; height:8px; border-top:2px solid var(--accent-hover); }
.role-crest[data-role="companion"] i::before { left:4px; transform:rotate(-38deg); }
.role-crest[data-role="companion"] i::after { right:4px; transform:rotate(38deg); }

.hp-block { margin-top:var(--space-4); padding:var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.hp-block > div { display:flex; justify-content:space-between; gap:var(--space-3); }
.hp-block span { color:var(--text-muted); font-size:var(--fs-xs); }
.hp-block strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-sm); font-variant-numeric:tabular-nums; }
.hp-track { display:block; height:6px; margin-top:var(--space-2); overflow:hidden; border-radius:var(--radius-pill); background:var(--border-soft); }
.hp-track i { display:block; height:100%; border-radius:inherit; background:linear-gradient(90deg,var(--danger),var(--success)); transition:width var(--dur-base) var(--ease-out); }
.entity-metrics { display:grid; min-width:0; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); margin:var(--space-3) 0 0; }
.entity-metrics > div { min-width:0; padding:var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card); }
.entity-metrics > div.is-wide { grid-column:1 / -1; }
.entity-metrics dt { color:var(--text-muted); font-size:var(--fs-xs); }
.entity-metrics dd { margin:3px 0 0; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-sm); font-variant-numeric:tabular-nums; overflow-wrap:anywhere; }
.entity-metrics .is-unavailable { background:color-mix(in srgb,var(--surface-sunken) 75%,transparent); }
.entity-metrics .is-unavailable dd { color:var(--text-muted); font-family:var(--font-body); font-size:var(--fs-xs); font-style:italic; }
.entity-technical { margin-top:var(--space-3); box-shadow:none; }
.entity-technical > summary { min-height:var(--control-height-sm); padding-block:var(--space-2); color:var(--text-muted); font-size:var(--fs-xs); }
.entity-technical code { display:block; color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-xs); overflow-wrap:anywhere; }

.read-only-banner { display:flex; align-items:flex-start; gap:var(--space-4); border-left-width:4px; }
.read-only-banner > div { min-width:0; }
.read-only-banner strong { display:block; font-family:var(--font-display); font-size:var(--fs-md); }
.read-only-banner p { margin:var(--space-1) 0 0; }
.read-only-banner small { display:block; margin-top:var(--space-2); opacity:.78; }
.shield-mark { position:relative; width:34px; height:38px; flex:0 0 34px; border:2px solid currentColor; border-radius:9px 9px 14px 14px; clip-path:polygon(50% 0,100% 16%,91% 76%,50% 100%,9% 76%,0 16%); }
.shield-mark::before,.shield-mark::after { content:""; position:absolute; background:currentColor; }
.shield-mark::before { width:7px; height:2px; left:7px; top:18px; transform:rotate(45deg); }
.shield-mark::after { width:14px; height:2px; left:12px; top:21px; transform:rotate(-49deg); }
.capture-steps { display:grid; min-width:0; grid-template-columns:repeat(4,minmax(0,1fr)); gap:var(--space-2); margin:0; padding:0; list-style:none; counter-reset:none; }
.capture-steps li { min-width:0; display:flex; align-items:center; gap:var(--space-2); padding:var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card-pop); color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.capture-steps li > span { width:23px; height:23px; flex:0 0 23px; display:grid; place-items:center; border:1px solid var(--accent-border); border-radius:50%; color:var(--accent-hover); font-family:var(--font-data); font-weight:var(--fw-bold); }
.capture-toolbar { align-items:center; justify-content:space-between; }
.capture-toolbar > div:first-child { min-width:0; }
.capture-toolbar strong,.capture-toolbar small { display:block; }
.capture-toolbar strong { color:var(--text-primary); }
.capture-toolbar small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.capture-grid { display:grid; min-width:0; grid-template-columns:minmax(0,1fr); gap:var(--space-4); }
.capture-card { min-width:0; padding:var(--space-4); border-color:var(--border-default); background:var(--surface-card-pop); }
.capture-card.phase-ready { border-color:var(--success); box-shadow:3px 0 0 var(--success) inset; }
.capture-card.phase-reselect { border-color:var(--warning); box-shadow:3px 0 0 var(--warning) inset; }
.capture-card-heading { justify-content:flex-start; }
.capture-card-heading > div { min-width:0; flex:1 1 auto; }
.capture-card-heading small { display:block; color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.capture-card-heading h4 { margin:2px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-md); }
.capture-glyph { position:relative; width:38px; height:38px; flex:0 0 38px; border:1px solid var(--accent-border); border-radius:var(--radius-sm); background:var(--accent-soft); }
.capture-glyph::before,.capture-glyph::after { content:""; position:absolute; }
.capture-glyph.is-material::before { inset:7px; border:2px solid var(--accent-hover); transform:rotate(45deg); }
.capture-glyph.is-material::after { inset:13px; border:1px solid var(--accent-hover); transform:rotate(45deg); }
.capture-glyph.is-keyItem::before { width:17px; height:17px; left:6px; top:7px; border:2px solid var(--accent-hover); border-radius:50%; }
.capture-glyph.is-keyItem::after { width:15px; height:3px; right:4px; bottom:9px; background:var(--accent-hover); transform:rotate(-43deg); box-shadow:5px 3px 0 -1px var(--accent-hover); }
.capture-address { min-height:68px; display:flex; min-width:0; flex-direction:column; justify-content:center; gap:var(--space-1); margin-top:var(--space-4); padding:var(--space-3) var(--space-4); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.capture-address span { color:var(--text-muted); font-size:var(--fs-xs); }
.capture-address code { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-md); overflow-wrap:anywhere; }
.capture-address strong { color:var(--text-secondary); font-size:var(--fs-sm); }
.capture-read { width:100%; margin-top:var(--space-3); }
.record-card { margin-top:var(--space-4); padding:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-md); background:color-mix(in srgb,var(--success-bg) 24%,var(--surface-card)); }
.record-title { align-items:flex-start; flex-direction:column; gap:2px; }
.record-title span { color:var(--success-ink); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.05em; }
.record-title strong { color:var(--text-primary); font-size:var(--fs-md); overflow-wrap:anywhere; }
.record-card dl { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); margin:var(--space-3) 0 0; }
.record-card dl > div { min-width:0; padding:var(--space-2) var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card-pop); }
.record-card dt { color:var(--text-muted); font-size:var(--fs-xs); }
.record-card dd { margin:2px 0 0; color:var(--text-primary); font-size:var(--fs-sm); font-weight:var(--fw-semibold); overflow-wrap:anywhere; }
.record-card code { font-family:var(--font-data); }
.record-empty { margin-top:var(--space-4); padding:var(--space-5); border:1px dashed var(--border-default); border-radius:var(--radius-sm); color:var(--text-muted); font-size:var(--fs-sm); text-align:center; }
.capture-technical { margin-top:var(--space-3); box-shadow:none; }
.capture-technical > summary { color:var(--text-muted); font-size:var(--fs-xs); }
.capture-technical dl { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); }
.capture-technical dt { color:var(--text-muted); font-size:var(--fs-xs); }
.capture-technical dd { margin:2px 0 0; color:var(--text-secondary); font-size:var(--fs-xs); overflow-wrap:anywhere; }

@container runtime-monitor (min-width:760px) {
  .capture-grid { grid-template-columns:repeat(2,minmax(0,1fr)); }
}

@container runtime-monitor (min-width:1500px) {
  .party-grid { grid-template-columns:repeat(5,minmax(0,1fr)); }
  .capture-card { padding:var(--space-5); }
  .record-card dl { grid-template-columns:repeat(3,minmax(0,1fr)); }
}

@container runtime-monitor (max-width:900px) {
  .verification-ribbon { align-items:flex-start; flex-wrap:wrap; }
  .verification-ribbon dl { width:100%; justify-content:space-between; padding-left:48px; }
  .capture-steps { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .capture-toolbar { align-items:stretch; flex-direction:column; }
}

@container runtime-monitor (max-width:620px) {
  .monitor-connection,.panel-heading { align-items:stretch; flex-direction:column; }
  .connection-actions,.connection-actions .ui-btn,.panel-heading .ui-btn { width:100%; }
  .connection-summary > .ui-tag { margin-left:auto; }
  .verification-ribbon dl { padding-left:0; }
  .capture-toolbar .ui-actions { width:100%; }
  .capture-toolbar .ui-btn { flex:1 1 180px; }
}

@container runtime-monitor (max-width:480px) {
  .runtime-patch-monitor-page { gap:var(--space-3); padding-bottom:var(--space-5); }
  .monitor-panel { padding:var(--space-4); }
  .monitor-tabs { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); overflow:visible; padding-inline:0; }
  .monitor-tabs .ui-tab { min-width:0; justify-content:center; padding-inline:var(--space-2); white-space:normal; }
  .capture-steps { grid-template-columns:minmax(0,1fr); }
  .entity-metrics,.record-card dl,.capture-technical dl { grid-template-columns:minmax(0,1fr); }
  .entity-metrics > div.is-wide { grid-column:1; }
  .verification-ribbon dl { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:var(--space-2); }
  .verification-ribbon dd { overflow-wrap:anywhere; }
  .capture-card-heading { flex-wrap:wrap; }
  .capture-card-heading > .ui-tag { margin-left:54px; }
}

@media (prefers-reduced-motion:reduce) {
  .hp-track i,.ui-tab,.ui-btn { transition:none; }
}
</style>
