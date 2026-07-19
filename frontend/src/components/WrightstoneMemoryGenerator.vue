<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  WrightstoneMemoryAcquire,
  WrightstoneMemoryGetOptions,
  WrightstoneMemoryGetStatus,
  WrightstoneMemoryRelease,
  WrightstoneMemoryUpdateOwned,
} from '../../wailsjs/go/main/App'
import { traitAssetIcon } from '../gameAssetIcons'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'
import ConfirmDialog from './ConfirmDialog.vue'

const EMPTY_HASH = 0x887AE0B0
const RUNTIME_LEASE_SCOPE = 'wrightstone-memory-generator'
const emit = defineEmits(['status'])
const confirmDialog = ref(null)
const loading = ref(false)
const writing = ref(false)
const stale = ref(false)
const liveMessage = ref('尚未启用读取。')
const traits = ref([])
const status = reactive({
  found: false,
  hooked: false,
  selectedAddr: 0,
  firstHash: 0,
  firstName: '',
  firstLevel: 0,
  secondHash: 0,
  secondName: '',
  secondLevel: 0,
  thirdHash: 0,
  thirdName: '',
  thirdLevel: 0,
})
const traitSlots = reactive([
  { label: '第一槽', hashKey: 'firstHash', nameKey: 'firstName', levelKey: 'firstLevel', maxLevel: 20, hash: 0, level: 0 },
  { label: '第二槽', hashKey: 'secondHash', nameKey: 'secondName', levelKey: 'secondLevel', maxLevel: 15, hash: 0, level: 0 },
  { label: '第三槽', hashKey: 'thirdHash', nameKey: 'thirdName', levelKey: 'thirdLevel', maxLevel: 10, hash: 0, level: 0 },
])
let pollTimer = 0
let lastSelectedAddr = 0
let disposed = false
let lifecycleEpoch = 0
let hookOwnerToken = ''

const statusLabel = computed(() => {
  if (stale.value) return '记录已失效'
  if (status.selectedAddr) return '已锁定记录'
  if (status.hooked) return '等待游戏内选择'
  if (status.found) return '已定位，尚未启用'
  return '未连接'
})

function formatHex(value) {
  const hash = normaliseHash(value)
  return hash ? `0x${hash.toString(16).toUpperCase().padStart(8, '0')}` : '—'
}

function normaliseHash(value) {
  const hash = Number(value || 0) >>> 0
  return hash === EMPTY_HASH ? 0 : hash
}
function currentHash(slot) { return normaliseHash(status[slot.hashKey]) }
function currentName(slot) { return status[slot.nameKey] || formatHex(currentHash(slot)) }
function currentLevel(slot) { return Number(status[slot.levelKey] || 0) }
function traitOption(hash) { return traits.value.find(item => normaliseHash(item.hash) === normaliseHash(hash)) }
function targetName(slot) { return traitOption(slot.hash)?.displayName || formatHex(slot.hash) }
function traitIcon(hash, name = '') { return traitAssetIcon({ hash, name }) }
function levelMax(slot) { return Math.min(slot.maxLevel, Number(traitOption(slot.hash)?.maxLevel || 0)) }
function allowedLevels(slot) {
  const option = traitOption(slot.hash)
  const explicit = Array.isArray(option?.allowedLevels) ? option.allowedLevels : []
  const levels = explicit.length ? explicit : Array.from({ length: levelMax(slot) }, (_, index) => index + 1)
  return [...new Set(levels.map(Number))].filter(level => Number.isInteger(level) && level >= 1 && level <= levelMax(slot)).sort((a, b) => a - b)
}

function syncDraftFromStatus() {
  for (const slot of traitSlots) {
    slot.hash = currentHash(slot)
    slot.level = currentLevel(slot)
  }
}

function clearDraft() {
  for (const slot of traitSlots) {
    slot.hash = 0
    slot.level = 0
  }
}

