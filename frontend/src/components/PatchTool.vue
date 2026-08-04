<script setup>
import { reactive, ref, computed, defineAsyncComponent, nextTick, onBeforeUnmount, onMounted, watch } from 'vue'
import {
  AutoDetect, SetExePath, GetStatus, BackupFile, RestoreFile,
  GetAppVersion, CheckUpdate, GetNaturalDropStartupRecoveryStatus, GetRuntimeCompanionSummary, OpenReleasePage,
} from '../../wailsjs/go/backend/App'
import {
  ClipboardSetText,
  EventsOn,
  WindowMinimise,
  WindowToggleMaximise,
  Quit,
} from '../../wailsjs/runtime/runtime'
import HomeJournal from './HomeJournal.vue'
import SaveBackupDrawer from './SaveBackupDrawer.vue'
import { language, translateText } from '../i18n'
import { functionAssetManifest } from '../generated/functionAssetManifest.js'
import { beginPerformanceMeasure } from '../performanceMonitor.js'
import { translateRuntimeCompatibilityError } from '../runtimeCompatibilityErrors.js'
import weaponMemoryLilithArt from '../assets/gbfr/cutouts/weapon-memory-lilith-generated.webp'

const pageLoaders = Object.freeze({
  progression: () => import('./ProgressionEditor.vue'),
  sigil: () => import('./SigilGenerator.vue'),
  sigilMemory: () => import('./SigilMemoryGenerator.vue'),
  loadout: () => import('./SigilLoadoutRestore.vue'),
  loadoutPresets: () => import('./LoadoutViewer.vue'),
  wrightstone: () => import('./WrightstoneGenerator.vue'),
  summonSave: () => import('./SummonSaveEditor.vue'),
  wrightstoneMemory: () => import('./WrightstoneMemoryGenerator.vue'),
  weaponMemory: () => import('./WeaponMemoryGenerator.vue'),
  summon: () => import('./SummonEditor.vue'),
  overlimit: () => import('./OverLimit.vue'),
  runtime: () => import('./MiscTools.vue'),
  chara: () => import('./CharaStats.vue'),
  save: () => import('./SaveEditor.vue'),
  monster: () => import('./MonsterEnhance.vue'),
  patchCombat: () => import('./RuntimePatchFeatures.vue'),
  patchCharacters: () => import('./RuntimePatchFeatures.vue'),
  patchQuest: () => import('./RuntimePatchFeatures.vue'),
  runtimeMonitor: () => import('./RuntimePatchMonitor.vue'),
  formulaSampler: () => import('./FormulaSampler.vue'),
  saveDiff: () => import('./SaveDiffLab.vue'),
  naturalDrop: () => import('./NaturalDropLab.vue'),
  audioMixer: () => import('./AudioMixerLab.vue'),
  camera: () => import('./CameraLab.vue'),
  virtualSigils: () => import('./VirtualSigilLab.vue'),
	runtimeQOL: () => import('./RuntimeQOLLab.vue'),
  language: () => import('./LanguageSettings.vue'),
})

const loadedPageModules = new Map()

function loadPageModule(id) {
  const loader = pageLoaders[id]
  if (!loader) return Promise.resolve(null)
  if (!loadedPageModules.has(id)) loadedPageModules.set(id, loader())
  return loadedPageModules.get(id)
}

function asyncPage(id) {
  return defineAsyncComponent({
    loader: () => loadPageModule(id).then(module => module?.default || module),
    delay: 0,
    timeout: 15000,
    suspensible: false,
    onError(_error, retry, fail, attempts) {
      loadedPageModules.delete(id)
      if (attempts < 2) retry()
      else fail()
    },
  })
}

const ProgressionEditor = asyncPage('progression')
const SigilGenerator = asyncPage('sigil')
const SigilMemoryGenerator = asyncPage('sigilMemory')
const SigilLoadoutRestore = asyncPage('loadout')
const LoadoutViewer = asyncPage('loadoutPresets')
const WrightstoneGenerator = asyncPage('wrightstone')
const SummonSaveEditor = asyncPage('summonSave')
const WrightstoneMemoryGenerator = asyncPage('wrightstoneMemory')
const WeaponMemoryGenerator = asyncPage('weaponMemory')
const SummonEditor = asyncPage('summon')
const OverLimit = asyncPage('overlimit')
const MiscTools = asyncPage('runtime')
const CharaStats = asyncPage('chara')
const SaveEditor = asyncPage('save')
const MonsterEnhance = asyncPage('monster')
const RuntimePatchFeatures = asyncPage('patchCombat')
const RuntimePatchMonitor = asyncPage('runtimeMonitor')
const FormulaSampler = asyncPage('formulaSampler')
const SaveDiffLab = asyncPage('saveDiff')
const NaturalDropLab = asyncPage('naturalDrop')
const AudioMixerLab = asyncPage('audioMixer')
const CameraLab = asyncPage('camera')
const VirtualSigilLab = asyncPage('virtualSigils')
const RuntimeQOLLab = asyncPage('runtimeQOL')
const LanguageSettings = asyncPage('language')
const cachedRuntimePages = Object.freeze({
  sigilMemory: SigilMemoryGenerator,
  loadout: SigilLoadoutRestore,
  wrightstoneMemory: WrightstoneMemoryGenerator,
  weaponMemory: WeaponMemoryGenerator,
  summon: SummonEditor,
  overlimit: OverLimit,
  runtime: MiscTools,
  formulaSampler: FormulaSampler,
})

const state = reactive({
  exePath: '',
  fileExists: false,
  fileSize: 0,
  backupExists: false,
  backupSize: 0,
})

const activeTab = ref('home')
const workspaceScroll = ref(null)
const toolSwitcher = ref(null)
const pageScrollPositions = new Map()
const RUNTIME_PATCH_MODES = Object.freeze({
  patchCombat: 'combat',
  patchCharacters: 'characters',
  patchQuest: 'quest',
})
const RUNTIME_MONITOR_MODES = Object.freeze({
  runtimeMonitor: 'party',
  spatialTools: 'spatial',
  selectedItemMonitor: 'items',
})
const runtimePatchesMounted = ref(false)
const runtimeMonitorMounted = ref(false)
const ctFeatureSession = reactive({ connected: false, releasePending: false, activeCount: 0, activeFeatures: [], recoveryCount: 0, pid: 0, route: 'patchCombat' })
const runtimeCompanionStates = reactive({
  camera: { id: 'camera', active: false, recoveryRequired: false },
  audioMixer: { id: 'audioMixer', active: false, recoveryRequired: false },
  virtualSigils: { id: 'virtualSigils', active: false, recoveryRequired: false },
  loadoutPresets: { id: 'loadoutPresets', active: false, recoveryRequired: false },
  runtimeQOL: { id: 'runtimeQOL', active: false, recoveryRequired: false },
  runtimeMonitor: { id: 'runtimeMonitor', active: false, recoveryRequired: false },
  spatialTools: { id: 'spatialTools', active: false, recoveryRequired: false },
  selectedItemMonitor: { id: 'selectedItemMonitor', active: false, recoveryRequired: false },
  weaponMemory: { id: 'weaponMemory', active: false, recoveryRequired: false },
  taskRewardMultiplier: { id: 'taskRewardMultiplier', active: false, recoveryRequired: false, multiplier: 1 },
})
const naturalDropRecovery = reactive({ blocked: false, detail: '' })
const naturalDropRecoveryCopy = computed(() => language.value === 'en'
  ? {
      label: 'Drop rules need safe recovery',
      title: 'The previous drop-rule deployment did not finish recovering. Fully exit the game and other tool instances, then open Drop & Crafting Rules and retry before starting the game.',
    }
  : {
      label: '掉落规则待安全恢复',
      title: '上次掉落规则部署尚未安全恢复。请完全退出游戏和其他工具实例，再打开“掉落与锻造规则”重试；恢复完成前不要启动游戏。',
    })
const runtimeCompanionLabels = {
  camera: ['镜头', 'Camera'],
  audioMixer: ['音频', 'Audio'],
  virtualSigils: ['虚拟因子', 'Virtual sigils'],
  loadoutPresets: ['战斗采集', 'Battle capture'],
  runtimeQOL: ['显示与房间', 'Display and room'],
  runtimeMonitor: ['配装检测', 'Loadout detector'],
  spatialTools: ['空间移动', 'Spatial controls'],
  selectedItemMonitor: ['物品捕获', 'Item capture'],
  weaponMemory: ['武器技能', 'Weapon skills'],
  taskRewardMultiplier: ['任务奖励', 'Quest rewards'],
}
const runtimeCompanionRoutes = Object.freeze({ taskRewardMultiplier: 'naturalDrop' })
const activeRuntimeCompanions = computed(() => Object.values(runtimeCompanionStates)
  .filter(item => item.active || item.recoveryRequired)
  .map(item => ({
    ...item,
    label: `${runtimeCompanionLabels[item.id]?.[language.value === 'en' ? 1 : 0] || item.id}${item.id === 'taskRewardMultiplier' && item.multiplier > 1 ? ` ${item.multiplier}×` : ''}`,
    route: runtimeCompanionRoutes[item.id] || item.id,
    stateLabel: item.recoveryRequired
      ? (language.value === 'en' ? 'needs recovery' : '待恢复')
      : (language.value === 'en' ? 'on' : '已开启'),
    stateTitle: item.recoveryRequired
      ? (language.value === 'en' ? 'Runtime needs recovery · open its page' : '运行时需要恢复 · 点击返回处理')
      : (language.value === 'en' ? 'Enabled · open its page to turn it off' : '已开启 · 点击返回对应页面关闭'),
  })))
const showCTFeatureStatus = computed(() => ctFeatureSession.releasePending || ctFeatureSession.activeCount > 0 || ctFeatureSession.recoveryCount > 0)
const ctFeatureStatusText = computed(() => {
  if (ctFeatureSession.releasePending) return language.value === 'en' ? 'Patches restoring' : '实时补丁正在恢复'
  if (ctFeatureSession.recoveryCount > 0) return language.value === 'en'
    ? `${ctFeatureSession.activeCount} patches on · ${ctFeatureSession.recoveryCount} need recovery`
    : `${ctFeatureSession.activeCount} 项补丁已开启 · ${ctFeatureSession.recoveryCount} 项待恢复`
  const names = ctFeatureSession.activeFeatures
  if (names.length === 1) return language.value === 'en' ? `${names[0]} on` : `${names[0]}已开启`
  if (names.length === 2) return language.value === 'en' ? `${names.join(' + ')} on` : `${names.join('、')}已开启`
  return language.value === 'en' ? `${ctFeatureSession.activeCount} patches on` : `${ctFeatureSession.activeCount} 项实时补丁已开启`
})
const ctFeatureStatusTitle = computed(() => {
  const detail = ctFeatureSession.activeFeatures.length ? ctFeatureSession.activeFeatures.join('、') : ctFeatureStatusText.value
  const pid = ctFeatureSession.pid ? ` · PID ${ctFeatureSession.pid}` : ''
  return language.value === 'en' ? `${detail}${pid} · Open patch controls` : `${detail}${pid} · 点击返回补丁页面关闭`
})
const lastRuntimePatchTab = ref('patchCombat')
const isRuntimePatchTab = computed(() => Boolean(RUNTIME_PATCH_MODES[activeTab.value]))
const runtimePatchMode = computed(() => RUNTIME_PATCH_MODES[activeTab.value] || RUNTIME_PATCH_MODES[lastRuntimePatchTab.value])
const isRuntimeMonitorTab = computed(() => Boolean(RUNTIME_MONITOR_MODES[activeTab.value]))
const runtimeMonitorMode = computed(() => RUNTIME_MONITOR_MODES[activeTab.value] || 'party')
const sidebarCollapsed = ref(window.localStorage.getItem('gbfr.sidebarCollapsed') === '1')
const artCollapsed = ref(window.localStorage.getItem('gbfr.artCollapsed') === '1')
const loadoutEditing = ref(false)
const pendingRuntimeLoadout = ref(null)
const manualPath = ref('')
const isLoaded = ref(false)
const isDetecting = ref(false)
const forceBackup = ref(false)
const saveStatus = ref('')
const statusType = ref('')
const updateLoading = ref(false)
const updateInfo = reactive({ currentVersion: '—', latestVersion: '', hasUpdate: false, body: '' })
const navigationBusy = ref(false)
const navigationError = ref(null)
let hasAttemptedGameDetection = false
let stopRuntimeQOLSessionEvents = () => {}
let runtimeCompanionSummaryTimer = 0
let runtimeCompanionSummaryRequest = 0

watch(activeTab, (value) => {
  if (RUNTIME_MONITOR_MODES[value]) runtimeMonitorMounted.value = true
  if (!RUNTIME_PATCH_MODES[value]) return
  runtimePatchesMounted.value = true
  lastRuntimePatchTab.value = value
}, { immediate: true })

function updateCTFeatureSession(value) {
  ctFeatureSession.connected = value?.connected === true
  ctFeatureSession.releasePending = value?.releasePending === true
  ctFeatureSession.activeCount = Number.isSafeInteger(value?.activeCount) && value.activeCount >= 0 ? value.activeCount : 0
  ctFeatureSession.activeFeatures = Array.isArray(value?.activeFeatures)
    ? value.activeFeatures.map(item => String(item || '').trim()).filter(Boolean).slice(0, 64)
    : []
  ctFeatureSession.recoveryCount = Number.isSafeInteger(value?.recoveryCount) && value.recoveryCount >= 0 ? value.recoveryCount : 0
  ctFeatureSession.pid = Number.isSafeInteger(value?.pid) && value.pid > 0 ? value.pid : 0
  ctFeatureSession.route = ['patchCombat', 'patchCharacters', 'patchQuest'].includes(value?.route) ? value.route : ctFeatureSession.route
}

function updateRuntimeCompanionState(value) {
  const target = runtimeCompanionStates[value?.id]
  if (!target) return
  target.active = value?.active === true
  target.recoveryRequired = value?.recoveryRequired === true
  if ('multiplier' in target) target.multiplier = Number.isSafeInteger(value?.multiplier) && value.multiplier > 0 ? value.multiplier : 1
}

function scheduleRuntimeCompanionSummary(delay = 2000) {
  window.clearTimeout(runtimeCompanionSummaryTimer)
  if (document.hidden) return
  runtimeCompanionSummaryTimer = window.setTimeout(refreshRuntimeCompanionSummary, delay)
}

