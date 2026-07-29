import assert from 'node:assert/strict'
import test from 'node:test'
import { boundedCombinations, buildCatalogCandidates, buildInventoryCandidates, buildTableExactCandidates, evaluateCombatBuild, solveEquipmentAwareSuggestions, solveEquipmentAwareSuggestionsByDomain, solveLoadoutSuggestions, solveLoadoutSuggestionsByDomain } from './loadoutOptimizer.js'

const targets = [{ name: '伤害上限', weight: 3, cap: 65 }, { name: '暴击率', weight: 2, cap: 45 }]

test('catalog candidates use only audited constructible shells and legal secondary rows', () => {
  const atlas = { sigils: [
    { internalId: 'A', hash: '1', displayName: 'A+', constructible: true, supportsSecondaryTrait: true, primaryTraitName: '伤害上限', firstTraitMaxLevel: 15, secondaryTraits: [{ internalId: 'CRIT', displayName: '暴击率', maxLevel: 15 }] },
    { internalId: 'B', hash: '2', displayName: 'B', constructible: false, primaryTraitName: '伤害上限', firstTraitMaxLevel: 99, secondaryTraits: [] },
  ] }
  const candidates = buildCatalogCandidates(atlas, targets)
  assert.equal(candidates.length, 1)
  assert.deepEqual(candidates[0].traits, [{ id: undefined, name: '伤害上限', level: 15 }, { id: 'CRIT', name: '暴击率', level: 15 }])
})

test('plus shells never become illegal single-trait candidates', () => {
  const atlas = { sigils: [
    { internalId: 'PLUS', hash: '1', displayName: 'Damage Cap V+', constructible: true, supportsSecondaryTrait: true, primaryTraitName: '伤害上限', firstTraitMaxLevel: 15, secondaryTraits: [{ internalId: 'HP', displayName: '体力', maxLevel: 15 }] },
    { internalId: 'BROKEN', hash: '2', displayName: 'Broken V+', constructible: true, supportsSecondaryTrait: true, primaryTraitName: '伤害上限', firstTraitMaxLevel: 15, secondaryTraits: [] },
    { internalId: 'SINGLE', hash: '3', displayName: 'Damage Cap V', constructible: true, supportsSecondaryTrait: false, primaryTraitName: '伤害上限', firstTraitMaxLevel: 15, secondaryTraits: [] },
  ] }
  const candidates = buildCatalogCandidates(atlas, [{ name: '伤害上限', weight: 1, cap: 65 }])
  assert.equal(candidates.length, 2)
  const plus = candidates.find(item => item.sigilId === 'PLUS')
  assert.equal(plus.secondaryTraitId, 'HP')
  assert.equal(plus.secondaryTraitName, '体力')
  assert.equal(candidates.some(item => item.sigilId === 'BROKEN'), false)
  assert.equal(candidates.find(item => item.sigilId === 'SINGLE').secondaryTraitId, '')
})

test('inventory candidates preserve independent real instances', () => {
  const candidates = buildInventoryCandidates([
    { slotId: 4, name: 'A+', primaryTraitName: '伤害上限', primaryTraitLevel: 15, secondaryTraitName: '暴击率', secondaryTraitLevel: 15 },
    { slotId: 9, name: 'A+', primaryTraitName: '伤害上限', primaryTraitLevel: 15, secondaryTraitName: '暴击率', secondaryTraitLevel: 15 },
    { slotId: 11, name: 'Other', primaryTraitName: '防御力', primaryTraitLevel: 15 },
  ], targets)
  assert.deepEqual(candidates.map(item => item.slotId), [4, 9])
})

test('suggestions cap effective levels and never reuse an inventory instance', () => {
  const candidates = buildInventoryCandidates([
    { slotId: 1, name: 'Cap+', primaryTraitName: '伤害上限', primaryTraitLevel: 40, secondaryTraitName: '暴击率', secondaryTraitLevel: 20 },
    { slotId: 2, name: 'Cap+', primaryTraitName: '伤害上限', primaryTraitLevel: 40, secondaryTraitName: '暴击率', secondaryTraitLevel: 20 },
  ], targets)
  const [result] = solveLoadoutSuggestions({ candidates, targets, slotCount: 12 })
  assert.deepEqual(result.picked.map(item => item.slotId), [1, 2])
  assert.equal(result.totals.find(item => item.name === '伤害上限').effective, 65)
  assert.equal(result.totals.find(item => item.name === '暴击率').effective, 40)
})

test('catalog suggestions are deterministic when unlimited copies are allowed', () => {
  const candidates = [
    { id: 'b', source: 'catalog', name: 'B', traits: [{ name: '伤害上限', level: 10 }] },
    { id: 'a', source: 'catalog', name: 'A', traits: [{ name: '伤害上限', level: 10 }, { name: '暴击率', level: 5 }] },
  ]
  const first = solveLoadoutSuggestions({ candidates, targets, slotCount: 3 })
  const second = solveLoadoutSuggestions({ candidates: candidates.slice().reverse(), targets, slotCount: 3 })
  assert.deepEqual(first, second)
  assert.deepEqual(first[0].picked.map(item => item.id), ['a', 'a', 'a'])
  assert.equal(first[0].method, 'exact-dp')
  assert.equal(first[0].exact, true)
})

