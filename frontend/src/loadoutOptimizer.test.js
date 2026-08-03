import assert from 'node:assert/strict'
import test from 'node:test'
import { boundedCombinations, buildCatalogCandidates, buildInventoryCandidates, buildOptimizerTargetCatalog, buildTableExactCandidates, evaluateCombatBuild, solveEquipmentAwareSuggestions, solveEquipmentAwareSuggestionsByDomain, solveFixedCharacterRoute, solveLoadoutSuggestions, solveLoadoutSuggestionsByDomain, synthesizeOwnedFirstSuggestion } from './loadoutOptimizer.js'

const targets = [{ name: '伤害上限', weight: 3, cap: 65 }, { name: '暴击率', weight: 2, cap: 45 }]

test('target catalog includes equipment-only final-panel skills', () => {
  const result = buildOptimizerTargetCatalog({ traits: [{ internalId: 'HP', displayName: '体力', maxLevel: 50 }] }, {
    baseFixedBonuses: [{ traitId: 'BASE', name: '因子强化', level: 1 }],
    stages: [{ key: 'weapon', options: [{ fixedBonuses: [{ traitId: 'CAP', name: '超凡破限', level: 55 }], variants: [{ fixedBonuses: [{ traitId: 'NOVA', name: '浩劫新星', level: 35 }] }] }] }],
  })
  assert.deepEqual(result.map(item => [item.internalId, item.displayName, item.sourceMaxLevel]).sort((a, b) => a[0].localeCompare(b[0], 'en')), [
    ['BASE', '因子强化', 1],
    ['CAP', '超凡破限', 55],
    ['HP', '体力', 50],
    ['NOVA', '浩劫新星', 35],
  ])
})

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

test('manufacturing never emits the same trait in both primary and secondary slots', () => {
  const atlas = {
    writableSecondaryTraits: [{ internalId: 'HP', displayName: '体力', maxLevel: 15 }],
    traits: [{ internalId: 'HP', displayName: '体力', maxLevel: 15 }],
    sigils: [{
      internalId: 'HP_PLUS', hash: '01', displayName: '体力 V+', constructible: true,
      supportsSecondaryTrait: true, primaryTraitId: 'HP', primaryTraitName: '体力', firstTraitMaxLevel: 15,
      secondaryTraits: [{ internalId: 'HP', displayName: '体力', maxLevel: 15 }],
    }],
  }

  assert.equal(buildCatalogCandidates(atlas, [{ traitId: 'HP', name: '体力', cap: 30, weight: 1 }]).length, 0)
})

test('manufacturing can use audited writable secondary pairs without promoting them to table-exact evidence', () => {
  const atlas = {
    traits: [
      { internalId: 'PRIMARY', hash: 'A1', displayName: '攻击力', maxLevel: 15 },
      { internalId: 'NATURAL', hash: 'B2', displayName: '暴击率', maxLevel: 15 },
      { internalId: 'WRITABLE', hash: 'C3', displayName: '明镜止水', maxLevel: 15 },
    ],
    writableSecondaryTraits: [
      { internalId: 'NATURAL', hash: 'B2', displayName: '暴击率', maxLevel: 15 },
      { internalId: 'WRITABLE', hash: 'C3', displayName: '明镜止水', maxLevel: 15 },
    ],
    sigils: [{
      internalId: 'ATTACK_PLUS', hash: 'D4', displayName: '攻击力 V+', constructible: true,
      tableExact: true, supportsSecondaryTrait: true, primaryTraitId: 'PRIMARY', primaryTraitName: '攻击力',
      firstTraitMaxLevel: 15, allowedSigilLevels: [15], defaultSigilLevel: 15,
      secondaryTraits: [{ internalId: 'NATURAL', hash: 'B2', displayName: '暴击率', maxLevel: 15 }],
    }],
  }
  const candidates = buildCatalogCandidates(atlas, [{ traitId: 'WRITABLE', name: '明镜止水', weight: 1, cap: 15 }])
  assert.equal(candidates.length, 1)
  assert.equal(candidates[0].secondaryTraitId, 'WRITABLE')
  assert.equal(candidates[0].naturalSecondary, false)
  assert.equal(candidates[0].tableExact, false)
  assert.equal(buildTableExactCandidates(atlas, [{ traitId: 'WRITABLE', name: '明镜止水', weight: 1, cap: 15 }]).length, 0)
})

test('catalog and table candidates exclude character sigils owned by another character', () => {
  const characterTargets = [{ name: '战气', weight: 1, cap: 15 }]
  const atlas = { sigils: [
    { internalId: 'GENERIC', hash: '1', displayName: 'Generic', category: 'normal', constructible: true, supportsSecondaryTrait: false, primaryTraitId: 'GENERIC_TRAIT', primaryTraitName: '战气', firstTraitMaxLevel: 15, tableExact: true },
    { internalId: 'IO', hash: '2', displayName: 'Io Warpath', category: 'character_sigil', allowedOwnerCodes: ['PL0400'], constructible: true, supportsSecondaryTrait: false, primaryTraitId: 'IO_TRAIT', primaryTraitName: '战气', firstTraitMaxLevel: 15, tableExact: true },
    { internalId: 'FOREIGN', hash: '3', displayName: 'Foreign Warpath', category: 'character_sigil', allowedOwnerCodes: ['PL0300'], constructible: true, supportsSecondaryTrait: false, primaryTraitId: 'FOREIGN_TRAIT', primaryTraitName: '战气', firstTraitMaxLevel: 15, tableExact: true },
    { internalId: 'DLC_OWNED', hash: '4', displayName: 'DLC Io Warpath', category: 'dlc_supplement', allowedOwnerCodes: ['PL0400'], constructible: true, supportsSecondaryTrait: false, primaryTraitId: 'DLC_IO_TRAIT', primaryTraitName: '战气', firstTraitMaxLevel: 15, tableExact: false },
    { internalId: 'DLC_FOREIGN', hash: '5', displayName: 'DLC Foreign Warpath', category: 'dlc_supplement', allowedOwnerCodes: ['PL2900'], constructible: true, supportsSecondaryTrait: false, primaryTraitId: 'DLC_FOREIGN_TRAIT', primaryTraitName: '战气', firstTraitMaxLevel: 15, tableExact: false },
  ] }

  assert.deepEqual(buildCatalogCandidates(atlas, characterTargets, 'PL0400').map(item => item.sigilId), ['DLC_OWNED', 'GENERIC', 'IO'])
  assert.deepEqual(buildTableExactCandidates(atlas, characterTargets, 'PL0400').map(item => item.sigilId), ['GENERIC', 'IO'])
  assert.deepEqual(buildCatalogCandidates(atlas, characterTargets, '').map(item => item.sigilId), ['GENERIC'])
})

