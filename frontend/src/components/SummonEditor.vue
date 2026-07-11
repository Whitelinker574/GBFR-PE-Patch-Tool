<script setup>
import { computed, reactive, ref } from 'vue'
import { CharaAttach, SummonGetAll, SummonGetOptions, SummonUpdate } from '../../wailsjs/go/main/App'

const emit = defineEmits(['status'])
const connected = ref(false)
const loading = ref(false)
const saving = ref(false)
const pid = ref(0)
const summons = ref([])
const options = reactive({ types: [], traits: [], subParams: [] })
const filter = ref('')
const traitFilter = ref('')
const selectedIndex = ref(-1)
const edit = reactive({ typeHash: '', mainTraitHash: '', subParamHash: '', mainTraitLevel: '1', subParamLevel: '1', rank: '1' })

const typeByHash = computed(() => new Map(options.types.map((item) => [item.hash, item])))
const traitByHash = computed(() => new Map(options.traits.map((item) => [item.hash, item])))
const subParamByHash = computed(() => new Map(options.subParams.map((item) => [item.hash, item])))
const filteredTraits = computed(() => {
  const query = traitFilter.value.trim().toLowerCase()
  if (!query) return options.traits
  return options.traits.filter((item) => optionLabel(item).toLowerCase().includes(query))
})
const filteredSummons = computed(() => {
  const query = filter.value.trim().toLowerCase()
  if (!query) return summons.value
  return summons.value.filter((item) => [item.index, nameForType(item.typeHash), nameForTrait(item.mainTraitHash), nameForSubParam(item.subParamHash), hex(item.typeHash)]
    .some((value) => String(value).toLowerCase().includes(query)))
})
const selected = computed(() => summons.value.find((item) => item.index === selectedIndex.value))

function hex(value) { return '0x' + Number(value || 0).toString(16).toUpperCase().padStart(8, '0') }
function nameForType(hash) { return typeByHash.value.get(hash)?.name || hex(hash) }
function nameForTrait(hash) { return traitByHash.value.get(hash)?.name || (hash ? hex(hash) : '无') }
function nameForSubParam(hash) { return subParamByHash.value.get(hash)?.name || (hash ? hex(hash) : '无') }
function optionLabel(item) { return `${item.name} · ${hex(item.hash)}` }
function parseHash(value, label) {
  const text = String(value).trim()
  const parsed = /^0x/i.test(text) ? Number.parseInt(text, 16) : Number.parseInt(text, 10)
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > 0xFFFFFFFF) throw new Error(`${label}必须是 32 位无符号整数`)
  return parsed >>> 0
}
function traitMax(hash) { return traitByHash.value.get(parseHash(hash, '因子'))?.maxLevel || 999 }