test('table-exact suggestions use the same unlimited table-row semantics as catalog', () => {
  const candidates = [{ id: 'table', source: 'table-exact', name: 'Table', traits: [{ name: '伤害上限', level: 15 }] }]
  const [result] = solveLoadoutSuggestions({ candidates, targets: [{ name: '伤害上限', cap: 65, weight: 1 }], slotCount: 5 })
  assert.equal(result.picked.length, 5)
  assert.equal(result.totals[0].effective, 65)
})

test('exact coverage solver beats the greedy marginal counterexample', () => {
  const counterexampleTargets = [{ name: 'A', weight: 1, cap: 10 }, { name: 'B', weight: 1, cap: 10 }]
  const candidates = [
    { id: 'balanced', source: 'inventory', slotId: 1, name: 'Balanced', traits: [{ name: 'A', level: 6 }, { name: 'B', level: 6 }] },
    { id: 'a', source: 'inventory', slotId: 2, name: 'A', traits: [{ name: 'A', level: 10 }] },
    { id: 'b', source: 'inventory', slotId: 3, name: 'B', traits: [{ name: 'B', level: 10 }] },
  ]
  const [result] = solveLoadoutSuggestions({ candidates, targets: counterexampleTargets, slotCount: 2, limit: 3 })
  assert.equal(result.score, 20)
  assert.equal(result.exact, true)
  assert.deepEqual(result.picked.map(item => item.id).sort(), ['a', 'b'])
})

test('exact inventory solver stays deterministic with repeated equivalent instances', () => {
  const repeated = Array.from({ length: 80 }, (_, index) => ({
    id: `slot:${index + 1}`, slotId: index + 1, source: 'inventory', name: 'Cap+',
    traits: [{ name: '伤害上限', level: 15 }, { name: '暴击率', level: index % 3 === 0 ? 15 : 10 }],
  }))
  const forward = solveLoadoutSuggestions({ candidates: repeated, targets, slotCount: 12, limit: 5 })
  const reverse = solveLoadoutSuggestions({ candidates: repeated.slice().reverse(), targets, slotCount: 12, limit: 5 })
  assert.deepEqual(forward, reverse)
  assert.equal(forward[0].exact, true)
})

test('suggestions expose deterministic ranked alternatives and scenario evidence', () => {
  const candidates = Array.from({ length: 12 }, (_, index) => ({
    id: `slot:${index + 1}`, slotId: index + 1, source: 'inventory', name: `Cap${index + 1}`,
    traits: [{ name: '伤害上限', level: index < 2 ? 15 : 10 }],
  }))
  const results = solveLoadoutSuggestions({ candidates, targets: [{ name: '伤害上限', cap: 65, weight: 1 }], slotCount: 2, scenario: { character: '伊欧', direction: 'output', domain: 'inventory' } })
  assert.ok(results.length > 1)
  assert.deepEqual(results.map(item => item.rank), results.map((_, index) => index + 1))
  assert.equal(results[0].explanation.character, '伊欧')
  assert.equal(results[0].explanation.direction, 'output')
  assert.equal(results[0].explanation.formulaEvidence.version, '2.0.2-v2-tardis98')
  assert.equal(results[0].explanation.comparisonBasis, 'current-loadout')
  assert.equal(results[1].explanation.comparisonBasis, 'domain-primary')
  assert.ok(results[1].explanation.scoreDelta <= 0)
  assert.match(results[1].explanation.inventoryReason, /SlotID/)
  assert.match(results[1].explanation.evidenceSource, /2\.0\.2/)
})

test('exact solver returns real ranked states instead of exclusion-only variants', () => {
  const candidates = [
    { id: 'a', source: 'inventory', slotId: 1, name: 'A', traits: [{ name: '伤害上限', level: 15 }] },
    { id: 'b', source: 'inventory', slotId: 2, name: 'B', traits: [{ name: '伤害上限', level: 14 }] },
    { id: 'c', source: 'inventory', slotId: 3, name: 'C', traits: [{ name: '伤害上限', level: 13 }] },
  ]
  const results = solveLoadoutSuggestions({ candidates, targets: [{ name: '伤害上限', cap: 65, weight: 1 }], slotCount: 2, limit: 3, scenario: { baseSigils: [{ name: 'Old 1' }, { name: 'Old 2' }] } })
  assert.deepEqual(results.map(item => item.score), [29, 28, 27])
  assert.match(results[0].explanation.slotChanges, /Old 1 → A/)
  assert.match(results[0].explanation.slotChangesEn, /Old 1 -> A/)
})

