<script setup>
import { onMounted, reactive, ref } from 'vue'
import {
  OverLimitCommit,
  OverLimitEnable,
  OverLimitGetOptions,
  OverLimitGetStatus,
  OverLimitScan,
  OverLimitSetSlot,
} from '../../wailsjs/go/main/App'

const emit = defineEmits(['status'])

const loading = ref(false)
const options = reactive({ attributes: [], levels: [] })
const status = reactive({ found: false, hooked: false, address: 0, rva: 0, selectedAddr: 0, commitRva: 0, currentBytes: '', slots: [] })
const edits = reactive([
  { attribute: 0, level: 0, value: 0 },
  { attribute: 0, level: 0, value: 0 },
  { attribute: 0, level: 0, value: 0 },
  { attribute: 0, level: 0, value: 0 },
])

onMounted(() => {
  OverLimitGetOptions()
    .then((res) => {
      options.attributes = res.attributes || []
      options.levels = res.levels || []
    })
    .catch((err) => emit('status', String(err), 'error'))
})

function formatHex(value) {
  if (!value) return '-'
  return '0x' + Number(value).toString(16).toUpperCase()
}

function optionName(list, id) {
  const found = list.find(x => Number(x.id) === Number(id))
  return found ? found.name : formatHex(id)
}

function attributeOption(id) {
  return options.attributes.find(x => Number(x.id) === Number(id))
}

function maxValue(id) {
  const opt = attributeOption(id)
  return opt ? Number(opt.maxValue || 0) : 0
}

function applyMaxValue(index) {
  edits[index].value = maxValue(edits[index].attribute)
}

function applyAllMaxValues() {
  edits.forEach((edit, index) => { edit.value = maxValue(edit.attribute); edits[index].level = options.levels.at(-1)?.id ?? edit.level })
}

function applyStatus(next) {
  Object.assign(status, next || { found: false, hooked: false, address: 0, rva: 0, selectedAddr: 0, commitRva: 0, currentBytes: '', slots: [] })
  ;(status.slots || []).forEach((slot, i) => {
    if (i < edits.length) {
      edits[i].attribute = Number(slot.attribute || 0)
      edits[i].level = Number(slot.level || 0)
      edits[i].value = Number(slot.value || maxValue(slot.attribute) || 0)
    }
  })
}

