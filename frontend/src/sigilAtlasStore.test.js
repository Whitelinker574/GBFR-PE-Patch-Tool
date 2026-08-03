import assert from 'node:assert/strict'
import test from 'node:test'

import { createSigilAtlasStore, hydrateSigilAtlasIndex } from './sigilAtlasStore.js'

const fixture = {
  dataVersion: 'GBFR 2.0.2',
  traits: [
    { internalId: 'trait-a', displayName: '攻击力', maxLevel: 65 },
    { internalId: 'trait-b', displayName: '伤害上限', maxLevel: 65 },
  ],
  writableSecondaryTraits: [
    { internalId: 'trait-a', displayName: '攻击力', maxLevel: 15 },
    { internalId: 'trait-b', displayName: '伤害上限', maxLevel: 15 },
  ],
  sigils: [{
    internalId: 'sigil-a', displayName: '攻击力 V+', primaryTraitName: '攻击力',
    secondaryTraitIndexes: [1], secondaryTraitMaxLevels: [15],
  }],
}

test('compact sigil atlas hydrates secondary traits without losing per-sigil levels', () => {
  const atlas = hydrateSigilAtlasIndex(fixture)
  assert.equal(atlas.sigils[0].secondaryTraits[0].internalId, 'trait-b')
  assert.equal(atlas.sigils[0].secondaryTraits[0].maxLevel, 15)
  assert.deepEqual(atlas.writableSecondaryTraits.map(item => item.internalId), ['trait-a', 'trait-b'])
  assert.match(atlas.sigils[0].searchText, /攻击力 V\+.*伤害上限/)
})

test('compact sigil atlas rejects malformed secondary trait references', () => {
  const mismatched = structuredClone(fixture)
  mismatched.sigils[0].secondaryTraitMaxLevels = []
  assert.throws(() => hydrateSigilAtlasIndex(mismatched), /secondary trait arrays/)

  const invalidIndex = structuredClone(fixture)
  invalidIndex.sigils[0].secondaryTraitIndexes = [fixture.traits.length]
  assert.throws(() => hydrateSigilAtlasIndex(invalidIndex), /trait index/)

  const invalidLevel = structuredClone(fixture)
  invalidLevel.sigils[0].secondaryTraitMaxLevels = [0]
  assert.throws(() => hydrateSigilAtlasIndex(invalidLevel), /trait level/)

})

test('atlas store shares one in-flight request per locale and retries failures', async () => {
  let calls = 0
  const store = createSigilAtlasStore(async () => {
    calls++
    if (calls === 1) throw new Error('temporary')
    return fixture
  })
  await assert.rejects(store.load('zh'), /temporary/)
  const [first, second] = await Promise.all([store.load('zh'), store.load('zh')])
  assert.equal(calls, 2)
  assert.equal(first, second)
  await store.load('en')
  assert.equal(calls, 3)
})