test('slot explanation includes unchanged trailing slots kept by editor staging', () => {
  const candidates = [
    { id: 'slot:20', source: 'inventory', slotId: 20, name: 'New', traits: [{ name: '伤害上限', level: 65 }] },
  ]
  const [result] = solveLoadoutSuggestions({
    candidates,
    targets: [{ name: '伤害上限', cap: 65, weight: 1 }],
    slotCount: 3,
    scenario: { baseSigils: [
      { id: 'old-1', slotId: 10, name: 'Old 1' },
      { id: 'old-2', slotId: 20, name: 'Old 2' },
      { id: 'old-3', slotId: 30, name: 'Old 3' },
    ] },
  })
  assert.equal(result.picked.length, 1)
  assert.match(result.explanation.slotChanges, /1: Old 1 → New/)
  assert.match(result.explanation.slotChanges, /2: 保留 Old 1/)
  assert.match(result.explanation.slotChanges, /3: 保留 Old 3/)
  assert.doesNotMatch(result.explanation.slotChanges, /Old 2/)
})

test('three-domain solver keeps table evidence separate from catalog and inventory', () => {
  const atlas = { sigils: [{ internalId: 'T', hash: '1', displayName: 'Table+', constructible: true, tableExact: true, supportsSecondaryTrait: false, primaryTraitName: '伤害上限', firstTraitMaxLevel: 15, secondaryTraits: [] }] }
  const table = buildTableExactCandidates(atlas, [{ name: '伤害上限', cap: 65, weight: 1 }])
  assert.equal(table.length, 1)
  const results = solveLoadoutSuggestionsByDomain({ domains: { table, catalog: table, inventory: [{ id: 'slot:1', source: 'inventory', slotId: 1, name: 'Owned', traits: [{ name: '伤害上限', level: 15 }] }] }, targets: [{ name: '伤害上限', cap: 65, weight: 1 }], slotCount: 1 })
  assert.deepEqual(new Set(results.map(item => item.domain)), new Set(['catalog', 'inventory', 'table']))
  assert.deepEqual(results.map(item => item.domain), ['inventory', 'catalog', 'table'])
  assert.ok(results.every(item => item.domainRank === 1))
})

const combatEvidence = {
  dataVersion: '2.0.2',
  traits: [
    { traitId: 'SKILL_020_00', maxLevel: 65, levels: [{ level: 15, totals: [
      { label: '普通攻击伤害上限', unit: 'pct', value: 45 },
      { label: '能力伤害上限', unit: 'pct', value: 45 },
      { label: '奥义伤害上限', unit: 'pct', value: 45 },
    ], components: [] }] },
    { traitId: 'SKILL_000_00', maxLevel: 50, levels: [{ level: 15, totals: [{ label: '攻击力', unit: 'flat', value: 40 }], components: [] }] },
    { traitId: 'SKILL_001_00', maxLevel: 50, levels: [{ level: 15, totals: [{ label: '最大HP', unit: 'flat', value: 3000 }], components: [] }] },
    { traitId: 'SKILL_146_00', maxLevel: 15, levels: [{ level: 15, totals: [], components: [] }] },
    { traitId: 'BF78FBFC', maxLevel: 20, levels: [{ level: 20, totals: [], components: [] }] },
    { traitId: '46EE3116', maxLevel: 15, levels: [{ level: 15, totals: [], components: [] }] },
  ],
}

const combatScenario = {
  mode: 'combat', actionType: 'normal', coverage: 1, evidence: combatEvidence,
  conditionalCurves: {
    enmity: [{ interpolation: 'SmoothSide', x: 0.5, y: 0.3388896 }, { interpolation: 'SmoothSide', x: 1, y: 0 }],
    stamina: [{ interpolation: 'Smooth', x: 0.5, y: 0.2500005 }, { interpolation: 'Smooth', x: 1, y: 1 }],
    garrison: [{ interpolation: 'SmoothSide', x: 0.6, y: 0.5 }, { interpolation: 'Smooth', x: 1, y: 0 }],
    sturdy: [{ interpolation: 'Smooth', x: 0.5, y: 0.2500005 }, { interpolation: 'Smooth', x: 1, y: 1 }],
  },
  baseStats: { attack: 100000, hp: 100000, critRate: 100 },
  baseDamageCap: 100000, baseUncappedDamage: 105263.1579,
  fixedTotals: [], fixedBonuses: [], minimumHp: 0, minimumDefense: 0,
}