test('manufactured character-exclusive traits stay in the primary slot even when a bad table row offers them as secondary', () => {
  const characterTargets = [{ name: '黑龙的战气', weight: 1, cap: 15 }]
  const atlas = {
    traits: [
      { internalId: 'SANDALPHON_WARPATH', displayName: '黑龙的战气', hash: 'AA' },
      { internalId: 'DAMAGE_CAP', displayName: '伤害上限', hash: 'BB' },
    ],
    sigils: [
      {
        internalId: 'SANDALPHON_EXCLUSIVE', hash: '10', displayName: '黑龙的战气+',
        category: 'character_sigil', allowedOwnerCodes: ['PL2900'], constructible: true,
        supportsSecondaryTrait: false, primaryTraitId: 'SANDALPHON_WARPATH',
        primaryTraitName: '黑龙的战气', firstTraitMaxLevel: 15,
      },
      {
        internalId: 'CORRUPT_PLUS_SHELL', hash: '11', displayName: '伤害上限 V+',
        category: 'normal', constructible: true, supportsSecondaryTrait: true,
        primaryTraitId: 'DAMAGE_CAP', primaryTraitName: '伤害上限', firstTraitMaxLevel: 15,
        secondaryTraits: [{ internalId: 'SANDALPHON_WARPATH', displayName: '黑龙的战气', hash: 'AA', maxLevel: 15 }],
      },
    ],
  }

  const candidates = buildCatalogCandidates(atlas, characterTargets, 'PL2900')
  assert.equal(candidates.some(item => item.secondaryTraitId === 'SANDALPHON_WARPATH'), false)
  assert.equal(candidates.some(item => item.primaryTraitId === 'SANDALPHON_WARPATH'), true)
})

test('fixed character routes assign twelve primary slots in linear time and reuse distinct inventory first', () => {
  const routeTargets = [
    { traitId: 'T0', name: 'T0', targetLevel: 45, cap: 45, weight: 100, slotCount: 3 },
    ...Array.from({ length: 9 }, (_, index) => ({
      traitId: `T${index + 1}`,
      name: `T${index + 1}`,
      targetLevel: 15,
      cap: 15,
      weight: 100,
      slotCount: 1,
    })),
  ]
  const inventoryCandidates = Array.from({ length: 1404 }, (_, index) => ({
    id: `slot:${index + 1}`,
    slotId: index + 1,
    source: 'inventory',
    name: `Owned T0 ${index + 1}`,
    primaryTraitId: 'T0',
    primaryTraitLevel: 15,
    traits: [{ id: 'T0', name: 'T0', level: 15 }],
  }))
  const catalogCandidates = routeTargets.map(target => ({
    id: `catalog:${target.traitId}`,
    sigilId: `catalog:${target.traitId}`,
    source: 'catalog',
    name: `${target.name} shell`,
    primaryTraitId: target.traitId,
    primaryLevel: 15,
    traits: [{ id: target.traitId, name: target.name, level: 15 }],
  }))

  const started = performance.now()
  const [result] = solveFixedCharacterRoute({ inventoryCandidates, catalogCandidates, targets: routeTargets })
  const elapsed = performance.now() - started
  assert.ok(elapsed < 1000, `fixed route assignment took ${elapsed.toFixed(1)}ms`)
  assert.equal(result.method, 'fixed-route-linear')
  assert.equal(result.picked.length, 12)
  assert.equal(result.ownedCount, 3)
  assert.equal(result.constructedCount, 9)
  assert.equal(result.exact, true)
  assert.deepEqual(result.picked.slice(0, 3).map(item => item.slotId), [1, 2, 3])
  assert.equal(new Set(result.picked.filter(item => item.source === 'inventory').map(item => item.slotId)).size, 3)
  assert.deepEqual(result.totals.map(item => item.effective), [45, 15, 15, 15, 15, 15, 15, 15, 15, 15])
})

test('fixed character routes preserve exact awakening shells instead of matching only the primary trait', () => {
  const candidates = [
    { id: 'slot:1', slotId: 1, source: 'inventory', sigilId: 'REGULAR', secondaryTraitId: 'REGULAR_SECONDARY', primaryTraitId: 'CHARACTER', primaryTraitLevel: 15, traits: [{ id: 'CHARACTER', name: '角色词条', level: 15 }] },
    { id: 'slot:2', slotId: 2, source: 'inventory', sigilId: 'AWAKENING', secondaryTraitId: 'WRONG_SECONDARY', primaryTraitId: 'CHARACTER', primaryTraitLevel: 15, traits: [{ id: 'CHARACTER', name: '角色词条', level: 15 }] },
    { id: 'slot:3', slotId: 3, source: 'inventory', sigilId: 'AWAKENING', secondaryTraitId: 'FIXED_SECONDARY', primaryTraitId: 'CHARACTER', primaryTraitLevel: 15, traits: [{ id: 'CHARACTER', name: '角色词条', level: 15 }] },
  ]
  const [result] = solveFixedCharacterRoute({
    inventoryCandidates: candidates,
    catalogCandidates: [],
    targets: [{ traitId: 'CHARACTER', name: '角色词条', targetLevel: 30, slotCount: 2, exactSigilIds: ['REGULAR', 'AWAKENING'], exactSecondaryTraitIds: ['REGULAR_SECONDARY', 'FIXED_SECONDARY'] }],
    slotCount: 2,
  })
  assert.equal(result.exact, true)
  assert.deepEqual(result.picked.map(item => item.sigilId), ['REGULAR', 'AWAKENING'])
  assert.deepEqual(result.picked.map(item => item.slotId), [1, 3])
})

