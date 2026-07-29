<script setup>
import { computed, ref } from 'vue'
import { DeployCameraMod, GetCameraWorkspace, RemoveCameraMod } from '../../wailsjs/go/backend/App'
import { language } from '../i18n'
import { runtimeCompanionMessage } from '../runtimeCompanionMessages.js'
import ConfirmDialog from './ConfirmDialog.vue'

const emit = defineEmits(['status'])
const tx = (zh, en) => language.value === 'zh' ? zh : en
const runtimeText = value => runtimeCompanionMessage(value, language.value)
const confirmDialog = ref(null)
const workspace = ref(null)
const maxDistance = ref(6)
const targetHeight = ref(1.8)
const zoomStep = ref(0.02)
const busy = ref(false)
const refreshing = ref(false)
const message = ref('')
const tone = ref('')
let refreshRequest = 0

const canSave = computed(() => !busy.value && !refreshing.value && !workspace.value?.recoveryRequired && Boolean(workspace.value?.gameRunning))

function report(text, nextTone = '') {
  message.value = text
  tone.value = nextTone
  emit('status', text, nextTone)
}

function applyWorkspace(value) {
  workspace.value = value
  const config = value?.config
  if (config) {
    maxDistance.value = Number(config.maxDistance ?? 6)
    targetHeight.value = Number(config.targetHeight ?? 1.8)
    zoomStep.value = Number(config.zoomStep ?? 0.02)
  }
}

async function refresh() {
  const request = ++refreshRequest
  refreshing.value = true
  try {
    const next = await GetCameraWorkspace('')
    if (request !== refreshRequest) return
    applyWorkspace(next)
  }
  catch (error) { report(runtimeText(error), 'danger') }
  finally { if (request === refreshRequest) refreshing.value = false }
}

function usePreset(kind) {
  if (kind === 'game') {
    maxDistance.value = 4.8
    targetHeight.value = 1.8
    zoomStep.value = 0.05
  } else {
    maxDistance.value = 6
    targetHeight.value = 1.8
    zoomStep.value = 0.02
  }
}

async function save() {
  busy.value = true
  try {
    const result = await DeployCameraMod({
      maxDistance: Number(maxDistance.value),
      targetHeight: Number(targetHeight.value),
      zoomStep: Number(zoomStep.value),
    })
    report(tx('内置镜头运行时已开启，三个参数均已应用；之后保存会直接热更新。', 'The built-in camera runtime is active. All three values are applied and later saves hot-update directly.'), 'ok')
    await refresh()
  } catch (error) { report(runtimeText(error), 'danger') }
  finally { busy.value = false }
}

async function remove() {
  const accepted = await confirmDialog.value?.ask({
    title: tx('停用镜头运行时', 'Disable camera runtime'),
    message: tx('会先恢复镜头原值和三个 Hook，再停用内置运行时；保存的参数会保留。', 'The original camera values and all three hooks are restored before the built-in runtime stops. Saved values are kept.'),
    confirmLabel: tx('确认停用', 'Disable'), cancelLabel: tx('取消', 'Cancel'), tone: 'danger',
  })
  if (!accepted) return
  busy.value = true
  try { await RemoveCameraMod(''); report(tx('镜头运行时已停用，原值和 Hook 已恢复。', 'Camera runtime disabled; original values and hooks restored.'), 'ok'); await refresh() }
  catch (error) { report(runtimeText(error), 'danger') }
  finally { busy.value = false }
}

refresh()
</script>

