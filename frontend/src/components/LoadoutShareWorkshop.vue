<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { SaveLoadoutSharePNG } from '../../wailsjs/go/backend/App'
import { characterSharePortraitProfile } from '../characterRoster'
import { characterAssetIcon, summonAssetIcon, traitAssetIcon, weaponAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'
import {
  createShareImageExportLifecycle,
  ShareImageExportCancelledError,
  withShareImageExportTimeout,
} from '../shareImageExportLifecycle.js'
import shareCardSkyFrame from '../assets/gbfr/share-card-sky-frame-v2.webp'

const props = defineProps({
  group: { type: Object, default: null },
  previewMap: { type: Object, default: () => new Map() },
  embedded: { type: Boolean, default: false },
  published: { type: Object, default: null },
  backdropUrl: { type: String, default: '' },
})
const emit = defineEmits(['status'])
const template = ref('landscape')
const loadoutId = ref(0)
const title = ref('')
const description = ref('')
const author = ref('')
const shareUrl = ref('')
const gameVersion = ref('2.0.2')
const generatedDate = ref(localDateValue())
const showGameVersion = ref(true)
const showGeneratedDate = ref(true)
const showProjectMark = ref(true)
const portraitOpacity = ref(88)
const density = ref('standard')
const canvasRef = ref(null)
const previewShellRef = ref(null)
const previewScale = ref(1)
const exportBusy = ref(false)
const exportLifecycle = createShareImageExportLifecycle()
const qrDataUrl = ref('')
const qrError = ref('')
const SHARE_IMAGE_EXPORT_TIMEOUT_MS = 20000
let qrRequest = 0
let qrPending = Promise.resolve()
let previewResizeObserver = null

const tx = (zh, en) => language.value === 'en' ? en : zh
const loadouts = computed(() => props.group?.loadouts || [])
const selected = computed(() => loadouts.value.find(item => item.unitId === loadoutId.value) || loadouts.value[0] || null)
const preview = computed(() => props.previewMap instanceof Map ? props.previewMap.get(selected.value?.unitId) : null)
const portraitProfile = computed(() => characterSharePortraitProfile(props.group?.charaHash))
const portrait = computed(() => portraitProfile.value?.path || '')
// Generated card backdrops can be connected either by the caller or later
// through the character portrait profile. The built-in sky treatment remains
// a complete fallback, so a missing optional asset never blocks export.
const cardBackdrop = computed(() => String(props.backdropUrl || portraitProfile.value?.cardBackdrop || shareCardSkyFrame).trim())
const characterIcon = computed(() => characterAssetIcon(props.group?.charaHash))
const weaponIcon = computed(() => {
  const weapon = selected.value?.weapon || {}
  return weaponAssetIcon({
    internalId: weapon.internalId,
    baseHash: weapon.baseHash,
    storedHash: weapon.storedHash,
    hash: selected.value?.weaponHash || weapon.storedHash,
  })
})
const canvasStyle = computed(() => {
  const profile = portraitProfile.value
  const anchor = profile?.anchors?.[template.value]
  const faceFocus = anchor?.focus || profile?.faceFocus || [0.5, 0.3]
  const safeFrame = profile?.weaponSafeFrame || [0, 0, 1, 1]
  const safeCenter = [
    Number(safeFrame[0]) + Number(safeFrame[2]) / 2,
    Number(safeFrame[1]) + Number(safeFrame[3]) / 2,
  ]
  const faceWeight = { landscape: 0.66, portrait: 0.72, square: 0.82 }[template.value] || 0.72
  const focus = faceFocus.map((value, index) => Number(value) * faceWeight + safeCenter[index] * (1 - faceWeight))
  const safeExtent = Math.max(Number(safeFrame[2]) || 1, Number(safeFrame[3]) || 1)
  const safeScale = Math.min(1.14, Math.max(1, 0.92 / safeExtent))
  return {
    '--portrait-opacity': String(portraitOpacity.value / 100),
    '--portrait-fit': anchor?.fit || 'cover',
    '--portrait-focus': `${Number(focus[0]) * 100}% ${Number(focus[1]) * 100}%`,
    '--portrait-safe-scale': String(safeScale),
    '--preview-scale': String(previewScale.value),
  }
})
const exportConfig = computed(() => ({
  landscape: { width: 960, height: 540, pixelRatio: 2, output: '1920x1080' },
  portrait: { width: 720, height: 960, pixelRatio: 2, output: '1440x1920' },
  square: { width: 640, height: 640, pixelRatio: 2.5, output: '1600x1600' },
}[template.value]))
const previewStageStyle = computed(() => ({
  width: `${Math.max(1, Math.round(exportConfig.value.width * previewScale.value))}px`,
  height: `${Math.max(1, Math.round(exportConfig.value.height * previewScale.value))}px`,
}))
const onlineShortCode = computed(() => String(props.published?.code || '').trim())
const normalizedShareUrl = computed(() => /^https:\/\//i.test(shareUrl.value.trim()) ? shareUrl.value.trim() : '')
function capturedLevelLabel(value) {
  const level = Number(value)
  return Number.isFinite(level) && level > 0
    ? `Lv${level}`
    : tx('等级未记录', 'Level Not Captured')
}
const wrightstoneSummary = computed(() => {
  const wrightstone = selected.value?.weapon?.wrightstone
  const traits = Array.isArray(wrightstone?.traits)
    ? wrightstone.traits.slice(0, 3).map((trait, index) => ({
      key: String(trait?.hash || trait?.internalId || index),
      name: String(trait?.name || trait?.displayName || '').trim() || tx('词条名称未记录', 'Trait Name Not Captured'),
      levelLabel: capturedLevelLabel(trait?.level),
    }))
    : []
  return {
    name: String(wrightstone?.name || wrightstone?.displayName || '').trim(),
    traits,
  }
})
const summonRows = computed(() => {
  const captured = Array.isArray(selected.value?.summons)
    ? selected.value.summons
    : Array.isArray(preview.value?.summons) ? preview.value.summons : []
  return captured.slice(0, 4).map((summon, index) => {
    const name = String(summon?.name || summon?.typeName || summon?.typeHashHex || summon?.typeHash || '').trim()
    const main = String(summon?.mainTraitName || summon?.mainTrait || '').trim()
    const sub = String(summon?.subParamName || summon?.subParam || '').trim()
    const mainLevel = Number(summon?.mainTraitLevel)
    const subLevel = Number(summon?.subParamLevel)
    const traits = [
      main ? `${main}${Number.isFinite(mainLevel) ? ` Lv${mainLevel}` : ''}` : '',
      sub ? `${sub}${Number.isFinite(subLevel) ? ` Lv${subLevel}` : ''}` : '',
    ].filter(Boolean).join(' · ')
    return {
      key: String(summon?.slotId || summon?.index || summon?.typeHashHex || summon?.typeHash || index),
      name: name || tx('名称未记录', 'Name Not Captured'),
      traits: traits || tx('词条未记录', 'Traits Not Captured'),
      source: summon,
    }
  })
})
const traitTotalsFallback = computed(() => !Array.isArray(preview.value?.combinedSkills) || preview.value.combinedSkills.length === 0)
const traitTotals = computed(() => {
  const combined = preview.value?.combinedSkills
  if (Array.isArray(combined) && combined.length) {
    return combined
      .map(item => ({ name: item.name, level: Number(item.level) || 0, rawLevel: Number(item.rawLevel) || 0, sources: item.sources || [] }))
      .sort((a, b) => b.level - a.level || b.rawLevel - a.rawLevel || a.name.localeCompare(b.name, language.value === 'en' ? 'en' : 'zh-Hans-CN'))
      .slice(0, density.value === 'compact' ? 6 : 12)
  }
  const totals = new Map()
  const add = (name, level, source) => {
    if (!name) return
    const current = totals.get(name) || { name, level: 0, sources: [] }
    current.level += Number(level) || 0
    current.sources.push(source)
    totals.set(name, current)
  }
  for (const sigil of selected.value?.sigils || []) {
    add(sigil.primaryTraitName, sigil.primaryTraitLevel, tx('因子', 'Sigil'))
    add(sigil.secondaryTraitName, sigil.secondaryTraitLevel, tx('因子', 'Sigil'))
  }
  for (const skill of selected.value?.weapon?.skills || []) add(skill.name, skill.level, tx('武器', 'Weapon'))
  for (const trait of selected.value?.weapon?.wrightstone?.traits || []) add(trait.name, trait.level, tx('祝福', 'Wrightstone'))
  return [...totals.values()].map(item => ({ ...item, rawLevel: item.level })).sort((a, b) => b.level - a.level || a.name.localeCompare(b.name, language.value === 'en' ? 'en' : 'zh-Hans-CN')).slice(0, density.value === 'compact' ? 6 : 12)
})
const masteryDirection = computed(() => {
  const counts = new Map()
  for (const node of selected.value?.mastery || []) counts.set(node.cat || tx('基础', 'Base'), (counts.get(node.cat || tx('基础', 'Base')) || 0) + 1)
  return [...counts.entries()].sort((a, b) => b[1] - a[1]).map(([name, count]) => `${name} ${count}`).join(' · ') || tx('未记录', 'Not Captured')
})

function traitIcon(sigil) { return traitAssetIcon({ hash: sigil?.primaryTraitHash, name: sigil?.primaryTraitName }) }
function summonIcon(summon) { return summonAssetIcon({ typeHash: summon?.typeHashHex || summon?.typeHash }) }
function localDateValue(now = new Date()) {
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
function sanitizeFileName(value) { return String(value || 'GBFR-loadout').replace(/[\\/:*?"<>|]+/g, '_').slice(0, 80) }
async function updateQR() {
  const request = ++qrRequest
  const value = shareUrl.value.trim()
  if (!/^https:\/\//i.test(value)) {
    qrDataUrl.value = ''
    qrError.value = ''
    return
  }
  try {
    const QRCode = await import('qrcode')
    const result = await QRCode.toDataURL(value, { errorCorrectionLevel: 'M', margin: 4, width: 240, color: { dark: '#17344e', light: '#f7fcff' } })
    if (!exportLifecycle.disposed && request === qrRequest) {
      qrDataUrl.value = result
      qrError.value = ''
    }
  } catch (error) {
    if (!exportLifecycle.disposed && request === qrRequest) {
      qrDataUrl.value = ''
      qrError.value = String(error)
    }
  }
}
function scheduleQR() {
  qrPending = updateQR()
  return qrPending
}
async function waitForCurrentQR() {
  if (!/^https:\/\//i.test(shareUrl.value.trim())) return
  let pending
  do {
    pending = qrPending
    await pending
  } while (pending !== qrPending)
  if (!qrDataUrl.value) {
    throw new Error(tx(
      `二维码生成失败，PNG 未导出。请检查 HTTPS 链接后重试。${qrError.value ? `（${qrError.value}）` : ''}`,
      `QR generation failed, so the PNG was not exported. Check the HTTPS link and retry.${qrError.value ? ` (${qrError.value})` : ''}`,
    ))
  }
}
async function waitForImage(image, timeoutMs = 12000) {
  if (image.complete) {
    if (!image.naturalWidth) throw new Error(tx('分享图素材加载失败，请重试。', 'A share-image asset failed to load. Please retry.'))
    await image.decode?.().catch(() => {})
    return
  }
  await new Promise((resolve, reject) => {
    const finish = callback => {
      window.clearTimeout(timeout)
      image.removeEventListener('load', onLoad)
      image.removeEventListener('error', onError)
      callback()
    }
    const onLoad = () => finish(resolve)
    const onError = () => finish(() => reject(new Error(tx('分享图素材加载失败，请重试。', 'A share-image asset failed to load. Please retry.'))))
    const timeout = window.setTimeout(() => finish(() => reject(new Error(tx('分享图素材加载超时，请重试。', 'Share-image assets timed out. Please retry.')))), timeoutMs)
    image.addEventListener('load', onLoad, { once: true })
    image.addEventListener('error', onError, { once: true })
  })
}
async function waitForImages(root) {
  await Promise.all([...root.querySelectorAll('img')].map(image => waitForImage(image)))
}
function assertCanvasFits(root) {
  for (const selector of ['.canvas-content', '.canvas-sigils', '.canvas-summary', '.canvas-levels', '.canvas-full-detail', '.canvas-summons', '.canvas-wrightstone', '.canvas-online', '.canvas-footer']) {
    const element = root.querySelector(selector)
    if (!element) continue
    if (element.scrollHeight > element.clientHeight + 2 || element.scrollWidth > element.clientWidth + 2) {
      throw new Error(tx(
        `当前分享图的“${selector}”区域超出了固定画布（${element.scrollWidth}×${element.scrollHeight} / ${element.clientWidth}×${element.clientHeight}）。请改用“群聊摘要”或缩短标题、简介后重试。`,
        `The ${selector} region exceeds the fixed canvas (${element.scrollWidth}×${element.scrollHeight} / ${element.clientWidth}×${element.clientHeight}). Use Chat Summary or shorten the title and description, then try again.`,
      ))
    }
  }
  const content = root.querySelector('.canvas-content')?.getBoundingClientRect()
  const footer = root.querySelector('.canvas-footer')?.getBoundingClientRect()
  if (content && footer &&
      content.right > footer.left + 1 &&
      content.left < footer.right - 1 &&
      content.bottom > footer.top - 1) {
    throw new Error(tx(
      '当前分享图的正文与二维码页脚发生重叠。请改用“群聊摘要”、缩短文字，或移除 HTTPS 链接后重试。',
      'The share-image body overlaps the QR footer. Use Chat Summary, shorten the text, or remove the HTTPS link and retry.',
    ))
  }
}
async function renderPNG(generation) {
  exportLifecycle.assertCurrent(generation)
  if (!canvasRef.value || !selected.value) throw new Error(tx('没有可导出的配装', 'No loadout is available for export'))
  await waitForCurrentQR()
  exportLifecycle.assertCurrent(generation)
  await nextTick()
  exportLifecycle.assertCurrent(generation)
  const canvas = canvasRef.value
  if (!canvas) throw new ShareImageExportCancelledError()
  await waitForImages(canvas)
  exportLifecycle.assertCurrent(generation)
  if (document.fonts?.ready) await document.fonts.ready
  exportLifecycle.assertCurrent(generation)
  assertCanvasFits(canvasRef.value)
  const { toPng } = await import('html-to-image')
  exportLifecycle.assertCurrent(generation)
  const dataUrl = await toPng(canvasRef.value, {
    width: exportConfig.value.width,
    height: exportConfig.value.height,
    pixelRatio: exportConfig.value.pixelRatio,
    cacheBust: false,
    backgroundColor: '#dff4ff',
    skipFonts: false,
    style: { transform: 'none', transformOrigin: 'top left' },
  })
  exportLifecycle.assertCurrent(generation)
  return dataUrl
}

function exportTimeoutError() {
  return new Error(tx(
    '分享图生成超过 20 秒，已停止本次导出。请等待素材加载完成后重试。',
    'Share-image rendering exceeded 20 seconds and was stopped. Wait for assets to finish loading, then retry.',
  ))
}

async function runExport(effect, successMessage) {
  if (exportBusy.value || exportLifecycle.disposed) return
  const generation = exportLifecycle.begin()
  exportBusy.value = true
  let renderTask = null
  let timeoutReported = false
  try {
    renderTask = Promise.resolve().then(() => renderPNG(generation))
    const url = await withShareImageExportTimeout(
      () => renderTask,
      SHARE_IMAGE_EXPORT_TIMEOUT_MS,
      exportTimeoutError,
      () => {
        timeoutReported = true
        exportLifecycle.invalidate(generation)
      },
    )
    exportLifecycle.assertCurrent(generation)
    await effect(url, generation)
    exportLifecycle.assertCurrent(generation)
    emit('status', successMessage(), 'success')
  } catch (error) {
    if (!(error instanceof ShareImageExportCancelledError) && (timeoutReported || exportLifecycle.isCurrent(generation))) {
      emit('status', String(error), 'error')
    }
  } finally {
    // html-to-image cannot be force-aborted while it is encoding. Keep this
    // workshop single-flight until that encoder actually settles; the timeout
    // invalidates its generation so it can never reach a save/clipboard effect.
    if (renderTask) await renderTask.catch(() => {})
    exportLifecycle.invalidate(generation)
    exportBusy.value = false
  }
}

async function download() {
  if (exportBusy.value) return
  const outputLabel = exportConfig.value.output
  const filename = `${sanitizeFileName(title.value || `${props.group?.charaName}-${selected.value?.name || tx('配装', 'Loadout')}`)}.png`
  await runExport(async (url, generation) => {
    exportLifecycle.assertCurrent(generation)
    const outputPath = await SaveLoadoutSharePNG(filename, url)
    if (!outputPath) throw new ShareImageExportCancelledError()
    exportLifecycle.assertCurrent(generation)
  }, () => tx(`已生成 ${outputLabel} 分享图`, `${outputLabel} share image generated`))
}
async function copyPNG() {
  if (exportBusy.value) return
  if (!navigator.clipboard?.write || typeof ClipboardItem === 'undefined') {
    emit('status', tx('当前 WebView 不支持图片剪贴板，请使用下载 PNG。', 'This WebView does not support image clipboard; download the PNG instead.'), 'error')
    return
  }
  await runExport(async (dataUrl, generation) => {
    const response = await fetch(dataUrl)
    exportLifecycle.assertCurrent(generation)
    const blob = await response.blob()
    exportLifecycle.assertCurrent(generation)
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
  }, () => tx('分享图已复制到剪贴板', 'Share image copied to clipboard'))
}

watch(() => props.group?.charaHash, () => {
  loadoutId.value = loadouts.value[0]?.unitId || 0
  title.value = props.group?.charaName ? tx(`${props.group.charaName} 配装记录`, `${props.group.charaName} Loadout`) : ''
}, { immediate: true })
watch(() => props.published?.url, (value, previous) => {
  const current = shareUrl.value.trim()
  if (value && (!current || current === String(previous || '').trim())) shareUrl.value = String(value)
}, { immediate: true })
watch(shareUrl, scheduleQR, { immediate: true })
watch([template, () => props.embedded], () => nextTick(updatePreviewScale))

function updatePreviewScale() {
  const shell = previewShellRef.value
  if (!shell) return
  const horizontalPadding = 24
  const available = Math.max(1, shell.clientWidth - horizontalPadding)
  previewScale.value = Math.min(1, available / exportConfig.value.width)
}

onMounted(() => {
  updatePreviewScale()
  if (typeof ResizeObserver !== 'undefined') {
    previewResizeObserver = new ResizeObserver(updatePreviewScale)
    previewResizeObserver.observe(previewShellRef.value)
  } else {
    window.addEventListener('resize', updatePreviewScale)
  }
})
onBeforeUnmount(() => {
  exportLifecycle.dispose()
  exportBusy.value = false
  qrRequest += 1
  previewResizeObserver?.disconnect()
  window.removeEventListener('resize', updatePreviewScale)
})
</script>

<template>
  <section class="share-workshop" :class="{ 'is-embedded': embedded }" :aria-label="tx('分享图工坊', 'Share Image Workshop')">
    <header class="share-heading"><div><small>{{ tx('固定画布导出', 'Fixed-Canvas Export') }}</small><h2>{{ tx('分享图工坊', 'Share Image Workshop') }}</h2><p>{{ tx('预览和 PNG 使用同一套字段；导出前会等待角色、武器和因子图片完成解码，不生成空白图。', 'Preview and PNG use the same fields. Export waits for character, weapon, and sigil images to decode, preventing blank artwork.') }}</p></div><span>{{ exportConfig.output }}</span></header>
    <div class="share-layout">
      <fieldset class="share-controls ui-card is-flat" :disabled="exportBusy" :aria-busy="exportBusy">
        <label><span>{{ tx('配装', 'Loadout') }}</span><select v-model="loadoutId" class="ui-select"><option v-for="item in loadouts" :key="item.unitId" :value="item.unitId">{{ item.isParty ? tx('当前实时配装', 'Current Live Loadout') : item.name || `${tx('槽位', 'Slot')} ${item.slot}` }}</option></select></label>
        <div><span>{{ tx('画布', 'Canvas') }}</span><div class="ui-seg"><button type="button" class="ui-seg-btn" :class="{ 'is-on': template === 'landscape' }" @click="template = 'landscape'">16:9</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': template === 'portrait' }" @click="template = 'portrait'">3:4</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': template === 'square' }" @click="template = 'square'">1:1</button></div></div>
        <label><span>{{ tx('标题', 'Title') }}</span><input v-model="title" class="ui-input" maxlength="48" /></label>
        <label><span>{{ tx('简介', 'Description') }}</span><textarea v-model="description" class="ui-textarea" maxlength="140" :placeholder="tx('这套配装适合什么场景', 'What this loadout is built for')"></textarea></label>
        <label><span>{{ tx('署名（可空）', 'Author (optional)') }}</span><input v-model="author" class="ui-input" maxlength="24" /></label>
        <label><span>{{ tx('HTTPS 分享链接（自动复用）', 'HTTPS Share Link (reused automatically)') }}</span><input v-model="shareUrl" class="ui-input" placeholder="https://…" /><small v-if="published?.url" class="share-link-source">{{ tx('已使用这套配装刚生成的短链；仍可手动覆盖。', 'Using the short link just published for this loadout; you can still override it.') }}</small><small v-else class="share-link-source">{{ tx('先生成线上短码后会自动带入；也可手动填写。', 'Publishing a short code fills this automatically; manual links remain available.') }}</small></label>
        <div class="share-meta-controls">
          <label class="share-meta-option"><span><input v-model="showGameVersion" type="checkbox" />{{ tx('显示游戏版本', 'Show Game Version') }}</span><input v-model="gameVersion" class="ui-input" maxlength="16" :disabled="!showGameVersion" /></label>
          <label class="share-meta-option"><span><input v-model="showGeneratedDate" type="checkbox" />{{ tx('显示生成日期', 'Show Generated Date') }}</span><input v-model="generatedDate" class="ui-input" type="date" :disabled="!showGeneratedDate" /></label>
          <label class="share-meta-option is-toggle-only"><span><input v-model="showProjectMark" type="checkbox" />{{ tx('显示项目标识', 'Show Project Mark') }}</span><small>{{ tx('关闭后导出图不显示工具名称', 'Hide the tool name from the exported image') }}</small></label>
        </div>
        <label class="opacity-control"><span>{{ tx('角色立绘强度', 'Character Art Strength') }} · {{ portraitOpacity }}%</span><input v-model.number="portraitOpacity" type="range" min="64" max="100" step="1" /></label>
        <div><span>{{ tx('信息密度', 'Information Density') }}</span><div class="ui-seg"><button type="button" class="ui-seg-btn" :class="{ 'is-on': density === 'compact' }" @click="density = 'compact'">{{ tx('群聊摘要', 'Chat Summary') }}</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': density === 'standard' }" @click="density = 'standard'">{{ tx('标准', 'Standard') }}</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': density === 'full' }" @click="density = 'full'">{{ tx('完整', 'Full') }}</button></div></div>
        <div class="share-actions"><button type="button" class="ui-btn share-action" :disabled="exportBusy || !selected" @click="copyPNG">{{ tx('复制 PNG', 'Copy PNG') }}</button><button type="button" class="ui-btn share-action share-action-primary" :disabled="exportBusy || !selected" @click="download">{{ exportBusy ? tx('正在生成…', 'Rendering…') : tx('下载 PNG', 'Download PNG') }}</button></div>
      </fieldset>

      <div ref="previewShellRef" class="share-preview-shell" :class="`is-${template}`">
        <div class="share-preview-stage" :style="previewStageStyle">
          <div ref="canvasRef" class="share-canvas" :class="[`is-${template}`, `density-${density}`, { 'has-qr': !!qrDataUrl }]" :style="canvasStyle">
            <img v-if="cardBackdrop" class="share-card-backdrop" :src="cardBackdrop" alt="" decoding="async" />
            <img v-if="portrait" class="share-portrait" :src="portrait" alt="" decoding="async" />
            <div class="share-wash"></div>
            <header class="canvas-title"><div><small>GRANBLUE FANTASY: RELINK · LOADOUT LOG</small><h3>{{ title || `${group?.charaName || ''} ${tx('配装记录', 'Loadout')}` }}</h3><p v-if="description">{{ description }}</p></div><div class="canvas-character"><img v-if="characterIcon" :src="characterIcon" alt="" /><span v-else class="canvas-character-fallback" aria-hidden="true">◆</span><span><b>{{ group?.charaName }}</b><small>{{ selected?.isParty ? tx('当前实时配装', 'Current Live Loadout') : selected?.name }}</small></span></div></header>
            <main class="canvas-content">
              <section class="canvas-weapon"><img v-if="weaponIcon" :src="weaponIcon" alt="" /><span v-else class="canvas-weapon-icon-fallback" aria-hidden="true">◇</span><div><small>{{ tx('武器', 'WEAPON') }}</small><strong>{{ selected?.weaponName || tx('未记录武器', 'Weapon Not Captured') }}</strong><span v-if="selected?.weapon">Lv{{ selected.weapon.level }} · {{ tx('觉醒', 'Awakening') }} {{ selected.weapon.awakening }} · {{ tx('超凡', 'Transcendence') }} {{ selected.weapon.transcendence }}</span></div><p>{{ (selected?.weapon?.skills || []).map(item => `${item.name} Lv${item.level}`).join(' · ') || tx('武器技能未记录', 'Weapon skills not captured') }}</p></section>
              <section class="canvas-sigils"><small>{{ tx('因子配置', 'SIGIL LOADOUT') }} · {{ Math.min(selected?.sigils?.length || 0, 12) }}/12</small><div><article v-for="(sigil, index) in (selected?.sigils || []).slice(0, 12)" :key="`${sigil.slotId}-${index}`"><img v-if="traitIcon(sigil)" :src="traitIcon(sigil)" alt="" /><span v-else class="canvas-icon-fallback" aria-hidden="true">◇</span><b>{{ String(index + 1).padStart(2, '0') }}</b><span><strong>{{ sigil.name || tx('因子', 'Sigil') }}</strong><small>{{ sigil.primaryTraitName }} Lv{{ sigil.primaryTraitLevel }}<template v-if="sigil.secondaryTraitName"> · {{ sigil.secondaryTraitName }} Lv{{ sigil.secondaryTraitLevel }}</template></small></span></article></div></section>
              <section class="canvas-summary"><div><small>{{ tx('专精方向', 'MASTERY') }}</small><strong>{{ masteryDirection }}</strong></div><div><small class="canvas-ledger-scope">{{ traitTotalsFallback ? tx('原始等级 · 未含召唤石', 'Raw levels · summons excluded') : tx('有效合并等级', 'Effective combined levels') }}</small><div class="canvas-levels"><span v-for="trait in traitTotals" :key="trait.name"><b>{{ trait.name }}</b><em>Lv{{ trait.level }}<small v-if="trait.rawLevel > trait.level">/{{ trait.rawLevel }}</small></em></span></div></div></section>
              <section v-if="density === 'full'" class="canvas-full-detail">
                <div class="canvas-summons"><small>{{ tx('召唤石摘要', 'SUMMONS') }}</small><div v-if="summonRows.length"><article v-for="summon in summonRows" :key="summon.key"><img v-if="summonIcon(summon.source)" :src="summonIcon(summon.source)" alt="" /><span v-else aria-hidden="true">◇</span><p><b>{{ summon.name }}</b><em>{{ summon.traits }}</em></p></article></div><p v-else class="canvas-missing">{{ tx('召唤石未记录', 'Summons Not Captured') }}</p></div>
                <div class="canvas-wrightstone"><small>{{ tx('武器祝福', 'WRIGHTSTONE') }}</small><b>{{ wrightstoneSummary.name || tx('祝福名称未记录', 'Wrightstone Name Not Captured') }}</b><div v-if="wrightstoneSummary.traits.length"><span v-for="trait in wrightstoneSummary.traits" :key="trait.key"><em>{{ trait.name }}</em><strong>{{ trait.levelLabel }}</strong></span></div><p v-else class="canvas-missing">{{ tx('祝福词条未记录', 'Wrightstone Traits Not Captured') }}</p></div>
                <div class="canvas-online"><small>{{ tx('线上分享', 'ONLINE SHARE') }}</small><dl><div><dt>{{ tx('短码', 'Code') }}</dt><dd>{{ onlineShortCode || tx('未生成', 'Not Generated') }}</dd></div><div><dt>HTTPS</dt><dd>{{ normalizedShareUrl || tx('未填写', 'Not Provided') }}</dd></div></dl></div>
              </section>
            </main>
            <footer class="canvas-footer"><div class="canvas-meta"><span v-if="author">{{ tx('整理', 'By') }} · {{ author }}</span><span v-if="showProjectMark">GBFR PE Patch Tool</span><span v-if="showGameVersion && gameVersion">GBFR {{ gameVersion }}</span><span v-if="showGeneratedDate && generatedDate">{{ generatedDate }}</span></div><div v-if="qrDataUrl" class="canvas-qr"><span>{{ normalizedShareUrl }}</span><img :src="qrDataUrl" alt="" /></div><div v-else class="canvas-stats"><span>HP <b>{{ preview?.finalStats?.hp?.toLocaleString?.() || '—' }}</b></span><span>ATK <b>{{ preview?.finalStats?.attack?.toLocaleString?.() || '—' }}</b></span></div></footer>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.share-workshop { min-width:0; display:grid; gap:var(--space-4); container:share / inline-size; }
.share-workshop.is-embedded { padding-top:var(--space-2); border-top:1px dashed var(--border-soft); }
.share-heading { min-width:0; display:flex; align-items:end; justify-content:space-between; gap:var(--space-4); padding:2px var(--space-2); border-bottom:1px solid var(--border-soft); }
.share-heading small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.share-heading h2 { margin:2px 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); letter-spacing:0; }
.share-heading p { margin:0 0 var(--space-3); color:var(--text-muted); font-size:var(--fs-sm); }
.share-heading > span { flex:0 0 auto; padding:var(--space-2) var(--space-3); border-left:2px solid var(--accent); color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-xs); }
.share-layout { min-width:0; display:grid; grid-template-columns:minmax(250px,320px) minmax(0,1fr); gap:var(--space-4); align-items:start; }
.share-controls { min-width:0; display:grid; gap:var(--space-3); padding:var(--space-4); }
.share-controls[disabled] { opacity:.84; }
.share-controls.ui-card { margin:0; border-width:1px; }
.share-controls > label,.share-controls > div:not(.share-actions) { min-width:0; display:grid; gap:5px; }
.share-controls span { color:var(--text-muted); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.share-controls .ui-input,.share-controls .ui-select,.share-controls .ui-textarea,.share-controls .ui-seg { width:100%; max-width:100%; min-width:0; }
.share-controls .ui-seg { flex-wrap:wrap; }
.share-controls textarea { min-height:72px; resize:vertical; }
.share-link-source { color:var(--text-muted); font-size:var(--fs-2xs); font-weight:var(--fw-normal); line-height:var(--lh-normal); }
.share-meta-controls { display:grid; gap:var(--space-2); padding:var(--space-3); border:1px solid color-mix(in srgb,var(--accent) 16%,var(--border-soft)); background:color-mix(in srgb,var(--surface-card-pop) 88%,#e8f6ff); }
.share-meta-option { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) minmax(112px,.72fr); gap:var(--space-2); align-items:center; }
.share-meta-option > span { min-width:0; display:flex; align-items:center; gap:7px; color:var(--text-secondary); }
.share-meta-option input[type="checkbox"] { width:16px; height:16px; margin:0; accent-color:var(--accent); }
.share-meta-option.is-toggle-only { grid-template-columns:minmax(0,1fr); }
.share-meta-option small { color:var(--text-muted); font-size:var(--fs-2xs); font-weight:var(--fw-normal); line-height:var(--lh-normal); }
.opacity-control input {
  width:100%;
  height:20px;
  margin:0;
  appearance:none;
  background:transparent;
  cursor:pointer;
}
.opacity-control input::-webkit-slider-runnable-track { height:5px; border:1px solid var(--border-default); border-radius:var(--radius-pill); background:var(--surface-sunken); }
.opacity-control input::-webkit-slider-thumb { width:16px; height:16px; margin-top:-6px; appearance:none; border:2px solid var(--surface-card-pop); border-radius:50%; background:var(--accent); box-shadow:0 0 0 1px var(--accent-border),var(--shadow-1); }
.opacity-control input::-moz-range-track { height:5px; border:1px solid var(--border-default); border-radius:var(--radius-pill); background:var(--surface-sunken); }
.opacity-control input::-moz-range-thumb { width:12px; height:12px; border:2px solid var(--surface-card-pop); border-radius:50%; background:var(--accent); box-shadow:0 0 0 1px var(--accent-border),var(--shadow-1); }
.opacity-control input:focus-visible { outline:var(--focus-outline); outline-offset:var(--focus-offset); box-shadow:var(--focus-ring); }
.share-actions { display:grid; grid-template-columns:1fr 1fr; gap:var(--space-2); }
.share-action { border-color:color-mix(in srgb,var(--accent) 22%,var(--border-soft)); background:color-mix(in srgb,var(--surface-card-pop) 90%,#e5f5ff); box-shadow:none; color:var(--text-secondary); }
.share-action:hover:not(:disabled) { border-color:color-mix(in srgb,var(--accent) 46%,var(--border-soft)); background:color-mix(in srgb,var(--surface-card-pop) 78%,#d8f1ff); color:var(--accent-hover); }
.share-action-primary { border-color:color-mix(in srgb,var(--accent) 38%,var(--border-soft)); background:color-mix(in srgb,var(--surface-card-pop) 70%,#c9ebff); color:var(--accent-hover); }
.share-preview-shell { width:100%; max-width:100%; min-width:0; display:grid; place-items:center; overflow:hidden; padding:var(--space-3); border:1px solid color-mix(in srgb,var(--accent) 16%,var(--border-soft)); background:linear-gradient(145deg,rgba(231,247,255,.5),rgba(184,220,241,.22)); }
.share-preview-stage { position:relative; min-width:0; flex:none; overflow:visible; transition:width .16s ease,height .16s ease; }
.share-canvas { position:absolute; isolation:isolate; top:0; left:0; flex:none; overflow:hidden; transform:scale(var(--preview-scale,1)); transform-origin:top left; border:1px solid rgba(44,106,157,.54); background:linear-gradient(135deg,#f7fcff 0%,#dff4ff 34%,#a8dbf7 70%,#6eafe2 100%); color:#17324b; box-shadow:0 18px 44px rgba(25,76,119,.24),inset 0 0 0 1px rgba(255,255,255,.62); font-family:var(--font-body); }
.share-canvas.is-landscape { width:960px; height:540px; }
.share-canvas.is-portrait { width:720px; height:960px; }
.share-canvas.is-square { width:640px; height:640px; }
.share-card-backdrop { position:absolute; z-index:0; inset:0; width:100%; height:100%; object-fit:cover; opacity:.92; }
.share-portrait { position:absolute; z-index:1; top:-18%; right:-10%; width:88%; height:138%; transform:scale(var(--portrait-safe-scale,1)); transform-origin:var(--portrait-focus,50% 28%); object-fit:var(--portrait-fit,cover); object-position:var(--portrait-focus,50% 28%); opacity:var(--portrait-opacity); filter:saturate(1.12) contrast(1.08) drop-shadow(0 18px 30px rgba(28,85,132,.32)); }
.share-wash { position:absolute; z-index:2; inset:0; pointer-events:none; background:
  linear-gradient(90deg,rgba(247,252,255,.84) 0%,rgba(235,248,255,.56) 38%,rgba(213,239,253,.16) 65%,rgba(170,216,244,.03) 100%),
  radial-gradient(circle at 78% 18%,rgba(255,255,255,.2),transparent 28%),
  linear-gradient(145deg,transparent 0 54%,rgba(66,151,213,.13) 54.2% 55%,transparent 55.2% 100%);
}
.share-wash::before { content:""; position:absolute; top:-18%; right:17%; width:280px; height:720px; transform:rotate(32deg); border:1px solid rgba(255,255,255,.68); background:linear-gradient(90deg,transparent,rgba(255,255,255,.2),transparent); }
.share-wash::after { content:""; position:absolute; inset:18px; border:1px solid rgba(45,121,177,.3); box-shadow:inset 0 0 0 4px rgba(255,255,255,.25); pointer-events:none; }
.canvas-title { position:relative; z-index:3; min-width:0; height:126px; box-sizing:border-box; display:flex; justify-content:space-between; align-items:start; gap:24px; overflow:hidden; padding:28px 42px 12px; border-bottom:1px solid rgba(54,123,174,.3); }
.canvas-title > div:first-child { min-width:0; }
.canvas-title small,.canvas-content > section > small,.canvas-weapon small,.canvas-summary small { color:#3178ad; font-size:10px; font-weight:800; letter-spacing:.04em; }
.canvas-title h3 { max-width:720px; margin:3px 0; display:-webkit-box; overflow:hidden; overflow-wrap:anywhere; -webkit-box-orient:vertical; -webkit-line-clamp:1; color:#16334d; font-family:var(--font-display); font-size:28px; line-height:1.08; letter-spacing:0; text-shadow:0 1px rgba(255,255,255,.72); }
.canvas-title p { max-width:640px; margin:4px 0 0; display:-webkit-box; overflow:hidden; overflow-wrap:anywhere; -webkit-box-orient:vertical; -webkit-line-clamp:2; color:#41657e; font-size:12px; line-height:1.35; }
.canvas-character { flex:0 0 auto; display:flex; align-items:center; gap:9px; }
.canvas-character > img,.canvas-character-fallback { width:54px; height:54px; box-sizing:border-box; display:grid; place-items:center; border:2px solid rgba(65,137,190,.62); border-radius:7px; background:rgba(241,250,255,.82); object-fit:cover; box-shadow:0 8px 20px rgba(38,101,148,.14); color:#4387b8; }
.canvas-character > span { display:grid; text-align:right; }
.canvas-character b { color:#17334c; font-size:15px; }
.canvas-character small { max-width:150px; overflow:hidden; text-overflow:ellipsis; color:#47718d; white-space:nowrap; }
.canvas-content { position:absolute; z-index:3; top:126px; right:42px; bottom:88px; left:42px; min-width:0; display:grid; grid-template-columns:minmax(0,1.4fr) minmax(250px,.6fr); grid-template-rows:82px minmax(0,1fr); gap:10px 18px; overflow:hidden; padding:10px 0; }
.canvas-weapon { min-width:0; grid-column:1 / -1; display:grid; grid-template-columns:132px minmax(0,1fr) minmax(200px,.7fr); align-items:center; gap:12px; padding:6px 12px; border:1px solid rgba(71,143,194,.2); background:rgba(247,252,255,.64); box-shadow:inset 0 1px rgba(255,255,255,.72); backdrop-filter:blur(4px); }
.canvas-weapon > img,.canvas-weapon-icon-fallback { width:132px; height:62px; box-sizing:border-box; display:grid; place-items:center; padding:6px 9px; border:1px solid rgba(47,125,181,.24); border-radius:6px; background:linear-gradient(135deg,rgba(255,255,255,.8),rgba(202,232,250,.7)); color:#4387b8; font-size:24px; object-fit:cover; object-position:center; filter:drop-shadow(0 2px 2px rgba(28,77,113,.35)); }
.canvas-weapon > div { min-width:0; display:grid; }
.canvas-weapon strong { overflow-wrap:anywhere; color:#17344e; font-size:16px; }
.canvas-weapon span,.canvas-weapon p { margin:0; color:#4b6e85; font-size:10px; line-height:1.35; }
.canvas-weapon p { display:-webkit-box; overflow:hidden; -webkit-box-orient:vertical; -webkit-line-clamp:2; }
.canvas-sigils { min-width:0; }
.canvas-sigils > div { min-width:0; display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:4px 6px; margin-top:6px; }
.canvas-sigils article { min-width:0; display:grid; grid-template-columns:22px 16px minmax(0,1fr); gap:4px; align-items:center; min-height:31px; padding:2px 5px; border:1px solid rgba(71,143,194,.16); border-left:2px solid rgba(48,126,183,.55); background:rgba(247,252,255,.58); backdrop-filter:blur(3px); }
.canvas-sigils article > img,.canvas-icon-fallback { width:22px; height:22px; display:grid; place-items:center; border-radius:4px; background:rgba(221,242,254,.78); color:#4387b8; object-fit:cover; }
.canvas-sigils article > b { color:#3e82b4; font-family:var(--font-data); font-size:9px; }
.canvas-sigils article > span { min-width:0; display:grid; }
.canvas-sigils article strong { min-width:0; overflow:hidden; text-overflow:ellipsis; color:#19364f; font-size:10px; white-space:nowrap; }
.canvas-sigils article small { min-width:0; overflow:hidden; text-overflow:ellipsis; color:#4e728a; font-size:8px; white-space:nowrap; }
.canvas-summary { min-width:0; display:grid; align-content:start; gap:8px; }
.canvas-summary > div { min-width:0; display:grid; align-content:start; gap:4px; padding:8px; border:1px solid rgba(71,143,194,.22); background:rgba(247,252,255,.62); backdrop-filter:blur(3px); }
.canvas-summary strong { overflow-wrap:anywhere; color:#19364f; font-size:11px; }
.canvas-ledger-scope { display:block; margin-bottom:4px; }
.canvas-levels { min-width:0; display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:4px; }
.canvas-levels span { min-width:0; display:flex; justify-content:space-between; gap:5px; padding:4px 6px; border:1px solid rgba(71,143,194,.12); background:rgba(247,252,255,.56); }
.canvas-levels b { min-width:0; overflow:hidden; text-overflow:ellipsis; color:#31566f; font-size:9px; white-space:nowrap; }
.canvas-levels em { flex:0 0 auto; color:#2876ab; font-size:9px; font-style:normal; font-weight:700; }
.canvas-full-detail { min-width:0; min-height:0; display:grid; align-content:start; gap:7px; overflow:hidden; }
.canvas-full-detail > div { min-width:0; overflow:hidden; padding:7px; border:1px solid rgba(71,143,194,.22); background:rgba(247,252,255,.64); backdrop-filter:blur(3px); }
.canvas-full-detail small { display:block; margin-bottom:4px; color:#3178ad; font-size:9px; font-weight:800; letter-spacing:.04em; }
.canvas-summons > div { min-width:0; display:grid; gap:3px; }
.canvas-summons article { min-width:0; display:grid; grid-template-columns:22px minmax(0,1fr); gap:5px; align-items:center; }
.canvas-summons article > img,.canvas-summons article > span { width:22px; height:22px; display:grid; place-items:center; border-radius:4px; background:rgba(221,242,254,.78); color:#4387b8; object-fit:cover; }
.canvas-summons article p { min-width:0; margin:0; display:grid; }
.canvas-summons article b,.canvas-summons article em { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.canvas-summons article b { color:#19364f; font-size:9px; }
.canvas-summons article em { color:#4e728a; font-size:7px; font-style:normal; }
.canvas-missing { margin:0; color:#4e728a; font-size:9px; }
.canvas-wrightstone > b { min-width:0; display:block; overflow:hidden; text-overflow:ellipsis; color:#19364f; font-size:9px; white-space:nowrap; }
.canvas-wrightstone > div { min-width:0; display:grid; gap:2px; margin-top:3px; }
.canvas-wrightstone span { min-width:0; display:flex; justify-content:space-between; gap:5px; color:#4e728a; font-size:8px; }
.canvas-wrightstone em { min-width:0; overflow:hidden; text-overflow:ellipsis; font-style:normal; white-space:nowrap; }
.canvas-wrightstone strong { flex:0 0 auto; color:#2876ab; font-size:8px; }
.canvas-online dl { min-width:0; margin:0; display:grid; gap:3px; }
.canvas-online dl > div { min-width:0; display:grid; grid-template-columns:34px minmax(0,1fr); gap:5px; }
.canvas-online dt { color:#4e728a; font-size:8px; }
.canvas-online dd { min-width:0; margin:0; overflow:hidden; text-overflow:ellipsis; color:#19364f; font-family:var(--font-data); font-size:8px; font-weight:700; white-space:nowrap; }
.canvas-footer { position:absolute; z-index:3; right:42px; bottom:24px; left:42px; display:flex; align-items:end; justify-content:space-between; gap:16px; padding-top:8px; border-top:1px solid rgba(54,123,174,.3); }
.canvas-meta { min-width:0; display:flex; flex-wrap:wrap; gap:4px 10px; overflow:hidden; color:#4b7089; font-size:8px; }
.canvas-meta span { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.canvas-qr { display:flex; align-items:end; gap:7px; }
.canvas-qr span { max-width:190px; overflow:hidden; text-overflow:ellipsis; color:#4b7089; font-size:7px; white-space:nowrap; }
.canvas-qr img { width:54px; height:54px; border:4px solid #f7fcff; }
.canvas-stats { display:flex; gap:12px; color:#4b7089; font-size:9px; }
.canvas-stats b { color:#19364f; }
.share-canvas.is-landscape.density-full .canvas-content { bottom:52px; grid-template-columns:minmax(0,1.35fr) minmax(220px,.82fr) minmax(184px,.66fr); grid-template-rows:76px minmax(0,1fr); gap:8px 10px; padding:8px 0; }
.share-canvas.has-qr .canvas-content { bottom:106px; }
.share-canvas.is-landscape.density-full.has-qr .canvas-content { bottom:100px; grid-template-rows:70px minmax(0,1fr); gap:6px 10px; padding:4px 0; }
.share-canvas.is-landscape.density-full .canvas-weapon { grid-column:1 / -1; grid-template-columns:120px minmax(0,1fr) minmax(180px,.6fr); }
.share-canvas.is-landscape.density-full .canvas-weapon > img,.share-canvas.is-landscape.density-full .canvas-weapon-icon-fallback { width:120px; height:56px; }
.share-canvas.is-landscape.density-full.has-qr .canvas-weapon > img,.share-canvas.is-landscape.density-full.has-qr .canvas-weapon-icon-fallback { height:52px; }
.share-canvas.is-landscape.density-full .canvas-sigils { grid-column:1; grid-row:2; }
.share-canvas.is-landscape.density-full .canvas-summary { grid-column:2; grid-row:2; gap:4px; }
.share-canvas.is-landscape.density-full .canvas-summary > div { gap:2px; padding:5px; }
.share-canvas.is-landscape.density-full.has-qr .canvas-summary > div { padding:4px; }
.share-canvas.is-landscape.density-full .canvas-ledger-scope { margin-bottom:2px; }
.share-canvas.is-landscape.density-full .canvas-levels { grid-template-columns:repeat(2,minmax(0,1fr)); }
.share-canvas.is-landscape.density-full .canvas-levels span { padding:3px 4px; }
.share-canvas.is-landscape.density-full .canvas-full-detail { grid-column:3; grid-row:2; gap:4px; }
.share-canvas.is-landscape.density-full .canvas-full-detail > div { padding:5px; }
.share-canvas.is-landscape.density-full .canvas-full-detail small { margin-bottom:2px; }
.share-canvas.is-portrait .share-portrait { top:-13%; right:-26%; width:122%; height:130%; }
.share-canvas.is-portrait .canvas-title { height:144px; padding:24px 32px 10px; }
.share-canvas.is-portrait .canvas-title h3 { -webkit-line-clamp:2; }
.share-canvas.is-portrait .canvas-title p { -webkit-line-clamp:3; }
.share-canvas.is-portrait .share-wash { background:
  linear-gradient(180deg,rgba(247,252,255,.78) 0%,rgba(232,247,255,.52) 44%,rgba(205,235,252,.12) 72%,rgba(170,216,244,.02) 100%),
  linear-gradient(90deg,rgba(247,252,255,.24) 0%,rgba(235,248,255,.1) 52%,rgba(213,239,253,.02) 100%),
  radial-gradient(circle at 78% 18%,rgba(255,255,255,.18),transparent 28%);
}
.share-canvas.is-portrait .canvas-content { top:144px; right:32px; bottom:90px; left:32px; grid-template-columns:1fr; grid-template-rows:auto minmax(0,1fr) auto; align-content:stretch; gap:16px; padding:16px 0; }
.share-canvas.is-portrait .canvas-weapon { grid-template-columns:110px minmax(0,1fr); }
.share-canvas.is-portrait .canvas-weapon > img,.share-canvas.is-portrait .canvas-weapon-icon-fallback { width:110px; height:62px; }
.share-canvas.is-portrait .canvas-sigils { min-height:0; display:grid; grid-template-rows:auto minmax(0,1fr); }
.share-canvas.is-portrait .canvas-sigils > div { min-height:0; grid-template-columns:repeat(2,minmax(0,1fr)); grid-template-rows:repeat(6,minmax(0,1fr)); }
.share-canvas.is-portrait .canvas-sigils article { min-height:0; }
.share-canvas.is-portrait .canvas-weapon p { grid-column:1 / -1; }
.share-canvas.is-portrait .canvas-summary { grid-template-columns:1fr 1.4fr; grid-template-rows:auto; align-content:start; }
.share-canvas.is-portrait .canvas-summary > div { box-sizing:border-box; }
.share-canvas.is-portrait.density-full .canvas-content { grid-template-columns:1fr; grid-template-rows:82px minmax(0,1fr) auto auto; gap:12px; padding:12px 0; }
.share-canvas.is-portrait.density-full .canvas-weapon { grid-column:1; grid-row:1; }
.share-canvas.is-portrait.density-full .canvas-sigils { grid-column:1; grid-row:2; }
.share-canvas.is-portrait.density-full .canvas-summary { grid-column:1; grid-row:3; }
.share-canvas.is-portrait.density-full .canvas-full-detail { grid-column:1; grid-row:4; grid-template-columns:1.35fr 1fr 1fr; }
.share-canvas.is-portrait.has-qr .canvas-content { bottom:108px; }
.share-canvas.is-portrait.density-full .canvas-summons > div { grid-template-columns:repeat(2,minmax(0,1fr)); }
.share-canvas.is-portrait .canvas-footer,.share-canvas.is-square .canvas-footer { right:32px; left:32px; }
.share-canvas.is-square .share-portrait { top:-24%; right:-22%; width:118%; height:132%; }
.share-canvas.is-square .canvas-title { height:110px; padding:14px 32px 6px; }
.share-canvas.is-square .canvas-title h3 { font-size:24px; }
.share-canvas.is-square .canvas-title h3,.share-canvas.is-square .canvas-title p { -webkit-line-clamp:2; }
.share-canvas.is-square .canvas-content { top:110px; right:32px; bottom:56px; left:32px; grid-template-columns:1fr; grid-template-rows:auto auto auto; align-content:start; gap:8px; padding:8px 0; }
.share-canvas.is-square .canvas-weapon { grid-template-columns:88px minmax(0,1fr); gap:8px; padding:4px 8px; }
.share-canvas.is-square .canvas-weapon > img,.share-canvas.is-square .canvas-weapon-icon-fallback { width:88px; height:50px; padding:4px 6px; }
.share-canvas.is-square .canvas-weapon p { grid-column:1 / -1; }
.share-canvas.is-square .canvas-weapon p { -webkit-line-clamp:1; }
.share-canvas.is-square .canvas-sigils > div { grid-template-columns:repeat(3,minmax(0,1fr)); gap:3px 5px; margin-top:4px; }
.share-canvas.is-square .canvas-sigils article { grid-template-columns:20px minmax(0,1fr); }
.share-canvas.is-square .canvas-sigils article > img,.share-canvas.is-square .canvas-icon-fallback { width:20px; height:20px; }
.share-canvas.is-square .canvas-sigils article > b { display:none; }
.share-canvas.is-square .canvas-summary { grid-template-columns:1fr 2fr; }
.share-canvas.is-square .canvas-levels { grid-template-columns:repeat(3,minmax(0,1fr)); gap:3px; }
.share-canvas.is-square .canvas-levels span { gap:3px; padding:3px 4px; }
.share-canvas.is-square.density-full .canvas-content { grid-template-columns:1fr; grid-template-rows:64px 160px 120px 104px; align-content:start; gap:5px; padding:5px 0; }
.share-canvas.is-square.density-full .canvas-weapon { grid-column:1; grid-row:1; }
.share-canvas.is-square.density-full .canvas-sigils { min-height:0; display:grid; grid-column:1; grid-row:2; grid-template-rows:auto minmax(0,1fr); overflow:hidden; }
.share-canvas.is-square.density-full .canvas-sigils > div { min-height:0; grid-template-columns:repeat(3,minmax(0,1fr)); grid-template-rows:repeat(4,minmax(0,1fr)); }
.share-canvas.is-square.density-full .canvas-sigils article { min-height:0; padding:1px 4px; }
.share-canvas.is-square.density-full .canvas-summary { grid-column:1; grid-row:3; grid-template-columns:1fr 2fr; min-height:0; overflow:hidden; }
.share-canvas.is-square.density-full .canvas-summary > div { gap:2px; padding:5px; }
.share-canvas.is-square.density-full .canvas-ledger-scope { margin-bottom:2px; }
.share-canvas.is-square.density-full .canvas-levels { grid-template-columns:repeat(3,minmax(0,1fr)); }
.share-canvas.is-square.density-full .canvas-levels span { padding:2px 3px; }
.share-canvas.is-square.density-full.has-qr .canvas-summary > div { padding:3px; }
.share-canvas.is-square.density-full.has-qr .canvas-ledger-scope { margin-bottom:0; }
.share-canvas.is-square.density-full.has-qr .canvas-levels span { padding:1px 3px; }
.share-canvas.is-square.density-full .canvas-full-detail { grid-column:1; grid-row:4; grid-template-columns:1.35fr 1fr 1fr; gap:6px; }
.share-canvas.is-square.has-qr .canvas-content { bottom:108px; }
.share-canvas.is-square.density-full.has-qr .canvas-content { grid-template-rows:58px 144px 106px 76px; }
.share-canvas.is-square.density-full .canvas-full-detail > div { padding:5px; }
.share-canvas.is-square.density-full.has-qr .canvas-full-detail > div { padding:3px; }
.share-canvas.is-square.density-full.has-qr .canvas-wrightstone > div { gap:0; margin-top:0; }
.share-canvas.is-square.density-full .canvas-summons > div { grid-template-columns:repeat(2,minmax(0,1fr)); }
.density-compact .canvas-levels span:nth-child(n+7) { display:none; }
@container share (max-width:1120px) { .share-layout { grid-template-columns:minmax(0,1fr); } .share-controls { grid-template-columns:repeat(2,minmax(0,1fr)); } .share-actions { align-self:end; } .share-preview-shell { justify-content:start; } }
@container share (max-width:620px) { .share-heading { align-items:stretch; flex-direction:column; gap:var(--space-2); } .share-heading > span { align-self:start; } .share-controls { grid-template-columns:minmax(0,1fr); padding:var(--space-3); } .share-preview-shell { padding:0; transform-origin:top left; } }
@container share (max-width:420px) { .share-actions { grid-template-columns:minmax(0,1fr); } }
</style>