test('conditional HP traits use only exact unpacked curve nodes and never leak maximum totals', () => {
  const evidence = { ...combatEvidence, traits: [...combatEvidence.traits,
    { traitId: 'SKILL_005_00', maxLevel: 30, levels: [{ level: 15, totals: [{ label: '效果最大时攻击力', unit: 'pct', value: 70 }], components: [{ value: 70 }] }] },
    { traitId: 'SKILL_006_00', maxLevel: 30, levels: [{ level: 15, totals: [{ label: '效果最大时攻击力', unit: 'pct', value: 50 }, { label: '效果最小时攻击力', unit: 'pct', value: 7.5 }], components: [{ value: 50 }, { value: 7.5 }] }] },
    { traitId: 'SKILL_036_00', maxLevel: 45, levels: [{ level: 15, totals: [{ label: '效果最大时防御力', unit: 'pct', value: 35 }], components: [{ value: 35 }] }] },
    { traitId: 'SKILL_144_00', maxLevel: 30, levels: [{ level: 15, totals: [{ label: '效果最大时防御力', unit: 'pct', value: 22 }, { label: '效果最小时防御力', unit: 'pct', value: 3.3 }], components: [{ value: 22 }, { value: 3.3 }] }] },
  ] }
  const candidates = [
    { id: 'enmity', traits: [{ id: 'SKILL_005_00', level: 15 }] },
    { id: 'stamina', traits: [{ id: 'SKILL_006_00', level: 15 }] },
    { id: 'garrison', traits: [{ id: 'SKILL_036_00', level: 15 }] },
    { id: 'sturdy', traits: [{ id: 'SKILL_144_00', level: 15 }] },
  ]
  const full = evaluateCombatBuild(candidates, { ...combatScenario, evidence, currentHpRatio: 1 })
  assert.equal(full.metrics.attackPercent, 50)
  assert.equal(full.metrics.stronghold, 22)
  assert.equal(full.metrics.garrison, 0)
  assert.equal(full.metrics.generalDefense || 0, 0)
  assert.deepEqual(full.metrics.unresolvedConditions, [])

  const half = evaluateCombatBuild(candidates, { ...combatScenario, evidence, currentHpRatio: 0.5 })
  assert.ok(Math.abs(half.metrics.attackPercent - (70 * 0.3388896 + 50 * 0.2500005)) < 1e-9)
  assert.ok(Math.abs(half.metrics.stronghold - 22 * 0.2500005) < 1e-9)
  assert.equal(half.metrics.generalDefense || 0, 0)

  const unresolved = evaluateCombatBuild(candidates, { ...combatScenario, evidence, currentHpRatio: 0.55 })
  assert.equal(unresolved.metrics.attackPercent || 0, 0)
  assert.equal(unresolved.metrics.stronghold || 0, 0)
  assert.equal(unresolved.metrics.garrison || 0, 0)
  assert.deepEqual(new Set(unresolved.metrics.unresolvedConditions.map(item => item.traitId)), new Set(['SKILL_005_00', 'SKILL_006_00', 'SKILL_036_00', 'SKILL_144_00']))
})

test('threshold proc traits do not invent a linear probability between audited endpoints', () => {
  const evidence = { ...combatEvidence, traits: [...combatEvidence.traits,
    { traitId: 'SKILL_233_00', maxLevel: 15, levels: [{ level: 15, totals: [], components: [{ value: 20000 }, { value: 20 }, { value: 25000 }] }] },
    { traitId: 'SKILL_234_00', maxLevel: 15, levels: [{ level: 15, totals: [], components: [{ value: 50000 }, { value: 20 }, { value: 80000 }] }] },
  ] }
  const candidates = [
    { id: 'berserker', traits: [{ id: 'SKILL_233_00', level: 15 }] },
    { id: 'spartan', traits: [{ id: 'SKILL_234_00', level: 15 }] },
  ]
  const unresolved = evaluateCombatBuild(candidates, { ...combatScenario, evidence, baseStats: { ...combatScenario.baseStats, attack: 22500, hp: 65000 } })
  assert.equal(unresolved.metrics.supplementalDamage || 0, 0)
  assert.deepEqual(new Set(unresolved.metrics.unresolvedConditions.map(item => item.traitId)), new Set(['SKILL_233_00', 'SKILL_234_00']))

  const explicit = evaluateCombatBuild(candidates, { ...combatScenario, evidence, baseStats: { ...combatScenario.baseStats, attack: 22500, hp: 65000 }, berserkerProcRate: 0.4, spartanProcRate: 0.25 })
  assert.equal(explicit.metrics.supplementalDamage, 13)
  assert.deepEqual(explicit.metrics.unresolvedConditions, [])
})

test('fixed gear traits stay in the baseline while candidate levels apply only their incremental totals', () => {
  const evidence = { ...combatEvidence, traits: [...combatEvidence.traits,
    { traitId: 'SKILL_141_00', maxLevel: 15, levels: [
      { level: 10, totals: [{ label: '最大HP', unit: 'pct', value: 10 }], components: [{ value: 5 }, { value: 10 }] },
      { level: 15, totals: [{ label: '最大HP', unit: 'pct', value: 20 }], components: [{ value: 10 }, { value: 20 }] },
    ] },
  ] }
  const fixedOnly = evaluateCombatBuild([], { ...combatScenario, evidence, baseStats: { ...combatScenario.baseStats, hp: 110000 }, fixedBonuses: [{ traitId: 'SKILL_141_00', level: 10 }] })
  assert.equal(fixedOnly.metrics.hp, 110000)
  assert.equal(fixedOnly.metrics.whiteShield, 10)

  const candidate = { id: 'crab', traits: [{ id: 'SKILL_141_00', level: 5 }] }
  const combined = evaluateCombatBuild([candidate], { ...combatScenario, evidence, baseStats: { ...combatScenario.baseStats, hp: 110000 }, fixedBonuses: [{ traitId: 'SKILL_141_00', level: 10 }] })
  assert.ok(Math.abs(combined.metrics.hp - 121000) < 1e-6)
  assert.equal(combined.metrics.whiteShield, 20)
})

