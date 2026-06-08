<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import { AutoDetect, SetExePath, GetStatus, PatchFile, BackupFile, RestoreFile } from '../../wailsjs/go/main/App'
import { WindowMinimise, Quit } from '../../wailsjs/runtime/runtime'
import SigilGenerator from './SigilGenerator.vue'
import WrightstoneGenerator from './WrightstoneGenerator.vue'
import SaveEditor from './SaveEditor.vue'
import CharaStats from './CharaStats.vue'
import MiscTools from './MiscTools.vue'

const state = reactive({
  exePath: '',
  fileExists: false,
  fileSize: 0,
  backupExists: false,
  backupSize: 0,
  patches: [],
})

const activeTab = ref('patch')
const manualPath = ref('')
const patchValues = reactive({}) // { patchID: 'value' }
const isLoaded = ref(false)
const isDetecting = ref(false)
const patchingID = ref('')
const forceBackup = ref(false)
const saveStatus = ref('')
const statusType = ref('')

onMounted(() => {
  isDetecting.value = true
  AutoDetect()
    .then((path) => {
      isDetecting.value = false
      if (path) {
        state.exePath = path
        manualPath.value = path
        return loadFile(path)
      }
    })
    .catch(() => { isDetecting.value = false })
})

function loadFile(path) {
  return GetStatus(path).then((info) => {
    Object.assign(state, info)
    ;(info.patches || []).forEach(p => {
      if (p.state === 'patched') patchValues[p.id] = String(p.currentValue)
      else if (!patchValues[p.id]) patchValues[p.id] = ''
    })
    isLoaded.value = true
    showStatus('文件加载成功', 'success')
  })
}

function applyManualPath() {
  const p = manualPath.value.trim()
  if (!p) { showStatus('请输入文件路径', 'error'); return }
  SetExePath(p)
    .then((info) => {
      Object.assign(state, info)
      ;(info.patches || []).forEach(p => {
        if (p.state === 'patched') patchValues[p.id] = String(p.currentValue)
        else if (!patchValues[p.id]) patchValues[p.id] = ''
      })
      isLoaded.value = true
      showStatus('文件加载成功', 'success')
    })
    .catch((err) => showStatus(String(err), 'error'))
}

function refreshStatus() {
  return GetStatus(state.exePath).then((info) => {
    Object.assign(state, info)
    ;(info.patches || []).forEach(p => {
      if (p.state === 'patched') patchValues[p.id] = String(p.currentValue)
    })
  })
}

function applyPatch(patchID) {
  const v = parseInt(patchValues[patchID])
  if (isNaN(v) || v < 0) { showStatus('请输入有效数值', 'error'); return }
  patchingID.value = patchID
  PatchFile(patchID, v)
    .then(() => refreshStatus())
    .then(() => { patchingID.value = ''; showStatus('补丁写入成功', 'success') })
    .catch((err) => { patchingID.value = ''; showStatus('补丁失败: ' + (err || '未知错误'), 'error') })
}

function backup() {
  BackupFile(forceBackup.value)
    .then(() => refreshStatus())
    .then(() => showStatus('备份创建成功', 'success'))
    .catch((err) => showStatus('备份失败: ' + (err || '未知错误'), 'error'))
}

function restore() {
  RestoreFile()
    .then(() => refreshStatus())
    .then(() => showStatus('文件已恢复', 'success'))
    .catch((err) => showStatus('恢复失败: ' + (err || '未知错误'), 'error'))
}

const CARD_COLORS = {
  mission: { bg: 'linear-gradient(135deg, rgba(124,58,237,0.25) 0%, rgba(249,212,35,0.1) 100%)', shadow: 'rgba(124,58,237,0.18)' },
  likes:   { bg: 'linear-gradient(135deg, rgba(245,158,11,0.25) 0%, rgba(249,212,35,0.1) 100%)', shadow: 'rgba(245,158,11,0.18)' },
}

const CARD_HINTS = {
  mission: '此修改不影响存档',
  likes: '被点赞后生效（影响存档）',
}

function showStatus(msg, type) {
  saveStatus.value = msg; statusType.value = type
  setTimeout(() => { saveStatus.value = '' }, 3000)
}

</script>

