<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { SigilMemoryAcquire, SigilMemoryGetOptions, SigilMemoryGetStatus, SigilMemoryRelease, SigilMemoryUpdateOwned } from '../../wailsjs/go/main/App'
import { matchText } from '../utils/matchText.js'
import { backendLanguageReady } from '../backendLanguage'
import { traitAssetIcon } from '../gameAssetIcons'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'
import { clearHistory, deleteTemplate, history, pushHistory, renameTemplate, saveTemplate, templates } from '../utils/sigilMemoryStore.js'
import SigilMemoryPicker from './SigilMemoryPicker.vue'
import LegalityIndicator from './LegalityIndicator.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const emit = defineEmits(['status'])
const RUNTIME_LEASE_SCOPE = 'sigil-memory-generator'

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

function traitIconByHash(hash, name = '') {
  const value = Number(hash) >>> 0
  if (!value || value === EMPTY_HASH) return ''
  const option = traitByHash.value.get(value)
  return traitAssetIcon({ hash: value, name: option?.displayName || name })
}

function traitOptionIcon(option) {
  return traitIconByHash(option?.hash, option?.displayName)
}

const loading = ref(false)
const applying = ref(false)
const templateSearch = ref('')
const tab = ref('templates')
const renamingId = ref(null)
const renameBuffer = ref('')
const confirmDialog = ref(null)
let disposed = false
let lifecycleEpoch = 0
let hookOwnerToken = ''
let pollTimer = 0
let pollInFlight = false

function show(msg, type) { emit('status', msg, type) }
function hex(v) { return '0x' + (Number(v) >>> 0).toString(16).toUpperCase().padStart(8, '0') }
const EMPTY_HASH = 0x887AE0B0
const HEX_RE = /^0x[0-9A-F]{8}$/i
function isRawHexName(n) { return typeof n === 'string' && HEX_RE.test(n.trim()) }
function isEmptyTraitHash(value) {
  const hash = Number(value) >>> 0
  return hash === 0 || hash === EMPTY_HASH
}
function normaliseSecondaryHash(value) {
  return isEmptyTraitHash(value) ? 0 : Number(value) >>> 0
}

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
  next = next || {}
  const normalised = {
    found: false, hooked: false, selectedAddr: 0,
    sigilHash: 0, sigilLevel: 0, sigilName: '',
    primaryTraitHash: 0, primaryTraitLevel: 0, primaryTraitName: '',
    secondaryTraitHash: 0, secondaryTraitLevel: 0, secondaryTraitName: '',
    ...next,
    secondaryTraitHash: normaliseSecondaryHash(next.secondaryTraitHash),
    secondaryTraitLevel: isEmptyTraitHash(next.secondaryTraitHash) ? 0 : Number(next.secondaryTraitLevel) >>> 0,
  }
  Object.assign(status, normalised)
  ensureRuntimeOption(runtimeOptions.sigils, normalised.sigilHash, normalised.sigilName)
  ensureRuntimeOption(runtimeOptions.traits, normalised.primaryTraitHash, normalised.primaryTraitName)
  ensureRuntimeOption(runtimeOptions.traits, normalised.secondaryTraitHash, normalised.secondaryTraitName)
}

function syncFormFromStatus() {
  form.sigilHash = status.sigilHash >>> 0
  form.sigilLevel = status.sigilLevel >>> 0
  form.primaryTraitHash = status.primaryTraitHash >>> 0
  form.primaryTraitLevel = status.primaryTraitLevel >>> 0
  form.secondaryTraitHash = status.secondaryTraitHash >>> 0
  form.secondaryTraitLevel = status.secondaryTraitLevel >>> 0
}

function stopPolling() {
  if (pollTimer) window.clearInterval(pollTimer)
  pollTimer = 0
}

function startPolling() {
  stopPolling()
  pollTimer = window.setInterval(pollStatus, 700)
}

