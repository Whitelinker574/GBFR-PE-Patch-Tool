<script setup>
import { onBeforeUnmount, reactive, ref } from 'vue'
import { CharaAcquire, CharaRelease, MonsterEnhanceGetStatusOwned, MonsterEnhanceSetPatchValueEnabledOwned, DamageMeterGetStatus } from '../../wailsjs/go/backend/App'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'

const emit = defineEmits(['status'])
const RUNTIME_LEASE_SCOPE = 'monster-enhance'

const defaultMultipliers = { monster_hp: '1', monster_stun: '1', monster_damage: '1', crocodile_damage: '1', sba_chain_timer: '3' }
const sessionMultipliers = window.gbfrMonsterEnhanceMultipliers || (window.gbfrMonsterEnhanceMultipliers = { ...defaultMultipliers })

const loading = ref(false)
const result = reactive({ pid: 0, dllPath: '', injected: false, enabled: false, currentBytes: '', items: [] })
const multipliers = reactive(sessionMultipliers)
const overdriveState = ref('3')
let disposed = false
let lifecycleEpoch = 0
let connectionOwnerToken = ''

function applyResult(res) {
  const previous = new Map((result.items || []).map(item => [item.id, item]))
  const incoming = (res && res.items) || []
  result.pid = (res && res.pid) || 0
  result.dllPath = (res && res.dllPath) || result.dllPath || ''
  result.injected = !!(res && res.injected)
  result.enabled = !!(res && res.enabled)
  result.currentBytes = (res && res.currentBytes) || ''
  result.items = incoming.filter(item => item.id !== 'inventory_set_45').map((item) => Object.assign(previous.get(item.id) || {}, item))
}

async function refreshStatus() {
  if (loading.value) return
  const epoch = ++lifecycleEpoch
  loading.value = true
  let acquiredOwnerToken = ''
  try {
    if (!connectionOwnerToken) {
      const info = await CharaAcquire(nextRuntimeAcquireRequestID())
      acquiredOwnerToken = String(info?.ownerToken || '')
      if (!acquiredOwnerToken) throw new Error('后端未返回怪物增强连接所有权令牌')
      if (disposed || epoch !== lifecycleEpoch) {
        queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, CharaRelease)
        return
      }
      connectionOwnerToken = acquiredOwnerToken
    }
    const ownerToken = connectionOwnerToken
    if (!ownerToken) throw new Error('当前页面不再持有怪物增强连接所有权')
    const res = await MonsterEnhanceGetStatusOwned(ownerToken)
    if (!disposed && epoch === lifecycleEpoch) applyResult(res)
  } catch (err) {
    let cleanupError = null
    if (acquiredOwnerToken) {
      try {
        await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, CharaRelease)
        if (connectionOwnerToken === acquiredOwnerToken) connectionOwnerToken = ''
      } catch (nextError) { cleanupError = nextError }
    }
    if (!disposed && epoch === lifecycleEpoch) {
      emit('status', cleanupError ? `${String(err)}；释放怪物增强连接也失败：${String(cleanupError)}` : String(err), 'error')
    }
  } finally {
    if (!disposed && epoch === lifecycleEpoch) loading.value = false
  }
}

async function disconnect() {
  if (loading.value) return
  const epoch = ++lifecycleEpoch
  const ownerToken = connectionOwnerToken
  if (!ownerToken) return
  loading.value = true
  try {
    await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease)
    if (disposed || epoch !== lifecycleEpoch) return
    if (connectionOwnerToken === ownerToken) connectionOwnerToken = ''
    applyResult(null)
    emit('status', '怪物增强 Hook 已恢复并断开连接', 'success')
  } catch (err) {
    if (!disposed && epoch === lifecycleEpoch) emit('status', String(err), 'error')
  } finally {
    if (!disposed && epoch === lifecycleEpoch) loading.value = false
  }
}

function needsMultiplier(item) {
  return item.id === 'monster_hp' || item.id === 'monster_stun' || item.id === 'monster_damage' || item.id === 'crocodile_damage' || item.id === 'sba_chain_timer'
}

function needsOverdriveState(item) {
  return item.id === 'overdrive_state'
}

function needsSbaTimer(item) {
  return item.id === 'sba_chain_timer'
}

function multiplierHint(item) {
  if (item.id === 'monster_hp') return '输入 10 = 怪物10倍血'
  if (item.id === 'monster_stun') return '输入 10 = 怪物10倍昏厥条'
  if (item.id === 'monster_damage') return '输入 32 = 怪物伤害32倍'
  if (item.id === 'crocodile_damage') return '输入 10 = 鳄鱼10倍血'
  if (item.id === 'sba_chain_timer') return '游戏默认 3 秒'
  return ''
}

