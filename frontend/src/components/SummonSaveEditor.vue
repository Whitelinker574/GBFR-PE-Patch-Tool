<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { FindSaveFiles, GetLastSavePath, SetLastSavePath } from '../../wailsjs/go/main/App'
import { Apply, GetOptions, LoadSaveFile, SelectInputSave, SelectOutputSave } from '../../wailsjs/go/main/SummonSaveGen'
import { summonAssetIcon, traitAssetIcon } from '../gameAssetIcons'
import ConfirmDialog from './ConfirmDialog.vue'
import SaveSourcePicker from './SaveSourcePicker.vue'

const emit = defineEmits(['status'])
const confirmDialog = ref(null)
const loading = ref(false)
const writing = ref(false)
const inputPath = ref('')
const outputPath = ref('')
const inPlace = ref(true)
const saveSlots = ref([])
const info = reactive({ path: '', inventory: { unlocked: false, capacity: 0, occupied: 0, maxSlotId: 0, records: [] } })
const options = reactive({ types: [], traits: [], subParams: [] })
const mode = ref('update')
const filter = ref('')
const selectedUnitID = ref(-1)
const form = reactive({ typeHash: 0, mainTraitHash: 0, subParamHash: 0, mainTraitLevel: 1, subParamLevel: 0, rank: 2 })

function hex(value) { return `0x${(Number(value || 0) >>> 0).toString(16).toUpperCase().padStart(8, '0')}` }
function defaultOutput(path) { return path ? path.replace(/\.dat$/i, '_summons.dat') : '' }
function typeName(hash) { return options.types.find(item => (item.hash >>> 0) === (hash >>> 0))?.name || hex(hash) }
function traitName(hash) { return options.traits.find(item => (item.hash >>> 0) === (hash >>> 0))?.name || hex(hash) }
function subName(hash) { return options.subParams.find(item => (item.hash >>> 0) === (hash >>> 0))?.name || hex(hash) }
function typeIcon(hash) { return summonAssetIcon({ typeHash: hash }) }
function mainIcon(hash) { return traitAssetIcon({ hash, name: traitName(hash) }) }
function optionLabel(item) { return `${item.name} · ${hex(item.hash)}` }

const rulesByType = computed(() => new Map((options.rules || []).map(rule => [Number.parseInt(rule.typeHash, 16) >>> 0, rule])))
const currentRule = computed(() => rulesByType.value.get(Number(form.typeHash) >>> 0) || null)
const allowedMainHashes = computed(() => new Set((currentRule.value?.mainTraitHashes || []).map(value => Number.parseInt(value, 16) >>> 0)))
const allowedSubHashes = computed(() => new Set((currentRule.value?.subParamHashes || []).map(value => Number.parseInt(value, 16) >>> 0)))
const mainChoices = computed(() => options.traits)
const subChoices = computed(() => options.subParams)
const naturalMainLevels = computed(() => currentRule.value?.mainTraitLevels || [])
const naturalSubLevels = computed(() => currentRule.value?.subParamLevels || [])
const fixedRule = computed(() => currentRule.value?.mode === '固定')
const currentFieldsUnchanged = computed(() => !!selected.value &&
  (form.typeHash >>> 0) === (selected.value.state.typeHash >>> 0) &&
  (form.mainTraitHash >>> 0) === (selected.value.state.mainTraitHash >>> 0) && Number(form.mainTraitLevel) === Number(selected.value.state.mainTraitLevel) &&
  (form.subParamHash >>> 0) === (selected.value.state.subParamHash >>> 0) && Number(form.subParamLevel) === Number(selected.value.state.subParamLevel))

