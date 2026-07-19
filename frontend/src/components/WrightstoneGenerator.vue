<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { FindSaveFiles, GetLastSavePath, SetLastSavePath } from '../../wailsjs/go/main/App'
import { GetWrightstoneList, GetTraitList, GetTraitLevels, GetDefaultTrait,
         LoadSaveFile, GetQueue, AddToQueue, RemoveFromQueue, ClearQueue,
         ApplyQueue, ApplyItems, CheckLegality, FileExists, SelectWrightstoneInputSave,
         SelectWrightstoneOutputSave } from '../../wailsjs/go/main/WrightstoneGen'
import { backendLanguageReady } from '../backendLanguage'
import { traitAssetIcon } from '../gameAssetIcons'
import LegalityIndicator from './LegalityIndicator.vue'
import ConfirmDialog from './ConfirmDialog.vue'
import CatalogSelect from './CatalogSelect.vue'

const emit = defineEmits(['status'])
function showStatus(msg, type) { emit('status', msg, type) }

const wrightstones = ref([])
const traits = ref([])
const slots = ref([])
const saveLoaded = ref(false)
const saveInfo = reactive({ path: '', occupiedWrightstones: 0, maxSlotId: 0 })
const isApplying = ref(false)
const inPlaceEdit = ref(false)
const applyFlash = ref(false)
const confirmDialog = ref(null)
let applyFlashTimer = 0

const inputPath = ref('')
const outputPath = ref('')
const selectedWrightstoneID = ref('')
const selectedTraits = reactive([
  { id: '', level: 0, levels: [] },
  { id: '', level: 0, levels: [] },
  { id: '', level: 0, levels: [] },
])
const quantity = ref(1)
const queue = ref([])
const legality = reactive({ status: 'impossible', writable: false, message: '请先完成祝福配置', reasons: [] })
let legalityTicket = 0

const dataLoading = ref(true)
const dataError = ref('')

const currentSelectionValid = computed(() => {
  return !!selectedWrightstoneID.value && selectedTraits.every(t => !!t.id && !!t.level) && quantity.value > 0
})
const canApply = computed(() => saveLoaded.value && !!outputPath.value.trim() && (queue.value.length > 0 || (currentSelectionValid.value && legality.writable)))

function naturalTraitMax(slot) {
  if (slot === 0) return 20
  if (slot === 1) return 15
  return 10
}

function writableTraitMax(slot) {
  return Math.max(naturalTraitMax(slot), ...selectedTraits[slot].levels)
}

function clampLevel(value, max) {
  const numeric = Number.isFinite(Number(value)) ? Number(value) : 0
  return Math.min(max, Math.max(0, Math.trunc(numeric)))
}

onMounted(async () => {
  try {
    await backendLanguageReady
    ;[wrightstones.value, traits.value, slots.value] = await Promise.all([GetWrightstoneList(), GetTraitList(), FindSaveFiles()])
    if (!wrightstones.value.length || !traits.value.length) {
      dataError.value = '祝福或特性数据为空'
    }
    const lastPath = await GetLastSavePath()
    if (lastPath) {
      inputPath.value = lastPath
      outputPath.value = defaultOutputPath(lastPath)
    }
  } catch (e) {
    dataError.value = '加载祝福数据失败: ' + String(e)
  } finally {
    dataLoading.value = false
  }
})

function defaultOutputPath(path) {
  if (!path) return ''
  if (/\.dat$/i.test(path)) return path.replace(/(\.dat)$/i, '_wrightstones.dat')
  return `${path}_wrightstones.dat`
}

watch(inPlaceEdit, (enabled) => {
  if (enabled) {
    outputPath.value = inputPath.value.trim()
  } else if (outputPath.value.trim() === inputPath.value.trim()) {
    outputPath.value = defaultOutputPath(inputPath.value.trim())
  }
})

async function browseInput() {
  try {
    const path = await SelectWrightstoneInputSave()
    if (!path) return
    inputPath.value = path
    await loadSave()
  } catch (e) { showStatus(String(e), 'error') }
}

