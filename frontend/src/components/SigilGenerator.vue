<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { FindSaveFiles, GetLastSavePath, SetLastSavePath } from '../../wailsjs/go/main/App'
import { GetSigilList, GetTraitList, GetCompatibleSecondaryTraits, GetAllowedLevels,
         GetPrimaryTraitLevels, GetSecondaryTraitLevels, GetPrimaryTrait,
         LoadSaveFile, GetLoadedSaveInfo,
         GetQueue, AddToQueue, RemoveFromQueue, ClearQueue, CheckLegality,
         ApplyQueue, RemoveAllSigils,
         GetExistingSigils, DeleteSelectedSigils,
         SelectSigilInputSave, SelectSigilOutputSave } from '../../wailsjs/go/main/SigilGen'
import { backendLanguageReady } from '../backendLanguage'
import LegalityIndicator from './LegalityIndicator.vue'
import ConfirmDialog from './ConfirmDialog.vue'
import CatalogSelect from './CatalogSelect.vue'

const emit = defineEmits(['status'])

function showStatus(msg, type) { emit('status', msg, type) }

// ── 状态 ──
const sigils = ref([])
const slots = ref([])
const saveLoaded = ref(false)
const saveInfo = reactive({ path: '', occupiedSigils: 0, maxSlotId: 0 })
const isApplying = ref(false)
const inPlaceEdit = ref(false)
const applyFlash = ref(false)
const confirmDialog = ref(null)
let applyFlashTimer = 0

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
const allTraits = ref([])
const allowedSecondaryIDs = ref(new Set())
const secondaryTraitLevels = ref([])
const primaryTraitName = ref('')
const supportsSecondary = ref(false)

// 队列
const queue = ref([])
const legality = reactive({ status: 'impossible', writable: false, message: '请先选择因子', reasons: [] })
let legalityTicket = 0

// 已有因子
const existingSigils = ref([])
const selectedForDelete = ref(new Set())
const showExisting = ref(false)
const isDeleting = ref(false)
const loadingExisting = ref(false)

const secondaryPickerOptions = computed(() => secondaryTraits.value.map(trait => ({
  ...trait,
  displayName: `${trait.displayName}${allowedSecondaryIDs.value.has(trait.internalId) ? '' : ' · 强制组合'}`,
})))
// The catalog may expose the full storable byte range (up to 50).  Natural
// sigils cap both trait slots at 15; larger values are still force-writable.
const primaryNaturalMax = computed(() => 15)
const secondaryNaturalMax = computed(() => 15)
const sigilWritableMax = 50
const primaryWritableMax = computed(() => Math.max(15, ...primaryTraitLevels.value))
const secondaryWritableMax = computed(() => Math.max(15, ...secondaryTraitLevels.value))

function clampLevel(value, max) {
  const numeric = Number.isFinite(Number(value)) ? Number(value) : 0
  return Math.min(max, Math.max(0, Math.trunc(numeric)))
}

// ── 加载数据 ──
const dataLoading = ref(true)
const dataError = ref('')
onMounted(async () => {
  try {
    await backendLanguageReady
    ;[sigils.value, allTraits.value, slots.value] = await Promise.all([GetSigilList(), GetTraitList(), FindSaveFiles()])
    if (!sigils.value || !sigils.value.length) {
      dataError.value = '因子数据为空'
    }
    const lastPath = await GetLastSavePath()
    if (lastPath) {
      inputPath.value = lastPath
      outputPath.value = defaultOutputPath(lastPath)
    }
  } catch (e) {
    dataError.value = '加载因子数据失败: ' + String(e)
  } finally {
    dataLoading.value = false
  }
})

// ── 存档 ──
const inputPath = ref('')

function defaultOutputPath(path) {
  if (!path) return ''
  if (/\.dat$/i.test(path)) return path.replace(/(\.dat)$/i, '_modified.dat')
  return `${path}_modified.dat`
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
    const path = await SelectSigilInputSave()
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
    const path = await SelectSigilOutputSave(outputPath.value.trim() || defaultOutputPath(inputPath.value.trim()))
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
    showExisting.value = true
    await refreshExisting()
    showStatus(`已加载存档: ${info.occupiedSigils} 个因子`, 'success')
  } catch (e) {
    showExisting.value = false
    showStatus(String(e), 'error')
  }
}

