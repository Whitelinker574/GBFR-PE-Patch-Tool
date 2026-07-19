import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const tokens = readFileSync(new URL('./style.css', import.meta.url), 'utf8')
const readComponent = name => readFileSync(new URL(`./components/${name}.vue`, import.meta.url), 'utf8')
const legality = readComponent('LegalityIndicator')
const catalog = readComponent('CatalogSelect')

const scopedCss = source => source.match(/<style\s+scoped>([\s\S]*?)<\/style>/)?.[1] || ''

function tokenHex(source, name) {
  const value = source.match(new RegExp(`${name}\\s*:\\s*(#[0-9a-f]{6})`, 'i'))?.[1]
  assert.ok(value, `missing literal color token ${name}`)
  return value
}

function luminance(hex) {
  const channels = hex.slice(1).match(/../g).map(part => Number.parseInt(part, 16) / 255)
  const linear = channels.map(channel => channel <= 0.04045
    ? channel / 12.92
    : ((channel + 0.055) / 1.055) ** 2.4)
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2]
}

function contrast(left, right) {
  const [bright, dark] = [luminance(left), luminance(right)].sort((a, b) => b - a)
  return (bright + 0.05) / (dark + 0.05)
}

test('functional muted text stays readable on the darkest shared paper field', () => {
  const muted = tokenHex(tokens, '--p-ink-500')
  const sunken = tokenHex(tokens, '--p-paper-500')
  assert.ok(contrast(muted, sunken) >= 4.5, `${muted} on ${sunken} must reach 4.5:1`)
})

test('legality status uses semantic parchment text and the shared minimum type scale', () => {
  const css = scopedCss(legality)
  assert.match(css, /\.text\s+small\s*\{[^}]*color\s*:\s*var\(--text-secondary\)/is)
  assert.match(css, /\.text\s+strong\s*\{[^}]*font-size\s*:\s*var\(--fs-sm\)/is)
  assert.match(css, /\.text\s+small\s*\{[^}]*font-size\s*:\s*var\(--fs-xs\)/is)
  assert.match(css, /\.icon\s*\{[^}]*font-size\s*:\s*var\(--fs-xs\)/is)
  assert.doesNotMatch(css, /rgba?\(\s*255\s*,\s*255\s*,\s*255/i)
  assert.doesNotMatch(css, /font-size\s*:\s*0?\.(?:5|6)\d*rem/i)
})

test('catalog select composes shared controls and contains no private color palette', () => {
  const css = scopedCss(catalog)
  assert.match(catalog, /class="catalog-trigger ui-btn"/)
  assert.match(catalog, /class="catalog-popover ui-card"/)
  assert.match(catalog, /class="catalog-search-input ui-input"/)
  assert.match(catalog, /class="catalog-option ui-row"/)
  assert.match(catalog, /class="catalog-empty ui-empty"/)
  assert.match(css, /\.catalog-trigger\s*\{[^}]*min-height\s*:\s*var\(--control-height\)/is)
  assert.match(css, /\.catalog-option\s*\{[^}]*min-height\s*:\s*var\(--control-height\)/is)
  assert.doesNotMatch(css, /#[0-9a-f]{3,8}\b|rgba?\(/i)
})
