<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  SigilMemoryEnable,
  SigilMemoryGetOptions,
  SigilMemoryGetStatus,
  SigilMemoryUpdate,
} from '../../wailsjs/go/main/App'

const emit = defineEmits(['status'])
const FORMAT = 'gbfr-sigil-loadout'
const VERSION = 1
const MAX_ENTRIES = 12
const EMPTY_HASH = 0x887AE0B0
const POLL_DELAY = 120

const mode = ref('idle')
const entries = ref([])
const options = reactive({ sigils: [], traits: [] })
const loadoutVersion = ref('DLC 2.0.2')
const comment = ref('')
const progress = ref(0)
const lastSignature = ref('')
const seen = new Set()
const fileInput = ref(null)
let pollTimer = 0
let polling = false
let disposed = false

const isActive = computed(() => mode.value !== 'idle')
const modeLabel = computed(() => mode.value === 'record' ? '正在记录' : mode.value === 'write' ? '正在复刻' : '等待操作')
const progressLabel = computed(() => mode.value === 'write'
  ? `${Math.min(progress.value, entries.value.length)} / ${entries.value.length}`
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
function formatHash(value) { return `0x${uint(value).toString(16).toUpperCase().padStart(8, '0')}` }
function sigilName(entry) { return sigilNames.value.get(uint(entry.sigilHash)) || formatHash(entry.sigilHash) }
function traitName(hash) {
  const value = uint(hash)
  if (!value || value === EMPTY_HASH) return '无副词条'
  return traitNames.value.get(value) || formatHash(value)
}

function stop(message = '') {
  window.clearTimeout(pollTimer)
  pollTimer = 0
  polling = false
  mode.value = 'idle'
  lastSignature.value = ''
  if (message) showStatus(message, 'success')
}

async function poll() {
  if (disposed || mode.value === 'idle' || polling) return
  polling = true
  try {
    const status = await SigilMemoryGetStatus()
    if (!isSelectable(status)) return
    const currentSignature = signature(status)
    if (currentSignature === lastSignature.value) return

    if (mode.value === 'record') {
      if (seen.has(currentSignature)) {
        lastSignature.value = currentSignature
        return
      }
      const entry = normalizeEntry(status)
      entries.value = [...entries.value, entry]
      seen.add(currentSignature)
      lastSignature.value = currentSignature
      progress.value = entries.value.length
      if (entries.value.length >= MAX_ENTRIES) stop('已记录完整的 12 个因子')
    } else if (mode.value === 'write') {
      const target = entries.value[progress.value]
      if (!target) {
        stop('因子配装复刻完成')
        return
      }
      const updated = await SigilMemoryUpdate({ ...target })
      progress.value += 1
      lastSignature.value = signature(updated || target)
      if (progress.value >= entries.value.length) stop(`已复刻 ${entries.value.length} 个因子`)
    }
  } catch (error) {
    stop()
    showStatus(String(error), 'error')
  } finally {
    polling = false
    if (!disposed && mode.value !== 'idle') pollTimer = window.setTimeout(poll, POLL_DELAY)
  }
}

async function startRecord() {
  stop()
  entries.value = []
  progress.value = 0
  seen.clear()
  try {
    const status = await SigilMemoryEnable()
    mode.value = 'record'
    lastSignature.value = ''
    showStatus('记录已开始：从第一个因子开始逐项移动选择', 'success')
    // Enable returns the item that is already selected.  Capture it before
    // polling; otherwise the first row would be skipped when the user starts
    // recording while already resting on row 1.
    if (isSelectable(status)) {
      const entry = normalizeEntry(status)
      const currentSignature = signature(entry)
      entries.value = [entry]
      seen.add(currentSignature)
      lastSignature.value = currentSignature
      progress.value = 1
    }
    pollTimer = window.setTimeout(poll, POLL_DELAY)
  } catch (error) { showStatus(String(error), 'error') }
}

async function startWrite() {
  if (!entries.value.length) {
    showStatus('请先记录或导入一份因子配装', 'error')
    return
  }
  stop()
  progress.value = 0
  try {
    const status = await SigilMemoryEnable()
    mode.value = 'write'
    lastSignature.value = ''
    showStatus('复刻已开始：停在第一件备用因子上，再逐项向下移动', 'success')
    // The current row is the first write target, so apply entry 1 immediately
    // and then wait for a changed selection before applying entry 2.
    if (isSelectable(status)) {
      const target = entries.value[0]
      const updated = await SigilMemoryUpdate({ ...target })
      progress.value = 1
      lastSignature.value = signature(updated || target)
      if (entries.value.length === 1) {
        stop('已复刻 1 个因子')
        return
      }
    }
    pollTimer = window.setTimeout(poll, POLL_DELAY)
  } catch (error) { showStatus(String(error), 'error') }
}

function removeEntry(index) {
  if (isActive.value) return
  entries.value = entries.value.filter((_, i) => i !== index)
  seen.clear()
  entries.value.forEach(item => seen.add(signature(item)))
}

function clearEntries() {
  if (isActive.value) return
  entries.value = []
  progress.value = 0
  seen.clear()
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
  if (!file) return
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
    seen.clear()
    entries.value.forEach(item => seen.add(signature(item)))
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
onBeforeUnmount(() => { disposed = true; stop() })
</script>

<template>
  <div class="loadout-root">
    <section class="section workflow-card">
      <div class="section-title"><span>配装读取</span><small>最多记录 12 个因子</small></div>
      <div class="status-strip" :class="mode">
        <div><b>{{ modeLabel }}</b><small>{{ mode === 'record' ? '在游戏因子列表中逐项向下移动' : mode === 'write' ? '每次移动到下一件备用因子都会写入' : '启动游戏并进入因子列表后开始' }}</small></div>
        <strong>{{ progressLabel }}</strong>
      </div>
      <div class="primary-actions">
        <button class="action primary" :disabled="mode === 'write'" @click="mode === 'record' ? stop('已停止记录') : startRecord()">
          {{ mode === 'record' ? '停止记录' : '记录当前配装' }}
        </button>
        <button class="action primary" :disabled="mode === 'record' || !entries.length" @click="mode === 'write' ? stop('已停止复刻') : startWrite()">
          {{ mode === 'write' ? '停止复刻' : '复刻到备用因子' }}
        </button>
      </div>
      <div class="operation-note"><b>操作顺序</b><span>先装备目标角色的 12 个因子，在物品列表按角色筛选；记录或复刻时从第一项开始，逐项向下移动，不要快速滚动。</span></div>
    </section>

    <section class="section file-card">
      <div class="section-title"><span>配装文件</span><small>可分享 JSON 文件</small></div>
      <div class="meta-grid">
        <label><span>适用版本</span><input v-model="loadoutVersion" :disabled="isActive" maxlength="80"></label>
        <label><span>备注</span><input v-model="comment" :disabled="isActive" maxlength="160" placeholder="角色、用途或配装说明"></label>
      </div>
      <div class="file-actions">
        <button class="action" :disabled="isActive" @click="openImport">导入配装</button>
        <button class="action" :disabled="isActive || !entries.length" @click="exportLoadout">导出配装</button>
        <button class="text-action" :disabled="isActive || !entries.length" @click="clearEntries">清空列表</button>
      </div>
      <input ref="fileInput" class="hidden-file" type="file" accept="application/json,.json" @change="importLoadout">
    </section>

    <section class="section entries-card">
      <div class="section-title"><span>因子清单</span><small>{{ entries.length }} / {{ MAX_ENTRIES }}</small></div>
      <div v-if="!entries.length" class="empty-state">
        还没有配装记录。可以从游戏内读取，也可以导入别人分享的配装文件。
      </div>
      <div v-else class="entry-list">
        <article v-for="(entry, index) in entries" :key="`${signature(entry)}-${index}`" class="entry-row" :class="{ current: mode === 'write' && index === progress }">
          <span class="entry-index">{{ String(index + 1).padStart(2, '0') }}</span>
          <div class="entry-main">
            <strong>{{ sigilName(entry) }} <em>Lv.{{ entry.sigilLevel }}</em></strong>
            <small>{{ traitName(entry.primaryTraitHash) }} Lv.{{ entry.primaryTraitLevel }} · {{ traitName(entry.secondaryTraitHash) }}<template v-if="entry.secondaryTraitHash && entry.secondaryTraitHash !== EMPTY_HASH"> Lv.{{ entry.secondaryTraitLevel }}</template></small>
          </div>
          <button class="remove" :disabled="isActive" title="移除此项" @click="removeEntry(index)">×</button>
        </article>
      </div>
    </section>
  </div>
</template>

<style scoped>
.loadout-root{display:flex;flex-direction:column;gap:12px;width:100%;color:#574936}.section{padding:14px;border:1px solid rgba(132,94,43,.28);border-radius:2px;background:#f6e8c9}.section-title{display:flex;align-items:baseline;justify-content:space-between;gap:12px;margin-bottom:11px;padding-bottom:8px;border-bottom:1px solid rgba(132,94,43,.22);font-weight:700}.section-title small{color:#8d7758;font-size:10px}.status-strip{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:11px 13px;border-left:3px solid #98703a;background:#edddba}.status-strip>div{display:flex;flex-direction:column;gap:3px}.status-strip b{font-size:12px}.status-strip small{color:#796950;font-size:10px}.status-strip strong{flex:0 0 auto;color:#78552c;font:750 15px/1 var(--font-data);font-variant-numeric:tabular-nums}.status-strip.record,.status-strip.write{border-left-color:#7b6041;background:#ead5a8}.primary-actions,.file-actions{display:flex;align-items:center;gap:8px;margin-top:10px}.action{min-height:34px;padding:7px 13px;border:1px solid rgba(126,91,42,.38);border-radius:1px;color:#5e4b32;background:#edddba;cursor:pointer}.action.primary{flex:1;color:#fff9e8;border-color:#765126;background:#8b6737}.action:hover:not(:disabled){box-shadow:inset 3px 0 #67451f}.action:disabled,.text-action:disabled{cursor:not-allowed;opacity:.45}.operation-note{display:grid;grid-template-columns:auto 1fr;gap:9px;margin-top:10px;padding-top:9px;border-top:1px solid rgba(132,94,43,.18);font-size:10px;line-height:1.55}.operation-note b{color:#765126}.operation-note span{color:#71614b}.meta-grid{display:grid;grid-template-columns:minmax(150px,.55fr) minmax(220px,1fr);gap:10px}.meta-grid label{display:flex;flex-direction:column;gap:5px}.meta-grid label span{font-size:10px;font-weight:650}.meta-grid input{width:100%;min-height:34px;padding:7px 9px;border:1px solid rgba(126,91,42,.34);border-radius:1px;color:#574936;background:#fff9e8;outline:0}.text-action{margin-left:auto;padding:6px;border:0;color:#80613b;background:transparent;cursor:pointer}.hidden-file{display:none}.entry-list{border:1px solid rgba(126,91,42,.22)}.entry-row{display:grid;grid-template-columns:38px minmax(0,1fr) 30px;align-items:center;min-height:52px;border-bottom:1px solid rgba(126,91,42,.22);background:#f8edcf}.entry-row:nth-child(even){background:#f0dfba}.entry-row:last-child{border-bottom:0}.entry-row.current{box-shadow:inset 3px 0 #8b6737;background:#ead5a8}.entry-index{color:#8b6b41;text-align:center;font:750 11px/1 var(--font-data)}.entry-main{min-width:0;padding:7px 0}.entry-main strong,.entry-main small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.entry-main strong{font-size:11px}.entry-main strong em{color:#81613a;font:700 10px/1 var(--font-data);font-style:normal}.entry-main small{margin-top:4px;color:#786951;font-size:9.5px}.remove{width:26px;height:28px;border:0;color:#8b6a43;background:transparent;cursor:pointer;font-size:16px}.remove:hover:not(:disabled){color:#844432;background:rgba(132,68,50,.08)}.empty-state{padding:24px 14px;border:1px dashed rgba(126,91,42,.28);color:#8a775c;background:#f8edcf;text-align:center;font-size:10.5px;line-height:1.65}@media(max-width:640px){.meta-grid{grid-template-columns:1fr}.primary-actions{flex-direction:column}.primary-actions .action{width:100%}.operation-note{grid-template-columns:1fr}}
</style>