async function refreshExisting() {
  loadingExisting.value = true
  try {
    existingSigils.value = await GetExistingSigils()
    selectedForDelete.value = new Set()
  } catch (e) {
    existingSigils.value = []
    showStatus('读取已有因子失败: ' + String(e), 'error')
  } finally {
    loadingExisting.value = false
  }
}

function toggleSelectAll() {
  if (selectedForDelete.value.size === existingSigils.value.length) {
    selectedForDelete.value = new Set()
  } else {
    selectedForDelete.value = new Set(existingSigils.value.map(s => s.gemUnitId))
  }
}

async function deleteSelected() {
  if (selectedForDelete.value.size === 0) {
    showStatus('未选中任何因子', 'error'); return
  }
  if (!outputPath.value.trim()) {
    showStatus('请填写输出路径', 'error'); return
  }
  const confirmed = await confirmDialog.value?.ask({
    title: '删除所选因子',
    message: `确定删除选中的 ${selectedForDelete.value.size} 个因子？`,
    detail: '该操作会写入目标存档，删除后无法从当前结果中撤销。',
    tone: 'danger',
    confirmLabel: '确认删除',
  })
  if (!confirmed) return
  isDeleting.value = true
  try {
    const ids = Array.from(selectedForDelete.value)
    const result = await DeleteSelectedSigils(ids, outputPath.value.trim())
    if (inPlaceEdit.value) {
      await loadSave()
    } else {
      await refreshExisting()
    }
    showStatus(`已删除 ${result.createdCount} 个因子`, 'success')
  } catch (e) {
    showStatus(String(e), 'error')
  } finally {
    isDeleting.value = false
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
      const allowed = await GetCompatibleSecondaryTraits(id)
      allowedSecondaryIDs.value = new Set(allowed.map(t => t.internalId))
      secondaryTraits.value = allTraits.value
      // v1.8.0 supports intentionally leaving the secondary slot empty to
      // generate a single-trait factor.  Do not silently restore a default
      // trait when the user changes the primary factor.
      selectedSecondaryTraitID.value = ''
    } catch (e) {
      secondaryTraits.value = []
      selectedSecondaryTraitID.value = ''
    }
  } else {
    secondaryTraits.value = []
    allowedSecondaryIDs.value = new Set()
    selectedSecondaryTraitID.value = ''
    secondaryTraitLevels.value = []
    selectedSecondaryLevel.value = 0
  }

  // 默认等级
  selectedLevel.value = 15
  selectedPrimaryLevel.value = 15
})

function buildCurrentItem() {
  return {
    sigilId: selectedSigilID.value,
    sigilName: '',
    level: selectedLevel.value,
    primaryTraitId: '',
    primaryTraitName: '',
    primaryLevel: selectedPrimaryLevel.value,
    secondaryTraitId: supportsSecondary.value ? selectedSecondaryTraitID.value : '',
    secondaryTraitName: '',
    secondaryLevel: supportsSecondary.value ? selectedSecondaryLevel.value : 0,
    quantity: quantity.value,
  }
}

async function refreshLegality() {
  const ticket = ++legalityTicket
  if (!selectedSigilID.value) {
    Object.assign(legality, { status: 'impossible', writable: false, message: '请先选择因子', reasons: [] })
    return
  }
  try {
    const report = await CheckLegality(buildCurrentItem())
    if (ticket === legalityTicket) Object.assign(legality, report)
  } catch (e) {
    if (ticket === legalityTicket) Object.assign(legality, { status: 'unknown', writable: true, message: `检验失败：${String(e)}`, reasons: [] })
  }
}

watch([selectedSigilID, selectedLevel, selectedPrimaryLevel, selectedSecondaryTraitID, selectedSecondaryLevel, quantity], refreshLegality)

