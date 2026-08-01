<script setup>
import { computed, onActivated, reactive, ref } from 'vue'
import {
  DeployAudioMixerMod,
  GetAudioMixerWorkspace,
  RemoveAudioMixerMod,
} from '../../wailsjs/go/backend/App'
import { language } from '../i18n'
import { runtimeCompanionMessage } from '../runtimeCompanionMessages.js'
import ConfirmDialog from './ConfirmDialog.vue'

const emit = defineEmits(['status', 'runtime-state'])
const workspace = ref(null)
const search = ref('')
const busy = ref(false)
const message = ref('')
const tone = ref('info')
const diagnostic = ref(false)
const uiVolume = ref(100)
const volumes = reactive({})
const confirmDialog = ref(null)
const presetName = ref('')
const presets = ref([])
const presetStorageKey = 'gbfr-codex-audio-presets-v1'
const tx = (zh, en) => language.value === 'en' ? en : zh
const runtimeText = value => runtimeCompanionMessage(value, language.value)
const displayName = item => language.value === 'en' ? item.nameEn : item.nameZh

const filteredCharacters = computed(() => {
  const needle = search.value.trim().toLocaleLowerCase(language.value === 'en' ? 'en' : 'zh-CN')
  if (!needle) return workspace.value?.characters || []
  return (workspace.value?.characters || []).filter(item => `${item.nameZh} ${item.nameEn} ${item.id}`.toLocaleLowerCase().includes(needle))
})
const changedCount = computed(() => Object.values(volumes).filter(value => value !== 100).length + (uiVolume.value !== 100 ? 1 : 0))
const canSave = computed(() => Boolean(!busy.value && !workspace.value?.recoveryRequired && workspace.value?.gameRunning))
const runtimeActive = computed(() => workspace.value?.installed === true && workspace.value?.owned === true && workspace.value?.state === 'active')

function report(value, nextTone = 'info') {
  message.value = value
  tone.value = nextTone
  emit('status', value, nextTone === 'danger' ? 'error' : nextTone === 'ok' ? 'success' : 'info')
}

function syncWorkspace(value) {
  workspace.value = value
  emit('runtime-state', {
    id: 'audioMixer',
    active: value?.state === 'active' && value?.owned === true,
    recoveryRequired: value?.recoveryRequired === true,
  })
  diagnostic.value = value?.diagnostic === true
  uiVolume.value = Number(value?.uiVolume ?? 100)
  for (const character of value?.characters || []) volumes[character.id] = Number(value?.volumes?.[character.id] ?? 100)
}

async function refresh() {
  busy.value = true
  try { syncWorkspace(await GetAudioMixerWorkspace('')) }
  catch (error) { report(runtimeText(error), 'danger') }
  finally { busy.value = false }
}

function resetAll() {
  for (const key of Object.keys(volumes)) volumes[key] = 100
  uiVolume.value = 100
}

function loadPresets() {
  try {
    const value = JSON.parse(localStorage.getItem(presetStorageKey) || '[]')
    presets.value = Array.isArray(value) ? value.filter(item => item?.id && item?.name && item?.volumes && typeof item.volumes === 'object').slice(0, 24) : []
  } catch { presets.value = [] }
}

function persistPresets() {
  localStorage.setItem(presetStorageKey, JSON.stringify(presets.value.slice(0, 24)))
}

async function savePreset() {
  const name = presetName.value.trim()
  if (!name) {
    report(tx('请先填写预设名称。', 'Enter a preset name first.'), 'danger')
    return
  }
  const existing = presets.value.find(item => item.name.toLocaleLowerCase() === name.toLocaleLowerCase())
  if (existing) {
    const accepted = await confirmDialog.value?.ask({
      title: tx('覆盖同名音量预设', 'Replace named volume preset'),
      message: tx(`“${existing.name}”已经存在。`, `“${existing.name}” already exists.`),
      detail: tx('只会替换保存在本机的预设，不会立刻改变游戏音量；点底部“保存并热更新”后才会应用。', 'Only the preset stored on this PC is replaced. Game audio changes only after you press Save and Hot-Update below.'),
      confirmLabel: tx('覆盖预设', 'Replace preset'), cancelLabel: tx('取消', 'Cancel'), tone: 'warning',
    })
    if (!accepted) return
    existing.volumes = { ...volumes }
    existing.uiVolume = uiVolume.value
    existing.diagnostic = diagnostic.value
    existing.updatedAt = new Date().toISOString()
  } else {
    presets.value.unshift({ id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, name, volumes: { ...volumes }, uiVolume: uiVolume.value, diagnostic: diagnostic.value, updatedAt: new Date().toISOString() })
  }
  persistPresets()
  presetName.value = ''
  report(tx(`音量组合“${name}”已保存在本机；当前游戏音量尚未改变。`, `Mix “${name}” was saved on this PC; current game audio has not changed.`), 'ok')
}

