<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { SigilMemoryGetOptions, SigilMemoryGetStatus, SigilMemoryEnable, SigilMemoryUpdate } from '../../wailsjs/go/main/App'
import { matchText } from '../utils/matchText.js'
import { backendLanguageReady } from '../backendLanguage'
import { clearHistory, deleteTemplate, history, pushHistory, renameTemplate, saveTemplate, templates } from '../utils/sigilMemoryStore.js'
import SigilMemoryPicker from './SigilMemoryPicker.vue'
import LegalityIndicator from './LegalityIndicator.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const emit = defineEmits(['status'])

const status = reactive({
  found: false, hooked: false, selectedAddr: 0,
  sigilHash: 0, sigilLevel: 0, sigilName: '',
  primaryTraitHash: 0, primaryTraitLevel: 0, primaryTraitName: '',
  secondaryTraitHash: 0, secondaryTraitLevel: 0, secondaryTraitName: '',
})

const form = reactive({
  sigilHash: 0, sigilLevel: 0,
  primaryTraitHash: 0, primaryTraitLevel: 0,
  secondaryTraitHash: 0, secondaryTraitLevel: 0,
})

const backendOptions = reactive({ sigils: [], traits: [] })
const runtimeOptions = reactive({ sigils: new Map(), traits: new Map() })

const allSigilOptions = computed(() => [...backendOptions.sigils, ...runtimeOptions.sigils.values()])
const allTraitOptions = computed(() => [...backendOptions.traits, ...runtimeOptions.traits.values()])

const sigilByHash = computed(() => new Map(allSigilOptions.value.map(o => [o.hash >>> 0, o])))
const traitByHash = computed(() => new Map(allTraitOptions.value.map(o => [o.hash >>> 0, o])))

const loading = ref(false)
const applying = ref(false)
const templateSearch = ref('')
const tab = ref('templates')
const renamingId = ref(null)
const renameBuffer = ref('')
const confirmDialog = ref(null)

function show(msg, type) { emit('status', msg, type) }
function hex(v) { return '0x' + (Number(v) >>> 0).toString(16).toUpperCase().padStart(8, '0') }
const HEX_RE = /^0x[0-9A-F]{8}$/i
function isRawHexName(n) { return typeof n === 'string' && HEX_RE.test(n.trim()) }

function ensureRuntimeOption(bucket, hash, name) {
  if (!hash) return
  const h = hash >>> 0
  if (bucket.has(h)) return
  const backendList = bucket === runtimeOptions.sigils ? backendOptions.sigils : backendOptions.traits
  if (backendList.some(o => (o.hash >>> 0) === h)) return
  bucket.set(h, {
    hash: h,
    displayName: name && !isRawHexName(name) ? name : `未知 · ${hex(h)}`,
    source: 'runtime',
  })
}

function applyStatus(next) {
  Object.assign(status, next)
  ensureRuntimeOption(runtimeOptions.sigils, next.sigilHash, next.sigilName)
  ensureRuntimeOption(runtimeOptions.traits, next.primaryTraitHash, next.primaryTraitName)
  ensureRuntimeOption(runtimeOptions.traits, next.secondaryTraitHash, next.secondaryTraitName)
}

function syncFormFromStatus() {
  form.sigilHash = status.sigilHash >>> 0
  form.sigilLevel = status.sigilLevel >>> 0
  form.primaryTraitHash = status.primaryTraitHash >>> 0
  form.primaryTraitLevel = status.primaryTraitLevel >>> 0
  form.secondaryTraitHash = status.secondaryTraitHash >>> 0
  form.secondaryTraitLevel = status.secondaryTraitLevel >>> 0
}

async function loadOptions() {
  try {
    await backendLanguageReady
    const res = await SigilMemoryGetOptions()
    backendOptions.sigils = res.sigils || []
    backendOptions.traits = res.traits || []
  } catch (e) { show('读取因子数据失败: ' + String(e), 'error') }
}

async function refresh(syncForm = false) {
  loading.value = true
  try {
    applyStatus(await SigilMemoryGetStatus())
    if (syncForm) syncFormFromStatus()
    if (!status.hooked) show('已就绪。启用读取后，在游戏内选中因子。', 'success')
    else if (!status.selectedAddr) show('等待游戏内因子选择。', 'success')
    else show(`已读取: ${status.sigilName}`, 'success')
  } catch (e) { show(String(e), 'error') }
  finally { loading.value = false }
}

async function enable() {
  loading.value = true
  try {
    applyStatus(await SigilMemoryEnable())
    syncFormFromStatus()
    show('已启用。请在游戏内选择一个因子。', 'success')
  } catch (e) { show(String(e), 'error') }
  finally { loading.value = false }
}

async function performWrite() {
  if (!status.hooked || !status.selectedAddr) { show('请先启用读取，并在游戏内选中一个因子', 'error'); return }
  applying.value = true
  try {
    const snapshot = { ...form }
    applyStatus(await SigilMemoryUpdate({ ...form }))
    pushHistory(snapshot)
    show(`已写入: ${status.sigilName}`, 'success')
  } catch (e) { show(String(e), 'error') }
  finally { applying.value = false }
}
async function write() { await performWrite() }

