<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  CharaAcquire,
  CharaRelease,
  CT084GetCatalog,
  CT084GetStatusesOwned,
  CT084ReleaseOwned,
  CT084SetEnabledOwned,
} from '../../wailsjs/go/main/App'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'
import { language } from '../i18n.js'
import {
  buildCT084Groups,
  buildCT084StatusIndex,
  findActiveCT084Conflict,
  replaceCT084FeatureIDs,
  validateCT084StatusSet,
} from '../ct084FeatureView.js'
import { createCT084OperationGate } from '../ct084OperationGate.js'
import {
  translateCT084FeatureName,
  translateCT084GroupName,
  translateCT084Text,
} from '../ct084Translations.js'

const props = defineProps({
  mode: {
    type: String,
    required: true,
    validator: value => ['combat', 'characters', 'quest'].includes(value),
  },
})
const emit = defineEmits(['status'])

const RUNTIME_LEASE_SCOPE = 'ct084-features'
const OFFLINE_CONFIRMATION_KEY = 'gbfr.ct084.offline-only-confirmed'
const EMPTY_STATUS = Object.freeze({ enabled: false, available: false, rvas: [], currentBytes: [], error: '' })

const catalog = ref([])
const statuses = ref([])
const searchQuery = ref('')
const activeGroupKey = ref('')
const catalogLoading = ref(true)
const activeOperation = ref(null)
const releasePending = ref(false)
const connected = ref(false)
const processInfo = ref({ pid: 0, moduleBase: 0 })
const liveMessage = ref(tr('正在读取 CT 0.8.4 功能目录…'))
const liveTone = ref('info')
const pendingConfirmationFeature = ref(null)
const confirmationCancelButton = ref(null)
const confirmationButton = ref(null)
const offlineConfirmationGranted = ref(false)
let disposed = false
let lifecycleEpoch = 0
let connectionOwnerToken = ''
let confirmationReturnTarget = null
const operationGate = createCT084OperationGate((operation) => {
  activeOperation.value = operation
})

function tr(value) {
  return translateCT084Text(value, language.value)
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
const statusIndex = computed(() => buildCT084StatusIndex(statuses.value))
const groups = computed(() => buildCT084Groups(catalog.value, props.mode, searchQuery.value, {
  featureLabel: feature => translateCT084FeatureName(feature, language.value),
  groupLabel: group => translateCT084GroupName(group, language.value),
}))
const currentGroup = computed(() => groups.value.find(group => group.key === activeGroupKey.value) || groups.value[0] || null)
const visibleFeatureCount = computed(() => groups.value.reduce((total, group) => total + group.features.length, 0))
const activeFeatureCount = computed(() => statuses.value.filter(status => status.enabled).length)
const operationBusy = computed(() => activeOperation.value !== null)
const interactionLocked = computed(() => operationBusy.value || releasePending.value)
const connectionLoading = computed(() => ['connect', 'disconnect'].includes(activeOperation.value?.kind))
const statusLoading = computed(() => activeOperation.value?.kind === 'refresh')
const busyFeatureID = computed(() => activeOperation.value?.kind === 'feature' ? activeOperation.value.featureID : '')

watch(groups, (nextGroups) => {
  if (!nextGroups.some(group => group.key === activeGroupKey.value)) activeGroupKey.value = nextGroups[0]?.key || ''
}, { immediate: true })
watch(pendingConfirmationFeature, async (feature) => {
  if (!feature) return
  await nextTick()
  confirmationButton.value?.focus()
})

function normalizeCatalog(value) {
  if (!Array.isArray(value)) throw new Error('CT 0.8.4 功能目录格式无效')
  return value.map((feature) => ({
    ...feature,
    groupPath: Array.isArray(feature?.groupPath) ? feature.groupPath : [],
    conflicts: Array.isArray(feature?.conflicts) ? feature.conflicts : [],
    sites: Array.isArray(feature?.sites) ? feature.sites : [],
  }))
}

function errorMessage(error) {
  const message = error instanceof Error ? error.message : String(error || '未知错误')
  return tr(replaceCT084FeatureIDs(message.replace(/^Error:\s*/i, ''), catalog.value))
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
    const nextCatalog = normalizeCatalog(await CT084GetCatalog())
    if (disposed || epoch !== lifecycleEpoch) return
    catalog.value = nextCatalog
    if (notify) announce(`已读取 ${catalog.value.length} 项 CT 0.8.4 安全补丁`, 'ok')
    else liveMessage.value = tr('功能目录已就绪；连接游戏后可读取实时状态。')
  } catch (error) {
    if (!disposed && epoch === lifecycleEpoch) announce(`读取 CT 0.8.4 功能目录失败：${errorMessage(error)}`, 'danger')
  } finally {
    if (!disposed) catalogLoading.value = false
  }
}