test('fixed non-panel totals and audited defense zones participate without duplicating represented traits', () => {
  const scenario = {
    ...combatScenario,
    fixedTotals: [
      { label: '造成的伤害', unit: 'pct', value: 12 },
      { label: '冷却时间', unit: 'pct', value: -8 },
    ],
    fixedDefenseZones: [
      { key: 'common', reduction: 10, included: true },
      { key: 'stronghold', reduction: 22, included: true },
    ],
  }
  const fixed = evaluateCombatBuild([], scenario)
  assert.equal(fixed.metrics.outsideCapBonus, 12)
  assert.equal(fixed.metrics.cooldownReduction, 8)
  assert.equal(fixed.metrics.generalDefense, 10)
  assert.equal(fixed.metrics.stronghold, 22)

  const strongholdEvidence = { ...combatEvidence, traits: [...combatEvidence.traits,
    { traitId: 'SKILL_144_00', maxLevel: 30, levels: [{ level: 15, totals: [{ label: '效果最大时防御力', unit: 'pct', value: 22 }], components: [{ value: 22 }] }] },
  ] }
  const withTrait = evaluateCombatBuild([], { ...scenario, evidence: strongholdEvidence, fixedBonuses: [{ traitId: 'SKILL_144_00', level: 15 }] })
  assert.equal(withTrait.metrics.stronghold, 22)
})

test('combat evaluation uses cap curves instead of trait-level coverage', () => {
  const cap = { id: 'cap', source: 'catalog', name: 'Damage Cap', traits: [{ id: 'SKILL_020_00', name: '伤害上限', level: 15 }] }
  const attack = { id: 'attack', source: 'catalog', name: 'Attack', traits: [{ id: 'SKILL_000_00', name: '攻击力', level: 15 }] }
  const capScore = evaluateCombatBuild([cap], combatScenario)
  const attackScore = evaluateCombatBuild([attack], combatScenario)
  assert.equal(capScore.valid, true)
  assert.ok(capScore.score > attackScore.score)
  assert.equal(capScore.metrics.actionCapBonus, 45)
})

test('combat evaluation applies elemental conversion and crab set only with both pieces', () => {
  const elemental = { id: 'elemental', source: 'catalog', name: 'Elemental', traits: [{ id: 'SKILL_146_00', name: '属性克制转换', level: 15 }] }
  const crabA = { id: 'crab-a', source: 'catalog', name: 'Crab A', traits: [{ id: 'BF78FBFC', name: '可怕的漆黑钳蟹因子', level: 20 }] }
  const crabB = { id: 'crab-b', source: 'catalog', name: 'Crab B', traits: [{ id: '46EE3116', name: '漆黑之谊', level: 15 }] }
  assert.equal(evaluateCombatBuild([elemental], combatScenario).metrics.elementalAdvantage, 30)
  assert.equal(evaluateCombatBuild([crabA], combatScenario).metrics.crabSetActive, false)
  const complete = evaluateCombatBuild([crabA, crabB], combatScenario)
  assert.equal(complete.metrics.crabSetActive, true)
  assert.equal(complete.metrics.defenseBuff, 2)
  assert.equal(complete.metrics.whiteShield, 0)
})

test('combat evaluation rejects a higher damage plan that misses survival constraints', () => {
  const damage = { id: 'cap', source: 'catalog', name: 'Damage Cap', traits: [{ id: 'SKILL_020_00', name: '伤害上限', level: 15 }] }
  const hp = { id: 'hp', source: 'catalog', name: 'HP', traits: [{ id: 'SKILL_001_00', name: '体力', level: 15 }] }
  const constrained = { ...combatScenario, baseStats: { ...combatScenario.baseStats, hp: 98000 }, minimumHp: 100000 }
  assert.equal(evaluateCombatBuild([damage], constrained).valid, false)
  assert.equal(evaluateCombatBuild([hp], constrained).valid, true)
})