function run(action, success) {
  loading.value = true
  action()
    .then((res) => { applyStatus(res); if (success) emit('status', success, 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { loading.value = false })
}

function scan() {
  run(() => OverLimitScan(), '上限突破特征定位成功')
}

function enable() {
  run(() => OverLimitEnable(), '上限突破读取已开启，请在突破界面加载角色')
}

function refresh() {
  loading.value = true
  OverLimitGetStatus()
    .then((res) => {
      applyStatus(res)
      emit('status', res?.selectedAddr ? '上限突破角色数据已刷新' : '已刷新，尚未读取到角色数据；请在突破界面切换一次角色或重新打开突破界面', res?.selectedAddr ? 'success' : 'error')
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { loading.value = false })
}

function writeSaveAll() {
  run(
    () => edits.reduce((p, edit, index) => p.then(() => OverLimitSetSlot({ index, attribute: Number(edit.attribute), level: Number(edit.level), value: Number(edit.value) })), Promise.resolve()).then(() => OverLimitCommit()),
    '四个上限突破槽位已写入，请返回游戏确认保存'
  )
}
</script>

<template>
  <div class="root">
    <div class="section">
      <div class="header">
        <span class="title">上限突破</span>
        <span class="info-dot" title="需游戏运行中使用；先开启读取，再进入角色上限突破界面加载角色。">!</span>
        <span class="hint">读取当前突破界面四个能力槽</span>
      </div>

      <div class="memory-card guide-card">
        <div class="memory-header">
          <span class="memory-title">使用提示</span>
          <span class="memory-hint">按顺序操作</span>
        </div>
        <ol class="guide-list">
          <li>启动游戏后开启读取，确保角色任意突破过一次否则无法识别</li>
          <li>先进行一次 3 级突破，停在 “Over the limit!” 界面。</li>
          <li>点击刷新，设置词条后写入，回到游戏确认。</li>
          <li>再次进行 3 级突破，但不确认替换之前突破，直接取消。</li>
          <li>以上操作完成后存档即可持久保存。</li>
        </ol>
      </div>

      <div class="memory-card" :class="{ active: status.hooked }">
        <div class="memory-header">
          <span class="memory-title">突破界面读取</span>
        </div>
        <div class="memory-info">
          <span>RVA: {{ formatHex(status.rva) }}</span>
          <span>状态: {{ status.hooked ? '已开启' : '未开启' }}</span>
          <span>角色数据: {{ formatHex(status.selectedAddr) }}</span>
        </div>
        <div class="memory-row">
          <button class="btn-batch" @click="enable" :disabled="loading || status.hooked">开启读取</button>
          <button class="btn-refresh" @click="refresh" :disabled="loading">刷新</button>
          <button class="btn-sort" @click="scan" :disabled="loading">重新扫描</button>
          <button class="btn-batch" @click="writeSaveAll" :disabled="loading || !status.selectedAddr">写入结果</button>
          <button class="btn-max-all" @click="applyAllMaxValues" :disabled="loading || !status.selectedAddr">四项一键最大</button>
        </div>
        <div class="memory-bytes">{{ status.currentBytes || '未定位' }}</div>
      </div>

      <div v-if="!status.selectedAddr" class="empty">开启读取后，在游戏上限突破界面加载角色</div>

      <div v-else class="slot-list">
        <div v-for="slot in status.slots" :key="slot.index" class="memory-card slot-card">
          <div class="memory-header">
            <span class="memory-title">能力 {{ slot.index + 1 }}</span>
            <span class="memory-hint">当前 {{ optionName(options.attributes, slot.attribute) }} / {{ optionName(options.levels, slot.level) }}</span>
          </div>
          <div class="slot-grid">
            <label>
              <span>词条</span>
              <select v-model.number="edits[slot.index].attribute" class="od-select" @change="applyMaxValue(slot.index)">
                <option v-for="opt in options.attributes" :key="opt.id" :value="opt.id">{{ opt.name }} ({{ opt.hex }})</option>
              </select>
            </label>
            <label>
              <span>等级</span>
              <select v-model.number="edits[slot.index].level" class="od-select level-select">
                <option v-for="opt in options.levels" :key="opt.id" :value="opt.id">{{ opt.name }}</option>
              </select>
            </label>
            <label class="value-edit">
              <span>数值</span>
              <span class="value-combo"><input v-model.number="edits[slot.index].value" type="number" min="0" :max="maxValue(edits[slot.index].attribute)" step="1" class="value-input" /><button @click="applyMaxValue(slot.index)">最大</button></span>
            </label>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.root { display:flex; flex-direction:column; gap:10px; width:100%; max-width:720px; margin:0 auto; padding-bottom:40px; container-type:inline-size; }
.section { border-radius:16px; padding:16px 18px; background:linear-gradient(135deg, rgba(56,189,248,0.12) 0%, rgba(103,232,249,0.06) 100%); border:1px solid rgba(103,232,249,0.15); display:flex; flex-direction:column; gap:10px; }
.header { display:flex; align-items:center; justify-content:space-between; gap:8px; }
.title { font-size:0.88rem; font-weight:600; color:rgba(255,255,255,0.65); letter-spacing:1px; }
.info-dot { display:inline-flex; align-items:center; justify-content:center; width:15px; height:15px; border-radius:50%; border:1px solid rgba(103,232,249,0.35); color:#67e8f9; background:rgba(103,232,249,0.08); font-size:0.68rem; font-weight:700; cursor:help; flex-shrink:0; }
.hint { font-size:0.68rem; color:rgba(255,255,255,0.25); margin-left:auto; }
.memory-card { position:relative; overflow:hidden; z-index:0; border-radius:12px; padding:12px; background:rgba(255,255,255,0.045); border:1px solid rgba(165,180,252,0.16); box-shadow:0 10px 26px rgba(0,0,0,0.18); display:flex; flex-direction:column; gap:8px; transition:border-color 0.3s, box-shadow 0.3s; }
.memory-card::after { content:""; position:absolute; inset:0; z-index:-1; border-radius:12px; background:#abd373; transform:translateY(calc(-100% - 2px)); transition:transform 0.5s ease; }
.memory-card.active { border-color:rgba(171,211,115,0.55); box-shadow:0 14px 34px rgba(171,211,115,0.18); }
.memory-card.active::after { transform:translateY(0); }
.memory-card.active .memory-title { color:#1f2937; }
.memory-card.active .memory-hint, .memory-card.active .memory-info, .memory-card.active .memory-bytes { color:rgba(31,41,55,0.72); }
.memory-card.active .btn-batch { border-color:rgba(31,41,55,0.22); background:rgba(31,41,55,0.12); color:#1f2937; }
.memory-card.active .btn-refresh, .memory-card.active .btn-sort { border-color:rgba(31,41,55,0.16); background:rgba(255,255,255,0.18); color:rgba(31,41,55,0.72); }
.memory-header, .memory-info, .memory-row { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
.memory-header .memory-hint { margin-left:auto; }
.memory-title { font-size:0.8rem; font-weight:600; color:rgba(255,255,255,0.62); }
.memory-hint, .memory-info { font-size:0.68rem; color:rgba(255,255,255,0.32); }
.memory-bytes { font-size:0.66rem; color:rgba(255,255,255,0.24); font-family:var(--font-data); word-break:break-all; }
.guide-card { gap:10px; }
.guide-list { margin:0; padding-left:18px; color:rgba(255,255,255,0.46); font-size:0.72rem; line-height:1.65; }
.guide-list li { padding-left:2px; }
.slot-list { display:flex; flex-direction:column; gap:10px; }
.slot-card::after { display:none; }
.slot-grid { display:grid; grid-template-columns:minmax(210px, 1fr) 96px 78px; gap:8px; align-items:end; }
.slot-grid label, .slot-value { display:flex; flex-direction:column; gap:5px; text-align:left; }
.slot-grid label span, .slot-value span { font-size:0.68rem; color:rgba(255,255,255,0.32); }
.slot-value strong, .value-input { min-height:30px; display:flex; align-items:center; color:#256e74; font-size:0.82rem; }
.value-input { box-sizing:border-box; width:100%; padding:6px 8px; border-radius:6px; border:1px solid rgba(255,255,255,0.15); background:rgba(255,255,255,0.07); outline:none; }
.od-select { width:100%; padding:6px 10px; border-radius:6px; border:1px solid rgba(255,255,255,0.15); background:rgba(255,255,255,0.07); color:#fff; font-size:0.8rem; outline:none; cursor:pointer; }
.od-select:focus { border-color:rgba(103,232,249,0.5); }
.od-select option { background:#1a1a2e; color:#fff; }
.btn-batch { padding:6px 14px; border-radius:6px; border:1px solid rgba(165,180,252,0.3); background:rgba(165,180,252,0.1); color:#a5b4fc; font-size:0.78rem; font-weight:600; cursor:pointer; transition:background 0.2s; white-space:nowrap; }
.btn-batch:not(:disabled):hover { background:rgba(165,180,252,0.2); }
.btn-batch:disabled { opacity:0.4; cursor:not-allowed; }
.btn-refresh, .btn-sort { padding:6px 14px; border-radius:6px; border:1px solid rgba(255,255,255,0.12); background:rgba(255,255,255,0.05); color:rgba(255,255,255,0.5); font-size:0.78rem; font-weight:600; cursor:pointer; transition:background 0.2s; }
.btn-refresh:hover, .btn-sort:hover { background:rgba(255,255,255,0.1); color:rgba(255,255,255,0.7); }
.btn-refresh:disabled, .btn-sort:disabled { opacity:0.4; cursor:not-allowed; }
.empty { font-size:0.78rem; color:rgba(255,255,255,0.3); text-align:center; padding:12px 0; }
@media (max-width: 640px) { .slot-grid { grid-template-columns:1fr 1fr; } .slot-grid .btn-batch { grid-column:1 / -1; } }
@container (max-width:520px) { .slot-grid { grid-template-columns:minmax(0,1fr) minmax(88px,.45fr); } .slot-grid .btn-batch { grid-column:1/-1; width:100%; } }
.section,.memory-card{border-color:rgba(154,202,224,.14);background:rgba(8,31,53,.7);border-radius:4px 12px 4px 12px}.memory-card.active{border-color:rgba(118,210,174,.34);background:rgba(118,210,174,.07)}.value-combo{display:grid;grid-template-columns:minmax(0,1fr) 45px;gap:4px}.value-combo button,.btn-max-all{border:1px solid rgba(218,187,115,.28);background:rgba(218,187,115,.07);color:#d9bd7c;border-radius:3px 7px 3px 7px;font-size:10px;cursor:pointer}.btn-max-all{padding:6px 12px}.value-combo button:hover,.btn-max-all:hover{border-color:var(--gold);background:rgba(218,187,115,.14)}
</style>
