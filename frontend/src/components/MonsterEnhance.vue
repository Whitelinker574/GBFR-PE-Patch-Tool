<script setup>
import { reactive, ref } from 'vue'
import { MonsterEnhanceGetStatus, MonsterEnhanceSetPatchValueEnabled } from '../../wailsjs/go/main/App'

const emit = defineEmits(['status'])

const loading = ref(false)
const result = reactive({ pid: 0, dllPath: '', injected: false, enabled: false, currentBytes: '', items: [] })
const multipliers = reactive({ monster_hp: '10', monster_stun: '10' })

function applyResult(res) {
  const previous = new Map((result.items || []).map(item => [item.id, item]))
  const incoming = (res && res.items) || []
  result.pid = (res && res.pid) || 0
  result.dllPath = (res && res.dllPath) || result.dllPath || ''
  result.injected = !!(res && res.injected)
  result.enabled = !!(res && res.enabled)
  result.currentBytes = (res && res.currentBytes) || ''
  result.items = incoming.map((item) => Object.assign(previous.get(item.id) || {}, item))
}

function refreshStatus() {
  loading.value = true
  MonsterEnhanceGetStatus()
    .then((res) => applyResult(res))
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { loading.value = false })
}

function getMultiplier(item) {
  return parseFloat(multipliers[item.id] || '10')
}

function setOne(item, enabled) {
  if (enabled && (item.id === 'monster_hp' || item.id === 'monster_stun')) {
    const v = getMultiplier(item)
    if (isNaN(v) || v <= 0 || v > 9999) { emit('status', '倍率请输入 0 到 9999 之间的数值', 'error'); return }
  }
  const previous = item.enabled
  item.enabled = enabled
  loading.value = true
  MonsterEnhanceSetPatchValueEnabled(item.id, enabled, (item.id === 'monster_hp' || item.id === 'monster_stun') ? getMultiplier(item) : 0)
    .then((res) => {
      applyResult(res)
      emit('status', `${item.name}${enabled ? '已开启' : '已关闭'}`, 'success')
    })
    .catch((err) => {
      item.enabled = previous
      emit('status', String(err), 'error')
    })
    .finally(() => { loading.value = false })
}

refreshStatus()
</script>

<template>
  <div class="root">
    <div class="section">
      <div class="header">
        <span class="title">怪物增强</span>
        <span class="info-dot" title="开启时释放内置 patch_core.dll 到临时目录并注入；关闭时 Go 侧恢复原始字节。">!</span>
        <span class="hint">DLL 注入开启 · Go 恢复关闭</span>
      </div>

      <div class="process-card">
        <div class="memory-info">
          <span>目标进程: granblue_fantasy_relink.exe</span>
          <span v-if="result.pid">PID: {{ result.pid }}</span>
          <button class="btn-refresh compact" @click="refreshStatus" :disabled="loading">刷新</button>
        </div>
      </div>

      <div class="card-grid">
        <div v-for="item in result.items" :key="item.id" class="memory-card">
        <div class="memory-header">
          <span class="memory-title">{{ item.name }}</span>
          <span class="state" :class="{ on: item.enabled }">{{ item.enabled ? '开启' : '关闭' }}</span>
          <span class="memory-hint">RVA: 0x{{ Number(item.rva).toString(16).toUpperCase() }}</span>
        </div>
        <div v-if="item.id === 'monster_hp' || item.id === 'monster_stun'" class="memory-row">
          <input v-model="multipliers[item.id]" type="number" min="0.1" max="9999" step="0.1" class="batch-input" placeholder="倍率" />
          <span class="memory-hint">{{ item.id === 'monster_hp' ? '输入 10 = 怪物10倍血' : '输入 10 = 怪物10倍昏厥条' }}</span>
        </div>
        <div class="memory-row">
          <button class="btn-batch" @click="setOne(item, true)" :disabled="loading || item.enabled">开启</button>
          <button class="btn-refresh" @click="setOne(item, false)" :disabled="loading || !item.enabled">关闭</button>
        </div>
        <div class="memory-bytes">{{ item.currentBytes }}</div>
        </div>
      </div>

      <div v-if="!result.items.length" class="empty">请启动游戏后刷新状态</div>
    </div>
  </div>
