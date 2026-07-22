<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  SigilMemoryAcquire,
  SigilMemoryGetOptions,
  SigilMemoryGetStatus,
  SigilMemoryRelease,
  SigilMemoryUpdateOwned,
  SigilMemoryValidateLoadout,
} from '../../wailsjs/go/backend/App'
import { traitAssetIcon } from '../gameAssetIcons'
import { createOperationGate, freezeSigilLoadout } from '../runtimeOperationGate'
import { createSelectionTracker, takeSelectionAddress } from '../sigilLoadoutSelection'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'

const emit = defineEmits(['status'])
const FORMAT = 'gbfr-sigil-loadout'
const VERSION = 1
const MAX_ENTRIES = 12
const EMPTY_HASH = 0x887AE0B0
const POLL_DELAY = 120
const RUNTIME_LEASE_SCOPE = 'sigil-loadout-restore'

const mode = ref('idle')
const entries = ref([])
const options = reactive({ sigils: [], traits: [] })
const loadoutVersion = ref('DLC 2.0.2')
const comment = ref('')
const progress = ref(0)
const fileInput = ref(null)
let pollTimer = 0
let polling = false
let disposed = false
let selectionTracker = createSelectionTracker()
const operationGate = createOperationGate()
let activeRunToken = null
let hookOwnerToken = ''
let stoppingPromise = null
let writeSnapshot = Object.freeze([])
let recordPrimeSeen = false

const isActive = computed(() => mode.value !== 'idle')
const modeLabel = computed(() => ({
  'starting-record': '正在启动记录',
  'record-prime': '正在确认第一项',
  'starting-write': '正在启动复刻',
  record: '正在记录',
  write: '正在复刻',
  stopping: '正在恢复游戏指令',
  error: '需要重试停止',
})[mode.value] || '等待操作')
const progressLabel = computed(() => mode.value.includes('write')
  ? `${Math.min(progress.value, writeSnapshot.length)} / ${writeSnapshot.length}`
  : `${entries.value.length} / ${MAX_ENTRIES}`)

const sigilNames = computed(() => new Map(options.sigils.map(item => [Number(item.hash) >>> 0, item.displayName])))
const traitNames = computed(() => new Map(options.traits.map(item => [Number(item.hash) >>> 0, item.displayName])))

function showStatus(message, type = 'info') { emit('status', message, type) }
function uint(value) { return Number(value) >>> 0 }
function signature(item) {
  return [item.sigilHash, item.sigilLevel, item.primaryTraitHash, item.primaryTraitLevel,
    item.secondaryTraitHash, item.secondaryTraitLevel].map(uint).join(':')
}
function normalizeEntry(item) {
  return {
    sigilHash: uint(item.sigilHash),
    sigilLevel: uint(item.sigilLevel),
    primaryTraitHash: uint(item.primaryTraitHash),
    primaryTraitLevel: uint(item.primaryTraitLevel),
    secondaryTraitHash: uint(item.secondaryTraitHash),
    secondaryTraitLevel: uint(item.secondaryTraitLevel),
  }
}
function validEntry(item) {
  if (!item || typeof item !== 'object') return false
  return ['sigilHash', 'sigilLevel', 'primaryTraitHash', 'primaryTraitLevel',
    'secondaryTraitHash', 'secondaryTraitLevel'].every(key =>
      Number.isInteger(Number(item[key])) && Number(item[key]) >= 0 && Number(item[key]) <= 0xffffffff)
}
function isSelectable(status) {
  return !!status?.hooked && Number(status?.selectedAddr || 0) !== 0 &&
    uint(status?.sigilHash) !== 0 && uint(status?.sigilHash) !== EMPTY_HASH
}
function takeSelectableAddress(status) {
  return takeSelectionAddress(selectionTracker, isSelectable(status) ? status.selectedAddr : 0)
}
function resetSelectionTracker(selectedAddr = 0) {
  selectionTracker = createSelectionTracker()
  if (selectedAddr) takeSelectionAddress(selectionTracker, selectedAddr)
}
function formatHash(value) { return `0x${uint(value).toString(16).toUpperCase().padStart(8, '0')}` }
function sigilName(entry) { return sigilNames.value.get(uint(entry.sigilHash)) || formatHash(entry.sigilHash) }
function traitName(hash) {
  const value = uint(hash)
  if (!value || value === EMPTY_HASH) return '无副词条'
  return traitNames.value.get(value) || formatHash(value)
}
function traitIcon(hash) {
  const value = uint(hash)
  if (!value || value === EMPTY_HASH) return ''
  return traitAssetIcon({ hash: value, name: traitNames.value.get(value) || '' })
}