function load() {
  loading.value = true
  return CharaAttach()
    .then((info) => { connected.value = true; pid.value = info.pid; return Promise.all([SummonGetOptions(), SummonGetAll()]) })
    .then(([catalog, items]) => {
      Object.assign(options, catalog || { types: [], traits: [], subParams: [] })
      summons.value = Array.isArray(items) ? items : []
      if (selectedIndex.value >= 0) select(summons.value.find((item) => item.index === selectedIndex.value))
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { loading.value = false })
}

function select(item) {
  if (!item) return
  selectedIndex.value = item.index
  edit.typeHash = hex(item.typeHash)
  edit.mainTraitHash = hex(item.mainTraitHash)
  edit.subParamHash = hex(item.subParamHash)
  edit.mainTraitLevel = String(item.mainTraitLevel)
  edit.subParamLevel = String(item.subParamLevel)
  edit.rank = String(item.rank)
}

function save() {
  if (!selected.value) { emit('status', '请选择召唤石', 'error'); return }
  let update
  try {
    const mainTraitHash = parseHash(edit.mainTraitHash, '主因子')
    const subParamHash = parseHash(edit.subParamHash, '副参数')
    update = {
      index: selected.value.index,
      typeHash: parseHash(edit.typeHash, '种类'),
      mainTraitHash,
      subParamHash,
      mainTraitLevel: Number.parseInt(edit.mainTraitLevel, 10),
      subParamLevel: Number.parseInt(edit.subParamLevel, 10),
      rank: Number.parseInt(edit.rank, 10),
    }
    if (!Number.isInteger(update.mainTraitLevel) || update.mainTraitLevel < 0 || update.mainTraitLevel > traitMax(mainTraitHash)) throw new Error(`主因子等级必须为 0 到 ${traitMax(mainTraitHash)}`)
    if (!Number.isInteger(update.subParamLevel) || update.subParamLevel < 0 || update.subParamLevel > 999) throw new Error('副参数等级必须为 0 到 999')
  } catch (err) {
    emit('status', String(err.message || err), 'error')
    return
  }
  saving.value = true
  SummonUpdate(update)
    .then((updated) => {
      const index = summons.value.findIndex((item) => item.index === updated.index)
      if (index >= 0) summons.value.splice(index, 1, updated)
      select(updated)
      emit('status', '召唤石已写入并保存', 'success')
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { saving.value = false })
}
</script>

<template>
  <div class="root">
    <section class="section">
      <header class="header">
        <div><h2>召唤石工坊</h2><p>打开游戏内召唤石背包后读取；写入会调用游戏保存函数。</p></div>
        <span v-if="connected" class="pid">PID {{ pid }}</span>
      </header>
      <div class="toolbar"><button class="primary" @click="load" :disabled="loading">{{ loading ? '读取中...' : connected ? '刷新背包' : '连接背包' }}</button><span v-if="connected" class="count">{{ summons.length }} 颗</span></div>
    </section>

    <section v-if="connected" class="workspace">
      <div class="list-panel">
        <input v-model="filter" class="filter" placeholder="搜索名称或 Hash（更改类型无法写入存档）" />
        <div class="list">
          <button v-for="item in filteredSummons" :key="item.index" class="summon-row" :class="{ selected: item.index === selectedIndex }" @click="select(item)">
            <span class="slot">#{{ item.index + 1 }}</span><span class="name">{{ nameForType(item.typeHash) }}</span><span class="rank">{{ ['-', 'I', 'II', 'III'][item.rank] || item.rank }}</span>
          </button>
          <p v-if="!summons.length" class="empty">未读取到召唤石。请打开游戏内召唤石背包后刷新。</p>
        </div>
      </div>

      <div class="editor-panel">
        <template v-if="selected">
          <div class="editor-head"><strong>{{ nameForType(selected.typeHash) }}</strong><span>#{{ selected.index + 1 }}</span></div>
          <label>种类<select v-model="edit.typeHash" class="type-select" disabled><option v-for="item in options.types" :key="item.hash" :value="hex(item.hash)">{{ optionLabel(item) }}</option></select></label>
          <label>主因子搜索<input v-model="traitFilter" placeholder="名称或 Hash" /></label>
          <label>主因子<select v-model="edit.mainTraitHash"><option v-for="item in filteredTraits" :key="item.hash" :value="hex(item.hash)">{{ optionLabel(item) }}</option></select></label>
          <label>主因子等级<input v-model="edit.mainTraitLevel" type="number" min="0" :max="traitMax(edit.mainTraitHash)" /></label>
          <label>副参数<select v-model="edit.subParamHash"><option v-for="item in options.subParams" :key="item.hash" :value="hex(item.hash)">{{ optionLabel(item) }}</option></select></label>
          <label>副参数等级<select v-model="edit.subParamLevel"><option v-for="level in 10" :key="level - 1" :value="String(level - 1)">{{ level - 1 }}</option></select></label>
          <button class="save" @click="save" :disabled="saving">{{ saving ? '写入中...' : '写入召唤石' }}</button>
        </template>
        <p v-else class="empty">从左侧选择召唤石。</p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.root{width:100%;max-width:none;display:flex;flex-direction:column;gap:12px;padding-bottom:28px}.section,.list-panel,.editor-panel{border:1px solid rgba(255,255,255,.08);background:rgba(255,255,255,.025);border-radius:8px}.section{padding:14px 16px}.header{display:flex;justify-content:space-between;gap:12px;align-items:start}.header h2{margin:0;color:rgba(255,255,255,.72);font-size:.95rem}.header p,.pid,.count{margin:5px 0 0;color:rgba(255,255,255,.35);font-size:.72rem}.pid{font-family:'Courier New',monospace}.toolbar{display:flex;align-items:center;gap:10px;margin-top:12px}.primary,.save{border:1px solid rgba(103,232,249,.35);background:rgba(103,232,249,.12);color:#67e8f9;border-radius:6px;padding:7px 13px;font-weight:600;cursor:pointer}.primary:disabled,.save:disabled{opacity:.45;cursor:not-allowed}.workspace{display:grid;grid-template-columns:minmax(340px,1fr) minmax(430px,1fr);gap:12px;min-height:390px}.list-panel{padding:10px;display:flex;flex-direction:column;min-width:0}.filter,label input,label select{box-sizing:border-box;border:1px solid rgba(255,255,255,.14);border-radius:6px;background:rgba(255,255,255,.06);color:#fff;outline:none}.filter{padding:8px 10px;width:100%;margin-bottom:8px}.list{overflow:auto;max-height:430px;scrollbar-width:thin;scrollbar-color:rgba(103,232,249,.35) rgba(255,255,255,.04)}.list::-webkit-scrollbar{width:8px}.list::-webkit-scrollbar-track{background:rgba(255,255,255,.04)}.list::-webkit-scrollbar-thumb{background:rgba(103,232,249,.28);border-radius:4px}.list::-webkit-scrollbar-thumb:hover{background:rgba(103,232,249,.48)}.summon-row{width:100%;border:0;border-bottom:1px solid rgba(255,255,255,.045);background:transparent;color:rgba(255,255,255,.6);padding:8px;display:grid;grid-template-columns:42px minmax(0,1fr) 28px;text-align:left;cursor:pointer}.summon-row:hover,.summon-row.selected{background:rgba(103,232,249,.1)}.slot,.rank{font-size:.72rem;color:rgba(255,255,255,.35)}.name{font-size:.78rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.editor-panel{padding:15px;display:flex;flex-direction:column;gap:9px;min-width:0}.editor-head{display:flex;justify-content:space-between;gap:10px;color:rgba(255,255,255,.65);font-size:.78rem;padding-bottom:5px;border-bottom:1px solid rgba(255,255,255,.08)}.editor-head strong{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.editor-head span{color:rgba(255,255,255,.28);font-size:.68rem;flex:none}label{display:grid;grid-template-columns:112px minmax(0,1fr);align-items:center;gap:10px;color:rgba(255,255,255,.45);font-size:.75rem}label input,label select{padding:7px 9px;min-width:0;width:100%}.type-select{appearance:none}label select{font-size:.75rem}label select option{background:#111c2b;color:#fff}.save{margin-top:7px;align-self:end}.empty{color:rgba(255,255,255,.3);font-size:.76rem;text-align:center;padding:18px 5px}@media(max-width:840px){.workspace{grid-template-columns:minmax(280px,1fr) minmax(320px,1fr)}label{grid-template-columns:96px minmax(0,1fr)}}@media(max-width:680px){.workspace{grid-template-columns:1fr}.list{max-height:220px}}
</style>
