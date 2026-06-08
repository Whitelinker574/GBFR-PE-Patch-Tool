<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { GetWrightstoneList, GetTraitList, GetTraitLevels, GetDefaultTrait,
         LoadSaveFile, GetQueue, AddToQueue, RemoveFromQueue, ClearQueue,
         ApplyQueue, ApplyItems, FileExists, SelectWrightstoneInputSave,
         SelectWrightstoneOutputSave } from '../../wailsjs/go/main/WrightstoneGen'

const emit = defineEmits(['status'])
function showStatus(msg, type) { emit('status', msg, type) }

const wrightstones = ref([])
const traits = ref([])
const saveLoaded = ref(false)
const saveInfo = reactive({ path: '', occupiedWrightstones: 0, maxSlotId: 0 })
const isApplying = ref(false)

const inputPath = ref('')
const outputPath = ref('')
const selectedWrightstoneID = ref('')
const wrightstoneSearch = ref('')
const selectedTraits = reactive([
  { id: '', level: 0, levels: [] },
  { id: '', level: 0, levels: [] },
  { id: '', level: 0, levels: [] },
])
const quantity = ref(1)
const queue = ref([])

const dataLoading = ref(true)
const dataError = ref('')
const traitSearches = ref(['', '', ''])

const filteredWrightstones = computed(() => {
  if (!wrightstoneSearch.value) return wrightstones.value
  const q = wrightstoneSearch.value.toLowerCase()
  return wrightstones.value.filter(w => w.displayName.toLowerCase().includes(q))
})

const currentSelectionValid = computed(() => {
  return !!selectedWrightstoneID.value && selectedTraits.every(t => !!t.id && !!t.level) && quantity.value > 0
})
const canApply = computed(() => saveLoaded.value && !!outputPath.value.trim() && (queue.value.length > 0 || currentSelectionValid.value))

function filteredTraits(slot) {
  const q = traitSearches.value[slot].toLowerCase()
  if (!q) return traits.value
  return traits.value.filter(t => t.displayName.toLowerCase().includes(q))
}

