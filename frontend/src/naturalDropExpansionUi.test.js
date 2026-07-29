import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('./components/NaturalDropLab.vue', import.meta.url), 'utf8')

test('natural drop workspace exposes Transmarvel sigils as a first-class configurable pool', () => {
  assert.match(source, /const sigilSelections = reactive\(\{\}\)/)
  assert.match(source, /workspace\.value\?\.sigils/)
  assert.match(source, /selectedSigils/)
  assert.match(source, /sigils:\s*selectedSigils\.value/)
  assert.match(source, /Transmarvel 因子/)
  assert.match(source, /addSigilDraft/)
  assert.match(source, /加入待部署/)
})

test('natural drop cards reuse exact game icons for summons, factors, traits and items', () => {
  assert.match(source, /import \{ itemAssetIcon, summonAssetIcon, traitAssetIcon \} from '\.\.\/gameAssetIcons'/)
  assert.match(source, /function summonIcon\(item\)/)
  assert.match(source, /function sigilIcon\(item\)/)
  assert.match(source, /function traitIcon\(item\)/)
  assert.match(source, /function pickerItemIcon\(option\)/)
  assert.match(source, /class="drop-item-icon"/)
  assert.match(source, /class="drop-trait-icon"/)
})

test('natural drop uses compact searchable builders and one reviewable deployment ledger', () => {
  assert.match(source, /import CatalogSelect/)
  assert.match(source, /class="drop-builder-grid"/)
  assert.match(source, /pendingDropRows/)
  assert.match(source, /待部署掉落清单/)
  assert.match(source, /removePendingDrop/)
  assert.match(source, /clearPendingDrops/)
  assert.doesNotMatch(source, /class="sigil-grid"/)
  assert.doesNotMatch(source, /class="summon-grid"/)
  assert.doesNotMatch(source, /class="wrightstone-grid"/)
})

test('Transmarvel result mode is mutually exclusive and generic items use a verified automatic reward target', () => {
  assert.match(source, /watch\(sigilOnly/)
  assert.match(source, /watch\(wrightstoneOnly/)
  assert.match(source, /不能同时/)
  assert.match(source, /itemPickerOptions/)
  assert.match(source, /genericDropQuantity/)
  assert.match(source, /genericDropWeight/)
  assert.match(source, /addItemDraft/)
  assert.match(source, /无尽模式 · 锻造师奖励/)
  assert.doesNotMatch(source, /genericDropRoute/)
  assert.doesNotMatch(source, /具体任务\/敌人\/宝箱/)
})