function getMultiplier(item) {
  return parseFloat(multipliers[item.id] || defaultMultipliers[item.id] || '1')
}

function patchValue(item) {
  return needsOverdriveState(item) ? parseInt(overdriveState.value, 10) : (needsMultiplier(item) ? getMultiplier(item) : 0)
}

function startsDamageMeter(item) {
  return item.id === 'monster_hp' || item.id === 'crocodile_damage'
}

function ensureDamageMeter() {
  return DamageMeterGetStatus().catch((err) => emit('status', `伤害记录开启失败: ${String(err)}`, 'error'))
}

async function setOne(item, enabled, id = item.id) {
  if (!item.available) {
    emit('status', `${item.name}不可用：${item.unavailableReason || '当前游戏版本定位未闭环'}`, 'error')
    return
  }
  if (enabled && needsMultiplier(item)) {
    const v = getMultiplier(item)
    if (isNaN(v) || v <= 0 || v > 9999) { emit('status', '倍率请输入 0 到 9999 之间的数值', 'error'); return }
  }
  if (enabled && needsOverdriveState(item)) {
    const v = patchValue(item)
    if (![0, 3, 9].includes(v)) { emit('status', 'Overdrive 状态请选择空条、满黄条或自动OD', 'error'); return }
  }
  const previous = item.enabled
  item.enabled = enabled
  const epoch = ++lifecycleEpoch
  loading.value = true
  try {
    const ownerToken = connectionOwnerToken
    if (!ownerToken) throw new Error('当前页面不再持有怪物增强连接所有权')
    const res = await MonsterEnhanceSetPatchValueEnabledOwned(ownerToken, id, enabled, patchValue(item))
    if (disposed || epoch !== lifecycleEpoch) return
    if (enabled && startsDamageMeter(item)) await ensureDamageMeter()
    applyResult(res)
    const verb = id === 'overdrive_state_apply' || (item.id === 'sba_chain_timer' && enabled) ? '已应用' : (enabled ? '已开启' : '已关闭')
    emit('status', `${item.name}${verb}`, 'success')
  } catch (err) {
    item.enabled = previous
    if (!disposed && epoch === lifecycleEpoch) emit('status', String(err), 'error')
  } finally {
    if (!disposed && epoch === lifecycleEpoch) loading.value = false
  }
}

refreshStatus()

onBeforeUnmount(() => {
  disposed = true
  lifecycleEpoch += 1
  const ownerToken = connectionOwnerToken
  connectionOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease)
})
</script>