test('combat solver ranks formula output and returns deterministic top alternatives', () => {
  const candidates = [
    { id: 'cap', source: 'catalog', name: 'Damage Cap', traits: [{ id: 'SKILL_020_00', name: '伤害上限', level: 15 }] },
    { id: 'attack', source: 'catalog', name: 'Attack', traits: [{ id: 'SKILL_000_00', name: '攻击力', level: 15 }] },
    { id: 'elemental', source: 'catalog', name: 'Elemental', traits: [{ id: 'SKILL_146_00', name: '属性克制转换', level: 15 }] },
  ]
  const first = solveLoadoutSuggestions({ candidates, targets, slotCount: 2, limit: 3, scenario: combatScenario })
  const second = solveLoadoutSuggestions({ candidates: candidates.slice().reverse(), targets, slotCount: 2, limit: 3, scenario: combatScenario })
  assert.deepEqual(first, second)
  assert.equal(first[0].method, 'combat-beam')
  assert.equal(first[0].picked.length, 2)
  assert.ok(first[0].combat.metrics.finalDamage > 0)
  assert.match(first[0].explanation.slotChanges, /预估收益/)
  assert.deepEqual(first[0].alternativeGroups, ['primary'])
  assert.ok(first.every(item => item.coverageSensitivity && Number.isInteger(item.coverageSensitivity.lowRank) && Number.isInteger(item.coverageSensitivity.highRank)))
  assert.ok(first.slice(1).some(item => item.alternativeGroups.includes('least-change')))
  assert.ok(first.slice(1).some(item => item.alternativeGroups.includes('robust-coverage')))
  assert.equal(first[0].explanation.comparisonBasis, 'current-loadout')
})

test('combat solver normalizes non-finite coverage bounds', () => {
  const candidate = { id: 'attack', source: 'catalog', name: 'Attack', traits: [{ id: 'SKILL_000_00', name: '攻击力', level: 15 }] }
  const results = solveLoadoutSuggestions({
    candidates: [candidate], targets, slotCount: 1, limit: 1,
    scenario: { ...combatScenario, coverage: Number.NaN, coverageRange: [Number.NaN, Number.NaN] },
  })
  assert.equal(results.length, 1)
  assert.equal(results[0].coverageSensitivity.low, 1)
  assert.equal(results[0].coverageSensitivity.high, 1)
  assert.ok(Number.isFinite(results[0].coverageScores.lowScore))
  assert.ok(Number.isFinite(results[0].coverageScores.highScore))
  assert.match(results[0].explanation.scenarioSummary, /uptime=100%/)
})

test('combat solver keeps three result domains independent', () => {
  const candidate = { id: 'cap', source: 'catalog', name: 'Damage Cap', traits: [{ id: 'SKILL_020_00', name: '伤害上限', level: 15 }] }
  const results = solveLoadoutSuggestionsByDomain({
    domains: { table: [{ ...candidate, source: 'table-exact' }], catalog: [candidate], inventory: [{ ...candidate, id: 'slot:1', source: 'inventory', slotId: 1 }] },
    targets, slotCount: 1, limit: 2, scenario: combatScenario,
  })
  assert.deepEqual(new Set(results.map(item => item.domain)), new Set(['catalog', 'inventory', 'table']))
  assert.ok(results.every(item => item.method === 'combat-beam'))
  assert.deepEqual(results.map(item => item.domain), ['inventory', 'catalog', 'table'])
  assert.ok(results.every(item => item.domainRank === 1))
})

const equipmentScenario = {
  ...combatScenario,
  baseStats: { attack: 1000, hp: 1000, critRate: 0 },
  baseDamageCap: 1000000,
  baseUncappedDamage: 1000,
}

function equipmentOption(id, attack, applyPayload = { id }) {
  return {
    id,
    label: id,
    baseStatDeltas: { attack },
    fixedBonuses: [],
    fixedTotals: [],
    applyPayload,
    unresolvedAtoms: [],
  }
}

function syntheticEquipmentDomain() {
  return {
    schemaVersion: 1,
    dataVersion: 'test-data-v1',
    formulaVersion: 'test-formula-v1',
    inputHash: 'input-a',
    tableHash: 'table-a',
    catalogHash: 'catalog-a',
    baseStats: { attack: 1000, hp: 1000, critRate: 0 },
    baseFixedBonuses: [],
    baseFixedTotals: [],
    baseDefenseZones: [],
    baseSelection: { weapon: 'weapon-low', wrightstone: 'stone-low', mastery: 'mastery-low', summons: ['summon-low-a', 'summon-low-b'] },
    stages: [
      { key: 'weapon', choose: 1, options: [equipmentOption('weapon-low', 1), equipmentOption('weapon-high', 100)] },
      { key: 'wrightstone', choose: 1, options: [equipmentOption('stone-low', 1), equipmentOption('stone-high', 80)] },
      { key: 'summons', choose: 2, unique: true, options: [equipmentOption('summon-low-a', 1), equipmentOption('summon-low-b', 2), equipmentOption('summon-high-a', 60), equipmentOption('summon-high-b', 50)] },
      { key: 'mastery', choose: 1, options: [equipmentOption('mastery-low', 1), equipmentOption('mastery-high', 40)] },
    ],
  }
}