function stopPolling() {
  window.clearTimeout(pollTimer)
  pollTimer = 0
  polling = false
}

async function stop(message = '') {
  if (stoppingPromise) return stoppingPromise
  stopPolling()
  if (activeRunToken) operationGate.finish(activeRunToken)
  activeRunToken = null
  const token = operationGate.begin('stop')
  mode.value = 'stopping'
  stoppingPromise = (async () => {
    try {
      const ownerToken = hookOwnerToken
      if (ownerToken) {
        await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, SigilMemoryRelease)
        if (hookOwnerToken === ownerToken) hookOwnerToken = ''
      }
      if (disposed || !operationGate.isCurrent(token)) return false
      writeSnapshot = Object.freeze([])
      mode.value = 'idle'
      if (message) showStatus(message, 'success')
      return true
    } catch (error) {
      if (!disposed && operationGate.isCurrent(token)) {
        mode.value = 'error'
        showStatus(`停止因子读取失败：${String(error)}`, 'error')
      }
      return false
    } finally {
      operationGate.finish(token)
      stoppingPromise = null
    }
  })()
  return stoppingPromise
}

async function failRun(token, error) {
  if (!operationGate.isCurrent(token)) return
  stopPolling()
  mode.value = 'stopping'
  let cleanupError = null
  const ownerToken = hookOwnerToken
  if (ownerToken) {
    try {
      await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, SigilMemoryRelease)
      if (hookOwnerToken === ownerToken) hookOwnerToken = ''
    } catch (nextError) { cleanupError = nextError }
  }
  if (disposed || !operationGate.isCurrent(token)) return
  if (cleanupError) {
    mode.value = 'error'
    showStatus(`${String(error)}；停止因子读取也失败：${String(cleanupError)}`, 'error')
    return
  }
  operationGate.finish(token)
  activeRunToken = null
  writeSnapshot = Object.freeze([])
  mode.value = 'idle'
  showStatus(String(error), 'error')
}

async function poll() {
  if (disposed || mode.value === 'idle' || polling) return
  const token = activeRunToken
  const activeMode = mode.value
  if (!token || !operationGate.isCurrent(token) || !['record-prime', 'record', 'write'].includes(activeMode)) return
  polling = true
  try {
    const status = await SigilMemoryGetStatus()
    if (disposed || !operationGate.isCurrent(token) || mode.value !== activeMode) return
    const selectedAddr = takeSelectableAddress(status)
    if (!selectedAddr) return

    if (activeMode === 'record-prime') {
      if (!recordPrimeSeen) {
        recordPrimeSeen = true
        showStatus('已捕获预热项；现在回到第一项，第一项会作为第 1 个因子正式记录', 'info')
        return
      }
      entries.value = [normalizeEntry(status)]
      progress.value = 1
      resetSelectionTracker(selectedAddr)
      mode.value = 'record'
      showStatus('第一项已确认，请从第二项开始逐项向下移动', 'success')
    } else if (activeMode === 'record') {
      const entry = normalizeEntry(status)
      entries.value = [...entries.value, entry]
      progress.value = entries.value.length
      if (entries.value.length >= MAX_ENTRIES) await stop('已记录完整的 12 个因子')
    } else if (activeMode === 'write') {
      const target = writeSnapshot[progress.value]
      if (!target) {
        await stop('因子配装复刻完成')
        return
      }
      const ownerToken = hookOwnerToken
      if (!ownerToken) throw new Error('当前页面不再持有因子读取所有权')
      await SigilMemoryUpdateOwned(ownerToken, { ...target, expectedSelectedAddr: selectedAddr })
      if (disposed || !operationGate.isCurrent(token) || mode.value !== activeMode) return
      progress.value += 1
      if (progress.value >= writeSnapshot.length) await stop(`已复刻 ${writeSnapshot.length} 个因子`)
    }
  } catch (error) {
    await failRun(token, error)
  } finally {
    polling = false
    if (!disposed && operationGate.isCurrent(token) && ['record-prime', 'record', 'write'].includes(mode.value)) {
      pollTimer = window.setTimeout(poll, POLL_DELAY)
    }
  }
}

