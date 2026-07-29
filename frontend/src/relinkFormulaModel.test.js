import assert from 'node:assert/strict'
import test from 'node:test'
import {
  SUMMON_ROLL_PROBABILITIES,
  attemptsForProbability,
  calculateDamageFormula,
  calculateIncomingDamage,
  damageCapDistance,
  probabilityAtLeastOnce,
} from './relinkFormulaModel.js'

test('v2 exact damage formula matches the workbook default and caps C x D at two', () => {
  const result = calculateDamageFormula({
    damageCap: 1_000_000,
    elementalAdvantage: 0.3,
    outsideCapBonus: 0.25,
    attackBuff: 0.2,
    defenseDown: 0.2,
    supplementalDamage: 0.6,
  })
  assert.equal(result.finalDamage, 2_952_000)
  assert.ok(Math.abs(result.stableCapMargin - (1 / 0.95 - 1)) < 1e-12)

  const saturated = calculateDamageFormula({ damageCap: 1000, attackBuff: 3, defenseDown: 3 })
  assert.equal(saturated.cappedAttackDefenseProduct, 2)
  assert.equal(saturated.attackDefenseTerm, 0.5)
})

test('OD bug suppression, repeated OD, and berserk remain independently selectable', () => {
  const firstOD = calculateDamageFormula({
    damageCap: 1_000_000,
    elementalAdvantage: 0.3,
    outsideCapBonus: 0.25,
    attackBuff: 0.2,
    defenseDown: 0.2,
    supplementalDamage: 0.6,
    odStage: 1,
    disableAttackDefenseInOD: true,
  })
  assert.equal(firstOD.effectiveAttackBuff, 0)
  assert.equal(firstOD.finalDamage, 2_340_000)
  assert.equal(calculateDamageFormula({ damageCap: 1000, odStage: 2, berserk: true }).stateMultiplier, 0.21)
})

test('defense zones multiply and keep crab defense and crab shield in separate inputs', () => {
  const base = calculateIncomingDamage({ baseDamage: 100_000, generalDefense: 0.55, superArmor: 0.3 })
  assert.ok(Math.abs(base.finalDamage - 31_500) < 1e-9)
  assert.ok(Math.abs(base.totalReduction - 0.685) < 1e-12)

  const crab = calculateIncomingDamage({ baseDamage: 100_000, defenseBuff: 0.02, whiteShield: 0.1 })
  assert.equal(crab.finalDamage, 88_200)
})

test('summon probability paths stay separate and expose exact attempt math', () => {
  assert.deepEqual(SUMMON_ROLL_PROBABILITIES, {
    namedSubTraitAnd100Cap: 0.0027,
    pairedMainAndSubAt100Cap: 0.0007,
    strictScreenCondition: 0.0002,
  })
  assert.equal(attemptsForProbability(0.0007, 0.5), 990)
  assert.ok(Math.abs(probabilityAtLeastOnce(0.0007, 1000) - (1 - 0.9993 ** 1000)) < 1e-12)
})

test('damage-cap distance reports below-cap and above-cap states without rounding away evidence', () => {
  assert.deepEqual(damageCapDistance(950, 1000), { ratio: 0.95, percent: 95, remaining: 50, exceeded: 0 })
  assert.deepEqual(damageCapDistance(1200, 1000), { ratio: 1.2, percent: 120, remaining: 0, exceeded: 200 })
})
