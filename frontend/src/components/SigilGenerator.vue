<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { FindSaveFiles, GetLastSavePath, SetLastSavePath } from '../../wailsjs/go/backend/App'
import { GetSigilList, GetTraitList, GetCompatibleSecondaryTraits, GetAllowedLevels,
         GetPrimaryTraitLevels, GetSecondaryTraitLevels,
         LoadSaveFile, GetLoadedSaveInfo,
         GetQueue, AddToQueue, RemoveFromQueue, ClearQueue, CheckLegality,
         ApplyQueue, RemoveAllSigils,
         GetExistingSigils, DeleteSelectedSigils,
         SelectSigilInputSave, SelectSigilOutputSave } from '../../wailsjs/go/backend/SigilGen'
import { backendLanguageReady } from '../backendLanguage'
import { traitAssetIcon } from '../gameAssetIcons'
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
const selectedPrimaryTraitID = ref('')
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
const selectedPrimaryTrait = ref(null)
const supportsSecondary = ref(false)

// 队列
const queue = ref([])
const legality = reactive({ status: 'impossible', writable: false, message: '请先选择因子', reasons: [] })
let legalityTicket = 0
let sigilSelectionEpoch = 0
let secondarySelectionEpoch = 0

// 已有因子
const existingSigils = ref([])
const selectedForDelete = ref(new Set())
const showExisting = ref(false)
const isDeleting = ref(false)
const loadingExisting = ref(false)

const secondaryPickerOptions = computed(() => allTraits.value)
const effectiveSupportsSecondary = computed(() => true)
const editableLevelMax = 50

function traitIconForOption(trait) {
  return traitAssetIcon({
    internalId: trait?.internalId,
    hash: trait?.hash,
    name: trait?.displayName,
  })
}

function sigilIconForOption(sigil) {
  const trait = allTraits.value.find(item => item.internalId === sigil?.primaryTraitId)
  return traitAssetIcon({
    internalId: sigil?.primaryTraitId,
    hash: trait?.hash,
    name: sigil?.primaryTraitName || trait?.displayName,
  })
}

function existingSigilIcon(sigil) {
  return traitAssetIcon({ name: sigil?.primaryTraitName })
}

function queueItemIcon(item) {
  return traitAssetIcon({ internalId: item?.primaryTraitId, name: item?.primaryTraitName })
}
const selectedSigil = computed(() => sigils.value.find(item => item.internalId === selectedSigilID.value) || null)
function effectCurveMax(levels, fallback = 15) {
  const known = (levels || []).filter(level => Number.isInteger(level) && level > 0)
  return known.length ? Math.max(...known) : fallback
}
const sigilNaturalMax = computed(() => effectCurveMax(selectedSigil.value?.allowedSigilLevels))
const primaryNaturalMax = computed(() => effectCurveMax(selectedSigil.value?.allowedFirstTraitLevels))
const secondaryNaturalMax = computed(() => 15)
const primaryWritableMax = computed(() => effectCurveMax(primaryTraitLevels.value))
const secondaryWritableMax = computed(() => effectCurveMax(secondaryTraitLevels.value))

