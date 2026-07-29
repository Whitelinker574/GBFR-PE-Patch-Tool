import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const componentURL = new URL('./components/RuntimePatchMonitor.vue', import.meta.url)
const source = existsSync(componentURL) ? readFileSync(componentURL, 'utf8') : ''
const detector = readFileSync(new URL('./components/RuntimeLoadoutDetector.vue', import.meta.url), 'utf8')

test('runtime monitor keeps one lifecycle implementation while the shell selects one destination mode', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /data-page="runtime-patch-runtime-monitor"/)
  assert.match(source, /validator:\s*value => \['party', 'spatial', 'items'\]\.includes\(value\)/)
  assert.match(source, /watch\(\(\) => props\.mode, value => \{ activeTab\.value = value \}/)
  assert.doesNotMatch(source, /role="tablist"|class="monitor-tabs/)
  assert.match(detector, /data-monitor-panel="party"/)
  assert.match(source, /data-monitor-panel="spatial"/)
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
  for (const operation of ['connect', 'disconnect', 'capture-enable', 'capture-disable', 'capture-refresh', 'item-read', 'spatial-read', 'spatial-teleport']) {
    assert.ok(source.includes(`beginOperation('${operation}'`), `${operation} must enter the shared operation gate`)
  }
})

test('quest history groups every stable party as one local record', () => {
  assert.match(detector, /v-for="record in records"/)
  assert.match(detector, /v-for="member in record\.members"/)
  assert.match(detector, /RuntimeLoadoutDetectorStatus/)
  assert.match(detector, /RuntimeLoadoutDetectorHistory/)
  assert.match(detector, /RuntimeLoadoutDetectorDelete/)
  assert.doesNotMatch(detector, /RuntimePatchPartyMonitorOwned|readPartyLoadouts/)
})

test('selected-item reading binds ExpectedSelectedAddr and becomes reselect-required after one read', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /RuntimePatchSelectedItemsEnableOwned/)
  assert.match(source, /RuntimePatchSelectedItemsStatusOwned/)
  assert.match(source, /RuntimePatchSelectedItemReadOwned\(ownerToken,\s*\{\s*kind,\s*expectedSelectedAddr\s*\}\s*\)/)
  assert.match(source, /consumeRuntimePatchSelectedCapture\(/)
  assert.match(source, /consumedSelections\[kind\]\s*=\s*true/)
  assert.match(source, /t\('needsReselection'\)/)
  assert.match(source, /selectedRecords\[kind\] = Object\.freeze\(\{ \.\.\.record \}\)/)
  assert.match(source, /selectedRecords\[kind\]\.hashHex/)
  assert.match(source, /selectedRecords\[kind\]\.name/)
  assert.match(source, /selectedRecords\[kind\]\.quantity/)
  assert.match(source, /selectedRecords\[kind\]\.flagsHex/)
})