async function startRecord() {
  if (disposed || mode.value !== 'idle' || operationGate.busy) return
  const token = operationGate.begin('record')
  if (!token) return
  activeRunToken = token
  mode.value = 'starting-record'
  entries.value = []
  progress.value = 0
  recordPrimeSeen = false
  resetSelectionTracker()
  try {
    const status = await SigilMemoryAcquire(nextRuntimeAcquireRequestID())
    const acquiredOwnerToken = String(status?.ownerToken || '')
    if (!acquiredOwnerToken) throw new Error('后端未返回因子读取所有权令牌')
    if (disposed || !operationGate.isCurrent(token)) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, SigilMemoryRelease)
      return
    }
    hookOwnerToken = acquiredOwnerToken
    mode.value = 'record'
    if (takeSelectableAddress(status)) {
      const entry = normalizeEntry(status)
      entries.value = [entry]
      progress.value = 1
      showStatus('已自动记录当前第一项，请移动到第二项并继续向下', 'success')
    } else {
      mode.value = 'record-prime'
      showStatus('读取器刚启用，尚无当前项：请按一次↓到第二项，再按↑回到第一项；程序会从第一项正式记录', 'info')
    }
    pollTimer = window.setTimeout(poll, POLL_DELAY)
  } catch (error) { await failRun(token, error) }
}

async function startWrite() {
  if (!entries.value.length) {
    showStatus('请先记录或导入一份因子配装', 'error')
    return
  }
  if (disposed || mode.value !== 'idle' || operationGate.busy) return
  const token = operationGate.begin('write')
  if (!token) return
  activeRunToken = token
  mode.value = 'starting-write'
  writeSnapshot = freezeSigilLoadout(entries.value, normalizeEntry)
  progress.value = 0
  resetSelectionTracker()
  try {
    await SigilMemoryValidateLoadout(writeSnapshot)
    if (disposed || !operationGate.isCurrent(token)) return
    const status = await SigilMemoryAcquire(nextRuntimeAcquireRequestID())
    const acquiredOwnerToken = String(status?.ownerToken || '')
    if (!acquiredOwnerToken) throw new Error('后端未返回因子读取所有权令牌')
    if (disposed || !operationGate.isCurrent(token)) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, SigilMemoryRelease)
      return
    }
    hookOwnerToken = acquiredOwnerToken
    mode.value = 'write'
    showStatus('复刻已开始：停在第一件备用因子上，再逐项向下移动', 'success')
    // The current row is the first write target, so apply entry 1 immediately
    // and then wait for a changed selection before applying entry 2.
    const selectedAddr = takeSelectableAddress(status)
    if (selectedAddr) {
      const target = writeSnapshot[0]
      const ownerToken = hookOwnerToken
      if (!ownerToken) throw new Error('当前页面不再持有因子读取所有权')
      await SigilMemoryUpdateOwned(ownerToken, { ...target, expectedSelectedAddr: selectedAddr })
      if (disposed || !operationGate.isCurrent(token) || mode.value !== 'write') return
      progress.value = 1
      if (writeSnapshot.length === 1) {
        await stop('已复刻 1 个因子')
        return
      }
    }
    pollTimer = window.setTimeout(poll, POLL_DELAY)
  } catch (error) { await failRun(token, error) }
}

function removeEntry(index) {
  if (isActive.value) return
  entries.value = entries.value.filter((_, i) => i !== index)
}

function clearEntries() {
  if (isActive.value) return
  entries.value = []
  progress.value = 0
}

function exportLoadout() {
  if (!entries.value.length) {
    showStatus('没有可导出的因子记录', 'error')
    return
  }
  const payload = {
    format: FORMAT,
    version: VERSION,
    loadoutVersion: loadoutVersion.value.trim(),
    comment: comment.value.trim(),
    entries: entries.value.map(normalizeEntry),
  }
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'gbfr-sigil-loadout.json'
  link.click()
  URL.revokeObjectURL(url)
  showStatus(`已导出 ${entries.value.length} 个因子`, 'success')
}

