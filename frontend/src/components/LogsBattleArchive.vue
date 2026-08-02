<script setup>
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref } from 'vue'
import {
  CloseLogsBattleArchive,
  LogsBattleArchiveDetail,
  LogsBattleArchiveFromCurrent,
  LogsBattleArchivePage,
  RuntimeDamageCaptureSnapshot,
  RuntimeDamageCaptureStart,
  RuntimeDamageCaptureStop,
  SelectLogsBattleArchive,
} from '../../wailsjs/go/backend/App'
import { characterAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'
import CapturedLoadoutPreview from './CapturedLoadoutPreview.vue'

const emit = defineEmits(['status'])
const connected = ref(false)
const loading = ref(false)
const search = ref('')
const items = ref([])
const nextCursorTime = ref(0)
const nextCursorID = ref(0)
const hasMore = ref(false)
const skippedUnsupported = ref(0)
const detail = ref(null)
const selectedPlayerIndex = ref(0)
const sourceMode = ref('live')
const liveSnapshot = ref({ active: false, events: [], skills: [], totalEvents: 0, droppedEvents: 0 })
const liveBusy = ref(false)
let archiveGeneration = 0
let liveTimer = 0
let componentActive = true

const tx = (zh, en) => language.value === 'en' ? en : zh
const selectedPlayer = computed(() => detail.value?.players?.[selectedPlayerIndex.value] || null)
const liveSourceOrdinals = computed(() => {
  const result = new Map()
  for (const skill of liveSnapshot.value?.skills || []) {
    const key = String(skill?.sourceAddress || '0')
    if (!result.has(key)) result.set(key, result.size + 1)
  }
  return result
})
function formatNumber(value, digits = 0) {
  const number = Number(value)
  return Number.isFinite(number) ? number.toLocaleString(language.value === 'en' ? 'en-US' : 'zh-CN', { maximumFractionDigits: digits }) : tx('未记录', 'Not Captured')
}
function formatTime(value) { return new Date(Number(value)).toLocaleString(language.value === 'en' ? 'en-US' : 'zh-CN') }
function formatDuration(value) {
  const seconds = Math.max(0, Number(value) || 0) / 1000
  return `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`
}
function characterIcon(player) { return characterAssetIcon(player?.characterHash) }
function timelineHeight(values, value) {
  const max = Math.max(1, ...(values || []))
  return `${Math.max(3, Math.round(Number(value || 0) / max * 100))}%`
}
function actionTypeLabel(value) {
  return ({ normal: tx('普通/能力', 'Normal / Ability'), sba: tx('奥义', 'SBA'), link: tx('连锁/Link', 'Link'), supplementary: tx('追击', 'Supplementary') })[value] || value
}
function actionLabel(skill) {
  const id = Number(skill?.actionId)
  const suffix = Number.isFinite(id) ? ` #${id === 0xFFFFFFFF ? '-1' : id}` : ''
  const source = liveSourceOrdinals.value.get(String(skill?.sourceAddress || '0')) || 1
  return `${tx('来源', 'Source')} ${source} · ${actionTypeLabel(skill?.actionType)}${suffix}`
}
function stopLivePolling() {
  if (liveTimer) window.clearTimeout(liveTimer)
  liveTimer = 0
}
function scheduleLivePolling() {
  stopLivePolling()
  if (!componentActive) return
  if (!liveSnapshot.value?.active || sourceMode.value !== 'live') return
  liveTimer = window.setTimeout(refreshLiveCapture, 500)
}
async function refreshLiveCapture() {
  if (liveBusy.value) return
  liveBusy.value = true
  try {
    liveSnapshot.value = await RuntimeDamageCaptureSnapshot(1024)
  } catch (error) {
    emit('status', String(error), 'error')
  } finally {
    liveBusy.value = false
    scheduleLivePolling()
  }
}
async function startLiveCapture() {
  if (liveBusy.value) return
  liveBusy.value = true
  try {
    liveSnapshot.value = await RuntimeDamageCaptureStart()
    emit('status', tx('当前会话伤害采集已开启', 'Current-session damage capture started'), 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally {
    liveBusy.value = false
    scheduleLivePolling()
  }
}
async function stopLiveCapture() {
  if (liveBusy.value) return
  liveBusy.value = true
  stopLivePolling()
  try {
    await RuntimeDamageCaptureStop()
    liveSnapshot.value = { ...liveSnapshot.value, active: false }
    emit('status', tx('当前会话伤害采集已停止，Hook 已恢复', 'Current-session damage capture stopped and its hook was restored'), 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally {
    liveBusy.value = false
  }
}
async function selectDatabase() {
  if (loading.value) return
  loading.value = true
  try {
    const page = await SelectLogsBattleArchive({ cursorTime: 0, cursorId: 0, limit: 40, search: search.value.trim() })
    if (!page) return
    items.value = page.items || []
    nextCursorTime.value = page.nextCursorTime || 0
    nextCursorID.value = page.nextCursorId || 0
    hasMore.value = !!page.hasMore
    skippedUnsupported.value = page.skippedUnsupported || 0
    connected.value = true
    detail.value = null
    emit('status', tx(`已载入 ${items.value.length} 场战斗记录`, `${items.value.length} battles loaded`), 'success')
  } catch (error) { emit('status', String(error), 'error') } finally { loading.value = false }
}
async function reuseCurrentDatabase() {
  if (loading.value) return
  loading.value = true
  try {
    const page = await LogsBattleArchiveFromCurrent({ cursorTime: 0, cursorId: 0, limit: 40, search: search.value.trim() })
    items.value = page.items || []
    nextCursorTime.value = page.nextCursorTime || 0
    nextCursorID.value = page.nextCursorId || 0
    hasMore.value = !!page.hasMore
    skippedUnsupported.value = page.skippedUnsupported || 0
    connected.value = true
    detail.value = null
  } catch (error) { emit('status', String(error), 'error') } finally { loading.value = false }
}
async function reload() {
  if (!connected.value || loading.value) return
  loading.value = true
  try {
    const page = await LogsBattleArchivePage({ cursorTime: 0, cursorId: 0, limit: 40, search: search.value.trim() })
    items.value = page.items || []
    nextCursorTime.value = page.nextCursorTime || 0
    nextCursorID.value = page.nextCursorId || 0
    hasMore.value = !!page.hasMore
    skippedUnsupported.value = page.skippedUnsupported || 0
    detail.value = null
  } catch (error) { emit('status', String(error), 'error') } finally { loading.value = false }
}
async function loadMore() {
  if (!connected.value || !hasMore.value || loading.value) return
  loading.value = true
  try {
    const page = await LogsBattleArchivePage({ cursorTime: nextCursorTime.value, cursorId: nextCursorID.value, limit: 40, search: search.value.trim() })
    items.value = [...items.value, ...(page.items || [])]
    nextCursorTime.value = page.nextCursorTime || 0
    nextCursorID.value = page.nextCursorId || 0
    hasMore.value = !!page.hasMore
  } catch (error) { emit('status', String(error), 'error') } finally { loading.value = false }
}
async function openDetail(item) {
  if (loading.value) return
  loading.value = true
  try {
    detail.value = await LogsBattleArchiveDetail(item.id)
    selectedPlayerIndex.value = 0
  } catch (error) { emit('status', String(error), 'error') } finally { loading.value = false }
}
function closeDetail() { detail.value = null }
function clearArchiveState() {
  connected.value = false
  items.value = []
  detail.value = null
  nextCursorTime.value = 0
  nextCursorID.value = 0
  hasMore.value = false
  skippedUnsupported.value = 0
}
async function disconnect() {
  if (loading.value) return
  const generation = ++archiveGeneration
  loading.value = true
  try {
    await CloseLogsBattleArchive()
    if (generation !== archiveGeneration) return
    clearArchiveState()
  } catch (error) {
    if (generation === archiveGeneration) emit('status', String(error), 'error')
  } finally {
    if (generation === archiveGeneration) loading.value = false
  }
}
onMounted(async () => {
  try {
    liveSnapshot.value = await RuntimeDamageCaptureSnapshot(1024)
    scheduleLivePolling()
  } catch {}
})
function deactivateLivePolling() {
  componentActive = false
  stopLivePolling()
}
function activateLivePolling() {
  componentActive = true
  scheduleLivePolling()
}
onDeactivated(deactivateLivePolling)
onActivated(activateLivePolling)
onBeforeUnmount(deactivateLivePolling)
</script>

<template>
  <section class="battle-archive" :aria-label="tx('战斗档案', 'Battle Archive')">
    <header class="archive-heading"><div><small>{{ sourceMode === 'live' ? tx('内置采集 · 当前游戏会话', 'Built-In Capture · Current Game Session') : tx('Relink Logs · 本机只读', 'Relink Logs · Local Read-Only') }}</small><h2>{{ tx('战斗档案', 'Battle Archive') }}</h2><p>{{ sourceMode === 'live' ? tx('直接记录当前游戏进程中的伤害事件、动作编号、伤害上限和上限前伤害；停止时会恢复 Hook。当前尚未完成玩家实体归属，统计代表 Hook 捕获的全局事件流。', 'Capture current-process damage events, action IDs, damage caps, and pre-cap damage; stopping restores the hook. Player-entity attribution is not yet verified, so totals represent the global event stream seen by the hook.') : tx('读取 Logs 已保存的场次、队伍伤害、技能明细和配装快照；不会注入游戏，也不会修改数据库。', 'Read saved encounters, party damage, skill breakdowns, and loadout snapshots without game injection or database writes.') }}</p></div><span :class="{ connected: sourceMode === 'live' ? liveSnapshot.active : connected }"><i></i>{{ sourceMode === 'live' ? (liveSnapshot.active ? tx('实时采集中', 'Live Capture Active') : tx('实时采集未开启', 'Live Capture Stopped')) : (connected ? tx('数据库已连接', 'Database Connected') : tx('尚未选择数据库', 'No Database Selected')) }}</span></header>

    <nav class="archive-source-tabs" :aria-label="tx('战斗数据来源', 'Battle Data Source')"><button type="button" class="ui-btn" :class="{ 'is-primary': sourceMode === 'live' }" @click="sourceMode = 'live'; scheduleLivePolling()">{{ tx('当前会话', 'Current Session') }}</button><button type="button" class="ui-btn" :class="{ 'is-primary': sourceMode === 'logs' }" @click="sourceMode = 'logs'; stopLivePolling()">{{ tx('Logs 历史', 'Logs History') }}</button></nav>

    <template v-if="sourceMode === 'live'">
      <div class="live-toolbar ui-card is-flat"><div><strong>{{ tx('游戏内实时伤害事件', 'In-Game Live Damage Events') }}</strong><small>{{ liveSnapshot.detail || tx('固定 4096 条环形缓冲区，不阻塞游戏线程。', 'A fixed 4096-event ring buffer never blocks the game thread.') }}</small></div><button v-if="!liveSnapshot.active" type="button" class="ui-btn is-primary" :disabled="liveBusy" @click="startLiveCapture">{{ liveBusy ? tx('正在连接…', 'Connecting…') : tx('开始采集', 'Start Capture') }}</button><button v-else type="button" class="ui-btn" :disabled="liveBusy" @click="refreshLiveCapture">{{ liveBusy ? tx('读取中…', 'Reading…') : tx('立即刷新', 'Refresh Now') }}</button><button v-if="liveSnapshot.active" type="button" class="ui-btn is-ghost" :disabled="liveBusy" @click="stopLiveCapture">{{ tx('停止并恢复 Hook', 'Stop and Restore Hook') }}</button></div>
      <p v-if="liveSnapshot.active && liveSnapshot.scope === 'global-unattributed'" class="ui-notice is-warning">{{ tx('当前版本无法可靠区分本机、队友与敌方来源；下方伤害和 DPS 仅用于验证事件采集，不代表个人战绩。', 'This version cannot reliably distinguish local-player, teammate, and enemy sources. Damage and DPS below validate event capture only and are not personal performance figures.') }}</p>
      <p v-if="liveSnapshot.droppedEvents" class="ui-notice is-warning">{{ tx(`缓冲区已覆盖 ${formatNumber(liveSnapshot.droppedEvents)} 条较早事件；当前统计基于最近 ${formatNumber(liveSnapshot.events?.length || 0)} 条。`, `${formatNumber(liveSnapshot.droppedEvents)} older events were overwritten; current totals use the latest ${formatNumber(liveSnapshot.events?.length || 0)} events.`) }}</p>
      <section v-if="liveSnapshot.active" class="live-overview ui-card is-flat"><dl><div><dt>{{ tx('当前窗口观测伤害', 'Observed Window Damage') }}</dt><dd>{{ formatNumber(liveSnapshot.totalDamage) }}</dd></div><div><dt>{{ tx('窗口总 DPS', 'Observed DPS') }}</dt><dd>{{ formatNumber(liveSnapshot.dps) }}</dd></div><div><dt>{{ tx('事件总数', 'Total Events') }}</dt><dd>{{ formatNumber(liveSnapshot.totalEvents) }}</dd></div><div><dt>{{ tx('窗口时长', 'Window Duration') }}</dt><dd>{{ formatDuration(liveSnapshot.durationMillis) }}</dd></div></dl></section>
      <div v-if="liveSnapshot.active" class="skill-table ui-card is-flat"><header><small>{{ tx('当前会话动作明细', 'Current-Session Action Breakdown') }}</small><span>{{ tx('动作已按来源实例分组；玩家、宠物与召唤物的归属仍待实战标定，当前不冒充个人 DPS。能力名称暂保留游戏原始动作编号。', 'Actions are separated by source instance. Player, pet, and summon attribution still needs field calibration, so these totals are not presented as personal DPS. Raw action IDs are retained for now.') }}</span></header><div class="skill-row skill-head"><span>{{ tx('来源与动作', 'Source & Action') }}</span><span>{{ tx('命中', 'Hits') }}</span><span>{{ tx('伤害', 'Damage') }}</span><span>{{ tx('范围', 'Range') }}</span><span>{{ tx('封顶', 'Capped') }}</span></div><div v-for="skill in liveSnapshot.skills || []" :key="skill.key" class="skill-row"><span><b>{{ actionLabel(skill) }}</b><small>{{ skill.actionType }}:{{ skill.actionId }}</small></span><span>{{ skill.hits }}</span><span>{{ formatNumber(skill.damage) }}</span><span>{{ formatNumber(skill.minDamage) }} – {{ formatNumber(skill.maxDamage) }}</span><span>{{ skill.cappableHits ? `${skill.cappedHits}/${skill.cappableHits}` : tx('不受上限', 'Uncapped') }}<small v-if="skill.overcapPercent">{{ tx('平均溢出', 'Avg Overcap') }} {{ formatNumber(skill.overcapPercent, 1) }}%</small></span></div><p v-if="!liveSnapshot.skills?.length" class="ui-empty">{{ tx('采集已开启。任务中出现伤害事件后，这里会自动显示按来源分组的记录。', 'Capture is active. Source-grouped records appear after damage events occur in a quest.') }}</p></div>
      <p v-else class="archive-empty ui-empty">{{ tx('点击“开始采集”后可继续正常游戏；采集会在后台持续运行，直到手动停止或退出应用。', 'Start capture, then keep playing normally. It continues in the background until stopped or the app exits.') }}</p>
    </template>

    <template v-else-if="detail">
      <button type="button" class="archive-back ui-btn" @click="closeDetail">← {{ tx('返回场次列表', 'Back to Battles') }}</button>
      <section class="battle-overview ui-card is-flat">
        <div><small>{{ formatTime(detail.summary.time) }}</small><strong>{{ detail.summary.questName }}</strong><span>{{ formatDuration(detail.summary.duration) }} · {{ detail.summary.completed === true ? tx('已完成', 'Completed') : detail.summary.completed === false ? tx('未完成', 'Incomplete') : tx('结果未记录', 'Result Not Captured') }}</span></div>
        <dl><div><dt>{{ tx('队伍伤害', 'Party Damage') }}</dt><dd>{{ formatNumber(detail.summary.totalDamage) }}</dd></div><div><dt>DPS</dt><dd>{{ formatNumber(detail.summary.dps) }}</dd></div><div><dt>{{ tx('事件识别', 'Events') }}</dt><dd>{{ detail.recognizedEvents }} / {{ detail.eventCount }}</dd></div></dl>
      </section>
      <div v-if="detail.missingFields?.length || detail.decodeWarnings?.length" class="archive-warnings ui-notice is-warning"><strong>{{ tx('来源说明', 'Source Notes') }}</strong><span v-for="item in detail.missingFields || []" :key="item">{{ item }}：{{ tx('未记录', 'Not Captured') }}</span><span v-for="item in detail.decodeWarnings || []" :key="item">{{ item }}</span></div>
      <nav class="player-tabs" :aria-label="tx('队伍成员', 'Party Members')"><button v-for="(player, index) in detail.players" :key="`${player.slot}-${player.playerName}`" type="button" :class="{ active: selectedPlayerIndex === index }" @click="selectedPlayerIndex = index"><img v-if="characterIcon(player)" :src="characterIcon(player)" alt="" /><span><small>{{ player.characterName }}</small><strong>{{ player.playerName || tx('未记录玩家名', 'Player Name Not Captured') }}</strong></span><em>{{ formatNumber(player.damage) }}</em></button></nav>
      <section v-if="selectedPlayer" class="player-detail">
        <div class="player-meter ui-card is-flat"><dl><div><dt>{{ tx('总伤害', 'Damage') }}</dt><dd>{{ formatNumber(selectedPlayer.damage) }}</dd></div><div><dt>DPS</dt><dd>{{ formatNumber(selectedPlayer.dps) }}</dd></div><div><dt>{{ tx('队伍占比', 'Party Share') }}</dt><dd>{{ formatNumber(selectedPlayer.percentage, 1) }}%</dd></div><div><dt>{{ tx('技能条目', 'Skill Rows') }}</dt><dd>{{ selectedPlayer.skills?.length || 0 }}</dd></div></dl><div v-if="selectedPlayer.timeline?.length" class="damage-timeline" :aria-label="tx('伤害时间线', 'Damage Timeline')"><i v-for="(value, index) in selectedPlayer.timeline" :key="index" :style="{ height: timelineHeight(selectedPlayer.timeline, value) }" :title="formatNumber(value)"></i></div></div>
        <div class="skill-table ui-card is-flat"><header><small>{{ tx('技能伤害明细', 'Skill Damage Breakdown') }}</small><span>{{ tx('封顶命中只在日志保存了伤害上限和上限前基础值时统计。', 'Capped hits require both saved cap and pre-cap base values.') }}</span></header><div class="skill-row skill-head"><span>{{ tx('技能', 'Skill') }}</span><span>{{ tx('命中', 'Hits') }}</span><span>{{ tx('伤害', 'Damage') }}</span><span>{{ tx('占比', 'Share') }}</span><span>{{ tx('封顶', 'Capped') }}</span></div><div v-for="skill in selectedPlayer.skills || []" :key="skill.key" class="skill-row"><span><b>{{ skill.name }}</b><small>{{ formatNumber(skill.minDamage) }} – {{ formatNumber(skill.maxDamage) }}</small></span><span>{{ skill.hits }}</span><span>{{ formatNumber(skill.damage) }}</span><span>{{ formatNumber(skill.percentage, 1) }}%</span><span>{{ skill.cappableHits ? `${skill.cappedHits}/${skill.cappableHits}` : tx('未记录', 'Not Captured') }}<small v-if="skill.overcapPercent">{{ formatNumber(skill.overcapPercent, 1) }}%</small></span></div><p v-if="!selectedPlayer.skills?.length" class="ui-empty">{{ tx('该记录没有可识别的伤害事件。', 'No recognizable damage events were stored for this player.') }}</p></div>
        <section class="legality-panel ui-card is-flat">
          <header><div><strong>{{ tx('GBFR Logs 配装检测', 'GBFR Logs Build Check') }}</strong><small>{{ tx('读取 Logs 1.12.6 已写入数据库的检测结果；低概率提示不等同于作弊证明。', 'Reads findings stored by GBFR Logs 1.12.6. An improbability notice is not proof of cheating.') }}</small></div><span class="ui-tag" :class="selectedPlayer.legalityFindings?.length ? 'is-warn' : 'is-ok'">{{ selectedPlayer.legalityFindings?.length ? tx(`${selectedPlayer.legalityFindings.length} 条提示`, `${selectedPlayer.legalityFindings.length} findings`) : tx('没有已保存提示', 'No Stored Findings') }}</span></header>
          <p class="legality-scope">{{ tx('检测范围只有 12 个物理因子槽、祝福石、上限突破、召唤石与专精节点数量。武器自带技能和本工具运行时虚拟因子不在这套规则输入中。', 'Scope is limited to the 12 physical sigil slots, wrightstone, overmasteries, summons, and master-trait count. Weapon innate skills and this tool’s runtime virtual sigils are not inputs to these rules.') }}</p>
          <div v-for="finding in selectedPlayer.legalityFindings || []" :key="`${finding.rule}-${finding.detail}`" class="legality-row" :class="finding.hardBreach ? 'is-hard' : 'is-odds'"><strong>{{ finding.label }}</strong><span>{{ finding.detail }}</span><small v-if="finding.odds != null">{{ tx('记录概率', 'Recorded odds') }}：{{ Number(finding.odds).toExponential(3) }}</small></div>
        </section>
        <CapturedLoadoutPreview v-if="selectedPlayer.loadout" :loadout="selectedPlayer.loadout" :source-label="tx('本场战斗配装快照', 'Battle Loadout Snapshot')" />
        <div v-else class="ui-empty">{{ tx('该场次没有可预览的完整配装快照。', 'This battle has no complete loadout snapshot to preview.') }}</div>
      </section>
    </template>

    <template v-else>
      <div class="archive-toolbar ui-card is-flat"><label><span>{{ tx('搜索角色、玩家或任务 ID', 'Search character, player, or quest ID') }}</span><input v-model="search" class="ui-input" :disabled="loading" :placeholder="tx('例如：泽塔 / 玩家名 / 123456', 'For example: Zeta / player / 123456')" @keyup.enter="reload" /></label><button v-if="connected" type="button" class="ui-btn" :disabled="loading" @click="reload">{{ tx('搜索', 'Search') }}</button><button v-if="!connected" type="button" class="ui-btn is-primary" :disabled="loading" @click="reuseCurrentDatabase">{{ tx('使用已选 Logs 数据库', 'Use Selected Logs Database') }}</button><button type="button" class="ui-btn is-primary" :disabled="loading" @click="selectDatabase">{{ connected ? tx('更换数据库', 'Change Database') : tx('选择 logs.db', 'Choose logs.db') }}</button><button v-if="connected" type="button" class="ui-btn is-ghost" :disabled="loading" @click="disconnect">{{ tx('断开', 'Disconnect') }}</button></div>
      <p v-if="skippedUnsupported" class="ui-notice is-warning">{{ tx(`已跳过 ${skippedUnsupported} 场非 v1 协议记录；当前只展示能够完整解码的 v1 场次。`, `${skippedUnsupported} non-v1 battles were skipped. Only fully supported v1 records are shown.`) }}</p>
      <p v-if="!connected" class="archive-empty ui-empty">{{ tx('选择 Relink Logs、GBFR Logs 或兼容分支生成的 logs.db。数据库会以只读方式打开。', 'Choose logs.db from Relink Logs, GBFR Logs, or a compatible fork. The database is opened read-only.') }}</p>
      <p v-else-if="!items.length && !loading" class="archive-empty ui-empty">{{ tx('没有符合筛选条件的场次。', 'No battles match the current filter.') }}</p>
      <div v-else class="battle-list"><button v-for="item in items" :key="item.id" type="button" class="battle-row ui-card is-flat" @click="openDetail(item)"><span class="battle-date"><small>{{ formatTime(item.time) }}</small><strong>{{ item.questName }}</strong><em>{{ item.playerNames?.join(' · ') || item.characterTypes?.join(' · ') || tx('队伍未记录', 'Party Not Captured') }}</em><mark v-if="item.legalityFindingCount">{{ tx(`${item.legalityFindingCount} 条配装提示`, `${item.legalityFindingCount} build findings`) }}</mark></span><span><small>{{ tx('时长', 'Duration') }}</small><b>{{ formatDuration(item.duration) }}</b></span><span><small>{{ tx('总伤害', 'Damage') }}</small><b>{{ formatNumber(item.totalDamage) }}</b></span><span><small>DPS</small><b>{{ formatNumber(item.dps) }}</b></span><i aria-hidden="true">→</i></button></div>
      <button v-if="hasMore" type="button" class="load-more ui-btn" :disabled="loading" @click="loadMore">{{ loading ? tx('正在读取…', 'Loading…') : tx('加载更早场次', 'Load Earlier Battles') }}</button>
    </template>
  </section>
</template>

<style scoped>
.battle-archive { min-width:0; display:grid; gap:var(--space-4); container:battle / inline-size; }
.archive-heading { min-width:0; display:flex; align-items:end; justify-content:space-between; gap:var(--space-4); padding:2px var(--space-2); border-bottom:1px solid var(--border-soft); }
.archive-heading > div { min-width:0; }
.archive-heading small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.archive-heading h2 { margin:2px 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); letter-spacing:0; }
.archive-heading p { margin:0 0 var(--space-3); color:var(--text-muted); font-size:var(--fs-sm); }
.archive-heading > span { flex:0 0 auto; display:flex; align-items:center; gap:6px; padding:var(--space-2) var(--space-3); border-left:2px solid var(--border-strong); color:var(--text-muted); font-size:var(--fs-xs); }
.archive-heading > span i { width:7px; height:7px; border-radius:50%; background:var(--text-muted); }
.archive-heading > span.connected { border-left-color:var(--success); color:var(--success-ink); }
.archive-heading > span.connected i { background:var(--success); }
.archive-source-tabs { display:flex; gap:var(--space-2); }
.archive-source-tabs .ui-btn { min-width:8rem; }
.live-toolbar { min-width:0; display:flex; align-items:center; gap:var(--space-2); padding:var(--space-3); }
.live-toolbar > div { min-width:0; flex:1 1 auto; display:grid; gap:2px; }
.live-toolbar strong { color:var(--text-primary); }
.live-toolbar small { color:var(--text-muted); overflow-wrap:anywhere; }
.live-overview { padding:var(--space-3); }
.live-overview dl { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:var(--space-3); margin:0; }
.live-overview dl > div { min-width:0; display:grid; gap:2px; }
.live-overview dt { color:var(--text-muted); font-size:var(--fs-2xs); }
.live-overview dd { margin:0; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-md); overflow-wrap:anywhere; }
.archive-toolbar { min-width:0; display:grid; grid-template-columns:minmax(240px,1fr) auto auto auto; gap:var(--space-2); align-items:end; padding:var(--space-3); }
.archive-toolbar label { min-width:0; display:grid; gap:4px; color:var(--text-muted); font-size:var(--fs-xs); }
.archive-empty { min-height:180px; }
.battle-list { min-width:0; display:grid; gap:6px; }
.battle-row { width:100%; min-width:0; display:grid; grid-template-columns:minmax(220px,1.6fr) repeat(3,minmax(100px,.5fr)) 20px; gap:var(--space-3); align-items:center; padding:var(--space-3) var(--space-4); border-left:3px solid var(--border-strong); color:inherit; text-align:left; cursor:pointer; }
.battle-row:hover { border-left-color:var(--accent); background:var(--accent-soft); }
.battle-row > span { min-width:0; display:grid; gap:1px; }
.battle-row small,.battle-row em { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-muted); font-size:var(--fs-2xs); font-style:normal; white-space:nowrap; }
.battle-row strong,.battle-row b { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); font-size:var(--fs-sm); white-space:nowrap; }
.battle-row i { color:var(--accent); font-style:normal; }
.load-more { justify-self:center; }
.archive-back { justify-self:start; }
.battle-overview { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-4); align-items:center; padding:var(--space-4); border-left:3px solid var(--accent); }
.battle-overview > div { min-width:0; display:grid; gap:2px; }
.battle-overview small,.battle-overview span { color:var(--text-muted); font-size:var(--fs-xs); }
.battle-overview strong { color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); }
.battle-overview dl,.player-meter dl { min-width:0; display:flex; gap:var(--space-4); margin:0; }
.battle-overview dl div,.player-meter dl div { min-width:90px; display:grid; }
.battle-overview dt,.player-meter dt { color:var(--text-muted); font-size:var(--fs-2xs); }
.battle-overview dd,.player-meter dd { margin:0; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-md); }
.archive-warnings { align-items:start; }
.player-tabs { min-width:0; display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:var(--space-2); }
.player-tabs button { min-width:0; display:grid; grid-template-columns:42px minmax(0,1fr) auto; gap:var(--space-2); align-items:center; padding:var(--space-2); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card); color:inherit; text-align:left; cursor:pointer; }
.player-tabs button.active { border-color:var(--accent); background:var(--accent-soft); }
.player-tabs img { width:42px; height:42px; border-radius:var(--radius-sm); object-fit:cover; }
.player-tabs span { min-width:0; display:grid; }
.player-tabs small,.player-tabs strong { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.player-tabs small { color:var(--text-muted); font-size:var(--fs-2xs); }
.player-tabs strong { color:var(--text-primary); font-size:var(--fs-xs); }
.player-tabs em { color:var(--accent); font-size:var(--fs-2xs); font-style:normal; }
.player-detail { min-width:0; display:grid; gap:var(--space-3); }
.legality-panel { min-width:0; display:grid; gap:var(--space-2); padding:var(--space-3); border-left:3px solid var(--border-strong); }
.legality-panel > header { display:flex; align-items:start; justify-content:space-between; gap:var(--space-3); }
.legality-panel > header > div { min-width:0; display:grid; gap:2px; }
.legality-panel > header strong { color:var(--text-primary); }
.legality-panel > header small,.legality-scope,.legality-row span,.legality-row small { color:var(--text-muted); font-size:var(--fs-xs); }
.legality-scope { margin:0; padding:var(--space-2); background:var(--surface-muted); }
.legality-row { display:grid; gap:2px; padding:var(--space-2) var(--space-3); border-left:3px solid var(--warning); background:var(--warning-soft); }
.legality-row.is-hard { border-left-color:var(--danger); background:var(--danger-soft); }
.legality-row strong { color:var(--text-primary); font-size:var(--fs-sm); }
.battle-date mark { justify-self:start; padding:2px 6px; border-radius:var(--radius-xs); background:var(--warning-soft); color:var(--warning-ink); font-size:var(--fs-2xs); }
.player-meter { min-width:0; display:grid; grid-template-columns:auto minmax(220px,1fr); gap:var(--space-4); align-items:end; padding:var(--space-3); }
.damage-timeline { height:64px; min-width:0; display:flex; align-items:end; gap:2px; border-bottom:1px solid var(--border-strong); }
.damage-timeline i { flex:1 1 2px; min-width:2px; max-width:12px; background:linear-gradient(to top,var(--accent),var(--accent-hover)); opacity:.72; }
.skill-table { min-width:0; overflow:auto; padding:var(--space-3); }
.skill-table > header { min-width:0; display:flex; justify-content:space-between; gap:var(--space-3); padding:0 var(--space-2) var(--space-2); }
.skill-table > header small { color:var(--accent); font-weight:var(--fw-bold); }
.skill-table > header span { color:var(--text-muted); font-size:var(--fs-2xs); }
.skill-row { min-width:660px; display:grid; grid-template-columns:minmax(180px,1.4fr) repeat(4,minmax(90px,.5fr)); gap:var(--space-2); align-items:center; padding:7px var(--space-2); border-top:1px solid var(--border-soft); color:var(--text-secondary); font-size:var(--fs-xs); }
.skill-row > span { min-width:0; display:grid; }
.skill-row b { color:var(--text-primary); }
.skill-row small { color:var(--text-muted); font-size:var(--fs-2xs); }
.skill-head { color:var(--text-muted); font-size:var(--fs-2xs); font-weight:var(--fw-bold); }
@container battle (max-width:850px) { .archive-toolbar { grid-template-columns:minmax(0,1fr) repeat(3,auto); } .archive-toolbar label { grid-column:1/-1; } .battle-row { grid-template-columns:minmax(0,1.6fr) repeat(2,minmax(82px,.5fr)) 18px; } .battle-row > span:nth-child(4) { display:none; } .player-tabs { grid-template-columns:repeat(2,minmax(0,1fr)); } .player-meter { grid-template-columns:minmax(0,1fr); } .live-overview dl { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@container battle (max-width:520px) { .archive-heading { align-items:stretch; flex-direction:column; } .archive-heading > span { align-self:start; } .battle-overview { align-items:start; grid-template-columns:minmax(0,1fr); } .archive-source-tabs,.live-toolbar { align-items:stretch; flex-direction:column; } .archive-source-tabs .ui-btn,.live-toolbar .ui-btn { width:100%; } .archive-toolbar { grid-template-columns:repeat(2,minmax(0,1fr)); } .archive-toolbar label { grid-column:1/-1; } .battle-row { grid-template-columns:minmax(0,1fr) 76px 18px; } .battle-row > span:nth-child(3),.battle-row > span:nth-child(4) { display:none; } .player-tabs { grid-template-columns:minmax(0,1fr); } .battle-overview dl,.player-meter dl,.live-overview dl { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); } }
</style>