async function refreshRuntimeCompanionSummary() {
  const request = ++runtimeCompanionSummaryRequest
  const [summariesResult, naturalDropResult] = await Promise.allSettled([
    GetRuntimeCompanionSummary(),
    GetNaturalDropStartupRecoveryStatus(),
  ])
  if (request !== runtimeCompanionSummaryRequest) return
  if (summariesResult.status === 'fulfilled') {
    for (const summary of summariesResult.value || []) {
      if (summary?.id === 'runtimePatches') {
        const activeCount = Number.isSafeInteger(summary?.activeCount) ? summary.activeCount : 0
        const recoveryCount = Number.isSafeInteger(summary?.recoveryCount) ? summary.recoveryCount : 0
        ctFeatureSession.activeCount = activeCount
        ctFeatureSession.recoveryCount = recoveryCount
        if (activeCount === 0) ctFeatureSession.activeFeatures = []
        if (Number.isSafeInteger(summary?.pid) && summary.pid > 0) ctFeatureSession.pid = summary.pid
        continue
      }
      updateRuntimeCompanionState(summary)
    }
  }
  if (naturalDropResult.status === 'fulfilled') {
    const naturalDropStatus = naturalDropResult.value
    naturalDropRecovery.blocked = naturalDropStatus?.blocked === true
    naturalDropRecovery.detail = String(naturalDropStatus?.detail || '')
  }
  // A failed endpoint does not hide the other endpoint's status. The next shell
  // poll retries both authoritative aggregates.
  if (request === runtimeCompanionSummaryRequest) scheduleRuntimeCompanionSummary()
}

function handleRuntimeCompanionVisibility() {
  if (document.hidden) {
    window.clearTimeout(runtimeCompanionSummaryTimer)
    return
  }
  void refreshRuntimeCompanionSummary()
}

const toolNavigationModes = Object.freeze({
  progression: 'offline',
  sigil: 'offline',
  loadoutPresets: 'offline',
  wrightstone: 'offline',
  summonSave: 'offline',
  chara: 'offline',
  save: 'offline',
  sigilMemory: 'live',
  wrightstoneMemory: 'live',
  weaponMemory: 'live',
  summon: 'live',
  overlimit: 'live',
  runtime: 'live',
  runtimeMonitor: 'background',
  loadout: 'live',
  runtimeQOL: 'live',
  virtualSigils: 'live',
  audioMixer: 'live',
  camera: 'live',
  spatialTools: 'live',
  patchCombat: 'live',
  patchCharacters: 'live',
  patchQuest: 'live',
  monster: 'live',
  naturalDrop: 'file',
  saveDiff: 'offline',
  selectedItemMonitor: 'live',
  formulaSampler: 'live',
  compatibility: 'local',
  patch: 'file',
  language: 'local',
})

const toolMeta = {
  home: {
    group: 'home', title: '首页', eyebrow: '功能入口', status: '游戏 2.0.3', tone: 'stable',
    description: '从目标出发选择功能，常用养成、实时工具和记录编辑都从这里进入。',
    usage: [], caution: '',
  },
  progression: {
    group: 'save', title: '物品与武器（存档修改）', eyebrow: '离线养成', status: '已适配 2.0.2', tone: 'stable',
    description: '给所选存档补充素材和养成资源，或调整武器等级与强化进度；只改你在页面中确认的项目。',
    usage: ['完全退出游戏并选择目标存档', '搜索物品或武器，填入数量与等级', '预览改动后保存；应用会自动备份并回读'],
    caution: '改动写回所选存档；需要恢复时，从页面右上角的存档保护中选择写入前备份。',
    speaker: '卡莉奥斯特罗', note: '先留好备份，再把素材和武器整理得漂漂亮亮——这才像完美的炼金术嘛。',
  },
  sigil: {
    group: 'save', title: '因子修改（存档修改）', eyebrow: '离线存档', status: '稳定', tone: 'stable',
    description: '在所选存档中新增独立因子实例，也能查看或删除已有因子；适合一次准备多颗合法因子。',
    usage: ['完全退出游戏并选择目标存档', '选择因子、主副词条、等级与数量，加入待写入队列', '核对队列后保存；应用会自动备份并回读'],
    caution: '天然或效果曲线之外的组合只会收到提醒；确认后仍按所选值写入，游戏可能在读取时自动替换。需要撤销时，从存档保护恢复本次写入前的备份。',
    speaker: '娜露梅亚', note: '先检验组合，再写入存档。稳稳完成每一步，理想的因子就不会跑掉。',
  },
  sigilMemory: {
    group: 'memory', title: '因子即时编辑', eyebrow: '游戏内养成', status: '实时', tone: 'live',
    description: '修改游戏里当前高亮的那一颗因子，适合少量调整；页面会并列显示当前值与准备写入的值。',
    usage: ['启动游戏，打开因子列表并点击“启用读取”', '在游戏中选中目标因子，回到工具核对名称和词条', '修改后点击“写入修改”；继续编辑前重新选择目标'],
    caution: '写入的是当前游戏进程，不是离线存档文件；停止读取会恢复捕获指令，重新进档后需重新连接。',
    speaker: '萝赛塔', note: '游戏重新载入后，记得再选一次目标。旧的指针可不会一直等你哦。',
  },
  loadout: {
    group: 'loadoutFlow', title: '因子配装·实时录制/复刻', eyebrow: '游戏内因子', status: '实时', tone: 'live',
    description: '从游戏内依次读取角色当前的 12 个因子，用于导出和分享；也能把导入配装逐颗写到你准备好的备用因子上。',
    usage: ['启动游戏，打开角色因子列表并选中第一颗', '选择“录制”或载入配装进行“复刻”', '按页面提示逐颗向下移动，完成后预览、导出或分享'],
    caution: '复刻会改写当前选中的备用因子；不要使用已经装备或需要保留的因子。',
    speaker: '芙劳', note: '把十二个因子的顺序先理清，再一步一步复刻。速度不必太快，准确才最重要。',
  },
  loadoutPresets: {
    group: 'save', title: '配装预设（查看与写入）', eyebrow: '离线存档', status: '稳定', tone: 'stable',
    description: '查看和编辑角色配装：武器、12 个因子、4 个技能、专精与其他已记录内容都在同一页核对。',
    usage: ['完全退出游戏并选择目标存档', '打开角色和配装槽，手动编辑或按技能目标生成因子方案', '先预览整套配装，再确认写入所选槽位'],
    caution: '保存草稿不会修改存档；只有确认写入才会覆盖目标配装槽，并自动创建备份和回读。',
    speaker: '古兰', note: '先备份，再确认角色和目标槽；已有配装会被覆盖。',
  },
  wrightstone: {
    group: 'save', title: '祝福修改（存档修改）', eyebrow: '离线存档', status: '稳定', tone: 'stable',
    description: '在所选存档中新增祝福石实例；选择祝福类型和三条技能后，可以一次生成一颗或批量加入队列。',
    usage: ['完全退出游戏并选择目标存档', '选择祝福、三条技能、等级与数量', '核对队列后保存；应用会自动备份并回读'],
    caution: '非天然组合会提示但不会阻止；只有无副槽、非因子词条、错误主词条或超过技能曲线的等级不会写入。可从存档保护恢复写入前备份。',
    speaker: '菲莉', note: '三条词条都确认好再应用，幽灵朋友们也会替你看着。',
  },
  summonSave: {
    group: 'save', title: '召唤石添加 / 修改（存档）', eyebrow: '离线存档', status: '新增', tone: 'stable',
    description: '在存档中新增召唤石，或原子修改已有记录的种类、主加护、副词条、等级和状态字段，写后重新打开存档验证。',
    usage: ['完全退出游戏并加载存档', '选择修改已有或新增', '核对目录、等级与输出路径后写入'],
    caution: '不会替未进入 DLC 的存档强开召唤系统；更换种类时会重建物品 SlotID 并迁移已装备引用。',
    speaker: '圣德芬', note: '系统未开放只会提示；种类、加护和副词条核对一致，再写入。',
  },
  wrightstoneMemory: {
    group: 'memory', title: '祝福石即时编辑', eyebrow: '游戏内祝福石', status: '实时', tone: 'live',
    description: '修改游戏中当前高亮的祝福石：先读取真实三槽，再一次写入你确认的技能和等级。',
    usage: ['启动游戏并启用读取', '在游戏内祝福石列表选中目标记录', '核对三槽变更后一次性写入'],
    caution: '写入成功会自动停止读取并恢复游戏指令；继续修改前，必须重新启用并在游戏中选择目标。',
    speaker: '玛琪拉菲菈', note: '写入后旧记录会失效。回到游戏里重新选中目标，再继续。',
  },
  weaponMemory: {
    group: 'memory', title: '武器技能即时编辑', eyebrow: '游戏内当前武器', status: '实时 · 2.0.3', tone: 'live',
    description: '读取游戏武器列表中当前高亮的武器，直接修改记录里真实存在的五个技能槽；支持添加、替换、清空和调整等级。',
    usage: ['启动 2.0.3 游戏并打开武器列表', '启用读取后高亮目标武器，核对武器编号和五槽', '预览变更后一次写入；页面会保存、回读并恢复捕获指令'],
    caution: '武器记录只有五个物理技能槽，不会虚构第六槽；写入不连带修改武器等级、觉醒、祝福或角色配装。非常规组合可能被游戏规范化。',
    speaker: '莉莉丝', note: '先看清当前武器，再决定赋予它怎样的力量。五个槽位，一个都不要写错。',
  },
  summon: {
    group: 'memory', title: '召唤石即时修改', eyebrow: '游戏内召唤石', status: '实时保存', tone: 'live',
    description: '读取游戏背包中当前选中的召唤石，修改它的技能、副参数和等级，再调用游戏自身保存流程。',
    usage: ['启动游戏并打开召唤石背包', '连接后在游戏中选中目标召唤石', '回到工具核对类型和等级，再写入并按提示保存'],
    caution: '实时页不更换召唤石种类；需要更换类型或新增实例，请使用“召唤石添加 / 修改（存档）”。',
    speaker: '露莉亚', note: '先在背包里选中目标召唤石，再核对稀有度和等级，我们一起慢慢来。',
  },
  overlimit: {
    group: 'memory', title: '角色上限突破', eyebrow: '游戏内修改', status: '流程型', tone: 'live',
    description: '读取角色上限突破结果页的四项能力，修改后仍由游戏原本的确认步骤保存。',
    usage: ['在游戏中完成一次 3 级上限突破', '停留在四项结果页，回到工具读取', '调整四项后写回，再返回游戏确认保存'],
    caution: '工具只改当前结果页；跳过游戏内最终确认不会保存。',
    speaker: '希耶提', note: '四个能力槽一个都别漏。真正的剑王，可不会跳过确认步骤。',
  },
  runtime: {
    group: 'memory', title: '货币、素材与任务掉落', eyebrow: '游戏内即时资源', status: '需连接游戏', tone: 'live',
    description: '在当前游戏进程中调整金币、MSP、药水和素材，或开启已验证的任务掉落功能。',
    usage: ['启动游戏并进入要使用的存档', '连接当前游戏进程', '选择资源或任务功能，按页面提示应用'],
    caution: '这些设置作用于当前游戏会话；重启游戏后需要重新连接，页面提供的停用操作会恢复相应补丁。',
    speaker: '碧', note: '进游戏、连进程、再修改！重启以后可得重新连接，别忘啦！',
  },
  runtimeMonitor: {
    group: 'loadoutFlow', title: '队友配装持续检测', eyebrow: '队伍配装 · 后台服务', status: '点击开启后持续检测', tone: 'live',
    description: '点击开启后持续在后台读取连续稳定的任务队伍快照；切换页面不会停止，直到你主动关闭。',
    usage: ['点击开启角色配装检测', '正常进入任务并与队友游玩', '在本地批次中预览、导出、部署或上传配装'],
    caution: '检测器不会默认开启；只读游戏数据，只有你点击关闭时才停止。',
    speaker: '尤斯提斯', note: '开启一次就够了。你继续游玩，连续一致的队伍配装会按批次归档。',
  },
  spatialTools: {
    group: 'runtimeTools', title: '坐标与移动工具', eyebrow: '单机空间操作', status: '可用 · 实验', tone: 'calibrate',
    description: '读取坐标、保存书签、传送，并把方向键映射为游戏原生 W/A/S/D；连续跳跃使用 2.0.3 双站点补丁与原生跳跃输入。',
    usage: ['进入离线或单机内容并连接游戏', '点击“起飞并悬停”，方向键移动，PageUp / PageDown 升降，松开升降键保持高度', '点击“降落并关闭”恢复自然重力；F12、断开或退出会恢复本工具管理的补丁'],
    caution: '不会再修改导致动作异常的模型 setter。高级坐标微调只用于传送校准；当前连续跳跃不是穿墙或相机相对飞行。',
    speaker: '泽塔', note: '先记下原点，再移动。没有碰撞证据的能力，不会冒充穿墙。',
  },
  selectedItemMonitor: {
    group: 'tools', title: '选中物品查看（只读）', eyebrow: '诊断 · 内存查看', status: '低频工具', tone: 'live',
    description: '查看游戏中当前高亮素材或关键物品的名称、数量、Hash 与 Flags；页面没有任何修改入口。',
    usage: ['启动游戏并连接，点击“启用只读捕获”', '在游戏的素材或关键物品列表中高亮目标', '回到工具刷新并读取一次；查看下一件前重新选择'],
    caution: '这是低频诊断工具；点击“安全断开”会移除临时捕获并恢复游戏原始指令。',
    speaker: '齐格飞', note: '换了物品要重新选中再读取；这里只看，不会写入。',
  },
  formulaSampler: {
    group: 'tools', title: '角色公式采样', eyebrow: '诊断 · 严格只读', status: 'A/B/A/B · 需连接游戏', tone: 'live',
    description: '用四次只读记录比较某一项装备或技能到底改变了多少 HP、攻击、暴击率与昏厥值。',
    usage: ['选择当前出战角色并连接，先记录原状态 A1', '只改变一个可逆项目，记录 B1，再重复记录 A2、B2', '四次结果可复现后，导出不含个人路径的证据包'],
    caution: '角色面板没稳定或一次改变多项都会让结果失效；本页不安装 Hook、不写游戏，也不改存档。',
    speaker: '卡塔莉娜', note: '一次只动一项，等数字站稳再记。前后能复现，公式才算有证据。',
  },
  patchCombat: {
    group: 'runtimeTools', title: '战斗规则补丁', eyebrow: '战斗补丁', status: '仅离线/单机', tone: 'live',
    description: '在离线或单机游玩中调整闪避、格挡、Link、召唤限制和部位破坏等战斗规则。',
    usage: ['启动游戏并进入离线或单机内容', '连接后逐项开启需要的规则', '切页不会停止；用完点击断开，恢复本工具开启的全部补丁'],
    caution: '这些功能只用于离线或单机游玩；不要带入联机房间。',
    speaker: '巴恩', note: '先确认只在单机里测试，再一项一项校准。切换页面不会打断，明确断开时才会全部恢复。',
  },
  patchCharacters: {
    group: 'runtimeTools', title: '角色机制补丁', eyebrow: '角色机制', status: '仅离线/单机', tone: 'live',
    description: '按角色查找专属机制调整；每个开关都会说明作用，互相冲突的功能不会同时开启。',
    usage: ['启动游戏并进入离线或单机内容', '搜索角色或机制名称，查看说明后开启', '切换互斥功能前，先停用并确认原机制已恢复'],
    caution: '这些功能只用于离线或单机游玩；互斥机制不会相互覆盖。',
    speaker: '巴萨拉卡', note: '冲突项不能同时开。先关掉亮着的那个，等状态回读后再切换。',
  },
  patchQuest: {
    group: 'runtimeTools', title: '任务与便利补丁', eyebrow: '任务与便利', status: '仅离线/单机', tone: 'live',
    description: '在离线或单机任务中调整倒计时、宝箱、结算、支线奖励与养成便利功能。',
    usage: ['启动游戏并进入离线或单机任务', '按“任务规则”或“体验便利”选择功能', '用完在本页停用，或断开并恢复本工具开启的全部补丁'],
    caution: '这些功能只用于离线或单机游玩；任务状态切换后请刷新回读。',
    speaker: '尤达拉哈', note: '任务路线先看清，宝箱和结算各归各位。用完恢复，下一趟才不会乱。',
  },
  chara: {
    group: 'save', title: '角色使用次数', eyebrow: '记录与统计', status: '离线存档', tone: 'stable',
    description: '查看每名角色记录的使用次数，并把同一个次数批量写给你勾选的角色。',
    usage: ['完全退出游戏并选择目标存档', '勾选角色并填入新的使用次数', '核对已选数量后保存'],
    caution: '只修改勾选角色；写入前会自动备份，需要时可从存档保护恢复。',
    speaker: '姬塔', note: '只会保存你勾选的角色。动手前再数一遍，团长的记录要清清楚楚。',
  },
  save: {
    group: 'save', title: '任务与称号记录', eyebrow: '记录与统计', status: '离线存档', tone: 'stable',
    description: '修改任务完成次数，或管理称号是否解锁、是否已查看；两个功能使用同一份所选存档。',
    usage: ['完全退出游戏并选择目标存档', '切到任务或称号，搜索并勾选要修改的记录', '核对数量后保存；应用会自动备份并回读'],
    caution: '称号奖励是否领取不会在这里改变；需要撤销时，从存档保护恢复写入前备份。',
    speaker: '拉卡姆', note: '任务记录就像航线图，先选准目标，再一次写入，别改错方向。',
  },
  saveDiff: {
    group: 'save', title: '存档对比与复制', eyebrow: '双存档 · 页内替换', status: '离线存档', tone: 'stable',
    description: '并排比较两份存档，把同结构差异直接从一侧拖到另一侧；不需要跳转到其他编辑页。',
    usage: ['完全退出游戏，选择左右两份不同的存档', '选择写入左侧或右侧，把需要的差异拖入目标侧或加入变更单', '核对变更单后确认；应用会自动备份、原子写入并逐条回读'],
    caution: '只会复制你确认的同结构记录；单侧新增、删除或长度不同的记录会保持禁用，避免破坏存档结构。',
    speaker: '兰斯洛特', note: '先确认写入哪一侧，再逐条挑选。变更单核对无误后，一次写入就够了。',
  },
  naturalDrop: {
    group: 'tools', title: '掉落与锻造规则（游戏文件）', eyebrow: 'data.i 模组 · 自动备份', status: '2.0.2 / 2.0.3 可用 · 实验', tone: 'calibrate',
    description: '修改游戏实际读取的掉落与锻造表：可添加 Transmarvel 因子、召唤石、祝福石和普通物品，不会直接向存档背包添加物品。',
    usage: ['完全退出游戏并选择游戏程序', '从应用内置的 2.0.2 目录搜索内容，填写数量或权重并加入待部署清单', '核对清单后部署；停用时恢复应用创建的游戏文件备份'],
    caution: '十一张 2.0.2 精确表已随应用内嵌并校验，不需要自行解包；普通物品会加入“无尽模式·锻造师奖励池”。发现同表冲突时会停止，避免覆盖其他模组。',
    speaker: '加兰查', note: '先确认战利品来自正确的原表，再把每一格分清。撞上别的模组时，别硬冲。',
  },
  audioMixer: {
    group: 'runtimeTools', title: '角色语音混音台', eyebrow: 'Wwise · 内置运行时', status: '2.0.2 / 2.0.3 可用 · 实验', tone: 'calibrate',
    description: '分别调低或静音各角色后续播放的语音，也能调整界面音效；不会替换游戏音频文件。',
    usage: ['启动游戏并在本页确认已连接', '调整角色或界面音量，可先保存为本机预设', '点击“开启音频运行时”；之后保存会立即更新当前游戏'],
    caution: '只处理能够明确归属的语音事件，未知和共享事件保持原音；点击“停用并恢复”会移除本工具的音频 Hook。',
    speaker: '冈达葛萨', note: '每一道声音都该有自己的分量。认不准的事件，就让它保持原样！',
  },
  camera: {
    group: 'runtimeTools', title: '城镇镜头工坊', eyebrow: '镜头 · 内置运行时', status: '2.0.2 / 2.0.3 可用 · 实验', tone: 'calibrate',
    description: '调整城镇镜头能拉多远、看向角色的高度，以及每格滚轮缩放多少；战斗镜头不会改变。',
    usage: ['启动游戏并在本页确认已连接', '选择默认或舒适预设，也可手动调三个参数', '点击“开启镜头运行时”；之后保存会立即更新当前游戏'],
    caution: '只影响城镇镜头；点击“停用并恢复”会还原开启前的镜头值和本工具安装的 Hook。',
    speaker: '索恩', note: '先看准距离和高度；顶部显示常驻后，切页也不会停。',
  },
  virtualSigils: {
    group: 'runtimeTools', title: '虚拟因子槽', eyebrow: '运行时配装 · 内置 Hook', status: '2.0.2 / 2.0.3 可用 · 实验', tone: 'calibrate',
    description: '让运行中角色额外读取 1 至 8 颗真实库存因子；它不会扩展存档的 12 个物理槽，也不会把虚拟槽写进存档。',
    usage: ['选择存档、角色和真实未装备因子实例', '核对 1 至 8 个虚拟槽后开启内置运行时', '切角色或场景后查看状态与实际效果；需要结束时点击停用并恢复相关 Hook'],
    caution: '同一真实实例只能占一个虚拟槽。跨角色、切场景、同角色队友和多 Hook 组合请按实际玩法测试并反馈，异常时先 F12 紧急停止。',
    speaker: '菲迪埃尔', note: '额外的力量不必刻进存档。把每一个真实实例认清，换了世界也不会把别人的力量拿错。',
  },
	runtimeQOL: {
		group: 'runtimeTools', title: '显示与房间工具', eyebrow: '界面显示 · 房间号 · 编队', status: '2.0.2 / 2.0.3 内置运行时', tone: 'live',
		description: '集中开启显示精度、房间 ID、主线队长替换、普通任务等级同步和重镶返还；后两项保留实验提示，但入口不会被隐藏。',
		usage: ['启动游戏', '选择需要的便利功能和显示精度', '开启后正常游玩；F12 可紧急恢复'],
		caution: '所有入口都会在启用前校验版本、原字节和唯一匹配；实验项的任务结果和背包增量仍需自行核对，F12 可以恢复。',
		speaker: '夏洛特', note: '实验开关可以先试，但记得核对任务和背包；F12 可以恢复。',
	},
  compatibility: {
    group: 'tools', title: '版本适配', eyebrow: '版本检测与功能状态', status: '2.0.3 分层适配', tone: 'calibrate',
    description: '查看游戏 2.0.3 的离线与实时功能适配情况；这里不修改游戏或存档。',
    usage: ['检查是否有新的工具版本', '确认已识别正确的游戏 EXE', '查看各功能的已适配、实验或未开放状态'],
    caution: '“已识别”只代表版本和文件匹配，不代表尚未完成的实机功能已经验证。',
    speaker: '罗兰', note: '先看工具版本、游戏文件和适配状态。修东西之前，总得弄清哪里不对。',
  },
  monster: {
    group: 'runtimeTools', title: '怪物倍率与状态控制', eyebrow: '实验', status: '实验', tone: 'live',
    description: '实验性调整当前怪物的血量、伤害、昏厥条与 Overdrive 状态，便于离线测试战斗。',
    usage: ['只在离线、单机或你明确控制的主机端测试', '连接后刷新并确认当前怪物状态', '一次调整一项，记录结果后恢复默认'],
    caution: '怪物切换、阶段变化和任务结束都可能让目标失效；不能把候选行为当作稳定联机功能。',
    speaker: '伊德', note: '先确认主机端和倍率，再动手。力量失控的话，记录也会失去意义。',
  },
  patch: {
    group: 'tools', title: '游戏文件维护', eyebrow: 'EXE 备份与恢复', status: '可用', tone: 'calibrate',
    description: '定位游戏 EXE、保存一份原始文件副本，并在文件补丁异常时恢复。',
    usage: ['自动检测或手动选择游戏 EXE', '修改游戏文件前先创建原始备份', '需要撤销文件补丁时点击“恢复备份”'],
    caution: '“重新创建原始备份”会替换旧备份；只有确认当前 EXE 是干净原版时才使用。',
    speaker: '欧根', note: '原始文件先备份，字节状态看清楚再修。老手从不省这一步。',
  },
  language: {
    group: 'tools', title: '语言与显示', eyebrow: '应用设置', status: '本机设置', tone: 'neutral',
    description: '选择界面语言。切换后会刷新应用，让所有功能使用同一语言。',
    usage: ['选择语言', '等待应用刷新', '返回上次使用的功能'],
    caution: '语言设置只保存在本机。',
    speaker: '伊欧', note: '选好语言后等界面刷新，别急着连点。清清楚楚才最好用嘛！',
  },
}