watch(selectedSecondaryTraitID, async (id) => {
  if (!id || !selectedSigilID.value) {
    secondaryTraitLevels.value = []
    selectedSecondaryLevel.value = 0
    return
  }
  try {
    secondaryTraitLevels.value = await GetSecondaryTraitLevels(selectedSigilID.value, id)
    selectedSecondaryLevel.value = 15
  } catch (e) { secondaryTraitLevels.value = [] }
})

// ── 队列操作 ──
async function addToQueue() {
  if (!selectedSigilID.value) { showStatus('请选择因子', 'error'); return }
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
  if (!outputPath.value.trim()) { showStatus('请输入输出路径', 'error'); return }
  isApplying.value = true
  try {
    const result = await ApplyQueue(outputPath.value.trim())
    queue.value = []
    if (inPlaceEdit.value) await loadSave()
    flashApplySuccess()
    showStatus(`已写入 ${result.createdCount} 个因子 (验证 ${result.verifiedCount})`, 'success')
  } catch (e) { showStatus(String(e), 'error') }
  finally { isApplying.value = false }
}

async function removeAll() {
  if (!inputPath.value.trim() || !outputPath.value.trim()) {
    showStatus('请先填写输入和输出路径', 'error'); return
  }
  const confirmed = await confirmDialog.value?.ask({
    title: '清除全部因子',
    message: '这将清除输出存档中的全部因子。',
    detail: outputPath.value.trim(),
    tone: 'danger',
    confirmLabel: '清除全部',
  })
  if (!confirmed) return
  try {
    const result = await RemoveAllSigils(inputPath.value.trim(), outputPath.value.trim())
    if (inPlaceEdit.value) {
      await loadSave()
    }
    showStatus(`已清除 ${result.createdCount} 个因子 (剩余 ${result.verifiedCount})`, 'success')
  } catch (e) { showStatus(String(e), 'error') }
}

</script>

