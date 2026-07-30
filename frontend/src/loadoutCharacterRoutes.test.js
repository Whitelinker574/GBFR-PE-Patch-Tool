import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

import { buildCatalogCandidates } from './loadoutOptimizer.js'
import {
  characterBuildRoutes,
  routeTraitTargets,
} from './loadoutCharacterRoutes.js'

const traits = JSON.parse(readFileSync(new URL('../../internal/backend/data/traits.json', import.meta.url), 'utf8')).traits
const traitById = new Map(traits.map(item => [item.internalId, item]))
const sigils = JSON.parse(readFileSync(new URL('../../internal/backend/data/sigils.json', import.meta.url), 'utf8')).sigils.map(entry => ({
  ...entry,
  constructible: true,
  tableExact: true,
  secondaryTraits: (entry.allowedSecondaryTraitIds || []).map(internalId => {
    const trait = traitById.get(internalId)
    return {
      internalId,
      hash: trait?.hash || '',
      displayName: trait?.displayName || internalId,
      maxLevel: Number(entry.secondaryTraitLevelOverrides?.[internalId] || trait?.maxLevel || 15),
    }
  }),
}))
const atlas = { sigils, traits }

const researchedEarlyCharacters = Object.freeze([
  ['PL0000', '2A26B1B2'],
  ['PL0100', 'A4ACBA76'],
  ['PL0200', '18E2F9F9'],
  ['PL0300', '079DF0CC'],
  ['PL0400', '4D0A60C3'],
  ['PL0500', 'DD7A151E'],
  ['PL0600', 'C8616284'],
  ['PL0700', 'C3FFD418'],
  ['PL0800', '22E437E5'],
  ['PL0900', '2EBE91D5'],
])

const allResearchedCharacters = Object.freeze([
  ...researchedEarlyCharacters,
  ['PL1000', 'BDEF7181'],
  ['PL1100', '627BCB0D'],
  ['PL1200', 'FD3BE362'],
  ['PL1300', 'FC6CDF7B'],
  ['PL1400', 'E7053919'],
  ['PL1500', '978E4B18'],
  ['PL1600', '0D21B430'],
  ['PL1700', 'F0EB77EF'],
  ['PL1800', 'AA66178A'],
  ['PL1900', 'A3A3CB2F'],
  ['PL2100', '718E1A14'],
  ['PL2200', '296471BE'],
  ['PL2300', 'BAD16E3B'],
  ['PL2400', '1BB37EF0'],
  ['PL2500', '25D46F4B'],
  ['PL2600', '9A8AF295'],
  ['PL2700', '9B15CFB1'],
  ['PL2800', '646C3168'],
  ['PL2900', '74DD4C79'],
])

test('every one of the 29 roster characters has a frame-verified twelve-slot route', () => {
  assert.equal(allResearchedCharacters.length, 29)
  for (const [ownerCode, characterHash] of allResearchedCharacters) {
    const routes = characterBuildRoutes(characterHash)
    assert.ok(routes.length > 0, `${ownerCode} is missing its researched route`)
    for (const route of routes) {
      assert.equal(route.ownerCode, ownerCode, `${route.id} belongs to the wrong character`)
      assert.equal(
        route.required.reduce((sum, item) => sum + Number(item.slotCount || 1), 0),
        12,
        `${route.id} does not describe exactly twelve physical sigil slots`,
      )
      assert.ok(!route.required.some(item => item.traitId === 'SKILL_235_00'), `${route.id} invented Super Ultimate Perfect Dodge`)
      assert.ok(route.sources.some(item => item.url.includes('bilibili.com/')), `${route.id} has no direct frame source`)
    }
  }
})

test('frame-verified early-character routes stay legal, complete, and character-owned', () => {
  for (const [ownerCode, characterHash] of researchedEarlyCharacters) {
    const routes = characterBuildRoutes(characterHash)
    assert.ok(routes.length > 0, `${ownerCode} is missing its researched route`)

    for (const route of routes) {
      const physicalSlots = route.required.reduce((sum, item) => sum + Number(item.slotCount || 1), 0)
      assert.equal(physicalSlots, 12, `${route.id} does not describe exactly twelve physical sigil slots`)
      assert.equal(route.ownerCode, ownerCode, `${route.id} belongs to the wrong character`)
      assert.ok(!route.required.some(item => item.traitId === 'SKILL_235_00'), `${route.id} invented Super Ultimate Perfect Dodge`)

      const targets = routeTraitTargets(route, atlas)
      assert.ok(targets.every(item => item.name && item.cap > 0), `${route.id} contains an unresolved trait target`)
      const candidates = buildCatalogCandidates(atlas, targets, ownerCode)
      const reachable = new Set(candidates.flatMap(item => item.traits.map(candidateTrait => candidateTrait.id)))
      for (const item of route.required) {
        assert.ok(reachable.has(item.traitId), `${route.id} cannot construct required trait ${item.traitId}`)
      }
    }
  }
})

