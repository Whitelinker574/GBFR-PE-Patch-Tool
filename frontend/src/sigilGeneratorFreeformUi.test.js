import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/SigilGenerator.vue', import.meta.url), 'utf8')

test('standalone factor generator exposes the complete trait catalog with advisory natural rules', () => {
  assert.match(source, /const selectedPrimaryTraitID = ref\(''\)/)
  assert.match(source, /<CatalogSelect v-model="selectedPrimaryTraitID" :options="allTraits"/)
  assert.match(source, /<CatalogSelect v-model="selectedSecondaryTraitID"/)
  assert.match(source, /const secondaryPickerOptions = computed\(\(\) => allTraits\.value\)/)
  assert.match(source, /secondaryTraits\.value = allowed/)
  assert.doesNotMatch(source, /强制组合/)
  assert.match(source, /天然词池与等级只作提醒/)
})

test('standalone factor generator preserves the selected primary trait in the queue item', () => {
  assert.match(source, /primaryTraitId:\s*selectedPrimaryTraitID\.value/)
})
