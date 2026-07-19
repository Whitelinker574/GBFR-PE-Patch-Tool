import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const editor = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')
const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')
const languageSettings = readFileSync(new URL('./components/LanguageSettings.vue', import.meta.url), 'utf8')

test('visible loadout fallbacks never expose raw hashes or invent item names', () => {
  assert.doesNotMatch(editor, /s\.name\s*\|\|\s*s\.hash/)
  assert.doesNotMatch(editor, /skill\.name\s*\|\|\s*skill\.hash/)
  assert.doesNotMatch(editor, /背包因子\s*#\$\{/)
  assert.doesNotMatch(viewer, /lo\.weaponName\s*\|\|\s*lo\.weaponHash/)
  assert.doesNotMatch(viewer, /s\.name\s*\|\|\s*s\.hash/)
  assert.doesNotMatch(viewer, /未收录[^'"`]*\+\s*[sm]\.hash/)
})

test('loadout list and workspace use standard font weights', () => {
  assert.doesNotMatch(viewer, /font-weight\s*:\s*(?:[58]50|560|680|750|800|850|900)\b/)
})

test('loadout simulation failures stay visible instead of looking like missing data', () => {
  assert.match(editor, /const simulationError = ref\(''\)/)
  assert.match(editor, /catch \(error\)[\s\S]*simulationError\.value = `配装计算失败：\$\{String\(error\)\}`/)
  assert.match(editor, /v-if="simulationError"[^>]*role="alert"[^>]*>\{\{ simulationError \}\}/)
})

test('language settings do not promise translation coverage the editor does not have', () => {
  assert.doesNotMatch(languageSettings, /完整英文界面|complete English interface/)
  assert.match(languageSettings, /部分新增专业术语仍保留中文/)
  assert.match(languageSettings, /some newly added technical terms remain Chinese/)
})
