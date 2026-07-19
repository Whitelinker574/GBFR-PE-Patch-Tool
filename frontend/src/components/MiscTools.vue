<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { CharaAcquire, CharaRelease,
         CurrencyGetAllOwned, CurrencySetOneOwned,
         PotionGetAllOwned, PotionSetOneOwned,
         CountdownGetStatus, CountdownScan, CountdownSet,
         FaceAccessoryGetStatus, FaceAccessoryScan, FaceAccessorySetHidden,
         InfiniteChallengeGetStatus, InfiniteChallengeSetEnabled,
         MaterialConsumeGetStatusOwned, MaterialConsumeSetEnabledOwned,
         CollectibleTaskCompleteOwned,
         TerminusDropGetStatusOwned, TerminusDropScanOwned, TerminusDropSetEnabledOwned,
         UnlockAllTrophyGetStatus, UnlockAllTrophyScan, UnlockAllTrophySetEnabled,
         OtherSkinPurpleRuneGetStatus, OtherSkinPurpleRuneSetEnabled,
         MonsterEnhanceSetPatchValueEnabledOwned,
         DamageMeterGetStatus, DamageMeterReset,
         DamageOverlaySetEnabled, DamageOverlaySetValue, DamageOverlaySetFontSize } from '../../wailsjs/go/main/App'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'

const emit = defineEmits(['status'])
const RUNTIME_LEASE_SCOPE = 'misc-tools'
const props = defineProps({
  mode: { type: String, default: 'stable' },
})

const connected = ref(false)
const info = reactive({ pid: 0, moduleBase: 0, manager: 0 })
const loading = ref(false)