test('equipment-aware solver matches a small brute-force oracle across every equipment category', () => {
  const sigils = [
    { id: 'sigil-low', source: 'inventory', slotId: 1, name: 'low', traits: [], baseStatDeltas: { attack: 1 }, applyPayload: { slotId: 1 } },
    { id: 'sigil-high', source: 'inventory', slotId: 2, name: 'high', traits: [], baseStatDeltas: { attack: 30 }, applyPayload: { slotId: 2 } },
  ]
  const results = solveEquipmentAwareSuggestions({ snapshot: syntheticEquipmentDomain(), sigilCandidates: sigils, sigilSlotCount: 1, limit: 10, scenario: equipmentScenario })
  assert.equal(results[0].exact, true)
  assert.deepEqual(results[0].equipment.map(item => [item.stage, item.id]), [
    ['weapon', 'weapon-high'], ['wrightstone', 'stone-high'], ['summons', 'summon-high-a'], ['summons', 'summon-high-b'], ['mastery', 'mastery-high'],
  ])
  assert.equal(results[0].sigils[0].id, 'sigil-high')
  assert.equal(results[0].combat.metrics.attack, 1360)
  assert.deepEqual(results[0].applyPayload, {
    schemaVersion: 1,
    equipment: { weapon: [{ id: 'weapon-high' }], wrightstone: [{ id: 'stone-high' }], summons: [{ id: 'summon-high-a' }, { id: 'summon-high-b' }], mastery: [{ id: 'mastery-high' }] },
    sigils: [{ slotId: 2 }],
  })
  assert.deepEqual(new Set(results[0].equipmentDiffs.map(item => item.stage)), new Set(['weapon', 'wrightstone', 'summons', 'mastery']))
})

test('equipment panel attack changes uncapped damage instead of falling back to option id ordering', () => {
  const snapshot = {
    schemaVersion: 1,
    baseStats: { attack: 1000, hp: 1000, critRate: 0 },
    baseFixedBonuses: [], baseFixedTotals: [], baseDefenseZones: [],
    stages: [{ key: 'weapon', choose: 1, options: [equipmentOption('aaa-low', 1), equipmentOption('zzz-high', 500)] }],
  }
  const rows = solveEquipmentAwareSuggestions({ snapshot, sigilCandidates: [], sigilSlotCount: 0, limit: 2, scenario: equipmentScenario })
  assert.equal(rows[0].equipment[0].id, 'zzz-high')
  assert.ok(rows[0].combat.metrics.finalDamage > rows[1].combat.metrics.finalDamage)
})

test('equipment mastery cap deltas adjust the absolute cap without reapplying the baseline cap', () => {
  const snapshot = {
    schemaVersion: 1,
    baseStats: { attack: 1000, hp: 1000, critRate: 0, normalDamageCap: 20 },
    baseFixedBonuses: [], baseFixedTotals: [], baseDefenseZones: [],
    stages: [{ key: 'mastery', choose: 1, options: [
      { ...equipmentOption('low', 0), baseStatDeltas: { normalDamageCap: 0 } },
      { ...equipmentOption('high', 0), baseStatDeltas: { normalDamageCap: 30 } },
    ] }],
  }
  const scenario = { ...equipmentScenario, actionType: 'normal', baseDamageCap: 1200, baseUncappedDamage: 100000 }
  const rows = solveEquipmentAwareSuggestions({ snapshot, sigilCandidates: [], sigilSlotCount: 0, limit: 2, scenario })
  assert.equal(rows[0].equipment[0].id, 'high')
  assert.equal(rows[0].combat.metrics.effectiveCap, 1500)
  assert.equal(rows[1].combat.metrics.effectiveCap, 1200)
})

test('weapon replacement-skill variants participate in ranking and deployment payloads', () => {
  const snapshot = {
    schemaVersion: 1,
    baseStats: { attack: 1000, hp: 1000, critRate: 0 },
    baseFixedBonuses: [], baseFixedTotals: [], baseDefenseZones: [],
    stages: [{ key: 'weapon', choose: 1, options: [{
      ...equipmentOption('weapon', 0),
      variants: [
        { id: 'current', label: 'current', fixedBonuses: [], unresolvedAtoms: [], applyPayload: { weaponSlotId: 7, weaponSkillHashes: ['A', 'B', 'C', 'D', 'E'] } },
        { id: 'attack', label: 'attack', fixedBonuses: [], unresolvedAtoms: [], baseStatDeltas: { attack: 300 }, applyPayload: { weaponSlotId: 7, weaponSkillHashes: ['F', 'G', 'H', 'I', 'J'] } },
      ],
    }] }],
  }
  snapshot.stages[0].options[0].variants[1].fixedTotals = []
  snapshot.stages[0].options[0].variants[1].fixedBonuses = []
  const rows = solveEquipmentAwareSuggestions({ snapshot, sigilCandidates: [], sigilSlotCount: 0, limit: 2, scenario: equipmentScenario })
  assert.equal(rows[0].equipment[0].variantId, 'attack')
  assert.deepEqual(rows[0].applyPayload.equipment.weapon[0].weaponSkillHashes, ['F', 'G', 'H', 'I', 'J'])
})

