import test from 'node:test'
import assert from 'node:assert/strict'

import * as finalStatFormatters from './loadoutFinalStats.js'

const { formatFinalStat } = finalStatFormatters

test('final stat formatter keeps missing values honest', () => {
  assert.equal(formatFinalStat(null), '—')
  assert.equal(formatFinalStat(undefined), '—')
  assert.equal(formatFinalStat(Number.NaN), '—')
})

test('final stat formatter distinguishes real zero from missing data', () => {
  assert.equal(formatFinalStat(0), '0')
  assert.equal(formatFinalStat(0, 'pct'), '0%')
})

test('final stat formatter groups integers and preserves useful fractions', () => {
  assert.equal(formatFinalStat(188975), '188,975')
  assert.equal(formatFinalStat(83, 'pct'), '83%')
  assert.equal(formatFinalStat(293.4), '293.4')
  assert.equal(formatFinalStat(1070, 'signedPct'), '+1,070%')
})

test('weapon skill level formatter prefers the real level and never leaks undefined', () => {
	assert.equal(typeof finalStatFormatters.formatWeaponSkillLevel, 'function')
	const { formatWeaponSkillLevel } = finalStatFormatters
  assert.equal(formatWeaponSkillLevel({ level: 15, effectiveLevel: 9 }), 'Lv15')
  assert.equal(formatWeaponSkillLevel({ effectiveLevel: 20 }), 'Lv20')
  assert.equal(formatWeaponSkillLevel({ level: 0 }), 'Lv0')
  assert.equal(formatWeaponSkillLevel({}), 'Lv—')
  assert.equal(formatWeaponSkillLevel(null), 'Lv—')
})

test('effect totals render one card per attribute without mixing flat and percent values', () => {
  assert.equal(typeof finalStatFormatters.groupEffectTotals, 'function')
  const { groupEffectTotals } = finalStatFormatters
  const grouped = groupEffectTotals([
    { key: 'pct|攻击力', label: '攻击力', unit: 'pct', value: 224.4, catLabel: '攻击类', sources: ['浩劫'] },
    { key: 'flat|攻击力', label: '攻击力', unit: 'flat', value: 9283, catLabel: '基础能力', sources: ['武器'] },
    { key: 'pct|最大HP', label: '最大HP', unit: 'pct', value: 50, catLabel: '防御类', sources: ['金刚'] },
    { key: 'flat|最大HP', label: '最大HP', unit: 'flat', value: 61099, catLabel: '基础能力', sources: ['武器', '专精'] },
  ])

  assert.deepEqual(grouped.map(total => total.label), ['攻击力', '最大HP'])
  assert.deepEqual(grouped[0].parts.map(part => [part.unit, part.value]), [['pct', 224.4], ['flat', 9283]])
  assert.deepEqual(grouped[0].sources, ['浩劫', '武器'])
  assert.deepEqual(grouped[1].parts.map(part => [part.unit, part.value]), [['pct', 50], ['flat', 61099]])
  assert.deepEqual(grouped[1].sources, ['金刚', '武器', '专精'])
})

test('trait level summary distinguishes effective levels from capped overflow', () => {
  assert.equal(typeof finalStatFormatters.summarizeTraitLevels, 'function')
  const summary = finalStatFormatters.summarizeTraitLevels([
    { name: '伤害上限', level: 65, rawLevel: 102, capped: true },
    { name: '体力', level: 50, rawLevel: 51, capped: true },
    { name: '昏厥', level: 44, rawLevel: 44, capped: false },
    { name: '未配置', level: 0, rawLevel: 0 },
  ])

  assert.deepEqual(summary, {
    effective: 159,
    invested: 197,
    overflow: 38,
    cappedCount: 2,
  })
})
