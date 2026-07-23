import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')
const progressionSource = readFileSync(new URL('./components/ProgressionEditor.vue', import.meta.url), 'utf8')

test('loadout editor submits preset and owned-resource edits through one transaction', () => {
  assert.match(source, /LoadoutApplyWithResources/)
  assert.doesNotMatch(source, /await\s+LoadoutApply\(/)
  assert.match(source, /changes:\s*\[w\]/)
  assert.match(source, /const weaponEdits\s*=\s*buildWeaponInlineEdits\(\)/)
  assert.match(source, /const summonEdits\s*=\s*buildSummonInlineEdits\(\)/)
  assert.match(source, /LoadoutApplyWithResources[\s\S]*?weaponEdits,\s*summonEdits,/)
})

test('weapon inline editing uses the exact five-slot snapshot and per-weapon audited choices', () => {
  assert.match(source, /weaponSkillDrafts/)
  assert.match(source, /selectedWeaponContext\.value\?\.skillSlots/)
  assert.match(source, /expectUnitId:\s*Number\(weapon\.unitId\)/)
  assert.match(source, /expectStoredHash:\s*weapon\.storedHash/)
  assert.match(source, /expectTranscendence:\s*Number\(weapon\.transcendence\)/)
  assert.match(source, /expectSkillHashes:/)
  assert.match(source, /skillHashes:/)
  assert.match(source, /v-for="slot in editableWeaponSkillSlots"/)
  assert.match(source, /v-for="option in slot\.options"/)
  assert.doesNotMatch(source, /const weaponTranscendenceSkills =/)
})

test('offline weapon editor uses each weapon replacement-skill snapshot', () => {
  assert.match(progressionSource, /replacementSkillSlots/)
  assert.ok(progressionSource.includes('weaponSkillDrafts.value = isOwned ? (weapon.skillSlots || []).map'))
  assert.ok(progressionSource.includes('change.skillHashes = [...weaponSkillDrafts.value]'))
  assert.match(progressionSource, /v-for="slot in replacementSkillSlots"/)
  assert.match(progressionSource, /v-for="option in slot.options"/)
  assert.doesNotMatch(progressionSource, /const weaponTranscendenceSkills =/)
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
