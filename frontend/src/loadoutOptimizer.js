import { calculateDamageFormula, calculateIncomingDamage, damageCapDistance, RELINK_FORMULA_VERSION } from './relinkFormulaModel.js'

function normalized(value) { return String(value || '').trim().toLocaleLowerCase() }

function targetMap(targets) {
  return new Map((targets || []).filter(item => item?.name).map(item => [normalized(item.name), {
    name: item.name,
    weight: Math.max(1, Number(item.weight) || 1),
    cap: Math.max(1, Number(item.cap) || 65),
  }]))
}

function contribution(candidate, targets) {
  const values = new Map()
  for (const trait of candidate.traits || []) {
    const target = targets.get(normalized(trait.name))
    if (!target) continue
    values.set(target.name, (values.get(target.name) || 0) + Math.max(0, Number(trait.level) || 0))
  }
  return values
}

function candidateKey(candidate) {
  return [candidate.name, ...(candidate.traits || []).map(item => `${item.name}:${item.level}`), candidate.slotId || 0].join('|')
}

export function buildCatalogCandidates(atlas, targets) {
  const wanted = targetMap(targets)
  const result = []
  for (const entry of atlas?.sigils || []) {
    if (!entry.constructible) continue
    const primaryRecord = (atlas?.traits || []).find(item => item.internalId === entry.primaryTraitId)
    const primary = { id: entry.primaryTraitId, name: entry.primaryTraitName, level: entry.firstTraitMaxLevel || 0 }
    const secondaryMatches = (entry.secondaryTraits || []).filter(item => wanted.has(normalized(item.displayName)))
    const requiresSecondary = entry.supportsSecondaryTrait === true
    const legalSecondaries = entry.secondaryTraits || []
    if (requiresSecondary && !legalSecondaries.length) continue
    const variants = requiresSecondary
      ? (secondaryMatches.length ? secondaryMatches : [legalSecondaries[0]])
      : [null]
    for (const secondary of variants) {
      const candidate = {
        id: `${entry.internalId}:${secondary?.internalId || ''}`,
        source: 'catalog', name: entry.displayName, hash: entry.hash,
        sigilId: entry.internalId,
        sigilLevel: Math.max(0, ...(entry.allowedSigilLevels || []), Number(entry.defaultSigilLevel || 0)),
        primaryTraitId: entry.primaryTraitId,
        primaryTraitName: entry.primaryTraitName,
        primaryLevel: Number(entry.firstTraitMaxLevel || 0),
        secondaryTraitId: secondary?.internalId || '',
        secondaryTraitName: secondary?.displayName || '',
        secondaryLevel: Number(secondary?.maxLevel || 0),
        exactSigilHash: entry.hash || '',
        exactPrimaryTraitHash: primaryRecord?.hash || '',
        exactSecondaryTraitHash: secondary?.hash || '',
        tableExact: entry.tableExact === true,
        traits: [primary, ...(secondary ? [{ id: secondary.internalId, name: secondary.displayName, level: secondary.maxLevel }] : [])],
      }
      if (contribution(candidate, wanted).size) result.push(candidate)
    }
  }
  return result.sort((a, b) => candidateKey(a).localeCompare(candidateKey(b), 'en'))
}

export function buildInventoryCandidates(sigils, targets, atlas = null) {
  const wanted = targetMap(targets)
  const traitByHash = new Map((atlas?.traits || []).map(item => [String(item.hash || '').replace(/^0x/i, '').toUpperCase(), item.internalId]))
  const traitByName = new Map((atlas?.traits || []).map(item => [normalized(item.displayName), item.internalId]))
  const traitId = (hash, name, explicit) => explicit || traitByHash.get(String(hash || '').replace(/^0x/i, '').toUpperCase()) || traitByName.get(normalized(name)) || ''
  return (sigils || []).filter(item => !item.missing).map(item => ({
    id: `slot:${item.slotId}`, slotId: item.slotId, source: 'inventory', name: item.name, hash: item.hash,
    traits: [
      ...(item.primaryTraitName ? [{ id: traitId(item.primaryTraitHash, item.primaryTraitName, item.primaryTraitId), name: item.primaryTraitName, level: item.primaryTraitLevel }] : []),
      ...(item.secondaryTraitName ? [{ id: traitId(item.secondaryTraitHash, item.secondaryTraitName, item.secondaryTraitId), name: item.secondaryTraitName, level: item.secondaryTraitLevel }] : []),
    ],
  })).filter(candidate => contribution(candidate, wanted).size)
    .sort((a, b) => Number(a.slotId) - Number(b.slotId))
}

const evidenceIndexes = new WeakMap()

function evidenceIndex(evidence) {
  if (!evidence || typeof evidence !== 'object') return new Map()
  if (evidenceIndexes.has(evidence)) return evidenceIndexes.get(evidence)
  const result = new Map()
  for (const curve of evidence.traits || []) {
    const levels = new Map((curve.levels || []).map(item => [Number(item.level || 0), item]))
    result.set(String(curve.traitId || ''), { ...curve, levels })
  }
  evidenceIndexes.set(evidence, result)
  return result
}

function addMetric(metrics, key, value) {
  const number = Number(value)
  if (Number.isFinite(number)) metrics[key] = (metrics[key] || 0) + number
}

function effectLevel(curve, level) {
  if (!curve || level <= 0) return null
  const capped = Math.min(Number(curve.maxLevel || level), Number(level || 0))
  if (curve.levels.has(capped)) return curve.levels.get(capped)
  const available = [...curve.levels.keys()].filter(value => value <= capped).sort((a, b) => b - a)
  return available.length ? curve.levels.get(available[0]) : null
}

function applyEffectTotal(metrics, total) {
  const label = String(total?.label || '').trim()
  const value = Number(total?.value || 0)
  const unit = String(total?.unit || '')
  if (!label || !Number.isFinite(value)) return
  if (label === '攻击力') addMetric(metrics, unit === 'pct' ? 'attackPercent' : 'attackFlat', value)
  else if (label === '最大HP' || label === '体力') addMetric(metrics, unit === 'pct' ? 'hpPercent' : 'hpFlat', value)
  else if (label === '暴击率') addMetric(metrics, 'critRate', value)
  else if (label === '昏厥值') addMetric(metrics, 'stunPower', value)
  else if (label === '普通攻击伤害上限') addMetric(metrics, 'normalCap', value)
  else if (label === '能力伤害上限') addMetric(metrics, 'abilityCap', value)
  else if (label === '奥义伤害上限') addMetric(metrics, 'sbaCap', value)
  else if (label === '奥义连锁伤害上限' || label === '连锁伤害上限') addMetric(metrics, 'chainCap', value)
  else if (label.includes('造成的伤害')) addMetric(metrics, 'outsideCapBonus', value)
  else if (label.includes('冷却时间')) addMetric(metrics, 'cooldownReduction', -value)
  else if (label.includes('防御力')) addMetric(metrics, 'generalDefense', value)
  else if (label.includes('霸体')) addMetric(metrics, 'superArmor', value)
  else if (label.includes('减伤')) addMetric(metrics, 'whiteShield', value)
}

function effectTotalKey(total) {
  return `${String(total?.unit || '')}|${String(total?.label || '').trim()}`
}

function applyEffectTotalDelta(metrics, currentTotals, baselineTotals) {
  const baseline = new Map((baselineTotals || []).map(total => [effectTotalKey(total), Number(total?.value || 0)]))
  for (const total of currentTotals || []) {
    applyEffectTotal(metrics, { ...total, value: Number(total?.value || 0) - Number(baseline.get(effectTotalKey(total)) || 0) })
  }
}

function componentValue(level, index) {
  return Number(level?.components?.[index]?.value || 0)
}

const CONDITIONAL_TRAIT_CURVES = new Map([
  ['SKILL_005_00', 'enmity'],
  ['SKILL_006_00', 'stamina'],
  ['SKILL_036_00', 'garrison'],
  ['SKILL_144_00', 'sturdy'],
])

function exactConditionalCurveValue(scenario, curveName, input) {
  const rows = scenario?.conditionalCurves?.[curveName]
  if (!Array.isArray(rows)) return null
  const node = rows.find(item => Math.abs(Number(item?.x) - input) <= 1e-5)
  const value = Number(node?.y)
  return Number.isFinite(value) ? value : null
}

