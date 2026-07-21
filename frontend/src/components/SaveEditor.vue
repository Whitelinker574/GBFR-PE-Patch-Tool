<script setup>
import { ref, computed } from 'vue'
import { FindSaveFiles, GetQuests, LoadSave, UpdateQuestCounts } from '../../wailsjs/go/backend/App'
import BadgeUnlock from './BadgeUnlock.vue'

const emit = defineEmits(['status'])
const slots = ref([])
const quests = ref([])
const total = ref(0)
const loading = ref(false)
const saving = ref(false)
const savePath = ref('')
const sortDesc = ref(true)
const search = ref('')
const selected = ref(new Set())
const batchCount = ref(0)
const activeMode = ref('quests')

const visibleQuests = computed(() => {
  const needle = search.value.trim().toLowerCase()
  let rows = needle ? quests.value.filter(q => `${q.questCode || q.questId} ${q.questNameCn || ''} ${q.questName || ''}`.toLowerCase().includes(needle)) : quests.value
  return sortDesc.value ? [...rows].sort((a, b) => b.clears - a.clears) : rows
})
const allVisibleSelected = computed(() => visibleQuests.value.length > 0 && visibleQuests.value.every(q => selected.value.has(q.index)))

async function scanSaves() { slots.value = await FindSaveFiles() || [] }

function saveSlotLabel(slot) {
  const fileName = String(slot?.name || slot?.path || '').split(/[\\/]/).pop()
  const match = fileName.match(/SaveData\d+/i)
  return match ? match[0].replace(/^savedata/i, 'SaveData') : fileName.replace(/\.dat$/i, '')
}

async function load(path) {
  loading.value = true
  savePath.value = path
  selected.value = new Set()
  try {
    const [summary, rows] = await Promise.all([LoadSave(path), GetQuests(path)])
    quests.value = (rows || []).map(q => ({ ...q, clears: Number(q.clears) }))
    total.value = summary?.questTotalClears || 0
  } catch (err) {
    quests.value = []
    emit('status', String(err), 'error')
  } finally { loading.value = false }
}

function toggle(index) {
  const next = new Set(selected.value)
  next.has(index) ? next.delete(index) : next.add(index)
  selected.value = next
}

function toggleVisible() {
  const next = new Set(selected.value)
  if (allVisibleSelected.value) visibleQuests.value.forEach(q => next.delete(q.index))
  else visibleQuests.value.forEach(q => next.add(q.index))
  selected.value = next
}

function applyBatch() {
  const count = Math.max(0, Math.min(99999999, Number(batchCount.value) || 0))
  quests.value.forEach(q => { if (selected.value.has(q.index)) q.clears = count })
}

async function saveSelected() {
  if (!savePath.value || !selected.value.size) return
  const changes = quests.value.filter(q => selected.value.has(q.index)).map(q => ({
    index: q.index,
    questId: q.questId,
    storedId: q.storedId,
    count: Math.max(0, Math.min(99999999, Number(q.clears) || 0)),
  }))
  saving.value = true
  try {
    const result = await UpdateQuestCounts(savePath.value, changes)
    emit('status', `已修改并验证 ${result.verified} 个任务，已自动备份`, 'success')
    await load(savePath.value)
  } catch (err) {
    emit('status', String(err), 'error')
  } finally { saving.value = false }
}

function forwardStatus(...args) { emit('status', ...args) }

scanSaves()
</script>

