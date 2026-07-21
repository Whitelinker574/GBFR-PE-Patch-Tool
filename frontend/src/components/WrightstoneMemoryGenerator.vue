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
import { language } from '../i18n.js'
import { backendLanguageReady } from '../backendLanguage.js'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'
import CatalogSelect from './CatalogSelect.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const EMPTY_HASH = 0x887AE0B0
const RUNTIME_LEASE_SCOPE = 'wrightstone-memory-generator'
const emit = defineEmits(['status'])
const confirmDialog = ref(null)
const loading = ref(false)
const writing = ref(false)
const stale = ref(false)
function text(zh, en) { return language.value === 'en' ? en : zh }
function isolatedError(error, englishFallback) {
  const raw = String(error || '')
  return language.value === 'en' && /[\u3400-\u9fff]/u.test(raw) ? englishFallback : raw
}
const liveMessage = ref(text('尚未启用读取。', 'Capture is not enabled.'))
const traits = ref([])
const status = reactive({
  found: false,
  hooked: false,
  captureSource: '',
  sourceVersion: '',
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
  { labelZh: '第一槽', labelEn: 'Slot One', hashKey: 'firstHash', nameKey: 'firstName', levelKey: 'firstLevel', maxLevel: 20, hash: 0, level: 0 },
  { labelZh: '第二槽', labelEn: 'Slot Two', hashKey: 'secondHash', nameKey: 'secondName', levelKey: 'secondLevel', maxLevel: 15, hash: 0, level: 0 },
  { labelZh: '第三槽', labelEn: 'Slot Three', hashKey: 'thirdHash', nameKey: 'thirdName', levelKey: 'thirdLevel', maxLevel: 10, hash: 0, level: 0 },
])
function slotLabel(slot) { return language.value === 'en' ? slot.labelEn : slot.labelZh }
const traitCatalogOptions = computed(() => traits.value
  .map(trait => ({
    ...trait,
    internalId: String(normaliseHash(trait?.hash) || ''),
    maxLevel: Number(trait?.maxLevel || 0),
  }))
  .filter(trait => trait.internalId))
let pollTimer = 0
let lastSelectedAddr = 0
let disposed = false
let lifecycleEpoch = 0
let hookOwnerToken = ''

const statusLabel = computed(() => {
  if (stale.value) return text('记录已失效', 'Record Expired')
  if (status.selectedAddr) return text('已锁定记录', 'Record Locked')
  if (status.hooked) return text('等待游戏内选择', 'Waiting for In-Game Selection')
  if (status.found) return text('已定位，尚未启用', 'Located, Not Enabled')
  return text('未连接', 'Not Connected')
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
function traitIconForOption(trait) { return traitAssetIcon({ internalId: trait?.internalId, hash: trait?.hash, name: trait?.displayName }) }
function traitCatalogValue(slot) { return slot.hash ? String(normaliseHash(slot.hash)) : '' }
function selectTrait(slot, value) {
  slot.hash = normaliseHash(value)
  normaliseLevel(slot)
}
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
    captureSource: '',
    sourceVersion: '',
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
    rows.push(language.value === 'en'
      ? `${slotLabel(slot)}: ${currentName(slot)} → ${targetName(slot)}`
      : `${slotLabel(slot)}：${currentName(slot)} → ${targetName(slot)}`)
  }
  if (Number(slot.level) !== currentLevel(slot)) {
    rows.push(language.value === 'en'
      ? `${slotLabel(slot)} Level: ${currentLevel(slot)} → ${Number(slot.level)}`
      : `${slotLabel(slot)}等级：${currentLevel(slot)} → ${Number(slot.level)}`)
  }
  return rows.map(text => ({ key: `${index}-${text}`, text }))
}))

