<script setup>
import { computed, defineAsyncComponent, nextTick, onMounted, ref, watch } from 'vue'
import { FindSaveFiles, LoadoutList, LoadoutPreviewList, ParseLogsLoadoutJSON, PublishLoadoutShare, PublishLoadoutShareNamed, PublishLogsLoadoutShare, SelectLogsLoadoutJSON, SelectLogsLoadoutShares, SelectLogsLoadoutSharesFresh, SelectProgressionSave } from '../../wailsjs/go/backend/App'
import { characterIdentityByPLID } from '../characterRoster.js'
import { characterAssetIcon, traitAssetIcon, weaponAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'
import { planAtlasOptimizerRoute } from '../loadoutAtlasOptimizerRoute.js'
import skillIconFiles from '../loadoutSkillIcons.json'
import { createOperationGate } from '../runtimeOperationGate.js'
import { copyShareText, loadoutShareSessionKey, publishedLoadoutShare, rememberPublishedLoadoutShare } from '../loadoutShareSession.js'
import CapturedLoadoutPreview from './CapturedLoadoutPreview.vue'
import LoadoutPublishDialog from './LoadoutPublishDialog.vue'
import SaveSourcePicker from './SaveSourcePicker.vue'

const LoadoutEditor = defineAsyncComponent(() => import('./LoadoutEditor.vue'))
const SigilAtlas = defineAsyncComponent(() => import('./SigilAtlas.vue'))
const LoadoutShareWorkshop = defineAsyncComponent(() => import('./LoadoutShareWorkshop.vue'))
const LogsBattleArchive = defineAsyncComponent(() => import('./LogsBattleArchive.vue'))

const props = defineProps({ pendingImport: { type: Object, default: null } })
const emit = defineEmits(['status', 'editing-change', 'import-consumed'])

const slots = ref([])
const savePath = ref('')
const loading = ref(false)
const groups = ref([])
const selectedChara = ref('')
const expanded = ref(new Set())
const mode = ref('view') // view | edit | logs | logs-preview
const previews = ref(new Map())
const previewLoading = ref(false)
const logsCandidates = ref([])
const logsLoading = ref(false)
const logsSourceKind = ref('')
const selectedLogsCandidate = ref(null)
const viewerRoot = ref(null)
const logsPendingImport = ref(null)
const logsJSONOpen = ref(false)
const logsJSONPayload = ref('')
const logsJSONError = ref('')
const logsJSONTextarea = ref(null)
const logsPublishCandidate = ref(null)
const logsPublishTitle = ref('')
const logsPublishBusy = ref(false)
const logsPublishError = ref('')
const logsPublishResult = ref(null)
const logsPublishGate = createOperationGate()
const cardPublishTarget = ref(null)
const cardPublishTitle = ref('')
const cardPublishBusy = ref(false)
const cardPublishError = ref('')
const cardPublishResult = ref(null)
const cardPublishGate = createOperationGate()
const pendingAtlasConstruct = ref(null)
const pendingOptimizerTarget = ref(null)
const editorTargetUnitId = ref(0)
const activeCardTool = ref(null)
let previewRequestId = 0

const CAT_LABELS = { SB_ATK: '真谛（攻击盘）', SB_DEF: '觉醒（防御盘）', SB_LIMIT: '秘义（界限盘）' }
const LOGS_FIELD_LABELS = {
  stats: ['面板', 'Panel'], sigils: ['因子', 'Sigils'], skills: ['角色技能', 'Abilities'], summons: ['召唤石', 'Summons'], weapon: ['武器', 'Weapon'],
  weaponSkills: ['武器技能', 'Weapon Skills'], wrightstone: ['武器祝福', 'Wrightstone'], mastery: ['专精', 'Mastery'], overLimit: ['上限突破', 'Overmastery'],
}

const tx = (zh, en) => language.value === 'en' ? en : zh
function catLabel(cat) { return CAT_LABELS[cat] || '基础盘' }
function assetPath(folder, file) {
  if (!file) return ''
  return `/loadout-icons/${folder}/${String(file).split('/').map(part => encodeURIComponent(part).replace(/'/g, '%27')).join('/')}`
}
function skillIcon(skill) {
  const verifiedFile = skillIconFiles[skill?.key || ''] || ''
  return assetPath('skills', verifiedFile || 'Plain_Skill_Frame.png')
}
function traitIcon(name, hash = '') { return traitAssetIcon({ name, hash }) }
function loadoutWeaponIcon(loadout) {
  const weapon = loadout?.weapon || {}
  return weaponAssetIcon({
    internalId: weapon.internalId,
    baseHash: weapon.baseHash,
    storedHash: weapon.storedHash,
    hash: loadout?.weaponHash || weapon.storedHash,
  })
}
function logsFieldLabel(field) {
  const labels = LOGS_FIELD_LABELS[field]
  return labels ? tx(labels[0], labels[1]) : field
}
function logsMissingText(fields) {
  return `${tx('未记录', 'Not captured')}: ${(fields || []).map(logsFieldLabel).join(language.value === 'en' ? ', ' : '、')}`
}
function logsLoadedText(count) { return count ? tx(`已载入 ${count} 套配装`, `${count} loadouts loaded`) : tx('等待导入来源', 'Waiting for an import source') }
function logsTime(value) { return new Date(value).toLocaleString(language.value === 'en' ? 'en-US' : 'zh-CN') }

const currentGroup = computed(() => groups.value.find(g => g.charaName === selectedChara.value) || null)
const isEditing = computed(() => mode.value === 'edit' && !!currentGroup.value)
const effectivePendingImport = computed(() => props.pendingImport || logsPendingImport.value)
const pendingCodeForEditor = computed(() => {
  if (!effectivePendingImport.value?.code || !currentGroup.value) return ''
  return String(currentGroup.value.charaHash).toUpperCase() === String(effectivePendingImport.value.charaHash || '').toUpperCase()
    ? effectivePendingImport.value.code
    : ''
})
const workspaceModes = computed(() => [
  { id: 'view', label: tx('角色配装', 'Character Loadouts'), description: tx('查看、编辑与智能配装', 'View, edit, and smart-build'), mark: '01' },
  { id: 'atlas', label: tx('因子图鉴', 'Sigil Atlas'), description: tx('查技能、壳与合法组合', 'Browse traits and legal shells'), mark: '02' },
  { id: 'battles', label: tx('战斗档案', 'Battle Archive'), description: tx('当前会话与 Logs 历史', 'Current session and Logs history'), mark: '03' },
])
function scrollWorkspaceModes(event) {
  const target = event.currentTarget
  if (!target || target.scrollWidth <= target.clientWidth || Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return
  target.scrollLeft += event.deltaY
  event.preventDefault()
}
const presetCount = computed(() => {
  let n = 0
  for (const g of groups.value) for (const lo of g.loadouts) if (!lo.isParty) n++
  return n
})
function previewFor(loadout) { return previews.value.get(loadout?.unitId) || null }
function formatNumber(value, digits = 0) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '—'
  return numeric.toLocaleString('zh-CN', { maximumFractionDigits: digits })
}
async function loadPreviews() {
  if (!savePath.value || !currentGroup.value) return
  const path = savePath.value
  const charaHash = currentGroup.value.charaHash
  const requestId = ++previewRequestId
  previewLoading.value = true
  try {
    const result = await LoadoutPreviewList(path, charaHash)
    if (requestId !== previewRequestId || path !== savePath.value || charaHash !== currentGroup.value?.charaHash) return
    previews.value = new Map((result || []).map(entry => [entry.unitId, entry]))
  } catch (err) {
    if (requestId !== previewRequestId) return
    previews.value = new Map()
    emit('status', `配装数值预览失败：${String(err)}`, 'error')
  } finally {
    if (requestId === previewRequestId) previewLoading.value = false
  }
}

function masterySummary(lo) {
  const order = ['R1', 'R2', 'R3', 'EX']
  const byRank = new Map()
  for (const m of lo.mastery || []) {
    const rank = m.rank || 'unknown'
    const current = byRank.get(rank) || { rank, label: m.rankLabel || rank, count: 0 }
    current.count += 1
    byRank.set(rank, current)
  }
  return [...byRank.values()].sort((a, b) => order.indexOf(a.rank) - order.indexOf(b.rank))
}

function masteryGrouped(lo) {
  const byRankAndCat = new Map()
  for (const m of lo.mastery || []) {
    const key = `${m.rank || 'unknown'}:${m.cat || ''}`
    if (!byRankAndCat.has(key)) byRankAndCat.set(key, { key, rankLabel: m.rankLabel || m.rank, cat: m.cat, nodes: [] })
    byRankAndCat.get(key).nodes.push(m)
  }
  return [...byRankAndCat.values()]
}

function toggle(lo) {
  const next = new Set(expanded.value)
  if (next.has(lo.unitId)) next.delete(lo.unitId)
  else next.add(lo.unitId)
  expanded.value = next
}

function enterEdit(unitId = 0) {
  if (!savePath.value || !currentGroup.value) return
  const requested = Number(unitId)
  editorTargetUnitId.value = Number.isFinite(requested) && requested > 0 ? requested : 0
  pendingOptimizerTarget.value = null
  mode.value = 'edit'
}

function leaveEdit() {
  mode.value = 'view'
}

function selectWorkspaceMode(value) {
  mode.value = value
}

function openCardTool(loadout, tool) {
  if (!loadout) return
  const next = new Set(expanded.value)
  next.add(loadout.unitId)
  expanded.value = next
  activeCardTool.value = { unitId: loadout.unitId, tool }
}

function closeCardTool() { activeCardTool.value = null }
function cardToolOpen(loadout, tool) { return activeCardTool.value?.unitId === loadout?.unitId && activeCardTool.value?.tool === tool }
function shareGroup(loadout) { return currentGroup.value && loadout ? { ...currentGroup.value, loadouts: [loadout] } : null }
function cardShareSessionKey(loadout) {
  if (!loadout || !currentGroup.value) return ''
  return loadoutShareSessionKey({
    savePath: savePath.value,
    charaHash: currentGroup.value.charaHash,
    unitId: loadout.unitId,
  })
}
function publishedShareFor(loadout) {
  return publishedLoadoutShare(cardShareSessionKey(loadout))
}
function openCardPublish(loadout) {
  if (!loadout || loadout.isParty || cardPublishBusy.value) return
  cardPublishGate.invalidate()
  cardPublishTarget.value = loadout
  cardPublishTitle.value = loadout.name || tx(
    `${currentGroup.value?.charaName || '角色'} · 槽 ${String(loadout.slot).padStart(2, '0')}`,
    `${currentGroup.value?.charaName || 'Character'} · Slot ${String(loadout.slot).padStart(2, '0')}`,
  )
  cardPublishError.value = ''
  cardPublishResult.value = publishedShareFor(loadout)
}
function closeCardPublish() {
  cardPublishGate.invalidate()
  cardPublishTarget.value = null
  cardPublishError.value = ''
  cardPublishResult.value = null
}
async function copyCardPublishedLink(result) {
  if (!result?.url) return
  try {
    await copyShareText(result.url)
    emit('status', tx(`已复制配装链接：${result.code}`, `Copied loadout link: ${result.code}`), 'success')
  } catch (error) {
    const message = tx(`复制失败，请手动复制链接：${String(error)}`, `Copy failed; copy the link manually: ${String(error)}`)
    cardPublishError.value = message
    emit('status', message, 'error')
  }
}
async function publishCardLoadout() {
  const target = cardPublishTarget.value
  if (!target || target.isParty || cardPublishBusy.value) return
  const cached = publishedShareFor(target)
  if (cached) {
    cardPublishResult.value = cached
    await copyCardPublishedLink(cached)
    return
  }
  const operation = cardPublishGate.begin('publish')
  if (!operation) return
  const title = cardPublishTitle.value.trim()
  const sessionKey = cardShareSessionKey(target)
  cardPublishBusy.value = true
  cardPublishError.value = ''
  try {
    const published = title
      ? await PublishLoadoutShareNamed(savePath.value, target.unitId, title)
      : await PublishLoadoutShare(savePath.value, target.unitId)
    rememberPublishedLoadoutShare(sessionKey, published)
    if (!cardPublishGate.isCurrent(operation) || cardPublishTarget.value !== target) return
    cardPublishResult.value = published
    await copyCardPublishedLink(published)
  } catch (error) {
    if (cardPublishGate.isCurrent(operation)) cardPublishError.value = String(error)
  } finally {
    cardPublishBusy.value = false
    cardPublishGate.finish(operation)
  }
}
function firstEditableLoadout() {
  return currentGroup.value?.loadouts?.find(item => !item.isParty) || null
}
function optimizerOwnerNames(ownerCodes) {
  return (ownerCodes || []).map(ownerCode => {
    const identity = characterIdentityByPLID(ownerCode)
    return identity ? (language.value === 'en' ? identity.nameEn : identity.nameZh) : ownerCode
  }).join(language.value === 'en' ? ' / ' : '／')
}
async function openAtlasOptimizer(payload) {
  const route = planAtlasOptimizerRoute({
    savePath: savePath.value,
    groups: groups.value,
    currentGroup: currentGroup.value,
    payload,
    requestId: Date.now(),
  })
  if (route.kind === 'needs-save') {
    mode.value = 'view'
    emit('status', tx('请先在角色配装页选择目标存档和角色，再回到因子图鉴点击“送入优化目标”', 'Choose a target save and character on the Character Loadouts page, then return to the atlas and choose “Use as Optimization Target”'), 'info')
    return
  }
  if (route.kind === 'needs-character') {
    mode.value = 'view'
    emit('status', tx('请先选择要优化的角色；通用因子会送入该角色的配装详情', 'Choose the character to optimize first; general sigils will open that character’s loadout details'), 'info')
    return
  }
  if (route.kind === 'missing-owner') {
    const names = optimizerOwnerNames(route.missingOwnerCodes)
    mode.value = 'view'
    emit('status', tx(
      `该因子只适用于 ${names}，当前存档没有对应角色配装。请先在游戏中为该角色保存一套配装，再重新读取存档`,
      `This sigil is limited to ${names}, but the current save has no loadout for that character. Save a loadout for that character in game, then reload the save`,
    ), 'info')
    return
  }
  if (route.kind === 'no-editable-slot') {
    mode.value = 'view'
    emit('status', tx(
      `${route.targetGroup?.charaName || '目标角色'}没有可编辑的存档配装槽。请先在游戏中保存一套角色配装，再重新读取存档`,
      `${route.targetGroup?.charaName || 'The target character'} has no editable saved loadout slot. Save a character loadout in game, then reload the save`,
    ), 'info')
    return
  }

  if (selectedChara.value !== route.targetGroup.charaName) {
    selectedChara.value = route.targetGroup.charaName
    // 切角色的 watcher 会清理上一角色的待处理状态，等它完成后再交付新意图。
    await nextTick()
  }
  pendingOptimizerTarget.value = route.intent
  editorTargetUnitId.value = Number(route.loadout.unitId || 0)
  mode.value = 'edit'
  await nextTick()
}

async function constructFromAtlas(payload) {
  if (!savePath.value || !currentGroup.value) {
    emit('status', tx('请先在配装预设页选择存档和角色', 'Choose a save and character on the preset page first'), 'info')
    mode.value = 'view'
    return
  }
  pendingAtlasConstruct.value = { ...payload, requestId: Date.now() }
  mode.value = 'edit'
  await nextTick()
}

async function activatePendingImport() {
  const pending = effectivePendingImport.value
  if (!pending?.code) return
  if (!savePath.value || !groups.value.length) {
	selectedLogsCandidate.value = null
	mode.value = 'view'
	emit('status', `已暂存 ${pending.charaName || '角色'} 配装，请先选择目标存档`, 'info')
	return
  }
  const target = groups.value.find(group => String(group.charaHash).toUpperCase() === String(pending.charaHash || '').toUpperCase())
  if (!target) {
    emit('status', `目标存档没有 ${pending.charaName || pending.charaHash} 的配装槽`, 'error')
    return
  }
  selectedChara.value = target.charaName
  mode.value = 'edit'
  await nextTick()
}

async function browseLogsLoadouts() {
	if (logsLoading.value) return
	logsLoading.value = true
	try {
		logsCandidates.value = await (logsSourceKind.value === 'database' ? SelectLogsLoadoutSharesFresh() : SelectLogsLoadoutShares()) || []
		logsSourceKind.value = 'database'
		mode.value = 'logs'
	} catch (error) {
		emit('status', String(error), 'error')
	} finally {
		logsLoading.value = false
	}
}

function openLogsJSONImport() {
	logsJSONError.value = ''
	logsJSONOpen.value = true
	nextTick(() => logsJSONTextarea.value?.focus())
}

function closeLogsJSONImport() {
	if (logsLoading.value) return
	logsJSONOpen.value = false
	logsJSONError.value = ''
}

function acceptLogsJSONCandidates(candidates) {
	logsCandidates.value = candidates || []
	logsSourceKind.value = 'json'
	selectedLogsCandidate.value = null
	mode.value = 'logs'
	logsJSONOpen.value = false
	logsJSONPayload.value = ''
	logsJSONError.value = ''
	emit('status', tx(`已解析 ${logsCandidates.value.length} 套 Relink Logs 角色配装`, `Parsed ${logsCandidates.value.length} Relink Logs character loadouts`), 'success')
}

async function readLogsJSONClipboard() {
	logsJSONError.value = ''
	try {
		if (!navigator.clipboard?.readText) throw new Error(tx('当前系统不允许直接读取剪贴板，请手动粘贴。', 'Clipboard reading is unavailable; paste the JSON manually.'))
		logsJSONPayload.value = await navigator.clipboard.readText()
	} catch (error) {
		logsJSONError.value = String(error)
	}
}

async function browseLogsJSONFile() {
	if (logsLoading.value) return
	logsLoading.value = true
	logsJSONError.value = ''
	try {
		const candidates = await SelectLogsLoadoutJSON()
		if (candidates?.length) acceptLogsJSONCandidates(candidates)
	} catch (error) {
		logsJSONError.value = String(error)
	} finally {
		logsLoading.value = false
	}
}

async function parseLogsJSON() {
	if (logsLoading.value) return
	if (!logsJSONPayload.value.trim()) {
		logsJSONError.value = tx('请先粘贴角色 JSON，或从文件读取。', 'Paste character JSON or choose a file first.')
		return
	}
	logsLoading.value = true
	logsJSONError.value = ''
	try {
		logsCandidates.value = await ParseLogsLoadoutJSON(logsJSONPayload.value) || []
		acceptLogsJSONCandidates(logsCandidates.value)
	} catch (error) {
		logsJSONError.value = String(error)
	} finally {
		logsLoading.value = false
	}
}

function openLogsLibrary() {
	selectedLogsCandidate.value = null
	mode.value = 'logs'
}

function closeLogsLibrary() {
	selectedLogsCandidate.value = null
	mode.value = 'view'
}

function previewLogsCandidate(candidate) {
	selectedLogsCandidate.value = candidate
	mode.value = 'logs-preview'
	nextTick(() => viewerRoot.value?.closest('.tool-center-scroll,.workspace-scroll')?.scrollTo({ top: 0 }))
}

function leaveLogsPreview() {
	selectedLogsCandidate.value = null
	mode.value = 'logs'
}

async function deployLogsCandidate(candidate) {
	logsPendingImport.value = {
		code: candidate.compatibilityCode,
		charaHash: candidate.characterHash,
		charaName: candidate.characterName,
		requestId: Date.now(),
	}
	await activatePendingImport()
}

function openLogsPublish(candidate) {
	logsPublishGate.invalidate()
	logsPublishCandidate.value = candidate
	logsPublishTitle.value = tx(`${candidate.characterName} · Logs 配装`, `${candidate.characterName} · Logs Loadout`)
	logsPublishError.value = ''
	logsPublishResult.value = publishedLoadoutShare(loadoutShareSessionKey({ compatibilityCode: candidate.compatibilityCode }))
}

function closeLogsPublish() {
	logsPublishGate.invalidate()
	logsPublishCandidate.value = null
	logsPublishError.value = ''
	logsPublishResult.value = null
}

async function copyLogsPublishedLink(result = logsPublishResult.value) {
	const value = result?.url
	if (!value) return
	try {
		await copyShareText(value)
		emit('status', tx(`已复制配装链接：${result.code}`, `Copied loadout link: ${result.code}`), 'success')
	} catch (error) {
		logsPublishError.value = tx(`复制失败，请手动复制链接：${String(error)}`, `Copy failed; copy the link manually: ${String(error)}`)
	}
}

async function publishLogsCandidate() {
	if (!logsPublishCandidate.value || logsPublishBusy.value) return
	const candidate = logsPublishCandidate.value
	const cacheKey = candidate.compatibilityCode
	const sessionKey = loadoutShareSessionKey({ compatibilityCode: cacheKey })
	const cached = publishedLoadoutShare(sessionKey)
	if (cached) {
		logsPublishResult.value = cached
		await copyLogsPublishedLink(cached)
		return
	}
	const operation = logsPublishGate.begin('publish')
	if (!operation) return
	logsPublishBusy.value = true
	logsPublishError.value = ''
	try {
		const published = await PublishLogsLoadoutShare(candidate, logsPublishTitle.value.trim())
		rememberPublishedLoadoutShare(sessionKey, published)
		if (!logsPublishGate.isCurrent(operation) || logsPublishCandidate.value !== candidate) return
		logsPublishResult.value = published
		await copyLogsPublishedLink(published)
	} catch (error) {
		if (logsPublishGate.isCurrent(operation)) logsPublishError.value = String(error)
	} finally {
		logsPublishBusy.value = false
		logsPublishGate.finish(operation)
	}
}

function consumePendingImport() {
	if (logsPendingImport.value) logsPendingImport.value = null
	else emit('import-consumed')
}

watch(isEditing, value => emit('editing-change', value), { immediate: true })
watch(currentGroup, value => {
  if (!value && mode.value === 'edit') mode.value = 'view'
  previews.value = new Map()
  previewRequestId++
  previewLoading.value = false
  activeCardTool.value = null
  editorTargetUnitId.value = 0
  pendingOptimizerTarget.value = null
  if (value && savePath.value) loadPreviews()
})

async function load(path) {
  if (!path) return
  loading.value = true
  try {
    const result = await LoadoutList(path)
    groups.value = result || []
    // 展开态按 unitId 记录，而 unitId 在不同存档间会复用，
    // 换档后必须清空，否则新档的同号卡片会凭空展开。
    if (savePath.value !== path) expanded.value = new Set()
    savePath.value = path
    if (!groups.value.find(g => g.charaName === selectedChara.value)) {
      const richest = [...groups.value].sort((a, b) => {
        const score = group => Math.max(0, ...(group.loadouts || []).filter(item => !item.isParty).map(item =>
          (item.mastery?.length || 0) * 100 + (item.sigils?.length || 0) * 10 + (item.skills?.length || 0)
        ))
        return score(b) - score(a)
      })[0]
      selectedChara.value = richest?.charaName || ''
    }
    emit('status', `已读取 ${groups.value.length} 个角色、${presetCount.value} 套配装预设`, 'success')
		await activatePendingImport()
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    loading.value = false
  }
}

async function browse() {
  try {
    const path = await SelectProgressionSave()
    if (path) await load(path)
  } catch (err) {
    emit('status', String(err), 'error')
  }
}

onMounted(async () => {
  try {
    slots.value = (await FindSaveFiles()) || []
  } catch { /* 找不到默认存档目录时静默，仍可手动浏览 */ }
})

watch(() => effectivePendingImport.value?.requestId, () => { activatePendingImport() })

</script>

<template>
  <div ref="viewerRoot" class="loadout-viewer ui-page is-wide ui-page-stack" :class="{ editing: isEditing }">
    <nav v-if="!isEditing && !String(mode).startsWith('logs')" class="loadout-subnav" :aria-label="tx('配装工作台功能', 'Loadout Workspace Features')" @wheel="scrollWorkspaceModes">
      <button v-for="item in workspaceModes" :key="item.id" type="button" :class="{ active: mode === item.id }" :disabled="item.disabled" :title="item.description" @click="selectWorkspaceMode(item.id)"><b>{{ item.label }}</b></button>
    </nav>
    <section v-if="isEditing" class="editor-workspace ui-page is-fluid" aria-label="配装编辑工作区">
      <header class="editor-workspace-bar ui-card">
        <button type="button" class="back-button ui-btn" @click="leaveEdit">
          <span aria-hidden="true">←</span> 返回配装列表
        </button>
        <div class="editor-workspace-title">
          <small>角色配装工作台</small>
          <strong>{{ currentGroup.charaName }}</strong>
          <span>构造或从真实背包选择资源，配置因子、技能与三方向专精</span>
        </div>
        <div class="editor-workspace-meta">
          <span class="preset-count-badge"><b>{{ currentGroup.loadouts.filter(l => !l.isParty).length }}</b> 套已有预设</span>
          <small :title="savePath">{{ savePath }}</small>
        </div>
      </header>
      <div class="editor-workspace-content">
        <LoadoutEditor :save-path="savePath" :chara-hash="currentGroup.charaHash" :chara-name="currentGroup.charaName"
          :loadouts="currentGroup.loadouts" :pending-import-code="pendingCodeForEditor" :pending-atlas-construct="pendingAtlasConstruct"
          :pending-optimizer-target="pendingOptimizerTarget" :preferred-unit-id="editorTargetUnitId"
          @import-consumed="consumePendingImport" @status="(m, t) => emit('status', m, t)" @reload="load(savePath)" />
      </div>
    </section>

	<section v-else-if="mode === 'logs-preview' && selectedLogsCandidate" class="logs-workspace" :aria-label="tx('Logs 配装预览', 'Logs Loadout Preview')">
	  <header class="subpage-bar ui-card">
		<button type="button" class="back-button ui-btn" @click="leaveLogsPreview"><span aria-hidden="true">←</span> {{ tx('返回 Logs 配装库', 'Back to Logs Library') }}</button>
		<div><small>{{ tx('GBFR Logs 配装预览', 'GBFR Logs Loadout Preview') }}</small><strong>{{ selectedLogsCandidate.characterName }} · {{ selectedLogsCandidate.playerName || tx('未记录玩家名', 'Player Name Not Recorded') }}</strong></div>
	  </header>
	  <div class="logs-preview-provenance ui-card is-flat">
		<div><small>{{ tx('检测协议', 'Detected Protocol') }}</small><strong>{{ selectedLogsCandidate.protocolLabel }}</strong></div>
		<div class="logs-capability-list" :aria-label="tx('日志已捕获范围', 'Captured Log Fields')"><span v-for="field in selectedLogsCandidate.capturedFields || []" :key="field">{{ logsFieldLabel(field) }}</span></div>
		<p v-if="selectedLogsCandidate.missingFields?.length">{{ logsMissingText(selectedLogsCandidate.missingFields) }}</p>
		<p v-for="warning in selectedLogsCandidate.warnings || []" :key="warning" class="is-warning">{{ warning }}</p>
	  </div>
	  <CapturedLoadoutPreview :loadout="selectedLogsCandidate.preview" :source-label="`${selectedLogsCandidate.protocolLabel} · ${tx('最终导入预览', 'Final Import Preview')}`">
		<template #actions>
		  <button type="button" class="ui-btn is-primary" @click="deployLogsCandidate(selectedLogsCandidate)">{{ tx('选择存档并导入', 'Choose Save & Import') }}</button>
		</template>
	  </CapturedLoadoutPreview>
	</section>

	<section v-else-if="mode === 'logs'" class="logs-workspace" :aria-label="tx('GBFR Logs 配装库', 'GBFR Logs Library')">
	  <header class="logs-library-header ui-card">
		<div class="logs-library-nav">
		  <button type="button" class="back-button logs-back-button ui-btn is-ghost is-sm" @click="closeLogsLibrary"><span aria-hidden="true">←</span> {{ tx('返回配装预设', 'Back to Loadout Presets') }}</button>
		  <span class="logs-source-status" :class="{ ready: logsCandidates.length }"><i aria-hidden="true"></i>{{ logsLoadedText(logsCandidates.length) }}</span>
		</div>
		<div class="logs-library-main">
		  <span class="logs-library-seal" aria-hidden="true">L</span>
		  <div class="logs-header-copy"><small>{{ tx('多角色配装导入', 'Multi-Character Import') }}</small><strong>{{ tx('GBFR Logs 配装库', 'GBFR Logs Library') }}</strong><span>{{ tx('从战斗记录中整理队伍配装，预览确认后再部署到存档。', 'Organize party loadouts from battle logs, preview them, then deploy to a save.') }}</span></div>
		  <div class="logs-source-actions">
			<button type="button" class="ui-btn" :disabled="logsLoading" @click="openLogsJSONImport"><span aria-hidden="true">↓</span>{{ tx('导入角色 JSON', 'Import Character JSON') }}</button>
			<button type="button" class="logs-source-button ui-btn is-primary" :disabled="logsLoading" @click="browseLogsLoadouts"><span aria-hidden="true">＋</span>{{ logsLoading ? tx('正在解析…', 'Parsing…') : logsSourceKind === 'database' ? tx('更换数据库', 'Change Database') : tx('选择 Logs 数据库', 'Choose Logs Database') }}</button>
		  </div>
		</div>
		<div class="logs-library-meta" :aria-label="tx('导入特性', 'Import Features')"><span>{{ tx('只读解析', 'Read-Only Parsing') }}</span><span>{{ tx('本地处理', 'Local Processing') }}</span><span>{{ tx('分项导入', 'Selective Import') }}</span></div>
	  </header>
	  <div v-if="logsCandidates.length" class="logs-candidate-grid">
		<article v-for="(candidate, candidateIndex) in logsCandidates" :key="`${candidate.logTime}-${candidate.playerName}-${candidate.ownerCode}-${candidateIndex}`" class="logs-candidate-card ui-card is-flat">
		  <header>
			<img v-if="characterAssetIcon(candidate.characterHash)" :src="characterAssetIcon(candidate.characterHash)" alt="" />
			<span v-else class="logs-character-fallback" aria-hidden="true">◇</span>
			<div><small>{{ candidate.protocolLabel }}</small><strong>{{ candidate.characterName }}</strong><span>{{ candidate.playerName || tx('未记录玩家名', 'Player Name Not Recorded') }} · {{ logsTime(candidate.logTime) }}</span></div>
		  </header>
		  <dl>
			<div><dt>{{ tx('武器', 'Weapon') }}</dt><dd>{{ candidate.weaponName || tx('未记录', 'Not Recorded') }}</dd></div>
			<div><dt>{{ tx('因子', 'Sigils') }}</dt><dd>{{ candidate.sigilCount }} / 12</dd></div>
			<div><dt>{{ tx('上限突破', 'Overmastery') }}</dt><dd>{{ candidate.overLimitCount }} / 4</dd></div>
		  </dl>
		  <div class="logs-capability-list" :aria-label="tx('日志已捕获范围', 'Captured Log Fields')"><span v-for="field in candidate.capturedFields || []" :key="field">{{ logsFieldLabel(field) }}</span></div>
		  <p v-if="candidate.missingFields?.length" class="logs-missing-fields">{{ logsMissingText(candidate.missingFields) }}</p>
		  <div class="logs-card-actions">
			<button type="button" class="ui-btn" @click="previewLogsCandidate(candidate)">{{ tx('预览实际配装', 'Preview Loadout') }}</button>
			<button type="button" class="ui-btn" @click="openLogsPublish(candidate)">{{ tx('上传分享', 'Publish') }}</button>
			<button type="button" class="ui-btn is-primary" @click="deployLogsCandidate(candidate)">{{ tx('导入到存档', 'Import to Save') }}</button>
		  </div>
		</article>
	  </div>
	  <div v-else class="logs-empty ui-card is-flat">
		<span class="logs-empty-index" aria-hidden="true">01</span>
		<div>
		  <strong>{{ tx('尚未载入外部配装', 'No External Loadouts Loaded') }}</strong>
		  <span>{{ tx('可以粘贴 Relink Logs 复制的角色 JSON，也可以选择 Logs 生成的 logs.db；解析后的队伍成员会分别列在这里。', 'Paste character JSON copied from Relink Logs or choose a logs.db; parsed party members will be listed separately here.') }}</span>
		  <div class="logs-database-location">
			<b>{{ tx('数据库在哪里？', 'Where is the database?') }}</b>
			<span>{{ tx('GBFR Logs、Endless、Relink Logs：右键程序快捷方式打开文件所在位置，或进入解压目录，选择与程序同目录的 logs.db。', 'GBFR Logs, Endless, and Relink Logs: right-click the app shortcut and open its file location, or open the extracted folder, then choose logs.db beside the app.') }}</span>
			<span>{{ tx('SkyMeter：打开 %APPDATA%\\app.skymeter.relink；旧版目录为 app.astralledger.relink。', 'SkyMeter: open %APPDATA%\\app.skymeter.relink; older builds use app.astralledger.relink.') }}</span>
			<span>{{ tx('先退出 Logs 再导入；不要选择 logs.db-wal 或 logs.db-shm。', 'Exit Logs before importing; do not choose logs.db-wal or logs.db-shm.') }}</span>
		  </div>
		</div>
	  </div>
	  <div v-if="logsJSONOpen" class="logs-json-backdrop" @click.self="closeLogsJSONImport" @keydown.esc="closeLogsJSONImport">
		<section class="logs-json-dialog ui-card" role="dialog" aria-modal="true" :aria-label="tx('导入 Relink Logs 角色 JSON', 'Import Relink Logs Character JSON')">
		  <header>
			<div><small>{{ tx('Relink Logs 剪贴板接口', 'Relink Logs Clipboard Export') }}</small><strong>{{ tx('粘贴 Relink Logs 复制的角色 JSON', 'Paste character JSON copied from Relink Logs') }}</strong></div>
			<button type="button" class="ui-btn is-ghost is-sm" :disabled="logsLoading" :aria-label="tx('关闭', 'Close')" @click="closeLogsJSONImport">×</button>
		  </header>
		  <p>{{ tx('在 Relink Logs 的战斗记录详情中打开“装备”，点击“复制角色数据到剪贴板 (JSON)”。可粘贴单个角色、角色数组，或选择已经保存的 JSON 文件。', 'Open Equipment in a Relink Logs battle record and click “Copy Character Data to Clipboard (JSON)”. Paste one character or an array, or choose a saved JSON file.') }}</p>
		  <textarea ref="logsJSONTextarea" v-model="logsJSONPayload" class="ui-textarea logs-json-textarea" spellcheck="false" :placeholder="tx('在这里粘贴 characterType、sigils、weaponState 等角色数据…', 'Paste characterType, sigils, weaponState, and other character data here…')"></textarea>
		  <p v-if="logsJSONError" class="logs-json-error" role="alert">{{ logsJSONError }}</p>
		  <footer>
			<button type="button" class="ui-btn" :disabled="logsLoading" @click="readLogsJSONClipboard">{{ tx('读取剪贴板', 'Read Clipboard') }}</button>
			<button type="button" class="ui-btn" :disabled="logsLoading" @click="browseLogsJSONFile">{{ tx('选择 JSON 文件', 'Choose JSON File') }}</button>
			<button type="button" class="ui-btn is-primary" :disabled="logsLoading" @click="parseLogsJSON">{{ logsLoading ? tx('正在解析…', 'Parsing…') : tx('解析并预览', 'Parse & Preview') }}</button>
		  </footer>
		</section>
	  </div>
	  <LoadoutPublishDialog
		:open="Boolean(logsPublishCandidate)"
		:title="logsPublishTitle"
		:character-name="logsPublishCandidate?.characterName || ''"
		:subtitle="logsPublishCandidate?.weaponName || ''"
		:image="logsPublishCandidate ? characterAssetIcon(logsPublishCandidate.characterHash) : ''"
		:busy="logsPublishBusy"
		:result="logsPublishResult"
		:error="logsPublishError"
		@update:title="logsPublishTitle = $event"
		@close="closeLogsPublish"
		@submit="publishLogsCandidate"
		@copy="copyLogsPublishedLink"
	  />
	</section>

    <KeepAlive v-else-if="mode === 'atlas'">
      <SigilAtlas class="loadout-tool-subpage" @construct="constructFromAtlas" @optimize="openAtlasOptimizer" @status="(m, t) => emit('status', m, t)" />
    </KeepAlive>
    <section v-else-if="mode === 'battles'" class="loadout-tool-subpage"><LogsBattleArchive @status="(m, t) => emit('status', m, t)" /></section>

    <template v-else>
	  <section v-if="effectivePendingImport?.code" class="section ui-notice is-info"><strong>已捕获 {{ effectivePendingImport.charaName || '角色' }} 配装</strong><span>选择目标存档后将自动进入分项导入，不会直接写档。</span></section>
      <SaveSourcePicker
        class="section"
        :slots="slots"
        :model-value="savePath"
        :busy="loading"
        :loaded="groups.length > 0"
        :title="tx('第一步 · 选择来源存档', 'Step 1 · Choose Source Save')"
        :summary="groups.length ? tx(`${groups.length} 个角色 · ${presetCount} 套预设`, `${groups.length} characters · ${presetCount} presets`) : ''"
        :helper="tx('后续角色、库存优化和写入目标都绑定这里选择的存档', 'Characters, inventory optimization, and write targets all use this save')"
        @select="load"
        @browse="browse"
      />

	  <button type="button" class="logs-library-entry ui-card" @click="openLogsLibrary">
		<span class="logs-entry-mark" aria-hidden="true">L</span>
		<span><small>{{ tx('外部战斗记录', 'External Battle Logs') }}</small><strong>{{ tx('从 GBFR Logs 批量获取队伍配装', 'Import Party Loadouts from GBFR Logs') }}</strong><em>{{ tx('扫描最近的兼容记录并拆分多名角色，可逐个预览后再导入。', 'Scan recent supported records into separate character previews, then import selectively.') }}</em></span>
		<b aria-hidden="true">→</b>
	  </button>

      <section v-if="groups.length" class="section ui-card ui-panel">
        <div class="section-title ui-section-title">
          <span>第二步 · 选择角色</span><small>{{ groups.length }} 个角色 · {{ presetCount }} 套预设；优化和分享都会使用当前角色数据</small>
          <button type="button" class="edit-launch ui-btn is-primary" :disabled="!currentGroup" @click="enterEdit">
            编辑 {{ currentGroup?.charaName || '' }} 配装 <span aria-hidden="true">→</span>
          </button>
        </div>
        <div class="chara-row">
          <button v-for="g in groups" :key="g.charaHash" class="chara-chip ui-chip" :class="{ 'is-on': selectedChara === g.charaName }" @click="selectedChara = g.charaName">
            <img v-if="characterAssetIcon(g.charaHash)" :src="characterAssetIcon(g.charaHash)" alt="" />
            <span class="chara-chip-name" :title="g.charaName">{{ g.charaName }}</span><i>{{ g.loadouts.filter(l => !l.isParty).length }}</i>
          </button>
        </div>
      </section>

      <section v-if="currentGroup" class="section ui-card ui-panel">
        <div class="section-title ui-section-title"><span>{{ currentGroup.charaName }} 的配装</span><small>{{ previewLoading ? '正在估算每套配装…' : '当前实时配装置顶；点击卡片展开因子与专精明细' }}</small></div>
        <div class="card-grid ui-card-grid">
          <article v-for="lo in currentGroup.loadouts" :key="lo.unitId" class="loadout-card ui-card is-flat" :class="{ open: expanded.has(lo.unitId), party: lo.isParty }">
            <button type="button" class="loadout-card-toggle" :aria-expanded="expanded.has(lo.unitId)" @click="toggle(lo)">
              <img v-if="loadoutWeaponIcon(lo)" class="loadout-weapon-icon" :src="loadoutWeaponIcon(lo)" alt="" />
              <span v-else class="loadout-weapon-icon loadout-weapon-icon-fallback" role="img" :aria-label="tx('武器图标未收录', 'Weapon icon not cataloged')" :title="tx('武器图标未收录', 'Weapon icon not cataloged')">{{ tx('未收录', 'No icon') }}</span>
              <b v-if="!lo.isParty">槽{{ String(lo.slot).padStart(2, '0') }}</b>
              <b v-else class="party-tag">队伍{{ lo.slot }}</b>
              <strong>{{ lo.name || (lo.isParty ? '当前实时配装' : '(未命名)') }}</strong>
              <span class="wep">{{ lo.weaponName || '未收录武器' }}</span>
              <em>{{ (lo.sigils || []).length }}因子 · {{ (lo.mastery || []).length }}专精</em>
              <span class="expand-mark">{{ expanded.has(lo.unitId) ? '收起' : '展开' }}<i aria-hidden="true">⌄</i></span>
            </button>
            <div v-if="lo.weapon" class="weapon-loadout-summary">
              <span><b>{{ lo.weapon.name || lo.weaponName }}</b><small>Lv{{ lo.weapon.level }} · 觉醒 {{ lo.weapon.awakening }} · 超凡 {{ lo.weapon.transcendence }}</small></span>
              <i v-for="skill in lo.weapon.skills" :key="`${skill.slot}-${skill.traitHash}`" :title="skill.effect || skill.unlockCondition">
                {{ skill.name || '未收录武器技能' }} <em>Lv{{ skill.level }}</em>
              </i>
              <i v-if="lo.weapon.wrightstone" class="wrightstone-chip">祝福 · {{ lo.weapon.wrightstone.name || '未收录祝福' }}</i>
              <i v-for="trait in lo.weapon.wrightstone?.traits || []" :key="`${lo.unitId}-stone-${trait.index}-${trait.hash}`" class="wrightstone-chip">{{ trait.name || trait.hash }} <em>Lv{{ trait.level }}</em></i>
            </div>
            <div class="skills-line">
              <span>技能</span>
              <i v-for="s in lo.skills" :key="s.hash"><img :src="skillIcon(s)" alt="" />{{ s.name || '未收录技能' }}</i>
            </div>
            <div class="mastery-summary">
              <span>专精</span>
              <i v-for="t in masterySummary(lo)" :key="t.rank">{{ t.label }} {{ t.count }}点</i>
              <i v-if="!(lo.mastery || []).length" class="dim">未保存</i>
            </div>
            <div v-if="previewFor(lo)?.finalStats" class="loadout-stat-strip">
              <span><small>HP</small><b>≈{{ formatNumber(previewFor(lo).finalStats.hp) }}</b></span>
              <span><small>攻击</small><b>≈{{ formatNumber(previewFor(lo).finalStats.attack) }}</b></span>
              <span><small>暴击</small><b>≈{{ formatNumber(previewFor(lo).finalStats.critRate, 1) }}%</b></span>
              <span><small>昏厥</small><b>≈{{ formatNumber(previewFor(lo).finalStats.stunPower, 1) }}</b></span>
            </div>
            <p v-if="previewFor(lo)?.error" class="preview-error">{{ previewFor(lo).error }}</p>
            <div v-if="!lo.isParty" class="loadout-share-row">
              <span class="loadout-share-state">
                <small>{{ tx('线上分享短码', 'Online Share Code') }}</small>
                <strong v-if="publishedShareFor(lo)?.code">{{ publishedShareFor(lo).code }}</strong>
                <strong v-else class="is-empty">{{ tx('尚未生成', 'Not Generated') }}</strong>
                <em>{{ publishedShareFor(lo)?.url ? tx('已上传，可直接复制链接或用于分享图二维码', 'Published and ready for link copying or share-image QR') : tx('上传后会在这里保留短码，并自动交给分享图工坊', 'Publish once to keep the code here and reuse it in the share-image workshop') }}</em>
              </span>
              <button
                type="button"
                class="ui-btn is-sm loadout-share-action"
                :disabled="cardPublishBusy"
                @click="publishedShareFor(lo)?.url ? copyCardPublishedLink(publishedShareFor(lo)) : openCardPublish(lo)"
              >
                {{ publishedShareFor(lo)?.url ? tx('复制链接', 'Copy Link') : tx('上传短码', 'Publish Code') }}
              </button>
            </div>
            <div v-if="expanded.has(lo.unitId)" class="detail">
              <div class="loadout-detail-actions" aria-label="当前配装操作">
                <span><b>{{ lo.name || (lo.isParty ? '当前实时配装' : `槽 ${String(lo.slot).padStart(2, '0')}`) }}</b><small>所有操作都绑定 {{ currentGroup.charaName }} 和当前来源存档</small></span>
                <button type="button" class="ui-btn" :disabled="lo.isParty" @click="enterEdit(lo.unitId)">编辑此配装</button>
                <button type="button" class="ui-btn share-tool-button" :class="{ 'is-active': cardToolOpen(lo, 'share') }" :disabled="lo.isParty" @click="cardToolOpen(lo, 'share') ? closeCardTool() : openCardTool(lo, 'share')">{{ cardToolOpen(lo, 'share') ? '收起分享图' : '生成分享图' }}</button>
              </div>
              <section v-if="cardToolOpen(lo, 'share')" class="loadout-inline-tool">
                <LoadoutShareWorkshop embedded :group="shareGroup(lo)" :preview-map="previews" :published="publishedShareFor(lo)" @status="(m, t) => emit('status', m, t)" />
              </section>
              <div class="detail-block sigil-detail-block">
                <div class="detail-block-heading">
                  <span><h4>当前因子</h4><small>智能配装已放入“编辑此配装”里的因子配置区</small></span>
                </div>
                <ul class="sigil-detail-list">
                  <li v-for="s in lo.sigils" :key="s.slotId" class="sigil-detail-item">
                    <div class="sigil-detail-title">
                      <img v-if="traitIcon(s.primaryTraitName, s.primaryTraitHash)" :src="traitIcon(s.primaryTraitName, s.primaryTraitHash)" alt="" />
                      <span v-else class="sigil-icon-fallback" aria-hidden="true">◇</span>
                      <b>{{ s.name || '因子' }}</b>
                      <small>因子 Lv{{ s.level }}</small>
                    </div>
                    <div v-if="!s.missing" class="sigil-traits">
                      <span v-if="s.primaryTraitName"><i>主</i>{{ s.primaryTraitName }}<em>Lv{{ s.primaryTraitLevel }}</em></span>
                      <span v-if="s.secondaryTraitName"><i>副</i>{{ s.secondaryTraitName }}<em>Lv{{ s.secondaryTraitLevel }}</em></span>
                    </div>
                    <small v-if="s.missing" class="warn">原背包因子已不存在</small>
                  </li>
                </ul>
              </div>
              <div v-for="grp in masteryGrouped(lo)" :key="grp.key" class="detail-block">
                <h4>{{ grp.rankLabel }} · {{ catLabel(grp.cat) }}（{{ grp.nodes.length }}点）</h4>
                <ul>
                  <li v-for="m in grp.nodes" :key="m.hash">
                    <b v-if="m.name">{{ m.name }} — </b>{{ m.desc || '未收录效果' }}
                  </li>
                </ul>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section v-else-if="!loading && savePath && !groups.length" class="section ui-card ui-panel">
        <p class="empty ui-empty">该存档中没有已保存的配装预设。</p>
      </section>

      <section v-else-if="!loading && !savePath" class="section ui-card ui-panel is-compact">
        <p class="empty ui-empty">选择存档位或浏览文件后，这里会显示真实角色与配装预设。</p>
      </section>

      <LoadoutPublishDialog
        :open="Boolean(cardPublishTarget)"
        :title="cardPublishTitle"
        :character-name="currentGroup?.charaName || ''"
        :subtitle="cardPublishTarget?.weaponName || cardPublishTarget?.weapon?.name || ''"
        :image="currentGroup ? characterAssetIcon(currentGroup.charaHash) : ''"
        :busy="cardPublishBusy"
        :result="cardPublishResult"
        :error="cardPublishError"
        @update:title="cardPublishTitle = $event"
        @close="closeCardPublish"
        @submit="publishCardLoadout"
        @copy="copyCardPublishedLink(cardPublishResult)"
      />
    </template>
  </div>
</template>

<style scoped>
.loadout-viewer { width:100%; max-width:100%; min-width:0; overflow-x:hidden; font-size:var(--fs-md); container:loadout-viewer / inline-size; }
.loadout-viewer.editing { width:100%; height:100%; min-height:0; gap:0; overflow:hidden; }
.loadout-subnav { position:sticky; z-index:12; top:0; min-width:0; min-height:44px; display:flex; flex-wrap:nowrap; align-items:stretch; gap:24px; overflow-x:auto; overflow-y:hidden; padding:0 var(--space-4); border-bottom:1px solid rgba(126,89,40,.38); background:rgba(239,224,190,.96); box-shadow:none; scrollbar-width:thin; scrollbar-color:var(--border-strong) transparent; }
.loadout-subnav button { position:relative; min-width:max-content; min-height:44px; flex:0 0 auto; display:flex; align-items:center; padding:0 2px; border:0; border-bottom:2px solid transparent; border-radius:0; background:transparent; box-shadow:none; color:var(--text-secondary); text-align:left; cursor:pointer; }
.loadout-subnav button:hover:not(:disabled) { border-bottom-color:rgba(126,89,40,.36); background:transparent; color:var(--accent-hover); }
.loadout-subnav button.active { border-bottom-color:#8f6736; background:transparent; box-shadow:none; color:var(--accent-hover); }
.loadout-subnav button:disabled { opacity:.46; cursor:not-allowed; }
.loadout-subnav button b { color:inherit; font-family:var(--font-display); font-size:var(--fs-sm); white-space:nowrap; }
.loadout-tool-subpage { min-width:0; width:100%; }
.section { width:100%; max-width:100%; min-width:0; overflow:hidden; }
.section-title { min-width:0; flex-wrap:wrap; }
.section-title > small { min-width:0; overflow-wrap:anywhere; }
.loadout-detail-actions { min-width:0; display:grid; grid-template-columns:minmax(180px,1fr) repeat(2,auto); gap:var(--space-2); align-items:center; padding:var(--space-3); border:1px solid var(--border-strong); background:var(--surface-sunken); }
.loadout-detail-actions > span { min-width:0; display:grid; gap:2px; }
.loadout-detail-actions > span b,.loadout-detail-actions > span small { min-width:0; overflow-wrap:anywhere; }
.share-tool-button { border-color:color-mix(in srgb,var(--accent) 22%,var(--border-soft)); background:color-mix(in srgb,var(--surface-card-pop) 90%,#e5f5ff); box-shadow:none; color:var(--text-secondary); }
.share-tool-button:hover:not(:disabled),.share-tool-button.is-active { border-color:color-mix(in srgb,var(--accent) 42%,var(--border-soft)); background:color-mix(in srgb,var(--surface-card-pop) 74%,#d4efff); color:var(--accent-hover); }
.loadout-detail-actions > span small { color:var(--text-muted); font-size:var(--fs-xs); }
.loadout-inline-tool { min-width:0; padding:var(--space-3); border:1px solid var(--accent-border); background:rgba(255,253,247,.58); }
.detail-block-heading { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); margin-bottom:var(--space-3); }
.detail-block-heading > span { min-width:0; display:flex; flex-direction:column; gap:2px; }
.detail-block-heading h4 { margin:0; }
.detail-block-heading small { color:var(--text-muted); font-size:var(--fs-xs); overflow-wrap:anywhere; }
.loadout-inline-tool.is-factor-tool { margin:0 0 var(--space-4); }
.edit-launch { margin-left:auto; }
.edit-launch span { font-size:var(--fs-lg); }
.editor-workspace { min-width:0; height:100%; min-height:0; display:flex; flex-direction:column; gap:14px; overflow:hidden; }
.editor-workspace-bar { position:sticky; z-index:20; top:0; min-width:0; min-height:72px; display:grid; grid-template-columns:auto minmax(0,1fr) minmax(180px,280px); gap:var(--space-5); align-items:center; padding:var(--space-3) var(--space-5); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }
.back-button span { font-size:1rem; }
.editor-workspace-title { min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr); align-items:baseline; column-gap:9px; row-gap:2px; }
.editor-workspace-title small { grid-column:1/-1; color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-semibold); letter-spacing:.08em; }
.editor-workspace-title strong { color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); font-weight:var(--fw-bold); }
.editor-workspace-title span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-secondary); font-size:var(--fs-sm); }
.editor-workspace-meta { min-width:0; display:flex; flex-direction:column; align-items:flex-end; gap:2px; text-align:right; }
.preset-count-badge { display:inline-flex; align-items:baseline; gap:5px; padding:3px 8px; border:1px solid var(--line-soft); border-radius:12px; background:rgba(139,103,55,.07); color:var(--text-secondary); font-size:var(--fs-sm); white-space:nowrap; }
.preset-count-badge b { color:var(--accent-hover); font-size:var(--fs-md); }
.editor-workspace-meta small { max-width:100%; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-muted); font-size:var(--fs-xs); }
.editor-workspace-content { min-width:0; min-height:0; flex:1; overflow:auto; scrollbar-gutter:stable; overscroll-behavior:contain; padding:0 2px 2px; }
.editor-workspace-content :deep(.loadout-editor) { height:100%; min-height:0; }
.path-line { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.chara-row { display:grid; grid-template-columns:repeat(auto-fit, minmax(156px, 1fr)); gap:var(--space-2); }
.chara-chip { width:100%; min-width:0; display:inline-flex; align-items:center; justify-content:flex-start; gap:5px; }
.chara-chip img { flex:0 0 auto; width:27px; height:27px; object-fit:cover; border-radius:6px; }
.chara-chip-name { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.chara-chip i { flex:0 0 auto; margin-left:auto; color:var(--text-muted); font-size:var(--fs-xs); font-style:normal; }
.chara-chip.is-on i { color:var(--accent-soft); }
.card-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,420px),1fr)); gap:var(--space-4); align-items:start; min-width:0; }
.loadout-card { min-width:0; display:flex; flex-direction:column; gap:var(--space-3); padding:var(--space-4); border-left:3px solid var(--accent); background:var(--surface-card-pop); }
.loadout-card.party { grid-column:1 / -1; order:-1; border-left-color:var(--success); background:linear-gradient(110deg,rgba(74,139,105,.1),rgba(255,253,247,.92) 52%); box-shadow:inset 0 0 0 1px rgba(74,139,105,.08); }
.loadout-card.party .loadout-card-toggle { grid-template-columns:62px auto minmax(120px,1.2fr) minmax(120px,.8fr) auto auto; }
.loadout-card.open { border-color:var(--border-strong); grid-column:1/-1; }
.loadout-card-toggle { width:100%; min-width:0; min-height:var(--control-height-sm); display:grid; grid-template-columns:62px auto minmax(120px,1fr) minmax(100px,.72fr) auto auto; align-items:center; gap:var(--space-3); padding:0; border:0; background:transparent; color:inherit; text-align:left; cursor:pointer; user-select:none; }
.loadout-weapon-icon { box-sizing:border-box; width:62px; height:44px; object-fit:contain; border-radius:6px; background:rgba(255,255,255,.55); }
.loadout-weapon-icon-fallback { display:grid; place-items:center; padding:3px; border:1px dashed var(--border-default); background:var(--surface-sunken); color:var(--text-muted); font-size:var(--fs-2xs); font-weight:var(--fw-semibold); line-height:1.1; text-align:center; }
.loadout-card-toggle:hover strong { color:var(--accent-hover); }
.loadout-card-toggle b { color:var(--accent); font-size:var(--fs-sm); }
.loadout-card-toggle b.party-tag { color:var(--success-ink); }
.loadout-card-toggle strong,.loadout-card-toggle .wep,.loadout-card-toggle em { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.loadout-card-toggle strong { color:var(--text-primary); font-size:var(--fs-md); transition:color var(--dur-base) var(--ease-out); }
.loadout-card-toggle .wep { color:var(--text-secondary); font-size:var(--fs-sm); }
.loadout-card-toggle em { color:var(--text-muted); font-size:var(--fs-xs); font-style:normal; }
.expand-mark { display:inline-flex; align-items:center; justify-content:flex-end; gap:3px; color:var(--accent-hover); font-size:var(--fs-xs); font-weight:var(--fw-semibold); white-space:nowrap; }
.expand-mark i { display:inline-block; font-size:14px; font-style:normal; transition:transform var(--dur-base) var(--ease-out); }
.loadout-card.open .expand-mark i { transform:rotate(180deg); }
.weapon-loadout-summary { min-width:0; display:flex; flex-wrap:wrap; gap:var(--space-2); align-items:center; padding:7px 9px; border:1px solid rgba(139,103,55,.16); border-radius:8px; background:rgba(139,103,55,.045); }
.weapon-loadout-summary > span { min-width:150px; display:flex; flex-direction:column; margin-right:3px; }
.weapon-loadout-summary > span b { color:var(--text-primary); font-size:var(--fs-sm); }
.weapon-loadout-summary > span small { color:var(--text-muted); font-size:var(--fs-xs); }
.weapon-loadout-summary > i { padding:2px 7px; border:1px solid var(--line-soft); border-radius:10px; background:var(--panel-solid); color:var(--text-secondary); font-size:var(--fs-xs); font-style:normal; }
.weapon-loadout-summary > i em { color:var(--accent); font-style:normal; }
.weapon-loadout-summary > i.wrightstone-chip { border-color:rgba(123,89,154,.28); background:rgba(123,89,154,.08); color:#6c4c82; }
.skills-line, .mastery-summary { display:flex; flex-wrap:wrap; gap:var(--space-2); align-items:center; font-size:var(--fs-sm); }
.skills-line span, .mastery-summary span { color:var(--text-muted); }
.skills-line i, .mastery-summary i { font-style:normal; padding:1px 8px; border:1px solid var(--line-soft); border-radius:12px; background:var(--panel-solid); color:var(--text-secondary); }
.skills-line i { display:inline-flex; align-items:center; gap:5px; padding-left:3px; }
.skills-line i img { width:24px; height:24px; border-radius:50%; object-fit:cover; }
.mastery-summary i b { font-weight:700; color:var(--amber); }
.mastery-summary i.dim { color:var(--text-muted); border-style:dashed; background:none; }
.detail { display:flex; flex-direction:column; gap:9px; padding-top:5px; border-top:1px dashed var(--line); }
.detail-block h4 { margin:0 0 5px; font-size:.74rem; color:var(--gold); }
.detail-block ul { margin:0; padding-left:17px; display:flex; flex-direction:column; gap:2px; }
.detail-block li { font-size:.72rem; line-height:1.5; color:var(--text-secondary); }
.detail-block li b { color:var(--text-primary); }
.detail-block li small { color:var(--text-muted); margin-left:5px; }
.detail-block li small.warn { color:var(--red); }
.sigil-detail-list { list-style:none; padding:0; display:grid; grid-template-columns:repeat(auto-fill,minmax(260px,1fr)); gap:7px; }
.sigil-detail-item { min-width:0; padding:var(--space-3) var(--space-4); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card); }
.sigil-detail-title { min-width:0; display:flex; align-items:center; gap:7px; }
.sigil-detail-title > img { width:30px; height:30px; flex:0 0 30px; border:1px solid var(--line-gold); border-radius:7px; object-fit:cover; }
.sigil-icon-fallback { width:30px; height:30px; flex:0 0 30px; display:grid; place-items:center; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--accent-soft); color:var(--accent-hover); font-size:1rem; }
.sigil-detail-title b { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-primary); font-size:var(--fs-sm); }
.sigil-detail-title small { margin-left:auto; flex:0 0 auto; color:var(--text-muted); font-size:var(--fs-xs); }
.sigil-traits { display:flex; flex-direction:column; gap:3px; margin-top:5px; }
.sigil-traits span { min-width:0; display:grid; grid-template-columns:20px minmax(0,1fr) auto; gap:var(--space-2); align-items:center; color:var(--text-secondary); font-size:var(--fs-sm); line-height:1.35; }
.sigil-traits i { width:20px; height:18px; display:grid; place-items:center; border:1px solid var(--border-strong); border-radius:var(--radius-sm); color:var(--accent-hover); background:var(--accent-soft); font-size:var(--fs-xs); font-style:normal; font-weight:var(--fw-bold); }
.sigil-traits em { color:var(--accent); font-size:var(--fs-xs); font-style:normal; font-weight:var(--fw-semibold); }
.preview-error { margin:var(--space-3) 0 0; color:var(--danger-ink,#a6473f); font-size:var(--fs-xs); line-height:1.5; }
.loadout-stat-strip { display:grid; grid-template-columns:repeat(4,minmax(72px,1fr)); gap:4px; padding-top:var(--space-2); border-top:1px dashed var(--line-soft); }
.loadout-stat-strip span { min-width:0; display:flex; flex-direction:column; padding:4px 6px; background:rgba(139,103,55,.045); }
.loadout-stat-strip small { color:var(--text-muted); font-size:10px; }
.loadout-stat-strip b { overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); font-size:var(--fs-xs); font-variant-numeric:tabular-nums; white-space:nowrap; }
.loadout-share-row { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-3); align-items:center; padding:8px 10px; border:1px solid color-mix(in srgb,var(--accent) 18%,var(--border-soft)); background:color-mix(in srgb,var(--surface-card-pop) 92%,#e6f4f7); }
.loadout-share-state { min-width:0; display:grid; grid-template-columns:auto auto minmax(0,1fr); gap:4px var(--space-3); align-items:baseline; }
.loadout-share-state small { color:var(--text-muted); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.loadout-share-state strong { color:var(--success-ink); font-family:var(--font-data); font-size:var(--fs-sm); letter-spacing:0; white-space:nowrap; }
.loadout-share-state strong.is-empty { color:var(--text-secondary); font-family:var(--font-body); font-weight:var(--fw-semibold); }
.loadout-share-state em { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-muted); font-size:var(--fs-xs); font-style:normal; white-space:nowrap; }
.loadout-share-action { min-width:88px; border-color:color-mix(in srgb,var(--accent) 28%,var(--border-soft)); background:var(--surface-card-pop); color:var(--accent-hover); box-shadow:none; }
.empty { margin:0; }
.logs-library-entry { width:100%; min-width:0; display:grid; grid-template-columns:42px minmax(0,1fr) auto; align-items:center; gap:var(--space-3); padding:var(--space-4); border:1px solid var(--border-strong); border-left:4px solid var(--accent); background:var(--surface-card-pop); color:inherit; text-align:left; cursor:pointer; box-shadow:var(--shadow-1); }
.logs-library-entry:hover { border-color:var(--accent); background:var(--accent-soft); }
.logs-library-entry > span:nth-child(2) { min-width:0; display:grid; gap:2px; }
.logs-library-entry small,.logs-library-entry strong,.logs-library-entry em { min-width:0; overflow-wrap:anywhere; }
.logs-library-entry small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.logs-library-entry strong { color:var(--text-primary); font-size:var(--fs-md); }
.logs-library-entry em { color:var(--text-muted); font-size:var(--fs-xs); font-style:normal; }
.logs-library-entry > b { color:var(--accent-hover); font-size:var(--fs-xl); }
.logs-entry-mark { width:42px; height:42px; display:grid; place-items:center; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-sunken); color:var(--accent-hover); font-family:var(--font-display); font-size:var(--fs-lg); font-weight:var(--fw-bold); }
.logs-workspace { width:100%; min-width:0; display:grid; gap:var(--space-4); }
.subpage-bar { position:sticky; z-index:10; top:0; min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr) auto; align-items:center; gap:var(--space-4); padding:var(--space-4); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }
.subpage-bar > div { min-width:0; display:grid; gap:2px; }
.subpage-bar small,.subpage-bar strong,.subpage-bar span { min-width:0; overflow-wrap:anywhere; }
.subpage-bar small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.subpage-bar strong { color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); }
.subpage-bar span { color:var(--text-muted); font-size:var(--fs-xs); }
.logs-library-header { position:relative; overflow:hidden; padding:0; border-color:var(--border-strong); background:linear-gradient(108deg,var(--surface-card-pop),var(--surface-card) 68%,var(--surface-sunken)); box-shadow:var(--shadow-2); }
.logs-library-header::after { content:""; position:absolute; right:-34px; bottom:-62px; width:170px; height:170px; border:1px solid rgba(126,89,40,.14); border-radius:50%; box-shadow:0 0 0 18px rgba(126,89,40,.035),0 0 0 38px rgba(126,89,40,.025); pointer-events:none; }
.logs-library-nav { position:relative; z-index:1; min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); padding:var(--space-3) var(--space-5); border-bottom:1px solid var(--border-soft); background:rgba(255,253,247,.38); }
.logs-back-button { justify-self:start; width:auto; }
.logs-source-status { min-width:0; display:inline-flex; align-items:center; gap:7px; color:var(--text-muted); font-size:var(--fs-xs); font-weight:var(--fw-semibold); text-align:right; }
.logs-source-status i { width:7px; height:7px; flex:0 0 7px; border:1px solid var(--border-strong); border-radius:50%; background:var(--surface-sunken); box-shadow:0 0 0 3px rgba(126,89,40,.06); }
.logs-source-status.ready { color:var(--success-ink); }
.logs-source-status.ready i { border-color:var(--success-ink); background:var(--success); box-shadow:0 0 0 3px var(--success-bg); }
.logs-library-main { position:relative; z-index:1; min-width:0; display:grid; grid-template-columns:56px minmax(0,1fr) auto; gap:var(--space-4); align-items:center; padding:var(--space-5); }
.logs-library-seal { width:56px; height:56px; display:grid; place-items:center; border:1px solid var(--accent-border); border-radius:var(--radius-sm); background:var(--accent); box-shadow:inset 0 0 0 3px rgba(255,253,247,.23),var(--shadow-1); color:var(--text-on-accent); font-family:var(--font-display); font-size:var(--fs-xl); font-weight:var(--fw-bold); }
.logs-header-copy { min-width:0; display:grid; gap:3px; align-self:center; }
.logs-header-copy small { color:var(--accent); font-size:var(--fs-2xs); font-weight:var(--fw-bold); }
.logs-header-copy strong { color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); line-height:var(--lh-tight); }
.logs-header-copy span { max-width:58ch; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); overflow-wrap:anywhere; }
.logs-source-actions { min-width:0; justify-self:end; display:flex; flex-wrap:wrap; justify-content:flex-end; gap:var(--space-2); }
.logs-source-actions .ui-btn { min-width:0; white-space:nowrap; }
.logs-source-button { justify-self:end; width:max-content; max-width:100%; }
.logs-source-button > span { font-size:var(--fs-lg); line-height:1; }
.logs-library-meta { position:relative; z-index:1; display:flex; flex-wrap:wrap; gap:var(--space-2); padding:0 var(--space-5) var(--space-4) calc(var(--space-5) + 72px); }
.logs-library-meta span { padding:2px 7px; border-left:2px solid rgba(126,89,40,.42); color:var(--text-muted); font-size:var(--fs-2xs); font-weight:var(--fw-semibold); }
.logs-candidate-grid { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,300px),1fr)); gap:var(--space-4); align-items:stretch; }
.logs-candidate-card { min-width:0; display:flex; flex-direction:column; gap:var(--space-3); padding:var(--space-4); border-left:3px solid var(--accent); background:var(--surface-card-pop); }
.logs-candidate-card > header { min-width:0; display:grid; grid-template-columns:52px minmax(0,1fr); gap:var(--space-3); align-items:center; }
.logs-candidate-card > header img,.logs-character-fallback { width:52px; height:52px; display:grid; place-items:center; overflow:hidden; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-sunken); object-fit:cover; }
.logs-candidate-card > header div { min-width:0; display:grid; gap:2px; }
.logs-candidate-card > header small,.logs-candidate-card > header strong,.logs-candidate-card > header span { min-width:0; overflow-wrap:anywhere; }
.logs-candidate-card > header small,.logs-candidate-card > header span { color:var(--text-muted); font-size:var(--fs-xs); }
.logs-candidate-card > header strong { color:var(--text-primary); font-size:var(--fs-lg); }
.logs-candidate-card dl { min-width:0; display:grid; grid-template-columns:minmax(0,1.5fr) repeat(2,minmax(72px,.65fr)); gap:var(--space-2); margin:0; }
.logs-candidate-card dl > div { min-width:0; padding:var(--space-2); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.logs-candidate-card dt { color:var(--text-muted); font-size:var(--fs-2xs); }
.logs-candidate-card dd { min-width:0; margin:2px 0 0; overflow-wrap:anywhere; color:var(--text-secondary); font-size:var(--fs-xs); }
.logs-capability-list { min-width:0; display:flex; flex-wrap:wrap; gap:5px; }
.logs-capability-list span { padding:3px 7px; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--accent-soft); color:var(--accent-hover); font-size:var(--fs-2xs); font-weight:var(--fw-semibold); }
.logs-missing-fields { margin:0; overflow-wrap:anywhere; color:var(--text-muted); font-size:var(--fs-2xs); line-height:1.45; }
.logs-preview-provenance { min-width:0; display:grid; gap:var(--space-2); padding:var(--space-3) var(--space-4); border-left:3px solid var(--accent); }
.logs-preview-provenance > div:first-child { min-width:0; display:grid; gap:2px; }
.logs-preview-provenance small { color:var(--text-muted); font-size:var(--fs-2xs); }
.logs-preview-provenance strong { overflow-wrap:anywhere; color:var(--text-primary); font-size:var(--fs-sm); }
.logs-preview-provenance p { margin:0; overflow-wrap:anywhere; color:var(--text-muted); font-size:var(--fs-xs); line-height:1.45; }
.logs-preview-provenance p.is-warning { color:var(--warning-ink,#7a5a19); }
.logs-card-actions { min-width:0; display:flex; flex-wrap:wrap; gap:var(--space-2); margin-top:auto; }
.logs-card-actions .ui-btn { flex:1 1 132px; min-width:0; }
.logs-empty { min-width:0; min-height:112px; display:grid; grid-template-columns:44px minmax(0,1fr); gap:var(--space-4); align-items:start; padding:var(--space-5); border-style:dashed; background:rgba(255,253,247,.42); }
.logs-empty-index { width:44px; height:44px; display:grid; place-items:center; border-right:1px solid var(--border-strong); color:var(--accent); font-family:var(--font-data); font-size:var(--fs-lg); font-weight:var(--fw-bold); }
.logs-empty > div { min-width:0; display:grid; gap:3px; }
.logs-empty strong { color:var(--text-primary); font-size:var(--fs-md); }
.logs-empty span:not(.logs-empty-index) { color:var(--text-muted); font-size:var(--fs-sm); line-height:var(--lh-normal); overflow-wrap:anywhere; }
.logs-database-location { min-width:0; display:grid; gap:3px; margin-top:var(--space-3); padding:var(--space-3); border-left:3px solid var(--accent-border); background:var(--surface-sunken); }
.logs-database-location b { color:var(--text-secondary); font-size:var(--fs-xs); }
.logs-database-location span { font-family:var(--font-body); font-size:var(--fs-xs) !important; }
.logs-json-backdrop { position:fixed; z-index:1200; inset:0; display:grid; place-items:center; padding:16px; overflow:auto; background:rgba(39,31,22,.56); backdrop-filter:blur(3px); }
.logs-json-dialog { width:100%; max-width:min(680px,calc(100vw - 32px)); max-height:calc(100vh - 32px); display:flex; flex-direction:column; gap:var(--space-4); overflow:auto; padding:var(--space-5); border-color:var(--accent-border); background:var(--surface-card-pop); box-shadow:var(--shadow-4); }
.logs-json-dialog > header { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-3); align-items:start; padding-bottom:var(--space-3); border-bottom:1px solid var(--border-soft); }
.logs-json-dialog > header > div { min-width:0; display:grid; gap:2px; }
.logs-json-dialog > header small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.logs-json-dialog > header strong { overflow-wrap:anywhere; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); }
.logs-json-dialog > p { margin:0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.logs-json-textarea { min-height:220px; max-height:42vh; overflow:auto; font-family:var(--font-data); font-size:var(--fs-xs); line-height:1.5; }
.logs-json-dialog > footer { min-width:0; display:flex; flex-wrap:wrap; justify-content:flex-end; gap:var(--space-2); }
.logs-json-dialog > footer .ui-btn { flex:0 1 auto; }
.logs-json-error { padding:var(--space-3); border-left:3px solid var(--danger); background:var(--danger-bg); color:var(--danger-ink) !important; overflow-wrap:anywhere; }
@container loadout-viewer (max-width:900px) {
  .section-title .edit-launch { width:100%; margin-left:0; }
  .loadout-detail-actions { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .loadout-detail-actions > span { grid-column:1/-1; }
  .loadout-detail-actions .ui-btn { min-width:0; white-space:normal; }
  .editor-workspace-bar { grid-template-columns:1fr auto; }
  .editor-workspace-title { grid-column:1/-1; grid-row:1; }
  .back-button { grid-row:2; }
  .editor-workspace-meta { grid-row:2; }
  .loadout-card-toggle { grid-template-columns:52px auto minmax(0,1fr) auto; }
  .loadout-card.party .loadout-card-toggle { grid-template-columns:52px auto minmax(0,1fr) auto; }
  .loadout-card-toggle > b { grid-column:2; grid-row:1; }
  .loadout-card-toggle > strong { grid-column:3; grid-row:1; }
  .loadout-card-toggle .wep { grid-column:2/4; grid-row:2; }
  .loadout-card-toggle > em { grid-column:2/4; grid-row:3; }
  .loadout-card-toggle .expand-mark { grid-column:4; grid-row:1/4; }
  .loadout-weapon-icon { width:52px; height:40px; grid-row:1/3; }
  .loadout-stat-strip { grid-template-columns:repeat(2,minmax(70px,1fr)); }
  .loadout-share-state { grid-template-columns:auto auto; }
  .loadout-share-state em { grid-column:1/-1; }
  .logs-library-main { grid-template-columns:48px minmax(0,1fr); }
  .logs-library-seal { width:48px; height:48px; }
  .logs-source-actions { grid-column:1/-1; justify-self:stretch; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); }
  .logs-source-actions .ui-btn { width:100%; white-space:normal; }
}
@container loadout-viewer (max-width:560px) {
	.loadout-subnav { gap:14px; }
	.loadout-detail-actions { grid-template-columns:1fr; }
	.loadout-detail-actions > span { grid-column:auto; }
	.detail-block-heading { align-items:stretch; flex-direction:column; }
	.loadout-inline-tool { padding:var(--space-2); }
	.loadout-share-row { grid-template-columns:minmax(0,1fr); }
	.loadout-share-state { grid-template-columns:minmax(0,1fr); }
	.loadout-share-state em { grid-column:auto; overflow:visible; white-space:normal; }
	.loadout-share-action { width:100%; }
	.logs-library-entry { grid-template-columns:36px minmax(0,1fr); }
	.logs-entry-mark { width:36px; height:36px; }
	.logs-library-entry > b { display:none; }
	.subpage-bar:not(.logs-library-bar) { grid-template-columns:minmax(0,1fr); }
	.subpage-bar:not(.logs-library-bar) > .back-button,.subpage-bar:not(.logs-library-bar) > div { grid-column:1; }
	.subpage-bar:not(.logs-library-bar) > .back-button { grid-row:1; justify-self:start; width:auto; }
	.subpage-bar:not(.logs-library-bar) > div { grid-row:2; width:100%; }
	.logs-library-nav { align-items:flex-start; padding:var(--space-3) var(--space-4); }
	.logs-source-status { max-width:44%; white-space:normal; }
	.logs-library-main { grid-template-columns:42px minmax(0,1fr); padding:var(--space-4); }
	.logs-library-seal { width:42px; height:42px; font-size:var(--fs-lg); }
	.logs-source-actions { grid-template-columns:minmax(0,1fr); }
	.logs-source-button { grid-column:1/-1; justify-self:stretch; width:100%; }
	.logs-library-meta { padding:0 var(--space-4) var(--space-4); }
	.logs-candidate-card dl { grid-template-columns:minmax(0,1fr); }
	.logs-json-dialog > footer { display:grid; grid-template-columns:minmax(0,1fr); }
	.logs-json-dialog > footer .ui-btn { width:100%; }
}
</style>