function primaryDefaultLevel(trait, sigil = selectedSigil.value) {
  if (!trait) return 0
  const writableMax = effectCurveMax(
    trait.allowedLevels?.length
      ? trait.allowedLevels
      : Array.from({ length: Math.max(0, Number(trait.maxLevel || 0)) }, (_, index) => index + 1),
  )
  const naturalMax = trait.internalId === sigil?.primaryTraitId
    ? effectCurveMax(sigil?.allowedFirstTraitLevels)
    : 15
  return Math.min(naturalMax, writableMax)
}

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
  const epoch = ++sigilSelectionEpoch
  ++secondarySelectionEpoch
  selectedPrimaryTrait.value = null
  primaryTraitName.value = ''
  selectedPrimaryTraitID.value = ''
  if (!id) return
  const sigil = sigils.value.find(s => s.internalId === id)
  if (!sigil) return

  supportsSecondary.value = Boolean(sigil.supportsSecondaryTrait)

  // 加载等级
  try {
    const levels = await GetAllowedLevels(id)
    if (epoch !== sigilSelectionEpoch) return
    const primaryLevels = await GetPrimaryTraitLevels(id)
    if (epoch !== sigilSelectionEpoch) return
    sigilLevels.value = levels
    primaryTraitLevels.value = primaryLevels
  } catch (e) { if (epoch === sigilSelectionEpoch) showStatus(String(e), 'error'); return }

  selectedPrimaryTraitID.value = sigil.primaryTraitId || ''

  // 副特性
  if (supportsSecondary.value) {
    try {
      const allowed = await GetCompatibleSecondaryTraits(id)
      if (epoch !== sigilSelectionEpoch) return
      allowedSecondaryIDs.value = new Set(allowed.map(t => t.internalId))
      secondaryTraits.value = allowed
      selectedSecondaryTraitID.value = ''
    } catch (e) {
      if (epoch !== sigilSelectionEpoch) return
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
  selectedLevel.value = clampLevel(Number(sigil.defaultSigilLevel || sigilNaturalMax.value), editableLevelMax)
  selectedPrimaryLevel.value = primaryDefaultLevel(selectedPrimaryTrait.value, sigil)
})

