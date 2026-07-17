<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { FindSaveFiles, GetLastSavePath, SetLastSavePath } from '../../wailsjs/go/main/App'
import { GetWrightstoneList, GetTraitList, GetTraitLevels, GetDefaultTrait,
         LoadSaveFile, GetQueue, AddToQueue, RemoveFromQueue, ClearQueue,
         ApplyQueue, ApplyItems, CheckLegality, FileExists, SelectWrightstoneInputSave,
         SelectWrightstoneOutputSave } from '../../wailsjs/go/main/WrightstoneGen'
import { backendLanguageReady } from '../backendLanguage'
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
    if (ticket === legalityTicket) Object.assign(legality, { status: 'unknown', writable: true, message: `检验失败：${String(e)}`, reasons: [] })
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
    <div class="section">
      <div class="section-title"><span>选择存档槽</span><small>与物品、武器页面使用同一组存档</small></div>
      <div class="save-slots">
        <button v-for="slot in slots" :key="slot.index" class="slot-choice" :class="{ on: inputPath === slot.path }" @click="selectSaveSlot(slot.path)">{{ saveSlotLabel(slot) }}</button>
        <button class="slot-choice secondary" @click="browseInput">选择其他存档</button>
      </div>
      <div class="selected-save" :class="{ empty: !inputPath }">{{ inputPath || '尚未选择存档' }}</div>
      <div v-if="saveLoaded" class="save-info">
        已加载 · {{ saveInfo.occupiedWrightstones }} 个祝福 · 最大槽位 {{ saveInfo.maxSlotId }}
      </div>
    </div>

    <div class="section">
      <div class="section-title">
        祝福配置
        <span class="info-dot" title="选择祝福后配置三个词条与等级；不加入队列时，直接点击应用会写入当前选择。">!</span>
      </div>
      <div v-if="dataError" class="data-error">{{ dataError }}</div>
      <div class="field">
        <label>祝福 <small>{{ dataLoading ? '正在加载目录' : '点击下拉后可搜索名称' }}</small></label>
        <CatalogSelect v-model="selectedWrightstoneID" :options="wrightstones" :disabled="dataLoading || !!dataError" placeholder="尚未选择祝福" search-placeholder="搜索祝福名称" />
      </div>

      <div v-for="(_, i) in selectedTraits" :key="i" class="trait-card">
        <div class="field flex-1">
          <label>{{ traitLabel(i) }} <small>点击下拉选择特性</small></label>
          <CatalogSelect v-model="selectedTraits[i].id" :options="traits" placeholder="尚未选择特性" search-placeholder="搜索特性名称" detail-key="maxLevel" @pick="loadTraitLevels(i)" />
        </div>
        <div class="field level-field">
          <label>等级 <small :class="{ overcap: selectedTraits[i].level > naturalTraitMax(i) }">{{ selectedTraits[i].level > naturalTraitMax(i) ? `超过合规上限 ${naturalTraitMax(i)} / 修改上限 ${writableTraitMax(i)}` : `合规上限 ${naturalTraitMax(i)} / 修改上限 ${writableTraitMax(i)}` }}</small></label>
          <input v-model.number="selectedTraits[i].level" type="number" min="0" :max="writableTraitMax(i)" class="text-input compact-number" :class="{ 'lv-over': selectedTraits[i].level > naturalTraitMax(i) }" :disabled="!selectedTraits[i].id" @change="selectedTraits[i].level = clampLevel(selectedTraits[i].level, writableTraitMax(i))" />
        </div>
      </div>

      <div class="config-footer">
        <LegalityIndicator v-if="currentSelectionValid" class="config-legality" :status="legality.status" :message="legality.message" />
        <span v-else class="selection-note">选完祝福与三项特性后显示合法性结果</span>
        <div class="qty-add">
          <div class="field quantity-field">
            <label>数量</label>
            <span class="quantity-combo"><input v-model.number="quantity" type="number" min="1" max="999" class="text-input" /><button type="button" @click="quantity=999">最大</button></span>
          </div>
          <button class="btn-action btn-purple add-btn" @click="addToQueue" :disabled="!selectedWrightstoneID || !legality.writable">
            添加到队列
          </button>
        </div>
      </div>
    </div>

    <div class="section">
      <div class="section-title">
        队列 ({{ queue.length }})
        <button v-if="queue.length" class="btn-link" @click="clearQueueAll">清空</button>
      </div>
      <div v-if="!queue.length" class="empty-hint">暂无队列；直接点击应用时会写入当前选择</div>
      <div v-else class="queue-list">
        <div v-for="(item, i) in queue" :key="i" class="queue-item">
          <div class="queue-info">
            <span class="queue-name">{{ item.wrightstoneName }} <em v-if="item.legalityStatus" class="queue-legality" :class="item.legalityStatus">{{ item.legalityStatus === 'forced' ? '强制' : '未完全验证' }}</em></span>
            <span class="queue-detail">
              {{ item.firstTraitName }} Lv {{ item.firstLevel }} /
              {{ item.secondTraitName }} Lv {{ item.secondLevel }} /
              {{ item.thirdTraitName }} Lv {{ item.thirdLevel }} · x{{ item.quantity }}
            </span>
          </div>
          <button class="btn-icon" @click="removeFromQueue(i)" title="移除">✕</button>
        </div>
      </div>
    </div>

    <div class="section apply-section" :class="{ 'apply-flash': applyFlash }">
      <div class="section-title"><span>写入方式</span><small>覆盖或另存为，两种方式任选</small></div>
      <div class="output-mode">
        <button class="mode-choice" :class="{ on: inPlaceEdit }" @click="inPlaceEdit = true"><b>覆盖当前存档</b><small>自动备份后写回所选槽位</small></button>
        <button class="mode-choice" :class="{ on: !inPlaceEdit }" @click="inPlaceEdit = false"><b>另存为新存档</b><small>保留原文件并生成副本</small></button>
      </div>
      <div class="input-row output-target">
        <div v-if="inPlaceEdit" class="selected-save overwrite flex-1">{{ inputPath || '请先选择存档槽' }}</div>
        <input v-else v-model="outputPath" type="text" class="text-input flex-1" placeholder="新存档输出路径..." />
        <button v-if="!inPlaceEdit" class="btn-action btn-cyan" @click="browseOutput">选择位置</button>
        <button class="btn-action btn-cyan" @click="applyQueueToSave" :disabled="isApplying || !canApply">
          {{ isApplying ? '写入中...' : '应用写入' }}
        </button>
      </div>
      <div v-if="inPlaceEdit" class="danger-hint">警告：启用后，应用写入将直接覆盖当前输入存档，建议先备份。</div>
      <div v-else class="warning-hint">安全提示：只写入输出存档，不会覆盖原始输入存档；已有输出文件会先确认。</div>
    </div>
  </div>
  <ConfirmDialog ref="confirmDialog" />
