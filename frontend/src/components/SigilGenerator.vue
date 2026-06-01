<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { GetSigilList, GetCompatibleSecondaryTraits, GetAllowedLevels,
         GetPrimaryTraitLevels, GetSecondaryTraitLevels, GetPrimaryTrait,
         GetDefaultSecondaryTrait, LoadSaveFile, GetLoadedSaveInfo,
         GetQueue, AddToQueue, RemoveFromQueue, ClearQueue,
         ApplyQueue, RemoveAllSigils } from '../../wailsjs/go/main/SigilGen'

const emit = defineEmits(['status'])

function showStatus(msg, type) { emit('status', msg, type) }

// ── 状态 ──
const sigils = ref([])
const saveLoaded = ref(false)
const saveInfo = reactive({ path: '', occupiedSigils: 0, maxSlotId: 0 })
const isApplying = ref(false)

// 表单
const selectedSigilID = ref('')
const selectedLevel = ref(0)
const selectedPrimaryLevel = ref(0)
const selectedSecondaryTraitID = ref('')
const selectedSecondaryLevel = ref(0)
const quantity = ref(1)
const outputPath = ref('')

// 下拉选项
const sigilLevels = ref([])
const primaryTraitLevels = ref([])
const secondaryTraits = ref([])
const secondaryTraitLevels = ref([])
const primaryTraitName = ref('')
const supportsSecondary = ref(false)

// 队列
const queue = ref([])

// 搜索
const sigilSearch = ref('')
const showSigilDropdown = ref(false)

const filteredSigils = computed(() => {
  if (!sigilSearch.value) return sigils.value
  const q = sigilSearch.value.toLowerCase()
  return sigils.value.filter(s => s.displayName.toLowerCase().includes(q))
})

// ── 加载数据 ──
const dataLoading = ref(true)
const dataError = ref('')
onMounted(async () => {
  try {
    sigils.value = await GetSigilList()
    if (!sigils.value || !sigils.value.length) {
      dataError.value = '因子数据为空'
    }
  } catch (e) {
    dataError.value = '加载因子数据失败: ' + String(e)
  } finally {
    dataLoading.value = false
  }
})

// ── 存档 ──
const inputPath = ref('')
async function loadSave() {
  if (!inputPath.value.trim()) { showStatus('请输入存档路径', 'error'); return }
  try {
    const info = await LoadSaveFile(inputPath.value.trim())
    Object.assign(saveInfo, info)
    saveLoaded.value = true
    outputPath.value = info.path.replace(/(\.dat)$/i, '_modified.dat')
    showStatus(`已加载存档: ${info.occupiedSigils} 个因子`, 'success')
  } catch (e) {
    showStatus(String(e), 'error')
  }
}

// ── 因子选择变化 ──
watch(selectedSigilID, async (id) => {
  if (!id) return
  const sigil = sigils.value.find(s => s.internalId === id)
  if (!sigil) return

  supportsSecondary.value = sigil.supportsSecondaryTrait

  // 加载等级
  try {
    sigilLevels.value = await GetAllowedLevels(id)
    primaryTraitLevels.value = await GetPrimaryTraitLevels(id)
  } catch (e) { showStatus(String(e), 'error'); return }

  // 主特性
  try {
    const pt = await GetPrimaryTrait(id)
    primaryTraitName.value = pt ? pt.displayName : ''
  } catch (e) { primaryTraitName.value = '' }

  // 副特性
  if (sigil.supportsSecondaryTrait) {
    try {
      secondaryTraits.value = await GetCompatibleSecondaryTraits(id)
      const def = await GetDefaultSecondaryTrait(id)
      selectedSecondaryTraitID.value = def ? def.internalId : ''
    } catch (e) {
      secondaryTraits.value = []
      selectedSecondaryTraitID.value = ''
    }
  } else {
    secondaryTraits.value = []
    selectedSecondaryTraitID.value = ''
    secondaryTraitLevels.value = []
    selectedSecondaryLevel.value = 0
  }

  // 默认等级
  selectedLevel.value = sigilLevels.value[0] || 0
  selectedPrimaryLevel.value = primaryTraitLevels.value[0] || 0
})