const records = computed(() => Array.isArray(info.inventory?.records) ? info.inventory.records : [])
const selected = computed(() => records.value.find(item => Number(item.unitId) === Number(selectedUnitID.value)) || null)
const filteredRecords = computed(() => {
  const query = filter.value.trim().toLowerCase()
  if (!query) return records.value
  return records.value.filter(item => `${typeName(item.state.typeHash)} ${hex(item.state.typeHash)} ${item.slotId}`.toLowerCase().includes(query))
})
const subOption = computed(() => options.subParams.find(item => (item.hash >>> 0) === (form.subParamHash >>> 0)))
const validation = computed(() => {
  if (mode.value === 'update' && !selected.value) return '请选择一颗已有召唤石。'
  if (!options.types.some(item => (item.hash >>> 0) === (form.typeHash >>> 0))) return '召唤石种类不在已审计目录。'
  if (!options.traits.some(item => (item.hash >>> 0) === (form.mainTraitHash >>> 0))) return '主加护不在已审计目录。'
  if (!options.subParams.some(item => (item.hash >>> 0) === (form.subParamHash >>> 0))) return '副词条不在已审计目录。'
  if (!Number.isInteger(Number(form.mainTraitLevel)) || Number(form.mainTraitLevel) < 0 || Number(form.mainTraitLevel) > 0xFFFFFFFF) return '主加护等级无法编码为 uint32。'
  if (!Number.isInteger(Number(form.subParamLevel)) || Number(form.subParamLevel) < 0 || Number(form.subParamLevel) > 0xFFFFFFFF) return '副词条等级无法编码为 uint32。'
  if (!Number.isInteger(Number(form.rank)) || Number(form.rank) < 0 || Number(form.rank) > 0xFFFFFFFF) return 'Rank 无法编码为 uint32。'
  if (!outputPath.value.trim()) return '请选择输出存档。'
  return ''
})

function syncForm(state) {
  if (state) { Object.assign(form, state); return }
  const rule = (options.rules || []).find(item => item.mode === '随机' && item.mainTraitHashes?.length && item.subParamHashes?.length)
  Object.assign(form, {
    typeHash: rule ? Number.parseInt(rule.typeHash, 16) >>> 0 : options.types[0]?.hash || 0,
    mainTraitHash: rule ? Number.parseInt(rule.mainTraitHashes[0], 16) >>> 0 : options.traits[0]?.hash || 0,
    subParamHash: rule ? Number.parseInt(rule.subParamHashes[0], 16) >>> 0 : options.subParams[0]?.hash || 0,
    mainTraitLevel: Number(rule?.mainTraitLevels?.[0] || 1), subParamLevel: Number(rule?.subParamLevels?.[0] || 0), rank: 2,
  })
}

function onTypeChanged() {
  const rule = currentRule.value
  if (!rule) return
  form.mainTraitHash = Number.parseInt(rule.mainTraitHashes?.[0] || '0', 16) >>> 0
  form.subParamHash = Number.parseInt(rule.subParamHashes?.[0] || '0', 16) >>> 0
  form.mainTraitLevel = Number(rule.mainTraitLevels?.[0] ?? 0)
  form.subParamLevel = Number(rule.subParamLevels?.[0] ?? 0)
}

function selectRecord(record) {
  if (!record) return
  mode.value = 'update'
  selectedUnitID.value = Number(record.unitId)
  syncForm(record.state)
}

function beginCreate() {
  mode.value = 'create'
  selectedUnitID.value = -1
  syncForm(null)
}

function applyInfo(next) {
  Object.assign(info, next || {})
  if (mode.value === 'update') {
    const record = records.value.find(item => Number(item.unitId) === Number(selectedUnitID.value)) || records.value[0]
    if (record) selectRecord(record)
    else beginCreate()
  }
}