function openImport() { if (!isActive.value) fileInput.value?.click() }
async function importLoadout(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file || isActive.value) return
  try {
    const payload = JSON.parse(await file.text())
    if (payload?.format !== FORMAT || Number(payload?.version) !== VERSION) throw new Error('不是受支持的 GBFR 因子配装文件')
    if (!Array.isArray(payload.entries) || payload.entries.length < 1 || payload.entries.length > MAX_ENTRIES) {
      throw new Error(`配装必须包含 1 到 ${MAX_ENTRIES} 个因子`)
    }
    if (!payload.entries.every(validEntry)) throw new Error('配装文件包含无效字段或超出 uint32 的数值')
    entries.value = payload.entries.map(normalizeEntry)
    loadoutVersion.value = String(payload.loadoutVersion || 'DLC 2.0.2')
    comment.value = String(payload.comment || '')
    progress.value = 0
    showStatus(`已导入 ${entries.value.length} 个因子`, 'success')
  } catch (error) { showStatus(`导入失败：${String(error)}`, 'error') }
}

onMounted(async () => {
  try {
    const result = await SigilMemoryGetOptions()
    options.sigils = result?.sigils || []
    options.traits = result?.traits || []
  } catch (error) { showStatus(`加载因子目录失败：${String(error)}`, 'error') }
})
onBeforeUnmount(() => {
  disposed = true
  operationGate.invalidate()
  activeRunToken = null
  stopPolling()
  const ownerToken = hookOwnerToken
  hookOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, SigilMemoryRelease)
})
</script>

<template>
  <div class="loadout-root ui-page is-wide ui-page-stack">
    <div class="loadout-overview">
      <section class="ui-card ui-panel workflow-card">
        <h2 class="ui-section-title">配装读取 <small>最多记录 12 个因子</small></h2>
        <div class="ui-notice status-strip" :class="{ 'is-info': mode !== 'idle' }" aria-live="polite">
          <div><b>{{ modeLabel }}</b><span>{{ mode === 'record-prime' ? '先按↓再按↑回到第一项，完成首项握手' : mode === 'record' ? '在游戏因子列表中逐项向下移动' : mode === 'write' ? '每次移动到下一件备用因子都会写入' : '启动游戏并进入因子列表后开始' }}</span></div>
          <strong>{{ progressLabel }}</strong>
        </div>
        <div class="ui-actions primary-actions">
          <button class="ui-btn is-primary is-grow" :disabled="!['idle', 'record-prime', 'record'].includes(mode)" @click="['record-prime', 'record'].includes(mode) ? stop('已停止记录') : startRecord()">
            {{ ['record-prime', 'record'].includes(mode) ? '停止记录' : mode === 'starting-record' ? '正在启动…' : '记录当前配装' }}
          </button>
          <button class="ui-btn is-primary is-grow" :disabled="!['idle', 'write'].includes(mode) || (mode === 'idle' && !entries.length)" @click="mode === 'write' ? stop('已停止复刻') : startWrite()">
            {{ mode === 'write' ? '停止复刻' : mode === 'starting-write' ? '正在启动…' : '复刻到备用因子' }}
          </button>
          <button v-if="mode === 'error'" class="ui-btn is-danger is-grow" @click="stop()">重试恢复游戏指令</button>
        </div>
        <div class="ui-notice operation-note"><b>操作顺序</b><span>先装备目标角色的 12 个因子并停在第一项。若启动时没有自动读到第一项，按提示“↓一次、↑一次”完成首项握手，再从第二项逐项向下移动。</span></div>
      </section>

      <section class="ui-card ui-panel file-card">
        <h2 class="ui-section-title">配装文件 <small>可分享 JSON 文件</small></h2>
        <div class="ui-form-grid meta-grid">
          <label class="ui-field"><span class="ui-field-label">适用版本</span><input v-model="loadoutVersion" class="ui-input" :disabled="isActive" maxlength="80"></label>
          <label class="ui-field"><span class="ui-field-label">备注</span><input v-model="comment" class="ui-input" :disabled="isActive" maxlength="160" placeholder="角色、用途或配装说明"></label>
        </div>
        <div class="ui-actions file-actions">
          <button class="ui-btn" :disabled="isActive" @click="openImport">导入配装</button>
          <button class="ui-btn" :disabled="isActive || !entries.length" @click="exportLoadout">导出配装</button>
          <button class="ui-btn is-ghost" :disabled="isActive || !entries.length" @click="clearEntries">清空列表</button>
        </div>
        <input ref="fileInput" class="hidden-file" type="file" accept="application/json,.json" @change="importLoadout">
      </section>
    </div>

    <section class="ui-card ui-panel entries-card">
      <h2 class="ui-section-title">因子清单 <small>{{ entries.length }} / {{ MAX_ENTRIES }}</small></h2>
      <div v-if="!entries.length" class="ui-empty empty-state">
        还没有配装记录。可以从游戏内读取，也可以导入别人分享的配装文件。
      </div>
      <div v-else class="ui-list entry-list">
        <article v-for="(entry, index) in entries" :key="`${signature(entry)}-${index}`" class="entry-row ui-row" :class="{ 'is-on': mode === 'write' && index === progress }">
          <span class="entry-index">{{ String(index + 1).padStart(2, '0') }}</span>
          <div class="entry-content">
            <img v-if="traitIcon(entry.primaryTraitHash)" class="entry-factor-icon" :src="traitIcon(entry.primaryTraitHash)" alt="" />
            <div class="entry-main">
              <strong>{{ sigilName(entry) }} <span class="ui-tag">Lv {{ entry.sigilLevel }}</span></strong>
              <div class="trait-summary">
                <span><b>主</b>{{ traitName(entry.primaryTraitHash) }} · Lv {{ entry.primaryTraitLevel }}</span>
                <span><img v-if="traitIcon(entry.secondaryTraitHash)" :src="traitIcon(entry.secondaryTraitHash)" alt="" /><b>副</b>{{ traitName(entry.secondaryTraitHash) }}<template v-if="entry.secondaryTraitHash && entry.secondaryTraitHash !== EMPTY_HASH"> · Lv {{ entry.secondaryTraitLevel }}</template></span>
              </div>
            </div>
          </div>
          <button class="ui-btn is-icon is-sm is-ghost remove" :disabled="isActive" title="移除此项" :aria-label="`移除第 ${index + 1} 个因子`" @click="removeEntry(index)">×</button>
        </article>
      </div>
    </section>
  </div>
