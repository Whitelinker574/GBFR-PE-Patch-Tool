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
  assert.match(offlineWrightstone, /Above legal cap/)
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