const navigation = computed(() => [
  { id: 'save', mark: '档', label: language.value === 'zh' ? '存档与配装（离线）' : 'Saves & Loadouts', caption: language.value === 'zh' ? '退出游戏后编辑；备份、写入并回读' : 'Edit offline with backup and readback', items: ['loadoutPresets', 'sigil', 'wrightstone', 'summonSave', 'progression', 'chara', 'save', 'saveDiff'] },
  { id: 'memory', mark: '改', label: language.value === 'zh' ? '游戏内即时编辑' : 'In-Game Live Editing', caption: language.value === 'zh' ? '选中目标后读取、修改并保存' : 'Select, read, edit, and save in game', items: ['sigilMemory', 'wrightstoneMemory', 'weaponMemory', 'summon', 'overlimit', 'runtime'] },
  { id: 'loadoutFlow', mark: '配', label: language.value === 'zh' ? '配装采集与复刻' : 'Loadout Capture & Restore', caption: language.value === 'zh' ? '队友后台检测与十二因子录制' : 'Party detection and 12-sigil recording', items: ['runtimeMonitor', 'loadout'] },
  { id: 'runtimeTools', mark: '运', label: language.value === 'zh' ? '单机运行时工具' : 'Solo Runtime Tools', caption: language.value === 'zh' ? '显示、因子、音频、镜头与规则补丁' : 'Display, sigils, audio, camera, and rules', items: ['runtimeQOL', 'virtualSigils', 'audioMixer', 'camera', 'spatialTools', 'patchCombat', 'patchCharacters', 'patchQuest', 'monster'] },
  { id: 'tools', mark: '具', label: language.value === 'zh' ? '游戏文件、诊断与设置' : 'Files, Diagnostics & Settings', caption: language.value === 'zh' ? '掉落表、实时诊断、适配与维护' : 'Drop tables, live diagnostics, compatibility, maintenance', items: ['naturalDrop', 'selectedItemMonitor', 'formulaSampler', 'compatibility', 'patch', 'language'] },
])

const compatibilityCopy = computed(() => language.value === 'zh' ? {
  manualFile: '可在游戏文件维护页手动选择',
  baseline: '适配基线',
  baselineVersion: '游戏 2.0.3（静态与离线）',
    baselineSummary: '2,116 张核心表和 332 份角色/战斗配置逐字节一致。',
  baselineBoundary: '静态目录、配装、分享与 Logs 数据已核对；实时布局与 data.i 事务已适配 2.0.3，任务结果和存档重启回读仍需现场核对',
  featureKicker: '功能适配',
  featureTitle: '当前实现与验证边界',
  featureHint: '只展示能由代码、测试与锁定游戏数据证明的状态。',
  resourceKicker: '资源适配',
  resourceTitle: '官方图标映射',
  resourceHint: '图标目录在 2.0.3 核心表差分中未变；缺口保持显式，不用相似图片伪装。',
  scope: '范围',
  status: '状态',
  evidence: '证据与边界',
  experimentKicker: '实验入口',
  experimentTitle: '不计入稳定完成项',
  experimentName: '怪物倍率与状态控制',
  experimentDetail: '四项 Hook 已在 DLC 2.0.2 实机完成安装与恢复验证；战斗效果仍按实验功能标注',
  open: '查看 ›',
} : {
  manualFile: 'Select it manually on the Game File Maintenance page',
  baseline: 'Compatibility Baseline',
  baselineVersion: 'Game 2.0.3 (static and offline)',
    baselineSummary: '2,116 core tables and 332 character/combat configs are byte-identical.',
  baselineBoundary: 'Static catalogs, loadouts, sharing, and Logs data are verified; live layouts and data.i transactions support 2.0.3, while quest results and save/restart readback still need field checks',
  featureKicker: 'Feature Compatibility',
  featureTitle: 'Current implementation and validation boundary',
  featureHint: 'Only states supported by code, tests, and locked game data are shown.',
  resourceKicker: 'Asset Compatibility',
  resourceTitle: 'Official icon mapping',
  resourceHint: 'The icon catalogs are unchanged in the 2.0.3 core-table diff; gaps stay explicit instead of using look-alike art.',
  scope: 'Scope',
  status: 'Status',
  evidence: 'Evidence and boundary',
  experimentKicker: 'Experimental Entry',
  experimentTitle: 'Not counted as stable completion',
  experimentName: 'Monster Multipliers & State Control',
  experimentDetail: 'All four hooks pass live install and restore checks on DLC 2.0.2; combat effects remain experimental',
  open: 'Open ›',
})

