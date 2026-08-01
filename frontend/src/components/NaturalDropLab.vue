<script setup>
import { computed, onActivated, onBeforeUnmount, reactive, ref, watch } from 'vue'
import {
  CharaAcquire,
  CharaRelease,
  DeployNaturalDropMod,
  GetNaturalDropWorkspace,
  RestoreNaturalDropDefaults,
  SelectNaturalDropGameExecutable,
  SelectNaturalDropTableDirectory,
  TaskRewardMultiplierSetOwned,
  TaskRewardMultiplierStatusOwned,
} from '../../wailsjs/go/backend/App'
import { language } from '../i18n'
import { itemAssetIcon, summonAssetIcon, traitAssetIcon } from '../gameAssetIcons'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'
import CatalogSelect from './CatalogSelect.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const emit = defineEmits(['status'])

const sourceDir = ref('')
const gameExePath = ref('')
const workspace = ref(null)
const busy = ref(false)
const actionMessage = ref('')
const actionTone = ref('')
const confirmDialog = ref(null)
const selections = reactive({})
const sigilSelections = reactive({})
const wrightstoneSelections = reactive({})
const itemSelections = reactive({})
const summonDraft = reactive({ typeHash: '', mainTrait: '', subParam: '' })
const sigilDraft = reactive({ sigilHash: '', secondaryTrait: '' })
const wrightstoneDraft = reactive({ familyHash: '', subTrait1: '', subTrait2: '' })
const genericDropItem = ref('')
const genericDropWeight = ref(10000)
const taskRewardMultipliers = [1, 2, 4, 8, 16]
const taskRewardConnected = ref(false)
const taskRewardBusy = ref(false)
const taskRewardStatus = reactive({ enabled: false, multiplier: 1, matchedItems: 0, rva: 0, gameVersion: '', detail: '' })
const taskRewardLeaseScope = 'task-reward-multiplier'
let taskRewardOwnerToken = ''
let taskRewardDisposed = false
let taskRewardEpoch = 0

const tx = (zh, en) => language.value === 'en' ? en : zh
const displayName = item => language.value === 'en' ? item?.nameEn : item?.nameZh
const displayKind = item => language.value === 'en' ? item?.typeNameEn : item?.typeNameZh
const usingBundledTables = computed(() => sourceDir.value === 'builtin://dlc-2.0.2')
const tableReady = computed(() => workspace.value?.summonTablesReady === true)
const sigilTableReady = computed(() => workspace.value?.sigilTablesReady === true)
const wrightstoneTableReady = computed(() => workspace.value?.wrightstoneTablesReady === true)
const itemTableReady = computed(() => workspace.value?.itemTablesReady === true)
const gameReady = computed(() => Boolean(workspace.value?.indexValid && gameExePath.value))
const selectedList = computed(() => Object.entries(selections)
  .filter(([, value]) => value?.enabled)
  .map(([typeHash, value]) => ({ typeHash, mainTrait: value.mainTrait, subParam: value.subParam })))
const selectedWrightstones = computed(() => Object.values(wrightstoneSelections).flatMap(value => (value?.variants || [])
  .filter(variant => variant.enabled)
  .map(variant => ({ mainTrait: value.mainTrait, subTrait1: variant.subTrait1, subTrait2: variant.subTrait2 }))))
const selectedSigils = computed(() => Object.entries(sigilSelections)
  .filter(([, value]) => value?.enabled)
  .map(([sigilHash, value]) => ({ sigilHash, secondaryTrait: value.secondaryTrait || '' })))
const selectedItems = computed(() => Object.entries(itemSelections)
  .filter(([, value]) => value?.enabled)
  .map(([itemHash, value]) => ({ itemHash, quantity: Number(value.quantity), weight: Number(value.weight) })))
const itemQuantitiesValid = computed(() => selectedItems.value.every(item => Number.isInteger(item.quantity) && item.quantity >= 1 && item.quantity <= 999))
const activeConflictScopes = computed(() => new Set([
  ...(selectedList.value.length ? ['summon'] : []),
  ...(selectedSigils.value.length ? ['sigil', 'transmarvel'] : []),
  ...(selectedWrightstones.value.length ? ['wrightstone', 'transmarvel'] : []),
  ...(selectedItems.value.length ? ['item'] : []),
]))
const activeConflicts = computed(() => (workspace.value?.conflicts || []).filter(item => activeConflictScopes.value.has(item.scope)))
const hasConflicts = computed(() => activeConflicts.value.length > 0)
const canDeploy = computed(() => gameReady.value && !hasConflicts.value && !busy.value && itemQuantitiesValid.value &&
  ((selectedList.value.length > 0 && tableReady.value) || (selectedSigils.value.length > 0 && sigilTableReady.value) ||
    (selectedWrightstones.value.length > 0 && wrightstoneTableReady.value) || (selectedItems.value.length > 0 && itemTableReady.value)) &&
  (selectedList.value.length === 0 || tableReady.value) && (selectedSigils.value.length === 0 || sigilTableReady.value) &&
  (selectedWrightstones.value.length === 0 || wrightstoneTableReady.value) && (selectedItems.value.length === 0 || itemTableReady.value))
const canRestore = computed(() => Boolean(workspace.value?.owned && gameReady.value && !busy.value))

function pickerOptions(items, key, detail = () => '') {
  return (items || []).map(item => ({
    internalId: item?.[key] || '',
    displayName: displayName(item),
    detail: detail(item),
    source: item,
  })).filter(item => item.internalId)
}

function applyTaskRewardStatus(status) {
  Object.assign(taskRewardStatus, status || { enabled: false, multiplier: 1, matchedItems: 0, rva: 0, gameVersion: '', detail: '' })
}

async function connectTaskRewardRuntime() {
  if (taskRewardBusy.value || taskRewardConnected.value) return
  const epoch = ++taskRewardEpoch
  taskRewardBusy.value = true
  let acquiredOwnerToken = ''
  try {
    const info = await CharaAcquire(nextRuntimeAcquireRequestID())
    acquiredOwnerToken = String(info?.ownerToken || '')
    if (!acquiredOwnerToken) throw new Error(tx('后端未返回运行时连接令牌。', 'The backend did not return a runtime owner token.'))
    if (taskRewardDisposed || epoch !== taskRewardEpoch) {
      queueRuntimeLeaseRelease(taskRewardLeaseScope, acquiredOwnerToken, CharaRelease)
      return
    }
    taskRewardOwnerToken = acquiredOwnerToken
    taskRewardConnected.value = true
    applyTaskRewardStatus(await TaskRewardMultiplierStatusOwned(taskRewardOwnerToken))
    setMessage(tx('已连接游戏。选择 2× / 4× / 8× / 16× 后，倍率会持续作用于后续任务结算。', 'Connected to the game. Choose 2× / 4× / 8× / 16× to keep multiplying future quest results.'), 'ok')
  } catch (error) {
    if (acquiredOwnerToken) {
      try { await releaseRuntimeLease(taskRewardLeaseScope, acquiredOwnerToken, CharaRelease) } catch {}
    }
    if (!taskRewardDisposed && epoch === taskRewardEpoch) setMessage(String(error), 'danger')
  } finally {
    if (!taskRewardDisposed && epoch === taskRewardEpoch) taskRewardBusy.value = false
  }
}