test('the item panel is conspicuously read-only and exposes no inventory writer controls', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /t\('readOnlyBanner'\)/)
  assert.match(source, /t\('neverWritesSave'\)/)
  assert.doesNotMatch(source, /RuntimePatchSetEnabledOwned|CurrencySet|PotionSet|MonsterEnhance|QuestScore|ActionSpeed/)
  const selectedPanel = source.slice(source.indexOf('id="runtime-monitor-panel-items"'))
  assert.doesNotMatch(selectedPanel, /type="number"|contenteditable|RuntimeSpatialTeleportOwned/)
  assert.doesNotMatch(source, /new Error\(['"](?:runtime|read-only capture)/, 'visible internal errors must come from bilingual copy')
})

test('spatial diagnostics use the verified three-snapshot reader and isolate the experimental writer', () => {
  assert.match(source, /RuntimePatchPartyMonitorOwned\(ownerToken\)/)
  assert.match(source, /normalizeRuntimePatchPartySnapshot/)
  assert.match(source, /RuntimeSpatialTeleportOwned\(ownerToken, teleportVector\(\)\)/)
  assert.match(source, /Math\.abs\(value\) > 10_000_000/)
  assert.match(source, /t\('spatialUnsupported'\)/)
  assert.match(source, /grid-template-columns:repeat\(auto-fit,minmax\(min\(100%,210px\),1fr\)\)/)
  assert.match(source, /gbfr-codex-spatial-bookmarks-v1/)
  assert.match(source, /if \(!spatialOrigin\.value\) spatialOrigin\.value = Object\.freeze/)
  assert.match(source, /@click="fillTeleportTarget\(bookmark\.position\)"/)
  assert.doesNotMatch(source, /@click="teleportPlayer\(bookmark/)
})

test('gravity suppression is an owned verified toggle while noclip remains unavailable', () => {
  assert.match(source, /RuntimeSpatialGravityStatusOwned\(acquiredOwnerToken\)/)
  assert.match(source, /RuntimeSpatialGravitySetEnabledOwned\(ownerToken, enabled\)/)
  assert.match(source, /normalizeRuntimeSpatialGravityStatus/)
  assert.match(source, /setGravityEnabled\(!gravityStatus\.enabled\)/)
  assert.match(source, /t\('spatialNoclip'\)[\s\S]*?disabled/)
  assert.doesNotMatch(source, /t\('spatialGravity'\)[\s\S]{0,160}?disabled>\{\{ t\('spatialUnavailable'\)/)
  assert.match(source, /\.flight-capability\.is-active/)
  assert.match(source, /@container runtime-monitor \(max-width:460px\)[\s\S]*?\.flight-capability/)
})

test('held flight uses the planned 0.1-1000 units-per-second range with an 8 default', () => {
  assert.match(source, /const FLIGHT_FRAME_MS = 45/)
  assert.match(source, /const flightSpeed = ref\(8\)/)
  assert.match(source, /speed \* FLIGHT_FRAME_MS \/ 1000/)
  assert.match(source, /speed < 0\.1 \|\| speed > 1000/)
  assert.match(source, /v-model\.number="flightSpeed"/)
  assert.match(source, /min="0\.1" max="1000" step="0\.1"/)
  assert.doesNotMatch(source, /const flightStep|v-model="flightStep"/)
})

test('the runtime monitor exposes a global and focused emergency stop', () => {
  assert.match(source, /RuntimeEmergencyStop/)
  assert.match(source, /EventsOn\('runtime-emergency-stop'/)
  assert.match(source, /event\.key !== 'Escape' && event\.key !== 'F12'/)
  assert.match(source, /@click="emergencyStop"/)
  assert.match(source, /stopEmergencyEvents\?\.\(\)/)
})

test('the page keeps the parchment atom system responsive from narrow to ultra-wide containers', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /class="[^"]*ui-card/)
  assert.match(source, /class="[^"]*ui-btn/)
  assert.match(source, /container:runtime-monitor\s*\/\s*inline-size/)
  assert.match(source, /@container runtime-monitor \(max-width:720px\)/)
  assert.match(detector, /@container loadout-detector \(max-width:860px\)/)
  assert.match(detector, /grid-template-columns:repeat\(auto-fit,minmax\(min\(100%,220px\),1fr\)\)/)
  assert.match(detector, /@media \(prefers-reduced-motion:reduce\)/)
})

test('the embedded page does not repeat the shell heading and keeps the narrow status badge intact', () => {
  assert.doesNotMatch(source, /<header class="monitor-hero/)
  assert.match(source, /data-page="runtime-patch-runtime-monitor"[^>]*>\s*<RuntimeLoadoutDetector/)
  assert.match(source, /\.connection-summary > \.ui-tag\s*\{[^}]*flex:none/s)
})

test('live status and operations expose screen-reader and busy state without a second local tab bar', () => {
  assert.ok(source, 'RuntimePatchMonitor.vue must exist')
  assert.match(source, /aria-live="polite"/)
  assert.doesNotMatch(source, /@keydown="onTabKeydown|:tabindex="activeTab === tab\.id/)
  assert.match(source, /:aria-busy="operationBusy"/)
  assert.match(source, /:disabled="interactionLocked/)
})
