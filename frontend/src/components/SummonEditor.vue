<script setup>
import { computed, reactive, ref } from 'vue'
import { CharaAttach, SummonGetAll, SummonGetOptions, SummonUpdate } from '../../wailsjs/go/main/App'
import { matchText } from '../utils/matchText.js'
import LegalityIndicator from './LegalityIndicator.vue'

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
const currentSubParam = computed(() => {
  const h = Number.parseInt(String(edit.subParamHash).trim(), 16)
  return Number.isNaN(h) ? null : subParamByHash.value.get(h) || null
})
const subParamMaxLevel = computed(() => {
  const sp = currentSubParam.value
  return sp && Number.isInteger(sp.maxLevel) && sp.maxLevel > 0 ? sp.maxLevel : 9
})
function subParamValueLabel(level) {
  const sp = currentSubParam.value
  const idx = Number.parseInt(level, 10)
  if (!sp || !Array.isArray(sp.values) || idx < 0 || idx >= sp.values.length) return String(idx)
  const v = sp.values[idx]
  return sp.isPercent ? `${idx} → +${v}%` : `${idx} → +${v}`
}
const filteredTraits = computed(() => {
  const query = traitFilter.value.trim()
  if (!query) return options.traits
  return options.traits.filter((item) => matchText(optionLabel(item), query))
})
const filteredSummons = computed(() => {
  const query = filter.value.trim()
  if (!query) return summons.value
  return summons.value.filter((item) => [item.index, nameForType(item.typeHash), nameForTrait(item.mainTraitHash), nameForSubParam(item.subParamHash), hex(item.typeHash)]
    .some((value) => matchText(value, query)))
})
const selected = computed(() => summons.value.find((item) => item.index === selectedIndex.value))
const rarityLabelsByCost = { 3: 'I', 4: 'II', 5: 'III' }

function hex(value) { return '0x' + Number(value || 0).toString(16).toUpperCase().padStart(8, '0') }
function nameForType(hash) { return typeByHash.value.get(hash)?.name || hex(hash) }
function nameForTrait(hash) { return traitByHash.value.get(hash)?.name || (hash ? hex(hash) : '无') }
function nameForSubParam(hash) { return subParamByHash.value.get(hash)?.name || (hash ? hex(hash) : '无') }
function optionLabel(item) { return `${item.name} · ${hex(item.hash)}` }
function rarityLabel(item) {
  const cost = typeByHash.value.get(item.typeHash)?.cost
  return rarityLabelsByCost[cost] || String(item.rank)
}
function parseHash(value, label) {
  const text = String(value).trim()
  const parsed = /^0x/i.test(text) ? Number.parseInt(text, 16) : Number.parseInt(text, 10)
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > 0xFFFFFFFF) throw new Error(`${label}必须是 32 位无符号整数`)
  return parsed >>> 0
}
function parsedHashOrNull(value) {
  try { return parseHash(value, 'Hash') } catch (_) { return null }
}
function traitMax(hash) { return traitByHash.value.get(parseHash(hash, '因子'))?.maxLevel || 999 }