function markUnresolvedCondition(metrics, traitId, curveName, input) {
  if (!Array.isArray(metrics.unresolvedConditions)) metrics.unresolvedConditions = []
  if (metrics.unresolvedConditions.some(item => item.traitId === traitId)) return
  metrics.unresolvedConditions.push({ traitId, curveName, input })
}

function thresholdCoverage(value, lower, upper, explicitRate) {
  if (value >= upper) return 1
  if (value <= lower) return 0
  const rate = Number(explicitRate)
  return Number.isFinite(rate) ? Math.max(0, Math.min(1, rate)) : null
}

function normalizedCoverage(value, fallback = 1) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? Math.max(0, Math.min(1, parsed)) : fallback
}

function applySpecialTrait(metrics, traitId, level, scenario, traitLevels) {
  const coverage = normalizedCoverage(scenario.coverage)
  const baseAttack = Number(scenario.baseStats?.attack || 0)
  const baseHP = Number(scenario.baseStats?.hp || 0)
  const hpRatio = Math.max(0, Math.min(1, Number(scenario.currentHpRatio ?? 1)))
  switch (traitId) {
    case 'SKILL_005_00':
    case 'SKILL_006_00': {
      const curveName = CONDITIONAL_TRAIT_CURVES.get(traitId)
      const factor = exactConditionalCurveValue(scenario, curveName, hpRatio)
      if (factor == null) markUnresolvedCondition(metrics, traitId, curveName, hpRatio)
      else addMetric(metrics, 'attackPercent', componentValue(level, 0) * factor * coverage)
      break
    }
    case 'SKILL_146_00':
      addMetric(metrics, 'elementalAdvantage', 30 * coverage)
      break
    case 'SKILL_151_00':
      addMetric(metrics, 'supplementalDamage', componentValue(level, 0) * 0.2 * coverage)
      break
    case 'SKILL_233_00':
      {
        const procRate = thresholdCoverage(baseAttack, 20000, 25000, scenario.berserkerProcRate)
        if (procRate == null) markUnresolvedCondition(metrics, traitId, 'berserker-proc-rate', baseAttack)
        else addMetric(metrics, 'supplementalDamage', 20 * procRate * coverage)
      }
      break
    case 'SKILL_234_00':
      {
        const procRate = thresholdCoverage(baseHP, 50000, 80000, scenario.spartanProcRate)
        if (procRate == null) markUnresolvedCondition(metrics, traitId, 'spartan-proc-rate', baseHP)
        else addMetric(metrics, 'supplementalDamage', 20 * procRate * coverage)
      }
      break
    case 'SKILL_144_00':
    case 'SKILL_036_00': {
      const curveName = CONDITIONAL_TRAIT_CURVES.get(traitId)
      const factor = exactConditionalCurveValue(scenario, curveName, hpRatio)
      if (factor == null) markUnresolvedCondition(metrics, traitId, curveName, hpRatio)
      else addMetric(metrics, traitId === 'SKILL_144_00' ? 'stronghold' : 'garrison', componentValue(level, 0) * factor * coverage)
      break
    }
    case 'SKILL_141_00':
      addMetric(metrics, 'whiteShield', componentValue(level, 1) * coverage)
      break
    case 'SKILL_320_00':
      if (hpRatio <= 0.25) {
        addMetric(metrics, 'attackPercent', componentValue(level, 0) * coverage)
        addMetric(metrics, 'normalCap', componentValue(level, 1) * coverage)
        addMetric(metrics, 'abilityCap', componentValue(level, 1) * coverage)
        addMetric(metrics, 'sbaCap', componentValue(level, 1) * coverage)
      }
      break
    case 'SKILL_321_00':
      if (hpRatio >= 0.75) {
        addMetric(metrics, 'attackPercent', componentValue(level, 0) * coverage)
        addMetric(metrics, 'normalCap', componentValue(level, 1) * coverage)
        addMetric(metrics, 'abilityCap', componentValue(level, 1) * coverage)
        addMetric(metrics, 'sbaCap', componentValue(level, 1) * coverage)
      }
      break
    case 'SKILL_323_00':
      if (Number(scenario.odStage || 0) > 0) {
        addMetric(metrics, 'normalCap', componentValue(level, 0) * coverage)
        addMetric(metrics, 'abilityCap', componentValue(level, 0) * coverage)
        addMetric(metrics, 'sbaCap', componentValue(level, 0) * coverage)
      }
      break
    case 'SKILL_324_00':
      addMetric(metrics, 'outsideCapBonus', componentValue(level, 0) * coverage)
      break
    case 'SKILL_325_00':
      if (scenario.endgameQuest !== false) {
        addMetric(metrics, 'attackPercent', componentValue(level, 0) * coverage)
        addMetric(metrics, 'generalDefense', componentValue(level, 1) * coverage)
        addMetric(metrics, 'normalCap', componentValue(level, 2) * coverage)
        addMetric(metrics, 'abilityCap', componentValue(level, 2) * coverage)
        addMetric(metrics, 'sbaCap', componentValue(level, 2) * coverage)
      }
      break
    case 'SKILL_326_00':
      addMetric(metrics, 'outsideCapBonus', componentValue(level, 0) * coverage)
      break
    default:
      break
  }
  if (traitLevels.get('BF78FBFC') >= 20 && traitLevels.get('46EE3116') >= 15 && !metrics.crabSetActive) {
    metrics.crabSetActive = true
    addMetric(metrics, 'attackBuff', 10 * coverage)
    addMetric(metrics, 'normalCap', 5 * coverage)
    addMetric(metrics, 'abilityCap', 5 * coverage)
    addMetric(metrics, 'sbaCap', 5 * coverage)
    addMetric(metrics, 'critRate', 10 * coverage)
    addMetric(metrics, 'defenseBuff', 2 * coverage)
    addMetric(metrics, 'supplementalDamage', 5 * coverage)
    addMetric(metrics, 'outsideCapBonus', 2 * coverage)
  }
}

function traitLevelMap(picked, fixedBonuses = []) {
  const result = new Map()
  for (const bonus of fixedBonuses || []) {
    if (bonus?.traitId && Number(bonus.level || 0) > 0) result.set(String(bonus.traitId), (result.get(String(bonus.traitId)) || 0) + Number(bonus.level))
  }
  for (const candidate of picked || []) for (const trait of candidate.traits || []) {
    const id = String(trait.id || '')
    if (id && Number(trait.level || 0) > 0) result.set(id, (result.get(id) || 0) + Number(trait.level))
  }
  return result
}

function fixedTraitLevelMap(fixedBonuses = []) {
  const result = new Map()
  for (const bonus of fixedBonuses || []) {
    const id = String(bonus?.traitId || '')
    const level = Number(bonus?.level || 0)
    if (id && level > 0) result.set(id, (result.get(id) || 0) + level)
  }
  return result
}

const DEFENSE_ZONE_METRICS = Object.freeze({
  common: 'generalDefense',
  'stout-heart': 'superArmor',
  stronghold: 'stronghold',
  garrison: 'garrison',
  'gray-white-shield': 'whiteShield',
  independent: 'otherReduction',
  'defense-up': 'defenseBuff',
})

const DEFENSE_ZONE_TRAITS = Object.freeze({
  'stout-heart': 'SKILL_044_00',
  stronghold: 'SKILL_144_00',
  garrison: 'SKILL_036_00',
  'gray-white-shield': 'SKILL_141_00',
})

function applyFixedDefenseZones(metrics, zones, fixedLevels, curves) {
  for (const zone of zones || []) {
    if (zone?.included !== true) continue
    const key = String(zone.key || '')
    const metric = DEFENSE_ZONE_METRICS[key]
    if (!metric) continue
    const representedByTrait = DEFENSE_ZONE_TRAITS[key]
    if (representedByTrait && Number(fixedLevels.get(representedByTrait) || 0) > 0 && curves.has(representedByTrait)) continue
    addMetric(metrics, metric, Number(zone.reduction || 0))
  }
}

function actionCapKey(actionType) {
  return ({ normal: 'normalCap', ability: 'abilityCap', sba: 'sbaCap', chain: 'chainCap' })[actionType] || 'normalCap'
}

