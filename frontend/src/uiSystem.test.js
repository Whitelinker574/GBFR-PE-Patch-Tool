import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const tokens = readFileSync(new URL('./style.css', import.meta.url), 'utf8')
const ui = readFileSync(new URL('./ui.css', import.meta.url), 'utf8')

test('the shared design system exposes one readable responsive control scale', () => {
  for (const token of [
    '--font-display',
    '--control-height',
    '--control-height-sm',
    '--content-gutter',
    '--transition-control',
    '--page-measure',
    '--page-measure-wide',
  ]) {
    assert.match(tokens, new RegExp(`${token}\\s*:`), `missing ${token}`)
  }

  assert.match(tokens, /--control-height\s*:\s*36px/)
  assert.match(tokens, /--fs-xs\s*:\s*11px/)
  assert.doesNotMatch(tokens, /--fs-(?:2xs|xs)\s*:\s*(?:[0-9]|10)px/)
})

test('shared UI includes the page, flow, grid, toolbar, stat and disclosure primitives', () => {
  for (const className of [
    'ui-page',
    'ui-page-stack',
    'ui-toolbar',
    'ui-actions',
    'ui-form-grid',
    'ui-card-grid',
    'ui-stat-grid',
    'ui-control-group',
    'ui-scroll-region',
    'ui-disclosure',
  ]) {
    assert.match(ui, new RegExp(`\\.${className}\\b`), `missing .${className}`)
  }
})

test('shared fields and buttons use the same control geometry', () => {
  assert.match(ui, /\.ui-btn\s*\{[^}]*min-height\s*:\s*var\(--control-height\)/is)
  assert.match(ui, /\.ui-input\s*,\s*\.ui-select\s*,\s*\.ui-textarea\s*\{[^}]*min-height\s*:\s*var\(--control-height\)/is)
  assert.match(ui, /\.ui-btn\.is-sm\s*\{[^}]*min-height\s*:\s*var\(--control-height-sm\)/is)
})

test('responsive primitives reflow instead of shrinking text or forcing fixed columns', () => {
  assert.match(ui, /grid-template-columns\s*:\s*repeat\(auto-fit,\s*minmax\(min\(100%,\s*var\(--ui-grid-min[^)]*\)\),\s*1fr\)\)/is)
  assert.match(ui, /@media\s*\(max-width\s*:\s*840px\)/i)
  assert.match(ui, /@media\s*\(max-width\s*:\s*640px\)/i)
  assert.match(ui, /@media\s*\(prefers-reduced-motion\s*:\s*reduce\)/i)
})