async function selectSaveSlot(path) {
  inputPath.value = path
  await loadSave()
}

function saveSlotLabel(slot) {
  const fileName = String(slot?.name || slot?.path || '').split(/[\\/]/).pop()
  const match = fileName.match(/SaveData\d+/i)
  return match ? match[0].replace(/^savedata/i, 'SaveData') : fileName.replace(/\.dat$/i, '')
}

async function browseOutput() {
  try {
    const path = await SelectWrightstoneOutputSave(outputPath.value.trim() || defaultOutputPath(inputPath.value.trim()))
    if (path) outputPath.value = path
  } catch (e) { showStatus(String(e), 'error') }
}

async function loadSave() {
  if (!inputPath.value.trim()) { showStatus('请输入存档路径', 'error'); return }
  try {
    const info = await LoadSaveFile(inputPath.value.trim())
    Object.assign(saveInfo, info)
    saveLoaded.value = true
    outputPath.value = inPlaceEdit.value ? info.path : defaultOutputPath(info.path)
    await SetLastSavePath(info.path)
    showStatus(`已加载存档: ${info.occupiedWrightstones} 个祝福`, 'success')
  } catch (e) {
    showStatus(String(e), 'error')
  }
}

watch(selectedWrightstoneID, async (id) => {
  if (!id) return
  try {
    const def = await GetDefaultTrait(id)
    if (def) {
      selectedTraits[0].id = def.internalId
      await loadTraitLevels(0)
    }
  } catch (e) { showStatus(String(e), 'error') }
})

async function loadTraitLevels(slot) {
  const traitID = selectedTraits[slot].id
  if (!traitID) {
    selectedTraits[slot].levels = []
    selectedTraits[slot].level = 0
    return
  }
  try {
    const levels = await GetTraitLevels(traitID)
    selectedTraits[slot].levels = levels
    selectedTraits[slot].level = naturalTraitMax(slot)
  } catch (e) {
    selectedTraits[slot].levels = []
    selectedTraits[slot].level = 0
    showStatus(String(e), 'error')
  }
}

function traitLabel(slot) {
  return ['第一特性', '第二特性', '第三特性'][slot]
}
function traitIconForOption(trait) {
  return traitAssetIcon({ internalId: trait?.internalId, hash: trait?.hash, name: trait?.displayName })
}
function traitIconByID(id) {
  const trait = traits.value.find(item => item.internalId === id)
  return traitIconForOption(trait || { internalId: id })
}

function buildCurrentItem() {
  return {
    wrightstoneId: selectedWrightstoneID.value,
    wrightstoneName: '',
    firstTraitId: selectedTraits[0].id,
    firstTraitName: '',
    firstLevel: selectedTraits[0].level,
    secondTraitId: selectedTraits[1].id,
    secondTraitName: '',
    secondLevel: selectedTraits[1].level,
    thirdTraitId: selectedTraits[2].id,
    thirdTraitName: '',
    thirdLevel: selectedTraits[2].level,
    quantity: quantity.value,
  }
}

async function refreshLegality() {
  const ticket = ++legalityTicket
  try {
    const report = await CheckLegality(buildCurrentItem())
    if (ticket === legalityTicket) Object.assign(legality, report)
  } catch (e) {
    if (ticket === legalityTicket) Object.assign(legality, { status: 'impossible', writable: false, message: `检验失败：${String(e)}`, reasons: [] })
  }
}

watch(() => [selectedWrightstoneID.value, quantity.value, ...selectedTraits.flatMap(t => [t.id, t.level])], refreshLegality)

function validateCurrentSelection() {
  if (!selectedWrightstoneID.value) { showStatus('请选择祝福', 'error'); return false }
  for (let i = 0; i < 3; i++) {
    if (!selectedTraits[i].id || !selectedTraits[i].level) {
      showStatus(`请选择${traitLabel(i)}及等级`, 'error')
      return false
    }
  }
  if (!quantity.value || quantity.value < 1) { showStatus('数量至少为 1', 'error'); return false }
  return true
}