async function releaseCT084PageOwner(ownerToken) {
  await CT084ReleaseOwned(ownerToken)
  await CharaRelease(ownerToken)
}

function clearConnectionState() {
  operationGate.reset()
  releasePending.value = false
  connected.value = false
  connectionOwnerToken = ''
  processInfo.value = { pid: 0, moduleBase: 0 }
  statuses.value = []
}

function completeRuntimeRelease(expectedOwnerToken, expectedEpoch, notification) {
  if (
    disposed
    || lifecycleEpoch !== expectedEpoch
    || connectionOwnerToken !== expectedOwnerToken
    || notification?.ownerToken !== expectedOwnerToken
  ) return
  clearConnectionState()
  announce('全部 CT 0.8.4 补丁已恢复，并已断开游戏进程', 'ok')
}

async function connect() {
  if (connected.value || releasePending.value) return
  const operationToken = beginOperation('connect')
  if (!operationToken) return
  const epoch = ++lifecycleEpoch
  let acquiredOwnerToken = ''
  try {
    if (!catalog.value.length) catalog.value = normalizeCatalog(await CT084GetCatalog())
    if (!operationIsCurrent(operationToken, epoch)) return
    const info = await CharaAcquire(nextRuntimeAcquireRequestID())
    acquiredOwnerToken = String(info?.ownerToken || '')
    if (!acquiredOwnerToken) throw new Error('后端未返回 CT 0.8.4 连接所有权令牌')
    if (!operationIsCurrent(operationToken, epoch)) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, releaseCT084PageOwner)
      return
    }
    const verifiedStatuses = await fetchVerifiedStatuses(acquiredOwnerToken)
    if (!operationIsCurrent(operationToken, epoch)) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, releaseCT084PageOwner)
      return
    }
    connectionOwnerToken = acquiredOwnerToken
    connected.value = true
    processInfo.value = { pid: Number(info?.pid || 0), moduleBase: Number(info?.moduleBase || 0) }
    applyStatuses(verifiedStatuses)
    announce(`已连接游戏进程 PID ${processInfo.value.pid}`, 'ok')
  } catch (error) {
    let cleanupError = null
    if (acquiredOwnerToken) {
      try {
        await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, releaseCT084PageOwner)
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
      releaseCT084PageOwner,
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
  return validateCT084StatusSet(catalog.value, await CT084GetStatusesOwned(ownerToken))
}

async function refreshStatuses() {
  const ownerToken = connectionOwnerToken
  if (!ownerToken || releasePending.value) return
  const operationToken = beginOperation('refresh')
  if (!operationToken) return
  const epoch = lifecycleEpoch
  try {
    const verifiedStatuses = await fetchVerifiedStatuses(ownerToken)
    if (!operationIsCurrent(operationToken, epoch) || ownerToken !== connectionOwnerToken) return
    applyStatuses(verifiedStatuses)
    announce('CT 0.8.4 补丁状态已回读', 'ok')
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
    if (!ownerToken) throw new Error('当前页面不再持有 CT 0.8.4 连接所有权')
    await CT084SetEnabledOwned(ownerToken, feature.id, enabled)
    const verifiedStatuses = await fetchVerifiedStatuses(ownerToken)
    if (!operationIsCurrent(operationToken, epoch) || ownerToken !== connectionOwnerToken) return
    const verifiedStatus = verifiedStatuses.find(status => status.id === feature.id)
    if (!verifiedStatus || verifiedStatus.enabled !== enabled) throw new Error(`${feature.name} 写后回读状态不一致`)
    applyStatuses(verifiedStatuses)
    announce(`${displayFeatureName(feature)}已${enabled ? '开启' : '恢复默认'}`, 'ok')
  } catch (error) {
    if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) {
      try {
        const recoveredStatuses = await fetchVerifiedStatuses(ownerToken)
        if (operationIsCurrent(operationToken, epoch) && ownerToken === connectionOwnerToken) applyStatuses(recoveredStatuses)
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
  await setFeatureEnabled(feature, true)
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
  activeGroupKey.value = key
}

function statusFor(feature) {
  return statusIndex.value.get(feature?.id) || EMPTY_STATUS
}

function ownsFeature(feature) {
  const status = statusFor(feature)
  return status.enabled || status.rvas.length > 0
}

function activeConflictFor(feature) {
  return findActiveCT084Conflict(feature, statusIndex.value, catalog.value)
}

function displayFeatureName(feature) {
  return translateCT084FeatureName(feature, language.value)
}

function displayGroupName(group) {
  return translateCT084GroupName(group, language.value)
}

function displayFeatureError(feature) {
  if (activeConflictFor(feature)) return ''
  return tr(replaceCT084FeatureIDs(statusFor(feature).error, catalog.value))
}

function featureDisabled(feature) {
  const status = statusFor(feature)
  if (!connected.value || interactionLocked.value) return true
  if (ownsFeature(feature)) return false
  return !status.available || !!activeConflictFor(feature)
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
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, releaseCT084PageOwner)
})
</script>

<template>
  <section class="ct084-page ui-page is-wide ui-page-stack" :data-mode="mode">
    <section class="ct-connection ui-card ui-panel is-compact">
      <div class="ct-connection-main">
        <span class="connection-emblem" aria-hidden="true"><i></i></span>
        <div class="connection-copy">
          <strong>{{ tr(releasePending ? '正在安全恢复并断开' : connected ? '游戏进程已连接' : '连接游戏后读取实时状态') }}</strong>
          <span v-if="connected">PID {{ processInfo.pid }} · {{ tr('已开启') }} {{ activeFeatureCount }} {{ tr('项') }}</span>
          <span v-else>{{ modeCopy.summary }}</span>
        </div>
        <span class="ui-tag" :class="releasePending ? 'is-warn' : connected ? 'is-ok' : 'is-info'">{{ tr(releasePending ? '等待恢复' : connected ? '已验证连接' : '未连接') }}</span>
      </div>
      <div class="ct-connection-actions ui-actions">
        <button v-if="connected" type="button" class="ui-btn is-ghost is-sm" :disabled="interactionLocked" @click="refreshStatuses">
          {{ tr(statusLoading ? '回读中…' : '刷新状态') }}
        </button>
        <button type="button" class="ui-btn is-sm" :class="connected ? 'is-danger' : 'is-primary'" :disabled="operationBusy" @click="connected ? disconnect() : connect()">
          {{ tr(connectionLoading ? '处理中…' : releasePending ? '重试安全恢复' : connected ? '恢复全部并断开' : '连接游戏进程') }}
        </button>
      </div>
    </section>

    <div class="ct-live-region ui-notice" :class="`is-${liveTone}`" aria-live="polite" aria-atomic="true">
      <span class="live-mark" aria-hidden="true"></span>
      <span>{{ liveMessage }}</span>
    </div>

    <section class="ct-browser ui-card ui-panel">
      <header class="ct-browser-head">
        <div>
          <h2 class="ui-section-title">{{ modeCopy.label }} {{ tr('目录') }} <small>{{ visibleFeatureCount }} {{ tr('项') }}</small></h2>
          <p class="ui-section-copy">{{ tr('目录来自本地 CT 0.8.4 安全重写；技术签名默认收起。') }}</p>
        </div>
        <label class="ct-search ui-field">
          <span class="ui-field-label">{{ tr('搜索名称、角色或分组') }}</span>
          <span class="search-field">
            <span class="search-glyph" aria-hidden="true"></span>
            <input v-model.trim="searchQuery" class="ui-input" type="search" autocomplete="off" :placeholder="tr('输入关键词筛选')">
          </span>
        </label>
      </header>

      <div v-if="catalogLoading" class="ct-empty ui-empty">{{ tr('正在读取功能目录…') }}</div>
      <div v-else-if="!groups.length" class="ct-empty ui-empty">
        <strong>{{ tr('没有匹配的功能') }}</strong>
        <span>{{ tr('换一个角色名、功能名或分组关键词。') }}</span>
      </div>
      <div v-else class="ct-feature-workspace">
        <aside class="ct-group-pane">
          <label class="ct-group-select ui-field">
            <span class="ui-field-label">{{ tr('当前分组') }}</span>
            <select class="ui-select" :value="currentGroup?.key" @change="selectGroup($event.target.value)">
              <option v-for="group in groups" :key="group.key" :value="group.key">{{ group.label }} ({{ group.features.length }})</option>
            </select>
          </label>
          <nav class="ct-group-disclosure" :aria-label="`${modeCopy.label} ${tr('分组')}`">
            <button
              v-for="group in groups"
              :key="group.key"
              type="button"
              class="ct-group-button"
              :class="{ 'is-on': currentGroup?.key === group.key }"
              :aria-expanded="currentGroup?.key === group.key"
              :aria-controls="`ct-group-${mode}-${group.key}`"
              @click="selectGroup(group.key)"
            >
              <span>{{ group.label }}</span>
              <b>{{ group.features.length }}</b>
            </button>
          </nav>
        </aside>

        <section v-if="currentGroup" :id="`ct-group-${mode}-${currentGroup.key}`" class="ct-feature-column" :aria-label="`${currentGroup.label} ${tr('功能')}`">
          <header class="ct-group-heading">
            <div>
              <span>{{ modeCopy.label }}</span>
              <h3>{{ currentGroup.label }}</h3>
            </div>
            <small>{{ currentGroup.features.length }} {{ tr('项已验证补丁') }}</small>
          </header>

          <div class="ct-feature-list">
            <article
              v-for="feature in currentGroup.features"
              :key="feature.id"
              class="ct-feature-card ui-card"
              :class="{ 'is-on': statusFor(feature).enabled, 'needs-recovery': ownsFeature(feature) && !statusFor(feature).enabled }"
              :aria-busy="busyFeatureID === feature.id"
            >
              <div class="ct-feature-summary">
                <div class="feature-title-block">
                  <div class="feature-kicker">
                    <span>{{ displayGroupName(feature.character || feature.group) }}</span>
                    <span>CT {{ feature.ctId }}</span>
                  </div>
                  <h4>{{ displayFeatureName(feature) }}</h4>
                </div>
                <span class="ui-tag" :class="statusFor(feature).enabled ? 'is-ok' : ownsFeature(feature) ? 'is-warn' : activeConflictFor(feature) ? 'is-danger' : 'is-info'">
                  {{ featureStateLabel(feature) }}
                </span>
              </div>

              <p v-if="activeConflictFor(feature)" class="feature-conflict ui-notice is-warn">
                {{ tr('与「') }}{{ displayFeatureName(activeConflictFor(feature)) }}{{ tr('」互斥；先恢复该功能后才能启用。') }}
              </p>
              <p v-else-if="displayFeatureError(feature)" class="feature-error ui-notice is-danger">{{ displayFeatureError(feature) }}</p>

              <div class="ct-feature-actions">
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

              <details class="ct-technical ui-disclosure">
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
    </section>

    <div v-if="pendingConfirmationFeature" class="ct-confirm-backdrop" @click.self="cancelOfflineConfirmation">
      <section class="ct-confirm-dialog ui-card" role="dialog" aria-modal="true" aria-labelledby="ct-offline-title" aria-describedby="ct-offline-copy" @keydown.esc="cancelOfflineConfirmation" @keydown.tab="trapConfirmationFocus">
        <span class="confirm-emblem" aria-hidden="true"><i></i></span>
        <div>
          <p class="confirm-kicker">{{ tr('首次启用确认') }}</p>
          <h2 id="ct-offline-title">{{ tr('仅离线/单机使用') }}</h2>
          <p id="ct-offline-copy">{{ tr('这些功能会直接修改游戏运行时规则。请确认当前不在联机房间，并只在离线或单机内容中使用。本次打开应用只确认一次。') }}</p>
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
.ct084-page {
  width:100%;
  max-width:none;
  padding-bottom:var(--space-8);
  gap:var(--space-4);
}

.ct-connection {
  display:flex;
  min-height:72px;
  flex-direction:row;
  align-items:center;
  justify-content:space-between;
  border-color:var(--border-strong);
  background:color-mix(in srgb,var(--surface-card-pop) 90%,transparent);
}

.ct-connection-main,
.ct-connection-actions,
.ct-browser-head,
.ct-feature-summary,
.ct-feature-actions,
.ct-group-heading {
  min-width:0;
  display:flex;
  align-items:center;
  gap:var(--space-3);
}

.ct-connection-main { flex:1 1 auto; }
.ct-connection-actions { flex:0 0 auto; }
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

.ct-live-region {
  min-height:42px;
  display:flex;
  align-items:center;
  gap:var(--space-3);
}
.live-mark { width:7px; height:7px; flex:0 0 7px; border-radius:50%; background:currentColor; }

.ct-browser {
  gap:var(--space-4);
  border-color:var(--border-default);
  background:color-mix(in srgb,var(--surface-card) 91%,transparent);
}
.ct-browser-head { align-items:flex-end; justify-content:space-between; }
.ct-browser-head > div { min-width:0; flex:1 1 auto; }
.ct-search { width:min(330px,44%); flex:0 1 330px; }
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

.ct-feature-workspace { min-width:0; display:grid; grid-template-columns:minmax(0,1fr); gap:var(--space-4); align-items:start; }
.ct-group-pane { min-width:0; }
.ct-group-select { display:flex; }
.ct-group-disclosure { display:none; min-width:0; }
.ct-group-button {
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
.ct-group-button:hover,.ct-group-button:focus-visible { border-color:var(--border-default); background:var(--state-hover); color:var(--text-primary); }
.ct-group-button.is-on { border-color:var(--selected-border); background:var(--selected-bg); color:var(--selected-fg); box-shadow:3px 0 0 var(--selected-bar) inset; }
.ct-group-button span { min-width:0; overflow-wrap:anywhere; }
.ct-group-button b { min-width:24px; padding:1px var(--space-2); border-radius:var(--radius-pill); background:var(--surface-card-pop); color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); text-align:center; }

.ct-feature-column { min-width:0; }
.ct-group-heading { justify-content:space-between; margin-bottom:var(--space-3); padding:0 var(--space-1) var(--space-3); border-bottom:1px solid var(--border-soft); }
.ct-group-heading > div span { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.08em; }
.ct-group-heading h3 { margin:2px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); line-height:var(--lh-tight); }
.ct-group-heading small { color:var(--text-muted); font-size:var(--fs-xs); }
.ct-feature-list { min-width:0; display:flex; flex-direction:column; gap:var(--space-3); }