async function disconnectTaskRewardRuntime() {
  if (taskRewardBusy.value || !taskRewardOwnerToken) return
  const epoch = ++taskRewardEpoch
  const ownerToken = taskRewardOwnerToken
  taskRewardBusy.value = true
  try {
    await releaseRuntimeLease(taskRewardLeaseScope, ownerToken, CharaRelease)
    if (taskRewardDisposed || epoch !== taskRewardEpoch) return
    taskRewardOwnerToken = ''
    taskRewardConnected.value = false
    applyTaskRewardStatus(null)
    setMessage(tx('已断开游戏，任务奖励倍率恢复为 1×。', 'Disconnected from the game and restored quest rewards to 1×.'), 'ok')
  } catch (error) {
    if (!taskRewardDisposed && epoch === taskRewardEpoch) setMessage(String(error), 'danger')
  } finally {
    if (!taskRewardDisposed && epoch === taskRewardEpoch) taskRewardBusy.value = false
  }
}

async function setTaskRewardMultiplier(multiplier) {
  if (taskRewardBusy.value || !taskRewardOwnerToken) return
  taskRewardBusy.value = true
  try {
    applyTaskRewardStatus(await TaskRewardMultiplierSetOwned(taskRewardOwnerToken, multiplier))
    setMessage(multiplier === 1
      ? tx('任务奖励倍率已关闭，后续任务恢复游戏原数量。', 'Quest reward multiplier is off; future quests use normal quantities.')
      : tx(`任务奖励倍率已设为 ${multiplier}×，保持连接即可持续生效。`, `Quest reward multiplier is set to ${multiplier}× and remains active while connected.`), 'ok')
  } catch (error) {
    setMessage(String(error), 'danger')
  } finally {
    taskRewardBusy.value = false
  }
}

function traitPickerOptions(items) {
  return pickerOptions(items, 'hash')
}

const summonPickerOptions = computed(() => pickerOptions(workspace.value?.summons, 'typeHash', item => `${item.tier} · ${displayKind(item)}`))
const sigilPickerOptions = computed(() => pickerOptions(workspace.value?.sigils, 'sigilHash', item => item.nativeTransmarvel ? tx('原生池', 'Native pool') : tx('加入池', 'Added to pool')))
const wrightstonePickerOptions = computed(() => (workspace.value?.wrightstones || []).map(item => ({
  internalId: item.mainTrait.hash,
  displayName: displayName(item),
  detail: displayName(item.mainTrait),
  source: item,
})))
const itemPickerOptions = computed(() => pickerOptions(workspace.value?.items, 'hash', item => item.category || ''))
const selectedSummonDraft = computed(() => (workspace.value?.summons || []).find(item => item.typeHash === summonDraft.typeHash))
const selectedSigilDraft = computed(() => (workspace.value?.sigils || []).find(item => item.sigilHash === sigilDraft.sigilHash))
const selectedWrightstoneDraft = computed(() => (workspace.value?.wrightstones || []).find(item => item.mainTrait.hash === wrightstoneDraft.familyHash))
const selectedItemDraft = computed(() => (workspace.value?.items || []).find(item => item.hash === genericDropItem.value))
const summonMainOptions = computed(() => traitPickerOptions(selectedSummonDraft.value?.mainTraits))
const summonSubOptions = computed(() => traitPickerOptions(selectedSummonDraft.value?.subParams))
const sigilSecondaryOptions = computed(() => traitPickerOptions(selectedSigilDraft.value?.secondaryTraits))
const wrightstoneSubOptions = computed(() => traitPickerOptions(selectedWrightstoneDraft.value?.subTraits))
function syncDraftDefaults() {
  if (!summonDraft.typeHash && summonPickerOptions.value[0]) summonDraft.typeHash = summonPickerOptions.value[0].internalId
  if (!sigilDraft.sigilHash && sigilPickerOptions.value[0]) sigilDraft.sigilHash = sigilPickerOptions.value[0].internalId
  if (!wrightstoneDraft.familyHash && wrightstonePickerOptions.value[0]) wrightstoneDraft.familyHash = wrightstonePickerOptions.value[0].internalId
  if (!genericDropItem.value && itemPickerOptions.value[0]) genericDropItem.value = itemPickerOptions.value[0].internalId
}

function summonIcon(item) {
  return summonAssetIcon({ typeHash: item?.typeHash })
}

function traitIcon(item) {
  return traitAssetIcon({ internalId: item?.internalId, hash: item?.hash, name: displayName(item) })
}

function sigilIcon(item) {
  return traitIcon(item?.primaryTrait)
}

function selectedOption(options, hash) {
  return (options || []).find(item => item.hash === hash)
}

function pickerSummonIcon(option) {
  return summonIcon(option?.source)
}

function pickerSigilIcon(option) {
  return sigilIcon(option?.source)
}

function pickerTraitIcon(option) {
  return traitIcon(option?.source)
}

function pickerWrightstoneIcon(option) {
  return traitIcon(option?.source?.mainTrait)
}

function pickerItemIcon(option) {
  return itemAssetIcon(option?.source)
}

const pendingDropRows = computed(() => {
  const rows = []
  for (const [sigilHash, selection] of Object.entries(sigilSelections)) {
    if (!selection?.enabled) continue
    const item = (workspace.value?.sigils || []).find(candidate => candidate.sigilHash === sigilHash)
    if (!item) continue
    const secondary = selectedOption(item.secondaryTraits, selection.secondaryTrait)
    rows.push({
      key: `sigil:${sigilHash}`, kind: 'sigil', target: sigilHash,
      kindLabel: tx('因子', 'Sigil'), name: displayName(item), icon: sigilIcon(item),
      detail: `${displayName(item.primaryTrait)} · ${secondary ? displayName(secondary) : tx('副技能随机', 'Random secondary')}`,
      sourceLabel: item.nativeTransmarvel ? tx('原生池', 'Native pool') : tx('加入池', 'Added to pool'),
      quantityLabel: tx('1 条规则', '1 rule'),
    })
  }
  for (const [typeHash, selection] of Object.entries(selections)) {
    if (!selection?.enabled) continue
    const item = (workspace.value?.summons || []).find(candidate => candidate.typeHash === typeHash)
    if (!item) continue
    rows.push({
      key: `summon:${typeHash}`, kind: 'summon', target: typeHash,
      kindLabel: tx('召唤石', 'Summon'), name: displayName(item), icon: summonIcon(item),
      detail: `${displayName(selectedOption(item.mainTraits, selection.mainTrait))} · ${displayName(selectedOption(item.subParams, selection.subParam))}`,
      sourceLabel: tx('天然掉落', 'Natural drop'),
      quantityLabel: tx('1 条规则', '1 rule'),
    })
  }
  for (const [familyHash, value] of Object.entries(wrightstoneSelections)) {
    const family = (workspace.value?.wrightstones || []).find(candidate => candidate.mainTrait.hash === familyHash)
    if (!family) continue
    for (const [index, variant] of (value?.variants || []).entries()) {
      if (!variant.enabled) continue
      rows.push({
        key: `wrightstone:${familyHash}:${index}`, kind: 'wrightstone', target: familyHash, variantIndex: index,
        kindLabel: tx('祝福石', 'Wrightstone'), name: displayName(family), icon: traitIcon(family.mainTrait),
        detail: `${displayName(family.mainTrait)} Lv20 · ${displayName(selectedOption(family.subTraits, variant.subTrait1))} Lv15 · ${displayName(selectedOption(family.subTraits, variant.subTrait2))} Lv10`,
        sourceLabel: tx('Transmarvel 锻造', 'Transmarvel forging'),
        quantityLabel: tx('1 个变体', '1 variant'),
      })
    }
  }
  for (const [itemHash, selection] of Object.entries(itemSelections)) {
    if (!selection?.enabled) continue
    const item = (workspace.value?.items || []).find(candidate => candidate.hash === itemHash)
    if (!item) continue
    rows.push({
      key: `item:${itemHash}`, kind: 'item', target: itemHash,
      kindLabel: tx('物品', 'Item'), name: displayName(item), icon: itemAssetIcon(item),
      detail: tx(`每次抽中直接掉落 ${selection.quantity} 个 · 抽取权重 ${selection.weight}`, `Drops ${selection.quantity} each time it is rolled · weight ${selection.weight}`),
      sourceLabel: language.value === 'en' ? workspace.value?.itemRewardTargetEn : workspace.value?.itemRewardTargetZh,
    })
  }
  return rows
})

