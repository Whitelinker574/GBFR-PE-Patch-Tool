import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { uiTranslations } from './i18n-ui.js'

const componentNames = [
  'SigilMemoryGenerator.vue',
  'SigilLoadoutRestore.vue',
  'SummonEditor.vue',
  'OverLimit.vue',
  'MonsterEnhance.vue',
]

const sources = Object.fromEntries(componentNames.map((name) => [
  name,
  readFileSync(new URL(`./components/${name}`, import.meta.url), 'utf8'),
]))
const picker = readFileSync(new URL('./components/SigilMemoryPicker.vue', import.meta.url), 'utf8')
const wailsModels = readFileSync(new URL('../wailsjs/go/models.ts', import.meta.url), 'utf8')
const miscTools = readFileSync(new URL('./components/MiscTools.vue', import.meta.url), 'utf8')
const i18nUi = readFileSync(new URL('./i18n-ui.js', import.meta.url), 'utf8')

test('runtime currency copy publishes resonance points instead of the incorrect CP label', () => {
  assert.equal((miscTools.match(/共鸣点数（RP）/g) || []).length, 3, 'catalog and both help surfaces must name RP')
  assert.doesNotMatch(miscTools, /\bCP\b/, 'the runtime UI must not publish the legacy CP label')
  assert.equal(uiTranslations['共鸣点数（RP）'], 'Resonance Points (RP)', 'a dynamic currency item name needs its own exact translation')
  assert.doesNotMatch(i18nUi, /\bCP\b/, 'the runtime translation catalog must not publish the legacy CP label')
})

test('generated Wails models retain sigil catalog legality metadata', () => {
  const optionStart = wailsModels.indexOf('export class SigilMemoryOption')
  const optionEnd = wailsModels.indexOf('export class SigilMemoryOptions', optionStart)
  const optionModel = wailsModels.slice(optionStart, optionEnd)
  assert.ok(optionStart >= 0 && optionEnd > optionStart, 'SigilMemoryOption binding is missing')
  assert.match(optionModel, /primaryTraitHash\??:\s*number/)
  assert.match(optionModel, /allowedPrimaryTraitLevels\??:\s*number\[\]/)
  assert.match(optionModel, /this\.primaryTraitHash\s*=\s*source\["primaryTraitHash"\]/)
  assert.match(optionModel, /this\.allowedPrimaryTraitLevels\s*=\s*source\["allowedPrimaryTraitLevels"\]/)
})

