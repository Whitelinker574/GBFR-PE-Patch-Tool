import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const patchTool = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const homeJournal = readFileSync(new URL('./components/HomeJournal.vue', import.meta.url), 'utf8')
const wrightstoneMemory = readFileSync(new URL('./components/WrightstoneMemoryGenerator.vue', import.meta.url), 'utf8')
const uiScaleSource = readFileSync(new URL('./utils/uiScale.js', import.meta.url), 'utf8')
const appGo = readFileSync(new URL('../../internal/backend/app.go', import.meta.url), 'utf8')
const globalStyle = readFileSync(new URL('./style.css', import.meta.url), 'utf8')

function navigationIds(source, groupId) {
  const match = source.match(new RegExp(`\\{ id: '${groupId}',[^\\n]*items: \\[([^\\]]*)\\] \\}`))
  assert.ok(match, `missing ${groupId} navigation group`)
  return [...match[1].matchAll(/'([^']+)'/g)].map(item => item[1])
}

function homeEntryIds(groupId) {
  const groupStart = homeJournal.indexOf(`id: '${groupId}'`)
  const itemsStart = homeJournal.indexOf('items: [', groupStart)
  const itemsEnd = homeJournal.indexOf('\n    ],', itemsStart)
  assert.ok(groupStart >= 0 && itemsStart >= 0 && itemsEnd >= 0, `missing ${groupId} home group`)
  return [...homeJournal.slice(itemsStart, itemsEnd).matchAll(/\{ id: '([^']+)'/g)].map(item => item[1])
}

test('ui scale keeps the app at the real viewport scale without inverse size compensation', async () => {
  const declarations = new Map([
    ['--ui-zoom', '0.66'],
    ['--ui-scale-inverse', '1.515'],
  ])
  const app = {
    style: {
      zoom: '0.66',
      width: '1455px',
      height: '1061px',
      setProperty(name, value) { declarations.set(name, value) },
      removeProperty(name) {
        declarations.delete(name)
        if (name === 'width' || name === 'height') this[name] = ''
      },
    },
  }
  const listeners = new Map()
  globalThis.window = {
    innerWidth: 960,
    innerHeight: 700,
    addEventListener(type, listener) { listeners.set(type, listener) },
  }
  globalThis.document = { getElementById: id => id === 'app' ? app : null }
  globalThis.requestAnimationFrame = callback => { callback(); return 1 }

  const { installUiScale } = await import(`./utils/uiScale.js?one-to-one=${Date.now()}`)
  installUiScale()

  assert.equal(app.style.zoom, '1')
  assert.equal(app.style.width, '')
  assert.equal(app.style.height, '')
  assert.equal(declarations.get('--ui-zoom'), '1')
  assert.equal(declarations.get('--ui-scale-inverse'), '1')
  assert.equal(typeof listeners.get('resize'), 'function')
  assert.doesNotMatch(uiScaleSource, /MIN_ZOOM|computeZoom|\b1\s*\/\s*z\b|\bw\s*\/\s*z\b|\bh\s*\/\s*z\b/)
})

test('application scrollbars use a transparent parchment track and brass thumb', () => {
  assert.match(globalStyle, /scrollbar-color:\s*rgba\(126,\s*89,\s*40,\s*\.42\)\s+transparent/)
  assert.match(globalStyle, /\*::-webkit-scrollbar-track\s*\{\s*background:\s*transparent;\s*\}/)
  assert.match(globalStyle, /\*::-webkit-scrollbar-thumb\s*\{[^}]*background:\s*rgba\(126,\s*89,\s*40,\s*\.42\)/is)
  assert.match(globalStyle, /\*::-webkit-scrollbar-thumb:hover\s*\{\s*background-color:\s*rgba\(110,\s*78,\s*40,\s*\.62\)/)
  assert.doesNotMatch(globalStyle, /scrollbar-thumb:hover[^}]*58,\s*169,\s*179/is)
})

test('frameless titlebar keeps only minimise, maximise or restore, and close on the right', () => {
  assert.match(patchTool, /WindowToggleMaximise/)
  assert.match(patchTool, /@dblclick\.self="WindowToggleMaximise"/)
  for (const label of ['最小化', '最大化或还原', '关闭']) {
    assert.match(patchTool, new RegExp(`aria-label="${label}"`))
  }
  assert.doesNotMatch(patchTool, /WindowFullscreen|WindowUnfullscreen|WindowIsFullscreen|toggleFullscreen|一键全屏|fullscreen-corners/)
  assert.match(patchTool, /\.titlebar-controls\s*\{[^}]*position\s*:\s*absolute[^}]*right\s*:\s*0/is)
  assert.match(patchTool, /--window-controls-width\s*:\s*126px/)
  assert.match(patchTool, /\.titlebar-status\s*\{[^}]*max-width\s*:\s*min\([^;]*calc\(100%\s*-\s*var\(--window-controls-width\)/is)
  assert.match(patchTool, /\.titlebar-status\s*\{[^}]*position\s*:\s*absolute[^}]*left\s*:\s*50%/is)
})

test('home scene owns a definite full-height chain and top-aligns before its heading can be clipped', () => {
  assert.match(patchTool, /\.home-mode \.workspace-scroll\s*\{[^}]*padding\s*:\s*0[^}]*overflow\s*:\s*auto[^}]*scrollbar-gutter\s*:\s*auto/is)
  assert.match(patchTool, /\.home-mode \.workspace-scene\s*\{[^}]*height\s*:\s*100%[^}]*min-height\s*:\s*100%/is)
  assert.match(homeJournal, /\.journal-home\s*\{[^}]*display:flex;[^}]*flex-direction:column;/is)
  assert.match(homeJournal, /\.illustrated-journal\s*\{[^}]*flex:1 0 auto;/is)
  assert.match(homeJournal, /@media \(max-height:920px\) and \(min-width:761px\)\s*\{[\s\S]*?\.journal-home\s*\{[^}]*height:auto;[^}]*min-height:100%;[^}]*\}[\s\S]*?\.illustrated-journal\s*\{[^}]*height:auto;[^}]*min-height:500px;[^}]*\}[\s\S]*?\.page-menu\s*\{[^}]*height:auto;[^}]*justify-content:flex-start;/is)
})