function applyStatus(next, { sync = false } = {}) {
  const incoming = next || {}
  Object.assign(status, {
    found: false,
    hooked: false,
    selectedAddr: 0,
    firstHash: 0,
    firstName: '',
    firstLevel: 0,
    secondHash: 0,
    secondName: '',
    secondLevel: 0,
    thirdHash: 0,
    thirdName: '',
    thirdLevel: 0,
    ...incoming,
  })
  const address = Number(status.selectedAddr || 0)
  if (!address) {
    lastSelectedAddr = 0
    return
  }
  if (sync || address !== lastSelectedAddr) {
    stale.value = false
    lastSelectedAddr = address
    syncDraftFromStatus()
  }
}

const changes = computed(() => traitSlots.flatMap((slot, index) => {
  const rows = []
  if (normaliseHash(slot.hash) !== currentHash(slot)) {
    rows.push(`${slot.label}：${currentName(slot)} → ${targetName(slot)}`)
  }
  if (Number(slot.level) !== currentLevel(slot)) {
    rows.push(`${slot.label}等级：${currentLevel(slot)} → ${Number(slot.level)}`)
  }
  return rows.map(text => ({ key: `${index}-${text}`, text }))
}))

const validationMessage = computed(() => {
  if (!status.hooked) return '请先启用读取。'
  if (stale.value) return '上次写入后记录已失效，请在游戏内重新选择记录。'
  if (!status.selectedAddr) return '请在游戏内祝福石列表重新选中目标记录。'
  if (!traitSlots[0].hash) return '第一槽词条不能为空。'
  for (const slot of traitSlots) {
    if (!slot.hash && Number(slot.level) !== 0) return `${slot.label}为空时等级必须为 0。`
    const option = traitOption(slot.hash)
    if (slot.hash && !option) return `${slot.label}词条不在已验证目录中。`
    if (slot.hash && !allowedLevels(slot).includes(Number(slot.level))) {
      return `${slot.label}等级不在该槽位的允许范围内。`
    }
  }
  const selectedHashes = traitSlots.map(slot => normaliseHash(slot.hash)).filter(Boolean)
  const uniqueHashes = new Set(traitSlots.map(slot => normaliseHash(slot.hash)).filter(Boolean))
  if (uniqueHashes.size !== selectedHashes.length) return '三个槽位不能选择重复词条。'
  if (!changes.value.length) return '目标值与当前记录相同。'
  return ''
})
const canWrite = computed(() => !loading.value && !writing.value && !validationMessage.value)

function normaliseLevel(slot) {
  if (!slot.hash) { slot.level = 0; return }
  const levels = allowedLevels(slot)
  const numeric = Number(slot.level)
  slot.level = levels.includes(numeric) ? numeric : (levels.at(-1) || 0)
}

function stopPolling() {
  if (pollTimer) window.clearInterval(pollTimer)
  pollTimer = 0
}

function startPolling() {
  stopPolling()
  pollTimer = window.setInterval(() => pollStatus(true), 700)
}

async function pollStatus(silent = false) {
  const epoch = lifecycleEpoch
  const previousSelectedAddr = Number(status.selectedAddr || 0)
  try {
    const next = await WrightstoneMemoryGetStatus()
    if (disposed || epoch !== lifecycleEpoch) return
    applyStatus(next)
    if (!status.hooked) stopPolling()
    if (!previousSelectedAddr && status.selectedAddr) liveMessage.value = '已读取当前祝福石记录。'
    else if (previousSelectedAddr && !status.selectedAddr && status.hooked) liveMessage.value = '当前记录已释放，请在游戏内重新选择祝福石。'
    else if (!silent) liveMessage.value = status.selectedAddr ? '已读取当前祝福石记录。' : '等待游戏内选择祝福石记录。'
  } catch (error) {
    if (disposed || epoch !== lifecycleEpoch) return
    stopPolling()
    applyStatus(null)
    liveMessage.value = `读取已停止：${String(error)}`
    if (!silent) emit('status', liveMessage.value, 'error')
  }
}

