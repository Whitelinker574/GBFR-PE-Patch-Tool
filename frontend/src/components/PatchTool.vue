<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import {
  AutoDetect, SetExePath, GetStatus, PatchFile, BackupFile, RestoreFile,
  GetAppVersion, CheckUpdate, OpenReleasePage,
} from '../../wailsjs/go/main/App'
import { WindowMinimise, Quit } from '../../wailsjs/runtime/runtime'
import StarfieldCanvas from './StarfieldCanvas.vue'
import HomeJournal from './HomeJournal.vue'
import SaveBackupDrawer from './SaveBackupDrawer.vue'
import { language, translateText } from '../i18n'
import progressionArt from '../assets/gbfr/cutouts/progression-official-edge-safe.webp'
import sigilArt from '../assets/gbfr/cutouts/sigil-official-edge-safe.webp'
import sigilMemoryArt from '../assets/gbfr/cutouts/sigil-memory-official-edge-safe.webp'
import loadoutArt from '../assets/gbfr/cutouts/loadout-official-edge-safe.webp'
import wrightstoneArt from '../assets/gbfr/cutouts/wrightstone-official-edge-safe.webp'
import summonArt from '../assets/gbfr/cutouts/summon-official-edge-safe.webp'
import overlimitArt from '../assets/gbfr/cutouts/overlimit-official-edge-safe.webp'
import runtimeArt from '../assets/gbfr/cutouts/runtime-official-edge-safe.webp'
import charaArt from '../assets/gbfr/cutouts/chara-official-edge-safe.webp'
import saveArt from '../assets/gbfr/cutouts/save-official-edge-safe.webp'
import compatibilityArt from '../assets/gbfr/cutouts/compatibility-official-edge-safe.webp'
import legacyRuntimeArt from '../assets/gbfr/cutouts/legacy-runtime-official-edge-safe.webp'
import monsterArt from '../assets/gbfr/cutouts/monster-official-edge-safe.webp'
import patchArt from '../assets/gbfr/cutouts/patch-official-edge-safe.webp'
import languageArt from '../assets/gbfr/cutouts/language-official-edge-safe.webp'
import progressionSticker from '../assets/gbfr/stickers/progression.webp'
import sigilSticker from '../assets/gbfr/stickers/sigil.webp'
import sigilMemorySticker from '../assets/gbfr/stickers/sigil-memory.webp'
import loadoutSticker from '../assets/gbfr/stickers/loadout.webp'
import wrightstoneSticker from '../assets/gbfr/stickers/wrightstone.webp'
import summonSticker from '../assets/gbfr/stickers/summon.webp'
import overlimitSticker from '../assets/gbfr/stickers/overlimit.webp'
import runtimeSticker from '../assets/gbfr/stickers/runtime.webp'
import charaSticker from '../assets/gbfr/stickers/chara.webp'
import saveSticker from '../assets/gbfr/stickers/save.webp'
import compatibilitySticker from '../assets/gbfr/stickers/compatibility.webp'
import legacyRuntimeSticker from '../assets/gbfr/stickers/legacy-runtime-blonde.webp'
import monsterSticker from '../assets/gbfr/stickers/monster.webp'
import patchSticker from '../assets/gbfr/stickers/patch.webp'
import languageSticker from '../assets/gbfr/stickers/language.webp'

const componentLoaders = {
  progression: () => import('./ProgressionEditor.vue'),
  sigil: () => import('./SigilGenerator.vue'),
  sigilMemory: () => import('./SigilMemoryGenerator.vue'),
  loadout: () => import('./SigilLoadoutRestore.vue'),
  wrightstone: () => import('./WrightstoneGenerator.vue'),
  summon: () => import('./SummonEditor.vue'),
  overlimit: () => import('./OverLimit.vue'),
  runtime: () => import('./MiscTools.vue'),
  legacyRuntime: () => import('./MiscTools.vue'),
  chara: () => import('./CharaStats.vue'),
  save: () => import('./SaveEditor.vue'),
  monster: () => import('./MonsterEnhance.vue'),
  language: () => import('./LanguageSettings.vue'),
}
// 桌面本地应用无网络加载成本，改用静态直引：全部组件打进主包，
// 切页时立绘与内容同帧渲染，不再出现“先出图、内容后到”的等待感。
// componentLoaders / warmTool 仍保留（预热已打包模块，无副作用），便于将来若需分包回退。
import ProgressionEditor from './ProgressionEditor.vue'
import SigilGenerator from './SigilGenerator.vue'
import SigilMemoryGenerator from './SigilMemoryGenerator.vue'
import SigilLoadoutRestore from './SigilLoadoutRestore.vue'
import WrightstoneGenerator from './WrightstoneGenerator.vue'
import SummonEditor from './SummonEditor.vue'
import OverLimit from './OverLimit.vue'
import MiscTools from './MiscTools.vue'
import CharaStats from './CharaStats.vue'
import SaveEditor from './SaveEditor.vue'
import MonsterEnhance from './MonsterEnhance.vue'
import LanguageSettings from './LanguageSettings.vue'

const state = reactive({
  exePath: '',
  fileExists: false,
  fileSize: 0,
  backupExists: false,
  backupSize: 0,
  patches: [],
})

const activeTab = ref('home')
const sidebarCollapsed = ref(window.localStorage.getItem('gbfr.sidebarCollapsed') === '1')
const artCollapsed = ref(window.localStorage.getItem('gbfr.artCollapsed') === '1')
const manualPath = ref('')
const patchValues = reactive({})
const isLoaded = ref(false)
const isDetecting = ref(false)
const patchingID = ref('')
const forceBackup = ref(false)
const saveStatus = ref('')
const statusType = ref('')
const updateLoading = ref(false)
const updateInfo = reactive({ currentVersion: '—', latestVersion: '', hasUpdate: false, releaseUrl: '', body: '' })
let hasAttemptedGameDetection = false

const toolMeta = {
  home: {
    group: 'home', title: '首页', eyebrow: '功能入口', status: 'DLC 2.0.2', tone: 'stable',
    description: '从目标出发选择功能，常用养成、实时工具和记录编辑都从这里进入。',
    usage: [], caution: '',
  },
  progression: {
    group: 'workshop', title: '物品与武器（存档修改）', eyebrow: '离线养成', status: '已适配 2.0.2', tone: 'stable',
    description: '统一处理物品、素材、武器等级与养成资源，适合大批量、可回滚的存档修改。',
    usage: ['完全退出游戏', '选择存档并确认空位', '写入后使用自动备份验证'],
    caution: '不要在游戏运行时编辑同一份存档。',
    speaker: '卡莉奥斯特罗', note: '先留好备份，再把素材和武器整理得漂漂亮亮——这才像完美的炼金术嘛。',
  },
  sigil: {
    group: 'workshop', title: '因子修改（存档修改）', eyebrow: '离线存档', status: '稳定', tone: 'stable',
    description: '生成、批量管理和删除存档内因子，适合一次性整理较多因子。',
    usage: ['退出游戏并加载存档', '配置因子与词条', '先检查合法性再写入'],
    caution: '不合法组合会提醒，但不会替你改变选择。',
    speaker: '娜露梅亚', note: '先检验组合，再写入存档。稳稳完成每一步，理想的因子就不会跑掉。',
  },
  sigilMemory: {
    group: 'workshop', title: '因子即时编辑', eyebrow: '游戏内养成', status: '实时', tone: 'live',
    description: '直接修改游戏中当前选中的因子，适合少量精确调整和反复试配。',
    usage: ['启动游戏并启用读取', '在游戏中选中目标因子', '刷新、核对后写入'],
    caution: '重新进档或因子列表刷新后，请重新选择目标。',
    speaker: '萝赛塔', note: '游戏重新载入后，记得再选一次目标。旧的指针可不会一直等你哦。',
  },
  loadout: {
    group: 'workshop', title: '因子配装导入/导出', eyebrow: '游戏内配装', status: '实时', tone: 'live',
    description: '记录角色当前的 12 个因子并导出分享，也可把配装文件逐项复刻到备用因子。',
    usage: ['启动游戏并按角色筛选因子', '从第一项开始记录或复刻', '逐项向下移动，不要快速滚动'],
    caution: '复刻会改写当前选中的备用因子；不要使用已经装备或需要保留的因子。',
    speaker: '芙劳', note: '把十二个因子的顺序先理清，再一步一步复刻。速度不必太快，准确才最重要。',
  },
  wrightstone: {
    group: 'workshop', title: '祝福修改（存档修改）', eyebrow: '离线存档', status: '稳定', tone: 'stable',
    description: '集中生成祝福与三条词条，使用与因子批量修改一致的存档工作流。',
    usage: ['退出游戏并加载存档', '选择祝福和三条词条', '校验队列并应用'],
    caution: '等级上限与组合合法性会在写入前提示。',
    speaker: '菲莉', note: '三条词条都确认好再应用，幽灵朋友们也会替你看着。',
  },
  summon: {
    group: 'workshop', title: '召唤石修改', eyebrow: '游戏内修改', status: '实时保存', tone: 'live',
    description: '读取召唤石背包并修改因子、副参数和等级，写入时调用游戏保存流程。',
    usage: ['打开游戏内召唤石背包', '连接并选择一颗召唤石', '核对稀有度与合法性后写入'],
    caution: '当前不支持安全更换召唤石种类。',
    speaker: '露莉亚', note: '先在背包里选中目标召唤石，再核对稀有度和等级，我们一起慢慢来。',
  },
  overlimit: {
    group: 'workshop', title: '角色上限突破', eyebrow: '游戏内修改', status: '流程型', tone: 'live',
    description: '读取角色突破界面的四个能力槽，按游戏原流程保存结果。',
    usage: ['先完成一次 3 级突破', '停在结果界面后刷新', '修改四项并按说明保存'],
    caution: '必须按页面步骤完成，不能跳过游戏内确认流程。',
    speaker: '希耶提', note: '四个能力槽一个都别漏。真正的剑王，可不会跳过确认步骤。',
  },
  runtime: {
    group: 'workshop', title: '游戏内实时修改', eyebrow: '金币、素材与掉落', status: '需连接游戏', tone: 'live',
    description: '集中管理货币、药水、素材消耗和任务掉落等运行时功能。',
    usage: ['先启动并进入游戏存档', '连接游戏进程', '按资源或任务分类切换功能'],
    caution: '重启游戏后运行时设置会失效，需要重新连接。',
    speaker: '碧', note: '进游戏、连进程、再修改！重启以后可得重新连接，别忘啦！',
  },
  chara: {
    group: 'records', title: '角色使用次数', eyebrow: '记录与统计', status: '离线存档', tone: 'stable',
    description: '查看所有角色的使用次数，可任意选择多个角色批量修改。',
    usage: ['完全退出游戏', '选择存档和目标角色', '填入次数后保存已选'],
    caution: '只修改勾选角色，保存前请检查选择数量。',
    speaker: '姬塔', note: '只会保存你勾选的角色。动手前再数一遍，团长的记录要清清楚楚。',
  },
  save: {
    group: 'records', title: '任务完成次数', eyebrow: '记录与统计', status: '离线存档', tone: 'stable',
    description: '搜索任务并批量修改完成次数，保留未选任务的原始数据。',
    usage: ['完全退出游戏', '搜索并勾选任务', '批量填入后保存已选'],
    caution: '每次保存都会创建时间戳备份并回读。',
    speaker: '拉卡姆', note: '任务记录就像航线图，先选准目标，再一次写入，别改错方向。',
  },
  compatibility: {
    group: 'compatibility', title: '版本适配', eyebrow: '版本检测与功能状态', status: 'DLC 2.0.2', tone: 'calibrate',
    description: '在一个位置查看工具版本、游戏文件和功能适配状态，避免把兼容问题混进日常操作。',
    usage: ['检查工具更新', '确认游戏文件已识别', '按适配状态选择功能'],
    caution: '标记为“等待适配”的功能不会自动启用。',
    speaker: '罗兰', note: '先看工具版本、游戏文件和适配状态。修东西之前，总得弄清哪里不对。',
  },
  legacyRuntime: {
    group: 'compatibility', title: '未适配的实时功能', eyebrow: '暂不建议使用', status: '等待作者更新', tone: 'waiting',
    description: '保留旧版运行时功能和诊断入口，便于后续校准，不再与稳定功能混排。',
    usage: ['先阅读每项兼容说明', '连接后仅执行扫描或刷新', '字节不匹配时立即停止'],
    caution: '这些功能未完成 DLC 2.0.2 全流程验证，默认不建议写入。',
    speaker: '泽塔', note: '地址或字节对不上就立刻停手。勇敢可不等于拿存档去冒险。',
  },
  monster: {
    group: 'compatibility', title: '怪物倍率与伤害记录', eyebrow: '未适配功能', status: '等待适配', tone: 'waiting',
    description: '保留怪物倍率、霸体和伤害记录相关实现，等待新版偏移与联机流程复核。',
    usage: ['仅在主机端测试', '先刷新并检查地址状态', '告知队友后再启用'],
    caution: '当前版本不保证可用，异常时不要反复开启。',
    speaker: '伊德', note: '先确认主机端和倍率，再动手。力量失控的话，记录也会失去意义。',
  },
  patch: {
    group: 'compatibility', title: '游戏文件维护', eyebrow: '备份、恢复与兼容诊断', status: '恢复可用', tone: 'waiting',
    description: '识别游戏 EXE、创建原始文件备份并一键恢复；旧二进制修改仅保留为兼容诊断。',
    usage: ['定位游戏 EXE', '先创建原始备份', '只在字节状态明确时应用'],
    caution: 'DLC 2.0.2 未重新标定，不建议应用写入。',
    speaker: '欧根', note: '原始文件先备份，字节状态看清楚再修。老手从不省这一步。',
  },
  language: {
    group: 'settings', title: '语言与显示', eyebrow: '应用设置', status: '本机设置', tone: 'neutral',
    description: '选择界面语言。切换后会刷新应用，让所有功能使用同一语言。',
    usage: ['选择语言', '等待应用刷新', '返回上次使用的功能'],
    caution: '语言设置只保存在本机。',
    speaker: '伊欧', note: '选好语言后等界面刷新，别急着连点。清清楚楚才最好用嘛！',
  },
}

const navigation = computed(() => [
  { id: 'workshop', mark: '改', label: '养成与实时修改', caption: '物品、因子、召唤石、资源', items: ['progression', 'sigilMemory', 'loadout', 'summon', 'overlimit', 'runtime', 'sigil', 'wrightstone'] },
  { id: 'records', mark: '数', label: '次数统计', caption: '角色与任务次数', items: ['chara', 'save'] },
  { id: 'compatibility', mark: '版', label: '版本适配', caption: '检测与未适配功能', items: ['compatibility', 'legacyRuntime', 'monster', 'patch'] },
  { id: 'settings', mark: '设', label: language.value === 'zh' ? '设置' : 'Settings', caption: language.value === 'zh' ? '语言与应用偏好' : 'Language and preferences', items: ['language'] },
])

const currentMeta = computed(() => toolMeta[activeTab.value] || toolMeta.home)
const functionArt = {
  progression: progressionArt,
  sigil: sigilArt,
  sigilMemory: sigilMemoryArt,
  loadout: loadoutArt,
  wrightstone: wrightstoneArt,
  summon: summonArt,
  overlimit: overlimitArt,
  runtime: runtimeArt,
  chara: charaArt,
  save: saveArt,
  compatibility: compatibilityArt,
  legacyRuntime: legacyRuntimeArt,
  monster: monsterArt,
  patch: patchArt,
  language: languageArt,
}
const currentArt = computed(() => functionArt[activeTab.value] || progressionArt)
const functionStickers = {
  progression: progressionSticker,
  sigil: sigilSticker,
  sigilMemory: sigilMemorySticker,
  loadout: loadoutSticker,
  wrightstone: wrightstoneSticker,
  summon: summonSticker,
  overlimit: overlimitSticker,
  runtime: runtimeSticker,
  chara: charaSticker,
  save: saveSticker,
  compatibility: compatibilitySticker,
  legacyRuntime: legacyRuntimeSticker,
  monster: monsterSticker,
  patch: patchSticker,
  language: languageSticker,
}
const currentSticker = computed(() => functionStickers[activeTab.value] || progressionSticker)
const warmedTools = new Set()
const warmedImages = new Map()
const warmQueue = []
let warmTimer = 0

function warmImage(src) {
  if (!src || warmedImages.has(src)) return warmedImages.get(src)
  const image = new Image()
  image.decoding = 'async'
  image.src = src
  const pending = typeof image.decode === 'function'
    ? image.decode().catch(() => undefined)
    : new Promise(resolve => { image.onload = image.onerror = resolve })
  warmedImages.set(src, pending)
  return pending
}

function warmTool(id) {
  if (!id || warmedTools.has(id)) return
  warmedTools.add(id)
  componentLoaders[id]?.().catch(() => warmedTools.delete(id))
  warmImage(functionArt[id])
  warmImage(functionStickers[id])
}

function drainWarmQueue() {
  window.clearTimeout(warmTimer)
  warmTimer = 0
  const id = warmQueue.shift()
  if (!id) return
  warmTool(id)
  warmTimer = window.setTimeout(drainWarmQueue, 90)
}

function queueWarmTools(ids = []) {
  for (const id of ids) {
    if (!warmedTools.has(id) && !warmQueue.includes(id)) warmQueue.push(id)
  }
  if (!warmTimer) drainWarmQueue()
}

function warmGroup(group) {
  if (!group?.items?.length) return
  warmTool(group.items[0])
  queueWarmTools(group.items.slice(1))
}

const activeGroup = computed(() => navigation.value.find(group => group.id === currentMeta.value.group) || navigation.value[0])
function selectGroup(group) {
  warmGroup(group)
  if (!group.items.includes(activeTab.value)) activeTab.value = group.items[0]
  if (group.id === 'compatibility') ensureGameDetection()
}

function selectTool(id) {
  warmTool(id)
  activeTab.value = id
  if (toolMeta[id]?.group === 'compatibility') ensureGameDetection()
}

function toggleArt() {
  artCollapsed.value = !artCollapsed.value
  window.localStorage.setItem('gbfr.artCollapsed', artCollapsed.value ? '1' : '0')
}
function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  window.localStorage.setItem('gbfr.sidebarCollapsed', sidebarCollapsed.value ? '1' : '0')
}

onMounted(() => {
  GetAppVersion().then(v => { updateInfo.currentVersion = v }).catch(() => {})
  window.setTimeout(() => warmTool('progression'), 60)
  const warmWorkshop = () => queueWarmTools((navigation.value[0]?.items || []).slice(1))
  if ('requestIdleCallback' in window) window.requestIdleCallback(warmWorkshop, { timeout: 800 })
  else window.setTimeout(warmWorkshop, 180)
  // Keep first paint light, then fill the remaining illustration/component cache
  // sequentially so the first visit to Settings or Compatibility does not flash.
  window.setTimeout(() => queueWarmTools(Object.keys(componentLoaders)), 1100)
})

function ensureGameDetection() {
  if (hasAttemptedGameDetection || isDetecting.value) return
  hasAttemptedGameDetection = true
  isDetecting.value = true
  AutoDetect()
    .then((path) => {
      isDetecting.value = false
      if (path) {
        state.exePath = path
        manualPath.value = path
        return loadFile(path, false)
      }
    })
    .catch(() => { isDetecting.value = false })
}

function syncPatchValues(info) {
  ;(info.patches || []).forEach(patch => {
    if (patch.state === 'patched') patchValues[patch.id] = String(patch.currentValue)
    else if (!patchValues[patch.id]) patchValues[patch.id] = ''
  })
}

function loadFile(path, notify = true) {
  return GetStatus(path).then((info) => {
    Object.assign(state, info)
    syncPatchValues(info)
    isLoaded.value = true
    if (notify) showStatus('游戏文件识别成功', 'success')
  })
}

function applyManualPath() {
  const path = manualPath.value.trim()
  if (!path) { showStatus('请输入文件路径', 'error'); return }
  SetExePath(path)
    .then((info) => {
      Object.assign(state, info)
      syncPatchValues(info)
      isLoaded.value = true
      showStatus('游戏文件识别成功', 'success')
    })
    .catch((err) => showStatus(String(err), 'error'))
}

function refreshStatus() {
  if (!state.exePath) return Promise.resolve()
  return GetStatus(state.exePath).then((info) => {
    Object.assign(state, info)
    syncPatchValues(info)
  })
}