<template>
  <div class="camera-lab ui-page-stack">
    <section class="camera-intro">
      <div>
        <p class="camera-kicker">TOWN CAMERA · BUILT-IN RUNTIME</p>
        <h2>{{ tx('城镇镜头工坊', 'Town Camera Workshop') }}</h2>
        <p>{{ tx('调整城镇镜头的最远距离、视线目标高度与滚轮缩放手感；不会改变战斗镜头。', 'Tune town-camera distance, target height, and wheel zoom feel without changing the combat camera.') }}</p>
      </div>
      <div class="camera-boundary"><b>{{ tx('锁定 DLC 2.0.2', 'Locked to DLC 2.0.2') }}</b><span>{{ tx('三个入口必须唯一命中；卸载时恢复原值', 'All three entries must match uniquely; unload restores the originals') }}</span></div>
    </section>

    <section class="camera-setup">
      <div><h3>{{ tx('第一步 · 启动并连接游戏', 'Step 1 · Start and connect the game') }}</h3><p>{{ tx('无需安装加载器或选择目录。保存时应用会把自有运行时直接注入当前游戏进程。', 'No loader installation or folder selection is needed. Saving injects the tool-owned runtime directly into the current game process.') }}</p></div>
      <div class="path-row"><div><b>{{ workspace?.gameRunning ? tx('游戏进程已连接', 'Game process connected') : tx('等待游戏进程', 'Waiting for game process') }}</b><code>{{ workspace?.recoveryRequired ? runtimeText(workspace?.detail) || tx('恢复失败，需要先停用恢复', 'Restoration failed; disable and restore first') : workspace?.state === 'active' ? tx('镜头运行时正在工作', 'Camera runtime active') : runtimeText(workspace?.detail) || tx('启动游戏后即可开启', 'Start the game to enable') }}</code></div><button class="ui-btn is-sm" type="button" :disabled="busy || refreshing" @click="refresh">{{ refreshing ? tx('正在检查…', 'Checking…') : tx('重新检测', 'Check again') }}</button></div>
    </section>

    <section class="camera-controls">
      <header class="controls-heading"><div><h3>{{ tx('第二步 · 调整镜头参数', 'Step 2 · Tune camera parameters') }}</h3><p>{{ tx('数值输入与滑轨保持同步；内置运行时开启后，三个参数都可以直接热更新。', 'Numeric inputs stay synchronized with the sliders. Once active, all three values hot-update directly.') }}</p></div><div class="preset-actions"><button class="ui-btn is-sm is-ghost" type="button" @click="usePreset('game')">{{ tx('游戏默认', 'Game defaults') }}</button><button class="ui-btn is-sm" type="button" @click="usePreset('comfort')">{{ tx('舒适预设', 'Comfort preset') }}</button></div></header>
      <div class="parameter-grid">
        <article class="parameter">
          <header><div><span>01</span><h4>{{ tx('最远距离', 'Maximum distance') }}</h4></div><label><input v-model.number="maxDistance" class="ui-input" type="number" min="0.5" max="30" step="0.1" /><small>0.5-30</small></label></header>
          <input v-model.number="maxDistance" class="camera-range" type="range" min="0.5" max="30" step="0.1" />
          <p>{{ tx('决定滚轮拉远后镜头与角色的最大距离。游戏默认 4.8。', 'Sets the farthest distance from the character after zooming out. Game default: 4.8.') }}</p>
        </article>
        <article class="parameter">
          <header><div><span>02</span><h4>{{ tx('视线目标高度', 'Target height') }}</h4></div><label><input v-model.number="targetHeight" class="ui-input" type="number" min="0" max="5" step="0.1" /><small>0-5</small></label></header>
          <input v-model.number="targetHeight" class="camera-range" type="range" min="0" max="5" step="0.1" />
          <p>{{ tx('调整拉远时镜头看向角色的垂直位置。游戏默认 1.8。', 'Adjusts the vertical point the zoomed-out camera looks toward. Game default: 1.8.') }}</p>
        </article>
        <article class="parameter">
          <header><div><span>03</span><h4>{{ tx('滚轮缩放步长', 'Wheel zoom step') }}</h4></div><label><input v-model.number="zoomStep" class="ui-input" type="number" min="0.001" max="1" step="0.001" /><small>0.001-1</small></label></header>
          <input v-model.number="zoomStep" class="camera-range" type="range" min="0.001" max="1" step="0.001" />
          <p>{{ tx('数值越小，每格滚轮移动越细。游戏默认 0.05，舒适预设为 0.02。', 'Lower values make each wheel notch finer. Game default: 0.05; comfort preset: 0.02.') }}</p>
        </article>
      </div>
    </section>

    <section class="camera-dock">
      <div class="runtime-state"><span :class="{ active: workspace?.state === 'active' }"></span><div><b>{{ tx('第三步 · 应用到当前游戏', 'Step 3 · Apply to the current game') }}</b><small>{{ workspace?.recoveryRequired ? tx('Hook 恢复未完成，只能先重试恢复；请勿重新注入。', 'Hook restoration is incomplete. Retry restoration before injecting again.') : workspace?.state === 'active' ? tx('内置运行时已开启；保存会立即热更新', 'Built-in runtime active; saving hot-updates immediately') : tx('启动游戏后即可一键开启，不需要其他程序', 'Start the game, then enable it here without another program') }}</small></div></div>
      <div class="dock-actions"><button v-if="workspace?.owned" class="ui-btn is-danger" type="button" :disabled="busy || refreshing" @click="remove">{{ workspace?.recoveryRequired ? tx('重试恢复', 'Retry restoration') : tx('停用并恢复', 'Disable and restore') }}</button><button class="ui-btn is-primary" type="button" :disabled="!canSave" @click="save">{{ busy ? tx('处理中…', 'Working…') : workspace?.recoveryRequired ? tx('需先恢复', 'Restore first') : workspace?.state === 'active' ? tx('保存并热更新', 'Save and hot-update') : tx('开启镜头运行时', 'Enable camera runtime') }}</button></div>
    </section>
    <div v-if="message" class="ui-notice" :class="{ 'is-danger': tone === 'danger', 'is-ok': tone === 'ok' }" role="status">{{ message }}</div>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.camera-lab { width:100%; max-width:100%; min-width:0; container:camera-lab / inline-size; overflow-x:clip; padding-bottom:80px; }
