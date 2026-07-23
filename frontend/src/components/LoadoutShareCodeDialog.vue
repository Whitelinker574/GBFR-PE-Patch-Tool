<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import {
  compactLoadoutShareCode,
  loadoutShareCodeCharacters,
  normalizeLoadoutShareText,
} from '../loadoutShareCode'

const props = defineProps({
  open: { type: Boolean, default: false },
  result: { type: Object, default: null },
  published: { type: Object, default: null },
  busy: { type: Boolean, default: false },
  publishing: { type: Boolean, default: false },
  error: { type: String, default: '' },
  canGenerate: { type: Boolean, default: false },
  selectedName: { type: String, default: '' },
})
const emit = defineEmits(['close', 'generate', 'publish', 'import'])

const pasteCode = ref('')
const copied = ref('')
const localError = ref('')
const pasteBox = ref(null)
const offlineMode = ref('compact')

const compactCode = computed(() => {
  if (!props.result?.compatibilityCode) return ''
  try {
    return compactLoadoutShareCode(props.result.compatibilityCode)
  } catch {
    return ''
  }
})
const offlineCode = computed(() => offlineMode.value === 'compact'
  ? compactCode.value
  : (props.result?.compatibilityCode || ''))
const offlineCount = computed(() => loadoutShareCodeCharacters(offlineCode.value))
const importReady = computed(() => normalizeLoadoutShareText(pasteCode.value).length > 8)
const working = computed(() => props.busy || props.publishing)

watch(() => props.open, value => {
  if (!value) return
  localError.value = ''
  copied.value = ''
  if (!props.result && props.canGenerate) emit('generate')
})

async function copyText(value, key) {
  if (!value) return
  localError.value = ''
  try {
    const legacyCopy = () => {
      const field = document.createElement('textarea')
      field.value = value
      field.style.position = 'fixed'
      field.style.left = '-9999px'
      field.style.opacity = '0'
      document.body.appendChild(field)
      field.focus()
      field.select()
      field.setSelectionRange(0, field.value.length)
      try {
        if (!document.execCommand('copy')) throw new Error('系统拒绝复制')
      } finally {
        field.remove()
      }
    }
    if (navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(value)
      } catch {
        legacyCopy()
      }
    } else {
      legacyCopy()
    }
    copied.value = key
    window.setTimeout(() => {
      if (copied.value === key) copied.value = ''
    }, 1800)
  } catch (error) {
    localError.value = `复制失败：${String(error?.message || error)}`
  }
}

async function readClipboard() {
  localError.value = ''
  try {
    if (!navigator.clipboard?.readText) throw new Error('当前系统不允许直接读取剪贴板')
    pasteCode.value = await navigator.clipboard.readText()
    await nextTick()
    pasteBox.value?.focus()
  } catch (error) {
    localError.value = `${String(error?.message || error)}，请手动粘贴到输入框。`
  }
}

