import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/SigilGenerator.vue', import.meta.url), 'utf8')

test('standalone factor generator locks the fixed primary and exposes only the audited secondary pool', () => {
  assert.match(source, /const selectedPrimaryTraitID = ref\(''\)/)
  assert.doesNotMatch(source, /<CatalogSelect v-model="selectedPrimaryTraitID"/)
  assert.match(source, /class="[^"]*fixed-primary-trait[^"]*"/)
  assert.match(source, /<CatalogSelect v-model="selectedSecondaryTraitID"/)
  assert.match(source, /const secondaryPickerOptions = computed\(\(\) => secondaryTraits\.value\)/)
  assert.match(source, /secondaryTraits\.value = allowed/)
  assert.match(source, /只显示该因子在 2\.0\.2 表中的合法副词条/)
  assert.match(source, /天然等级是默认值；最高可填到对应技能效果曲线的目录上限/)
})

test('standalone factor generator preserves the selected primary trait in the queue item', () => {
  assert.match(source, /primaryTraitId:\s*selectedPrimaryTraitID\.value/)
})

test('standalone factor legality failures are fail-closed', () => {
  assert.match(source, /status: 'impossible', writable: false, message: `检验失败，已禁止写入/)
  assert.doesNotMatch(source, /status: 'unknown', writable: true/)
})

test('standalone factor defaults respect special-factor and low-curve levels', () => {
  assert.match(source, /selectedLevel\.value = clampLevel\(Number\(sigil\.defaultSigilLevel \|\| sigilNaturalMax\.value\), editableLevelMax\)/)
  assert.match(source, /function primaryDefaultLevel\(/)
  assert.match(source, /Math\.min\(naturalMax, writableMax\)/)
  assert.match(source, /selectedSecondaryLevel\.value = Math\.min\(15, effectCurveMax\(levels\)\)/)
})