async function pollStatus() {
  if (disposed || loading.value || applying.value || pollInFlight || !status.hooked) return
  const epoch = lifecycleEpoch
  const previousSelectedAddr = Number(status.selectedAddr || 0)
  pollInFlight = true
  try {
    const next = await SigilMemoryGetStatus()
    if (disposed || epoch !== lifecycleEpoch) return
    applyStatus(next)
    const nextSelectedAddr = Number(status.selectedAddr || 0)
    if (nextSelectedAddr && nextSelectedAddr !== previousSelectedAddr) syncFormFromStatus()
    if (!status.hooked) stopPolling()
  } catch (e) {
    if (!disposed && epoch === lifecycleEpoch) {
      stopPolling()
      show(`自动读取已暂停：${String(e)}`, 'error')
    }
  } finally {
    pollInFlight = false
  }
}

async function loadOptions() {
  try {
    await backendLanguageReady
    if (disposed) return
    const res = await SigilMemoryGetOptions()
    if (disposed) return
    backendOptions.sigils = res.sigils || []
    backendOptions.traits = res.traits || []
  } catch (e) { if (!disposed) show('读取因子数据失败: ' + String(e), 'error') }
}

async function refresh(syncForm = false) {
  if (loading.value || applying.value) return
  const epoch = ++lifecycleEpoch
  loading.value = true
  try {
    const next = await SigilMemoryGetStatus()
    if (disposed || epoch !== lifecycleEpoch) return
    applyStatus(next)
    if (syncForm) syncFormFromStatus()
    if (status.hooked) startPolling()
    else stopPolling()
    if (!status.hooked) show('已就绪。启用读取后，在游戏内选中因子。', 'success')
    else if (!status.selectedAddr) show('等待游戏内因子选择。', 'success')
    else show(`已读取: ${status.sigilName}`, 'success')
  } catch (e) { if (!disposed && epoch === lifecycleEpoch) show(String(e), 'error') }
  finally { if (!disposed && epoch === lifecycleEpoch) loading.value = false }
}

async function enable() {
  if (loading.value || applying.value) return
  const epoch = ++lifecycleEpoch
  loading.value = true
  let acquiredOwnerToken = ''
  try {
    const next = await SigilMemoryAcquire(nextRuntimeAcquireRequestID())
    acquiredOwnerToken = String(next?.ownerToken || '')
    if (!acquiredOwnerToken) throw new Error('后端未返回因子读取所有权令牌')
    if (disposed || epoch !== lifecycleEpoch) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, SigilMemoryRelease)
      return
    }
    hookOwnerToken = acquiredOwnerToken
    applyStatus(next)
    syncFormFromStatus()
    startPolling()
    show('已启用。请在游戏内选择一个因子。', 'success')
  } catch (e) {
    stopPolling()
    let cleanupError = null
    if (acquiredOwnerToken) {
      try { await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, SigilMemoryRelease) } catch (nextError) { cleanupError = nextError }
      if (!cleanupError && hookOwnerToken === acquiredOwnerToken) hookOwnerToken = ''
    }
    if (!disposed && epoch === lifecycleEpoch) {
      if (cleanupError) {
        status.hooked = true
        status.selectedAddr = 0
        show(`${String(e)}；停止因子读取也失败：${String(cleanupError)}`, 'error')
      } else {
        applyStatus({})
        syncFormFromStatus()
        show(String(e), 'error')
      }
    }
  }
  finally { if (!disposed && epoch === lifecycleEpoch) loading.value = false }
}

async function disable() {
  if (loading.value || applying.value) return
  const epoch = ++lifecycleEpoch
  const ownerToken = hookOwnerToken
  stopPolling()
  if (!ownerToken) {
    applyStatus({})
    syncFormFromStatus()
    return
  }
  loading.value = true
  try {
    const next = await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, SigilMemoryRelease)
    if (disposed || epoch !== lifecycleEpoch) return
    if (hookOwnerToken === ownerToken) hookOwnerToken = ''
    applyStatus(next)
    syncFormFromStatus()
    show('读取已停止，游戏指令已恢复。', 'success')
  } catch (e) { if (!disposed && epoch === lifecycleEpoch) show(String(e), 'error') }
  finally { if (!disposed && epoch === lifecycleEpoch) loading.value = false }
}