</template>

<style scoped>
.wrightstone-container { display: flex; flex-direction: column; gap: 14px; width: 100%; container-type:inline-size; }
.save-slots,.output-mode { display:flex;gap:8px;flex-wrap:wrap }.slot-choice,.mode-choice { border:1px solid rgba(145,108,52,.25);background:rgba(244,231,199,.62);color:#705f49;cursor:pointer }.slot-choice { padding:8px 15px }.slot-choice.on,.mode-choice.on { border-color:rgba(42,145,154,.5);background:rgba(89,193,201,.17);color:#286f76;box-shadow:inset 0 -2px #3aa9b3 }.slot-choice.secondary { margin-left:auto }.selected-save { min-height:32px;display:flex;align-items:center;padding:0 11px;border:1px dashed rgba(142,106,52,.28);background:rgba(255,249,229,.38);color:#71634f;font:750 10px/1.4 var(--font-data);font-variant-numeric:tabular-nums lining-nums;overflow:hidden;text-overflow:ellipsis;white-space:nowrap }.selected-save.empty { color:#a18f74 }.selected-save.overwrite { border-style:solid;border-color:rgba(177,104,69,.31);background:rgba(228,166,116,.12) }.mode-choice { flex:1 1 220px;min-height:58px;padding:9px 13px;text-align:left }.mode-choice b,.mode-choice small { display:block }.mode-choice b { font-size:11px }.mode-choice small { margin-top:4px;color:#8f7d64;font-size:9px }.mode-choice.on small { color:#4e7d79 }.output-target { margin-top:2px }.section-title small { color:#9a876c;font-size:9px;font-weight:700;letter-spacing:0 }
.config-footer { display:flex; align-items:flex-end; gap:10px; padding-top:2px; }
.config-legality { flex:1 1 auto; }
.qty-add { flex:0 0 auto; display:flex; align-items:flex-end; gap:8px; }
.quantity-field { width:108px; }
.quantity-field{width:150px}.quantity-combo{display:grid;grid-template-columns:minmax(0,1fr) 45px;gap:5px}.quantity-combo button{border:1px solid rgba(218,187,115,.28);background:rgba(218,187,115,.07);color:#d9bd7c;font-size:10px;cursor:pointer}
.queue-legality { margin-left:5px; padding:1px 5px; border:1px solid rgba(154,116,64,.25); border-radius:4px; color:#9a7440; background:rgba(154,116,64,.06); font-size:.58rem; font-style:normal; font-weight:700; }
.queue-legality.forced { border-color:rgba(251,191,36,.28); color:#fbbf24; background:rgba(245,158,11,.08); }
@media (max-width:720px) { .config-footer { align-items:stretch; flex-direction:column; } .qty-add { width:100%; } .quantity-field { flex:1; width:auto; } }
@container (max-width:500px) { .config-footer { align-items:stretch; flex-direction:column; } .qty-add { width:100%; } .quantity-field { flex:1; width:auto; } .input-row { flex-wrap:wrap; } .input-row>.flex-1 { min-width:0; flex-basis:220px; } .output-target>.flex-1 { flex-basis:100%; } .trait-card { flex-wrap:wrap; } }
.section { border-radius: 12px; padding: 14px 16px; background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.06); display: flex; flex-direction: column; gap: 10px; }
.apply-section { position: relative; overflow: hidden; z-index: 0; transition: border-color 0.3s, box-shadow 0.3s; }
.apply-section > * { position: relative; z-index: 1; }
.apply-section::after { content: ""; position: absolute; inset: 0; z-index: 0; border-radius: 12px; background: #abd373; transform: translateY(calc(-100% - 2px)); transition: transform 0.5s ease; }
.apply-section.apply-flash { border-color: rgba(171,211,115,0.55); box-shadow: 0 14px 34px rgba(171,211,115,0.18); }
.apply-section.apply-flash::after { transform: translateY(0); }
.apply-section.apply-flash .section-title,
.apply-section.apply-flash .toggle-row { color: #1f2937; }
.apply-section.apply-flash .text-input { border-color: rgba(31,41,55,0.22); background: rgba(255,255,255,0.22); color: #1f2937; }
.apply-section.apply-flash .btn-cyan { border-color: rgba(31,41,55,0.22); background: rgba(31,41,55,0.12); color: #1f2937; }
.apply-section.apply-flash .warning-hint,
.apply-section.apply-flash .danger-hint { border-color: rgba(31,41,55,0.18); background: rgba(255,255,255,0.18); color: rgba(31,41,55,0.78); }
.section-title { font-size: 0.78rem; font-weight: 600; color: rgba(255,255,255,0.35); letter-spacing: 1px; display: flex; align-items: center; justify-content: space-between; }
.info-dot { display: inline-flex; align-items: center; justify-content: center; width: 15px; height: 15px; border-radius: 50%; border: 1px solid rgba(154,116,64,0.35); color: #9a7440; background: rgba(154,116,64,0.08); font-size: 0.68rem; font-weight: 700; cursor: help; letter-spacing: 0; }
.field { display: flex; flex-direction: column; gap: 4px; }
.field label { font-size: 0.7rem; color: rgba(255,255,255,0.3); }
.field label small { margin-left:6px;color:#8b795f;font-size:.9em;font-weight:550; }
.field label small.overcap { color:#984f42;font-weight:700; }
.compact-number { width:82px;max-width:100%;text-align:center; }
.selection-note { flex:1;color:#8f7d64;font-size:.72rem;line-height:1.5; }
.text-input, .select-input { padding: 8px 12px; border-radius: 8px; border: 1px solid rgba(255,255,255,0.12); background: rgba(255,255,255,0.06); color: #fff; font-size: 0.82rem; font-family: inherit; outline: none; transition: border-color 0.2s; box-sizing: border-box; }
.select-input { background-color: transparent; scrollbar-width: thin; scrollbar-color: rgba(255,255,255,0.15) rgba(0,0,0,0.2); }
.select-input::-webkit-scrollbar { width: 6px; }
.select-input::-webkit-scrollbar-track { background: rgba(0,0,0,0.2); border-radius: 3px; }
.select-input::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.15); border-radius: 3px; }
.select-input::-webkit-scrollbar-thumb:hover { background: rgba(255,255,255,0.25); }
.select-input option { min-height:38px;padding:9px 12px;border-bottom:1px solid rgba(123,88,43,.25);background:#1b2636;color:#fff; }
.select-input[size] option:nth-child(even) { background:rgba(255,255,255,.035); }
.select-input[size] option:checked { box-shadow:inset 4px 0 #9a7440; }
.text-input:focus { border-color: rgba(154,116,64,0.4); background: rgba(255,255,255,0.1); }
.select-input:focus { border-color: rgba(154,116,64,0.4); background: transparent; }
.select-input:disabled { opacity: 0.4; cursor: not-allowed; }
.input-row { display: flex; gap: 8px; align-items: flex-end; }
.flex-1 { flex: 1; }
.trait-card { display: flex; gap: 10px; align-items: flex-end; padding: 10px; border-radius: 10px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.05); }
.level-field { width: 120px; flex-shrink: 0; }
.btn-action { padding: 8px 16px; border-radius: 8px; border: none; font-size: 0.8rem; font-weight: 600; cursor: pointer; white-space: nowrap; transition: transform 0.15s, opacity 0.2s; }
.btn-action:not(:disabled):hover { transform: scale(1.03); }
.btn-action:disabled { opacity: 0.35; cursor: not-allowed; }
.btn-green { background: rgba(34,197,94,0.18); color: #4ade80; border: 1px solid rgba(34,197,94,0.3); }
.btn-purple { background: rgba(165,180,252,0.15); color: #a5b4fc; border: 1px solid rgba(165,180,252,0.3); }
.btn-cyan { background: rgba(154,116,64,0.15); color: #9a7440; border: 1px solid rgba(154,116,64,0.3); }
.add-btn { padding-top: 8px; padding-bottom: 8px; align-self: flex-end; }
.btn-link { background: none; border: none; color: rgba(255,255,255,0.3); font-size: 0.72rem; cursor: pointer; padding: 0 4px; }
.btn-link:hover { color: rgba(239,68,68,0.7); }
.btn-icon { background: none; border: none; color: rgba(255,255,255,0.3); cursor: pointer; font-size: 0.85rem; padding: 2px 6px; border-radius: 4px; transition: color 0.15s; }
.btn-icon:hover { color: #f87171; }
.save-info { font-size: 0.72rem; color: rgba(74,222,128,0.6); }
.empty-hint { font-size: 0.75rem; color: rgba(255,255,255,0.2); text-align: center; padding: 8px 0; }
.warning-hint { font-size: 0.72rem; color: rgba(251,191,36,0.8); }
.toggle-row { display: flex; align-items: center; gap: 8px; font-size: 0.78rem; color: rgba(255,255,255,0.75); }
.danger-hint { font-size: 0.72rem; color: rgba(248,113,113,0.95); padding: 8px 12px; background: rgba(239,68,68,0.08); border: 1px solid rgba(239,68,68,0.2); border-radius: 6px; line-height: 1.5; }
.danger-path { background: rgba(239,68,68,0.14) !important; border-color: rgba(239,68,68,0.55) !important; color: #fecaca; }
.data-error { font-size: 0.75rem; color: #f87171; }
.queue-list { display: flex; flex-direction: column; gap: 6px; }
.queue-item { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 10px 12px; border-radius: 10px; background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.06); }
.queue-info { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.queue-name { color: rgba(255,255,255,0.7); font-size: 0.84rem; font-weight: 600; }
.queue-detail { color: rgba(255,255,255,0.35); font-size: 0.72rem; }
.wrightstone-container{gap:13px}.section{border-color:rgba(154,202,224,.14);border-radius:4px 12px 4px 12px;background:rgba(8,31,53,.7);box-shadow:0 12px 34px rgba(0,7,17,.12)}.section-title{color:#eee7d8;font:500 .9rem Georgia,"Noto Serif SC","STSong",serif;letter-spacing:.04em}.trait-card{border-color:rgba(154,202,224,.12);border-radius:3px 9px 3px 9px;background:rgba(13,45,70,.5)}.text-input,.select-input{border-color:rgba(154,202,224,.18);border-radius:4px;background:rgba(12,43,68,.78);color:#edf4f6}.select-input option{background:#0a2943}.btn-purple,.btn-green,.btn-cyan{border-color:rgba(218,187,115,.34);background:rgba(218,187,115,.08);color:#f0d99d}.queue-item{border-color:rgba(154,202,224,.12);border-radius:3px 9px 3px 9px;background:rgba(13,45,70,.58)}.queue-name{color:#dce8ec}.queue-detail{color:#6f8b99}
.quantity-combo .text-input,.quantity-combo .text-input:hover,.quantity-combo .text-input:focus{border-color:#9a7440!important;background:#fff9e8!important;box-shadow:none!important;outline:0!important}.quantity-combo input[type=number]::-webkit-inner-spin-button,.quantity-combo input[type=number]::-webkit-outer-spin-button{-webkit-appearance:none;margin:0}.quantity-combo input[type=number]{-moz-appearance:textfield}.add-btn,.add-btn:disabled{border:1px solid #9a7440!important;border-radius:1px!important;color:#5e4c34!important;background:#ead8b2!important;box-shadow:none!important;opacity:1!important}.add-btn:hover:not(:disabled){color:#fff9e9!important;background:#8b6737!important}.add-btn:disabled{color:#8f7a5c!important;border-color:rgba(154,116,64,.38)!important;background:#e7d8b6!important}
</style>
