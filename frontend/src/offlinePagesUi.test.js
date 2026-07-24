import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = name => readFileSync(new URL(`./components/${name}.vue`, import.meta.url), 'utf8')

const progression = read('ProgressionEditor')
const sigil = read('SigilGenerator')
const wrightstone = read('WrightstoneGenerator')
const chara = read('CharaStats')
const save = read('SaveEditor')
const offlinePages = [progression, sigil, wrightstone, chara, save]

test('offline pages consume the shared parchment primitives instead of a second dark skin', () => {
  for (const source of offlinePages) {
    assert.match(source, /ui-card/)
    assert.match(source, /ui-btn/)
    assert.doesNotMatch(source, /rgba\(255\s*,\s*255\s*,\s*255|rgba\(8\s*,\s*31\s*,\s*53|#(?:e5f7fa|edf4f6|f0e8d7|8be9f7)\b/i)
  }
})

test('progression keeps a compact save strip and container-driven master detail layout', () => {
  assert.match(progression, /class="save-card ui-card compact-save-bar"/)
  assert.match(progression, /\.workspace\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,\s*1\.6fr\)\s+minmax\(300px,\s*\.9fr\)/is)
  assert.match(progression, /@container\s*\(max-width:\s*760px\)[\s\S]*?\.workspace\s*\{[^}]*grid-template-columns\s*:\s*1fr/is)
  assert.doesNotMatch(progression, /@media\s*\(max-width:\s*900px\)[\s\S]*?\.workspace/)
})

test('sigil queue and write target share a responsive lower grid and danger stays collapsed', () => {
  assert.match(sigil, /class="sigil-lower-grid"/)
  assert.match(sigil, /<details class="section ui-card section-danger">/)
  assert.match(sigil, /<summary class="section-title ui-section-title">危险操作<\/summary>/)
  assert.match(sigil, /\.sigil-lower-grid\s*\{[^}]*grid-template-columns\s*:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/is)
  assert.match(sigil, /@container\s*\(max-width:\s*760px\)[\s\S]*?\.sigil-lower-grid\s*\{[^}]*grid-template-columns\s*:\s*1fr/is)
})

test('wrightstone renders the three trait choices as adaptive cards', () => {
  assert.match(wrightstone, /class="trait-grid"/)
  assert.match(wrightstone, /class="trait-card ui-card"/)
  assert.match(wrightstone, /\.trait-grid\s*\{[^}]*repeat\(auto-fit,\s*minmax\(240px,\s*1fr\)\)/is)
  assert.match(wrightstone, /class="wrightstone-lower-grid"/)
  assert.match(wrightstone, /class="field ui-field wrightstone-pick"/)
  assert.match(wrightstone, /\.wrightstone-pick\s*\{[^}]*width\s*:\s*min\(100%,\s*680px\)/is)
})

test('simple offline tables keep an 840px reading measure and show a real preselection empty state', () => {
  for (const source of [chara, save]) {
    assert.match(source, /\.root\s*\{[^}]*max-width\s*:\s*840px/is)
    assert.match(source, /v-else-if="!savePath" class="empty ui-empty"/)
    assert.match(source, /@container\s*\(max-width:\s*620px\)/)
  }
})

test('offline pages remove permanent backup slogans and keep supporting type readable', () => {
  for (const source of [chara, save]) {
    assert.doesNotMatch(source, /写入时自动备份并回读验证|每次保存都会创建时间戳备份并回读验证/)
  }
  for (const source of offlinePages) {
    assert.doesNotMatch(source, /font-size\s*:\s*(?:8|9|10)(?:\.\d+)?px/i)
    assert.doesNotMatch(source, /font-size\s*:\s*\.(?:5|6)\d*rem/i)
  }
})

test('offline generators do not repeat backup or output-safety notices beside the write controls', () => {
  for (const source of [sigil, wrightstone]) {
    assert.doesNotMatch(source, /建议先备份|安全提示：只写入输出存档/)
  }
})

test('offline selectors and save loaders reject stale async responses', () => {
  for (const source of [sigil, wrightstone, chara, save, progression]) {
    assert.match(source, /Epoch|epoch/)
    assert.match(source, /epoch !==|!== .*Epoch/)
  }
})
