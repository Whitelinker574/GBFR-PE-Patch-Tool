import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const badge = readFileSync(new URL('./components/BadgeUnlock.vue', import.meta.url), 'utf8')
const backendLanguage = readFileSync(new URL('./backendLanguage.js', import.meta.url), 'utf8')
const liveWrightstone = readFileSync(new URL('./components/WrightstoneMemoryGenerator.vue', import.meta.url), 'utf8')
const offlineWrightstone = readFileSync(new URL('./components/WrightstoneGenerator.vue', import.meta.url), 'utf8')
const uiTranslations = readFileSync(new URL('./i18n-ui.js', import.meta.url), 'utf8')

test('title records follow the application language without showing the opposite language underneath', () => {
  assert.match(badge, /import \{ language \} from '\.\.\/i18n\.js'/)
  assert.match(badge, /language\.value === 'en' \? badge\.nameEn : badge\.nameZhSimplified/)
  assert.doesNotMatch(badge, /nameMode/)
  assert.doesNotMatch(badge, /<small>\{\{\s*[^}]*badge\.name(?:En|Zh)/)
  assert.doesNotMatch(badge, /称号名称语言|中文名|<option value="en">English/)
  assert.match(badge, /const copy = computed\(\(\) => language\.value === 'en'/)
  assert.match(badge, /isolatedError\(err, 'Failed to load title records\.'/)
})

test('catalog loading fails closed when backend language synchronisation fails', () => {
  assert.match(backendLanguage, /throw new Error\(`/)
  assert.doesNotMatch(backendLanguage, /return selectedLanguage/)
})

test('both wrightstone pages wait for language sync and render dynamic copy from one language', () => {
  assert.match(liveWrightstone, /await backendLanguageReady[\s\S]*WrightstoneMemoryGetOptions\(\)/)
  assert.match(liveWrightstone, /function text\(zh, en\)/)
  assert.match(liveWrightstone, /isolatedError\(error, 'Failed to load the wrightstone catalog\.'/)
  assert.match(offlineWrightstone, /await backendLanguageReady[\s\S]*GetWrightstoneList\(\)/)
  assert.match(offlineWrightstone, /displayedLegalityMessage/)
  assert.match(offlineWrightstone, /Above natural reference/)
})

test('the title-record shell has exact English copy instead of mixed substring translation', () => {
  assert.match(uiTranslations, /'任务与称号记录': 'Quest & Title Records'/)
  assert.match(uiTranslations, /'修改任务完成次数，或搜索并维护称号解锁与已查看记录。': 'Edit quest completion counts, or search and maintain title unlock and viewed records\.'/)
  assert.match(uiTranslations, /'称号记录': 'Title Records'/)
})

test('the full home shell uses exact English copy instead of substring hybrids', () => {
  for (const text of [
    'GBFR 存档修改工具',
    '最大化或还原',
    '改存档：先',
    '完全退出游戏',
    '；游戏内实时改：先',
    '启动并进入游戏',
    '。同一份存档，两种方式别同时用。',
    '斗',
    '角',
    '任',
    '证',
    '存档修改',
    '退出游戏后离线改存档文件，可批量、可回滚',
    '配装预设',
    '查看与写入配装、因子加成模拟',
    '因子修改',
    '批量生成因子与合法性校验',
    '召唤石存档修改',
    '改',
    '配',
    '运',
    '存档与配装（离线）',
    '完全退出游戏后编辑；保存前自动备份，写入后回读',
    '游戏内即时编辑',
    '先启动并连接游戏，再修改当前选中的装备或会话资源',
    '配装采集与复刻',
    '检测不会默认开启；点击后可持续后台采集',
    '单机运行时工具',
    '按需主动开启；切页后保持运行，停用时安全恢复',
    '祝福石即时编辑',
    '配装录制与复刻',
    '引用真实库存实例的额外运行时槽位',
    '战斗规则补丁',
    '角色机制补丁',
    '内存监测',
    '角色配装检测',
    '公式采样',
    '游戏文件、诊断与设置',
  ]) {
    const escaped = text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const match = uiTranslations.match(new RegExp(`'${escaped}': '([^']+)'`))
    assert.ok(match, `missing shell translation: ${text}`)
    assert.doesNotMatch(match[1], /[\u3400-\u9fff]/u)
  }
})

test('tool metadata and image descriptions use complete English translations', () => {
  for (const text of [
    '物品与武器（存档修改）',
    '因子修改（存档修改）',
    '因子配装·实时录制/复刻',
    '记录角色当前的 12 个因子并导出分享，也可把配装文件逐项复刻到备用因子。（改的是游戏内因子；写存档配装预设请用「配装预设」。）',
    '祝福修改（存档修改）',
    '召唤石添加 / 修改（存档）',
    '在存档中新增召唤石，或原子修改已有记录的种类、主加护、副词条、等级和状态字段，写后重新打开存档验证。',
    '不会替未进入 DLC 的存档强开召唤系统；更换种类时会重建物品 SlotID 并迁移已装备引用。',
    '捕获游戏内当前选中的祝福石记录，并以一次事务核对、写入三条词条。',
    '常驻记录任务队伍配装，并提供选中物品读取、稳定坐标诊断和受限的一次性传送实验。',
    '队伍记录与物品读取保持只读；空间坐标写入和连续跳跃仅限离线/单机，并且需要逐项确认。穿墙/无碰撞仍未开放。',
    '存档实验室',
    '双存档只读研究',
    '逐条比较两份存档的逻辑记录，按类型、ID、位置和内容哈希定位差异，并导出不含路径与原始值的脱敏证据。',
    '未知字段只显示结构与哈希，不提供猜测写入。',
    '在一个位置查看工具版本、游戏文件和功能适配状态。',
    '识别游戏 EXE、创建原始文件备份并一键恢复。',
    '镜头 · 内置运行时',
    '运行时配装 · 内置 Hook',
  ]) {
    const escaped = text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const match = uiTranslations.match(new RegExp(`'${escaped}': '([^']+)'`))
    assert.ok(match, `missing complete tool translation: ${text}`)
    assert.doesNotMatch(match[1], /[\u3400-\u9fff]/u)
  }

  const i18n = readFileSync(new URL('./i18n.js', import.meta.url), 'utf8')
  assert.match(i18n, /const translatedAttributes = \['placeholder', 'title', 'aria-label', 'alt'\]/)
})

test('new freeform factor and inferred mastery copy has exact English translations', () => {
  for (const text of [
    '主方向由已点节点自动推导',
    '继续配置2阶节点；未形成或存在冲突时只提示，不会删除选择。',
    '方向与激活状态实时计算',
    '搜索主词条',
    '搜索副词条',
    '搜索主特性名称',
  ]) {
    assert.match(uiTranslations, new RegExp(`'${text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}': '[^']+'`))
  }
})

test('defense totals and scope copy remain English-only in English mode', () => {
  for (const text of [
    '防御力',
    '防御类',
    '防御力加成',
    '配装防御加成',
    '防御分区',
    '同区相加，跨区相乘',
    '通用加算区',
    '霸体乘区',
    '刚健乘区',
    '坚守乘区',
    '参考候选 · 待本机复测',
    '伊欧 +5% 实测将同一攻击从 36,938 降至 35,091，重复两次一致。当前满血参考按“同区相加，跨区相乘”展示；攻击 DOWN、战斗 Buff、坚守低血曲线、格挡和无敌没有当前状态时不强行计入。',
    '无条件防御力按百分比降低受击伤害；伊欧 +5% 实测将同一攻击从 36,938 降至 35,091，重复两次一致。条件防御、格挡、独立减伤和无敌仍保留在效果明细中，不混入该倍率。',
  ]) {
    assert.match(uiTranslations, new RegExp(`'${text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}': '[^']+'`))
  }
})

test('selective loadout import and first-sigil capture have exact English copy', () => {
  for (const text of [
    '选择要带入当前存档的内容',
    '默认保留目标角色强化、当前武器成长和整组武器收藏；只有你勾选的范围才会写入。',
    '同步武器强化',
    '只导入武器祝福',
    '角色强化进度',
    '攻击与 HP·抗性页进度；不改任何武器',
    '整组角色武器收藏',
    '同步该角色全部武器的等级、突破、幻晶、觉醒与超凡；会影响武器收集加成',
		'同步全部武器祝福',
		'逐把复制祝福类型与实际生效的三条附加技能；未佩戴祝福的源武器会清空目标对应武器',
    '匹配已有实例；缺少时自动新增并登记',
    '目标存档尚未建立该角色的专精字段；请先在游戏内开放专精系统，其他项目仍可单独导入。',
    '先装备目标角色的 12 个因子并停在第一项。若启动时没有自动读到第一项，按提示“↓一次、↑一次”完成首项握手，再从第二项逐项向下移动。',
    '导入文件后会先选择写入范围。因子、技能、专精、装备武器、祝福、召唤石与上限突破可任意多选；当前武器强化、角色强化进度和整组武器收藏默认不改，只有明确勾选才会覆盖。',
    '已载入导入草稿',
    '取消导入草稿',
    '已自动修正无效副词条',
    '已载入所选草稿并自动修正：%s',
  ]) {
    assert.match(uiTranslations, new RegExp(`'${text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}': '[^']+'`))
  }
})

test('GBFR Logs library copy has exact English translations', () => {
  for (const text of [
    'Logs 配装预览',
    '返回 Logs 配装库',
    'GBFR Logs 配装预览',
    '未记录玩家名',
    '选择存档并导入',
    'GBFR Logs 配装库',
    '返回配装预设',
    '等待选择数据库',
    '多角色配装导入',
    '从战斗记录中整理队伍配装，预览确认后再部署到存档。',
    '正在解析…',
    '更换数据库',
    '选择 Logs 数据库',
    '导入特性',
    '只读解析',
    '本地处理',
    '分项导入',
    '预览实际配装',
    '导入到存档',
    '尚未载入战斗记录',
    '选择 GBFR Logs 生成的 logs.db，队伍成员会分别列在这里。',
    '数据库在哪里？',
    'GBFR Logs、Endless、Relink Logs：右键程序快捷方式打开文件所在位置，或进入解压目录，选择与程序同目录的 logs.db。',
    String.raw`SkyMeter：打开 %APPDATA%\\app.skymeter.relink；旧版目录为 app.astralledger.relink。`,
    '先退出 Logs 再导入；不要选择 logs.db-wal 或 logs.db-shm。',
    '外部战斗记录',
    '从 GBFR Logs 批量获取队伍配装',
    '独立解析数据库中的多名角色，可逐个预览后再导入。',
  ]) {
    const escaped = text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const match = uiTranslations.match(new RegExp(`'${escaped}': '([^']+)'`))
    assert.ok(match, `missing Logs translation: ${text}`)
    assert.doesNotMatch(match[1], /[\u3400-\u9fff]/u)
  }
})
