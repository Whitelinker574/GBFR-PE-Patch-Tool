import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')
const editor = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')
const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('loadout viewer consumes the shared page, card, control and empty-state primitives', () => {
  assert.match(viewer, /class="loadout-viewer ui-page is-wide ui-page-stack"/)
  assert.match(viewer, /class="section ui-card ui-panel"/)
  assert.match(viewer, /class="action ui-btn/)
  assert.match(viewer, /class="chara-chip ui-chip"/)
  assert.match(viewer, /class="empty ui-empty"/)
})

test('character and preset grids reflow by available component width', () => {
  assert.match(viewer, /\.loadout-viewer\s*\{[^}]*container\s*:\s*loadout-viewer\s*\/\s*inline-size/is)
  assert.match(viewer, /\.chara-row\s*\{[^}]*grid-template-columns\s*:\s*repeat\(auto-fit,\s*minmax\(156px,\s*1fr\)\)/is)
  assert.match(viewer, /\.chara-chip-name\s*\{[^}]*min-width\s*:\s*0[^}]*text-overflow\s*:\s*ellipsis[^}]*white-space\s*:\s*nowrap/is)
  assert.match(viewer, /\.loadout-card-toggle\s*\{[^}]*grid-template-columns\s*:\s*62px\s+auto\s+minmax\(0,1fr\)\s+minmax\(120px,/is)
  assert.match(viewer, /@container\s+loadout-viewer\s*\(max-width\s*:\s*760px\)/i)
})

test('loadout editor uses its own container rather than viewport breakpoints', () => {
  assert.match(editor, /class="loadout-editor ui-page is-fluid"/)
  assert.match(editor, /\.loadout-editor\s*\{[^}]*container\s*:\s*loadout-editor\s*\/\s*inline-size/is)
  assert.match(editor, /@container\s+loadout-editor\s*\(max-width\s*:\s*1199px\)/i)
  assert.match(editor, /@container\s+loadout-editor\s*\(max-width\s*:\s*760px\)/i)
  assert.doesNotMatch(editor, /@media\s*\(max-width\s*:\s*(?:1279|900|650)px\)/i)
})

test('loadout editor consumes shared controls without flattening domain-specific factor and mastery cards', () => {
  assert.match(editor, /class="ed-select ui-select"/)
  assert.match(editor, /class="ed-input ui-input"/)
  assert.match(editor, /class="factor-mode-tabs ui-seg"/)
  assert.match(editor, /class="bag-toolbar ui-toolbar"/)
  assert.match(editor, /class="result-card[^\"]*ui-card ui-panel/)
  assert.match(editor, /\.skill-grid\s*\{[^}]*repeat\(auto-fit,\s*minmax\(116px,\s*1fr\)\)/is)
})

test('full-screen loadout editing keeps one reachable scroll owner at compact desktop widths', () => {
  assert.match(viewer, /\.editor-workspace-content\s*\{[^}]*overflow\s*:\s*auto[^}]*scrollbar-gutter\s*:\s*stable/is)
  assert.match(editor, /@container\s+loadout-editor\s*\(max-width\s*:\s*1199px\)\s*\{[\s\S]*?\.loadout-editor\s*,\s*\.editor-layout\s*\{[^}]*height\s*:\s*auto/is)
  assert.match(shell, /\.loadout-workspace\s+\.workspace-scroll\s*\{[^}]*overflow\s*:\s*hidden/is)
  assert.match(shell, /\.loadout-workspace\s+\.tool-center-scroll\s*\{[^}]*overflow\s*:\s*hidden/is)
})

test('dense editor details stay readable and the narrow identity header cannot overlap', () => {
  assert.doesNotMatch(editor, /font-size\s*:\s*(?:calc\((?:8|9|10)(?:\.\d+)?px\s*\*\s*var\(--editor-scale\)\)|\.(?:[0-7]\d*)rem)/i)
  assert.match(editor, /\.result-metrics\s*\{[^}]*grid-template-columns\s*:\s*repeat\(2,\s*minmax\(0,1fr\)\)/is)
  assert.match(editor, /\.ed-head\s+strong\s*\{[^}]*overflow\s*:\s*hidden[^}]*text-overflow\s*:\s*ellipsis/is)
  assert.match(editor, /@container\s+loadout-editor\s*\(max-width\s*:\s*760px\)\s*\{[\s\S]*?\.ed-head\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,1fr\)/is)
})

test('formula and weapon content wrap while the 1269-factor bag uses a bounded virtual viewport', () => {
  assert.match(editor, /\.profile-stat-heading\s*\{[^}]*min-width\s*:\s*0[^}]*flex-wrap\s*:\s*wrap/is)
  assert.match(editor, /\.runtime-read-row\s+small\s*\{[^}]*white-space\s*:\s*normal/is)
  assert.match(editor, /\.weapon-context-strip\s*\{[^}]*grid-template-columns\s*:\s*58px\s+minmax\(0,1fr\)/is)
  assert.match(editor, /\.weapon-context-strip\s+em\s*\{[^}]*grid-column\s*:\s*2\s*\/\s*-1[^}]*white-space\s*:\s*normal/is)
  assert.match(editor, /from '\.\.\/loadoutVirtualGrid'/)
  assert.match(editor, /class="bag-virtual-viewport ui-scroll-region"/)
  assert.match(editor, /v-for="\(s,\s*visibleIndex\) in visibleSigils"/)
  assert.match(editor, /:data-virtual-index="bagWindow\.startIndex \+ visibleIndex"/)
  assert.doesNotMatch(editor, /v-for="s in filteredSigils"/)
  assert.match(editor, /new ResizeObserver\(/)
  assert.match(editor, /watch\(sigilSearch,\s*resetBagScroll\)/)
  assert.match(editor, /\.bag-virtual-viewport\s*\{[^}]*height\s*:\s*clamp\([^}]*overflow-y\s*:\s*auto/is)
  assert.match(editor, /\.bag-virtual-window\s+\.sigil-pick\s*\{[^}]*height\s*:\s*86px/is)
  assert.match(editor, /\.pick-grid\.sigils\.bag-virtual-window\s*\{[^}]*grid-template-columns\s*:\s*repeat\(var\(--bag-columns\),minmax\(0,1fr\)\)/is)
})