async function performWrite() {
  if (loading.value || applying.value) return
  if (!status.hooked || !status.selectedAddr) { show('请先启用读取，并在游戏内选中一个因子', 'error'); return }
  if (!canWrite.value) { show(legality.value.message || '当前因子组合未通过写入校验', 'error'); return }
  const epoch = ++lifecycleEpoch
  applying.value = true
  try {
    const ownerToken = hookOwnerToken
    if (!ownerToken) throw new Error('当前页面不再持有因子读取所有权')
    const snapshot = { ...form }
    const expectedSelectedAddr = Number(status.selectedAddr || 0)
    const next = await SigilMemoryUpdateOwned(ownerToken, { ...snapshot, expectedSelectedAddr })
    if (disposed || epoch !== lifecycleEpoch) return
    applyStatus(next)
    pushHistory(snapshot)
    show(`已写入: ${status.sigilName}`, 'success')
  } catch (e) { if (!disposed && epoch === lifecycleEpoch) show(String(e), 'error') }
  finally { if (!disposed && epoch === lifecycleEpoch) applying.value = false }
}
async function write() { await performWrite() }

async function oneClickMax() {
  if (loading.value || applying.value) return
  if (!status.hooked || !status.selectedAddr) { show('请先启用读取，并在游戏内选中一个因子', 'error'); return }
  if (sigilMax.value != null) form.sigilLevel = sigilMax.value
  if (primaryMax.value != null) form.primaryTraitLevel = primaryMax.value
  if (secondaryMax.value != null) form.secondaryTraitLevel = secondaryMax.value
}

function onPickSigil(opt) {
  if (opt && opt.maxLevel != null) form.sigilLevel = opt.maxLevel
  else if (!opt) form.sigilLevel = 0
  if (opt?.primaryTraitHash) {
    form.primaryTraitHash = Number(opt.primaryTraitHash) >>> 0
    const verifiedLevels = Array.isArray(opt.allowedPrimaryTraitLevels) ? opt.allowedPrimaryTraitLevels : []
    const nextLevel = verifiedLevels.length ? Math.max(...verifiedLevels) : Number(opt.firstTraitMaxLevel || 0)
    if (nextLevel > 0) form.primaryTraitLevel = nextLevel
  }
}
function preferredOptionLevel(opt) {
  const levels = Array.isArray(opt?.allowedLevels) ? opt.allowedLevels.filter(Number.isInteger) : []
  if (levels.length) return Math.max(...levels)
  return Number(opt?.maxLevel || 15)
}
function onPickPrimary(opt) {
  if (opt) form.primaryTraitLevel = preferredOptionLevel(opt)
  else if (!opt) form.primaryTraitLevel = 0
}
function onPickSecondary(opt) {
  if (opt) form.secondaryTraitLevel = preferredOptionLevel(opt)
  else if (!opt) form.secondaryTraitLevel = 0
}