onMounted(async () => {
  try {
    wrightstones.value = await GetWrightstoneList()
    traits.value = await GetTraitList()
    if (!wrightstones.value.length || !traits.value.length) {
      dataError.value = '祝福或特性数据为空'
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

async function browseInput() {
  try {
    const path = await SelectWrightstoneInputSave()
    if (!path) return
    inputPath.value = path
    await loadSave()
  } catch (e) { showStatus(String(e), 'error') }
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
    outputPath.value = defaultOutputPath(info.path)
    showStatus(`已加载存档: ${info.occupiedWrightstones} 个祝福`, 'success')
  } catch (e) {
    showStatus(String(e), 'error')
  }
}

watch(selectedWrightstoneID, async (id) => {
  if (!id) return
  const w = wrightstones.value.find(x => x.internalId === id)
  if (w) wrightstoneSearch.value = w.displayName
  try {
    const def = await GetDefaultTrait(id)
    if (def) {
      selectedTraits[0].id = def.internalId
      traitSearches.value[0] = def.displayName
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
  const trait = traits.value.find(t => t.internalId === traitID)
  if (trait) traitSearches.value[slot] = trait.displayName
  try {
    const levels = await GetTraitLevels(traitID)
    selectedTraits[slot].levels = levels
    selectedTraits[slot].level = levels[levels.length - 1] || 0
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

async function applyQueueToSave() {
  if (!saveLoaded.value) { showStatus('请先加载存档', 'error'); return }
  if (!outputPath.value.trim()) { showStatus('请输入输出路径', 'error'); return }
  if (!queue.value.length && !validateCurrentSelection()) return

  isApplying.value = true
  try {
    const output = outputPath.value.trim()
    const exists = await FileExists(output)
    if (exists && !window.confirm(`输出文件已存在，是否覆盖？\n${output}`)) return

    const result = queue.value.length
      ? await ApplyQueue(output)
      : await ApplyItems([buildCurrentItem()], output)
    queue.value = []
    showStatus(`已写入 ${result.createdCount} 个祝福 (验证 ${result.verifiedCount})`, 'success')
  } catch (e) { showStatus(String(e), 'error') }
  finally { isApplying.value = false }
}
</script>

<template>
  <div class="wrightstone-container">
    <div class="section">
      <div class="section-title">存档文件</div>
      <div class="input-row">
        <input v-model="inputPath" type="text" class="text-input flex-1" placeholder="选择 GBFR 存档文件 (.dat)..." />
        <button class="btn-action btn-cyan" @click="browseInput">浏览</button>
        <button class="btn-action btn-green" @click="loadSave">加载</button>
      </div>
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
        <label>祝福 {{ dataLoading ? '(加载中...)' : '' }}</label>
        <input v-model="wrightstoneSearch" type="text" class="text-input" placeholder="输入关键词过滤..." />
        <select v-model="selectedWrightstoneID" class="select-input" size="5">
          <option value="">— 请选择祝福 —</option>
          <option v-for="w in filteredWrightstones" :key="w.internalId" :value="w.internalId">
            {{ w.displayName }}<template v-if="w.defaultTraitName"> · 默认 {{ w.defaultTraitName }}</template>
          </option>
        </select>
      </div>

      <div v-for="(_, i) in selectedTraits" :key="i" class="trait-card">
        <div class="field flex-1">
          <label>{{ traitLabel(i) }}</label>
          <input v-model="traitSearches[i]" type="text" class="text-input" placeholder="输入关键词过滤特性..." />
          <select v-model="selectedTraits[i].id" class="select-input" size="4" @change="loadTraitLevels(i)">
            <option value="">— 请选择特性 —</option>
            <option v-for="t in filteredTraits(i)" :key="t.internalId" :value="t.internalId">
              {{ t.displayName }} · Max {{ t.maxLevel }}
            </option>
          </select>
        </div>
        <div class="field level-field">
          <label>等级</label>
          <select v-model="selectedTraits[i].level" class="select-input" :disabled="!selectedTraits[i].levels.length">
            <option v-for="l in selectedTraits[i].levels" :key="l" :value="l">Lv {{ l }}</option>
          </select>
        </div>
      </div>

      <div class="input-row">
        <div class="field flex-1">
          <label>数量</label>
          <input v-model.number="quantity" type="number" min="1" max="999" class="text-input" />
        </div>
        <button class="btn-action btn-purple add-btn" @click="addToQueue" :disabled="!selectedWrightstoneID">
          添加到队列
        </button>
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
            <span class="queue-name">{{ item.wrightstoneName }}</span>
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

    <div class="section">
      <div class="section-title">输出</div>
      <div class="input-row">
        <input v-model="outputPath" type="text" class="text-input flex-1" placeholder="输出存档路径..." />
        <button class="btn-action btn-cyan" @click="browseOutput">浏览</button>
        <button class="btn-action btn-cyan" @click="applyQueueToSave" :disabled="isApplying || !canApply">
          {{ isApplying ? '写入中...' : '应用写入' }}
        </button>
      </div>
      <div class="warning-hint">安全提示：只写入输出存档，不会覆盖原始输入存档；已有输出文件会先确认。</div>
    </div>
  </div>
</template>

<style scoped>
.wrightstone-container { display: flex; flex-direction: column; gap: 14px; width: 100%; }
.section { border-radius: 12px; padding: 14px 16px; background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.06); display: flex; flex-direction: column; gap: 10px; }
.section-title { font-size: 0.78rem; font-weight: 600; color: rgba(255,255,255,0.35); letter-spacing: 1px; display: flex; align-items: center; justify-content: space-between; }
.info-dot { display: inline-flex; align-items: center; justify-content: center; width: 15px; height: 15px; border-radius: 50%; border: 1px solid rgba(103,232,249,0.35); color: #67e8f9; background: rgba(103,232,249,0.08); font-size: 0.68rem; font-weight: 700; cursor: help; letter-spacing: 0; }
.field { display: flex; flex-direction: column; gap: 4px; }
.field label { font-size: 0.7rem; color: rgba(255,255,255,0.3); }
.text-input, .select-input { padding: 8px 12px; border-radius: 8px; border: 1px solid rgba(255,255,255,0.12); background: rgba(255,255,255,0.06); color: #fff; font-size: 0.82rem; font-family: inherit; outline: none; transition: border-color 0.2s; box-sizing: border-box; }
.select-input { scrollbar-width: thin; scrollbar-color: rgba(255,255,255,0.15) rgba(0,0,0,0.2); }
.select-input::-webkit-scrollbar { width: 6px; }
.select-input::-webkit-scrollbar-track { background: rgba(0,0,0,0.2); border-radius: 3px; }
.select-input::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.15); border-radius: 3px; }
.select-input::-webkit-scrollbar-thumb:hover { background: rgba(255,255,255,0.25); }
.select-input option { background: rgba(27,38,54,1); color: #fff; }
.text-input:focus, .select-input:focus { border-color: rgba(103,232,249,0.4); background: rgba(255,255,255,0.1); }
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
.btn-cyan { background: rgba(103,232,249,0.15); color: #67e8f9; border: 1px solid rgba(103,232,249,0.3); }
.add-btn { padding-top: 8px; padding-bottom: 8px; align-self: flex-end; }
.btn-link { background: none; border: none; color: rgba(255,255,255,0.3); font-size: 0.72rem; cursor: pointer; padding: 0 4px; }
.btn-link:hover { color: rgba(239,68,68,0.7); }
.btn-icon { background: none; border: none; color: rgba(255,255,255,0.3); cursor: pointer; font-size: 0.85rem; padding: 2px 6px; border-radius: 4px; transition: color 0.15s; }
.btn-icon:hover { color: #f87171; }
.save-info { font-size: 0.72rem; color: rgba(74,222,128,0.6); }
.empty-hint { font-size: 0.75rem; color: rgba(255,255,255,0.2); text-align: center; padding: 8px 0; }
.warning-hint { font-size: 0.72rem; color: rgba(251,191,36,0.8); }
.data-error { font-size: 0.75rem; color: #f87171; }
.queue-list { display: flex; flex-direction: column; gap: 6px; }
.queue-item { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 10px 12px; border-radius: 10px; background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.06); }
.queue-info { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.queue-name { color: rgba(255,255,255,0.7); font-size: 0.84rem; font-weight: 600; }
.queue-detail { color: rgba(255,255,255,0.35); font-size: 0.72rem; }
</style>