</template>

<style scoped>
.root { display:flex; flex-direction:column; gap:10px; width:100%; max-width:720px; margin:0 auto; padding-bottom:40px; }
.section {
  border-radius:16px; padding:16px 18px;
  background:linear-gradient(135deg, rgba(56,189,248,0.12) 0%, rgba(103,232,249,0.06) 100%);
  border:1px solid rgba(103,232,249,0.15);
  display:flex; flex-direction:column; gap:10px;
}
.header { display:flex; align-items:center; justify-content:space-between; gap:8px; }
.title { font-size:0.88rem; font-weight:600; color:rgba(255,255,255,0.65); letter-spacing:1px; }
.info-dot { display:inline-flex; align-items:center; justify-content:center; width:15px; height:15px; border-radius:50%; border:1px solid rgba(103,232,249,0.35); color:#67e8f9; background:rgba(103,232,249,0.08); font-size:0.68rem; font-weight:700; cursor:help; flex-shrink:0; }
.hint { font-size:0.68rem; color:rgba(255,255,255,0.25); margin-left:auto; }
.process-card {
  border-radius:10px; padding:8px 10px;
  background:rgba(255,255,255,0.035); border:1px solid rgba(103,232,249,0.12);
}
.card-grid { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:10px; }
.memory-card {
  border-radius:12px; padding:12px;
  background:rgba(255,255,255,0.045); border:1px solid rgba(165,180,252,0.16);
  display:flex; flex-direction:column; gap:8px;
}
.memory-header, .memory-info, .memory-row { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
.memory-header { justify-content:flex-start; }
.memory-header .memory-hint { margin-left:auto; }
.memory-title { font-size:0.8rem; font-weight:600; color:rgba(255,255,255,0.62); }
.memory-hint, .memory-info { font-size:0.68rem; color:rgba(255,255,255,0.32); }
.batch-input { width:80px; padding:6px 10px; border-radius:6px; border:1px solid rgba(255,255,255,0.15); background:rgba(255,255,255,0.07); color:#fff; font-size:0.82rem; outline:none; }
.memory-bytes { font-size:0.66rem; color:rgba(255,255,255,0.24); font-family:'Courier New',monospace; word-break:break-all; }
.btn-batch, .btn-refresh {
  padding:6px 14px; border-radius:6px; font-size:0.78rem; font-weight:600; cursor:pointer;
  transition:background 0.2s; white-space:nowrap;
}
.btn-batch { border:1px solid rgba(165,180,252,0.3); background:rgba(165,180,252,0.1); color:#a5b4fc; }
.btn-refresh { border:1px solid rgba(255,255,255,0.12); background:rgba(255,255,255,0.05); color:rgba(255,255,255,0.5); }
.btn-batch:not(:disabled):hover { background:rgba(165,180,252,0.2); }
.btn-refresh:not(:disabled):hover { background:rgba(255,255,255,0.1); color:rgba(255,255,255,0.7); }
.btn-batch:disabled, .btn-refresh:disabled { opacity:0.4; cursor:not-allowed; }
.btn-refresh.compact { padding:4px 10px; font-size:0.72rem; }
.state { font-size:0.68rem; padding:2px 8px; border-radius:999px; color:#f87171; background:rgba(239,68,68,0.12); border:1px solid rgba(239,68,68,0.22); }
.state.on { color:#4ade80; background:rgba(34,197,94,0.12); border-color:rgba(34,197,94,0.22); }
.empty { font-size:0.78rem; color:rgba(255,255,255,0.3); text-align:center; padding:12px 0; }
@media (max-width: 640px) { .card-grid { grid-template-columns:1fr; } }
</style>
