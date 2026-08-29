import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/MiscTools.vue', import.meta.url), 'utf8')
const progression = readFileSync(new URL('./components/ProgressionEditor.vue', import.meta.url), 'utf8')
const scopedStyle = source.match(/<style scoped>([\s\S]*?)<\/style>/)?.[1] || ''

test('blank runtime values are rejected before numeric conversion', () => {
  assert.equal((source.match(/trim\(\) === ''/g) || []).length, 2)
})

test('currency and potion editors keep their own numeric ranges and error copy', () => {
  const currencyHandler = source.match(/function setCurrency\(item\) \{([\s\S]*?)\n\}/)?.[1] || ''
  const potionHandler = source.match(/function setPotion\(item\) \{([\s\S]*?)\n\}/)?.[1] || ''

  assert.match(currencyHandler, /value > 2147483647/)
  assert.match(currencyHandler, /请输入 0 到 2147483647 之间的整数/)
  assert.doesNotMatch(currencyHandler, /药水请输入/)

  assert.match(potionHandler, /value > 999/)
  assert.match(potionHandler, /药水请输入 0 到 999 之间的整数/)
  assert.doesNotMatch(potionHandler, /2147483647/)
})

test('game 2.0.5 uses the new natural MSP cap without turning it into a hard write limit', () => {
  assert.match(source, /item\?\.id === 'msp' \? 9999999 : 2147483647/)
  assert.match(source, /currencyNaturalMax\(item\)[\s\S]*?自然上限/)
  assert.match(source, /value > 2147483647/)
  assert.match(progression, /mastery:\s*9999999/)
  assert.match(progression, /resources\.mastery=9999999[\s\S]*?自然上限/)
  assert.match(progression, /max="999999999"/)
})