function addSummonDraft() {
  if (!selectedSummonDraft.value || !summonDraft.mainTrait || !summonDraft.subParam) return
  selections[summonDraft.typeHash] = {
    enabled: true,
    mainTrait: summonDraft.mainTrait,
    subParam: summonDraft.subParam,
  }
  setMessage(tx(`已把 ${displayName(selectedSummonDraft.value)} 加入待部署清单。`, `${displayName(selectedSummonDraft.value)} was added to the deployment list.`))
}

function addSigilDraft() {
  if (!selectedSigilDraft.value) return
  sigilSelections[sigilDraft.sigilHash] = {
    enabled: true,
    secondaryTrait: sigilDraft.secondaryTrait || '',
  }
  setMessage(tx(`已把 ${displayName(selectedSigilDraft.value)} 加入待部署清单。`, `${displayName(selectedSigilDraft.value)} was added to the deployment list.`))
}

function addWrightstoneDraft() {
  const family = selectedWrightstoneDraft.value
  if (!family || !wrightstoneDraft.subTrait1 || !wrightstoneDraft.subTrait2) return
  if (wrightstoneDraft.subTrait1 === family.mainTrait.hash || wrightstoneDraft.subTrait2 === family.mainTrait.hash) {
    setMessage(tx('祝福石副词条不能与固定主词条重复。', 'Wrightstone sub traits cannot duplicate the fixed primary trait.'), 'danger')
    return
  }
  const state = wrightstoneSelections[wrightstoneDraft.familyHash] || {
    mainTrait: family.mainTrait.hash,
    variants: [],
  }
  wrightstoneSelections[wrightstoneDraft.familyHash] = state
  const duplicate = state.variants.find(variant => variant.enabled &&
    variant.subTrait1 === wrightstoneDraft.subTrait1 && variant.subTrait2 === wrightstoneDraft.subTrait2)
  if (duplicate) {
    setMessage(tx('这个祝福石变体已经在待部署清单中。', 'This wrightstone variant is already in the deployment list.'), 'danger')
    return
  }
  let slot = state.variants.find(variant => !variant.enabled)
  if (!slot) {
    if (state.variants.length >= (family.maxVariants || 3)) {
      setMessage(tx(`每种祝福石最多添加 ${family.maxVariants || 3} 个变体。`, `Each wrightstone family supports up to ${family.maxVariants || 3} variants.`), 'danger')
      return
    }
    slot = {}
    state.variants.push(slot)
  }
  Object.assign(slot, {
    enabled: true,
    subTrait1: wrightstoneDraft.subTrait1,
    subTrait2: wrightstoneDraft.subTrait2,
  })
  setMessage(tx(`已把 ${displayName(family)} 加入待部署清单。`, `${displayName(family)} was added to the deployment list.`))
}

function addItemDraft() {
  const item = selectedItemDraft.value
  const weight = Math.trunc(Number(genericDropWeight.value))
  if (!item) return
  if (!Number.isInteger(weight) || weight < 1 || weight > 1000000) {
    setMessage(tx('掉落权重必须是 1–1,000,000 的整数。', 'Drop weight must be an integer from 1 to 1,000,000.'), 'danger')
    return
  }
  const existing = itemSelections[item.hash]
  itemSelections[item.hash] = { enabled: true, quantity: existing?.quantity || 1, weight }
  setMessage(tx(`已把 ${displayName(item)} 加入待部署清单；可在清单里直接调整掉落数量。`, `${displayName(item)} was added to the deployment list. Adjust its quantity directly in the list.`))
}

function removePendingDrop(row) {
  if (row.kind === 'sigil') delete sigilSelections[row.target]
  if (row.kind === 'summon') delete selections[row.target]
  if (row.kind === 'wrightstone') {
    const state = wrightstoneSelections[row.target]
    if (state?.variants?.[row.variantIndex]) state.variants.splice(row.variantIndex, 1)
    if (!state?.variants?.some(variant => variant.enabled)) delete wrightstoneSelections[row.target]
  }
  if (row.kind === 'item') delete itemSelections[row.target]
}

function clearPendingDrops() {
  for (const key of Object.keys(selections)) delete selections[key]
  for (const key of Object.keys(sigilSelections)) delete sigilSelections[key]
  for (const key of Object.keys(wrightstoneSelections)) delete wrightstoneSelections[key]
  for (const key of Object.keys(itemSelections)) delete itemSelections[key]
  setMessage(tx('已清空待部署清单。', 'The pending deployment list was cleared.'))
}

function setMessage(message, tone = 'info') {
  actionMessage.value = message
  actionTone.value = tone
  emit('status', message, tone === 'danger' ? 'error' : tone === 'ok' ? 'success' : 'info')
}

async function refreshWorkspace() {
  busy.value = true
  actionMessage.value = ''
  try {
    workspace.value = await GetNaturalDropWorkspace(sourceDir.value, gameExePath.value)
    sourceDir.value = workspace.value?.sourceDir || sourceDir.value
    gameExePath.value = workspace.value?.gameExePath || gameExePath.value
    syncDraftDefaults()
  } catch (error) {
    workspace.value = null
    setMessage(String(error), 'danger')
  } finally {
    busy.value = false
  }
}

async function chooseSource() {
  const path = await SelectNaturalDropTableDirectory()
  if (!path) return
  sourceDir.value = path
  await refreshWorkspace()
}

async function chooseGame() {
  const path = await SelectNaturalDropGameExecutable()
  if (!path) return
  gameExePath.value = path
  await refreshWorkspace()
}

watch(() => summonDraft.typeHash, typeHash => {
  const item = (workspace.value?.summons || []).find(candidate => candidate.typeHash === typeHash)
  summonDraft.mainTrait = item?.mainTraits?.[0]?.hash || ''
  summonDraft.subParam = item?.subParams?.[0]?.hash || ''
})
watch(() => sigilDraft.sigilHash, sigilHash => {
  const item = (workspace.value?.sigils || []).find(candidate => candidate.sigilHash === sigilHash)
  sigilDraft.secondaryTrait = item?.secondaryTraits?.[0]?.hash || ''
})
watch(() => wrightstoneDraft.familyHash, familyHash => {
  const family = (workspace.value?.wrightstones || []).find(candidate => candidate.mainTrait.hash === familyHash)
  wrightstoneDraft.subTrait1 = family?.subTraits?.[0]?.hash || ''
  wrightstoneDraft.subTrait2 = family?.subTraits?.[1]?.hash || family?.subTraits?.[0]?.hash || ''
})

