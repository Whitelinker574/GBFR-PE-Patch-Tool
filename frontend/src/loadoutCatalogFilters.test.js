import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildConstructCatalog,
  collectBagTraitOptions,
  filterConstructCatalog,
  filterAndSortBagSigils,
  resolveConstructSelection,
} from './loadoutCatalogFilters.js'

const constructCatalog = [
  { internalId: 'A', displayName: '攻击力 V+', primaryTraitName: '攻击力' },
  { internalId: 'B', displayName: '浪迹天涯 V+', primaryTraitName: '浪迹天涯' },
  { internalId: 'C', displayName: '体力 V+', primaryTraitName: '体力' },
]

test('constructor search selects the first real match instead of leaving the native select blank', () => {
  const matches = filterConstructCatalog(constructCatalog, '浪')
  assert.deepEqual(matches.map(item => item.internalId), ['B'])
  assert.equal(resolveConstructSelection(matches, 'A', '浪'), 'B')
  assert.equal(resolveConstructSelection(matches, 'B', '浪'), 'B')
  assert.equal(resolveConstructSelection([], 'A', '不存在'), '')
})

test('constructor catalog stays independent from real-save bag instances', () => {
  const merged = buildConstructCatalog(
    [{ internalId: 'A', hash: '11111111', displayName: '攻击力 V+', primaryTraitName: '攻击力' }],
    [{
      slotId: 3179, hash: '22222222', name: '浪迹天涯V+', level: 15,
      primaryTraitHash: '5BF84FD1', primaryTraitName: '浪迹天涯', primaryTraitLevel: 15,
      secondaryTraitHash: 'C4925BD7', secondaryTraitName: '攻击力', secondaryTraitLevel: 15,
    }],
  )
  assert.equal(merged.length, 1)
  assert.equal(merged[0].internalId, 'A')
  assert.deepEqual(filterConstructCatalog(merged, '浪').map(item => item.internalId), [])
})

const bag = [
  { slotId: 30, name: '热血 V+', primaryTraitName: '热血', primaryTraitLevel: 15, secondaryTraitName: '攻击力', secondaryTraitLevel: 15 },
  { slotId: 10, name: '攻击力 V+', primaryTraitName: '攻击力', primaryTraitLevel: 11, secondaryTraitName: '', secondaryTraitLevel: 0 },
  { slotId: 20, name: '暴击率 V+', primaryTraitName: '暴击率', primaryTraitLevel: 15, secondaryTraitName: '热血', secondaryTraitLevel: 10 },
]

test('bag factors support trait/state filters and deterministic sorting', () => {
  assert.deepEqual(collectBagTraitOptions(bag), ['暴击率', '攻击力', '热血'])

  const dual = filterAndSortBagSigils(bag, {
    query: '', state: 'dual', trait: '', sort: 'slot-desc', usedSlotIds: new Set([30]),
  })
  assert.deepEqual(dual.map(item => item.slotId), [30, 20])

  const unusedAttack = filterAndSortBagSigils(bag, {
    query: 'V+', state: 'unused', trait: '攻击力', sort: 'primary-desc', usedSlotIds: new Set([30]),
  })
  assert.deepEqual(unusedAttack.map(item => item.slotId), [10])
})
