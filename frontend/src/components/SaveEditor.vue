<script setup>
import { ref, computed } from 'vue'
import { FindSaveFiles, GetQuests, LoadSave, UpdateQuestCounts } from '../../wailsjs/go/main/App'

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

scanSaves()
</script>

<template>
  <div class="root">
    <div class="slots">
      <button v-for="s in slots" :key="s.index" class="slot-btn" :class="{ on: savePath === s.path }" @click="load(s.path)">{{ saveSlotLabel(s) }}</button>
      <button class="plain-btn" @click="scanSaves">刷新</button>
    </div>

    <div v-if="loading" class="loading">解析中…</div>
    <div v-else-if="quests.length" class="quests">
      <div class="head">
        <span>{{ quests.length }} 个任务 · {{ total }} 次挑战</span>
        <input v-model="search" class="search" placeholder="搜索任务或 ID">
        <button class="plain-btn" @click="sortDesc = !sortDesc">{{ sortDesc ? '次数排序' : '默认顺序' }}</button>
      </div>
      <div class="batch">
        <label><input type="checkbox" :checked="allVisibleSelected" @change="toggleVisible"> 选择当前结果</label>
        <input v-model.number="batchCount" class="number-input" type="number" min="0" max="99999999">
        <button class="plain-btn max-btn" @click="batchCount=99999999">最大</button>
        <button class="plain-btn" :disabled="!selected.size" @click="applyBatch">填入已选</button>
        <span>已选 {{ selected.size }} 项</span>
        <button class="save-btn" :disabled="saving || !selected.size" @click="saveSelected">{{ saving ? '写入中…' : '保存已选' }}</button>
      </div>
      <div class="list">
        <label v-for="q in visibleQuests" :key="q.index" class="row">
          <input type="checkbox" :checked="selected.has(q.index)" @change="toggle(q.index)">
          <span class="id">{{ q.questCode || q.questId }}</span>
          <span class="name">{{ q.questNameCn || q.questName }}</span>
          <input v-model.number="q.clears" class="number-input count" type="number" min="0" max="99999999" @focus="selected.add(q.index)">
        </label>
      </div>
      <div class="foot">写入前请完全退出游戏；每次保存都会创建时间戳备份并回读验证。</div>
    </div>
  </div>
</template>

<style scoped>
.root { display:flex; flex-direction:column; gap:10px; width:100%; max-width:720px; height:100%; min-height:0; margin:0 auto; container-type:inline-size; }
.slots { display:flex; gap:8px; flex-wrap:wrap; justify-content:center; align-items:center; }
.slot-btn,.plain-btn,.save-btn { padding:7px 13px; border-radius:7px; border:1px solid rgba(255,255,255,.11); background:rgba(255,255,255,.045); color:rgba(255,255,255,.5); font:inherit; font-size:.74rem; cursor:pointer; }
.slot-btn { padding:9px 17px; }
.slot-btn.on { border-color:rgba(103,232,249,.42); background:rgba(103,232,249,.1); color:#67e8f9; }
button:hover:not(:disabled) { color:rgba(255,255,255,.78); background:rgba(255,255,255,.08); }
button:disabled { opacity:.35; cursor:not-allowed; }
.loading { text-align:center; color:#67e8f9; font-size:.82rem; padding:16px; }
.quests { border-radius:12px; border:1px solid rgba(255,255,255,.07); background:rgba(255,255,255,.025); overflow:hidden; flex:1; min-height:0; display:flex; flex-direction:column; }
.head,.batch { display:flex; align-items:center; gap:9px; padding:9px 12px; border-bottom:1px solid rgba(255,255,255,.055); }
.head>span { flex:1; font-size:.72rem; color:rgba(255,255,255,.38); }
.search,.number-input { box-sizing:border-box; padding:6px 8px; border-radius:6px; border:1px solid rgba(255,255,255,.12); background:rgba(255,255,255,.055); color:#d8f6fb; outline:none; font:inherit; font-size:.73rem; }
.search { width:180px; }
.search:focus,.number-input:focus { border-color:rgba(103,232,249,.45); }
.batch { background:rgba(255,255,255,.018); font-size:.7rem; color:rgba(255,255,255,.35); }
.batch .number-input { width:100px; margin-left:auto; }
.save-btn { border-color:rgba(74,222,128,.28); background:rgba(34,197,94,.11); color:#4ade80; }
.list { flex:1; min-height:0; overflow-y:auto; scrollbar-width:thin; scrollbar-color:rgba(255,255,255,.1) transparent; }
.row { min-height:39px; display:flex; align-items:center; gap:9px; padding:4px 12px; border-bottom:1px solid rgba(255,255,255,.03); box-sizing:border-box; }
.row:hover { background:rgba(255,255,255,.025); }
.id { width:52px; font-size:.66rem; color:rgba(255,255,255,.24); font-family:var(--font-data); }
.name { flex:1; font-size:.78rem; color:rgba(255,255,255,.58); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.count { width:86px; text-align:right; color:#8be9f7; font-family:var(--font-data); }
.foot { padding:8px 12px; border-top:1px solid rgba(255,255,255,.05); font-size:.65rem; color:rgba(74,222,128,.58); }
input[type=checkbox] { accent-color:#67e8f9; }
@container (max-width:460px) { .head,.batch { flex-wrap:wrap; } .search { width:100%; order:3; } .batch .number-input { margin-left:0; flex:1; min-width:90px; } .batch .save-btn { flex:1; } }
</style>