async function load(path = inputPath.value) {
  if (!String(path || '').trim()) return
  loading.value = true
  try {
    const next = await LoadSaveFile(String(path).trim())
    inputPath.value = next.path
    outputPath.value = inPlace.value ? next.path : defaultOutput(next.path)
    applyInfo(next)
    await SetLastSavePath(next.path)
    emit('status', next.inventory.unlocked
      ? `已加载召唤石存档：${next.inventory.occupied}/${next.inventory.capacity}`
      : '已加载存档，但召唤系统尚未由游戏开放；仍可写入预分配记录，但不保证游戏开放该系统。', next.inventory.unlocked ? 'success' : 'warning')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally { loading.value = false }
}

async function browseInput() {
  try { const path = await SelectInputSave(); if (path) await load(path) } catch (error) { emit('status', String(error), 'error') }
}
async function browseOutput() {
  try { const path = await SelectOutputSave(outputPath.value || defaultOutput(inputPath.value)); if (path) { outputPath.value = path; inPlace.value = path === inputPath.value } } catch (error) { emit('status', String(error), 'error') }
}
function toggleInPlace() { outputPath.value = inPlace.value ? inputPath.value : defaultOutput(inputPath.value) }
function setWriteMode(value) { inPlace.value = value; toggleInPlace() }

async function write() {
  if (validation.value || writing.value) { if (validation.value) emit('status', validation.value, 'error'); return }
  const operationLabel = mode.value === 'create' ? '新增召唤石' : `修改 SlotID ${selected.value.slotId}`
  const confirmed = await confirmDialog.value?.ask({
    title: operationLabel,
    message: `将${inPlace.value ? '覆盖当前存档（自动备份）' : '写入新存档'}。`,
    detail: '种类、主加护、副词条、等级和 Rank 会作为一条完整记录写入；天然规则只作提醒，写后逐字段回读。',
    confirmLabel: '确认写入', tone: 'warning',
  })
  if (!confirmed) return
  writing.value = true
  try {
    const draft = {
      typeHash: Number(form.typeHash) >>> 0,
      mainTraitHash: Number(form.mainTraitHash) >>> 0,
      subParamHash: Number(form.subParamHash) >>> 0,
      mainTraitLevel: Number(form.mainTraitLevel),
      subParamLevel: Number(form.subParamLevel),
      rank: Number(form.rank),
    }
    const result = await Apply({ operation: mode.value, expected: mode.value === 'update' ? selected.value : null, draft }, outputPath.value.trim())
    inputPath.value = result.outputPath
    outputPath.value = inPlace.value ? result.outputPath : defaultOutput(result.outputPath)
    applyInfo({ path: result.outputPath, inventory: result.inventory })
    selectRecord(result.record)
    emit('status', `${operationLabel}完成，已回读验证${result.backupPath ? `；备份：${result.backupPath}` : ''}`, 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally { writing.value = false }
}

onMounted(async () => {
  loading.value = true
  try {
    const [catalog, slots, lastPath] = await Promise.all([GetOptions(), FindSaveFiles(), GetLastSavePath()])
    Object.assign(options, catalog || {})
    saveSlots.value = Array.isArray(slots) ? slots : []
    syncForm(null)
    if (lastPath) await load(lastPath)
  } catch (error) { emit('status', String(error), 'error') }
  finally { loading.value = false }
})
</script>

<template>
  <div class="summon-save-page ui-page is-wide ui-page-stack">
    <ConfirmDialog ref="confirmDialog" />
    <SaveSourcePicker v-model="inputPath" :slots="saveSlots" :busy="loading" :loaded="!!info.path" :summary="info.path ? `${info.inventory.occupied} / ${info.inventory.capacity} 条` : ''" @select="load" @browse="browseInput" />

    <section v-if="info.path" class="system-card ui-card ui-panel is-compact">
      <div class="ui-split">
        <div><h2 class="ui-section-title">召唤石存档结构</h2><p class="ui-section-copy">复用存档预分配的 1000 条记录；写入记录不等于替游戏解锁 DLC 系统。</p></div>
        <span class="ui-tag" :class="info.inventory?.unlocked ? 'is-success' : 'is-warning'">{{ info.inventory?.unlocked ? '系统已开放' : '系统未开放' }}</span>
      </div>
      <p v-if="info.path && !info.inventory.unlocked" class="locked-note">检测到召唤系统尚未开放；仍可写入预分配记录，但不承诺替游戏解锁 DLC 系统。</p>
      <small class="ui-hint">天然词池、等级与系统开放状态只作提醒；所选可编码值不会被拦截。</small>
    </section>

    <section v-if="info.path" class="workspace ui-card-grid">
      <aside class="ui-card ui-panel is-compact">
        <div class="ui-split"><h3 class="ui-section-title">已有召唤石</h3><button class="ui-btn is-primary is-sm" @click="beginCreate">＋ 新增</button></div>
        <input v-model="filter" class="ui-input" placeholder="名称、Hash 或 SlotID" />
        <div class="record-list ui-list ui-scroll-region">
          <button v-for="record in filteredRecords" :key="record.unitId" class="record-row ui-row" :class="{ 'is-on': selectedUnitID === record.unitId && mode === 'update' }" @click="selectRecord(record)">
            <img v-if="typeIcon(record.state.typeHash)" :src="typeIcon(record.state.typeHash)" alt="" />
            <span class="slot">#{{ record.slotId }}</span><span class="ui-truncate">{{ typeName(record.state.typeHash) }}</span><span class="ui-tag">R{{ record.state.rank }}</span>
          </button>
        </div>
      </aside>

      <section class="ui-card ui-panel editor">
        <div class="ui-split"><div><small>{{ mode === 'create' ? 'CREATE' : `UNIT ${selected?.unitId}` }}</small><h3 class="ui-section-title">{{ mode === 'create' ? '新增一颗召唤石' : `修改 SlotID ${selected?.slotId || '—'}` }}</h3></div><span class="ui-tag" :class="fixedRule ? 'is-warning' : 'is-success'">{{ fixedRule ? '固定词条已证 · 等级待证' : '天然词池与等级已校验' }}</span></div>
        <div class="ui-form-grid">
          <label class="ui-field wide"><span class="ui-field-label">召唤石种类</span><select v-model.number="form.typeHash" class="ui-select" @change="onTypeChanged"><option v-for="item in options.types" :key="item.hash" :value="item.hash">{{ optionLabel(item) }} · {{ item.tier }} · 消耗{{ item.equipCost }} · {{ item.mode }}</option></select></label>
          <label class="ui-field wide"><span class="ui-field-label">主加护</span><span class="select-with-icon"><img v-if="mainIcon(form.mainTraitHash)" :src="mainIcon(form.mainTraitHash)" alt="" /><select v-model.number="form.mainTraitHash" class="ui-select"><option v-for="item in mainChoices" :key="item.hash" :value="item.hash">{{ optionLabel(item) }}</option></select></span></label>
          <label class="ui-field"><span class="ui-field-label">主加护等级</span><input v-model.number="form.mainTraitLevel" class="ui-input" type="number" min="0" max="4294967295" /></label>
          <label class="ui-field wide"><span class="ui-field-label">副词条</span><select v-model.number="form.subParamHash" class="ui-select"><option v-for="item in subChoices" :key="item.hash" :value="item.hash">{{ optionLabel(item) }}</option></select><small>{{ subName(form.subParamHash) }}</small></label>
          <label class="ui-field"><span class="ui-field-label">副词条档位</span><input v-model.number="form.subParamLevel" class="ui-input" type="number" min="0" max="4294967295" /><small v-if="subOption?.values?.[Number(form.subParamLevel)] !== undefined">当前表值：+{{ subOption.values[Number(form.subParamLevel)] }}{{ subOption.isPercent ? '%' : '' }}</small></label>
          <label class="ui-field"><span class="ui-field-label">Rank</span><input v-model.number="form.rank" class="ui-input" type="number" min="0" max="4294967295" /></label>
        </div>
        <div class="write-panel">
          <div class="ui-section-title"><span>写入方式</span><small>覆盖或另存为，两种方式任选</small></div>
          <div class="output-mode">
            <button class="mode-choice ui-btn" :class="{ on: inPlace }" @click="setWriteMode(true)"><b>覆盖当前存档</b><small>自动备份后写回所选槽位</small></button>
            <button class="mode-choice ui-btn" :class="{ on: !inPlace }" @click="setWriteMode(false)"><b>另存为新存档</b><small>保留原文件并生成副本</small></button>
          </div>
          <div class="output-target">
            <div v-if="inPlace" class="selected-save overwrite" :title="inputPath">{{ inputPath }}</div>
            <span v-else class="ui-control-group"><input v-model="outputPath" class="ui-input" placeholder="新存档输出路径…" /><button class="ui-btn" @click="browseOutput">选择位置</button></span>
          </div>
        </div>
        <div class="write-row"><p :class="validation ? 'validation-error' : 'validation-ok'">{{ validation || '可编码字段检查通过；写后会重新打开存档逐字段验证。' }}</p><button class="ui-btn is-primary" :disabled="!!validation || writing" @click="write">{{ writing ? '写入并验证中…' : mode === 'create' ? '新增到存档' : '保存修改' }}</button></div>
      </section>
    </section>
    <div v-else-if="!loading" class="ui-empty">先从上方选择一个游戏存档槽，或浏览其他存档文件。</div>
  </div>
</template>

<style scoped>
.summon-save-page { padding-bottom:var(--space-8); }
.system-card { display:grid; gap:var(--space-3); }
.locked-note { margin:0; padding:var(--space-3); border-left:3px solid var(--warning); background:var(--surface-sunken); color:var(--text-secondary); }
.workspace { --ui-grid-min:360px; grid-template-columns:minmax(300px,.85fr) minmax(0,1.3fr); }
.record-list { max-height:min(62vh,600px); }
.record-row { width:100%; grid-template-columns:40px 48px minmax(0,1fr) auto; text-align:left; cursor:pointer; }
.record-row img,.select-with-icon img { width:36px; height:36px; object-fit:contain; border:1px solid var(--line-soft); border-radius:7px; background:var(--surface-field); }
.slot,small { font-family:var(--font-data); color:var(--text-muted); }
.editor { align-content:start; }
.editor .wide { grid-column:1 / -1; }
.select-with-icon { display:flex; align-items:center; gap:var(--space-3); }
.select-with-icon .ui-select { min-width:0; flex:1; }
.write-panel { display:grid; gap:var(--space-3); padding-top:var(--space-4); border-top:1px solid var(--line-soft); }
.write-panel .ui-section-title { display:flex; align-items:baseline; justify-content:space-between; gap:var(--space-3); }
.write-panel .ui-section-title small { color:var(--text-muted); font-size:var(--fs-xs); font-weight:var(--weight-regular); }
.output-mode { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); }
.mode-choice { height:auto; min-height:62px; flex-direction:column; align-items:flex-start; justify-content:center; white-space:normal; }
.mode-choice b { color:inherit; font-size:var(--fs-md); }
.mode-choice small { color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.mode-choice.on { border-color:var(--selected-border); background:var(--selected-bg); color:var(--selected-fg); }
.mode-choice.on small { color:var(--selected-fg); }
.output-target .ui-control-group { width:100%; }
.output-target .ui-input { min-width:0; flex:1; }
.selected-save { min-width:0; padding:8px 10px; overflow:hidden; border:1px solid var(--line-soft); border-radius:var(--radius-sm); font-family:var(--font-data); font-size:var(--fs-xs); text-overflow:ellipsis; white-space:nowrap; }
.selected-save.overwrite { border-color:var(--warning); background:var(--warning-bg); color:var(--warning-ink); }
.write-row { display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); }
.write-row p { margin:0; }
.validation-error { color:var(--danger); }.validation-ok { color:var(--success); }
@container ui-page (max-width:800px) { .workspace { grid-template-columns:minmax(0,1fr); }.record-list { max-height:280px; } }
@container ui-page (max-width:520px) { .output-mode { grid-template-columns:1fr; }.write-row { align-items:stretch; flex-direction:column; }.write-row .ui-btn { width:100%; } }
</style>