async function addToQueue() {
  if (!validateCurrentSelection()) return
  try {
    await AddToQueue(buildCurrentItem())
    queue.value = await GetQueue()
    showStatus('已添加到队列', 'success')
  } catch (e) { showStatus(String(e), 'error') }
}

async function removeFromQueue(index) {
  try {
    await RemoveFromQueue(index)
    queue.value = await GetQueue()
  } catch (e) { showStatus(String(e), 'error') }
}

async function clearQueueAll() {
  await ClearQueue()
  queue.value = []
}

function flashApplySuccess() {
  applyFlash.value = false
  clearTimeout(applyFlashTimer)
  requestAnimationFrame(() => {
    applyFlash.value = true
    applyFlashTimer = window.setTimeout(() => { applyFlash.value = false }, 900)
  })
}

async function applyQueueToSave() {
  if (!saveLoaded.value) { showStatus('请先加载存档', 'error'); return }
  if (!outputPath.value.trim()) { showStatus('请输入输出路径', 'error'); return }
  if (!queue.value.length && !validateCurrentSelection()) return

  isApplying.value = true
  try {
    const output = outputPath.value.trim()
    const exists = await FileExists(output)
    if (exists) {
      const confirmed = await confirmDialog.value?.ask({
        title: '覆盖已有存档',
        message: '输出位置已经存在同名文件，是否覆盖？',
        detail: output,
        tone: 'danger',
        confirmLabel: '确认覆盖',
      })
      if (!confirmed) return
    }

    const result = queue.value.length
      ? await ApplyQueue(output)
      : await ApplyItems([buildCurrentItem()], output)
    queue.value = []
    if (inPlaceEdit.value) await loadSave()
    flashApplySuccess()
    showStatus(`已写入 ${result.createdCount} 个祝福 (验证 ${result.verifiedCount})`, 'success')
  } catch (e) { showStatus(String(e), 'error') }
  finally { isApplying.value = false }
}
</script>