function applyPreset(preset) {
  resetAll()
  for (const [id, value] of Object.entries(preset?.volumes || {})) {
    if (Object.hasOwn(volumes, id)) volumes[id] = Math.max(0, Math.min(100, Number(value) || 0))
  }
  uiVolume.value = Math.max(0, Math.min(100, Number(preset?.uiVolume ?? 100)))
  diagnostic.value = preset?.diagnostic === true
  report(tx(`已载入“${preset.name}”；确认后点击底部保存音量。`, `Loaded “${preset.name}”. Review it, then press Save volumes.`), 'info')
}

function deletePreset(id) {
  presets.value = presets.value.filter(item => item.id !== id)
  persistPresets()
}

async function save() {
  if (!canSave.value) return
  const firstInstall = !workspace.value?.owned
  if (firstInstall) {
    const accepted = await confirmDialog.value?.ask({
      title: tx('部署角色语音 Hook', 'Deploy character voice hook'),
      message: tx('应用会把自有音频运行时直接注入当前游戏，不需要外部加载器。', 'The app injects its own audio runtime directly into the current game; no external loader is needed.'),
      detail: tx('只调整能够明确归属角色的后续语音事件；未知和共享事件保持原样。', 'Only subsequent voice events with an unambiguous character owner are adjusted; unknown and shared events stay untouched.'),
      confirmLabel: tx('确认开启', 'Enable'), cancelLabel: tx('取消', 'Cancel'), tone: 'warning',
    })
    if (!accepted) return
  }
  busy.value = true
  try {
    await DeployAudioMixerMod({ diagnostic: diagnostic.value, volumes: { ...volumes }, uiVolume: uiVolume.value })
    report(firstInstall ? tx('内置音频运行时已开启，后续角色语音会使用当前音量。', 'The built-in audio runtime is active; subsequent character voices use these volumes.') : tx('音量配置已保存，后续语音事件会使用新设置。', 'Volume settings saved; subsequent voice events use the new values.'), 'ok')
    await refresh()
  } catch (error) { report(runtimeText(error), 'danger') }
  finally { busy.value = false }
}

async function remove() {
  if (!workspace.value?.owned || busy.value) return
  const accepted = await confirmDialog.value?.ask({
    title: tx('停用音频运行时', 'Disable audio runtime'), message: tx('会先恢复 Wwise Hook，再停用内置运行时；音量预设仍会保留。', 'The Wwise hook is restored before the built-in runtime stops. Volume presets are kept.'),
    detail: tx('正在播放的回调会先安全结束，不需要退出游戏。', 'Active callbacks are allowed to finish safely; the game does not need to exit.'),
    confirmLabel: tx('确认停用', 'Disable'), cancelLabel: tx('取消', 'Cancel'), tone: 'danger',
  })
  if (!accepted) return
  busy.value = true
  try { await RemoveAudioMixerMod(''); report(tx('音频运行时已停用，Hook 已恢复。', 'Audio runtime disabled and the hook was restored.'), 'ok'); await refresh() }
  catch (error) { report(runtimeText(error), 'danger') }
  finally { busy.value = false }
}

loadPresets()
onActivated(() => { void refresh() })
</script>