function evaluateCombatTraitLevels(levels, scenario) {
  const metrics = { crabSetActive: false, unresolvedConditions: [] }
  for (const total of scenario.fixedTotals || []) applyEffectTotal(metrics, total)
  const curves = evidenceIndex(scenario.evidence)
  const fixedLevels = fixedTraitLevelMap(scenario.fixedBonuses)
  applyFixedDefenseZones(metrics, scenario.fixedDefenseZones, fixedLevels, curves)
  for (const [traitId, rawLevel] of levels) {
    const curve = curves.get(traitId)
    const level = effectLevel(curve, rawLevel)
    if (!level) continue
    if (!CONDITIONAL_TRAIT_CURVES.has(traitId)) {
      const fixedLevel = effectLevel(curve, fixedLevels.get(traitId) || 0)
      applyEffectTotalDelta(metrics, level.totals || [], fixedLevel?.totals || [])
    }
    applySpecialTrait(metrics, traitId, level, scenario, levels)
  }

  const baseAttack = Math.max(1, Number(scenario.baseStats?.attack || 1))
  const baseHP = Math.max(1, Number(scenario.baseStats?.hp || 1))
  const attack = (baseAttack + Number(metrics.attackFlat || 0)) * (1 + Number(metrics.attackPercent || 0) / 100)
  const hp = (baseHP + Number(metrics.hpFlat || 0)) * Math.max(0, 1 + Number(metrics.hpPercent || 0) / 100)
  const critRate = Math.max(0, Math.min(100, Number(scenario.baseStats?.critRate || 0) + Number(metrics.critRate || 0)))
  const criticalMultiplier = 1 + critRate / 100 * Math.max(0, Number(scenario.criticalDamageBonus ?? 20)) / 100
  const capBonus = Number(metrics[actionCapKey(scenario.actionType)] || 0)
  const baseCap = Math.max(1, Number(scenario.baseDamageCap || 1))
  const effectiveCap = baseCap * (1 + capBonus / 100)
  const baseUncapped = Math.max(1, Number(scenario.baseUncappedDamage || baseCap / 0.95))
  const uncapped = baseUncapped * (attack / baseAttack) * criticalMultiplier
  const damageBase = Math.min(uncapped, effectiveCap)
  const damage = calculateDamageFormula({
    damageCap: damageBase,
    elementalAdvantage: Number(metrics.elementalAdvantage || 0) / 100,
    outsideCapBonus: Number(metrics.outsideCapBonus || 0) / 100,
    attackBuff: Number(metrics.attackBuff || 0) / 100,
    defenseDown: Number(scenario.defenseDown || 0) / 100,
    supplementalDamage: Number(metrics.supplementalDamage || 0) / 100,
    odStage: Number(scenario.odStage || 0), berserk: scenario.berserk === true,
    disableAttackDefenseInOD: scenario.disableAttackDefenseInOD !== false,
    wallReduction: Number(scenario.wallReduction || 0) / 100,
  })
  const incoming = calculateIncomingDamage({
    baseDamage: Number(scenario.incomingDamage || 100000),
    attackDown: Number(scenario.attackDown || 0) / 100,
    generalDefense: Number(metrics.generalDefense || 0) / 100,
    defenseBuff: Number(metrics.defenseBuff || 0) / 100,
    stronghold: Number(metrics.stronghold || 0) / 100,
    garrison: Number(metrics.garrison || 0) / 100,
    superArmor: Number(metrics.superArmor || 0) / 100,
    whiteShield: Number(metrics.whiteShield || 0) / 100,
    otherReduction: Number(metrics.otherReduction || 0) / 100,
  })
  const defense = incoming.totalReduction * 100
  const minimumHP = Math.max(0, Number(scenario.minimumHp || 0))
  const minimumDefense = Math.max(0, Number(scenario.minimumDefense || 0))
  const requiredHits = Math.max(0, Number(scenario.surviveHits || 0))
  const valid = hp >= minimumHP && defense >= minimumDefense && (!requiredHits || hp >= incoming.finalDamage * requiredHits)
  const direction = String(scenario.direction || scenario.actionType || 'normal')
  const utility = Math.max(0, Number(metrics.cooldownReduction || 0)) * 500 + Math.max(0, Number(metrics.stunPower || 0)) * 25
  const survival = hp * 0.02 + defense * 500
  let score = damage.finalDamage
  if (direction === 'stun') score = damage.finalDamage * 0.55 + Math.max(0, Number(metrics.stunPower || 0)) * 1500
  else if (direction === 'cooldown') score = damage.finalDamage * 0.7 + utility * 10
  else if (direction === 'support') score = damage.finalDamage * 0.35 + utility * 4 + survival
  const constraintPenalty = valid ? 0 : 1e15 + Math.max(0, minimumHP - hp) * 1e6 + Math.max(0, minimumDefense - defense) * 1e8
  return {
    valid, score: score - constraintPenalty, rawScore: score,
    metrics: {
      ...metrics, attack, hp, critRate, criticalMultiplier, defense, incomingDamage: incoming.finalDamage,
      actionCapBonus: capBonus, effectiveCap, uncappedDamage: uncapped,
      finalDamage: damage.finalDamage, elementalAdvantage: Number(metrics.elementalAdvantage || 0),
      defenseBuff: Number(metrics.defenseBuff || 0), whiteShield: Number(metrics.whiteShield || 0),
      crabSetActive: metrics.crabSetActive === true,
    },
  }
}

export function evaluateCombatBuild(picked, scenario = {}) {
  return evaluateCombatTraitLevels(traitLevelMap(picked, scenario.fixedBonuses), scenario)
}

function combatTraitTotals(picked, scenario) {
  const curves = evidenceIndex(scenario.evidence)
  return [...traitLevelMap(picked, scenario.fixedBonuses).entries()]
    .map(([traitId, rawLevel]) => {
      const curve = curves.get(traitId)
      const effective = Math.min(Number(curve?.maxLevel || rawLevel), rawLevel)
      return { name: curve?.name || traitId, traitId, level: rawLevel, effective, weight: 1, cap: Number(curve?.maxLevel || effective) }
    })
    .sort((left, right) => right.effective - left.effective || left.name.localeCompare(right.name, 'en'))
}

function combatCandidateSignature(candidate) {
  return (candidate.traits || [])
    .filter(trait => trait?.id && Number(trait.level || 0) > 0)
    .map(trait => `${trait.id}:${Number(trait.level)}`)
    .sort()
    .join('|')
}

function combatStateSignature(picked) {
  const levels = traitLevelMap(picked)
  return [...levels.entries()].sort(([left], [right]) => left.localeCompare(right, 'en')).map(([id, level]) => `${id}:${level}`).join('|')
}

function compareCombatStates(left, right) {
  if (left.combat.valid !== right.combat.valid) return left.combat.valid ? -1 : 1
  return right.combat.score - left.combat.score || left.picked.length - right.picked.length || pickedSignature(left.picked).localeCompare(pickedSignature(right.picked), 'en')
}

function prepareCombatCandidates(candidates, scenario, slotCount) {
  const baseline = evaluateCombatBuild([], scenario)
  const special = new Set(['SKILL_146_00', 'SKILL_151_00', 'SKILL_233_00', 'SKILL_234_00', 'SKILL_141_00', 'BF78FBFC', '46EE3116'])
  const ranked = (candidates || []).filter(candidate => combatCandidateSignature(candidate)).map(candidate => {
    const combat = evaluateCombatBuild([candidate], scenario)
    const hasSpecial = (candidate.traits || []).some(trait => special.has(String(trait.id || '')))
    return { candidate, combat, hasSpecial, gain: combat.rawScore - baseline.rawScore }
  }).sort((left, right) => Number(right.hasSpecial) - Number(left.hasSpecial) || right.gain - left.gain || candidateKey(left.candidate).localeCompare(candidateKey(right.candidate), 'en'))

  const byEffect = new Map()
  for (const row of ranked) {
    const signature = combatCandidateSignature(row.candidate)
    const bucket = byEffect.get(signature) || []
    const limit = row.candidate.source === 'inventory' ? slotCount : 1
    if (bucket.length < limit) bucket.push(row)
    byEffect.set(signature, bucket)
  }
  const reduced = [...byEffect.values()].flat()
  const always = reduced.filter(row => row.hasSpecial)
  const regular = reduced.filter(row => !row.hasSpecial).slice(0, 88)
  return [...new Map([...always, ...regular].map(row => [row.candidate.id, row.candidate])).values()]
    .sort((left, right) => candidateKey(left).localeCompare(candidateKey(right), 'en'))
}

