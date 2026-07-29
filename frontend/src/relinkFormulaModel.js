export const RELINK_FORMULA_VERSION = '2.0.2-v2-tardis98'

export const SUMMON_ROLL_PROBABILITIES = Object.freeze({
  namedSubTraitAnd100Cap: 0.0027,
  pairedMainAndSubAt100Cap: 0.0007,
  strictScreenCondition: 0.0002,
})

function finite(value, fallback = 0) {
  const number = Number(value)
  return Number.isFinite(number) ? number : fallback
}

function nonNegative(value) {
  return Math.max(0, finite(value))
}

function reduction(value) {
  return Math.max(0, Math.min(1, finite(value)))
}

export function monsterStateMultiplier({ odStage = 0, berserk = false, wallReduction = 0 } = {}) {
  const stage = Math.max(0, Math.min(2, Math.trunc(finite(odStage))))
  const odMultiplier = stage === 1 ? 0.9 : stage >= 2 ? 0.7 : 1
  const berserkMultiplier = berserk ? 0.3 : 1
  return odMultiplier * berserkMultiplier * (1 - reduction(wallReduction))
}

export function calculateDamageFormula({
  damageCap = 0,
  elementalAdvantage = 0,
  outsideCapBonus = 0,
  attackBuff = 0,
  defenseDown = 0,
  supplementalDamage = 0,
  odStage = 0,
  berserk = false,
  disableAttackDefenseInOD = true,
  wallReduction = 0,
} = {}) {
  const cap = nonNegative(damageCap)
  const stage = Math.max(0, Math.min(2, Math.trunc(finite(odStage))))
  const suppressBuffs = disableAttackDefenseInOD && stage > 0
  const effectiveAttackBuff = suppressBuffs ? 0 : nonNegative(attackBuff)
  const effectiveDefenseDown = suppressBuffs ? 0 : nonNegative(defenseDown)
  const cappedAttackDefenseProduct = Math.min((1 + effectiveAttackBuff) * (1 + effectiveDefenseDown), 2)
  const attackDefenseTerm = (cappedAttackDefenseProduct - 1) / 2
  const beforeMonsterState = cap * (
    (1 + nonNegative(elementalAdvantage)) * (1 + nonNegative(outsideCapBonus)) + attackDefenseTerm
  ) * (1 + nonNegative(supplementalDamage))
  const stateMultiplier = monsterStateMultiplier({ odStage: stage, berserk, wallReduction })
  const finalDamage = beforeMonsterState * stateMultiplier
  return {
    formulaVersion: RELINK_FORMULA_VERSION,
    effectiveAttackBuff,
    effectiveDefenseDown,
    cappedAttackDefenseProduct,
    attackDefenseTerm,
    beforeMonsterState,
    stateMultiplier,
    finalDamage,
    monsterReduction: 1 - stateMultiplier,
    stableCapRawDamage: cap / 0.95,
    stableCapMargin: cap > 0 ? (cap / 0.95) / cap - 1 : 0,
  }
}

export function calculateIncomingDamage({
  baseDamage = 0,
  attackDown = 0,
  generalDefense = 0,
  defenseBuff = 0,
  stronghold = 0,
  garrison = 0,
  superArmor = 0,
  whiteShield = 0,
  otherReduction = 0,
} = {}) {
  const zones = {
    attackDown: reduction(attackDown),
    generalDefense: reduction(generalDefense),
    defenseBuff: reduction(defenseBuff),
    stronghold: reduction(stronghold),
    garrison: reduction(garrison),
    superArmor: reduction(superArmor),
    whiteShield: reduction(whiteShield),
    otherReduction: reduction(otherReduction),
  }
  const multiplier = Object.values(zones).reduce((product, rate) => product * (1 - rate), 1)
  return {
    formulaVersion: RELINK_FORMULA_VERSION,
    zones,
    multiplier,
    finalDamage: nonNegative(baseDamage) * multiplier,
    totalReduction: 1 - multiplier,
  }
}

export function probabilityAtLeastOnce(probability, attempts) {
  const chance = reduction(probability)
  const count = Math.max(0, Math.trunc(finite(attempts)))
  return 1 - (1 - chance) ** count
}

export function attemptsForProbability(probability, targetProbability) {
  const chance = reduction(probability)
  const target = reduction(targetProbability)
  if (target <= 0) return 0
  if (chance <= 0) return Infinity
  if (chance >= 1) return 1
  if (target >= 1) return Infinity
  return Math.ceil(Math.log(1 - target) / Math.log(1 - chance))
}

export function damageCapDistance(observedDamage, damageCap) {
  const damage = nonNegative(observedDamage)
  const cap = nonNegative(damageCap)
  if (!cap) return { ratio: 0, percent: 0, remaining: 0, exceeded: 0 }
  const ratio = damage / cap
  return {
    ratio,
    percent: ratio * 100,
    remaining: Math.max(0, cap - damage),
    exceeded: Math.max(0, damage - cap),
  }
}
