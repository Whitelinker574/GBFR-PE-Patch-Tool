<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { CharaAttach, CharaDetach,
         CurrencyGetAll, CurrencySetOne,
         PotionGetAll, PotionSetOne,
         CountdownGetStatus, CountdownScan, CountdownSet,
         FaceAccessoryGetStatus, FaceAccessoryScan, FaceAccessorySetHidden,
         InfiniteChallengeGetStatus, InfiniteChallengeSetEnabled,
         MaterialConsumeGetStatus, MaterialConsumeSetEnabled,
         TerminusDropGetStatus, TerminusDropScan, TerminusDropSetEnabled,
         UnlockAllTrophyGetStatus, UnlockAllTrophyScan, UnlockAllTrophySetEnabled,
         OtherSkinPurpleRuneGetStatus, OtherSkinPurpleRuneSetEnabled,
         MonsterEnhanceSetPatchValueEnabled,
         DamageMeterGetStatus, DamageMeterReset,
         DamageOverlaySetEnabled, DamageOverlaySetValue, DamageOverlaySetFontSize } from '../../wailsjs/go/main/App'

const emit = defineEmits(['status'])
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

function getMonsterEnhanceMultiplier(id) {
  const saved = window.gbfrMonsterEnhanceMultipliers || {}
  const value = parseFloat(saved[id] || '1')
  return isNaN(value) || value <= 0 || value > 9999 ? 1 : value
}