<template>
  <div class="sigil-container">
    <!-- 存档选择 -->
    <div class="section">
      <div class="section-title"><span>选择存档槽</span><small>与物品、武器页面使用同一组存档</small></div>
      <div class="save-slots">
        <button v-for="slot in slots" :key="slot.index" class="slot-choice" :class="{ on: inputPath === slot.path }" @click="selectSaveSlot(slot.path)">{{ saveSlotLabel(slot) }}</button>
        <button class="slot-choice secondary" @click="browseInput">选择其他存档</button>
      </div>
      <div class="selected-save" :class="{ empty: !inputPath }">{{ inputPath || '尚未选择存档' }}</div>
      <div v-if="saveLoaded" class="save-info">
        已加载 · {{ saveInfo.occupiedSigils }} 个因子 · 最大槽位 {{ saveInfo.maxSlotId }}
      </div>
    </div>

    <!-- 已有因子 -->
    <div v-if="showExisting" class="section">
      <div class="section-title">
        已有因子 {{ loadingExisting ? '加载中...' : `(${existingSigils.length})` }}
        <div class="existing-actions">
          <button class="btn-link" @click="toggleSelectAll"
            :disabled="loadingExisting">
            {{ selectedForDelete.size === existingSigils.length ? '取消全选' : '全选' }}
          </button>
          <button class="btn-link" @click="refreshExisting" :disabled="loadingExisting">刷新</button>
          <button class="btn-action btn-red btn-sm"
            @click="deleteSelected"
            :disabled="isDeleting || loadingExisting || selectedForDelete.size === 0">
            {{ isDeleting ? '删除中...' : `删除选中 (${selectedForDelete.size})` }}
          </button>
        </div>
      </div>
      <div v-if="loadingExisting" class="loading-hint">正在读取已有因子，数量较多时请耐心等待...</div>
      <div v-else-if="saveInfo.occupiedSigils > 500" class="warning-hint">
        注意：当前存档有 {{ saveInfo.occupiedSigils }} 个因子，目前批量编辑处于测试阶段，不建议使用
      </div>
      <div v-if="!loadingExisting && existingSigils.length === 0" class="empty-hint">暂无已有因子或读取失败</div>
      <div v-else class="existing-table">
        <div class="existing-row existing-header">
          <span class="ex-col-cb"><input type="checkbox" :checked="selectedForDelete.size === existingSigils.length && existingSigils.length > 0" @change="toggleSelectAll" /></span>
          <span class="ex-col-name">因子</span>
          <span class="ex-col-level">等级</span>
          <span class="ex-col-trait">特性</span>
        </div>
        <div v-for="s in existingSigils" :key="s.gemUnitId" class="existing-row">
          <span class="ex-col-cb">
            <input type="checkbox" :checked="selectedForDelete.has(s.gemUnitId)"
              @change="selectedForDelete.has(s.gemUnitId) ? selectedForDelete.delete(s.gemUnitId) : selectedForDelete.add(s.gemUnitId)" />
          </span>
          <span class="ex-col-name">{{ s.sigilName }}</span>
          <span class="ex-col-level">Lv {{ s.level }}</span>
          <span class="ex-col-trait">
            {{ s.primaryTraitName }} Lv {{ s.primaryLevel }}
            <template v-if="s.secondaryTraitName"> / {{ s.secondaryTraitName }} Lv {{ s.secondaryLevel }}</template>
          </span>
        </div>
      </div>
    </div>

    <!-- 因子配置 -->
    <div class="section">
      <div class="section-title">因子配置</div>

      <div class="field-row">
      <div class="field">
        <label>因子 <small>{{ dataLoading ? '正在加载目录' : dataError ? '目录加载失败' : '点击下拉后可搜索名称' }}</small></label>
        <div v-if="dataError" class="data-error">{{ dataError }}</div>
        <CatalogSelect v-model="selectedSigilID" :options="sigils" :disabled="dataLoading || !!dataError" placeholder="尚未选择因子" search-placeholder="搜索因子名称" />
      </div>

      <!-- 因子等级 -->
      <div class="field level-field">
        <label>因子等级 <small :class="{ overcap: selectedLevel > 15 }">{{ selectedLevel > 15 ? `超过合规上限 15 / 修改上限 ${sigilWritableMax}` : `合规上限 15 / 修改上限 ${sigilWritableMax}` }}</small></label>
        <input v-model.number="selectedLevel" type="number" min="0" :max="sigilWritableMax" class="text-input compact-number" :class="{ 'lv-over': selectedLevel > 15 }" :disabled="!selectedSigilID" @change="selectedLevel = clampLevel(selectedLevel, sigilWritableMax)" />
      </div>
      </div>

      <!-- 主特性 -->
      <div class="field-row">
      <div class="field">
        <label>主特性</label>
        <div class="readonly-field">{{ primaryTraitName || '—' }}</div>
      </div>

      <div class="field level-field">
        <label>主特性等级 <small :class="{ overcap: selectedPrimaryLevel > primaryNaturalMax }">{{ selectedPrimaryLevel > primaryNaturalMax ? `超过合规上限 ${primaryNaturalMax} / 修改上限 ${primaryWritableMax}` : `合规上限 ${primaryNaturalMax} / 修改上限 ${primaryWritableMax}` }}</small></label>
        <input v-model.number="selectedPrimaryLevel" type="number" min="0" :max="primaryWritableMax" class="text-input compact-number" :class="{ 'lv-over': selectedPrimaryLevel > primaryNaturalMax }" :disabled="!primaryTraitLevels.length" @change="selectedPrimaryLevel = clampLevel(selectedPrimaryLevel, primaryWritableMax)" />
      </div>
      </div>

      <!-- 副特性 -->
      <template v-if="supportsSecondary">
        <div class="field-row">
        <div class="field">
          <label>副特性 <small>非自然组合会提示，但不会阻止写入</small></label>
          <CatalogSelect v-model="selectedSecondaryTraitID" :options="secondaryPickerOptions" :disabled="!secondaryTraits.length" optional placeholder="不选择（生成单词条因子）" search-placeholder="搜索副特性名称" />
        </div>
        <div class="field level-field">
          <label>副特性等级 <small :class="{ overcap: selectedSecondaryLevel > secondaryNaturalMax }">{{ selectedSecondaryLevel > secondaryNaturalMax ? `超过合规上限 ${secondaryNaturalMax} / 修改上限 ${secondaryWritableMax}` : `合规上限 ${secondaryNaturalMax} / 修改上限 ${secondaryWritableMax}` }}</small></label>
          <input v-model.number="selectedSecondaryLevel" type="number" min="0" :max="secondaryWritableMax" class="text-input compact-number" :class="{ 'lv-over': selectedSecondaryLevel > secondaryNaturalMax }" :disabled="!secondaryTraitLevels.length" @change="selectedSecondaryLevel = clampLevel(selectedSecondaryLevel, secondaryWritableMax)" />
        </div>
        </div>
      </template>

      <!-- 数量 + 添加 -->
      <div class="config-footer">
        <LegalityIndicator v-if="selectedSigilID" class="config-legality" :status="legality.status" :message="legality.message" />
        <span v-else class="selection-note">选择因子后显示合法性结果</span>
        <div class="qty-add">
          <div class="field quantity-field">
            <label>数量</label>
            <span class="quantity-combo"><input v-model.number="quantity" type="number" min="1" max="999" class="text-input" /><button type="button" @click="quantity=999">最大</button></span>
          </div>
          <button class="btn-action btn-purple add-btn" @click="addToQueue"
            :disabled="!selectedSigilID || !legality.writable">
            添加到队列
          </button>
        </div>
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
            <span class="queue-name">{{ item.sigilName }} <em v-if="item.legalityStatus === 'forced'" class="queue-warning">强制</em></span>
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
        <button class="btn-action btn-cyan" @click="applyQueueToSave"
          :disabled="isApplying || !queue.length">
          {{ isApplying ? '写入中...' : '应用写入' }}
        </button>
      </div>
      <div v-if="inPlaceEdit" class="danger-hint">警告：启用后，应用写入将直接覆盖当前输入存档，建议先备份。</div>
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
  <ConfirmDialog ref="confirmDialog" />