function applyPatch(patchID) {
  const value = parseInt(patchValues[patchID])
  if (Number.isNaN(value) || value < 0) { showStatus('请输入有效数值', 'error'); return }
  patchingID.value = patchID
  PatchFile(patchID, value)
    .then(() => refreshStatus())
    .then(() => showStatus('补丁写入成功', 'success'))
    .catch((err) => showStatus('补丁失败: ' + (err || '未知错误'), 'error'))
    .finally(() => { patchingID.value = '' })
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

function checkUpdate() {
  updateLoading.value = true
  CheckUpdate()
    .then((info) => {
      Object.assign(updateInfo, info)
      showStatus(info.hasUpdate ? `发现新版本 ${info.latestVersion}` : '当前已是最新版本', 'success')
    })
    .catch((err) => showStatus(String(err), 'error'))
    .finally(() => { updateLoading.value = false })
}

function openReleasePage() {
  OpenReleasePage(updateInfo.releaseUrl || '').catch((err) => showStatus(String(err), 'error'))
}

let statusTimer = 0
function showStatus(message, type) {
  window.clearTimeout(statusTimer)
  saveStatus.value = translateText(String(message))
  statusType.value = type
  statusTimer = window.setTimeout(() => { saveStatus.value = '' }, 3600)
}
</script>

<template>
  <div class="app-window">
    <StarfieldCanvas />
    <div class="sky-haze" aria-hidden="true"></div>
    <header class="titlebar" style="--wails-draggable:drag">
      <div class="titlebar-brand">
        <span class="brand-glyph">✦</span>
        <span class="titlebar-title">GBFR 存档修改工具</span>
        <span class="build-chip">DLC 2.0.2</span>
      </div>
      <transition name="toast">
        <div v-if="saveStatus" class="titlebar-status" :class="statusType">
          <span class="status-light"></span>{{ saveStatus }}
        </div>
      </transition>
      <div class="titlebar-controls" style="--wails-draggable:no-drag">
        <button class="win-btn" @click="WindowMinimise" title="最小化" aria-label="最小化"><span class="minimize-line"></span></button>
        <button class="win-btn close" @click="Quit" title="关闭" aria-label="关闭"><span class="close-lines"></span></button>
      </div>
    </header>

    <div class="app-body" :class="{ 'home-mode': activeTab === 'home', 'sidebar-collapsed': sidebarCollapsed }" style="--wails-draggable:no-drag">
      <aside class="sidebar">
        <button class="sidebar-collapse" :title="sidebarCollapsed ? '展开目录' : '收起目录'" :aria-label="sidebarCollapsed ? '展开目录' : '收起目录'" @click="toggleSidebar">{{ sidebarCollapsed ? '›' : '‹' }}</button>
        <div class="sidebar-heading" role="button" tabindex="0" title="返回功能首页" @click="selectTool('home')" @keyup.enter="selectTool('home')">
          <span class="sidebar-kicker">GBFR PE PATCH TOOL</span>
          <strong>GBFR 存档修改工具</strong>
          <span>碧蓝幻想 Relink 养成工具集</span>
        </div>
        <nav class="primary-nav" aria-label="主要功能">
          <button
            v-for="group in navigation"
            :key="group.id"
            class="nav-item"
            :class="{ active: activeGroup.id === group.id }"
            :title="`${group.label} · ${group.caption}`"
            @pointerenter="warmGroup(group)"
            @focus="warmGroup(group)"
            @click="selectGroup(group)"
          >
            <span class="nav-mark">{{ group.mark }}</span>
            <span class="nav-copy"><strong>{{ group.label }}</strong><small>{{ group.caption }}</small></span>
            <span class="nav-arrow">›</span>
          </button>
        </nav>
        <div class="sidebar-foot">
          <div class="target-row"><span class="target-dot"></span><div><strong>当前游戏版本</strong><small>Relink DLC 2.0.2</small></div></div>
          <a href="https://github.com/BitterG/GBFR-PE-Patch-Tool" target="_blank">项目仓库 ↗</a>
        </div>
      </aside>

      <section class="workspace">
        <div v-if="activeTab !== 'home'" class="workspace-bar">
            <div class="breadcrumb"><span>{{ activeGroup.label }}</span><b>/</b><strong>{{ currentMeta.title }}</strong></div>
            <div class="workspace-actions">
              <div class="workspace-state"><span :class="['state-dot', currentMeta.tone]"></span>{{ currentMeta.status }}</div>
              <SaveBackupDrawer @status="showStatus" />
            </div>
        </div>

        <nav v-if="activeTab !== 'home' && activeGroup.items.length > 1" class="tool-switcher" aria-label="同类功能切换">
            <button
              v-for="id in activeGroup.items"
              :key="id"
              :class="{ active: activeTab === id, waiting: toolMeta[id].tone === 'waiting' }"
              :title="`${toolMeta[id].title} · ${toolMeta[id].eyebrow}${toolMeta[id].tone === 'live' ? ' · 需先启动游戏并连接进程' : toolMeta[id].tone === 'stable' ? ' · 需先完全退出游戏' : ''}`"
              @pointerenter="warmTool(id)"
              @focus="warmTool(id)"
              @click="selectTool(id)"
            >
              {{ toolMeta[id].title.replace(/（[^）]*）/g, '') }}
              <span v-if="toolMeta[id].tone === 'live'" class="switcher-tag live">实时</span>
              <span v-else-if="toolMeta[id].tone === 'stable'" class="switcher-tag offline">离线</span>
              <span v-if="toolMeta[id].tone === 'waiting'" class="switcher-dot"></span>
            </button>
        </nav>

        <div class="workspace-scroll" :class="{ 'tool-workspace': activeTab !== 'home' }">
          <div class="workspace-scene">
          <HomeJournal v-if="activeTab === 'home'" key="home" :version="updateInfo.currentVersion" @warm="warmTool" @open="selectTool" />

          <section v-else :key="activeTab" class="tool-stage" :class="{ 'art-collapsed': artCollapsed }" :data-tool="activeTab">
            <aside class="guide-rail">
              <div class="guide-heading"><span>操作指南</span><small>始终显示</small></div>
              <ol class="guide-steps">
                <li v-for="(step, index) in currentMeta.usage" :key="step"><b>{{ index + 1 }}</b><span>{{ step }}</span></li>
              </ol>
              <div class="guide-caution"><span>使用前确认</span><p>{{ currentMeta.caution }}</p></div>
              <aside class="guide-character-note">
                <div class="note-bubble"><b>{{ currentMeta.speaker }}的建议</b><p>{{ currentMeta.note }}</p></div>
                <img class="guide-sticker" :src="currentSticker" :alt="`${currentMeta.speaker}表情贴纸`" loading="eager" decoding="async">
              </aside>
            </aside>

            <section class="tool-center-scroll">
              <header class="tool-page-heading">
                <div class="eyebrow">{{ currentMeta.eyebrow }}</div>
                <h1>{{ currentMeta.title }}</h1>
                <p>{{ currentMeta.description }}</p>
              </header>

              <main class="tool-panel" :data-tool="activeTab">
            <ProgressionEditor v-if="activeTab === 'progression'" @status="showStatus" />
            <SigilGenerator v-else-if="activeTab === 'sigil'" @status="showStatus" />
            <SigilMemoryGenerator v-else-if="activeTab === 'sigilMemory'" @status="showStatus" />
            <SigilLoadoutRestore v-else-if="activeTab === 'loadout'" @status="showStatus" />
            <WrightstoneGenerator v-else-if="activeTab === 'wrightstone'" @status="showStatus" />
            <SummonEditor v-else-if="activeTab === 'summon'" @status="showStatus" />
            <OverLimit v-else-if="activeTab === 'overlimit'" @status="showStatus" />
            <MiscTools v-else-if="activeTab === 'runtime'" mode="stable" @status="showStatus" />
            <CharaStats v-else-if="activeTab === 'chara'" @status="showStatus" />
            <SaveEditor v-else-if="activeTab === 'save'" @status="showStatus" />
            <MiscTools v-else-if="activeTab === 'legacyRuntime'" mode="compatibility" @status="showStatus" />
            <MonsterEnhance v-else-if="activeTab === 'monster'" @status="showStatus" />
            <LanguageSettings v-else-if="activeTab === 'language'" />

            <div v-else-if="activeTab === 'compatibility'" class="compat-dashboard">
              <section class="calibration-grid">
                <article class="calibration-card primary-card">
                  <div class="card-kicker">工具版本</div>
                  <strong>{{ updateInfo.currentVersion }}</strong>
                  <p>{{ updateInfo.latestVersion ? `社区最新 ${updateInfo.latestVersion}` : '尚未检查社区 Release' }}</p>
                  <div class="card-actions">
                    <button class="action primary" @click="checkUpdate" :disabled="updateLoading">{{ updateLoading ? '检查中…' : '检查更新' }}</button>
                    <button class="action" @click="openReleasePage">打开 Release</button>
                  </div>
                </article>
                <article class="calibration-card">
                  <div class="card-kicker">游戏文件</div>
                  <strong>{{ isDetecting ? '检测中' : isLoaded ? '已识别' : '未识别' }}</strong>
                  <p :title="state.exePath">{{ state.exePath || '未找到 granblue_fantasy_relink.exe' }}</p>
                  <span class="file-meta">{{ state.fileSize ? `${(state.fileSize / 1024 / 1024).toFixed(1)} MB` : '可在旧版文件补丁页手动选择' }}</span>
                </article>
                <article class="calibration-card">
                  <div class="card-kicker">校准目标</div>
                  <strong>DLC 2.0.2</strong>
                  <p>实时货币与素材指令已按当前版本特征校验。</p>
                  <span class="file-meta">未知字节会拒绝写入</span>
                </article>
              </section>

              <section class="compat-section">
                <div class="compat-heading"><div><span>功能状态</span><h2>按验证程度分层</h2></div><p>已适配功能可以日常使用；旧实现集中放在未适配区域。</p></div>
                <div class="matrix">
                  <div class="matrix-row head"><span>范围</span><span>状态</span><span>说明</span></div>
                  <div class="matrix-row"><span>物品、武器、因子、祝福</span><b class="ok">已适配</b><span>离线存档路径，自动备份与回读</span></div>
                  <div class="matrix-row"><span>货币、药水、素材消耗、巴武掉落</span><b class="ok">已适配</b><span>运行时连接，地址或字节不符即停止</span></div>
                  <div class="matrix-row"><span>召唤石、上限突破、即时因子</span><b class="flow">流程验证</b><span>需停留在指定游戏界面后操作</span></div>
                  <div class="matrix-row"><span>怪物增强与旧版运行时补丁</span><b class="wait">等待适配</b><span>保留实现和诊断，不进入稳定工具区</span></div>
                </div>
              </section>

              <section class="compat-section legacy-links">
                <div class="compat-heading"><div><span>未适配功能</span><h2>保留入口，避免误操作</h2></div></div>
                <button @click="selectTool('legacyRuntime')"><strong>待适配运行时功能</strong><small>倒计时、无限挑战、称号、皮肤符文等</small><span>查看 ›</span></button>
                <button @click="selectTool('monster')"><strong>怪物增强</strong><small>倍率、霸体、OD 与团队伤害记录</small><span>查看 ›</span></button>
                <button @click="selectTool('patch')"><strong>游戏文件维护</strong><small>EXE 识别、原始备份、恢复与兼容诊断</small><span>查看 ›</span></button>
              </section>
            </div>

            <div v-else-if="activeTab === 'patch'" class="legacy-patch">
              <div class="legacy-warning"><strong>等待 DLC 2.0.2 重新标定</strong><span>入口被保留用于诊断和恢复；当前不要应用未知状态的补丁。</span></div>
              <section class="path-card">
                <label>{{ isDetecting ? '正在扫描 Steam 安装路径…' : isLoaded ? '已定位游戏文件' : '游戏 EXE 路径' }}</label>
                <div class="path-input-row"><input v-model="manualPath" placeholder="粘贴 granblue_fantasy_relink.exe 完整路径" @keyup.enter="applyManualPath"><button class="action primary" @click="applyManualPath" :disabled="!manualPath.trim()">识别文件</button></div>
                <div v-if="state.exePath" class="detected-file"><span :title="state.exePath">{{ state.exePath }}</span><b>{{ (state.fileSize / 1024 / 1024).toFixed(1) }} MB</b></div>
              </section>
              <section v-if="isLoaded" class="patch-grid">
                <article v-for="patch in state.patches" :key="patch.id" class="patch-card">
                  <header><div><strong>{{ patch.name }}</strong><small>旧版二进制补丁</small></div><span :class="['patch-state', patch.state]">{{ patch.state === 'original' ? '原始' : patch.state === 'patched' ? '已补丁' : '未知' }}</span></header>
                  <p v-if="patch.state === 'patched'">当前值 {{ patch.currentValue }} · 0x{{ patch.currentValue.toString(16).toUpperCase() }}</p>
                  <div class="patch-edit"><input v-model="patchValues[patch.id]" type="number" min="0" placeholder="输入数值"><button class="action" @click="applyPatch(patch.id)" :disabled="patchingID === patch.id || patch.state === 'unknown'">{{ patchingID === patch.id ? '写入中…' : '应用' }}</button></div>
                </article>
              </section>
              <section class="backup-card"><div><strong>EXE 备份与恢复</strong><span>{{ state.backupExists ? `已有 ${(state.backupSize / 1024 / 1024).toFixed(1)} MB 备份` : '尚未创建备份' }}</span></div><div class="backup-policy" role="group" aria-label="备份策略"><button type="button" :class="{ active: !forceBackup }" @click="forceBackup=false"><b>保留现有备份</b><small>推荐</small></button><button type="button" :class="{ active: forceBackup }" @click="forceBackup=true"><b>重新创建原始备份</b><small>会替换旧备份</small></button></div><button class="action" @click="backup">创建备份</button><button class="action" @click="restore" :disabled="!state.backupExists">恢复备份</button></section>
            </div>
              </main>
            </section>

            <button class="art-toggle" :class="{ 'is-collapsed': artCollapsed }" :title="artCollapsed ? '展开立绘' : '收起立绘 · 拓宽操作区'" :aria-label="artCollapsed ? '展开立绘' : '收起立绘'" @click="toggleArt">{{ artCollapsed ? '‹' : '›' }}</button>
            <aside class="art-rail" aria-hidden="true">
              <figure class="function-character" :key="`art-${activeTab}`">
                <img class="character-blend" :src="currentArt" alt="" loading="eager" decoding="async">
                <img class="character-main" :src="currentArt" :alt="`${currentMeta.title}角色立绘`" loading="eager" decoding="async">
              </figure>
              <div class="art-caption"><span>{{ currentMeta.speaker }}</span><small>{{ currentMeta.eyebrow }}</small></div>
            </aside>
          </section>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.app-window {
  --bg:#071019; --panel:#0c1722; --panel-2:#101e2b; --line:rgba(174,204,224,.12);
  --text:#e7edf2; --muted:#7f929f; --cyan:#62d5e7; --green:#63d6a0; --amber:#e6b96b; --red:#ec7f78;
  position:relative; height:100vh; overflow:hidden; color:var(--text); background:var(--bg);
  font-family:"Microsoft YaHei UI","Noto Sans SC",sans-serif;
}
.app-window::before { content:""; position:absolute; inset:38px 0 0; pointer-events:none; opacity:.16; background-image:linear-gradient(rgba(99,213,231,.08) 1px,transparent 1px),linear-gradient(90deg,rgba(99,213,231,.06) 1px,transparent 1px); background-size:40px 40px; mask-image:linear-gradient(to bottom,black,transparent 75%); }
button,input,select { font:inherit; }
button:focus-visible,input:focus-visible,select:focus-visible { outline:2px solid rgba(98,213,231,.65); outline-offset:2px; }
.titlebar { position:relative; z-index:10; height:38px; display:grid; grid-template-columns:1fr auto 1fr; align-items:center; border-bottom:1px solid var(--line); background:#09131d; user-select:none; }
.titlebar-brand { display:flex; align-items:center; gap:9px; min-width:0; padding-left:12px; }
.brand-glyph { width:20px; height:20px; display:grid; place-items:center; border:1px solid rgba(98,213,231,.48); color:var(--cyan); font:700 11px Bahnschrift,"Segoe UI",sans-serif; transform:rotate(45deg); }
.brand-glyph::first-letter { transform:rotate(-45deg); }
.titlebar-title { color:#d7e5ed; font:600 11px/1 Bahnschrift,"Segoe UI",sans-serif; letter-spacing:.12em; }
.build-chip { padding:3px 7px; border:1px solid rgba(99,214,160,.18); border-radius:3px; color:rgba(99,214,160,.85); background:rgba(99,214,160,.06); font:600 9px Bahnschrift,"Segoe UI",sans-serif; letter-spacing:.06em; }
.titlebar-status { justify-self:center; display:flex; align-items:center; gap:7px; max-width:460px; padding:4px 11px; border:1px solid var(--line); border-radius:4px; background:#0d1a25; color:#aebdc6; font-size:11px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
.status-light { width:6px; height:6px; border-radius:50%; background:currentColor; box-shadow:0 0 8px currentColor; }
.titlebar-status.success { color:var(--green); }.titlebar-status.error { color:var(--red); }
.titlebar-controls { justify-self:end; display:flex; height:100%; }
.win-btn { width:42px; height:100%; display:grid; place-items:center; border:0; color:#7e909b; background:transparent; cursor:pointer; }
.win-btn:hover { color:#e7edf2; background:rgba(255,255,255,.07); }.win-btn.close:hover { background:#b84646; color:#fff; }
.minimize-line { width:11px; height:1px; background:currentColor; }.close-lines { width:12px; height:12px; position:relative; }.close-lines::before,.close-lines::after { content:""; position:absolute; top:5px; left:0; width:12px; height:1px; background:currentColor; transform:rotate(45deg); }.close-lines::after { transform:rotate(-45deg); }
.app-body { position:relative; z-index:1; height:calc(100vh - 38px); display:grid; grid-template-columns:200px minmax(0,1fr); }
.sidebar { display:flex; flex-direction:column; min-height:0; padding:20px 12px 14px; border-right:1px solid var(--line); background:rgba(8,18,27,.94); }
.sidebar-heading { padding:0 10px 16px; border-bottom:1px solid var(--line); }.sidebar-heading strong { display:block; font:600 14px Bahnschrift,"Microsoft YaHei UI",sans-serif; letter-spacing:.05em; }.sidebar-heading span { display:block; margin-top:5px; color:#607580; font-size:10px; line-height:1.5; }
.primary-nav { display:flex; flex-direction:column; gap:5px; padding-top:15px; }
.nav-item { width:100%; min-height:54px; display:grid; grid-template-columns:30px 1fr 12px; align-items:center; gap:8px; padding:7px 9px; border:1px solid transparent; border-radius:6px; color:#80929e; background:transparent; text-align:left; cursor:pointer; transition:.16s ease; }
.nav-item:hover { color:#c9d7df; background:rgba(255,255,255,.035); }.nav-item.active { color:#edf7fa; border-color:rgba(98,213,231,.18); background:linear-gradient(90deg,rgba(98,213,231,.11),rgba(98,213,231,.025)); }
.nav-mark { width:25px; height:25px; display:grid; place-items:center; border:1px solid rgba(255,255,255,.1); border-radius:4px; color:#6f8794; font-size:11px; }.nav-item.active .nav-mark { color:var(--cyan); border-color:rgba(98,213,231,.38); background:rgba(98,213,231,.08); }
.nav-copy { min-width:0; }.nav-copy strong,.nav-copy small { display:block; }.nav-copy strong { color:inherit; font-size:12px; font-weight:600; }.nav-copy small { margin-top:3px; color:#536873; font-size:9px; white-space:nowrap; }.nav-arrow { color:#425965; font:18px Bahnschrift,sans-serif; }
.sidebar-foot { margin-top:auto; padding:13px 9px 0; border-top:1px solid var(--line); }.target-row { display:flex; align-items:center; gap:9px; }.target-dot { width:7px; height:7px; border-radius:50%; background:var(--green); box-shadow:0 0 9px rgba(99,214,160,.6); }.target-row strong,.target-row small { display:block; }.target-row strong { color:#8fa1ab; font-size:10px; }.target-row small { margin-top:2px; color:#536873; font-size:9px; }.sidebar-foot a { display:inline-block; margin-top:12px; color:#58707c; font-size:9px; text-decoration:none; }.sidebar-foot a:hover { color:var(--cyan); }
.workspace { min-width:0; min-height:0; display:flex; flex-direction:column; }
.workspace-bar { height:42px; flex:0 0 42px; display:flex; align-items:center; justify-content:space-between; padding:0 22px; border-bottom:1px solid var(--line); background:rgba(9,19,29,.86); }
.breadcrumb { display:flex; align-items:center; gap:8px; color:#536a76; font-size:10px; }.breadcrumb b { color:#324955; }.breadcrumb strong { color:#9aadb7; font-weight:600; }
.workspace-state { display:flex; align-items:center; gap:7px; color:#708590; font-size:10px; }.state-dot { width:6px; height:6px; border-radius:50%; background:#71818a; }.state-dot.stable { background:var(--green); }.state-dot.live { background:var(--cyan); }.state-dot.calibrate { background:var(--amber); }.state-dot.waiting { background:var(--red); }
.workspace-actions { display:flex;align-items:center;gap:13px }
.tool-switcher { min-height:43px; display:flex; align-items:end; gap:3px; padding:0 20px; border-bottom:1px solid var(--line); background:rgba(10,21,31,.76); overflow-x:auto; }
.tool-switcher button { position:relative; height:43px; padding:0 12px; border:0; color:#667b86; background:transparent; font-size:10px; white-space:nowrap; cursor:pointer; }.tool-switcher button:hover { color:#b6c7d0; }.tool-switcher button.active { color:#e5f6f8; }.tool-switcher button.active::after { content:""; position:absolute; left:10px; right:10px; bottom:0; height:2px; background:var(--cyan); box-shadow:0 -3px 10px rgba(98,213,231,.25); }.switcher-dot { position:absolute; top:9px; right:5px; width:4px; height:4px; border-radius:50%; background:var(--amber); }
.workspace-scroll { flex:1; min-height:0; overflow:auto; padding:22px 24px 48px; scrollbar-gutter:stable; }
.tool-panel { max-width:1120px; margin:0 auto; }
.tool-panel :deep(.root),.tool-panel :deep(.sigil-container),.tool-panel :deep(.wrightstone-container),.tool-panel :deep(.memory-sigil) { width:100%; max-width:none; margin:0; }
.tool-panel[data-tool="runtime"] :deep(.root > .section > .header),.tool-panel[data-tool="legacyRuntime"] :deep(.root > .section > .header),.tool-panel[data-tool="chara"] :deep(.root > .section > .header),.tool-panel[data-tool="overlimit"] :deep(.root > .section > .header),.tool-panel[data-tool="monster"] :deep(.root > .section > .header),.tool-panel[data-tool="summon"] :deep(.root > .section > .header) { display:none; }
.tool-panel[data-tool="progression"] :deep(.save-title > div:first-child),.tool-panel[data-tool="language"] :deep(.section-header) { display:none; }
.tool-panel[data-tool="progression"] :deep(.save-title) { min-height:0; justify-content:flex-end; }
.tool-panel :deep(input),.tool-panel :deep(select),.legacy-patch input { border-radius:5px!important; border-color:rgba(174,204,224,.15)!important; background:#101f2c!important; color:#dce7ec!important; box-shadow:none!important; }
.tool-panel :deep(button) { border-radius:5px!important; box-shadow:none!important; transform:none!important; }
.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.memory-card),.tool-panel :deep(.language-panel) { border-radius:7px!important; border-color:var(--line)!important; box-shadow:none!important; }
.tool-panel :deep(.memory-card.active) { background:rgba(99,214,160,.08)!important; border-color:rgba(99,214,160,.26)!important; }
.tool-panel :deep(.memory-card::after) { display:none!important; }
.tool-panel :deep(.memory-card.active .memory-title),.tool-panel :deep(.memory-card.active .memory-hint),.tool-panel :deep(.memory-card.active .memory-info) { color:inherit!important; }
.compat-dashboard,.legacy-patch { display:flex; flex-direction:column; gap:14px; }
.calibration-grid { display:grid; grid-template-columns:1.1fr 1fr 1fr; gap:10px; }.calibration-card,.compat-section,.path-card,.patch-card,.backup-card { border:1px solid var(--line); border-radius:7px; background:rgba(12,24,34,.92); }.calibration-card { min-height:140px; padding:16px; }.calibration-card.primary-card { border-top:2px solid var(--cyan); }.card-kicker { color:#5c7380; font:600 9px Bahnschrift,"Microsoft YaHei UI",sans-serif; letter-spacing:.12em; }.calibration-card>strong { display:block; margin-top:10px; color:#d9e7ed; font:600 20px Bahnschrift,"Microsoft YaHei UI",sans-serif; }.calibration-card p { height:34px; margin:7px 0; color:#708691; font-size:10px; line-height:1.55; overflow:hidden; text-overflow:ellipsis; }.file-meta { color:#506772; font-size:9px; }.card-actions { display:flex; gap:6px; margin-top:10px; }
.action { min-height:30px; padding:0 11px; border:1px solid rgba(174,204,224,.16); border-radius:5px; color:#91a4ae; background:#10212e; font-size:10px; cursor:pointer; }.action:hover:not(:disabled) { color:#e3f5f6; border-color:rgba(98,213,231,.35); }.action.primary { color:var(--cyan); border-color:rgba(98,213,231,.28); background:rgba(98,213,231,.08); }.action:disabled { opacity:.38; cursor:not-allowed; }
.compat-section { padding:17px; }.compat-heading { display:flex; align-items:end; justify-content:space-between; margin-bottom:13px; }.compat-heading span { color:var(--cyan); font-size:9px; letter-spacing:.12em; }.compat-heading h2 { margin:3px 0 0; font-size:15px; font-weight:600; }.compat-heading p { margin:0; color:#607782; font-size:10px; }.matrix { border:1px solid rgba(174,204,224,.08); border-radius:5px; overflow:hidden; }.matrix-row { display:grid; grid-template-columns:1.2fr 90px 1.5fr; min-height:38px; align-items:center; padding:0 12px; border-top:1px solid rgba(174,204,224,.07); color:#8498a2; font-size:10px; }.matrix-row:first-child { border-top:0; }.matrix-row.head { min-height:29px; color:#506773; background:rgba(255,255,255,.02); font-size:9px; }.matrix-row b { font-size:9px; font-weight:600; }.matrix-row .ok { color:var(--green); }.matrix-row .flow { color:var(--cyan); }.matrix-row .wait { color:var(--amber); }
.legacy-links { display:grid; grid-template-columns:repeat(3,1fr); gap:8px; }.legacy-links .compat-heading { grid-column:1/-1; }.legacy-links button { min-height:72px; display:grid; grid-template-columns:1fr auto; grid-template-rows:auto auto; align-content:center; gap:4px 10px; padding:12px; border:1px solid rgba(174,204,224,.1); border-radius:5px; color:#9cafb8; background:rgba(255,255,255,.018); text-align:left; cursor:pointer; }.legacy-links button:hover { border-color:rgba(230,185,107,.28); background:rgba(230,185,107,.04); }.legacy-links strong { font-size:11px; }.legacy-links small { color:#536a75; font-size:9px; }.legacy-links button>span { grid-column:2; grid-row:1/3; align-self:center; color:var(--amber); font-size:10px; }
.legacy-warning { display:flex; align-items:center; gap:10px; padding:10px 13px; border:1px solid rgba(230,185,107,.22); border-radius:6px; background:rgba(230,185,107,.06); }.legacy-warning strong { color:var(--amber); font-size:10px; }.legacy-warning span { color:#897d68; font-size:10px; }.path-card { padding:15px; }.path-card>label { display:block; margin-bottom:8px; color:#788e99; font-size:10px; }.path-input-row { display:flex; gap:7px; }.path-input-row input { flex:1; min-width:0; padding:8px 10px; }.detected-file { display:flex; gap:12px; margin-top:8px; color:#607681; font-size:9px; }.detected-file span { flex:1; min-width:0; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }.detected-file b { color:#708895; }.patch-grid { display:grid; grid-template-columns:repeat(2,1fr); gap:10px; }.patch-card { padding:14px; }.patch-card header { display:flex; align-items:start; justify-content:space-between; }.patch-card header strong,.patch-card header small { display:block; }.patch-card header strong { font-size:11px; }.patch-card header small { margin-top:3px; color:#536a75; font-size:9px; }.patch-state { padding:3px 6px; border-radius:3px; font-size:8px; }.patch-state.original { color:var(--cyan); background:rgba(98,213,231,.08); }.patch-state.patched { color:var(--green); background:rgba(99,214,160,.08); }.patch-state.unknown { color:var(--red); background:rgba(236,127,120,.08); }.patch-card p { color:#687e89; font-size:9px; }.patch-edit { display:flex; gap:6px; margin-top:12px; }.patch-edit input { flex:1; min-width:0; padding:7px 9px; }.backup-card { display:flex; align-items:center; gap:9px; padding:13px 15px; }.backup-card>div { flex:1; }.backup-card strong,.backup-card span { display:block; }.backup-card strong { font-size:11px; }.backup-card span { margin-top:3px; color:#566d78; font-size:9px; }.backup-card label { color:#6f838d; font-size:9px; }
.toast-enter-active,.toast-leave-active { transition:.2s ease; }.toast-enter-from,.toast-leave-to { opacity:0; transform:translateY(-4px); }
@media (max-width:980px) { .app-body { grid-template-columns:178px minmax(0,1fr); }.nav-copy small { display:none; }.workspace-scroll { padding:18px 16px 40px; }.calibration-grid { grid-template-columns:1fr; }.legacy-links { grid-template-columns:1fr; }.matrix-row { grid-template-columns:1fr 76px 1.3fr; } }

/* Lyria's Field Notes — unified shell */
.app-window { background:radial-gradient(circle at 78% 5%,rgba(58,151,196,.16),transparent 36%),linear-gradient(155deg,var(--sky-950),var(--sky-1000) 76%); }
.app-window::before { inset:44px 0 0; opacity:.28; background-image:linear-gradient(115deg,transparent 0 73%,rgba(218,187,115,.035) 73% 73.2%,transparent 73.2%); mask-image:none; }
.sky-haze { position:absolute; inset:44px 0 0; pointer-events:none; background:radial-gradient(ellipse at 72% 120%,rgba(43,139,183,.18),transparent 55%),linear-gradient(to bottom,transparent 55%,rgba(1,9,20,.34)); }
.titlebar { height:44px; display:flex; grid-template-columns:none; border-bottom-color:rgba(218,187,115,.19); background:linear-gradient(90deg,rgba(5,17,31,.98),rgba(9,34,57,.96)); box-shadow:0 5px 24px rgba(0,5,14,.25); }
.titlebar-brand { height:100%; padding-left:15px; }.brand-glyph { width:22px;height:22px;border-color:rgba(218,187,115,.48);color:var(--gold);font-size:10px;transform:rotate(45deg); }.brand-glyph::first-letter { transform:none; }
.titlebar-title { color:#e8e3d6; font:600 10px Georgia,"Times New Roman",serif; letter-spacing:.21em; }.build-chip { color:#90d8ec;border-color:rgba(139,221,243,.22);background:rgba(139,221,243,.06);font-size:8px }
.titlebar-status { position:absolute; left:50%; top:8px; transform:translateX(-50%); justify-self:auto; z-index:1; background:rgba(7,27,46,.96); border-color:rgba(218,187,115,.2); }
.titlebar-controls { position:absolute; top:0; right:0; height:44px; margin-left:auto; justify-self:auto; z-index:3; }.win-btn { width:46px; }.win-btn.close:hover { background:#a7484c; }
.app-body { height:calc(100vh - 44px); grid-template-columns:220px minmax(0,1fr); }
.sidebar { padding:23px 12px 14px; border-right-color:rgba(218,187,115,.15); background:linear-gradient(180deg,rgba(5,22,39,.93),rgba(4,17,31,.97)); box-shadow:10px 0 42px rgba(0,6,16,.2); }
.sidebar-heading { padding:0 11px 19px; border-bottom-color:rgba(218,187,115,.14); }.sidebar-heading .sidebar-kicker { margin:0 0 7px;color:var(--gold);font:600 8px Georgia,serif;letter-spacing:.23em }.sidebar-heading strong { color:#eee8da;font:500 17px Georgia,"Noto Serif SC","STSong",serif;letter-spacing:.08em }.sidebar-heading>span:last-child { color:#607d8e;font-size:9px }
.primary-nav { gap:6px;padding-top:17px }.nav-item { min-height:57px;grid-template-columns:30px 1fr 12px;padding:8px 9px;border-radius:3px 10px 3px 10px }.nav-item:hover { background:rgba(84,166,201,.07) }.nav-item.active { border-color:rgba(218,187,115,.2);background:linear-gradient(90deg,rgba(36,112,150,.18),rgba(14,47,74,.07));box-shadow:inset 2px 0 var(--gold) }.nav-mark { border-radius:50%;color:#718fa0;border-color:rgba(141,197,220,.16);font-family:Georgia,serif }.nav-item.active .nav-mark { color:var(--gold);border-color:rgba(218,187,115,.4);background:rgba(218,187,115,.06) }.nav-copy strong { font:500 12px Georgia,"Noto Serif SC","STSong",serif;letter-spacing:.04em }.nav-copy small { color:#587486 }.nav-arrow { color:#597789 }
.sidebar-foot { border-top-color:rgba(218,187,115,.13) }.target-dot { background:var(--cyan);box-shadow:0 0 10px rgba(139,221,243,.65) }.sidebar-foot a:hover { color:var(--gold) }
.workspace-bar { height:39px;flex-basis:39px;padding:0 23px;border-bottom-color:rgba(154,202,224,.11);background:rgba(6,24,42,.7) }.breadcrumb strong { color:#b8c9d1 }.workspace-state { color:#758f9c }
.tool-switcher { min-height:47px;padding:0 22px;border-bottom-color:rgba(218,187,115,.13);background:rgba(7,28,48,.67) }.tool-switcher button { height:47px;padding:0 13px;color:#6c8796 }.tool-switcher button.active { color:#f2ead9 }.tool-switcher button.active::after { left:12px;right:12px;height:1px;background:var(--gold);box-shadow:0 -3px 10px rgba(218,187,115,.28) }
.workspace-scroll { padding:23px 26px 52px }
.tool-panel :deep(input),.tool-panel :deep(select),.legacy-patch input { border-radius:4px!important;border-color:rgba(151,202,224,.18)!important;background:rgba(12,43,68,.78)!important;color:#edf4f6!important }.tool-panel :deep(input:focus),.tool-panel :deep(select:focus) { border-color:rgba(139,221,243,.52)!important }.tool-panel :deep(button) { border-radius:3px 8px 3px 8px!important }.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.memory-card),.tool-panel :deep(.language-panel) { border-radius:4px 12px 4px 12px!important;border-color:rgba(154,202,224,.14)!important;background-color:rgba(8,31,53,.7)!important;box-shadow:0 10px 32px rgba(0,7,16,.12)!important }
.tool-panel :deep(.primary-btn),.tool-panel :deep(.apply-btn),.tool-panel :deep(.ed-apply-btn),.tool-panel :deep(.write-btn) { border-color:rgba(218,187,115,.38)!important;background:linear-gradient(135deg,rgba(184,142,61,.24),rgba(91,65,24,.15))!important;color:#f0d99d!important }
.tool-panel :deep(.primary-btn:hover:not(:disabled)),.tool-panel :deep(.apply-btn:hover:not(:disabled)) { border-color:var(--gold)!important;background:rgba(218,187,115,.19)!important }
.calibration-card,.compat-section,.path-card,.patch-card,.backup-card { border-color:rgba(154,202,224,.14);border-radius:4px 12px 4px 12px;background:rgba(8,31,53,.76) }.calibration-card.primary-card { border-top-color:var(--gold) }.card-kicker,.compat-heading span { color:var(--gold);font-family:Georgia,serif }.calibration-card>strong,.compat-heading h2 { font-family:Georgia,"Noto Serif SC","STSong",serif;color:#eee7d8 }.action.primary { color:var(--gold);border-color:rgba(218,187,115,.3);background:rgba(218,187,115,.07) }.legacy-links button { border-radius:3px 9px 3px 9px }
@media(max-width:980px){.app-body{grid-template-columns:184px minmax(0,1fr)}.workspace-scroll{padding:18px 16px 42px}}

/* Warm Lyria notebook — parchment pages, bold type and chapter ribbons */
.app-window {
  --bg:#ede1c5; --panel:#fffaf0; --panel-2:#f4e8cb; --line:rgba(139,106,55,.22);
  --text:#51483e; --muted:#8d7e6a; --cyan:#3aa9b3; --green:#4c9c76; --amber:#b78237; --red:#bd625e;
  color:var(--text); font-weight:600;
  background:radial-gradient(circle at 82% 18%,rgba(255,255,255,.9),transparent 34%),linear-gradient(120deg,#e5d5b3,#f8f1dc 45%,#eee1c2);
}
.app-window::before { inset:42px 0 0; opacity:.46; mask-image:none; background:linear-gradient(118deg,rgba(255,255,255,.32),transparent 42%,rgba(93,184,196,.08)); }
.app-window :deep(.starfield) { display:none; }
.sky-haze { inset:42px 0 0; background:radial-gradient(ellipse at 74% 100%,rgba(170,190,128,.2),transparent 50%),linear-gradient(to bottom,rgba(255,253,242,.25),rgba(213,193,148,.13)); }
.sky-haze::before { display:none }
.titlebar { height:42px; border-bottom:1px solid rgba(126,91,42,.35); background:linear-gradient(90deg,#594937,#756044 52%,#5b4a37); box-shadow:0 4px 15px rgba(76,55,28,.18); }
.titlebar-brand { padding-left:14px; }.brand-glyph { width:23px;height:23px;border:2px solid rgba(255,229,169,.7);color:#ffe5a9;background:rgba(255,255,255,.06);font-size:11px }.titlebar-title { color:#fff4d8;font:800 11px "Microsoft YaHei UI","Microsoft YaHei",sans-serif;letter-spacing:.15em }.build-chip { color:#d9fbfb;border-color:rgba(121,221,225,.5);background:rgba(76,187,194,.18);font-size:9px;font-weight:800 }
.titlebar-status { top:7px; color:#55493c; border-color:rgba(133,100,49,.35); background:#fff5d9; box-shadow:0 3px 12px rgba(75,54,25,.16); font-weight:800; }
.titlebar-controls { height:42px; }.win-btn { width:46px;color:#e5d7bc }.win-btn:hover { color:#fff;background:rgba(255,255,255,.12) }.win-btn.close:hover { background:#b85957 }
.app-body { height:calc(100vh - 42px); grid-template-columns:230px minmax(0,1fr); }
.app-body.home-mode { grid-template-columns:minmax(0,1fr); }.home-mode .sidebar { display:none }.home-mode .workspace-scroll { padding:0;overflow:auto }.home-mode .workspace { background:#efe1bf }
.sidebar { padding:20px 13px 14px;border-right:1px solid rgba(130,96,48,.3);background:linear-gradient(180deg,rgba(255,249,230,.96),rgba(235,220,187,.97));box-shadow:8px 0 28px rgba(90,66,31,.12),inset -4px 0 rgba(145,110,57,.04) }
.sidebar-heading { padding:0 11px 17px;border-bottom:1px solid rgba(143,107,51,.26) }.sidebar-heading .sidebar-kicker { color:#8a6a34;font:800 9px "Microsoft YaHei UI",sans-serif;letter-spacing:.17em }.sidebar-heading strong { color:#51463a;font:800 19px "Microsoft YaHei UI","Microsoft YaHei",sans-serif;letter-spacing:.04em }.sidebar-heading>span:last-child { color:#8c7d68;font-size:10px;font-weight:700 }
.primary-nav { gap:8px;padding-top:17px }.nav-item { min-height:58px;grid-template-columns:32px 1fr 13px;padding:8px 13px 8px 11px;border:1px solid rgba(114,92,58,.24);border-radius:0;color:#574c40;background:linear-gradient(90deg,rgba(145,126,89,.65),rgba(186,170,132,.44));box-shadow:0 3px 6px rgba(84,63,32,.1),inset 0 1px rgba(255,255,255,.5);clip-path:none }.nav-item:hover { color:#3f5f5f;background:linear-gradient(90deg,rgba(116,202,207,.7),rgba(177,228,228,.6)) }.nav-item.active { color:#28585c;border-color:rgba(54,158,166,.45);background:linear-gradient(90deg,#62d0d8,#b2e9eb);box-shadow:0 4px 9px rgba(44,130,137,.16),inset 0 1px rgba(255,255,255,.65) }.nav-mark { width:28px;height:28px;border:2px solid rgba(83,66,43,.38);border-radius:50%;color:#4f4438;background:rgba(255,249,229,.36);font-family:"Microsoft YaHei UI",sans-serif;font-weight:900 }.nav-item.active .nav-mark { color:#1f6268;border-color:rgba(31,98,104,.48);background:rgba(255,255,255,.34) }.nav-copy strong { color:inherit;font:800 13px "Microsoft YaHei UI","Microsoft YaHei",sans-serif;letter-spacing:.02em }.nav-copy small { color:rgba(71,60,46,.72);font-size:9px;font-weight:700 }.nav-arrow { color:#66543c;font-weight:900 }
.sidebar-foot { border-top-color:rgba(142,106,51,.25) }.target-dot { background:var(--cyan);box-shadow:0 0 8px rgba(58,169,179,.42) }.target-row strong { color:#675a49;font-weight:800 }.target-row small,.sidebar-foot a { color:#927f64;font-weight:700 }.sidebar-foot a:hover { color:#298f98 }
.workspace { background:linear-gradient(rgba(255,250,236,.68),rgba(242,229,199,.76)),url('../assets/gbfr/journal-scene-4k.webp') center/cover fixed }.workspace-bar { height:40px;flex-basis:40px;padding:0 23px;border-bottom:1px solid rgba(145,108,51,.22);background:rgba(255,249,229,.9) }.breadcrumb { color:#8d7c66;font-size:10px;font-weight:700 }.breadcrumb b { color:#c3a96e }.breadcrumb strong { color:#51473c;font-weight:800 }.workspace-state { color:#776b5c;font-weight:700 }.state-dot { box-shadow:0 0 0 3px rgba(103,84,56,.07) }
.tool-switcher { min-height:48px;padding:0 22px;border-bottom:1px solid rgba(140,104,49,.23);background:rgba(239,225,192,.64) }.tool-switcher button { height:48px;padding:0 14px;color:#87765f;font-size:11px;font-weight:800 }.tool-switcher button:hover { color:#2f888f;background:rgba(83,190,197,.08) }.tool-switcher button.active { color:#2c7379;background:rgba(255,255,255,.24) }.tool-switcher button.active::after { left:10px;right:10px;height:3px;background:#45b9c2;box-shadow:none }.workspace-scroll { padding:22px 25px 50px }
.tool-panel { color:#554b40;font-weight:600 }
.tool-panel :deep(.root),.tool-panel :deep(.sigil-container),.tool-panel :deep(.wrightstone-container),.tool-panel :deep(.memory-sigil) { color:#554b40!important;font-weight:600 }
.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.memory-card),.tool-panel :deep(.language-panel),.tool-panel :deep(.library-card),.tool-panel :deep(.detail-panel),.tool-panel :deep(.quests) { border-color:rgba(143,106,51,.3)!important;border-radius:2px 7px 2px 7px!important;background:linear-gradient(135deg,rgba(255,248,226,.78),rgba(231,214,176,.68))!important;backdrop-filter:blur(5px);box-shadow:0 7px 16px rgba(90,65,29,.08),inset 0 0 0 1px rgba(255,255,255,.14)!important }
.tool-panel :deep(input),.tool-panel :deep(select),.tool-panel :deep(textarea),.legacy-patch input { border-color:rgba(139,105,52,.34)!important;background:rgba(246,234,207,.78)!important;color:#4d453b!important;font-weight:700!important;color-scheme:light!important }.tool-panel :deep(input::placeholder),.tool-panel :deep(textarea::placeholder) { color:#917e65!important }.tool-panel :deep(select option) { background:#f2e4c4!important;color:#51483e!important }
.tool-panel :deep(button) { font-weight:900!important;border-radius:2px 5px 2px 5px!important }.tool-panel :deep(.section-title),.tool-panel :deep(.title),.tool-panel :deep(h2),.tool-panel :deep(h3),.tool-panel :deep(.editor-title),.tool-panel :deep(.save-title strong) { color:#54493c!important;font-family:"Microsoft YaHei UI","Microsoft YaHei",sans-serif!important;font-weight:900!important }.tool-panel :deep(label),.tool-panel :deep(.hint),.tool-panel :deep(.field label),.tool-panel :deep(.section-header p),.tool-panel :deep(.memory-hint),.tool-panel :deep(.memory-info),.tool-panel :deep(.save-hint),.tool-panel :deep(.empty-hint) { color:#8b7c68!important;font-weight:800!important }
.tool-panel :deep(.row-name-text),.tool-panel :deep(.queue-name),.tool-panel :deep(.catalog-row strong),.tool-panel :deep(.col-name),.tool-panel :deep(.name),.tool-panel :deep(.ex-col-name),.tool-panel :deep(.ed-current-name) { color:#544a3f!important;font-weight:800!important }.tool-panel :deep(.row-chip),.tool-panel :deep(.queue-item),.tool-panel :deep(.trait-card),.tool-panel :deep(.catalog-row),.tool-panel :deep(.batch-row),.tool-panel :deep(.row-head),.tool-panel :deep(.existing-row) { border-color:rgba(142,106,53,.22)!important;background:rgba(239,225,192,.58)!important }.tool-panel :deep(.catalog-row:hover),.tool-panel :deep(.row-item:hover),.tool-panel :deep(.row:hover) { background:rgba(226,237,203,.72)!important }.tool-panel :deep(.catalog-row.on) { border-color:rgba(49,161,170,.42)!important;background:linear-gradient(90deg,rgba(104,210,217,.25),rgba(248,239,217,.64))!important;box-shadow:inset 3px 0 #43aeb7!important }
.tool-panel :deep(.btn-cyan),.tool-panel :deep(.primary),.tool-panel :deep(.primary-btn),.tool-panel :deep(.apply-btn),.tool-panel :deep(.ed-apply-btn),.tool-panel :deep(.write-btn),.tool-panel :deep(.ed-write),.tool-panel :deep(.save-btn),.tool-panel :deep(.language-button) { border-color:rgba(28,91,98,.55)!important;background:linear-gradient(135deg,#348087,#23646b)!important;color:#fffef4!important;text-shadow:0 1px rgba(26,67,71,.45)!important }.tool-panel :deep(.btn-green),.tool-panel :deep(.btn-connect) { border-color:rgba(43,101,71,.48)!important;background:linear-gradient(135deg,#3c805d,#2e684a)!important;color:#fffef5!important }.tool-panel :deep(.btn-purple),.tool-panel :deep(.plain-btn),.tool-panel :deep(.slot-btn),.tool-panel :deep(.btn-refresh) { border-color:rgba(145,108,52,.28)!important;background:#f2e4c5!important;color:#705c3e!important }.tool-panel :deep(.btn-link),.tool-panel :deep(.btn-icon),.tool-panel :deep(.ed-link),.tool-panel :deep(.row-tool) { color:#6b573b!important }
.tool-panel :deep(.tab),.tool-panel :deep(.chip),.tool-panel :deep(.version-label),.tool-panel :deep(.selection),.tool-panel :deep(.select-all),.tool-panel :deep(.head>span),.tool-panel :deep(.id),.tool-panel :deep(.row-meta),.tool-panel :deep(.row-name-lv),.tool-panel :deep(.row-chip-tag),.tool-panel :deep(.row-chip-lv),.tool-panel :deep(.queue-detail),.tool-panel :deep(.ex-col-level),.tool-panel :deep(.ex-col-trait) { color:#8d7d68!important;font-weight:700!important }.tool-panel :deep(.tab.active) { color:#2f8e96!important;border-bottom-color:#3ba7b0!important }.tool-panel :deep(code),.tool-panel :deep(.count),.tool-panel :deep(.catalog-row em) { color:#2e8f98!important }.tool-panel :deep(input[type=checkbox]) { accent-color:#3ba7b0 }
.tool-panel :deep(.section-tabs button),.tool-panel :deep(.mini-tabs button) { color:#846f51!important;border-color:transparent!important;background:transparent!important }.tool-panel :deep(.section-tabs button.on),.tool-panel :deep(.mini-tabs button.on) { color:#2f8e96!important;border-color:rgba(52,157,166,.31)!important;background:rgba(78,186,194,.12)!important;box-shadow:inset 0 -2px #3aa9b3!important }.tool-panel :deep(.offline-note) { color:#8e682d!important;border-color:rgba(177,126,45,.28)!important;background:rgba(229,193,112,.2)!important }.tool-panel :deep(.empty),.tool-panel :deep(.path),.tool-panel :deep(.hint-inline),.tool-panel :deep(.ed-label),.tool-panel :deep(.ed-changed),.tool-panel :deep(.tpl-empty) { color:#94836d!important;font-weight:700!important }.tool-panel :deep(.capacity),.tool-panel :deep(.safety),.tool-panel :deep(.save-info),.tool-panel :deep(.foot) { color:#4b9470!important;font-weight:800!important }
.tool-panel :deep(.readonly-field),.tool-panel :deep(.ed-level),.tool-panel :deep(.row-chip) { color:#655849!important;border-color:rgba(142,106,53,.19)!important;background:#f4e8cd!important;font-weight:700!important }.tool-panel :deep(.toggle-row),.tool-panel :deep(.owned-toggle) { color:#776856!important;font-weight:700!important }
.tool-panel :deep(.runtime-tabs) { border-color:rgba(141,105,51,.2)!important;background:#e8deca!important }.tool-panel :deep(.runtime-tabs button) { color:#66533d!important;background:transparent!important }.tool-panel :deep(.runtime-tabs button.active) { color:#fffef4!important;border-color:#286b72!important;background:linear-gradient(135deg,#348087,#23646b)!important }.tool-panel :deep(.compatibility-note),.tool-panel :deep(.preflight-grid article) { border-color:rgba(143,106,51,.2)!important;background:#fbf2dd!important }.tool-panel :deep(.preflight-grid strong),.tool-panel :deep(.memory-title),.tool-panel :deep(.currency-name) { color:#574b3e!important;font-weight:800!important }.tool-panel :deep(.preflight-grid p),.tool-panel :deep(.preflight-grid article>span),.tool-panel :deep(.memory-info),.tool-panel :deep(.memory-hint),.tool-panel :deep(.memory-bytes),.tool-panel :deep(.feature-help),.tool-panel :deep(.currency-meta),.tool-panel :deep(.damage-meter-raw) { color:#665845!important;font-weight:700!important }.tool-panel :deep(.feature-help) { border-left-color:#387f85!important;background:rgba(78,180,188,.08)!important }.tool-panel :deep(.btn-batch),.tool-panel :deep(.btn-sort),.tool-panel :deep(.btn-max) { color:#745c37!important;border-color:rgba(149,108,48,.29)!important;background:#eddfbe!important }
.tool-panel { animation:none }.sidebar-heading { cursor:pointer;transition:background-color .18s ease }.sidebar-heading:hover { background:rgba(79,183,190,.05) }.tool-switcher button::after { transition:left .2s ease,right .2s ease,opacity .2s ease }.tool-panel :deep(.guide-list),.tool-panel :deep(.slot-grid label span),.tool-panel :deep(.slot-value span),.tool-panel :deep(.editor-head),.tool-panel :deep(.editor-head strong),.tool-panel :deep(.editor-head span),.tool-panel :deep(.summon-row),.tool-panel :deep(.slot),.tool-panel :deep(.rank),.tool-panel :deep(.name) { color:#6f604e!important }.tool-panel :deep(.od-select),.tool-panel :deep(.value-input) { color:#4d453b!important;background:#f7eed8!important;border-color:rgba(139,105,52,.3)!important }.tool-panel :deep(.od-select option) { color:#51483e!important;background:#fffaf0!important }.tool-panel :deep(.memory-card.active) { color:#51483e!important;background:#edf3df!important;border-color:rgba(72,142,103,.35)!important }.tool-panel :deep(.memory-card.active .memory-title),.tool-panel :deep(.memory-card.active .memory-hint),.tool-panel :deep(.memory-card.active .memory-info),.tool-panel :deep(.memory-card.active .memory-bytes) { color:#5d6d56!important }.tool-panel :deep(.memory-card.active button) { color:#705c3e!important;background:#f2e4c5!important;border-color:rgba(145,108,52,.28)!important }
.tool-panel :deep(.language-card),.tool-panel :deep(.calibration-card),.tool-panel :deep(.compat-section),.tool-panel :deep(.path-card),.tool-panel :deep(.patch-card),.tool-panel :deep(.backup-card) { background:linear-gradient(135deg,rgba(255,248,226,.78),rgba(231,214,176,.67))!important;backdrop-filter:blur(5px) }
.tool-panel :deep(.custom-note-text) { color:#6c5b45!important;font-weight:700!important }.tool-panel :deep(.custom-note-text .note-warn) { color:#267f87!important;font-weight:900!important }
.calibration-card,.compat-section,.path-card,.patch-card,.backup-card { color:#574c40;border-color:rgba(143,106,51,.23);background:#fffaf0;box-shadow:0 7px 20px rgba(85,62,29,.07) }.calibration-card.primary-card { border-top-color:#3ba7b0 }.card-kicker,.compat-heading span { color:#a07637;font-family:"Microsoft YaHei UI",sans-serif;font-weight:800 }.calibration-card>strong,.compat-heading h2 { color:#51473c;font-family:"Microsoft YaHei UI","Microsoft YaHei",sans-serif;font-weight:800 }.calibration-card p,.file-meta,.compat-heading p,.matrix-row,.patch-card p,.backup-card span,.path-card>label,.detected-file { color:#83745f;font-weight:700 }.matrix { border-color:rgba(141,104,49,.17) }.matrix-row { border-top-color:rgba(141,104,49,.13) }.matrix-row.head { color:#8b795e;background:#f2e5c6 }.legacy-links button { color:#594c3e;border-color:rgba(142,105,50,.21);background:#f7eed7;font-weight:800 }.legacy-links button:hover { border-color:#45aeb8;background:#dff2f1 }.legacy-links small { color:#8c7a62 }.action { color:#6e5b3f;border-color:rgba(142,106,52,.3);background:#f2e3c2;font-weight:800 }.action.primary { color:#fffef4;border-color:#286b72;background:linear-gradient(135deg,#348087,#23646b) }.action:hover:not(:disabled) { color:#fffef4;border-color:#1f565c }.legacy-warning { border-color:rgba(180,126,43,.3);background:rgba(229,193,115,.2) }.legacy-warning span { color:#7c6a50 }

/* Three-part workbench: fixed guide, independently scrolling controls, fixed character art. */
.sidebar { position:relative }
.sidebar-collapse { position:absolute;z-index:8;top:13px;right:-12px;width:24px;height:34px;padding:0;border:1px solid rgba(133,99,48,.34);border-radius:0 10px 10px 0;background:#f1e2bd;color:#6d5b40;font:900 19px/1 Georgia,serif;cursor:pointer;box-shadow:3px 3px 10px rgba(73,53,27,.13);transition:background .18s ease,color .18s ease }
.sidebar-collapse:hover { color:#267f87;background:#dff2f1 }
.app-body.sidebar-collapsed { grid-template-columns:70px minmax(0,1fr) }
/* 首页隐藏侧栏，若同时处于侧栏收起态，两列网格会让 workspace 掉进 70px 列而塌陷；
   首页恒为单列，特异性(3 类)高于上面两条，收起与否都正确。 */
.app-body.home-mode.sidebar-collapsed { grid-template-columns:minmax(0,1fr) }
.sidebar-collapsed .sidebar { padding:13px 8px }
.sidebar-collapsed .sidebar-heading,.sidebar-collapsed .nav-copy,.sidebar-collapsed .nav-arrow,.sidebar-collapsed .sidebar-foot { display:none }
.sidebar-collapsed .primary-nav { align-items:center;padding-top:45px;gap:9px }
.sidebar-collapsed .nav-item { width:48px;min-height:48px;display:grid;grid-template-columns:1fr;place-items:center;padding:5px;clip-path:none;border-radius:5px 13px 5px 13px }
.sidebar-collapsed .nav-mark { width:30px;height:30px }
.workspace-scroll.tool-workspace { overflow:hidden;padding:12px 0 0 14px;scrollbar-gutter:auto }
.tool-stage { position:relative;isolation:isolate;width:100%;height:100%;min-height:0;display:grid;grid-template-columns:190px minmax(0,620px) minmax(220px,1fr);gap:12px;overflow:hidden }
.guide-rail { position:relative;z-index:5;min-height:0;align-self:stretch;margin-bottom:12px;padding:16px 14px;border:1px solid rgba(143,106,51,.31);border-radius:3px 13px 3px 13px;background:linear-gradient(155deg,rgba(255,249,229,.91),rgba(228,208,166,.86));box-shadow:0 8px 18px rgba(85,61,27,.08),inset 0 0 0 1px rgba(255,255,255,.22);overflow:hidden }
.guide-heading { display:flex;align-items:end;justify-content:space-between;padding-bottom:11px;border-bottom:1px solid rgba(145,108,51,.22) }.guide-heading span { color:#53473a;font-size:15px;font-weight:900 }.guide-heading small { color:#735b34;font-size:9px;font-weight:900 }
.guide-steps { list-style:none;display:flex;flex-direction:column;gap:9px;margin:15px 0 0;padding:0 }.guide-steps li { display:grid;grid-template-columns:25px 1fr;gap:8px;align-items:center;color:#66533d;font-size:11px;font-weight:800;line-height:1.5 }.guide-steps li b { display:grid;place-items:center;width:25px;height:25px;border-radius:50%;background:linear-gradient(135deg,#397f85,#235d63);color:#fffef4;font-size:10px;box-shadow:0 3px 8px rgba(42,139,147,.2) }
.guide-caution { margin-top:17px;padding:10px 11px;border:1px solid rgba(174,119,49,.25);background:rgba(229,190,106,.16) }.guide-caution span { color:#7d541e;font-size:9px;font-weight:900 }.guide-caution p { margin:5px 0 0;color:#66533d;font-size:10px;font-weight:800;line-height:1.6 }
.guide-character-note { position:absolute;z-index:3;left:10px;right:10px;bottom:10px;height:250px;pointer-events:none }.note-bubble { position:absolute;z-index:3;left:0;right:0;top:0;padding:10px 11px 11px;border:1px solid rgba(151,109,50,.27);border-radius:12px 5px 12px 5px;background:rgba(255,252,238,.88);box-shadow:0 5px 14px rgba(76,54,25,.09) }.note-bubble::after { content:"";position:absolute;left:30px;bottom:-8px;width:14px;height:14px;border-right:1px solid rgba(151,109,50,.27);border-bottom:1px solid rgba(151,109,50,.27);background:#fffbed;transform:rotate(45deg) }.note-bubble b { color:#246a70;font-size:10px;font-weight:900 }.note-bubble p { margin:5px 0 0;color:#5e5141;font-size:9px;font-weight:800;line-height:1.55 }.guide-sticker { position:absolute;z-index:2;left:50%;bottom:-14px;width:150px;height:160px;object-fit:contain;transform:translateX(-50%) rotate(-2deg);filter:drop-shadow(0 8px 7px rgba(78,55,25,.17)) }
.tool-center-scroll { position:relative;z-index:6;min-width:0;min-height:0;overflow-y:auto;overflow-x:hidden;padding:0 2px 32px 0;scrollbar-gutter:stable;scrollbar-width:thin;scrollbar-color:transparent transparent;container-type:inline-size }
.tool-center-scroll:hover,.tool-center-scroll:focus-within { scrollbar-color:rgba(44,112,118,.34) transparent }
.tool-center-scroll::-webkit-scrollbar { width:6px;height:6px }
.tool-center-scroll::-webkit-scrollbar-track { background:transparent }
.tool-center-scroll::-webkit-scrollbar-thumb { min-height:54px;border:2px solid transparent;border-radius:999px;background:transparent;background-clip:padding-box;transition:background-color .18s ease }
.tool-center-scroll:hover::-webkit-scrollbar-thumb,.tool-center-scroll:focus-within::-webkit-scrollbar-thumb { background-color:rgba(44,112,118,.34) }
.tool-center-scroll::-webkit-scrollbar-thumb:hover { background-color:rgba(32,91,97,.52) }
.tool-center-scroll::-webkit-scrollbar-button { display:none;width:0;height:0;background:transparent }
.tool-center-scroll::-webkit-scrollbar-corner { display:none;background:transparent }
.tool-page-heading { position:relative;width:100%;box-sizing:border-box;margin:0 0 13px;padding:19px 21px 18px;border:1px solid rgba(127,88,38,.42);border-radius:3px 14px 3px 14px;background:linear-gradient(120deg,rgba(255,250,232,.98),rgba(235,219,181,.95));box-shadow:0 7px 16px rgba(84,61,28,.08),inset 0 0 0 1px rgba(255,255,255,.36) }
.tool-page-heading .eyebrow { color:#765428;font-size:10px;font-weight:900 }.tool-page-heading h1 { margin:5px 0 7px;color:#4f4438;font:900 26px/1.25 "Microsoft YaHei UI","Microsoft YaHei",sans-serif;letter-spacing:.03em }.tool-page-heading p { margin:0;color:#5e5141;font-size:12px;font-weight:800;line-height:1.65 }
.tool-panel { position:relative;z-index:6;width:100%;max-width:none;margin:0;font-size:12px;box-sizing:border-box }
.tool-panel :deep(*) { box-sizing:border-box }
.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.memory-card),.tool-panel :deep(.language-panel),.tool-panel :deep(.library-card),.tool-panel :deep(.detail-panel),.tool-panel :deep(.catalog-list),.tool-panel :deep(.quests) { width:100%;border-color:rgba(127,88,38,.4)!important;background:linear-gradient(135deg,rgba(255,249,229,.97),rgba(233,215,176,.94))!important;backdrop-filter:none!important;box-shadow:0 6px 14px rgba(77,54,25,.08),inset 0 0 0 1px rgba(255,255,255,.3)!important }
.tool-panel :deep(.section) { padding:16px 18px!important }
.tool-panel :deep(.root),.tool-panel :deep(.sigil-container),.tool-panel :deep(.wrightstone-container),.tool-panel :deep(.memory-sigil) { gap:13px!important }
.tool-panel :deep(input),.tool-panel :deep(select),.tool-panel :deep(textarea) { min-height:36px;font-size:12px!important }
.tool-panel :deep(button) { min-height:36px;font-size:12px!important }
.tool-panel :deep(.btn-icon),.tool-panel :deep(.btn-link),.tool-panel :deep(.ed-link),.tool-panel :deep(.row-tool),.tool-panel :deep(.picker-inline-clear),.tool-panel :deep(.clear-btn),.tool-panel :deep(.rename-confirm),.tool-panel :deep(.rename-cancel),.tool-panel :deep(.ed-max-all) { min-height:28px!important }
.tool-panel :deep(.ed-level input) { width:26px!important;min-height:24px!important;font-size:12px!important }
.tool-panel :deep(.picker-search input),.tool-panel :deep(.rename-input) { min-height:28px!important }
.tool-panel :deep(.section.muted) { opacity:1!important;filter:none!important }
.art-rail { position:relative;z-index:1;min-width:0;min-height:0;margin-left:0;overflow:visible;pointer-events:none }
.art-rail::before { content:"";position:absolute;z-index:0;inset:3% 0 0 16%;border-radius:48% 48% 8px 8px;background:radial-gradient(ellipse at 55% 42%,rgba(255,250,229,.74),rgba(222,198,146,.18) 54%,transparent 72%);filter:blur(2px) }
.art-rail::after { content:"";position:absolute;z-index:0;right:-8px;bottom:-2px;width:390px;height:64px;background:radial-gradient(ellipse at 62% 100%,rgba(87,61,27,.19),rgba(129,99,55,.08) 44%,transparent 72%);filter:blur(7px) }
.art-rail .function-character { position:absolute;z-index:1;left:-110px;right:0;top:0;bottom:0;width:auto;height:auto;margin:0;overflow:visible }
.art-rail .function-character img { position:absolute;right:var(--art-right,-12px);bottom:var(--art-bottom,-12px);display:block;width:auto;max-width:100%;height:100%;max-height:100%;object-fit:contain;object-position:right bottom;transform:none;transform-origin:right bottom }
.art-rail .function-character .character-blend { z-index:0;opacity:.2;filter:blur(5px) saturate(.82);transform:scale(1.012);transform-origin:right bottom }
.art-rail .function-character .character-main { z-index:1;filter:drop-shadow(0 7px 7px rgba(66,46,22,.12)) drop-shadow(0 0 1px rgba(255,248,224,.68)) }
.tool-stage[data-tool="progression"],.tool-stage[data-tool="sigil"],.tool-stage[data-tool="sigilMemory"],.tool-stage[data-tool="loadout"],
.tool-stage[data-tool="wrightstone"],.tool-stage[data-tool="summon"],.tool-stage[data-tool="overlimit"],
.tool-stage[data-tool="runtime"],.tool-stage[data-tool="chara"],.tool-stage[data-tool="save"],
.tool-stage[data-tool="compatibility"],.tool-stage[data-tool="legacyRuntime"],.tool-stage[data-tool="monster"] { --art-right:-12px;--art-bottom:-12px }
.tool-stage[data-tool="patch"] { --art-right:-10px;--art-bottom:-11px }
.tool-stage[data-tool="language"] { --art-right:-13px;--art-bottom:-13px }
.art-caption { position:absolute;z-index:2;right:12px;bottom:14px;display:flex;flex-direction:column;align-items:flex-end;padding:7px 10px;border-right:3px solid rgba(154,116,64,.72);background:rgba(255,249,229,.62);backdrop-filter:blur(3px) }.art-caption span { color:#594d3f;font-size:13px;font-weight:900 }.art-caption small { color:#644f32;font-size:9px;font-weight:900 }

/* Parchment legibility pass: neutralise the remaining dark-theme text inherited by child tools. */
.tool-panel :deep(.section-title small),.tool-panel :deep(.selected-save.empty),.tool-panel :deep(.version-label),.tool-panel :deep(.selection),.tool-panel :deep(.select-all),.tool-panel :deep(.memory-hint),.tool-panel :deep(.memory-info),.tool-panel :deep(.memory-bytes),.tool-panel :deep(.header p),.tool-panel :deep(.pid),.tool-panel :deep(.count) { color:#6d5d47!important;font-weight:800!important }
.tool-panel :deep(.empty),.tool-panel :deep(.empty-hint),.tool-panel :deep(.tpl-empty),.tool-panel :deep(.picker-none),.tool-panel :deep(.ed-current-name.dim),.tool-panel :deep(.ed-level-hint),.tool-panel :deep(.row-meta),.tool-panel :deep(.row-name-lv),.tool-panel :deep(.row-chip-tag),.tool-panel :deep(.row-chip-lv),.tool-panel :deep(.queue-detail),.tool-panel :deep(.ex-col-level),.tool-panel :deep(.ex-col-trait) { color:#705f49!important;font-weight:700!important }
.tool-panel :deep(.field label),.tool-panel :deep(.ed-label),.tool-panel :deep(.slot-grid label span),.tool-panel :deep(.slot-value span),.tool-panel :deep(.editor-head span),.tool-panel :deep(.picker-placeholder),.tool-panel :deep(.cheveron),.tool-panel :deep(.picker-inline-clear) { color:#6b5a45!important;font-weight:800!important }
.tool-panel :deep(.picker-selected),.tool-panel :deep(.ed-current-name),.tool-panel :deep(.ed-current-lv),.tool-panel :deep(.ed-level input),.tool-panel :deep(.row-name-text),.tool-panel :deep(.row-chip-name),.tool-panel :deep(.opt),.tool-panel :deep(.opt-name),.tool-panel :deep(.opt-max) { color:#51483e!important;font-weight:800!important }
.tool-panel :deep(.section-title),.tool-panel :deep(.editor-title),.tool-panel :deep(.memory-title),.tool-panel :deep(.editor-head strong) { font-size:14px!important;font-weight:900!important }
.tool-panel :deep(.field label),.tool-panel :deep(.ed-label),.tool-panel :deep(.picker-selected),.tool-panel :deep(.ed-current-name),.tool-panel :deep(.row-name-text),.tool-panel :deep(.row-chip-name),.tool-panel :deep(.opt),.tool-panel :deep(.opt-name),.tool-panel :deep(.catalog-row),.tool-panel :deep(.queue-name),.tool-panel :deep(.tab),.tool-panel :deep(.preflight-grid strong) { font-size:12px!important }
.tool-panel :deep(.section-title small),.tool-panel :deep(.empty),.tool-panel :deep(.empty-hint),.tool-panel :deep(.tpl-empty),.tool-panel :deep(.picker-none),.tool-panel :deep(.picker-placeholder),.tool-panel :deep(.ed-current-lv),.tool-panel :deep(.ed-level-hint),.tool-panel :deep(.row-meta),.tool-panel :deep(.row-name-lv),.tool-panel :deep(.row-chip-tag),.tool-panel :deep(.row-chip-lv),.tool-panel :deep(.queue-detail),.tool-panel :deep(.opt-max),.tool-panel :deep(.feature-help),.tool-panel :deep(.memory-hint),.tool-panel :deep(.memory-info),.tool-panel :deep(.memory-bytes),.tool-panel :deep(.currency-meta),.tool-panel :deep(.damage-meter-raw),.tool-panel :deep(.result-count),.tool-panel :deep(.save-info),.tool-panel :deep(.warning-hint),.tool-panel :deep(.danger-hint),.tool-panel :deep(.data-error),.tool-panel :deep(.selected-save),.tool-panel :deep(.mode-choice small),.tool-panel :deep(.preflight-grid p),.tool-panel :deep(.preflight-grid article>span) { font-size:11px!important }
.tool-panel :deep(.picker-selected),.tool-panel :deep(.ed-level) { border-color:rgba(139,105,52,.34)!important;background:rgba(246,234,207,.86)!important }
.tool-panel :deep(.picker-dropdown) { border-color:rgba(42,145,154,.55)!important;background:#f3e6c8!important;box-shadow:0 8px 18px rgba(78,55,25,.2)!important }
.tool-panel :deep(.picker-search) { border-bottom-color:rgba(139,105,52,.22)!important }.tool-panel :deep(.picker-search input) { color:#51483e!important;background:transparent!important }
.tool-panel :deep(.opt:hover),.tool-panel :deep(.opt.hi) { color:#244f52!important;background:rgba(78,186,194,.18)!important }
.tool-panel :deep(.ed-arrow),.tool-panel :deep(.info-dot),.tool-panel :deep(.ed-max-all),.tool-panel :deep(.ed-max-btn) { color:#246a70!important }
.tool-panel :deep(.data-error),.tool-panel :deep(.empty.error) { color:#9b3333!important;font-weight:900!important }
.tool-panel :deep(.custom-note-text),.tool-panel :deep(.feature-help),.tool-panel :deep(.compatibility-note) { color:#5e5141!important }.tool-panel :deep(.custom-note-text .note-warn) { color:#246a70!important }
.tool-panel :deep(button:disabled),.tool-panel :deep(input:disabled),.tool-panel :deep(select:disabled) { opacity:.62!important }
.tool-panel :deep(.chip.dim),.tool-panel :deep(.ed-link),.tool-panel :deep(.ed-changed),.tool-panel :deep(.tab),.tool-panel :deep(.tab-count) { color:#66533d!important;font-weight:800!important }
.tool-panel :deep(.tab.active),.tool-panel :deep(.section-tabs button.on),.tool-panel :deep(.mini-tabs button.on) { color:#1d666d!important }.tool-panel :deep(.section-tabs button),.tool-panel :deep(.mini-tabs button),.tool-panel :deep(.runtime-tabs button) { color:#66533d!important }
.tool-panel :deep(.btn-max-all),.tool-panel :deep(.value-combo button) { color:#6c5127!important;border-color:rgba(145,108,52,.3)!important;background:#eddfbe!important }
.tool-panel :deep(.legality) { color:#5e5141!important;border-color:rgba(139,105,52,.28)!important;background:rgba(246,234,207,.75)!important }.tool-panel :deep(.legality .icon) { background:rgba(255,250,236,.64)!important }.tool-panel :deep(.legality .text strong),.tool-panel :deep(.legality .text small) { font-size:11px!important }.tool-panel :deep(.legality .text small) { color:#665845!important }.tool-panel :deep(.legality.legal) { color:#286a4a!important }.tool-panel :deep(.legality.forced) { color:#7d5715!important }.tool-panel :deep(.legality.unknown) { color:#1d666d!important }.tool-panel :deep(.legality.impossible) { color:#963d3d!important }
.tool-panel :deep(.preflight-grid p),.tool-panel :deep(.preflight-grid article>span),.tool-panel :deep(.memory-info),.tool-panel :deep(.memory-hint),.tool-panel :deep(.memory-bytes),.tool-panel :deep(.feature-help),.tool-panel :deep(.currency-meta),.tool-panel :deep(.damage-meter-raw) { color:#665845!important }
.tool-panel :deep(.language-copy p) { color:#665845!important }.tool-panel :deep(.path-card>label),.tool-panel :deep(.backup-card label) { color:#665845!important }
.tool-panel :deep(.matrix-row.head>span) { color:#665845!important }
.tool-panel :deep(.save-title small),.tool-panel :deep(.catalog-row small),.tool-panel :deep(.detail-panel p),.tool-panel :deep(.resource-grid label),.tool-panel :deep(.result-count),.tool-panel :deep(.col-name small),.tool-panel :deep(.existing-header),.tool-panel :deep(.batch),.tool-panel :deep(.od-indicator),.tool-panel :deep(.update-body) { color:#665845!important;font-weight:700!important }
.tool-panel :deep(.update-body),.tool-panel :deep(.od-indicator),.tool-panel :deep(.batch) { border-color:rgba(139,105,52,.2)!important;background:rgba(242,229,198,.72)!important }
.tool-panel :deep(.current),.tool-panel :deep(.catalog-row em),.tool-panel :deep(.update-new) { color:#246a70!important }.tool-panel :deep(.experimental),.tool-panel :deep(.catalog-row.danger),.tool-panel :deep(.danger-confirm),.tool-panel :deep(.offline-note) { color:#74511b!important }.tool-panel :deep(.danger-hint) { color:#963d3d!important }
.tool-panel :deep(.confirm-overlay) { z-index:9998!important;background:rgba(43,31,19,.38)!important;backdrop-filter:blur(5px) saturate(.8)!important }
.tool-panel :deep(.confirm-dialog) { width:min(480px,calc(100vw - 48px))!important;padding:20px 22px!important;border-width:1px 1px 1px 3px!important;border-color:rgba(150,94,39,.38)!important;border-left-color:#a06a32!important;border-radius:3px 15px 3px 15px!important;background:linear-gradient(145deg,#fff8e5,#ead4a9)!important;box-shadow:0 22px 62px rgba(58,40,20,.34),inset 0 0 0 1px rgba(255,255,255,.36)!important }.tool-panel :deep(.confirm-title),.tool-panel :deep(.btn-warn) { color:#74511b!important }.tool-panel :deep(.confirm-title) { padding-bottom:10px!important;border-bottom:1px solid rgba(126,88,39,.24)!important;font-size:15px!important;font-weight:900!important }.tool-panel :deep(.confirm-body) { color:#51483e!important;font-size:12px!important;font-weight:800!important }.tool-panel :deep(.confirm-actions) { padding-top:4px!important;border-top:1px solid rgba(126,88,39,.16)!important }.tool-panel :deep(.btn-warn) { border-color:rgba(150,94,39,.38)!important;background:#e8cf9d!important }
.tool-panel :deep(.clear-btn),.tool-panel :deep(.rename-cancel) { color:#665845!important;border-color:rgba(139,105,52,.25)!important }
.tool-panel :deep(select[size]) { padding:4px!important;line-height:1.55!important;background:rgba(248,237,211,.94)!important }
.tool-panel :deep(select[size] option) { min-height:36px;padding:7px 10px!important;border-bottom:1px solid rgba(103,71,31,.3)!important;background:#f7ecd3!important;color:#51483e!important;box-shadow:none!important;font-size:12px!important;font-weight:750!important }
.tool-panel :deep(select[size] option:nth-child(even)) { background:#efe0bd!important }
.tool-panel :deep(select[size] option:checked) { color:#184f54!important;background:linear-gradient(90deg,rgba(90,190,198,.34),rgba(224,235,205,.8))!important;box-shadow:inset 3px 0 #2c7a81!important;font-weight:900!important }
.tool-panel :deep(.picker-list) { padding:3px 0!important }.tool-panel :deep(.opt) { min-height:36px;padding:7px 10px!important;border-bottom:1px solid rgba(103,71,31,.3)!important;box-shadow:none!important;font-size:12px!important }.tool-panel :deep(.opt:nth-child(even)) { background:rgba(231,214,176,.35)!important }.tool-panel :deep(.opt:last-child) { border-bottom:0!important;box-shadow:none!important }.tool-panel :deep(.opt:hover),.tool-panel :deep(.opt.hi),.tool-panel :deep(.opt.selected) { background:linear-gradient(90deg,rgba(90,190,198,.24),rgba(247,236,208,.72))!important;box-shadow:inset 3px 0 #2c7a81!important }
/* Unified feedback language: pale journal slips with one restrained state accent. */
.titlebar-status { min-height:29px;padding:5px 14px 5px 11px;border-width:1px 1px 1px 3px;border-color:rgba(113,81,39,.34);border-left-color:#347d84;border-radius:2px 9px 2px 9px;color:#51463a;background:linear-gradient(120deg,#fff9e8,#ead8ad);box-shadow:0 5px 17px rgba(54,38,18,.22),inset 0 0 0 1px rgba(255,255,255,.32);font-weight:900 }
.titlebar-status.success { color:#285d46;border-color:rgba(56,112,79,.33);border-left-color:#377d5b;background:linear-gradient(120deg,#f6f4d9,#dce9c8) }.titlebar-status.error { color:#843b35;border-color:rgba(145,69,56,.34);border-left-color:#a04f42;background:linear-gradient(120deg,#fff1dc,#eccdb8) }.titlebar-status .status-light { width:7px;height:7px;box-shadow:0 0 0 3px color-mix(in srgb,currentColor 13%,transparent) }
.tool-panel :deep(.memory-card::after),.tool-panel :deep(.apply-section::after) { display:none!important }
.tool-panel :deep(.memory-card.active),.tool-panel :deep(.apply-section.apply-flash) { color:#4f493e!important;border-color:rgba(48,126,113,.48)!important;background:linear-gradient(120deg,rgba(246,246,216,.98),rgba(218,232,198,.94))!important;box-shadow:inset 4px 0 #397e72,0 8px 20px rgba(57,83,50,.12),inset 0 0 0 1px rgba(255,255,255,.42)!important }
.tool-panel :deep(.memory-card.active .memory-title),.tool-panel :deep(.memory-card.active .currency-name),.tool-panel :deep(.memory-card.active .damage-meter-value),.tool-panel :deep(.apply-section.apply-flash .section-title),.tool-panel :deep(.apply-section.apply-flash .toggle-row) { color:#345a4d!important;font-weight:900!important }
.tool-panel :deep(.memory-card.active .memory-hint),.tool-panel :deep(.memory-card.active .memory-info),.tool-panel :deep(.memory-card.active .memory-bytes),.tool-panel :deep(.memory-card.active .currency-meta),.tool-panel :deep(.memory-card.active .damage-meter-raw),.tool-panel :deep(.memory-card.active .custom-note-text) { color:#5c6854!important;font-weight:750!important }
.tool-panel :deep(.memory-card.active .feature-help) { color:#4d6256!important;border-left-color:#397e72!important;background:rgba(255,250,231,.52)!important }
.tool-panel :deep(.memory-card.active input),.tool-panel :deep(.memory-card.active select),.tool-panel :deep(.apply-section.apply-flash input),.tool-panel :deep(.apply-section.apply-flash select) { color:#51483e!important;border-color:rgba(77,120,88,.32)!important;background:#fff8e7!important;color-scheme:light!important }
.tool-panel :deep(.memory-card.active .btn-batch) { color:#fffdf0!important;border-color:#2b665b!important;background:linear-gradient(135deg,#438779,#2d695e)!important }.tool-panel :deep(.memory-card.active .btn-refresh),.tool-panel :deep(.memory-card.active .btn-sort),.tool-panel :deep(.memory-card.active .btn-max) { color:#66533d!important;border-color:rgba(128,94,48,.29)!important;background:#efe0bd!important }
.tool-panel :deep(.chip.state),.tool-panel :deep(.state.on),.tool-panel :deep(.save-info),.tool-panel :deep(.capacity),.tool-panel :deep(.safety) { color:#2e6b50!important;border-color:rgba(54,116,82,.32)!important;background:rgba(221,234,199,.78)!important;text-shadow:none!important }
.tool-panel :deep(.chip.state),.tool-panel :deep(.state.on) { padding:5px 10px!important;border-style:solid!important;border-width:1px!important;border-radius:999px!important;font-weight:900!important;box-shadow:inset 0 0 0 1px rgba(255,255,255,.35)!important }
.tool-panel :deep(.btn-connect) { color:#fffdf0!important;border-color:#2c665c!important;background:linear-gradient(135deg,#43877a,#2e695f)!important;text-shadow:0 1px rgba(22,69,62,.35)!important }.tool-panel :deep(.btn-disconnect) { color:#813d37!important;border-color:rgba(151,72,59,.34)!important;background:#f1d7c3!important }
.tool-panel :deep(.warning-hint),.tool-panel :deep(.offline-note),.tool-panel :deep(.unlock-risk),.tool-panel :deep(.danger-confirm) { color:#74551f!important;border-color:rgba(155,108,40,.32)!important;border-left:3px solid #b18239!important;background:rgba(241,221,174,.78)!important;font-weight:800!important }.tool-panel :deep(.danger-hint),.tool-panel :deep(.data-error),.tool-panel :deep(.empty.error) { color:#8b3f37!important;border-color:rgba(153,72,58,.32)!important;border-left:3px solid #a65243!important;background:rgba(239,208,184,.7)!important;font-weight:900!important }
.tool-panel :deep(.queue-warning),.tool-panel :deep(.queue-legality.forced) { color:#74551f!important;border-color:rgba(155,108,40,.34)!important;background:#efdfb9!important }.tool-panel :deep(.queue-legality:not(.forced)) { color:#246a70!important;border-color:rgba(43,126,133,.32)!important;background:#dcebdd!important }

/* Official atlas skin: reproduce the journal's parchment, gilt rules and cyan selection language. */
.app-window {
  --cyan:#48c9df;
  --green:#48c9df;
  --official-cyan:#48c9df;
  --official-cyan-deep:#387d92;
  --official-gold:#a98a50;
  --official-gold-line:rgba(157,116,54,.38);
  --official-paper:#fff8df;
  --official-ink:#554b40;
}
.workspace {
  background:
    linear-gradient(rgba(255,251,235,.42),rgba(241,224,184,.5)),
    url('../assets/gbfr/parchment-ui-v2.webp') center/cover fixed;
}
.sidebar {
  background:
    linear-gradient(rgba(255,252,239,.72),rgba(230,209,166,.72)),
    url('../assets/gbfr/parchment-ui-v2.webp') left center/cover;
}
.sidebar::before {
  content:"";
  position:absolute;
  z-index:0;
  left:-7px;
  top:-4px;
  width:112px;
  height:96px;
  pointer-events:none;
  background:url('../assets/gbfr/journal-page-corner.svg') left top/contain no-repeat;
  opacity:.46;
}
.sidebar-heading,.primary-nav,.sidebar-foot { position:relative;z-index:1 }
.app-body { grid-template-columns:205px minmax(0,1fr) }
.sidebar { padding:15px 11px 12px }
.sidebar-heading { padding:0 9px 12px }
.sidebar-heading .sidebar-kicker { margin:0 0 4px;font-size:8px;letter-spacing:.14em }
.sidebar-heading strong { font-size:17px;line-height:1.24;letter-spacing:.025em }
.sidebar-heading>span:last-child { margin-top:3px;font-size:9px;line-height:1.35 }
.primary-nav { gap:5px;padding-top:12px }
.nav-item {
  min-height:46px;
  grid-template-columns:24px minmax(0,1fr) 9px;
  gap:6px;
  padding:5px 9px 5px 8px;
  clip-path:none;
  border-radius:2px 5px 2px 5px;
}
.nav-mark { width:23px;height:23px;border-width:1px;font-size:10px }
.nav-copy strong { max-width:100%;overflow:hidden;font-size:11.5px;line-height:1.18;letter-spacing:0;text-overflow:ellipsis;white-space:nowrap }
.nav-copy small { max-width:100%;margin-top:2px;overflow:hidden;font-size:8px;line-height:1.15;text-overflow:ellipsis;white-space:nowrap }
.nav-arrow { font-size:15px;line-height:1 }
.sidebar-foot { padding:10px 8px 0 }
.sidebar-foot a { margin-top:8px }
.workspace-bar { background:rgba(255,250,231,.9) }
.tool-switcher {
  display:flex;
  align-items:center;
  min-height:50px;
  gap:7px;
  padding:5px 19px;
  background:linear-gradient(90deg,rgba(229,211,171,.68),rgba(255,250,232,.76),rgba(225,205,161,.62));
  box-shadow:inset 0 1px rgba(255,255,255,.76),inset 0 -1px rgba(139,101,47,.16);
}
.tool-switcher button {
  display:inline-flex;
  align-items:center;
  justify-content:center;
  gap:5px;
  flex:1 1 0;
  min-width:0;
  height:38px;
  min-height:38px;
  padding:0 10px;
  border:1px solid transparent;
  color:#756347;
  background:linear-gradient(180deg,rgba(255,251,236,.34),rgba(210,190,148,.18));
  font-size:10.5px;
  font-weight:900;
  letter-spacing:.01em;
  clip-path:none;
  border-radius:2px 4px 0 0;
}
.tool-switcher button:hover {
  color:#5c4628;
  border-color:rgba(133,96,43,.34);
  background:linear-gradient(180deg,rgba(255,250,231,.84),rgba(222,202,160,.62));
}
.tool-switcher button.active {
  color:#4f3d25;
  border-color:rgba(128,90,38,.48);
  background:linear-gradient(180deg,#fff8df 0%,#e7d2a4 100%);
  box-shadow:inset 0 1px rgba(255,255,255,.9),inset 0 -3px #4ba9b7;
  text-shadow:0 1px rgba(255,255,255,.66);
}
.tool-switcher button.active::after { display:none }
.switcher-dot { top:7px;right:8px;background:#fff9da;box-shadow:0 0 0 1px rgba(105,80,42,.3) }
.switcher-tag { flex:0 0 auto;font-size:7.5px;font-weight:900;letter-spacing:.02em;line-height:1;padding:2px 5px;border-radius:20px;opacity:.5;transition:opacity .15s ease }
.switcher-tag.offline { color:#2f7d5c;background:rgba(47,125,92,.12);border:1px solid rgba(47,125,92,.28) }
.switcher-tag.live { color:#256e74;background:rgba(37,110,116,.12);border:1px solid rgba(37,110,116,.28) }
.tool-switcher button:hover .switcher-tag { opacity:.82 }
.tool-switcher button.active .switcher-tag { opacity:1 }
.tool-switcher button.active .switcher-tag.offline { background:rgba(47,125,92,.2) }
.tool-switcher button.active .switcher-tag.live { background:rgba(37,110,116,.2) }
.nav-item {
  color:#655742;
  border-color:rgba(130,96,45,.28);
  background:linear-gradient(90deg,rgba(149,129,91,.62),rgba(216,201,166,.45));
  box-shadow:inset 0 1px rgba(255,255,255,.58),inset 0 -1px rgba(106,77,36,.16),0 2px 5px rgba(77,55,27,.09);
}
.nav-item:hover,.nav-item.active {
  color:#4f3f2c;
  border-color:rgba(117,82,36,.46);
  background:linear-gradient(90deg,#f4e6c4,#fff7dd 72%,#e6d3aa);
}
.nav-item.active { box-shadow:inset 4px 0 #4ba8b6,inset 0 0 0 1px rgba(255,255,255,.38),0 3px 8px rgba(84,59,27,.1) }
.nav-item.active .nav-mark { color:#416f78;border-color:rgba(65,126,139,.42);background:rgba(255,252,235,.72) }
.guide-rail,.tool-page-heading {
  border-color:var(--official-gold-line);
  background:
    linear-gradient(130deg,rgba(255,253,242,.88),rgba(236,217,174,.78)),
    url('../assets/gbfr/parchment-ui-v2.webp') center/cover;
  box-shadow:inset 0 0 0 1px rgba(255,255,255,.55),inset 0 0 0 3px rgba(157,116,54,.055),0 6px 14px rgba(77,54,25,.08);
}
.guide-heading,.tool-panel :deep(.editor-header),.tool-panel :deep(.tabs-head),.tool-panel :deep(.section-header),.tool-panel :deep(.header) {
  border-bottom-color:rgba(151,110,50,.28)!important;
}
.guide-heading { background:url('../assets/gbfr/journal-rule.svg') center bottom/100% 9px no-repeat }
.guide-steps li b {
  border:1px solid rgba(69,126,145,.52);
  background:linear-gradient(180deg,#77dce9,#43acc3);
  box-shadow:inset 0 1px rgba(255,255,255,.72),0 3px 7px rgba(54,116,134,.15);
}
.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.memory-card),
.tool-panel :deep(.language-panel),.tool-panel :deep(.library-card),.tool-panel :deep(.detail-panel),
.tool-panel :deep(.catalog-list),.tool-panel :deep(.quests),.calibration-card,.compat-section,.path-card,.patch-card,.backup-card {
  border-color:rgba(154,113,52,.38)!important;
  background:
    linear-gradient(135deg,rgba(255,252,237,.9),rgba(237,219,177,.82)),
    url('../assets/gbfr/parchment-ui-v2.webp') center/cover!important;
  box-shadow:inset 0 0 0 1px rgba(255,255,255,.52),inset 0 0 0 3px rgba(156,115,53,.04),0 5px 12px rgba(74,52,25,.07)!important;
}
.tool-panel :deep(.section-title),.tool-panel :deep(.section-header),.tool-panel :deep(.editor-header),
.tool-panel :deep(.tabs-head),.compat-heading {
  padding-bottom:11px!important;
  border-bottom:0!important;
  background:url('../assets/gbfr/journal-rule.svg') center bottom/100% 9px no-repeat!important;
}
.tool-panel :deep(.save-title),.tool-panel :deep(.memory-header),.tool-panel :deep(.header) {
  padding-bottom:8px!important;
  border-bottom:1px solid rgba(151,110,50,.24)!important;
  background:none!important;
}
.tool-panel :deep(.save-title:empty) { display:none!important }
.tool-panel :deep(input),.tool-panel :deep(select),.tool-panel :deep(textarea),.legacy-patch input {
  border-color:rgba(151,111,53,.4)!important;
  border-radius:1px 5px 1px 5px!important;
  color:#51483e!important;
  background:linear-gradient(180deg,rgba(255,253,243,.92),rgba(237,222,189,.86))!important;
  box-shadow:inset 0 1px 3px rgba(91,65,29,.08),0 1px rgba(255,255,255,.55)!important;
}
.tool-panel :deep(input:hover),.tool-panel :deep(select:hover),.tool-panel :deep(textarea:hover) { border-color:rgba(64,157,179,.5)!important }
.tool-panel :deep(input:focus),.tool-panel :deep(select:focus),.tool-panel :deep(textarea:focus) {
  border-color:#4bbdd4!important;
  outline:1px solid rgba(75,189,212,.24)!important;
  box-shadow:inset 3px 0 #55d3e5,0 0 0 2px rgba(74,188,211,.1)!important;
}
.tool-panel :deep(.btn-cyan),.tool-panel :deep(.btn-green),.tool-panel :deep(.primary),.tool-panel :deep(.primary-btn),
.tool-panel :deep(.apply-btn),.tool-panel :deep(.ed-apply-btn),.tool-panel :deep(.write-btn),.tool-panel :deep(.ed-write),
.tool-panel :deep(.save-btn),.tool-panel :deep(.language-button),.tool-panel :deep(.btn-connect),.action.primary {
  color:#fff9e9!important;
  border-color:#765126!important;
  border-radius:2px 8px 2px 8px!important;
  background:linear-gradient(180deg,#b58c4d,#88612f)!important;
  box-shadow:inset 0 1px rgba(255,255,255,.35),inset 0 -1px rgba(70,43,16,.24)!important;
  text-shadow:0 1px rgba(66,40,15,.42)!important;
}
.tool-panel :deep(.btn-cyan:hover:not(:disabled)),.tool-panel :deep(.primary:hover:not(:disabled)),
.tool-panel :deep(.primary-btn:hover:not(:disabled)),.tool-panel :deep(.apply-btn:hover:not(:disabled)),
.tool-panel :deep(.btn-connect:hover:not(:disabled)),.action.primary:hover:not(:disabled) {
  border-color:#604018!important;
  background:linear-gradient(180deg,#c69d5a,#956a32)!important;
}
.tool-panel :deep(.section-tabs),.tool-panel :deep(.mini-tabs),.tool-panel :deep(.runtime-tabs),.tool-panel :deep(.tabs) {
  gap:6px!important;
  padding:3px!important;
  border-color:rgba(146,106,48,.18)!important;
  background:rgba(255,251,237,.34)!important;
}
.tool-panel :deep(.section-tabs button),.tool-panel :deep(.mini-tabs button),.tool-panel :deep(.runtime-tabs button),.tool-panel :deep(.tab) {
  flex:1 1 0!important;
  min-width:0!important;
  min-height:34px!important;
  padding:6px 10px!important;
  border:1px solid rgba(150,111,54,.24)!important;
  color:#705f45!important;
  background:linear-gradient(180deg,rgba(255,253,242,.72),rgba(226,208,168,.52))!important;
  clip-path:none;
  border-radius:2px 4px 0 0!important;
}
.tool-panel :deep(.section-tabs button.on),.tool-panel :deep(.mini-tabs button.on),.tool-panel :deep(.runtime-tabs button.active),.tool-panel :deep(.tab.active) {
  color:#4f3f2b!important;
  border-color:rgba(126,89,39,.48)!important;
  background:linear-gradient(180deg,#fff8df,#e5cfa0)!important;
  box-shadow:inset 0 1px rgba(255,255,255,.78),inset 0 -3px #4ba9b7!important;
}
.tool-panel :deep(.tabs) {
  flex:0 1 auto!important;
  gap:8px!important;
  padding:0!important;
  border:0!important;
  background:transparent!important;
}
.tool-panel :deep(.tab) {
  flex:0 0 auto!important;
  min-width:88px!important;
  padding:7px 14px!important;
  white-space:nowrap!important;
}
/* Options and logs share the official indexed-row treatment. */
.tool-panel :deep(select[size]),.tool-panel :deep(.picker-list),.tool-panel :deep(.catalog-list),.tool-panel :deep(.row-list) {
  padding:5px!important;
  border:1px solid rgba(147,106,49,.32)!important;
  background:rgba(255,251,235,.62)!important;
  box-shadow:inset 0 0 12px rgba(126,91,43,.05)!important;
}
.tool-panel :deep(select[size] option),.tool-panel :deep(.opt),.tool-panel :deep(.catalog-row),.tool-panel :deep(.row-item),
.tool-panel :deep(.history-row),.tool-panel :deep(.log-row),.tool-panel :deep(.record-row),.tool-panel :deep(.result-row) {
  min-height:38px!important;
  margin:4px 2px!important;
  padding:8px 12px!important;
  border:1px solid rgba(154,115,58,.2)!important;
  border-bottom-color:rgba(126,90,42,.28)!important;
  border-radius:1px!important;
  color:#564b3e!important;
  background:linear-gradient(90deg,rgba(255,253,244,.9),rgba(232,216,180,.72))!important;
  box-shadow:inset 0 1px rgba(255,255,255,.48)!important;
}
.tool-panel :deep(.row-item),.tool-panel :deep(.catalog-row),.tool-panel :deep(.summon-row),
.tool-panel :deep(.row),.tool-panel :deep(.existing-row) { border-left-width:3px!important;border-left-color:rgba(155,115,55,.22)!important }
.tool-panel :deep(.row-item:hover),.tool-panel :deep(.catalog-row:hover),.tool-panel :deep(.catalog-row.on),
.tool-panel :deep(.summon-row.selected),.tool-panel :deep(.row.on) { border-left-color:#4bc7dc!important }
.tool-panel :deep(select[size] option:nth-child(even)),.tool-panel :deep(.opt:nth-child(even)),
.tool-panel :deep(.catalog-row:nth-child(even)),.tool-panel :deep(.row-item:nth-child(even)) {
  background:linear-gradient(90deg,rgba(244,235,211,.94),rgba(222,203,162,.66))!important;
}
.tool-panel :deep(select[size] option:checked),.tool-panel :deep(.opt:hover),.tool-panel :deep(.opt.hi),.tool-panel :deep(.opt.selected),
.tool-panel :deep(.catalog-row:hover),.tool-panel :deep(.catalog-row.on),.tool-panel :deep(.row-item:hover),.tool-panel :deep(.row-item.active) {
  color:#4e412f!important;
  border-color:rgba(117,82,38,.48)!important;
  background:linear-gradient(90deg,#fff9e5,#e8d5aa 80%,#f3e8c9)!important;
  box-shadow:inset 4px 0 #4ba8b6,inset 0 0 0 1px rgba(255,255,255,.38)!important;
}
.tool-panel :deep(.row-chip),.tool-panel :deep(.queue-item),.tool-panel :deep(.trait-card),.tool-panel :deep(.batch-row),
.tool-panel :deep(.row-head),.tool-panel :deep(.existing-row),.tool-panel :deep(.update-body),.tool-panel :deep(.od-indicator) {
  border-color:rgba(148,108,50,.25)!important;
  background:rgba(255,249,228,.58)!important;
}
.tool-panel :deep(.memory-card.active),.tool-panel :deep(.apply-section.apply-flash) {
  color:#4f493e!important;
  border-color:rgba(67,152,174,.54)!important;
  background:linear-gradient(120deg,rgba(245,249,225,.98),rgba(207,237,235,.94))!important;
  box-shadow:inset 4px 0 #4bc8dc,0 7px 17px rgba(55,112,127,.11),inset 0 0 0 1px rgba(255,255,255,.48)!important;
}
.tool-panel :deep(.memory-card.active .memory-title),.tool-panel :deep(.memory-card.active .currency-name),
.tool-panel :deep(.memory-card.active .damage-meter-value),.tool-panel :deep(.apply-section.apply-flash .section-title),
.tool-panel :deep(.apply-section.apply-flash .toggle-row) { color:#315c69!important }
.tool-panel :deep(.memory-card.active .memory-hint),.tool-panel :deep(.memory-card.active .memory-info),
.tool-panel :deep(.memory-card.active .memory-bytes),.tool-panel :deep(.memory-card.active .currency-meta),
.tool-panel :deep(.memory-card.active .damage-meter-raw),.tool-panel :deep(.memory-card.active .custom-note-text) { color:#58685f!important }
.tool-panel :deep(.memory-card.active .feature-help) { color:#4c6264!important;border-left-color:#4bc8dc!important }
.tool-panel :deep(.memory-card.active input),.tool-panel :deep(.memory-card.active select),
.tool-panel :deep(.apply-section.apply-flash input),.tool-panel :deep(.apply-section.apply-flash select) { border-color:rgba(64,150,171,.38)!important }
.tool-panel :deep(.memory-card.active .btn-batch) { color:#fffdf0!important;border-color:#43849a!important;background:linear-gradient(180deg,#65d5e6,#3ca6bf)!important }
.tool-panel :deep(.chip.state),.tool-panel :deep(.state.on),.tool-panel :deep(.save-info),.tool-panel :deep(.capacity),.tool-panel :deep(.safety),.tool-panel :deep(.foot),.tool-panel :deep(.legality.legal),
.tool-panel :deep(.queue-legality:not(.forced)) { color:#2e6878!important;border-color:rgba(59,143,164,.34)!important;background:rgba(207,237,236,.78)!important }
.titlebar-status.success { color:#4d5e57;border-color:rgba(124,88,39,.34);border-left:3px solid #47a8b6;background:linear-gradient(120deg,#fff7df,#ead8b3) }
.state-dot.stable,.target-dot { background:#4dcce0;box-shadow:0 0 8px rgba(77,204,224,.5) }
.tool-center-scroll {
  scrollbar-gutter:auto;
  scrollbar-color:rgba(67,190,214,.78) rgba(164,125,67,.09);
}
.tool-center-scroll:hover,.tool-center-scroll:focus-within { scrollbar-color:rgba(55,174,199,.9) rgba(164,125,67,.12) }
.tool-center-scroll::-webkit-scrollbar { width:7px }
.tool-center-scroll::-webkit-scrollbar-track {
  margin:5px 0;
  background:linear-gradient(90deg,transparent 3px,rgba(149,108,51,.2) 3px 4px,transparent 4px);
}
.tool-center-scroll::-webkit-scrollbar-thumb,
.tool-center-scroll:hover::-webkit-scrollbar-thumb,.tool-center-scroll:focus-within::-webkit-scrollbar-thumb {
  min-height:54px;
  border:2px solid transparent;
  border-radius:0;
  background:linear-gradient(#8ce5ed,#43bad2) padding-box;
  box-shadow:inset 0 0 0 1px rgba(57,119,137,.2);
}
.tool-center-scroll::-webkit-scrollbar-thumb:hover { background:linear-gradient(#a7edf2,#3eb2cb) padding-box }
/* Final control language: cyan is a focus marker, never a generic fill. */
.tool-panel :deep(.plain-btn),.tool-panel :deep(.slot-btn),.tool-panel :deep(.btn-refresh),.tool-panel :deep(.btn-sort),
.tool-panel :deep(.btn-max),.tool-panel :deep(.btn-max-all),.tool-panel :deep(.ed-max-btn),.tool-panel :deep(.ed-max-all),
.tool-panel :deep(.btn-batch),.tool-panel :deep(.row-apply),.tool-panel :deep(.mode-choice),.tool-panel :deep(.slot-choice),
.tool-panel :deep(.language-button),.tool-panel :deep(.value-combo button),.tool-panel :deep(.qty-add) {
  color:#665136!important;
  border-color:rgba(126,89,39,.36)!important;
  border-radius:2px 7px 2px 7px!important;
  background:linear-gradient(180deg,#fff8df,#e8d3a7)!important;
  box-shadow:inset 0 1px rgba(255,255,255,.72),inset 0 -1px rgba(113,77,31,.1)!important;
  text-shadow:none!important;
}
.tool-panel :deep(.plain-btn:hover:not(:disabled)),.tool-panel :deep(.slot-btn:hover:not(:disabled)),
.tool-panel :deep(.btn-refresh:hover:not(:disabled)),.tool-panel :deep(.btn-sort:hover:not(:disabled)),
.tool-panel :deep(.btn-max:hover:not(:disabled)),.tool-panel :deep(.btn-max-all:hover:not(:disabled)),
.tool-panel :deep(.ed-max-btn:hover:not(:disabled)),.tool-panel :deep(.ed-max-all:hover:not(:disabled)),
.tool-panel :deep(.btn-batch:hover:not(:disabled)),.tool-panel :deep(.row-apply:hover:not(:disabled)),
.tool-panel :deep(.mode-choice:hover:not(:disabled)),.tool-panel :deep(.slot-choice:hover:not(:disabled)),
.tool-panel :deep(.language-button:hover:not(:disabled)),.tool-panel :deep(.value-combo button:hover:not(:disabled)),
.tool-panel :deep(.qty-add:hover:not(:disabled)) {
  color:#4d3d28!important;
  border-color:rgba(74,137,151,.52)!important;
  background:linear-gradient(180deg,#fffbed,#e6d6b2)!important;
  box-shadow:inset 3px 0 #4ba8b6,inset 0 1px rgba(255,255,255,.78)!important;
}
.tool-panel :deep(.slot-btn.on),.tool-panel :deep(.slot-choice.on),.tool-panel :deep(.mode-choice.on),
.tool-panel :deep(.language-card.active .language-button) {
  color:#4d3d28!important;
  border-color:rgba(115,81,36,.5)!important;
  background:linear-gradient(180deg,#fff9e2,#e4cca0)!important;
  box-shadow:inset 0 -3px #4ba8b6,inset 0 1px rgba(255,255,255,.78)!important;
}
.tool-panel :deep(.memory-card.active .btn-batch),.tool-panel :deep(.apply-section.apply-flash .btn-cyan) {
  color:#fff9e9!important;
  border-color:#765126!important;
  background:linear-gradient(180deg,#b58c4d,#88612f)!important;
  box-shadow:inset 0 1px rgba(255,255,255,.35)!important;
}
.tool-panel :deep(.chip),.tool-panel :deep(.state),.tool-panel :deep(.capacity),.tool-panel :deep(.safety),
.tool-panel :deep(.save-info),.tool-panel :deep(.foot),.tool-panel :deep(.queue-legality),.tool-panel :deep(.legality) {
  border-radius:2px 6px 2px 6px!important;
  box-shadow:inset 0 1px rgba(255,255,255,.55)!important;
  text-shadow:none!important;
}
.tool-panel :deep(.chip.state),.tool-panel :deep(.state.on),.tool-panel :deep(.save-info),.tool-panel :deep(.capacity),
.tool-panel :deep(.safety),.tool-panel :deep(.foot),.tool-panel :deep(.legality.legal),.tool-panel :deep(.queue-legality:not(.forced)) {
  color:#376b70!important;
  border-color:rgba(63,128,136,.32)!important;
  border-left:3px solid #4ba8b6!important;
  background:linear-gradient(90deg,#fbf4da,#e4dbc0)!important;
}
/* Flat journal pass: controls belong to the page instead of sitting on it as raised tiles. */
.brand-glyph {
  width:17px!important;
  height:17px!important;
  border:0!important;
  border-radius:0!important;
  color:#f1d89d!important;
  background:transparent!important;
  box-shadow:none!important;
  transform:none!important;
}
.build-chip {
  border-color:rgba(244,224,177,.34)!important;
  border-radius:2px!important;
  color:#f2e2bd!important;
  background:rgba(255,250,230,.07)!important;
  box-shadow:none!important;
}
.sidebar,.workspace-bar,.tool-switcher { box-shadow:none!important }
.sidebar-collapse {
  top:9px!important;
  right:5px!important;
  width:22px!important;
  height:22px!important;
  border:0!important;
  border-radius:0!important;
  background:transparent!important;
  box-shadow:none!important;
}
.sidebar-heading:hover { transform:none!important }
.nav-item {
  border-width:0 0 1px!important;
  border-color:rgba(126,92,45,.2)!important;
  border-radius:0!important;
  background:transparent!important;
  box-shadow:none!important;
  clip-path:none!important;
  transform:none!important;
}
.nav-item:hover {
  color:#514431!important;
  border-color:rgba(117,82,36,.34)!important;
  background:rgba(255,250,232,.32)!important;
  transform:none!important;
}
.nav-item.active {
  color:#4c3e2c!important;
  border-color:rgba(117,82,36,.42)!important;
  background:linear-gradient(90deg,rgba(255,249,227,.72),rgba(228,211,174,.26))!important;
  box-shadow:inset 3px 0 #9a7440!important;
}
.nav-mark,.nav-item.active .nav-mark {
  border:0!important;
  border-radius:0!important;
  color:#715c3d!important;
  background:transparent!important;
  box-shadow:none!important;
}
.nav-item.active .nav-mark { color:#765528!important }
.tool-switcher {
  min-height:44px!important;
  gap:13px!important;
  padding:3px 20px 0!important;
  background:rgba(239,225,192,.56)!important;
}
.tool-switcher button {
  height:41px!important;
  min-height:41px!important;
  padding:0 4px!important;
  border:0!important;
  border-radius:0!important;
  color:#78684f!important;
  background:transparent!important;
  box-shadow:none!important;
  clip-path:none!important;
  text-shadow:none!important;
}
.tool-switcher button:hover { color:#57472f!important;background:rgba(255,252,239,.22)!important }
.tool-switcher button.active {
  color:#4e402e!important;
  background:transparent!important;
  box-shadow:inset 0 -2px #9a7440!important;
}
.tool-page-heading {
  margin-bottom:10px!important;
  padding:14px 17px 13px!important;
  border-width:0 0 1px!important;
  border-radius:0!important;
  background:rgba(255,249,229,.5)!important;
  box-shadow:none!important;
}
.tool-page-heading .eyebrow { font-size:9px!important;letter-spacing:.08em!important }
.tool-page-heading h1 { margin:4px 0 5px!important;font-size:21px!important;line-height:1.25!important;letter-spacing:.015em!important }
.tool-page-heading p { font-size:10.5px!important;line-height:1.55!important }
.guide-rail,
.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.memory-card),
.tool-panel :deep(.language-panel),.tool-panel :deep(.library-card),.tool-panel :deep(.detail-panel),
.tool-panel :deep(.catalog-list),.tool-panel :deep(.quests),.calibration-card,.compat-section,.path-card,.patch-card,.backup-card {
  border-color:rgba(137,99,47,.28)!important;
  border-radius:2px!important;
  background:rgba(250,240,214,.72)!important;
  box-shadow:none!important;
}
.tool-panel :deep(.section) { padding:13px 15px!important }
.tool-panel :deep(.section-title),.tool-panel :deep(.editor-title),.tool-panel :deep(.memory-title),.tool-panel :deep(.editor-head strong) {
  font-size:12px!important;
  line-height:1.35!important;
  letter-spacing:.015em!important;
}
.tool-panel :deep(.section-title),.tool-panel :deep(.section-header),.tool-panel :deep(.editor-header),
.tool-panel :deep(.tabs-head),.compat-heading {
  padding-bottom:8px!important;
  background:none!important;
  border-bottom:1px solid rgba(143,104,49,.2)!important;
}
.tool-panel :deep(input),.tool-panel :deep(select),.tool-panel :deep(textarea),.legacy-patch input {
  min-height:32px!important;
  border-radius:2px!important;
  background:rgba(255,251,237,.62)!important;
  box-shadow:none!important;
}
.tool-panel :deep(input:focus),.tool-panel :deep(select:focus),.tool-panel :deep(textarea:focus) {
  border-color:#9a7440!important;
  outline:0!important;
  box-shadow:inset 2px 0 #9a7440!important;
}
.tool-panel :deep(.ed-level-control input),.tool-panel :deep(.ed-level-control input:focus) {
  width:100%!important;
  min-height:24px!important;
  height:24px!important;
  padding:0!important;
  border:1px solid rgba(139,105,52,.34)!important;
  color:#51483e!important;
  background:rgba(255,251,237,.62)!important;
  box-shadow:none!important;
  text-align:center!important;
}
.tool-panel :deep(.ed-level-control),.tool-panel :deep(.ed-level-control>span),.tool-panel :deep(.ed-level-control small),
.tool-panel :deep(.ed-current-prefix),.tool-panel :deep(.ed-level-empty) { color:#776650!important }
.tool-panel :deep(.ed-level-control small) { font-size:8px!important;font-weight:800!important;white-space:nowrap!important }
.tool-panel :deep(.ed-legality) { padding:2px 0 2px 8px!important;border-width:0 0 0 2px!important;border-radius:0!important;background:transparent!important;box-shadow:none!important }
.tool-panel :deep(.ed-legality .icon) { background:transparent!important }
.tool-panel :deep(.ed-legality .text small) { color:#665845!important;font-size:9px!important;white-space:normal!important;overflow:visible!important;text-overflow:clip!important }
.tool-panel :deep(.ed-changed) { color:#796950!important;font-size:9px!important }
.tool-panel :deep(button) { min-height:32px;font-size:11px!important }
.tool-panel :deep(.btn-cyan),.tool-panel :deep(.btn-green),.tool-panel :deep(.primary),.tool-panel :deep(.primary-btn),
.tool-panel :deep(.apply-btn),.tool-panel :deep(.ed-apply-btn),.tool-panel :deep(.write-btn),.tool-panel :deep(.ed-write),
.tool-panel :deep(.save-btn),.tool-panel :deep(.language-button),.tool-panel :deep(.btn-connect),.action.primary {
  border-radius:2px!important;
  color:#fff9e9!important;
  background:#8b6737!important;
  box-shadow:none!important;
  text-shadow:none!important;
}
.tool-panel :deep(.btn-cyan:hover:not(:disabled)),.tool-panel :deep(.primary:hover:not(:disabled)),
.tool-panel :deep(.primary-btn:hover:not(:disabled)),.tool-panel :deep(.apply-btn:hover:not(:disabled)),
.tool-panel :deep(.btn-connect:hover:not(:disabled)),.action.primary:hover:not(:disabled) { background:#76552d!important }
.tool-panel :deep(.plain-btn),.tool-panel :deep(.slot-btn),.tool-panel :deep(.btn-refresh),.tool-panel :deep(.btn-sort),
.tool-panel :deep(.btn-max),.tool-panel :deep(.btn-max-all),.tool-panel :deep(.ed-max-btn),.tool-panel :deep(.ed-max-all),
.tool-panel :deep(.btn-batch),.tool-panel :deep(.row-apply),.tool-panel :deep(.mode-choice),.tool-panel :deep(.slot-choice),
.tool-panel :deep(.language-button),.tool-panel :deep(.value-combo button),.tool-panel :deep(.qty-add) {
  border-radius:2px!important;
  background:rgba(239,224,188,.65)!important;
  box-shadow:none!important;
}
.tool-panel :deep(.plain-btn:hover:not(:disabled)),.tool-panel :deep(.slot-btn:hover:not(:disabled)),
.tool-panel :deep(.btn-refresh:hover:not(:disabled)),.tool-panel :deep(.btn-sort:hover:not(:disabled)),
.tool-panel :deep(.btn-max:hover:not(:disabled)),.tool-panel :deep(.btn-max-all:hover:not(:disabled)),
.tool-panel :deep(.ed-max-btn:hover:not(:disabled)),.tool-panel :deep(.ed-max-all:hover:not(:disabled)),
.tool-panel :deep(.btn-batch:hover:not(:disabled)),.tool-panel :deep(.row-apply:hover:not(:disabled)),
.tool-panel :deep(.mode-choice:hover:not(:disabled)),.tool-panel :deep(.slot-choice:hover:not(:disabled)),
.tool-panel :deep(.language-button:hover:not(:disabled)),.tool-panel :deep(.value-combo button:hover:not(:disabled)),
.tool-panel :deep(.qty-add:hover:not(:disabled)) { background:rgba(255,248,225,.78)!important;box-shadow:inset 2px 0 #9a7440!important }
.tool-panel :deep(.section-tabs),.tool-panel :deep(.mini-tabs),.tool-panel :deep(.runtime-tabs),.tool-panel :deep(.tabs) {
  gap:12px!important;
  padding:0!important;
  border:0!important;
  border-bottom:1px solid rgba(143,104,49,.22)!important;
  background:transparent!important;
}
.tool-panel :deep(.section-tabs button),.tool-panel :deep(.mini-tabs button),.tool-panel :deep(.runtime-tabs button),.tool-panel :deep(.tab) {
  min-height:31px!important;
  padding:5px 2px!important;
  border:0!important;
  border-radius:0!important;
  background:transparent!important;
  box-shadow:none!important;
  clip-path:none!important;
}
.tool-panel :deep(.section-tabs button.on),.tool-panel :deep(.mini-tabs button.on),.tool-panel :deep(.runtime-tabs button.active),.tool-panel :deep(.tab.active) {
  color:#4f402d!important;
  background:transparent!important;
  box-shadow:inset 0 -2px #9a7440!important;
}
.tool-panel :deep(select[size]),.tool-panel :deep(.picker-list),.tool-panel :deep(.catalog-list),.tool-panel :deep(.row-list) {
  background:rgba(255,250,233,.48)!important;
  box-shadow:none!important;
}
.tool-panel :deep(select[size] option),.tool-panel :deep(.opt),.tool-panel :deep(.catalog-row),.tool-panel :deep(.row-item),
.tool-panel :deep(.history-row),.tool-panel :deep(.log-row),.tool-panel :deep(.record-row),.tool-panel :deep(.result-row) {
  min-height:34px!important;
  margin:0!important;
  padding:7px 9px!important;
  border-width:0 0 1px!important;
  border-radius:0!important;
  background:transparent!important;
  box-shadow:none!important;
  font-size:11px!important;
}
.tool-panel :deep(select[size] option:nth-child(even)),.tool-panel :deep(.opt:nth-child(even)),
.tool-panel :deep(.catalog-row:nth-child(even)),.tool-panel :deep(.row-item:nth-child(even)) { background:rgba(228,210,169,.18)!important }
.tool-panel :deep(select[size] option:checked),.tool-panel :deep(.opt:hover),.tool-panel :deep(.opt.hi),.tool-panel :deep(.opt.selected),
.tool-panel :deep(.catalog-row:hover),.tool-panel :deep(.catalog-row.on),.tool-panel :deep(.row-item:hover),.tool-panel :deep(.row-item.active) {
  color:#4e412f!important;
  border-color:rgba(117,82,38,.3)!important;
  background:rgba(237,224,192,.55)!important;
  box-shadow:inset 2px 0 #9a7440!important;
}
.tool-panel :deep(.picker-dropdown) { border-color:rgba(126,91,42,.34)!important;border-radius:2px!important;background:#f3e6c8!important;box-shadow:0 6px 14px rgba(78,55,25,.14)!important }
.guide-steps li b { border-color:rgba(119,84,38,.42)!important;color:#fff9e8!important;background:#9a7440!important;box-shadow:none!important }
.tool-panel :deep(.ed-arrow),.tool-panel :deep(.info-dot),.tool-panel :deep(.ed-max-all),.tool-panel :deep(.ed-max-btn) { color:#765528!important }
.tool-panel :deep(.chip.state),.tool-panel :deep(.state.on),.tool-panel :deep(.save-info),.tool-panel :deep(.capacity),
.tool-panel :deep(.safety),.tool-panel :deep(.foot),.tool-panel :deep(.legality.legal),.tool-panel :deep(.queue-legality:not(.forced)) {
  color:#53654f!important;
  border-color:rgba(91,111,79,.34)!important;
  border-left-color:#6f875f!important;
  background:rgba(235,232,196,.62)!important;
}
.tool-panel :deep(.conn-left .chip) { padding:2px 0 2px 8px!important;border-width:0 0 0 2px!important;border-radius:0!important;background:transparent!important;box-shadow:none!important }
.tool-panel :deep(.btn.tiny:not(.btn-cyan)) { color:#715f47!important;border-color:rgba(132,96,45,.26)!important;border-radius:2px!important;background:transparent!important;box-shadow:none!important }
.tool-panel :deep(.btn.tiny:not(.btn-cyan):disabled) { color:#9a896f!important;background:rgba(239,225,192,.28)!important }
.tool-panel :deep(.ed-legality.legal),.tool-panel :deep(.ed-legality.forced),
.tool-panel :deep(.ed-legality.unknown),.tool-panel :deep(.ed-legality.impossible) {
  padding:2px 0 2px 8px!important;
  border-width:0 0 0 2px!important;
  border-radius:0!important;
  background:transparent!important;
  box-shadow:none!important;
}
.state-dot.live,.state-dot.stable,.target-dot { background:#6f8b72!important;box-shadow:0 0 0 3px rgba(111,139,114,.12)!important }
.titlebar-status { top:0!important;height:100%!important;padding:0 13px!important;border-width:0 1px!important;border-color:rgba(244,224,177,.18)!important;border-radius:0!important;color:#f3e8cd!important;background:rgba(255,250,230,.035)!important;box-shadow:none!important }
.titlebar-status.success { color:#dce7c7!important;border-color:rgba(207,224,177,.18)!important;background:rgba(116,139,91,.08)!important }
.tool-center-scroll { scrollbar-color:rgba(114,126,116,.48) transparent!important }
.tool-center-scroll::-webkit-scrollbar-thumb,
.tool-center-scroll:hover::-webkit-scrollbar-thumb,.tool-center-scroll:focus-within::-webkit-scrollbar-thumb {
  border:2px solid transparent!important;
  border-radius:999px!important;
  background:rgba(114,126,116,.48) padding-box!important;
  box-shadow:none!important;
}
/* Opaque paper surfaces: artwork never bleeds through controls or text. */
.sidebar { background:#f0e2c2!important }
.workspace-bar { background:#fff8e4!important }
.tool-switcher { background:#eddfc0!important }
.guide-rail { background:#f4e6c7!important }
.tool-page-heading { background:#f7ebcf!important }
.tool-center-scroll { background:transparent!important }
.art-caption { background:#f4e6c7!important;backdrop-filter:none!important }
.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.memory-card),
.tool-panel :deep(.language-panel),.tool-panel :deep(.library-card),.tool-panel :deep(.detail-panel),
.tool-panel :deep(.catalog-list),.tool-panel :deep(.quests),.calibration-card,.compat-section,.path-card,.patch-card,.backup-card {
  background:#f6e8c9!important;
}
.tool-panel :deep(input),.tool-panel :deep(select),.tool-panel :deep(textarea),.legacy-patch input,
.tool-panel :deep(.picker-selected),.tool-panel :deep(.readonly-field),.tool-panel :deep(.value-input),.tool-panel :deep(.od-select) {
  background:#fff9e8!important;
}
.tool-panel :deep(.picker-dropdown),.tool-panel :deep(.picker-list),.tool-panel :deep(select[size]),
.tool-panel :deep(.row-list),.tool-panel :deep(.catalog-list) { background:#f3e4c3!important }
.tool-panel :deep(select[size] option),.tool-panel :deep(.opt),.tool-panel :deep(.catalog-row),.tool-panel :deep(.row-item),
.tool-panel :deep(.history-row),.tool-panel :deep(.log-row),.tool-panel :deep(.record-row),.tool-panel :deep(.result-row) { background:#f8edcf!important }
.tool-panel :deep(select[size] option:nth-child(even)),.tool-panel :deep(.opt:nth-child(even)),
.tool-panel :deep(.catalog-row:nth-child(even)),.tool-panel :deep(.row-item:nth-child(even)) { background:#f0dfba!important }
.tool-panel :deep(select[size] option:checked),.tool-panel :deep(.opt:hover),.tool-panel :deep(.opt.hi),.tool-panel :deep(.opt.selected),
.tool-panel :deep(.catalog-row:hover),.tool-panel :deep(.catalog-row.on),.tool-panel :deep(.row-item:hover),.tool-panel :deep(.row-item.active) { background:#ead5a8!important }
.tool-panel :deep(.row-chip),.tool-panel :deep(.queue-item),.tool-panel :deep(.trait-card),.tool-panel :deep(.batch-row),
.tool-panel :deep(.row-head),.tool-panel :deep(.existing-row),.tool-panel :deep(.update-body),.tool-panel :deep(.od-indicator) { background:#efe0be!important }
.tool-panel :deep(.plain-btn),.tool-panel :deep(.slot-btn),.tool-panel :deep(.btn-refresh),.tool-panel :deep(.btn-sort),
.tool-panel :deep(.btn-max),.tool-panel :deep(.btn-max-all),.tool-panel :deep(.ed-max-btn),.tool-panel :deep(.ed-max-all),
.tool-panel :deep(.btn-batch),.tool-panel :deep(.row-apply),.tool-panel :deep(.mode-choice),.tool-panel :deep(.slot-choice),
.tool-panel :deep(.value-combo button),.tool-panel :deep(.qty-add) { background:#edddba!important }
.tool-panel :deep(.plain-btn:hover:not(:disabled)),.tool-panel :deep(.slot-btn:hover:not(:disabled)),
.tool-panel :deep(.btn-refresh:hover:not(:disabled)),.tool-panel :deep(.btn-sort:hover:not(:disabled)),
.tool-panel :deep(.btn-max:hover:not(:disabled)),.tool-panel :deep(.btn-max-all:hover:not(:disabled)),
.tool-panel :deep(.ed-max-btn:hover:not(:disabled)),.tool-panel :deep(.ed-max-all:hover:not(:disabled)),
.tool-panel :deep(.btn-batch:hover:not(:disabled)),.tool-panel :deep(.row-apply:hover:not(:disabled)) { background:#f8edcf!important }
.tool-panel :deep(.conn-left .chip),.tool-panel :deep(.ed-legality.legal),.tool-panel :deep(.ed-legality.forced),
.tool-panel :deep(.ed-legality.unknown),.tool-panel :deep(.ed-legality.impossible) { background:none!important }
.titlebar-status { background:#6b573d!important }
.titlebar-status.success { background:#5f6b50!important }
.art-rail .function-character .character-blend {
  opacity:.3!important;
  filter:blur(10px) saturate(.88)!important;
  transform:scale(1.022)!important;
}
.art-rail .function-character .character-main {
  filter:blur(.16px) drop-shadow(0 6px 10px rgba(66,46,22,.1)) drop-shadow(0 0 2px rgba(255,248,224,.48))!important;
}
/* Warm structural lines: cyan is reserved for information, never for borders. */
.tool-panel :deep(.slot-btn.on),.tool-panel :deep(.slot-choice.on),.tool-panel :deep(.mode-choice.on),.tool-panel :deep(.language-card.active),
.tool-panel :deep(.language-card.active .language-button),
.tool-panel :deep(.summon-row.selected),.tool-panel :deep(.row.on),.tool-panel :deep(.picker-selected:hover),
.tool-panel :deep(.picker-open .picker-selected),.tool-panel :deep(.opt:hover),.tool-panel :deep(.opt.hi),
.tool-panel :deep(.opt.selected),.tool-panel :deep(select[size] option:checked) {
  border-color:rgba(126,91,42,.42)!important;
  box-shadow:inset 3px 0 #9a7440!important;
}
.tool-panel :deep(.slot-btn.on),.tool-panel :deep(.mode-choice.on),.tool-panel :deep(.slot-choice.on),
.tool-panel :deep(.language-card.active .language-button) {
  color:#59462e!important;
  background:#ead5a8!important;
  box-shadow:inset 0 -3px #9a7440!important;
}
.tool-panel :deep(.plain-btn:hover:not(:disabled)),.tool-panel :deep(.slot-btn:hover:not(:disabled)),
.tool-panel :deep(.btn-refresh:hover:not(:disabled)),.tool-panel :deep(.btn-sort:hover:not(:disabled)),
.tool-panel :deep(.btn-max:hover:not(:disabled)),.tool-panel :deep(.btn-max-all:hover:not(:disabled)),
.tool-panel :deep(.ed-max-btn:hover:not(:disabled)),.tool-panel :deep(.ed-max-all:hover:not(:disabled)),
.tool-panel :deep(.btn-batch:hover:not(:disabled)),.tool-panel :deep(.row-apply:hover:not(:disabled)),
.tool-panel :deep(.mode-choice:hover:not(:disabled)),.tool-panel :deep(.slot-choice:hover:not(:disabled)),
.tool-panel :deep(.language-button:hover:not(:disabled)),.tool-panel :deep(.value-combo button:hover:not(:disabled)) {
  border-color:rgba(126,91,42,.48)!important;
  box-shadow:inset 3px 0 #9a7440!important;
}
.tool-panel :deep(.info-dot),.tool-panel :deep(.primary),.tool-panel :deep(.save),
.tool-panel :deep(.btn-cyan),.tool-panel :deep(.primary-btn),.tool-panel :deep(.language-button),
.tool-panel :deep(.btn-connect),.tool-panel :deep(.ed-apply-btn),.tool-panel :deep(.ed-write) {
  border-color:rgba(118,82,36,.48)!important;
}
.tool-panel :deep(.feature-help),.tool-panel :deep(.note-warn),.tool-panel :deep(.protection-note) {
  border-left-color:#9a7440!important;
}
/* Lightweight mode: page changes are immediate; only low-cost hover feedback remains. */
.workspace-scene { position:relative;width:100%;height:100%;min-height:100% }
.tool-switcher button { transition:color .1s ease,background-color .1s ease }
.nav-item .nav-mark,.nav-item .nav-arrow { transition:color .1s ease,background-color .1s ease }
.guide-rail,.tool-page-heading,.art-caption,
.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.memory-card),
.tool-panel :deep(.language-panel),.tool-panel :deep(.language-card),.tool-panel :deep(.catalog-list),.tool-panel :deep(.detail-panel),
.tool-panel :deep(.quests),.tool-panel :deep(.compat-section),.tool-panel :deep(.calibration-card),.tool-panel :deep(.path-card),
.tool-panel :deep(.patch-card),.tool-panel :deep(.backup-card) { animation:none!important;backdrop-filter:none!important }
.card-kicker,.compat-heading span { color:#765428!important }.calibration-card p,.file-meta,.compat-heading p,.matrix-row,.patch-card p,.backup-card span,.path-card>label,.detected-file,.legacy-links small { color:#665845!important }.matrix-row.head { color:#665845!important }.matrix-row b.ok { color:#2e7387!important }.matrix-row b.flow { color:#246a70!important }.matrix-row b.wait,.legacy-warning strong,.legacy-links button span { color:#80591f!important }.legacy-warning span { color:#66533d!important }
/* Readable Latin and figures: one bundled face across English UI and every data surface. */
.app-window { font-family:var(--font-ui) }
.titlebar-title,.sidebar-heading .sidebar-kicker,.sidebar-heading strong,.nav-copy strong,.nav-arrow,
.card-kicker,.calibration-card>strong,.compat-heading h2,.compat-heading span,
.tool-page-heading h1,.tool-page-heading .eyebrow,.sidebar-collapse {
  font-family:var(--font-ui)!important;
  font-weight:800!important;
}
.build-chip,.matrix-row,.patch-state,.file-meta,.detected-file,
.tool-panel :deep(input[type="number"]),.tool-panel :deep(input[inputmode="numeric"]),
.tool-panel :deep(input[inputmode="decimal"]),.tool-panel :deep(code),
.tool-panel :deep(.count),.tool-panel :deep(.pid),.tool-panel :deep(.memory-bytes),
.tool-panel :deep(.currency-meta),.tool-panel :deep(.damage-meter-value),
.tool-panel :deep(.damage-meter-raw),.tool-panel :deep(.burst-timer),
.tool-panel :deep(.ed-current-lv),.tool-panel :deep(.ed-level),
.tool-panel :deep(.ed-level-hint),.tool-panel :deep(.row-name-lv),
.tool-panel :deep(.row-chip-lv),.tool-panel :deep(.tab-count),
.tool-panel :deep(.slot),.tool-panel :deep(.rank),.tool-panel :deep(.id),
.tool-panel :deep(.version-label),.tool-panel :deep(.catalog-row em) {
  font-family:var(--font-data)!important;
  font-weight:750!important;
  font-variant-numeric:tabular-nums lining-nums;
  letter-spacing:.018em;
}
.tool-panel :deep(input[type="number"]),.tool-panel :deep(input[inputmode="numeric"]),
.tool-panel :deep(input[inputmode="decimal"]),.tool-panel :deep(.damage-meter-value),
.tool-panel :deep(.count),.tool-panel :deep(.ed-level) { font-weight:800!important }
/* Relink-like typography pass: softer CJK shapes, restrained hierarchy, sturdy figures. */
.app-window,.sidebar,.workspace-bar,.tool-switcher,.guide-rail,.tool-center-scroll,.tool-panel,
.tool-panel :deep(*) { font-family:var(--font-ui)!important;text-rendering:optimizeLegibility }
.titlebar-title,.sidebar-heading strong,.nav-copy strong,.guide-heading span,
.tool-page-heading h1,.tool-panel :deep(h2),.tool-panel :deep(h3),
.tool-panel :deep(.section-title),.tool-panel :deep(.editor-title),.compat-heading h2,
.calibration-card>strong { font-weight:700!important;letter-spacing:0!important }
.sidebar-heading .sidebar-kicker,.card-kicker,.tool-page-heading .eyebrow,
.tool-panel :deep(label),.tool-panel :deep(.field label),.guide-caution span { font-weight:600!important;letter-spacing:.02em!important }
.nav-copy strong,.tool-switcher button,.guide-steps li,.guide-caution p,.note-bubble p,
.tool-panel :deep(p),.tool-panel :deep(span),.tool-panel :deep(small) { font-weight:560 }
.tool-panel :deep(button),.action,.legacy-links button { font-weight:650!important }
.build-chip,.matrix-row,.patch-state,.file-meta,.detected-file,
.tool-panel :deep(input[type="number"]),.tool-panel :deep(input[inputmode="numeric"]),
.tool-panel :deep(input[inputmode="decimal"]),.tool-panel :deep(code),
.tool-panel :deep(.count),.tool-panel :deep(.pid),.tool-panel :deep(.memory-bytes),
.tool-panel :deep(.currency-meta),.tool-panel :deep(.damage-meter-value),
.tool-panel :deep(.damage-meter-raw),.tool-panel :deep(.burst-timer),
.tool-panel :deep(.ed-current-lv),.tool-panel :deep(.ed-level),
.tool-panel :deep(.ed-level-hint),.tool-panel :deep(.row-name-lv),
.tool-panel :deep(.row-chip-lv),.tool-panel :deep(.tab-count),
.tool-panel :deep(.slot),.tool-panel :deep(.rank),.tool-panel :deep(.id),
.tool-panel :deep(.version-label),.tool-panel :deep(.catalog-row em) {
  font-family:var(--font-data)!important;font-weight:680!important;letter-spacing:0!important
}
.guide-steps li { font-size:11px!important;line-height:1.55!important }
.guide-caution p { font-size:10.5px!important;line-height:1.6!important }
.tool-page-heading h1 { font-size:20px!important }
.tool-page-heading p { width:100%;max-width:none;font-size:11px!important;font-weight:560!important }
.tool-panel :deep(input),.tool-panel :deep(select),.tool-panel :deep(textarea),
.tool-panel :deep(.picker-selected),.tool-panel :deep(.catalog-trigger) { font-size:12px!important;font-weight:560!important }
.tool-panel :deep(.field label),.tool-panel :deep(label) { font-size:10.5px!important;line-height:1.45!important }
.tool-panel :deep(.field label small),.tool-panel :deep(label small) { font-size:9px!important;font-weight:500!important }
.tool-panel :deep(input[type="number"]) { font-size:13px!important;font-weight:680!important }
.tool-panel :deep(.level-field) { width:105px!important }
.tool-panel :deep(.compact-number) { width:78px!important;min-height:31px!important }
.tool-panel :deep(.weapon-fields .number-combo) { grid-template-columns:minmax(64px,82px) 42px!important;justify-content:start!important }
.tool-panel :deep(.weapon-fields .field) { min-width:0!important }
.tool-panel :deep(.weapon-fields input[type="number"]) { width:100%!important }
.calibration-card { min-height:126px!important;padding:13px!important }
.calibration-card>strong { margin-top:7px!important;font-size:17px!important;line-height:1.28!important;overflow-wrap:anywhere }
.calibration-card p { height:auto!important;min-height:31px;margin:5px 0!important;font-size:10px!important;font-weight:520!important }
.card-actions { margin-top:7px!important }
.card-actions .action { min-height:29px!important;padding:5px 9px!important;font-size:10px!important }
.tool-panel :deep(.save-info) { display:inline-flex!important;align-items:center!important;align-self:flex-start!important;min-height:0!important;padding:2px 0!important;border:0!important;border-radius:0!important;color:#5f6c51!important;background:transparent!important;font-size:10px!important;font-weight:600!important }
.tool-panel :deep(.legality) { max-width:100%; }
.tool-panel :deep(.legality .text strong) { font-size:10px!important;font-weight:650!important }
.tool-panel :deep(.legality .text small) { color:#75654f!important;font-size:9px!important;font-weight:520!important }
.tool-panel :deep(.language-button) {
  color:#5b4930!important;border-color:rgba(126,91,42,.34)!important;background:#ead8b2!important;text-shadow:none!important
}
.tool-panel :deep(.language-button:hover:not(:disabled)) {
  color:#fff9e9!important;background:#8b6737!important
}
.tool-panel :deep(.language-button:disabled),.tool-panel :deep(.language-card.active .language-button) {
  color:#59462e!important;border-color:rgba(126,91,42,.4)!important;background:#e1c995!important;text-shadow:none!important
}
/* Final warm-material pass: structural accents are parchment, walnut and brass. */
.nav-item:hover,.nav-item.active { color:#523f28!important;border-color:rgba(130,91,39,.46)!important;background:linear-gradient(90deg,#ead6a7,#f7edcf)!important;box-shadow:inset 3px 0 #9b7137!important }
.nav-item.active .nav-mark,.note-bubble b { color:#755329!important;border-color:rgba(127,88,37,.48)!important }
.tool-switcher button.active::after,.tool-panel :deep(.section-tabs button.on),.tool-panel :deep(.mini-tabs button.on),.tool-panel :deep(.slot-btn.on) { border-color:#98703a!important;box-shadow:inset 0 -2px #98703a!important }
.tool-panel :deep(input:focus),.tool-panel :deep(select:focus),.tool-panel :deep(textarea:focus),.path-input-row input:focus { border-color:#9b7137!important;outline:1px solid rgba(155,113,55,.2)!important }
.tool-panel :deep(.catalog-row.on),.tool-panel :deep(.summon-row.selected) { border-color:rgba(139,97,43,.38)!important;background:#ead7ae!important;box-shadow:inset 3px 0 #936934!important }
.tool-panel :deep(.catalog-row:hover),.tool-panel :deep(.summon-row:hover) { background:#f2e4c5!important }
.tool-center-scroll,.workspace-scroll,.tool-panel :deep(.catalog-list),.tool-panel :deep(.list),.tool-panel :deep(select) { scrollbar-width:thin;scrollbar-color:rgba(137,96,44,.62) transparent }
.tool-center-scroll::-webkit-scrollbar,.workspace-scroll::-webkit-scrollbar,.tool-panel :deep(.catalog-list::-webkit-scrollbar),.tool-panel :deep(.list::-webkit-scrollbar),.tool-panel :deep(select::-webkit-scrollbar) { width:7px;height:7px }
.tool-center-scroll::-webkit-scrollbar-track,.workspace-scroll::-webkit-scrollbar-track,.tool-panel :deep(.catalog-list::-webkit-scrollbar-track),.tool-panel :deep(.list::-webkit-scrollbar-track),.tool-panel :deep(select::-webkit-scrollbar-track) { background:transparent }
.tool-center-scroll::-webkit-scrollbar-thumb,.workspace-scroll::-webkit-scrollbar-thumb,.tool-panel :deep(.catalog-list::-webkit-scrollbar-thumb),.tool-panel :deep(.list::-webkit-scrollbar-thumb),.tool-panel :deep(select::-webkit-scrollbar-thumb) { border:2px solid transparent;border-radius:999px;background-clip:padding-box;background-color:rgba(137,96,44,.62) }
.backup-policy{display:grid;grid-template-columns:1fr 1fr;gap:6px;min-width:260px}.backup-policy button{min-height:42px;padding:6px 10px;text-align:left;color:#735f43;border:1px solid rgba(132,94,43,.24);background:#f4e7c7;cursor:pointer}.backup-policy button.active{color:#fff9e8;border-color:#755126;background:linear-gradient(180deg,#ad8245,#825b2d);box-shadow:inset 0 1px rgba(255,255,255,.3)}.backup-policy b,.backup-policy small{display:block}.backup-policy b{font-size:10px}.backup-policy small{margin-top:3px;font-size:8.5px;opacity:.78}
/* Final flat-control cleanup for generated editors. */
.tool-panel :deep(.capacity){display:inline!important;min-height:0!important;padding:0!important;border:0!important;border-radius:0!important;color:#75654d!important;background:transparent!important;box-shadow:none!important;font-size:9px!important;font-weight:650!important}
.tool-panel :deep(.weapon-kind){border-color:rgba(133,96,44,.26)!important;border-left:3px solid #8b6737!important;border-radius:0!important;background:#edddba!important;box-shadow:none!important}.tool-panel :deep(.weapon-kind b){color:#6d4f27!important}.tool-panel :deep(.weapon-kind span){color:#6d604e!important}
.tool-panel :deep(input:hover),.tool-panel :deep(input:focus),.tool-panel :deep(select:hover),.tool-panel :deep(select:focus),.tool-panel :deep(textarea:hover),.tool-panel :deep(textarea:focus){background-color:#fdf6e4!important}
.tool-panel :deep(input[type=number]::-webkit-inner-spin-button),.tool-panel :deep(input[type=number]::-webkit-outer-spin-button){-webkit-appearance:none!important;margin:0!important}.tool-panel :deep(input[type=number]){-moz-appearance:textfield!important}
.tool-panel :deep(.btn-purple),.tool-panel :deep(.add-btn),.tool-panel :deep(.add-btn:disabled){border:1px solid #9a7440!important;border-radius:1px!important;color:#5e4c34!important;background:#ead8b2!important;box-shadow:none!important;text-shadow:none!important;opacity:1!important}.tool-panel :deep(.btn-purple:hover:not(:disabled)),.tool-panel :deep(.add-btn:hover:not(:disabled)){color:#fff9e9!important;background:#8b6737!important}.tool-panel :deep(.add-btn:disabled){color:#8f7a5c!important;border-color:rgba(154,116,64,.38)!important;background:#e7d8b6!important}
/* Wide reading column: the default 1120px window must not collapse editors to 460px. */
.tool-stage{grid-template-columns:180px minmax(0,700px) minmax(190px,1fr);gap:10px}
.tool-switcher{overflow-x:auto;scrollbar-width:none}.tool-switcher::-webkit-scrollbar{display:none}
.tool-switcher button{flex:0 0 auto;font-size:11.5px!important;font-weight:700!important;white-space:nowrap}
.guide-rail{border-radius:2px!important;box-shadow:none!important}
.note-bubble{border-radius:2px!important;background:#fff9e8!important;box-shadow:none!important}
.note-bubble::after{box-shadow:none!important}
.guide-sticker{filter:none!important}
.tool-panel :deep(.level-field){width:180px!important;max-width:100%!important}
.tool-panel :deep(.level-field>label){display:flex!important;align-items:baseline!important;gap:7px!important;white-space:nowrap!important}
.tool-panel :deep(.level-field>label small){white-space:nowrap!important}
.tool-panel :deep(.ed-edit-line){grid-template-columns:minmax(0,1.35fr) 118px 74px!important;gap:9px!important}
.tool-panel :deep(.ed-level-control){grid-template-columns:30px minmax(0,1fr)!important;grid-template-rows:25px 15px!important}
.tool-panel :deep(.ed-level-control small){font-size:8.5px!important;white-space:nowrap!important;text-align:left!important}
.tool-panel[data-tool="progression"] :deep(.workspace){grid-template-columns:minmax(230px,1fr) minmax(300px,.95fr)!important}
.tool-panel[data-tool="progression"] :deep(.weapon-name-line strong){overflow:hidden!important;text-overflow:ellipsis!important;white-space:nowrap!important}
.tool-panel[data-tool="progression"] :deep(.detail-panel){min-width:0!important}
.tool-panel[data-tool="progression"] :deep(.weapon-fields){grid-template-columns:repeat(2,minmax(0,1fr))!important}
.tool-panel[data-tool="chara"] :deep(.root),.tool-panel[data-tool="save"] :deep(.root){max-width:none!important}
.tool-panel[data-tool="chara"] :deep(.batch-row){display:grid!important;grid-template-columns:auto 110px 64px 88px minmax(70px,1fr) auto!important;gap:8px!important}
.tool-panel[data-tool="chara"] :deep(.selection){margin-left:0!important;white-space:nowrap!important}
.calibration-grid{grid-template-columns:repeat(auto-fit,minmax(220px,1fr))!important}
.calibration-grid>.calibration-card:last-child{grid-column:1/-1}
.calibration-card>strong{font-size:17px!important;line-height:1.25!important;overflow:hidden!important;text-overflow:ellipsis!important;white-space:nowrap!important;overflow-wrap:normal!important}
.calibration-card p{overflow-wrap:anywhere}
.backup-card{display:grid!important;grid-template-columns:minmax(0,1fr) auto auto!important;align-items:center!important;gap:9px!important}
.backup-card>div:first-child{grid-column:1/-1!important;display:flex!important;align-items:baseline!important;justify-content:space-between!important;gap:12px!important}
.backup-card>div:first-child strong,.backup-card>div:first-child span{margin:0!important;white-space:nowrap!important}
.backup-policy{min-width:280px!important}.backup-policy button,.backup-policy button.active{border-radius:1px!important;background:#edddba!important;box-shadow:none!important}.backup-policy button.active{color:#fff9e8!important;border-color:#765126!important;background:#8b6737!important}
.legacy-warning{border-radius:2px!important}.path-card,.patch-card,.backup-card{border-radius:2px!important;box-shadow:none!important}
/*
 * Portrait optical calibration.
 * Every source is a 2048px square, but the character occupies a different
 * percentage of that canvas.  Calibrate the visible figure instead of giving
 * every image the same CSS height; this keeps heads aligned and crops near the
 * thigh at both the default and compact window sizes.
 */
/* 每页立绘旋钮（--ah 大小 / --ay 上下:越负越往上顶且越裁下方 / --ax 左右:越负越靠右），逐页在真实尺寸下校准： */
.tool-stage[data-tool="progression"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="sigilMemory"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="loadout"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="summon"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="overlimit"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="runtime"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="sigil"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="wrightstone"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="chara"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="save"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="compatibility"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="legacyRuntime"]{--ah:182%;--ay:-52%;--ax:-250px}
.tool-stage[data-tool="monster"]{--ah:160%;--ay:-63%;--ax:-165px}
.tool-stage[data-tool="patch"]{--ah:160%;--ay:-63%;--ax:-250px}
.tool-stage[data-tool="language"]{--ah:178%;--ay:-61%;--ax:-300px}
/* Quantity is a compact form row, not a raised card. */
.tool-panel :deep(.qty-add),.tool-panel :deep(.qty-add:hover){padding:0!important;border:0!important;border-radius:0!important;background:transparent!important;box-shadow:none!important}
.tool-panel :deep(.quantity-combo button){min-width:50px!important;border:1px solid #9a7440!important;border-radius:1px!important;color:#5e4c34!important;background:#edddba!important;box-shadow:none!important;opacity:1!important}
.tool-panel :deep(.quantity-combo button:hover){color:#fff9e9!important;background:#8b6737!important}
.tool-panel :deep(.add-btn:not(:disabled)){border-color:#765126!important;color:#fff9e9!important;background:#8b6737!important}
.tool-panel :deep(.add-btn:not(:disabled):hover){background:#76552d!important}
@media(max-width:1320px){.tool-stage{grid-template-columns:165px minmax(0,640px) minmax(170px,1fr);gap:10px}.art-rail .function-character{left:-105px;right:0;top:0;bottom:0}.guide-rail{padding:15px 12px}.guide-steps li{font-size:10px}.tool-page-heading{padding:14px 17px 13px}.tool-page-heading h1{font-size:21px}.tool-panel :deep(.section){padding:13px 15px!important}}
@media(max-width:1120px){.app-body{grid-template-columns:170px minmax(0,1fr)}.tool-stage{grid-template-columns:145px minmax(0,530px) minmax(230px,1fr);gap:8px}.art-rail .function-character{left:-100px;right:0;top:0;bottom:0}.guide-character-note{height:205px}.guide-sticker{width:120px;height:128px}.workspace-scroll.tool-workspace{padding-left:12px}.guide-heading span{font-size:13px}.guide-caution{padding:8px}.note-bubble{padding:8px 9px}.tool-page-heading h1{font-size:20px}.tool-switcher{gap:8px}.tool-switcher button{padding:0 7px!important;font-size:11px!important}}
@media(max-width:1000px){.app-body{grid-template-columns:70px minmax(0,1fr)}.sidebar{padding:13px 8px}.sidebar-heading,.nav-copy,.nav-arrow,.sidebar-foot{display:none!important}.sidebar-collapse{display:none!important}.primary-nav{align-items:center;padding-top:45px;gap:9px}.nav-item{width:48px;min-height:48px;display:grid;grid-template-columns:1fr;place-items:center;padding:5px!important}.nav-mark{width:30px;height:30px}.tool-stage{grid-template-columns:130px minmax(0,500px) minmax(235px,1fr);gap:8px}.art-rail .function-character{left:-90px}.tool-page-heading{padding:14px 16px 13px}.tool-page-heading h1{font-size:20px}.tool-page-heading p{font-size:10.5px}.tool-switcher{gap:5px;padding-left:8px!important;padding-right:8px!important}.tool-switcher button{padding:0 5px!important;font-size:10.5px!important}.tool-panel[data-tool="progression"] :deep(.workspace){grid-template-columns:1fr!important}.tool-panel[data-tool="chara"] :deep(.batch-row){grid-template-columns:auto 1fr auto auto!important}.tool-panel[data-tool="chara"] :deep(.selection),.tool-panel[data-tool="chara"] :deep(.save-btn){grid-column:span 2}.backup-card{grid-template-columns:1fr 1fr!important}.backup-policy{grid-column:1/-1!important;min-width:0!important}}
@media(prefers-reduced-motion:reduce){.app-window *,.app-window *::before,.app-window *::after{scroll-behavior:auto!important;animation-duration:.001ms!important;animation-delay:0ms!important;transition-duration:.001ms!important;transition-delay:0ms!important}}
</style>

<style scoped>
/* ═══════════ UI Polish · 布局优化 & 史诗感（追加覆盖，后写生效） ═══════════ */

/* 1. 立绘可折叠：折叠态收起立绘列，展开态沿用原响应式网格（见 1651 及媒体查询），
   不再钳死立绘列宽——否则各页按“宽列”手调的立绘偏移会被推出屏幕外裁断。 */
.tool-stage.art-collapsed { grid-template-columns: 180px minmax(0,1fr) 0 !important; }
.tool-stage.art-collapsed .art-rail { display:none; }

/* 立绘折叠开关：贴右上角 */
.art-toggle { position:absolute; z-index:9; top:5px; right:4px; width:26px; height:26px; display:grid; place-items:center;
  border:1px solid rgba(143,106,51,.36); border-radius:8px 4px 8px 4px; background:rgba(255,250,232,.92);
  color:#7d6440; font:900 15px/1 "Microsoft YaHei UI",sans-serif; cursor:pointer;
  box-shadow:0 3px 9px rgba(85,61,27,.12); transition:.16s ease; }
.art-toggle:hover { border-color:var(--gold); color:#57400f; background:#fffdf5; box-shadow:0 4px 14px rgba(167,125,61,.28); }

/* 2. 顶部子导航：两端渐隐 + 真正开启横向滚动（标签多时可滑动到，不再被裁掉够不到） */
.tool-switcher { overflow-x:auto; overflow-y:hidden; scrollbar-width:none;
  -webkit-mask-image:linear-gradient(90deg,transparent,#000 22px,#000 calc(100% - 22px),transparent);
  mask-image:linear-gradient(90deg,transparent,#000 22px,#000 calc(100% - 22px),transparent); }
.tool-switcher::-webkit-scrollbar { height:0; }
.tool-switcher button { flex:0 0 auto; }

/* 3. 史诗感：页面标题——金质渐变字 + 顶部烫金线 + 角落光晕 */
.tool-page-heading { overflow:hidden; box-shadow:0 11px 26px rgba(84,61,28,.13), inset 0 0 0 1px rgba(255,255,255,.42), inset 0 2px 0 rgba(214,182,111,.44); }
.tool-page-heading::before { content:""; position:absolute; left:0; right:0; top:0; height:3px;
  background:linear-gradient(90deg,transparent,var(--gold-soft),var(--gold),var(--gold-soft),transparent); opacity:.9; }
.tool-page-heading::after { content:""; position:absolute; right:-46px; top:-46px; width:160px; height:160px; pointer-events:none;
  background:radial-gradient(circle at center,rgba(215,182,111,.18),transparent 65%); }
.tool-page-heading .eyebrow { letter-spacing:.2em; }
.tool-page-heading .eyebrow::before { content:"◆ "; color:var(--gold-soft); font-size:9px; }
.tool-page-heading h1 { font-size:29px; letter-spacing:.045em;
  background:linear-gradient(176deg,#4c4031 10%,#7a5a29 94%); -webkit-background-clip:text; background-clip:text; -webkit-text-fill-color:transparent; }

/* 4. 面板景深 + 金质内边（更干脆的可操作区边界）*/
.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.language-panel),.tool-panel :deep(.library-card),.tool-panel :deep(.detail-panel),.tool-panel :deep(.quests),.tool-panel :deep(.calibration-card) {
  box-shadow:0 9px 22px rgba(77,54,25,.12), inset 0 0 0 1px rgba(255,255,255,.34), inset 0 1px 0 rgba(214,182,111,.32)!important; }
.calibration-card,.compat-section { box-shadow:0 9px 22px rgba(77,54,25,.12), inset 0 0 0 1px rgba(255,255,255,.34), inset 0 1px 0 rgba(214,182,111,.32)!important; }

/* 5. 主/高危按钮：hover 金光，强化主次层级 */
.tool-panel :deep(.primary-btn),.tool-panel :deep(.apply-btn),.tool-panel :deep(.ed-apply-btn),.tool-panel :deep(.write-btn),.tool-panel :deep(.ed-write),.tool-panel :deep(.save-btn),.tool-panel :deep(.add-btn),.tool-panel :deep(.language-button) {
  box-shadow:0 4px 13px rgba(35,100,107,.26)!important; }
.tool-panel :deep(.primary-btn:hover:not(:disabled)),.tool-panel :deep(.apply-btn:hover:not(:disabled)),.tool-panel :deep(.write-btn:hover:not(:disabled)),.tool-panel :deep(.ed-apply-btn:hover:not(:disabled)),.tool-panel :deep(.save-btn:hover:not(:disabled)),.tool-panel :deep(.add-btn:hover:not(:disabled)) {
  box-shadow:0 6px 20px rgba(52,157,166,.42), 0 0 0 1px rgba(214,182,111,.5)!important; }

/* 6. 版本适配矩阵：状态语义色胶囊 */
.matrix-row b.ok,.matrix-row b.flow,.matrix-row b.wait { padding:2px 10px; border-radius:999px; font-weight:800; }
.matrix-row b.ok { color:#2f6a4c!important; background:rgba(76,156,118,.18); }
.matrix-row b.flow { color:#256e74!important; background:rgba(58,169,179,.18); }
.matrix-row b.wait { color:#8a5f1e!important; background:rgba(183,130,55,.2); }

/* 7. 大段说明行高提升，改善可读性 */
.tool-panel :deep(p),.tool-panel :deep(.hint),.tool-panel :deep(.feature-help),.tool-panel :deep(.memory-hint),.tool-panel :deep(.memory-info),.tool-panel :deep(.custom-note-text),.tool-panel :deep(.compatibility-note),.tool-panel :deep(.save-hint) { line-height:1.62!important; }

/* 8. 侧栏选中态更明确（左侧金边）*/
.nav-item.active { box-shadow:inset 3px 0 0 var(--gold)!important; }
</style>

<style scoped>
/* ═══════════ UI Polish · 第二轮微调（据 Gemini 复审）═══════════ */
/* 输入框/下拉框：羊皮纸底 + 暗金细边 + 内阴影，融入复古面板 */
.tool-panel :deep(input:not([type=checkbox]):not([type=radio])),.tool-panel :deep(select),.tool-panel :deep(textarea) {
  background:linear-gradient(180deg,rgba(247,238,214,.92),rgba(239,227,198,.88))!important;
  border-color:rgba(150,112,52,.44)!important; box-shadow:inset 0 1px 2px rgba(96,68,28,.1)!important; }
.tool-panel :deep(input:not([type=checkbox]):not([type=radio]):focus),.tool-panel :deep(select:focus),.tool-panel :deep(textarea:focus) {
  border-color:var(--gold)!important; box-shadow:inset 0 1px 2px rgba(96,68,28,.1), 0 0 0 2px rgba(214,182,111,.3)!important; }
/* 次级按钮 hover 金光，提升点击游戏感 */
.tool-panel :deep(.btn-batch:hover),.tool-panel :deep(.btn-sort:hover),.tool-panel :deep(.btn-max:hover),.tool-panel :deep(.btn-refresh:hover),.tool-panel :deep(.slot-btn:hover),.tool-panel :deep(.plain-btn:hover),.tool-panel :deep(.btn-purple:hover) {
  border-color:var(--gold-soft)!important; box-shadow:0 3px 12px rgba(167,125,61,.26)!important; }
/* 子导航当前项加粗，强化定位 */
.tool-switcher button.active { font-weight:900!important; }
</style>

<style scoped>
/* ═══════════ UI Polish · 第三轮（空态柔化 + 按钮材质 + 双列配对）═══════════ */
/* 空态/错误：暗朱红 + 复古图标 + 柔和底，降低突兀感 */
.tool-panel :deep(.data-error) { display:inline-flex!important; align-items:center; gap:5px; margin:3px 0 0; padding:5px 11px;
  font-size:.72rem!important; font-weight:800!important; color:#9c4b3f!important;
  border-radius:0 7px 7px 0; border-left:3px solid #c06a5f; background:linear-gradient(90deg,rgba(192,106,95,.13),rgba(192,106,95,.03)); }
.tool-panel :deep(.data-error)::before { content:"⚠"; font-size:.86rem; }
/* 次级按钮材质渐变，去扁平 */
.tool-panel :deep(.btn-batch),.tool-panel :deep(.btn-sort),.tool-panel :deep(.btn-max),.tool-panel :deep(.plain-btn),.tool-panel :deep(.btn-refresh),.tool-panel :deep(.slot-btn),.tool-panel :deep(.btn-purple),.tool-panel :deep(.quantity-combo button) {
  background:linear-gradient(180deg,#f4e7c9,#e8d7b2)!important; }
/* 特性 + 等级 双列配对行 */
.tool-panel :deep(.field-row) { display:flex; gap:12px; align-items:flex-end; }
.tool-panel :deep(.field-row) > .field:first-child { flex:1; min-width:0; }
.tool-panel :deep(.field-row) > .level-field { flex:0 0 auto; }
@media(max-width:900px){ .tool-panel :deep(.field-row){ flex-direction:column; gap:13px; align-items:stretch; } .tool-panel :deep(.field-row) > .level-field{ width:100%; } }
</style>

<style scoped>
/* ═══════════ UI Polish · 第四轮（交互态深审）═══════════ */
/* 超出上限的等级输入框整体变红，警示更明确 */
.tool-panel :deep(input.lv-over) { border-color:#c0574c!important;
  background:linear-gradient(180deg,rgba(214,120,110,.18),rgba(214,120,110,.08))!important;
  color:#9c3f34!important; box-shadow:inset 0 1px 2px rgba(150,50,40,.14),0 0 0 2px rgba(192,87,76,.2)!important; }
/* 下拉列表：斑马纹对比更清晰 + hover 金边高亮 + 搜索区分割线与悬浮阴影 */
/* 下拉框统一到 CatalogSelect 组件内的重设计配色（见 CatalogSelect.vue），此处不再用杂色覆盖 */
.tool-panel :deep(.catalog-option:nth-child(even)) { background:rgba(154,116,64,.035)!important; }
.tool-panel :deep(.catalog-option:hover),.tool-panel :deep(.catalog-option.highlight),.tool-panel :deep(.catalog-option.selected) { background:#efe1c0!important; box-shadow:inset 3px 0 #9a7440!important; }
.tool-panel :deep(.catalog-search) { border-bottom:1px solid rgba(154,116,64,.15)!important; }
/* 成功提示更贴合暖色主题（柔和青绿，去亮绿）*/
.titlebar-status.success { color:#7fd0b0!important; }

/* 第五轮修正：默认 1120 窗口下 8 个子标签 + 离线/实时徽标要能不横滑地整排展示。
   1040–1200 这个区间侧栏满宽又未触发紧凑样式，收紧标签间距/内边距/徽标尺寸补足 56px 缺口。 */
@media(max-width:1240px){
  .tool-switcher { gap:2px!important; }
  .tool-switcher button { padding:0 6px!important; gap:4px!important; }
  .switcher-tag { padding:1px 4px!important; font-size:7px!important; }
}

/* ═══ 立绘呈现（逐页校准框架）═══
   贴右下角、半身(截到大腿)、脸在上不被截、够大、右缘抵右不留空。
   每页三个旋钮：--ah 大小(高度%)、--ay 上下(bottom%，越负越往上顶且越裁下方)、--ax 左右(px，负=更靠右)。 */
/* 立绘列 = 固定的右侧保留区(足够容下角色的头/脸，不被内容卡片盖住)；中间内容自适应剩余空间。
   立绘尺寸恒定、靠右；内容 UI 随窗口自适应，窄时靠各组件自身重排（见下方自适应规则）。 */
.tool-stage { grid-template-columns:146px minmax(0,1fr) clamp(300px,25vw,450px)!important; }
/* 立绘在内容之下(背景层)，被不透明卡片盖住的左半身没关系；立绘列给足宽度让"头/脸"
   落在立绘列内(不被卡片盖住)，只有身体左侧渗入内容被盖。立绘列不裁剪，整体由
   .tool-stage overflow:hidden 兜底裁在工作区内。 */
.art-rail { overflow:visible!important; }
.art-rail .function-character { position:absolute!important; inset:0!important; left:0!important; right:0!important; top:0!important; bottom:0!important; }
.art-rail .function-character img,
.art-rail .function-character .character-main,
.art-rail .function-character .character-blend {
  position:absolute!important; right:var(--ax,0px)!important; bottom:var(--ay,-40%)!important; top:auto!important; left:auto!important;
  height:var(--ah,175%)!important; width:auto!important; max-width:none!important; max-height:none!important;
  object-fit:contain!important; object-position:right bottom!important; transform:none!important;
  /* 左侧渐隐：向左渗入内容的部分柔和淡出，不再硬盖住卡片，右侧主体清晰 */
  -webkit-mask-image:linear-gradient(90deg,transparent 0%,#000 40%)!important;
  mask-image:linear-gradient(90deg,transparent 0%,#000 40%)!important;
}

/* 祝福页特性行去掉深色嵌套框，改成和因子页一致的浅色扁平行（透明底、无边框，仅靠间距分隔） */
.tool-panel :deep(.trait-card) { background:transparent!important; border-color:transparent!important; box-shadow:none!important; padding:6px 0!important; }

/* 两个下拉组件(CatalogSelect / SigilMemoryPicker)的文字统一放大到 13px，更醒目且彼此一致 */
.tool-panel :deep(.catalog-trigger),.tool-panel :deep(.catalog-value),.tool-panel :deep(.catalog-placeholder),.tool-panel :deep(.catalog-option),.tool-panel :deep(.picker-selected),.tool-panel :deep(.picker-label),.tool-panel :deep(.picker-placeholder),.tool-panel :deep(.opt),.tool-panel :deep(.opt-name) { font-size:13px!important; }
.tool-panel :deep(.catalog-option small),.tool-panel :deep(.opt-max) { font-size:11px!important; }

/* 内容卡片全不透明，渗入的立绘/背景不会透出来使文字发糊；
   去掉白色内描边(inset 白线在羊皮纸上像奇怪的白边)，只留柔和外阴影 */
.tool-panel :deep(.section),.tool-panel :deep(.save-card),.tool-panel :deep(.editor-card),.tool-panel :deep(.memory-card),.tool-panel :deep(.language-panel),.tool-panel :deep(.library-card),.tool-panel :deep(.detail-panel),.tool-panel :deep(.catalog-list),.tool-panel :deep(.quests),.tool-panel :deep(.calibration-card),.tool-panel :deep(.compat-section),
.legacy-patch .path-card,.legacy-patch .patch-card,.legacy-patch .backup-card,.legacy-patch .legacy-warning {
  background:linear-gradient(135deg,#fdf6e4,#efe1c0)!important; backdrop-filter:none!important;
  box-shadow:0 4px 12px rgba(77,54,25,.07)!important;
}
/* 备份页输入框改为不透明，立绘不再透出 */
.legacy-patch input,.tool-panel :deep(input:not([type=checkbox]):not([type=radio])),.tool-panel :deep(textarea) { background:#fbf3dd!important; }

/* ═══ 响应式：窄到放不下时，立绘让位，保证“教程 + 内容”可读（教程列给足宽度，不再被压成竖排单字）═══ */
@media(max-width:1040px){
  .tool-stage { grid-template-columns:168px minmax(0,1fr)!important; }
  .art-rail,.art-toggle { display:none!important; }
}
@media(max-width:780px){
  .tool-stage { grid-template-columns:minmax(0,1fr)!important; }
  .guide-rail { display:none!important; }
}
</style>