test('every realtime page composes the shared page, panel, card and control primitives', () => {
  for (const [name, source] of Object.entries(sources)) {
    assert.match(source, /class="[^"]*\bui-page\b[^"]*\bui-page-stack\b|class="[^"]*\bui-page-stack\b[^"]*\bui-page\b/, `${name} needs the shared page flow`)
    assert.match(source, /\bui-card\b/, `${name} needs shared cards`)
    assert.match(source, /\bui-panel\b/, `${name} needs shared panel spacing`)
    assert.match(source, /\bui-btn\b/, `${name} needs shared buttons`)
    assert.match(source, /@container\s+ui-page\s*\(/, `${name} must reflow from its actual content width`)
  }
})

test('realtime page styles no longer carry the old dark skin or sub-11px text', () => {
  for (const [name, source] of Object.entries({ ...sources, 'SigilMemoryPicker.vue': picker })) {
    assert.doesNotMatch(source, /rgba\(\s*(?:255\s*,\s*255\s*,\s*255|8\s*,\s*31\s*,\s*53)/i, `${name} still contains the old dark translucent skin`)
    assert.doesNotMatch(source, /font-size\s*:\s*(?:0\.[0-7]\d*rem|(?:[0-9]|10(?:\.\d+)?)px)/i, `${name} still renders helper text below 11px`)
  }
})

test('sigil editor tabs and row tools are discoverable without hover', () => {
  const source = sources['SigilMemoryGenerator.vue']
  assert.match(source, /<button[^>]*class="[^"]*ui-tab[^"]*"[^>]*>\s*模板/)
  assert.match(source, /<button[^>]*class="[^"]*ui-tab[^"]*"[^>]*>\s*最近写入/)
  assert.doesNotMatch(source, /\.row-tools\s*\{[^}]*opacity\s*:\s*0/is)
  assert.match(source, /class="[^"]*row-tool[^"]*ui-btn[^"]*|class="[^"]*ui-btn[^"]*row-tool[^"]*"/)
})

test('sigil live editors bind writes to the captured row and release their owned hook', () => {
	const editor = sources['SigilMemoryGenerator.vue']
	const restore = sources['SigilLoadoutRestore.vue']
	for (const [name, source] of Object.entries({ editor, restore })) {
		assert.match(source, /SigilMemoryAcquire/, `${name} must acquire an owner-scoped hook lease`)
		assert.match(source, /SigilMemoryRelease/, `${name} must import owner-scoped hook teardown`)
		assert.match(source, /expectedSelectedAddr/, `${name} must bind writes to the captured address`)
		assert.match(source, /onBeforeUnmount\([\s\S]*queueRuntimeLeaseRelease\([^;]*ownerToken[^;]*SigilMemoryRelease/i, `${name} must queue only its owned hook on unmount`)
		assert.doesNotMatch(source, /SigilMemory(?:Enable|Disable)/, `${name} must not use the unconditional compatibility hook API`)
	}
})

test('manual and natural loadout stops release the sigil hook immediately', () => {
  const source = sources['SigilLoadoutRestore.vue']
  assert.match(source, /async function stop\([^)]*\)\s*\{[\s\S]*?await releaseRuntimeLease\([^;]*SigilMemoryRelease\)/)
  assert.match(source, /async function startRecord\([^)]*\)[\s\S]*?await SigilMemoryAcquire\(nextRuntimeAcquireRequestID\(\)\)/)
  assert.match(source, /async function startWrite\([^)]*\)[\s\S]*?await SigilMemoryAcquire\(nextRuntimeAcquireRequestID\(\)\)/)
  for (const [name, endMarker] of [['startRecord', '\nasync function startWrite'], ['startWrite', '\nfunction removeEntry']]) {
    const start = source.indexOf(`async function ${name}()`)
    const end = source.indexOf(endMarker, start)
    const body = source.slice(start, end)
    const acquireAt = body.indexOf('await SigilMemoryAcquire(nextRuntimeAcquireRequestID())')
    const releaseAt = body.indexOf('releaseRuntimeLease(')
    assert.ok(acquireAt >= 0 && (releaseAt < 0 || acquireAt < releaseAt), `${name} must acquire before any stale-result cleanup`)
  }
  assert.doesNotMatch(source, /function stopPolling\([^)]*\)\s*\{[\s\S]{0,180}?mode\.value\s*=/)
})

test('live sigil legality is advisory by default', () => {
  const source = sources['SigilMemoryGenerator.vue']
  assert.doesNotMatch(source, /forceWrite/)
  assert.match(source, /status: 'forced'/)
  assert.match(source, /合规检测仅作提示/)
  assert.match(source, /legality\.value\.status === 'forced'/)
})

test('sigil memory has an explicit stop action and locks draft controls until a row is captured', () => {
  const source = sources['SigilMemoryGenerator.vue']
  assert.match(source, /async function disable\(\)[\s\S]*?releaseRuntimeLease\([^;]*SigilMemoryRelease\)/)
  assert.match(source, /class="ui-btn is-sm is-ghost"[^>]*@click="disable"[^>]*>\s*停止读取\s*</)
  assert.match(source, /class="ui-btn is-sm is-primary"[^>]*:disabled="loading \|\| applying \|\| status\.hooked"[^>]*@click="enable"/)
  assert.equal((source.match(/<SigilMemoryPicker[^>]*:disabled="!status\.selectedAddr \|\| loading \|\| applying[^\"]*"/g) || []).length, 3)
  assert.equal((source.match(/<input[^>]*class="ui-input"[^>]*:disabled="!status\.selectedAddr \|\| loading \|\| applying"/g) || []).length, 3)
})

test('the factor picker uses shared controls without nesting an interactive clear action', () => {
  assert.match(picker, /class="[^"]*picker-selected[^"]*ui-btn[^"]*|class="[^"]*ui-btn[^"]*picker-selected[^"]*"/)
  assert.match(picker, /class="[^"]*picker-search[\s\S]*class="ui-input"/)
  assert.doesNotMatch(picker, /<button[^>]*picker-selected[\s\S]*role="button"[\s\S]*<\/button>/)
  assert.match(picker, /<button[^>]*picker-inline-clear/)
  assert.match(picker, /function clearSelection\(\)\s*\{\s*if \(props\.disabled\) return/)
  assert.match(picker, /<button[^>]*picker-inline-clear[^>]*:disabled="disabled"/)
})

