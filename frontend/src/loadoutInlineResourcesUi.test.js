import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')

test('loadout editor submits preset and owned-resource edits through one transaction', () => {
  assert.match(source, /LoadoutApplyWithResources/)
  assert.doesNotMatch(source, /await\s+LoadoutApply\(/)
  assert.match(source, /changes:\s*\[w\]/)
  assert.match(source, /const weaponEdits\s*=\s*buildWeaponInlineEdits\(\)/)
  assert.match(source, /const summonEdits\s*=\s*buildSummonInlineEdits\(\)/)
  assert.match(source, /LoadoutApplyWithResources[\s\S]*?weaponEdits,\s*summonEdits,/)
})

test('weapon inline editing is limited to the audited seventh-stage field and four real effects', () => {
  for (const hash of ['BBD77C33', '020DB733', '3F682593', '79027FC8']) {
    assert.match(source, new RegExp(hash))
  }
  assert.match(source, /selectedWeaponContext\.value\?\.transcendence\)\s*>=\s*7/)
  assert.match(source, /expectUnitId:\s*Number\(weapon\.unitId\)/)
  assert.match(source, /expectStoredHash:\s*weapon\.storedHash/)
  assert.match(source, /expectTranscendence:\s*Number\(weapon\.transcendence\)/)
  assert.match(source, /expectTranscendenceSkill:\s*weapon\.transcendenceSkill/)
  assert.match(source, /transcendenceSkill:\s*weaponTranscendenceSkillDraft\.value/)
})

test('summon inline editing uses the save snapshot as a stale-write guard', () => {
  assert.match(source, /SummonGetOptions/)
  for (const field of [
    'expectUnitId', 'expectTypeHash', 'expectMainTraitHash', 'expectMainTraitLevel',
    'expectSubParamHash', 'expectSubParamLevel', 'expectRank',
    'mainTraitHash', 'mainTraitLevel', 'subParamHash', 'subParamLevel', 'rank',
  ]) {
    assert.match(source, new RegExp(`${field}:`))
  }
  assert.doesNotMatch(source, /SummonUpdateOwned/)
})

test('owned-instance scope is explicit and summon edits also select the same global four slots', () => {
  assert.match(source, /编辑的是背包中的武器与召唤石实例/)
  assert.match(source, /所有引用它的配装/)
  assert.match(source, /watch\(summonInlineEnabled[\s\S]*?if\s*\(enabled\)\s*writeGlobalSummons\.value\s*=\s*true/)
  assert.match(source, /同一事务/)
  assert.match(source, /if\s*\(op\.value\s*!==\s*'write'/)
  assert.match(source, /if\s*\(nextOp\s*!==\s*'write'\)[\s\S]*?weaponInlineEnabled\.value\s*=\s*false[\s\S]*?summonInlineEnabled\.value\s*=\s*false/)
})

test('inline resource controls retain the parchment component language and compact responsivity', () => {
  assert.match(source, /class="inline-resource-panel weapon-inline-panel"/)
  assert.match(source, /class="summon-inline-grid"/)
  assert.match(source, /\.inline-resource-panel\s*\{[^}]*var\(--line-soft\)[^}]*rgba\(139,103,55/is)
  assert.match(source, /\.summon-inline-grid\s*\{[^}]*repeat\(3,minmax\(0,1fr\)\)/is)
  assert.match(source, /@container\s+loadout-editor\s*\(max-width\s*:\s*760px\)[\s\S]*?\.summon-inline-grid\s*\{[^}]*grid-template-columns\s*:\s*1fr/is)
})
