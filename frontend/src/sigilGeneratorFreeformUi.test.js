import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/SigilGenerator.vue', import.meta.url), 'utf8')

test('standalone factor generator fixes the primary and searches only table-backed secondary traits', () => {
  assert.match(source, /const selectedPrimaryTraitID = ref\(''\)/)
  assert.match(source, /由 gem\.tbl 固定/)
  assert.doesNotMatch(source, /<CatalogSelect v-model="selectedPrimaryTraitID"/)
  assert.match(source, /<CatalogSelect v-model="selectedSecondaryTraitID"/)
  assert.match(source, /secondaryTraits\.value = allowed/)
  assert.doesNotMatch(source, /强制组合/)
})

test('standalone factor generator preserves the selected primary trait in the queue item', () => {
  assert.match(source, /primaryTraitId:\s*selectedPrimaryTraitID\.value/)
})