.ct-feature-card {
  min-width:0;
  padding:var(--space-4);
  border-color:var(--border-default);
  background:var(--surface-card-pop);
  box-shadow:none;
  transition:border-color var(--dur-base) var(--ease-out),background-color var(--dur-base) var(--ease-out),box-shadow var(--dur-base) var(--ease-out);
}
.ct-feature-card.is-on { border-color:var(--success); background:color-mix(in srgb,var(--success-bg) 36%,var(--surface-card-pop)); box-shadow:3px 0 0 var(--success) inset; }
.ct-feature-card.needs-recovery { border-color:var(--warning); box-shadow:3px 0 0 var(--warning) inset; }
.ct-feature-summary { align-items:flex-start; justify-content:space-between; }
.feature-title-block { min-width:0; }
.feature-kicker { display:flex; flex-wrap:wrap; gap:var(--space-2); color:var(--text-muted); font-size:var(--fs-xs); }
.feature-kicker span + span::before { content:"·"; margin-right:var(--space-2); }
.feature-title-block h4 { margin:var(--space-1) 0 0; color:var(--text-primary); font-size:var(--fs-base); line-height:var(--lh-tight); overflow-wrap:anywhere; }
.feature-conflict,.feature-error { margin:var(--space-3) 0 0; }
.ct-feature-actions { justify-content:space-between; margin-top:var(--space-3); padding-top:var(--space-3); border-top:1px solid var(--border-soft); }
.feature-proof { min-width:0; color:var(--text-muted); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.feature-switch { min-width:124px; flex:0 0 auto; }
.switch-track { width:27px; height:16px; flex:0 0 27px; padding:2px; border-radius:var(--radius-pill); background:color-mix(in srgb,var(--text-muted) 44%,transparent); transition:background-color var(--dur-base) var(--ease-out); }
.switch-track i { display:block; width:12px; height:12px; border-radius:50%; background:var(--surface-card-pop); box-shadow:0 1px 2px rgba(55,39,19,.24); transition:transform var(--dur-base) var(--ease-out); }
.switch-track.is-on { background:var(--success); }
.switch-track.is-on i { transform:translateX(11px); }

.ct-technical { margin-top:var(--space-3); box-shadow:none; }
.ct-technical > summary { min-height:var(--control-height-sm); padding-block:var(--space-2); color:var(--text-muted); font-size:var(--fs-xs); }
.ct-technical dl { display:flex; flex-wrap:wrap; gap:var(--space-3) var(--space-5); }
.ct-technical dl > div { min-width:0; }
.ct-technical dt { color:var(--text-muted); font-size:var(--fs-xs); }
.ct-technical dd { margin:2px 0 0; color:var(--text-secondary); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.site-list { display:flex; min-width:0; flex-direction:column; gap:var(--space-2); padding:0; list-style:none; }
.site-list li { min-width:0; padding:var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.site-list li > div { display:flex; min-width:0; justify-content:space-between; gap:var(--space-3); color:var(--text-secondary); font-size:var(--fs-xs); }
.site-list code,.site-list small { display:block; margin-top:var(--space-2); color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); overflow-wrap:anywhere; word-break:break-word; }
.site-list code { color:var(--text-secondary); }

.ct-empty { min-height:180px; display:grid; place-content:center; gap:var(--space-2); text-align:center; }
.ct-empty strong { color:var(--text-primary); font-size:var(--fs-md); }
.ct-empty span { color:var(--text-muted); font-size:var(--fs-sm); }

.ct-confirm-backdrop {
  position:fixed;
  z-index:60;
  inset:0;
  display:grid;
  place-items:center;
  padding:var(--space-5);
  background:rgba(50,37,20,.44);
  backdrop-filter:blur(3px);
}
.ct-confirm-dialog { width:min(520px,100%); max-height:calc(100dvh - 32px); overflow:auto; display:grid; grid-template-columns:auto minmax(0,1fr); gap:var(--space-4); padding:var(--space-7); border-color:var(--border-strong); background:var(--surface-card-pop); box-shadow:var(--shadow-3); }
.confirm-emblem::before,.confirm-emblem::after { content:""; position:absolute; background:var(--accent-hover); }
.confirm-emblem::before { width:2px; height:13px; top:7px; }
.confirm-emblem::after { width:2px; height:2px; bottom:7px; border-radius:50%; }
.confirm-kicker { margin:0; color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.08em; }
.ct-confirm-dialog h2 { margin:var(--space-1) 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); }
.ct-confirm-dialog p:not(.confirm-kicker) { margin:var(--space-3) 0 0; color:var(--text-secondary); font-size:var(--fs-md); line-height:var(--lh-relaxed); }
.confirm-feature { grid-column:1 / -1; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); padding:var(--space-3) var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--accent-soft); }
.confirm-feature span { color:var(--text-muted); font-size:var(--fs-sm); }
.confirm-feature strong { color:var(--text-primary); text-align:right; }
.confirm-actions { grid-column:1 / -1; }

