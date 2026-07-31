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
    masteryUnitId: 0,
    masteryHashes: [],
  })
})