const compatibilityRows = computed(() => language.value === 'zh' ? [
  { scope: '游戏 2.0.3 静态数据', status: '已核对', tone: 'ok', detail: '2,120 张表中仅 4 个本地化文本包变化；因子、祝福、召唤石、掉落、武器、伤害上限、成长与规则表逐字节不变' },
  { scope: '静态与离线流程', status: '数据已核对', tone: 'ok', detail: '内置目录、配装编辑与优化、配装 JSON、短码、二维码、分享图和 Logs 使用未变化的数据；现有真实存档副本事务通过，2.0.3 游戏重启回读仍待验收' },
  { scope: '游戏 2.0.3 实时功能', status: '已迁移 · 实验边界', tone: 'flow', detail: 'EXE 地址已迁移；共享连接入口会识别 2.0.2 / 2.0.3 并拒绝未知版本，关键安装、回读和恢复路径已实测，具体任务结果仍需现场核对' },
  { scope: '天然掉落 data.i 部署', status: '2.0.3 可部署', tone: 'flow', detail: '内置表与 2.0.3 目标表身份已核对；归档生成、事务日志、原子替换和恢复支持 2.0.3，实际掉落与锻造结果仍需游戏内核对' },
  { scope: '存档修改页面', status: '8 / 8', tone: 'ok', detail: '配装预设、因子、物品与武器、祝福、召唤石存档、角色次数、任务与称号记录、双存档对比复制' },
  { scope: '游戏内即时编辑', status: '5 / 5', tone: 'flow', detail: '因子、祝福石、召唤石、上限突破与当前会话资源；均需启动并连接游戏' },
  { scope: '配装采集与复刻', status: '2 / 2', tone: 'flow', detail: '队友配装检测按点击开启后持续后台运行；十二因子录制与复刻使用实时捕获' },
  { scope: '单机运行时工具', status: '9 页接入', tone: 'flow', detail: '显示与房间、虚拟因子、角色语音、城镇镜头、坐标移动、三类规则补丁与怪物控制' },
  { scope: '实时只读诊断', status: '2 / 2', tone: 'ok', detail: '选中物品查看与角色公式采样需要连接游戏，但不会修改进程数据或存档' },
  { scope: '游戏文件与设置', status: '4 页已接入', tone: 'ok', detail: '掉落与锻造规则、版本适配、游戏文件维护、语言与显示' },
  { scope: '运行时补丁覆盖', status: '60 已接入 / 4 待证据', tone: 'ok', detail: '59 个双版本验证目录功能和 1 个 EXE 锁定候选项已接入；另有 4 个候选项因缺少充分字段或实机证据，仍未作为可用开关暴露' },
  { scope: '运行时补丁目录', status: '60 / 83 / 81', tone: 'ok', detail: '60 功能 / 83 站点 / 81 AOB；按 2.0.2 / 2.0.3 EXE 选择严格签名、原字节与唯一命中证据' },
  { scope: 'DLC 2.0.2 增量审计', status: '58 稳定项 + 1 EXE 候选 + 1 现场修复', tone: 'ok', detail: '新增刀上舞自身眩晕移除候选；祝福石捕获与自动完美格挡连招修复继续使用独立版本守卫和写后回读' },
  { scope: '当前维护增量', status: '2 / 2 已验证', tone: 'ok', detail: '称号搜索支持拼音；连续挑战使用唯一特征码、三字节补丁与写后回读' },
  { scope: '真实游戏进程 E2E', status: '关键路径已验证', tone: 'ok', detail: '2.0.3 已验证镜头、音频、QOL、伤害捕获、虚拟因子入口、空间/重力和运行时补丁生命周期；未逐项覆盖的游戏效果仍保留实验等级' },
] : [
  { scope: 'Game 2.0.3 static data', status: 'Verified', tone: 'ok', detail: 'Only 4 localization bundles changed among 2,120 tables; sigil, wrightstone, summon, drop, weapon, damage-cap, progression, and rule tables are byte-identical' },
  { scope: 'Static and offline flows', status: 'Data verified', tone: 'ok', detail: 'Embedded catalogs, loadout editing and optimization, loadout JSON, short codes, QR import, share images, and Logs use unchanged data. Existing real-save-copy transactions pass; game 2.0.3 save/restart readback remains pending' },
  { scope: 'Game 2.0.3 live features', status: 'Migrated · experimental boundary', tone: 'flow', detail: 'Executable addresses moved. The shared attach boundary identifies 2.0.2 / 2.0.3 and refuses unknown versions; key install, read-back, and restoration paths pass, while quest-specific results still need field checks' },
  { scope: 'Natural-drop data.i deployment', status: 'Deployable on 2.0.3', tone: 'flow', detail: 'Embedded tables and the 2.0.3 executable identity are verified; archive generation, transaction journaling, atomic replacement, and restoration support 2.0.3, while actual drop and crafting results still need in-game checks' },
  { scope: 'Save editing pages', status: '8 / 8', tone: 'ok', detail: 'Loadout presets, sigils, items and weapons, wrightstones, summon saves, character counts, quest and title records, and two-save copying' },
  { scope: 'In-game live editing', status: '5 / 5', tone: 'flow', detail: 'Sigils, wrightstones, summons, overmastery, and current-session resources all require the running game' },
  { scope: 'Loadout capture and restore', status: '2 / 2', tone: 'flow', detail: 'Party detection starts only on click and then runs in the background; 12-sigil recording and restore use live capture' },
  { scope: 'Solo runtime tools', status: '9 pages integrated', tone: 'flow', detail: 'Display and room tools, virtual sigils, voice audio, town camera, spatial controls, three patch groups, and monster control' },
  { scope: 'Live read-only diagnostics', status: '2 / 2', tone: 'ok', detail: 'Selected-item viewing and formula sampling require the running game but do not alter process data or saves' },
  { scope: 'Game files and settings', status: '4 pages integrated', tone: 'ok', detail: 'Drop and crafting rules, version compatibility, game-file maintenance, and language/display' },
  { scope: 'Runtime patch coverage', status: '60 integrated / 4 pending', tone: 'ok', detail: '59 dual-version verified catalog features and 1 executable-locked candidate are integrated; 4 other candidates remain hidden until field or layout evidence is sufficient' },
  { scope: 'Runtime patch catalog', status: '60 / 83 / 81', tone: 'ok', detail: '60 features / 83 sites / 81 AOBs select strict signatures, original bytes, and unique-hit evidence for the 2.0.2 or 2.0.3 executable' },
  { scope: 'DLC 2.0.2 delta audit', status: '58 stable + 1 EXE candidate + 1 field fix', tone: 'ok', detail: 'The Glass Cannon self-stun removal candidate was added; wrightstone capture and the auto-perfect-guard combo fix keep their independent version guards and writeback' },
  { scope: 'Current maintenance delta', status: '2 / 2 verified', tone: 'ok', detail: 'Title search supports pinyin; continuous challenges use a unique signature, three-byte patch, and writeback verification' },
  { scope: 'Real game-process E2E', status: 'Critical paths verified', tone: 'ok', detail: 'Game 2.0.3 lifecycle checks cover camera, audio, QOL, damage capture, virtual-sigil entry, spatial/gravity, and runtime patches; untested gameplay effects remain experimental' },
])

const iconCoverageRows = computed(() => language.value === 'zh' ? [
  { scope: '角色图标', status: '29 / 29', tone: 'ok', detail: '当前角色目录全部精确映射' },
  { scope: '可玩主动技能', status: '261 / 262', tone: 'flow', detail: '缺 1 个可证明精确对应的官方 PNG' },
  { scope: '因子图标', status: '186 / 187', tone: 'flow', detail: '缺口保持空缺，不使用近似图标' },
  { scope: '武器图标', status: '159 / 163', tone: 'flow', detail: '缺 4 个 DLC 武器的可证明精确资源' },
  { scope: '召唤石图标', status: '189 / 189', tone: 'ok', detail: '当前召唤石目录全部精确映射' },
  { scope: '物品图标', status: '301 / 312', tone: 'flow', detail: '11 个目录项尚无可证明精确 PNG' },
] : [
  { scope: 'Character icons', status: '29 / 29', tone: 'ok', detail: 'Every current character entry has an exact mapping' },
  { scope: 'Playable active skills', status: '261 / 262', tone: 'flow', detail: '1 exact official PNG is still missing' },
  { scope: 'Sigil icons', status: '186 / 187', tone: 'flow', detail: 'The gap remains empty instead of using a look-alike icon' },
  { scope: 'Weapon icons', status: '159 / 163', tone: 'flow', detail: '4 DLC weapons still lack provably exact assets' },
  { scope: 'Summon icons', status: '189 / 189', tone: 'ok', detail: 'Every current summon entry has an exact mapping' },
  { scope: 'Item icons', status: '301 / 312', tone: 'flow', detail: '11 catalog entries still lack provably exact PNGs' },
])

function localizedMeta(meta) {
  return {
    ...meta,
    title: translateText(meta.title),
    eyebrow: translateText(meta.eyebrow),
    status: translateText(meta.status),
    description: translateText(meta.description),
    usage: (meta.usage || []).map(translateText),
    caution: translateText(meta.caution),
    speaker: translateText(meta.speaker),
    note: translateText(meta.note),
  }
}
function toolTabTitle(id) {
  const meta = localizedToolMeta.value[id]
  if (!meta) return ''
  const mode = toolNavigationModes[id]
  const hints = language.value === 'en'
    ? {
        offline: 'Fully exit the game first',
        live: 'Start the game and connect first',
        background: 'Starts only when you click; then keeps running in the background',
        file: 'Edits or checks local game files',
        readonly: 'Read-only local analysis',
        local: 'Local tool; no game connection required',
      }
    : {
        offline: '需先完全退出游戏',
        live: '需先启动游戏并连接进程',
        background: '点击后才开启，并持续在后台运行',
        file: '修改或检查本地游戏文件',
        readonly: '本地只读分析',
        local: '本机工具，无需连接游戏',
      }
  const hint = hints[mode] || ''
  return [meta.title, meta.eyebrow, hint].filter(Boolean).join(' · ')
}
function toolNavigationMode(id) {
  return toolNavigationModes[id] || ''
}
function toolTagLabel(id) {
  const labels = language.value === 'en'
    ? { offline: 'Offline', live: 'Live', background: 'Background', file: 'Game File', readonly: 'Read Only', local: 'Local' }
    : { offline: '离线', live: '实时', background: '后台', file: '游戏文件', readonly: '只读', local: '本机' }
  return labels[toolNavigationMode(id)] || ''
}
const localizedToolMeta = computed(() => Object.fromEntries(Object.entries(toolMeta).map(([id, meta]) => [id, localizedMeta(meta)])))
const currentMeta = computed(() => localizedToolMeta.value[activeTab.value] || localizedToolMeta.value.home)
const activeCachedRuntimePage = computed(() => cachedRuntimePages[activeTab.value] || null)
const isLoadoutWorkspace = computed(() => activeTab.value === 'loadoutPresets' && loadoutEditing.value)
const functionArt = reactive(Object.fromEntries(Object.entries(functionAssetManifest.assets)
  .map(([id, asset]) => [id, asset.art.variants.display.url])))
functionArt.weaponMemory = weaponMemoryLilithArt
const currentArt = computed(() => functionArt[activeTab.value] || '')
const functionStickers = reactive(Object.fromEntries(Object.entries(functionAssetManifest.assets)
  .map(([id, asset]) => [id, asset.sticker.variants.display.url])))
const currentSticker = computed(() => functionStickers[activeTab.value] || '')
const warmedImages = new Map()
const warmedTools = new Map()

function decodeImage(src) {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.decoding = 'async'
    image.onload = () => resolve(src)
    image.onerror = () => reject(new Error(`图片加载失败：${src}`))
    image.src = src
    if (typeof image.decode === 'function') image.decode().then(() => resolve(src), () => {})
  })
}

function warmImage(src, fallback = '') {
  const cacheKey = `${src}|${fallback}`
  if (!src || warmedImages.has(cacheKey)) return warmedImages.get(cacheKey)
  const pending = decodeImage(src).catch(error => fallback && fallback !== src ? decodeImage(fallback) : Promise.reject(error))
  warmedImages.set(cacheKey, pending)
  pending.catch(() => warmedImages.delete(cacheKey))
  return pending
}

function warmTool(id) {
  if (!id) return Promise.resolve()
  if (warmedTools.has(id)) return warmedTools.get(id)
  const asset = functionAssetManifest.assets[id]
  const pending = Promise.all([
    loadPageModule(id),
    warmImage(functionArt[id]),
    warmImage(functionStickers[id]),
  ]).then(([, art, sticker]) => {
    if (art) functionArt[id] = art
    if (sticker) functionStickers[id] = sticker
  })
  warmedTools.set(id, pending)
  pending.catch(() => warmedTools.delete(id))
  return pending
}

function queueWarmTool(id) {
  void warmTool(id).catch(() => {})
}

let warmIntentTimer = 0
let warmIntentID = ''
function queueWarmToolIntent(id) {
  window.clearTimeout(warmIntentTimer)
  warmIntentID = id
  warmIntentTimer = window.setTimeout(() => {
    warmIntentTimer = 0
    warmIntentID = ''
    queueWarmTool(id)
  }, 160)
}