const legality = computed(() => {
  if (!selected.value) return { status: 'impossible', writable: false, message: '请先选择召唤石' }
  const typeHash = parsedHashOrNull(edit.typeHash)
  const mainHash = parsedHashOrNull(edit.mainTraitHash)
  const subHash = parsedHashOrNull(edit.subParamHash)
  const mainLevel = Number.parseInt(edit.mainTraitLevel, 10)
  const subLevel = Number.parseInt(edit.subParamLevel, 10)
  if (typeHash === null || mainHash === null || subHash === null || !Number.isInteger(mainLevel) || !Number.isInteger(subLevel)) {
    return { status: 'impossible', writable: false, message: '存在无法编码的 Hash 或等级' }
  }
  if (typeHash !== (selected.value.typeHash >>> 0)) return { status: 'impossible', writable: false, message: '当前保存函数不支持更换召唤石种类' }
  if (subLevel < 0 || subLevel > subParamMaxLevel.value) return { status: 'impossible', writable: false, message: `副参数档位必须为 0 到 ${subParamMaxLevel.value}，越界会读取错误数值表` }
  if (mainLevel < 0 || mainLevel > 0x7FFFFFFF) return { status: 'impossible', writable: false, message: '主因子等级超出存档可写范围' }
  const reasons = []
  if (!typeByHash.value.has(typeHash)) reasons.push('种类不在本地资料库中')
  if (!mainHash) reasons.push('主因子为空，游戏内通常不会自然生成')
  else if (!traitByHash.value.has(mainHash)) reasons.push('主因子不在本地资料库中')
  if (subHash && !subParamByHash.value.has(subHash)) reasons.push('副参数不在本地资料库中')
  const max = traitByHash.value.get(mainHash)?.maxLevel
  if (Number.isInteger(max) && mainLevel > max) reasons.push(`主因子等级超过已知上限 ${max}`)
  if (reasons.length) return { status: 'forced', writable: true, message: `${reasons.join('；')}；仍会按所选数值写入` }
  return { status: 'unknown', writable: true, message: 'Hash 与等级可写；召唤石的完整天然主因子/副参数词池尚未完全验证' }
})

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
    if (!Number.isInteger(update.mainTraitLevel) || update.mainTraitLevel < 0 || update.mainTraitLevel > 0x7FFFFFFF) throw new Error('主因子等级超出存档可写范围')
    if (!Number.isInteger(update.subParamLevel) || update.subParamLevel < 0 || update.subParamLevel > subParamMaxLevel.value) throw new Error(`副参数等级必须为 0 到 ${subParamMaxLevel.value}`)
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
        <div><h2>召唤石编辑</h2><p>打开游戏内召唤石背包后读取；种类只读，主因子与副词条可以修改。</p></div>
        <span v-if="connected" class="pid">PID {{ pid }}</span>
      </header>
      <div class="toolbar"><button class="primary" @click="load" :disabled="loading">{{ loading ? '读取中...' : connected ? '刷新背包' : '连接背包' }}</button><span v-if="connected" class="count">{{ summons.length }} 颗</span></div>
    </section>

    <section v-if="connected" class="workspace">
      <div class="list-panel">
        <input v-model="filter" class="filter" placeholder="搜索名称或 Hash（更改类型无法写入存档）" />
        <div class="list">
          <button v-for="item in filteredSummons" :key="item.index" class="summon-row" :class="{ selected: item.index === selectedIndex }" @click="select(item)">
            <span class="slot">#{{ item.index + 1 }}</span><span class="name">{{ nameForType(item.typeHash) }}</span><span class="rank">{{ rarityLabel(item) }}</span>
          </button>
          <p v-if="!summons.length" class="empty">未读取到召唤石。请打开游戏内召唤石背包后刷新。</p>
        </div>
      </div>

      <div class="editor-panel">
        <template v-if="selected">
          <div class="editor-head"><strong>{{ nameForType(selected.typeHash) }}</strong><span>#{{ selected.index + 1 }}</span></div>
          <div class="field-group"><strong>召唤石信息</strong><label>种类<select v-model="edit.typeHash" class="type-select" disabled><option v-for="item in options.types" :key="item.hash" :value="hex(item.hash)">{{ optionLabel(item) }}</option></select></label><label>稀有度<span class="readonly-value">{{ rarityLabel(selected) }}</span></label></div>
          <div class="field-group"><strong>主因子</strong><label>搜索<input v-model="traitFilter" placeholder="按名称或 Hash 筛选" /></label><label>因子<select v-model="edit.mainTraitHash"><option v-for="item in filteredTraits" :key="item.hash" :value="hex(item.hash)">{{ optionLabel(item) }}</option></select></label>
          <label>主因子等级<span class="number-combo"><input v-model="edit.mainTraitLevel" type="number" min="0" :max="traitMax(edit.mainTraitHash)" /><button type="button" @click="edit.mainTraitLevel=String(traitMax(edit.mainTraitHash))">最大</button></span></label>
          </div><div class="field-group"><strong>副词条</strong><label>副词条<select v-model="edit.subParamHash"><option v-for="item in options.subParams" :key="item.hash" :value="hex(item.hash)">{{ optionLabel(item) }}</option></select></label>
          <label>副参数等级<span class="number-combo"><select v-model="edit.subParamLevel"><option v-for="level in (subParamMaxLevel + 1)" :key="level - 1" :value="String(level - 1)">{{ subParamValueLabel(level - 1) }}</option></select><button type="button" @click="edit.subParamLevel=String(subParamMaxLevel)">最大</button></span></label>
          </div>
          <div class="save-row">
            <LegalityIndicator :status="legality.status" :message="legality.message" />
            <button class="save" @click="save" :disabled="saving || !legality.writable">{{ saving ? '写入中...' : '写入召唤石' }}</button>
          </div>
        </template>
        <p v-else class="empty">从左侧选择召唤石。</p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.root{width:100%;max-width:none;display:flex;flex-direction:column;gap:12px;padding-bottom:28px;container-type:inline-size}.section,.list-panel,.editor-panel{border:1px solid rgba(255,255,255,.08);background:rgba(255,255,255,.025);border-radius:8px}.section{padding:14px 16px}.header{display:flex;justify-content:space-between;gap:12px;align-items:start}.header h2{margin:0;color:rgba(255,255,255,.72);font-size:.95rem}.header p,.pid,.count{margin:5px 0 0;color:rgba(255,255,255,.35);font-size:.72rem}.pid{font-family:var(--font-data)}.toolbar{display:flex;align-items:center;gap:10px;margin-top:12px;flex-wrap:wrap}.primary,.save{border:1px solid rgba(154,116,64,.35);background:rgba(154,116,64,.12);color:#9a7440;border-radius:6px;padding:7px 13px;font-weight:600;cursor:pointer}.primary:disabled,.save:disabled{opacity:.45;cursor:not-allowed}.workspace{display:grid;grid-template-columns:minmax(340px,1fr) minmax(430px,1fr);gap:12px;min-height:390px}.list-panel{padding:10px;display:flex;flex-direction:column;min-width:0}.filter,label input,label select{box-sizing:border-box;border:1px solid rgba(255,255,255,.14);border-radius:6px;background:rgba(255,255,255,.06);color:#fff;outline:none}.filter{padding:8px 10px;width:100%;margin-bottom:8px}.list{overflow:auto;max-height:430px;scrollbar-width:thin;scrollbar-color:rgba(154,116,64,.35) rgba(255,255,255,.04)}.list::-webkit-scrollbar{width:8px}.list::-webkit-scrollbar-track{background:rgba(255,255,255,.04)}.list::-webkit-scrollbar-thumb{background:rgba(154,116,64,.28);border-radius:4px}.list::-webkit-scrollbar-thumb:hover{background:rgba(154,116,64,.48)}.summon-row{width:100%;border:0;border-bottom:1px solid rgba(255,255,255,.045);background:transparent;color:rgba(255,255,255,.6);padding:8px;display:grid;grid-template-columns:42px minmax(0,1fr) 28px;text-align:left;cursor:pointer}.summon-row:hover,.summon-row.selected{background:rgba(154,116,64,.1)}.slot,.rank{font-size:.72rem;color:rgba(255,255,255,.35)}.name{font-size:.78rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.editor-panel{padding:15px;display:flex;flex-direction:column;gap:9px;min-width:0}.editor-head{display:flex;justify-content:space-between;gap:10px;color:rgba(255,255,255,.65);font-size:.78rem;padding-bottom:5px;border-bottom:1px solid rgba(255,255,255,.08)}.editor-head strong{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.editor-head span{color:rgba(255,255,255,.28);font-size:.68rem;flex:none}label{display:grid;grid-template-columns:112px minmax(0,1fr);align-items:center;gap:10px;color:rgba(255,255,255,.45);font-size:.75rem}label input,label select{padding:7px 9px;min-width:0;width:100%}.type-select{appearance:none}label select{font-size:.75rem}label select option{background:#111c2b;color:#fff}.save{margin-top:7px;align-self:end}.empty{color:rgba(255,255,255,.3);font-size:.76rem;text-align:center;padding:18px 5px}@media(max-width:840px){.workspace{grid-template-columns:minmax(280px,1fr) minmax(320px,1fr)}label{grid-template-columns:96px minmax(0,1fr)}}@media(max-width:680px){.workspace{grid-template-columns:1fr}.list{max-height:220px}}
@container(max-width:680px){.workspace{grid-template-columns:minmax(180px,.8fr) minmax(0,1.2fr)}label{grid-template-columns:88px minmax(0,1fr)}}
@container(max-width:460px){.workspace{grid-template-columns:1fr}.list{max-height:220px}.save-row{align-items:stretch;flex-direction:column}.save-row .save{width:100%}}
.save-row{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-top:7px}.save-row .save{margin-top:0;flex:none}
.number-combo{display:grid;grid-template-columns:minmax(0,1fr) 48px;gap:5px}.number-combo button{border:1px solid rgba(218,187,115,.27);background:rgba(218,187,115,.07);color:#d9bd7c;font-size:10px;cursor:pointer}.section,.list-panel,.editor-panel{border-color:rgba(154,202,224,.14);background:rgba(8,31,53,.7);border-radius:4px 12px 4px 12px}.summon-row.selected{background:linear-gradient(90deg,rgba(218,187,115,.1),rgba(90,177,210,.05));box-shadow:inset 2px 0 var(--gold)}.rank{color:var(--gold)}.primary,.save{border-color:rgba(218,187,115,.38);background:rgba(218,187,115,.09);color:#f0d99d}
.section,.list-panel,.editor-panel{color:#594a38;border-color:rgba(130,91,40,.36);background:#f2e3bd;border-radius:2px 7px 2px 7px}.header h2,.editor-head,.editor-head strong{color:#4d4032}.header p,.pid,.count,.slot,.editor-head span,.empty{color:#89765c}.filter,label input,label select,.readonly-value{border-color:rgba(130,91,40,.3);background:#fff8e2;color:#554636}.filter::placeholder,label input::placeholder{color:#9a876b}.list{scrollbar-color:rgba(137,96,44,.62) transparent}.list::-webkit-scrollbar-track{background:transparent}.list::-webkit-scrollbar-thumb{background:rgba(137,96,44,.62)}.summon-row{color:#5c4d3b;border-bottom-color:rgba(130,91,40,.16)}.summon-row:hover{background:#ead8af}.summon-row.selected{background:#dfc796;box-shadow:inset 3px 0 #916633}.rank{color:#805b2d;font-weight:800}.primary,.save{border-color:#765127;background:linear-gradient(180deg,#ad8245,#815a2c);color:#fff9e8}.editor-panel{gap:10px}.field-group{display:flex;flex-direction:column;gap:8px;padding:10px 11px;border:1px solid rgba(131,92,40,.22);background:#ead7aa}.field-group>strong{padding-bottom:7px;border-bottom:1px solid rgba(130,91,40,.18);color:#5c472e;font-size:11px}.field-group label{color:#705e47}.readonly-value{display:flex;align-items:center;min-height:31px;padding:0 9px;box-sizing:border-box}.number-combo button{border-color:rgba(128,89,39,.34);background:#e3c98f;color:#684821;font-weight:700}.editor-head{border-bottom-color:rgba(130,91,40,.2)}
</style>