<template>
  <div class="wrightstone-container">
    <div class="section ui-card compact-save-bar">
      <div class="section-title ui-section-title"><span>选择存档槽</span><small>与物品、武器页面使用同一组存档</small></div>
      <div class="save-slots">
        <button v-for="slot in slots" :key="slot.index" class="slot-choice ui-btn is-sm" :class="{ on: inputPath === slot.path }" @click="selectSaveSlot(slot.path)">{{ saveSlotLabel(slot) }}</button>
        <button class="slot-choice secondary ui-btn is-sm" @click="browseInput">选择其他存档</button>
      </div>
      <div class="selected-save" :class="{ empty: !inputPath }">{{ inputPath || '尚未选择存档' }}</div>
      <div v-if="saveLoaded" class="save-info">
        已加载 · {{ saveInfo.occupiedWrightstones }} 个祝福 · 最大槽位 {{ saveInfo.maxSlotId }}
      </div>
    </div>

    <div class="section ui-card">
      <div class="section-title ui-section-title">
        祝福配置
        <span class="info-dot" title="选择祝福后配置三个词条与等级；不加入队列时，直接点击应用会写入当前选择。">!</span>
      </div>
      <div v-if="dataError" class="data-error">{{ dataError }}</div>
      <div class="field ui-field wrightstone-pick">
        <label class="ui-field-label">祝福 <small>{{ dataLoading ? '正在加载目录' : '点击下拉后可搜索名称' }}</small></label>
        <CatalogSelect v-model="selectedWrightstoneID" :options="wrightstones" :disabled="dataLoading || !!dataError" placeholder="尚未选择祝福" search-placeholder="搜索祝福名称" />
      </div>

      <div class="trait-grid">
      <div v-for="(_, i) in selectedTraits" :key="i" class="trait-card ui-card">
        <div class="field flex-1 ui-field">
          <label class="ui-field-label trait-label-with-icon">
            <img v-if="traitIconByID(selectedTraits[i].id)" :src="traitIconByID(selectedTraits[i].id)" alt="" />
            <span>{{ traitLabel(i) }} <small>点击下拉选择特性</small></span>
          </label>
          <CatalogSelect v-model="selectedTraits[i].id" :options="traits" :icon-resolver="traitIconForOption" placeholder="尚未选择特性" search-placeholder="搜索特性名称" detail-key="maxLevel" @pick="loadTraitLevels(i)" />
        </div>
        <div class="field level-field ui-field">
          <label class="ui-field-label">等级 <small :class="{ overcap: selectedTraits[i].level > naturalTraitMax(i) }">{{ selectedTraits[i].level > naturalTraitMax(i) ? `超过合规上限 ${naturalTraitMax(i)} / 修改上限 ${writableTraitMax(i)}` : `合规上限 ${naturalTraitMax(i)} / 修改上限 ${writableTraitMax(i)}` }}</small></label>
          <input v-model.number="selectedTraits[i].level" type="number" min="0" :max="writableTraitMax(i)" class="text-input compact-number ui-input" :class="{ 'lv-over': selectedTraits[i].level > naturalTraitMax(i) }" :disabled="!selectedTraits[i].id" @change="selectedTraits[i].level = clampLevel(selectedTraits[i].level, writableTraitMax(i))" />
        </div>
      </div>
      </div>

      <div class="config-footer">
        <LegalityIndicator v-if="currentSelectionValid" class="config-legality" :status="legality.status" :message="legality.message" />
        <span v-else class="selection-note">选完祝福与三项特性后显示合法性结果</span>
        <div class="qty-add">
          <div class="field quantity-field ui-field">
            <label class="ui-field-label">数量</label>
            <span class="quantity-combo"><input v-model.number="quantity" type="number" min="1" max="999" class="text-input ui-input" /><button class="ui-btn is-sm" type="button" @click="quantity=999">最大</button></span>
          </div>
          <button class="btn-action btn-purple add-btn ui-btn is-primary" @click="addToQueue" :disabled="!selectedWrightstoneID || !legality.writable">
            添加到队列
          </button>
        </div>
      </div>
    </div>

    <div class="wrightstone-lower-grid">
    <div class="section ui-card">
      <div class="section-title ui-section-title">
        队列 ({{ queue.length }})
        <button v-if="queue.length" class="btn-link ui-btn is-subtle" @click="clearQueueAll">清空</button>
      </div>
      <div v-if="!queue.length" class="empty-hint ui-empty">暂无队列；直接点击应用时会写入当前选择</div>
      <div v-else class="queue-list">
        <div v-for="(item, i) in queue" :key="i" class="queue-item ui-row">
          <div class="queue-info">
            <span class="queue-name">{{ item.wrightstoneName }} <em v-if="item.legalityStatus" class="queue-legality" :class="item.legalityStatus">{{ item.legalityStatus === 'forced' ? '强制' : '未完全验证' }}</em></span>
            <span class="queue-detail">
              {{ item.firstTraitName }} Lv {{ item.firstLevel }} /
              {{ item.secondTraitName }} Lv {{ item.secondLevel }} /
              {{ item.thirdTraitName }} Lv {{ item.thirdLevel }} · x{{ item.quantity }}
            </span>
          </div>
          <button class="btn-icon ui-btn is-subtle" @click="removeFromQueue(i)" title="移除">✕</button>
        </div>
      </div>
    </div>

    <div class="section ui-card apply-section" :class="{ 'apply-flash': applyFlash }">
      <div class="section-title ui-section-title"><span>写入方式</span><small>覆盖或另存为，两种方式任选</small></div>
      <div class="output-mode">
        <button class="mode-choice ui-btn" :class="{ on: inPlaceEdit }" @click="inPlaceEdit = true"><b>覆盖当前存档</b><small>自动备份后写回所选槽位</small></button>
        <button class="mode-choice ui-btn" :class="{ on: !inPlaceEdit }" @click="inPlaceEdit = false"><b>另存为新存档</b><small>保留原文件并生成副本</small></button>
      </div>
      <div class="input-row output-target">
        <div v-if="inPlaceEdit" class="selected-save overwrite flex-1">{{ inputPath || '请先选择存档槽' }}</div>
        <input v-else v-model="outputPath" type="text" class="text-input flex-1 ui-input" placeholder="新存档输出路径..." />
        <button v-if="!inPlaceEdit" class="btn-action btn-cyan ui-btn" @click="browseOutput">选择位置</button>
        <button class="btn-action btn-cyan ui-btn is-primary" @click="applyQueueToSave" :disabled="isApplying || !canApply">
          {{ isApplying ? '写入中...' : '应用写入' }}
        </button>
      </div>
    </div>
    </div>
  </div>
  <ConfirmDialog ref="confirmDialog" />