function buildCurrentItem() {
  return {
    sigilId: selectedSigilID.value,
    sigilName: '',
    level: selectedLevel.value,
    primaryTraitId: selectedPrimaryTraitID.value,
    primaryTraitName: '',
    primaryLevel: selectedPrimaryLevel.value,
    secondaryTraitId: effectiveSupportsSecondary.value ? selectedSecondaryTraitID.value : '',
    secondaryTraitName: '',
    secondaryLevel: effectiveSupportsSecondary.value ? selectedSecondaryLevel.value : 0,
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

watch([selectedSigilID, selectedLevel, selectedPrimaryTraitID, selectedPrimaryLevel, selectedSecondaryTraitID, selectedSecondaryLevel, quantity], refreshLegality)

watch(selectedPrimaryTraitID, id => {
  const trait = allTraits.value.find(item => item.internalId === id) || null
  selectedPrimaryTrait.value = trait
  primaryTraitName.value = trait?.displayName || ''
  primaryTraitLevels.value = trait?.allowedLevels?.length
    ? [...trait.allowedLevels]
    : Array.from({ length: Math.max(0, Number(trait?.maxLevel || 0)) }, (_, index) => index + 1)
  selectedPrimaryLevel.value = primaryDefaultLevel(trait)
})

watch(selectedSecondaryTraitID, async (id) => {
  const epoch = ++secondarySelectionEpoch
  const sigilID = selectedSigilID.value
  if (!id || !selectedSigilID.value) {
    secondaryTraitLevels.value = []
    selectedSecondaryLevel.value = 0
    return
  }
  try {
    const levels = await GetSecondaryTraitLevels(sigilID, id)
    if (epoch !== secondarySelectionEpoch || sigilID !== selectedSigilID.value || id !== selectedSecondaryTraitID.value) return
    secondaryTraitLevels.value = levels
    selectedSecondaryLevel.value = Math.min(15, effectCurveMax(levels))
  } catch (e) { if (epoch === secondarySelectionEpoch) secondaryTraitLevels.value = [] }
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
    <div class="section ui-card compact-save-bar">
      <div class="section-title ui-section-title"><span>选择存档槽</span><small>与物品、武器页面使用同一组存档</small></div>
      <div class="save-slots">
        <button v-for="slot in slots" :key="slot.index" class="slot-choice ui-btn is-sm" :class="{ on: inputPath === slot.path }" @click="selectSaveSlot(slot.path)">{{ saveSlotLabel(slot) }}</button>
        <button class="slot-choice secondary ui-btn is-sm" @click="browseInput">选择其他存档</button>
      </div>
      <div class="selected-save" :class="{ empty: !inputPath }">{{ inputPath || '尚未选择存档' }}</div>
      <div v-if="saveLoaded" class="save-info">
        已加载 · {{ saveInfo.occupiedSigils }} 个因子 · 最大槽位 {{ saveInfo.maxSlotId }}
      </div>
    </div>

    <!-- 已有因子 -->
    <div v-if="showExisting" class="section ui-card">
      <div class="section-title ui-section-title">
        已有因子 {{ loadingExisting ? '加载中...' : `(${existingSigils.length})` }}
        <div class="existing-actions">
          <button class="btn-link ui-btn is-subtle" @click="toggleSelectAll"
            :disabled="loadingExisting">
            {{ selectedForDelete.size === existingSigils.length ? '取消全选' : '全选' }}
          </button>
          <button class="btn-link ui-btn is-subtle" @click="refreshExisting" :disabled="loadingExisting">刷新</button>
          <button class="btn-action btn-red btn-sm ui-btn is-danger is-sm"
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
      <div v-if="!loadingExisting && existingSigils.length === 0" class="empty-hint ui-empty">暂无已有因子或读取失败</div>
      <div v-else class="existing-table">
        <div class="existing-row existing-header">
          <span class="ex-col-cb"><input type="checkbox" :checked="selectedForDelete.size === existingSigils.length && existingSigils.length > 0" @change="toggleSelectAll" /></span>
          <span class="ex-col-name">因子</span>
          <span class="ex-col-level">等级</span>
          <span class="ex-col-trait">特性</span>
        </div>
        <div v-for="s in existingSigils" :key="s.gemUnitId" class="existing-row ui-row">
          <span class="ex-col-cb">
            <input type="checkbox" :checked="selectedForDelete.has(s.gemUnitId)"
              @change="selectedForDelete.has(s.gemUnitId) ? selectedForDelete.delete(s.gemUnitId) : selectedForDelete.add(s.gemUnitId)" />
          </span>
          <span class="ex-col-name existing-name-cell">
            <img v-if="existingSigilIcon(s)" class="factor-icon" :src="existingSigilIcon(s)" alt="" />
            <span>{{ s.sigilName }}</span>
          </span>
          <span class="ex-col-level">Lv {{ s.level }}</span>
          <span class="ex-col-trait">
            {{ s.primaryTraitName }} Lv {{ s.primaryLevel }}
            <template v-if="s.secondaryTraitName"> / {{ s.secondaryTraitName }} Lv {{ s.secondaryLevel }}</template>
          </span>
        </div>
      </div>
    </div>

    <!-- 因子配置 -->
    <div class="section ui-card">
      <div class="section-title ui-section-title">因子配置</div>

      <div class="field-row">
      <div class="field ui-field">
        <label class="ui-field-label">因子 <small>{{ dataLoading ? '正在加载目录' : dataError ? '目录加载失败' : '点击下拉后可搜索名称' }}</small></label>
        <div v-if="dataError" class="data-error">{{ dataError }}</div>
        <CatalogSelect v-model="selectedSigilID" :options="sigils" :disabled="dataLoading || !!dataError" :icon-resolver="sigilIconForOption" placeholder="尚未选择因子" search-placeholder="搜索因子名称" />
      </div>

      <!-- 因子等级 -->
      <div class="field level-field ui-field">
        <label class="ui-field-label">因子等级 <small>自然参考 {{ sigilNaturalMax }} / 修改上限 {{ editableLevelMax }}</small></label>
        <input v-model.number="selectedLevel" type="number" min="0" :max="editableLevelMax" class="text-input compact-number ui-input" :disabled="!selectedSigilID" @change="selectedLevel = clampLevel(selectedLevel, editableLevelMax)" />
      </div>
      </div>

      <!-- 主特性 -->
      <div class="field-row">
      <div class="field ui-field">
        <label class="ui-field-label">主特性 <small>由 gem.tbl 固定</small></label>
        <CatalogSelect v-model="selectedPrimaryTraitID" :options="allTraits" :icon-resolver="traitIconForOption" placeholder="选择主特性" search-placeholder="搜索全部特性" />
      </div>

      <div class="field level-field ui-field">
        <label class="ui-field-label">主特性等级 <small :class="{ overcap: selectedPrimaryLevel > primaryNaturalMax }">{{ selectedPrimaryLevel > primaryNaturalMax ? `高于自然参考 ${primaryNaturalMax} / 修改上限 ${primaryWritableMax}` : `自然参考 ${primaryNaturalMax} / 修改上限 ${primaryWritableMax}` }}</small></label>
        <input v-model.number="selectedPrimaryLevel" type="number" min="0" :max="primaryWritableMax" class="text-input compact-number ui-input" :disabled="!selectedPrimaryTraitID" @change="selectedPrimaryLevel = clampLevel(selectedPrimaryLevel, primaryWritableMax)" />
      </div>
      </div>

      <!-- 副特性 -->
      <template v-if="effectiveSupportsSecondary">
        <div class="field-row">
        <div class="field ui-field">
          <label class="ui-field-label">副特性 <small>显示全部已知特性；天然词池仅用于提醒</small></label>
          <CatalogSelect v-model="selectedSecondaryTraitID" :options="secondaryPickerOptions" :disabled="!secondaryPickerOptions.length" :icon-resolver="traitIconForOption" optional placeholder="不选择（生成单词条因子）" search-placeholder="搜索副特性名称" />
        </div>
        <div class="field level-field ui-field">
          <label class="ui-field-label">副特性等级 <small :class="{ overcap: selectedSecondaryLevel > secondaryNaturalMax }">{{ selectedSecondaryLevel > secondaryNaturalMax ? `高于自然参考 ${secondaryNaturalMax} / 修改上限 ${secondaryWritableMax}` : `自然参考 ${secondaryNaturalMax} / 修改上限 ${secondaryWritableMax}` }}</small></label>
          <input v-model.number="selectedSecondaryLevel" type="number" min="0" :max="secondaryWritableMax" class="text-input compact-number ui-input" :disabled="!selectedSecondaryTraitID" @change="selectedSecondaryLevel = clampLevel(selectedSecondaryLevel, secondaryWritableMax)" />
        </div>
        </div>
      </template>

      <!-- 数量 + 添加 -->
      <div class="config-footer">
        <small class="ui-hint">天然等级是默认值；最高可填到对应技能效果曲线的目录上限。</small>
        <LegalityIndicator v-if="selectedSigilID" class="config-legality" :status="legality.status" :message="legality.message" />
        <span v-else class="selection-note">选择因子后显示合法性结果</span>
        <div class="qty-add">
          <div class="field quantity-field ui-field">
            <label class="ui-field-label">数量</label>
            <span class="quantity-combo"><input v-model.number="quantity" type="number" min="1" max="999" class="text-input ui-input" /><button class="ui-btn is-sm" type="button" @click="quantity=999">最大</button></span>
          </div>
          <button class="btn-action btn-purple add-btn ui-btn is-primary" @click="addToQueue"
            :disabled="!selectedSigilID || !legality.writable">
            添加到队列
          </button>
        </div>
      </div>
    </div>

    <div class="sigil-lower-grid">
    <!-- 队列 -->
    <div class="section ui-card">
      <div class="section-title ui-section-title">
        队列 ({{ queue.length }})
        <button v-if="queue.length" class="btn-link ui-btn is-subtle" @click="clearQueueAll">清空</button>
      </div>
      <div v-if="!queue.length" class="empty-hint ui-empty">暂无待写入因子，请先添加</div>
      <div v-else class="queue-list">
        <div v-for="(item, i) in queue" :key="i" class="queue-item ui-row">
          <img v-if="queueItemIcon(item)" class="queue-icon" :src="queueItemIcon(item)" alt="" />
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
          <button class="btn-icon ui-btn is-subtle" @click="removeFromQueue(i)" title="移除">✕</button>
        </div>
      </div>
    </div>

    <!-- 输出 + 应用 -->
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
        <button class="btn-action btn-cyan ui-btn is-primary" @click="applyQueueToSave"
          :disabled="isApplying || !queue.length">
          {{ isApplying ? '写入中...' : '应用写入' }}
        </button>
      </div>
    </div>
    </div>

    <!-- 清除所有 -->
    <details class="section ui-card section-danger">
      <summary class="section-title ui-section-title">危险操作</summary>
      <div class="danger-body">
        <button class="btn-action btn-red ui-btn is-danger" @click="removeAll"
          :disabled="!inputPath.trim() || !outputPath.trim()">
          清除输出存档中所有因子
        </button>
      </div>
    </details>
  </div>
  <ConfirmDialog ref="confirmDialog" />
</template>

<style scoped>
.sigil-container {
  width:100%;
  max-width:1000px;
  display:flex;
  flex-direction:column;
  gap:var(--space-5);
  color:var(--text-secondary);
  container-type:inline-size;
}

.section {
  min-width:0;
  padding:var(--space-6);
}
.compact-save-bar { padding:var(--space-4) var(--space-5); }
.section-title { margin-bottom:var(--space-4); }
.section-title > small { margin-left:auto; text-align:right; }
.save-slots,
.existing-actions {
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
.loading-hint,
.warning-hint,
.data-error {
  padding:var(--space-3) var(--space-4);
  border-radius:var(--radius-sm);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}
.loading-hint { background:var(--surface-sunken); color:var(--text-secondary); }
.warning-hint { border-left:3px solid var(--warning); background:var(--warning-bg); color:var(--warning-ink); }
.data-error { border-left:3px solid var(--danger); background:var(--danger-bg); color:var(--danger-ink); }
.empty-hint { color:var(--text-secondary); font-size:var(--fs-md); text-align:center; }

.existing-table {
  max-height:320px;
  overflow:auto;
  border:1px solid var(--border-soft);
  border-radius:var(--radius-md);
  background:var(--surface-card-pop);
}
.existing-row {
  display:grid;
  grid-template-columns:28px minmax(140px,.9fr) 70px minmax(220px,1.5fr);
  align-items:center;
  gap:var(--space-3);
  min-height:40px;
  padding:var(--space-2) var(--space-4);
  border:0;
  border-bottom:1px solid var(--border-soft);
  border-radius:0;
  font-size:var(--fs-sm);
}
.existing-row:last-child { border-bottom:0; }
.existing-header {
  position:sticky;
  top:0;
  z-index:1;
  min-height:34px;
  background:var(--surface-sunken);
  color:var(--text-secondary);
  font-weight:var(--fw-semibold);
}
.ex-col-cb { display:flex; justify-content:center; }
.ex-col-name { min-width:0; overflow:hidden; color:var(--text-primary); text-overflow:ellipsis; white-space:nowrap; }
.existing-name-cell { display:flex; align-items:center; gap:var(--space-2); }
.existing-name-cell > span { min-width:0; overflow:hidden; text-overflow:ellipsis; }
.factor-icon { width:32px; height:32px; flex:0 0 32px; object-fit:cover; border:1px solid var(--line-soft); border-radius:6px; background:var(--surface-field); }
.ex-col-level { color:var(--info-ink); font-family:var(--font-data); }
.ex-col-trait { min-width:0; color:var(--text-secondary); line-height:var(--lh-normal); }

.field-row {
  display:flex;
  align-items:flex-end;
  gap:var(--space-4);
  margin-bottom:var(--space-4);
}
.field { min-width:0; flex:1; }
.field label small {
  display:block;
  margin-top:var(--space-1);
  color:var(--text-secondary);
  font-size:var(--fs-xs);
  font-weight:var(--fw-normal);
  line-height:var(--lh-normal);
}
.level-field { width:220px; flex:0 0 220px; }
.compact-number { width:100%; }
.readonly-field {
  display:flex;
  align-items:center;
  width:100%;
  color:var(--text-secondary);
  gap:var(--space-3);
}
.overcap { color:var(--warning-ink); }
.lv-over { border-color:var(--warning); background:var(--warning-bg); color:var(--warning-ink); }

.config-footer {
  display:flex;
  align-items:flex-end;
  justify-content:space-between;
  gap:var(--space-5);
  padding-top:var(--space-4);
  border-top:1px solid var(--border-soft);
}
.config-legality { min-width:0; flex:1; }
.selection-note { color:var(--text-secondary); font-size:var(--fs-sm); }
.qty-add { display:flex; align-items:flex-end; gap:var(--space-3); }
.quantity-field { width:184px; flex:0 0 184px; }
.quantity-combo { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-2); }
.quantity-combo .ui-input { min-width:0; width:100%; }

.sigil-lower-grid {
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
  gap:var(--space-5);
  align-items:start;
}
.sigil-lower-grid .section { height:100%; box-sizing:border-box; }
.queue-list { display:flex; flex-direction:column; gap:var(--space-2); }
.queue-item {
  justify-content:space-between;
  border-color:var(--border-soft);
  background:var(--surface-card-pop);
}
.queue-icon { width:40px; height:40px; flex:0 0 40px; object-fit:cover; border:1px solid var(--line-soft); border-radius:7px; background:var(--surface-field); }
.queue-info { display:flex; min-width:0; flex-direction:column; gap:var(--space-1); }
.queue-name { color:var(--text-primary); font-size:var(--fs-md); font-weight:var(--fw-semibold); }
.queue-detail { color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.queue-warning {
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
.flex-1 { min-width:0; flex:1; }
.output-target .selected-save { margin-top:0; }
.danger-hint {
  margin-top:var(--space-3);
  padding:var(--space-3) var(--space-4);
  border-left:3px solid var(--warning);
  border-radius:var(--radius-sm);
  background:var(--warning-bg);
  color:var(--warning-ink);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}
.apply-flash { animation:apply-confirm var(--dur-base) var(--ease-out) 2 alternate; }
@keyframes apply-confirm {
  to { border-color:var(--success); box-shadow:0 0 0 2px var(--success-bg); }
}

.section-danger { padding:0; overflow:hidden; }
.section-danger > summary {
  margin:0;
  padding:var(--space-4) var(--space-5);
  color:var(--danger-ink);
  cursor:pointer;
  list-style:none;
}
.section-danger > summary::-webkit-details-marker { display:none; }
.section-danger > summary::after { content:"＋"; margin-left:auto; }
.section-danger[open] > summary::after { content:"−"; }
.danger-body {
  padding:0 var(--space-5) var(--space-5);
  border-top:1px solid var(--border-soft);
}
.danger-body .ui-btn { margin-top:var(--space-4); }

input[type="checkbox"] { accent-color:var(--accent); }

@container (max-width:760px) {
  .sigil-lower-grid { grid-template-columns:1fr; }
  .config-footer { align-items:stretch; flex-direction:column; }
  .qty-add { width:100%; }
  .quantity-field { width:auto; flex:1; }
}

@container (max-width:620px) {
  .section,
  .compact-save-bar { padding:var(--space-4); }
  .field-row { align-items:stretch; flex-direction:column; }
  .level-field { width:100%; flex-basis:auto; }
  .section-title { align-items:flex-start; }
  .section-title > small { width:100%; margin-left:0; text-align:left; }
  .existing-table { overflow-x:auto; }
  .existing-row { min-width:620px; }
  .output-mode { grid-template-columns:1fr; }
  .input-row { align-items:stretch; flex-direction:column; }
  .input-row > * { width:100%; }
  .output-target .selected-save { box-sizing:border-box; }
}

@container (max-width:440px) {
  .save-slots .ui-btn { flex:1; }
  .qty-add { align-items:stretch; flex-direction:column; }
  .quantity-field { width:100%; }
  .add-btn { width:100%; }
}
</style>