<template>
  <div class="app-window">
    <div class="titlebar" style="--wails-draggable:drag">
      <div class="titlebar-left">
        <span class="titlebar-title">GBFR 存档修改工具</span>
        <transition name="fade">
          <span v-if="saveStatus" class="titlebar-status" :class="statusType">
            {{ statusType === 'success' ? '●' : '✕' }} {{ saveStatus }}
          </span>
        </transition>
      </div>
      <div class="titlebar-controls" style="--wails-draggable:no-drag">
        <button class="win-btn minimize" @click="WindowMinimise" title="最小化">
          <svg width="10" height="1" viewBox="0 0 10 1"><rect width="10" height="1.5" rx="0.75" fill="currentColor"/></svg>
        </button>
        <button class="win-btn close" @click="Quit" title="关闭">
          <svg width="10" height="10" viewBox="0 0 10 10"><line x1="1" y1="1" x2="9" y2="9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/><line x1="9" y1="1" x2="1" y2="9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
        </button>
      </div>
    </div>

    <div class="tab-bar" style="--wails-draggable:no-drag">
      <button class="tab-btn" :class="{ active: activeTab === 'patch' }" @click="activeTab = 'patch'">
        补丁修改
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'sigil' }" @click="activeTab = 'sigil'">
        因子生成
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'wrightstone' }" @click="activeTab = 'wrightstone'">
        祝福生成
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'chara' }" @click="activeTab = 'chara'">
        角色次数统计
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'save' }" @click="activeTab = 'save'">
        副本次数
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'misc' }" @click="activeTab = 'misc'">
        杂项
      </button>
    </div>

    <main v-if="activeTab === 'patch'" class="container" style="--wails-draggable:no-drag">
      <div class="path-section">
        <div class="path-label">
          <span v-if="isDetecting">正在扫描 Steam 安装路径...</span>
          <span v-else-if="isLoaded" class="path-found">已定位游戏文件</span>
          <span v-else>输入 granblue_fantasy_relink.exe 路径</span>
        </div>
        <div class="path-input-row">
          <input v-model="manualPath" type="text" class="path-input"
            placeholder="粘贴 exe 文件完整路径..." @keyup.enter="applyManualPath" />
          <button class="btn-path-confirm" @click="applyManualPath" :disabled="!manualPath.trim()">确定</button>
        </div>
      </div>

      <transition name="slide-up">
        <div v-if="isLoaded" class="data-panel">
          <div v-if="state.exePath" class="path-bar">
            <span class="path-text" :title="state.exePath">{{ state.exePath }}</span>
            <span class="file-size">{{ (state.fileSize / 1024 / 1024).toFixed(1) }} MB</span>
          </div>

          <!-- 每个补丁点一张卡片 -->
          <div v-for="p in state.patches" :key="p.id" class="data-card"
            :style="{ background: (CARD_COLORS[p.id]||CARD_COLORS.mission).bg, boxShadow: '0 4px 20px '+(CARD_COLORS[p.id]||CARD_COLORS.mission).shadow }">
            <div class="card-header">
              <span class="card-label">{{ p.name }}</span>
              <span v-if="CARD_HINTS[p.id]" class="card-hint">{{ CARD_HINTS[p.id] }}</span>
              <span v-if="p.state==='original'" class="state-badge original">未补丁</span>
              <span v-else-if="p.state==='patched'" class="state-badge patched">已补丁</span>
              <span v-else class="state-badge unknown">未知</span>
            </div>
            <div v-if="p.state==='patched'" class="card-detail">
              当前值: {{ p.currentValue }} (0x{{ p.currentValue.toString(16).toUpperCase() }})
            </div>
            <div class="card-edit-row">
              <input v-model="patchValues[p.id]" type="number" min="0" class="edit-input" placeholder="输入数值" />
              <button class="btn-patch" @click="applyPatch(p.id)"
                :disabled="patchingID === p.id || !patchValues[p.id] || isNaN(parseInt(patchValues[p.id]))">
                {{ patchingID === p.id ? '写入中...' : '应用' }}
              </button>
            </div>
          </div>

          <!-- 备份/恢复 -->
          <div class="backup-section">
            <div class="backup-row">
              <button class="btn-secondary" @click="backup">备份</button>
              <button class="btn-secondary restore" @click="restore" :disabled="!state.backupExists">恢复</button>
            </div>
            <label class="force-label">
              <input type="checkbox" v-model="forceBackup" />
              <span>强制覆盖已有备份</span>
            </label>
            <div v-if="state.backupExists" class="backup-info">备份: {{ (state.backupSize / 1024 / 1024).toFixed(1) }} MB (仅 exe)</div>
          </div>
        </div>
      </transition>

            <transition name="fade">
        <div v-if="!isLoaded && !isDetecting" class="placeholder">
          <p>未自动检测到游戏，请手动输入 exe 路径</p>
          <p class="placeholder-tip">建议先备份原始文件再进行补丁操作</p>
        </div>
      </transition>
      <div class="footer-hint"><a href="https://github.com/BitterG/GBFR-PE-Patch-Tool" target="_blank" class="footer-link">github.com/BitterG/GBFR-PE-Patch-Tool</a></div>
    </main>

    <main v-else-if="activeTab === 'sigil'" class="container" style="--wails-draggable:no-drag">
      <SigilGenerator @status="showStatus" />
    </main>

    <main v-else-if="activeTab === 'wrightstone'" class="container" style="--wails-draggable:no-drag">
      <WrightstoneGenerator @status="showStatus" />
    </main>

    <main v-else-if="activeTab === 'chara'" class="container" style="--wails-draggable:no-drag">
      <CharaStats @status="showStatus" />
    </main>

    <main v-else-if="activeTab === 'save'" class="container" style="--wails-draggable:no-drag">
      <SaveEditor />
    </main>

    <main v-else-if="activeTab === 'misc'" class="container" style="--wails-draggable:no-drag">
      <MiscTools @status="showStatus" />
    </main>
  </div>
