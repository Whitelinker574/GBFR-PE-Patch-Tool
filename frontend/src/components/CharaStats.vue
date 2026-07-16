<script setup>
import { ref, computed, onMounted } from 'vue'
import { FindSaveFiles, GetCharacterStats, UpdateCharacterStats } from '../../wailsjs/go/main/App'

const emit = defineEmits(['status'])
const slots = ref([])
const list = ref([])
const savePath = ref('')
const loading = ref(false)
const saving = ref(false)
const sortDesc = ref(false)
const selected = ref(new Set())
const batchCount = ref(0)
const error = ref('')

const sorted = computed(() => sortDesc.value
  ? [...list.value].sort((a, b) => b.count - a.count)
  : list.value)
const allSelected = computed(() => list.value.length > 0 && selected.value.size === list.value.length)

async function scanSaves() { slots.value = await FindSaveFiles() || [] }

function saveSlotLabel(slot) {
  const fileName = String(slot?.name || slot?.path || '').split(/[\\/]/).pop()
  const match = fileName.match(/SaveData\d+/i)
  return match ? match[0].replace(/^savedata/i, 'SaveData') : fileName.replace(/\.dat$/i, '')
}

async function load(path) {
  loading.value = true
  savePath.value = path
  error.value = ''
  selected.value = new Set()
  try {
    list.value = (await GetCharacterStats(path, false) || []).map(c => ({ ...c, count: Number(c.count) }))
  } catch (err) {
    list.value = []
    error.value = String(err)
  } finally { loading.value = false }
}

async function refresh() {
  await scanSaves()
  if (savePath.value) await load(savePath.value)
}

function toggle(slot) {
  const next = new Set(selected.value)
  next.has(slot) ? next.delete(slot) : next.add(slot)
  selected.value = next
}

function toggleAll() {
  selected.value = allSelected.value ? new Set() : new Set(list.value.map(c => c.slot))
}

function applyBatch() {
  const value = Math.max(0, Math.min(99999999, Number(batchCount.value) || 0))
  list.value.forEach(c => { if (selected.value.has(c.slot)) c.count = value })
}

async function saveSelected() {
  if (!savePath.value || !selected.value.size) return
  const changes = list.value
    .filter(c => selected.value.has(c.slot))
    .map(c => ({ slot: c.slot, count: Math.max(0, Math.min(99999999, Number(c.count) || 0)) }))
  saving.value = true
  try {
    const result = await UpdateCharacterStats(savePath.value, changes)
    emit('status', `已修改并验证 ${result.verified} 个角色，已自动备份`, 'success')
    await load(savePath.value)
  } catch (err) {
    emit('status', String(err), 'error')
  } finally { saving.value = false }
}

onMounted(scanSaves)
</script>

<template>
  <div class="root">
    <div class="section">
      <div class="header">
        <div>
          <div class="title">角色使用次数</div>
          <div class="hint">可单独选择任意角色，也可批量修改；写入前需退出游戏</div>
        </div>
        <span class="safety">写入时自动备份并回读验证</span>
      </div>

      <div class="slots">
        <button v-for="s in slots" :key="s.index" class="slot-btn" :class="{ on: savePath === s.path }" @click="load(s.path)">{{ saveSlotLabel(s) }}</button>
        <button class="plain-btn" @click="refresh">刷新</button>
      </div>

      <div class="version-row">
        <span class="version-label">已按存档内角色身份 Hash 自动识别，不再需要选择新档/转换档</span>
        <button v-if="list.length" class="plain-btn" @click="sortDesc = !sortDesc">{{ sortDesc ? '恢复槽位序' : '按次数排序' }}</button>
      </div>

      <div v-if="list.length" class="batch-row">
        <label class="select-all"><input type="checkbox" :checked="allSelected" @change="toggleAll"> 全选</label>
        <input v-model.number="batchCount" class="number-input batch-input" type="number" min="0" max="99999999">
        <button class="plain-btn max-btn" @click="batchCount=99999999">最大</button>
        <button class="plain-btn" :disabled="!selected.size" @click="applyBatch">填入已选</button>
        <span class="selection">已选 {{ selected.size }} 项</span>
        <button class="save-btn" :disabled="saving || !selected.size" @click="saveSelected">{{ saving ? '写入中…' : '保存已选' }}</button>
      </div>

      <div v-if="loading" class="empty">解析中…</div>
      <div v-else-if="error" class="empty error">{{ error }}</div>
      <div v-else-if="list.length" class="table">
        <div class="row row-head"><span class="col-check"></span><span class="col-name">角色</span><span class="col-count">次数</span></div>
        <label v-for="c in sorted" :key="c.slot" class="row">
          <span class="col-check"><input type="checkbox" :checked="selected.has(c.slot)" @change="toggle(c.slot)"></span>
          <span class="col-name">{{ c.name }} <small>#{{ c.slot }}</small></span>
          <input v-model.number="c.count" class="number-input col-count" type="number" min="0" max="99999999" @focus="selected.add(c.slot)">
        </label>
      </div>
      <div v-else-if="savePath" class="empty">未找到当前档案角色次数</div>
    </div>
  </div>