function combatSlotReasons(picked, scenario) {
  const rows = []
  let previous = evaluateCombatBuild([], scenario)
  for (let index = 0; index < picked.length; index++) {
    const current = evaluateCombatBuild(picked.slice(0, index + 1), scenario)
    const gain = current.rawScore - previous.rawScore
    rows.push({
      index: index + 1,
      gain,
      traits: (picked[index].traits || []).map(trait => `${trait.name} Lv${trait.level}`).join(' + '),
    })
    previous = current
  }
  return rows
}

function optimizerCoverageBounds(scenario = {}) {
  const rawCenter = Number(scenario.coverage ?? 1)
  const center = Number.isFinite(rawCenter) ? Math.max(0, Math.min(1, rawCenter)) : 1
  const raw = Array.isArray(scenario.coverageRange) ? scenario.coverageRange : [Math.max(0, center - 0.15), Math.min(1, center + 0.15)]
  const rawLow = Number(raw[0] ?? center)
  const rawHigh = Number(raw[1] ?? center)
  const low = Number.isFinite(rawLow) ? Math.max(0, Math.min(1, rawLow)) : center
  const highCandidate = Number.isFinite(rawHigh) ? Math.max(0, Math.min(1, rawHigh)) : center
  const high = Math.max(low, highCandidate)
  return { low, high }
}

function combatCoverageScores(picked, scenario) {
  const bounds = optimizerCoverageBounds(scenario)
  return {
    ...bounds,
    lowScore: evaluateCombatBuild(picked, { ...scenario, coverage: bounds.low }).rawScore,
    highScore: evaluateCombatBuild(picked, { ...scenario, coverage: bounds.high }).rawScore,
  }
}

function solveCombatRanked(candidates, slotCount, limit, scenario) {
  const pool = prepareCombatCandidates(candidates, scenario, slotCount)
  if (!pool.length) return []
  const reusable = pool.every(item => item.source === 'catalog' || item.source === 'table-exact')
  const beamWidth = Math.max(160, Math.min(720, limit * 72))
  let exploredStates = 1
  let beam = [{ picked: [], nextIndex: 0, combat: evaluateCombatBuild([], scenario) }]
  const finalists = []

  for (let depth = 0; depth < slotCount; depth++) {
    const expanded = []
    for (const state of beam) {
      for (let index = state.nextIndex; index < pool.length; index++) {
        const picked = [...state.picked, pool[index]]
        expanded.push({ picked, nextIndex: reusable ? index : index + 1, combat: evaluateCombatBuild(picked, scenario) })
      }
    }
    exploredStates += expanded.length
    if (!expanded.length) break
    expanded.sort(compareCombatStates)
    const unique = new Map()
    for (const state of expanded) {
      const signature = combatStateSignature(state.picked)
      const bucket = unique.get(signature) || []
      if (bucket.length < 2) bucket.push(state)
      unique.set(signature, bucket)
      if ([...unique.values()].reduce((sum, rows) => sum + rows.length, 0) >= beamWidth) break
    }
    beam = [...unique.values()].flat().sort(compareCombatStates).slice(0, beamWidth)
    finalists.push(...beam.slice(0, Math.max(limit * 3, 20)))
  }

  const uniqueFinal = new Map()
  for (const state of finalists.sort(compareCombatStates)) {
    if (!state.picked.length) continue
    const signature = pickedSignature(state.picked)
    if (!uniqueFinal.has(signature)) uniqueFinal.set(signature, state)
  }
  return [...uniqueFinal.values()].sort(compareCombatStates).slice(0, limit).map(state => ({
    score: state.combat.rawScore,
    picked: state.picked,
    totals: combatTraitTotals(state.picked, scenario),
    method: 'combat-beam', exact: false, exploredStates,
    combat: state.combat,
    coverageScores: combatCoverageScores(state.picked, scenario),
    slotReasons: combatSlotReasons(state.picked, scenario),
  }))
}

const EQUIPMENT_STAGE_ORDER = Object.freeze({ weapon: 0, wrightstone: 1, summons: 2, mastery: 3 })
const OPTIMIZER_DOMAIN_ORDER = Object.freeze({ inventory: 0, catalog: 1, table: 2 })

function compareOptimizerDomains(left, right) {
  return (OPTIMIZER_DOMAIN_ORDER[left.domain] ?? 99) - (OPTIMIZER_DOMAIN_ORDER[right.domain] ?? 99)
    || Number(left.domainRank || 0) - Number(right.domainRank || 0)
    || String(left.domain || '').localeCompare(String(right.domain || ''), 'en')
}

function sortOptimizerDomainResults(results) {
  return results.sort(compareOptimizerDomains)
}
const equipmentExactStateLimit = 250000

function equipmentStageOrder(left, right) {
  const leftRank = EQUIPMENT_STAGE_ORDER[left.key] ?? 99
  const rightRank = EQUIPMENT_STAGE_ORDER[right.key] ?? 99
  return leftRank - rightRank || String(left.key).localeCompare(String(right.key), 'en')
}

function normalizedEquipmentSnapshot(snapshot) {
  if (Number(snapshot?.schemaVersion) !== 1) throw new Error('optimizer.unsupported_equipment_schema')
  const stages = (snapshot.stages || []).map(stage => ({
    key: String(stage?.key || '').trim(),
    choose: Math.max(0, Math.floor(Number(stage?.choose || 0))),
    unique: stage?.unique !== false,
    options: (stage?.options || []).filter(option => option?.id).map(option => ({
      ...option,
      id: String(option.id),
      label: String(option.label || option.id),
      fixedBonuses: Array.isArray(option.fixedBonuses) ? option.fixedBonuses : [],
      fixedTotals: Array.isArray(option.fixedTotals) ? option.fixedTotals : [],
      unresolvedAtoms: Array.isArray(option.unresolvedAtoms) ? option.unresolvedAtoms.map(String) : [],
      baseStatDeltas: option.baseStatDeltas || {},
      variants: (option.variants || []).filter(variant => variant?.id).map(variant => ({
        ...variant,
        id: String(variant.id),
        label: String(variant.label || variant.id),
        fixedBonuses: Array.isArray(variant.fixedBonuses) ? variant.fixedBonuses : [],
        fixedTotals: Array.isArray(variant.fixedTotals) ? variant.fixedTotals : [],
        baseStatDeltas: variant.baseStatDeltas || {},
        unresolvedAtoms: Array.isArray(variant.unresolvedAtoms) ? variant.unresolvedAtoms.map(String) : [],
      })).sort((left, right) => left.id.localeCompare(right.id, 'en')),
    })).sort((left, right) => left.id.localeCompare(right.id, 'en')),
  })).filter(stage => stage.key).sort(equipmentStageOrder)
  return {
    ...snapshot,
    baseStats: snapshot.baseStats || {},
    baseFixedBonuses: Array.isArray(snapshot.baseFixedBonuses) ? snapshot.baseFixedBonuses : [],
    baseFixedTotals: Array.isArray(snapshot.baseFixedTotals) ? snapshot.baseFixedTotals : [],
    baseDefenseZones: Array.isArray(snapshot.baseDefenseZones) ? snapshot.baseDefenseZones : [],
    stages,
  }
}

export function boundedCombinations(options, count, unique = true, budget = equipmentExactStateLimit) {
  const limit = Math.max(1, Math.floor(Number(budget) || 1))
  if (count === 0) return { rows: [[]], exact: true, visited: 1 }
  if (!Array.isArray(options) || !options.length || (unique && options.length < count)) return { rows: [], exact: true, visited: 1 }
  const rows = []
  let exact = true
  let visited = 0
  const visit = (picked, start) => {
    visited++
    if (picked.length === count) {
      if (rows.length >= limit) {
        exact = false
        return false
      }
      rows.push(picked.slice())
      return true
    }
    const needed = count - picked.length
    if (unique && options.length - start < needed) return true
    for (let index = start; index < options.length; index++) {
      picked.push(options[index])
      const keepGoing = visit(picked, unique ? index + 1 : index)
      picked.pop()
      if (!keepGoing) return false
    }
    return true
  }
  visit([], 0)
  return { rows, exact, visited }
}

function equipmentSelectionCompatible(selection, final = false) {
  const byStage = new Map()
  for (const item of selection) {
    const ids = byStage.get(item.stage) || new Set()
    ids.add(String(item.id))
    byStage.set(item.stage, ids)
  }
  for (const item of selection) {
    for (const [stage, rawAllowed] of Object.entries(item.requires || {})) {
      const selected = byStage.get(stage)
      if (!selected) {
        if (final) return false
        continue
      }
      const allowed = new Set((Array.isArray(rawAllowed) ? rawAllowed : [rawAllowed]).map(String))
      if (![...selected].some(id => allowed.has(id))) return false
    }
  }
  return true
}

