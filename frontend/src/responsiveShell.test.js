import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const patchTool = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const wrightstoneMemory = readFileSync(new URL('./components/WrightstoneMemoryGenerator.vue', import.meta.url), 'utf8')
const uiScaleSource = readFileSync(new URL('./utils/uiScale.js', import.meta.url), 'utf8')

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

test('frameless titlebar keeps controls on the right and exposes maximise plus fullscreen', () => {
  assert.match(patchTool, /WindowToggleMaximise/)
  assert.match(patchTool, /WindowFullscreen/)
  assert.match(patchTool, /@dblclick\.self="WindowToggleMaximise"/)
  for (const label of ['最小化', '最大化或还原', '一键全屏', '关闭']) {
    assert.match(patchTool, new RegExp(`aria-label="${label}"`))
  }
  assert.match(patchTool, /\.titlebar-controls\s*\{[^}]*position\s*:\s*absolute[^}]*right\s*:\s*0/is)
  assert.match(patchTool, /--window-controls-width\s*:\s*168px/)
  assert.match(patchTool, /\.titlebar-status\s*\{[^}]*max-width\s*:\s*min\([^;]*calc\(100%\s*-\s*var\(--window-controls-width\)/is)
  assert.match(patchTool, /\.titlebar-status\s*\{[^}]*position\s*:\s*absolute[^}]*left\s*:\s*50%/is)
})

test('the branded parchment skin remains visible behind translucent work surfaces', () => {
  assert.match(patchTool, /parchment-ui-v2\.webp/)
  assert.match(patchTool, /journal-scene-4k\.webp/)
  assert.match(patchTool, /\.app-window::before\s*\{[^}]*background-image/is)
  assert.match(patchTool, /\.workspace\s*\{[^}]*background\s*:[^}]*rgba\([^)]*,\s*\.[0-9]+\)[^}]*journal-scene-4k\.webp/is)
})

test('ordinary tool pages preserve the fluid left-panel and large right-art dual column', () => {
  assert.match(patchTool, /\.tool-stage\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,\s*62fr\)\s+minmax\(260px,\s*38fr\)/is)
  assert.match(patchTool, /\.tool-page-heading\s*,\s*\.tool-panel\s*\{[^}]*width\s*:\s*100%[^}]*max-width\s*:\s*none/is)
  assert.doesNotMatch(patchTool, /--tool-measure\s*:/)
  assert.doesNotMatch(patchTool, /width\s*:\s*min\(100%,\s*var\(--tool-measure\)\)/)
  assert.match(patchTool, /\.tool-stage\.loadout-dedicated\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,\s*1fr\)/is)
})

test('portrait is an unframed anchored scene layer with per-page optical calibration', () => {
  assert.match(patchTool, /\.tool-stage\[data-tool="progression"\]\s*\{[^}]*--art-scale\s*:/is)
  assert.match(patchTool, /\.art-rail\s*\{[^}]*overflow\s*:\s*visible[^}]*border\s*:\s*0[^}]*background\s*:\s*transparent/is)
  assert.match(patchTool, /\.art-rail\s*\{[^}]*height\s*:\s*clamp\(420px,\s*calc\(100dvh\s*-\s*166px\),\s*1400px\)/is)
  assert.match(patchTool, /\.art-rail \.function-character img\s*\{[^}]*right\s*:\s*var\(--art-x\)[^}]*bottom\s*:\s*var\(--art-y\)[^}]*width\s*:\s*var\(--art-scale\)/is)
  assert.doesNotMatch(patchTool, /\.art-rail \.function-character img\s*\{[^}]*object-fit\s*:\s*contain/is)
})

test('compact navigation always retains a real home control and the Q sticker', () => {
  assert.match(patchTool, /class="sidebar-home-compact"[^>]*aria-label="返回功能首页"/)
  assert.match(patchTool, /@media\s*\(max-width\s*:\s*1024px\)[\s\S]*?\.sidebar-home-compact\s*\{[^}]*display\s*:\s*grid/is)
  assert.doesNotMatch(patchTool, /\.app-body\.art-visible \.sidebar-mascot\s*\{[^}]*display\s*:\s*none/is)
  assert.match(patchTool, /@media\s*\(max-height\s*:\s*620px\)[\s\S]*?\.sidebar-mascot-say\s*\{[^}]*display\s*:\s*none/is)
  assert.doesNotMatch(patchTool, /@media\s*\(max-height\s*:\s*620px\)[\s\S]*?\.sidebar-mascot\s*\{[^}]*display\s*:\s*none/is)
})

test('wrightstone live selection guidance is a compact inline notice, not a centered empty panel', () => {
  assert.match(wrightstoneMemory, /class="selection-inline-notice"/)
  assert.doesNotMatch(wrightstoneMemory, /启用读取后，在游戏内祝福石列表选中目标记录。[^<]*<\/p>/)
  assert.match(wrightstoneMemory, /\.selection-inline-notice\s*\{[^}]*min-height\s*:\s*0[^}]*text-align\s*:\s*left/is)
})

test('experimental runtime pages are discoverable and active in the memory switcher', () => {
  assert.match(patchTool, /id:\s*'memory'[\s\S]*?items:\s*\[[^\]]*'legacyRuntime'[^\]]*'monster'[^\]]*\]/)
  assert.match(patchTool, /:class="\{ active: activeTab === id,/)
})

test('memory tools reflow into a visible tab grid before any item leaves the viewport', () => {
  assert.match(patchTool, /class="tool-switcher ui-tabs"[^>]*:data-group="activeGroup\.id"/)
  assert.match(patchTool, /@media\s*\(max-width\s*:\s*1439px\)[\s\S]*?\.tool-switcher\[data-group="memory"\]\s*\{[^}]*display\s*:\s*grid[^}]*grid-template-columns\s*:\s*repeat\(auto-fit,\s*minmax\(180px,\s*1fr\)\)[^}]*overflow\s*:\s*visible/is)
  assert.match(patchTool, /\.tool-switcher\[data-group="memory"\] \.ui-tab\s*\{[^}]*min-width\s*:\s*0[^}]*white-space\s*:\s*normal/is)
})
