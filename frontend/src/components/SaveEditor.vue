<script setup>
import { ref, computed } from 'vue'
import { FindSaveFiles, GetQuests, LoadSave } from '../../wailsjs/go/main/App'

const slots = ref([])
const quests = ref([])
const total = ref(0)
const loading = ref(false)
const savePath = ref('')
const sortDesc = ref(true)

const sortedQuests = computed(() => {
  if (!sortDesc.value) return quests.value
  return [...quests.value].sort((a, b) => b.clears - a.clears)
})

async function scanSaves() {
  slots.value = await FindSaveFiles() || []
}

async function load(path) {
  loading.value = true
  savePath.value = path
  try {
    const [summary, qs] = await Promise.all([LoadSave(path), GetQuests(path)])
    quests.value = qs || []
    total.value = summary?.questTotalClears || 0
  } catch (err) { console.error(err) } finally { loading.value = false }
}

scanSaves()
</script>

<template>
  <div class="root">
    <!-- 存档选择 -->
    <div class="slots">
      <button v-for="s in slots" :key="s.index" class="slot-btn"
        :class="{ on: savePath === s.path }" @click="load(s.path)">
        {{ s.name }}
      </button>
      <button class="refresh" @click="scanSaves">刷新</button>
    </div>

    <div v-if="loading" class="loading">解析中...</div>

    <div v-else-if="quests.length" class="quests">
      <div class="head">
        <span>{{ quests.length }} 个任务 · {{ total }} 次挑战</span>
        <button class="refresh" @click="load(savePath)">刷新</button>
        <button class="sort" @click="sortDesc = !sortDesc">{{ sortDesc ? '↓次数' : '↑默认' }}</button>
      </div>
      <div class="list">
        <div v-for="q in sortedQuests" :key="q.questId" class="row">
          <span class="id">{{ q.questId }}</span>
          <span class="name">{{ q.questNameCn || q.questName }}</span>
          <span class="count" :class="{ hot: q.clears > 100 }">{{ q.clears }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.root { display:flex; flex-direction:column; gap:10px; width:100%; max-width:720px; height:100%; min-height:0; margin:0 auto; padding-bottom:0; box-sizing:border-box; }
.slots { display:flex; gap:8px; flex-wrap:wrap; justify-content:center; align-items:center; }
.slot-btn {
  padding:10px 20px; border-radius:10px; border:1px solid rgba(255,255,255,0.1);
  background:rgba(255,255,255,0.04); color:rgba(255,255,255,0.45);
  font-size:0.82rem; font-family:inherit; cursor:pointer; transition:all 0.2s;
}
.slot-btn:hover { border-color:rgba(103,232,249,0.2); color:rgba(255,255,255,0.7); }
.slot-btn.on { border-color:rgba(103,232,249,0.4); background:rgba(103,232,249,0.1); color:#67e8f9; }
.refresh {
  padding:6px 14px; border-radius:6px; border:1px solid rgba(255,255,255,0.08);
  background:transparent; color:rgba(255,255,255,0.3); font-size:0.75rem; cursor:pointer;
}
.refresh:hover { color:rgba(255,255,255,0.6); border-color:rgba(255,255,255,0.15); }

.loading { text-align:center; color:#67e8f9; font-size:0.82rem; padding:16px; }

.quests { border-radius:12px; border:1px solid rgba(255,255,255,0.06); background:rgba(255,255,255,0.02); overflow:hidden; flex:1; min-height:0; display:flex; flex-direction:column; }
.head { display:flex; align-items:center; padding:10px 14px; background:rgba(255,255,255,0.03); border-bottom:1px solid rgba(255,255,255,0.05); gap:10px; }
.head span { font-size:0.75rem; color:rgba(255,255,255,0.35); flex:1; }
.sort {
  padding:3px 10px; border-radius:4px; border:1px solid rgba(255,255,255,0.1);
  background:transparent; color:rgba(255,255,255,0.3); font-size:0.7rem; cursor:pointer;
}
.sort:hover { color:#67e8f9; border-color:rgba(103,232,249,0.3); }

.list { flex:1; min-height:0; overflow-y:auto; scrollbar-width:thin; scrollbar-color:rgba(255,255,255,0.08) transparent; }
.row { display:flex; align-items:center; gap:8px; padding:7px 14px; border-bottom:1px solid rgba(255,255,255,0.02); }
.row:hover { background:rgba(255,255,255,0.02); }
.id { width:48px; font-size:0.68rem; color:rgba(255,255,255,0.2); font-family:'Courier New',monospace; flex-shrink:0; }
.name { flex:1; font-size:0.8rem; color:rgba(255,255,255,0.5); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.count { width:40px; text-align:right; font-size:0.8rem; font-weight:600; color:rgba(255,255,255,0.35); font-family:'Courier New',monospace; flex-shrink:0; }
.count.hot { color:#fbbf24; }
</style>