test('recorded awakening and runtime-plus routes keep their exact physical shells', () => {
  const charlotta = characterBuildRoutes('FD3BE362')[0].required.find(item => item.traitId === 'SKILL_125_00')
  assert.equal(charlotta.sigilId, 'GEEN_125_90')

  const narmaya = characterBuildRoutes('E7053919')[0].required.find(item => item.traitId === 'SKILL_127_00')
  assert.equal(narmaya.sigilId, 'GEEN_127_90')
  assert.equal(narmaya.secondaryTraitId, 'SKILL_127_01')

  const vaseraga = characterBuildRoutes('F0EB77EF')[0].required.find(item => item.traitId === 'SKILL_132_00')
  assert.equal(vaseraga.sigilId, 'GEEN_132_90')

  const cagliostro = characterBuildRoutes('AA66178A')[0].required.find(item => item.traitId === 'SKILL_129_00')
  assert.deepEqual(cagliostro.exactSigilIds, ['GEEN_129_91', 'GEEN_129_90'])

  const narmayaFatebreaker = characterBuildRoutes('E7053919')[0].required.find(item => item.traitId === 'MEMORY_TRAIT_D029FE08')
  assert.equal(narmayaFatebreaker.sigilId, 'MEMORY_SIGIL_5BF84FD1')

  const idVentus = characterBuildRoutes('A3A3CB2F')[0].required.find(item => item.traitId === 'MEMORY_TRAIT_73220725')
  assert.equal(idVentus.sigilId, 'MEMORY_SIGIL_9300FADB')
  assert.equal(idVentus.secondaryTraitId, 'MEMORY_TRAIT_A7726190')
})

test('Io community route preserves the frame-verified 12-slot factor blueprint', () => {
  const routes = characterBuildRoutes('4D0A60C3')
  assert.equal(routes.length, 3)
  for (const route of routes) {
    assert.equal(route.required.reduce((sum, item) => sum + Number(item.slotCount || 1), 0), 12, route.id)
    assert.ok(!route.required.some(item => item.traitId === 'SKILL_235_00'), route.id)
  }

  const charge = routes.find(route => route.id === 'io-online-focus-chain')
  assert.ok(charge, 'missing Io online focus/chain route')
  assert.deepEqual(
    new Set(charge.required.map(item => item.traitId)),
    new Set([
      'BF78FBFC',
      'SKILL_004_00',
      'SKILL_020_00',
      'SKILL_063_00',
      'SKILL_069_00',
      'SKILL_070_00',
      'SKILL_087_00',
      'SKILL_117_00',
      'SKILL_117_02',
      'SKILL_166_00',
    ]),
  )
  assert.deepEqual(charge.finalChecks, [], 'the July 22 video did not expose enough fixed-source detail to borrow another route’s summons')
  assert.ok(!charge.required.some(item => item.traitId === 'SKILL_235_00'))

  const magicChain = routes.find(route => route.id === 'io-magic-chain-20260729')
  assert.ok(magicChain)
  assert.equal(magicChain.required.find(item => item.traitId === 'SKILL_004_00')?.targetLevel, 45)
  assert.ok(magicChain.required.some(item => item.traitId === 'SKILL_159_00'))
  assert.ok(magicChain.finalChecks.some(item => item.traitId === 'SKILL_020_00' && item.targetLevel === 63))
  assert.ok(magicChain.finalChecks.some(item => item.traitId === 'SKILL_233_00'))
  assert.ok(magicChain.finalChecks.some(item => item.traitId === 'SKILL_234_00'))
  assert.ok(magicChain.finalChecks.some(item => item.traitId === 'SKILL_146_00'))

  const dlc = routes.find(route => route.id === 'io-dlc-graduation-20260718')
  assert.ok(dlc)
  assert.equal(dlc.required.find(item => item.traitId === 'SKILL_001_00')?.targetLevel, 30)
  assert.ok(dlc.required.some(item => item.traitId === 'SKILL_036_00'))

  const targets = routeTraitTargets(charge, atlas)
  assert.equal(targets.find(item => item.traitId === 'SKILL_020_00')?.cap, 30)
  assert.equal(targets.find(item => item.traitId === 'SKILL_004_00')?.cap, 30)
  assert.ok(targets.every(item => item.name && item.cap > 0))

  const candidates = buildCatalogCandidates(atlas, targets, 'PL0400')
  const candidateTraits = new Set(candidates.flatMap(item => item.traits.map(trait => trait.id)))
  for (const traitId of charge.required.map(item => item.traitId)) {
    assert.ok(candidateTraits.has(traitId), `route-required trait was pruned: ${traitId}`)
  }
  assert.ok(candidates.some(item => item.characterSpecific && item.allowedOwnerCodes.includes('PL0400')))

  for (const route of routes) {
    const routeTargets = routeTraitTargets(route, atlas)
    const routeCandidates = buildCatalogCandidates(atlas, routeTargets, 'PL0400')
    const reachable = new Set(routeCandidates.flatMap(item => item.traits.map(item => item.id)))
    for (const item of route.required) {
      assert.ok(reachable.has(item.traitId), `${route.id} cannot construct required trait ${item.traitId}`)
    }
  }
})