async function oneClickMax() {
  if (!status.hooked || !status.selectedAddr) { show('请先启用读取，并在游戏内选中一个因子', 'error'); return }
  if (sigilMax.value != null) form.sigilLevel = sigilMax.value
  if (form.primaryTraitHash) form.primaryTraitLevel = 15
  if (form.secondaryTraitHash) form.secondaryTraitLevel = 15
}

function onPickSigil(opt) {
  if (opt && opt.maxLevel != null) form.sigilLevel = opt.maxLevel
  else if (!opt) form.sigilLevel = 0
}
function onPickPrimary(opt) {
  if (opt) form.primaryTraitLevel = 15
  else if (!opt) form.primaryTraitLevel = 0
}
function onPickSecondary(opt) {
  if (opt) form.secondaryTraitLevel = 15
  else if (!opt) form.secondaryTraitLevel = 0
}

const sigilMax = computed(() => sigilByHash.value.get(form.sigilHash)?.maxLevel ?? null)
// Primary/secondary max come from the picked trait itself, not the sigil's default trait cap.
// Prior bug: primaryMax fell back to sigil.firstTraitMaxLevel even when the picked trait had its own maxLevel,
// so switching to a memory-only trait still showed the sigil's default primary max.
const primaryMax = computed(() => form.primaryTraitHash ? 15 : null)
const secondaryMax = computed(() => form.secondaryTraitHash ? 15 : null)
const sigilWritableMax = computed(() => form.sigilHash ? 50 : null)
const primaryWritableMax = computed(() => form.primaryTraitHash ? (traitByHash.value.get(form.primaryTraitHash)?.maxLevel ?? 50) : null)
const secondaryWritableMax = computed(() => form.secondaryTraitHash ? (traitByHash.value.get(form.secondaryTraitHash)?.maxLevel ?? 50) : null)

function clampLevel(value, max) {
  if (max == null) return 0
  const numeric = Number.isFinite(Number(value)) ? Number(value) : 0
  return Math.min(max, Math.max(0, Math.trunc(numeric)))
}

const sigilAtMax = computed(() => sigilMax.value != null && form.sigilLevel === sigilMax.value)
const primaryAtMax = computed(() => primaryMax.value != null && form.primaryTraitLevel === primaryMax.value)
const secondaryAtMax = computed(() => secondaryMax.value != null && form.secondaryTraitLevel === secondaryMax.value)

function maxSigil() { if (sigilMax.value != null) form.sigilLevel = sigilMax.value }
function maxPrimary() { if (primaryMax.value != null) form.primaryTraitLevel = primaryMax.value }
function maxSecondary() { if (secondaryMax.value != null) form.secondaryTraitLevel = secondaryMax.value }

const canOneClickMax = computed(() => !!status.selectedAddr && (
  (sigilMax.value != null && !sigilAtMax.value) ||
  (primaryMax.value != null && !primaryAtMax.value) ||
  (secondaryMax.value != null && !!form.secondaryTraitHash && !secondaryAtMax.value)
))

const warnings = computed(() => {
  const out = []
  const sigil = sigilByHash.value.get(form.sigilHash)
  const primary = traitByHash.value.get(form.primaryTraitHash)
  const secondary = traitByHash.value.get(form.secondaryTraitHash)
  if (Array.isArray(sigil?.allowedLevels) && sigil.allowedLevels.length && !sigil.allowedLevels.includes(form.sigilLevel)) out.push('因子等级不是已知自然等级')
  else if (sigilMax.value != null && form.sigilLevel > sigilMax.value) out.push(`因子等级超过上限 ${sigilMax.value}`)
  if (Array.isArray(primary?.allowedLevels) && primary.allowedLevels.length && !primary.allowedLevels.includes(form.primaryTraitLevel)) out.push('主词条等级不是已知自然等级')
  else if (primaryMax.value != null && form.primaryTraitLevel > primaryMax.value) out.push(`主词条等级超过上限 ${primaryMax.value}`)
  if (!form.secondaryTraitHash && form.secondaryTraitLevel) out.push('未选择副词条，但副词条等级不为 0')
  else if (Array.isArray(secondary?.allowedLevels) && secondary.allowedLevels.length && !secondary.allowedLevels.includes(form.secondaryTraitLevel)) out.push('副词条等级不是已知自然等级')
  else if (secondaryMax.value != null && form.secondaryTraitLevel > secondaryMax.value) out.push(`副词条等级超过上限 ${secondaryMax.value}`)
  if (sigilWritableMax.value != null && form.sigilLevel > sigilWritableMax.value) out.push(`因子等级超过修改上限 ${sigilWritableMax.value}`)
  if (primaryWritableMax.value != null && form.primaryTraitLevel > primaryWritableMax.value) out.push(`主词条等级超过修改上限 ${primaryWritableMax.value}`)
  if (secondaryWritableMax.value != null && form.secondaryTraitLevel > secondaryWritableMax.value) out.push(`副词条等级超过修改上限 ${secondaryWritableMax.value}`)
  if (form.primaryTraitHash && form.secondaryTraitHash && (form.primaryTraitHash >>> 0) === (form.secondaryTraitHash >>> 0)) {
    out.push(`主词条「${primary?.displayName || hex(form.primaryTraitHash)}」与副词条「${secondary?.displayName || hex(form.secondaryTraitHash)}」重复冲突`)
  }
  if (form.secondaryTraitHash && sigil && sigil.supportsSecondaryTrait === false) {
    out.push('该因子不支持副词条')
  } else if (
    form.secondaryTraitHash && sigil &&
    Array.isArray(sigil.allowedSecondaryTraitHashes) && sigil.allowedSecondaryTraitHashes.length > 0 &&
    !sigil.allowedSecondaryTraitHashes.map(h => h >>> 0).includes(form.secondaryTraitHash >>> 0)
  ) {
    out.push('副词条不在该因子允许名单中')
  }
  return out
})

