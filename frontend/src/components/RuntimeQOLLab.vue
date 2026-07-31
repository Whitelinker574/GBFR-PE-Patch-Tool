<script setup>
import { computed, onActivated, onBeforeUnmount, onDeactivated, ref } from 'vue'
import { DeployRuntimeQOL, GetRuntimeQOLWorkspace, RemoveRuntimeQOL } from '../../wailsjs/go/backend/App'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime.js'
import { language } from '../i18n.js'
import { runtimeCompanionMessage } from '../runtimeCompanionMessages.js'

const emit = defineEmits(['status'])
const tx = (zh, en) => language.value === 'zh' ? zh : en
const workspace = ref(null)
const config = ref({
  damageCapPercentage: true,
  detailedEnemyHp: true,
  detailedSba: true,
  sessionCapture: true,
  normalQuestLevelSync: false,
  returnWrightstone: false,
  freeCaptain: false,
  enemyHpPrecision: 2,
  sbaPrecision: 2,
})
const busy = ref(false)
const message = ref('')
const tone = ref('')
let pollTimer = 0
let componentActive = true

const selectedCount = computed(() => ['damageCapPercentage', 'detailedEnemyHp', 'detailedSba', 'sessionCapture', 'normalQuestLevelSync', 'returnWrightstone', 'freeCaptain'].filter(key => config.value[key]).length)
const canEnable = computed(() => !busy.value && !workspace.value?.recoveryRequired && workspace.value?.gameRunning && selectedCount.value > 0)

function report(value, nextTone = '') {
  message.value = runtimeCompanionMessage(value, language.value)
  tone.value = nextTone
  emit('status', message.value, nextTone)
}

function applyWorkspace(value, replaceConfig = true) {
  workspace.value = value
  if (replaceConfig && value?.config) config.value = { ...config.value, ...value.config }
}

function pausePoll() {
  window.clearTimeout(pollTimer)
  pollTimer = 0
}

function schedulePoll() {
  pausePoll()
  if (!componentActive) return
  if (!workspace.value?.active) return
  pollTimer = window.setTimeout(async () => {
    try { applyWorkspace(await GetRuntimeQOLWorkspace(), false) }
    catch (error) { report(error, 'danger') }
    finally { schedulePoll() }
  }, 1000)
}

async function refresh() {
  busy.value = true
  try { applyWorkspace(await GetRuntimeQOLWorkspace()) }
  catch (error) { report(error, 'danger') }
  finally { busy.value = false; schedulePoll() }
}

async function enable() {
  busy.value = true
  try {
    applyWorkspace(await DeployRuntimeQOL({ ...config.value }))
    report(tx('游戏便利运行时已开启；切换页面不会停用。', 'Convenience runtime enabled and remains active across page navigation.'), 'ok')
  } catch (error) { report(error, 'danger') }
  finally { busy.value = false; schedulePoll() }
}

async function disable() {
  busy.value = true
  try {
    await RemoveRuntimeQOL('')
    applyWorkspace(await GetRuntimeQOLWorkspace(), false)
    report(tx('全部便利 Hook 已恢复。', 'All convenience hooks were restored.'), 'ok')
  } catch (error) { report(error, 'danger') }
  finally { busy.value = false; schedulePoll() }
}

async function copySession() {
	const sessionId = String(workspace.value?.latestSessionId || '').trim()
	if (!sessionId) return
	const copied = await ClipboardSetText(sessionId)
  report(copied ? tx('房间 ID 已复制。', 'Room ID copied.') : tx('复制房间 ID 失败。', 'Failed to copy the room ID.'), copied ? 'ok' : 'danger')
}

function deactivatePolling() {
  componentActive = false
  pausePoll()
}

function activatePolling() {
  componentActive = true
  schedulePoll()
}

onDeactivated(deactivatePolling)
onActivated(activatePolling)
onBeforeUnmount(deactivatePolling)
refresh()
</script>