</template>

<style scoped>
.root { width:100%; max-width:720px; margin:0 auto; padding-bottom:40px; container-type:inline-size; }
.section { border-radius:12px; padding:16px; background:rgba(255,255,255,.025); border:1px solid rgba(255,255,255,.07); display:flex; flex-direction:column; gap:12px; }
.header { display:flex; align-items:flex-start; justify-content:space-between; gap:16px; }
.title { font-size:.9rem; font-weight:650; color:rgba(255,255,255,.72); letter-spacing:.5px; }
.hint,.safety { margin-top:4px; font-size:.68rem; color:rgba(255,255,255,.3); }
.safety { color:rgba(74,222,128,.7); white-space:nowrap; }
.slots,.version-row,.batch-row { display:flex; gap:8px; flex-wrap:wrap; align-items:center; }
.version-label,.selection,.select-all { font-size:.74rem; color:rgba(255,255,255,.4); }
.slot-btn,.plain-btn,.save-btn { padding:7px 13px; border-radius:7px; border:1px solid rgba(255,255,255,.12); background:rgba(255,255,255,.05); color:rgba(255,255,255,.55); font:inherit; font-size:.75rem; cursor:pointer; }
.slot-btn.on { border-color:rgba(103,232,249,.45); background:rgba(103,232,249,.11); color:#67e8f9; }
.plain-btn:hover:not(:disabled),.slot-btn:hover { color:rgba(255,255,255,.8); background:rgba(255,255,255,.09); }
button:disabled { opacity:.35; cursor:not-allowed; }
.batch-row { padding:10px; border-radius:9px; background:rgba(255,255,255,.025); }
.selection { margin-left:auto; }
.save-btn { border-color:rgba(74,222,128,.28); background:rgba(34,197,94,.11); color:#4ade80; }
.table { border:1px solid rgba(255,255,255,.07); border-radius:10px; overflow:hidden; }
.row { min-height:39px; display:flex; align-items:center; gap:10px; padding:4px 12px; border-bottom:1px solid rgba(255,255,255,.035); box-sizing:border-box; }
.row:last-child { border-bottom:0; }
.row:hover { background:rgba(255,255,255,.025); }
.row-head { min-height:32px; background:rgba(255,255,255,.035); font-size:.68rem; color:rgba(255,255,255,.32); }
.col-check { width:20px; flex:0 0 20px; }
.col-name { flex:1; font-size:.79rem; color:rgba(255,255,255,.62); }
.col-name small { color:rgba(255,255,255,.2); font-size:.64rem; }
.number-input { box-sizing:border-box; padding:6px 8px; border-radius:6px; border:1px solid rgba(255,255,255,.12); background:rgba(255,255,255,.055); color:#8be9f7; outline:none; font-family:var(--font-data); }
.number-input:focus { border-color:rgba(103,232,249,.45); }
.col-count { width:100px; text-align:right; }
.batch-input { width:110px; }
input[type=checkbox] { accent-color:#67e8f9; }
@container (max-width:460px) { .header { flex-direction:column; } .safety { white-space:normal; } .selection { margin-left:0; flex-basis:100%; } .col-count { width:82px; } }
.empty { font-size:.78rem; color:rgba(255,255,255,.32); text-align:center; padding:16px 0; }
.empty.error { color:#f87171; }
</style>
