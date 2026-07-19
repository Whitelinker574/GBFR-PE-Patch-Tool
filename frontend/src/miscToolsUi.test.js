import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/MiscTools.vue', import.meta.url), 'utf8')
const scopedStyle = source.match(/<style scoped>([\s\S]*?)<\/style>/)?.[1] || ''

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
  assert.match(source, /class="memory-title">素材不消耗</)
  assert.doesNotMatch(source, /class="memory-title">药神（/)
  assert.doesNotMatch(source, /class="memory-title">升级\/强化\/练成不材料消耗（/)
})

test('connection catalog and connected views keep every feature discoverable', () => {
  assert.match(source, /\['小钳蟹相关'/)
  assert.match(source, /runtimeCatalog\.slice\(3\)/)
  for (const label of ['任务得分倍率', '强制支线目标奖励', '任务内倍率']) {
    assert.match(source, new RegExp(label))
  }
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
  assert.ok(legacyActionButtons.length >= 30, `expected the runtime action set, got ${legacyActionButtons.length}`)
  for (const [, classes] of legacyActionButtons) {
    assert.match(classes, /(?:^|\s)ui-btn(?:\s|$)/, `missing ui-btn in: ${classes}`)
  }
})

test('experimental controls declare shared semantic button variants', () => {
  for (const handler of [
    'enableDamageMeter',
    'setCountdown',
    'setFaceAccessoryHidden\\(true\\)',
    'setInfiniteChallengeEnabled\\(true\\)',
    'setUnlockAllTrophyEnabled\\(true\\)',
    'setOtherSkinPurpleRuneEnabled\\(true\\)',
  ]) {
    assert.match(source, new RegExp(`<button class="[^"]*ui-btn[^"]*is-primary[^"]*"[^>]*@click="${handler}`))
  }
  assert.match(source, /<button class="[^"]*ui-btn[^"]*is-danger[^"]*" @click="confirmUnlockAllTrophy"/)
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
  assert.doesNotMatch(scopedStyle, /\.(?:update-new|update-body|od-select|od-indicator|od-mode-active|od-burst-active|burst-timer)\b/)
})