<template>
  <div class="root">
    <div class="slots">
      <button v-for="s in slots" :key="s.index" class="slot-btn ui-btn is-sm" :class="{ on: savePath === s.path }" @click="load(s.path)">{{ saveSlotLabel(s) }}</button>
      <button class="plain-btn ui-btn is-sm" @click="scanSaves">刷新</button>
    </div>

    <nav class="record-tabs" role="tablist" aria-label="记录类型">
      <button type="button" class="ui-tab" role="tab" :aria-selected="activeMode === 'quests'" :class="{ active: activeMode === 'quests' }" @click="activeMode = 'quests'">任务完成次数</button>
      <button type="button" class="ui-tab" role="tab" :aria-selected="activeMode === 'badges'" :class="{ active: activeMode === 'badges' }" @click="activeMode = 'badges'">称号记录</button>
    </nav>

    <BadgeUnlock v-if="activeMode === 'badges'" :save-path="savePath" @status="forwardStatus" />
    <div v-else-if="loading" class="loading empty ui-empty">解析中…</div>
    <div v-else-if="quests.length" class="quests ui-card">
      <div class="head">
        <span>{{ quests.length }} 个任务 · {{ total }} 次挑战</span>
        <input v-model="search" class="search ui-input" placeholder="搜索任务或 ID">
        <button class="plain-btn ui-btn is-sm" @click="sortDesc = !sortDesc">{{ sortDesc ? '次数排序' : '默认顺序' }}</button>
      </div>
      <div class="batch ui-card">
        <label><input type="checkbox" :checked="allVisibleSelected" @change="toggleVisible"> 选择当前结果</label>
        <input v-model.number="batchCount" class="number-input ui-input" type="number" min="0" max="99999999">
        <button class="plain-btn max-btn ui-btn is-sm" @click="batchCount=99999999">最大</button>
        <button class="plain-btn ui-btn is-sm" :disabled="!selected.size" @click="applyBatch">填入已选</button>
        <span>已选 {{ selected.size }} 项</span>
        <button class="save-btn ui-btn is-primary" :disabled="saving || !selected.size" @click="saveSelected">{{ saving ? '写入中…' : '保存已选' }}</button>
      </div>
      <div class="list">
        <label v-for="q in visibleQuests" :key="q.index" class="row ui-row">
          <input type="checkbox" :checked="selected.has(q.index)" @change="toggle(q.index)">
          <span class="id">{{ q.questCode || q.questId }}</span>
          <span class="name">{{ q.questNameCn || q.questName }}</span>
          <input v-model.number="q.clears" class="number-input count ui-input" type="number" min="0" max="99999999" @focus="selected.add(q.index)">
        </label>
      </div>
    </div>
    <div v-else-if="!savePath" class="empty ui-empty">选择存档后读取任务完成次数</div>
    <div v-else class="empty ui-empty">当前存档没有可编辑的任务记录</div>
  </div>
</template>

<style scoped>
.root {
  width:100%;
  max-width:840px;
  height:100%;
  min-height:0;
  display:flex;
  flex-direction:column;
  gap:var(--space-4);
  margin:0 auto;
  color:var(--text-secondary);
  container-type:inline-size;
}
.slots {
  display:flex;
  flex-wrap:wrap;
  align-items:center;
  justify-content:center;
  gap:var(--space-2);
}
.record-tabs { display:flex; justify-content:center; gap:var(--space-2); }
.record-tabs .ui-tab { min-width:132px; }
.slot-btn.on {
  border-color:var(--selected-border);
  background:var(--selected-bg);
  color:var(--selected-fg);
}
.quests {
  min-width:0;
  min-height:0;
  flex:1;
  overflow:hidden;
  display:flex;
  flex-direction:column;
}
.head,
.batch {
  display:flex;
  align-items:center;
  gap:var(--space-3);
  padding:var(--space-3) var(--space-4);
  border-bottom:1px solid var(--border-soft);
}
.head > span {
  min-width:140px;
  flex:1;
  color:var(--text-secondary);
  font-size:var(--fs-sm);
}
.search { width:210px; }
.batch {
  flex-wrap:wrap;
  border:0;
  border-bottom:1px solid var(--border-soft);
  border-radius:0;
  background:var(--surface-sunken);
  box-shadow:none;
  color:var(--text-secondary);
  font-size:var(--fs-sm);
}
.batch label { display:flex; align-items:center; gap:var(--space-2); }
.batch .number-input { width:112px; margin-left:auto; }
.list {
  min-height:0;
  flex:1;
  overflow-y:auto;
  scrollbar-width:thin;
  scrollbar-color:var(--border-default) transparent;
}
.row {
  min-height:42px;
  display:grid;
  grid-template-columns:20px minmax(54px,72px) minmax(0,1fr) 100px;
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
.id {
  overflow:hidden;
  color:var(--text-secondary);
  font-family:var(--font-data);
  font-size:var(--fs-xs);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.name {
  min-width:0;
  overflow:hidden;
  color:var(--text-primary);
  font-size:var(--fs-md);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.count {
  width:100%;
  text-align:right;
  font-family:var(--font-data);
  font-variant-numeric:tabular-nums;
}
.loading { color:var(--info-ink); }
input[type="checkbox"] { accent-color:var(--accent); }

@container (max-width:620px) {
  .head,
  .batch { align-items:stretch; flex-wrap:wrap; }
  .head > span { width:100%; flex-basis:100%; }
  .search { min-width:0; flex:1; width:auto; }
  .batch .number-input { min-width:90px; flex:1; width:auto; margin-left:0; }
  .batch .save-btn { flex:1; }
  .row {
    grid-template-columns:20px minmax(48px,62px) minmax(0,1fr) 88px;
    padding-inline:var(--space-3);
    gap:var(--space-2);
  }
}

@container (max-width:420px) {
  .slots .ui-btn { flex:1; }
  .head .search,
  .head .ui-btn { width:100%; flex-basis:100%; }
  .row { grid-template-columns:18px 48px minmax(0,1fr) 78px; }
}
</style>