const validationMessage = computed(() => {
  if (!status.hooked) return text('请先启用读取。', 'Enable capture first.')
  if (stale.value) return text('上次写入后记录已失效，请在游戏内重新选择记录。', 'The record expired after the last write. Select it again in-game.')
  if (!status.selectedAddr) return text('请在游戏内祝福石列表重新选中目标记录。', 'Select the target again in the in-game wrightstone list.')
  if (!traitSlots[0].hash) return text('第一槽词条不能为空。', 'Slot One cannot be empty.')
  for (const slot of traitSlots) {
    if (!slot.hash && Number(slot.level) !== 0) return language.value === 'en' ? `${slotLabel(slot)} must have level 0 when empty.` : `${slotLabel(slot)}为空时等级必须为 0。`
    const option = traitOption(slot.hash)
    if (slot.hash && !option) return language.value === 'en' ? `${slotLabel(slot)} is not in the verified catalog.` : `${slotLabel(slot)}词条不在已验证目录中。`
    if (slot.hash && !allowedLevels(slot).includes(Number(slot.level))) {
      return language.value === 'en' ? `${slotLabel(slot)} level is outside this slot's allowed range.` : `${slotLabel(slot)}等级不在该槽位的允许范围内。`
    }
  }
  const selectedHashes = traitSlots.map(slot => normaliseHash(slot.hash)).filter(Boolean)
  const uniqueHashes = new Set(traitSlots.map(slot => normaliseHash(slot.hash)).filter(Boolean))
  if (uniqueHashes.size !== selectedHashes.length) return text('三个槽位不能选择重复词条。', 'The three slots cannot use duplicate traits.')
  if (!changes.value.length) return text('目标值与当前记录相同。', 'Target values are identical to the current record.')
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
    if (!previousSelectedAddr && status.selectedAddr) liveMessage.value = text('已读取当前祝福石记录。', 'The current wrightstone record was captured.')
    else if (previousSelectedAddr && !status.selectedAddr && status.hooked) liveMessage.value = text('当前记录已释放，请在游戏内重新选择祝福石。', 'The current record was released. Select the wrightstone again in-game.')
    else if (!silent) liveMessage.value = status.selectedAddr
      ? text('已读取当前祝福石记录。', 'The current wrightstone record was captured.')
      : text('等待游戏内选择祝福石记录。', 'Waiting for an in-game wrightstone selection.')
  } catch (error) {
    if (disposed || epoch !== lifecycleEpoch) return
    stopPolling()
    applyStatus(null)
    liveMessage.value = language.value === 'en'
      ? `Capture stopped: ${isolatedError(error, 'The runtime capture status could not be read.')}`
      : `读取已停止：${String(error)}`
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
    if (!acquiredOwnerToken) throw new Error(text('后端未返回祝福石读取所有权令牌', 'The backend did not return a wrightstone capture owner token.'))
    if (disposed || epoch !== lifecycleEpoch) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, WrightstoneMemoryRelease)
      return
    }
    hookOwnerToken = acquiredOwnerToken
    applyStatus(next, { sync: true })
    liveMessage.value = status.selectedAddr
      ? text('读取已启用，并已捕获一条记录。', 'Capture is enabled and one record has been captured.')
      : text('读取已启用，请在游戏内选择一条祝福石记录。', 'Capture is enabled. Select a wrightstone record in-game.')
    emit('status', liveMessage.value, 'success')
    startPolling()
  } catch (error) {
    if (disposed || epoch !== lifecycleEpoch) return
    liveMessage.value = isolatedError(error, 'Failed to enable wrightstone capture.')
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
    liveMessage.value = text('读取已停止，游戏指令已恢复。', 'Capture stopped and the game instruction was restored.')
    emit('status', liveMessage.value, 'success')
  } catch (error) {
    if (disposed || epoch !== lifecycleEpoch) return
    liveMessage.value = isolatedError(error, 'Failed to stop wrightstone capture.')
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
    title: text('写入当前祝福石', 'Write Current Wrightstone'),
    message: language.value === 'en' ? `Write ${changes.value.length} changes?` : `确认写入 ${changes.value.length} 项变更？`,
    detail: changeDetail,
    tone: 'warning',
    confirmLabel: text('确认写入', 'Confirm Write'),
  })
  if (!confirmed) return
  writing.value = true
  try {
    const ownerToken = hookOwnerToken
    if (!ownerToken) throw new Error(text('当前页面不再持有祝福石读取所有权', 'This page no longer owns the wrightstone capture lease.'))
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
      liveMessage.value = text('写入成功，读取已自动停止，游戏指令已恢复。', 'Write succeeded. Capture stopped automatically and the game instruction was restored.')
      emit('status', liveMessage.value, 'success')
    } catch (releaseError) {
      if (disposed) return
      liveMessage.value = language.value === 'en'
        ? `Write succeeded, but automatic capture shutdown failed. Click “Stop Capture” now: ${isolatedError(releaseError, 'runtime restore failed')}`
        : `写入已成功，但自动停止读取失败；请立即点击“停止读取”：${String(releaseError)}`
      emit('status', liveMessage.value, 'error')
    }
  } catch (error) {
    liveMessage.value = isolatedError(error, 'Failed to write the wrightstone record.')
    emit('status', liveMessage.value, 'error')
  } finally {
    writing.value = false
  }
}