function submitImport() {
  if (!importReady.value || working.value) return
  localError.value = ''
  emit('import', pasteCode.value)
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="share-backdrop" role="presentation" @click.self="emit('close')">
      <section class="share-dialog" role="dialog" aria-modal="true" aria-labelledby="share-title">
        <header class="share-header">
          <div class="share-mark" aria-hidden="true">交</div>
          <div>
            <small>单套配装 · 在线短码</small>
            <h2 id="share-title">分享与接收配装</h2>
            <p>只分享当前配装。接收后仍会进入分项导入确认，不会直接写入存档。</p>
          </div>
          <button class="icon-close" type="button" aria-label="关闭" title="关闭" @click="emit('close')">×</button>
        </header>

        <div class="share-body">
          <section class="online-section">
            <div class="section-heading">
              <span><i>01</i><b>生成短链接</b><small>{{ result ? `${result.characterName} · ${result.loadoutName}` : selectedName || '请先选择一套配装' }}</small></span>
              <button v-if="result" type="button" class="ui-btn is-ghost" :disabled="working || !canGenerate" @click="emit('generate')">重新读取</button>
            </div>

            <div v-if="published" class="published-ticket">
              <div class="published-code">
                <small>聊天群直接发送</small>
                <strong>{{ published.code }}</strong>
                <span>{{ published.reused ? '相同配装已存在，沿用原短码' : '短码已生成' }} · {{ published.bytes.toLocaleString() }} B</span>
              </div>
              <div class="published-actions">
                <button type="button" class="ui-btn is-primary" @click="copyText(published.code, 'short')">{{ copied === 'short' ? '已复制' : '复制短码' }}</button>
                <button type="button" class="ui-btn is-ghost" @click="copyText(published.url, 'link')">{{ copied === 'link' ? '已复制' : '复制链接' }}</button>
              </div>
              <div class="published-url" title="在线分享链接">{{ published.url }}</div>
            </div>

            <div v-else class="publish-prompt">
              <span class="prompt-glyph" aria-hidden="true">⌁</span>
              <div>
                <b>{{ result ? '配装已在本机打包完成' : (canGenerate ? '正在读取当前配装…' : '当前配装不可分享') }}</b>
                <small>{{ result ? '发布后得到 16 位短码和可点击链接，适合 QQ、微信与论坛。' : '请选择一个非队伍槽的单套配装。' }}</small>
              </div>
              <button type="button" class="ui-btn is-primary publish-button" :disabled="working || !result" @click="emit('publish')">
                {{ publishing ? '正在生成…' : '生成短链接' }}
              </button>
            </div>
            <p class="online-note">线上仅保存压缩后的单套配装，不包含整个存档。获得短码的人可读取该配装；当前短码不自动过期。</p>
          </section>

          <section class="receive-section">
            <div class="section-heading">
              <span><i>02</i><b>接收别人的配装</b><small>短码、完整链接和离线长码都能识别</small></span>
              <button type="button" class="ui-btn is-ghost" :disabled="working" @click="readClipboard">读取剪贴板</button>
            </div>
            <textarea ref="pasteBox" v-model="pasteCode" spellcheck="false" placeholder="输入 0123-4567-89AB-CDEF，或粘贴分享链接 / GBFRC1 离线长码"></textarea>
            <div class="receive-actions">
              <span v-if="pasteCode">已输入 {{ loadoutShareCodeCharacters(pasteCode) }} 个字符</span>
              <span v-else>解析后可选择因子、技能、专精、武器、祝福、召唤石与上限突破等范围。</span>
              <button type="button" class="ui-btn is-primary" :disabled="working || !importReady" @click="submitImport">{{ busy ? '解析中…' : '解析并选择导入范围' }}</button>
            </div>
          </section>

          <details class="offline-section">
            <summary>
              <span><b>离线长码</b><small>服务不可用时的备用方式，无需联网</small></span>
              <i aria-hidden="true"></i>
            </summary>
            <div class="offline-body">
              <div class="offline-tabs" role="tablist" aria-label="离线分享码格式">
                <button type="button" :class="{ on: offlineMode === 'compact' }" role="tab" :aria-selected="offlineMode === 'compact'" @click="offlineMode = 'compact'">较短 Unicode 码</button>
                <button type="button" :class="{ on: offlineMode === 'compatible' }" role="tab" :aria-selected="offlineMode === 'compatible'" @click="offlineMode = 'compatible'">纯 ASCII 兼容码</button>
              </div>
              <textarea :value="offlineCode" readonly spellcheck="false" aria-label="离线配装长码" @focus="$event.target.select()"></textarea>
              <footer>
                <span>{{ offlineCount.toLocaleString() }} 字符 · GBLC1 完整性校验</span>
                <button type="button" class="ui-btn is-ghost" :disabled="!offlineCode" @click="copyText(offlineCode, 'offline')">{{ copied === 'offline' ? '已复制' : '复制离线码' }}</button>
              </footer>
            </div>
          </details>

          <p v-if="error || localError" class="share-error" role="alert">{{ error || localError }}</p>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.share-backdrop { position:fixed; z-index:10000; inset:0; display:grid; place-items:center; padding:20px; background:rgba(38,28,17,.58); backdrop-filter:blur(8px); }
.share-dialog { width:min(860px,96vw); max-height:min(880px,95vh); overflow:auto; border:1px solid rgba(126,88,42,.62); border-radius:12px; color:#3c3020; background:#fbf4e4; box-shadow:0 30px 86px rgba(31,21,11,.38); }
.share-header { position:relative; display:grid; grid-template-columns:auto minmax(0,1fr) auto; gap:15px; align-items:center; padding:22px 26px 19px; border-bottom:1px solid rgba(126,88,42,.22); background:linear-gradient(112deg,#fffaf0,#ecd8ad); }
.share-mark { width:48px; height:48px; display:grid; place-items:center; border:1px solid #8f642e; border-radius:50%; box-shadow:inset 0 0 0 5px rgba(143,100,46,.1); color:#765126; font-family:var(--font-display); font-size:1.18rem; font-weight:800; }
.share-header small { color:#8c673a; font-weight:800; }
.share-header h2 { margin:2px 0; font-family:var(--font-display); font-size:1.45rem; letter-spacing:0; }
.share-header p { margin:0; color:#746653; font-size:.83rem; line-height:1.5; }
.icon-close { width:34px; height:34px; border:1px solid rgba(126,88,42,.3); border-radius:50%; background:rgba(255,255,255,.55); color:#705533; font-size:1.3rem; cursor:pointer; }
.share-body { display:flex; flex-direction:column; gap:20px; padding:20px 26px 24px; }
.online-section,.receive-section { min-width:0; }
.section-heading { display:flex; align-items:center; justify-content:space-between; gap:14px; margin-bottom:11px; }
.section-heading > span { min-width:0; display:grid; grid-template-columns:30px auto; column-gap:9px; align-items:center; }
.section-heading i { grid-row:1/3; width:28px; height:28px; display:grid; place-items:center; border-radius:50%; background:#e8d5ae; color:#765126; font-size:.68rem; font-style:normal; font-weight:800; }
.section-heading b { color:#443424; font-size:.92rem; }
.section-heading small { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:#86745e; font-size:.72rem; }
.publish-prompt { min-height:104px; display:grid; grid-template-columns:auto minmax(0,1fr) auto; gap:15px; align-items:center; padding:17px 18px; border:1px dashed rgba(126,88,42,.42); border-radius:8px; background:rgba(255,253,247,.72); }
.prompt-glyph { width:44px; height:44px; display:grid; place-items:center; border-radius:50%; background:#e9ddc3; color:#765126; font-size:1.5rem; }
.publish-prompt div { display:flex; flex-direction:column; gap:4px; }
.publish-prompt b { color:#483725; }
.publish-prompt small { color:#7b6a55; line-height:1.45; }
.publish-button { min-width:118px; min-height:40px; }
.published-ticket { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:14px 18px; align-items:center; padding:18px; border:1px solid rgba(74,119,89,.45); border-left:4px solid #4a7759; border-radius:8px; background:#f7fbf4; }
.published-code { min-width:0; display:flex; flex-direction:column; gap:3px; }
.published-code small { color:#65816c; font-weight:700; }
.published-code strong { color:#31563e; font-family:Consolas,"Cascadia Mono",monospace; font-size:clamp(1.12rem,3vw,1.55rem); letter-spacing:0; overflow-wrap:anywhere; }
.published-code span { color:#6f7b6f; font-size:.73rem; }
.published-actions { display:flex; gap:8px; }
.published-url { grid-column:1/-1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; padding-top:10px; border-top:1px solid rgba(74,119,89,.2); color:#607361; font: .72rem Consolas,"Cascadia Mono",monospace; }
.online-note { margin:8px 0 0; color:#7b6b57; font-size:.72rem; line-height:1.5; }
.receive-section { padding-top:17px; border-top:1px solid rgba(126,88,42,.22); }
.receive-section > textarea { width:100%; min-height:82px; resize:vertical; padding:12px 13px; border:1px solid rgba(126,88,42,.34); border-radius:7px; outline:none; background:#fffdf7; color:#413426; font: .8rem Consolas,"Cascadia Mono",monospace; line-height:1.5; }
.receive-section > textarea:focus,.offline-body > textarea:focus { border-color:#8b6737; box-shadow:0 0 0 3px rgba(139,103,55,.1); }
.receive-actions { display:flex; align-items:center; justify-content:space-between; gap:14px; margin-top:9px; }
.receive-actions span { color:#7b6a55; font-size:.72rem; line-height:1.45; }
.receive-actions button { flex:none; }
.offline-section { border-top:1px solid rgba(126,88,42,.22); }
.offline-section > summary { display:flex; align-items:center; justify-content:space-between; gap:12px; padding-top:16px; cursor:pointer; list-style:none; }
.offline-section > summary::-webkit-details-marker { display:none; }
.offline-section > summary span { display:flex; flex-direction:column; gap:2px; }
.offline-section > summary b { color:#564431; font-size:.84rem; }
.offline-section > summary small { color:#8a7964; font-size:.7rem; }
.offline-section > summary i::before { content:"＋"; color:#7c5b32; font-style:normal; }
.offline-section[open] > summary i::before { content:"－"; }
.offline-body { margin-top:12px; padding:13px; border:1px solid rgba(126,88,42,.25); border-radius:7px; background:rgba(255,253,247,.62); }
.offline-tabs { display:flex; gap:6px; margin-bottom:9px; }
.offline-tabs button { min-height:31px; padding:0 10px; border:1px solid rgba(126,88,42,.28); border-radius:5px; background:#fffdf7; color:#68543d; cursor:pointer; }
.offline-tabs button.on { border-color:#765126; background:#806039; color:#fff9e9; }
.offline-body > textarea { width:100%; min-height:94px; resize:vertical; padding:10px; border:1px solid rgba(126,88,42,.3); border-radius:6px; outline:none; background:#fffdf7; color:#554733; font: .7rem Consolas,"Cascadia Mono",monospace; line-height:1.4; overflow-wrap:anywhere; }
.offline-body footer { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-top:8px; color:#82715c; font-size:.7rem; }
.share-error { margin:0; padding:9px 11px; border:1px solid rgba(171,75,54,.32); border-radius:6px; background:#fff0eb; color:#9a3f30; font-size:.78rem; line-height:1.5; }
@media(max-width:680px) {
  .share-backdrop { padding:8px; }
  .share-dialog { width:100%; max-height:97vh; }
  .share-header,.share-body { padding-left:16px; padding-right:16px; }
  .share-mark { display:none; }
  .share-header { grid-template-columns:minmax(0,1fr) auto; }
  .publish-prompt { grid-template-columns:auto minmax(0,1fr); }
  .publish-button { grid-column:1/-1; width:100%; }
  .published-ticket { grid-template-columns:1fr; }
  .published-actions { display:grid; grid-template-columns:1fr 1fr; }
  .published-url { grid-column:1; }
  .receive-actions { align-items:stretch; flex-direction:column; }
  .receive-actions button { width:100%; }
}
</style>
