import test from 'node:test'
import assert from 'node:assert/strict'
import {
  FACTOR_SLOT_COUNT,
  buildFactorWritePayload,
  createFactorSlots,
  factorSlotCount,
  putBagFactor,
  putConstructedFactor,
} from './loadoutFactorSlots.js'

test('hydrates the real loadout into twelve stable factor slots', () => {
  const slots = createFactorSlots([{ slotId: 11 }, { slotId: 22 }])
  assert.equal(slots.length, FACTOR_SLOT_COUNT)
  assert.deepEqual(slots.slice(0, 3), [
    { kind: 'bag', slotId: 11 },
    { kind: 'bag', slotId: 22 },
    null,
  ])
  assert.equal(factorSlotCount(slots), 2)
})

test('hydrates sparse save factors by their original 1403 index', () => {
  const slots = createFactorSlots([
    { index: 0, slotId: 11 },
    { index: 4, slotId: 55 },
    { index: 10, slotId: 111 },
  ])

  assert.equal(slots.length, FACTOR_SLOT_COUNT)
  assert.deepEqual(slots[0], { kind: 'bag', slotId: 11 })
  assert.equal(slots[1], null)
  assert.deepEqual(slots[4], { kind: 'bag', slotId: 55 })
  assert.equal(slots[9], null)
  assert.deepEqual(slots[10], { kind: 'bag', slotId: 111 })
})

test('a constructed replacement stays a draft until the loadout write payload', () => {
  const slots = createFactorSlots([{ slotId: 11 }, { slotId: 22 }])
  const item = { sigilId: 'GEEN_001_05', level: 15, primaryLevel: 15, quantity: 1 }
  const next = putConstructedFactor(slots, 0, item, { name: '攻击力 V+', primaryTraitName: '攻击力' })
  const payload = buildFactorWritePayload(next)

  assert.deepEqual(payload.sigilSlotIds.slice(0, 3), [0, 22, 0])
  assert.deepEqual(payload.constructedSigils, [{ index: 0, item }])
  assert.equal(factorSlotCount(next), 2)
  assert.equal(slots[0].kind, 'bag', 'slot updates must not mutate the source draft')
})

test('a real-save template clone keeps its source slot in the atomic constructed payload', () => {
  const slots = putConstructedFactor(createFactorSlots([]), 0, {
    sigilId: 'template:3179', templateSlotId: 3179, level: 15, primaryLevel: 15,
  }, { name: '浪迹天涯V+', primaryTraitName: '浪迹天涯' })
  const payload = buildFactorWritePayload(slots)
  assert.equal(payload.constructedSigils[0].templateSlotId, 3179)
  assert.equal(payload.constructedSigils[0].item.templateSlotId, undefined)
})

test('an imported combination preserves its exact hashes in the write payload', () => {
  const item = {
    sigilId: 'HASH_80C94A24', sigilName: '怒发冲冠 + 伤害上限',
    level: 15, primaryLevel: 15, secondaryLevel: 15,
  }
  const exact = {
    exactSigilHash: '80C94A24',
    exactPrimaryTraitHash: '7EDD69D0',
    exactSecondaryTraitHash: 'DC584F60',
  }
  const slots = putConstructedFactor([], 0, item, {}, exact)
  const payload = buildFactorWritePayload(slots)
  assert.deepEqual(payload.constructedSigils[0], { index: 0, ...exact, item })
})

test('choosing an already equipped bag factor swaps slots instead of duplicating it', () => {
  const slots = createFactorSlots([{ slotId: 11 }, { slotId: 22 }])
  const next = putBagFactor(slots, 0, 22)
  assert.deepEqual(next.slice(0, 2), [
    { kind: 'bag', slotId: 22 },
    { kind: 'bag', slotId: 11 },
  ])
  assert.equal(factorSlotCount(next), 2)
})