const sigilMax = computed(() => sigilByHash.value.get(form.sigilHash)?.maxLevel ?? null)
// Primary/secondary max come from the picked trait itself, not the sigil's default trait cap.
// Prior bug: primaryMax fell back to sigil.firstTraitMaxLevel even when the picked trait had its own maxLevel,
// so switching to a memory-only trait still showed the sigil's default primary max.
const primaryMax = computed(() => {
  if (!form.primaryTraitHash) return null
  const sigil = sigilByHash.value.get(form.sigilHash)
  const verifiedLevels = Array.isArray(sigil?.allowedPrimaryTraitLevels) ? sigil.allowedPrimaryTraitLevels : []
  if (verifiedLevels.length) return Math.max(...verifiedLevels)
  return sigil?.firstTraitMaxLevel ?? traitByHash.value.get(form.primaryTraitHash)?.maxLevel ?? null
})
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
  if (sigil?.primaryTraitHash && (Number(sigil.primaryTraitHash) >>> 0) !== (form.primaryTraitHash >>> 0)) out.push('主词条与该因子的固定主词条不匹配')
  const allowedPrimaryLevels = Array.isArray(sigil?.allowedPrimaryTraitLevels) && sigil.allowedPrimaryTraitLevels.length
    ? sigil.allowedPrimaryTraitLevels
    : primary?.allowedLevels
  if (Array.isArray(allowedPrimaryLevels) && allowedPrimaryLevels.length && !allowedPrimaryLevels.includes(form.primaryTraitLevel)) out.push('主词条等级不是已知自然等级')
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
  const selectedSigilOption = sigilByHash.value.get(form.sigilHash >>> 0)
  const selectedPrimaryOption = traitByHash.value.get(form.primaryTraitHash >>> 0)
  const selectedSecondaryOption = traitByHash.value.get(form.secondaryTraitHash >>> 0)
  if (!selectedSigilOption || selectedSigilOption.source === 'runtime') reasons.push('因子 Hash 不在本地资料库中')
  if (!selectedPrimaryOption || selectedPrimaryOption.source === 'runtime') reasons.push('主词条 Hash 不在本地资料库中')
  if (form.secondaryTraitHash && (!selectedSecondaryOption || selectedSecondaryOption.source === 'runtime')) reasons.push('副词条 Hash 不在本地资料库中')
  if (reasons.length) return { status: 'impossible', message: `${reasons.join('；')}；后端会拒绝这次写入` }
  const sigil = sigilByHash.value.get(form.sigilHash >>> 0)
  if (form.secondaryTraitHash && (!sigil || !Array.isArray(sigil.allowedSecondaryTraitHashes) || !sigil.allowedSecondaryTraitHashes.length)) {
    return { status: 'unknown', message: '可提交；该因子的完整天然副词条池尚未完全验证，写入前仍由后端校验' }
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
const canWrite = computed(() => !!status.selectedAddr && changedCount.value > 0 &&
  (legality.value.status === 'legal' || legality.value.status === 'unknown'))

function revertToRead() { syncFormFromStatus() }

function applyEntry(entry) {
  if (loading.value || applying.value) return
  form.sigilHash = entry.sigilHash >>> 0
  form.sigilLevel = entry.sigilLevel >>> 0
  form.primaryTraitHash = entry.primaryTraitHash >>> 0
  form.primaryTraitLevel = entry.primaryTraitLevel >>> 0
  form.secondaryTraitHash = normaliseSecondaryHash(entry.secondaryTraitHash)
  form.secondaryTraitLevel = isEmptyTraitHash(entry.secondaryTraitHash) ? 0 : entry.secondaryTraitLevel >>> 0
}
async function applyAndWrite(entry) {
  if (loading.value || applying.value) return
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
onBeforeUnmount(() => {
  disposed = true
  lifecycleEpoch++
  stopPolling()
  document.removeEventListener('mousedown', onRenameOutsideClick)
  const ownerToken = hookOwnerToken
  hookOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, SigilMemoryRelease)
})
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
  if (disposed) return
  await refresh(true)
})
</script>

