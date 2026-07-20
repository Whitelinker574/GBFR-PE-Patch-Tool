import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/SigilGenerator.vue', import.meta.url), 'utf8')

test('standalone factor generator uses clean searchable primary and secondary trait names', () => {
  assert.match(source, /const selectedPrimaryTraitID = ref\(''\)/)
  assert.match(source, /<CatalogSelect v-model="selectedPrimaryTraitID"/)
  assert.match(source, /search-placeholder="搜索主特性名称"/)
  assert.match(source, /<CatalogSelect v-model="selectedSecondaryTraitID"/)
  assert.doesNotMatch(source, /强制组合/)
})

test('standalone factor generator preserves the selected primary trait in the queue item', () => {
  assert.match(source, /primaryTraitId:\s*selectedPrimaryTraitID\.value/)
})
