<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  RuntimeLoadoutDetectorDelete,
  RuntimeLoadoutDetectorExport,
  RuntimeLoadoutDetectorHistory,
  RuntimeLoadoutDetectorPublish,
  RuntimeLoadoutDetectorShare,
  RuntimeLoadoutDetectorStart,
  RuntimeLoadoutDetectorStatus,
  RuntimeLoadoutDetectorStop,
} from '../../wailsjs/go/backend/App'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { language } from '../i18n.js'
import { characterAssetIcon } from '../gameAssetIcons.js'
import CapturedLoadoutPreview from './CapturedLoadoutPreview.vue'

const emit = defineEmits(['status', 'deploy-loadout'])
const detectorRoot = ref(null)
const detectorStatus = ref({ enabled: false, state: 'stopped', historyCount: 0 })
const records = ref([])
const preview = ref(null)
const busy = ref('')
const historyLoading = ref(false)
const titles = reactive({})
let disposed = false
let stopStatusEvents = () => {}
let knownHistoryCount = -1
const DETECTOR_STATUS_EVENT = 'runtime-loadout-detector:status'

const tx = (zh, en) => language.value === 'en' ? en : zh
const isRunning = computed(() => detectorStatus.value?.enabled === true)
const statusCopy = computed(() => ({
  waiting_game: [tx('等待游戏启动', 'Waiting for Game'), tx('检测保持开启，游戏进程出现后会自动连接。', 'Detection stays active and reconnects when the game starts.')],
  waiting_task: [tx('等待进入任务', 'Waiting for a Quest'), tx('进入任务并形成稳定队伍后会自动归档。', 'A stable party is archived automatically after entering a quest.')],
  stabilizing: [tx('正在确认队伍', 'Confirming Party'), tx('正在核对连续稳定快照，避免把场景切换误记为任务。', 'Checking consecutive stable snapshots to avoid recording scene transitions.')],
  recording: [tx('后台检测中', 'Detecting in Background'), tx(`当前识别 ${detectorStatus.value.currentTeamSize || 0} 名角色，本场已保存。`, `${detectorStatus.value.currentTeamSize || 0} characters detected; this quest is saved.`)],
  stopped: [tx('检测已关闭', 'Detection Off'), tx('本地记录仍会保留，下次开启后继续追加。', 'Local records are retained and new quests append after restart.')],
})[detectorStatus.value?.state] || [tx('准备检测', 'Preparing Detection'), tx('正在读取检测器状态。', 'Reading detector status.')])

function errorMessage(error) {
  return (error instanceof Error ? error.message : String(error || '')).replace(/^Error:\s*/i, '')
}

function announce(message, tone = 'info') {
  emit('status', message, tone === 'danger' ? 'error' : tone === 'ok' ? 'success' : tone)
}