<template>
  <div class="sigil-memory-page ui-page is-wide ui-page-stack">
    <section class="ui-card ui-panel is-compact connection-card">
      <div class="ui-split connection-row">
        <div class="ui-cluster">
          <span class="ui-tag" :class="status.hooked ? 'is-ok' : status.found ? 'is-info' : ''">{{ statusLabel }}</span>
          <span v-if="!status.hooked && status.found" class="ui-hint">点击启用读取，然后在游戏内选择因子</span>
          <span v-else-if="status.hooked && !status.selectedAddr" class="ui-hint">等待游戏内因子选择</span>
          <span v-else-if="status.selectedAddr" class="ui-hint">已锁定游戏内当前选中的因子</span>
        </div>
        <div class="ui-actions">
          <button v-if="status.hooked" class="ui-btn is-sm is-ghost" :disabled="loading || applying" @click="refresh(true)">{{ loading ? '刷新中…' : '刷新' }}</button>
          <button v-if="status.hooked" class="ui-btn is-sm is-ghost" :disabled="loading || applying" @click="disable">停止读取</button>
          <button class="ui-btn is-sm is-primary" :disabled="loading || applying || status.hooked" @click="enable">启用读取</button>
        </div>
      </div>
    </section>

    <section class="ui-card ui-panel editor-card" :aria-disabled="!status.selectedAddr">
      <div class="ui-split editor-header">
        <h2 class="ui-section-title">因子编辑 <small>当前值与待写入值并列核对</small></h2>
        <div class="ui-actions">
          <button class="ui-btn is-sm is-subtle" :disabled="!status.selectedAddr || loading || applying" @click="saveCurrentAsTemplate" title="保存当前目标为模板，稍后可重命名">保存为模板</button>
          <button class="ui-btn is-sm is-ghost" :disabled="!status.selectedAddr || loading || applying || changedCount === 0" @click="revertToRead" title="放弃修改，恢复为游戏内当前值">还原当前值</button>
        </div>
      </div>

      <div class="editor-fields">
        <div class="editor-field">
          <div class="editor-field-head">
            <strong>因子</strong>
            <span class="current-value" :class="{ 'is-dim': sigilRead.dim }" :title="sigilRead.text">当前：{{ sigilRead.text }} <b v-if="status.sigilHash">Lv {{ status.sigilLevel }}</b></span>
          </div>
          <div class="editor-control-grid">
            <SigilMemoryPicker v-model="form.sigilHash" :options="allSigilOptions" :disabled="!status.selectedAddr || loading || applying" @pick="onPickSigil" placeholder="选择因子" />
            <label class="ui-field level-control">
              <span class="ui-field-label">等级</span>
              <input v-model.number="form.sigilLevel" class="ui-input" :disabled="!status.selectedAddr || loading || applying" type="number" min="0" :max="sigilWritableMax" aria-label="因子等级" @change="form.sigilLevel = clampLevel(form.sigilLevel, sigilWritableMax)" />
              <small v-if="sigilMax != null" class="ui-hint">合规 {{ sigilMax }} / 可写 {{ sigilWritableMax }}</small>
            </label>
            <button class="ui-btn is-sm limit-button" :disabled="!status.selectedAddr || loading || applying || sigilMax == null || sigilAtMax" @click="maxSigil" :title="sigilMax != null ? `设为上限 ${sigilMax}` : '无等级元数据'">设为上限</button>
          </div>
        </div>

        <div class="editor-field">
          <div class="editor-field-head">
            <strong>主词条</strong>
            <span class="trait-preview" aria-hidden="true">
              <img v-if="traitIconByHash(status.primaryTraitHash, status.primaryTraitName)" :src="traitIconByHash(status.primaryTraitHash, status.primaryTraitName)" alt="" />
              <span v-if="traitIconByHash(status.primaryTraitHash, status.primaryTraitName) && traitIconByHash(form.primaryTraitHash)">→</span>
              <img v-if="traitIconByHash(form.primaryTraitHash)" :src="traitIconByHash(form.primaryTraitHash)" alt="" />
            </span>
            <span class="current-value" :class="{ 'is-dim': primaryRead.dim }" :title="primaryRead.text">当前：{{ primaryRead.text }} <b v-if="status.primaryTraitHash">Lv {{ status.primaryTraitLevel }}</b></span>
          </div>
          <div class="editor-control-grid">
            <SigilMemoryPicker v-model="form.primaryTraitHash" :options="allTraitOptions" :icon-resolver="traitOptionIcon" :disabled="!status.selectedAddr || loading || applying" @pick="onPickPrimary" placeholder="选择主词条" />
            <label class="ui-field level-control">
              <span class="ui-field-label">等级</span>
              <input v-model.number="form.primaryTraitLevel" class="ui-input" :disabled="!status.selectedAddr || loading || applying" type="number" min="0" :max="primaryWritableMax" aria-label="主词条等级" @change="form.primaryTraitLevel = clampLevel(form.primaryTraitLevel, primaryWritableMax)" />
              <small v-if="primaryMax != null" class="ui-hint">合规 {{ primaryMax }} / 可写 {{ primaryWritableMax }}</small>
            </label>
            <button class="ui-btn is-sm limit-button" :disabled="!status.selectedAddr || loading || applying || primaryMax == null || primaryAtMax" @click="maxPrimary" :title="primaryMax != null ? `设为上限 ${primaryMax}` : '无等级元数据'">设为上限</button>
          </div>
        </div>

        <div class="editor-field">
          <div class="editor-field-head">
            <strong>副词条</strong>
            <span class="trait-preview" aria-hidden="true">
              <img v-if="traitIconByHash(status.secondaryTraitHash, status.secondaryTraitName)" :src="traitIconByHash(status.secondaryTraitHash, status.secondaryTraitName)" alt="" />
              <span v-if="traitIconByHash(status.secondaryTraitHash, status.secondaryTraitName) && traitIconByHash(form.secondaryTraitHash)">→</span>
              <img v-if="traitIconByHash(form.secondaryTraitHash)" :src="traitIconByHash(form.secondaryTraitHash)" alt="" />
            </span>
            <span class="current-value" :class="{ 'is-dim': secondaryRead.dim }" :title="secondaryRead.text">当前：{{ secondaryRead.text }} <b v-if="status.secondaryTraitHash">Lv {{ status.secondaryTraitLevel }}</b></span>
          </div>
          <div class="editor-control-grid">
            <SigilMemoryPicker v-model="form.secondaryTraitHash" :options="allTraitOptions" :icon-resolver="traitOptionIcon" :disabled="!status.selectedAddr || loading || applying" @pick="onPickSecondary" optional placeholder="未选择（可选）" />
            <label class="ui-field level-control" :class="{ 'is-disabled': !form.secondaryTraitHash }">
              <span class="ui-field-label">等级</span>
              <input v-if="form.secondaryTraitHash" v-model.number="form.secondaryTraitLevel" class="ui-input" :disabled="!status.selectedAddr || loading || applying" type="number" min="0" :max="secondaryWritableMax" aria-label="副词条等级" @change="form.secondaryTraitLevel = clampLevel(form.secondaryTraitLevel, secondaryWritableMax)" />
              <span v-else class="empty-level">未选择</span>
              <small v-if="secondaryMax != null" class="ui-hint">合规 {{ secondaryMax }} / 可写 {{ secondaryWritableMax }}</small>
            </label>
            <button class="ui-btn is-sm limit-button" :disabled="!status.selectedAddr || loading || applying || secondaryMax == null || secondaryAtMax" @click="maxSecondary" :title="secondaryMax != null ? `设为上限 ${secondaryMax}` : '请先选择副词条'">设为上限</button>
          </div>
        </div>
      </div>

      <div v-if="warnings.length" class="ui-notice is-warn warning-list" role="alert">
        <span v-for="(w, i) in warnings" :key="i">{{ w }}</span>
      </div>

      <div class="ui-toolbar write-toolbar">
        <div class="write-summary">
          <LegalityIndicator v-if="status.selectedAddr" :status="legality.status" :message="legality.message" />
          <span v-else class="ui-hint">在游戏内选中一个因子后，这里会显示合法性与写入状态</span>
          <span class="ui-tag" :class="changedCount ? 'is-info' : ''">{{ changeSummary }}</span>
        </div>
        <div class="ui-actions write-actions">
          <button class="ui-btn is-ghost" :disabled="loading || applying || !canOneClickMax" @click="oneClickMax" title="只填写已知上限，不会立即写入">全部设为上限</button>
          <button class="ui-btn is-primary" :disabled="loading || applying || !canWrite" @click="write">{{ applying ? '写入中…' : '写入修改' }}</button>
        </div>
      </div>
    </section>

    <section class="ui-card ui-panel library-card">
      <div class="ui-split library-header">
        <div class="ui-tabs" role="tablist" aria-label="因子模板与写入历史">
          <button class="ui-tab" :class="{ 'is-on': tab === 'templates' }" role="tab" :aria-selected="tab === 'templates'" @click="tab = 'templates'">模板 <span class="ui-tag">{{ templates.length }}</span></button>
          <button class="ui-tab" :class="{ 'is-on': tab === 'history' }" role="tab" :aria-selected="tab === 'history'" @click="tab = 'history'">最近写入 <span class="ui-tag">{{ history.length }}</span></button>
        </div>
        <div v-if="tab === 'templates'">
          <input v-model="templateSearch" class="ui-input template-search" aria-label="搜索模板" placeholder="搜索模板…" />
        </div>
        <button v-else-if="history.length" class="row-tool ui-btn is-sm is-ghost" title="清空最近写入" @click="clearWriteHistory">清空记录</button>
      </div>

      <div v-if="tab === 'templates'">
        <div v-if="!filteredTemplates.length" class="ui-empty">
          {{ templates.length ? '无匹配模板' : '尚无模板 · 在编辑器点击 "＋ 保存为模板"' }}
        </div>
        <ul v-else class="ui-list template-list">
          <li v-for="t in filteredTemplates" :key="t.id" class="ui-row template-entry" role="button" tabindex="0" @click="applyEntry(t)" @keydown.enter="applyEntry(t)" @keydown.space.prevent="applyEntry(t)">
            <span class="entry-name">
              <template v-if="renamingId === t.id">
                <span class="rename-group" ref="renameEl">
                  <input v-model="renameBuffer" class="ui-input rename-input" aria-label="模板名称" @click.stop @keydown.enter="confirmRename" @keydown.escape="cancelRename" />
                  <button class="ui-btn is-icon is-sm" :disabled="!renameBuffer.trim()" @click.stop="confirmRename" title="保存名称">✓</button>
                  <button class="ui-btn is-icon is-sm is-ghost" @click.stop="cancelRename" title="取消重命名">×</button>
                </span>
              </template>
              <template v-else>
                <strong>{{ t.name }}</strong>
                <span class="ui-tag">Lv {{ t.sigilLevel }}</span>
              </template>
            </span>
            <span class="trait-line">
              <img v-if="traitIconByHash(t.primaryTraitHash)" class="trait-line-icon" :src="traitIconByHash(t.primaryTraitHash)" alt="" />
              <span class="ui-tag is-info">主</span>
              <span>{{ nameFor(traitByHash, t.primaryTraitHash, '—') }}</span>
              <b>Lv {{ t.primaryTraitLevel }}</b>
            </span>
            <span class="trait-line" :class="{ 'is-empty': !t.secondaryTraitHash }">
              <img v-if="traitIconByHash(t.secondaryTraitHash)" class="trait-line-icon" :src="traitIconByHash(t.secondaryTraitHash)" alt="" />
              <span class="ui-tag">副</span>
              <span>{{ t.secondaryTraitHash ? nameFor(traitByHash, t.secondaryTraitHash) : '无副词条' }}</span>
              <b v-if="t.secondaryTraitHash">Lv {{ t.secondaryTraitLevel }}</b>
            </span>
            <span class="row-tools ui-actions" @click.stop>
              <button class="row-tool ui-btn is-sm is-ghost" title="重命名" @click="startRename(t.id, t.name)">重命名</button>
              <button class="row-tool ui-btn is-sm is-ghost" title="删除" @click="deleteTemplate(t.id)">删除</button>
            </span>
            <button class="ui-btn is-sm is-primary entry-apply" :disabled="!status.selectedAddr || loading || applying" @click.stop="applyAndWrite(t)" title="立即应用并写入">一键应用</button>
          </li>
        </ul>
      </div>

      <div v-else>
        <div v-if="!history.length" class="ui-empty">尚无历史</div>
        <ul v-else class="ui-list template-list">
          <li v-for="h in history" :key="h.id" class="ui-row template-entry" role="button" tabindex="0" @click="applyEntry(h)" @keydown.enter="applyEntry(h)" @keydown.space.prevent="applyEntry(h)">
            <span class="entry-name">
              <strong>{{ nameFor(sigilByHash, h.sigilHash, '—') }}</strong>
              <span class="ui-tag">Lv {{ h.sigilLevel }}</span>
            </span>
            <span class="trait-line">
              <img v-if="traitIconByHash(h.primaryTraitHash)" class="trait-line-icon" :src="traitIconByHash(h.primaryTraitHash)" alt="" />
              <span class="ui-tag is-info">主</span><span>{{ nameFor(traitByHash, h.primaryTraitHash, '—') }}</span><b>Lv {{ h.primaryTraitLevel }}</b>
            </span>
            <span class="trait-line" :class="{ 'is-empty': !h.secondaryTraitHash }">
              <img v-if="traitIconByHash(h.secondaryTraitHash)" class="trait-line-icon" :src="traitIconByHash(h.secondaryTraitHash)" alt="" />
              <span class="ui-tag">副</span><span>{{ h.secondaryTraitHash ? nameFor(traitByHash, h.secondaryTraitHash) : '无副词条' }}</span><b v-if="h.secondaryTraitHash">Lv {{ h.secondaryTraitLevel }}</b>
            </span>
            <span class="entry-meta">{{ fmtRelTime(h.createdAt) }}</span>
            <button class="ui-btn is-sm is-primary entry-apply" :disabled="!status.selectedAddr || loading || applying" @click.stop="applyAndWrite(h)" title="立即应用并写入">一键应用</button>
          </li>
        </ul>
      </div>
    </section>
  </div>
  <ConfirmDialog ref="confirmDialog" />
