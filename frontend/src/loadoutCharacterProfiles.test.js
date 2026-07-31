import assert from 'node:assert/strict'
import test from 'node:test'
import { characterRoster } from './characterRoster.js'
import { LOADOUT_CHARACTER_PROFILES, LOADOUT_CHARACTER_PROFILE_VERSION } from './loadoutScenarioConfig.js'

test('all 29 playable characters have separate evidence-scoped optimizer profiles', () => {
  assert.equal(Object.keys(LOADOUT_CHARACTER_PROFILES).length, 29)
  assert.match(LOADOUT_CHARACTER_PROFILE_VERSION, /^2\.0\.2-character-evidence-/)
  for (const character of characterRoster) {
    const profile = LOADOUT_CHARACTER_PROFILES[character.hash]
    assert.ok(profile, `${character.nameEn} is missing a profile`)
    assert.equal(profile.ownerCode, character.plId)
    assert.equal(profile.research.identity, 'verified-roster')
    assert.equal(profile.research.skills, 'save-and-unpacked-skill-catalog')
    assert.equal(profile.research.capCurves, '2.0.2-runtime-reference')
    assert.equal(profile.research.defense, 'audited-common-formula')
    assert.equal(profile.research.actionCoefficients, 'unverified-per-action')
    assert.equal(profile.research.rotation, 'unverified')
  }
})