async function deploy() {
  if (!canDeploy.value) return
  const accepted = await confirmDialog.value?.ask({
    title: tx('部署天然掉落模组', 'Deploy natural-drop mod'),
    message: tx(`将部署 ${selectedList.value.length} 颗召唤石、${selectedSigils.value.length} 个 Transmarvel 因子、${selectedWrightstones.value.length} 个祝福石变体和 ${selectedItems.value.length} 种物品。`, `Deploy ${selectedList.value.length} summons, ${selectedSigils.value.length} Transmarvel sigils, ${selectedWrightstones.value.length} wrightstone variants and ${selectedItems.value.length} items.`),
    detail: tx('每种物品按待部署清单里的数量写入，不与实时全任务倍率相乘。应用会备份原始 data.i；游戏必须完全退出。', 'Each item uses the exact quantity shown in the pending list and does not stack with the live all-quest multiplier. The app backs up data.i; the game must be closed.'),
    confirmLabel: tx('确认部署', 'Deploy'),
    cancelLabel: tx('取消', 'Cancel'),
    tone: 'warning',
  })
  if (!accepted) return
  busy.value = true
  try {
    const result = await DeployNaturalDropMod({
      sourceDir: sourceDir.value,
      gameExePath: gameExePath.value,
      selections: selectedList.value,
      sigils: selectedSigils.value,
      wrightstones: selectedWrightstones.value,
      items: selectedItems.value,
      itemMultiplier: 1,
      sigilOnly: false,
      wrightstoneOnly: false,
    })
    setMessage(tx(`已部署 ${result.selectedSummons} 颗召唤石、${result.selectedSigils} 个因子、${result.selectedWrightstones} 个祝福石变体与 ${result.selectedItems} 种物品。现在可正常启动游戏。`, `Deployed ${result.selectedSummons} summons, ${result.selectedSigils} sigils, ${result.selectedWrightstones} wrightstone variants and ${result.selectedItems} items. The game can now be launched normally.`), 'ok')
    await refreshWorkspace()
  } catch (error) {
    setMessage(String(error), 'danger')
  } finally {
    busy.value = false
  }
}

async function restoreOrRemove() {
  if (!canRestore.value) return
  const accepted = await confirmDialog.value?.ask({
    title: tx('恢复游戏原始掉落', 'Restore original drops'),
    message: tx('将恢复部署前自动备份的 data.i，并移除本工具生成的掉落表。', 'Restore the automatically backed-up data.i and remove the drop tables generated by this tool.'),
    detail: tx('请先完全退出游戏。恢复只接受本工具清单和 SHA-256 校验都通过的备份。', 'Close the game first. Restoration only accepts a tool-owned backup whose manifest and SHA-256 both validate.'),
    confirmLabel: tx('确认恢复', 'Restore'), cancelLabel: tx('取消', 'Cancel'), tone: 'warning',
  })
  if (!accepted) return
  busy.value = true
  try {
    await RestoreNaturalDropDefaults({ gameExePath: gameExePath.value })
    setMessage(tx('原始 data.i 已恢复，本工具生成的掉落表和备份清单已清理。', 'The original data.i has been restored, and the generated tables and deployment manifest were removed.'), 'ok')
    await refreshWorkspace()
  } catch (error) {
    setMessage(String(error), 'danger')
  } finally {
    busy.value = false
  }
}

onActivated(() => {
  if (!workspace.value) void refreshWorkspace()
})

onBeforeUnmount(() => {
  taskRewardDisposed = true
  taskRewardEpoch += 1
  if (taskRewardOwnerToken) queueRuntimeLeaseRelease(taskRewardLeaseScope, taskRewardOwnerToken, CharaRelease)
})
</script>