test('live sigil factor and trait fields match the save editor responsive form geometry', () => {
  const source = sources['SigilMemoryGenerator.vue']
  assert.equal((source.match(/class="aligned-picker"/g) || []).length, 3)
  assert.match(source, /\.editor-control-grid\s*\{[^}]*width:100%;[^}]*max-width:none;[^}]*grid-template-columns:minmax\(0,1fr\) minmax\(180px,220px\) auto;/s)
  assert.match(source, /\.aligned-picker,\.limit-button\s*\{[^}]*margin-top:22px;/s)
  assert.match(source, /\.limit-button\s*\{[^}]*align-self:start;/s)
  assert.match(source, /@container ui-page \(max-width:620px\)\s*\{[\s\S]*?\.editor-control-grid\s*\{\s*grid-template-columns:minmax\(0,1fr\);\s*\}/)
  assert.match(source, /@container ui-page \(max-width:620px\)\s*\{[\s\S]*?\.aligned-picker,\.limit-button\s*\{[^}]*margin-top:0;/)
  assert.match(source, /@container ui-page \(max-width:620px\)\s*\{[\s\S]*?\.limit-button\s*\{[^}]*width:100%;[^}]*min-width:0;/)
  assert.doesNotMatch(source, /\.editor-control-grid\s*\{[^}]*max-width:760px;/s)
  assert.doesNotMatch(source, /grid-template-columns:minmax\(240px,520px\) 124px 88px;/)
})

