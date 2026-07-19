import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const componentUrl = new URL('./components/WrightstoneMemoryGenerator.vue', import.meta.url)
const component = existsSync(componentUrl) ? readFileSync(componentUrl, 'utf8') : ''
const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const offlineGenerator = readFileSync(new URL('./components/WrightstoneGenerator.vue', import.meta.url), 'utf8')

test('live wrightstone editor is reachable from the realtime group and reuses its existing artwork', () => {
  assert.ok(component, 'WrightstoneMemoryGenerator.vue must exist')
  assert.match(shell, /import WrightstoneMemoryGenerator from '.\/WrightstoneMemoryGenerator\.vue'/)
  assert.match(shell, /wrightstoneMemory:\s*\{[\s\S]*?group:\s*'memory'[\s\S]*?tone:\s*'live'/)
  assert.match(shell, /items:\s*\[[^\]]*'wrightstoneMemory'/)
  assert.match(shell, /wrightstoneMemory:\s*wrightstoneArt/)
  assert.match(shell, /wrightstoneMemory:\s*wrightstoneSticker/)
  assert.match(shell, /<WrightstoneMemoryGenerator\s+v-else-if="activeTab === 'wrightstoneMemory'"/)
  assert.match(shell, /\.tool-stage\[data-tool="wrightstoneMemory"\]/)
})

test('live wrightstone editor owns an explicit enable, polling and disable lifecycle', () => {
  assert.match(component, /WrightstoneMemoryGetOptions/)
  assert.match(component, /WrightstoneMemoryGetStatus/)
  assert.match(component, /WrightstoneMemoryAcquire\(nextRuntimeAcquireRequestID\(\)\)/)
  assert.match(component, /WrightstoneMemoryUpdateOwned\(ownerToken,/)
  assert.match(component, /WrightstoneMemoryRelease/)
  assert.match(component, /setInterval\([^,]+,\s*700\)/s)
  assert.match(component, /onBeforeUnmount\([\s\S]*?queueRuntimeLeaseRelease\([^;]*ownerToken[^;]*WrightstoneMemoryRelease/)
  assert.doesNotMatch(component, /WrightstoneMemory(?:Enable|Disable)/)
  assert.match(component, />\s*启用读取\s*</)
  assert.match(component, />\s*停止读取\s*</)
})

test('live wrightstone feedback uses the shared warning and low-emphasis stop treatments', () => {
  assert.match(component, /class="ui-btn is-ghost"[^>]*@click="disable"[^>]*>\s*停止读取\s*</)
  assert.match(component, /:class="\{\s*'is-warn'\s*:\s*stale\s*\}"/)
  assert.doesNotMatch(component, /\bis-warning\b|class="ui-btn is-danger"[^>]*@click="disable"/)
})

test('live wrightstone editor exposes current and target values for all three slots', () => {
	assert.match(component, /const EMPTY_HASH\s*=\s*0x887AE0B0/)
	assert.match(component, /function normaliseHash\([^)]*\)[\s\S]*EMPTY_HASH/)
  assert.match(component, /v-for="\(slot, index\) in traitSlots"/)
  assert.match(component, /class="[^"]*trait-current[^"]*"/)
  assert.match(component, /class="[^"]*trait-target[^"]*"/)
  assert.match(component, /v-model\.number="slot\.level"/)
  assert.match(component, /Array\.isArray\(option\?\.allowedLevels\)/)
  assert.match(component, /v-for="level in allowedLevels\(slot\)"/)
  assert.match(component, /label:\s*'第一槽'[^}]*maxLevel:\s*20/)
  assert.match(component, /label:\s*'第二槽'[^}]*maxLevel:\s*15/)
  assert.match(component, /label:\s*'第三槽'[^}]*maxLevel:\s*10/)
  assert.match(component, /第一槽/)
  assert.match(component, /第二槽/)
  assert.match(component, /第三槽/)
  assert.match(component, /<details[^>]*class="[^"]*ui-disclosure[^"]*change-summary/)
})

test('successful writes restore the hook immediately and explain the blacksmith safety boundary', () => {
  assert.match(component, /await WrightstoneMemoryUpdateOwned\(ownerToken,/)
  assert.match(component, /expectedSelectedAddr/)
  assert.match(component, /await releaseRuntimeLease\(RUNTIME_LEASE_SCOPE, ownerToken, WrightstoneMemoryRelease\)/)
  assert.match(component, /hookOwnerToken\s*=\s*''/)
  assert.match(component, /写入成功[\s\S]*自动停止/)
  assert.match(component, /铁匠铺[\s\S]*(?:崩溃|闪退)/)
  assert.match(component, /aria-live="polite"/)
  assert.doesNotMatch(component, /\b(?:window\.)?(?:alert|confirm)\s*\(/)
  assert.match(component, /<ConfirmDialog\s+ref="confirmDialog"/)
})

test('selection and lifecycle races are closed before a live write', () => {
  assert.match(component, /const expectedSelectedAddr\s*=\s*Number\(status\.selectedAddr/)
  assert.match(component, /let disposed\s*=\s*false/)
  assert.match(component, /let lifecycleEpoch\s*=\s*0/)
  assert.match(component, /if \(disposed \|\| epoch !== lifecycleEpoch\)[\s\S]*queueRuntimeLeaseRelease\([^;]*acquiredOwnerToken[^;]*WrightstoneMemoryRelease/)
  assert.match(component, /onBeforeUnmount\(\(\) => \{[\s\S]*disposed\s*=\s*true[\s\S]*lifecycleEpoch\+\+/)
  assert.match(component, /new Set\([^)]*normaliseHash/)
})

test('live wrightstone editor uses shared atoms, one scroll container and responsive container rules', () => {
  for (const atom of ['ui-page', 'ui-page-stack', 'ui-card', 'ui-panel', 'ui-btn', 'ui-field', 'ui-input', 'ui-select']) {
    assert.match(component, new RegExp(`\\b${atom}\\b`), `missing shared ${atom} primitive`)
  }
  assert.match(component, /\.wrightstone-memory-actions\s*\{[^}]*position:\s*sticky/is)
  assert.match(component, /\.wrightstone-memory-actions\s*\{[^}]*font-family:\s*var\(--font-data\)/is)
  assert.equal((component.match(/overflow(?:-y)?:\s*(?:auto|scroll)/g) || []).length, 0, 'page must not nest another scrolling surface')
  assert.match(shell, /\.workspace-scroll\s*\{[^}]*overflow:\s*auto/is)
  assert.match(component, /@container\s+ui-page\s*\(max-width:\s*375px\)/)
  assert.match(component, /@container\s+ui-page\s*\(max-width:\s*768px\)/)
  assert.match(component, /@container\s+ui-page\s*\(max-width:\s*1024px\)/)
  assert.match(component, /@container\s+ui-page\s*\(min-width:\s*1080px\)/)
  assert.doesNotMatch(component, /@container\s+ui-page\s*\(min-width:\s*1440px\)/)
  assert.match(component, /previousSelectedAddr[\s\S]*liveMessage\.value\s*=\s*'已读取当前祝福石记录。'/)
})

test('offline wrightstone legality failure is fail-closed', () => {
  const catchBody = offlineGenerator.match(/async function refreshLegality\(\)[\s\S]*?catch \(e\) \{([\s\S]*?)\n\s*\}/)?.[1] || ''
  assert.match(catchBody, /status:\s*'impossible'/)
  assert.match(catchBody, /writable:\s*false/)
  assert.doesNotMatch(catchBody, /writable:\s*true/)
})