test('inventory candidates preserve independent real instances', () => {
  const candidates = buildInventoryCandidates([
    { slotId: 4, hash: '0xABC', name: 'A+', primaryTraitName: '伤害上限', primaryTraitLevel: 15, secondaryTraitName: '暴击率', secondaryTraitLevel: 15 },
    { slotId: 9, name: 'A+', primaryTraitName: '伤害上限', primaryTraitLevel: 15, secondaryTraitName: '暴击率', secondaryTraitLevel: 15 },
    { slotId: 11, name: 'Other', primaryTraitName: '防御力', primaryTraitLevel: 15 },
  ], targets, { sigils: [{ internalId: 'EXACT_A', hash: 'ABC' }] })
  assert.deepEqual(candidates.map(item => item.slotId), [4, 9])
  assert.equal(candidates[0].sigilId, 'EXACT_A')
})

test('inventory candidate builder can expose non-target factors for final-slot filling', () => {
  const rows = buildInventoryCandidates([
    { slotId: 1, name: '目标', primaryTraitName: '伤害上限', primaryTraitLevel: 15 },
    { slotId: 2, name: '填充', primaryTraitName: '其他技能', primaryTraitLevel: 15 },
  ], [{ name: '伤害上限', cap: 15 }], null, { includeUnmatched: true })
  assert.deepEqual(rows.map(item => item.slotId), [1, 2])
})

test('owned-first deployment maximizes real SlotID reuse before creating missing sigils', () => {
  const requestedTargets = [{ name: 'A', weight: 2, cap: 10 }, { name: 'B', weight: 1, cap: 10 }]
  const desired = {
    domain: 'catalog',
    score: 30,
    exact: true,
    picked: [
      { id: 'catalog-a', source: 'catalog', name: 'A shell', traits: [{ name: 'A', level: 10 }] },
      { id: 'catalog-ab', source: 'catalog', name: 'AB shell', traits: [{ name: 'A', level: 10 }, { name: 'B', level: 10 }] },
    ],
    totals: [],
  }
  const inventoryCandidates = [
    { id: 'slot:20', source: 'inventory', slotId: 20, name: 'Owned AB', traits: [{ name: 'A', level: 10 }, { name: 'B', level: 10 }] },
    { id: 'slot:10', source: 'inventory', slotId: 10, name: 'Owned A', traits: [{ name: 'A', level: 10 }] },
  ]
  const result = synthesizeOwnedFirstSuggestion({ desired, inventoryCandidates, targets: requestedTargets })
  assert.equal(result.domain, 'owned-first')
  assert.equal(result.ownedCount, 2)
  assert.equal(result.constructedCount, 0)
  assert.deepEqual(result.picked.map(item => item.slotId), [10, 20])
  assert.deepEqual(result.totals.map(item => item.effective), [10, 10])
})

test('owned-first deployment keeps unmatched catalog rows as independent constructor gaps', () => {
  const requestedTargets = [{ name: 'A', weight: 1, cap: 20 }]
  const desired = {
    domain: 'catalog',
    score: 20,
    exact: true,
    picked: [
      { id: 'catalog-a-1', source: 'catalog', name: 'A shell', traits: [{ name: 'A', level: 10 }] },
      { id: 'catalog-a-2', source: 'catalog', name: 'A shell', traits: [{ name: 'A', level: 10 }] },
    ],
    totals: [],
  }
  const inventoryCandidates = [
    { id: 'slot:7', source: 'inventory', slotId: 7, name: 'Owned A', traits: [{ name: 'A', level: 10 }] },
  ]
  const result = synthesizeOwnedFirstSuggestion({ desired, inventoryCandidates, targets: requestedTargets })
  assert.equal(result.ownedCount, 1)
  assert.equal(result.constructedCount, 1)
  assert.deepEqual(result.picked.map(item => item.source), ['inventory', 'catalog'])
  assert.equal(result.totals[0].effective, 20)
})