.camera-intro,.camera-setup,.camera-controls,.controls-heading,.parameter-grid,.camera-dock { width:100%; max-width:100%; min-width:0; }
.camera-intro { display:grid; grid-template-columns:minmax(0,1fr) minmax(240px,320px); align-items:end; gap:var(--space-6); padding:var(--space-6) 0 var(--space-5); border-bottom:1px solid var(--border-default); }
.camera-kicker { margin:0 0 var(--space-2); color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.06em; }
.camera-intro h2,.camera-setup h3,.camera-controls h3 { margin:0; color:var(--text-primary); font-family:var(--font-display); }
.camera-intro h2 { font-size:var(--fs-2xl); }.camera-intro p,.camera-setup p,.controls-heading p { margin:var(--space-2) 0 0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.camera-boundary { padding:var(--space-4); border-left:3px solid var(--accent); background:var(--accent-soft); color:var(--text-primary); }.camera-boundary b,.camera-boundary span { display:block; }.camera-boundary span { margin-top:4px; color:var(--text-secondary); font-size:var(--fs-xs); }
.camera-setup { display:grid; grid-template-columns:minmax(220px,.7fr) minmax(0,1.3fr); gap:var(--space-5); align-items:center; }.camera-setup .ui-notice { grid-column:1 / -1; }
.path-row { display:flex; align-items:center; gap:var(--space-3); min-width:0; padding:var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-card); }.path-row > div { min-width:0; flex:1; }.path-row b,.path-row code { display:block; }.path-row b { color:var(--text-primary); font-size:var(--fs-sm); }.path-row code { margin-top:4px; overflow:hidden; color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); text-overflow:ellipsis; white-space:nowrap; }
.controls-heading { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-4); margin-bottom:var(--space-4); }.preset-actions { display:flex; gap:var(--space-2); flex:0 0 auto; }
.parameter-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:var(--space-3); }.parameter { min-width:0; padding:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-card); box-shadow:var(--shadow-1); }.parameter > header { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-3); }.parameter header > div { min-width:0; }.parameter header span { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); }.parameter h4 { margin:3px 0 0; color:var(--text-primary); font-size:var(--fs-md); }.parameter label { width:92px; flex:0 0 92px; }.parameter label .ui-input { width:100%; min-width:0; text-align:right; }.parameter label small { display:block; margin-top:3px; color:var(--text-muted); font-family:var(--font-data); font-size:10px; text-align:right; }.parameter p { min-height:3.8em; margin:var(--space-2) 0 0; color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.camera-range { width:100%; height:22px; margin:var(--space-4) 0 var(--space-2); appearance:none; background:transparent; accent-color:var(--accent); }.camera-range::-webkit-slider-runnable-track { height:5px; border:1px solid var(--border-strong); border-radius:999px; background:linear-gradient(90deg,var(--accent-soft),var(--surface-field)); }.camera-range::-webkit-slider-thumb { width:16px; height:16px; margin-top:-6px; appearance:none; border:2px solid var(--surface-card-pop); border-radius:50%; background:var(--accent); box-shadow:0 0 0 1px var(--accent-border); cursor:pointer; }
.camera-dock { position:sticky; z-index:10; bottom:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); min-height:66px; padding:var(--space-3) var(--space-4); border:1px solid var(--border-strong); border-left:4px solid var(--accent); border-radius:var(--radius-md); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }.runtime-state { display:flex; align-items:center; gap:var(--space-3); }.runtime-state > span { width:10px; height:10px; border:1px solid var(--border-strong); border-radius:50%; background:var(--surface-field); }.runtime-state > span.active { border-color:var(--success); background:var(--success); }.runtime-state b,.runtime-state small { display:block; }.runtime-state b { color:var(--text-primary); font-size:var(--fs-sm); }.runtime-state small { color:var(--text-muted); font-size:var(--fs-xs); }.dock-actions { display:flex; gap:var(--space-2); }
@container camera-lab (max-width:900px) { .parameter-grid { grid-template-columns:minmax(0,1fr); }.parameter p { min-height:0; } }
@container camera-lab (max-width:680px) { .camera-intro,.camera-setup { grid-template-columns:minmax(0,1fr); }.controls-heading,.camera-dock { align-items:stretch; flex-direction:column; }.preset-actions,.dock-actions,.dock-actions .ui-btn { width:100%; }.preset-actions .ui-btn { flex:1; } }
@container camera-lab (max-width:480px) { .path-row { align-items:stretch; flex-direction:column; }.path-row .ui-btn { width:100%; }.parameter > header { align-items:stretch; flex-direction:column; }.parameter label { width:100%; flex-basis:auto; } }
</style>