function timeText(value) {
  const milliseconds = Number(value || 0)
  if (!milliseconds) return tx('尚无', 'None')
  return new Intl.DateTimeFormat(language.value === 'en' ? 'en-US' : 'zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).format(new Date(milliseconds))
}

function recordTitle(record) {
  return tx(`第 ${record.sequence} 场`, `Quest ${record.sequence}`)
}

function titleKey(record, member) {
  return `${record.id}\u0000${member.role}`
}

function shareTitle(record, member) {
  return String(titles[titleKey(record, member)] || '').trim()
}

async function readHistory(force = false) {
  if (historyLoading.value || disposed) return
  historyLoading.value = true
  try {
    const next = await RuntimeLoadoutDetectorHistory()
    if (disposed) return
    records.value = Array.isArray(next) ? next : []
    knownHistoryCount = records.value.length
  } catch (error) {
    if (force) announce(tx(`读取本地配装记录失败：${errorMessage(error)}`, `Failed to read local loadouts: ${errorMessage(error)}`), 'danger')
  } finally {
    historyLoading.value = false
  }
}

async function readStatus({ forceHistory = false } = {}) {
  try {
    const next = await RuntimeLoadoutDetectorStatus()
    await acceptStatus(next, { forceHistory })
  } catch (error) {
    if (!disposed) announce(tx(`读取检测器状态失败：${errorMessage(error)}`, `Failed to read detector status: ${errorMessage(error)}`), 'danger')
  }
}

async function acceptStatus(next, { forceHistory = false } = {}) {
  if (disposed) return
  detectorStatus.value = next || { enabled: false, state: 'stopped', historyCount: 0 }
  if (forceHistory || Number(detectorStatus.value.historyCount || 0) !== knownHistoryCount) await readHistory(false)
}

async function startDetector() {
  if (busy.value) return
  busy.value = 'toggle'
  try {
    detectorStatus.value = await RuntimeLoadoutDetectorStart()
    announce(tx('角色配装检测已开启，会在后台自动记录每场任务。', 'Loadout detection is active and will archive every quest in the background.'), 'ok')
    await readHistory(true)
  } catch (error) {
    announce(tx(`开启角色配装检测失败：${errorMessage(error)}`, `Failed to start loadout detection: ${errorMessage(error)}`), 'danger')
  } finally {
    busy.value = ''
  }
}

async function stopDetector() {
  if (busy.value) return
  busy.value = 'toggle'
  try {
    detectorStatus.value = await RuntimeLoadoutDetectorStop()
    announce(tx('角色配装检测已关闭，本地记录已保留。', 'Loadout detection stopped; local records were retained.'), 'ok')
  } catch (error) {
    announce(tx(`关闭角色配装检测失败：${errorMessage(error)}`, `Failed to stop loadout detection: ${errorMessage(error)}`), 'danger')
  } finally {
    busy.value = ''
  }
}

function openPreview(record, member) {
  preview.value = { record, member }
  nextTick(() => detectorRoot.value?.closest('.tool-center-scroll,.workspace-scroll')?.scrollTo({ top: 0 }))
}

function closePreview() {
  preview.value = null
}

async function runAction(record, member, action) {
  if (busy.value) return
  const operationKey = `${record.id}:${member.role}`
  busy.value = operationKey
  const title = shareTitle(record, member)
  try {
    if (action === 'export') {
      const output = await RuntimeLoadoutDetectorExport(record.id, member.role, title)
      if (output) announce(tx(`配装已导出：${output}`, `Loadout exported: ${output}`), 'ok')
      return
    }
    if (action === 'publish') {
      const published = await RuntimeLoadoutDetectorPublish(record.id, member.role, title)
      await navigator.clipboard.writeText(published.url)
      announce(tx(`已上传并复制链接：${published.code}`, `Uploaded and copied link: ${published.code}`), 'ok')
      return
    }
    const result = await RuntimeLoadoutDetectorShare(record.id, member.role, title)
    if (action === 'deploy') {
      emit('deploy-loadout', {
        code: result.compatibilityCode,
        charaHash: member.loadout.characterHash,
        charaName: member.loadout.characterName,
      })
      announce(tx('已转到配装预设，可选择目标存档和槽位。', 'Opened loadout presets; choose a target save and slot.'), 'ok')
      return
    }
    await navigator.clipboard.writeText(result.compatibilityCode)
    announce(tx('完整配装码已复制。', 'Full loadout code copied.'), 'ok')
  } catch (error) {
    announce(tx(`配装操作失败：${errorMessage(error)}`, `Loadout action failed: ${errorMessage(error)}`), 'danger')
  } finally {
    busy.value = ''
  }
}

async function deleteRecord(record) {
  if (busy.value || !window.confirm(tx(`删除${recordTitle(record)}的本地记录？`, `Delete the local record for ${recordTitle(record)}?`))) return
  busy.value = `delete:${record.id}`
  try {
    await RuntimeLoadoutDetectorDelete(record.id)
    if (preview.value?.record?.id === record.id) preview.value = null
    await readStatus({ forceHistory: true })
    announce(tx('本地场次记录已删除。', 'Local quest record deleted.'), 'ok')
  } catch (error) {
    announce(tx(`删除失败：${errorMessage(error)}`, `Delete failed: ${errorMessage(error)}`), 'danger')
  } finally {
    busy.value = ''
  }
}

onMounted(async () => {
  stopStatusEvents = EventsOn(DETECTOR_STATUS_EVENT, next => { void acceptStatus(next) })
  await readStatus({ forceHistory: true })
})

onBeforeUnmount(() => {
  disposed = true
  stopStatusEvents()
  stopStatusEvents = () => {}
})
</script>

<template>
  <section ref="detectorRoot" class="loadout-detector" data-monitor-panel="party">
    <template v-if="!preview">
      <header class="detector-console ui-card ui-panel">
        <div class="detector-state">
          <span class="detector-pulse" :class="{ active: isRunning }" aria-hidden="true"><i></i></span>
          <div><small>{{ tx('只读后台服务', 'Read-Only Background Service') }}</small><h2>{{ tx('角色配装检测', 'Character Loadout Detection') }}</h2><p>{{ statusCopy[1] }}</p></div>
        </div>
        <div class="detector-control">
          <span class="ui-tag" :class="isRunning ? 'is-ok' : 'is-info'">{{ statusCopy[0] }}</span>
          <button v-if="!isRunning" type="button" class="ui-btn is-primary" :disabled="!!busy" @click="startDetector">{{ busy === 'toggle' ? tx('正在开启…', 'Starting…') : tx('开启后台检测', 'Start Detection') }}</button>
          <button v-else type="button" class="ui-btn is-ghost" :disabled="!!busy" @click="stopDetector">{{ busy === 'toggle' ? tx('正在关闭…', 'Stopping…') : tx('关闭检测', 'Stop Detection') }}</button>
        </div>
      </header>

      <dl class="detector-metrics">
        <div><dt>{{ tx('本次运行', 'This Session') }}</dt><dd>{{ detectorStatus.sessionCaptured || 0 }}<small>{{ tx('场', 'quests') }}</small></dd></div>
        <div><dt>{{ tx('本地总计', 'Local Total') }}</dt><dd>{{ detectorStatus.historyCount || 0 }}<small>{{ tx('场', 'quests') }}</small></dd></div>
        <div><dt>{{ tx('当前队伍', 'Current Party') }}</dt><dd>{{ detectorStatus.currentTeamSize || 0 }}<small>{{ tx('人', 'members') }}</small></dd></div>
        <div><dt>{{ tx('最近记录', 'Latest Capture') }}</dt><dd class="is-time">{{ timeText(detectorStatus.lastCaptureAt) }}</dd></div>
      </dl>

      <div v-if="isRunning && detectorStatus.lastError && detectorStatus.state !== 'waiting_game'" class="detector-observation ui-notice is-info">
        <strong>{{ tx('等待稳定场景', 'Waiting for a Stable Scene') }}</strong><span>{{ detectorStatus.lastError }}</span>
      </div>

      <section class="history-section">
        <header class="history-heading"><div><small>{{ tx('本机自动归档', 'Local Automatic Archive') }}</small><h3>{{ tx('任务配装记录', 'Quest Loadout History') }}</h3></div><span>{{ records.length }} {{ tx('场', 'quests') }}</span></header>
        <div v-if="historyLoading && !records.length" class="history-empty ui-empty">{{ tx('正在读取本地记录…', 'Reading local records…') }}</div>
        <div v-else-if="!records.length" class="history-empty ui-empty"><strong>{{ tx('还没有捕获到任务队伍', 'No Quest Party Captured Yet') }}</strong><span>{{ tx('开启检测后正常进入任务即可，页面可以切走。', 'Start detection and play normally; this page may remain in the background.') }}</span></div>
        <ol v-else class="quest-timeline">
          <li v-for="record in records" :key="record.id" class="quest-record">
            <div class="quest-index"><b>{{ record.sequence }}</b><i></i></div>
            <article class="quest-card ui-card">
              <header>
                <div><small>{{ timeText(record.capturedAt) }}</small><h4>{{ recordTitle(record) }}</h4><span>{{ record.members.length }} {{ tx('名角色配装', 'character loadouts') }}</span></div>
                <button type="button" class="record-delete" :aria-label="tx('删除本场记录', 'Delete quest record')" :title="tx('删除本场记录', 'Delete quest record')" :disabled="!!busy" @click="deleteRecord(record)">×</button>
              </header>
              <div class="member-strip">
                <button v-for="member in record.members" :key="member.role" type="button" class="member-entry" @click="openPreview(record, member)">
                  <img v-if="characterAssetIcon(member.characterHash)" :src="characterAssetIcon(member.characterHash)" alt="" />
                  <span v-else class="member-fallback">{{ member.characterName.slice(0, 1) }}</span>
                  <span><strong>{{ member.characterName }}</strong><small>{{ member.loadout.weapon.name }}</small><em>{{ member.loadout.sigils.length }}/12 {{ tx('因子', 'sigils') }}</em></span>
                  <b aria-hidden="true">›</b>
                </button>
              </div>
            </article>
          </li>
        </ol>
      </section>
    </template>

    <section v-else class="detector-preview">
      <header class="preview-bar ui-card">
        <button type="button" class="ui-btn is-ghost" @click="closePreview"><span aria-hidden="true">←</span> {{ tx('返回任务记录', 'Back to Quest History') }}</button>
        <div><small>{{ recordTitle(preview.record) }} · {{ timeText(preview.record.capturedAt) }}</small><strong>{{ preview.member.characterName }} · {{ preview.member.loadout.weapon.name }}</strong></div>
        <label><span>{{ tx('分享名称', 'Share Title') }}</span><input v-model="titles[titleKey(preview.record, preview.member)]" type="text" maxlength="80" :placeholder="`${preview.member.characterName} · ${tx('任务捕获配装', 'Quest Capture')}`" /></label>
      </header>
      <CapturedLoadoutPreview :loadout="preview.member.loadout" :source-label="tx('任务后台稳定捕获', 'Stable Background Quest Capture')">
        <template #actions>
          <button type="button" class="ui-btn is-sm" :disabled="!!busy" @click="runAction(preview.record, preview.member, 'copy')">{{ tx('复制配装码', 'Copy Code') }}</button>
          <button type="button" class="ui-btn is-sm" :disabled="!!busy" @click="runAction(preview.record, preview.member, 'export')">{{ tx('导出 JSON', 'Export JSON') }}</button>
          <button type="button" class="ui-btn is-primary is-sm" :disabled="!!busy" @click="runAction(preview.record, preview.member, 'publish')">{{ tx('上传并复制链接', 'Upload & Copy Link') }}</button>
          <button type="button" class="ui-btn is-sm" :disabled="!!busy" @click="runAction(preview.record, preview.member, 'deploy')">{{ tx('部署到存档', 'Deploy to Save') }}</button>
        </template>
      </CapturedLoadoutPreview>
    </section>
  </section>
</template>

<style scoped>
.loadout-detector { width:100%; min-width:0; display:grid; gap:var(--space-4); container:loadout-detector / inline-size; }
.detector-console { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-5); border-left:4px solid var(--accent); background:linear-gradient(100deg,var(--surface-card-pop),color-mix(in srgb,var(--accent-soft) 34%,var(--surface-card-pop))); }
.detector-state { min-width:0; display:flex; align-items:center; gap:var(--space-4); }
.detector-state > div { min-width:0; }
.detector-state small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.detector-state h2 { margin:2px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); letter-spacing:0; }
.detector-state p { margin:5px 0 0; color:var(--text-secondary); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.detector-pulse { position:relative; width:48px; height:48px; flex:0 0 48px; border:1px solid var(--border-strong); border-radius:50%; background:var(--surface-sunken); }
.detector-pulse::before,.detector-pulse::after { content:""; position:absolute; inset:9px; border:1px solid var(--text-muted); border-radius:50%; }
.detector-pulse::after { inset:17px; background:var(--text-muted); }
.detector-pulse i { position:absolute; inset:-1px; border:2px solid transparent; border-top-color:var(--accent); border-radius:50%; opacity:0; }
.detector-pulse.active { border-color:var(--success); background:var(--success-bg); }
.detector-pulse.active::before { border-color:var(--success); }
.detector-pulse.active::after { background:var(--success); box-shadow:0 0 12px var(--success); }
.detector-pulse.active i { opacity:1; animation:detector-scan 2.4s linear infinite; }
.detector-control { flex:0 0 auto; display:flex; align-items:center; gap:var(--space-3); }
.detector-control .ui-tag { min-width:7em; justify-content:center; }
@keyframes detector-scan { to { transform:rotate(360deg); } }

.detector-metrics { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:var(--space-3); margin:0; }
.detector-metrics > div { min-width:0; padding:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card-pop); box-shadow:var(--shadow-1); }
.detector-metrics dt { color:var(--text-muted); font-size:var(--fs-xs); }
.detector-metrics dd { margin:5px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); font-weight:var(--fw-bold); font-variant-numeric:tabular-nums; overflow-wrap:anywhere; }
.detector-metrics dd small { margin-left:5px; color:var(--text-muted); font-family:var(--font-body); font-size:var(--fs-xs); font-weight:var(--fw-normal); }
.detector-metrics dd.is-time { font-family:var(--font-data); font-size:var(--fs-sm); line-height:1.4; }
.detector-observation { display:flex; gap:var(--space-3); align-items:flex-start; }
.detector-observation span { min-width:0; overflow-wrap:anywhere; }

