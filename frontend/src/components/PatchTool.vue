<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import {
  AutoDetect, SetExePath, GetStatus, PatchFile, BackupFile, RestoreFile,
  GetAppVersion, CheckUpdate, OpenReleasePage,
} from '../../wailsjs/go/main/App'
import {
  WindowFullscreen,
  WindowIsFullscreen,
  WindowMinimise,
  WindowToggleMaximise,
  WindowUnfullscreen,
  Quit,
} from '../../wailsjs/runtime/runtime'
import HomeJournal from './HomeJournal.vue'
import SaveBackupDrawer from './SaveBackupDrawer.vue'
import { language, translateText } from '../i18n'
import progressionArt from '../assets/gbfr/cutouts/progression-official-edge-safe.webp'
import sigilArt from '../assets/gbfr/cutouts/sigil-official-edge-safe.webp'
import sigilMemoryArt from '../assets/gbfr/cutouts/sigil-memory-official-edge-safe.webp'
import loadoutLiveArt from '../assets/gbfr/cutouts/loadout-live-official-edge-safe.webp'
import loadoutPresetsArt from '../assets/gbfr/cutouts/loadout-presets-official-edge-safe.webp'
import wrightstoneArt from '../assets/gbfr/cutouts/wrightstone-official-edge-safe.webp'
import wrightstoneMemoryArt from '../assets/gbfr/cutouts/wrightstone-memory-official-edge-safe.webp'
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
import loadoutPresetsSticker from '../assets/gbfr/stickers/loadout-presets.webp'
import wrightstoneSticker from '../assets/gbfr/stickers/wrightstone.webp'
import wrightstoneMemorySticker from '../assets/gbfr/stickers/wrightstone-memory.webp'
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

// These page-specific assets are produced by the approved portrait workflow.
// URL construction keeps this frontend slice independently buildable while the
// asset task lands, without silently substituting another page's character.
const ctCombatArt = new URL('../assets/gbfr/cutouts/ct-combat-official-edge-safe.webp', import.meta.url).href
const ctCharactersArt = new URL('../assets/gbfr/cutouts/ct-characters-official-edge-safe.webp', import.meta.url).href
const ctQuestArt = new URL('../assets/gbfr/cutouts/ct-quest-official-edge-safe.webp', import.meta.url).href
const ctCombatSticker = new URL('../assets/gbfr/stickers/ct-combat.webp', import.meta.url).href
const ctCharactersSticker = new URL('../assets/gbfr/stickers/ct-characters.webp', import.meta.url).href
const ctQuestSticker = new URL('../assets/gbfr/stickers/ct-quest.webp', import.meta.url).href