async function enable() {
  const epoch = ++lifecycleEpoch
  loading.value = true
  let acquiredOwnerToken = ''
  try {
    const next = await WrightstoneMemoryAcquire(nextRuntimeAcquireRequestID())
    acquiredOwnerToken = String(next?.ownerToken || '')
    if (!acquiredOwnerToken) throw new Error('后端未返回祝福石读取所有权令牌')
    if (disposed || epoch !== lifecycleEpoch) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, WrightstoneMemoryRelease)
      return
    }
    hookOwnerToken = acquiredOwnerToken
    applyStatus(next, { sync: true })
    liveMessage.value = status.selectedAddr ? '读取已启用，并已捕获一条记录。' : '读取已启用，请在游戏内选择一条祝福石记录。'
    emit('status', liveMessage.value, 'success')
    startPolling()
  } catch (error) {
    if (disposed || epoch !== lifecycleEpoch) return
    liveMessage.value = String(error)
    emit('status', liveMessage.value, 'error')
  } finally {
    if (!disposed && epoch === lifecycleEpoch) loading.value = false
  }
}

async function disable() {
  const epoch = ++lifecycleEpoch
  const ownerToken = hookOwnerToken
  stopPolling()
  if (!ownerToken) {
    applyStatus(null)
    stale.value = false
    clearDraft()
    return
  }
  loading.value = true
  try {
    const next = await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, WrightstoneMemoryRelease)
    if (disposed || epoch !== lifecycleEpoch) return
    if (hookOwnerToken === ownerToken) hookOwnerToken = ''
    applyStatus(next)
    stale.value = false
    clearDraft()
    liveMessage.value = '读取已停止，游戏指令已恢复。'
    emit('status', liveMessage.value, 'success')
  } catch (error) {
    if (disposed || epoch !== lifecycleEpoch) return
    liveMessage.value = String(error)
    emit('status', liveMessage.value, 'error')
  } finally {
    if (!disposed && epoch === lifecycleEpoch) loading.value = false
  }
}

async function write() {
  if (!canWrite.value) {
    liveMessage.value = validationMessage.value
    emit('status', liveMessage.value, 'error')
    return
  }
  const expectedSelectedAddr = Number(status.selectedAddr || 0)
  const payload = {
    expectedSelectedAddr,
    firstHash: Number(traitSlots[0].hash) >>> 0,
    firstLevel: Number(traitSlots[0].level),
    secondHash: Number(traitSlots[1].hash) >>> 0,
    secondLevel: Number(traitSlots[1].level),
    thirdHash: Number(traitSlots[2].hash) >>> 0,
    thirdLevel: Number(traitSlots[2].level),
  }
  const changeDetail = changes.value.map(item => item.text).join('\n')
  const confirmed = await confirmDialog.value?.ask({
    title: '写入当前祝福石',
    message: `确认写入 ${changes.value.length} 项变更？`,
    detail: changeDetail,
    tone: 'warning',
    confirmLabel: '确认写入',
  })
  if (!confirmed) return
  writing.value = true
  try {
    const ownerToken = hookOwnerToken
    if (!ownerToken) throw new Error('当前页面不再持有祝福石读取所有权')
    const result = await WrightstoneMemoryUpdateOwned(ownerToken, payload)
    applyStatus(result)
    clearDraft()
    stale.value = true
    stopPolling()
    try {
      const released = await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, WrightstoneMemoryRelease)
      if (disposed) return
      if (hookOwnerToken === ownerToken) hookOwnerToken = ''
      applyStatus(released)
      stale.value = false
      liveMessage.value = '写入成功，读取已自动停止，游戏指令已恢复。'
      emit('status', liveMessage.value, 'success')
    } catch (releaseError) {
      if (disposed) return
      liveMessage.value = `写入已成功，但自动停止读取失败；请立即点击“停止读取”：${String(releaseError)}`
      emit('status', liveMessage.value, 'error')
    }
  } catch (error) {
    liveMessage.value = String(error)
    emit('status', liveMessage.value, 'error')
  } finally {
    writing.value = false
  }
}

onMounted(async () => {
  loading.value = true
  try {
    const result = await WrightstoneMemoryGetOptions()
    traits.value = Array.isArray(result?.traits) ? result.traits : []
    await pollStatus(true)
    if (status.hooked) startPolling()
  } catch (error) {
    liveMessage.value = String(error)
  } finally {
    loading.value = false
  }
})