<template>
  <div class="qol-lab ui-page-stack">
    <section class="qol-intro">
      <div><p>INTEGRATED QOL RUNTIME · DLC 2.0.3</p><h2>{{ tx('游戏便利运行时', 'Game Convenience Runtime') }}</h2><span>{{ tx('把显示、联机信息、任务同步和编队便利集中到一个可恢复的内置运行时；无需外部加载器。', 'Keep display, session, quest-sync, and formation conveniences in one restorable built-in runtime without an external loader.') }}</span></div>
      <aside><b>{{ workspace?.recoveryRequired ? tx('需要恢复', 'Recovery required') : workspace?.active ? tx('运行中', 'Active') : tx('未开启', 'Inactive') }}</b><small>{{ workspace?.recoveryRequired ? runtimeCompanionMessage(workspace?.detail, language.value) : workspace?.active ? `PID ${workspace.pid}` : workspace?.gameRunning ? tx('游戏已启动，可以开启', 'Game detected and ready') : tx('请先启动游戏', 'Start the game first') }}</small></aside>
    </section>

    <section class="qol-options ui-card ui-panel">
      <header><div><h3>{{ tx('选择要常驻的功能', 'Choose Persistent Features') }}</h3><p>{{ tx('保存会先恢复旧 Hook，再按当前组合重新校验和启用。', 'Saving restores the previous hooks before validating and enabling this exact combination.') }}</p></div><span class="ui-tag">{{ selectedCount }} / 7</span></header>
      <div class="option-grid">
        <label class="qol-option"><input v-model="config.damageCapPercentage" type="checkbox" /><span><b>{{ tx('训练场外显示伤害上限百分比', 'Damage-Cap Percentage Anywhere') }}</b><small>{{ tx('只对玩家伤害显示上限百分比；任务检查在启用时安装一次，停用时完整恢复。', 'Shows cap percentage only for player damage; the quest check is installed once and fully restored on disable.') }}</small></span></label>
        <label class="qol-option"><input v-model="config.detailedEnemyHp" type="checkbox" /><span><b>{{ tx('敌人 HP 百分比小数', 'Detailed Enemy HP Percentage') }}</b><small>{{ tx('保留敌人血条百分比的小数，不改变实际 HP。', 'Keeps decimal precision in enemy HP percentages without changing HP.') }}</small></span><select v-model.number="config.enemyHpPrecision" class="ui-select" :disabled="!config.detailedEnemyHp"><option v-for="value in 5" :key="value - 1" :value="value - 1">{{ value - 1 }}</option></select></label>
        <label class="qol-option"><input v-model="config.detailedSba" type="checkbox" /><span><b>{{ tx('奥义槽百分比小数', 'Detailed SBA Percentage') }}</b><small>{{ tx('只在奥义槽更新上下文中格式化小数，不会误改等级和 HP 文本。', 'Formats decimals only in the SBA update context, leaving level and HP text untouched.') }}</small></span><select v-model.number="config.sbaPrecision" class="ui-select" :disabled="!config.detailedSba"><option v-for="value in 5" :key="value - 1" :value="value - 1">{{ value - 1 }}</option></select></label>
        <label class="qol-option"><input v-model="config.sessionCapture" type="checkbox" /><span><b>{{ tx('捕获并自动复制房间 ID', 'Capture and Auto-Copy Room ID') }}</b><small>{{ tx('识别游戏实际显示的四段房间 ID，新房间出现后自动复制；屏蔽文本和普通 UI 文本不会记录。', 'Recognizes the four-part room ID shown by the game and copies each new room automatically while ignoring masked and ordinary UI text.') }}</small></span></label>
        <label class="qol-option is-experimental"><input v-model="config.normalQuestLevelSync" type="checkbox" /><span><b>{{ tx('普通任务等级同步 · 实验', 'Normal-Quest Level Sync · Experimental') }}</b><small>{{ tx('已验证入口、写后状态和停用恢复；普通任务类型白名单与完整任务结果仍需长测，开启后请自行核对任务等级。', 'The entry point, post-write state, and restoration are verified. Quest-type coverage and full quest results still need long-run testing, so check the quest level after enabling.') }}</small></span></label>
        <label class="qol-option is-experimental"><input v-model="config.returnWrightstone" type="checkbox" /><span><b>{{ tx('重镶返还原祝福石 · 实验', 'Return Replaced Wrightstone · Experimental') }}</b><small>{{ tx('已验证入口、写后状态和停用恢复；完整交易提交、旧石消耗与背包增量仍需游戏内长测，开启后请核对背包。', 'The entry point, post-write state, and restoration are verified. Full transaction commit, old-stone consumption, and inventory increment still need long-run in-game testing, so check the inventory after enabling.') }}</small></span></label>
        <label class="qol-option"><input v-model="config.freeCaptain" type="checkbox" /><span><b>{{ tx('主线自由替换古兰 / 姬塔', 'Free Captain in Main Story') }}</b><small>{{ tx('仅解除主线编队中的团长固定限制；命运篇章、教程和其他固定编队保持原样。', 'Removes only the Captain lock in main-story formations; Fate Episodes, tutorials, and other fixed parties remain unchanged.') }}</small></span></label>
      </div>
    </section>

    <section class="session-panel ui-card ui-panel">
      <div><small>{{ tx('最近捕获', 'LATEST CAPTURE') }}</small><strong>{{ workspace?.latestSessionId || tx('进入联机房间后自动显示', 'Appears after entering an online room') }}</strong><span v-if="workspace?.sessionSequence">#{{ workspace.sessionSequence }}</span></div>
      <button type="button" class="ui-btn" :disabled="!workspace?.latestSessionId" @click="copySession">{{ tx('复制房间 ID', 'Copy Room ID') }}</button>
    </section>

    <section class="qol-dock">
      <div><b>{{ tx('恢复边界', 'Restoration Boundary') }}</b><small>{{ tx('明确停用、F12 紧急停止或关闭应用都会恢复本工具管理的 Hook。', 'Explicit disable, F12 emergency stop, or app shutdown restores every hook owned here.') }}</small></div>
      <div class="ui-actions"><button type="button" class="ui-btn is-ghost" :disabled="busy" @click="refresh">{{ tx('刷新状态', 'Refresh') }}</button><button v-if="workspace?.owned" type="button" class="ui-btn is-danger" :disabled="busy" @click="disable">{{ workspace?.recoveryRequired ? tx('重试恢复', 'Retry Restoration') : tx('停用并恢复', 'Disable and Restore') }}</button><button type="button" class="ui-btn is-primary" :disabled="!canEnable" @click="enable">{{ busy ? tx('处理中…', 'Working…') : workspace?.recoveryRequired ? tx('需先恢复', 'Restore First') : workspace?.active ? tx('按当前设置重启', 'Restart with Settings') : tx('开启运行时', 'Enable Runtime') }}</button></div>
    </section>
    <div v-if="message" class="ui-notice" :class="{ 'is-danger': tone === 'danger', 'is-ok': tone === 'ok' }" role="status">{{ message }}</div>
  </div>
