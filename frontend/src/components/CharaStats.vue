<script setup>
import { reactive, ref, computed } from 'vue'
import { CharaAttach, CharaDetach, CharaGetAll, CharaSetOne, CharaSetAll } from '../../wailsjs/go/main/App'

const connected = ref(false)
const info = reactive({ pid: 0, moduleBase: 0, manager: 0 })
const list = ref([])
const editValues = reactive({})
const batchValue = ref('')
const loading = ref(false)
const sortDesc = ref(false)
const charaEditEnabled = false // 开关：是否显示次数修改功能

const sorted = computed(() => {
  if (!sortDesc.value) return list.value
  return [...list.value].sort((a, b) => b.count - a.count)
})

function connect() {
  loading.value = true
  CharaAttach()
    .then((res) => {
      connected.value = true
      Object.assign(info, res)
      return refresh()
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { loading.value = false })
}

function disconnect() {
  CharaDetach()
    .then(() => {
      connected.value = false
      list.value = []
      Object.assign(info, { pid: 0, moduleBase: 0, manager: 0 })
    })
    .catch((err) => emit('status', String(err), 'error'))
}

function refresh() {
  return CharaGetAll()
    .then((res) => {
      list.value = res || []
      res.forEach(c => {
        if (editValues[c.index] === undefined) editValues[c.index] = String(c.count)
      })
    })
    .catch((err) => emit('status', String(err), 'error'))
}

function setOne(index) {
  const v = parseInt(editValues[index])
  if (isNaN(v) || v < 0) { emit('status', '请输入有效数值', 'error'); return }
  CharaSetOne(index, v)
    .then(() => refresh())
    .then(() => emit('status', '设置成功', 'success'))
    .catch((err) => emit('status', String(err), 'error'))
}

function setBatch() {
  const v = parseInt(batchValue.value)
  if (isNaN(v) || v < 0) { emit('status', '请输入有效数值', 'error'); return }
  CharaSetAll(v)
    .then((n) => refresh().then(() => emit('status', `已设置 ${n} 个角色`, 'success')))
    .catch((err) => emit('status', String(err), 'error'))
}

const emit = defineEmits(['status'])
</script>

<template>
  <div class="root">
    <div class="section">
      <div class="header">
        <span class="title">角色次数统计</span>
        <span class="hint">需游戏运行中使用 · 修改后需对应角色结算一局</span>
      </div>
      <div class="connect-row">
        <button v-if="!connected" class="btn-connect" @click="connect" :disabled="loading">
          {{ loading ? '连接中...' : '连接游戏进程' }}
        </button>
        <button v-else class="btn-disconnect" @click="disconnect">断开连接</button>
        <span v-if="connected" class="pid">PID: {{ info.pid }}</span>
      </div>

      <template v-if="connected && list.length">
        <div v-if="charaEditEnabled" class="batch-row">
          <input v-model="batchValue" type="number" min="0" class="batch-input" placeholder="目标值" />
          <button class="btn-batch" @click="setBatch" :disabled="!batchValue || isNaN(parseInt(batchValue))">全部设置为</button>
          <button class="btn-refresh" @click="refresh">刷新</button>
          <button class="btn-sort" @click="sortDesc = !sortDesc">
            {{ sortDesc ? '恢复原序' : '按次数排序' }}
          </button>
        </div>
        <div v-else class="batch-row">
          <button class="btn-refresh" @click="refresh">刷新</button>
          <button class="btn-sort" @click="sortDesc = !sortDesc">
            {{ sortDesc ? '恢复原序' : '按次数排序' }}
          </button>
        </div>
        <div class="table">
          <div class="row row-head">
            <span class="col-idx">#</span>
            <span class="col-name">角色</span>
            <span class="col-count">次数</span>
            <span v-if="charaEditEnabled" class="col-edit">修改</span>
          </div>
          <div v-for="c in sorted" :key="c.index" class="row">
            <span class="col-idx">{{ c.index }}</span>
            <span class="col-name">{{ c.name }}</span>
            <span class="col-count">{{ c.count }}</span>
            <div v-if="charaEditEnabled" class="col-edit">
              <input v-model="editValues[c.index]" type="number" min="0" class="edit-input" @keyup.enter="setOne(c.index)" />
              <button class="btn-set" @click="setOne(c.index)">设置</button>
            </div>
          </div>
        </div>
      </template>
      <div v-else-if="connected" class="empty">未读取到角色数据，请确保已进入游戏存档</div>
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
.header { display:flex; align-items:center; justify-content:space-between; }
.title { font-size:0.88rem; font-weight:600; color:rgba(255,255,255,0.65); letter-spacing:1px; }
.hint { font-size:0.68rem; color:rgba(255,255,255,0.25); }

.connect-row { display:flex; align-items:center; gap:10px; }
.btn-connect {
  padding:8px 18px; border-radius:8px; border:1px solid rgba(34,197,94,0.4);
  background:rgba(34,197,94,0.12); color:#4ade80; font-size:0.82rem; font-weight:600; cursor:pointer;
  transition:background 0.2s,transform 0.15s;
}
.btn-connect:not(:disabled):hover { background:rgba(34,197,94,0.22); transform:scale(1.02); }
.btn-connect:disabled { opacity:0.5; cursor:not-allowed; }
.btn-disconnect {
  padding:8px 18px; border-radius:8px; border:1px solid rgba(239,68,68,0.4);
  background:rgba(239,68,68,0.12); color:#f87171; font-size:0.82rem; font-weight:600; cursor:pointer;
  transition:background 0.2s;
}
.btn-disconnect:hover { background:rgba(239,68,68,0.22); }
.pid { font-size:0.72rem; color:rgba(255,255,255,0.35); font-family:'Courier New',monospace; }

.batch-row { display:flex; gap:8px; align-items:center; }
.batch-input {
  width:80px; padding:6px 10px; border-radius:6px; border:1px solid rgba(255,255,255,0.15);
  background:rgba(255,255,255,0.07); color:#fff; font-size:0.82rem; outline:none;
}
.batch-input:focus { border-color:rgba(103,232,249,0.5); }
.batch-input::-webkit-outer-spin-button, .batch-input::-webkit-inner-spin-button { -webkit-appearance:none; margin:0; }
.btn-batch {
  padding:6px 14px; border-radius:6px; border:1px solid rgba(165,180,252,0.3);
  background:rgba(165,180,252,0.1); color:#a5b4fc; font-size:0.78rem; font-weight:600; cursor:pointer;
  transition:background 0.2s; white-space:nowrap;
}
.btn-batch:not(:disabled):hover { background:rgba(165,180,252,0.2); }
.btn-batch:disabled { opacity:0.4; cursor:not-allowed; }
.btn-refresh {
  padding:6px 14px; border-radius:6px; border:1px solid rgba(255,255,255,0.12);
  background:rgba(255,255,255,0.05); color:rgba(255,255,255,0.5); font-size:0.78rem; font-weight:600; cursor:pointer;
  transition:background 0.2s;
}
.btn-refresh:hover { background:rgba(255,255,255,0.1); color:rgba(255,255,255,0.7); }
.btn-sort {
  padding:6px 14px; border-radius:6px; border:1px solid rgba(255,255,255,0.12);
  background:rgba(255,255,255,0.05); color:rgba(255,255,255,0.5); font-size:0.78rem; font-weight:600; cursor:pointer;
  transition:background 0.2s;
}
.btn-sort:hover { background:rgba(255,255,255,0.1); color:rgba(255,255,255,0.7); }

.table { display:flex; flex-direction:column; gap:1px; background:rgba(255,255,255,0.04); border-radius:10px; overflow:hidden; }
.row { display:flex; align-items:center; padding:6px 10px; gap:6px; background:rgba(27,38,54,0.6); }
.row-head { background:rgba(255,255,255,0.06); font-size:0.7rem; color:rgba(255,255,255,0.3); font-weight:600; padding:5px 10px; }
.col-idx { width:24px; text-align:center; font-size:0.72rem; color:rgba(255,255,255,0.3); font-family:'Courier New',monospace; flex-shrink:0; }
.col-name { flex:1; font-size:0.8rem; color:rgba(255,255,255,0.6); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.col-count { width:48px; text-align:right; font-size:0.8rem; color:#67e8f9; font-family:'Courier New',monospace; flex-shrink:0; }
.col-edit { width:120px; display:flex; gap:4px; align-items:center; flex-shrink:0; }
.edit-input {
  width:56px; padding:4px 6px; border-radius:4px; border:1px solid rgba(255,255,255,0.12);
  background:rgba(255,255,255,0.06); color:#fff; font-size:0.78rem; outline:none; text-align:center;
}
.edit-input:focus { border-color:rgba(103,232,249,0.4); }
.edit-input::-webkit-outer-spin-button, .edit-input::-webkit-inner-spin-button { -webkit-appearance:none; margin:0; }
.btn-set {
  padding:4px 10px; border-radius:4px; border:1px solid rgba(165,180,252,0.25);
  background:rgba(165,180,252,0.08); color:#a5b4fc; font-size:0.72rem; font-weight:600; cursor:pointer;
  transition:background 0.15s; white-space:nowrap;
}
.btn-set:hover { background:rgba(165,180,252,0.18); }

.empty { font-size:0.78rem; color:rgba(255,255,255,0.3); text-align:center; padding:12px 0; }
</style>