test('loadout workflow uses a responsive overview instead of three full-width bars', () => {
  const source = sources['SigilLoadoutRestore.vue']
  assert.match(source, /class="loadout-overview"/)
  assert.match(source, /\.loadout-overview\s*\{[^}]*grid-template-columns/is)
  assert.match(source, /@container\s+ui-page\s*\(max-width:680px\)\s*\{[\s\S]*?\.loadout-overview\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,1fr\)/is)
  assert.doesNotMatch(source, /@container\s+ui-page\s*\(max-width:760px\)\s*\{[\s\S]*?\.loadout-overview/)
  assert.match(source, /class="[^"]*entry-row[^"]*ui-row/)
  assert.doesNotMatch(source, /font-size\s*:\s*9\.5px/)
})

test('summon search exposes an explicit no-match state and one searchable editor surface', () => {
  const source = sources['SummonEditor.vue']
  assert.match(source, /v-if="!filteredSummons\.length"/)
  assert.match(source, /无匹配/)
  assert.match(source, /class="[^"]*workspace[^"]*ui-card-grid/)
  assert.doesNotMatch(source, /@container\s*\(max-width:680px\)[\s\S]*grid-template-columns:minmax\(180px/is)
})

test('summon editor exposes all encodable values and keeps natural rules advisory', () => {
  const source = sources['SummonEditor.vue']
  assert.match(source, /currentMainTraitIsLegacy/)
  assert.match(source, /当前值（非天然）/)
  assert.match(source, /mainHash\s*===\s*\(selected\.value\.mainTraitHash\s*>>>\s*0\)/)
  assert.match(source, /mainLevel\s*===\s*selected\.value\.mainTraitLevel/)
  assert.doesNotMatch(source, /function traitMax\([^)]*\)\s*\{[^}]*\|\|\s*999/)
  assert.match(source, /type="number" min="0" max="4294967295"/)
  assert.match(source, /allowedMainHashes/)
  assert.match(source, /naturalSubLevels/)
  assert.match(source, /天然词池、等级与种类对应关系只作提醒/)
  assert.doesNotMatch(source, /forceWrite/)
})

test('summon memory page exposes an explicit global-process disconnect boundary', () => {
  const source = sources['SummonEditor.vue']
  assert.match(source, /CharaAcquire/)
  assert.match(source, /CharaRelease/)
  assert.match(source, /async function disconnect\(/)
  assert.match(source, /@click="disconnect\(\)"[^>]*>\s*断开连接\s*</)
  assert.match(source, /onBeforeUnmount\([\s\S]*queueRuntimeLeaseRelease\([^;]*ownerToken[^;]*CharaRelease/i)
  assert.doesNotMatch(source, /Chara(?:Attach|Detach)/, 'summon editor must not use unconditional process lifecycle APIs')
  assert.doesNotMatch(source, /仍会按所选数值写入/)
})

test('runtime editors close asynchronous lifecycle races in the visible controls', () => {
  const loadout = sources['SigilLoadoutRestore.vue']
  assert.match(loadout, /createOperationGate/)
  assert.match(loadout, /freezeSigilLoadout/)
  assert.match(loadout, /writeSnapshot/)
  assert.match(loadout, /mode\.value\s*=\s*'starting-write'/)
  assert.match(loadout, /async function failRun\([^)]*\)[\s\S]*?await releaseRuntimeLease\([^;]*SigilMemoryRelease/)

  const sigil = sources['SigilMemoryGenerator.vue']
  assert.match(sigil, /if\s*\(loading\.value\s*\|\|\s*applying\.value\)\s*return/)
  assert.match(sigil, /if\s*\(!canWrite\.value\)/)
  assert.match(sigil, /primaryTraitHash/)
  assert.match(sigil, /queueRuntimeLeaseRelease\([^;]*SigilMemoryRelease/)
  assert.match(sigil, /window\.setInterval\(pollStatus,\s*700\)/)

  const summon = sources['SummonEditor.vue']
  assert.match(summon, /@click="disconnect\(\)"/)
  assert.match(summon, /if\s*\(loading\.value\s*\|\|\s*saving\.value\)\s*return/)
  assert.match(summon, /Rank 无法编码为 uint32/)
  assert.doesNotMatch(summon, /forceWrite/)
})

test('async option loading and generated bindings honor the owner lease contract', () => {
  const sigil = sources['SigilMemoryGenerator.vue']
  const loadOptionsStart = sigil.indexOf('async function loadOptions()')
  const loadOptionsEnd = sigil.indexOf('\nasync function refresh', loadOptionsStart)
  const loadOptionsBody = sigil.slice(loadOptionsStart, loadOptionsEnd)
  assert.match(loadOptionsBody, /await backendLanguageReady[\s\S]*if \(disposed/)
  assert.match(loadOptionsBody, /await SigilMemoryGetOptions\(\)[\s\S]*if \(disposed/)

  const bindings = readFileSync(new URL('../wailsjs/go/backend/App.js', import.meta.url), 'utf8')
  for (const method of ['SigilMemoryAcquire', 'SigilMemoryRelease', 'SigilMemoryUpdateOwned', 'CharaAcquire', 'CharaRelease', 'SummonUpdateOwned']) {
    assert.match(bindings, new RegExp(`export function ${method}\\(`), `${method} binding is missing`)
  }
})

test('overlimit exposes diagnostics on demand and lays four slots out as an adaptive grid', () => {
  const source = sources['OverLimit.vue']
  assert.match(source, /<details[^>]*class="[^"]*ui-disclosure[^"]*guide-disclosure/)
  assert.match(source, /<details[^>]*class="[^"]*ui-disclosure[^"]*diagnostics/)
  assert.match(source, /class="[^"]*slot-list[^"]*ui-card-grid/)
  assert.match(source, /\.slot-list\s*\{[^}]*--ui-grid-min\s*:\s*3\d{2}px/is)
})

test('monster safety copy is a full-width notice and raw patch bytes are disclosed on demand', () => {
  const source = sources['MonsterEnhance.vue']
  assert.match(source, /class="[^"]*usage-notice[^"]*ui-notice/)
  assert.match(source, /class="[^"]*card-grid[^"]*ui-card-grid/)
  assert.equal((source.match(/class="[^"]*usage-notice[^"]*ui-notice/g) || []).length, 1, 'monster page must render exactly one safety notice')
  assert.equal((source.match(/v-if="result\.items\.length" class="[^"]*card-grid[^"]*ui-card-grid/g) || []).length, 1, 'monster page must render exactly one result grid')
  assert.match(source, /<details[^>]*class="[^"]*ui-disclosure[^"]*diagnostics/)
  assert.doesNotMatch(source, /custom-note-card/)
})