<template>
  <div class="natural-drop-lab ui-page-stack">
    <section class="drop-intro">
      <div>
        <p class="drop-kicker">{{ tx('常用倍率 + 高级指定掉落', 'COMMON MULTIPLIER + ADVANCED CUSTOM DROPS') }}</p>
        <h2>{{ tx('任务奖励与指定掉落', 'Quest rewards and custom drops') }}</h2>
        <p>{{ tx('平时只需连接游戏并选择任务奖励倍率。需要指定因子、祝福石、召唤石或物品时，再展开下方高级功能，把想要的内容加入清单后统一部署。', 'For everyday use, connect to the game and choose a quest reward multiplier. Open Advanced only when you want specific sigils, wrightstones, summons, or items, then add them to one list and deploy together.') }}</p>
      </div>
      <div class="drop-safety">
        <strong>{{ tx('两种修改互相独立', 'The two paths are independent') }}</strong>
        <span>{{ tx('实时倍率不写游戏文件；只有高级清单部署会备份并修改 data.i', 'The live multiplier does not edit game files. Only an advanced-list deployment backs up and changes data.i') }}</span>
      </div>
    </section>

    <section class="task-reward-runtime ui-card ui-panel" aria-labelledby="task-reward-title">
      <div class="task-reward-copy">
        <span>{{ tx('常用功能 · 游戏运行时', 'COMMON · WHILE THE GAME IS RUNNING') }}</span>
        <h3 id="task-reward-title">{{ tx('全任务普通物品倍率', 'All-quest regular-item multiplier') }}</h3>
        <p>{{ tx('先连接游戏，再点 2× / 4× / 8× / 16×。保持连接后，普通任务、首领任务、任务宝箱和支线目标进入结算页的可堆叠物品会持续按所选倍率结算。', 'Connect to the game, then choose 2× / 4× / 8× / 16×. While connected, stackable items reaching the results screen from regular quests, bosses, quest chests, and side objectives keep using that multiplier.') }}</p>
        <small>{{ tx('因子、祝福石、召唤石和武器属于独立实例奖励，保持原数量，避免复制同一个实例；商店购买、锻造和手工物品编辑不受影响。单项最终数量封顶 999。这里改的是 PC 本地任务结果；跨平台队友是否收到同样倍率，仍需双方实机验收。', 'Sigils, wrightstones, summons, and weapons are independent instances and keep their normal count to avoid duplicating one instance. Shops, forging, and manual item editing are unaffected. Each final stack is capped at 999. This changes the local PC result records; whether cross-platform teammates receive the same multiplier still requires paired in-game verification.') }}</small>
      </div>
      <div class="task-reward-controls">
        <div class="ui-seg task-reward-options" :aria-disabled="!taskRewardConnected">
          <button v-for="multiplier in taskRewardMultipliers" :key="multiplier" type="button" class="ui-seg-btn" :class="{ 'is-on': taskRewardStatus.multiplier === multiplier }" :disabled="!taskRewardConnected || taskRewardBusy" @click="setTaskRewardMultiplier(multiplier)">{{ multiplier }}×</button>
        </div>
        <div class="task-reward-actions">
          <button v-if="!taskRewardConnected" class="ui-btn is-primary" type="button" :disabled="taskRewardBusy" @click="connectTaskRewardRuntime">{{ taskRewardBusy ? tx('连接中…', 'Connecting…') : tx('连接游戏', 'Connect to game') }}</button>
          <button v-else class="ui-btn is-secondary" type="button" :disabled="taskRewardBusy" @click="disconnectTaskRewardRuntime">{{ tx('断开并恢复 1×', 'Disconnect and restore 1×') }}</button>
        </div>
        <div class="task-reward-state" :class="{ active: taskRewardStatus.enabled }"><b>{{ taskRewardConnected ? (taskRewardStatus.enabled ? tx(`${taskRewardStatus.multiplier}× 持续生效`, `${taskRewardStatus.multiplier}× active`) : tx('已连接 · 当前 1×', 'Connected · currently 1×')) : tx('尚未连接游戏', 'Not connected') }}</b><small v-if="taskRewardStatus.enabled">{{ tx(`当前倍率已处理 ${taskRewardStatus.matchedItems} 条普通物品奖励`, `${taskRewardStatus.matchedItems} ordinary item rewards processed at the current multiplier`) }}</small></div>
      </div>
    </section>

    <details class="advanced-drop-panel">
      <summary>
        <span class="advanced-toggle" aria-hidden="true">＋</span>
        <div class="advanced-summary-copy">
          <span>{{ tx('高级功能 · 需要时再展开', 'ADVANCED · OPEN WHEN NEEDED') }}</span>
          <b>{{ tx('指定掉落与锻造内容', 'Custom drops and forging results') }}</b>
          <small>{{ tx('把想要的因子、祝福石、召唤石或物品逐条加入同一份待部署清单，再统一核对和写入游戏文件。', 'Add the sigils, wrightstones, summons, or items you want to one pending list, review it, then write the game files once.') }}</small>
        </div>
        <strong>{{ pendingDropRows.length ? tx(`${pendingDropRows.length} 项待部署`, `${pendingDropRows.length} pending`) : tx('当前未配置', 'Not configured') }}</strong>
      </summary>

      <div class="advanced-drop-content">
    <section class="drop-setup" aria-labelledby="drop-setup-title">
      <div class="section-heading">
        <div><h3 id="drop-setup-title">{{ tx('第一步 · 确认游戏目录', 'Step 1 · Verify the game folder') }}</h3><p>{{ tx('十一张内置表会自动完成大小和 SHA-256 校验；正常使用只需选择游戏程序。维护者仍可切换到本机表做严格对照。', 'All eleven built-in tables are checked automatically by size and SHA-256. Normal use only requires the game executable; maintainers can still select local tables for strict comparison.') }}</p></div>
        <span v-if="workspace?.owned" class="install-state owned">{{ workspace.installed ? tx('天然掉落已启用', 'Natural drops enabled') : tx('部署状态需恢复', 'Deployment needs recovery') }}</span>
      </div>
      <div class="path-rows">
        <div class="path-row">
          <span class="path-index">01</span>
          <div><b>{{ tx('2.0.2 原表模板', '2.0.2 source templates') }}</b><code :title="sourceDir">{{ usingBundledTables ? tx('程序内置 · 11/11 逐张 SHA-256 校验', 'Built in · 11/11 files SHA-256 verified') : sourceDir }}</code></div>
          <button class="ui-btn is-sm is-ghost" type="button" :disabled="busy" @click="chooseSource">{{ tx('维护者对照', 'Maintainer override') }}</button>
        </div>
        <div class="path-row">
          <span class="path-index">02</span>
          <div><b>{{ tx('游戏程序', 'Game executable') }}</b><code :title="gameExePath">{{ gameExePath || tx('尚未选择', 'Not selected') }}</code></div>
          <button class="ui-btn is-sm" type="button" :disabled="busy" @click="chooseGame">{{ tx('选择游戏', 'Choose game') }}</button>
        </div>
      </div>
      <div v-if="workspace?.tables?.length" class="table-ledger" aria-label="table validation">
        <div v-for="item in workspace.tables" :key="item.name" class="table-line" :class="{ valid: item.valid }">
          <span class="table-mark" aria-hidden="true">{{ item.valid ? '✓' : '!' }}</span><b>{{ item.name }}</b><span>{{ item.size.toLocaleString() }} B</span><code>{{ item.sha256.slice(0, 12) }}…</code>
        </div>
      </div>
      <div v-if="hasConflicts" class="ui-notice is-danger">
        <b>{{ tx('发现表覆盖冲突，部署已锁定。', 'Table override conflicts found; deployment is locked.') }}</b>
        <span v-for="conflict in activeConflicts" :key="`${conflict.modId}:${conflict.file}`">{{ conflict.modId }} · {{ conflict.file }}</span>
      </div>
      <div v-if="gameReady" class="ui-notice is-ok">
        <b>{{ tx('原生索引已验证', 'Native index verified') }}</b>
        <span>{{ workspace.indexSummary }}</span>
        <code>{{ workspace.indexPath }}</code>
      </div>
    </section>

    <section class="drop-configurator" aria-labelledby="drop-configurator-title">
      <div class="section-heading">
        <div>
          <h3 id="drop-configurator-title">{{ tx('第二步 · 添加指定掉落', 'Step 2 · Add custom drops') }}</h3>
          <p>{{ tx('四类内容可以同时添加，不需要先选模式。选中的内容才会进入待部署清单；没加入的内容保持游戏原样。所有下拉框都可搜索名称或内部编号。', 'All four content types can be added together without choosing a mode. Only entries added to the pending list are changed; everything else stays untouched. Every picker supports names and internal ids.') }}</p>
        </div>
      </div>

      <div class="drop-builders">
        <article class="drop-builder" :class="{ unavailable: !sigilTableReady }">
          <header>
            <div><span class="builder-index">A</span><div><h4>{{ tx('Transmarvel 因子', 'Transmarvel sigil') }}</h4><p>{{ tx('因子 → 固定主技能 → 随机或指定副技能', 'Sigil → fixed primary → random or pinned secondary') }}</p></div></div>
          </header>
          <div class="drop-builder-grid">
            <label class="builder-field"><span>{{ tx('因子', 'Sigil') }}</span><CatalogSelect v-model="sigilDraft.sigilHash" :options="sigilPickerOptions" :disabled="!sigilTableReady" :icon-resolver="pickerSigilIcon" :placeholder="tx('选择因子', 'Choose a sigil')" :search-placeholder="tx('搜索因子或哈希', 'Search sigils or hashes')" detail-key="detail" /></label>
            <div class="builder-field"><span>{{ tx('主技能（由因子固定）', 'Primary trait (fixed)') }}</span><div class="drop-readonly"><img v-if="traitIcon(selectedSigilDraft?.primaryTrait)" class="drop-trait-icon" :src="traitIcon(selectedSigilDraft?.primaryTrait)" alt=""><span>{{ displayName(selectedSigilDraft?.primaryTrait) || tx('先选择因子', 'Choose a sigil first') }}</span></div></div>
            <label class="builder-field"><span>{{ tx('副技能', 'Secondary trait') }}</span><CatalogSelect v-model="sigilDraft.secondaryTrait" :options="sigilSecondaryOptions" :disabled="!selectedSigilDraft" :icon-resolver="pickerTraitIcon" optional :placeholder="tx('按游戏原池随机', 'Random from game pool')" :search-placeholder="tx('搜索合法副技能', 'Search compatible traits')" /></label>
            <button class="ui-btn is-primary builder-add" type="button" :disabled="!selectedSigilDraft" @click="addSigilDraft">{{ tx('加入待部署', 'Add to list') }}</button>
          </div>
          <p class="builder-note">{{ tx('主技能由因子本体决定；副技能可留空按原池随机，也可指定。加入后不会排斥下方的祝福石或召唤石。', 'The sigil determines its primary trait. Leave the secondary blank for the original pool or pin one. Adding it does not exclude wrightstones or summons below.') }}</p>
        </article>

        <article class="drop-builder" :class="{ unavailable: !wrightstoneTableReady }">
          <header>
            <div><span class="builder-index">B</span><div><h4>{{ tx('Transmarvel 祝福石', 'Transmarvel wrightstone') }}</h4><p>{{ tx('祝福种类/主词条 → 副词条 Lv15 → 副词条 Lv10', 'Family / primary → Lv15 sub trait → Lv10 sub trait') }}</p></div></div>
          </header>
          <div class="drop-builder-grid">
            <label class="builder-field"><span>{{ tx('祝福种类（主词条 Lv20）', 'Family (primary Lv20)') }}</span><CatalogSelect v-model="wrightstoneDraft.familyHash" :options="wrightstonePickerOptions" :disabled="!wrightstoneTableReady" :icon-resolver="pickerWrightstoneIcon" :placeholder="tx('选择祝福石', 'Choose a wrightstone')" :search-placeholder="tx('搜索祝福或主词条', 'Search families or primary traits')" detail-key="detail" /></label>
            <label class="builder-field"><span>{{ tx('副词条 Lv15', 'Sub trait Lv15') }}</span><CatalogSelect v-model="wrightstoneDraft.subTrait1" :options="wrightstoneSubOptions" :disabled="!selectedWrightstoneDraft" :icon-resolver="pickerTraitIcon" :placeholder="tx('选择副词条', 'Choose sub trait')" :search-placeholder="tx('搜索副词条', 'Search sub traits')" /></label>
            <label class="builder-field"><span>{{ tx('副词条 Lv10', 'Sub trait Lv10') }}</span><CatalogSelect v-model="wrightstoneDraft.subTrait2" :options="wrightstoneSubOptions" :disabled="!selectedWrightstoneDraft" :icon-resolver="pickerTraitIcon" :placeholder="tx('选择副词条', 'Choose sub trait')" :search-placeholder="tx('搜索副词条', 'Search sub traits')" /></label>
            <button class="ui-btn is-primary builder-add" type="button" :disabled="!selectedWrightstoneDraft || !wrightstoneDraft.subTrait1 || !wrightstoneDraft.subTrait2" @click="addWrightstoneDraft">{{ tx('加入待部署', 'Add to list') }}</button>
          </div>
          <p class="builder-note">{{ tx('每种祝福最多添加三个不同变体；加入哪一种，就只把哪一种写进清单。', 'Each family supports up to three distinct variants. Only the variants you add are written to the pending list.') }}</p>
        </article>

        <article class="drop-builder" :class="{ unavailable: !tableReady }">
          <header>
            <div><span class="builder-index">C</span><div><h4>{{ tx('召唤石天然掉落', 'Natural summon drop') }}</h4><p>{{ tx('召唤石种类 → 主加护 → 附加词条', 'Summon type → main trait → bonus trait') }}</p></div></div>
          </header>
          <div class="drop-builder-grid">
            <label class="builder-field"><span>{{ tx('召唤石种类', 'Summon type') }}</span><CatalogSelect v-model="summonDraft.typeHash" :options="summonPickerOptions" :disabled="!tableReady" :icon-resolver="pickerSummonIcon" :placeholder="tx('选择召唤石', 'Choose a summon')" :search-placeholder="tx('搜索召唤石或哈希', 'Search summons or hashes')" detail-key="detail" /></label>
            <label class="builder-field"><span>{{ tx('主加护', 'Main trait') }}</span><CatalogSelect v-model="summonDraft.mainTrait" :options="summonMainOptions" :disabled="!selectedSummonDraft" :icon-resolver="pickerTraitIcon" :placeholder="tx('选择主加护', 'Choose main trait')" :search-placeholder="tx('搜索主加护', 'Search main traits')" /></label>
            <label class="builder-field"><span>{{ tx('附加词条', 'Bonus trait') }}</span><CatalogSelect v-model="summonDraft.subParam" :options="summonSubOptions" :disabled="!selectedSummonDraft" :icon-resolver="pickerTraitIcon" :placeholder="tx('选择附加词条', 'Choose bonus trait')" :search-placeholder="tx('搜索附加词条', 'Search bonus traits')" /></label>
            <button class="ui-btn is-primary builder-add" type="button" :disabled="!selectedSummonDraft || !summonDraft.mainTrait || !summonDraft.subParam" @click="addSummonDraft">{{ tx('加入待部署', 'Add to list') }}</button>
          </div>
          <p class="builder-note">{{ tx('选择种类、主加护和附加词条后加入清单；没有加入的召唤石掉落保持不变。', 'Choose a summon, its main trait, and bonus trait, then add it. Summons not added here stay unchanged.') }}</p>
        </article>
        <article class="drop-builder" :class="{ unavailable: !itemTableReady }">
          <header>
            <div><span class="builder-index">D</span><div><h4>{{ tx('普通物品掉落', 'Regular item drop') }}</h4><p>{{ tx('物品 → 抽取权重；加入后在清单里填写每次掉落数量', 'Item → roll weight; set its per-roll quantity in the pending list') }}</p></div></div>
          </header>
          <div class="drop-builder-grid item-builder-grid">
            <label class="builder-field item-picker"><span>{{ tx('掉落物品', 'Drop item') }}</span><CatalogSelect v-model="genericDropItem" :options="itemPickerOptions" :disabled="!itemTableReady" :icon-resolver="pickerItemIcon" :placeholder="tx('选择物品', 'Choose an item')" :search-placeholder="tx('搜索物品名称或 Hash', 'Search item names or hashes')" detail-key="detail" /></label>
            <label class="builder-field"><span>{{ tx('抽取权重', 'Roll weight') }}</span><input v-model.number="genericDropWeight" class="drop-number-input" type="number" min="1" max="1000000" step="1"></label>
            <button class="ui-btn is-primary builder-add" type="button" :disabled="!itemTableReady || !selectedItemDraft" @click="addItemDraft">{{ tx('加入待部署', 'Add to list') }}</button>
          </div>
          <p class="builder-note">{{ tx('加入后默认每次掉落 1 个，可在下方清单直接改为 1–999。它只写入无尽模式锻造师奖励池，不与上方实时倍率相乘。', 'New entries default to 1 item per roll; edit the quantity to 1–999 in the list below. This only writes the Endless Mode forger reward pool and does not stack with the live multiplier above.') }}</p>
        </article>
      </div>
    </section>

    <section class="pending-drops" aria-labelledby="pending-drops-title">
      <div class="section-heading">
        <div><h3 id="pending-drops-title">{{ tx('第三步 · 核对待部署清单', 'Step 3 · Review the pending list') }}</h3><p>{{ tx('清单逐条写明类型、词条、目标位置和数量。普通物品的数量可直接修改；删除一行就不会部署那一项。', 'The list shows each type, trait setup, destination, and quantity. Regular-item quantities are editable; removing a row excludes it from deployment.') }}</p></div>
        <button v-if="pendingDropRows.length" class="ui-btn is-sm is-ghost" type="button" @click="clearPendingDrops">{{ tx('清空清单', 'Clear list') }}</button>
      </div>
      <div v-if="pendingDropRows.length" class="pending-drop-table">
        <div class="pending-drop-head" aria-hidden="true"><span>{{ tx('类型', 'Type') }}</span><span>{{ tx('掉落项目与完整配置', 'Drop entry and full configuration') }}</span><span>{{ tx('写入位置', 'Destination') }}</span><span>{{ tx('数量', 'Quantity') }}</span><span>{{ tx('操作', 'Action') }}</span></div>
        <article v-for="row in pendingDropRows" :key="row.key" class="pending-drop-row">
          <span class="pending-kind">{{ row.kindLabel }}</span>
          <div class="pending-main"><img v-if="row.icon" class="drop-item-icon" :src="row.icon" alt=""><div><b>{{ row.name }}</b><small>{{ row.detail }}</small></div></div>
          <span class="pending-source">{{ row.sourceLabel }}</span>
          <label v-if="row.kind === 'item'" class="pending-quantity"><span class="sr-only">{{ tx(`${row.name} 数量`, `${row.name} quantity`) }}</span><input v-model.number="itemSelections[row.target].quantity" class="drop-number-input" type="number" min="1" max="999" step="1"></label>
          <span v-else class="pending-quantity-label">{{ row.quantityLabel }}</span>
          <button class="ui-btn is-sm is-ghost pending-remove" type="button" @click="removePendingDrop(row)">{{ tx('移除', 'Remove') }}</button>
        </article>
      </div>
      <div v-else class="pending-empty"><span aria-hidden="true">＋</span><div><b>{{ tx('还没有待部署项目', 'No pending entries') }}</b><small>{{ tx('在上方选择一条配置并点击“加入待部署”。', 'Choose a configuration above and click “Add to list.”') }}</small></div></div>
    </section>

    <section class="deploy-dock" :class="{ ready: canDeploy }">
      <div><b>{{ tx('第四步 · 生成并部署', 'Step 4 · Build and deploy') }}</b><span v-if="!tableReady && !sigilTableReady && !wrightstoneTableReady && !itemTableReady">{{ tx('内置表校验未通过', 'Built-in table verification failed') }}</span><span v-else-if="hasConflicts">{{ tx('先处理冲突模组', 'Resolve conflicting mods first') }}</span><span v-else-if="!itemQuantitiesValid">{{ tx('普通物品数量必须是 1–999 的整数', 'Regular-item quantities must be integers from 1–999') }}</span><span v-else>{{ tx(`${selectedSigils.length} 个因子 · ${selectedList.length} 颗召唤石 · ${selectedWrightstones.length} 个祝福石变体 · ${selectedItems.length} 种物品`, `${selectedSigils.length} sigils · ${selectedList.length} summons · ${selectedWrightstones.length} wrightstone variants · ${selectedItems.length} items`) }}</span></div>
      <div class="dock-actions"><button v-if="workspace?.owned" class="ui-btn is-secondary" type="button" :disabled="!canRestore" @click="restoreOrRemove">{{ tx('恢复原始掉落', 'Restore original drops') }}</button><button class="ui-btn is-primary" type="button" :disabled="!canDeploy" @click="deploy">{{ busy ? tx('处理中…', 'Working…') : workspace?.installed ? tx('更新部署', 'Update deployment') : tx('生成并部署', 'Build and deploy') }}</button></div>
    </section>
      </div>
    </details>
    <div v-if="actionMessage" class="ui-notice" :class="{ 'is-danger': actionTone === 'danger', 'is-ok': actionTone === 'ok', 'is-info': actionTone === 'info' }" role="status">{{ actionMessage }}</div>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.natural-drop-lab { container: natural-drop / inline-size; padding-bottom:86px; }