</template>

<style scoped>
.wrightstone-container {
  width:100%;
  max-width:1000px;
  display:flex;
  flex-direction:column;
  gap:var(--space-5);
  color:var(--text-secondary);
  container-type:inline-size;
}
.section { min-width:0; padding:var(--space-6); }
.compact-save-bar { padding:var(--space-4) var(--space-5); }
.section-title { margin-bottom:var(--space-4); }
.section-title > small { margin-left:auto; text-align:right; }

.save-slots {
  display:flex;
  flex-wrap:wrap;
  align-items:center;
  gap:var(--space-2);
}
.slot-choice.on {
  border-color:var(--selected-border);
  background:var(--selected-bg);
  color:var(--selected-fg);
}
.selected-save {
  min-width:0;
  margin-top:var(--space-3);
  padding:var(--space-3) var(--space-4);
  overflow:hidden;
  border:1px solid var(--border-soft);
  border-radius:var(--radius-sm);
  background:var(--surface-sunken);
  color:var(--text-secondary);
  font-family:var(--font-data);
  font-size:var(--fs-sm);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.selected-save.empty { color:var(--text-secondary); }
.selected-save.overwrite { border-color:var(--warning); background:var(--warning-bg); color:var(--warning-ink); }
.save-info { margin-top:var(--space-2); color:var(--success-ink); font-size:var(--fs-sm); }
.info-dot {
  display:inline-grid;
  width:20px;
  height:20px;
  margin-left:var(--space-1);
  place-items:center;
  border:1px solid var(--border-strong);
  border-radius:var(--radius-pill);
  color:var(--accent-hover);
  font-size:var(--fs-xs);
}

.field { min-width:0; }
.wrightstone-pick { width:min(100%,680px); }
.field label small {
  display:block;
  margin-top:var(--space-1);
  color:var(--text-secondary);
  font-size:var(--fs-xs);
  font-weight:var(--fw-normal);
  line-height:var(--lh-normal);
}
.data-error,
.warning-hint,
.danger-hint {
  margin-bottom:var(--space-4);
  padding:var(--space-3) var(--space-4);
  border-radius:var(--radius-sm);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}
.data-error { border-left:3px solid var(--danger); background:var(--danger-bg); color:var(--danger-ink); }
.warning-hint { border-left:3px solid var(--info); background:var(--info-bg); color:var(--info-ink); }
.danger-hint { border-left:3px solid var(--warning); background:var(--warning-bg); color:var(--warning-ink); }

.trait-grid {
  display:grid;
  grid-template-columns:repeat(auto-fit,minmax(240px,1fr));
  gap:var(--space-3);
  margin-top:var(--space-4);
}
.trait-card {
  display:flex;
  min-width:0;
  flex-direction:column;
  gap:var(--space-4);
  padding:var(--space-4);
  background:var(--surface-card-pop);
  box-shadow:none;
}
.trait-label-with-icon { min-height:38px; display:flex; align-items:center; gap:var(--space-2); }
.trait-label-with-icon img { width:36px; height:36px; flex:0 0 36px; object-fit:cover; border:1px solid var(--line-soft); border-radius:7px; background:var(--surface-field); }
.trait-label-with-icon > span { min-width:0; display:flex; flex-direction:column; gap:1px; }
.level-field { width:100%; }
.compact-number { width:100%; }
.overcap { color:var(--warning-ink); }
.lv-over { border-color:var(--warning); background:var(--warning-bg); color:var(--warning-ink); }
.flex-1 { min-width:0; flex:1; }

.config-footer {
  display:flex;
  align-items:flex-end;
  justify-content:space-between;
  gap:var(--space-5);
  margin-top:var(--space-5);
  padding-top:var(--space-4);
  border-top:1px solid var(--border-soft);
}
.config-legality { min-width:0; flex:1; }
.selection-note { color:var(--text-secondary); font-size:var(--fs-sm); }
.qty-add { display:flex; align-items:flex-end; gap:var(--space-3); }
.quantity-field { width:184px; flex:0 0 184px; }
.quantity-combo { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-2); }
.quantity-combo .ui-input { min-width:0; width:100%; }