function connect() {
  loading.value = true
  CharaAttach()
    .then((res) => {
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
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { loading.value = false })
}

function disconnect() {
  CharaDetach()
    .then(() => {
      connected.value = false
      stopDamageMeterTimer()
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
    })
    .catch((err) => emit('status', String(err), 'error'))
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
  MaterialConsumeGetStatus()
    .then(applyMaterialConsumeStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { materialConsumeLoading.value = false })
}

function setMaterialConsumeEnabled(enabled) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  materialConsumeLoading.value = true
  MaterialConsumeSetEnabled(enabled)
    .then((status) => { applyMaterialConsumeStatus(status); emit('status', enabled ? '已开启升级/强化不材料消耗' : '已恢复升级/强化材料变化', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { materialConsumeLoading.value = false })
}

function applyTerminusDropStatus(status) {
  Object.assign(terminusDropStatus, status || { found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
}

function loadTerminusDropStatus() {
  if (!connected.value) return
  terminusDropLoading.value = true
  TerminusDropGetStatus()
    .then(applyTerminusDropStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { terminusDropLoading.value = false })
}

function scanTerminusDrop() {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  terminusDropLoading.value = true
  TerminusDropScan()
    .then((status) => { applyTerminusDropStatus(status); emit('status', '巴武掉落特征定位成功', 'success') })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { terminusDropLoading.value = false })
}

function setTerminusDropEnabled(enabled) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  terminusDropLoading.value = true
  TerminusDropSetEnabled(enabled)
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
  CurrencyGetAll()
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
  CurrencySetOne(item.id, value)
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
  PotionGetAll()
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
  PotionSetOne(item.id, value)
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
  MonsterEnhanceSetPatchValueEnabled('monster_hp', true, getMonsterEnhanceMultiplier('monster_hp'))
    .then(() => MonsterEnhanceSetPatchValueEnabled('crocodile_damage', true, getMonsterEnhanceMultiplier('crocodile_damage')))
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
  stopDamageMeterTimer()
  if (damageOverlayEnabled.value) disableDamageOverlay()
})

</script>

<template>
  <div class="root">
    <div class="section">
      <div class="header">
        <span class="title">{{ showOutdatedFeatures ? '待适配运行时功能' : '游戏内工具' }}</span>
        <span class="info-dot" title="这些功能会修改游戏运行时内存，不写入存档；重启游戏或切换版本后需要重新连接并设置。">!</span>
        <span class="hint">{{ showOutdatedFeatures ? '兼容性实验室 · 默认仅建议扫描与诊断' : '需游戏运行中使用 · 重启后重新连接' }}</span>
      </div>
      <div class="connect-row">
        <button v-if="!connected" class="btn-connect" @click="connect" :disabled="loading">
          {{ loading ? '连接中...' : '连接游戏进程' }}
        </button>
        <button v-else class="btn-disconnect" @click="disconnect">断开连接</button>
        <span v-if="connected" class="pid">PID: {{ info.pid }}</span>
      </div>

      <div class="runtime-tabs">
        <template v-if="showStableFeatures">
          <button :class="{ active: activeRuntimeGroup === 'resources' }" @click="activeRuntimeGroup = 'resources'">资源与药水</button>
          <button :class="{ active: activeRuntimeGroup === 'mission' }" @click="activeRuntimeGroup = 'mission'">任务与掉落</button>
        </template>
        <template v-else>
          <button :class="{ active: activeRuntimeGroup === 'battle' }" @click="activeRuntimeGroup = 'battle'">战斗与任务</button>
          <button :class="{ active: activeRuntimeGroup === 'display' }" @click="activeRuntimeGroup = 'display'">显示与解锁</button>
        </template>
      </div>

      <div v-if="showStableFeatures" class="memory-card compatibility-note">
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
        <div v-if="showStableFeatures && activeRuntimeGroup === 'resources'" class="memory-card" :class="{ active: currencies.length }">
          <div class="memory-header">
            <span class="memory-title">实时货币编辑</span>
            <span class="memory-hint">AOB 捕获玩家结构 · 写入后回读</span>
          </div>
          <p class="feature-help">用途：实时修改金币、MSP、高级炼成点数和 CP。首次连接后若没有读数，请在游戏内打开主菜单或让资源发生一次变化。</p>
          <div class="currency-grid">
            <div v-for="item in currencies" :key="item.id" class="currency-row">
              <div class="currency-name">{{ item.name }}</div>
              <div class="currency-meta">{{ formatInt(item.value) }} · {{ formatHex(item.rva) }} + {{ formatHex(item.offset) }}</div>
              <input v-model="currencyInputs[item.id]" type="number" min="0" max="2147483647" step="1" class="batch-input currency-input" />
              <button class="btn-max" @click="currencyInputs[item.id]='2147483647'">最大</button>
              <button class="btn-batch" @click="setCurrency(item)" :disabled="currencyLoading">写入</button>
            </div>
          </div>
          <div class="memory-row">
            <button class="btn-refresh" @click="loadCurrencyValues" :disabled="currencyLoading">{{ currencyLoading ? '定位中…' : '重新定位 / 刷新' }}</button>
          </div>
          <div v-if="!currencies.length" class="memory-info"><span>首次连接会安装临时读取点；若尚无数据，请在游戏内打开主菜单或让金币/MSP刷新一次后再点刷新。</span></div>
        </div>

        <div v-if="showStableFeatures && activeRuntimeGroup === 'resources'" class="memory-card" :class="{ active: potions.length }">
          <div class="memory-header">
            <span class="memory-title">药神（进入副本后点刷新看到药水数量正常后设置即可）</span>
            <span class="memory-hint">稳定指针链读取/写入 int32</span>
          </div>
          <p class="feature-help">用途：在副本内调整复活药和群疗药数量。必须先进入副本，刷新看到正常数量后再写入。</p>
          <div class="currency-grid">
            <div v-for="item in potions" :key="item.id" class="currency-row">
              <div class="currency-name">{{ item.name }}</div>
              <div class="currency-meta">{{ formatInt(item.value) }} · {{ formatHex(item.rva) }} + {{ formatOffsets(item.offsets) }}</div>
              <input v-model="potionInputs[item.id]" type="number" min="0" max="2147483647" step="1" class="batch-input currency-input" />
              <button class="btn-max" @click="potionInputs[item.id]='2147483647'">最大</button>
              <button class="btn-batch" @click="setPotion(item)" :disabled="potionLoading">写入</button>
            </div>
          </div>
          <div class="memory-row">
            <button class="btn-refresh" @click="loadPotionValues" :disabled="potionLoading">刷新药水</button>
          </div>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'battle'" class="memory-card" :class="{ active: damageMeterStatus.connected && damageMeterStatus.totalDamage > 0 }">
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
            <button class="btn-batch" @click="enableDamageMeter" :disabled="damageMeterLoading">开启记录</button>
            <button class="btn-refresh" @click="toggleDamageOverlay" :disabled="damageMeterLoading || !damageMeterStatus.connected">{{ damageOverlayEnabled ? '关闭悬浮窗' : '开启悬浮窗' }}</button>
            <button class="btn-refresh" @click="loadDamageMeterStatus" :disabled="damageMeterLoading">刷新</button>
            <button class="btn-refresh" @click="resetDamageMeter" :disabled="damageMeterLoading">清零</button>
            <button class="btn-sort" @click="setDamageOverlayFontSize(damageOverlayFontSize - 4)" :disabled="!damageOverlayEnabled">字号 -</button>
            <button class="btn-sort" @click="setDamageOverlayFontSize(damageOverlayFontSize + 4)" :disabled="!damageOverlayEnabled">字号 +</button>
          </div>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'battle'" class="memory-card" :class="{ active: isCountdownActive() }">
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
            <input v-model="countdownValue" type="number" min="0" max="9999" step="0.1" class="batch-input countdown-input" placeholder="秒数" />
            <button class="btn-batch" @click="setCountdown" :disabled="countdownLoading">设置倒计时</button>
            <button class="btn-refresh" @click="loadCountdownStatus" :disabled="countdownLoading">刷新</button>
            <button class="btn-sort" @click="scanCountdown" :disabled="countdownLoading">重新扫描</button>
          </div>
          <div class="memory-bytes">{{ countdownStatus.currentBytes || '未定位' }}</div>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'display'" class="memory-card" :class="{ active: faceAccessoryStatus.hidden }">
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
            <button class="btn-batch" @click="setFaceAccessoryHidden(true)" :disabled="faceAccessoryLoading || faceAccessoryStatus.hidden">隐藏脸部符文</button>
            <button class="btn-refresh" @click="setFaceAccessoryHidden(false)" :disabled="faceAccessoryLoading || !faceAccessoryStatus.hidden">恢复符文显示</button>
            <button class="btn-refresh" @click="loadFaceAccessoryStatus" :disabled="faceAccessoryLoading">刷新</button>
            <button class="btn-sort" @click="scanFaceAccessory" :disabled="faceAccessoryLoading">重新扫描</button>
          </div>
          <div class="memory-bytes">{{ faceAccessoryStatus.currentBytes || '未定位' }}</div>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'battle'" class="memory-card" :class="{ active: infiniteChallengeStatus.enabled }">
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
            <button class="btn-batch" @click="setInfiniteChallengeEnabled(true)" :disabled="infiniteChallengeLoading || infiniteChallengeStatus.enabled">开启无限挑战</button>
            <button class="btn-refresh" @click="setInfiniteChallengeEnabled(false)" :disabled="infiniteChallengeLoading || !infiniteChallengeStatus.enabled">恢复默认</button>
            <button class="btn-refresh" @click="loadInfiniteChallengeStatus" :disabled="infiniteChallengeLoading">刷新</button>
          </div>
          <div class="memory-bytes">{{ infiniteChallengeStatus.currentBytes || '未读取' }}</div>
        </div>

        <div v-if="showStableFeatures && activeRuntimeGroup === 'resources'" class="memory-card" :class="{ active: materialConsumeStatus.enabled }">
          <div class="memory-header">
            <span class="memory-title">升级/强化/练成不材料消耗（及开及用，开启后进入副本会导致无药水/无法获得奖励材料等问题）</span>
            <span class="info-dot" title="开启后材料数量不会减少；同一指令也会阻止材料增加。">!</span>
            <span class="memory-hint">校验 RVA，失效时 AOB 重定位</span>
          </div>
          <p class="feature-help">用途：强化或练成时让素材数量不减少；同一指令也会阻止素材增加，因此用完立刻恢复，开启时不要进入副本。</p>
          <div class="memory-info">
            <span>RVA: {{ formatHex(materialConsumeStatus.rva) }}</span>
            <span>状态: {{ materialConsumeStatus.enabled ? '开启' : '默认' }}</span>
          </div>
          <div class="memory-row">
            <button class="btn-batch" @click="setMaterialConsumeEnabled(true)" :disabled="materialConsumeLoading || materialConsumeStatus.enabled">开启不消耗</button>
            <button class="btn-refresh" @click="setMaterialConsumeEnabled(false)" :disabled="materialConsumeLoading || !materialConsumeStatus.enabled">恢复默认</button>
            <button class="btn-refresh" @click="loadMaterialConsumeStatus" :disabled="materialConsumeLoading">刷新</button>
          </div>
          <div class="memory-bytes">{{ materialConsumeStatus.currentBytes || '未读取' }}</div>
        </div>

        <div v-if="showStableFeatures && activeRuntimeGroup === 'mission'" class="memory-card" :class="{ active: terminusDropStatus.enabled }">
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
            <button class="btn-batch" @click="setTerminusDropEnabled(true)" :disabled="terminusDropLoading || terminusDropStatus.enabled">开启巴武 100%</button>
            <button class="btn-refresh" @click="setTerminusDropEnabled(false)" :disabled="terminusDropLoading || !terminusDropStatus.enabled">恢复默认</button>
            <button class="btn-refresh" @click="loadTerminusDropStatus" :disabled="terminusDropLoading">刷新</button>
            <button class="btn-sort" @click="scanTerminusDrop" :disabled="terminusDropLoading">重新扫描</button>
          </div>
          <div class="memory-bytes">{{ terminusDropStatus.currentBytes || '未定位' }}</div>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'display'" class="memory-card" :class="{ active: unlockAllTrophyStatus.enabled }">
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
            <button class="btn-batch" @click="setUnlockAllTrophyEnabled(true)" :disabled="unlockAllTrophyLoading || unlockAllTrophyStatus.enabled">开启全称号</button>
            <button class="btn-refresh" @click="setUnlockAllTrophyEnabled(false)" :disabled="unlockAllTrophyLoading || !unlockAllTrophyStatus.enabled">恢复默认</button>
            <button class="btn-refresh" @click="loadUnlockAllTrophyStatus" :disabled="unlockAllTrophyLoading">刷新</button>
            <button class="btn-sort" @click="scanUnlockAllTrophy" :disabled="unlockAllTrophyLoading">重新扫描</button>
          </div>
          <div class="memory-bytes">{{ unlockAllTrophyStatus.currentBytes || '未定位' }}</div>
        </div>

        <div v-if="showOutdatedFeatures && activeRuntimeGroup === 'display'" class="memory-card" :class="{ active: otherSkinPurpleRuneStatus.enabled }">
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
            <button class="btn-batch" @click="setOtherSkinPurpleRuneEnabled(true)" :disabled="otherSkinPurpleRuneLoading || otherSkinPurpleRuneStatus.enabled">开启显示</button>
            <button class="btn-refresh" @click="setOtherSkinPurpleRuneEnabled(false)" :disabled="otherSkinPurpleRuneLoading || !otherSkinPurpleRuneStatus.enabled">恢复默认</button>
            <button class="btn-refresh" @click="loadOtherSkinPurpleRuneStatus" :disabled="otherSkinPurpleRuneLoading">刷新</button>
          </div>
          <div class="memory-bytes">{{ otherSkinPurpleRuneStatus.currentBytes || '未读取' }}</div>
        </div>

      </template>
      <div v-else class="preflight-grid">
        <article v-for="item in runtimeCatalog" :key="item[0]">
          <div><strong>{{ item[0] }}</strong><p>{{ item[1] }}</p></div>
          <span :class="{ waiting: item[2] === '等待适配' }">{{ item[2] }}</span>
        </article>
        <div class="empty">连接游戏进程后显示读取值和操作按钮</div>
      </div>
    </div>
    <div v-if="showUnlockAllTrophyConfirm" class="confirm-overlay" @click.self="showUnlockAllTrophyConfirm = false">
      <div class="confirm-dialog">
        <div class="confirm-title">确认开启游戏内全称号解锁</div>
        <div class="confirm-body">目前存档时机尚不明确，可以领取任务奖励、佩戴选定称号、选择佩戴界面有多个“未设置”是正常现象</div>
        <div class="confirm-actions">
          <button class="btn-refresh" @click="showUnlockAllTrophyConfirm = false">取消</button>
          <button class="btn-warn" @click="confirmUnlockAllTrophy" :disabled="unlockAllTrophyLoading">确认开启</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.root { display:flex; flex-direction:column; gap:10px; width:100%; max-width:720px; margin:0 auto; padding-bottom:40px; container-type:inline-size; }
.section {
  border-radius:16px; padding:16px 18px;
  background:linear-gradient(135deg, rgba(56,189,248,0.12) 0%, rgba(103,232,249,0.06) 100%);
  border:1px solid rgba(103,232,249,0.15);
  display:flex; flex-direction:column; gap:10px;
}
.header { display:flex; align-items:center; justify-content:space-between; gap:8px; }
.title { font-size:0.88rem; font-weight:600; color:rgba(255,255,255,0.65); letter-spacing:1px; }
.info-dot { display:inline-flex; align-items:center; justify-content:center; width:15px; height:15px; border-radius:50%; border:1px solid rgba(103,232,249,0.35); color:#67e8f9; background:rgba(103,232,249,0.08); font-size:0.68rem; font-weight:700; cursor:help; flex-shrink:0; }
.hint { font-size:0.68rem; color:rgba(255,255,255,0.25); margin-left:auto; }
.connect-row { display:flex; align-items:center; gap:10px; }
.runtime-tabs { display:flex; align-items:center; gap:4px; padding:4px; border:1px solid rgba(255,255,255,.08); border-radius:7px; background:rgba(0,0,0,.12); }
.runtime-tabs button { flex:1; min-height:30px; border:1px solid transparent; border-radius:5px; background:transparent; color:rgba(255,255,255,.36); font-size:.7rem; font-weight:600; cursor:pointer; }
.runtime-tabs button:hover { color:rgba(255,255,255,.7); background:rgba(255,255,255,.035); }
.runtime-tabs button.active { color:#67e8f9; border-color:rgba(103,232,249,.2); background:rgba(103,232,249,.08); }
.preflight-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:7px; }
.preflight-grid article { min-height:58px; display:flex; align-items:center; justify-content:space-between; gap:12px; padding:10px 12px; border:1px solid rgba(255,255,255,.07); border-radius:7px; background:rgba(255,255,255,.018); }
.preflight-grid strong { display:block; color:rgba(255,255,255,.62); font-size:.72rem; }
.preflight-grid p { margin:3px 0 0; color:rgba(255,255,255,.28); font-size:.62rem; line-height:1.45; }
.preflight-grid article>span { flex-shrink:0; padding:3px 6px; border:1px solid rgba(103,232,249,.16); border-radius:4px; color:rgba(103,232,249,.66); font-size:.58rem; }
.preflight-grid article>span.waiting { border-color:rgba(251,191,36,.18); color:rgba(251,191,36,.66); }
.preflight-grid .empty { grid-column:1/-1; }
.btn-connect {
  padding:8px 18px; border-radius:8px; border:1px solid rgba(34,197,94,0.4);
  background:rgba(34,197,94,0.12); color:#4ade80; font-size:0.82rem; font-weight:600; cursor:pointer;
  transition:background 0.2s,transform 0.15s;
}
.btn-connect:not(:disabled):hover { background:rgba(34,197,94,0.22); transform:scale(1.02); }
.btn-connect:disabled { opacity:0.5; cursor:not-allowed; }
.btn-disconnect {
  padding:8px 18px; border-radius:8px; border:1px solid rgba(239,68,68,0.4);
  background:rgba(239,68,68,0.12); color:#f87171; font-size:0.82rem; font-weight:600; cursor:pointer;
  transition:background 0.2s;
}
.btn-disconnect:hover { background:rgba(239,68,68,0.22); }
.pid { font-size:0.72rem; color:rgba(255,255,255,0.35); font-family:var(--font-data); }
.memory-card {
  position:relative; overflow:hidden; z-index:0;
  border-radius:12px; padding:12px;
  background:rgba(255,255,255,0.045); border:1px solid rgba(165,180,252,0.16);
  box-shadow:0 10px 26px rgba(0,0,0,0.18);
  display:flex; flex-direction:column; gap:8px;
  transition:border-color 0.3s, box-shadow 0.3s, transform 0.3s;
}
.memory-card::after {
  content:""; position:absolute; inset:0; z-index:-1; border-radius:12px;
  background:#abd373; transform:translateY(calc(-100% - 2px));
  transition:transform 0.5s ease;
}
.memory-card.active { border-color:rgba(171,211,115,0.55); box-shadow:0 14px 34px rgba(171,211,115,0.18); }
.memory-card.active::after { transform:translateY(0); }
.memory-card.active .memory-title { color:#1f2937; }
.memory-card.active .memory-hint,
.memory-card.active .memory-info,
.memory-card.active .memory-bytes { color:rgba(31,41,55,0.72); }
.memory-card.active .info-dot { border-color:rgba(31,41,55,0.28); color:#1f2937; background:rgba(31,41,55,0.08); }
.memory-card.active .btn-batch { border-color:rgba(31,41,55,0.22); background:rgba(31,41,55,0.12); color:#1f2937; }
.memory-card.active .btn-refresh,
.memory-card.active .btn-sort { border-color:rgba(31,41,55,0.16); background:rgba(255,255,255,0.18); color:rgba(31,41,55,0.72); }
.memory-card.active .batch-input { border-color:rgba(31,41,55,0.22); background:rgba(255,255,255,0.22); color:#1f2937; }
.memory-header, .memory-info, .memory-row { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
.memory-header { justify-content:flex-start; }
.memory-header .memory-hint { margin-left:auto; }
.memory-title { font-size:0.8rem; font-weight:600; color:rgba(255,255,255,0.62); }
.memory-hint, .memory-info { font-size:0.68rem; color:rgba(255,255,255,0.32); }
.memory-bytes { font-size:0.66rem; color:rgba(255,255,255,0.24); font-family:var(--font-data); word-break:break-all; }
.feature-help { margin:0; padding:7px 9px; border-left:2px solid rgba(103,232,249,.34); background:rgba(103,232,249,.035); color:rgba(255,255,255,.38); font-size:.67rem; line-height:1.55; }
.feature-help.waiting { border-left-color:rgba(251,191,36,.42); background:rgba(251,191,36,.035); color:rgba(251,191,36,.55); }
.damage-meter-info { justify-content:space-between; }
.damage-meter-value { font-size:1.8rem; font-weight:700; color:#67e8f9; line-height:1; }
.damage-meter-raw { margin-top:-4px; font-size:0.72rem; color:rgba(255,255,255,0.28); }
.memory-card.active .damage-meter-value { color:#1f2937; }
.memory-card.active .damage-meter-raw { color:rgba(31,41,55,0.56); }
.currency-grid { display:flex; flex-direction:column; gap:8px; }
.currency-row { display:grid; grid-template-columns:90px 1fr 120px auto auto; align-items:center; gap:8px; }
.currency-name { font-size:0.78rem; font-weight:600; color:rgba(255,255,255,0.62); }
.currency-meta { font-size:0.66rem; color:rgba(255,255,255,0.28); font-family:var(--font-data); }
.currency-input { width:120px; }
.btn-max { padding:6px 9px;border:1px solid rgba(218,187,115,.28);border-radius:3px 7px 3px 7px;background:rgba(218,187,115,.07);color:#d9bd7c;font-size:.7rem;cursor:pointer }
.memory-card.active .currency-name { color:#1f2937; }
.memory-card.active .currency-meta { color:rgba(31,41,55,0.56); }
.update-new { color:#4ade80; }
.update-body { max-height:86px; overflow-y:auto; padding:8px 10px; border-radius:8px; background:rgba(255,255,255,0.03); color:rgba(255,255,255,0.36); font-size:0.7rem; line-height:1.45; white-space:pre-wrap; scrollbar-width:thin; scrollbar-color:rgba(255,255,255,0.12) transparent; }
.batch-input {
  width:80px; padding:6px 10px; border-radius:6px; border:1px solid rgba(255,255,255,0.15);
  background:rgba(255,255,255,0.07); color:#fff; font-size:0.82rem; outline:none;
}
.countdown-input { width:96px; }
.batch-input:focus { border-color:rgba(103,232,249,0.5); }
.batch-input::-webkit-outer-spin-button, .batch-input::-webkit-inner-spin-button { -webkit-appearance:none; margin:0; }
.btn-batch {
  padding:6px 14px; border-radius:6px; border:1px solid rgba(165,180,252,0.3);
  background:rgba(165,180,252,0.1); color:#a5b4fc; font-size:0.78rem; font-weight:600; cursor:pointer;
  transition:background 0.2s; white-space:nowrap;
}
.btn-batch:not(:disabled):hover { background:rgba(165,180,252,0.2); }
.btn-batch:disabled { opacity:0.4; cursor:not-allowed; }
@container (max-width:620px) {
  .runtime-tabs { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); }
  .preflight-grid { grid-template-columns:1fr; }
  .currency-row { grid-template-columns:minmax(90px,1fr) minmax(96px,120px) auto auto; }
  .currency-meta { grid-column:1/-1; grid-row:2; overflow-wrap:anywhere; }
  .currency-input { width:100%; min-width:0; }
}
@container (max-width:430px) {
  .currency-row { grid-template-columns:minmax(0,1fr) minmax(90px,110px) auto; }
  .currency-row>.btn-max { grid-column:3; }
  .currency-row>.btn-batch { grid-column:1/-1; width:100%; }
}
.btn-refresh, .btn-sort {
  padding:6px 14px; border-radius:6px; border:1px solid rgba(255,255,255,0.12);
  background:rgba(255,255,255,0.05); color:rgba(255,255,255,0.5); font-size:0.78rem; font-weight:600; cursor:pointer;
  transition:background 0.2s;
}
.btn-refresh:hover, .btn-sort:hover { background:rgba(255,255,255,0.1); color:rgba(255,255,255,0.7); }
.btn-refresh:disabled, .btn-sort:disabled { opacity:0.4; cursor:not-allowed; }
.empty { font-size:0.78rem; color:rgba(255,255,255,0.3); text-align:center; padding:12px 0; }
.od-select {
  padding:6px 10px; border-radius:6px; border:1px solid rgba(255,255,255,0.15);
  background:rgba(255,255,255,0.07); color:#fff; font-size:0.8rem; outline:none; cursor:pointer;
}
.od-select:focus { border-color:rgba(103,232,249,0.5); }
.od-select option { background:#1a1a2e; color:#fff; }
.od-indicator {
  font-size:0.72rem; padding:4px 10px; border-radius:6px; text-align:center;
  background:rgba(255,255,255,0.05); color:rgba(255,255,255,0.35);
  transition:all 0.3s;
}
.od-mode-active { background:rgba(250,204,21,0.15); color:#facc15; border:1px solid rgba(250,204,21,0.25); }
.od-burst-active { background:rgba(239,68,68,0.15); color:#ef4444; border:1px solid rgba(239,68,68,0.25); animation:od-burst-pulse 1s infinite alternate; }
@keyframes od-burst-pulse { from { opacity:0.7; } to { opacity:1; } }
.burst-timer { color:#facc15; font-weight:750; font-family:var(--font-data); }
.confirm-overlay { position:fixed; inset:0; z-index:20; display:flex; align-items:center; justify-content:center; padding:20px; background:rgba(0,0,0,0.48); }
.confirm-dialog { width:min(420px, 100%); border-radius:12px; padding:16px; background:linear-gradient(135deg, rgba(251,191,36,0.22) 0%, rgba(239,68,68,0.16) 100%); border:1px solid rgba(251,191,36,0.34); box-shadow:0 12px 40px rgba(0,0,0,0.42); display:flex; flex-direction:column; gap:12px; }
.confirm-title { font-size:0.9rem; font-weight:700; color:#facc15; }
.confirm-body { font-size:0.78rem; line-height:1.65; color:rgba(255,255,255,0.72); }
.confirm-actions { display:flex; justify-content:flex-end; gap:8px; flex-wrap:wrap; }
.btn-warn { padding:6px 14px; border-radius:6px; border:1px solid rgba(251,191,36,0.45); background:rgba(251,191,36,0.16); color:#facc15; font-size:0.78rem; font-weight:600; cursor:pointer; transition:background 0.2s; white-space:nowrap; }
.btn-warn:not(:disabled):hover { background:rgba(251,191,36,0.26); }
.btn-warn:disabled { opacity:0.4; cursor:not-allowed; }
</style>