</template>

<style scoped>
.sigil-container {
  display: flex;
  flex-direction: column;
  gap: 14px;
  width: 100%;
  container-type: inline-size;
}
.save-slots,.output-mode { display:flex;gap:8px;flex-wrap:wrap }.slot-choice,.mode-choice { border:1px solid rgba(145,108,52,.25);background:rgba(244,231,199,.62);color:#705f49;cursor:pointer }.slot-choice { padding:8px 15px }.slot-choice.on,.mode-choice.on { border-color:rgba(42,145,154,.5);background:rgba(89,193,201,.17);color:#286f76;box-shadow:inset 0 -2px #3aa9b3 }.slot-choice.secondary { margin-left:auto }.selected-save { min-height:32px;display:flex;align-items:center;padding:0 11px;border:1px dashed rgba(142,106,52,.28);background:rgba(255,249,229,.38);color:#71634f;font:750 10px/1.4 var(--font-data);font-variant-numeric:tabular-nums lining-nums;overflow:hidden;text-overflow:ellipsis;white-space:nowrap }.selected-save.empty { color:#a18f74 }.selected-save.overwrite { border-style:solid;border-color:rgba(177,104,69,.31);background:rgba(228,166,116,.12) }.mode-choice { flex:1 1 220px;min-height:58px;padding:9px 13px;text-align:left }.mode-choice b,.mode-choice small { display:block }.mode-choice b { font-size:11px }.mode-choice small { margin-top:4px;color:#8f7d64;font-size:9px }.mode-choice.on small { color:#4e7d79 }.output-target { margin-top:2px }.section-title small { color:#9a876c;font-size:9px;font-weight:700;letter-spacing:0 }
.config-footer { display:flex; align-items:flex-end; gap:10px; padding-top:2px; }
.config-legality { flex:1 1 auto; }
.qty-add { flex:0 0 auto; display:flex; align-items:flex-end; gap:8px; }
.quantity-field { width:108px; }
.quantity-field{width:150px}.quantity-combo{display:grid;grid-template-columns:minmax(0,1fr) 45px;gap:5px}.quantity-combo button{border:1px solid rgba(218,187,115,.28);background:rgba(218,187,115,.07);color:#d9bd7c;font-size:10px;cursor:pointer}
.queue-warning { margin-left:5px; padding:1px 5px; border:1px solid rgba(251,191,36,.28); border-radius:4px; color:#fbbf24; background:rgba(245,158,11,.08); font-size:.58rem; font-style:normal; font-weight:700; }
@media (max-width: 720px) { .config-footer { align-items:stretch; flex-direction:column; } .qty-add { width:100%; } .quantity-field { flex:1; width:auto; } }
@container (max-width:500px) { .config-footer { align-items:stretch; flex-direction:column; } .qty-add { width:100%; } .quantity-field { flex:1; width:auto; } .input-row { flex-wrap:wrap; } .input-row>.flex-1 { min-width:0; flex-basis:220px; } .output-target>.flex-1 { flex-basis:100%; } }

.section {
  border-radius: 12px;
  padding: 14px 16px;
  background: rgba(13,27,44,0.9);
  border: 1px solid rgba(148,190,220,0.13);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.apply-section {
  position: relative;
  overflow: hidden;
  z-index: 0;
  transition: border-color 0.3s, box-shadow 0.3s;
}
.apply-section > * { position: relative; z-index: 1; }
.apply-section::after {
  content: "";
  position: absolute;
  inset: 0;
  z-index: 0;
  border-radius: 12px;
  background: #abd373;
  transform: translateY(calc(-100% - 2px));
  transition: transform 0.5s ease;
}
.apply-section.apply-flash { border-color: rgba(171,211,115,0.55); box-shadow: 0 14px 34px rgba(171,211,115,0.18); }
.apply-section.apply-flash::after { transform: translateY(0); }
.apply-section.apply-flash .section-title,
.apply-section.apply-flash .toggle-row { color: #1f2937; }
.apply-section.apply-flash .text-input { border-color: rgba(31,41,55,0.22); background: rgba(255,255,255,0.22); color: #1f2937; }
.apply-section.apply-flash .btn-cyan { border-color: rgba(31,41,55,0.22); background: rgba(31,41,55,0.12); color: #1f2937; }
.apply-section.apply-flash .danger-hint { border-color: rgba(31,41,55,0.18); background: rgba(255,255,255,0.18); color: rgba(31,41,55,0.78); }

.section-danger {
  border-color: rgba(239,68,68,0.15);
  background: rgba(239,68,68,0.04);
}

.section-title {
  font-size: 0.78rem;
  font-weight: 600;
  color: rgba(226,241,255,0.68);
  letter-spacing: 1px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.field { display: flex; flex-direction: column; gap: 4px; }
.level-field { width: 132px; max-width: 100%; }
.field label {
  font-size: 0.7rem;
  color: rgba(226,241,255,0.6);
}
.field label small { margin-left:6px;color:#8b795f;font-size:.9em;font-weight:550; }
.field label small.overcap { color:#984f42;font-weight:700; }
.compact-number { width:84px;max-width:100%;text-align:center; }
.selection-note { flex:1;color:#8f7d64;font-size:.72rem;line-height:1.5; }

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

.select-input option {
  background: #1b2636;
  color: #fff;
}

.text-input:focus, .select-input:focus {
  border-color: rgba(154,116,64,0.4);
  background: transparent;
}

.select-input {
  cursor: pointer;
  appearance: none;
  background-color: transparent;
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
  color: rgba(255,255,255,0.68);
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
  background: rgba(154,116,64,0.15);
  color: #9a7440;
  border: 1px solid rgba(154,116,64,0.3);
}
.btn-cyan:not(:disabled):hover { background: rgba(154,116,64,0.25); }

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
  color: rgba(255,255,255,0.48);
  text-align: center;
  padding: 8px 0;
}

.loading-hint {
  font-size: 0.78rem;
  color: #9a7440;
  text-align: center;
  padding: 12px 0;
}

.warning-hint {
  font-size: 0.72rem;
  color: rgba(251,191,36,0.8);
  text-align: center;
  padding: 8px 12px;
  background: rgba(251,191,36,0.08);
  border: 1px solid rgba(251,191,36,0.15);
  border-radius: 6px;
  line-height: 1.5;
}

.toggle-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.78rem;
  color: rgba(255,255,255,0.75);
}

.danger-hint {
  font-size: 0.72rem;
  color: rgba(248,113,113,0.95);
  text-align: center;
  padding: 8px 12px;
  background: rgba(239,68,68,0.08);
  border: 1px solid rgba(239,68,68,0.2);
  border-radius: 6px;
  line-height: 1.5;
}

.danger-path {
  background: rgba(239,68,68,0.14) !important;
  border-color: rgba(239,68,68,0.55) !important;
  color: #fecaca;
}

/* 因子选择列表 */
.sigil-select {
  height: auto;
  min-height: 120px;
  overflow-y: auto;
  cursor: pointer;
  appearance: auto !important;
  background-image: none !important;
  padding-right: 6px;
}
.sigil-select option {
  min-height: 38px;
  padding: 9px 12px;
  border-bottom: 1px solid rgba(123,88,43,.25);
  color: #fff;
  background: transparent;
  font-size: 0.82rem;
}
.sigil-select option:nth-child(even) { background:rgba(255,255,255,.035); }
.sigil-select option:checked {
  background: rgba(154,116,64,0.25);
  color: #9a7440;
  box-shadow:inset 4px 0 #9a7440;
}

/* 暗色滚动条 */
.sigil-select::-webkit-scrollbar {
  width: 6px;
}
.sigil-select::-webkit-scrollbar-track {
  background: rgba(0,0,0,0.2);
  border-radius: 3px;
}
.sigil-select::-webkit-scrollbar-thumb {
  background: rgba(255,255,255,0.15);
  border-radius: 3px;
}
.sigil-select::-webkit-scrollbar-thumb:hover {
  background: rgba(255,255,255,0.25);
}

.data-error {
  font-size: 0.72rem;
  color: #f87171;
  padding: 4px 0;
}

/* 已有因子列表 */
.existing-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
.btn-sm {
  padding: 4px 10px !important;
  font-size: 0.7rem !important;
}
.existing-table {
  display: flex;
  flex-direction: column;
  gap: 1px;
  background: rgba(255,255,255,0.04);
  border-radius: 8px;
  overflow: hidden;
  max-height: 250px;
  overflow-y: auto;
}
.existing-table::-webkit-scrollbar { width: 5px; }
.existing-table::-webkit-scrollbar-track { background: transparent; }
.existing-table::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.12); border-radius: 3px; }
.existing-row {
  display: flex;
  align-items: center;
  padding: 5px 10px;
  gap: 6px;
  background: rgba(27,38,54,0.6);
  font-size: 0.76rem;
}
.existing-header {
  background: rgba(255,255,255,0.06);
  font-size: 0.7rem;
  color: rgba(255,255,255,0.56);
  font-weight: 600;
  padding: 4px 10px;
}
.existing-row input[type="checkbox"] { accent-color: #9a7440; cursor: pointer; }
.ex-col-cb { width: 20px; flex-shrink: 0; text-align: center; }
.ex-col-name { flex: 1; color: rgba(255,255,255,0.6); min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ex-col-level { width: 40px; text-align: right; color: rgba(255,255,255,0.58); flex-shrink: 0; }
.ex-col-trait { width: 160px; color: rgba(255,255,255,0.54); font-size: 0.7rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex-shrink: 0; }

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
/* Unified Relink workshop language */
.sigil-container{gap:13px}.section{border-color:rgba(154,202,224,.14);border-radius:4px 12px 4px 12px;background:rgba(8,31,53,.7);box-shadow:0 12px 34px rgba(0,7,17,.12)}.section-title{color:#eee7d8;font:500 .9rem Georgia,"Noto Serif SC","STSong",serif;letter-spacing:.04em}.field label{color:#7894a2}.text-input,.select-input{border-color:rgba(154,202,224,.18);border-radius:4px;background:rgba(12,43,68,.78);color:#edf4f6}.select-input option{background:#0a2943}.btn-purple,.btn-green,.btn-cyan{border-color:rgba(218,187,115,.34);background:rgba(218,187,115,.08);color:#f0d99d}.queue-item{border-color:rgba(154,202,224,.12);border-radius:3px 9px 3px 9px;background:rgba(13,45,70,.58)}.queue-name{color:#dce8ec}.queue-detail{color:#6f8b99}
</style>