test('owned-first domain exhausts real inventory coverage before constructing a shorter catalog plan', () => {
  const requestedTargets = [{ name: 'A', weight: 1, cap: 10 }]
  const results = solveLoadoutSuggestionsByDomain({
    domains: {
      'owned-first': [
        { id: 'slot:1', source: 'inventory', slotId: 1, name: 'Owned A 1', traits: [{ name: 'A', level: 5 }] },
        { id: 'slot:2', source: 'inventory', slotId: 2, name: 'Owned A 2', traits: [{ name: 'A', level: 5 }] },
        { id: 'catalog:a', source: 'catalog', name: 'Constructed A', traits: [{ name: 'A', level: 10 }] },
      ],
    },
    targets: requestedTargets,
    slotCount: 12,
    limit: 10,
  })
  assert.equal(results[0].domain, 'owned-first')
  assert.deepEqual(results[0].picked.map(item => item.slotId), [1, 2])
  assert.equal(results[0].ownedCount, 2)
  assert.equal(results[0].constructedCount, 0)
  assert.equal(results[0].totals[0].effective, 10)
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

test('user-entered target level changes the required slot plan', () => {
  const candidates = [
    { id: 'catalog:a', source: 'catalog', name: 'A shell', traits: [{ name: 'A', level: 5 }] },
  ]
  const [levelFive] = solveLoadoutSuggestions({
    candidates,
    targets: [{ name: 'A', weight: 1, cap: 5 }],
    slotCount: 12,
  })
  const [levelTen] = solveLoadoutSuggestions({
    candidates,
    targets: [{ name: 'A', weight: 1, cap: 10 }],
    slotCount: 12,
  })
  assert.equal(levelFive.picked.length, 1)
  assert.equal(levelFive.totals[0].effective, 5)
  assert.equal(levelTen.picked.length, 2)
  assert.equal(levelTen.totals[0].effective, 10)
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

test('skill-target solver treats display order as a strict priority contract', () => {
  const orderedTargets = [
    { name: 'First', cap: 10, weight: 1 },
    { name: 'Later', cap: 10, weight: 100 },
  ]
  const candidates = [
    { id: 'complete-first', source: 'inventory', slotId: 1, name: 'Complete first', traits: [{ name: 'First', level: 10 }] },
    { id: 'tempting-total', source: 'inventory', slotId: 2, name: 'Tempting total', traits: [{ name: 'First', level: 9 }, { name: 'Later', level: 10 }] },
  ]
  const [result] = solveLoadoutSuggestions({ candidates, targets: orderedTargets, slotCount: 1, limit: 2 })
  assert.equal(result.picked[0].id, 'complete-first')
  assert.equal(result.totals[0].effective, 10)
  assert.equal(result.totals[1].effective, 0)
})

test('skill-target solver accepts more than four goals and fills them from first to last', () => {
  const orderedTargets = Array.from({ length: 7 }, (_, index) => ({ name: `Target ${index + 1}`, cap: 10, weight: 1 }))
  const candidates = orderedTargets.map((target, index) => ({
    id: `target-${index + 1}`,
    source: 'inventory',
    slotId: index + 1,
    name: target.name,
    traits: [{ name: target.name, level: 10 }],
  }))
  const [result] = solveLoadoutSuggestions({ candidates, targets: orderedTargets, slotCount: 5, limit: 3 })
  assert.deepEqual(result.picked.map(item => item.id), ['target-1', 'target-2', 'target-3', 'target-4', 'target-5'])
  assert.deepEqual(result.totals.map(item => item.effective), [10, 10, 10, 10, 10, 0, 0])
  assert.equal(result.method, 'heuristic-fallback')
})

test('many-target fallback does not waste a slot on one large early trait when paired rows can satisfy every goal', () => {
  const orderedTargets = ['A', 'B', 'C', 'D', 'E'].map(name => ({ name, cap: name === 'A' ? 20 : 10, weight: 1 }))
  const candidates = [
    ...Array.from({ length: 20 }, (_, index) => ({
      id: `a-distractor-${String(index).padStart(2, '0')}`,
      source: 'catalog',
      name: `Late-only ${index}`,
      traits: [{ name: 'E', level: 1 }],
    })),
    { id: 'waste-a', source: 'catalog', name: 'A only', traits: [{ name: 'A', level: 20 }] },
    { id: 'pair-ab', source: 'catalog', name: 'A+B', traits: [{ name: 'A', level: 10 }, { name: 'B', level: 10 }] },
    { id: 'pair-ac', source: 'catalog', name: 'A+C', traits: [{ name: 'A', level: 10 }, { name: 'C', level: 10 }] },
    { id: 'pair-de', source: 'catalog', name: 'D+E', traits: [{ name: 'D', level: 10 }, { name: 'E', level: 10 }] },
  ]

  const [result] = solveLoadoutSuggestions({ candidates, targets: orderedTargets, slotCount: 3, limit: 3 })

  assert.deepEqual(result.totals.map(item => item.effective), [20, 10, 10, 10, 10])
  assert.deepEqual(result.picked.map(item => item.id).sort(), ['pair-ab', 'pair-ac', 'pair-de'])
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

test('runtime celestial trait ids reuse canonical 2.0.2 curves without losing route requirements', () => {
  const evidence = { ...combatEvidence, traits: [...combatEvidence.traits,
    { traitId: 'SKILL_321_00', name: '天枢·阳', maxLevel: 15, levels: [{
      level: 15,
      totals: [],
      components: [{ value: 20 }, { value: 70 }, { value: 75 }],
    }] },
  ] }
  const runtimeCelestial = {
    id: 'celestial-lumen',
    source: 'catalog',
    name: '天枢·阳',
    traits: [{ id: 'MEMORY_TRAIT_A7726190', name: '天枢·阳', level: 15 }],
  }
  const result = evaluateCombatBuild([runtimeCelestial], {
    ...combatScenario,
    evidence,
    currentHpRatio: 1,
    requiredTraitTargets: [{ traitId: 'MEMORY_TRAIT_A7726190', targetLevel: 15 }],
  })

  assert.equal(result.valid, true)
  assert.deepEqual(result.metrics.missingRequiredTraits, [])
  assert.equal(result.metrics.attackPercent, 20)
  assert.equal(result.metrics.actionCapBonus, 70)
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

test('action-specific damage bonuses only score for their matching action type', () => {
  const fixedTotals = [
    { label: '造成的伤害', unit: 'pct', value: 5 },
    { label: '普通攻击造成的伤害', unit: 'pct', value: 11 },
    { label: '能力造成的伤害', unit: 'pct', value: 13 },
    { label: '奥义造成的伤害', unit: 'pct', value: 17 },
    { label: '连锁攻击造成的伤害', unit: 'pct', value: 19 },
    { label: '奥义连锁造成的伤害', unit: 'pct', value: 23 },
  ]
  const evaluate = actionType => evaluateCombatBuild([], { ...combatScenario, actionType, fixedTotals })

  assert.equal(evaluate('normal').metrics.outsideCapBonus, 16)
  assert.equal(evaluate('ability').metrics.outsideCapBonus, 18)
  assert.equal(evaluate('sba').metrics.outsideCapBonus, 22)
  assert.equal(evaluate('chain').metrics.outsideCapBonus, 47)
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

test('combat solver finishes a 1404-instance backpack as a bounded twelve-slot cap-aware plan', () => {
  const capLevels = Array.from({ length: 65 }, (_, index) => ({
    level: index + 1,
    totals: [
      { label: '普通攻击伤害上限', unit: 'pct', value: Math.min(250, (index + 1) * 3.85) },
      { label: '能力伤害上限', unit: 'pct', value: Math.min(250, (index + 1) * 3.85) },
      { label: '奥义伤害上限', unit: 'pct', value: Math.min(250, (index + 1) * 3.85) },
    ],
    components: [],
  }))
  const attackLevels = Array.from({ length: 50 }, (_, index) => ({
    level: index + 1,
    totals: [{ label: '攻击力', unit: 'pct', value: (index + 1) * 2 }],
    components: [],
  }))
  const definitions = [
    { id: 'SKILL_020_00', name: '伤害上限', maxLevel: 65, levels: capLevels },
    { id: 'SKILL_000_00', name: '攻击力', maxLevel: 50, levels: attackLevels },
    { id: 'SKILL_146_00', name: '属性克制转换', maxLevel: 15, levels: [{ level: 15, totals: [], components: [] }] },
    { id: 'SKILL_151_00', name: '追击', maxLevel: 45, levels: [{ level: 15, totals: [], components: [{ value: 100 }] }, { level: 45, totals: [], components: [{ value: 100 }] }] },
    { id: 'SKILL_233_00', name: '狂战士', maxLevel: 15, levels: [{ level: 15, totals: [], components: [{ value: 20000 }, { value: 20 }, { value: 25000 }] }] },
    { id: 'SKILL_234_00', name: '斯巴达', maxLevel: 15, levels: [{ level: 15, totals: [], components: [{ value: 50000 }, { value: 20 }, { value: 80000 }] }] },
    { id: 'SKILL_324_00', name: '天星之耀', maxLevel: 15, levels: [{ level: 15, totals: [], components: [{ value: 15 }] }] },
    { id: 'SKILL_001_00', name: '体力', maxLevel: 50, levels: [{ level: 15, totals: [{ label: '最大HP', unit: 'flat', value: 3000 }], components: [] }] },
    { id: 'SKILL_069_00', name: '迅捷能力', maxLevel: 15, levels: [{ level: 15, totals: [{ label: '冷却时间', unit: 'pct', value: -10 }], components: [] }] },
    { id: 'SKILL_072_00', name: '高扬', maxLevel: 45, levels: [{ level: 15, totals: [], components: [] }] },
    { id: 'SKILL_070_00', name: '迅捷能力', maxLevel: 45, levels: [{ level: 15, totals: [{ label: '冷却时间', unit: 'pct', value: -10 }], components: [] }] },
    { id: 'SKILL_044_00', name: '霸体', maxLevel: 15, levels: [{ level: 15, totals: [], components: [] }] },
  ]
  const evidence = {
    dataVersion: '2.0.2',
    traits: definitions.map(item => ({ traitId: item.id, name: item.name, maxLevel: item.maxLevel, levels: item.levels })),
  }
  const candidates = Array.from({ length: 1404 }, (_, index) => {
    const primary = index % definitions.length
    let secondary = Math.floor(index / definitions.length) % definitions.length
    if (secondary === primary) secondary = (secondary + 1) % definitions.length
    return {
      id: `slot:${index + 1}`,
      source: 'inventory',
      slotId: index + 1,
      name: `${definitions[primary].name}+${definitions[secondary].name}`,
      traits: [
        { id: definitions[primary].id, name: definitions[primary].name, level: 15 },
        { id: definitions[secondary].id, name: definitions[secondary].name, level: 15 },
      ],
    }
  })
  const scenario = {
    ...combatScenario,
    evidence,
    baseStats: { attack: 90000, hp: 100000, critRate: 100 },
    baseDamageCap: 10000,
    baseUncappedDamage: 90000,
  }
  const [result] = solveLoadoutSuggestions({ candidates, targets: [], slotCount: 12, limit: 10, scenario })
  assert.equal(result.picked.length, 12)
  assert.equal(new Set(result.picked.map(item => item.slotId)).size, result.picked.length)
  const pickedTraits = new Set(result.picked.flatMap(item => item.traits.map(trait => trait.id)))
  for (const traitId of ['SKILL_020_00', 'SKILL_146_00', 'SKILL_233_00', 'SKILL_234_00']) {
    assert.ok(pickedTraits.has(traitId), `missing universal damage trait ${traitId}`)
  }
  assert.equal(result.combat.metrics.actionCapBonus, 250)
  assert.ok(result.exploredStates <= 120000, `explored ${result.exploredStates} states`)
})

test('combat plans score the same complete slot set that the preview and apply flow receive', () => {
  const retained = {
    id: 'slot:1', slotId: 1, source: 'inventory', retained: true, name: 'Retained Attack',
    traits: [{ id: 'SKILL_000_00', name: '攻击力', level: 15 }],
  }
  const replacement = {
    id: 'replacement', source: 'catalog', name: 'Replacement Cap',
    traits: [{ id: 'SKILL_020_00', name: '伤害上限', level: 15 }],
  }
  const [result] = solveLoadoutSuggestions({
    candidates: [retained, replacement],
    targets: [],
    slotCount: 2,
    limit: 1,
    scenario: { ...combatScenario, baseSigils: [retained] },
  })

  assert.equal(result.picked.length, 2)
  assert.ok(result.picked.some(item => item.retained === true))
  assert.ok(result.picked.some(item => item.id === 'replacement'))
  assert.equal(result.combat.rawScore, evaluateCombatBuild(result.picked, combatScenario).rawScore)
})

test('community routes satisfy locked trait levels before ranking optional damage', () => {
  const evidence = {
    dataVersion: '2.0.2',
    traits: [
      { traitId: 'SKILL_020_00', maxLevel: 65, levels: [{ level: 15, totals: [{ label: '普通攻击伤害上限', unit: 'pct', value: 15 }], components: [] }] },
      { traitId: 'OPTIONAL_ATTACK', maxLevel: 15, levels: [{ level: 15, totals: [{ label: '攻击力', unit: 'pct', value: 200 }], components: [] }] },
    ],
  }
  const cap = { id: 'cap', source: 'catalog', name: 'Damage Cap', traits: [{ id: 'SKILL_020_00', name: '伤害上限', level: 15 }] }
  const optional = { id: 'optional', source: 'catalog', name: 'Optional Attack', traits: [{ id: 'OPTIONAL_ATTACK', name: '可选攻击', level: 15 }] }
  const scenario = {
    ...combatScenario,
    evidence,
    baseStats: { attack: 1000, hp: 100000, critRate: 0 },
    baseUncappedDamage: 1000,
    baseDamageCap: 100000,
    requiredTraitTargets: [{ traitId: 'SKILL_020_00', targetLevel: 15 }],
  }

  const [result] = solveLoadoutSuggestions({ candidates: [optional, cap], targets: [], slotCount: 1, limit: 1, scenario })
  assert.equal(result.picked[0].id, 'cap')
  assert.equal(result.combat.valid, true)
  assert.deepEqual(result.combat.metrics.missingRequiredTraits, [])
})

test('combat candidate pruning keeps the current twelve-slot build as a reachable fallback', () => {
  const retainedTraits = Array.from({ length: 12 }, (_, index) => ({
    traitId: `RETAINED_ATTACK_${index}`,
    name: `Retained Attack ${index}`,
    maxLevel: 15,
    levels: [{ level: 15, totals: [{ label: '攻击力', unit: 'pct', value: 10 }], components: [] }],
  }))
  const decoyTraits = Array.from({ length: 70 }, (_, index) => ({
    traitId: `CRIT_DECOY_${index}`,
    name: `Crit Decoy ${index}`,
    maxLevel: 15,
    levels: [{ level: 15, totals: [{ label: '暴击率', unit: 'pct', value: 100 }], components: [] }],
  }))
  const retained = retainedTraits.map((trait, index) => ({
    id: `slot:${index + 1}`,
    slotId: index + 1,
    source: 'inventory',
    retained: true,
    name: trait.name,
    traits: [{ id: trait.traitId, name: trait.name, level: 15 }],
  }))
  const candidates = [
    ...retained,
    ...decoyTraits.map((trait, index) => ({
      id: `decoy:${index}`,
      source: 'catalog',
      name: trait.name,
      traits: [{ id: trait.traitId, name: trait.name, level: 15 }],
    })),
  ]
  const scenario = {
    ...combatScenario,
    evidence: { dataVersion: '2.0.2', traits: [...retainedTraits, ...decoyTraits] },
    baseStats: { attack: 1000, hp: 1000, critRate: 0 },
    baseUncappedDamage: 1000,
    baseDamageCap: 100000,
  }
  const baseline = evaluateCombatBuild(retained, scenario)
  const [result] = solveLoadoutSuggestions({ candidates, targets: [], slotCount: 12, limit: 1, scenario })

  assert.equal(result.picked.length, 12)
  assert.ok(result.picked.filter(item => item.retained === true).length >= 11)
  assert.ok(result.combat.rawScore >= baseline.rawScore)
})

test('owned-first combat synthesis preserves formula ranking after replacing catalog rows', () => {
  const desired = solveLoadoutSuggestions({
    candidates: [
      { id: 'cap', source: 'catalog', name: 'Damage Cap', traits: [{ id: 'SKILL_020_00', name: '伤害上限', level: 15 }] },
      { id: 'elemental', source: 'catalog', name: 'Elemental', traits: [{ id: 'SKILL_146_00', name: '属性克制转换', level: 15 }] },
    ],
    targets,
    slotCount: 2,
    limit: 1,
    scenario: combatScenario,
  })[0]
  const inventoryCandidates = [
    { id: 'slot:9', source: 'inventory', slotId: 9, name: 'Owned Cap', traits: [{ id: 'SKILL_020_00', name: '伤害上限', level: 15 }] },
  ]
  const result = synthesizeOwnedFirstSuggestion({ desired, inventoryCandidates, targets, scenario: combatScenario })
  const recalculated = evaluateCombatBuild(result.picked, combatScenario)
  assert.equal(result.score, recalculated.rawScore)
  assert.deepEqual(result.combat, recalculated)
  assert.equal(result.ownedCount, 1)
  assert.equal(result.constructedCount, 1)
})

test('unmodelled character conditions are fail-closed instead of becoming permanent damage', () => {
  const conditional = {
    id: 'foreign-warpath',
    source: 'catalog',
    name: '条件战气',
    traits: [{ id: 'SKILL_CHARACTER_CONDITION', name: '条件战气', level: 15 }],
  }
  const evidence = {
    traits: [{
      traitId: 'SKILL_CHARACTER_CONDITION',
      name: '条件战气',
      maxLevel: 15,
      levels: [{ level: 15, totals: [{ label: '造成的伤害', unit: 'pct', value: 100 }], components: [] }],
    }],
  }
  const scenario = { ...combatScenario, evidence, conditionalTraitIds: ['SKILL_CHARACTER_CONDITION'], coverage: 0.8 }
  const baseline = evaluateCombatBuild([], scenario)
  const result = evaluateCombatBuild([conditional], scenario)
  assert.equal(result.rawScore, baseline.rawScore)
  assert.deepEqual(result.metrics.unresolvedConditions, [{ traitId: 'SKILL_CHARACTER_CONDITION', curveName: 'character-condition', input: 0.8 }])
})

test('combat candidate pruning retains damage cap for later attack-cap synergy', () => {
  const attackTraits = Array.from({ length: 70 }, (_, index) => ({
    traitId: `ATTACK_${index}`,
    name: `Attack ${index}`,
    maxLevel: 10,
    levels: [{ level: 10, totals: [{ label: '攻击力', unit: 'pct', value: 10 }], components: [] }],
  }))
  const evidence = {
    traits: [
      ...attackTraits,
      { traitId: 'SKILL_020_00', name: '伤害上限', maxLevel: 45, levels: [{ level: 45, totals: [{ label: '普通攻击伤害上限', unit: 'pct', value: 45 }], components: [] }] },
    ],
  }
  const candidates = [
    ...attackTraits.map((trait, index) => ({ id: `attack-${index}`, source: 'catalog', name: trait.name, traits: [{ id: trait.traitId, name: trait.name, level: 10 }] })),
    { id: 'cap', source: 'catalog', name: 'Damage Cap', traits: [{ id: 'SKILL_020_00', name: '伤害上限', level: 45 }] },
  ]
  const scenario = {
    ...combatScenario,
    evidence,
    baseStats: { attack: 1000, hp: 1000, critRate: 0 },
    baseUncappedDamage: 530,
    baseDamageCap: 1000,
  }
  const [result] = solveLoadoutSuggestions({ candidates, targets: [], slotCount: 12, limit: 10, scenario })
  assert.ok(result.picked.some(item => item.id === 'cap'), 'damage-cap candidate was pruned before attack synergy could make it useful')
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

test('skill-target equipment solver combines weapon, wrightstone, summon, and sigil levels before filling the gap', () => {
  const skillOption = (id, stage, level, applyPayload) => ({
    id,
    label: id,
    baseStatDeltas: {},
    fixedBonuses: level ? [{ traitId: 'DAMAGE_CAP', name: 'Damage Cap', level }] : [],
    fixedTotals: [],
    applyPayload,
    unresolvedAtoms: [],
    ...(stage === 'wrightstone' ? { requires: { weapon: ['weapon:2'] } } : {}),
  })
  const snapshot = {
    schemaVersion: 1,
    domain: 'inventory',
    baseStats: { attack: 1000, hp: 1000, critRate: 0 },
    baseFixedBonuses: [], baseFixedTotals: [], baseDefenseZones: [],
    baseSelection: { weapon: ['weapon:1'], wrightstone: ['wrightstone:weapon:1'], summons: ['summon:1'] },
    stages: [
      { key: 'weapon', choose: 1, options: [
        skillOption('weapon:1', 'weapon', 0, { weaponSlotId: 1 }),
        skillOption('weapon:2', 'weapon', 15, { weaponSlotId: 2, weaponSkillHashes: ['A', 'B', 'C', 'D', 'E'] }),
      ] },
      { key: 'wrightstone', choose: 1, options: [
        { ...skillOption('wrightstone:weapon:1', 'wrightstone', 0, { weaponSlotId: 1 }), requires: { weapon: ['weapon:1'] } },
        skillOption('wrightstone:weapon:2', 'wrightstone', 20, { weaponSlotId: 2, wrightstoneHash: 'C0FFEE' }),
      ] },
      { key: 'summons', choose: 1, options: [
        skillOption('summon:1', 'summons', 0, { slotId: 1 }),
        skillOption('summon:2', 'summons', 15, { slotId: 2 }),
      ] },
    ],
  }
  const sigils = [{
    id: 'slot:99', slotId: 99, source: 'inventory', name: '伤害上限 V',
    primaryTraitId: 'DAMAGE_CAP', traits: [{ id: 'DAMAGE_CAP', name: '伤害上限', level: 15 }],
  }]

  const [result] = solveEquipmentAwareSuggestions({
    snapshot,
    sigilCandidates: sigils,
    sigilSlotCount: 12,
    limit: 10,
    scenario: { mode: 'target', targets: [{ traitId: 'DAMAGE_CAP', name: '伤害上限', weight: 1, cap: 65 }] },
  })

  assert.equal(result.totals[0].level, 65)
  assert.equal(result.totals[0].effective, 65)
  assert.equal(result.picked.length, 1)
  assert.equal(result.picked[0].slotId, 99)
  assert.deepEqual(result.targetSources[0].sources.map(item => [item.stage, item.level]), [
    ['weapon', 15], ['wrightstone', 20], ['summons', 15], ['sigils', 15],
  ])
  assert.deepEqual(new Set(result.equipmentDiffs.map(item => item.stage)), new Set(['weapon', 'wrightstone', 'summons']))
  assert.deepEqual(result.applyPayload.equipment, {
    weapon: [{ weaponSlotId: 2, weaponSkillHashes: ['A', 'B', 'C', 'D', 'E'] }],
    wrightstone: [{ weaponSlotId: 2, wrightstoneHash: 'C0FFEE' }],
    summons: [{ slotId: 2 }],
  })
})

test('skill-target equipment solver can stage confirmed summon main-trait edits to free scarce factor slots', () => {
  const editableSummon = (slotId) => equipmentOption(`summon:${slotId}`, 0, {
    slotId,
    editableMainTrait: true,
    expectUnitId: 100 + slotId,
    expectTypeHash: `TYPE${slotId}`,
    expectMainTraitHash: `OLD${slotId}`,
    expectMainTraitLevel: 1,
    expectSubParamHash: `SUB${slotId}`,
    expectSubParamLevel: 2,
    expectRank: 3,
    subParamHash: `SUB${slotId}`,
    subParamLevel: 2,
    rank: 3,
  })
  const snapshot = {
    schemaVersion: 1,
    domain: 'inventory',
    baseStats: { attack: 1000, hp: 1000, critRate: 0 },
    baseFixedBonuses: [], baseFixedTotals: [], baseDefenseZones: [],
    baseSelection: { summons: ['summon:1', 'summon:2'] },
    stages: [{ key: 'summons', choose: 2, unique: true, options: [editableSummon(1), editableSummon(2)] }],
  }
  const trait = (id, hash, name, level = 10) => ({ id, name, level, hash })
  const sigils = [
    {
      id: 'pair-ab', source: 'catalog', name: 'A+B', exactPrimaryTraitHash: 'A0', exactSecondaryTraitHash: 'B0',
      primaryTraitId: 'A', secondaryTraitId: 'B', traits: [trait('A', 'A0', 'A'), trait('B', 'B0', 'B')],
    },
    {
      id: 'pair-cd', source: 'catalog', name: 'C+D', exactPrimaryTraitHash: 'C0', exactSecondaryTraitHash: 'D0',
      primaryTraitId: 'C', secondaryTraitId: 'D', traits: [trait('C', 'C0', 'C'), trait('D', 'D0', 'D')],
    },
    {
      id: 'single-e', source: 'catalog', name: 'E', exactPrimaryTraitHash: 'E0',
      primaryTraitId: 'E', traits: [trait('E', 'E0', 'E')],
    },
  ]
  const targets = ['A', 'B', 'C', 'D', 'E'].map(name => ({ traitId: name, name, weight: 1, cap: name === 'A' || name === 'B' ? 30 : 10 }))

  const [result] = solveEquipmentAwareSuggestions({
    snapshot, sigilCandidates: sigils, sigilSlotCount: 2, limit: 4,
    scenario: { mode: 'target', targets },
  })

  assert.deepEqual(result.totals.map(item => item.effective), [30, 30, 10, 10, 10])
  assert.deepEqual(result.picked.map(item => item.id).sort(), ['pair-cd', 'single-e'])
  assert.deepEqual(result.applyPayload.equipment.summons.map(item => [item.slotId, item.mainTraitHash, item.mainTraitLevel]), [
    [1, 'A0', 30],
    [2, 'B0', 30],
  ])
})

test('factor boost raises normal factor traits but leaves fixed-effect traits at their stored level', () => {
  const snapshot = {
    schemaVersion: 1,
    domain: 'inventory',
    baseStats: { attack: 1000, hp: 1000, critRate: 0 },
    baseFixedBonuses: [{ traitId: 'SKILL_113_00', name: '因子强化', level: 1 }],
    baseFixedTotals: [], baseDefenseZones: [], baseSelection: {}, stages: [],
  }
  const sigils = [
    { id: 'normal', source: 'inventory', slotId: 1, name: '普通因子', traits: [{ id: 'NORMAL', name: '普通技能', level: 15 }] },
    { id: 'fixed', source: 'inventory', slotId: 2, name: '固定因子', traits: [{ id: 'FIXED', name: '狂战士', level: 15, fixedLevel: true }] },
  ]
  const [result] = solveEquipmentAwareSuggestions({
    snapshot,
    sigilCandidates: sigils,
    sigilSlotCount: 2,
    limit: 10,
    scenario: { mode: 'target', targets: [
      { traitId: 'NORMAL', name: '普通技能', weight: 1, cap: 16 },
      { traitId: 'FIXED', name: '狂战士', weight: 1, cap: 15 },
    ] },
  })

  assert.deepEqual(result.totals.map(item => [item.traitId, item.level]), [['NORMAL', 16], ['FIXED', 15]])
})

test('manufactured target factors lower stored levels so the final panel matches exact requested levels', () => {
  const [result] = solveEquipmentAwareSuggestions({
    snapshot: {
      schemaVersion: 1, domain: 'catalog', baseStats: {}, baseFixedTotals: [], baseDefenseZones: [], baseSelection: {}, stages: [],
      baseFixedBonuses: [{ traitId: 'SKILL_113_00', name: '因子强化', level: 1 }],
    },
    sigilCandidates: [{
      id: 'made', source: 'catalog', name: '制造因子', sigilId: 'MADE',
      primaryTraitId: 'A', primaryLevel: 15, secondaryTraitId: 'B', secondaryLevel: 15,
      traits: [{ id: 'A', name: '属性克制转换', level: 15 }, { id: 'B', name: '金刚', level: 15 }],
    }],
    sigilSlotCount: 1,
    limit: 4,
    scenario: { mode: 'target', targets: [
      { traitId: 'A', name: '属性克制转换', weight: 2, cap: 15 },
      { traitId: 'B', name: '金刚', weight: 1, cap: 10 },
    ] },
  })

  assert.equal(result.picked[0].primaryLevel, 14)
  assert.equal(result.picked[0].secondaryLevel, 9)
  assert.deepEqual(result.totals.map(item => item.level), [15, 10])
})

test('many-target equipment solver budgets one boosted Lv15 factor per Lv16 goal instead of duplicating slots', () => {
  const targets = ['A', 'B', 'C', 'D', 'E'].map(name => ({ traitId: name, name, weight: 1, cap: 16 }))
  const sigils = targets.map((target, index) => ({
    id: `factor-${target.name}`, source: 'catalog', name: target.name,
    sigilId: `SIGIL_${target.name}`, primaryTraitId: target.traitId, primaryLevel: 15,
    traits: [{ id: target.traitId, name: target.name, level: 15 }],
    exactPrimaryTraitHash: `0${index + 1}`,
  }))
  const [result] = solveEquipmentAwareSuggestions({
    snapshot: {
      schemaVersion: 1, domain: 'inventory', baseStats: {}, baseFixedTotals: [], baseDefenseZones: [], baseSelection: {}, stages: [],
      baseFixedBonuses: [{ traitId: 'SKILL_113_00', name: '因子强化', level: 1 }],
    },
    sigilCandidates: sigils, sigilSlotCount: 5, limit: 4,
    scenario: { mode: 'target', targets },
  })

  assert.deepEqual(result.totals.map(item => item.effective), [16, 16, 16, 16, 16])
  assert.deepEqual(result.picked.map(item => item.id).sort(), ['factor-A', 'factor-B', 'factor-C', 'factor-D', 'factor-E'])
})

test('target equipment solver completes all staged slots without retaining target-overflow factors', () => {
  const target = { traitId: 'A', name: '目标技能', weight: 1, cap: 16 }
  const wanted = {
    id: 'wanted', source: 'inventory', slotId: 1, name: '目标因子', retained: true,
    traits: [{ id: 'A', name: '目标技能', level: 15 }],
  }
  const overflowing = {
    id: 'overflowing', source: 'inventory', slotId: 2, name: '重复目标', retained: true,
    traits: [{ id: 'A', name: '目标技能', level: 15 }],
  }
  const neutral = {
    id: 'neutral', source: 'inventory', slotId: 3, name: '无冲突填充',
    traits: [{ id: 'B', name: '其他技能', level: 15 }],
  }
  const [result] = solveEquipmentAwareSuggestions({
    snapshot: {
      schemaVersion: 1, domain: 'inventory', baseStats: {}, baseFixedTotals: [], baseDefenseZones: [], baseSelection: {}, stages: [],
      baseFixedBonuses: [{ traitId: 'SKILL_113_00', name: '因子强化', level: 1 }],
    },
    sigilCandidates: [wanted, overflowing, neutral],
    sigilSlotCount: 2,
    limit: 4,
    scenario: { mode: 'target', targets: [target], baseSigils: [wanted, overflowing] },
  })

  assert.equal(result.picked.length, 2)
  assert.ok(result.picked.some(item => item.slotId === 3))
  assert.equal(result.picked.some(item => item.slotId === 1) || result.picked.some(item => item.slotId === 2), true)
  assert.equal(result.totals[0].level, 16)
})

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