const componentLoaders = {
  progression: () => import('./ProgressionEditor.vue'),
  sigil: () => import('./SigilGenerator.vue'),
  sigilMemory: () => import('./SigilMemoryGenerator.vue'),
  loadout: () => import('./SigilLoadoutRestore.vue'),
  loadoutPresets: () => import('./LoadoutViewer.vue'),
  wrightstone: () => import('./WrightstoneGenerator.vue'),
  wrightstoneMemory: () => import('./WrightstoneMemoryGenerator.vue'),
  summon: () => import('./SummonEditor.vue'),
  overlimit: () => import('./OverLimit.vue'),
  runtime: () => import('./MiscTools.vue'),
  legacyRuntime: () => import('./MiscTools.vue'),
  chara: () => import('./CharaStats.vue'),
  save: () => import('./SaveEditor.vue'),
  monster: () => import('./MonsterEnhance.vue'),
  ctCombat: () => import('./CT084Features.vue'),
  ctCharacters: () => import('./CT084Features.vue'),
  ctQuest: () => import('./CT084Features.vue'),
  language: () => import('./LanguageSettings.vue'),
}
// 桌面本地应用无网络加载成本，改用静态直引：全部组件打进主包，
// 切页时立绘与内容同帧渲染，不再出现“先出图、内容后到”的等待感。
// componentLoaders / warmTool 仍保留（预热已打包模块，无副作用），便于将来若需分包回退。
import ProgressionEditor from './ProgressionEditor.vue'
import SigilGenerator from './SigilGenerator.vue'
import SigilMemoryGenerator from './SigilMemoryGenerator.vue'
import SigilLoadoutRestore from './SigilLoadoutRestore.vue'
import LoadoutViewer from './LoadoutViewer.vue'
import WrightstoneGenerator from './WrightstoneGenerator.vue'
import WrightstoneMemoryGenerator from './WrightstoneMemoryGenerator.vue'
import SummonEditor from './SummonEditor.vue'
import OverLimit from './OverLimit.vue'
import MiscTools from './MiscTools.vue'
import CharaStats from './CharaStats.vue'
import SaveEditor from './SaveEditor.vue'
import MonsterEnhance from './MonsterEnhance.vue'
import CT084Features from './CT084Features.vue'
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
const loadoutEditing = ref(false)
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
    group: 'save', title: '物品与武器（存档修改）', eyebrow: '离线养成', status: '已适配 2.0.2', tone: 'stable',
    description: '统一处理物品、素材、武器等级与养成资源，适合大批量、可回滚的存档修改。',
    usage: ['完全退出游戏', '选择存档并确认空位', '写入后使用自动备份验证'],
    caution: '不要在游戏运行时编辑同一份存档。',
    speaker: '卡莉奥斯特罗', note: '先留好备份，再把素材和武器整理得漂漂亮亮——这才像完美的炼金术嘛。',
  },
  sigil: {
    group: 'save', title: '因子修改（存档修改）', eyebrow: '离线存档', status: '稳定', tone: 'stable',
    description: '生成、批量管理和删除存档内因子，适合一次性整理较多因子。',
    usage: ['退出游戏并加载存档', '配置因子与词条', '先检查合法性再写入'],
    caution: '不合法组合会提醒，但不会替你改变选择。',
    speaker: '娜露梅亚', note: '先检验组合，再写入存档。稳稳完成每一步，理想的因子就不会跑掉。',
  },
  sigilMemory: {
    group: 'memory', title: '因子即时编辑', eyebrow: '游戏内养成', status: '实时', tone: 'live',
    description: '直接修改游戏中当前选中的因子，适合少量精确调整和反复试配。',
    usage: ['启动游戏并启用读取', '在游戏中选中目标因子', '刷新、核对后写入'],
    caution: '重新进档或因子列表刷新后，请重新选择目标。',
    speaker: '萝赛塔', note: '游戏重新载入后，记得再选一次目标。旧的指针可不会一直等你哦。',
  },
  loadout: {
    group: 'memory', title: '因子配装·实时录制/复刻', eyebrow: '游戏内因子', status: '实时', tone: 'live',
    description: '记录角色当前的 12 个因子并导出分享，也可把配装文件逐项复刻到备用因子。（改的是游戏内因子；写存档配装预设请用「配装预设」。）',
    usage: ['启动游戏并按角色筛选因子', '从第一项开始记录或复刻', '逐项向下移动，不要快速滚动'],
    caution: '复刻会改写当前选中的备用因子；不要使用已经装备或需要保留的因子。',
    speaker: '芙劳', note: '把十二个因子的顺序先理清，再一步一步复刻。速度不必太快，准确才最重要。',
  },
  loadoutPresets: {
    group: 'save', title: '配装预设（查看与写入）', eyebrow: '离线存档', status: '稳定', tone: 'stable',
    description: '查看游戏配装界面保存的预设（武器/12 因子/4 技能/专精），也可把自定义配装写入指定槽位。',
    usage: ['完全退出游戏', '选择存档位或浏览存档文件', '查看，或切到「编辑写入」自定义配装'],
    caution: '',
    speaker: '芙劳', note: '想改配装也可以啦——先备份，写完我再帮你核对一遍，不会弄丢的。',
  },
  wrightstone: {
    group: 'save', title: '祝福修改（存档修改）', eyebrow: '离线存档', status: '稳定', tone: 'stable',
    description: '集中生成祝福与三条词条，使用与因子批量修改一致的存档工作流。',
    usage: ['退出游戏并加载存档', '选择祝福和三条词条', '校验队列并应用'],
    caution: '等级上限与组合合法性会在写入前提示。',
    speaker: '菲莉', note: '三条词条都确认好再应用，幽灵朋友们也会替你看着。',
  },
  wrightstoneMemory: {
    group: 'memory', title: '祝福石即时编辑', eyebrow: '游戏内祝福石', status: '实时', tone: 'live',
    description: '捕获游戏内当前选中的祝福石记录，并以一次事务核对、写入三条词条。',
    usage: ['启动游戏并启用读取', '在游戏内祝福石列表选中目标记录', '核对三槽变更后一次性写入'],
    caution: '每次写入后旧记录都会失效；继续操作前必须在游戏内重新选择记录。',
    speaker: '菲莉', note: '三条词条要一项一项核对。写完以后重新选中目标，我和幽灵朋友都会继续看着。',
  },
  summon: {
    group: 'memory', title: '召唤石修改', eyebrow: '游戏内修改', status: '实时保存', tone: 'live',
    description: '读取召唤石背包并修改因子、副参数和等级，写入时调用游戏保存流程。',
    usage: ['打开游戏内召唤石背包', '连接并选择一颗召唤石', '核对稀有度与合法性后写入'],
    caution: '当前不支持安全更换召唤石种类。',
    speaker: '露莉亚', note: '先在背包里选中目标召唤石，再核对稀有度和等级，我们一起慢慢来。',
  },
  overlimit: {
    group: 'memory', title: '角色上限突破', eyebrow: '游戏内修改', status: '流程型', tone: 'live',
    description: '读取角色突破界面的四个能力槽，按游戏原流程保存结果。',
    usage: ['先完成一次 3 级突破', '停在结果界面后刷新', '修改四项并按说明保存'],
    caution: '必须按页面步骤完成，不能跳过游戏内确认流程。',
    speaker: '希耶提', note: '四个能力槽一个都别漏。真正的剑王，可不会跳过确认步骤。',
  },
  runtime: {
    group: 'memory', title: '游戏内实时修改', eyebrow: '金币、素材与掉落', status: '需连接游戏', tone: 'live',
    description: '集中管理货币、药水、素材消耗和任务掉落等运行时功能。',
    usage: ['先启动并进入游戏存档', '连接游戏进程', '按资源或任务分类切换功能'],
    caution: '重启游戏后运行时设置会失效，需要重新连接。',
    speaker: '碧', note: '进游戏、连进程、再修改！重启以后可得重新连接，别忘啦！',
  },
  ctCombat: {
    group: 'memory', title: '战斗规则补丁', eyebrow: 'CT 0.8.4 · 战斗', status: '仅离线/单机', tone: 'live',
    description: '集中管理闪避、格挡、Link、召唤限制与部位破坏等已验证的实时补丁。',
    usage: ['启动游戏并进入单机内容', '连接后选择需要的战斗规则', '离开页面或断开时恢复全部补丁'],
    caution: '这些功能只用于离线或单机游玩；不要带入联机房间。',
    speaker: '巴恩', note: '先确认只在单机里测试，再一项一项校准。离开页面时，我会把规则全部恢复。',
  },
  ctCharacters: {
    group: 'memory', title: '角色机制补丁', eyebrow: 'CT 0.8.4 · 角色', status: '仅离线/单机', tone: 'live',
    description: '按角色整理已验证的专属机制补丁，可搜索角色与功能名称并查看明确冲突。',
    usage: ['启动游戏并进入单机内容', '选择角色分组后启用机制', '冲突项先恢复当前功能再切换'],
    caution: '这些功能只用于离线或单机游玩；互斥机制不会相互覆盖。',
    speaker: '姬塔', note: '角色机制已经按名字分好组。看到冲突提示时，先恢复原来的那一项再继续。',
  },
  ctQuest: {
    group: 'memory', title: '任务与便利补丁', eyebrow: 'CT 0.8.4 · 任务', status: '仅离线/单机', tone: 'live',
    description: '管理任务倒计时、宝箱、结算、支线奖励与养成便利等已验证实时补丁。',
    usage: ['启动游戏并进入单机任务', '按任务或体验优化分组选择', '任务结束前按需恢复默认'],
    caution: '这些功能只用于离线或单机游玩；任务状态切换后请刷新回读。',
    speaker: '尤达哈拉', note: '任务路线先看清，宝箱和结算各归各位。用完恢复，下一趟才不会乱。',
  },
  chara: {
    group: 'save', title: '角色使用次数', eyebrow: '记录与统计', status: '离线存档', tone: 'stable',
    description: '查看所有角色的使用次数，可任意选择多个角色批量修改。',
    usage: ['完全退出游戏', '选择存档和目标角色', '填入次数后保存已选'],
    caution: '只修改勾选角色，保存前请检查选择数量。',
    speaker: '姬塔', note: '只会保存你勾选的角色。动手前再数一遍，团长的记录要清清楚楚。',
  },
  save: {
    group: 'save', title: '任务完成次数', eyebrow: '记录与统计', status: '离线存档', tone: 'stable',
    description: '搜索任务并批量修改完成次数，保留未选任务的原始数据。',
    usage: ['完全退出游戏', '搜索并勾选任务', '批量填入后保存已选'],
    caution: '每次保存都会创建时间戳备份并回读。',
    speaker: '拉卡姆', note: '任务记录就像航线图，先选准目标，再一次写入，别改错方向。',
  },
  compatibility: {
    group: 'tools', title: '版本适配', eyebrow: '版本检测与功能状态', status: 'DLC 2.0.2', tone: 'calibrate',
    description: '在一个位置查看工具版本、游戏文件和功能适配状态。',
    usage: ['检查工具更新', '确认游戏文件已识别', '查看适配状态'],
    caution: '',
    speaker: '罗兰', note: '先看工具版本、游戏文件和适配状态。修东西之前，总得弄清哪里不对。',
  },
  legacyRuntime: {
    group: 'memory', title: '实验性实时功能', eyebrow: '实验', status: '实验', tone: 'stable',
    description: '一批仍在打磨的运行时功能与诊断入口。',
    usage: ['先阅读每项说明', '连接后先扫描或刷新', '字节不匹配时停止'],
    caution: '',
    speaker: '泽塔', note: '地址或字节对不上就立刻停手。稳一点，准一点。',
  },
  monster: {
    group: 'memory', title: '怪物倍率与伤害记录', eyebrow: '实验', status: '实验', tone: 'stable',
    description: '怪物倍率、霸体和伤害记录相关功能。',
    usage: ['仅在主机端测试', '先刷新并检查状态', '告知队友后再启用'],
    caution: '',
    speaker: '伊德', note: '先确认主机端和倍率，再动手。力量失控的话，记录也会失去意义。',
  },
  patch: {
    group: 'tools', title: '游戏文件维护', eyebrow: 'EXE 备份与恢复', status: '可用', tone: 'calibrate',
    description: '识别游戏 EXE、创建原始文件备份并一键恢复。',
    usage: ['定位游戏 EXE', '先创建原始备份', '需要时一键恢复'],
    caution: '',
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

// 顶层按「存档修改 / 内存注入 / 工具」三分（内存注入=运行时改进程内存，退游戏即失效；
// 存档修改=离线改存档文件，可批量可回滚）。items 顺序即台前优先级。
const navigation = computed(() => [
  { id: 'save', mark: '档', label: language.value === 'zh' ? '存档修改（离线）' : 'Save Editing', caption: language.value === 'zh' ? '退出游戏后改存档文件' : 'Edit the save file offline', items: ['progression', 'sigil', 'wrightstone', 'loadoutPresets', 'chara', 'save'] },
  { id: 'memory', mark: '注', label: language.value === 'zh' ? '内存注入（实时）' : 'Live Injection', caption: language.value === 'zh' ? '连接游戏改进程内存' : 'Edit process memory in-game', items: ['runtime', 'ctCombat', 'ctCharacters', 'ctQuest', 'sigilMemory', 'wrightstoneMemory', 'loadout', 'summon', 'overlimit', 'legacyRuntime', 'monster'] },
  { id: 'tools', mark: '具', label: language.value === 'zh' ? '工具与设置' : 'Tools & Settings', caption: language.value === 'zh' ? '版本诊断 · EXE维护 · 语言' : 'Diagnostics, EXE, language', items: ['compatibility', 'patch', 'language'] },
])

const currentMeta = computed(() => toolMeta[activeTab.value] || toolMeta.home)
const isLoadoutWorkspace = computed(() => activeTab.value === 'loadoutPresets' && loadoutEditing.value)
const functionArt = {
  progression: progressionArt,
  sigil: sigilArt,
  sigilMemory: sigilMemoryArt,
  loadout: loadoutLiveArt,
  loadoutPresets: loadoutPresetsArt,
  wrightstone: wrightstoneArt,
  wrightstoneMemory: wrightstoneMemoryArt,
  summon: summonArt,
  overlimit: overlimitArt,
  runtime: runtimeArt,
  ctCombat: ctCombatArt,
  ctCharacters: ctCharactersArt,
  ctQuest: ctQuestArt,
  chara: charaArt,
  save: saveArt,
  compatibility: compatibilityArt,
  legacyRuntime: legacyRuntimeArt,
  monster: monsterArt,
  patch: patchArt,
  language: languageArt,
}
const currentArt = computed(() => functionArt[activeTab.value] || '')
const functionStickers = {
  progression: progressionSticker,
  sigil: sigilSticker,
  sigilMemory: sigilMemorySticker,
  loadout: loadoutSticker,
  loadoutPresets: loadoutPresetsSticker,
  wrightstone: wrightstoneSticker,
  wrightstoneMemory: wrightstoneMemorySticker,
  summon: summonSticker,
  overlimit: overlimitSticker,
  runtime: runtimeSticker,
  ctCombat: ctCombatSticker,
  ctCharacters: ctCharactersSticker,
  ctQuest: ctQuestSticker,
  chara: charaSticker,
  save: saveSticker,
  compatibility: compatibilitySticker,
  legacyRuntime: legacyRuntimeSticker,
  monster: monsterSticker,
  patch: patchSticker,
  language: languageSticker,
}
const currentSticker = computed(() => functionStickers[activeTab.value] || '')
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
  if (!group.items.includes(activeTab.value)) {
    loadoutEditing.value = false
    activeTab.value = group.items[0]
  }
  if (group.id === 'tools') ensureGameDetection()
}

