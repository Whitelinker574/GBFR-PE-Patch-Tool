import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const componentURL = new URL('./components/RuntimePatchMonitor.vue', import.meta.url)
const source = existsSync(componentURL) ? readFileSync(componentURL, 'utf8') : ''

test('runtime monitor is a dedicated two-tab memory-monitoring page', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /data-page="runtime-patch-runtime-monitor"/)
  assert.match(source, /role="tablist"/)
  assert.match(source, /role="tab"[\s\S]*?:aria-selected=/)
  assert.match(source, /role="tabpanel"/)
  assert.match(source, /data-monitor-panel="party"/)
  assert.match(source, /data-monitor-panel="selected-items"/)
  assert.match(source, /t\('memoryMonitoring'\)/)
})

test('the page uses one owned Chara lease and retriable release lifecycle', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /CharaAcquire\(nextRuntimeAcquireRequestID\(\)\)/)
  assert.match(source, /from ['"]\.\.\/runtimeLeaseManager\.js['"]/)
  assert.match(source, /releaseRuntimeLease\(/)
  assert.match(source, /queueRuntimeLeaseRelease\(RUNTIME_LEASE_SCOPE,[\s\S]*?CharaRelease/)
  assert.match(source, /onBeforeUnmount\([\s\S]*?queueRuntimeLeaseRelease\(/)
  assert.match(source, /releasePending\.value\s*=\s*true/)
  assert.match(source, /completeRuntimeRelease/)
  assert.match(source, /clearRuntimeState\(\)/)
})

test('connect records the acquired owner before validating any other process fields', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  const acquireIndex = source.indexOf('await CharaAcquire(nextRuntimeAcquireRequestID())')
  const captureOwnerIndex = source.indexOf('acquiredOwnerToken = String(', acquireIndex)
  const validateIndex = source.indexOf('normalizedProcessInfo(', acquireIndex)
  assert.ok(acquireIndex >= 0, 'connect must acquire through CharaAcquire')
  assert.ok(captureOwnerIndex > acquireIndex, 'connect must immediately capture the returned owner token')
  assert.ok(validateIndex > captureOwnerIndex, 'owner capture must happen before PID/module validation so cleanup cannot lose ownership')
})

test('all async page actions share one epoch-aware operation gate', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /createOperationGate\(\)/)
  assert.match(source, /lifecycleEpoch/)
  assert.match(source, /operationIsCurrent\(/)
  for (const operation of ['connect', 'disconnect', 'party', 'capture-enable', 'capture-disable', 'capture-refresh', 'item-read']) {
    assert.ok(source.includes(`beginOperation('${operation}'`), `${operation} must enter the shared operation gate`)
  }
})

test('party cards render capabilities honestly instead of coercing absent values to zero', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /partyOptionalMetric\(entity, 'dodge'/)
  assert.match(source, /partyOptionalMetric\(entity, 'sba'/)
  assert.match(source, /t\('fieldUnavailable'\)/)
  assert.doesNotMatch(source, /(?:dodgeCount|sba|maxSba)\s*\|\|\s*0/)
  assert.match(source, /v-for="entity in partySnapshot\.entities"/)
  assert.match(source, /formatPosition\(entity\.position\)/)
  assert.match(source, /entity\.present/)
  assert.match(source, /t\('notInParty'\)/)
})

test('selected-item reading binds ExpectedSelectedAddr and becomes reselect-required after one read', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /RuntimePatchSelectedItemsEnableOwned/)
  assert.match(source, /RuntimePatchSelectedItemsStatusOwned/)
  assert.match(source, /RuntimePatchSelectedItemReadOwned\(ownerToken,\s*\{\s*kind,\s*expectedSelectedAddr\s*\}\s*\)/)
  assert.match(source, /consumeRuntimePatchSelectedCapture\(/)
  assert.match(source, /consumedSelections\[kind\]\s*=\s*true/)
  assert.match(source, /t\('needsReselection'\)/)
  assert.match(source, /record\.hashHex/)
  assert.match(source, /record\.name/)
  assert.match(source, /record\.quantity/)
  assert.match(source, /record\.flagsHex/)
})

test('the item panel is conspicuously read-only and exposes no inventory writer controls', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /t\('readOnlyBanner'\)/)
  assert.match(source, /t\('neverWritesSave'\)/)
  assert.doesNotMatch(source, /RuntimePatchSetEnabledOwned|CurrencySet|PotionSet|MonsterEnhance|QuestScore|ActionSpeed/)
  assert.doesNotMatch(source, /type="number"|contenteditable|ui-input/)
  assert.doesNotMatch(source, /new Error\(['"](?:runtime|read-only capture)/, 'visible internal errors must come from bilingual copy')
})

test('the page keeps the parchment atom system responsive from narrow to ultra-wide containers', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /class="[^"]*ui-card/)
  assert.match(source, /class="[^"]*ui-btn/)
  assert.match(source, /container-name:\s*runtime-monitor/)
  assert.match(source, /@container runtime-monitor \(min-width:\s*760px\)/)
  assert.match(source, /@container runtime-monitor \(min-width:\s*1500px\)/)
  assert.match(source, /@container runtime-monitor \(max-width:\s*480px\)/)
  assert.match(source, /grid-template-columns:\s*repeat\(auto-fit,\s*minmax\(min\(100%,\s*260px\),\s*1fr\)\)/)
  assert.match(source, /@media \(prefers-reduced-motion:\s*reduce\)/)
})

test('the embedded page does not repeat the shell heading and keeps the narrow status badge intact', () => {
  assert.doesNotMatch(source, /<header class="monitor-hero/)
  assert.match(source, /data-page="runtime-patch-runtime-monitor"[^>]*>\s*<section class="monitor-connection/)
  assert.match(source, /\.connection-summary > \.ui-tag\s*\{[^}]*flex:\s*none[^}]*white-space:\s*nowrap/s)
})

test('tabs and live status expose keyboard and screen-reader state', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /aria-live="polite"/)
  assert.match(source, /@keydown="onTabKeydown/)
  assert.match(source, /:tabindex="activeTab === tab\.id \? 0 : -1"/)
  assert.match(source, /:aria-busy="operationBusy"/)
  assert.match(source, /:disabled="interactionLocked/)
})