function equipmentOptionCandidate(option, stage) {
  const traits = [
    ...(option.traits || []),
    ...(option.fixedBonuses || []).map(bonus => ({
      id: String(bonus?.traitId || ''),
      name: String(bonus?.name || bonus?.traitId || ''),
      level: Number(bonus?.level || 0),
    })),
  ].filter(trait => trait.id && Number(trait.level || 0) > 0)
  return { ...option, id: `equipment:${stage}:${option.id}`, equipmentId: option.id, stage, traits }
}

function addBaseStatDeltas(baseStats, selections) {
  const result = { ...(baseStats || {}) }
  for (const selection of selections) {
    for (const [key, rawValue] of Object.entries(selection?.baseStatDeltas || {})) {
      const value = Number(rawValue)
      if (Number.isFinite(value)) result[key] = Number(result[key] || 0) + value
    }
  }
  return result
}

function damageCapStatKey(actionType) {
  return ({ normal: 'normalDamageCap', ability: 'abilityDamageCap', sba: 'skyboundDamageCap', chain: 'chainDamageCap' })[actionType] || 'normalDamageCap'
}

function equipmentPlanSignature(equipment, sigils) {
  return [
    ...equipment.map(item => `${item.stage}:${item.id}:${item.variantId || ''}`),
    ...sigils.map(item => `sigil:${item.id}`),
  ].join('|')
}

function expandEquipmentVariants(selection) {
  let rows = [[]]
  for (const item of selection) {
    const variants = item.variants?.length ? item.variants : [null]
    const next = []
    for (const prefix of rows) for (const variant of variants) {
      next.push([...prefix, variant ? {
        ...item,
        variantId: variant.id,
        variantLabel: variant.label,
        label: `${item.label} · ${variant.label}`,
        fixedBonuses: variant.fixedBonuses,
        fixedTotals: [...(item.fixedTotals || []), ...(variant.fixedTotals || [])],
        baseStatDeltas: Object.fromEntries([...new Set([...Object.keys(item.baseStatDeltas || {}), ...Object.keys(variant.baseStatDeltas || {})])]
          .map(key => [key, Number(item.baseStatDeltas?.[key] || 0) + Number(variant.baseStatDeltas?.[key] || 0)])),
        unresolvedAtoms: [...new Set([...(item.unresolvedAtoms || []), ...(variant.unresolvedAtoms || [])])],
        applyPayload: variant.applyPayload || item.applyPayload,
      } : item])
    }
    rows = next
  }
  return rows
}

function diversifiedEquipmentShortlist(results, limit) {
  const groups = new Map()
  for (const result of results) {
    const weapon = result.equipment.find(item => item.stage === 'weapon')
    const key = String(weapon?.id || 'none')
    const bucket = groups.get(key) || []
    if (bucket.length < Math.max(4, Math.ceil(limit / Math.max(1, groups.size || 1)))) bucket.push(result)
    groups.set(key, bucket)
  }
  const perGroup = Math.max(4, Math.ceil(limit / Math.max(1, groups.size)))
  return [...groups.values()].flatMap(rows => rows.slice(0, perGroup)).sort(compareEquipmentPlans).slice(0, limit)
}

function equipmentApplyPayload(snapshot, equipment, sigils) {
  const grouped = {}
  for (const item of equipment) {
    if (!grouped[item.stage]) grouped[item.stage] = []
    grouped[item.stage].push(item.applyPayload ?? { id: item.id })
  }
  return {
    schemaVersion: Number(snapshot.schemaVersion),
    equipment: grouped,
    sigils: sigils.map(item => item.applyPayload ?? (item.slotId ? { slotId: item.slotId } : { id: item.id })),
  }
}

function equipmentDiffs(snapshot, equipment) {
  const base = snapshot.baseSelection || {}
  const stages = [...new Set(equipment.map(item => item.stage))]
  return stages.map(stage => {
    const before = Array.isArray(base[stage]) ? base[stage].map(String) : (base[stage] ? [String(base[stage])] : [])
    const after = equipment.filter(item => item.stage === stage).map(item => String(item.id))
    return { stage, before, after, changed: before.join('|') !== after.join('|') }
  }).filter(item => item.changed)
}

function evaluateEquipmentPlan(snapshot, equipment, sigils, scenario, exact, exploredStates) {
  const selected = [...equipment, ...sigils]
  const candidateRows = [
    ...equipment.map(item => equipmentOptionCandidate(item, item.stage)),
    ...sigils,
  ]
  const fixedTotals = [...snapshot.baseFixedTotals, ...selected.flatMap(item => item.fixedTotals || [])]
  const adjustedBaseStats = addBaseStatDeltas(snapshot.baseStats, selected)
  const baselineAttack = Math.max(1, Number(snapshot.baseStats?.attack || 1))
  const capStatKey = damageCapStatKey(scenario.actionType)
  const baselineCapPercent = Number(snapshot.baseStats?.[capStatKey] || 0)
  const adjustedCapPercent = Number(adjustedBaseStats?.[capStatKey] || 0)
  const rawDamageCap = Number(scenario.baseDamageCap || 1) / Math.max(0.01, 1 + baselineCapPercent / 100)
  const adjustedScenario = {
    ...scenario,
    baseStats: adjustedBaseStats,
    baseDamageCap: Math.max(1, rawDamageCap * Math.max(0.01, 1 + adjustedCapPercent / 100)),
    baseUncappedDamage: Math.max(1, Number(scenario.baseUncappedDamage || 1) * Math.max(1, Number(adjustedBaseStats.attack || 1)) / baselineAttack),
    fixedTotals,
    fixedBonuses: snapshot.baseFixedBonuses,
    fixedDefenseZones: snapshot.baseDefenseZones,
  }
  const combat = evaluateCombatBuild(candidateRows, adjustedScenario)
  const coverageScores = combatCoverageScores(candidateRows, adjustedScenario)
  const unresolvedAtoms = [...new Set(selected.flatMap(item => item.unresolvedAtoms || []).map(String))].sort((a, b) => a.localeCompare(b, 'en'))
  return {
    score: combat.rawScore,
    exact,
    method: exact ? 'equipment-exact' : 'equipment-budgeted',
    exploredStates,
    equipment,
    sigils,
    picked: sigils,
    combat,
    coverageScores,
    totals: combatTraitTotals(candidateRows, adjustedScenario),
    unresolvedAtoms,
    applyPayload: equipmentApplyPayload(snapshot, equipment, sigils),
    equipmentDiffs: equipmentDiffs(snapshot, equipment),
    evidence: {
      schemaVersion: snapshot.schemaVersion,
      dataVersion: snapshot.dataVersion || '',
      formulaVersion: snapshot.formulaVersion || '',
      inputHash: snapshot.inputHash || '',
      tableHash: snapshot.tableHash || '',
      catalogHash: snapshot.catalogHash || '',
    },
  }
}

function compareEquipmentPlans(left, right) {
  if (left.combat.valid !== right.combat.valid) return left.combat.valid ? -1 : 1
  return right.score - left.score || equipmentPlanSignature(left.equipment, left.sigils).localeCompare(equipmentPlanSignature(right.equipment, right.sigils), 'en')
}

function resultChangeCount(result, scenario) {
  const equipmentChanges = (result.equipmentDiffs || []).length
  const { baseSigils, final } = finalSlotPlan(result, scenario)
  let sigilChanges = 0
  for (let index = 0; index < final.length; index++) {
    const current = final[index]
    const previous = baseSigils[index]
    const currentID = String(current?.slotId || current?.id || current?.hash || current?.name || '')
    const previousID = String(previous?.slotId || previous?.id || previous?.hash || previous?.name || '')
    if (currentID !== previousID) sigilChanges++
  }
  return equipmentChanges + sigilChanges
}