<template>
  <div class="audio-lab ui-page-stack">
    <section class="audio-intro">
      <div><p class="audio-kicker">WWISE · BUILT-IN RUNTIME</p><h2>{{ tx('分别调整角色语音音量', 'Adjust Character Voice Volumes') }}</h2><p>{{ tx('想让某个角色更安静或暂时静音，就在这里单独调整；不会替换音频文件，游戏设置里的“语音音量”仍是总开关。', 'Lower or mute individual characters here without replacing audio files. The in-game Voice Volume setting remains the master control.') }}</p></div>
      <div class="audio-boundary"><b>{{ tx('认不准的声音保持原样', 'Uncertain Sounds Stay Unchanged') }}</b><span>{{ tx('共享语音、场景音频和未收录事件不会被误静音', 'Shared voices, scene audio, and unmapped events are never muted by mistake') }}</span></div>
    </section>

    <section class="audio-setup">
      <div class="setup-copy"><h3>{{ tx('第一步 · 启动游戏并确认连接', 'Step 1 · Start the Game and Check the Connection') }}</h3><p>{{ tx('不需要安装其他程序。游戏启动后点“重新检测”，本页会确认能否安全开启角色音量控制。', 'No other program is required. After starting the game, select Check Again so this page can confirm that character volume control is ready.') }}</p></div>
      <div class="path-row"><div><b>{{ workspace?.gameRunning ? tx('游戏进程已连接', 'Game process connected') : tx('等待游戏进程', 'Waiting for game process') }}</b><code>{{ workspace?.recoveryRequired ? runtimeText(workspace?.detail) || tx('恢复失败，需要先停用恢复', 'Restoration failed; disable and restore first') : runtimeActive ? tx('音频运行时正在工作', 'Audio runtime active') : tx('当前未开启；调整后在页面底部主动应用', 'Currently off. Adjust settings, then apply them explicitly at the bottom.') }}</code></div><button class="ui-btn is-sm" type="button" :disabled="busy" @click="refresh">{{ tx('重新检测', 'Check again') }}</button></div>
    </section>

    <section class="preset-panel">
      <header><div><h3>{{ tx('第二步 · 调整音量，或载入本机组合', 'Step 2 · Adjust Volumes or Load a Local Mix') }}</h3><p>{{ tx('保存组合只会记在这台电脑上，不会立刻改变游戏；载入后还要在页面底部确认应用。', 'Saving a mix stores it on this PC only and does not change the game. After loading, apply it explicitly at the bottom of the page.') }}</p></div><div class="preset-compose"><input v-model="presetName" class="ui-input" maxlength="40" :placeholder="tx('例如：联机安静模式', 'Example: Quiet co-op')" @keyup.enter="savePreset" /><button type="button" class="ui-btn is-sm" @click="savePreset">{{ tx('保存当前组合', 'Save Current Mix') }}</button></div></header>
      <div v-if="presets.length" class="preset-list"><article v-for="preset in presets" :key="preset.id"><span><b>{{ preset.name }}</b><small>{{ Object.values(preset.volumes || {}).filter(value => value !== 100).length }} {{ tx('条调整', 'adjusted') }}</small></span><button type="button" class="ui-btn is-sm" @click="applyPreset(preset)">{{ tx('载入', 'Load') }}</button><button type="button" class="preset-delete" :aria-label="tx(`删除 ${preset.name}`, `Delete ${preset.name}`)" @click="deletePreset(preset.id)">×</button></article></div>
      <p v-else class="preset-empty">{{ tx('还没有本机预设。先调整音轨，再保存当前组合。', 'No local presets yet. Adjust tracks, then save the current mix.') }}</p>
    </section>

    <section class="mixer-panel">
      <div class="mixer-toolbar"><div><h3>{{ tx('角色音轨', 'Character tracks') }}</h3><p>{{ tx(`${changedCount} 条音轨偏离 100%`, `${changedCount} tracks differ from 100%`) }}</p></div><div class="toolbar-actions"><input v-model="search" class="ui-input" type="search" :placeholder="tx('搜索角色或编号', 'Search character or ID')" /><button class="ui-btn is-sm is-ghost" type="button" @click="resetAll">{{ tx('全部恢复 100%', 'Reset all to 100%') }}</button></div></div>
      <div class="ui-track-row">
        <article class="track ui-track" :class="{ changed: uiVolume !== 100 }">
          <header><div class="track-mark">UI</div><div><h4>{{ tx('界面音效', 'UI sound effects') }}</h4><code>ui*.bnk · Volume_SE</code></div><output>{{ uiVolume }}%</output></header>
          <input v-model.number="uiVolume" class="audio-range" type="range" min="0" max="100" step="1" :aria-label="tx(`界面音效 ${uiVolume}%`, `UI sound effects ${uiVolume}%`)" />
          <div class="quick-levels"><button v-for="value in [0, 25, 50, 75, 100]" :key="value" type="button" :class="{ active: uiVolume === value }" @click="uiVolume = value">{{ value }}</button></div>
          <p>{{ tx('只处理游戏目录中明确以 ui 命名的 8 个界面 bank；战斗、环境与未知音效保持原样。', 'Only the eight banks explicitly named ui in the game directory are adjusted; combat, ambience, and unknown effects stay unchanged.') }}</p>
        </article>
      </div>
      <div class="track-grid">
        <article v-for="character in filteredCharacters" :key="character.id" class="track" :class="{ changed: volumes[character.id] !== 100 }">
          <header><div class="track-mark">{{ character.id.slice(0, 2) }}</div><div><h4>{{ displayName(character) }}</h4><code>{{ character.id }}</code></div><output>{{ volumes[character.id] }}%</output></header>
          <input v-model.number="volumes[character.id]" class="audio-range" type="range" min="0" max="100" step="1" :aria-label="`${displayName(character)} ${volumes[character.id]}%`" />
          <div class="quick-levels"><button v-for="value in [0, 25, 50, 75, 100]" :key="value" type="button" :class="{ active: volumes[character.id] === value }" @click="volumes[character.id] = value">{{ value }}</button></div>
        </article>
      </div>
    </section>

    <section class="audio-dock"><label class="diagnostic-toggle"><input v-model="diagnostic" type="checkbox" /><span><b>{{ tx('诊断日志', 'Diagnostic Logging') }}</b><small>{{ workspace?.recoveryRequired ? tx('上次恢复尚未完成，请先点“重试恢复”。', 'The previous restoration is incomplete. Select Retry Restoration first.') : tx('默认关闭；只有排查声音识别问题时才需要开启', 'Off by default; enable only while diagnosing sound-identification issues') }}</small></span></label><div class="dock-actions"><button v-if="workspace?.owned" class="ui-btn is-danger" type="button" :disabled="busy" @click="remove">{{ workspace?.recoveryRequired ? tx('重试恢复', 'Retry Restoration') : tx('停用并恢复原音', 'Disable and Restore Original Audio') }}</button><button class="ui-btn is-primary" type="button" :disabled="!canSave" @click="save">{{ busy ? tx('处理中…', 'Working…') : workspace?.recoveryRequired ? tx('需先恢复', 'Restore First') : runtimeActive ? tx('保存并热更新', 'Save and Hot-Update') : tx('开启角色音量控制', 'Enable Character Volume Control') }}</button></div></section>
    <div v-if="message" class="ui-notice" :class="{ 'is-danger': tone === 'danger', 'is-ok': tone === 'ok' }" role="status">{{ message }}</div>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.audio-lab { width:100%; max-width:100%; min-width:0; container:audio-lab / inline-size; overflow-x:clip; padding-bottom:80px; }