test('remembered directory collapse never narrows the hidden home or loadout shell', () => {
  assert.match(patchTool, /\.app-body\.home-mode\.sidebar-collapsed,\s*\.app-body\.loadout-workspace\.sidebar-collapsed\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,1fr\)/is)
})

test('the directory keeps its older layered paper treatment distinct from the workspace', () => {
  assert.match(patchTool, /\.sidebar\s*\{[^}]*border-right\s*:\s*1px solid rgba\(130,96,48,\.3\)[^}]*background\s*:\s*#f0e2c2[^}]*box-shadow\s*:\s*8px 0 28px rgba\(90,66,31,\.12\)/is)
})

test('the directory restores the original upper-left journal ornament', () => {
  assert.match(patchTool, /\.sidebar::before\s*\{[^}]*left\s*:\s*-7px[^}]*top\s*:\s*-4px[^}]*width\s*:\s*112px[^}]*height\s*:\s*96px[^}]*background\s*:\s*url\('\.\.\/assets\/gbfr\/journal-page-corner\.svg'\)\s+left top\s*\/\s*contain no-repeat[^}]*opacity\s*:\s*\.46/is)
  assert.match(patchTool, /\.sidebar-heading\s*,\s*\.sidebar-home-compact\s*,\s*\.primary-nav\s*,\s*\.sidebar-mascot\s*,\s*\.sidebar-foot\s*\{[^}]*position\s*:\s*relative[^}]*z-index\s*:\s*1/is)
})