onBeforeUnmount(() => {
  disposed = true
  lifecycleEpoch++
  stopPolling()
  const ownerToken = hookOwnerToken
  hookOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, WrightstoneMemoryRelease)
})
</script>

<template>
  <div class="wrightstone-memory-page ui-page is-wide ui-page-stack">
    <div class="wrightstone-memory-content">
      <section class="connection-card ui-card ui-panel">
        <header class="ui-split">
          <div>
            <h2 class="ui-section-title">捕获状态</h2>
            <p class="ui-section-copy">捕获游戏内当前选中的祝福石记录，核对三槽后一次性写入。</p>
          </div>
          <span class="ui-tag" :class="status.selectedAddr && !stale ? 'is-ok' : status.hooked ? 'is-info' : ''">{{ statusLabel }}</span>
        </header>
        <div class="connection-actions ui-actions">
          <button type="button" class="ui-btn is-primary" :disabled="loading || status.hooked" @click="enable">启用读取</button>
          <button type="button" class="ui-btn" :disabled="loading || !status.hooked" @click="pollStatus(false)">刷新状态</button>
          <button type="button" class="ui-btn is-ghost" :disabled="loading || !status.hooked" @click="disable">停止读取</button>
        </div>
        <p class="connection-message ui-notice" :class="{ 'is-warn': stale }" aria-live="polite">{{ liveMessage }}</p>
        <p class="ui-hint">读取 Hook 启用期间不要切入铁匠铺的其他操作；写入成功后会自动停止读取，避免游戏闪退。</p>
      </section>

      <section class="record-panel ui-card ui-panel">
        <header class="ui-split">
          <div>
            <h3 class="ui-section-title">三槽记录</h3>
            <p class="ui-section-copy">当前值保持只读；目标值只有在重新捕获记录后才可写。</p>
          </div>
          <code>{{ status.selectedAddr ? `0x${Number(status.selectedAddr).toString(16).toUpperCase()}` : '未选择' }}</code>
        </header>

        <div v-if="!status.selectedAddr || stale" class="selection-inline-notice" role="status">
          <span>{{ stale ? '写入后的旧记录不可复用，请在游戏内重新选中一条记录。' : '启用读取后，在游戏内祝福石列表选中目标记录。' }}</span>
        </div>

        <div class="trait-grid ui-card-grid" :aria-disabled="!status.selectedAddr || stale">
          <article v-for="(slot, index) in traitSlots" :key="slot.label" class="trait-card ui-card ui-panel is-compact">
            <header class="ui-split"><h4 class="ui-section-title">{{ slot.label }}</h4><span class="ui-tag">{{ index + 1 }} / 3</span></header>
            <div class="trait-current">
              <img v-if="traitIcon(currentHash(slot), currentName(slot))" :src="traitIcon(currentHash(slot), currentName(slot))" alt="" />
              <small>当前</small>
              <strong>{{ currentName(slot) }}</strong>
              <code>{{ formatHex(currentHash(slot)) }} · Lv {{ currentLevel(slot) }}</code>
            </div>
            <div class="trait-target">
              <img v-if="traitIcon(slot.hash, targetName(slot))" class="trait-target-icon" :src="traitIcon(slot.hash, targetName(slot))" alt="" />
              <label class="ui-field">
                <span class="ui-field-label">目标词条</span>
                <select v-model.number="slot.hash" class="ui-select" :disabled="!status.selectedAddr || stale" @change="normaliseLevel(slot)">
                  <option :value="0">— 空槽 —</option>
                  <option v-for="trait in traits" :key="trait.hash" :value="Number(trait.hash)">{{ trait.displayName }} · {{ formatHex(trait.hash) }}</option>
                </select>
              </label>
              <label class="ui-field">
                <span class="ui-field-label">目标等级</span>
                <select v-model.number="slot.level" class="ui-select ui-input" :disabled="!slot.hash || !status.selectedAddr || stale">
                  <option v-for="level in allowedLevels(slot)" :key="level" :value="level">Lv {{ level }}</option>
                </select>
              </label>
            </div>
          </article>
        </div>

        <details class="ui-disclosure change-summary">
          <summary>变更摘要 · {{ changes.length }} 项</summary>
          <ul v-if="changes.length"><li v-for="item in changes" :key="item.key">{{ item.text }}</li></ul>
          <p v-else class="ui-hint">当前没有待写入变更。</p>
        </details>
      </section>

      <aside class="wrightstone-memory-actions ui-card ui-panel is-compact">
        <span><b>{{ statusLabel }}</b><small>{{ validationMessage || `${changes.length} 项待写入` }}</small></span>
        <button type="button" class="ui-btn" :disabled="!status.selectedAddr || stale" @click="syncDraftFromStatus">还原当前值</button>
        <button type="button" class="ui-btn is-primary" :disabled="!canWrite" @click="write">{{ writing ? '写入中…' : '写入三槽' }}</button>
      </aside>
    </div>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.wrightstone-memory-page { min-height:0; padding:0; }
