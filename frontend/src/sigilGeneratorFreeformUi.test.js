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
  assert.match(source, /天然等级是默认值；最高可填到对应技能效果曲线的目录上限/)
})

test('standalone factor generator preserves the selected primary trait in the queue item', () => {
  assert.match(source, /primaryTraitId:\s*selectedPrimaryTraitID\.value/)
})

test('standalone factor defaults respect special-factor and low-curve levels', () => {
  assert.match(source, /selectedLevel\.value = clampLevel\(Number\(sigil\.defaultSigilLevel \|\| sigilNaturalMax\.value\), editableLevelMax\)/)
  assert.match(source, /function primaryDefaultLevel\(/)
  assert.match(source, /Math\.min\(naturalMax, writableMax\)/)
  assert.match(source, /selectedSecondaryLevel\.value = Math\.min\(15, effectCurveMax\(levels\)\)/)
})
