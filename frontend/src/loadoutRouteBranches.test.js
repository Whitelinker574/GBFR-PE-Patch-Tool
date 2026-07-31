import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

import { characterBuildRoutes } from './loadoutCharacterRoutes.js'
import { graduationRouteBranches, LOADOUT_ROUTE_BRANCHES } from './loadoutRouteBranches.js'

const traits = JSON.parse(readFileSync(new URL('../../internal/backend/data/traits.json', import.meta.url), 'utf8')).traits
const atlas = { traits }
const characterHashes = Object.freeze([
  '2A26B1B2', 'A4ACBA76', '18E2F9F9', '079DF0CC', '4D0A60C3', 'DD7A151E', 'C8616284', 'C3FFD418',
  '22E437E5', '2EBE91D5', 'BDEF7181', '627BCB0D', 'FD3BE362', 'FC6CDF7B', 'E7053919', '978E4B18',
  '0D21B430', 'F0EB77EF', 'AA66178A', 'A3A3CB2F', '718E1A14', '296471BE', 'BAD16E3B', '1BB37EF0',
  '25D46F4B', '9A8AF295', '9B15CFB1', '646C3168', '74DD4C79',
])

const physicalSlots = route => route.required.reduce((sum, item) => sum + Number(item.slotCount || 1), 0)
const traitSlots = (route, traitId) => route.required
  .filter(item => item.traitId === traitId)
  .reduce((sum, item) => sum + Number(item.slotCount || 1), 0)

test('every verified graduation route exposes deterministic twelve-slot direction branches', () => {
  assert.equal(LOADOUT_ROUTE_BRANCHES.length, 9)
  for (const characterHash of characterHashes) {
    for (const route of characterBuildRoutes(characterHash)) {
      const first = graduationRouteBranches(route, atlas)
      const second = graduationRouteBranches(route, atlas)
      assert.deepEqual(first, second, `${route.id} branch ordering changed between runs`)
      assert.deepEqual(first.map(item => item.branchId), LOADOUT_ROUTE_BRANCHES.map(item => item.id))
      for (const derived of first) {
        assert.equal(physicalSlots(derived), 12, `${derived.id} does not fill exactly twelve physical slots`)
        assert.equal(derived.baseRouteId, route.id)
        assert.ok(derived.preservedSlots >= 1, `${derived.id} discarded the entire verified graduation route`)
      }
    }
  }
})

test('direction branches preserve every character-specific graduation core', () => {
  const isCharacterTrait = traitId => {
    const trait = traits.find(item => item.internalId === traitId)
    if (trait?.category === 'character_trait') return true
    const match = String(traitId || '').match(/^SKILL_(\d{3})_(?:00|01|02)$/)
    const index = Number(match?.[1] || 0)
    return (index >= 114 && index <= 139) || (index >= 172 && index <= 178)
  }
  for (const characterHash of characterHashes) {
    for (const route of characterBuildRoutes(characterHash)) {
      const cores = route.required.filter(item => isCharacterTrait(item.traitId))
      for (const derived of graduationRouteBranches(route, atlas)) {
        for (const core of cores) {
          assert.ok(
            traitSlots(derived, core.traitId) >= Number(core.slotCount || 1),
            `${derived.id} removed character core ${core.traitId}`,
          )
        }
      }
    }
  }
})

test('branch contracts use the intended verified trait groups without contradictory shortcuts', () => {
  const route = characterBuildRoutes('4D0A60C3')[0]
  const byId = new Map(graduationRouteBranches(route, atlas).map(item => [item.branchId, item]))

  const high = byId.get('celestial-high-hp')
  assert.ok(traitSlots(high, 'MEMORY_TRAIT_A7726190') >= 1)
  assert.equal(traitSlots(high, 'MEMORY_TRAIT_0DE887A0'), 0)
  assert.ok(traitSlots(high, 'MEMORY_TRAIT_9232DC17') >= 1)
  assert.ok(traitSlots(high, 'MEMORY_TRAIT_73220725') >= 1)
  assert.ok(traitSlots(high, 'MEMORY_TRAIT_D029FE08') >= 1)

  const low = byId.get('celestial-low-hp')
  assert.ok(traitSlots(low, 'MEMORY_TRAIT_0DE887A0') >= 1)
  assert.equal(traitSlots(low, 'MEMORY_TRAIT_A7726190'), 0)

  const defense = byId.get('defense')
  for (const traitId of ['SKILL_166_00', 'SKILL_001_00', 'SKILL_144_00', 'SKILL_036_00', 'SKILL_096_00']) {
    assert.ok(traitSlots(defense, traitId) >= 1, `defense branch is missing ${traitId}`)
  }

  const stun = byId.get('stun')
  assert.equal(traitSlots(stun, 'SKILL_004_00'), 3)
  assert.equal(stun.required.find(item => item.traitId === 'SKILL_004_00')?.targetLevel, 45)

  const sustain = byId.get('sustain')
  assert.equal(traitSlots(sustain, 'SKILL_067_00'), 2)
  for (const traitId of ['MEMORY_TRAIT_F26BAEA5', 'SKILL_106_00', 'SKILL_063_00']) {
    assert.ok(traitSlots(sustain, traitId) >= 1, `sustain branch is missing ${traitId}`)
  }

  const revive = byId.get('revive-potions')
  for (const traitId of ['SKILL_045_00', 'SKILL_068_00', 'SKILL_073_00', 'SKILL_023_00']) {
    assert.ok(traitSlots(revive, traitId) >= 1, `revive branch is missing ${traitId}`)
  }

  const crab = byId.get('black-crab')
  const firstCrab = crab.required.find(item => item.traitId === 'BF78FBFC')
  const secondCrab = crab.required.find(item => item.traitId === '46EE3116')
  assert.equal(firstCrab?.sigilId, '66CB28BA')
  assert.equal(secondCrab?.sigilId, '76786869')
  assert.equal(firstCrab?.secondaryTraitId, 'SKILL_141_00')
  assert.equal(secondCrab?.secondaryTraitId, 'SKILL_141_00')
})
