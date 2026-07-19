import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const patchTool = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
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

test('tool shell uses one bounded portrait column system at the real viewport breakpoints', () => {
  assert.match(patchTool, /\.tool-stage\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,\s*1fr\)\s+clamp\(220px,\s*18vw,\s*300px\)/is)
  assert.match(patchTool, /@media\s*\(max-width\s*:\s*1279px\)\s*\{[\s\S]*?\.tool-stage\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,\s*1fr\)/i)
  assert.match(patchTool, /@media\s*\(max-width\s*:\s*1279px\)\s*\{[\s\S]*?\.art-rail\s*,\s*\.art-toggle\s*\{[^}]*display\s*:\s*none\s*;/is)
  assert.doesNotMatch(patchTool, /\.art-rail\s*,\s*\.art-toggle\s*\{[^}]*display\s*:\s*none\s*!important/is)
  assert.doesNotMatch(patchTool, /grid-template-columns\s*:\s*168px\s+minmax\(0,\s*1fr\)/i)
})

test('portrait images are clipped and contained by the art rail', () => {
  assert.match(patchTool, /\.art-rail\s*\{[^}]*overflow\s*:\s*hidden/is)
  assert.match(patchTool, /\.art-rail\s+\.function-character\s*\{[^}]*inset\s*:\s*0[^}]*overflow\s*:\s*hidden/is)
  assert.match(patchTool, /\.art-rail\s+\.function-character\s+img\s*\{[^}]*width\s*:\s*100%[^}]*height\s*:\s*100%[^}]*object-fit\s*:\s*contain/is)
  assert.doesNotMatch(patchTool, /--ah\s*:\s*160%|--ax\s*:\s*-250px|max-width\s*:\s*none\s*!important;\s*max-height\s*:\s*none/i)
})

test('compact desktop widths use an icon sidebar and do not reserve a ghost portrait column', () => {
  assert.match(patchTool, /@media\s*\(max-width\s*:\s*1024px\)\s*\{[\s\S]*?\.app-body\s*\{[^}]*grid-template-columns\s*:\s*70px\s+minmax\(0,\s*1fr\)/i)
  assert.doesNotMatch(patchTool, /@media\s*\(max-width\s*:\s*900px\)[\s\S]{0,160}?\.tool-stage\s*\{[^}]*168px/i)
})

test('mid desktop keeps the portrait rail without making the center narrower than compact desktop', () => {
  assert.match(patchTool, /<div class="app-body"\s+:class="\{[^\"]*'art-visible'\s*:/s)
  assert.match(patchTool, /@media\s*\(min-width\s*:\s*1280px\)\s+and\s+\(max-width\s*:\s*1399px\)\s*\{[\s\S]*?\.app-body\s*\{[^}]*grid-template-columns\s*:\s*70px\s+minmax\(0,\s*1fr\)/i)
  assert.match(patchTool, /@media\s*\(min-width\s*:\s*1280px\)[\s\S]*?\.app-body\.art-visible\s+\.sidebar-mascot\s*\{[^}]*display\s*:\s*none/i)
})

test('page heading and panel share a dense or narrow reading measure', () => {
  assert.match(patchTool, /\.tool-center-scroll\s*\{[^}]*--tool-measure\s*:\s*840px/is)
  assert.match(patchTool, /\.tool-stage:is\([^}]+\)\s+\.tool-center-scroll\s*\{[^}]*--tool-measure\s*:\s*960px/is)
  assert.match(patchTool, /\.tool-page-heading\s*,\s*\.tool-panel\s*\{[^}]*width\s*:\s*min\(100%,\s*var\(--tool-measure\)\)[^}]*margin-inline\s*:\s*auto/is)
  assert.doesNotMatch(patchTool, /\.tool-panel\s*:\s*deep\(\.root\)[^{]*\{[^}]*max-width\s*:\s*none/is)
})

test('experimental runtime pages are discoverable and active in the memory switcher', () => {
  assert.match(patchTool, /id:\s*'memory'[\s\S]*?items:\s*\[[^\]]*'legacyRuntime'[^\]]*'monster'[^\]]*\]/)
  assert.match(patchTool, /:class="\{ active: activeTab === id,/)
})

test('memory tools reflow into a visible tab grid before any item leaves the viewport', () => {
  assert.match(patchTool, /class="tool-switcher ui-tabs"[^>]*:data-group="activeGroup\.id"/)
  assert.match(patchTool, /@media\s*\(max-width\s*:\s*1439px\)[\s\S]*?\.tool-switcher\[data-group="memory"\]\s*\{[^}]*display\s*:\s*grid[^}]*grid-template-columns\s*:\s*repeat\(auto-fit,\s*minmax\(180px,\s*1fr\)\)[^}]*overflow\s*:\s*visible/is)
  assert.match(patchTool, /\.tool-switcher\[data-group="memory"\]\s+\.ui-tab\s*\{[^}]*min-width\s*:\s*0[^}]*white-space\s*:\s*normal/is)
})
