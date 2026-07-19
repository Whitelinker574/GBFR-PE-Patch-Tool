import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = name => readFileSync(new URL(`./components/${name}.vue`, import.meta.url), 'utf8')

const catalogSelect = read('CatalogSelect')
const sigilGenerator = read('SigilGenerator')
const memoryPicker = read('SigilMemoryPicker')
const memoryGenerator = read('SigilMemoryGenerator')
const loadoutRestore = read('SigilLoadoutRestore')
const wrightstoneGenerator = read('WrightstoneGenerator')
const summonEditor = read('SummonEditor')
const assetIcons = readFileSync(new URL('./gameAssetIcons.js', import.meta.url), 'utf8')

test('numeric runtime hashes are normalized to the catalog eight-digit hex keys', () => {
  assert.match(assetIcons, /typeof value === 'number'[\s\S]*value >>> 0[\s\S]*toString\(16\)[\s\S]*padStart\(8, '0'\)/)
})

test('catalog pickers render a resolved official icon only when one exists', () => {
  for (const source of [catalogSelect, memoryPicker]) {
    assert.match(source, /iconResolver:\s*\{\s*type:\s*Function/)
    assert.match(source, /function optionIcon\(/)
    assert.match(source, /<img v-if="optionIcon\(/)
    assert.doesNotMatch(source, /(?:fallback|placeholder)[^\n]*(?:icon|image)|(?:icon|image)[^\n]*(?:fallback|placeholder)/i)
  }
})

test('save sigil generator resolves factor and trait icons from authoritative IDs or names', () => {
  assert.match(sigilGenerator, /import \{ traitAssetIcon \} from '\.\.\/gameAssetIcons'/)
  assert.match(sigilGenerator, /function traitIconForOption\(/)
  assert.match(sigilGenerator, /function sigilIconForOption\(/)
  assert.match(sigilGenerator, /:icon-resolver="sigilIconForOption"/)
  assert.match(sigilGenerator, /:icon-resolver="traitIconForOption"/)
  assert.match(sigilGenerator, /v-if="existingSigilIcon\(s\)"/)
  assert.match(sigilGenerator, /v-if="queueItemIcon\(item\)"/)
})

test('live sigil editor shows exact trait icons in controls, current values and reusable entries', () => {
  assert.match(memoryGenerator, /import \{ traitAssetIcon \} from '\.\.\/gameAssetIcons'/)
  assert.match(memoryGenerator, /function traitIconByHash\(/)
  assert.equal((memoryGenerator.match(/:icon-resolver="traitOptionIcon"/g) || []).length, 2)
  assert.match(memoryGenerator, /v-if="traitIconByHash\(status\.primaryTraitHash/)
  assert.match(memoryGenerator, /v-if="traitIconByHash\(form\.primaryTraitHash/)
  assert.match(memoryGenerator, /v-if="traitIconByHash\(t\.primaryTraitHash/)
  assert.match(memoryGenerator, /v-if="traitIconByHash\(h\.primaryTraitHash/)
})

test('loadout restore derives each displayed icon from the recorded trait hash', () => {
  assert.match(loadoutRestore, /import \{ traitAssetIcon \} from '\.\.\/gameAssetIcons'/)
  assert.match(loadoutRestore, /function traitIcon\(hash\)/)
  assert.match(loadoutRestore, /v-if="traitIcon\(entry\.primaryTraitHash\)"/)
  assert.match(loadoutRestore, /v-if="traitIcon\(entry\.secondaryTraitHash\)"/)
})

test('remaining blessing and summon trait selectors expose the same real icon treatment', () => {
  assert.match(wrightstoneGenerator, /:icon-resolver="traitIconForOption"/)
  assert.match(summonEditor, /import \{ summonAssetIcon, traitAssetIcon \} from '\.\.\/gameAssetIcons'/)
  assert.match(summonEditor, /function currentTraitIcon\(/)
  assert.match(summonEditor, /v-if="currentTraitIcon\(\)"/)
})
