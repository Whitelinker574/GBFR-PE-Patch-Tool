<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { CharaAcquire, CharaRelease,
         CurrencyGetAllOwned, CurrencySetOneOwned,
         PotionGetAllOwned, PotionSetOneOwned,
         MaterialConsumeGetStatusOwned, MaterialConsumeSetEnabledOwned,
         CollectibleTaskCompleteOwned,
         InfiniteChallengeGetStatusOwned, InfiniteChallengeSetEnabledOwned,
         TerminusDropGetStatusOwned, TerminusDropScanOwned, TerminusDropSetEnabledOwned,
         MonsterEnhanceSetPatchValueEnabledOwned } from '../../wailsjs/go/main/App'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'

const emit = defineEmits(['status'])
const RUNTIME_LEASE_SCOPE = 'misc-tools'

const connected = ref(false)
const info = reactive({ pid: 0, moduleBase: 0, manager: 0 })
const loading = ref(false)

const materialConsumeStatus = reactive({ rva: 0, enabled: false, currentBytes: '' })
const materialConsumeLoading = ref(false)
const collectibleTaskLoading = ref(false)
const infiniteChallengeStatus = reactive({ rva: 0, enabled: false, owned: false, currentBytes: '' })
const infiniteChallengeLoading = ref(false)
const inventorySet45Enabled = ref(false)
const inventorySet45Loading = ref(false)
const inventorySet45Seconds = ref(0)
const inventorySetQuantity = ref(45)
const terminusDropStatus = reactive({ found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
const terminusDropLoading = ref(false)
const currencies = ref([])
const currencyInputs = reactive({})
const currencyLoading = ref(false)
const potions = ref([])
const potionInputs = reactive({})
const potionLoading = ref(false)
const activeRuntimeGroup = ref('resources')
const runtimeCatalog = computed(() => {
  const catalogs = {
    resources: [
      ['实时货币编辑', '金币、MSP、高级炼成点数与共鸣点数（RP）', '已适配'],
      ['副本药水', '复活药水与群疗药水数量', '需进入副本'],
      ['素材不消耗', '强化、练成期间临时阻止素材变化', '已适配'],
      ['小钳蟹相关', '临时调整拾取数量与完成收集任务', '运行时钩子'],
    ],
    mission: [
      ['连续挑战', '阻止挑战完成次数递增，可重复完成当前挑战', 'DLC 2.0.2 特征'],
      ['巴武掉落 100%', '移除巴武掉落的随机排除，保留原始资格检查', 'AOB 定位'],
    ],
  }
  return catalogs[activeRuntimeGroup.value] || []
})
let inventorySet45Timer = 0
let disposed = false
let lifecycleEpoch = 0
let connectionOwnerToken = ''

function clearConnectionState() {
  connected.value = false
  stopInventorySet45Timer()
  inventorySet45Enabled.value = false
  Object.assign(info, { pid: 0, moduleBase: 0, manager: 0 })
  Object.assign(materialConsumeStatus, { rva: 0, enabled: false, currentBytes: '' })
  Object.assign(infiniteChallengeStatus, { rva: 0, enabled: false, owned: false, currentBytes: '' })
  Object.assign(terminusDropStatus, { found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
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
    loadMaterialConsumeStatus()
    loadInfiniteChallengeStatus()
    loadTerminusDropStatus()
    loadCurrencyValues()
    loadPotionValues()
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

function applyInfiniteChallengeStatus(status) {
  Object.assign(infiniteChallengeStatus, status || { rva: 0, enabled: false, owned: false, currentBytes: '' })
}

function loadInfiniteChallengeStatus() {
  if (!connected.value) return
  infiniteChallengeLoading.value = true
  InfiniteChallengeGetStatusOwned(connectionOwnerToken)
    .then(applyInfiniteChallengeStatus)
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { infiniteChallengeLoading.value = false })
}

function setInfiniteChallengeEnabled(enabled) {
  if (!connected.value) { emit('status', '请先连接游戏进程', 'error'); return }
  infiniteChallengeLoading.value = true
  InfiniteChallengeSetEnabledOwned(connectionOwnerToken, enabled)
    .then((status) => {
      applyInfiniteChallengeStatus(status)
      emit('status', enabled ? '已开启连续挑战' : '已恢复挑战次数递增', 'success')
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { infiniteChallengeLoading.value = false })
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

onBeforeUnmount(() => {
  disposed = true
  lifecycleEpoch += 1
  const ownerToken = connectionOwnerToken
  connectionOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease)
  stopInventorySet45Timer()
})

</script>

<template>
  <div class="root ui-page is-wide ui-page-stack">
    <div class="section ui-card ui-panel">
      <div class="header">
        <span class="title">游戏内工具</span>
        <span class="info-dot" title="这些功能会修改游戏运行时内存，不写入存档；重启游戏或切换版本后需要重新连接并设置。">!</span>
        <span class="hint">需游戏运行中使用 · 重启后重新连接</span>
      </div>
      <div class="connect-row ui-toolbar">
        <button v-if="!connected" class="btn-connect ui-btn is-primary" @click="connect" :disabled="loading">
          {{ loading ? '连接中...' : '连接游戏进程' }}
        </button>
        <button v-else class="btn-disconnect ui-btn is-danger" @click="disconnect">断开连接</button>
        <span v-if="connected" class="pid ui-tag is-ok">PID: {{ info.pid }}</span>
      </div>

      <div class="runtime-tabs ui-seg">
        <button class="ui-seg-btn" :class="{ 'is-on': activeRuntimeGroup === 'resources' }" @click="activeRuntimeGroup = 'resources'">资源与药水</button>
        <button class="ui-seg-btn" :class="{ 'is-on': activeRuntimeGroup === 'mission' }" @click="activeRuntimeGroup = 'mission'">任务与掉落</button>
      </div>

      <div class="memory-card compatibility-note ui-card ui-panel is-compact">
        <div class="memory-header">
          <span class="memory-title">实时修改与离线编辑</span>
          <span class="memory-hint">DLC 2.0.2 分工</span>
        </div>
        <div class="memory-info">
          <span>金币、MSP、高级炼成点数和共鸣点数（RP）使用 2.0.2 特征动态定位，实时写入后立即回读。</span>
          <span>药水和“不消耗素材”在游戏运行时使用；添加具体物品、素材和武器仍放在“养成编辑（离线）”。</span>
          <span>实时数值需要让游戏正常触发一次保存；游戏运行时不要同时离线修改同一存档。</span>
        </div>
      </div>

      <template v-if="connected">
        <div v-if="activeRuntimeGroup === 'resources'" class="memory-card ui-card ui-panel is-compact" :class="{ active: currencies.length }">
          <div class="memory-header">
            <span class="memory-title">实时货币编辑</span>
            <span class="memory-hint">AOB 捕获玩家结构 · 写入后回读</span>
          </div>
          <p class="feature-help">用途：实时修改金币、MSP、高级炼成点数和共鸣点数（RP）。首次连接后若没有读数，请在游戏内打开主菜单或让资源发生一次变化。</p>
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

        <div v-if="activeRuntimeGroup === 'resources'" class="memory-card ui-card ui-panel is-compact" :class="{ active: potions.length }">
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

        <div v-if="activeRuntimeGroup === 'resources'" class="memory-card ui-card ui-panel is-compact" :class="{ active: materialConsumeStatus.enabled }">
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

        <div v-if="activeRuntimeGroup === 'resources'" class="memory-card ui-card ui-panel is-compact" :class="{ active: inventorySet45Enabled }">
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

        <div v-if="activeRuntimeGroup === 'mission'" class="memory-card ui-card ui-panel is-compact" :class="{ active: infiniteChallengeStatus.enabled && infiniteChallengeStatus.owned }">
          <div class="memory-header">
            <span class="memory-title">连续挑战</span>
            <span class="info-dot" title="使用 DLC 2.0.2 唯一特征定位，开启后阻止挑战完成次数递增。">!</span>
            <span class="memory-hint">唯一 AOB · 三字节补丁 · 写后回读</span>
          </div>
          <p class="feature-help">用途：完成挑战后保持当前挑战次数，便于连续重复。进入挑战前开启，用完后立即恢复默认。</p>
          <div class="memory-info">
            <span>RVA: {{ formatHex(infiniteChallengeStatus.rva) }}</span>
            <span>状态: {{ infiniteChallengeStatus.enabled ? (infiniteChallengeStatus.owned ? '已由本页开启' : '检测到外部补丁') : '默认' }}</span>
          </div>
          <div class="memory-row">
            <button class="btn-batch ui-btn is-primary is-sm" @click="setInfiniteChallengeEnabled(true)" :disabled="infiniteChallengeLoading || infiniteChallengeStatus.enabled">开启连续挑战</button>
            <button class="btn-refresh ui-btn is-sm" @click="setInfiniteChallengeEnabled(false)" :disabled="infiniteChallengeLoading || !infiniteChallengeStatus.enabled || !infiniteChallengeStatus.owned">恢复默认</button>
            <button class="btn-refresh ui-btn is-sm" @click="loadInfiniteChallengeStatus" :disabled="infiniteChallengeLoading">重新扫描 / 刷新</button>
          </div>
          <details class="memory-diagnostics ui-disclosure"><summary>技术详情</summary><code class="memory-bytes">{{ infiniteChallengeStatus.currentBytes || '未定位' }}</code></details>
        </div>

        <div v-if="activeRuntimeGroup === 'mission'" class="memory-card ui-card ui-panel is-compact" :class="{ active: terminusDropStatus.enabled }">
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

      </template>
      <div v-else class="preflight-grid ui-card-grid">
        <article v-for="item in runtimeCatalog" :key="item[0]" class="ui-card ui-panel is-compact">
          <div><strong>{{ item[0] }}</strong><p>{{ item[1] }}</p></div>
          <span :class="{ waiting: item[2] === '等待适配' }">{{ item[2] }}</span>
        </article>
        <div class="empty ui-empty">连接游戏进程后显示读取值和操作按钮</div>
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

.empty {
  padding:var(--space-5) 0;
  color:var(--text-muted);
  font-size:var(--fs-md);
  text-align:center;
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