@container tool-panel (min-width:680px) {
  .ct-feature-workspace { grid-template-columns:minmax(146px,30fr) minmax(0,70fr); gap:var(--space-4); }
  .ct-group-select { display:none; }
  .ct-group-disclosure { display:flex; flex-direction:column; gap:var(--space-1); }
}

@container tool-panel (max-width:679px) {
  .ct-browser-head { align-items:stretch; flex-direction:column; }
  .ct-search { width:100%; flex-basis:auto; }
  .ct-feature-workspace { grid-template-columns:minmax(0,1fr); }
  .ct-connection { align-items:stretch; flex-direction:column; }
  .ct-connection-actions { width:100%; }
  .ct-connection-actions .ui-btn { flex:1 1 150px; }
}

@container tool-panel (max-width:520px) {
  .ct084-page { gap:var(--space-3); padding-bottom:var(--space-5); }
  .ct-browser { padding:var(--space-4); }
  .ct-feature-actions { align-items:stretch; flex-direction:column; }
  .feature-switch { width:100%; }
  .ct-group-heading { align-items:flex-start; flex-direction:column; }
}

@container tool-panel (max-width:340px) {
  .ct-browser-head .ui-section-copy { display:none; }
  .ct-connection-main { align-items:flex-start; flex-wrap:wrap; }
  .ct-connection-main > .ui-tag { margin-left:46px; }
  .ct-feature-card { padding:var(--space-3); }
  .ct-feature-summary { gap:var(--space-2); }
  .ct-feature-summary > .ui-tag { flex:0 0 auto; }
  .ct-confirm-dialog { grid-template-columns:minmax(0,1fr); padding:var(--space-5); }
  .confirm-emblem { display:none; }
  .confirm-feature,.confirm-actions { grid-column:1; }
  .confirm-actions .ui-btn { width:100%; }
}

@media (prefers-reduced-motion:reduce) {
  .ct-feature-card,.ct-group-button,.switch-track,.switch-track i { transition:none; }
}
</style>