</template>

<style scoped>
.qol-lab{width:100%;min-width:0;padding-bottom:72px;container:qol-lab / inline-size}.qol-intro{display:grid;grid-template-columns:minmax(0,1fr) minmax(220px,300px);gap:var(--space-5);align-items:end;padding:var(--space-6) 0 var(--space-5);border-bottom:1px solid var(--border-default)}.qol-intro p{margin:0 0 var(--space-2);color:var(--accent);font-family:var(--font-data);font-size:var(--fs-xs);font-weight:var(--fw-bold)}.qol-intro h2{margin:0;color:var(--text-primary);font-family:var(--font-display);font-size:var(--fs-2xl)}.qol-intro span{display:block;margin-top:var(--space-2);color:var(--text-secondary)}.qol-intro aside{padding:var(--space-4);border-left:3px solid var(--accent);background:var(--accent-soft)}.qol-intro aside b,.qol-intro aside small{display:block}.qol-intro aside small{margin-top:4px;color:var(--text-muted)}.qol-options>header{display:flex;align-items:flex-start;justify-content:space-between;gap:var(--space-4)}.qol-options h3,.qol-options p{margin:0}.qol-options p{margin-top:4px;color:var(--text-secondary);font-size:var(--fs-sm)}.option-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:var(--space-3);margin-top:var(--space-4)}.qol-option{display:grid;grid-template-columns:auto minmax(0,1fr) auto;gap:var(--space-3);align-items:start;min-width:0;padding:var(--space-4);border:1px solid var(--border-default);border-radius:var(--radius-sm);background:var(--surface-card-pop);cursor:pointer}.qol-option:has(input:checked){border-color:var(--accent-border);box-shadow:3px 0 0 var(--accent) inset;background:color-mix(in srgb,var(--accent-soft) 36%,var(--surface-card-pop))}.qol-option input{width:18px;height:18px;margin:2px 0 0;accent-color:var(--accent)}.qol-option span{min-width:0}.qol-option b,.qol-option small{display:block}.qol-option b{color:var(--text-primary);font-size:var(--fs-sm)}.qol-option small{margin-top:4px;color:var(--text-muted);font-size:var(--fs-xs);line-height:var(--lh-normal)}.qol-option .ui-select{width:64px}.session-panel,.qol-dock{display:flex;align-items:center;justify-content:space-between;gap:var(--space-4)}.session-panel>div{min-width:0}.session-panel small,.session-panel strong,.session-panel span{display:block}.session-panel small{color:var(--accent);font-family:var(--font-data);font-weight:var(--fw-bold)}.session-panel strong{margin-top:4px;color:var(--text-primary);font-family:var(--font-data);font-size:var(--fs-lg);overflow-wrap:anywhere}.session-panel span{margin-top:2px;color:var(--text-muted);font-size:var(--fs-xs)}.qol-dock{position:sticky;z-index:10;bottom:0;padding:var(--space-3) var(--space-4);border:1px solid var(--border-strong);border-left:4px solid var(--accent);border-radius:var(--radius-sm);background:var(--surface-card-pop);box-shadow:var(--shadow-2)}.qol-dock b,.qol-dock small{display:block}.qol-dock small{margin-top:3px;color:var(--text-muted);font-size:var(--fs-xs)}
@container qol-lab (max-width:760px){.option-grid{grid-template-columns:minmax(0,1fr)}.qol-intro{grid-template-columns:minmax(0,1fr)}}@container qol-lab (max-width:520px){.qol-options>header,.session-panel,.qol-dock{align-items:stretch;flex-direction:column}.session-panel .ui-btn,.qol-dock .ui-actions,.qol-dock .ui-btn{width:100%}.qol-dock .ui-actions{display:grid;grid-template-columns:minmax(0,1fr)}}
.qol-option.is-experimental{border-style:dashed}.qol-option.is-experimental input{cursor:pointer}
</style>