.audio-intro,.audio-setup,.preset-panel,.mixer-panel,.mixer-toolbar,.track-grid,.audio-dock { width:100%; max-width:100%; min-width:0; }
.audio-intro { display:grid; grid-template-columns:minmax(0,1fr) minmax(240px,320px); align-items:end; gap:var(--space-6); padding:var(--space-6) 0 var(--space-5); border-bottom:1px solid var(--border-default); }
.audio-kicker { margin:0 0 var(--space-2); color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.06em; }
.audio-intro h2,.audio-setup h3,.mixer-panel h3 { margin:0; color:var(--text-primary); font-family:var(--font-display); }
.audio-intro h2 { font-size:var(--fs-2xl); }
.audio-intro p,.setup-copy p,.mixer-toolbar p { margin:var(--space-2) 0 0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.audio-boundary { padding:var(--space-4); border-left:3px solid var(--success); background:var(--success-bg); color:var(--success-ink); }
.audio-boundary b,.audio-boundary span { display:block; }.audio-boundary span { margin-top:4px; font-size:var(--fs-xs); }
.audio-setup { display:grid; grid-template-columns:minmax(220px,.7fr) minmax(0,1.3fr); gap:var(--space-5); align-items:center; }
.audio-setup .ui-notice { grid-column:1 / -1; }
.preset-panel { display:grid; gap:var(--space-3); padding:var(--space-4); border:1px solid var(--border-default); border-left:3px solid var(--accent); border-radius:var(--radius-sm); background:var(--surface-card); }
.preset-panel > header { min-width:0; display:grid; grid-template-columns:minmax(220px,1fr) minmax(300px,.8fr); gap:var(--space-4); align-items:end; }
.preset-panel h3 { margin:0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); }
.preset-panel p { margin:var(--space-1) 0 0; color:var(--text-muted); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.preset-compose { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-2); }
.preset-list { min-width:0; display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:var(--space-2); }
.preset-list article { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto 28px; gap:var(--space-2); align-items:center; padding:var(--space-2) var(--space-3); border:1px solid var(--border-soft); background:var(--surface-sunken); }
.preset-list article span { min-width:0; display:grid; gap:1px; }.preset-list b,.preset-list small { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.preset-list b { color:var(--text-primary); font-size:var(--fs-xs); }.preset-list small { color:var(--text-muted); font-size:var(--fs-2xs); }
.preset-delete { width:28px; height:28px; border:0; background:transparent; color:var(--text-muted); cursor:pointer; }.preset-delete:hover { color:var(--danger-ink); background:var(--danger-bg); }
.preset-empty { padding:var(--space-3); border:1px dashed var(--border-default); text-align:center; }
.path-row { display:flex; align-items:center; gap:var(--space-3); min-width:0; padding:var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-card); }
.path-row > div { min-width:0; flex:1; }.path-row b,.path-row code { display:block; }.path-row b { color:var(--text-primary); font-size:var(--fs-sm); }.path-row code { margin-top:4px; overflow:hidden; color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); text-overflow:ellipsis; white-space:nowrap; }
.mixer-toolbar { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-4); margin-bottom:var(--space-4); }.toolbar-actions { display:flex; gap:var(--space-2); width:min(100%,560px); }.toolbar-actions .ui-input { min-width:0; flex:1; }
.track-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:var(--space-3); overflow:hidden; }
.ui-track-row { display:grid; grid-template-columns:minmax(0,1fr); margin-bottom:var(--space-3); }
.ui-track { border-left:3px solid var(--accent); }
.ui-track p { margin:var(--space-2) 0 0; color:var(--text-muted); font-size:var(--fs-2xs); line-height:var(--lh-normal); }
.track { min-width:0; padding:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-card); box-shadow:var(--shadow-1); }.track.changed { border-color:var(--selected-border); background:var(--selected-bg); }
.track header { display:grid; grid-template-columns:34px minmax(0,1fr) auto; align-items:center; gap:var(--space-3); }.track-mark { display:grid; place-items:center; width:34px; height:34px; border:1px solid var(--accent-border); border-radius:50%; color:var(--accent); background:var(--surface-card-pop); font-family:var(--font-data); font-size:10px; font-weight:var(--fw-bold); }.track h4 { margin:0; overflow:hidden; color:var(--text-primary); font-size:var(--fs-sm); text-overflow:ellipsis; white-space:nowrap; }.track code { color:var(--text-muted); font-family:var(--font-data); font-size:10px; }.track output { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-lg); font-weight:var(--fw-bold); }
.audio-range { width:100%; height:22px; margin:var(--space-3) 0 var(--space-2); appearance:none; background:transparent; accent-color:var(--accent); }.audio-range::-webkit-slider-runnable-track { height:5px; border:1px solid var(--border-strong); border-radius:999px; background:linear-gradient(90deg,var(--accent-soft),var(--surface-field)); }.audio-range::-webkit-slider-thumb { width:16px; height:16px; margin-top:-6px; appearance:none; border:2px solid var(--surface-card-pop); border-radius:50%; background:var(--accent); box-shadow:0 0 0 1px var(--accent-border); cursor:pointer; }
.quick-levels { display:grid; grid-template-columns:repeat(5,minmax(0,1fr)); gap:3px; }.quick-levels button { width:100%; min-width:0; height:24px; padding:0; overflow:hidden; border:1px solid var(--border-default); border-radius:var(--radius-xs); color:var(--text-muted); background:var(--surface-field); font-family:var(--font-data); font-size:10px; cursor:pointer; }.quick-levels button.active { border-color:var(--accent-border); color:var(--text-on-accent); background:var(--accent); }
.audio-dock { position:sticky; z-index:10; bottom:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); min-height:66px; padding:var(--space-3) var(--space-4); border:1px solid var(--border-strong); border-left:4px solid var(--accent); border-radius:var(--radius-md); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }.diagnostic-toggle { display:flex; align-items:center; gap:var(--space-3); cursor:pointer; }.diagnostic-toggle input { width:18px; height:18px; accent-color:var(--accent); }.diagnostic-toggle b,.diagnostic-toggle small { display:block; }.diagnostic-toggle b { color:var(--text-primary); font-size:var(--fs-sm); }.diagnostic-toggle small { color:var(--text-muted); font-size:var(--fs-xs); }.dock-actions { display:flex; gap:var(--space-2); }
@container audio-lab (max-width:980px) { .track-grid,.preset-list { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@container audio-lab (max-width:720px) { .audio-intro,.audio-setup,.preset-panel > header { grid-template-columns:minmax(0,1fr); }.mixer-toolbar,.audio-dock { align-items:stretch; flex-direction:column; }.toolbar-actions,.dock-actions,.dock-actions .ui-btn { width:100%; } }
@container audio-lab (max-width:520px) { .track-grid,.preset-list,.preset-compose { grid-template-columns:minmax(0,1fr); }.toolbar-actions { flex-direction:column; }.path-row { align-items:stretch; flex-direction:column; }.path-row .ui-btn,.preset-compose .ui-btn { width:100%; } }
</style>