<template>
  <div class="monster-page ui-page is-wide ui-page-stack">
    <section class="monster-shell ui-card ui-panel">
      <header class="ui-split">
        <div class="title-copy">
          <h2 class="ui-section-title">怪物倍率与伤害记录</h2>
          <p class="ui-section-copy">统一查看运行状态，按功能分别设置倍率或开关。</p>
        </div>
        <span class="ui-tag" :class="result.injected ? 'is-ok' : 'is-info'">{{ result.injected ? 'DLL 已注入' : '等待注入' }}</span>
      </header>

      <div class="process-toolbar ui-toolbar">
        <div class="process-info">
          <span class="ui-field-label">目标进程</span>
          <strong>granblue_fantasy_relink.exe</strong>
        </div>
        <span v-if="result.pid" class="ui-tag is-info">PID {{ result.pid }}</span>
        <button type="button" class="ui-btn" @click="refreshStatus" :disabled="loading">{{ loading ? '刷新中...' : '刷新状态' }}</button>
        <button v-if="result.pid" type="button" class="ui-btn is-ghost" @click="disconnect" :disabled="loading">安全断开</button>
      </div>

      <div class="usage-notice ui-notice is-warn">
        <strong>本页功能仅在主机端使用时生效，开启前请告知队友。</strong>
        倍率、霸体、OD 与团队伤害记录属于实验性功能。
      </div>

      <div v-if="result.items.length" class="card-grid ui-card-grid">
        <article v-for="item in result.items" :key="item.id" class="feature-card ui-card ui-panel is-compact" :class="{ 'is-active': item.enabled, 'is-unavailable': !item.available }">
          <header class="feature-head ui-split">
            <h3 class="ui-section-title">{{ item.name }}</h3>
            <span class="ui-tag" :class="item.enabled ? 'is-ok' : ''">{{ !item.available ? '待适配' : (item.enabled ? '已开启' : '已关闭') }}</span>
          </header>

          <p v-if="!item.available" class="unavailable-copy">{{ item.unavailableReason || '当前游戏版本定位未闭环' }}</p>

          <label v-if="needsMultiplier(item)" class="ui-field">
            <span class="ui-field-label">参数值 <em>{{ multiplierHint(item) }}</em></span>
            <input v-model="multipliers[item.id]" type="number" min="0.1" max="9999" step="0.1" class="ui-input" placeholder="倍率" :disabled="!item.available" />
          </label>

          <label v-if="needsOverdriveState(item)" class="ui-field">
            <span class="ui-field-label">Overdrive 状态 <em>空条/满黄条会立即写入一次；自动 OD 会在退出 OD 后自动补满</em></span>
            <select v-model="overdriveState" class="ui-select" :disabled="!item.available">
              <option value="0">空条</option>
              <option value="3">满黄条</option>
              <option value="9">自动 OD</option>
            </select>
          </label>

          <div v-if="needsOverdriveState(item)" class="feature-actions ui-actions">
            <button type="button" class="ui-btn is-primary" @click="setOne(item, true)" :disabled="loading || !item.available || item.enabled || overdriveState !== '9'">自动 OD</button>
            <button type="button" class="ui-btn" @click="setOne(item, true, 'overdrive_state_apply')" :disabled="loading || !item.available || overdriveState === '9'">应用一次</button>
            <button type="button" class="ui-btn is-ghost" @click="setOne(item, false)" :disabled="loading || !item.available || !item.enabled">关闭</button>
          </div>
          <div v-else-if="needsSbaTimer(item)" class="feature-actions ui-actions">
            <button type="button" class="ui-btn is-primary" @click="setOne(item, true)" :disabled="loading || !item.available">应用</button>
            <button type="button" class="ui-btn is-ghost" @click="setOne(item, false)" :disabled="loading || !item.available || !item.enabled">恢复默认</button>
          </div>
          <div v-else class="feature-actions ui-actions">
            <button type="button" class="ui-btn is-primary" @click="setOne(item, true)" :disabled="loading || !item.available || item.enabled">开启</button>
            <button type="button" class="ui-btn is-ghost" @click="setOne(item, false)" :disabled="loading || !item.available || !item.enabled">关闭</button>
          </div>

          <details class="ui-disclosure diagnostics">
            <summary>定位诊断</summary>
            <dl class="diagnostic-data">
              <div><dt>RVA</dt><dd>0x{{ Number(item.rva).toString(16).toUpperCase() }}</dd></div>
              <div><dt>当前字节</dt><dd>{{ item.currentBytes || '未读取' }}</dd></div>
            </dl>
          </details>
        </article>
      </div>

      <p v-else class="ui-empty">请启动游戏后刷新状态。</p>
    </section>
  </div>
</template>

<style scoped>
.monster-page { padding-bottom:var(--space-8); }
.title-copy { display:flex; min-width:0; flex-direction:column; gap:var(--space-2); }
.process-toolbar { align-items:center; }
.process-info { display:flex; min-width:0; flex:1 1 280px; flex-direction:column; gap:var(--space-1); }
.process-info strong { min-width:0; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.usage-notice { display:flex; flex-wrap:wrap; gap:var(--space-1) var(--space-2); }
.card-grid { --ui-grid-min:320px; }
.feature-card { align-content:start; }
.feature-card.is-active { border-color:var(--success); background:var(--success-bg); }
.feature-card.is-unavailable { border-style:dashed; }
.feature-head { align-items:flex-start; }
.unavailable-copy { margin:0; color:var(--text-muted); font-size:var(--fs-sm); }
.feature-actions { margin-top:auto; }
.feature-actions .ui-btn { flex:1 1 96px; }
.diagnostics { margin-top:var(--space-1); }
.diagnostic-data { display:flex; min-width:0; flex-direction:column; gap:var(--space-3); }
.diagnostic-data div { min-width:0; }
.diagnostic-data dt { color:var(--text-muted); font-size:var(--fs-xs); }
.diagnostic-data dd { margin:var(--space-1) 0 0; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-sm); overflow-wrap:anywhere; }

@container ui-page (max-width:720px) {
  .card-grid { --ui-grid-min:100%; grid-template-columns:minmax(0,1fr); }
}
@container ui-page (max-width:420px) {
  .feature-actions .ui-btn { flex-basis:100%; }
}
</style>