</template>

<style scoped>
.app-window { display:flex; flex-direction:column; height:100vh; overflow:hidden; background-color:rgba(27,38,54,1); border-radius:10px; box-shadow:0 8px 40px rgba(0,0,0,0.6); }

.tab-bar {
  display: flex;
  gap: 2px;
  padding: 4px 14px;
  background: rgba(18,26,38,0.95);
  border-bottom: 1px solid rgba(255,255,255,0.06);
  flex-shrink: 0;
}
.tab-btn {
  padding: 4px 14px;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: rgba(255,255,255,0.35);
  font-size: 0.76rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.tab-btn:hover { color: rgba(255,255,255,0.6); background: rgba(255,255,255,0.05); }
.tab-btn.active { color: #67e8f9; background: rgba(103,232,249,0.12); }
.titlebar { display:flex; align-items:center; justify-content:space-between; height:38px; padding:0 6px 0 14px; background:rgba(18,26,38,0.95); border-bottom:1px solid rgba(255,255,255,0.06); flex-shrink:0; user-select:none; }
.titlebar-left { display:flex; align-items:center; gap:8px; }
.titlebar-title { font-size:0.8rem; font-weight:600; color:rgba(255,255,255,0.55); letter-spacing:0.5px; }
.titlebar-controls { display:flex; align-items:center; gap:2px; }
.win-btn { width:32px; height:28px; border:none; border-radius:6px; background:transparent; color:rgba(255,255,255,0.45); cursor:pointer; display:flex; align-items:center; justify-content:center; transition:background 0.15s,color 0.15s; }
.win-btn.minimize:hover { background:rgba(255,255,255,0.1); color:rgba(255,255,255,0.9); }
.win-btn.close:hover { background:rgba(239,68,68,0.8); color:#fff; }
.titlebar-status { font-size:0.68rem; font-weight:600; padding:2px 8px; border-radius:20px; white-space:nowrap; }
.titlebar-status.success { color:#4ade80; background:rgba(34,197,94,0.15); }
.titlebar-status.error { color:#f87171; background:rgba(239,68,68,0.15); }

.container { flex:1; overflow-y:auto; max-width:720px; width:100%; margin:0 auto; padding:20px 20px 40px; box-sizing:border-box; display:flex; flex-direction:column; align-items:center; gap:14px; scrollbar-width:none; }
.container::-webkit-scrollbar { display:none; }

.path-section { width:100%; }
.path-label { font-size:0.78rem; color:rgba(255,255,255,0.4); margin-bottom:8px; }
.path-found { color:#4ade80; }
.path-input-row { display:flex; gap:8px; }
.path-input { flex:1; padding:10px 14px; border-radius:10px; border:1px solid rgba(255,255,255,0.15); background:rgba(255,255,255,0.07); color:#fff; font-size:0.85rem; font-family:'Courier New',monospace; outline:none; transition:border-color 0.2s; }
.path-input::placeholder { color:rgba(255,255,255,0.22); }
.path-input:focus { border-color:rgba(103,232,249,0.5); background:rgba(255,255,255,0.1); }
.btn-path-confirm { padding:10px 18px; border-radius:10px; border:1px solid rgba(103,232,249,0.3); background:rgba(103,232,249,0.1); color:#67e8f9; font-size:0.85rem; font-weight:600; cursor:pointer; transition:background 0.2s,transform 0.15s; }
.btn-path-confirm:not(:disabled):hover { background:rgba(103,232,249,0.2); transform:scale(1.02); }
.btn-path-confirm:disabled { opacity:0.4; cursor:not-allowed; }

.path-bar { width:100%; box-sizing:border-box; padding:8px 14px; border-radius:10px; background:rgba(255,255,255,0.05); border:1px solid rgba(255,255,255,0.08); display:flex; align-items:center; justify-content:space-between; gap:8px; }
.path-text { font-size:0.72rem; color:rgba(255,255,255,0.4); white-space:nowrap; overflow:hidden; text-overflow:ellipsis; font-family:'Courier New',monospace; flex:1; }
.file-size { font-size:0.68rem; color:rgba(255,255,255,0.3); flex-shrink:0; }

.data-panel { width:100%; display:flex; flex-direction:column; gap:12px; }
.data-card { border-radius:16px; padding:16px 18px; border:1px solid rgba(255,255,255,0.08); transition:transform 0.2s ease; display:flex; flex-direction:column; gap:8px; }
.data-card:hover { transform:translateY(-2px); }
.card-header { display:flex; align-items:center; justify-content:space-between; }
.card-label { font-size:0.88rem; font-weight:600; color:rgba(255,255,255,0.65); letter-spacing:1px; }
.card-hint { font-size:0.68rem; color:rgba(255,255,255,0.25); margin-left:4px; }
.card-detail { font-size:0.75rem; color:rgba(255,255,255,0.45); font-family:'Courier New',monospace; }

.state-badge { font-size:0.72rem; font-weight:700; padding:3px 10px; border-radius:20px; letter-spacing:0.5px; }
.state-badge.original { color:#67e8f9; background:rgba(103,232,249,0.15); }
.state-badge.patched { color:#4ade80; background:rgba(34,197,94,0.15); }
.state-badge.unknown { color:#fbbf24; background:rgba(251,191,36,0.15); }

.card-edit-row { display:flex; gap:8px; align-items:center; }
.edit-input { flex:1; padding:8px 14px; border-radius:8px; border:1px solid rgba(255,255,255,0.15); background:rgba(255,255,255,0.07); color:#fff; font-size:0.95rem; font-family:inherit; outline:none; transition:border-color 0.2s; }
.edit-input::placeholder { color:rgba(255,255,255,0.22); }
.edit-input:focus { border-color:rgba(255,255,255,0.4); background:rgba(255,255,255,0.12); }
.edit-input::-webkit-outer-spin-button, .edit-input::-webkit-inner-spin-button { -webkit-appearance:none; margin:0; }

.btn-patch { padding:8px 20px; border-radius:8px; border:1px solid rgba(165,180,252,0.3); background:rgba(165,180,252,0.1); color:#a5b4fc; font-size:0.85rem; font-weight:600; cursor:pointer; transition:background 0.2s,transform 0.15s; white-space:nowrap; }
.btn-patch:not(:disabled):hover { background:rgba(165,180,252,0.2); transform:scale(1.02); }
.btn-patch:disabled { opacity:0.4; cursor:not-allowed; }

.backup-section { padding:14px 18px; border-radius:16px; background:rgba(255,255,255,0.04); border:1px solid rgba(255,255,255,0.06); display:flex; flex-direction:column; gap:10px; }
.backup-row { display:flex; gap:10px; }
.btn-secondary { flex:1; padding:10px 0; border-radius:10px; border:1px solid rgba(255,255,255,0.12); background:rgba(40,48,64,0.8); color:rgba(255,255,255,0.6); font-size:0.85rem; font-weight:600; cursor:pointer; display:flex; align-items:center; justify-content:center; gap:6px; transition:color 0.2s,border-color 0.2s,transform 0.15s; }
.btn-secondary:not(:disabled):hover { color:#67e8f9; border-color:rgba(103,232,249,0.35); transform:scale(1.02); }
.btn-secondary.restore:not(:disabled):hover { color:#fbbf24; border-color:rgba(251,191,36,0.35); }
.btn-secondary:disabled { opacity:0.4; cursor:not-allowed; }
.force-label { display:flex; align-items:center; gap:6px; font-size:0.75rem; color:rgba(255,255,255,0.35); cursor:pointer; }
.force-label input[type="checkbox"] { accent-color:#67e8f9; }
.backup-info { font-size:0.72rem; color:rgba(255,255,255,0.3); font-family:'Courier New',monospace; }

.placeholder { margin-top:40px; color:rgba(255,255,255,0.25); text-align:center; font-size:0.88rem; line-height:1.8; }
.placeholder-tip { font-size:0.78rem; color:rgba(255,255,255,0.18); margin-top:8px; }

.footer-hint { width:100%; text-align:center; font-size:0.72rem; color:rgba(255,255,255,0.2); margin-top:auto; padding-top:16px; }
.footer-link { color:rgba(255,255,255,0.25); text-decoration:none; transition:color 0.2s; }
.footer-link:hover { color:rgba(103,232,249,0.6); }

.fade-enter-active, .fade-leave-active { transition:opacity 0.3s ease; }
.fade-enter-from, .fade-leave-to { opacity:0; }
.slide-up-enter-active { transition:all 0.4s cubic-bezier(0.25,0.46,0.45,0.94); }
.slide-up-enter-from { opacity:0; transform:translateY(24px); }

</style>