onMounted(async () => {
  loading.value = true
  try {
    await backendLanguageReady
    if (disposed) return
    const result = await WrightstoneMemoryGetOptions()
    traits.value = Array.isArray(result?.traits) ? result.traits : []
    await pollStatus(true)
    if (status.hooked) startPolling()
  } catch (error) {
    liveMessage.value = isolatedError(error, 'Failed to load the wrightstone catalog.')
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
      <section class="connection-card section ui-card">
        <header class="ui-split">
          <div>
            <h2 class="ui-section-title">{{ text('捕获状态', 'Capture Status') }}</h2>
            <p class="ui-section-copy">{{ text('捕获游戏内当前选中的祝福石记录，核对三槽后一次性写入。', 'Capture the currently selected in-game wrightstone, verify all three slots, then write once.') }}</p>
          </div>
          <span class="ui-tag" :class="status.selectedAddr && !stale ? 'is-ok' : status.hooked ? 'is-info' : ''">{{ statusLabel }}</span>
        </header>
        <div class="connection-body">
          <div class="connection-actions ui-actions">
            <button type="button" class="ui-btn is-primary" :disabled="loading || status.hooked" @click="enable">{{ text('启用读取', 'Enable Capture') }}</button>
            <button type="button" class="ui-btn" :disabled="loading || !status.hooked" @click="pollStatus(false)">{{ text('刷新状态', 'Refresh Status') }}</button>
            <button type="button" class="ui-btn is-ghost" :disabled="loading || !status.hooked" @click="disable">{{ text('停止读取', 'Stop Capture') }}</button>
          </div>
          <p class="connection-message ui-notice" :class="{ 'is-warn': stale }" aria-live="polite">{{ liveMessage }}</p>
        </div>
        <p class="ui-hint">{{ text('读取 Hook 启用期间不要切入铁匠铺的其他操作；写入成功后会自动停止读取，避免游戏闪退。', 'Do not switch to other blacksmith actions while the capture hook is active. Capture stops automatically after a successful write to avoid crashes.') }}</p>
        <p v-if="status.sourceVersion" class="ui-hint capture-source">{{ text(`捕获来源：CT ${status.sourceVersion} 当前查看项（2.0.2 完整指令守卫）`, `Capture source: CT ${status.sourceVersion} current-view entry (full 2.0.2 instruction guard)`) }}</p>
      </section>

      <section class="record-panel section ui-card">
        <header class="ui-split">
          <div>
            <h3 class="ui-section-title">{{ text('三槽记录', 'Three-Slot Record') }}</h3>
            <p class="ui-section-copy">{{ text('当前值保持只读；目标值只有在重新捕获记录后才可写。', 'Current values remain read-only. Target values become writable only after a fresh capture.') }}</p>
          </div>
          <code>{{ status.selectedAddr ? `0x${Number(status.selectedAddr).toString(16).toUpperCase()}` : text('未选择', 'Not Selected') }}</code>
        </header>

        <div v-if="!status.selectedAddr || stale" class="selection-inline-notice" role="status">
          <span>{{ stale
            ? text('写入后的旧记录不可复用，请在游戏内重新选中一条记录。', 'The old record cannot be reused after writing. Select a record again in-game.')
            : text('启用读取后，在游戏内祝福石列表选中目标记录。', 'After enabling capture, select the target in the in-game wrightstone list.') }}</span>
        </div>

        <div class="trait-grid" :aria-disabled="!status.selectedAddr || stale">
          <article v-for="(slot, index) in traitSlots" :key="slot.hashKey" class="trait-card ui-card">
            <header class="trait-card-heading ui-split"><h4 class="ui-section-title">{{ slotLabel(slot) }}</h4><span class="ui-tag">{{ index + 1 }} / 3</span></header>
            <div class="trait-current">
              <span class="trait-current-icon">
                <img v-if="traitIcon(currentHash(slot), currentName(slot))" :src="traitIcon(currentHash(slot), currentName(slot))" alt="" />
                <span v-else aria-hidden="true">—</span>
              </span>
              <span class="trait-current-copy">
                <small>{{ text('当前配置', 'Current Configuration') }}</small>
                <strong>{{ currentName(slot) }}</strong>
              </span>
              <code>Lv {{ currentLevel(slot) }} · {{ formatHex(currentHash(slot)) }}</code>
            </div>
            <div class="trait-target">
              <label class="ui-field">
                <span class="ui-field-label">{{ text('目标词条', 'Target Trait') }}</span>
                <CatalogSelect
                  :model-value="traitCatalogValue(slot)"
                  :options="traitCatalogOptions"
                  :icon-resolver="traitIconForOption"
                  :optional="index > 0"
                  :disabled="!status.selectedAddr || stale"
                  :placeholder="text('尚未选择特性', 'No Trait Selected')"
                  :search-placeholder="text('搜索特性名称', 'Search Traits')"
                  detail-key="maxLevel"
                  @update:model-value="selectTrait(slot, $event)"
                />
              </label>
              <label class="ui-field trait-level-field">
                <span class="ui-field-label">{{ text('目标等级', 'Target Level') }}</span>
                <select v-model.number="slot.level" class="ui-select ui-input" :disabled="!slot.hash || !status.selectedAddr || stale">
                  <option v-for="level in allowedLevels(slot)" :key="level" :value="level">Lv {{ level }}</option>
                </select>
              </label>
            </div>
          </article>
        </div>

        <div class="record-footer">
          <details class="ui-disclosure change-summary">
            <summary>{{ text('变更摘要', 'Change Summary') }} · {{ changes.length }} {{ text('项', 'changes') }}</summary>
            <ul v-if="changes.length"><li v-for="item in changes" :key="item.key">{{ item.text }}</li></ul>
            <p v-else class="ui-hint">{{ text('当前没有待写入变更。', 'There are no pending changes.') }}</p>
          </details>

          <aside class="wrightstone-memory-actions ui-card ui-panel is-compact">
            <span><b>{{ statusLabel }}</b><small>{{ validationMessage || (language === 'en' ? `${changes.length} changes pending` : `${changes.length} 项待写入`) }}</small></span>
            <button type="button" class="ui-btn" :disabled="!status.selectedAddr || stale" @click="syncDraftFromStatus">{{ text('还原当前值', 'Restore Current Values') }}</button>
            <button type="button" class="ui-btn is-primary" :disabled="!canWrite" @click="write">{{ writing ? text('写入中…', 'Writing…') : text('写入三槽', 'Write Three Slots') }}</button>
          </aside>
        </div>
      </section>
    </div>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.wrightstone-memory-page { min-height:0; padding:0; }
.wrightstone-memory-content { width:100%; max-width:1000px; min-height:0; padding:0 var(--space-1) var(--space-8) 0; }
.wrightstone-memory-content > * + * { margin-top:var(--space-4); }
.section { min-width:0; padding:var(--space-6); }
.connection-card,.record-panel { background:var(--surface-card); }
.connection-body { display:grid; grid-template-columns:auto minmax(240px,1fr); gap:var(--space-4); align-items:center; margin-top:var(--space-4); }
.connection-message { min-width:0; margin:0; }
.connection-card > .ui-hint { margin-top:var(--space-3); }
.record-panel > header code { color:var(--text-muted); font-family:var(--font-data); }
.selection-inline-notice {
  min-height:0;
  display:flex;
  align-items:center;
  margin-top:var(--space-3);
  padding:var(--space-2) var(--space-3);
  border-left:3px solid var(--accent);
  color:var(--text-secondary);
  background:var(--surface-field);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
  text-align:left;
}
.trait-grid {
  display:grid;
  grid-template-columns:repeat(auto-fit,minmax(min(100%,240px),1fr));
  gap:var(--space-3);
  margin-top:var(--space-4);
}
.trait-card {
  display:flex;
  min-width:0;
  flex-direction:column;
  gap:var(--space-4);
  padding:var(--space-4);
  background:var(--surface-card-pop);
  box-shadow:none;
}
.trait-card-heading { align-items:center; }
.trait-current {
  display:grid;
  grid-template-columns:38px minmax(0,1fr) auto;
  gap:var(--space-3);
  align-items:center;
  padding:var(--space-3);
  border:1px solid var(--border-soft);
  border-radius:var(--radius-sm);
  background:var(--surface-card-pop);
}
.trait-current-icon { display:grid; width:38px; height:38px; place-items:center; border:1px solid var(--line-soft); border-radius:7px; background:var(--surface-field); color:var(--text-muted); font-family:var(--font-data); }
.trait-current-icon img { width:100%; height:100%; object-fit:cover; border-radius:6px; }
.trait-current-copy { display:flex; min-width:0; flex-direction:column; gap:1px; }
.trait-current small { color:var(--text-muted); font-size:var(--fs-xs); }
.trait-current strong { min-width:0; overflow:hidden; color:var(--text-primary); text-overflow:ellipsis; white-space:nowrap; }
.trait-current code { color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-sm); overflow-wrap:normal; text-align:right; white-space:nowrap; }
.trait-target { display:grid; grid-template-columns:minmax(0,1fr) 104px; gap:var(--space-3); align-items:end; }
.trait-level-field { width:100%; }
.record-footer { display:grid; grid-template-columns:minmax(0,1fr) minmax(300px,.8fr); gap:var(--space-4); align-items:stretch; margin-top:var(--space-4); padding-top:var(--space-4); border-top:1px solid var(--border-soft); }
.change-summary { height:100%; }
.change-summary ul { display:grid; gap:var(--space-2); margin:0; padding-left:var(--space-5); color:var(--text-secondary); }
.wrightstone-memory-actions { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); align-content:center; border-color:var(--border-default); border-top:3px solid var(--accent); background:var(--surface-card-pop); box-shadow:none; font-family:var(--font-data); }
.wrightstone-memory-actions span { display:grid; min-width:0; grid-column:1 / -1; gap:var(--space-1); }
.wrightstone-memory-actions small { color:var(--text-muted); overflow-wrap:anywhere; }

@container ui-page (max-width:760px) {
  .connection-body,.record-footer { grid-template-columns:minmax(0,1fr); }
  .trait-card:last-child { grid-column:1 / -1; }
}
@container ui-page (max-width:620px) {
  .section { padding:var(--space-4); }
  .trait-target { grid-template-columns:minmax(0,1fr); }
}
@container ui-page (max-width:440px) {
  .connection-actions,.wrightstone-memory-actions { display:grid; grid-template-columns:minmax(0,1fr); }
  .wrightstone-memory-actions span { grid-column:auto; }
  .wrightstone-memory-actions .ui-btn { width:100%; }
}
</style>