</template>

<style scoped>
.loadout-root { padding-bottom:var(--space-9); }
.loadout-overview { display:grid; grid-template-columns:minmax(300px,.92fr) minmax(340px,1.08fr); gap:var(--space-5); align-items:stretch; }
.workflow-card,.file-card { height:100%; }
.status-strip { display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); }
.status-strip > div { display:flex; min-width:0; flex-direction:column; gap:var(--space-1); }
.status-strip span { line-height:var(--lh-normal); }
.status-strip strong { flex:0 0 auto; color:var(--text-primary); font:var(--fw-bold) var(--fs-lg)/1 var(--font-data); font-variant-numeric:tabular-nums; }
.primary-actions { margin-top:auto; }
.operation-note { display:grid; grid-template-columns:auto minmax(0,1fr); gap:var(--space-3); }
.operation-note b { color:var(--accent-hover); }
.meta-grid { grid-template-columns:minmax(140px,.55fr) minmax(210px,1fr); }
.file-actions { margin-top:auto; }
.hidden-file { display:none; }
.entry-row { display:grid; grid-template-columns:36px minmax(0,1fr) auto; align-items:center; }
.entry-index { color:var(--text-muted); text-align:center; font:var(--fw-bold) var(--fs-sm)/1 var(--font-data); }
.entry-content { display:flex; min-width:0; align-items:center; gap:var(--space-3); }
.entry-factor-icon { width:42px; height:42px; flex:0 0 42px; object-fit:cover; border:1px solid var(--line-soft); border-radius:7px; background:var(--surface-field); }
.entry-main { min-width:0; }
.entry-main > strong { display:flex; min-width:0; flex-wrap:wrap; align-items:center; gap:var(--space-2); overflow-wrap:anywhere; }
.trait-summary { display:flex; min-width:0; flex-wrap:wrap; gap:var(--space-1) var(--space-5); margin-top:var(--space-1); color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.trait-summary span { min-width:0; display:inline-flex; align-items:center; gap:var(--space-1); overflow-wrap:anywhere; }
.trait-summary img { width:22px; height:22px; flex:0 0 22px; object-fit:cover; border:1px solid var(--line-soft); border-radius:5px; background:var(--surface-field); }
.trait-summary b { margin-right:var(--space-2); color:var(--accent-hover); }
.remove { align-self:center; }

@container ui-page (max-width:680px) {
  .loadout-overview { grid-template-columns:minmax(0,1fr); }
  .meta-grid { grid-template-columns:minmax(0,1fr); }
}

@container ui-page (max-width:520px) {
  .operation-note { grid-template-columns:minmax(0,1fr); }
  .entry-row { grid-template-columns:30px minmax(0,1fr); }
  .remove { grid-column:2; justify-self:start; }
  .primary-actions .ui-btn,.file-actions .ui-btn { flex:1 1 160px; }
}
</style>