</template>

<style scoped>
.sigil-memory-page { padding-bottom:var(--space-9); }
.connection-card { position:sticky; top:0; z-index:4; }
.editor-header { padding-bottom:var(--space-4); border-bottom:1px solid var(--border-soft); }
.editor-fields { display:flex; flex-direction:column; }
.editor-field { display:flex; min-width:0; flex-direction:column; gap:var(--space-3); padding:var(--space-4) 0; }
.editor-field + .editor-field { border-top:1px solid var(--border-soft); }
.editor-field-head { display:flex; min-width:0; flex-wrap:wrap; align-items:baseline; gap:var(--space-2) var(--space-4); }
.editor-field-head > strong { flex:0 0 74px; font-size:var(--fs-md); }
.trait-preview { display:inline-flex; min-height:30px; align-items:center; gap:var(--space-1); color:var(--text-muted); font-family:var(--font-data); }
.trait-preview img,.trait-line-icon { width:30px; height:30px; flex:0 0 30px; object-fit:cover; border:1px solid var(--line-soft); border-radius:6px; background:var(--surface-field); }
.current-value { min-width:0; color:var(--text-secondary); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.current-value b { color:var(--text-primary); font-family:var(--font-data); }
.current-value.is-dim { color:var(--text-muted); }
.editor-control-grid { display:grid; min-width:0; max-width:760px; grid-template-columns:minmax(240px,520px) 124px 88px; align-items:start; gap:var(--space-3); }
.level-control { display:grid; grid-template-rows:auto var(--control-height) auto; }
.level-control .ui-input { text-align:center; }
.level-control.is-disabled { opacity:var(--state-disabled-opacity); }
.empty-level { display:flex; min-height:var(--control-height); align-items:center; justify-content:center; border:1px solid var(--border-soft); border-radius:var(--radius-sm); color:var(--text-muted); background:var(--surface-sunken); font-size:var(--fs-sm); }
.limit-button { align-self:end; min-height:var(--control-height); }
.warning-list { display:flex; flex-direction:column; gap:var(--space-1); }
.write-toolbar { align-items:center; justify-content:space-between; }
.write-summary { display:flex; min-width:0; flex:1 1 360px; flex-wrap:wrap; align-items:center; gap:var(--space-3); }
.write-actions { margin-left:auto; }
.library-header { align-items:center; }
.template-search { width:clamp(190px,25cqi,280px); }
.template-list { margin:0; padding:0; list-style:none; }
.template-entry { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-2) var(--space-4); cursor:pointer; }
.template-entry:focus-visible { outline:var(--focus-outline); outline-offset:var(--focus-offset); }
.entry-name { grid-column:1; display:flex; min-width:0; flex-wrap:wrap; align-items:center; gap:var(--space-2); }
.entry-name strong { overflow-wrap:anywhere; }
.trait-line { grid-column:1; display:flex; min-width:0; flex-wrap:wrap; align-items:center; gap:var(--space-2); color:var(--text-secondary); font-size:var(--fs-sm); }
.trait-line b { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); }
.trait-line.is-empty { color:var(--text-muted); }
.trait-line-icon { width:26px; height:26px; flex-basis:26px; }
.row-tools { grid-column:2; grid-row:1 / span 2; align-self:start; justify-self:end; }
.entry-apply { grid-column:2; align-self:end; justify-self:end; }
.entry-meta { grid-column:2; grid-row:1; color:var(--text-muted); font-size:var(--fs-sm); white-space:nowrap; }
.rename-group { display:flex; min-width:0; flex:1; align-items:center; gap:var(--space-2); }
.rename-input { flex:1 1 220px; }

@container ui-page (max-width:760px) {
  .editor-control-grid { max-width:none; grid-template-columns:minmax(0,1fr) 112px 84px; }
  .template-entry { grid-template-columns:minmax(0,1fr); }
  .row-tools,.entry-apply,.entry-meta { grid-column:1; grid-row:auto; justify-self:start; }
  .entry-apply { width:100%; }
}

@container ui-page (max-width:560px) {
  .connection-card { position:static; }
  .editor-control-grid { grid-template-columns:minmax(0,1fr) 108px; }
  .limit-button { grid-column:1 / -1; width:100%; }
  .editor-field-head > strong { flex-basis:100%; }
  .write-actions { width:100%; margin-left:0; }
  .write-actions .ui-btn { flex:1 1 180px; }
  .library-header { align-items:stretch; }
  .template-search { width:100%; }
}
</style>