function cancelWarmToolIntent(id) {
  if (warmIntentID !== id) return
  window.clearTimeout(warmIntentTimer)
  warmIntentTimer = 0
  warmIntentID = ''
}

function waitForTool(id, timeoutMs = 15000) {
  let timeout = 0
  return Promise.race([
    warmTool(id),
    new Promise((_, reject) => {
      timeout = window.setTimeout(() => reject(new Error('页面资源加载超时，请重试。')), timeoutMs)
    }),
  ]).finally(() => window.clearTimeout(timeout))
}

function afterNextPaint() {
  return new Promise(resolve => window.requestAnimationFrame(() => window.requestAnimationFrame(resolve)))
}

function warmGroup(group) {
  if (!group?.items?.length) return Promise.resolve()
  return warmTool(group.items[0]).catch(() => undefined)
}

function warmGroupIntent(group) {
  if (group?.items?.length) queueWarmToolIntent(group.items[0])
}

const activeGroup = computed(() => navigation.value.find(group => group.id === currentMeta.value.group) || navigation.value[0])
function scrollToolSwitcher(event) {
  const target = event.currentTarget
  if (!target || target.scrollWidth <= target.clientWidth || Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return
  event.currentTarget.scrollLeft += event.deltaY
  event.preventDefault()
}
let navigationRequest = 0
async function selectGroup(group) {
  if (!group.items.includes(activeTab.value)) {
    await selectTool(group.items[0])
  }
  if (group.id === 'tools') ensureGameDetection()
}

async function selectTool(id) {
  if (id === activeTab.value) return
  const request = ++navigationRequest
  const finishMeasure = beginPerformanceMeasure('page-switch', { from: activeTab.value, to: id })
  navigationBusy.value = true
  navigationError.value = null
  try {
    await waitForTool(id)
  } catch (error) {
    if (request === navigationRequest) navigationError.value = { id, message: String(error?.message || error) }
    finishMeasure({ cancelled: false, failed: true })
    return
  } finally {
    if (request === navigationRequest) navigationBusy.value = false
  }
  if (request !== navigationRequest) {
    finishMeasure({ cancelled: true })
    return
  }
  const previousPage = activeTab.value
  if (workspaceScroll.value) pageScrollPositions.set(previousPage, workspaceScroll.value.scrollTop)
  activeTab.value = id
  await nextTick()
  if (workspaceScroll.value) workspaceScroll.value.scrollTop = pageScrollPositions.get(id) || 0
  await afterNextPaint()
  finishMeasure({ cancelled: false })
  if (toolMeta[id]?.group === 'tools') ensureGameDetection()
}

function deployRuntimeLoadout(payload) {
  pendingRuntimeLoadout.value = payload && payload.code ? { ...payload, requestId: Date.now() } : null
  selectTool('loadoutPresets')
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
  stopRuntimeQOLSessionEvents = EventsOn('runtime-qol-session', event => {
    const sessionId = String(event?.sessionId || '').trim()
    if (!sessionId) return
    void ClipboardSetText(sessionId).then(copied => {
      if (!copied) showStatus('自动复制房间 ID 失败，请在游戏便利运行时页面手动复制。', 'error')
    }).catch(() => showStatus('自动复制房间 ID 失败，请在游戏便利运行时页面手动复制。', 'error'))
  })
  GetAppVersion().then(v => { updateInfo.currentVersion = v }).catch(() => {})
  document.addEventListener('visibilitychange', handleRuntimeCompanionVisibility)
  void refreshRuntimeCompanionSummary()
})
onBeforeUnmount(() => {
  window.clearTimeout(warmIntentTimer)
  window.clearTimeout(runtimeCompanionSummaryTimer)
  runtimeCompanionSummaryRequest++
  document.removeEventListener('visibilitychange', handleRuntimeCompanionVisibility)
  stopRuntimeQOLSessionEvents()
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

function loadFile(path, notify = true) {
  return GetStatus(path).then((info) => {
    Object.assign(state, info)
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
      isLoaded.value = true
      showStatus('游戏文件识别成功', 'success')
    })
    .catch((err) => showStatus(String(err), 'error'))
}

function refreshStatus() {
  if (!state.exePath) return Promise.resolve()
  return GetStatus(state.exePath).then((info) => {
    Object.assign(state, info)
  })
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
  OpenReleasePage().catch((err) => showStatus(String(err), 'error'))
}

let statusTimer = 0
function showStatus(message, type) {
  window.clearTimeout(statusTimer)
  saveStatus.value = translateText(translateRuntimeCompatibilityError(message, language.value))
  statusType.value = type
  statusTimer = window.setTimeout(() => { saveStatus.value = '' }, 3600)
}

</script>

<template>
  <div class="app-window">
    <header class="titlebar" style="--wails-draggable:drag" @dblclick.self="WindowToggleMaximise">
      <div class="titlebar-brand">
        <span class="brand-glyph">✦</span>
        <span class="titlebar-title">碧蓝幻想：Relink 空域工坊</span>
        <span class="build-chip">GAME 2.0.3</span>
        <span class="build-chip release-build">v2.0.14</span>
      </div>
      <div v-if="naturalDropRecovery.blocked || showCTFeatureStatus || activeRuntimeCompanions.length" class="titlebar-runtime-sessions" style="--wails-draggable:no-drag">
        <button
          v-if="naturalDropRecovery.blocked"
          type="button"
          class="titlebar-natural-drop-recovery"
          role="status"
          :aria-label="naturalDropRecoveryCopy.label"
          :title="language === 'en' ? naturalDropRecoveryCopy.title : (naturalDropRecovery.detail || naturalDropRecoveryCopy.title)"
          @click="selectTool('naturalDrop')"
        >
          <span aria-hidden="true"></span>
          {{ naturalDropRecoveryCopy.label }}
        </button>
        <button
          v-if="showCTFeatureStatus"
          type="button"
          class="titlebar-patch-session"
          :class="{ 'is-releasing': ctFeatureSession.releasePending }"
          :title="ctFeatureStatusTitle"
          @click="selectTool(ctFeatureSession.route)"
        >
          <span aria-hidden="true"></span>
          {{ ctFeatureStatusText }}
        </button>
        <button
          v-for="companion in activeRuntimeCompanions"
          :key="companion.id"
          type="button"
          class="titlebar-companion-session"
          :class="{ 'needs-recovery': companion.recoveryRequired }"
          :title="`${companion.label} · ${companion.stateTitle}`"
          @click="selectTool(companion.route)"
        >
          <span aria-hidden="true"></span>
          {{ companion.label }}{{ language === 'en' ? ' ' : '' }}{{ companion.stateLabel }}
        </button>
      </div>
      <transition name="toast">
        <div v-if="saveStatus" class="titlebar-status" :class="statusType">
          <span class="status-light"></span>{{ saveStatus }}
        </div>
      </transition>
      <div class="titlebar-controls" style="--wails-draggable:no-drag">
        <button class="win-btn" @click="WindowMinimise" title="最小化" aria-label="最小化"><span class="minimize-line"></span></button>
        <button class="win-btn" @click="WindowToggleMaximise" title="最大化或还原" aria-label="最大化或还原"><span class="maximise-box"></span></button>
        <button class="win-btn close" @click="Quit" title="关闭" aria-label="关闭"><span class="close-lines"></span></button>
      </div>
    </header>

    <div class="app-body" :class="{ 'home-mode': activeTab === 'home', 'sidebar-collapsed': sidebarCollapsed, 'loadout-workspace': isLoadoutWorkspace, 'art-visible': activeTab !== 'home' && !isLoadoutWorkspace && !artCollapsed }" style="--wails-draggable:no-drag">
      <aside class="sidebar">
        <button class="sidebar-home-compact" type="button" title="返回功能首页" aria-label="返回功能首页" @click="selectTool('home')">
          <span aria-hidden="true">⌂</span>
        </button>
        <button class="sidebar-heading" type="button" title="返回功能首页" @click="selectTool('home')">
          <span class="sidebar-kicker">GBFR PE PATCH TOOL</span>
          <strong>碧蓝幻想：Relink 空域工坊</strong>
          <span>碧蓝幻想 Relink 养成工具集</span>
        </button>
        <nav class="primary-nav" aria-label="主要功能">
          <button
            v-for="group in navigation"
            :key="group.id"
            class="nav-item"
            :class="{ active: activeGroup.id === group.id }"
            :aria-current="activeGroup.id === group.id ? 'page' : undefined"
            :title="`${group.label} · ${group.caption}`"
            @pointerenter="warmGroupIntent(group)"
            @pointerleave="cancelWarmToolIntent(group.items[0])"
            @pointerdown="warmGroup(group)"
            @focus="warmGroup(group)"
            @click="selectGroup(group)"
          >
            <span class="nav-mark">{{ group.mark }}</span>
            <span class="nav-copy"><strong>{{ group.label }}</strong><small>{{ group.caption }}</small></span>
            <span class="nav-arrow">›</span>
          </button>
        </nav>
        <!-- Q版角色是左栏常驻元素；紧凑尺寸只收起气泡，不删除图片。 -->
        <div class="sidebar-mascot" :class="{ 'has-sticker': currentSticker }" v-if="activeTab !== 'home' && currentMeta.speaker" :title="`${currentMeta.speaker}：${currentMeta.note}`">
          <img v-if="currentSticker" class="sidebar-mascot-img" :src="currentSticker" :alt="currentMeta.speaker" loading="eager" decoding="async">
          <div class="sidebar-mascot-say"><b>{{ currentMeta.speaker }}</b><p>{{ currentMeta.note }}</p></div>
        </div>
        <div class="sidebar-foot">
          <div class="target-row"><span class="target-dot"></span><div><strong>{{ language === 'en' ? 'Current Compatibility' : '当前适配版本' }}</strong><small>{{ language === 'en' ? 'Relink 2.0.3 · live layouts and offline tables are integrated' : 'Relink 2.0.3 · 实时布局与离线表已集成' }}</small></div></div>
          <a href="https://github.com/Whitelinker574/GBFR-PE-Patch-Tool" target="_blank" rel="noreferrer">项目仓库 ↗</a>
        </div>
      </aside>

      <section class="workspace">
        <div v-if="activeTab !== 'home' && !isLoadoutWorkspace" class="workspace-bar">
            <div class="breadcrumb"><span>{{ activeGroup.label }}</span><b>/</b><strong>{{ currentMeta.title }}</strong></div>
            <div class="workspace-actions">
              <div class="workspace-state"><span :class="['state-dot', currentMeta.tone]"></span>{{ currentMeta.status }}</div>
              <SaveBackupDrawer v-if="!['formulaSampler', 'selectedItemMonitor'].includes(activeTab)" @status="showStatus" />
            </div>
        </div>

        <div v-if="activeTab !== 'home' && !isLoadoutWorkspace" class="tool-switcher-shell" :data-group="activeGroup.id">
          <button
            type="button"
            class="tool-switcher-collapse"
            :title="sidebarCollapsed ? '展开左侧目录' : '收起左侧目录'"
            aria-label="收起或展开左侧目录"
            @click="toggleSidebar"
          >
            <span aria-hidden="true">{{ sidebarCollapsed ? '›' : '‹' }}</span>
          </button>
          <nav ref="toolSwitcher" class="tool-switcher ui-tabs" :data-group="activeGroup.id" aria-label="同类功能切换" @wheel="scrollToolSwitcher">
            <button
              v-for="id in activeGroup.items"
              :key="id"
              :data-tool="id"
              class="ui-tab"
              :class="{ active: activeTab === id, waiting: localizedToolMeta[id].tone === 'waiting' }"
              :aria-current="activeTab === id ? 'page' : undefined"
              :title="toolTabTitle(id)"
              @pointerenter="queueWarmToolIntent(id)"
              @pointerleave="cancelWarmToolIntent(id)"
              @pointerdown="queueWarmTool(id)"
              @focus="queueWarmTool(id)"
              @click="selectTool(id)"
            >
              {{ localizedToolMeta[id].title.replace(/（[^）]*）/g, '') }}
              <span v-if="toolNavigationMode(id)" :class="['switcher-tag', toolNavigationMode(id)]">{{ toolTagLabel(id) }}</span>
              <span v-if="localizedToolMeta[id].tone === 'waiting'" class="switcher-dot"></span>
            </button>
          </nav>
        </div>

        <div v-if="navigationBusy || navigationError" class="navigation-load-state" :class="{ error: navigationError }" role="status">
          <span>{{ navigationError ? navigationError.message : (language === 'en' ? 'Preparing page and images…' : '正在准备页面与图片…') }}</span>
          <button v-if="navigationError" class="ui-btn is-ghost is-sm" @click="selectTool(navigationError.id)">{{ language === 'en' ? 'Retry' : '重试' }}</button>
        </div>

        <div ref="workspaceScroll" class="workspace-scroll" :class="{ 'tool-workspace': activeTab !== 'home', 'loadout-workspace-scroll': isLoadoutWorkspace }">
          <div class="workspace-scene">
          <HomeJournal v-if="activeTab === 'home'" key="home" :version="updateInfo.currentVersion" @warm="queueWarmTool" @open="selectTool" />

          <section v-show="activeTab !== 'home'" class="tool-stage" :class="{ 'art-collapsed': artCollapsed || !currentArt, 'loadout-dedicated': isLoadoutWorkspace }" :data-tool="activeTab" :style="{ '--function-art': `url('${currentArt}')` }">
            <section class="tool-center-scroll">
              <header v-if="!isLoadoutWorkspace" class="tool-page-heading">
                <div class="eyebrow">{{ currentMeta.eyebrow }}</div>
                <h1>{{ currentMeta.title }}</h1>
                <p>{{ currentMeta.description }}</p>
              </header>

              <main class="tool-panel" :data-tool="activeTab">
            <RuntimePatchFeatures
              v-if="runtimePatchesMounted"
              v-show="isRuntimePatchTab"
              :mode="runtimePatchMode"
              @status="showStatus"
              @session-change="updateCTFeatureSession"
            />
            <RuntimePatchMonitor
              v-if="runtimeMonitorMounted"
              v-show="isRuntimeMonitorTab"
              :mode="runtimeMonitorMode"
              :page-active="isRuntimeMonitorTab"
              @status="showStatus"
              @deploy-loadout="deployRuntimeLoadout"
              @runtime-state="updateRuntimeCompanionState"
            />
            <KeepAlive>
              <component v-if="activeCachedRuntimePage" :is="activeCachedRuntimePage" :key="activeTab" @status="showStatus" @runtime-state="updateRuntimeCompanionState" />
            </KeepAlive>
            <KeepAlive>
              <LoadoutViewer v-if="activeTab === 'loadoutPresets'" :pending-import="pendingRuntimeLoadout" @import-consumed="pendingRuntimeLoadout = null" @status="showStatus" @editing-change="loadoutEditing = $event" />
            </KeepAlive>
            <KeepAlive>
              <NaturalDropLab v-if="activeTab === 'naturalDrop'" @status="showStatus" />
            </KeepAlive>
            <KeepAlive>
              <AudioMixerLab v-if="activeTab === 'audioMixer'" @status="showStatus" @runtime-state="updateRuntimeCompanionState" />
            </KeepAlive>
            <KeepAlive>
              <CameraLab v-if="activeTab === 'camera'" @status="showStatus" @runtime-state="updateRuntimeCompanionState" />
            </KeepAlive>
            <KeepAlive>
              <VirtualSigilLab v-if="activeTab === 'virtualSigils'" @status="showStatus" @runtime-state="updateRuntimeCompanionState" />
            </KeepAlive>
			<KeepAlive>
			  <RuntimeQOLLab v-if="activeTab === 'runtimeQOL'" @status="showStatus" />
			</KeepAlive>
            <ProgressionEditor v-if="!activeCachedRuntimePage && !isRuntimePatchTab && activeTab === 'progression'" @status="showStatus" />
            <SigilGenerator v-else-if="activeTab === 'sigil'" @status="showStatus" />
            <WrightstoneGenerator v-else-if="activeTab === 'wrightstone'" @status="showStatus" />
            <SummonSaveEditor v-else-if="activeTab === 'summonSave'" @status="showStatus" />
            <CharaStats v-else-if="activeTab === 'chara'" @status="showStatus" />
            <SaveEditor v-else-if="activeTab === 'save'" @status="showStatus" />
            <SaveDiffLab v-else-if="activeTab === 'saveDiff'" @status="showStatus" />
            <MonsterEnhance v-else-if="activeTab === 'monster'" @status="showStatus" />
            <LanguageSettings v-else-if="activeTab === 'language'" />

            <div v-else-if="activeTab === 'compatibility'" class="compat-dashboard ui-page-stack">
              <section class="calibration-grid ui-stat-grid">
                <article class="calibration-card ui-card ui-stat primary-card">
                  <div class="card-kicker">工具版本</div>
                  <strong>{{ updateInfo.currentVersion }}</strong>
                  <p>{{ updateInfo.latestVersion ? `社区最新 ${updateInfo.latestVersion}` : '尚未检查社区 Release' }}</p>
                  <div class="card-actions">
                    <button class="action ui-btn is-primary is-sm" @click="checkUpdate" :disabled="updateLoading">{{ updateLoading ? '检查中…' : '检查更新' }}</button>
                    <button class="action ui-btn is-ghost is-sm" @click="openReleasePage">打开 Release</button>
                  </div>
                </article>
                <article class="calibration-card ui-card ui-stat">
                  <div class="card-kicker">游戏文件</div>
                  <strong>{{ isDetecting ? '检测中' : isLoaded ? '已识别' : '未识别' }}</strong>
                  <p :title="state.exePath">{{ state.exePath || '未找到 granblue_fantasy_relink.exe' }}</p>
                  <span class="file-meta">{{ state.fileSize ? `${(state.fileSize / 1024 / 1024).toFixed(1)} MB` : compatibilityCopy.manualFile }}</span>
                </article>
                <article class="calibration-card ui-card ui-stat">
                  <div class="card-kicker">{{ compatibilityCopy.baseline }}</div>
                  <strong>{{ compatibilityCopy.baselineVersion }}</strong>
                  <p>{{ compatibilityCopy.baselineSummary }}</p>
                  <span class="file-meta">{{ compatibilityCopy.baselineBoundary }}</span>
                </article>
              </section>

              <section class="compat-section ui-card ui-panel">
                <div class="compat-heading"><div><span>{{ compatibilityCopy.featureKicker }}</span><h2>{{ compatibilityCopy.featureTitle }}</h2></div><p>{{ compatibilityCopy.featureHint }}</p></div>
                <div class="matrix">
                  <div class="matrix-row head"><span>{{ compatibilityCopy.scope }}</span><span>{{ compatibilityCopy.status }}</span><span>{{ compatibilityCopy.evidence }}</span></div>
                  <div v-for="row in compatibilityRows" :key="row.scope" class="matrix-row"><span>{{ row.scope }}</span><b :class="row.tone">{{ row.status }}</b><span>{{ row.detail }}</span></div>
                </div>
              </section>

              <section class="compat-section ui-card ui-panel">
                <div class="compat-heading"><div><span>{{ compatibilityCopy.resourceKicker }}</span><h2>{{ compatibilityCopy.resourceTitle }}</h2></div><p>{{ compatibilityCopy.resourceHint }}</p></div>
                <div class="matrix">
                  <div class="matrix-row head"><span>{{ compatibilityCopy.scope }}</span><span>{{ compatibilityCopy.status }}</span><span>{{ compatibilityCopy.evidence }}</span></div>
                  <div v-for="row in iconCoverageRows" :key="row.scope" class="matrix-row"><span>{{ row.scope }}</span><b :class="row.tone">{{ row.status }}</b><span>{{ row.detail }}</span></div>
                </div>
              </section>

              <section class="compat-section legacy-links ui-card ui-panel">
                <div class="compat-heading"><div><span>{{ compatibilityCopy.experimentKicker }}</span><h2>{{ compatibilityCopy.experimentTitle }}</h2></div></div>
                <button class="ui-card" @click="selectTool('monster')"><strong>{{ compatibilityCopy.experimentName }}</strong><small>{{ compatibilityCopy.experimentDetail }}</small><span>{{ compatibilityCopy.open }}</span></button>
              </section>
            </div>

            <div v-else-if="activeTab === 'patch'" class="legacy-patch ui-page-stack">
              <section class="patch-file-row ui-card path-card ui-panel is-compact">
                <label class="ui-field-label" for="game-exe-path">{{ isDetecting ? '正在扫描 Steam 安装路径…' : isLoaded ? '已定位游戏文件' : '游戏 EXE 路径' }}</label>
                <div class="path-input-row ui-control-group is-responsive"><input id="game-exe-path" v-model="manualPath" class="ui-input" placeholder="粘贴 granblue_fantasy_relink.exe 完整路径" @keyup.enter="applyManualPath"><button class="action ui-btn is-primary" @click="applyManualPath" :disabled="!manualPath.trim()">识别文件</button></div>
                <div v-if="state.exePath" class="detected-file"><span :title="state.exePath">{{ state.exePath }}</span><b>{{ (state.fileSize / 1024 / 1024).toFixed(1) }} MB</b></div>
              </section>
              <section class="backup-card ui-card ui-panel is-compact"><div><strong>EXE 备份与恢复</strong><span>{{ state.backupExists ? `已有 ${(state.backupSize / 1024 / 1024).toFixed(1)} MB 备份` : '尚未创建备份' }}</span></div><div class="backup-policy ui-seg" role="group" aria-label="备份策略"><button type="button" class="ui-seg-btn" :class="{ 'is-on': !forceBackup }" :aria-pressed="!forceBackup" @click="forceBackup=false"><b>保留现有备份</b><small>推荐</small></button><button type="button" class="ui-seg-btn" :class="{ 'is-on': forceBackup }" :aria-pressed="forceBackup" @click="forceBackup=true"><b>重新创建原始备份</b><small>会替换旧备份</small></button></div><div class="patch-actions ui-actions"><button class="action ui-btn" @click="backup">创建备份</button><button class="action ui-btn is-danger" @click="restore" :disabled="!state.backupExists">恢复备份</button></div></section>
            </div>
              </main>
            </section>

            <button v-if="!isLoadoutWorkspace && currentArt" class="art-toggle" :class="{ 'is-collapsed': artCollapsed }" :title="artCollapsed ? '展开立绘' : '收起立绘 · 拓宽操作区'" :aria-label="artCollapsed ? '展开立绘' : '收起立绘'" @click="toggleArt">{{ artCollapsed ? '‹' : '›' }}</button>
            <div v-if="!isLoadoutWorkspace && currentArt && !artCollapsed" class="art-caption" aria-hidden="true"><span>{{ currentMeta.speaker }}</span><small>{{ currentMeta.eyebrow }}</small></div>
          </section>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.app-window {
  --titlebar-size:42px;
  position:relative;
  isolation:isolate;
  width:100%;
  height:100%;
  overflow:hidden;
  color:var(--text-primary);
  background:#ead9b6;
  font-family:var(--font-ui);
}
.app-window::before {
  content:"";
  position:absolute;
  z-index:-2;
  inset:0;
  background-image:
    linear-gradient(120deg,rgba(255,252,239,.54),rgba(225,202,154,.3)),
    url('../assets/gbfr/parchment-ui-v2.webp');
  background-position:center;
  background-size:cover;
  filter:saturate(.92) contrast(.98);
}
button,input,select { font:inherit; }

.titlebar {
  --window-controls-width:126px;
  position:relative;
  z-index:20;
  height:var(--titlebar-size);
  display:flex;
  align-items:center;
  padding-right:var(--window-controls-width);
  border-bottom:1px solid rgba(126,91,42,.35);
  background:linear-gradient(90deg,#594937,#756044 52%,#5b4a37);
  box-shadow:0 4px 15px rgba(76,55,28,.18);
  user-select:none;
}
.titlebar-brand {
  min-width:0;
  display:flex;
  align-items:center;
  gap:var(--space-3);
  padding-left:var(--space-5);
}
.brand-glyph {
  width:22px;
  height:22px;
  flex:0 0 22px;
  display:grid;
  place-items:center;
  border:1px solid rgba(255,229,169,.7);
  border-radius:var(--radius-sm);
  color:#ffe5a9;
  background:rgba(255,255,255,.06);
  font-size:var(--fs-sm);
}
.titlebar-title {
  min-width:0;
  overflow:hidden;
  color:#fff4d8;
  font-size:var(--fs-sm);
  font-weight:var(--fw-bold);
  letter-spacing:.04em;
  text-overflow:ellipsis;
  white-space:nowrap;
}
.build-chip {
  flex:0 0 auto;
  padding:2px var(--space-2);
  border:1px solid rgba(255,229,169,.35);
  border-radius:var(--radius-pill);
  color:#f3e3c2;
  background:rgba(255,255,255,.08);
  font-size:var(--fs-xs);
}
.build-chip.test-build {
  border-color:rgba(255,208,118,.58);
  color:#fff1c7;
  background:rgba(151,92,23,.46);
  font-weight:var(--fw-bold);
  letter-spacing:.05em;
}
.titlebar-runtime-sessions {
  min-width:0;
  display:flex;
  align-items:center;
  gap:var(--space-2);
  margin-left:var(--space-4);
  overflow-x:auto;
  overflow-y:hidden;
  scrollbar-width:none;
  overscroll-behavior-x:contain;
}
.titlebar-runtime-sessions::-webkit-scrollbar { display:none; }
.titlebar-patch-session,
.titlebar-companion-session {
  flex:0 0 auto;
  display:inline-flex;
  align-items:center;
  gap:var(--space-2);
  padding:3px var(--space-3);
  border:1px solid rgba(150,224,204,.52);
  border-radius:var(--radius-pill);
  color:#dff9ef;
  background:rgba(24,100,83,.58);
  font-size:var(--fs-xs);
  font-weight:var(--fw-semibold);
  white-space:nowrap;
  cursor:pointer;
}
.titlebar-patch-session span,
.titlebar-companion-session span {
  width:7px;
  height:7px;
  border-radius:50%;
  background:#8ce0c8;
  box-shadow:0 0 0 3px rgba(140,224,200,.13);
}
.titlebar-patch-session.is-releasing {
  border-color:rgba(255,213,133,.52);
  color:#fff0c7;
  background:rgba(130,85,25,.62);
}
.titlebar-patch-session.is-releasing span { background:#ffd585; }
.titlebar-companion-session {
  color:#f8ead0;
  border-color:rgba(255,229,169,.35);
  background:rgba(63,49,33,.34);
}
.titlebar-companion-session.needs-recovery {
  color:#fff0c7;
  border-color:rgba(255,213,133,.52);
  background:rgba(130,85,25,.62);
}
.titlebar-companion-session.needs-recovery span { background:#ffd585; }
.titlebar-natural-drop-recovery {
  color:#fff0c7;
  border-color:rgba(255,213,133,.58);
  background:rgba(130,52,25,.72);
}
.titlebar-natural-drop-recovery span { background:#ffad85; }
.titlebar-status {
  position:absolute;
  z-index:1;
  top:50%;
  left:50%;
  transform:translate(-50%,-50%);
  min-width:0;
  max-width:min(42vw,520px,calc(100% - var(--window-controls-width) - 320px));
  display:flex;
  align-items:center;
  gap:var(--space-2);
  overflow:hidden;
  padding:4px var(--space-4);
  border:1px solid var(--border-default);
  border-radius:var(--radius-pill);
  color:var(--text-secondary);
  background:#ead8b2;
  box-shadow:var(--shadow-1);
  font-size:var(--fs-sm);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.titlebar-status.success { color:var(--success-ink); }
.titlebar-status.error { color:var(--danger-ink); }
.status-light {
  width:7px;
  height:7px;
  flex:0 0 7px;
  border-radius:50%;
  background:currentColor;
}
.titlebar-controls {
  position:absolute;
  z-index:2;
  top:0;
  right:0;
  display:flex;
  height:100%;
}
.win-btn {
  width:42px;
  height:100%;
  display:grid;
  place-items:center;
  border:0;
  color:#e5d7bc;
  background:transparent;
  cursor:pointer;
}
.win-btn:hover { color:#fff; background:rgba(255,255,255,.12); }
.win-btn.close:hover { color:var(--text-on-accent); background:var(--danger-ink); }
.minimize-line { width:12px; height:1px; background:currentColor; }
.maximise-box {
  width:12px;
  height:10px;
  border:1px solid currentColor;
  border-radius:1px;
}
.close-lines { position:relative; width:13px; height:13px; }
.close-lines::before,.close-lines::after {
  content:"";
  position:absolute;
  top:6px;
  left:0;
  width:13px;
  height:1px;
  background:currentColor;
  transform:rotate(45deg);
}
.close-lines::after { transform:rotate(-45deg); }

.app-body {
  position:relative;
  height:calc(100% - var(--titlebar-size));
  display:grid;
  grid-template-columns:208px minmax(0,1fr);
  overflow:hidden;
}
.app-body.home-mode,
.app-body.loadout-workspace { grid-template-columns:minmax(0,1fr); }
.home-mode .sidebar,
.loadout-workspace > .sidebar { display:none; }

.sidebar {
  position:relative;
  min-height:0;
  display:flex;
  flex-direction:column;
  padding:var(--space-7) var(--space-4) var(--space-5);
  overflow:hidden;
  border-right:1px solid rgba(130,96,48,.3);
  background:#f0e2c2;
  box-shadow:8px 0 28px rgba(90,66,31,.12),inset -4px 0 rgba(145,110,57,.04);
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
  background:url('../assets/gbfr/journal-page-corner.svg') left top / contain no-repeat;
  opacity:.46;
}
.sidebar-heading,
.sidebar-home-compact,
.primary-nav,
.sidebar-mascot,
.sidebar-foot { position:relative; z-index:1; }
.sidebar-home-compact {
  width:48px;
  height:48px;
  flex:0 0 48px;
  display:none;
  place-items:center;
  border:1px solid var(--accent-border);
  border-radius:var(--radius-md);
  color:var(--accent-hover);
  background:color-mix(in srgb,var(--surface-card-pop) 72%,transparent);
  box-shadow:inset 0 1px rgba(255,255,255,.58);
  font-size:24px;
  cursor:pointer;
}
.sidebar-home-compact:hover { background:var(--state-hover); }
.sidebar-heading {
  width:100%;
  min-width:0;
  display:block;
  padding:var(--space-4) var(--space-3) var(--space-5);
  border:0;
  border-bottom:1px solid var(--border-soft);
  color:var(--text-primary);
  background:transparent;
  text-align:left;
  cursor:pointer;
}
.sidebar-heading:hover { background:var(--state-hover); }
.sidebar-heading span,.sidebar-heading strong { display:block; }
.sidebar-kicker {
  margin-bottom:var(--space-2);
  color:var(--accent);
  font-size:var(--fs-xs);
  font-weight:var(--fw-bold);
  letter-spacing:.12em;
}
.sidebar-heading strong {
  font-size:var(--fs-base);
  font-weight:var(--fw-bold);
  line-height:var(--lh-tight);
}
.sidebar-heading > span:last-child {
  margin-top:var(--space-1);
  color:var(--text-muted);
  font-size:var(--fs-xs);
  line-height:var(--lh-normal);
}
.primary-nav {
  min-height:0;
  display:flex;
  flex-direction:column;
  gap:var(--space-2);
  padding-top:var(--space-5);
}
.nav-item {
  width:100%;
  min-height:54px;
  display:grid;
  grid-template-columns:32px minmax(0,1fr) auto;
  align-items:center;
  gap:var(--space-3);
  padding:var(--space-2) var(--space-3);
  border:1px solid transparent;
  border-radius:var(--radius-md);
  color:var(--text-secondary);
  background:transparent;
  text-align:left;
  cursor:pointer;
}
.nav-item:hover { border-color:var(--border-soft); background:var(--state-hover); }
.nav-item.active {
  border-color:var(--selected-border);
  color:var(--selected-fg);
  background:var(--selected-bg);
  box-shadow:3px 0 0 var(--selected-bar);
}
.nav-mark {
  width:30px;
  height:30px;
  display:grid;
  place-items:center;
  border:1px solid currentColor;
  border-radius:var(--radius-sm);
  font-size:var(--fs-sm);
  font-weight:var(--fw-bold);
}
.nav-copy { min-width:0; }
.nav-copy strong,.nav-copy small { display:block; }
.nav-copy strong {
  overflow:hidden;
  font-size:var(--fs-md);
  font-weight:var(--fw-bold);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.nav-copy small {
  margin-top:2px;
  overflow:hidden;
  color:inherit;
  font-size:var(--fs-xs);
  opacity:.78;
  text-overflow:ellipsis;
  white-space:nowrap;
}
.nav-arrow { font-size:var(--fs-lg); }
.sidebar-foot {
  min-width:0;
  margin-top:0;
  padding:var(--space-4) var(--space-2) 0;
  border-top:1px solid var(--border-soft);
}
.sidebar-mascot {
  min-width:0;
  display:grid;
  grid-template-columns:minmax(0,1fr);
  align-items:end;
  gap:var(--space-2);
  margin-top:auto;
  padding:var(--space-3) var(--space-2) var(--space-4);
}
.sidebar-mascot.has-sticker { grid-template-columns:46px minmax(0,1fr); }
.sidebar-mascot-img {
  width:46px;
  height:50px;
  object-fit:contain;
  object-position:center bottom;
}
.sidebar-mascot-say {
  min-width:0;
  padding:var(--space-2);
  border:1px solid var(--border-soft);
  border-radius:var(--radius-md);
  background:var(--surface-card-pop);
}
.sidebar-mascot-say b {
  color:var(--accent-hover);
  font-size:var(--fs-xs);
}
.sidebar-mascot-say p {
  display:-webkit-box;
  margin:2px 0 0;
  overflow:hidden;
  color:var(--text-muted);
  font-size:var(--fs-xs);
  line-height:var(--lh-tight);
  -webkit-box-orient:vertical;
  -webkit-line-clamp:2;
}
.target-row {
  min-width:0;
  display:flex;
  align-items:center;
  gap:var(--space-2);
}
.target-dot {
  width:7px;
  height:7px;
  flex:0 0 7px;
  border-radius:50%;
  background:var(--success);
}
.target-row strong,.target-row small { display:block; }
.target-row strong { color:var(--text-secondary); font-size:var(--fs-xs); }
.target-row small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.sidebar-foot a {
  display:inline-block;
  margin-top:var(--space-3);
  color:var(--text-link);
  font-size:var(--fs-xs);
  text-decoration:none;
}
.sidebar-foot a:hover { text-decoration:underline; }

.app-body.sidebar-collapsed { grid-template-columns:70px minmax(0,1fr); }
/* Home and the dedicated loadout workspace intentionally hide the sidebar.
   Keep their content full-width even when the sidebar's collapsed state is
   remembered from another page. */
.app-body.home-mode.sidebar-collapsed,
.app-body.loadout-workspace.sidebar-collapsed { grid-template-columns:minmax(0,1fr); }
.sidebar-collapsed .sidebar { padding:var(--space-7) var(--space-3) var(--space-4); }
.sidebar-collapsed .sidebar-heading,
.sidebar-collapsed .nav-copy,
.sidebar-collapsed .nav-arrow,
.sidebar-collapsed .sidebar-foot { display:none; }
.sidebar-collapsed .sidebar-home-compact { display:grid; }
.sidebar-collapsed .primary-nav {
  width:48px;
  align-self:center;
  align-items:center;
  padding-top:var(--space-3);
}
.sidebar-collapsed .nav-item {
  width:48px;
  height:48px;
  min-height:48px;
  grid-template-columns:1fr;
  place-items:center;
  padding:0;
  box-sizing:border-box;
}
.sidebar-collapsed .nav-item.active { box-shadow:inset 3px 0 0 var(--selected-bar); }
.sidebar-collapsed .nav-mark {
  width:100%;
  height:100%;
  border:0;
}
.sidebar-collapsed .sidebar-mascot {
  width:48px;
  grid-template-columns:48px;
  place-items:center;
  padding:var(--space-2) 0;
}
.sidebar-collapsed .sidebar-mascot-img { width:48px; height:54px; }
.sidebar-collapsed .sidebar-mascot-say { display:none; }
.sidebar-collapsed .sidebar-mascot:not(.has-sticker) { display:none; }

.workspace {
  position:relative;
  isolation:isolate;
  min-width:0;
  min-height:0;
  display:flex;
  flex-direction:column;
  overflow:hidden;
  background:
    linear-gradient(105deg,rgba(255,251,238,.36),rgba(239,220,180,.18)),
    url('../assets/gbfr/parchment-ui-v2.webp') center / cover fixed;
}
.workspace-bar {
  min-height:44px;
  flex:0 0 44px;
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:var(--space-4);
  padding:0 var(--content-gutter);
  border-bottom:1px solid var(--border-soft);
  background:#ead8b2;
}
.breadcrumb {
  min-width:0;
  display:flex;
  align-items:center;
  gap:var(--space-2);
  overflow:hidden;
  color:var(--text-muted);
  font-size:var(--fs-sm);
  white-space:nowrap;
}
.breadcrumb b { color:var(--border-strong); }
.breadcrumb strong {
  overflow:hidden;
  color:var(--text-primary);
  font-weight:var(--fw-semibold);
  text-overflow:ellipsis;
}
.workspace-actions {
  min-width:0;
  display:flex;
  align-items:center;
  gap:var(--space-4);
}
.workspace-state {
  display:flex;
  align-items:center;
  gap:var(--space-2);
  color:var(--text-secondary);
  font-size:var(--fs-xs);
  white-space:nowrap;
}
.state-dot { width:7px; height:7px; border-radius:50%; background:var(--text-muted); }
.state-dot.stable { background:var(--success); }
.state-dot.live { background:var(--info); }
.state-dot.calibrate { background:var(--warning); }
.state-dot.waiting { background:var(--danger); }

.tool-switcher-shell {
  min-height:46px;
  flex:0 0 auto;
  min-width:0;
  display:grid;
  grid-template-columns:46px minmax(0,1fr);
  border-bottom:1px solid rgba(140,104,49,.23);
  background:#eddfc0;
}
.tool-switcher-collapse {
  width:46px;
  min-height:46px;
  display:grid;
  place-items:center;
  padding:0;
  border:0;
  border-right:1px solid rgba(140,104,49,.23);
  border-radius:0;
  color:#78684f;
  background:transparent;
  box-shadow:none;
  font-size:var(--fs-lg);
  cursor:pointer;
}
.tool-switcher-collapse:hover {
  color:#4e402e;
  background:rgba(126,89,40,.08);
}
.tool-switcher {
  min-width:0;
  min-height:46px;
  display:flex;
  flex-wrap:nowrap;
  align-items:stretch;
  gap:0;
  padding:0 var(--content-gutter);
  border-bottom:0;
  background:#eddfc0;
  overflow-x:auto;
  overflow-y:hidden;
  overscroll-behavior-x:contain;
  scrollbar-width:thin;
}
.tool-switcher .ui-tab {
  min-width:max-content;
  min-height:46px;
  flex:0 0 auto;
  display:inline-flex;
  justify-content:center;
  align-items:center;
  gap:var(--space-2);
  padding:0 var(--space-4);
  border:0;
  border-bottom:2px solid transparent;
  border-radius:0;
  font-size:var(--fs-sm);
  font-weight:var(--fw-bold);
  color:#78684f;
  background:transparent;
  box-shadow:none;
  white-space:nowrap;
  text-align:center;
  line-height:1;
}
.tool-switcher .ui-tab.active {
  color:#4e402e;
  background:transparent;
  border-bottom-color:#9a7440;
  box-shadow:none;
}
.switcher-tag {
  display:inline-flex;
  min-height:20px;
  align-items:center;
  padding:0 var(--space-2);
  border-radius:var(--radius-pill);
  font-size:var(--fs-xs);
  line-height:1;
}
.switcher-tag.live { color:var(--info-ink); background:var(--info-bg); }
.switcher-tag.offline { color:var(--success-ink); background:var(--success-bg); }
.switcher-tag.background { color:var(--info-ink); background:color-mix(in srgb,var(--info-bg) 70%,var(--accent-soft)); }
.switcher-tag.file { color:var(--accent-hover); background:var(--accent-soft); }
.switcher-tag.readonly { color:var(--text-secondary); background:var(--surface-field); }
.switcher-tag.local { color:var(--text-muted); background:rgba(121,104,78,.11); }
.switcher-dot { width:6px; height:6px; border-radius:50%; background:var(--danger); }

.navigation-load-state {
  min-height:36px;
  flex:0 0 auto;
  display:flex;
  align-items:center;
  justify-content:center;
  gap:var(--space-3);
  padding:var(--space-2) var(--content-gutter);
  border-bottom:1px solid rgba(140,104,49,.23);
  color:var(--text-secondary);
  background:rgba(245,234,208,.94);
  font-size:var(--fs-sm);
}
.navigation-load-state.error { color:var(--danger-ink); background:var(--danger-bg); }

.workspace-scroll {
  min-width:0;
  min-height:0;
  flex:1;
  overflow:auto;
  overscroll-behavior:contain;
  scrollbar-gutter:stable;
}
.workspace-scroll.tool-workspace { padding:var(--content-gutter); }
.home-mode .workspace-scroll { padding:0; overflow:auto; scrollbar-gutter:auto; }
.home-mode .workspace-scene { height:100%; min-height:100%; }
.workspace-scene { min-width:0; min-height:100%; }

.tool-stage {
  --art-scale:160%;
  --art-x:calc(-32.55dvh + 43px);
  --art-y:calc(3dvh - 4px);
  position:relative;
  isolation:isolate;
  min-width:0;
  min-height:100%;
  display:block;
  overflow:clip;
}
.tool-stage::before {
  content:"";
  position:fixed;
  z-index:0;
  inset:calc(var(--titlebar-size) + 90px) 0 0;
  background-image:var(--function-art);
  background-repeat:no-repeat;
  background-position:right var(--art-x) top var(--art-y);
  background-size:auto var(--art-scale);
  pointer-events:none;
}
.tool-stage.art-collapsed::before,
.tool-stage.loadout-dedicated::before { display:none; }
.tool-center-scroll {
  position:relative;
  z-index:2;
  width:62%;
  min-width:0;
  min-height:0;
  padding-bottom:var(--space-8);
  container:tool-center / inline-size;
}
.tool-stage.art-collapsed .tool-center-scroll,
.tool-stage.loadout-dedicated .tool-center-scroll { width:100%; }
.tool-page-heading,.tool-panel {
  width:100%;
  max-width:none;
  margin-inline:0;
}
.tool-page-heading {
  margin-bottom:var(--space-5);
  padding:var(--space-6) var(--space-7);
  border:1px solid rgba(127,88,38,.42);
  border-radius:var(--radius-lg);
  background:#f7ebcf;
  box-shadow:var(--shadow-1);
}
.tool-page-heading .eyebrow {
  color:var(--accent);
  font-size:var(--fs-xs);
  font-weight:var(--fw-bold);
  letter-spacing:.12em;
}
.tool-page-heading h1 {
  margin:var(--space-1) 0 var(--space-2);
  color:var(--text-primary);
  font-family:var(--font-display);
  font-size:clamp(20px,2vw,var(--fs-xl));
  font-weight:var(--fw-bold);
  line-height:var(--lh-tight);
}
.tool-page-heading p {
  max-width:72ch;
  margin:0;
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}
.tool-panel {
  min-width:0;
  container:tool-panel / inline-size;
}
.tool-panel :deep(.ui-page),
.tool-panel :deep(.ui-page-stack) {
  width:100%;
  max-width:none;
}
.tool-panel :deep(.root),
.tool-panel :deep(.sigil-container),
.tool-panel :deep(.wrightstone-container),
.tool-panel :deep(.memory-sigil) {
  width:100%;
  max-width:100%;
  margin:0;
}
.tool-panel[data-tool="runtime"] :deep(.root > .section > .header),
.tool-panel[data-tool="chara"] :deep(.root > .section > .header),
.tool-panel[data-tool="overlimit"] :deep(.root > .section > .header),
.tool-panel[data-tool="monster"] :deep(.root > .section > .header),
.tool-panel[data-tool="summon"] :deep(.root > .section > .header) { display:none; }
.tool-panel[data-tool="progression"] :deep(.save-title > div:first-child) { display:none; }
.tool-panel[data-tool="progression"] :deep(.save-title) { min-height:0; justify-content:flex-end; }

.tool-stage[data-tool="progression"] { --art-scale:160%; --art-x:calc(-32.55dvh + 43px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="sigil"] { --art-scale:160%; --art-x:calc(-32.55dvh + 43px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="sigilMemory"] { --art-scale:160%; --art-x:calc(-32.55dvh + 43px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="loadout"] { --art-scale:160%; --art-x:calc(-8.20dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="loadoutPresets"] { --art-scale:160%; --art-x:calc(-8.33dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="wrightstone"] { --art-scale:160%; --art-x:calc(-32.55dvh + 43px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="summonSave"] { --art-scale:160%; --art-x:calc(-20dvh + 27px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="wrightstoneMemory"] { --art-scale:160%; --art-x:calc(-6.77dvh + 9px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="weaponMemory"] { --art-scale:150%; --art-x:calc(-4.2dvh + 6px); --art-y:calc(2dvh - 3px); }
.tool-stage[data-tool="summon"] { --art-scale:160%; --art-x:calc(-32.55dvh + 43px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="overlimit"] { --art-scale:160%; --art-x:calc(-32.55dvh + 43px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="runtime"] { --art-scale:160%; --art-x:calc(-32.55dvh + 43px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="runtimeMonitor"] { --art-scale:160%; --art-x:calc(-9.11dvh + 12px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="spatialTools"] { --art-scale:160%; --art-x:calc(-8.20dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="selectedItemMonitor"] { --art-scale:160%; --art-x:calc(-8.20dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="saveDiff"] { --art-scale:160%; --art-x:calc(-8.20dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="naturalDrop"] { --art-scale:160%; --art-x:calc(-8.20dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="audioMixer"] { --art-scale:160%; --art-x:calc(-8.20dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="camera"] { --art-scale:160%; --art-x:calc(-8.20dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="virtualSigils"] { --art-scale:160%; --art-x:calc(-8.20dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="runtimeQOL"] { --art-scale:160%; --art-x:calc(-8.20dvh + 11px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="formulaSampler"] { --art-scale:160%; --art-x:calc(-9.11dvh + 12px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="patchCombat"] { --art-scale:160%; --art-x:calc(-7.03dvh + 9px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="patchCharacters"] { --art-scale:160%; --art-x:calc(-7.29dvh + 10px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="patchQuest"] { --art-scale:160%; --art-x:calc(-7.03dvh + 9px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="chara"] { --art-scale:160%; --art-x:calc(-32.55dvh + 43px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="save"] { --art-scale:160%; --art-x:calc(-43.10dvh + 57px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="compatibility"] { --art-scale:160%; --art-x:calc(-35.81dvh + 47px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="monster"] { --art-scale:160%; --art-x:calc(-21.48dvh + 28px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="patch"] { --art-scale:160%; --art-x:calc(-32.55dvh + 43px); --art-y:calc(3dvh - 4px); }
.tool-stage[data-tool="language"] { --art-scale:178%; --art-x:calc(-39.06dvh + 52px); --art-y:calc(-17dvh + 22px); }
.art-caption {
  position:fixed;
  z-index:3;
  right:var(--space-3);
  bottom:var(--space-3);
  left:auto;
  padding:var(--space-2) var(--space-3);
  border:1px solid var(--border-default);
  border-right:3px solid rgba(154,116,64,.72);
  border-radius:var(--radius-sm);
  background:#f4e6c7;
  box-shadow:var(--shadow-1);
  text-align:right;
}
.art-caption span,.art-caption small { display:block; }
.art-caption span { color:var(--text-primary); font-size:var(--fs-sm); font-weight:var(--fw-bold); }
.art-caption small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.art-toggle {
  position:fixed;
  z-index:4;
  top:calc(var(--titlebar-size) + 94px);
  right:var(--space-2);
  width:30px;
  height:36px;
  border:1px solid var(--border-default);
  border-radius:var(--radius-sm) 0 0 var(--radius-sm);
  color:var(--text-secondary);
  background:var(--surface-card-pop);
  box-shadow:var(--shadow-1);
  transform:translateX(1px);
  cursor:pointer;
}
.art-toggle:hover { color:var(--accent-hover); background:var(--surface-field-hover); }
.tool-stage.art-collapsed .art-toggle { right:0; border-radius:var(--radius-sm); transform:none; }

.compat-dashboard,.legacy-patch { min-width:0; }
.calibration-grid { --ui-grid-min:200px; }
.calibration-card { display:flex; min-height:150px; flex-direction:column; }
.calibration-card.primary-card { border-top:3px solid var(--accent); }
.card-kicker {
  color:var(--text-muted);
  font-size:var(--fs-xs);
  font-weight:var(--fw-bold);
  letter-spacing:.08em;
}
.calibration-card > strong {
  display:block;
  margin-top:var(--space-2);
  color:var(--text-primary);
  font-family:var(--font-data);
  font-size:var(--fs-lg);
  overflow-wrap:anywhere;
}
.calibration-card p {
  min-height:3em;
  margin:var(--space-2) 0;
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
  overflow-wrap:anywhere;
}
.file-meta { margin-top:auto; color:var(--text-muted); font-size:var(--fs-xs); }
.card-actions { display:flex; flex-wrap:wrap; gap:var(--space-2); margin-top:var(--space-3); }
.compat-heading {
  display:flex;
  align-items:flex-start;
  justify-content:space-between;
  gap:var(--space-5);
}
.compat-heading span { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.compat-heading h2 { margin:2px 0 0; color:var(--text-primary); font-size:var(--fs-base); }
.compat-heading p { margin:0; color:var(--text-muted); font-size:var(--fs-sm); }
.matrix {
  overflow:hidden;
  border:1px solid var(--border-soft);
  border-radius:var(--radius-md);
}
.matrix-row {
  display:grid;
  grid-template-columns:minmax(160px,1.1fr) minmax(96px,max-content) minmax(180px,1.4fr);
  gap:var(--space-3);
  align-items:center;
  padding:var(--space-3) var(--space-4);
  border-bottom:1px solid var(--border-soft);
  color:var(--text-secondary);
  background:var(--surface-card-pop);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}
.matrix-row:last-child { border-bottom:0; }
.matrix-row.head { color:var(--text-muted); background:var(--surface-field); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.matrix-row b { justify-self:start; padding:2px var(--space-2); border-radius:var(--radius-pill); font-size:var(--fs-xs); white-space:nowrap; }
.matrix-row b.ok { color:var(--success-ink); background:var(--success-bg); }
.matrix-row b.flow { color:var(--info-ink); background:var(--info-bg); }
.matrix-row b.pending { color:var(--warning-ink); background:var(--warning-bg); }
.legacy-links {
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
}
.legacy-links .compat-heading { grid-column:1 / -1; }
.legacy-links > button {
  min-width:0;
  display:grid;
  grid-template-columns:minmax(0,1fr) auto;
  gap:var(--space-1) var(--space-3);
  padding:var(--space-4);
  color:var(--text-primary);
  text-align:left;
  cursor:pointer;
}
.legacy-links > button:hover { border-color:var(--accent-border); background:var(--surface-field-hover); }
.legacy-links > button strong { font-size:var(--fs-sm); }
.legacy-links > button small { grid-column:1; color:var(--text-muted); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.legacy-links > button span { grid-column:2; grid-row:1 / span 2; align-self:center; color:var(--accent-hover); font-size:var(--fs-sm); }

.patch-file-row { align-items:flex-start; }
.path-input-row { width:min(100%,680px); }
.path-input-row .ui-btn { flex:0 0 auto; }
.detected-file {
  width:min(100%,680px);
  display:flex;
  justify-content:space-between;
  gap:var(--space-4);
  padding:var(--space-2) var(--space-3);
  border-radius:var(--radius-sm);
  color:var(--text-secondary);
  background:var(--surface-field);
  font-size:var(--fs-xs);
}
.detected-file span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.detected-file b { flex:0 0 auto; color:var(--text-primary); }
.backup-card {
  display:grid;
  grid-template-columns:minmax(170px,.8fr) minmax(300px,1.5fr) auto;
  align-items:center;
}
.backup-card > div:first-child strong,.backup-card > div:first-child span { display:block; }
.backup-card > div:first-child strong { color:var(--text-primary); font-size:var(--fs-md); }
.backup-card > div:first-child span { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.backup-policy { width:100%; }
.backup-policy button { display:flex; flex-direction:column; align-items:flex-start; justify-content:center; white-space:normal; }
.backup-policy b { font-size:var(--fs-sm); }
.backup-policy small { font-size:var(--fs-xs); font-weight:var(--fw-normal); }
.patch-actions { justify-content:flex-end; }

.loadout-workspace .workspace { height:100%; }
.loadout-workspace .workspace-scroll {
  height:100%;
  padding:0;
  overflow:hidden;
}
.loadout-workspace .workspace-scene,
.loadout-workspace .tool-stage,
.loadout-workspace .tool-center-scroll,
.loadout-workspace .tool-panel {
  width:100%;
  height:100%;
  min-height:0;
}
.loadout-workspace .tool-center-scroll { padding:var(--space-3); overflow:hidden; }
.tool-panel[data-tool="loadoutPresets"] :deep(.loadout-viewer.editing) { width:100%; height:100%; min-height:0; }

.toast-enter-active,.toast-leave-active { transition:opacity var(--dur-fast) var(--ease-out),transform var(--dur-fast) var(--ease-out); }
.toast-enter-from,.toast-leave-to { opacity:0; transform:translateY(-4px); }

@container tool-panel (max-width:760px) {
  .compat-heading { flex-direction:column; gap:var(--space-2); }
  .backup-card { grid-template-columns:minmax(0,1fr); align-items:stretch; }
  .patch-actions { justify-content:stretch; }
  .patch-actions .ui-btn { flex:1 1 160px; }
}
@container tool-panel (max-width:680px) {
  .matrix { border:0; border-radius:0; background:transparent; }
  .matrix-row {
    grid-template-columns:minmax(0,1fr) auto;
    margin-bottom:var(--space-2);
    border:1px solid var(--border-soft);
    border-radius:var(--radius-md);
  }
  .matrix-row.head { display:none; }
  .matrix-row > span:last-child { grid-column:1 / -1; color:var(--text-muted); }
  .legacy-links { grid-template-columns:minmax(0,1fr); }
  .legacy-links .compat-heading { grid-column:1; }
  .path-input-row { flex-wrap:wrap; }
  .path-input-row > * { flex:1 1 100%; }
  .patch-edit { flex-wrap:wrap; }
  .patch-edit > * { flex:1 1 140px; }
  .backup-policy { display:grid; grid-template-columns:minmax(0,1fr); }
}
@media (max-width:900px) {
  .tool-center-scroll { width:100%; }
  .tool-stage::before,.art-toggle,.art-caption { display:none; }
}
@media (max-width:1024px) {
  .workspace-bar { padding-inline:var(--space-4); }
  .tool-switcher { padding-inline:var(--space-3); }
}
@media (max-width:960px) {
  .build-chip { display:none; }
  .titlebar-runtime-sessions { margin-left:var(--space-2); }
  .titlebar-patch-session,.titlebar-companion-session { max-width:140px; overflow:hidden; text-overflow:ellipsis; }
  .titlebar-status { max-width:36vw; }
  .workspace-state { display:none; }
  .tool-page-heading { padding:var(--space-5) var(--space-6); }
  .workspace-scroll.tool-workspace { padding:var(--space-4); }
}
@media (max-height:620px) {
  .app-window { --titlebar-size:38px; }
  .workspace-bar { min-height:40px; flex-basis:40px; }
  .tool-switcher { min-height:38px; }
  .tool-switcher .ui-tab { min-height:36px; padding-block:var(--space-1); }
  .sidebar { padding-top:var(--space-5); padding-bottom:var(--space-3); }
  .sidebar-heading { padding-block:var(--space-2) var(--space-3); }
  .primary-nav { gap:var(--space-1); padding-top:var(--space-3); }
  .nav-item { min-height:46px; }
  .sidebar-mascot-say { display:none; }
  .sidebar-mascot { padding-block:var(--space-1); }
  .sidebar-mascot-img { height:48px; }
  .workspace-scroll.tool-workspace { padding-block:var(--space-3); }
  .tool-page-heading { margin-bottom:var(--space-3); padding-block:var(--space-4); }
}
@media (prefers-reduced-motion:reduce) {
  .toast-enter-active,.toast-leave-active { transition:none; }
}
</style>