.wrightstone-lower-grid {
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
  gap:var(--space-5);
  align-items:start;
}
.wrightstone-lower-grid .section { height:100%; box-sizing:border-box; }
.empty-hint { color:var(--text-secondary); font-size:var(--fs-md); text-align:center; }
.queue-list { display:flex; flex-direction:column; gap:var(--space-2); }
.queue-item {
  justify-content:space-between;
  border-color:var(--border-soft);
  background:var(--surface-card-pop);
}
.queue-info { display:flex; min-width:0; flex-direction:column; gap:var(--space-1); }
.queue-name { color:var(--text-primary); font-size:var(--fs-md); font-weight:var(--fw-semibold); }
.queue-detail { color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.queue-legality {
  margin-left:var(--space-1);
  padding:1px var(--space-2);
  border-radius:var(--radius-pill);
  background:var(--warning-bg);
  color:var(--warning-ink);
  font-size:var(--fs-xs);
  font-style:normal;
}
.btn-icon { flex:0 0 auto; }

.output-mode {
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
  gap:var(--space-3);
}
.mode-choice {
  height:auto;
  min-height:62px;
  flex-direction:column;
  align-items:flex-start;
  justify-content:center;
  white-space:normal;
}
.mode-choice b { color:inherit; font-size:var(--fs-md); }
.mode-choice small { color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.mode-choice.on {
  border-color:var(--selected-border);
  background:var(--selected-bg);
  color:var(--selected-fg);
}
.mode-choice.on small { color:var(--selected-fg); }
.input-row {
  display:flex;
  align-items:center;
  gap:var(--space-2);
  margin-top:var(--space-4);
}
.output-target .selected-save { margin-top:0; }
.apply-section .danger-hint,
.apply-section .warning-hint { margin:var(--space-3) 0 0; }
.apply-flash { animation:apply-confirm var(--dur-base) var(--ease-out) 2 alternate; }
@keyframes apply-confirm {
  to { border-color:var(--success); box-shadow:0 0 0 2px var(--success-bg); }
}

input[type="checkbox"] { accent-color:var(--accent); }

@container (max-width:760px) {
  .wrightstone-lower-grid { grid-template-columns:1fr; }
  .config-footer { align-items:stretch; flex-direction:column; }
  .qty-add { width:100%; }
  .quantity-field { width:auto; flex:1; }
}

@container (max-width:620px) {
  .section,
  .compact-save-bar { padding:var(--space-4); }
  .section-title { align-items:flex-start; }
  .section-title > small { width:100%; margin-left:0; text-align:left; }
  .output-mode { grid-template-columns:1fr; }
  .input-row { align-items:stretch; flex-direction:column; }
  .input-row > * { width:100%; }
  .output-target .selected-save { box-sizing:border-box; }
}

@container (max-width:440px) {
  .save-slots .ui-btn { flex:1; }
  .qty-add { align-items:stretch; flex-direction:column; }
  .quantity-field,
  .add-btn { width:100%; }
}
</style>