const legality = computed(() => {
  if (!status.selectedAddr) return { status: 'impossible', message: '请先在游戏内选中一个因子' }
  if (!form.sigilHash || !form.primaryTraitHash) return { status: 'impossible', message: '因子和主词条不能为空' }
  if (sigilWritableMax.value != null && form.sigilLevel > sigilWritableMax.value) return { status: 'impossible', message: `因子等级修改上限是 ${sigilWritableMax.value}` }
  if (primaryWritableMax.value != null && form.primaryTraitLevel > primaryWritableMax.value) return { status: 'impossible', message: `主词条等级修改上限是 ${primaryWritableMax.value}` }
  if (secondaryWritableMax.value != null && form.secondaryTraitLevel > secondaryWritableMax.value) return { status: 'impossible', message: `副词条等级修改上限是 ${secondaryWritableMax.value}` }
  const reasons = [...warnings.value]
  if (!sigilByHash.value.has(form.sigilHash >>> 0)) reasons.push('因子 Hash 不在本地资料库中')
  if (!traitByHash.value.has(form.primaryTraitHash >>> 0)) reasons.push('主词条 Hash 不在本地资料库中')
  if (form.secondaryTraitHash && !traitByHash.value.has(form.secondaryTraitHash >>> 0)) reasons.push('副词条 Hash 不在本地资料库中')
  if (reasons.length) return { status: 'forced', message: `${reasons.join('；')}；仍会按所选数值写入` }
  const sigil = sigilByHash.value.get(form.sigilHash >>> 0)
  if (form.secondaryTraitHash && (!sigil || !Array.isArray(sigil.allowedSecondaryTraitHashes) || !sigil.allowedSecondaryTraitHashes.length)) {
    return { status: 'unknown', message: '可写入；该因子的完整天然副词条池尚未完全验证' }
  }
  return { status: 'legal', message: '符合当前已验证的因子、词条与等级规则' }
})

const changedCount = computed(() => {
  let n = 0
  if ((form.sigilHash >>> 0) !== (status.sigilHash >>> 0)) n++
  if ((form.sigilLevel >>> 0) !== (status.sigilLevel >>> 0)) n++
  if ((form.primaryTraitHash >>> 0) !== (status.primaryTraitHash >>> 0)) n++
  if ((form.primaryTraitLevel >>> 0) !== (status.primaryTraitLevel >>> 0)) n++
  if ((form.secondaryTraitHash >>> 0) !== (status.secondaryTraitHash >>> 0)) n++
  if ((form.secondaryTraitLevel >>> 0) !== (status.secondaryTraitLevel >>> 0)) n++
  return n
})
const changeSummary = computed(() => changedCount.value ? `待写入 ${changedCount.value} 个字段` : '尚未修改')
const canWrite = computed(() => !!status.selectedAddr && changedCount.value > 0 && legality.value.status !== 'impossible')

function revertToRead() { syncFormFromStatus() }

function applyEntry(entry) {
  form.sigilHash = entry.sigilHash >>> 0
  form.sigilLevel = entry.sigilLevel >>> 0
  form.primaryTraitHash = entry.primaryTraitHash >>> 0
  form.primaryTraitLevel = entry.primaryTraitLevel >>> 0
  form.secondaryTraitHash = entry.secondaryTraitHash >>> 0
  form.secondaryTraitLevel = entry.secondaryTraitLevel >>> 0
}
async function applyAndWrite(entry) {
  applyEntry(entry)
  await performWrite()
}
function nameFor(map, hash, fallback = '?') {
  const opt = map.get(hash >>> 0)
  if (opt && !isRawHexName(opt.displayName)) return opt.displayName
  return hash ? `未知 · ${hex(hash)}` : fallback
}

function autoTemplateName() {
  return nameFor(sigilByHash.value, form.sigilHash, '空模板')
}
function saveCurrentAsTemplate() {
  const saved = saveTemplate(autoTemplateName(), form)
  if (saved) show(`模板已保存: ${saved.name}`, 'success')
}

function readDisplay(name, hash) {
  if (!hash) return { text: '— 未设置', dim: true }
  if (isRawHexName(name)) return { text: '未知条目', dim: true }
  return { text: name, dim: false }
}
const sigilRead = computed(() => readDisplay(status.sigilName, status.sigilHash))
const primaryRead = computed(() => readDisplay(status.primaryTraitName, status.primaryTraitHash))
const secondaryRead = computed(() => readDisplay(status.secondaryTraitName, status.secondaryTraitHash))

function entrySubtitle(entry) {
  const s = nameFor(sigilByHash.value, entry.sigilHash, '—')
  const p = nameFor(traitByHash.value, entry.primaryTraitHash, '—')
  const sec = entry.secondaryTraitHash ? nameFor(traitByHash.value, entry.secondaryTraitHash) : '无'
  return `${s} · 主 ${p} · 副 ${sec}`
}

