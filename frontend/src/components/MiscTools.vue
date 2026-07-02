<script setup>
import { reactive, ref } from 'vue'
import { CharaAttach, CharaDetach,
         CountdownGetStatus, CountdownScan, CountdownSet,
         FaceAccessoryGetStatus, FaceAccessoryScan, FaceAccessorySetHidden,
         InfiniteChallengeGetStatus, InfiniteChallengeSetEnabled,
         TerminusDropGetStatus, TerminusDropScan, TerminusDropSetEnabled,
         UnlockAllTrophyGetStatus, UnlockAllTrophyScan, UnlockAllTrophySetEnabled,
         OtherSkinPurpleRuneGetStatus, OtherSkinPurpleRuneSetEnabled,
         GetAppVersion, CheckUpdate, OpenReleasePage } from '../../wailsjs/go/main/App'

const emit = defineEmits(['status'])

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
const terminusDropStatus = reactive({ found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
const terminusDropLoading = ref(false)
const unlockAllTrophyStatus = reactive({ found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
const unlockAllTrophyLoading = ref(false)
const showUnlockAllTrophyConfirm = ref(false)
const otherSkinPurpleRuneStatus = reactive({ rva: 0, enabled: false, jumpOpcode: '', currentBytes: '' })
const otherSkinPurpleRuneLoading = ref(false)
const updateInfo = reactive({ currentVersion: 'v1.5.0', latestVersion: '', hasUpdate: false, releaseUrl: '', body: '', assets: [] })
const updateLoading = ref(false)

GetAppVersion().then(v => { updateInfo.currentVersion = v }).catch(() => {})

function connect() {
  loading.value = true
  CharaAttach()
    .then((res) => {
      connected.value = true
      Object.assign(info, res)
      loadCountdownStatus()
      loadFaceAccessoryStatus()
      loadInfiniteChallengeStatus()
      loadTerminusDropStatus()
      loadUnlockAllTrophyStatus()
      loadOtherSkinPurpleRuneStatus()
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { loading.value = false })
}

function disconnect() {
  CharaDetach()
    .then(() => {
      connected.value = false
      Object.assign(info, { pid: 0, moduleBase: 0, manager: 0 })
      Object.assign(countdownStatus, { found: false, address: 0, rva: 0, value1: 0, value2: 0, currentBytes: '' })
      Object.assign(faceAccessoryStatus, { found: false, address: 0, rva: 0, hidden: false, jumpOpcode: '', currentBytes: '' })
      Object.assign(infiniteChallengeStatus, { rva: 0, enabled: false, currentBytes: '' })
      Object.assign(terminusDropStatus, { found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
      Object.assign(unlockAllTrophyStatus, { found: false, address: 0, rva: 0, enabled: false, currentBytes: '' })
      Object.assign(otherSkinPurpleRuneStatus, { rva: 0, enabled: false, jumpOpcode: '', currentBytes: '' })
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

function checkUpdate() {
  updateLoading.value = true
  CheckUpdate()
    .then((info) => {
      Object.assign(updateInfo, info)
      emit('status', info.hasUpdate ? `发现新版本 ${info.latestVersion}` : '当前已是最新版本', info.hasUpdate ? 'success' : 'success')
    })
    .catch((err) => emit('status', String(err), 'error'))
    .finally(() => { updateLoading.value = false })
}

function openReleasePage() {
  OpenReleasePage(updateInfo.releaseUrl || '')
    .catch((err) => emit('status', String(err), 'error'))
}


</script>

<template>
  <div class="root">
    <div class="section">
      <div class="header">
        <span class="title">杂项</span>
        <span class="info-dot" title="这些功能会修改游戏运行时内存，不写入存档；重启游戏或切换版本后需要重新连接并设置。">!</span>
        <span class="hint">需游戏运行中使用 · 重启游戏后需重新设置</span>
      </div>
      <div class="connect-row">
        <button v-if="!connected" class="btn-connect" @click="connect" :disabled="loading">
          {{ loading ? '连接中...' : '连接游戏进程' }}
        </button>
        <button v-else class="btn-disconnect" @click="disconnect">断开连接</button>
        <span v-if="connected" class="pid">PID: {{ info.pid }}</span>
      </div>

      <div class="memory-card">
        <div class="memory-header">
          <span class="memory-title">检查更新</span>
          <span class="memory-hint">当前版本 {{ updateInfo.currentVersion }}</span>
        </div>
        <div class="memory-info">
          <span>最新版本: {{ updateInfo.latestVersion || '未检查' }}</span>
          <span v-if="updateInfo.hasUpdate" class="update-new">有新版本</span>
          <span v-else-if="updateInfo.latestVersion">已是最新</span>
        </div>
        <div v-if="updateInfo.body" class="update-body">{{ updateInfo.body }}</div>
        <div class="memory-row">
          <button class="btn-batch" @click="checkUpdate" :disabled="updateLoading">{{ updateLoading ? '检查中...' : '检查更新' }}</button>
          <button class="btn-refresh" @click="openReleasePage">打开 Release 页面</button>
        </div>
      </div>

      <template v-if="connected">
        <div class="memory-card">
          <div class="memory-header">
            <span class="memory-title">任务结算倒计时/零帧开箱</span>
            <span class="info-dot" title="任务结算倒计时超过30s会导致进度条消失，但计时正常；零帧开箱需设置为0s。">!</span>
            <span class="memory-hint">AOB 定位后动态写入两个 float 值</span>
          </div>
          <div class="memory-info">
            <span>RVA: {{ formatHex(countdownStatus.rva) }}</span>
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

        <div class="memory-card">
          <div class="memory-header">
            <span class="memory-title">脸部符文显示(紫色皮肤包)</span>
            <span class="memory-hint">切换 JE/JNE 控制渲染判断</span>
          </div>
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

        <div class="memory-card">
          <div class="memory-header">
            <span class="memory-title">无限挑战</span>
            <span class="memory-hint">NOP 挑战次数递增</span>
          </div>
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

        <div class="memory-card">
          <div class="memory-header">
            <span class="memory-title">巴武掉落 100%</span>
            <span class="info-dot" title="仅让原型巴哈姆特任务的巴武 lot 不再被 80% 排除；仍保留未拥有、角色已解锁等游戏原始检查。">!</span>
            <span class="memory-hint">AOB 定位后 NOP 巴武 lot 排除跳转</span>
          </div>
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

        <div class="memory-card">
          <div class="memory-header">
            <span class="memory-title">游戏内全称号解锁</span>
            <span class="memory-hint">AOB 定位后切换 SETNE/SETNO</span>
          </div>
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

        <div class="memory-card">
          <div class="memory-header">
            <span class="memory-title">在其他皮肤显示紫色符文</span>
            <span class="memory-hint">固定 RVA 切换 JNE/JE</span>
          </div>
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
      <div v-else class="empty">请先连接游戏进程</div>
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
.root { display:flex; flex-direction:column; gap:10px; width:100%; max-width:720px; margin:0 auto; padding-bottom:40px; }
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
.pid { font-size:0.72rem; color:rgba(255,255,255,0.35); font-family:'Courier New',monospace; }
.memory-card {
  border-radius:12px; padding:12px;
  background:rgba(255,255,255,0.045); border:1px solid rgba(165,180,252,0.16);
  display:flex; flex-direction:column; gap:8px;
}
.memory-header, .memory-info, .memory-row { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
.memory-header { justify-content:flex-start; }
.memory-header .memory-hint { margin-left:auto; }
.memory-title { font-size:0.8rem; font-weight:600; color:rgba(255,255,255,0.62); }
.memory-hint, .memory-info { font-size:0.68rem; color:rgba(255,255,255,0.32); }
.memory-bytes { font-size:0.66rem; color:rgba(255,255,255,0.24); font-family:'Courier New',monospace; word-break:break-all; }
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
.burst-timer { color:#facc15; font-weight:600; font-family:'Courier New',monospace; }
.confirm-overlay { position:fixed; inset:0; z-index:20; display:flex; align-items:center; justify-content:center; padding:20px; background:rgba(0,0,0,0.48); }
.confirm-dialog { width:min(420px, 100%); border-radius:12px; padding:16px; background:linear-gradient(135deg, rgba(251,191,36,0.22) 0%, rgba(239,68,68,0.16) 100%); border:1px solid rgba(251,191,36,0.34); box-shadow:0 12px 40px rgba(0,0,0,0.42); display:flex; flex-direction:column; gap:12px; }
.confirm-title { font-size:0.9rem; font-weight:700; color:#facc15; }
.confirm-body { font-size:0.78rem; line-height:1.65; color:rgba(255,255,255,0.72); }
.confirm-actions { display:flex; justify-content:flex-end; gap:8px; flex-wrap:wrap; }
.btn-warn { padding:6px 14px; border-radius:6px; border:1px solid rgba(251,191,36,0.45); background:rgba(251,191,36,0.16); color:#facc15; font-size:0.78rem; font-weight:600; cursor:pointer; transition:background 0.2s; white-space:nowrap; }
.btn-warn:not(:disabled):hover { background:rgba(251,191,36,0.26); }
.btn-warn:disabled { opacity:0.4; cursor:not-allowed; }
</style>
