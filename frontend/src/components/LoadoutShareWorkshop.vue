<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { characterSharePortraitProfile } from '../characterRoster'
import { characterAssetIcon, traitAssetIcon, weaponAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'

const props = defineProps({
  group: { type: Object, default: null },
  previewMap: { type: Object, default: () => new Map() },
  embedded: { type: Boolean, default: false },
  published: { type: Object, default: null },
  suggestedDescription: { type: Object, default: null },
})
const emit = defineEmits(['status'])
const template = ref('landscape')
const loadoutId = ref(0)
const title = ref('')
const description = ref('')
const author = ref('')
const shareUrl = ref('')
const portraitOpacity = ref(24)
const density = ref('standard')
const canvasRef = ref(null)
const exportBusy = ref(false)
const qrDataUrl = ref('')
let qrRequest = 0

const tx = (zh, en) => language.value === 'en' ? en : zh
const loadouts = computed(() => props.group?.loadouts || [])
const selected = computed(() => loadouts.value.find(item => item.unitId === loadoutId.value) || loadouts.value[0] || null)
const preview = computed(() => props.previewMap instanceof Map ? props.previewMap.get(selected.value?.unitId) : null)
const portraitProfile = computed(() => characterSharePortraitProfile(props.group?.charaHash))
const portrait = computed(() => portraitProfile.value?.path || '')
const characterIcon = computed(() => characterAssetIcon(props.group?.charaHash))
const weaponIcon = computed(() => weaponAssetIcon({ hash: selected.value?.weaponHash }))
const canvasStyle = computed(() => {
  const anchor = portraitProfile.value?.anchors?.[template.value]
  const focus = anchor?.focus || portraitProfile.value?.faceFocus || [0.5, 0.3]
  return {
    '--portrait-opacity': String(portraitOpacity.value / 100),
    '--portrait-fit': anchor?.fit || 'cover',
    '--portrait-focus': `${Number(focus[0]) * 100}% ${Number(focus[1]) * 100}%`,
  }
})
const exportConfig = computed(() => ({
  landscape: { width: 960, height: 540, pixelRatio: 2, output: '1920x1080' },
  portrait: { width: 720, height: 960, pixelRatio: 2, output: '1440x1920' },
  square: { width: 640, height: 640, pixelRatio: 2.5, output: '1600x1600' },
}[template.value]))
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
function sanitizeFileName(value) { return String(value || 'GBFR-loadout').replace(/[\\/:*?"<>|]+/g, '_').slice(0, 80) }
async function updateQR() {
  const request = ++qrRequest
  const value = shareUrl.value.trim()
  if (!/^https:\/\//i.test(value)) { qrDataUrl.value = ''; return }
  try {
    const QRCode = await import('qrcode')
    const result = await QRCode.toDataURL(value, { errorCorrectionLevel: 'M', margin: 4, width: 240, color: { dark: '#2c241b', light: '#fffdf7' } })
    if (request === qrRequest) qrDataUrl.value = result
  } catch (error) {
    if (request === qrRequest) qrDataUrl.value = ''
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
async function renderPNG() {
  if (!canvasRef.value || !selected.value) throw new Error(tx('没有可导出的配装', 'No loadout is available for export'))
  await nextTick()
  await waitForImages(canvasRef.value)
  const { toPng } = await import('html-to-image')
  return toPng(canvasRef.value, { pixelRatio: exportConfig.value.pixelRatio, cacheBust: false, backgroundColor: '#d8c39b', skipFonts: false })
}
async function download() {
  if (exportBusy.value) return
  exportBusy.value = true
  try {
    const url = await renderPNG()
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${sanitizeFileName(title.value || `${props.group?.charaName}-${selected.value?.name || tx('配装', 'Loadout')}`)}.png`
    anchor.click()
    emit('status', tx(`已生成 ${exportConfig.value.output} 分享图`, `${exportConfig.value.output} share image generated`), 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally { exportBusy.value = false }
}
async function copyPNG() {
  if (exportBusy.value) return
  exportBusy.value = true
  try {
    if (!navigator.clipboard?.write || typeof ClipboardItem === 'undefined') throw new Error(tx('当前 WebView 不支持图片剪贴板，请使用下载 PNG。', 'This WebView does not support image clipboard; download the PNG instead.'))
    const dataUrl = await renderPNG()
    const blob = await (await fetch(dataUrl)).blob()
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
    emit('status', tx('分享图已复制到剪贴板', 'Share image copied to clipboard'), 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally { exportBusy.value = false }
}

watch(() => props.group?.charaHash, () => {
  loadoutId.value = loadouts.value[0]?.unitId || 0
  title.value = props.group?.charaName ? tx(`${props.group.charaName} 配装记录`, `${props.group.charaName} Loadout`) : ''
}, { immediate: true })
watch(() => props.published?.url, (value, previous) => {
  const current = shareUrl.value.trim()
  if (value && (!current || current === String(previous || '').trim())) shareUrl.value = String(value)
}, { immediate: true })
watch(() => props.suggestedDescription?.requestId, () => {
  const value = String(props.suggestedDescription?.description || '').trim()
  if (value) description.value = value
}, { immediate: true })
watch(shareUrl, updateQR)
</script>

<template>
  <section class="share-workshop" :class="{ 'is-embedded': embedded }" :aria-label="tx('分享图工坊', 'Share Image Workshop')">
    <header class="share-heading"><div><small>{{ tx('固定画布导出', 'Fixed-Canvas Export') }}</small><h2>{{ tx('分享图工坊', 'Share Image Workshop') }}</h2><p>{{ tx('预览和 PNG 使用同一套字段；导出前会等待角色、武器和因子图片完成解码，不生成空白图。', 'Preview and PNG use the same fields. Export waits for character, weapon, and sigil images to decode, preventing blank artwork.') }}</p></div><span>{{ exportConfig.output }}</span></header>
    <div class="share-layout">
      <aside class="share-controls ui-card is-flat">
        <label><span>{{ tx('配装', 'Loadout') }}</span><select v-model="loadoutId" class="ui-select"><option v-for="item in loadouts" :key="item.unitId" :value="item.unitId">{{ item.isParty ? tx('当前实时配装', 'Current Live Loadout') : item.name || `${tx('槽位', 'Slot')} ${item.slot}` }}</option></select></label>
        <div><span>{{ tx('画布', 'Canvas') }}</span><div class="ui-seg"><button type="button" class="ui-seg-btn" :class="{ 'is-on': template === 'landscape' }" @click="template = 'landscape'">16:9</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': template === 'portrait' }" @click="template = 'portrait'">3:4</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': template === 'square' }" @click="template = 'square'">1:1</button></div></div>
        <label><span>{{ tx('标题', 'Title') }}</span><input v-model="title" class="ui-input" maxlength="48" /></label>
        <label><span>{{ tx('简介', 'Description') }}</span><textarea v-model="description" class="ui-textarea" maxlength="140" :placeholder="tx('这套配装适合什么场景', 'What this loadout is built for')"></textarea></label>
        <label><span>{{ tx('署名（可空）', 'Author (optional)') }}</span><input v-model="author" class="ui-input" maxlength="24" /></label>
        <label><span>{{ tx('HTTPS 分享链接（自动复用）', 'HTTPS Share Link (reused automatically)') }}</span><input v-model="shareUrl" class="ui-input" placeholder="https://…" /><small v-if="published?.url" class="share-link-source">{{ tx('已使用这套配装刚生成的短链；仍可手动覆盖。', 'Using the short link just published for this loadout; you can still override it.') }}</small><small v-else class="share-link-source">{{ tx('先生成线上短码后会自动带入；也可手动填写。', 'Publishing a short code fills this automatically; manual links remain available.') }}</small></label>
        <label class="opacity-control"><span>{{ tx('角色底图强度', 'Character Background Strength') }} · {{ portraitOpacity }}%</span><input v-model.number="portraitOpacity" type="range" min="10" max="42" step="1" /></label>
        <div><span>{{ tx('信息密度', 'Information Density') }}</span><div class="ui-seg"><button type="button" class="ui-seg-btn" :class="{ 'is-on': density === 'compact' }" @click="density = 'compact'">{{ tx('群聊摘要', 'Chat Summary') }}</button><button type="button" class="ui-seg-btn" :class="{ 'is-on': density === 'standard' }" @click="density = 'standard'">{{ tx('标准', 'Standard') }}</button></div></div>
        <div class="share-actions"><button type="button" class="ui-btn" :disabled="exportBusy || !selected" @click="copyPNG">{{ tx('复制 PNG', 'Copy PNG') }}</button><button type="button" class="ui-btn is-primary" :disabled="exportBusy || !selected" @click="download">{{ exportBusy ? tx('正在生成…', 'Rendering…') : tx('下载 PNG', 'Download PNG') }}</button></div>
      </aside>

      <div class="share-preview-shell" :class="`is-${template}`">
        <div ref="canvasRef" class="share-canvas" :class="[`is-${template}`, `density-${density}`]" :style="canvasStyle">
          <img v-if="portrait" class="share-portrait" :src="portrait" alt="" decoding="async" />
          <div class="share-wash"></div>
          <header class="canvas-title"><div><small>GRANBLUE FANTASY: RELINK · LOADOUT LOG</small><h3>{{ title || `${group?.charaName || ''} ${tx('配装记录', 'Loadout')}` }}</h3><p v-if="description">{{ description }}</p></div><div class="canvas-character"><img v-if="characterIcon" :src="characterIcon" alt="" /><span><b>{{ group?.charaName }}</b><small>{{ selected?.isParty ? tx('当前实时配装', 'Current Live Loadout') : selected?.name }}</small></span></div></header>
          <main class="canvas-content">
            <section class="canvas-weapon"><img v-if="weaponIcon" :src="weaponIcon" alt="" /><div><small>{{ tx('武器', 'WEAPON') }}</small><strong>{{ selected?.weaponName || tx('未记录武器', 'Weapon Not Captured') }}</strong><span v-if="selected?.weapon">Lv{{ selected.weapon.level }} · {{ tx('觉醒', 'Awakening') }} {{ selected.weapon.awakening }} · {{ tx('超凡', 'Transcendence') }} {{ selected.weapon.transcendence }}</span></div><p>{{ (selected?.weapon?.skills || []).map(item => `${item.name} Lv${item.level}`).join(' · ') || tx('武器技能未记录', 'Weapon skills not captured') }}</p></section>
            <section class="canvas-sigils"><small>{{ tx('因子配置', 'SIGIL LOADOUT') }} · {{ selected?.sigils?.length || 0 }}/12</small><div><article v-for="(sigil, index) in selected?.sigils || []" :key="`${sigil.slotId}-${index}`"><img v-if="traitIcon(sigil)" :src="traitIcon(sigil)" alt="" /><b>{{ String(index + 1).padStart(2, '0') }}</b><span><strong>{{ sigil.name || tx('因子', 'Sigil') }}</strong><small>{{ sigil.primaryTraitName }} Lv{{ sigil.primaryTraitLevel }}<template v-if="sigil.secondaryTraitName"> · {{ sigil.secondaryTraitName }} Lv{{ sigil.secondaryTraitLevel }}</template></small></span></article></div></section>
            <section class="canvas-summary"><div><small>{{ tx('专精方向', 'MASTERY') }}</small><strong>{{ masteryDirection }}</strong></div><div><small class="canvas-ledger-scope">{{ traitTotalsFallback ? tx('原始等级 · 未含召唤石', 'Raw levels · summons excluded') : tx('有效合并等级', 'Effective combined levels') }}</small><div class="canvas-levels"><span v-for="trait in traitTotals" :key="trait.name"><b>{{ trait.name }}</b><em>Lv{{ trait.level }}<small v-if="trait.rawLevel > trait.level">/{{ trait.rawLevel }}</small></em></span></div></div></section>
          </main>
          <footer class="canvas-footer"><div><span v-if="author">{{ tx('整理', 'By') }} · {{ author }}</span><span>GBFR PE Patch Tool</span><span>GBFR 2.0.2</span></div><div v-if="qrDataUrl" class="canvas-qr"><span>{{ shareUrl }}</span><img :src="qrDataUrl" alt="" /></div><div v-else class="canvas-stats"><span>HP <b>{{ preview?.finalStats?.hp?.toLocaleString?.() || '—' }}</b></span><span>ATK <b>{{ preview?.finalStats?.attack?.toLocaleString?.() || '—' }}</b></span></div></footer>
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
.share-controls > label,.share-controls > div:not(.share-actions) { min-width:0; display:grid; gap:5px; }
.share-controls span { color:var(--text-muted); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.share-controls textarea { min-height:72px; resize:vertical; }
.share-link-source { color:var(--text-muted); font-size:var(--fs-2xs); font-weight:var(--fw-normal); line-height:var(--lh-normal); }
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
.share-preview-shell { min-width:0; display:grid; place-items:center; overflow:auto; padding:var(--space-3); border:1px solid var(--border-soft); background:rgba(62,46,28,.08); }
.share-canvas { position:relative; isolation:isolate; flex:none; overflow:hidden; border:1px solid #745a35; background:#d8c39b url('../assets/gbfr/parchment-ui-v2.webp') center/cover; color:#2d241b; box-shadow:0 16px 36px rgba(52,38,22,.22); font-family:var(--font-body); }
.share-canvas.is-landscape { width:960px; height:540px; }
.share-canvas.is-portrait { width:720px; height:960px; }
.share-canvas.is-square { width:640px; height:640px; }
.share-portrait { position:absolute; z-index:-3; inset:0; width:100%; height:100%; object-fit:var(--portrait-fit,cover); object-position:var(--portrait-focus,50% 30%); opacity:var(--portrait-opacity); filter:saturate(.82) contrast(.94); }
.share-wash { position:absolute; z-index:-2; inset:0; background:linear-gradient(90deg,rgba(232,214,177,.96) 0%,rgba(232,214,177,.84) 46%,rgba(232,214,177,.34) 78%,rgba(232,214,177,.64) 100%); }
.share-wash::after { content:""; position:absolute; inset:18px; border:1px solid rgba(88,62,31,.38); box-shadow:inset 0 0 0 4px rgba(245,234,208,.33); pointer-events:none; }
.canvas-title { min-width:0; display:flex; justify-content:space-between; align-items:start; gap:24px; padding:34px 42px 16px; border-bottom:1px solid rgba(88,62,31,.34); }
.canvas-title > div:first-child { min-width:0; }
.canvas-title small,.canvas-content > section > small,.canvas-weapon small,.canvas-summary small { color:#73562f; font-size:10px; font-weight:800; }
.canvas-title h3 { max-width:720px; margin:3px 0; overflow-wrap:anywhere; color:#251d16; font-family:var(--font-display); font-size:30px; line-height:1.08; letter-spacing:0; }
.canvas-title p { max-width:640px; margin:4px 0 0; color:#5d4a35; font-size:12px; line-height:1.35; }
.canvas-character { flex:0 0 auto; display:flex; align-items:center; gap:9px; }
.canvas-character > img { width:54px; height:54px; border:2px solid #987544; border-radius:7px; background:#efe3c9; object-fit:cover; }
.canvas-character > span { display:grid; text-align:right; }
.canvas-character b { color:#2d241b; font-size:15px; }
.canvas-character small { max-width:150px; overflow:hidden; text-overflow:ellipsis; color:#725b3e; white-space:nowrap; }
.canvas-content { min-width:0; display:grid; grid-template-columns:minmax(0,1.4fr) minmax(250px,.6fr); grid-template-rows:auto 1fr; gap:12px 18px; padding:16px 42px; }
.canvas-weapon { min-width:0; grid-column:1 / -1; display:grid; grid-template-columns:92px minmax(0,1fr) minmax(200px,.7fr); align-items:center; gap:12px; padding-bottom:10px; border-bottom:1px dashed rgba(88,62,31,.32); }
.canvas-weapon > img { width:92px; height:58px; object-fit:contain; }
.canvas-weapon > div { min-width:0; display:grid; }
.canvas-weapon strong { overflow-wrap:anywhere; color:#2c2218; font-size:16px; }
.canvas-weapon span,.canvas-weapon p { margin:0; color:#69533a; font-size:10px; line-height:1.35; }
.canvas-sigils { min-width:0; }
.canvas-sigils > div { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:4px 8px; margin-top:6px; }
.canvas-sigils article { min-width:0; display:grid; grid-template-columns:25px 18px minmax(0,1fr); gap:5px; align-items:center; min-height:34px; padding:3px 6px; border-left:2px solid rgba(119,83,39,.55); background:rgba(248,239,216,.5); }
.canvas-sigils article > img { width:25px; height:25px; border-radius:4px; object-fit:cover; }
.canvas-sigils article > b { color:#8a693b; font-family:var(--font-data); font-size:9px; }
.canvas-sigils article > span { min-width:0; display:grid; }
.canvas-sigils article strong { min-width:0; overflow:hidden; text-overflow:ellipsis; color:#30251a; font-size:10px; white-space:nowrap; }
.canvas-sigils article small { min-width:0; overflow:hidden; text-overflow:ellipsis; color:#70593d; font-size:8px; white-space:nowrap; }
.canvas-summary { min-width:0; display:grid; align-content:start; gap:8px; }
.canvas-summary > div:first-child { min-width:0; display:grid; padding:8px; border:1px solid rgba(98,72,39,.27); background:rgba(244,232,203,.58); }
.canvas-summary strong { overflow-wrap:anywhere; color:#30251a; font-size:11px; }
.canvas-ledger-scope { display:block; margin-bottom:4px; }
.canvas-levels { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:4px; }
.canvas-levels span { min-width:0; display:flex; justify-content:space-between; gap:5px; padding:4px 6px; background:rgba(244,232,203,.53); }
.canvas-levels b { min-width:0; overflow:hidden; text-overflow:ellipsis; color:#4d3d2b; font-size:9px; white-space:nowrap; }
.canvas-levels em { flex:0 0 auto; color:#805e32; font-size:9px; font-style:normal; font-weight:700; }
.canvas-footer { position:absolute; right:42px; bottom:24px; left:42px; display:flex; align-items:end; justify-content:space-between; gap:16px; padding-top:8px; border-top:1px solid rgba(88,62,31,.34); }
.canvas-footer > div:first-child { display:flex; flex-wrap:wrap; gap:10px; color:#705a3e; font-size:8px; }
.canvas-qr { display:flex; align-items:end; gap:7px; }
.canvas-qr span { max-width:190px; overflow:hidden; text-overflow:ellipsis; color:#705a3e; font-size:7px; white-space:nowrap; }
.canvas-qr img { width:54px; height:54px; border:4px solid #fffdf7; }
.canvas-stats { display:flex; gap:12px; color:#705a3e; font-size:9px; }
.canvas-stats b { color:#30251a; }
.share-canvas.is-portrait .canvas-title,.share-canvas.is-square .canvas-title { padding-right:32px; padding-left:32px; }
.share-canvas.is-portrait .canvas-content { grid-template-columns:1fr; padding:18px 32px; }
.share-canvas.is-portrait .canvas-weapon { grid-template-columns:88px minmax(0,1fr); }
.share-canvas.is-portrait .canvas-weapon p { grid-column:1 / -1; }
.share-canvas.is-portrait .canvas-summary { grid-template-columns:1fr 1.4fr; }
.share-canvas.is-portrait .canvas-footer,.share-canvas.is-square .canvas-footer { right:32px; left:32px; }
.share-canvas.is-square .canvas-title { padding-top:28px; }
.share-canvas.is-square .canvas-title h3 { font-size:24px; }
.share-canvas.is-square .canvas-content { grid-template-columns:1fr; gap:8px; padding:12px 32px; }
.share-canvas.is-square .canvas-weapon { grid-template-columns:68px minmax(0,1fr); }
.share-canvas.is-square .canvas-weapon > img { width:68px; height:44px; }
.share-canvas.is-square .canvas-weapon p { grid-column:1 / -1; }
.share-canvas.is-square .canvas-sigils > div { grid-template-columns:repeat(3,minmax(0,1fr)); }
.share-canvas.is-square .canvas-sigils article { grid-template-columns:20px minmax(0,1fr); }
.share-canvas.is-square .canvas-sigils article > img { width:20px; height:20px; }
.share-canvas.is-square .canvas-sigils article > b { display:none; }
.share-canvas.is-square .canvas-summary { grid-template-columns:1fr 1.6fr; }
.density-compact .canvas-sigils article:nth-child(n+7),.density-compact .canvas-levels span:nth-child(n+7) { display:none; }
@container share (max-width:1120px) { .share-layout { grid-template-columns:minmax(0,1fr); } .share-controls { grid-template-columns:repeat(2,minmax(0,1fr)); } .share-actions { align-self:end; } .share-preview-shell { justify-content:start; } }
@container share (max-width:620px) { .share-controls { grid-template-columns:minmax(0,1fr); } .share-preview-shell { padding:0; transform-origin:top left; } }
</style>