test('runtime tools consume shared page, panel, toolbar, tabs, controls and cards', () => {
  assert.match(source, /class="root ui-page is-wide ui-page-stack"/)
  assert.match(source, /class="section ui-card ui-panel"/)
  assert.match(source, /class="connect-row ui-toolbar"/)
  assert.match(source, /class="runtime-tabs ui-seg"/)
  assert.match(source, /class="memory-card[^\"]*ui-card ui-panel is-compact/)
  assert.match(source, /class="[^\"]*ui-input/)
  assert.match(source, /class="[^\"]*ui-btn/)
})

test('runtime feature titles stay short and put operational detail in helper text', () => {
  assert.match(source, /class="memory-title">副本药水</)
  assert.match(source, /class="memory-title">免费制作、交易与升级</)
  assert.match(source, /class="memory-title">库存素材扣减保护</)
  assert.doesNotMatch(source, /class="memory-title">药神（/)
  assert.doesNotMatch(source, /class="memory-title">升级\/强化\/练成不材料消耗（/)
})

test('connection catalog and connected views keep every feature discoverable', () => {
  assert.match(source, /\['小钳蟹相关'/)
  for (const label of ['实时货币编辑', '副本药水', '免费制作、交易与升级', '库存素材扣减保护', '连续挑战', '巴武掉落 100%']) {
    assert.match(source, new RegExp(label))
  }
  assert.doesNotMatch(source, /runtimeCatalog\.slice\(/)
})

test('connection catalog centers an even card grid while card content shares a left edge', () => {
  assert.match(source, /class="preflight-copy"/)
  assert.match(source, /class="preflight-status"/)
  assert.match(scopedStyle, /\.preflight-grid\s*\{[\s\S]*?width\s*:\s*min\(100%,960px\)[\s\S]*?margin-inline\s*:\s*auto[\s\S]*?grid-template-columns\s*:\s*repeat\(2,minmax\(0,1fr\)\)/)
  assert.match(scopedStyle, /\.preflight-grid article\s*\{[\s\S]*?align-items\s*:\s*flex-start[\s\S]*?text-align\s*:\s*left/)
  assert.match(scopedStyle, /\.preflight-grid article:last-of-type:nth-child\(odd\)\s*\{[\s\S]*?grid-column\s*:\s*1\s*\/\s*-1[\s\S]*?justify-self\s*:\s*center/)
  assert.match(scopedStyle, /\.preflight-status\s*\{[\s\S]*?margin-top\s*:\s*auto[\s\S]*?justify-content\s*:\s*flex-start/)
  assert.match(scopedStyle, /@container\s+runtime-page\s*\(max-width\s*:\s*480px\)\s*\{[\s\S]*?\.preflight-grid\s*\{[\s\S]*?grid-template-columns\s*:\s*minmax\(0,1fr\)[\s\S]*?article:last-of-type:nth-child\(odd\)[\s\S]*?width\s*:\s*100%/)
})

test('technical bytes are collapsed into shared disclosures', () => {
  assert.match(source, /class="memory-diagnostics ui-disclosure"/)
  assert.match(source, /<summary>技术详情<\/summary>/)
  assert.doesNotMatch(source, /<div class="memory-bytes">/)
})

test('runtime layout reflows from its container and keeps readable text', () => {
  assert.match(source, /container\s*:\s*runtime-page\s*\/\s*inline-size/)
  assert.match(source, /@container\s+runtime-page\s*\(max-width\s*:\s*720px\)/)
  assert.doesNotMatch(source, /font-size\s*:\s*(?:0\.[0-6][0-9]?rem|(?:[0-9]|10)px)/i)
})

test('every runtime action button uses the shared button primitive', () => {
  const legacyActionButtons = [...source.matchAll(/<button\b[^>]*class="([^"]*\bbtn-(?:connect|disconnect|max|batch|refresh|sort|warn)\b[^"]*)"[^>]*>/g)]
  assert.ok(legacyActionButtons.length >= 18, `expected the stable runtime action set, got ${legacyActionButtons.length}`)
  for (const [, classes] of legacyActionButtons) {
    assert.match(classes, /(?:^|\s)ui-btn(?:\s|$)/, `missing ui-btn in: ${classes}`)
  }
})

test('runtime tools expose only the stable resources and mission groups', () => {
  assert.doesNotMatch(source, /defineProps\s*\(/)
  assert.doesNotMatch(source, /\b(?:showOutdatedFeatures|showStableFeatures)\b/)
  assert.match(source, /const activeRuntimeGroup = ref\('resources'\)/)
  for (const group of ['resources', 'mission']) {
    assert.match(source, new RegExp(`activeRuntimeGroup === '${group}'`))
  }
  for (const group of ['battle', 'display', 'compatibility']) {
    assert.doesNotMatch(source, new RegExp(`['"]${group}['"]`))
  }
})

test('experimental runtime integrations are absent from the stable page', () => {
  for (const symbol of [
    'Countdown',
    'FaceAccessory',
    'UnlockAllTrophy',
    'OtherSkinPurpleRune',
    'DamageMeter',
    'DamageOverlay',
  ]) {
    assert.doesNotMatch(source, new RegExp(symbol), `${symbol} must not remain in MiscTools.vue`)
  }
  for (const label of ['待适配运行时功能', '兼容性实验室', '战斗与任务', '显示与解锁', '团队伤害记录', '任务结算倒计时', '无限挑战']) {
    assert.doesNotMatch(source, new RegExp(label), `${label} must not remain visible`)
  }
})

test('continuous challenge is a stable owned mission action', () => {
  assert.match(source, /InfiniteChallengeGetStatusOwned/)
  assert.match(source, /InfiniteChallengeSetEnabledOwned/)
  assert.match(source, /连续挑战[\s\S]*2\.0\.3–2\.0\.5 唯一 AOB · 三字节补丁 · 写后回读/)
  assert.match(source, /infiniteChallengeStatus\.owned/)
})

test('full free consumption is a separate owned atomic action, not a renamed material hook', () => {
  assert.match(source, /FreeConsumptionGetStatusOwned\(connectionOwnerToken\)/)
  assert.match(source, /FreeConsumptionSetEnabledOwned\(connectionOwnerToken, enabled\)/)
  assert.match(source, /11 站点原子补丁/)
  assert.match(source, /它不是上方完整的免费制作功能/)
})

test('runtime scoped styles contain no legacy dark palette or scale hover', () => {
  assert.doesNotMatch(scopedStyle, /rgba\(\s*255\s*,\s*255\s*,\s*255/i)
  assert.doesNotMatch(scopedStyle, /rgba\(\s*0\s*,\s*0\s*,\s*0/i)
  assert.doesNotMatch(scopedStyle, /#(?:fff(?:fff)?|1a1a2e|1f2937|a5b4fc|4ade80|f87171|d9bd7c)\b/i)
  assert.doesNotMatch(scopedStyle, /scale\s*\(/i)
})

test('runtime scoped styles have one semantic layer without dead legacy selectors', () => {
  for (const selector of ['section', 'runtime-tabs', 'preflight-grid', 'memory-card', 'memory-title', 'feature-help', 'currency-row']) {
    const declarations = scopedStyle.match(new RegExp(`^\\.${selector}\\s*\\{`, 'gm')) || []
    assert.equal(declarations.length, 1, `${selector} has ${declarations.length} base declarations`)
  }
  assert.doesNotMatch(scopedStyle, /\.memory-card::after/)
  assert.doesNotMatch(scopedStyle, /\.(?:update-new|update-body|od-select|od-indicator|od-mode-active|od-burst-active|burst-timer|damage-meter-info|damage-meter-value|damage-meter-raw|countdown-input|reference-grid|reference-card|confirm-overlay|confirm-dialog|confirm-title|confirm-body|confirm-actions)\b/)
})