function annotateOptimizerAlternatives(results, scenario) {
  if (!results.length) return results
  const deterministicTieBreak = (left, right) => equipmentPlanSignature(left.equipment || [], left.picked || []).localeCompare(equipmentPlanSignature(right.equipment || [], right.picked || []), 'en')
  const lowOrder = [...results].sort((left, right) => Number(right.coverageScores?.lowScore ?? right.score) - Number(left.coverageScores?.lowScore ?? left.score) || deterministicTieBreak(left, right))
  const highOrder = [...results].sort((left, right) => Number(right.coverageScores?.highScore ?? right.score) - Number(left.coverageScores?.highScore ?? left.score) || deterministicTieBreak(left, right))
  const lowRanks = new Map(lowOrder.map((item, index) => [item, index + 1]))
  const highRanks = new Map(highOrder.map((item, index) => [item, index + 1]))
  const alternatives = results.slice(1)
  const leastChange = alternatives.slice().sort((left, right) => resultChangeCount(left, scenario) - resultChangeCount(right, scenario) || Number(right.score) - Number(left.score))[0]
  const preserveEquipment = alternatives.find(item => !(item.equipmentDiffs || []).length)
  const robustCoverage = alternatives.filter(item => lowRanks.get(item) === highRanks.get(item))
    .sort((left, right) => Math.min(Number(right.coverageScores?.lowScore || right.score), Number(right.coverageScores?.highScore || right.score)) - Math.min(Number(left.coverageScores?.lowScore || left.score), Number(left.coverageScores?.highScore || left.score)) || Number(right.score) - Number(left.score))[0]
  return results.map((result, index) => {
    const groups = []
    if (index === 0) groups.push('primary')
    if (result === leastChange) groups.push('least-change')
    if (result === preserveEquipment) groups.push('preserve-equipment')
    if (result === robustCoverage) groups.push('robust-coverage')
    const lowRank = lowRanks.get(result) || index + 1
    const highRank = highRanks.get(result) || index + 1
    return {
      ...result,
      alternativeGroups: groups,
      changeCount: resultChangeCount(result, scenario),
      coverageSensitivity: {
        low: Number(result.coverageScores?.low ?? optimizerCoverageBounds(scenario).low),
        high: Number(result.coverageScores?.high ?? optimizerCoverageBounds(scenario).high),
        lowRank,
        highRank,
        stable: lowRank === highRank,
      },
    }
  })
}

function enumerateEquipmentSelections(stages, budget = equipmentExactStateLimit) {
  let selections = [[]]
  let exact = true
  let exploredStates = 1
  for (const stage of stages) {
    if (stage.choose > 0 && (!stage.options.length || (stage.unique && stage.options.length < stage.choose))) {
      return { selections: [], exact: true, exploredStates, unsatisfiedStage: stage.key }
    }
    const combinationBudget = Math.max(1, Math.floor(budget / Math.max(1, selections.length)))
    const groups = boundedCombinations(stage.options, stage.choose, stage.unique, combinationBudget)
    exploredStates += groups.visited
    if (!groups.exact) exact = false
    const next = []
    outer: for (const prefix of selections) for (const group of groups.rows) {
      exploredStates++
      if (next.length >= budget) {
        exact = false
        break outer
      }
      const candidate = [...prefix, ...group.map(option => ({ ...option, stage: stage.key }))]
      if (equipmentSelectionCompatible(candidate, false)) next.push(candidate)
    }
    selections = next
    if (!selections.length) break
  }
  selections = selections.filter(selection => equipmentSelectionCompatible(selection, true))
  return { selections, exact, exploredStates }
}

function enumerateSigilSelections(candidates, count, budget = equipmentExactStateLimit) {
  if (count <= 0) return { selections: [[]], exact: true, exploredStates: 1 }
  const sorted = (candidates || []).filter(item => item?.id).slice().sort((left, right) => candidateKey(left).localeCompare(candidateKey(right), 'en'))
  const reusable = sorted.length > 0 && sorted.every(item => item.source === 'catalog' || item.source === 'table-exact')
  const result = boundedCombinations(sorted, count, !reusable, budget)
  return { selections: result.rows, exact: result.exact, exploredStates: result.visited }
}

// Solves the complete versioned equipment domain. Small domains are exhausted
// and checked against the same combat evaluator used by the UI. Large domains
// stop at an explicit state budget and are labelled budgeted; unresolved atoms
// remain visible and never contribute an invented score.
export function solveEquipmentAwareSuggestions({ snapshot, sigilCandidates = [], sigilSlotCount = 12, limit = 10, scenario = {} }) {
  const domain = normalizedEquipmentSnapshot(snapshot)
  const equipmentRows = enumerateEquipmentSelections(domain.stages)
  const remainingBudget = Math.max(1, Math.floor(equipmentExactStateLimit / Math.max(1, equipmentRows.selections.length)))
  const sigilRows = enumerateSigilSelections(sigilCandidates, Math.max(0, Math.floor(Number(sigilSlotCount || 0))), remainingBudget)
  const hasVariants = equipmentRows.selections.some(selection => selection.some(item => item.variants?.length))
  const exact = !hasVariants && equipmentRows.exact && sigilRows.exact && equipmentRows.selections.length * sigilRows.selections.length <= equipmentExactStateLimit
  const exploredStates = equipmentRows.exploredStates + equipmentRows.selections.length * sigilRows.selections.length
  const ranked = []
  let equipmentSelections = equipmentRows.selections
  let sigilSelections = sigilRows.selections
  if (!exact) {
    const shortlistSize = Math.max(40, Math.min(120, Number(limit || 10) * 8))
    const baseShortlist = diversifiedEquipmentShortlist(equipmentRows.selections
      .map(equipment => evaluateEquipmentPlan(domain, equipment, [], scenario, false, exploredStates))
      .sort(compareEquipmentPlans)
      , shortlistSize)
    equipmentSelections = baseShortlist
      .flatMap(result => expandEquipmentVariants(result.equipment))
      .map(equipment => evaluateEquipmentPlan(domain, equipment, [], scenario, false, exploredStates))
      .sort(compareEquipmentPlans)
      .slice(0, shortlistSize)
      .map(result => result.equipment)
    const sigilVariants = scenario.mode === 'combat'
      ? solveCombatRanked(sigilCandidates || [], Math.max(0, Number(sigilSlotCount || 0)), shortlistSize, scenario)
      : solveRanked(sigilCandidates || [], scenario.targets || [], Math.max(0, Number(sigilSlotCount || 0)), shortlistSize)
    sigilSelections = sigilVariants.length ? sigilVariants.map(result => result.picked) : (Number(sigilSlotCount || 0) === 0 ? [[]] : [])
  }
  for (const equipment of equipmentSelections) {
    for (const sigils of sigilSelections) {
      ranked.push(evaluateEquipmentPlan(domain, equipment, sigils, scenario, exact, exploredStates))
      if (!exact && ranked.length >= equipmentExactStateLimit) break
    }
    if (!exact && ranked.length >= equipmentExactStateLimit) break
  }
  const unique = new Map()
  for (const result of ranked.sort(compareEquipmentPlans)) {
    const signature = equipmentPlanSignature(result.equipment, result.sigils)
    if (!unique.has(signature)) unique.set(signature, result)
  }
  const primary = [...unique.values()][0] || null
  const limited = [...unique.values()].slice(0, Math.max(1, Number(limit) || 10))
  const annotated = annotateOptimizerAlternatives(limited, { ...scenario, slotCount: sigilSlotCount })
  const annotatedPrimary = annotated[0] || primary
  return annotated.map((result, index) => ({
    ...result,
    domain: result.domain || scenario.domain || domain.domain,
    rank: index + 1,
    explanation: resultExplanation(result, { ...scenario, slotCount: sigilSlotCount }, annotatedPrimary),
  }))
}

export function solveEquipmentAwareSuggestionsByDomain({ domains, sigilCandidatesByDomain = {}, sigilSlotCount = 12, limit = 10, scenario = {} }) {
  const output = []
  for (const domain of Object.keys(domains || {}).sort((left, right) => compareOptimizerDomains({ domain: left }, { domain: right }))) {
    const results = solveEquipmentAwareSuggestions({ snapshot: domains[domain], sigilCandidates: sigilCandidatesByDomain[domain] || [], sigilSlotCount, limit, scenario: { ...scenario, domain } })
    results.forEach((result, index) => output.push({ ...result, domain, domainRank: index + 1 }))
  }
  return sortOptimizerDomainResults(output)
}