.history-section { min-width:0; display:grid; gap:var(--space-3); padding-top:var(--space-2); }
.history-heading { display:flex; align-items:end; justify-content:space-between; gap:var(--space-4); padding:0 var(--space-1); }
.history-heading small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.history-heading h3 { margin:2px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); }
.history-heading > span { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-sm); }
.history-empty { min-height:180px; }
.history-empty strong,.history-empty span { display:block; }
.history-empty span { margin-top:var(--space-2); }
.quest-timeline { display:grid; gap:var(--space-3); margin:0; padding:0; list-style:none; }
.quest-record { min-width:0; display:grid; grid-template-columns:42px minmax(0,1fr); gap:var(--space-3); }
.quest-index { position:relative; display:flex; justify-content:center; }
.quest-index b { position:relative; z-index:1; width:34px; height:34px; display:grid; place-items:center; border:1px solid var(--accent-border); border-radius:50%; background:var(--surface-card-pop); color:var(--accent-hover); font-family:var(--font-data); font-size:var(--fs-xs); }
.quest-index i { position:absolute; top:34px; bottom:calc(var(--space-3) * -1); width:1px; background:var(--border-default); }
.quest-record:last-child .quest-index i { display:none; }
.quest-card { min-width:0; padding:var(--space-4); border-radius:var(--radius-sm); }
.quest-card > header { min-width:0; display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-3); padding-bottom:var(--space-3); border-bottom:1px solid var(--border-soft); }
.quest-card > header > div { min-width:0; }
.quest-card header small,.quest-card header h4,.quest-card header span { display:block; }
.quest-card header small { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); }
.quest-card header h4 { margin:2px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-md); }
.quest-card header span { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.record-delete { width:32px; height:32px; flex:0 0 32px; display:grid; place-items:center; border:1px solid transparent; border-radius:50%; background:transparent; color:var(--text-muted); font-size:var(--fs-lg); cursor:pointer; }
.record-delete:hover { border-color:var(--danger); background:var(--danger-bg); color:var(--danger); }
.member-strip { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,220px),1fr)); gap:var(--space-2); margin-top:var(--space-3); }
.member-entry { min-width:0; min-height:68px; display:grid; grid-template-columns:44px minmax(0,1fr) auto; align-items:center; gap:var(--space-3); padding:var(--space-2); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); color:inherit; text-align:left; cursor:pointer; }
.member-entry:hover { border-color:var(--accent-border); background:var(--accent-soft); }
.member-entry img,.member-fallback { width:44px; height:44px; display:grid; place-items:center; overflow:hidden; border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card); object-fit:cover; }
.member-entry > span:nth-child(2) { min-width:0; }
.member-entry strong,.member-entry small,.member-entry em { display:block; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.member-entry strong { color:var(--text-primary); font-size:var(--fs-sm); }
.member-entry small { margin-top:2px; color:var(--text-secondary); font-size:var(--fs-xs); }
.member-entry em { margin-top:2px; color:var(--accent); font-size:var(--fs-2xs); font-style:normal; }
.member-entry > b { color:var(--accent-hover); font-size:var(--fs-lg); }

.detector-preview { width:100%; min-width:0; display:grid; gap:var(--space-4); }
.preview-bar { position:sticky; z-index:10; top:0; min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr) minmax(220px,320px); align-items:center; gap:var(--space-4); padding:var(--space-4); border-radius:var(--radius-sm); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }
.preview-bar > div { min-width:0; }
.preview-bar small,.preview-bar strong { display:block; min-width:0; overflow-wrap:anywhere; }
.preview-bar small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.preview-bar strong { margin-top:2px; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); }
.preview-bar label { min-width:0; display:grid; gap:3px; }
.preview-bar label span { color:var(--text-muted); font-size:var(--fs-2xs); font-weight:var(--fw-semibold); }
.preview-bar input { width:100%; min-width:0; min-height:34px; padding:0 var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-sunken); color:var(--text-primary); }

@container loadout-detector (max-width:860px) {
  .detector-console { align-items:stretch; flex-direction:column; }
  .detector-control { justify-content:space-between; }
  .detector-metrics { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .preview-bar { grid-template-columns:minmax(0,1fr); }
  .preview-bar .ui-btn,.preview-bar label { width:100%; }
}
@container loadout-detector (max-width:520px) {
  .detector-state { align-items:flex-start; }
  .detector-control { align-items:stretch; flex-direction:column; }
  .detector-control .ui-btn { width:100%; }
  .detector-metrics { grid-template-columns:minmax(0,1fr); }
  .quest-record { grid-template-columns:30px minmax(0,1fr); gap:var(--space-2); }
  .quest-index b { width:28px; height:28px; }
  .quest-index i { top:28px; }
  .member-strip { grid-template-columns:minmax(0,1fr); }
}
@media (prefers-reduced-motion:reduce) { .detector-pulse.active i { animation:none; } }
</style>