test('equipment-aware Top 10 is deterministic regardless of option and sigil input order', () => {
  const domain = syntheticEquipmentDomain()
  const sigils = [
    { id: 'a', source: 'inventory', slotId: 1, name: 'A', traits: [], baseStatDeltas: { attack: 10 } },
    { id: 'b', source: 'inventory', slotId: 2, name: 'B', traits: [], baseStatDeltas: { attack: 20 } },
  ]
  const first = solveEquipmentAwareSuggestions({ snapshot: domain, sigilCandidates: sigils, sigilSlotCount: 1, limit: 10, scenario: equipmentScenario })
  const reversed = { ...domain, stages: domain.stages.slice().reverse().map(stage => ({ ...stage, options: stage.options.slice().reverse() })) }
  const second = solveEquipmentAwareSuggestions({ snapshot: reversed, sigilCandidates: sigils.slice().reverse(), sigilSlotCount: 1, limit: 10, scenario: equipmentScenario })
  assert.deepEqual(first, second)
  assert.equal(first.length, 10)
})

test('equipment-aware domains stay independent and supersets are monotonic', () => {
  const inventory = syntheticEquipmentDomain()
  const catalog = structuredClone(inventory)
  catalog.stages.find(item => item.key === 'weapon').options.push(equipmentOption('weapon-catalog', 200))
  const rows = solveEquipmentAwareSuggestionsByDomain({ domains: { inventory, catalog }, sigilCandidatesByDomain: { inventory: [], catalog: [] }, sigilSlotCount: 0, limit: 3, scenario: equipmentScenario })
  const inventoryTop = rows.find(item => item.domain === 'inventory' && item.domainRank === 1)
  const catalogTop = rows.find(item => item.domain === 'catalog' && item.domainRank === 1)
  assert.ok(catalogTop.score >= inventoryTop.score)
  assert.equal(inventoryTop.equipment.some(item => item.id === 'weapon-catalog'), false)
  assert.equal(catalogTop.equipment.some(item => item.id === 'weapon-catalog'), true)
})

test('unresolved equipment atoms are excluded from score and disclosed', () => {
  const snapshot = syntheticEquipmentDomain()
  snapshot.stages[0].options.push({ ...equipmentOption('weapon-unknown', 0), unresolvedAtoms: ['unknown-action-coefficient'] })
  const rows = solveEquipmentAwareSuggestions({ snapshot, sigilCandidates: [], sigilSlotCount: 0, limit: 100, scenario: equipmentScenario })
  const unknown = rows.find(row => row.equipment.some(item => item.id === 'weapon-unknown'))
  assert.ok(unknown)
  assert.deepEqual(unknown.unresolvedAtoms, ['unknown-action-coefficient'])
  assert.equal(unknown.combat.metrics.attack, 1230)
})

test('equipment combination budget stops inside DFS instead of materializing the full search space', () => {
  const options = Array.from({ length: 100 }, (_, index) => ({ id: `option-${String(index).padStart(3, '0')}` }))
  const result = boundedCombinations(options, 12, true, 120)
  assert.equal(result.rows.length, 120)
  assert.equal(result.exact, false)
  assert.ok(result.visited < 10000)
})

test('a required equipment stage with too few options fails closed', () => {
  const snapshot = syntheticEquipmentDomain()
  snapshot.stages.push({ key: 'required-missing-stage', choose: 2, options: [equipmentOption('only-one', 99999)] })
  const results = solveEquipmentAwareSuggestions({ snapshot, sigilCandidates: [], sigilSlotCount: 0, limit: 10, scenario: equipmentScenario })
  assert.deepEqual(results, [])
})

test('equipment requirements prevent invalid weapon and bound-wrightstone pairings', () => {
  const snapshot = {
    schemaVersion: 1,
    stages: [
      { key: 'weapon', choose: 1, options: [equipmentOption('weapon-a', 10), equipmentOption('weapon-b', 20)] },
      { key: 'wrightstone', choose: 1, options: [
        { ...equipmentOption('stone-a', 1000), requires: { weapon: ['weapon-a'] } },
        { ...equipmentOption('stone-b', 1), requires: { weapon: ['weapon-b'] } },
      ] },
    ],
  }
  const results = solveEquipmentAwareSuggestions({ snapshot, sigilCandidates: [], sigilSlotCount: 0, limit: 10, scenario: equipmentScenario })
  assert.equal(results.some(result => result.equipment.some(item => item.id === 'weapon-b') && result.equipment.some(item => item.id === 'stone-a')), false)
  assert.equal(results[0].equipment.some(item => item.id === 'weapon-a'), true)
  assert.equal(results[0].equipment.some(item => item.id === 'stone-a'), true)
})