export function solveMixedOptimizerDomains({ inventorySnapshot, domains = {}, sigilCandidatesByDomain = {}, targets = [], sigilSlotCount = 12, limit = 10, scenario = {}, inventoryScenario = null }) {
  const output = []
  if (inventorySnapshot) {
    const inventory = solveEquipmentAwareSuggestions({
      snapshot: inventorySnapshot,
      sigilCandidates: sigilCandidatesByDomain.inventory || [],
      sigilSlotCount,
      limit,
      scenario: { ...scenario, ...(inventoryScenario || {}), targets, domain: 'inventory' },
    })
    inventory.forEach((result, index) => output.push({ ...result, domain: 'inventory', domainRank: index + 1 }))
  }
  const sigilOnly = solveLoadoutSuggestionsByDomain({
    domains,
    targets,
    slotCount: sigilSlotCount,
    limit,
    scenario,
  })
  output.push(...sigilOnly)
  return sortOptimizerDomainResults(output)
}

// TABLE_EXACT deliberately accepts only rows explicitly marked as coming from
// the unpacked table layer. It is a separate evidence domain, not a synonym
// for the legal catalog: an empty table domain is more honest than silently
// treating every catalog row as a proven game-table optimum.
export function buildTableExactCandidates(atlas, targets) {
  const candidates = buildCatalogCandidates(atlas, targets)
  return candidates.filter(item => item.tableExact === true || item.source === 'table-exact')
    .map(item => ({ ...item, source: 'table-exact' }))
}

function marginal(candidate, totals, targets) {
  let score = 0
  for (const [name, value] of contribution(candidate, targets)) {
    const target = [...targets.values()].find(item => item.name === name)
    const before = Math.min(target.cap, totals.get(name) || 0)
    const after = Math.min(target.cap, before + value)
    score += (after - before) * target.weight
  }
  return score
}

function scoreTotals(levels, orderedTargets) {
  return orderedTargets.reduce((sum, target, index) => sum + Math.min(target.cap, levels[index] || 0) * target.weight, 0)
}

function pickedSignature(picked) {
  const counts = new Map()
  for (const item of picked) counts.set(item.id, (counts.get(item.id) || 0) + 1)
  return [...counts.entries()].sort(([left], [right]) => left.localeCompare(right, 'en')).map(([id, count]) => `${id}*${count}`).join('|')
}

const exactStateLimit = 250000

function addRankedState(states, key, candidate, limit) {
  const bucket = states.get(key) || []
  const signature = pickedSignature(candidate.picked)
  if (bucket.some(item => pickedSignature(item.picked) === signature)) return
  bucket.push(candidate)
  bucket.sort((left, right) => left.picked.length - right.picked.length || pickedSignature(left.picked).localeCompare(pickedSignature(right.picked), 'en'))
  if (bucket.length > limit) bucket.length = limit
  states.set(key, bucket)
}

function solveExactRanked(candidates, targets, slotCount, limit) {
  const wanted = targetMap(targets)
  const orderedTargets = [...wanted.values()]
  const usable = (candidates || []).map(item => ({
    item,
    vector: orderedTargets.map(target => contribution(item, wanted).get(target.name) || 0),
  })).filter(group => group.vector.some(Boolean)).sort((left, right) => candidateKey(left.item).localeCompare(candidateKey(right.item), 'en'))
  let states = new Map([['0|' + orderedTargets.map(() => 0).join(','), [{ levels: orderedTargets.map(() => 0), picked: [] }]]])
  let exploredStates = 1

  for (const group of usable) {
    const next = new Map()
    for (const [key, bucket] of states) for (const state of bucket) addRankedState(next, key, state, limit)
    const repeat = group.item.source === 'catalog' || group.item.source === 'table-exact' ? slotCount : 1
    for (const bucket of states.values()) {
      for (const state of bucket) {
        let levels = state.levels
        const picked = state.picked.slice()
        for (let count = 1; count <= repeat && picked.length < slotCount; count++) {
          const advanced = levels.map((value, index) => Math.min(orderedTargets[index].cap, value + group.vector[index]))
          if (advanced.every((value, index) => value === levels[index])) break
          picked.push(group.item)
          levels = advanced
          const key = `${picked.length}|${levels.join(',')}`
          addRankedState(next, key, { levels, picked: picked.slice() }, limit)
        }
      }
    }
    states = next
    const stateCount = [...states.values()].reduce((sum, bucket) => sum + bucket.length, 0)
    exploredStates += stateCount
    if (stateCount > exactStateLimit) throw new Error('optimizer.exact_state_limit')
  }

  const ranked = []
  const signatures = new Set()
  for (const bucket of states.values()) for (const state of bucket) {
    if (!state.picked.length) continue
    const signature = pickedSignature(state.picked)
    if (signatures.has(signature)) continue
    signatures.add(signature)
    ranked.push({ ...state, score: scoreTotals(state.levels, orderedTargets) })
  }
  return ranked
    .sort((left, right) => right.score - left.score || left.picked.length - right.picked.length || pickedSignature(left.picked).localeCompare(pickedSignature(right.picked), 'en'))
    .slice(0, limit)
    .map(resolved => {
      const rawLevels = orderedTargets.map(target => resolved.picked.reduce((sum, item) => sum + (contribution(item, wanted).get(target.name) || 0), 0))
      return {
        score: resolved.score,
        picked: resolved.picked,
        totals: orderedTargets.map((target, index) => ({ ...target, level: rawLevels[index] || 0, effective: resolved.levels[index] || 0 })),
        method: 'exact-dp', exact: true, exploredStates,
      }
    })
}

function solveGreedyOnce(candidates, targets, slotCount, excludedFirstId = '') {
  const wanted = targetMap(targets)
  const totals = new Map([...wanted.values()].map(item => [item.name, 0]))
  const picked = []
  const pool = candidates.slice()
  while (picked.length < slotCount && pool.length) {
    let bestIndex = -1
    let bestScore = 0
    let bestKey = ''
    for (let index = 0; index < pool.length; index++) {
      const item = pool[index]
      if (!picked.length && item.id === excludedFirstId) continue
      const score = marginal(item, totals, wanted)
      const key = candidateKey(item)
      if (score > bestScore || (score === bestScore && score > 0 && key < bestKey)) {
        bestIndex = index
        bestScore = score
        bestKey = key
      }
    }
    if (bestIndex < 0 || bestScore <= 0) break
    const candidate = pool[bestIndex]
    picked.push(candidate)
    for (const [name, value] of contribution(candidate, wanted)) totals.set(name, (totals.get(name) || 0) + value)
    if (candidate.source === 'inventory') pool.splice(bestIndex, 1)
  }
  const score = [...wanted.values()].reduce((sum, target) => sum + Math.min(target.cap, totals.get(target.name) || 0) * target.weight, 0)
  return {
    score, picked,
    totals: [...wanted.values()].map(target => ({ ...target, level: totals.get(target.name) || 0, effective: Math.min(target.cap, totals.get(target.name) || 0) })),
    method: 'heuristic-fallback', exact: false, exploredStates: 0,
  }
}

function solveRanked(candidates, targets, slotCount, limit) {
  try {
    return solveExactRanked(candidates, targets, slotCount, limit)
  } catch (error) {
    if (error?.message !== 'optimizer.exact_state_limit') throw error
    const primary = solveGreedyOnce(candidates, targets, slotCount)
    const variants = [primary]
    const rankedIds = [...new Set([...(primary.picked || []).map(item => item.id), ...(candidates || []).map(item => item.id)])].filter(Boolean).sort((a, b) => String(a).localeCompare(String(b), 'en'))
    for (const id of rankedIds.slice(0, Math.max(0, limit * 2 - 1))) variants.push(solveGreedyOnce(candidates, targets, slotCount, id))
    return variants
  }
}

function finalSlotPlan(result, scenario = {}) {
  const slotCount = Math.max(1, Number(scenario.slotCount) || 12)
  const baseSigils = Array.isArray(scenario.baseSigils) ? scenario.baseSigils : []
  const picked = (result.picked || []).slice(0, slotCount)
  const usedInventorySlots = new Set(picked
    .filter(item => item.source === 'inventory')
    .map(item => Number(item.slotId || 0))
    .filter(Boolean))
  const final = picked.slice()
  for (const base of baseSigils) {
    if (final.length >= slotCount) break
    if (Number(base?.slotId || 0) && usedInventorySlots.has(Number(base.slotId))) continue
    final.push({ ...base, retained: true })
  }
  return { baseSigils, final }
}

const COMPARISON_METRICS = Object.freeze({
  attack: '攻击', hp: 'HP', critRate: '暴击率', actionCapBonus: '动作上限', effectiveCap: '有效上限',
  uncappedDamage: '未封顶伤害', finalDamage: '最终伤害', defense: '减伤', incomingDamage: '承伤',
  cooldownReduction: '冷却', stunPower: '眩晕', attackBuff: '攻击 Buff', defenseBuff: '防御 Buff',
  outsideCapBonus: '上限外伤害', supplementalDamage: '追击',
})