.wrightstone-memory-content { min-height:0; padding:0 var(--space-1) var(--space-8) 0; }
.wrightstone-memory-content > * + * { margin-top:var(--space-4); }
.connection-card,.record-panel { background:var(--surface-card-pop); }
.connection-actions { margin-top:var(--space-4); }
.connection-message { margin:var(--space-4) 0 0; }
.record-panel > header code { color:var(--text-muted); font-family:var(--font-data); }
.selection-inline-notice {
  min-height:0;
  display:flex;
  align-items:center;
  margin-top:var(--space-3);
  padding:var(--space-2) var(--space-3);
  border-left:3px solid var(--info);
  color:var(--text-secondary);
  background:color-mix(in srgb,var(--info-bg) 58%,transparent);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
  text-align:left;
}
.trait-grid { --ui-grid-min:260px; margin-top:var(--space-4); }
.trait-card { display:grid; gap:var(--space-4); }
.trait-current { display:grid; grid-template-columns:44px minmax(0,1fr); gap:var(--space-1) var(--space-3); align-items:center; padding:var(--space-3); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.trait-current img { width:44px; height:44px; grid-row:1/4; object-fit:cover; border:1px solid var(--line-soft); border-radius:7px; background:rgba(255,255,255,.65); }
.trait-current small { color:var(--text-muted); font-size:var(--fs-xs); }
.trait-current strong { color:var(--text-primary); }
.trait-current code { color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.trait-target { display:grid; grid-template-columns:42px minmax(0,1fr) 92px; gap:var(--space-3); align-items:end; }
.trait-target-icon { width:42px; height:42px; object-fit:cover; align-self:end; border:1px solid var(--line-soft); border-radius:7px; background:rgba(255,255,255,.65); }
.change-summary { margin-top:var(--space-4); }
.change-summary ul { display:grid; gap:var(--space-2); margin:0; padding-left:var(--space-5); color:var(--text-secondary); }
.wrightstone-memory-actions { position:sticky; z-index:4; bottom:0; display:grid; grid-template-columns:minmax(0,1fr) auto auto; gap:var(--space-3); align-items:center; border-top:3px solid var(--accent); background:var(--surface-card-pop); box-shadow:var(--shadow-3); font-family:var(--font-data); }
.wrightstone-memory-actions span { display:grid; min-width:0; gap:var(--space-1); }
.wrightstone-memory-actions small { color:var(--text-muted); overflow-wrap:anywhere; }

@container ui-page (max-width:1024px) {
  .trait-grid { --ui-grid-min:220px; }
}
@container ui-page (max-width:768px) {
  .trait-grid { --ui-grid-min:100%; }
  .wrightstone-memory-actions { grid-template-columns:minmax(0,1fr) auto; }
  .wrightstone-memory-actions span { grid-column:1 / -1; }
}
@container ui-page (max-width:375px) {
  .connection-actions { display:grid; grid-template-columns:minmax(0,1fr); }
  .trait-target,.wrightstone-memory-actions { grid-template-columns:minmax(0,1fr); }
  .wrightstone-memory-actions span { grid-column:auto; }
  .wrightstone-memory-actions .ui-btn { width:100%; }
}
@container ui-page (min-width:1080px) {
  .trait-grid { --ui-grid-min:320px; }
  .record-panel { padding-inline:var(--space-8); }
}
</style>