function selectTool(id) {
  warmTool(id)
  if (id !== 'loadoutPresets') loadoutEditing.value = false
  activeTab.value = id
  if (toolMeta[id]?.group === 'tools') ensureGameDetection()
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

async function toggleFullscreen() {
  try {
    if (await WindowIsFullscreen()) WindowUnfullscreen()
    else WindowFullscreen()
  } catch (error) {
    showStatus(`切换全屏失败：${String(error)}`, 'error')
  }
}
</script>

<template>
  <div class="app-window">
    <header class="titlebar" style="--wails-draggable:drag" @dblclick.self="WindowToggleMaximise">
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
        <button class="win-btn" @click="WindowToggleMaximise" title="最大化或还原" aria-label="最大化或还原"><span class="maximise-box"></span></button>
        <button class="win-btn" @click="toggleFullscreen" title="一键全屏" aria-label="一键全屏"><span class="fullscreen-corners"></span></button>
        <button class="win-btn close" @click="Quit" title="关闭" aria-label="关闭"><span class="close-lines"></span></button>
      </div>
    </header>

    <div class="app-body" :class="{ 'home-mode': activeTab === 'home', 'sidebar-collapsed': sidebarCollapsed, 'loadout-workspace': isLoadoutWorkspace, 'art-visible': activeTab !== 'home' && !isLoadoutWorkspace && !artCollapsed }" style="--wails-draggable:no-drag">
      <aside class="sidebar">
        <button class="sidebar-collapse" :title="sidebarCollapsed ? '展开目录' : '收起目录'" :aria-label="sidebarCollapsed ? '展开目录' : '收起目录'" @click="toggleSidebar">{{ sidebarCollapsed ? '›' : '‹' }}</button>
        <button class="sidebar-home-compact" type="button" title="返回功能首页" aria-label="返回功能首页" @click="selectTool('home')">
          <span aria-hidden="true">⌂</span>
        </button>
        <button class="sidebar-heading" type="button" title="返回功能首页" @click="selectTool('home')">
          <span class="sidebar-kicker">GBFR PE PATCH TOOL</span>
          <strong>GBFR 存档修改工具</strong>
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
            @pointerenter="warmGroup(group)"
            @focus="warmGroup(group)"
            @click="selectGroup(group)"
          >
            <span class="nav-mark">{{ group.mark }}</span>
            <span class="nav-copy"><strong>{{ group.label }}</strong><small>{{ group.caption }}</small></span>
            <span class="nav-arrow">›</span>
          </button>
        </nav>
        <!-- Q版角色是左栏常驻元素；紧凑尺寸只收起气泡，不删除图片。 -->
        <div class="sidebar-mascot" v-if="activeTab !== 'home' && currentMeta.speaker" :title="`${currentMeta.speaker}：${currentMeta.note}`">
          <img class="sidebar-mascot-img" :src="currentSticker" :alt="currentMeta.speaker" loading="eager" decoding="async">
          <div class="sidebar-mascot-say"><b>{{ currentMeta.speaker }}</b><p>{{ currentMeta.note }}</p></div>
        </div>
        <div class="sidebar-foot">
          <div class="target-row"><span class="target-dot"></span><div><strong>当前游戏版本</strong><small>Relink DLC 2.0.2</small></div></div>
          <a href="https://github.com/BitterG/GBFR-PE-Patch-Tool" target="_blank">项目仓库 ↗</a>
        </div>
      </aside>

      <section class="workspace">
        <div v-if="activeTab !== 'home' && !isLoadoutWorkspace" class="workspace-bar">
            <div class="breadcrumb"><span>{{ activeGroup.label }}</span><b>/</b><strong>{{ currentMeta.title }}</strong></div>
            <div class="workspace-actions">
              <div class="workspace-state"><span :class="['state-dot', currentMeta.tone]"></span>{{ currentMeta.status }}</div>
              <SaveBackupDrawer @status="showStatus" />
            </div>
        </div>

        <nav v-if="activeTab !== 'home' && !isLoadoutWorkspace && activeGroup.items.length > 1" class="tool-switcher ui-tabs" :data-group="activeGroup.id" aria-label="同类功能切换">
            <button
              v-for="id in activeGroup.items"
              :key="id"
              class="ui-tab"
              :class="{ active: activeTab === id, waiting: toolMeta[id].tone === 'waiting' }"
              :aria-current="activeTab === id ? 'page' : undefined"
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

        <div class="workspace-scroll" :class="{ 'tool-workspace': activeTab !== 'home', 'loadout-workspace-scroll': isLoadoutWorkspace }">
          <div class="workspace-scene">
          <HomeJournal v-if="activeTab === 'home'" key="home" :version="updateInfo.currentVersion" @warm="warmTool" @open="selectTool" />

          <section v-else :key="activeTab" class="tool-stage" :class="{ 'art-collapsed': artCollapsed, 'loadout-dedicated': isLoadoutWorkspace }" :data-tool="activeTab">
            <section class="tool-center-scroll">
              <header v-if="!isLoadoutWorkspace" class="tool-page-heading">
                <div class="eyebrow">{{ currentMeta.eyebrow }}</div>
                <h1>{{ currentMeta.title }}</h1>
                <p>{{ currentMeta.description }}</p>
              </header>

              <main class="tool-panel" :data-tool="activeTab">
            <ProgressionEditor v-if="activeTab === 'progression'" @status="showStatus" />
            <SigilGenerator v-else-if="activeTab === 'sigil'" @status="showStatus" />
            <SigilMemoryGenerator v-else-if="activeTab === 'sigilMemory'" @status="showStatus" />
            <SigilLoadoutRestore v-else-if="activeTab === 'loadout'" @status="showStatus" />
            <LoadoutViewer v-else-if="activeTab === 'loadoutPresets'" @status="showStatus" @editing-change="loadoutEditing = $event" />
            <WrightstoneGenerator v-else-if="activeTab === 'wrightstone'" @status="showStatus" />
            <WrightstoneMemoryGenerator v-else-if="activeTab === 'wrightstoneMemory'" @status="showStatus" />
            <SummonEditor v-else-if="activeTab === 'summon'" @status="showStatus" />
            <OverLimit v-else-if="activeTab === 'overlimit'" @status="showStatus" />
            <MiscTools v-else-if="activeTab === 'runtime'" mode="stable" @status="showStatus" />
            <CT084Features v-else-if="activeTab === 'ctCombat'" mode="combat" @status="showStatus" />
            <CT084Features v-else-if="activeTab === 'ctCharacters'" mode="characters" @status="showStatus" />
            <CT084Features v-else-if="activeTab === 'ctQuest'" mode="quest" @status="showStatus" />
            <CharaStats v-else-if="activeTab === 'chara'" @status="showStatus" />
            <SaveEditor v-else-if="activeTab === 'save'" @status="showStatus" />
            <MiscTools v-else-if="activeTab === 'legacyRuntime'" mode="compatibility" @status="showStatus" />
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
                  <span class="file-meta">{{ state.fileSize ? `${(state.fileSize / 1024 / 1024).toFixed(1)} MB` : '可在旧版文件补丁页手动选择' }}</span>
                </article>
                <article class="calibration-card ui-card ui-stat">
                  <div class="card-kicker">校准目标</div>
                  <strong>DLC 2.0.2</strong>
                  <p>实时货币与素材指令已按当前版本特征校验。</p>
                  <span class="file-meta">未知字节会拒绝写入</span>
                </article>
              </section>

              <section class="compat-section ui-card ui-panel">
                <div class="compat-heading"><div><span>功能状态</span><h2>当前版本 DLC 2.0.2</h2></div><p>核心功能已按当前版本校验。</p></div>
                <div class="matrix">
                  <div class="matrix-row head"><span>范围</span><span>状态</span><span>说明</span></div>
                  <div class="matrix-row"><span>物品、武器、因子、祝福、配装</span><b class="ok">存档修改</b><span>离线存档路径，自动备份与回读</span></div>
                  <div class="matrix-row"><span>货币、药水、素材消耗、巴武掉落</span><b class="ok">内存注入</b><span>运行时连接，地址或字节不符即停止</span></div>
                  <div class="matrix-row"><span>召唤石、上限突破、即时因子</span><b class="flow">内存注入</b><span>需停留在指定游戏界面后操作</span></div>
                </div>
              </section>

              <section class="compat-section legacy-links ui-card ui-panel">
                <div class="compat-heading"><div><span>实验性功能</span><h2>仍在打磨</h2></div></div>
                <button class="ui-card" @click="selectTool('legacyRuntime')"><strong>实验性实时功能</strong><small>倒计时、无限挑战、称号、皮肤符文等</small><span>查看 ›</span></button>
                <button class="ui-card" @click="selectTool('monster')"><strong>怪物倍率与伤害记录</strong><small>倍率、霸体、OD 与团队伤害记录</small><span>查看 ›</span></button>
              </section>
            </div>

            <div v-else-if="activeTab === 'patch'" class="legacy-patch ui-page-stack">
              <section class="patch-file-row ui-card path-card ui-panel is-compact">
                <label class="ui-field-label" for="game-exe-path">{{ isDetecting ? '正在扫描 Steam 安装路径…' : isLoaded ? '已定位游戏文件' : '游戏 EXE 路径' }}</label>
                <div class="path-input-row ui-control-group is-responsive"><input id="game-exe-path" v-model="manualPath" class="ui-input" placeholder="粘贴 granblue_fantasy_relink.exe 完整路径" @keyup.enter="applyManualPath"><button class="action ui-btn is-primary" @click="applyManualPath" :disabled="!manualPath.trim()">识别文件</button></div>
                <div v-if="state.exePath" class="detected-file"><span :title="state.exePath">{{ state.exePath }}</span><b>{{ (state.fileSize / 1024 / 1024).toFixed(1) }} MB</b></div>
              </section>
              <section v-if="isLoaded" class="patch-grid ui-card-grid">
                <article v-for="patch in state.patches" :key="patch.id" class="patch-card ui-card ui-panel is-compact">
                  <header><div><strong>{{ patch.name }}</strong><small>二进制补丁</small></div><span :class="['patch-state', patch.state]">{{ patch.state === 'original' ? '原始' : patch.state === 'patched' ? '已补丁' : '未知' }}</span></header>
                  <p v-if="patch.state === 'patched'">当前值 {{ patch.currentValue }} · 0x{{ patch.currentValue.toString(16).toUpperCase() }}</p>
                  <div class="patch-edit ui-control-group"><input v-model="patchValues[patch.id]" class="ui-input" type="number" min="0" :aria-label="`${patch.name}数值`" placeholder="输入数值"><button class="action ui-btn" @click="applyPatch(patch.id)" :disabled="patchingID === patch.id || patch.state === 'unknown'">{{ patchingID === patch.id ? '写入中…' : '应用' }}</button></div>
                </article>
              </section>
              <section class="backup-card ui-card ui-panel is-compact"><div><strong>EXE 备份与恢复</strong><span>{{ state.backupExists ? `已有 ${(state.backupSize / 1024 / 1024).toFixed(1)} MB 备份` : '尚未创建备份' }}</span></div><div class="backup-policy ui-seg" role="group" aria-label="备份策略"><button type="button" class="ui-seg-btn" :class="{ 'is-on': !forceBackup }" :aria-pressed="!forceBackup" @click="forceBackup=false"><b>保留现有备份</b><small>推荐</small></button><button type="button" class="ui-seg-btn" :class="{ 'is-on': forceBackup }" :aria-pressed="forceBackup" @click="forceBackup=true"><b>重新创建原始备份</b><small>会替换旧备份</small></button></div><div class="patch-actions ui-actions"><button class="action ui-btn" @click="backup">创建备份</button><button class="action ui-btn is-danger" @click="restore" :disabled="!state.backupExists">恢复备份</button></div></section>
            </div>
              </main>
            </section>

            <button v-if="!isLoadoutWorkspace" class="art-toggle" :class="{ 'is-collapsed': artCollapsed }" :title="artCollapsed ? '展开立绘' : '收起立绘 · 拓宽操作区'" :aria-label="artCollapsed ? '展开立绘' : '收起立绘'" @click="toggleArt">{{ artCollapsed ? '‹' : '›' }}</button>
            <aside v-if="!isLoadoutWorkspace" class="art-rail" aria-hidden="true">
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
  --window-controls-width:168px;
  position:relative;
  z-index:20;
  height:var(--titlebar-size);
  display:flex;
  align-items:center;
  padding-right:var(--window-controls-width);
  border-bottom:1px solid var(--border-default);
  background:color-mix(in srgb,var(--surface-card) 82%,transparent);
  backdrop-filter:blur(12px) saturate(.9);
  box-shadow:0 1px 0 rgba(255,255,255,.5) inset;
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
  border:1px solid var(--accent-border);
  border-radius:var(--radius-sm);
  color:var(--accent);
  font-size:var(--fs-sm);
}
.titlebar-title {
  min-width:0;
  overflow:hidden;
  color:var(--text-primary);
  font-size:var(--fs-sm);
  font-weight:var(--fw-bold);
  letter-spacing:.04em;
  text-overflow:ellipsis;
  white-space:nowrap;
}
.build-chip {
  flex:0 0 auto;
  padding:2px var(--space-2);
  border:1px solid var(--border-default);
  border-radius:var(--radius-pill);
  color:var(--text-secondary);
  background:var(--surface-card-pop);
  font-size:var(--fs-xs);
}
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
  background:var(--surface-card-pop);
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
  color:var(--text-secondary);
  background:transparent;
  cursor:pointer;
}
.win-btn:hover { color:var(--text-primary); background:var(--state-hover); }
.win-btn.close:hover { color:var(--text-on-accent); background:var(--danger-ink); }
.minimize-line { width:12px; height:1px; background:currentColor; }
.maximise-box {
  width:12px;
  height:10px;
  border:1px solid currentColor;
  border-radius:1px;
}
.fullscreen-corners {
  position:relative;
  width:14px;
  height:14px;
  border:1px solid transparent;
  background:
    linear-gradient(currentColor,currentColor) left top / 5px 1px no-repeat,
    linear-gradient(currentColor,currentColor) left top / 1px 5px no-repeat,
    linear-gradient(currentColor,currentColor) right top / 5px 1px no-repeat,
    linear-gradient(currentColor,currentColor) right top / 1px 5px no-repeat,
    linear-gradient(currentColor,currentColor) left bottom / 5px 1px no-repeat,
    linear-gradient(currentColor,currentColor) left bottom / 1px 5px no-repeat,
    linear-gradient(currentColor,currentColor) right bottom / 5px 1px no-repeat,
    linear-gradient(currentColor,currentColor) right bottom / 1px 5px no-repeat;
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
  border-right:1px solid var(--border-default);
  background:
    linear-gradient(180deg,rgba(255,251,236,.88),rgba(228,207,164,.84)),
    url('../assets/gbfr/parchment-ui-v2.webp') left center / cover;
  box-shadow:6px 0 20px rgba(72,50,22,.06);
}
.sidebar-collapse {
  position:absolute;
  z-index:2;
  top:var(--space-2);
  right:var(--space-2);
  width:30px;
  height:30px;
  display:grid;
  place-items:center;
  border:0;
  border-radius:var(--radius-sm);
  color:var(--text-muted);
  background:transparent;
  font-size:var(--fs-lg);
  cursor:pointer;
}
.sidebar-collapse:hover { color:var(--text-primary); background:var(--state-hover); }
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
  grid-template-columns:46px minmax(0,1fr);
  align-items:end;
  gap:var(--space-2);
  margin-top:auto;
  padding:var(--space-3) var(--space-2) var(--space-4);
}
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
.sidebar-collapsed .sidebar { padding:var(--space-7) var(--space-3) var(--space-4); }
.sidebar-collapsed .sidebar-heading,
.sidebar-collapsed .nav-copy,
.sidebar-collapsed .nav-arrow,
.sidebar-collapsed .sidebar-foot { display:none; }
.sidebar-collapsed .sidebar-home-compact { display:grid; }
.sidebar-collapsed .primary-nav { align-items:center; padding-top:var(--space-3); }
.sidebar-collapsed .nav-item {
  width:48px;
  min-height:48px;
  grid-template-columns:1fr;
  place-items:center;
  padding:var(--space-2);
}
.sidebar-collapsed .nav-mark { width:32px; height:32px; }
.sidebar-collapsed .sidebar-mascot {
  width:48px;
  grid-template-columns:48px;
  place-items:center;
  padding:var(--space-2) 0;
}
.sidebar-collapsed .sidebar-mascot-img { width:48px; height:54px; }
.sidebar-collapsed .sidebar-mascot-say { display:none; }