const filteredTemplates = computed(() => {
  const q = templateSearch.value.trim()
  if (!q) return templates.value
  return templates.value.filter(t => matchText(t.name, q) || matchText(entrySubtitle(t), q))
})

const renameEl = ref(null)

function startRename(id, currentName) {
  renamingId.value = id
  renameBuffer.value = currentName
}
function confirmRename() {
  if (renamingId.value) renameTemplate(renamingId.value, renameBuffer.value)
  renamingId.value = null
  renameBuffer.value = ''
}
function cancelRename() {
  renamingId.value = null
  renameBuffer.value = ''
}

async function clearWriteHistory() {
  if (!history.value.length) return
  const confirmed = await confirmDialog.value?.ask({
    title: '清空最近写入',
    message: '确定清空全部最近写入记录？',
    detail: '这只会清除工具内的历史记录，不会回退已经写入游戏的因子。',
    tone: 'danger',
    confirmLabel: '清空记录',
  })
  if (!confirmed) return
  clearHistory()
  show('最近写入已清空', 'success')
}

function onRenameOutsideClick(e) {
  if (!renamingId.value) return
  if (renameEl.value && !renameEl.value.contains(e.target)) cancelRename()
}
watch(renamingId, (v) => {
  if (v) document.addEventListener('mousedown', onRenameOutsideClick)
  else document.removeEventListener('mousedown', onRenameOutsideClick)
})
onBeforeUnmount(() => document.removeEventListener('mousedown', onRenameOutsideClick))
function fmtRelTime(ts) {
  const diffSec = Math.floor((Date.now() - ts) / 1000)
  if (diffSec < 60) return '刚刚'
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} 小时前`
  return `${Math.floor(diffSec / 86400)} 天前`
}

const statusLabel = computed(() => {
  if (status.hooked) return '已启用'
  if (status.found) return '就绪'
  return '未连接'
})

onMounted(async () => {
  await loadOptions()
  await refresh(true)
})
</script>

<template>
  <div class="memory-sigil">
    <!-- Connection strip -->
    <div class="section conn-section">
      <div class="conn-row">
        <div class="conn-left">
          <span class="chip" :class="{ state: status.hooked, dim: !status.hooked }">● {{ statusLabel }}</span>
          <span v-if="!status.hooked && status.found" class="hint-inline">点击启用读取，然后在游戏内选择因子</span>
          <span v-else-if="status.hooked && !status.selectedAddr" class="hint-inline">等待游戏内因子选择</span>
        </div>
        <div class="conn-right">
          <button v-if="status.hooked" class="btn tiny" :disabled="loading" @click="refresh(true)">{{ loading ? '刷新中…' : '刷新' }}</button>
          <button class="btn tiny btn-cyan" :disabled="loading" @click="enable">{{ status.hooked ? '重新连接' : '启用读取' }}</button>
        </div>
      </div>
    </div>

    <!-- Editor -->
    <div class="section" :class="{ muted: !status.selectedAddr }">
      <div class="editor-header">
        <span class="section-title">因子编辑</span>
        <div class="editor-actions">
          <button class="ed-link" :disabled="!status.selectedAddr" @click="saveCurrentAsTemplate" title="保存当前目标为模板 (稍后可重命名)">＋ 保存为模板</button>
          <button class="ed-link" :disabled="!status.selectedAddr || changedCount === 0" @click="revertToRead" title="放弃修改，恢复为游戏内当前值">↺ 还原</button>
        </div>
      </div>

      <div class="ed-row">
        <div class="ed-row-head">
          <span class="ed-label">因子</span>
          <div class="ed-current">
          <span class="ed-current-prefix">当前：</span>
          <span class="ed-current-name" :class="{ dim: sigilRead.dim }" :title="sigilRead.text">{{ sigilRead.text }}</span>
          <span v-if="status.sigilHash" class="ed-current-lv">Lv {{ status.sigilLevel }}</span>
          </div>
        </div>
        <div class="ed-edit-line">
          <SigilMemoryPicker v-model="form.sigilHash" :options="allSigilOptions" @pick="onPickSigil" placeholder="选择因子" />
          <label class="ed-level-control">
            <span>等级</span>
            <input v-model.number="form.sigilLevel" type="number" min="0" :max="sigilWritableMax" aria-label="因子等级" @change="form.sigilLevel = clampLevel(form.sigilLevel, sigilWritableMax)" />
            <small v-if="sigilMax != null">合规上限 {{ sigilMax }} / 修改上限 {{ sigilWritableMax }}</small>
          </label>
          <button class="ed-max-btn" :disabled="sigilMax == null || sigilAtMax" @click="maxSigil" :title="sigilMax != null ? `设为上限 ${sigilMax}` : '无等级元数据'">设为上限</button>
        </div>
      </div>

      <div class="ed-row">
        <div class="ed-row-head">
          <span class="ed-label">主词条</span>
          <div class="ed-current">
          <span class="ed-current-prefix">当前：</span>
          <span class="ed-current-name" :class="{ dim: primaryRead.dim }" :title="primaryRead.text">{{ primaryRead.text }}</span>
          <span v-if="status.primaryTraitHash" class="ed-current-lv">Lv {{ status.primaryTraitLevel }}</span>
          </div>
        </div>
        <div class="ed-edit-line">
          <SigilMemoryPicker v-model="form.primaryTraitHash" :options="allTraitOptions" @pick="onPickPrimary" placeholder="选择主词条" />
          <label class="ed-level-control">
            <span>等级</span>
            <input v-model.number="form.primaryTraitLevel" type="number" min="0" :max="primaryWritableMax" aria-label="主词条等级" @change="form.primaryTraitLevel = clampLevel(form.primaryTraitLevel, primaryWritableMax)" />
            <small v-if="primaryMax != null">合规上限 {{ primaryMax }} / 修改上限 {{ primaryWritableMax }}</small>
          </label>
          <button class="ed-max-btn" :disabled="primaryMax == null || primaryAtMax" @click="maxPrimary" :title="primaryMax != null ? `设为上限 ${primaryMax}` : '无等级元数据'">设为上限</button>
        </div>
      </div>

      <div class="ed-row">
        <div class="ed-row-head">
          <span class="ed-label">副词条</span>
          <div class="ed-current">
          <span class="ed-current-prefix">当前：</span>
          <span class="ed-current-name" :class="{ dim: secondaryRead.dim }" :title="secondaryRead.text">{{ secondaryRead.text }}</span>
          <span v-if="status.secondaryTraitHash" class="ed-current-lv">Lv {{ status.secondaryTraitLevel }}</span>
          </div>
        </div>
        <div class="ed-edit-line">
          <SigilMemoryPicker v-model="form.secondaryTraitHash" :options="allTraitOptions" @pick="onPickSecondary" optional placeholder="未选择（可选）" />
          <label class="ed-level-control" :class="{ disabled: !form.secondaryTraitHash }">
            <span>等级</span>
            <input v-if="form.secondaryTraitHash" v-model.number="form.secondaryTraitLevel" type="number" min="0" :max="secondaryWritableMax" aria-label="副词条等级" @change="form.secondaryTraitLevel = clampLevel(form.secondaryTraitLevel, secondaryWritableMax)" />
            <b v-else class="ed-level-empty">—</b>
            <small v-if="secondaryMax != null">合规上限 {{ secondaryMax }} / 修改上限 {{ secondaryWritableMax }}</small>
          </label>
          <button class="ed-max-btn" :disabled="secondaryMax == null || secondaryAtMax" @click="maxSecondary" :title="secondaryMax != null ? `设为上限 ${secondaryMax}` : '请先选择副词条'">设为上限</button>
        </div>
      </div>

      <div class="warn-slot">
        <div v-if="warnings.length" class="warn-list">
          <div v-for="(w, i) in warnings" :key="i" class="warn-inline">⚠ {{ w }}</div>
        </div>
      </div>

      <div class="ed-bar">
        <div class="ed-summary">
          <LegalityIndicator v-if="status.selectedAddr" class="ed-legality" :status="legality.status" :message="legality.message" />
          <span v-else class="selection-note">在游戏内选中一个因子后，这里会显示合法性与写入状态</span>
          <span class="ed-changed">{{ changeSummary }}</span>
        </div>
        <div class="ed-bar-actions">
          <button class="ed-max-all" :disabled="applying || !canOneClickMax" @click="oneClickMax" title="只填写已知上限，不会立即写入">全部等级设为上限</button>
          <button class="ed-write" :disabled="applying || !canWrite" @click="write">{{ applying ? '写入中…' : '写入修改' }}</button>
        </div>
      </div>
    </div>

    <!-- Shared list: templates + history -->
    <div class="section">
      <div class="tabs-head">
        <div class="tabs">
          <span class="tab" :class="{ active: tab === 'templates' }" @click="tab = 'templates'">模板 <span class="tab-count">{{ templates.length }}</span></span>
          <span class="tab" :class="{ active: tab === 'history' }" @click="tab = 'history'">最近写入 <span class="tab-count">{{ history.length }}</span></span>
        </div>
        <div v-if="tab === 'templates'">
          <input v-model="templateSearch" class="search-input" placeholder="搜索模板..." />
        </div>
        <button v-else-if="history.length" class="row-tool" title="清空最近写入" @click="clearWriteHistory">清空</button>
      </div>

      <div v-if="tab === 'templates'">
        <div v-if="!filteredTemplates.length" class="tpl-empty">
          {{ templates.length ? '无匹配模板' : '尚无模板 · 在编辑器点击 "＋ 保存为模板"' }}
        </div>
        <ul v-else class="row-list">
          <li v-for="t in filteredTemplates" :key="t.id" class="row-item" @click="applyEntry(t)">
            <span class="row-name">
              <template v-if="renamingId === t.id">
                <span class="rename-group" ref="renameEl">
                  <input v-model="renameBuffer" class="rename-input" @click.stop @keydown.enter="confirmRename" @keydown.escape="cancelRename" />
                  <button class="rename-confirm" :disabled="!renameBuffer.trim()" @click.stop="confirmRename" title="保存 (Enter)">✓</button>
                  <button class="rename-cancel" @click.stop="cancelRename" title="取消 (Esc)">✕</button>
                </span>
              </template>
              <template v-else>
                <span class="row-name-text">{{ t.name }}</span>
                <span class="row-name-lv">Lv {{ t.sigilLevel }}</span>
              </template>
            </span>
            <span class="row-chip primary-chip">
              <span class="row-chip-tag">主</span>
              <span class="row-chip-name">{{ nameFor(traitByHash, t.primaryTraitHash, '—') }}</span>
              <span class="row-chip-lv">Lv {{ t.primaryTraitLevel }}</span>
            </span>
            <span class="row-chip secondary-chip" :class="{ 'empty-slot': !t.secondaryTraitHash }">
              <span class="row-chip-tag">副</span>
              <span class="row-chip-name">{{ t.secondaryTraitHash ? nameFor(traitByHash, t.secondaryTraitHash) : '—' }}</span>
              <span class="row-chip-lv">Lv {{ t.secondaryTraitLevel }}</span>
            </span>
            <span class="row-tools" @click.stop>
              <button class="row-tool" title="重命名" @click="startRename(t.id, t.name)">✎</button>
              <button class="row-tool" title="删除" @click="deleteTemplate(t.id)">✕</button>
            </span>
            <button class="row-apply" :disabled="!status.selectedAddr || applying" @click.stop="applyAndWrite(t)" title="立即应用并写入">一键应用</button>
          </li>
        </ul>
      </div>

      <div v-else>
        <div v-if="!history.length" class="tpl-empty">尚无历史</div>
        <ul v-else class="row-list">
          <li v-for="h in history" :key="h.id" class="row-item" @click="applyEntry(h)">
            <span class="row-name">
              <span class="row-name-text">{{ nameFor(sigilByHash, h.sigilHash, '—') }}</span>
              <span class="row-name-lv">Lv {{ h.sigilLevel }}</span>
            </span>
            <span class="row-chip primary-chip">
              <span class="row-chip-tag">主</span>
              <span class="row-chip-name">{{ nameFor(traitByHash, h.primaryTraitHash, '—') }}</span>
              <span class="row-chip-lv">Lv {{ h.primaryTraitLevel }}</span>
            </span>
            <span class="row-chip secondary-chip" :class="{ 'empty-slot': !h.secondaryTraitHash }">
              <span class="row-chip-tag">副</span>
              <span class="row-chip-name">{{ h.secondaryTraitHash ? nameFor(traitByHash, h.secondaryTraitHash) : '—' }}</span>
              <span class="row-chip-lv">Lv {{ h.secondaryTraitLevel }}</span>
            </span>
            <span class="row-meta">{{ fmtRelTime(h.createdAt) }}</span>
            <button class="row-apply" :disabled="!status.selectedAddr || applying" @click.stop="applyAndWrite(h)" title="立即应用并写入">一键应用</button>
          </li>
        </ul>
      </div>
    </div>
  </div>
  <ConfirmDialog ref="confirmDialog" />
</template>

<style scoped>
.memory-sigil { width:100%; display:flex; flex-direction:column; gap:16px; font-family:inherit; container-type:inline-size; }
.section { padding:18px 20px; border:1px solid rgba(255,255,255,.08); border-radius:8px; background:rgba(255,255,255,.04); display:flex; flex-direction:column; gap:12px; font-family:inherit; }
.section.muted { opacity:1; }
.section-title { color:rgba(255,255,255,.4); font-size:.72rem; font-weight:600; letter-spacing:.1em; text-transform:uppercase; }
.hint-inline { font-size:.72rem; color:rgba(255,255,255,.4); }

/* Connection */
.conn-section { padding:14px 18px; }
.conn-row { display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:10px; }
.conn-left { display:flex; gap:10px; align-items:center; flex-wrap:wrap; }
.conn-right { display:flex; gap:8px; align-items:center; }
.chip { padding:3px 10px; border:1px solid rgba(255,255,255,.12); border-radius:999px; background:rgba(255,255,255,.05); font-size:.72rem; color:rgba(255,255,255,.55); font-family:inherit; }
.chip.state { color:#4ade80; border-color:rgba(74,222,128,.3); background:rgba(74,222,128,.06); }
.chip.dim { color:rgba(255,255,255,.4); }

/* Button base */
.btn { padding:8px 14px; border:1px solid rgba(255,255,255,.16); border-radius:6px; background:rgba(255,255,255,.06); color:rgba(255,255,255,.75); font-size:.8rem; font-weight:600; cursor:pointer; font-family:inherit; }
.btn:disabled { opacity:.4; cursor:not-allowed; }
.btn-cyan { border-color:rgba(154,116,64,.35); color:#9a7440; background:rgba(154,116,64,.1); }
.btn.tiny { padding:4px 9px; font-size:.7rem; }

/* Editor header */
.editor-header { display:flex; justify-content:space-between; align-items:center; min-height:34px; margin-bottom:2px; padding-bottom:10px; border-bottom:1px solid rgba(126,91,43,.16); }
.editor-actions { display:flex; gap:14px; }
.ed-link { color:rgba(255,255,255,.5); font-size:.72rem; cursor:pointer; background:none; border:0; font-family:inherit; padding:0; }
.ed-link:hover:not(:disabled) { color:#9a7440; }
.ed-link:disabled { opacity:.35; cursor:not-allowed; }

/* Each editor field is a small journal entry: current value first, editable controls below. */
.ed-row { display:flex;flex-direction:column;gap:7px;min-width:0;padding:9px 0 11px; }
.ed-row + .ed-row { border-top:1px solid rgba(126,91,43,.16); }
.ed-row-head { display:grid;grid-template-columns:58px minmax(0,1fr);align-items:start;gap:8px;min-width:0 }
.ed-label { color:rgba(255,255,255,.4); font-size:.75rem; font-weight:800; }
.ed-current { display:flex;align-items:baseline;gap:5px;min-width:0;flex-wrap:wrap;line-height:1.35 }
.ed-current-prefix { color:rgba(255,255,255,.35);font-size:.67rem;font-weight:700;flex:0 0 auto }
.ed-current-name { color:rgba(255,255,255,.85);font-weight:800;font-size:.75rem;white-space:normal;overflow:visible;text-overflow:clip;overflow-wrap:anywhere }
.ed-current-name.dim { color:rgba(255,255,255,.35); font-weight:400; }
.ed-current-lv { color:rgba(255,255,255,.4); font-size:.68rem; font-family:var(--font-data); flex-shrink:0; }
.ed-edit-line { display:grid;grid-template-columns:minmax(0,1fr) 78px 60px;align-items:stretch;gap:8px;min-width:0 }
.ed-level-control { min-width:0;display:grid;grid-template-columns:26px minmax(0,1fr);grid-template-rows:24px 12px;align-items:center;column-gap:4px;color:rgba(255,255,255,.42);font-size:.63rem;font-weight:800 }
.ed-level-control>span { text-align:right }
.ed-level-control input { width:100%;height:24px;min-height:24px;padding:0 3px;border:1px solid rgba(255,255,255,.14);border-radius:2px;background:rgba(255,255,255,.05);color:rgba(255,255,255,.92);font:800 .75rem var(--font-data);font-variant-numeric:tabular-nums lining-nums;text-align:center;outline:none }
.ed-level-control small { grid-column:1 / -1;color:rgba(255,255,255,.35);font-size:.58rem;font-weight:700;text-align:right;white-space:nowrap }
.ed-level-control.disabled { opacity:.58 }
.ed-level-empty { display:grid;place-items:center;height:24px;color:rgba(255,255,255,.34);font-size:.75rem }
.ed-level-control input[type=number]::-webkit-inner-spin-button,.ed-level-control input[type=number]::-webkit-outer-spin-button { -webkit-appearance:none;margin:0 }
.ed-level-control input[type=number] { -moz-appearance:textfield }
.ed-max-btn { width:100%;min-height:32px;align-self:start;padding:4px 5px;border:1px solid rgba(154,116,64,.35);background:transparent;color:#9a7440;border-radius:2px;font:800 .62rem var(--font-ui);cursor:pointer;letter-spacing:0 }
.ed-max-btn:hover:not(:disabled) { background:rgba(154,116,64,.08); }
.ed-max-btn:disabled { opacity:.3; cursor:not-allowed; }

/* Warnings + write bar */
.warn-slot { min-height:0; }
.warn-list { display:flex; flex-direction:column; gap:3px; padding:7px 9px; border-left:2px solid #9a7440; background:#edddba; }
.warn-inline { color:#8a5d1b; font-size:.68rem; line-height:1.45; }
.ed-bar { min-height:62px;display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:end;gap:12px;padding-top:12px;margin-top:3px;border-top:1px solid rgba(126,91,43,.16) }
.ed-summary { min-width:0;display:flex;flex-direction:column;gap:5px }
.selection-note { color:#8a775d;font-size:.66rem;font-weight:650;line-height:1.45; }
.ed-legality { margin:0 }
.ed-changed { color:rgba(255,255,255,.4);font-size:.66rem;font-weight:700 }
.ed-bar-actions { display:flex;align-items:center;gap:8px }
.ed-max-all { min-height:32px;color:#9a7440;background:none;border:1px solid currentColor;border-radius:2px;font:750 .66rem var(--font-ui);cursor:pointer;padding:5px 9px;white-space:nowrap }
.ed-max-all:hover:not(:disabled) { text-decoration:underline; }
.ed-max-all:disabled { opacity:.35; cursor:not-allowed; text-decoration:none; }
.ed-write { min-height:32px;background:rgba(74,222,128,.14);border:1px solid rgba(74,222,128,.4);color:#4ade80;border-radius:2px;padding:6px 17px;font:750 .7rem var(--font-ui);cursor:pointer;letter-spacing:.02em;white-space:nowrap }
.ed-write:hover:not(:disabled) { background:rgba(74,222,128,.22); }
.ed-write:disabled { opacity:.4; cursor:not-allowed; }

/* Tabs */
.tabs-head { min-height:42px;display:flex; justify-content:space-between; align-items:center; gap:16px; margin-bottom:10px; }
.tabs { display:flex; gap:18px; }
.tab { color:rgba(255,255,255,.4); font-size:.75rem; font-weight:600; cursor:pointer; padding-bottom:6px; border-bottom:2px solid transparent; }
.tab.active { color:rgba(255,255,255,.9); border-bottom-color:#9a7440; }
.tab-count { color:rgba(255,255,255,.3); font-weight:400; margin-left:4px; }
.search-input { padding:5px 10px; border:1px solid rgba(255,255,255,.12); border-radius:6px; background:rgba(255,255,255,.05); color:#fff; font:inherit; font-size:.72rem; width:170px; font-family:inherit; }

.row-list { list-style:none; margin:0; padding:0; display:flex;flex-direction:column;gap:3px }
.row-item { display:grid;grid-template-columns:minmax(0,1fr) auto;gap:5px 10px;align-items:center;padding:8px 10px;border-radius:2px;cursor:pointer }
.row-item:hover { background:rgba(154,116,64,.05); }

.row-name { grid-column:1 / -1;display:inline-flex;align-items:baseline;gap:6px;min-width:0;flex-wrap:wrap }
.row-name-text { color:rgba(255,255,255,.88);font-weight:600;font-size:.8rem;white-space:normal;overflow:visible;text-overflow:clip;overflow-wrap:anywhere }
.row-name-lv { color:rgba(255,255,255,.4); font-size:.66rem; font-family:var(--font-data); flex-shrink:0; }
.rename-group { display:inline-flex; align-items:center; gap:4px; min-width:0; flex:1; }
.rename-input { padding:2px 6px; border:1px solid rgba(154,116,64,.4); border-radius:4px; background:rgba(154,116,64,.06); color:#fff; font:inherit; font-size:.78rem; font-weight:600; min-width:0; flex:1; box-sizing:border-box; outline:none; font-family:inherit; }
.rename-confirm, .rename-cancel { flex-shrink:0; background:transparent; border:1px solid transparent; padding:2px 7px; cursor:pointer; font-size:.7rem; border-radius:3px; font-family:inherit; line-height:1; }
.rename-confirm { color:#4ade80; border-color:rgba(74,222,128,.35); background:rgba(74,222,128,.08); }
.rename-confirm:hover:not(:disabled) { background:rgba(74,222,128,.18); }
.rename-confirm:disabled { opacity:.35; cursor:not-allowed; }
.rename-cancel { color:rgba(255,255,255,.5); }
.rename-cancel:hover { color:#f87171; background:rgba(248,113,113,.1); border-color:rgba(248,113,113,.3); }

.row-chip { min-width:0;display:inline-flex;align-items:baseline;gap:5px;padding:2px 8px;border:1px solid rgba(255,255,255,.08);border-radius:2px;background:rgba(255,255,255,.04);font-size:.72rem;max-width:none;flex-wrap:wrap }
.primary-chip { grid-column:1;grid-row:2 }.secondary-chip { grid-column:1;grid-row:3 }
.row-chip-tag { color:rgba(255,255,255,.35); font-size:.62rem; letter-spacing:.05em; font-weight:600; flex-shrink:0; }
.row-chip-name { color:rgba(255,255,255,.78);font-weight:500;white-space:normal;overflow:visible;text-overflow:clip;overflow-wrap:anywhere }
.row-chip-lv { color:rgba(255,255,255,.42); font-size:.64rem; font-family:var(--font-data); flex-shrink:0; }
.row-chip.empty-slot { visibility:hidden; }

.row-meta { grid-column:2;grid-row:2;color:rgba(255,255,255,.35);font-size:.68rem;justify-self:end;white-space:nowrap }
.row-tools { grid-column:2;grid-row:2;display:flex;gap:4px;opacity:0;transition:opacity .12s;justify-self:end }
.row-item:hover .row-tools { opacity:1; }
.row-tool { background:transparent; border:1px solid transparent; color:rgba(255,255,255,.5); padding:3px 7px; cursor:pointer; font-size:.7rem; border-radius:4px; font-family:inherit; line-height:1; }
.row-tool:hover { color:#9a7440; background:rgba(154,116,64,.1); border-color:rgba(154,116,64,.3); }

.row-apply { grid-column:2;grid-row:3;padding:4px 12px;border:1px solid rgba(74,222,128,.35);background:rgba(74,222,128,.1);color:#4ade80;border-radius:2px;font:750 .7rem var(--font-ui);cursor:pointer;letter-spacing:.02em }
.row-apply:hover:not(:disabled) { background:rgba(74,222,128,.2); }
.row-apply:disabled { opacity:.35; cursor:not-allowed; }

.tpl-empty { padding:22px; text-align:center; color:rgba(255,255,255,.3); font-size:.75rem; }
.memory-sigil{gap:13px}.section,.editor-card,.library-card{border-color:rgba(154,202,224,.14)!important;border-radius:4px 12px 4px 12px!important;background:rgba(8,31,53,.7)!important}.section-title,.editor-title{color:#eee7d8!important;font-family:Georgia,"Noto Serif SC","STSong",serif!important}.row-item{border-color:rgba(154,202,224,.1)!important;background:rgba(13,45,70,.46)!important}.row-item:hover{background:rgba(45,112,145,.12)!important}.row-chip{border-color:rgba(154,202,224,.13);background:rgba(12,43,68,.72)}.row-apply,.ed-apply-btn,.ed-max-all{border-color:rgba(218,187,115,.34)!important;background:rgba(218,187,115,.08)!important;color:#f0d99d!important}
@container (max-width:520px){.ed-edit-line{grid-template-columns:minmax(0,1fr) 76px 58px;gap:6px}.ed-row-head{grid-template-columns:54px minmax(0,1fr)}.editor-actions{gap:8px}.warn-slot{padding-left:0}.ed-bar{grid-template-columns:1fr;align-items:start}.ed-bar-actions{justify-content:flex-end}}
</style>