watch(selectedSecondaryTraitID, async (id) => {
  if (!id || !selectedSigilID.value) {
    secondaryTraitLevels.value = []
    selectedSecondaryLevel.value = 0
    return
  }
  try {
    secondaryTraitLevels.value = await GetSecondaryTraitLevels(selectedSigilID.value, id)
    selectedSecondaryLevel.value = secondaryTraitLevels.value[0] || 0
  } catch (e) { secondaryTraitLevels.value = [] }
})

// ── 队列操作 ──
async function addToQueue() {
  if (!selectedSigilID.value) { showStatus('请选择因子', 'error'); return }
  try {
    await AddToQueue({
      sigilId: selectedSigilID.value,
      sigilName: '',
      level: selectedLevel.value,
      primaryTraitId: '',
      primaryTraitName: '',
      primaryLevel: selectedPrimaryLevel.value,
      secondaryTraitId: supportsSecondary.value ? selectedSecondaryTraitID.value : '',
      secondaryTraitName: '',
      secondaryLevel: selectedSecondaryLevel.value,
      quantity: quantity.value,
    })
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
  if (!outputPath.value.trim()) { showStatus('请输入输出路径', 'error'); return }
  isApplying.value = true
  try {
    const result = await ApplyQueue(outputPath.value.trim())
    queue.value = []
    showStatus(`已写入 ${result.createdCount} 个因子 (验证 ${result.verifiedCount})`, 'success')
  } catch (e) { showStatus(String(e), 'error') }
  finally { isApplying.value = false }
}

async function removeAll() {
  if (!inputPath.value.trim() || !outputPath.value.trim()) {
    showStatus('请先填写输入和输出路径', 'error'); return
  }
  if (!confirm('这将清除输出存档中的所有因子，确定继续？')) return
  try {
    const result = await RemoveAllSigils(inputPath.value.trim(), outputPath.value.trim())
    showStatus(`已清除 ${result.createdCount} 个因子 (剩余 ${result.verifiedCount})`, 'success')
  } catch (e) { showStatus(String(e), 'error') }
}

function onSigilSelect() {
  const sigil = sigils.value.find(s => s.internalId === selectedSigilID.value)
  if (sigil) {
    sigilSearch.value = sigil.displayName
  }
}
</script>

<template>
  <div class="sigil-container">
    <!-- 存档选择 -->
    <div class="section">
      <div class="section-title">存档文件</div>
      <div class="input-row">
        <input v-model="inputPath" type="text" class="text-input flex-1"
          placeholder="选择 GBFR 存档文件 (.dat)..." />
        <button class="btn-action btn-green" @click="loadSave">加载</button>
      </div>
      <div v-if="saveLoaded" class="save-info">
        已加载 · {{ saveInfo.occupiedSigils }} 个因子 · 最大槽位 {{ saveInfo.maxSlotId }}
      </div>
    </div>

    <!-- 因子配置 -->
    <div class="section">
      <div class="section-title">因子配置</div>

      <!-- 因子搜索 -->
      <div class="field">
        <label>因子 {{ dataLoading ? '(加载中...)' : dataError ? '(加载失败)' : '' }}</label>
        <div v-if="dataError" class="data-error">{{ dataError }}</div>
        <input v-model="sigilSearch" type="text" class="text-input"
          placeholder="输入关键词过滤..." @input="showSigilDropdown = true" />
        <select v-model="selectedSigilID" class="select-input sigil-select"
          size="6" @change="onSigilSelect">
          <option value="">— 请先搜索并选择因子 —</option>
          <option v-for="s in filteredSigils" :key="s.internalId" :value="s.internalId">
            {{ s.displayName }}<template v-if="s.supportsSecondaryTrait"> [V+]</template>
          </option>
        </select>
      </div>

      <!-- 因子等级 -->
      <div class="field">
        <label>因子等级</label>
        <select v-model="selectedLevel" class="select-input" :disabled="!sigilLevels.length">
          <option v-for="l in sigilLevels" :key="l" :value="l">Lv {{ l }}</option>
        </select>
      </div>

      <!-- 主特性 -->
      <div class="field">
        <label>主特性</label>
        <div class="readonly-field">{{ primaryTraitName || '—' }}</div>
      </div>

      <div class="field">
        <label>主特性等级</label>
        <select v-model="selectedPrimaryLevel" class="select-input" :disabled="!primaryTraitLevels.length">
          <option v-for="l in primaryTraitLevels" :key="l" :value="l">Lv {{ l }}</option>
        </select>
      </div>

      <!-- 副特性 -->
      <template v-if="supportsSecondary">
        <div class="field">
          <label>副特性</label>
          <select v-model="selectedSecondaryTraitID" class="select-input"
            :disabled="!secondaryTraits.length">
            <option value="">— 不选择 —</option>
            <option v-for="t in secondaryTraits" :key="t.internalId" :value="t.internalId">
              {{ t.displayName }}
            </option>
          </select>
        </div>
        <div class="field">
          <label>副特性等级</label>
          <select v-model="selectedSecondaryLevel" class="select-input"
            :disabled="!secondaryTraitLevels.length">
            <option v-for="l in secondaryTraitLevels" :key="l" :value="l">Lv {{ l }}</option>
          </select>
        </div>
      </template>

      <!-- 数量 + 添加 -->
      <div class="input-row">
        <div class="field flex-1">
          <label>数量</label>
          <input v-model.number="quantity" type="number" min="1" max="999" class="text-input" />
        </div>
        <button class="btn-action btn-purple add-btn" @click="addToQueue"
          :disabled="!selectedSigilID">
          添加到队列
        </button>
      </div>
    </div>

    <!-- 队列 -->
    <div class="section">
      <div class="section-title">
        队列 ({{ queue.length }})
        <button v-if="queue.length" class="btn-link" @click="clearQueueAll">清空</button>
      </div>
      <div v-if="!queue.length" class="empty-hint">暂无待写入因子，请先添加</div>
      <div v-else class="queue-list">
        <div v-for="(item, i) in queue" :key="i" class="queue-item">
          <div class="queue-info">
            <span class="queue-name">{{ item.sigilName }}</span>
            <span class="queue-detail">
              Lv {{ item.level }} · {{ item.primaryTraitName }} Lv {{ item.primaryLevel }}
              <template v-if="item.secondaryTraitId">
                / {{ item.secondaryTraitName }} Lv {{ item.secondaryLevel }}
              </template>
              · x{{ item.quantity }}
            </span>
          </div>
          <button class="btn-icon" @click="removeFromQueue(i)" title="移除">✕</button>
        </div>
      </div>
    </div>

    <!-- 输出 + 应用 -->
    <div class="section">
      <div class="section-title">输出</div>
      <div class="input-row">
        <input v-model="outputPath" type="text" class="text-input flex-1"
          placeholder="输出存档路径..." />
        <button class="btn-action btn-cyan" @click="applyQueueToSave"
          :disabled="isApplying || !queue.length">
          {{ isApplying ? '写入中...' : '应用写入' }}
        </button>
      </div>
    </div>

    <!-- 清除所有 -->
    <div class="section section-danger">
      <div class="section-title">危险操作</div>
      <button class="btn-action btn-red" @click="removeAll"
        :disabled="!inputPath.trim() || !outputPath.trim()">
        清除输出存档中所有因子
      </button>
    </div>
  </div>
</template>

<style scoped>
.sigil-container {
  display: flex;
  flex-direction: column;
  gap: 14px;
  width: 100%;
}

.section {
  border-radius: 12px;
  padding: 14px 16px;
  background: rgba(255,255,255,0.04);
  border: 1px solid rgba(255,255,255,0.06);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-danger {
  border-color: rgba(239,68,68,0.15);
  background: rgba(239,68,68,0.04);
}

.section-title {
  font-size: 0.78rem;
  font-weight: 600;
  color: rgba(255,255,255,0.35);
  letter-spacing: 1px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.field { display: flex; flex-direction: column; gap: 4px; }
.field label {
  font-size: 0.7rem;
  color: rgba(255,255,255,0.3);
}

.text-input, .select-input {
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.12);
  background: rgba(255,255,255,0.06);
  color: #fff;
  font-size: 0.82rem;
  font-family: inherit;
  outline: none;
  transition: border-color 0.2s;
  box-sizing: border-box;
}

.text-input:focus, .select-input:focus {
  border-color: rgba(103,232,249,0.4);
  background: rgba(255,255,255,0.1);
}

.select-input {
  cursor: pointer;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='8' height='5'%3E%3Cpath d='M0 0l4 5 4-5z' fill='rgba(255,255,255,0.3)'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 10px center;
  padding-right: 28px;
}

.select-input:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.readonly-field {
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.08);
  background: rgba(255,255,255,0.03);
  color: rgba(255,255,255,0.45);
  font-size: 0.82rem;
}

.input-row {
  display: flex;
  gap: 8px;
  align-items: flex-end;
}

.flex-1 { flex: 1; }

.btn-action {
  padding: 8px 16px;
  border-radius: 8px;
  border: none;
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  transition: transform 0.15s, opacity 0.2s;
}

.btn-action:not(:disabled):hover { transform: scale(1.03); }
.btn-action:disabled { opacity: 0.35; cursor: not-allowed; }

.btn-green {
  background: rgba(34,197,94,0.18);
  color: #4ade80;
  border: 1px solid rgba(34,197,94,0.3);
}
.btn-green:not(:disabled):hover { background: rgba(34,197,94,0.28); }

.btn-purple {
  background: rgba(165,180,252,0.15);
  color: #a5b4fc;
  border: 1px solid rgba(165,180,252,0.3);
}
.btn-purple:not(:disabled):hover { background: rgba(165,180,252,0.25); }

.btn-cyan {
  background: rgba(103,232,249,0.15);
  color: #67e8f9;
  border: 1px solid rgba(103,232,249,0.3);
}
.btn-cyan:not(:disabled):hover { background: rgba(103,232,249,0.25); }

.btn-red {
  background: rgba(239,68,68,0.15);
  color: #f87171;
  border: 1px solid rgba(239,68,68,0.3);
}
.btn-red:not(:disabled):hover { background: rgba(239,68,68,0.25); }

.add-btn { padding-top: 8px; padding-bottom: 8px; align-self: flex-end; }

.btn-link {
  background: none;
  border: none;
  color: rgba(255,255,255,0.3);
  font-size: 0.72rem;
  cursor: pointer;
  padding: 0 4px;
}
.btn-link:hover { color: rgba(239,68,68,0.7); }

.btn-icon {
  background: none;
  border: none;
  color: rgba(255,255,255,0.3);
  cursor: pointer;
  font-size: 0.85rem;
  padding: 2px 6px;
  border-radius: 4px;
  transition: color 0.15s;
}
.btn-icon:hover { color: #f87171; }

.save-info {
  font-size: 0.72rem;
  color: rgba(74,222,128,0.6);
}

.empty-hint {
  font-size: 0.75rem;
  color: rgba(255,255,255,0.2);
  text-align: center;
  padding: 8px 0;
}

/* 因子选择列表 */
.sigil-select {
  height: auto;
  min-height: 120px;
  overflow-y: auto;
  cursor: pointer;
}
.sigil-select option {
  padding: 4px 8px;
  color: #fff;
  background: rgba(27,38,54,1);
}
.data-error {
  font-size: 0.72rem;
  color: #f87171;
  padding: 4px 0;
}

/* 队列列表 */
.queue-list { display: flex; flex-direction: column; gap: 6px; }
.queue-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-radius: 8px;
  background: rgba(255,255,255,0.04);
  border: 1px solid rgba(255,255,255,0.06);
  gap: 8px;
}
.queue-info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.queue-name { font-size: 0.8rem; color: rgba(255,255,255,0.6); font-weight: 600; }
.queue-detail { font-size: 0.7rem; color: rgba(255,255,255,0.3); }
</style>