test('window chrome, flat tabs, page heading and portrait caption use the legacy warm paper hierarchy instead of white cards', () => {
  assert.match(patchTool, /\.titlebar\s*\{[^}]*background\s*:\s*linear-gradient\(90deg,#594937,#756044 52%,#5b4a37\)/is)
  assert.match(patchTool, /\.workspace-bar\s*\{[^}]*background\s*:\s*#ead8b2/is)
  assert.match(patchTool, /\.tool-switcher\s*\{[^}]*background\s*:\s*#eddfc0/is)
  assert.match(patchTool, /\.tool-switcher \.ui-tab\.active\s*\{[^}]*background\s*:\s*transparent[^}]*border-bottom-color\s*:\s*#9a7440[^}]*box-shadow\s*:\s*none/is)
  assert.match(patchTool, /\.tool-page-heading\s*\{[^}]*background\s*:\s*#f7ebcf/is)
  assert.match(patchTool, /\.art-caption\s*\{[^}]*background\s*:\s*#f4e6c7/is)
})

test('top tool tabs use the bold label weight requested for quick scanning', () => {
  assert.match(patchTool, /\.tool-switcher \.ui-tab\s*\{[^}]*font-weight\s*:\s*var\(--fw-bold\)/is)
})

test('sidebar and top-tab groups put common functions first in an exact stable order', () => {
  assert.deepEqual(navigationIds(patchTool, 'save'), ['loadoutPresets', 'sigil', 'wrightstone', 'summonSave', 'progression', 'chara', 'save', 'saveDiff'])
  assert.deepEqual(navigationIds(patchTool, 'memory'), ['sigilMemory', 'wrightstoneMemory', 'summon', 'overlimit', 'runtime'])
  assert.deepEqual(navigationIds(patchTool, 'loadoutFlow'), ['runtimeMonitor', 'loadout'])
  assert.deepEqual(navigationIds(patchTool, 'runtimeTools'), ['runtimeQOL', 'virtualSigils', 'audioMixer', 'camera', 'spatialTools', 'patchCombat', 'patchCharacters', 'patchQuest', 'monster'])
  assert.deepEqual(navigationIds(patchTool, 'tools'), ['naturalDrop', 'selectedItemMonitor', 'formulaSampler', 'compatibility', 'patch', 'language'])
  assert.doesNotMatch(patchTool, /\{ id: 'monitor',/)
  assert.match(patchTool, /@pointerenter="queueWarmToolIntent\(id\)"/)
  assert.match(patchTool, /@pointerleave="cancelWarmToolIntent\(id\)"/)
  assert.match(patchTool, /@pointerdown="queueWarmTool\(id\)"/)
  assert.match(patchTool, /await waitForTool\(id\)/)
  assert.match(patchTool, /navigationError\.value = \{ id, message:/)
  assert.match(patchTool, /await afterNextPaint\(\)/)
  assert.match(patchTool, /warmImage\(functionArt\[id\]\)/)
  assert.match(patchTool, /warmImage\(functionStickers\[id\]\)/)
  assert.doesNotMatch(patchTool, /queueWarmTools\(Object\.keys\(functionArt\)\)/)
})

test('every top tab has one explicit work-mode badge independent from maturity status', () => {
  const expectedModes = {
    progression: 'offline', sigil: 'offline', loadoutPresets: 'offline', wrightstone: 'offline',
    summonSave: 'offline', chara: 'offline', save: 'offline',
    sigilMemory: 'live', wrightstoneMemory: 'live', summon: 'live', overlimit: 'live', runtime: 'live',
    runtimeMonitor: 'background', loadout: 'live',
    runtimeQOL: 'live', virtualSigils: 'live', audioMixer: 'live', camera: 'live', spatialTools: 'live',
    patchCombat: 'live', patchCharacters: 'live', patchQuest: 'live', monster: 'live',
    naturalDrop: 'file', saveDiff: 'offline', selectedItemMonitor: 'live', formulaSampler: 'live',
    compatibility: 'local', patch: 'file', language: 'local',
  }
  for (const [id, mode] of Object.entries(expectedModes)) {
    assert.match(patchTool, new RegExp(`\\b${id}: '${mode}'`), `${id} mode`)
  }
  assert.match(patchTool, /v-if="toolNavigationMode\(id\)"\s+:class="\['switcher-tag', toolNavigationMode\(id\)\]"/)
  for (const label of ['离线', '实时', '后台', '游戏文件', '只读', '本机']) {
    assert.match(patchTool, new RegExp(`'${label}'`))
  }
  assert.match(patchTool, /\.switcher-tag\.background/)
  assert.match(patchTool, /\.switcher-tag\.file/)
  assert.match(patchTool, /\.switcher-tag\.readonly/)
  assert.match(patchTool, /\.switcher-tag\.local/)
})

test('home journal mirrors the common-first entry order and exposes live blessing and loadout editors', () => {
  assert.deepEqual(homeEntryIds('save'), ['loadoutPresets', 'sigil', 'wrightstone', 'summonSave', 'progression', 'saveDiff'])
  assert.deepEqual(homeEntryIds('memory'), ['sigilMemory', 'wrightstoneMemory', 'summon', 'overlimit', 'runtime'])
  assert.deepEqual(homeEntryIds('loadoutFlow'), ['runtimeMonitor', 'loadout'])
  assert.deepEqual(homeEntryIds('runtimeTools'), ['runtimeQOL', 'virtualSigils', 'audioMixer', 'camera', 'spatialTools', 'patchCombat', 'patchCharacters', 'patchQuest', 'monster'])
  assert.doesNotMatch(homeJournal, /id: 'monitor'/)
})

test('user-facing page titles omit historical source-version suffixes', () => {
  assert.match(patchTool, /runtimeMonitor:\s*\{[\s\S]*?title:\s*'队友配装持续检测'[\s\S]*?eyebrow:\s*'队伍配装 · 后台服务'/)
  assert.match(patchTool, /spatialTools:\s*\{[\s\S]*?title:\s*'坐标与移动工具'[\s\S]*?eyebrow:\s*'单机空间操作'/)
  assert.match(patchTool, /patchCombat:\s*\{[\s\S]*?eyebrow:\s*'战斗补丁'/)
  assert.match(patchTool, /patchCharacters:\s*\{[\s\S]*?eyebrow:\s*'角色机制'/)
  assert.match(patchTool, /patchQuest:\s*\{[\s\S]*?eyebrow:\s*'任务与便利'/)
  assert.match(patchTool, /baselineVersion:\s*'游戏 2\.0\.3（静态与离线）'/)
  assert.doesNotMatch(homeJournal, /运行监测（[^）]*\d+\.\d+\.\d+[^）]*）/)
  assert.match(appGo, /appVersion\s*=\s*"v2\.0\.5"/)
  assert.doesNotMatch(appGo, /appVersion\s*=\s*"[^"]*-(?:patch|preview)\d+"/i)
})

test('the workspace uses the character-free ornamental parchment as its only scene background', () => {
  assert.match(patchTool, /parchment-ui-v2\.webp/)
  assert.match(patchTool, /\.app-window::before\s*\{[^}]*background-image\s*:\s*linear-gradient\([^}]*url\('\.\.\/assets\/gbfr\/parchment-ui-v2\.webp'\)[^}]*filter\s*:\s*saturate\(\.92\) contrast\(\.98\)/is)
  assert.match(patchTool, /\.workspace\s*\{[^}]*background\s*:\s*linear-gradient\([^}]*url\('\.\.\/assets\/gbfr\/parchment-ui-v2\.webp'\)\s+center\s*\/\s*cover\s+fixed/is)
  assert.doesNotMatch(patchTool, /journal-scene-4k\.webp/)
})

test('ordinary tool pages reserve a fluid left panel without making portrait placement part of the grid', () => {
  assert.match(patchTool, /\.tool-center-scroll\s*\{[^}]*width\s*:\s*62%/is)
  assert.match(patchTool, /\.tool-page-heading\s*,\s*\.tool-panel\s*\{[^}]*width\s*:\s*100%[^}]*max-width\s*:\s*none/is)
  assert.match(patchTool, /\.tool-stage\.art-collapsed \.tool-center-scroll\s*,\s*\.tool-stage\.loadout-dedicated \.tool-center-scroll\s*\{[^}]*width\s*:\s*100%/is)
  assert.doesNotMatch(patchTool, /\.tool-stage\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,\s*62fr\)/is)
})

test('portrait is a fixed top-anchored right background with stable per-page optical calibration', () => {
  assert.match(patchTool, /class="tool-stage"[^>]*:style="\{ '--function-art': `url\('\$\{currentArt\}'\)` \}"/)
  assert.match(patchTool, /\.tool-stage\[data-tool="progression"\]\s*\{[^}]*--art-scale\s*:/is)
  assert.match(patchTool, /\.tool-stage::before\s*\{[^}]*position\s*:\s*fixed[^}]*background-image\s*:\s*var\(--function-art\)[^}]*background-position\s*:\s*right var\(--art-x\) top var\(--art-y\)[^}]*background-size\s*:\s*auto var\(--art-scale\)/is)
  assert.match(patchTool, /\.tool-stage\[data-tool="formulaSampler"\][^\{]*\{[^}]*--art-x\s*:[^;}]+;[^}]*--art-y\s*:/is)
  assert.doesNotMatch(patchTool, /--art-right|--art-bottom|--art-position-y|background-position\s*:\s*right bottom/)
  assert.doesNotMatch(patchTool, /class="character-blend"|class="art-rail"|\.art-rail::before/)
})

test('compact navigation always retains a real home control and the Q sticker', () => {
  assert.match(patchTool, /class="sidebar-home-compact"[^>]*aria-label="返回功能首页"/)
  assert.match(patchTool, /class="tool-switcher-collapse"[^>]*aria-label="[^"]*目录"/)
  assert.doesNotMatch(patchTool, /<aside class="sidebar">[\s\S]*?class="sidebar-collapse"/)
  assert.match(patchTool, /v-for="id in activeGroup\.items"/)
  assert.match(patchTool, /\.sidebar-collapsed \.sidebar-home-compact\s*\{[^}]*display\s*:\s*grid/is)
  assert.doesNotMatch(patchTool, /@media\s*\(max-width\s*:\s*1024px\)[\s\S]*?\.app-body\s*\{[^}]*grid-template-columns\s*:\s*70px/is)
  assert.doesNotMatch(patchTool, /\.app-body\.art-visible \.sidebar-mascot\s*\{[^}]*display\s*:\s*none/is)
  assert.match(patchTool, /@media\s*\(max-height\s*:\s*620px\)[\s\S]*?\.sidebar-mascot-say\s*\{[^}]*display\s*:\s*none/is)
  assert.doesNotMatch(patchTool, /@media\s*\(max-height\s*:\s*620px\)[\s\S]*?\.sidebar-mascot\s*\{[^}]*display\s*:\s*none/is)
})

test('wrightstone live selection guidance is a compact inline notice, not a centered empty panel', () => {
  assert.match(wrightstoneMemory, /class="selection-inline-notice"/)
  assert.doesNotMatch(wrightstoneMemory, /启用读取后，在游戏内祝福石列表选中目标记录。[^<]*<\/p>/)
  assert.match(wrightstoneMemory, /\.selection-inline-notice\s*\{[^}]*min-height\s*:\s*0[^}]*text-align\s*:\s*left/is)
})

test('the obsolete experimental runtime page is removed while the verified monster page remains reachable', () => {
  assert.doesNotMatch(patchTool, /legacyRuntime|实验性实时功能|待适配运行时功能|兼容性实验室/)
  assert.doesNotMatch(patchTool, /<MiscTools[^>]*\bmode=/)
  assert.match(patchTool, /id:\s*'runtimeTools'[\s\S]*?items:\s*\[[^\]]*'monster'[^\]]*\]/)
  assert.match(patchTool, /monster:\s*\{[\s\S]*?group:\s*'runtimeTools'[\s\S]*?status:\s*'实验'[\s\S]*?tone:\s*'live'/)
  assert.match(patchTool, /:class="\{ active: activeTab === id,/)
})

test('large tool groups expose every function in one flat horizontally scrollable row without a hidden More menu', () => {
  assert.match(patchTool, /class="tool-switcher-shell"[^>]*:data-group="activeGroup\.id"/)
  assert.match(patchTool, /class="tool-switcher-collapse"[\s\S]*?<nav ref="toolSwitcher"/)
  assert.match(patchTool, /ref="toolSwitcher" class="tool-switcher ui-tabs"[^>]*@wheel="scrollToolSwitcher"/)
  assert.match(patchTool, /v-for="id in activeGroup\.items"/)
  assert.match(patchTool, /function scrollToolSwitcher\(event\)/)
  assert.match(patchTool, /event\.currentTarget\.scrollLeft \+= event\.deltaY/)
  assert.match(patchTool, /\.tool-switcher\s*\{[^}]*display\s*:\s*flex[^}]*flex-wrap\s*:\s*nowrap[^}]*overflow-x\s*:\s*auto[^}]*overflow-y\s*:\s*hidden/is)
  assert.match(patchTool, /\.tool-switcher \.ui-tab\s*\{[^}]*flex\s*:\s*0 0 auto[^}]*border-radius\s*:\s*0[^}]*background\s*:\s*transparent/is)
  assert.doesNotMatch(patchTool, /更多|moreMenu|switcher-more|visibleSwitcherItems|hiddenSwitcherItems/)
})