.workspace {
  min-width:0;
  min-height:0;
  display:flex;
  flex-direction:column;
  overflow:hidden;
  background:
    linear-gradient(105deg,rgba(255,251,238,.55),rgba(239,220,180,.35)),
    url('../assets/gbfr/journal-scene-4k.webp') center / cover fixed;
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
  background:color-mix(in srgb,var(--surface-card) 92%,transparent);
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

.tool-switcher {
  min-height:46px;
  flex:0 0 46px;
  align-items:stretch;
  gap:var(--space-1);
  padding:0 var(--content-gutter);
  background:var(--surface-card);
  scrollbar-width:thin;
}
.tool-switcher .ui-tab {
  min-height:46px;
  display:inline-flex;
  align-items:center;
  gap:var(--space-2);
  padding-inline:var(--space-4);
  font-size:var(--fs-sm);
}
.tool-switcher .ui-tab.active {
  border-bottom-color:var(--selected-bar);
  color:var(--accent-hover);
  background:var(--state-hover);
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
.switcher-dot { width:6px; height:6px; border-radius:50%; background:var(--danger); }

.workspace-scroll {
  min-width:0;
  min-height:0;
  flex:1;
  overflow:auto;
  overscroll-behavior:contain;
  scrollbar-gutter:stable;
}
.workspace-scroll.tool-workspace { padding:var(--content-gutter); }
.home-mode .workspace-scroll { padding:0; }
.workspace-scene { min-width:0; min-height:100%; }

.tool-stage {
  --art-scale:184%;
  --art-x:-8%;
  --art-y:-22%;
  position:relative;
  isolation:isolate;
  min-width:0;
  min-height:100%;
  display:grid;
  grid-template-columns:minmax(0,62fr) minmax(260px,38fr);
  align-items:stretch;
  gap:clamp(4px,1vw,16px);
  overflow:clip;
}
.tool-stage.art-collapsed { grid-template-columns:minmax(0,1fr) 0; gap:0; }
.tool-stage.loadout-dedicated { grid-template-columns:minmax(0,1fr); gap:0; }
.tool-center-scroll {
  position:relative;
  z-index:2;
  min-width:0;
  min-height:0;
  padding-bottom:var(--space-8);
  container:tool-center / inline-size;
}
.tool-page-heading,.tool-panel {
  width:100%;
  max-width:none;
  margin-inline:0;
}
.tool-page-heading {
  margin-bottom:var(--space-5);
  padding:var(--space-6) var(--space-7);
  border:1px solid var(--border-default);
  border-radius:var(--radius-lg);
  background:var(--surface-card);
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
.tool-panel[data-tool="legacyRuntime"] :deep(.root > .section > .header),
.tool-panel[data-tool="chara"] :deep(.root > .section > .header),
.tool-panel[data-tool="overlimit"] :deep(.root > .section > .header),
.tool-panel[data-tool="monster"] :deep(.root > .section > .header),
.tool-panel[data-tool="summon"] :deep(.root > .section > .header) { display:none; }
.tool-panel[data-tool="progression"] :deep(.save-title > div:first-child) { display:none; }
.tool-panel[data-tool="progression"] :deep(.save-title) { min-height:0; justify-content:flex-end; }

.art-rail {
  position:sticky;
  z-index:1;
  top:0;
  min-width:0;
  height:clamp(420px,calc(100dvh - 166px),1400px);
  min-height:420px;
  overflow:visible;
  border:0;
  border-radius:0;
  background:transparent;
  box-shadow:none;
  pointer-events:none;
}
.art-rail::before {
  content:"";
  position:absolute;
  inset:5% -4% 2% -18%;
  z-index:0;
  background:radial-gradient(ellipse at 68% 46%,rgba(255,250,229,.54),rgba(219,191,139,.14) 54%,transparent 74%);
  filter:blur(3px);
}
.art-rail .function-character {
  position:absolute;
  z-index:1;
  inset:0 0 0 -34%;
  margin:0;
  overflow:visible;
}
.art-rail .function-character img {
  position:absolute;
  right:var(--art-x);
  bottom:var(--art-y);
  width:var(--art-scale);
  height:auto;
  max-width:none;
  max-height:none;
  object-position:right bottom;
  transform-origin:right bottom;
  -webkit-mask-image:linear-gradient(90deg,transparent 0%,rgba(0,0,0,.58) 24%,#000 43%);
  mask-image:linear-gradient(90deg,transparent 0%,rgba(0,0,0,.58) 24%,#000 43%);
}
.art-rail .character-blend {
  z-index:0;
  opacity:.2;
  filter:blur(9px) saturate(.82);
  transform:scale(1.025);
}
.art-rail .character-main {
  z-index:1;
  filter:drop-shadow(0 8px 8px rgba(72,50,22,.12));
}
.tool-stage[data-tool="progression"] { --art-scale:184%; --art-x:-8%; --art-y:-22%; }
.tool-stage[data-tool="sigil"] { --art-scale:184%; --art-x:-9%; --art-y:-22%; }
.tool-stage[data-tool="sigilMemory"] { --art-scale:182%; --art-x:-8%; --art-y:-21%; }
.tool-stage[data-tool="loadout"] { --art-scale:184%; --art-x:-9%; --art-y:-22%; }
.tool-stage[data-tool="loadoutPresets"] { --art-scale:185%; --art-x:-8%; --art-y:-22%; }
.tool-stage[data-tool="wrightstone"],
.tool-stage[data-tool="wrightstoneMemory"] { --art-scale:184%; --art-x:-8%; --art-y:-22%; }
.tool-stage[data-tool="summon"] { --art-scale:188%; --art-x:-8%; --art-y:-24%; }
.tool-stage[data-tool="overlimit"] { --art-scale:184%; --art-x:-8%; --art-y:-22%; }
.tool-stage[data-tool="runtime"] { --art-scale:208%; --art-x:-22%; --art-y:-31%; }
.tool-stage[data-tool="ctCombat"] { --art-scale:190%; --art-x:-10%; --art-y:-23%; }
.tool-stage[data-tool="ctCharacters"] { --art-scale:188%; --art-x:-9%; --art-y:-22%; }
.tool-stage[data-tool="ctQuest"] { --art-scale:190%; --art-x:-10%; --art-y:-23%; }
.tool-stage[data-tool="chara"],
.tool-stage[data-tool="save"] { --art-scale:184%; --art-x:-8%; --art-y:-22%; }
.tool-stage[data-tool="compatibility"] { --art-scale:184%; --art-x:-8%; --art-y:-22%; }
.tool-stage[data-tool="legacyRuntime"] { --art-scale:194%; --art-x:-10%; --art-y:-18%; }
.tool-stage[data-tool="monster"] { --art-scale:180%; --art-x:-2%; --art-y:-22%; }
.tool-stage[data-tool="patch"] { --art-scale:184%; --art-x:-8%; --art-y:-22%; }
.tool-stage[data-tool="language"] { --art-scale:192%; --art-x:-14%; --art-y:-23%; }
.art-caption {
  position:absolute;
  z-index:2;
  right:var(--space-3);
  bottom:var(--space-3);
  left:auto;
  padding:var(--space-2) var(--space-3);
  border:0;
  border-right:3px solid color-mix(in srgb,var(--accent) 68%,transparent);
  border-radius:0;
  background:color-mix(in srgb,var(--surface-card-pop) 62%,transparent);
  box-shadow:none;
  backdrop-filter:blur(4px);
  text-align:right;
}
.art-caption span,.art-caption small { display:block; }
.art-caption span { color:var(--text-primary); font-size:var(--fs-sm); font-weight:var(--fw-bold); }
.art-caption small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.art-toggle {
  position:absolute;
  z-index:4;
  top:var(--space-4);
  right:38%;
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
.tool-stage.art-collapsed .art-rail { visibility:hidden; opacity:0; }
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
  grid-template-columns:minmax(160px,1.1fr) 96px minmax(180px,1.4fr);
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
.matrix-row b { justify-self:start; padding:2px var(--space-2); border-radius:var(--radius-pill); font-size:var(--fs-xs); }
.matrix-row b.ok { color:var(--success-ink); background:var(--success-bg); }
.matrix-row b.flow { color:var(--info-ink); background:var(--info-bg); }
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
.patch-grid { --ui-grid-min:280px; }
.patch-card header {
  display:flex;
  align-items:flex-start;
  justify-content:space-between;
  gap:var(--space-3);
}
.patch-card header strong,.patch-card header small { display:block; }
.patch-card header strong { color:var(--text-primary); font-size:var(--fs-md); }
.patch-card header small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.patch-card p { margin:0; color:var(--text-secondary); font-size:var(--fs-sm); }
.patch-state {
  padding:2px var(--space-2);
  border-radius:var(--radius-pill);
  color:var(--text-secondary);
  background:var(--surface-field);
  font-size:var(--fs-xs);
  white-space:nowrap;
}
.patch-state.original { color:var(--success-ink); background:var(--success-bg); }
.patch-state.patched { color:var(--info-ink); background:var(--info-bg); }
.patch-state.unknown { color:var(--danger-ink); background:var(--danger-bg); }
.patch-edit .ui-btn { flex:0 0 auto; }
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
@media (max-width:1439px) {
  .tool-switcher[data-group="memory"] {
    display:flex;
    min-height:46px;
    flex:0 0 46px;
    align-items:stretch;
    gap:var(--space-1);
    padding-block:0;
    overflow-x:auto;
    overflow-y:hidden;
  }
  .tool-switcher[data-group="memory"] .ui-tab {
    min-width:max-content;
    flex:0 0 auto;
    min-height:46px;
    justify-content:center;
    padding:0 var(--space-3);
    line-height:1.25;
    white-space:nowrap;
  }
}
@media (min-width:1280px) and (max-width:1399px) {
  .app-body { grid-template-columns:70px minmax(0,1fr); }
  .sidebar { padding:var(--space-7) var(--space-3) var(--space-4); }
  .sidebar-heading,.nav-copy,.nav-arrow,.sidebar-foot,.sidebar-collapse { display:none; }
  .sidebar-home-compact { display:grid; }
  .primary-nav { align-items:center; padding-top:var(--space-3); }
  .nav-item {
    width:48px;
    min-height:48px;
    grid-template-columns:1fr;
    place-items:center;
    padding:var(--space-2);
  }
  .nav-mark { width:32px; height:32px; }
  .sidebar-mascot {
    width:48px;
    grid-template-columns:48px;
    place-items:center;
    padding:var(--space-2) 0;
  }
  .sidebar-mascot-img { width:48px; height:54px; }
  .sidebar-mascot-say { display:none; }
}
@media (max-width:900px) {
  .tool-stage { grid-template-columns:minmax(0,1fr); gap:0; }
  .art-rail,.art-toggle { display:none; }
}
@media (max-width:1024px) {
  .app-body { grid-template-columns:70px minmax(0,1fr); }
  .sidebar { padding:var(--space-7) var(--space-3) var(--space-4); }
  .sidebar-heading,.nav-copy,.nav-arrow,.sidebar-foot,.sidebar-collapse { display:none; }
  .sidebar-home-compact { display:grid; }
  .primary-nav { align-items:center; padding-top:var(--space-3); }
  .nav-item {
    width:48px;
    min-height:48px;
    grid-template-columns:1fr;
    place-items:center;
    padding:var(--space-2);
  }
  .nav-mark { width:32px; height:32px; }
  .sidebar-mascot {
    width:48px;
    grid-template-columns:48px;
    place-items:center;
    padding:var(--space-2) 0;
  }
  .sidebar-mascot-img { width:48px; height:54px; }
  .sidebar-mascot-say { display:none; }
  .workspace-bar { padding-inline:var(--space-4); }
  .tool-switcher { padding-inline:var(--space-3); }
}
@media (max-width:960px) {
  .build-chip { display:none; }
  .titlebar-status { max-width:36vw; }
  .workspace-state { display:none; }
  .tool-page-heading { padding:var(--space-5) var(--space-6); }
  .workspace-scroll.tool-workspace { padding:var(--space-4); }
}
@media (max-height:620px) {
  .app-window { --titlebar-size:38px; }
  .workspace-bar { min-height:40px; flex-basis:40px; }
  .tool-switcher { min-height:42px; flex-basis:42px; }
  .tool-switcher .ui-tab { min-height:42px; }
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
