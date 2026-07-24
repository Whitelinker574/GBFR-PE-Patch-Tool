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
  ]) {
    assert.match(uiTranslations, new RegExp(`'${text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}': '[^']+'`))
  }
})
