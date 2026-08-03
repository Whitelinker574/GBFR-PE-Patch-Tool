import test from 'node:test'
import assert from 'node:assert/strict'
import { optimizerEquipmentDraft } from './loadoutOptimizerApply.js'

test('optimizer equipment payload becomes one editor draft without inventing resources', () => {
  const draft = optimizerEquipmentDraft({ applyPayload: { equipment: {
    weapon: [{ weaponSlotId: 17, weaponSkillHashes: ['01', '02', '03', '04', '05'] }],
    wrightstone: [{ weaponSlotId: 17, wrightstoneHash: 'ABCD' }],
    summons: [{ slotId: 4 }, { slotId: 8 }, { slotId: 12 }, { slotId: 16 }],
    mastery: [{ unitId: 91, nodeHashes: ['AA', 'BB'] }],
  } } })
  assert.deepEqual(draft, {
    changed: true,
    weaponSlotId: 17,
    weaponSkillHashes: ['01', '02', '03', '04', '05'],
    summonSlotIds: [4, 8, 12, 16],
    summonEdits: [],
    masteryUnitId: 91,
    masteryHashes: ['AA', 'BB'],
  })
})

test('optimizer equipment payload fails closed when no deployable selection exists', () => {
  assert.deepEqual(optimizerEquipmentDraft({}), {
    changed: false,
    weaponSlotId: 0,
    weaponSkillHashes: [],
    summonSlotIds: [],
    summonEdits: [],
    masteryUnitId: 0,
    masteryHashes: [],
  })
})

test('optimizer equipment draft preserves summon main-trait edit snapshots for one confirmed transaction', () => {
  const summon = {
    slotId: 4, expectUnitId: 44, expectTypeHash: 'AA', expectMainTraitHash: 'BB', expectMainTraitLevel: 1,
    expectSubParamHash: 'CC', expectSubParamLevel: 2, expectRank: 3,
    mainTraitHash: 'DD', mainTraitLevel: 30, subParamHash: 'CC', subParamLevel: 2, rank: 3,
  }
  const draft = optimizerEquipmentDraft({ applyPayload: { equipment: { summons: [summon] } } })
  assert.deepEqual(draft.summonEdits, [summon])
})