const countdownValue = ref('30')
const countdownStatus = reactive({ found: false, address: 0, rva: 0, value1: 0, value2: 0, currentBytes: '' })
const countdownLoading = ref(false)
const faceAccessoryStatus = reactive({ found: false, address: 0, rva: 0, hidden: false, jumpOpcode: '', currentBytes: '' })
const faceAccessoryLoading = ref(false)
const infiniteChallengeStatus = reactive({ rva: 0, enabled: false, currentBytes: '' })
const infiniteChallengeLoading = ref(false)
const materialConsumeStatus = reactive({ rva: 0, enabled: false, currentBytes: '' })
const materialConsumeLoading = ref(false)
const collectibleTaskLoading = ref(false)
const inventorySet45Enabled = ref(false)
const inventorySet45Loading = ref(false)
const inventorySet45Seconds = ref(0)
const inventorySetQuantity = ref(45)
const terminusDropStatus = reactive({ found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
const terminusDropLoading = ref(false)
const unlockAllTrophyStatus = reactive({ found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
const unlockAllTrophyLoading = ref(false)
const showUnlockAllTrophyConfirm = ref(false)
const otherSkinPurpleRuneStatus = reactive({ rva: 0, enabled: false, jumpOpcode: '', currentBytes: '' })
const otherSkinPurpleRuneLoading = ref(false)
const damageMeterStatus = reactive({ connected: false, totalDamage: 0, monsterDamage: 0, crocodileDamage: 0 })
const damageMeterLoading = ref(false)
const currencies = ref([])
const currencyInputs = reactive({})
const currencyLoading = ref(false)
const potions = ref([])
const potionInputs = reactive({})
const potionLoading = ref(false)
const damageOverlayEnabled = ref(false)
const damageOverlayFontSize = ref(Number(localStorage.getItem('gbfrDamageOverlayFontSize') || 48))
const showOutdatedFeatures = computed(() => props.mode === 'compatibility')
const showStableFeatures = computed(() => props.mode !== 'compatibility')
const activeRuntimeGroup = ref(props.mode === 'compatibility' ? 'battle' : 'resources')
const runtimeCatalog = computed(() => {
  const catalogs = {
    resources: [
      ['实时货币编辑', '金币、MSP、高级炼成点数与 CP', '已适配'],
      ['副本药水', '复活药水与群疗药水数量', '需进入副本'],
      ['素材不消耗', '强化、练成期间临时阻止素材变化', '已适配'],
      ['小钳蟹相关', '临时调整拾取数量与完成收集任务', '运行时钩子'],
    ],
    mission: [
      ['巴武掉落 100%', '移除巴武掉落的随机排除，保留原始资格检查', 'AOB 定位'],
    ],
    battle: [
      ['团队伤害记录', '共享内存统计与悬浮显示', '等待适配'],
      ['任务结算倒计时', '修改结算等待时间与零帧开箱', '等待适配'],
      ['无限挑战', '阻止挑战次数递增', '等待适配'],
      ['任务得分倍率', '0.8.1 新增：调整任务结算得分倍率', '等待 2.0.2 特征定位'],
      ['强制支线目标奖励', '0.8.1 新增：结算时强制取得支线目标奖励', '等待 2.0.2 特征定位'],
      ['任务内倍率', '0.8.1 新增的通用倍率入口，字段含义仍需逐项核对', '仅保留资料'],
    ],
    display: [
      ['脸部符文显示', '控制特定皮肤脸部符文', '等待适配'],
      ['游戏内全称号', '临时切换称号解锁判断', '等待适配'],
      ['其他皮肤紫色符文', '让紫色符文显示在其他皮肤', '等待适配'],
    ],
  }
  return catalogs[activeRuntimeGroup.value] || []
})
let damageMeterTimer = 0
let inventorySet45Timer = 0
let disposed = false
let lifecycleEpoch = 0
let connectionOwnerToken = ''

function getMonsterEnhanceMultiplier(id) {
  const saved = window.gbfrMonsterEnhanceMultipliers || {}
  const value = parseFloat(saved[id] || '1')
  return isNaN(value) || value <= 0 || value > 9999 ? 1 : value
}

function clearConnectionState() {
  connected.value = false
  stopDamageMeterTimer()
  stopInventorySet45Timer()
  inventorySet45Enabled.value = false
  Object.assign(info, { pid: 0, moduleBase: 0, manager: 0 })
  Object.assign(countdownStatus, { found: false, address: 0, rva: 0, value1: 0, value2: 0, currentBytes: '' })
  Object.assign(faceAccessoryStatus, { found: false, address: 0, rva: 0, hidden: false, jumpOpcode: '', currentBytes: '' })
  Object.assign(infiniteChallengeStatus, { rva: 0, enabled: false, currentBytes: '' })
  Object.assign(materialConsumeStatus, { rva: 0, enabled: false, currentBytes: '' })
  Object.assign(terminusDropStatus, { found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
  Object.assign(unlockAllTrophyStatus, { found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
  Object.assign(otherSkinPurpleRuneStatus, { rva: 0, enabled: false, jumpOpcode: '', currentBytes: '' })
  Object.assign(damageMeterStatus, { connected: false, totalDamage: 0, monsterDamage: 0, crocodileDamage: 0 })
  currencies.value = []
  Object.keys(currencyInputs).forEach((key) => delete currencyInputs[key])
  potions.value = []
  Object.keys(potionInputs).forEach((key) => delete potionInputs[key])
}

async function connect() {
  if (loading.value) return
  const epoch = ++lifecycleEpoch
  loading.value = true
  let acquiredOwnerToken = ''
  try {
    const res = await CharaAcquire(nextRuntimeAcquireRequestID())
    acquiredOwnerToken = String(res?.ownerToken || '')
    if (!acquiredOwnerToken) throw new Error('后端未返回运行时连接所有权令牌')
    if (disposed || epoch !== lifecycleEpoch) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, CharaRelease)
      return
    }
    connectionOwnerToken = acquiredOwnerToken
    connected.value = true
    Object.assign(info, res)
    if (showOutdatedFeatures.value) {
      loadCountdownStatus()
      loadFaceAccessoryStatus()
      loadInfiniteChallengeStatus()
      loadUnlockAllTrophyStatus()
      loadOtherSkinPurpleRuneStatus()
      startDamageMeterTimer()
    }
    if (showStableFeatures.value) {
      loadMaterialConsumeStatus()
      loadTerminusDropStatus()
      loadCurrencyValues()
      loadPotionValues()
    }
  } catch (err) {
    let cleanupError = null
    if (acquiredOwnerToken) {
      try {
        await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, CharaRelease)
        if (connectionOwnerToken === acquiredOwnerToken) connectionOwnerToken = ''
      } catch (nextError) { cleanupError = nextError }
    }
    if (!cleanupError && !connectionOwnerToken) clearConnectionState()
    if (!disposed && epoch === lifecycleEpoch) {
      emit('status', cleanupError ? `${String(err)}；释放运行时连接也失败：${String(cleanupError)}` : String(err), 'error')
    }
  } finally {
    if (!disposed && epoch === lifecycleEpoch) loading.value = false
  }
}

async function disconnect() {
  if (loading.value) return false
  const epoch = ++lifecycleEpoch
  const ownerToken = connectionOwnerToken
  loading.value = true
  try {
    if (ownerToken) await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease)
    if (disposed || epoch !== lifecycleEpoch) return false
    if (connectionOwnerToken === ownerToken) connectionOwnerToken = ''
    clearConnectionState()
    return true
  } catch (err) {
    if (!disposed && epoch === lifecycleEpoch) emit('status', String(err), 'error')
    return false
  } finally {
    if (!disposed && epoch === lifecycleEpoch) loading.value = false
  }
}

function formatHex(value) {
  if (!value) return '—'
  return '0x' + Number(value).toString(16).toUpperCase()
}

function formatFloat(value) {
  if (value === undefined || value === null) return '—'
  return Number(value).toFixed(2)
}

function isCountdownActive() {
  return countdownStatus.found && Math.abs(Number(countdownStatus.value1) - 30) > 0.001
}

function applyCountdownStatus(status) {
  Object.assign(countdownStatus, status || { found: false, address: 0, rva: 0, value1: 0, value2: 0, currentBytes: '' })
  if (status && status.found) countdownValue.value = String(Number(status.value1.toFixed(2)))
}

function loadCountdownStatus() {
  if (!connected.value) return
  countdownLoading.value = true
  CountdownGetStatus()
    .then(applyCountdownStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { countdownLoading.value = false })
}

function scanCountdown() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  countdownLoading.value = true
  CountdownScan()
    .then((status) => { applyCountdownStatus(status); emit('status', '倒计时特征定位成功', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { countdownLoading.value = false })
}

function setCountdown() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  const v = parseFloat(countdownValue.value)
  if (isNaN(v) || v < 0 || v > 9999) { emit('status', '请输入 0 到 9999 之间的数值', 'error'); return }
  countdownLoading.value = true
  CountdownSet(v)
    .then((status) => { applyCountdownStatus(status); emit('status', '倒计时写入成功', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { countdownLoading.value = false })
}

function applyFaceAccessoryStatus(status) {
  Object.assign(faceAccessoryStatus, status || { found: false, address: 0, rva: 0, hidden: false, jumpOpcode: '', currentBytes: '' })
}

function loadFaceAccessoryStatus() {
  if (!connected.value) return
  faceAccessoryLoading.value = true
  FaceAccessoryGetStatus()
    .then(applyFaceAccessoryStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { faceAccessoryLoading.value = false })
}

function scanFaceAccessory() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  faceAccessoryLoading.value = true
  FaceAccessoryScan()
    .then((status) => { applyFaceAccessoryStatus(status); emit('status', '脸部符文特征定位成功', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { faceAccessoryLoading.value = false })
}

function setFaceAccessoryHidden(hidden) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  faceAccessoryLoading.value = true
  FaceAccessorySetHidden(hidden)
    .then((status) => { applyFaceAccessoryStatus(status); emit('status', hidden ? '已隐藏脸部符文' : '已恢复脸部符文显示', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { faceAccessoryLoading.value = false })
}

function applyInfiniteChallengeStatus(status) {
  Object.assign(infiniteChallengeStatus, status || { rva: 0, enabled: false, currentBytes: '' })
}

function loadInfiniteChallengeStatus() {
  if (!connected.value) return
  infiniteChallengeLoading.value = true
  InfiniteChallengeGetStatus()
    .then(applyInfiniteChallengeStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { infiniteChallengeLoading.value = false })
}

function setInfiniteChallengeEnabled(enabled) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  infiniteChallengeLoading.value = true
  InfiniteChallengeSetEnabled(enabled)
    .then((status) => { applyInfiniteChallengeStatus(status); emit('status', enabled ? '已开启无限挑战' : '已恢复挑战次数递增', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { infiniteChallengeLoading.value = false })
}

function applyMaterialConsumeStatus(status) {
  Object.assign(materialConsumeStatus, status || { rva: 0, enabled: false, currentBytes: '' })
}

function loadMaterialConsumeStatus() {
  if (!connected.value) return
  materialConsumeLoading.value = true
  MaterialConsumeGetStatusOwned(connectionOwnerToken)
    .then(applyMaterialConsumeStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { materialConsumeLoading.value = false })
}

function setMaterialConsumeEnabled(enabled) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  materialConsumeLoading.value = true
  MaterialConsumeSetEnabledOwned(connectionOwnerToken, enabled)
    .then((status) => { applyMaterialConsumeStatus(status); emit('status', enabled ? '已开启升级/强化不材料消耗' : '已恢复升级/强化材料变化', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { materialConsumeLoading.value = false })
}

function completeCollectibleTask() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  collectibleTaskLoading.value = true
  CollectibleTaskCompleteOwned(connectionOwnerToken)
    .then((status) => emit('status', `收集任务已完成 ${status.completed}/${status.total}`, 'success'))
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { collectibleTaskLoading.value = false })
}

function stopInventorySet45Timer() {
  if (inventorySet45Timer) window.clearInterval(inventorySet45Timer)
  inventorySet45Timer = 0
  inventorySet45Seconds.value = 0
}

function startInventorySet45Timer() {
  stopInventorySet45Timer()
  inventorySet45Seconds.value = 10
  inventorySet45Timer = window.setInterval(() => {
    inventorySet45Seconds.value -= 1
    if (inventorySet45Seconds.value > 0) return
    stopInventorySet45Timer()
    setInventorySet45Enabled(false, 0, true)
  }, 1000)
}

function setInventorySet45Enabled(enabled, quantity = inventorySetQuantity.value, automatic = false) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  if (!enabled) stopInventorySet45Timer()
  inventorySet45Loading.value = true
  MonsterEnhanceSetPatchValueEnabledOwned(connectionOwnerToken, 'inventory_set_45', enabled, quantity)
    .then(() => {
      inventorySet45Enabled.value = enabled
      if (enabled) {
        inventorySetQuantity.value = quantity
        startInventorySet45Timer()
      }
      emit('status', enabled ? `已开启背包物品数量设为 ${quantity}，10 秒后自动恢复` : (automatic ? '背包物品数量已自动恢复正常' : '已恢复背包物品正常添加'), 'success')
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { inventorySet45Loading.value = false })
}

function applyTerminusDropStatus(status) {
  Object.assign(terminusDropStatus, status || { found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
}

function loadTerminusDropStatus() {
  if (!connected.value) return
  terminusDropLoading.value = true
  TerminusDropGetStatusOwned(connectionOwnerToken)
    .then(applyTerminusDropStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { terminusDropLoading.value = false })
}

function scanTerminusDrop() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  terminusDropLoading.value = true
  TerminusDropScanOwned(connectionOwnerToken)
    .then((status) => { applyTerminusDropStatus(status); emit('status', '巴武掉落特征定位成功', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { terminusDropLoading.value = false })
}

function setTerminusDropEnabled(enabled) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  terminusDropLoading.value = true
  TerminusDropSetEnabledOwned(connectionOwnerToken, enabled)
    .then((status) => { applyTerminusDropStatus(status); emit('status', enabled ? '已开启巴武掉落 100%' : '已恢复巴武默认掉率', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { terminusDropLoading.value = false })
}

function applyUnlockAllTrophyStatus(status) {
  Object.assign(unlockAllTrophyStatus, status || { found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
}

function loadUnlockAllTrophyStatus() {
  if (!connected.value) return
  unlockAllTrophyLoading.value = true
  UnlockAllTrophyGetStatus()
    .then(applyUnlockAllTrophyStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { unlockAllTrophyLoading.value = false })
}

function scanUnlockAllTrophy() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  unlockAllTrophyLoading.value = true
  UnlockAllTrophyScan()
    .then((status) => { applyUnlockAllTrophyStatus(status); emit('status', '全称号解锁特征定位成功', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { unlockAllTrophyLoading.value = false })
}

function setUnlockAllTrophyEnabled(enabled) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  if (enabled) { showUnlockAllTrophyConfirm.value = true; return }
  applyUnlockAllTrophyEnabled(false)
}

function confirmUnlockAllTrophy() {
  showUnlockAllTrophyConfirm.value = false
  applyUnlockAllTrophyEnabled(true)
}

function applyUnlockAllTrophyEnabled(enabled) {
  unlockAllTrophyLoading.value = true
  UnlockAllTrophySetEnabled(enabled)
    .then((status) => { applyUnlockAllTrophyStatus(status); emit('status', enabled ? '已开启游戏内全称号解锁' : '已恢复称号默认判断', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { unlockAllTrophyLoading.value = false })
}

function applyOtherSkinPurpleRuneStatus(status) {
  Object.assign(otherSkinPurpleRuneStatus, status || { rva: 0, enabled: false, jumpOpcode: '', currentBytes: '' })
}

function loadOtherSkinPurpleRuneStatus() {
  if (!connected.value) return
  otherSkinPurpleRuneLoading.value = true
  OtherSkinPurpleRuneGetStatus()
    .then(applyOtherSkinPurpleRuneStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { otherSkinPurpleRuneLoading.value = false })
}

function setOtherSkinPurpleRuneEnabled(enabled) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  otherSkinPurpleRuneLoading.value = true
  OtherSkinPurpleRuneSetEnabled(enabled)
    .then((status) => { applyOtherSkinPurpleRuneStatus(status); emit('status', enabled ? '已开启其他皮肤紫色符文显示' : '已恢复其他皮肤紫色符文判断', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { otherSkinPurpleRuneLoading.value = false })
}

function formatDamage(value) {
  return Number(value || 0).toLocaleString()
}

function formatInt(value) {
  return Number(value || 0).toLocaleString()
}

function applyCurrencyValues(items) {
  currencies.value = Array.isArray(items) ? items : []
  currencies.value.forEach((item) => {
    currencyInputs[item.id] = String(item.value)
  })
}

function loadCurrencyValues() {
  if (!connected.value) return
  currencyLoading.value = true
  CurrencyGetAllOwned(connectionOwnerToken)
    .then(applyCurrencyValues)
    .catch((err) => {
      applyCurrencyValues([])
      emit('status', String(err), 'error')
    })
    .finally(() => { currencyLoading.value = false })
}

function setCurrency(item) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  const value = Number(currencyInputs[item.id])
  if (!Number.isInteger(value) || value < 0 || value > 2147483647) { emit('status', '请输入 0 到 2147483647 之间的整数', 'error'); return }
  currencyLoading.value = true
  CurrencySetOneOwned(connectionOwnerToken, item.id, value)
    .then((updated) => {
      const index = currencies.value.findIndex((entry) => entry.id === updated.id)
      if (index >= 0) currencies.value.splice(index, 1, updated)
      currencyInputs[updated.id] = String(updated.value)
      emit('status', `${updated.name}写入成功`, 'success')
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { currencyLoading.value = false })
}

function formatOffsets(offsets) {
  return (offsets || []).map(formatHex).join(' + ')
}

function applyPotionValues(items) {
  potions.value = Array.isArray(items) ? items : []
  potions.value.forEach((item) => {
    potionInputs[item.id] = String(item.value)
  })
}

function loadPotionValues() {
  if (!connected.value) return
  potionLoading.value = true
  PotionGetAllOwned(connectionOwnerToken)
    .then(applyPotionValues)
    .catch((err) => {
      applyPotionValues([])
      emit('status', String(err), 'error')
    })
    .finally(() => { potionLoading.value = false })
}

function setPotion(item) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  const value = Number(potionInputs[item.id])
  if (!Number.isInteger(value) || value < 0 || value > 2147483647) { emit('status', '请输入 0 到 2147483647 之间的整数', 'error'); return }
  potionLoading.value = true
  PotionSetOneOwned(connectionOwnerToken, item.id, value)
    .then((updated) => {
      const index = potions.value.findIndex((entry) => entry.id === updated.id)
      if (index >= 0) potions.value.splice(index, 1, updated)
      potionInputs[updated.id] = String(updated.value)
      emit('status', `${updated.name}写入成功`, 'success')
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { potionLoading.value = false })
}

function applyDamageMeterStatus(status) {
  Object.assign(damageMeterStatus, {
    connected: !!(status && status.connected),
    totalDamage: Number((status && status.totalDamage) || 0),
    monsterDamage: Number((status && status.monsterDamage) || 0),
    crocodileDamage: Number((status && status.crocodileDamage) || 0),
  })
  if (damageOverlayEnabled.value) DamageOverlaySetValue(displayDamage()).catch(() => {})
}

function displayDamage() {
  return Math.round(damageMeterStatus.monsterDamage * getMonsterEnhanceMultiplier('monster_hp') + damageMeterStatus.crocodileDamage * getMonsterEnhanceMultiplier('crocodile_damage'))
}

function startDamageMeterTimer() {
  stopDamageMeterTimer()
  loadDamageMeterStatus()
  damageMeterTimer = window.setInterval(() => loadDamageMeterStatus(true), 500)
}

function stopDamageMeterTimer() {
  if (!damageMeterTimer) return
  window.clearInterval(damageMeterTimer)
  damageMeterTimer = 0
}

function loadDamageMeterStatus(silent = false) {
  if (!connected.value) return
  if (!silent) damageMeterLoading.value = true
  DamageMeterGetStatus()
    .then(applyDamageMeterStatus)
    .catch((err) => { if (!silent) emit('status', String(err), 'error') })
    .finally(() => { if (!silent) damageMeterLoading.value = false })
}

function enableDamageMeter() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  damageMeterLoading.value = true
  MonsterEnhanceSetPatchValueEnabledOwned(connectionOwnerToken, 'monster_hp', true, getMonsterEnhanceMultiplier('monster_hp'))
    .then(() => MonsterEnhanceSetPatchValueEnabledOwned(connectionOwnerToken, 'crocodile_damage', true, getMonsterEnhanceMultiplier('crocodile_damage')))
    .then(() => DamageMeterGetStatus())
    .then((status) => {
      applyDamageMeterStatus(status)
      emit('status', '伤害记录已开启，已自动开启怪物多倍血和鳄鱼多倍血', 'success')
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { damageMeterLoading.value = false })
}

function resetDamageMeter() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  damageMeterLoading.value = true
  DamageMeterReset()
    .then((status) => { applyDamageMeterStatus(status); emit('status', '团队伤害已清零', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { damageMeterLoading.value = false })
}

function clampOverlayFontSize(value) {
  return Math.min(120, Math.max(18, Number(value) || 48))
}

function setDamageOverlayFontSize(value) {
  damageOverlayFontSize.value = clampOverlayFontSize(value)
  localStorage.setItem('gbfrDamageOverlayFontSize', String(damageOverlayFontSize.value))
  DamageOverlaySetFontSize(damageOverlayFontSize.value).catch(() => {})
}

function enableDamageOverlay() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  DamageOverlaySetFontSize(damageOverlayFontSize.value)
    .then(() => DamageOverlaySetValue(displayDamage()))
    .then(() => DamageOverlaySetEnabled(true))
    .then(() => {
      damageOverlayEnabled.value = true
      startDamageMeterTimer()
      emit('status', '伤害悬浮窗已开启', 'success')
    })
    .catch((err) => emit('status', String(err), 'error'))
}

function disableDamageOverlay() {
  DamageOverlaySetEnabled(false).catch(() => {})
  damageOverlayEnabled.value = false
  emit('status', '伤害悬浮窗已关闭', 'success')
}

function toggleDamageOverlay() {
  if (damageOverlayEnabled.value) disableDamageOverlay()
  else enableDamageOverlay()
}

onBeforeUnmount(() => {
  disposed = true
  lifecycleEpoch += 1
  const ownerToken = connectionOwnerToken
  connectionOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease)
  stopDamageMeterTimer()
  stopInventorySet45Timer()
  if (damageOverlayEnabled.value) disableDamageOverlay()
})

</script>

<template>
  <div class="root ui-page is-wide ui-page-stack">
    <div class="section ui-card ui-panel">
      <div class="header">
        <span class="title">{{ showOutdatedFeatures ? '待适配运行时功能' : '游戏内工具' }}</span>
        <span class="info-dot" title="这些功能会修改游戏运行时内存，不写入存档；重启游戏或切换版本后需要重新连接并设置。">!</span>
        <span class="hint">{{ showOutdatedFeatures ? '兼容性实验室 · 默认仅建议扫描与诊断' : '需游戏运行中使用 · 重启后重新连接' }}</span>
      </div>
      <div class="connect-row ui-toolbar">
        <button v-if="!connected" class="btn-connect ui-btn is-primary" @click="connect" :disabled="loading">
          {{ loading ? '连接中...' : '连接游戏进程' }}
        </button>
        <button v-else class="btn-disconnect ui-btn is-danger" @click="disconnect">断开连接</button>
        <span v-if="connected" class="pid ui-tag is-ok">PID: {{ info.pid }}</span>
      </div>

      <div class="runtime-tabs ui-seg">
        <template v-if="showStableFeatures">
          <button class="ui-seg-btn" :class="{ 'is-on': activeRuntimeGroup === 'resources' }" @click="activeRuntimeGroup = 'resources'">资源与药水</button>
          <button class="ui-seg-btn" :class="{ 'is-on': activeRuntimeGroup === 'mission' }" @click="activeRuntimeGroup = 'mission'">任务与掉落</button>
        </template>
        <template v-else>
          <button class="ui-seg-btn" :class="{ 'is-on': activeRuntimeGroup === 'battle' }" @click="activeRuntimeGroup = 'battle'">战斗与任务</button>
          <button class="ui-seg-btn" :class="{ 'is-on': activeRuntimeGroup === 'display' }" @click="activeRuntimeGroup = 'display'">显示与解锁</button>
        </template>
      </div>

      <div v-if="showStableFeatures" class="memory-card compatibility-note ui-card ui-panel is-compact">
        <div class="memory-header">
          <span class="memory-title">实时修改与离线编辑</span>
          <span class="memory-hint">DLC 2.0.2 分工</span>
        </div>
        <div class="memory-info">
          <span>金币、MSP、高级炼成点数和 CP 使用 2.0.2 特征动态定位，实时写入后立即回读。</span>
          <span>药水和“不消耗素材”在游戏运行时使用；添加具体物品、素材和武器仍放在“养成编辑（离线）”。</span>
          <span>实时数值需要让游戏正常触发一次保存；游戏运行时不要同时离线修改同一存档。</span>
        </div>
      </div>

      <template v-if="connected">
        <div v-if="showStableFeatures && activeRuntimeGroup === 'resources'" class="memory-card ui-card ui-panel is-compact" :class="{ active: currencies.length }">
          <div class="memory-header">
            <span class="memory-title">实时货币编辑</span>
            <span class="memory-hint">AOB 捕获玩家结构 · 写入后回读</span>
          </div>
          <p class="feature-help">用途：实时修改金币、MSP、高级炼成点数和 CP。首次连接后若没有读数，请在游戏内打开主菜单或让资源发生一次变化。</p>
          <div class="currency-grid">
            <div v-for="item in currencies" :key="item.id" class="currency-row">
              <div class="currency-name">{{ item.name }}</div>
              <div class="currency-meta">{{ formatInt(item.value) }} · {{ formatHex(item.rva) }} + {{ formatHex(item.offset) }}</div>
              <input v-model="currencyInputs[item.id]" type="number" min="0" max="2147483647" step="1" class="batch-input currency-input ui-input" />
              <button class="btn-max ui-btn is-sm" @click="currencyInputs[item.id]='2147483647'">最大</button>
              <button class="btn-batch ui-btn is-primary is-sm" @click="setCurrency(item)" :disabled="currencyLoading">写入</button>
            </div>
          </div>
          <div class="memory-row">
            <button class="btn-refresh ui-btn" @click="loadCurrencyValues" :disabled="currencyLoading">{{ currencyLoading ? '定位中…' : '重新定位 / 刷新' }}</button>
          </div>
          <div v-if="!currencies.length" class="memory-info"><span>首次连接会安装临时读取点；若尚无数据，请在游戏内打开主菜单或让金币/MSP刷新一次后再点刷新。</span></div>
        </div>

        <div v-if="showStableFeatures && activeRuntimeGroup === 'resources'" class="memory-card ui-card ui-panel is-compact" :class="{ active: potions.length }">
          <div class="memory-header">
            <span class="memory-title">副本药水</span>
            <span class="memory-hint">稳定指针链读取/写入 int32</span>
          </div>
          <p class="feature-help">用途：在副本内调整复活药和群疗药数量。必须先进入副本，刷新看到正常数量后再写入。</p>
          <div class="currency-grid">
            <div v-for="item in potions" :key="item.id" class="currency-row">
              <div class="currency-name">{{ item.name }}</div>
              <div class="currency-meta">{{ formatInt(item.value) }} · {{ formatHex(item.rva) }} + {{ formatOffsets(item.offsets) }}</div>
              <input v-model="potionInputs[item.id]" type="number" min="0" max="2147483647" step="1" class="batch-input currency-input ui-input" />
              <button class="btn-max ui-btn is-sm" @click="potionInputs[item.id]='2147483647'">最大</button>
              <button class="btn-batch ui-btn is-primary is-sm" @click="setPotion(item)" :disabled="potionLoading">写入</button>
            </div>
          </div>
          <div class="memory-row">
            <button class="btn-refresh ui-btn" @click="loadPotionValues" :disabled="potionLoading">刷新药水</button>
          </div>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'battle'" class="memory-card ui-card ui-panel is-compact" :class="{ active: damageMeterStatus.connected && damageMeterStatus.totalDamage > 0 }">
          <div class="memory-header">
            <span class="memory-title">团队伤害记录</span>
            <span class="memory-hint">依赖怪物增强中倍率血量，本功能自动开启默认1倍</span>
          </div>
          <p class="feature-help waiting">待适配：依赖怪物增强共享内存与倍率换算。当前仅建议确认共享内存是否建立。</p>
          <div class="memory-info damage-meter-info">
            <span>状态: {{ damageMeterStatus.connected ? '记录中' : '等待共享内存' }}</span>
            <span>原始扣血点会按怪物增强倍率折算显示</span>
          </div>
          <div class="damage-meter-value">{{ formatDamage(displayDamage()) }}</div>
          <div class="damage-meter-raw">原始: {{ formatDamage(damageMeterStatus.totalDamage) }}</div>
          <div class="memory-row">
            <button class="btn-batch ui-btn is-primary is-sm" @click="enableDamageMeter" :disabled="damageMeterLoading">开启记录</button>
            <button class="btn-refresh ui-btn is-sm" @click="toggleDamageOverlay" :disabled="damageMeterLoading || !damageMeterStatus.connected">{{ damageOverlayEnabled ? '关闭悬浮窗' : '开启悬浮窗' }}</button>
            <button class="btn-refresh ui-btn is-sm" @click="loadDamageMeterStatus" :disabled="damageMeterLoading">刷新</button>
            <button class="btn-refresh ui-btn is-sm" @click="resetDamageMeter" :disabled="damageMeterLoading">清零</button>
            <button class="btn-sort ui-btn is-ghost is-sm" @click="setDamageOverlayFontSize(damageOverlayFontSize - 4)" :disabled="!damageOverlayEnabled">字号 -</button>
            <button class="btn-sort ui-btn is-ghost is-sm" @click="setDamageOverlayFontSize(damageOverlayFontSize + 4)" :disabled="!damageOverlayEnabled">字号 +</button>
          </div>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'battle'" class="memory-card ui-card ui-panel is-compact" :class="{ active: isCountdownActive() }">
          <div class="memory-header">
            <span class="memory-title">任务结算倒计时/零帧开箱</span>
            <span class="info-dot" title="任务结算倒计时超过30s会导致进度条消失，但计时正常；零帧开箱需设置为0s。">!</span>
            <span class="memory-hint">AOB 定位后动态写入两个 float 值</span>
          </div>
          <p class="feature-help waiting">待适配：控制任务结算等待时间；设为 0 用于零帧开箱。扫描字节不一致时不要继续。</p>
          <div class="memory-info">
            <span>RVA: {{ formatHex(countdownStatus.rva) }}</span>
            <span>状态: {{ isCountdownActive() ? '开启' : '默认' }}</span>
            <span>当前: {{ formatFloat(countdownStatus.value1) }} / {{ formatFloat(countdownStatus.value2) }}</span>
          </div>
          <div class="memory-row">
            <input v-model="countdownValue" type="number" min="0" max="9999" step="0.1" class="batch-input countdown-input ui-input" placeholder="秒数" />
            <button class="btn-batch ui-btn is-primary is-sm" @click="setCountdown" :disabled="countdownLoading">设置倒计时</button>
            <button class="btn-refresh ui-btn is-sm" @click="loadCountdownStatus" :disabled="countdownLoading">刷新</button>
            <button class="btn-sort ui-btn is-ghost is-sm" @click="scanCountdown" :disabled="countdownLoading">重新扫描</button>
          </div>
          <details class="memory-diagnostics ui-disclosure"><summary>技术详情</summary><code class="memory-bytes">{{ countdownStatus.currentBytes || '未定位' }}</code></details>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'display'" class="memory-card ui-card ui-panel is-compact" :class="{ active: faceAccessoryStatus.hidden }">
          <div class="memory-header">
            <span class="memory-title">脸部符文显示(紫色皮肤包)</span>
            <span class="memory-hint">切换 JE/JNE 控制渲染判断</span>
          </div>
          <p class="feature-help waiting">待适配：切换脸部符文的渲染判断，仅用于特定紫色皮肤组合。</p>
          <div class="memory-info">
            <span>RVA: {{ formatHex(faceAccessoryStatus.rva) }}</span>
            <span>状态: {{ faceAccessoryStatus.hidden ? '隐藏' : '显示' }}</span>
            <span>跳转: {{ faceAccessoryStatus.jumpOpcode || '—' }}</span>
          </div>
          <div class="memory-row">
            <button class="btn-batch ui-btn is-primary is-sm" @click="setFaceAccessoryHidden(true)" :disabled="faceAccessoryLoading || faceAccessoryStatus.hidden">隐藏脸部符文</button>
            <button class="btn-refresh ui-btn is-sm" @click="setFaceAccessoryHidden(false)" :disabled="faceAccessoryLoading || !faceAccessoryStatus.hidden">恢复符文显示</button>
            <button class="btn-refresh ui-btn is-sm" @click="loadFaceAccessoryStatus" :disabled="faceAccessoryLoading">刷新</button>
            <button class="btn-sort ui-btn is-ghost is-sm" @click="scanFaceAccessory" :disabled="faceAccessoryLoading">重新扫描</button>
          </div>
          <details class="memory-diagnostics ui-disclosure"><summary>技术详情</summary><code class="memory-bytes">{{ faceAccessoryStatus.currentBytes || '未定位' }}</code></details>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'battle'" class="memory-card ui-card ui-panel is-compact" :class="{ active: infiniteChallengeStatus.enabled }">
          <div class="memory-header">
            <span class="memory-title">无限挑战</span>
            <span class="memory-hint">NOP 挑战次数递增</span>
          </div>
          <p class="feature-help waiting">待适配：阻止挑战次数递增。确认当前字节与预期一致后才可测试。</p>
          <div class="memory-info">
            <span>RVA: {{ formatHex(infiniteChallengeStatus.rva) }}</span>
            <span>状态: {{ infiniteChallengeStatus.enabled ? '开启' : '默认' }}</span>
          </div>
          <div class="memory-row">
            <button class="btn-batch ui-btn is-primary is-sm" @click="setInfiniteChallengeEnabled(true)" :disabled="infiniteChallengeLoading || infiniteChallengeStatus.enabled">开启无限挑战</button>
            <button class="btn-refresh ui-btn is-sm" @click="setInfiniteChallengeEnabled(false)" :disabled="infiniteChallengeLoading || !infiniteChallengeStatus.enabled">恢复默认</button>
            <button class="btn-refresh ui-btn is-sm" @click="loadInfiniteChallengeStatus" :disabled="infiniteChallengeLoading">刷新</button>
          </div>
          <details class="memory-diagnostics ui-disclosure"><summary>技术详情</summary><code class="memory-bytes">{{ infiniteChallengeStatus.currentBytes || '未读取' }}</code></details>
        </div>

        <div v-if="showStableFeatures && activeRuntimeGroup === 'resources'" class="memory-card ui-card ui-panel is-compact" :class="{ active: materialConsumeStatus.enabled }">
          <div class="memory-header">
            <span class="memory-title">素材不消耗</span>
            <span class="info-dot" title="开启后材料数量不会减少；同一指令也会阻止材料增加。">!</span>
            <span class="memory-hint">校验 RVA，失效时 AOB 重定位</span>
          </div>
          <p class="feature-help">用途：强化或练成时让素材数量不减少；同一指令也会阻止素材增加，因此用完立刻恢复，开启时不要进入副本。</p>
          <p v-if="inventorySet45Enabled" class="feature-help waiting">小钳蟹数量钩子正在占用同一指令地址；先恢复小钳蟹功能，才能切换素材不消耗。</p>
          <div class="memory-info">
            <span>RVA: {{ formatHex(materialConsumeStatus.rva) }}</span>
            <span>状态: {{ materialConsumeStatus.enabled ? '开启' : '默认' }}</span>
          </div>
          <div class="memory-row">
            <button class="btn-batch ui-btn is-primary is-sm" @click="setMaterialConsumeEnabled(true)" :disabled="materialConsumeLoading || materialConsumeStatus.enabled || inventorySet45Enabled">开启不消耗</button>
            <button class="btn-refresh ui-btn is-sm" @click="setMaterialConsumeEnabled(false)" :disabled="materialConsumeLoading || !materialConsumeStatus.enabled || inventorySet45Enabled">恢复默认</button>
            <button class="btn-refresh ui-btn is-sm" @click="loadMaterialConsumeStatus" :disabled="materialConsumeLoading">刷新</button>
          </div>
          <details class="memory-diagnostics ui-disclosure"><summary>技术详情</summary><code class="memory-bytes">{{ materialConsumeStatus.currentBytes || '未读取' }}</code></details>
        </div>

        <div v-if="showStableFeatures && activeRuntimeGroup === 'resources'" class="memory-card ui-card ui-panel is-compact" :class="{ active: inventorySet45Enabled }">
          <div class="memory-header">
            <span class="memory-title">小钳蟹相关</span>
            <span class="info-dot" title="使用后需要拾取一次对应种类螃蟹，不要提前开，拾取之前开，记得用完关闭">!</span>
            <span class="memory-hint">{{ inventorySet45Enabled ? `${inventorySet45Seconds} 秒后自动恢复` : '使用后需要拾取一次对应种类螃蟹，不要提前开，拾取之前开，记得用完关闭' }}</span>
          </div>
          <p v-if="materialConsumeStatus.enabled" class="feature-help waiting">素材不消耗正在占用同一指令地址；先恢复素材变化，才能启用小钳蟹数量钩子。</p>
          <div class="memory-row">
            <button class="btn-batch ui-btn is-primary is-sm" @click="setInventorySet45Enabled(true, 45)" :disabled="inventorySet45Loading || inventorySet45Enabled || materialConsumeStatus.enabled">小钳蟹</button>
            <button class="btn-batch ui-btn is-primary is-sm" @click="setInventorySet45Enabled(true, 20)" :disabled="inventorySet45Loading || inventorySet45Enabled || materialConsumeStatus.enabled">漆黑小钳蟹</button>
            <button class="btn-refresh ui-btn is-sm" @click="setInventorySet45Enabled(false)" :disabled="inventorySet45Loading || !inventorySet45Enabled || materialConsumeStatus.enabled">恢复正常</button>
            <button class="btn-batch ui-btn is-primary is-sm" @click="completeCollectibleTask" :disabled="collectibleTaskLoading">{{ collectibleTaskLoading ? '收集任务处理中...' : '小钳蟹成就' }}</button>
          </div>
        </div>

        <div v-if="showStableFeatures && activeRuntimeGroup === 'mission'" class="memory-card ui-card ui-panel is-compact" :class="{ active: terminusDropStatus.enabled }">
          <div class="memory-header">
            <span class="memory-title">巴武掉落 100%</span>
            <span class="info-dot" title="仅让原型巴哈姆特任务的巴武 lot 不再被 80% 排除；仍保留未拥有、角色已解锁等游戏原始检查。">!</span>
            <span class="memory-hint">AOB 定位后 NOP 巴武 lot 排除跳转</span>
          </div>
          <p class="feature-help">用途：移除原型巴哈姆特任务中巴武掉落的随机排除；角色解锁、未拥有等原始条件仍然保留。</p>
          <div class="memory-info">
            <span>RVA: {{ formatHex(terminusDropStatus.rva) }}</span>
            <span>状态: {{ terminusDropStatus.enabled ? '开启' : '默认' }}</span>
          </div>
          <div class="memory-row">
            <button class="btn-batch ui-btn is-primary is-sm" @click="setTerminusDropEnabled(true)" :disabled="terminusDropLoading || terminusDropStatus.enabled">开启巴武 100%</button>
            <button class="btn-refresh ui-btn is-sm" @click="setTerminusDropEnabled(false)" :disabled="terminusDropLoading || !terminusDropStatus.enabled">恢复默认</button>
            <button class="btn-refresh ui-btn is-sm" @click="loadTerminusDropStatus" :disabled="terminusDropLoading">刷新</button>
            <button class="btn-sort ui-btn is-ghost is-sm" @click="scanTerminusDrop" :disabled="terminusDropLoading">重新扫描</button>
          </div>
          <details class="memory-diagnostics ui-disclosure"><summary>技术详情</summary><code class="memory-bytes">{{ terminusDropStatus.currentBytes || '未定位' }}</code></details>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'display'" class="memory-card ui-card ui-panel is-compact" :class="{ active: unlockAllTrophyStatus.enabled }">
          <div class="memory-header">
            <span class="memory-title">游戏内全称号解锁</span>
            <span class="memory-hint">AOB 定位后切换 SETNE/SETNO</span>
          </div>
          <p class="feature-help waiting">待适配：临时改变游戏内称号解锁判断，持久化时机尚未完整确认。</p>
          <div class="memory-info">
            <span>RVA: {{ formatHex(unlockAllTrophyStatus.rva) }}</span>
            <span>状态: {{ unlockAllTrophyStatus.enabled ? '开启' : '默认' }}</span>
          </div>
          <div class="memory-row">
            <button class="btn-batch ui-btn is-primary is-sm" @click="setUnlockAllTrophyEnabled(true)" :disabled="unlockAllTrophyLoading || unlockAllTrophyStatus.enabled">开启全称号</button>
            <button class="btn-refresh ui-btn is-sm" @click="setUnlockAllTrophyEnabled(false)" :disabled="unlockAllTrophyLoading || !unlockAllTrophyStatus.enabled">恢复默认</button>
            <button class="btn-refresh ui-btn is-sm" @click="loadUnlockAllTrophyStatus" :disabled="unlockAllTrophyLoading">刷新</button>
            <button class="btn-sort ui-btn is-ghost is-sm" @click="scanUnlockAllTrophy" :disabled="unlockAllTrophyLoading">重新扫描</button>
          </div>
          <details class="memory-diagnostics ui-disclosure"><summary>技术详情</summary><code class="memory-bytes">{{ unlockAllTrophyStatus.currentBytes || '未定位' }}</code></details>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'display'" class="memory-card ui-card ui-panel is-compact" :class="{ active: otherSkinPurpleRuneStatus.enabled }">
          <div class="memory-header">
            <span class="memory-title">在其他皮肤显示紫色符文</span>
            <span class="memory-hint">固定 RVA 切换 JNE/JE</span>
          </div>
          <p class="feature-help waiting">待适配：让紫色符文在其他皮肤上显示，依赖旧版固定指令位置。</p>
          <div class="memory-info">
            <span>RVA: {{ formatHex(otherSkinPurpleRuneStatus.rva) }}</span>
            <span>状态: {{ otherSkinPurpleRuneStatus.enabled ? '开启' : '默认' }}</span>
            <span>跳转: {{ otherSkinPurpleRuneStatus.jumpOpcode || '—' }}</span>
          </div>
          <div class="memory-row">
            <button class="btn-batch ui-btn is-primary is-sm" @click="setOtherSkinPurpleRuneEnabled(true)" :disabled="otherSkinPurpleRuneLoading || otherSkinPurpleRuneStatus.enabled">开启显示</button>
            <button class="btn-refresh ui-btn is-sm" @click="setOtherSkinPurpleRuneEnabled(false)" :disabled="otherSkinPurpleRuneLoading || !otherSkinPurpleRuneStatus.enabled">恢复默认</button>
            <button class="btn-refresh ui-btn is-sm" @click="loadOtherSkinPurpleRuneStatus" :disabled="otherSkinPurpleRuneLoading">刷新</button>
          </div>
          <details class="memory-diagnostics ui-disclosure"><summary>技术详情</summary><code class="memory-bytes">{{ otherSkinPurpleRuneStatus.currentBytes || '未读取' }}</code></details>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'battle'" class="reference-grid ui-card-grid">
          <article v-for="item in runtimeCatalog.slice(3)" :key="item[0]" class="memory-card reference-card ui-card ui-panel is-compact">
            <div class="memory-header"><span class="memory-title">{{ item[0] }}</span><span class="ui-tag is-warn">{{ item[2] }}</span></div>
            <p class="feature-help waiting">{{ item[1] }}</p>
            <small class="ui-hint">资料入口保留；当前版本没有可安全操作的控件。</small>
          </article>
        </div>
        </div>

      </template>
      <div v-else class="preflight-grid ui-card-grid">
        <article v-for="item in runtimeCatalog" :key="item[0]" class="ui-card ui-panel is-compact">
          <div><strong>{{ item[0] }}</strong><p>{{ item[1] }}</p></div>
          <span :class="{ waiting: item[2] === '等待适配' }">{{ item[2] }}</span>
        </article>
        <div class="empty ui-empty">连接游戏进程后显示读取值和操作按钮</div>
      </div>
    </div>
    <div v-if="showUnlockAllTrophyConfirm" class="confirm-overlay" @click.self="showUnlockAllTrophyConfirm = false">
      <div class="confirm-dialog">
        <div class="confirm-title">确认开启游戏内全称号解锁</div>
        <div class="confirm-body">目前存档时机尚不明确，可以领取任务奖励、佩戴选定称号、选择佩戴界面有多个“未设置”是正常现象</div>
        <div class="confirm-actions">
          <button class="btn-refresh ui-btn is-sm" @click="showUnlockAllTrophyConfirm = false">取消</button>
          <button class="btn-warn ui-btn is-danger is-sm" @click="confirmUnlockAllTrophy" :disabled="unlockAllTrophyLoading">确认开启</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.root {
  display:flex;
  width:100%;
  max-width:var(--page-measure-wide);
  margin:0 auto;
  padding-bottom:var(--space-9);
  flex-direction:column;
  gap:var(--space-4);
  container:runtime-page / inline-size;
}

.section {
  display:flex;
  padding:var(--space-6);
  flex-direction:column;
  gap:var(--space-4);
  border-color:var(--border-default);
  border-radius:var(--radius-lg);
  background:var(--surface-card);
  box-shadow:var(--shadow-1);
}

.header {
  display:flex;
  align-items:center;
  justify-content:flex-start;
  gap:var(--space-2);
  flex-wrap:wrap;
}

.title {
  color:var(--text-primary);
  font-family:var(--font-display);
  font-size:var(--fs-lg);
  font-weight:var(--fw-bold);
}

.info-dot {
  display:inline-flex;
  width:18px;
  height:18px;
  flex:0 0 auto;
  align-items:center;
  justify-content:center;
  border:1px solid var(--border-strong);
  border-radius:var(--radius-pill);
  background:var(--accent-soft);
  color:var(--accent-hover);
  font-size:var(--fs-xs);
  font-weight:var(--fw-bold);
  cursor:help;
}

.hint {
  margin-left:auto;
  color:var(--text-muted);
  font-size:var(--fs-sm);
}

.connect-row {
  display:flex;
  align-items:center;
  gap:var(--space-2);
  padding:var(--space-3);
}

.runtime-tabs {
  width:fit-content;
  max-width:100%;
}

.preflight-grid {
  --ui-grid-min:260px;
}

.preflight-grid article {
  display:flex;
  min-height:76px;
  align-items:center;
  justify-content:space-between;
  gap:var(--space-4);
  border-color:var(--border-default);
  border-radius:var(--radius-md);
  background:var(--surface-card-pop);
}

.preflight-grid strong {
  display:block;
  color:var(--text-primary);
  font-size:var(--fs-md);
}

.preflight-grid p {
  margin:var(--space-1) 0 0;
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}

.preflight-grid article > span {
  flex:0 0 auto;
  padding:var(--space-1) var(--space-2);
  border:1px solid var(--border-default);
  border-radius:var(--radius-sm);
  background:var(--surface-row);
  color:var(--text-secondary);
  font-size:var(--fs-xs);
}

.preflight-grid article > span.waiting {
  border-color:var(--warning);
  background:var(--warning-bg);
  color:var(--warning-ink);
}

.preflight-grid .empty {
  grid-column:1 / -1;
}

.pid {
  font-family:var(--font-data);
  font-variant-numeric:tabular-nums;
}

.memory-card {
  display:flex;
  overflow:visible;
  padding:var(--space-5);
  flex-direction:column;
  gap:var(--space-3);
  border-color:var(--border-default);
  border-radius:var(--radius-md);
  background:var(--surface-card-pop);
  box-shadow:var(--shadow-1);
  transition:border-color var(--dur-base) var(--ease-out), background-color var(--dur-base) var(--ease-out), box-shadow var(--dur-base) var(--ease-out);
}

.memory-card.active {
  border-color:var(--success);
  background:color-mix(in srgb,var(--success-bg) 44%,var(--surface-card-pop));
  box-shadow:3px 0 0 var(--success) inset,var(--shadow-1);
}

.compatibility-note {
  border-left:3px solid var(--info);
  background:var(--info-bg);
}

.memory-header,
.memory-info,
.memory-row {
  display:flex;
  align-items:center;
  gap:var(--space-2);
  flex-wrap:wrap;
}

.memory-header {
  align-items:flex-start;
  justify-content:flex-start;
}

.memory-header .memory-hint {
  margin-left:auto;
}

.memory-title {
  min-width:0;
  color:var(--text-primary);
  font-size:var(--fs-base);
  font-weight:var(--fw-bold);
  line-height:var(--lh-tight);
}

.memory-hint,
.memory-info {
  color:var(--text-muted);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}

.memory-info {
  color:var(--text-secondary);
}

.memory-bytes {
  color:var(--text-secondary);
  font-family:var(--font-data);
  font-size:var(--fs-xs);
  word-break:break-all;
}

.feature-help {
  margin:0;
  padding:var(--space-3) var(--space-4);
  border-left:2px solid var(--info);
  border-radius:var(--radius-sm);
  background:var(--info-bg);
  color:var(--info-ink);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}

.feature-help.waiting {
  border-left-color:var(--warning);
  background:var(--warning-bg);
  color:var(--warning-ink);
}

.damage-meter-info {
  justify-content:space-between;
}

.damage-meter-value {
  color:var(--info-ink);
  font-family:var(--font-data);
  font-size:var(--fs-xl);
  font-weight:var(--fw-bold);
  line-height:var(--lh-tight);
  font-variant-numeric:tabular-nums;
}

.damage-meter-raw {
  margin-top:calc(-1 * var(--space-1));
  color:var(--text-muted);
  font-family:var(--font-data);
  font-size:var(--fs-sm);
}

.currency-grid {
  display:flex;
  flex-direction:column;
  gap:var(--space-2);
}

.currency-row {
  display:grid;
  grid-template-columns:minmax(92px,.7fr) minmax(150px,1.2fr) minmax(118px,.8fr) auto auto;
  align-items:center;
  gap:var(--space-2);
  padding:var(--space-2);
  border:1px solid var(--border-soft);
  border-radius:var(--radius-sm);
  background:var(--surface-card);
}

.currency-name {
  color:var(--text-primary);
  font-size:var(--fs-md);
  font-weight:var(--fw-semibold);
}

.currency-meta {
  min-width:0;
  color:var(--text-muted);
  font-family:var(--font-data);
  font-size:var(--fs-xs);
  overflow-wrap:anywhere;
}

.currency-input {
  width:100%;
}

.batch-input {
  width:80px;
  min-height:var(--control-height);
  font-family:var(--font-data);
  font-size:var(--fs-md);
}

.countdown-input {
  width:96px;
}

.batch-input::-webkit-outer-spin-button,
.batch-input::-webkit-inner-spin-button {
  margin:0;
  -webkit-appearance:none;
}

.memory-diagnostics {
  margin-top:var(--space-1);
  box-shadow:none;
}

.memory-diagnostics > summary {
  min-height:var(--control-height-sm);
  color:var(--text-muted);
  font-size:var(--fs-xs);
}

.memory-diagnostics .memory-bytes {
  display:block;
  margin:0 var(--space-4) var(--space-4);
}

.reference-grid {
  --ui-grid-min:260px;
}

.reference-card {
  min-height:130px;
}

.empty {
  padding:var(--space-5) 0;
  color:var(--text-muted);
  font-size:var(--fs-md);
  text-align:center;
}

.confirm-overlay {
  position:fixed;
  inset:0;
  z-index:20;
  display:flex;
  align-items:center;
  justify-content:center;
  padding:var(--space-7);
  background:color-mix(in srgb,var(--text-primary) 42%,transparent);
}

.confirm-dialog {
  display:flex;
  width:min(420px,100%);
  padding:var(--space-7);
  flex-direction:column;
  gap:var(--space-5);
  border:1px solid var(--border-strong);
  border-radius:var(--radius-lg);
  background:var(--surface-card-pop);
  box-shadow:var(--shadow-3);
}

.confirm-title {
  color:var(--text-primary);
  font-size:var(--fs-lg);
  font-weight:var(--fw-bold);
}

.confirm-body {
  color:var(--text-secondary);
  font-size:var(--fs-md);
  line-height:var(--lh-relaxed);
}

.confirm-actions {
  display:flex;
  justify-content:flex-end;
  gap:var(--space-2);
  flex-wrap:wrap;
}

@container runtime-page (max-width:720px) {
  .section {
    padding:var(--space-4);
  }

  .runtime-tabs {
    width:100%;
  }

  .currency-row {
    grid-template-columns:minmax(0,1fr) minmax(118px,.8fr) auto auto;
  }

  .currency-meta {
    grid-column:1 / -1;
    grid-row:2;
  }
}

@container runtime-page (max-width:480px) {
  .header {
    align-items:flex-start;
  }

  .hint {
    width:100%;
    margin-left:0;
  }

  .currency-row {
    grid-template-columns:minmax(0,1fr) auto;
  }

  .currency-input {
    width:100%;
    grid-column:1 / -1;
  }

  .currency-row > .ui-btn.is-primary {
    width:100%;
    grid-column:1 / -1;
  }
}
</style>