function combatMetricChanges(result, primary) {
  if (!result?.combat?.metrics || !primary?.combat?.metrics || result === primary) return []
  const changes = []
  for (const [key, label] of Object.entries(COMPARISON_METRICS)) {
    const current = Number(result.combat.metrics[key] || 0)
    const reference = Number(primary.combat.metrics[key] || 0)
    const delta = current - reference
    if (Math.abs(delta) <= 1e-6) continue
    changes.push({ key, label, delta, current, reference })
  }
  return changes.sort((left, right) => Math.abs(right.delta) - Math.abs(left.delta)).slice(0, 6)
}

function resultExplanation(result, scenario = {}, domainPrimary = null) {
  const top = (result.totals || []).filter(item => item.effective > 0).sort((a, b) => b.effective - a.effective || a.name.localeCompare(b.name, 'en'))
  const covered = top.slice(0, 3).map(item => `${item.name} Lv${item.effective}`).join('、')
  const { baseSigils, final } = finalSlotPlan(result, scenario)
  const comparisonSigils = domainPrimary && domainPrimary !== result ? finalSlotPlan(domainPrimary, scenario).final : baseSigils
  const slotChangeRows = final.map((item, index) => {
    const base = comparisonSigils[index]
    const coveredTraits = (item.traits || []).filter(trait => trait.level > 0).map(trait => trait.name).join(' + ')
    const same = item.retained === true || (base && (
      (Number(base.slotId || 0) > 0 && Number(base.slotId) === Number(item.slotId || 0)) ||
      String(base.id || '') === String(item.hash || item.id || '') ||
      String(base.name || '') === String(item.name || '')
    ))
    const reason = result.slotReasons?.[index]
    const gainZh = reason ? `，预估收益 +${Math.max(0, reason.gain).toFixed(0)}` : ''
    const gainEn = reason ? `, estimated gain +${Math.max(0, reason.gain).toFixed(0)}` : ''
    return {
      zh: same ? `${index + 1}: 保留 ${item.name}` : `${index + 1}: ${base?.name || '空槽'} → ${item.name}${coveredTraits ? `（${coveredTraits}）` : ''}${gainZh}`,
      en: same ? `${index + 1}: keep ${item.name}` : `${index + 1}: ${base?.name || 'empty'} -> ${item.name}${coveredTraits ? ` (${coveredTraits})` : ''}${gainEn}`,
    }
  })
  const formula = calculateDamageFormula({ damageCap: 1, ...(scenario.directionProfile || {}) })
  const combat = result.combat
  const capDistance = combat ? damageCapDistance(combat.metrics.finalDamage, combat.metrics.effectiveCap) : null
  const unresolved = combat?.metrics?.unresolvedConditions || []
  const unresolvedZh = unresolved.length ? ` 当前场景有 ${unresolved.length} 个条件效果未命中精确表节点，已从评分中排除。` : ''
  const unresolvedEn = unresolved.length ? ` ${unresolved.length} conditional effects did not match an exact table node and were excluded from scoring.` : ''
  const capEvidenceZh = scenario.actionCapEvidence === 'global-baseline' ? ' 当前动作倍率未命中角色上限表节点，使用全局基线估算。' : ''
  const capEvidenceEn = scenario.actionCapEvidence === 'global-baseline' ? ' The action rate did not match a character-cap table node, so the global baseline is used as an estimate.' : ''
  const metricChanges = combatMetricChanges(result, domainPrimary)
  const scoreDelta = domainPrimary && domainPrimary !== result ? Number(result.score || 0) - Number(domainPrimary.score || 0) : 0
  const inventorySlots = (result.picked || []).filter(item => Number(item.slotId || 0) > 0).length
  const inventoryReason = scenario.domain === 'inventory'
    ? `使用 ${inventorySlots} 个存档实例，按 SlotID 去重且不复用库存。`
    : scenario.domain === 'table'
      ? '只使用明确标记为解包表精确行的候选。'
      : '使用工具合法目录构造，不代表当前存档持有。'
  const inventoryReasonEn = scenario.domain === 'inventory'
    ? `Uses ${inventorySlots} save instances, deduplicated by SlotID without inventory reuse.`
    : scenario.domain === 'table'
      ? 'Uses only candidates explicitly marked as exact unpacked-table rows.'
      : 'Uses legal tool-catalog constructions and does not claim current-save ownership.'
  return {
    character: String(scenario.character || ''),
    direction: String(scenario.direction || 'custom'),
    domain: String(scenario.domain || ''),
    summary: combat ? `预估单次伤害 ${combat.metrics.finalDamage.toFixed(0)} · 上限距离 ${capDistance.percent.toFixed(1)}% · HP ${combat.metrics.hp.toFixed(0)}` : (covered || '未覆盖目标技能'),
    slotChanges: slotChangeRows.map(item => item.zh).join('；'),
    slotChangesEn: slotChangeRows.map(item => item.en).join('; '),
    comparisonBasis: domainPrimary && domainPrimary !== result ? 'domain-primary' : 'current-loadout',
    scoreDelta,
    metricChanges,
    inventoryReason,
    inventoryReasonEn,
    scenarioSummary: `action=${scenario.actionType || 'custom'}; uptime=${Math.round(normalizedCoverage(scenario.coverage) * 100)}%; hp=${Math.round(Number(scenario.currentHp ?? 100))}%; od=${Number(scenario.odStage || 0)}; berserk=${scenario.berserk === true}`,
    evidenceSource: `${scenario.evidence?.dataVersion || 'GBFR 2.0.2'} / ${scenario.evidence?.formulaVersion || RELINK_FORMULA_VERSION} / ${scenario.actionCapEvidence || 'catalog-formula'}`,
    formulaEvidence: { version: formula.formulaVersion || RELINK_FORMULA_VERSION, stateMultiplier: formula.stateMultiplier, stableCapMargin: formula.stableCapMargin, actionCapEvidence: scenario.actionCapEvidence || '' },
    limitation: combat ? `按当前公式、角色方向、覆盖率与生存约束排名；角色独有动作帧和未审计触发仍按估算处理。${capEvidenceZh}${unresolvedZh}` : (result.exact ? '当前目标技能的槽位覆盖已穷尽；不代表完整战斗伤害最优。' : '状态空间超过安全预算，结果为启发式覆盖建议。'),
    limitationEn: combat ? `Ranked by the current formula, character direction, uptime, and survival constraints. Character-specific frames and unaudited triggers remain estimates.${capEvidenceEn}${unresolvedEn}` : (result.exact ? 'The reachable slot coverage for these targets is exhausted; this is not a complete combat-damage optimum.' : 'The state space exceeded the safety budget; this is a heuristic coverage suggestion.'),
  }
}

export function solveLoadoutSuggestionsByDomain({ domains, targets, slotCount = 12, limit = 10, scenario = {} }) {
  const output = []
  for (const [domain, candidates] of Object.entries(domains || {})) {
    const results = solveLoadoutSuggestions({ candidates, targets, slotCount, limit, scenario: { ...scenario, domain } })
    results.forEach((result, index) => output.push({ ...result, domain, domainRank: index + 1 }))
  }
  return sortOptimizerDomainResults(output)
}

export function solveLoadoutSuggestions({ candidates, targets, slotCount = 12, limit = 10, scenario = {} }) {
  const variants = scenario.mode === 'combat'
    ? solveCombatRanked(candidates || [], slotCount, limit, scenario)
    : solveRanked(candidates || [], targets || [], slotCount, limit)
  const unique = new Map()
  for (const result of variants) {
    const key = result.picked.map(item => item.id).sort().join('|')
    if (!unique.has(key)) unique.set(key, result)
  }
  const ranked = [...unique.values()]
    .sort((a, b) => b.score - a.score || a.picked.map(candidateKey).join('|').localeCompare(b.picked.map(candidateKey).join('|'), 'en'))
    .slice(0, limit)
  const primary = ranked[0] || null
  const annotated = annotateOptimizerAlternatives(ranked, { ...scenario, slotCount })
  const annotatedPrimary = annotated[0] || primary
  return annotated.map((result, index) => ({ ...result, domain: result.domain || scenario.domain || '', rank: index + 1, explanation: resultExplanation(result, { ...scenario, slotCount }, annotatedPrimary) }))
}
