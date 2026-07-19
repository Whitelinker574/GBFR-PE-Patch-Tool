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
    <div class="section ui-card ui-panel">
      <div class="slots">
        <button v-for="s in slots" :key="s.index" class="slot-btn ui-btn is-sm" :class="{ on: savePath === s.path }" @click="load(s.path)">{{ saveSlotLabel(s) }}</button>
        <button class="plain-btn ui-btn is-sm" @click="refresh">刷新</button>
      </div>

      <div class="version-row">
        <span class="version-label ui-hint">已按存档内角色身份 Hash 自动识别，不再需要选择新档/转换档</span>
        <button v-if="list.length" class="plain-btn ui-btn is-sm" @click="sortDesc = !sortDesc">{{ sortDesc ? '恢复槽位序' : '按次数排序' }}</button>
      </div>

      <div v-if="list.length" class="batch-row ui-card">
        <label class="select-all"><input type="checkbox" :checked="allSelected" @change="toggleAll"> 全选</label>
        <input v-model.number="batchCount" class="number-input batch-input ui-input" type="number" min="0" max="99999999">
        <button class="plain-btn max-btn ui-btn is-sm" @click="batchCount=99999999">最大</button>
        <button class="plain-btn ui-btn is-sm" :disabled="!selected.size" @click="applyBatch">填入已选</button>
        <span class="selection">已选 {{ selected.size }} 项</span>
        <button class="save-btn ui-btn is-primary" :disabled="saving || !selected.size" @click="saveSelected">{{ saving ? '写入中…' : '保存已选' }}</button>
      </div>

      <div v-if="loading" class="empty ui-empty">解析中…</div>
      <div v-else-if="error" class="empty ui-empty error">{{ error }}</div>
      <div v-else-if="list.length" class="table ui-card">
        <div class="row row-head"><span class="col-check"></span><span class="col-name">角色</span><span class="col-count">次数</span></div>
        <label v-for="c in sorted" :key="c.slot" class="row ui-row">
          <span class="col-check"><input type="checkbox" :checked="selected.has(c.slot)" @change="toggle(c.slot)"></span>
          <span class="col-name">{{ c.name }} <small>#{{ c.slot }}</small></span>
          <input v-model.number="c.count" class="number-input col-count ui-input" type="number" min="0" max="99999999" @focus="selected.add(c.slot)">
        </label>
      </div>
      <div v-else-if="!savePath" class="empty ui-empty">选择存档后读取角色使用次数</div>
      <div v-else class="empty ui-empty">未找到当前档案角色次数</div>
    </div>
  </div>
</template>

<style scoped>
.root {
  width:100%;
  max-width:840px;
  min-height:0;
  margin:0 auto;
  padding-bottom:var(--space-9);
  color:var(--text-secondary);
  container-type:inline-size;
}
.section { min-width:0; }
.slots,
.version-row,
.batch-row {
  display:flex;
  flex-wrap:wrap;
  align-items:center;
  gap:var(--space-2);
}
.slot-btn.on {
  border-color:var(--selected-border);
  background:var(--selected-bg);
  color:var(--selected-fg);
}
.version-row { justify-content:space-between; }
.version-label { min-width:220px; flex:1; color:var(--text-secondary); }
.batch-row {
  padding:var(--space-3) var(--space-4);
  background:var(--surface-sunken);
  box-shadow:none;
}
.select-all,
.selection {
  color:var(--text-secondary);
  font-size:var(--fs-sm);
}
.selection { margin-left:auto; }
.batch-input { width:116px; }

.table {
  min-width:0;
  overflow:hidden;
  background:var(--surface-card-pop);
  box-shadow:none;
}
.row {
  min-height:42px;
  display:flex;
  align-items:center;
  gap:var(--space-3);
  padding:var(--space-2) var(--space-4);
  border:0;
  border-bottom:1px solid var(--border-soft);
  border-radius:0;
  background:transparent;
  box-sizing:border-box;
}
.row:last-child { border-bottom:0; }
.row-head {
  min-height:34px;
  background:var(--surface-sunken);
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  font-weight:var(--fw-semibold);
}
.col-check { width:20px; flex:0 0 20px; }
.col-name {
  min-width:0;
  flex:1;
  overflow:hidden;
  color:var(--text-primary);
  font-size:var(--fs-md);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.col-name small { color:var(--text-secondary); font-size:var(--fs-xs); }
.number-input { font-family:var(--font-data); font-variant-numeric:tabular-nums; }
.col-count { width:112px; text-align:right; }
input[type="checkbox"] { accent-color:var(--accent); }
.empty.error { color:var(--danger-ink); }

@container (max-width:620px) {
  .section { padding:var(--space-4); }
  .version-row { align-items:stretch; flex-direction:column; }
  .version-row .ui-btn { align-self:flex-start; }
  .batch-row { align-items:stretch; }
  .selection { width:100%; margin-left:0; }
  .batch-input { min-width:90px; flex:1; width:auto; }
  .save-btn { flex:1; }
  .col-count { width:92px; }
}

@container (max-width:420px) {
  .slots .ui-btn { flex:1; }
  .row { padding-inline:var(--space-3); }
  .col-count { width:80px; }
}
</style>
