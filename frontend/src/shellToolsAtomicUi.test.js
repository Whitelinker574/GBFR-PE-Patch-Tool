import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = name => readFileSync(new URL(`./components/${name}.vue`, import.meta.url), 'utf8')

const patchTool = read('PatchTool')
const home = read('HomeJournal')
const language = read('LanguageSettings')
const backup = read('SaveBackupDrawer')
const dialog = read('ConfirmDialog')

const styleBlocks = source => [...source.matchAll(/<style\s+scoped>([\s\S]*?)<\/style>/g)].map(match => match[1])

test('the shell has one owned style layer and no hidden starfield or guide dead code', () => {
  assert.equal(styleBlocks(patchTool).length, 1)
  assert.doesNotMatch(patchTool, /StarfieldCanvas|guideOpen|guide-rail|guide-heading/)
})

test('the shell does not repaint child controls through deep important skinning', () => {
  const shellCss = styleBlocks(patchTool).join('\n')
  assert.doesNotMatch(shellCss, /!important/)
  for (const property of ['color', 'background', 'border', 'border-radius', 'box-shadow', 'font']) {
    assert.doesNotMatch(shellCss, new RegExp(`:deep\\([^)]*\\)\\s*\\{[^}]*${property}\\s*:`, 'is'))
  }
})

test('portrait is a fixed parchment background layer and loadout editing stays art free', () => {
  const shellCss = styleBlocks(patchTool).join('\n')
  assert.match(shellCss, /\.tool-stage::before\s*\{[^}]*position\s*:\s*fixed[^}]*background-image\s*:\s*var\(--function-art\)/is)
  assert.match(shellCss, /@media\s*\(max-width\s*:\s*900px\)[\s\S]*?\.tool-stage::before\s*,\s*\.art-toggle\s*,\s*\.art-caption\s*\{[^}]*display\s*:\s*none/is)
  assert.match(shellCss, /@media\s*\(max-width\s*:\s*1024px\)[\s\S]*?\.app-body\s*\{[^}]*grid-template-columns\s*:\s*70px\s+minmax\(0,\s*1fr\)/is)
  assert.doesNotMatch(patchTool, /<aside\s+v-if="!isLoadoutWorkspace"\s+class="art-rail"|class="character-blend"/)
  assert.match(shellCss, /\.tool-stage\.loadout-dedicated::before\s*\{[^}]*display\s*:\s*none/is)
})

test('inline compatibility and patch tools consume the shared primitives', () => {
  assert.match(patchTool, /class="compat-dashboard ui-page-stack"/)
  assert.match(patchTool, /class="calibration-card ui-card ui-stat"/)
  assert.match(patchTool, /class="legacy-patch ui-page-stack"/)
  assert.match(patchTool, /class="[^"]*\bpatch-file-row\b[^"]*\bui-card\b[^"]*"/)
  assert.match(patchTool, /class="patch-actions ui-actions"/)
  assert.doesNotMatch(patchTool, /calibration-grid\s*>\s*\.calibration-card:last-child/)
})

test('home and language use shared page, card, button and selection primitives', () => {
  assert.match(home, /class="[^"]*\bjournal-home\b[^"]*\bui-page\b[^"]*"/)
  assert.match(home, /class="chapter-ribbon ui-card"/)
  assert.match(home, /class="[^"]*ui-btn[^"]*"/)
  assert.doesNotMatch(home, /safety-note/)

  assert.match(language, /class="language-panel ui-page-stack"/)
  assert.match(language, /class="language-card ui-card"/)
  assert.match(language, /class="language-button ui-btn/)
  assert.match(language, /:aria-pressed="isActive/)
})

test('backup flyout and confirmation dialog expose shared controls and overlay semantics', () => {
  assert.match(backup, /class="protection-trigger ui-btn/)
  assert.match(backup, /:aria-expanded="open"/)
  assert.match(backup, /aria-controls="save-backup-flyout"/)
  assert.match(backup, /id="save-backup-flyout"/)
  assert.match(backup, /class="backup-drawer ui-card"/)
  assert.match(backup, /class="manual-backup ui-btn is-primary"/)

  assert.match(dialog, /class="journal-dialog ui-card"/)
  assert.match(dialog, /class="cancel ui-btn"/)
  assert.match(dialog, /class="confirm ui-btn is-primary"/)
  assert.match(dialog, /max-height\s*:\s*calc\(100dvh\s*-\s*32px\)/)
})

test('owned page styles keep supporting copy readable and cover the minimum window height', () => {
  for (const source of [patchTool, home, language, backup, dialog]) {
    const css = styleBlocks(source).join('\n')
    assert.doesNotMatch(css, /font-size\s*:\s*(?:8|9|10)(?:\.\d+)?px/i)
    assert.doesNotMatch(css, /font-size\s*:\s*0?\.(?:5|6)\d*rem/i)
  }
  assert.match(patchTool, /@media\s*\(max-height\s*:\s*620px\)/i)
  assert.match(backup, /@media\s*\(max-width\s*:\s*520px\)/i)
  assert.match(dialog, /@media\s*\(max-width\s*:\s*520px\)/i)
})