.drop-intro { display:grid; grid-template-columns:minmax(0,1fr) minmax(230px,310px); gap:var(--space-6); align-items:end; padding:var(--space-6) 0 var(--space-5); border-bottom:1px solid var(--border-default); }
.drop-kicker { margin:0 0 var(--space-2); color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.06em; }
.drop-intro h2,.section-heading h3 { margin:0; color:var(--text-primary); font-family:var(--font-display); }
.drop-intro h2 { font-size:var(--fs-2xl); }
.drop-intro p,.section-heading p { margin:var(--space-2) 0 0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.drop-safety { padding:var(--space-4); border-left:3px solid var(--success); background:var(--success-bg); color:var(--success-ink); }
.drop-safety strong,.drop-safety span { display:block; }
.drop-safety span { margin-top:var(--space-1); font-size:var(--fs-xs); }
.task-reward-runtime { display:grid; grid-template-columns:minmax(0,1fr) auto; align-items:center; gap:var(--space-5); padding:var(--space-5); border-left:4px solid var(--accent); background:var(--surface-card-pop); }
.task-reward-copy > span { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.05em; }
.task-reward-copy h3 { margin:4px 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); }
.task-reward-copy p,.task-reward-copy small { display:block; margin:var(--space-2) 0 0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.task-reward-copy small { color:var(--text-muted); font-size:var(--fs-xs); }
.task-reward-controls { display:grid; min-width:310px; gap:var(--space-3); justify-items:end; }
.task-reward-options { width:100%; }
.task-reward-options .ui-seg-btn { flex:1 1 0; }
.task-reward-actions { display:flex; justify-content:flex-end; }
.task-reward-state { display:grid; justify-items:end; color:var(--text-muted); font-size:var(--fs-xs); }
.task-reward-state.active { color:var(--success-ink); }
.task-reward-state small { margin-top:2px; }
.advanced-drop-panel { min-width:0; overflow:hidden; border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-card); box-shadow:var(--shadow-1); }
.advanced-drop-panel > summary { display:grid; grid-template-columns:38px minmax(0,1fr) auto; align-items:center; gap:var(--space-3); min-height:78px; padding:var(--space-4); color:var(--text-primary); cursor:pointer; list-style:none; background:linear-gradient(90deg,var(--surface-card-pop),var(--surface-card)); }
.advanced-drop-panel > summary::-webkit-details-marker { display:none; }
.advanced-drop-panel > summary:hover { background:var(--accent-soft); }
.advanced-toggle { display:grid; width:34px; height:34px; place-items:center; border:1px solid var(--accent-border); border-radius:50%; color:var(--accent); background:var(--surface-field); font-size:var(--fs-lg); transition:transform .18s ease; }
.advanced-drop-panel[open] .advanced-toggle { transform:rotate(45deg); }
.advanced-summary-copy { display:grid; gap:2px; min-width:0; }
.advanced-summary-copy > span { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.04em; }
.advanced-summary-copy > b { font-family:var(--font-display); font-size:var(--fs-md); }
.advanced-summary-copy > small { color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.advanced-drop-panel > summary > strong { padding:5px 9px; border:1px solid var(--border-default); border-radius:999px; color:var(--accent-strong); background:var(--surface-field); font-size:var(--fs-xs); white-space:nowrap; }
.advanced-drop-content { display:grid; gap:var(--space-6); padding:var(--space-5); border-top:1px solid var(--border-default); }
.drop-setup,.drop-configurator,.pending-drops { min-width:0; }
.section-heading { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-5); }
.section-heading > div { min-width:0; }
.section-heading h3 { font-size:var(--fs-lg); }
.install-state { flex:0 0 auto; padding:5px 9px; border:1px solid var(--warning); border-radius:var(--radius-sm); color:var(--warning-ink); background:var(--warning-bg); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.install-state.owned { border-color:var(--success); color:var(--success-ink); background:var(--success-bg); }
.path-rows { margin-top:var(--space-4); border-top:1px solid var(--border-default); }
.path-row { display:grid; grid-template-columns:36px minmax(0,1fr) auto; align-items:center; gap:var(--space-3); min-height:64px; padding:var(--space-3) 0; border-bottom:1px solid var(--border-default); }
.path-index { display:grid; place-items:center; width:30px; height:30px; border:1px solid var(--accent-border); border-radius:50%; color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.path-row b,.path-row code { display:block; }
.path-row b { color:var(--text-primary); font-size:var(--fs-sm); }
.path-row code { max-width:100%; margin-top:3px; overflow:hidden; color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); text-overflow:ellipsis; white-space:nowrap; }
.table-ledger { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:1px; margin-top:var(--space-4); overflow:hidden; border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--border-default); }
.table-line { display:grid; grid-template-columns:22px minmax(0,1fr) auto auto; align-items:center; gap:var(--space-2); min-width:0; padding:9px 11px; color:var(--danger-ink); background:var(--surface-card-pop); font-size:var(--fs-xs); }
.table-line.valid { color:var(--success-ink); }
.table-line b { overflow:hidden; color:var(--text-primary); text-overflow:ellipsis; white-space:nowrap; }
.table-line code { color:var(--text-muted); font-family:var(--font-data); }
.table-mark { display:grid; place-items:center; width:19px; height:19px; border:1px solid currentColor; border-radius:50%; font-weight:var(--fw-bold); }
.drop-setup .ui-notice { display:flex; flex-direction:column; gap:3px; margin-top:var(--space-4); }
.drop-item-icon { display:block; width:42px; height:42px; flex:0 0 42px; border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-field); object-fit:contain; }
.drop-trait-icon { display:block; width:24px; height:24px; flex:0 0 24px; border-radius:5px; object-fit:contain; }
.drop-builders { margin-top:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-card); box-shadow:var(--shadow-1); }
.drop-builder { padding:var(--space-4); border-bottom:1px solid var(--border-default); }
.drop-builder:last-child { border-bottom:0; }
.drop-builder.unavailable { opacity:.62; }
.drop-builder > header { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-4); margin-bottom:var(--space-3); }
.drop-builder > header > div { display:flex; align-items:flex-start; gap:var(--space-3); min-width:0; }
.builder-index { display:grid; width:28px; height:28px; flex:0 0 28px; place-items:center; border:1px solid var(--accent-border); border-radius:50%; color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.drop-builder h4 { margin:0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-md); }
.drop-builder header p { margin:2px 0 0; color:var(--text-muted); font-size:var(--fs-xs); }
.drop-builder-grid { display:grid; grid-template-columns:minmax(180px,1.2fr) minmax(160px,1fr) minmax(160px,1fr) auto; align-items:end; gap:var(--space-3); }
.item-builder-grid { grid-template-columns:minmax(220px,1fr) minmax(160px,.45fr) auto; }
.builder-field { display:block; min-width:0; }
.builder-field > span { display:block; min-height:18px; margin-bottom:5px; color:var(--text-secondary); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.drop-number-input { width:100%; min-height:var(--control-height); padding:0 var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-sm); color:var(--text-primary); background:var(--surface-field); font:inherit; font-size:var(--fs-sm); }
.drop-number-input:focus { border-color:var(--accent); outline:2px solid color-mix(in srgb,var(--accent) 18%,transparent); outline-offset:1px; }
.drop-readonly { display:flex; align-items:center; gap:var(--space-2); min-height:var(--control-height); padding:0 var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-sm); color:var(--text-secondary); background:var(--surface-field); font-size:var(--fs-sm); }
.drop-readonly > span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.builder-add { min-width:112px; min-height:var(--control-height); }
.builder-note { margin:var(--space-2) 0 0 41px; color:var(--text-muted); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.pending-drop-table { margin-top:var(--space-4); overflow:hidden; border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-card); box-shadow:var(--shadow-1); }
.pending-drop-head,.pending-drop-row { display:grid; grid-template-columns:80px minmax(220px,1fr) minmax(120px,.28fr) 104px 74px; align-items:center; gap:var(--space-3); padding:var(--space-2) var(--space-3); }
.pending-drop-head { min-height:34px; color:var(--text-muted); background:var(--surface-field); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.pending-drop-row { min-height:62px; border-top:1px solid var(--border-default); }
.pending-kind { width:max-content; padding:3px 8px; border:1px solid var(--accent-border); border-radius:999px; color:var(--accent-strong); background:var(--accent-soft); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.pending-main { display:flex; align-items:center; gap:var(--space-3); min-width:0; }
.pending-main > div { min-width:0; }
.pending-main b,.pending-main small { display:block; }
.pending-main b { overflow:hidden; color:var(--text-primary); text-overflow:ellipsis; white-space:nowrap; font-size:var(--fs-sm); }
.pending-main small { margin-top:3px; overflow-wrap:anywhere; color:var(--text-muted); font-size:var(--fs-xs); }
.pending-source { color:var(--text-secondary); font-size:var(--fs-xs); }
.pending-quantity { display:block; min-width:0; }
.pending-quantity .drop-number-input { min-height:34px; padding:0 var(--space-2); text-align:center; }
.pending-quantity-label { color:var(--text-secondary); font-size:var(--fs-xs); }
.pending-remove { width:100%; }
.pending-empty { display:flex; align-items:center; justify-content:center; gap:var(--space-3); min-height:92px; margin-top:var(--space-4); border:1px dashed var(--border-strong); border-radius:var(--radius-md); color:var(--text-muted); background:var(--surface-field); }
.pending-empty > span { display:grid; width:34px; height:34px; place-items:center; border:1px solid var(--accent-border); border-radius:50%; color:var(--accent); font-size:var(--fs-lg); }
.pending-empty b,.pending-empty small { display:block; }
.pending-empty b { color:var(--text-secondary); font-size:var(--fs-sm); }
.pending-empty small { margin-top:3px; font-size:var(--fs-xs); }
.deploy-dock { position:sticky; z-index:10; bottom:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); min-height:64px; padding:var(--space-3) var(--space-4); border:1px solid var(--border-strong); border-left:4px solid var(--border-strong); border-radius:var(--radius-md); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }
.deploy-dock.ready { border-left-color:var(--accent); }
.deploy-dock b,.deploy-dock span { display:block; }
.deploy-dock b { color:var(--text-primary); font-family:var(--font-display); }
.deploy-dock span { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.dock-actions { display:flex; gap:var(--space-2); flex:0 0 auto; }
.sr-only { position:absolute; width:1px; height:1px; padding:0; margin:-1px; overflow:hidden; clip:rect(0,0,0,0); white-space:nowrap; border:0; }

@container natural-drop (max-width:860px) {
  .drop-intro { grid-template-columns:minmax(0,1fr); align-items:start; }
  .task-reward-runtime { grid-template-columns:minmax(0,1fr); align-items:start; }
  .task-reward-controls { width:100%; min-width:0; justify-items:stretch; }
  .task-reward-actions,.task-reward-actions .ui-btn { width:100%; }
  .task-reward-state { justify-items:start; }
  .drop-safety { max-width:none; }
  .drop-builder-grid { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .item-builder-grid { grid-template-columns:minmax(0,1fr) minmax(160px,.45fr); }
  .item-builder-grid .builder-add { grid-column:1 / -1; }
  .builder-add { width:100%; }
  .pending-drop-head,.pending-drop-row { grid-template-columns:70px minmax(0,1fr) 90px 70px; }
  .pending-drop-head span:nth-child(3),.pending-source { display:none; }
}
@container natural-drop (max-width:620px) {
  .advanced-drop-panel > summary { grid-template-columns:36px minmax(0,1fr); }
  .advanced-drop-panel > summary > strong { grid-column:2; width:max-content; }
  .advanced-drop-content { padding:var(--space-4); }
  .section-heading { flex-direction:column; }
  .path-row { grid-template-columns:32px minmax(0,1fr); }
  .path-row .ui-btn { grid-column:1 / -1; width:100%; }
  .table-ledger { grid-template-columns:minmax(0,1fr); }
  .table-line { grid-template-columns:22px minmax(0,1fr) auto; }
  .table-line code { display:none; }
  .drop-builder > header { align-items:stretch; flex-direction:column; }
  .drop-builder-grid { grid-template-columns:minmax(0,1fr); }
  .builder-note { margin-left:0; }
  .pending-drop-head { display:none; }
  .pending-drop-row { grid-template-columns:minmax(0,1fr) 70px; }
  .pending-kind { grid-column:1 / -1; }
  .pending-main { grid-column:1 / -1; }
  .pending-quantity,.pending-quantity-label { grid-column:1; max-width:112px; }
  .pending-remove { grid-column:2; }
  .pending-main .drop-item-icon { width:36px; height:36px; flex-basis:36px; }
  .deploy-dock { position:static; align-items:stretch; flex-direction:column; }
  .dock-actions,.dock-actions .ui-btn { width:100%; }
  .dock-actions { flex-direction:column-reverse; }
}
</style>
