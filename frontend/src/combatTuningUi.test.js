import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildChargeRequest,
  buildCooldownRequest,
  combatTuningStatusMatchesRequest,
  normalizeCombatTuningStatus,
  parseCombatTuningMultiplier,
} from './combatTuningUi.js'

const feature = overrides => ({
  available: true,
  enabled: false,
  candidate: true,
  speedMultiplier: 2,
  rvas: [4096],
  currentBytes: ['C5 FA 10 4E 1C'],
  evidenceNote: '待实机',
  ...overrides,
})

test('combat tuning request builders keep instant, multiplier and party scope explicit', () => {
  assert.deepEqual(buildCooldownRequest({ mode: 'multiplier', multiplier: '2.5', scope: 'self' }), {
    enabled: true,
    noCooldown: false,
    speedMultiplier: 2.5,
    applyWholeParty: false,
  })
  assert.deepEqual(buildCooldownRequest({ mode: 'instant', multiplier: '', scope: 'party' }), {
    enabled: true,
    noCooldown: true,
    speedMultiplier: 2,
    applyWholeParty: true,
  })
  assert.deepEqual(buildChargeRequest({ mode: 'multiplier', multiplier: '0.1' }), {
    enabled: true,
    instant: false,
    speedMultiplier: 0.1,
  })
  assert.deepEqual(buildChargeRequest({ mode: 'instant', multiplier: '' }), {
    enabled: true,
    instant: true,
    speedMultiplier: 2,
  })
  assert.deepEqual(buildCooldownRequest({ enabled: false }), {
    enabled: false,
    noCooldown: false,
    speedMultiplier: 2,
    applyWholeParty: false,
  })
})

test('combat tuning multipliers reject blanks, non-finite values and values outside 0.1 to 100', () => {
  assert.equal(parseCombatTuningMultiplier('100', '倍率'), 100)
  for (const value of ['', '0.09', '101', 'NaN', 'Infinity']) {
    assert.throws(() => parseCombatTuningMultiplier(value, '倍率'), /0\.1 到 100/)
  }
})

test('combat tuning readback stays strict and verifies the exact applied request', () => {
  const status = normalizeCombatTuningStatus({
    cooldown: feature({ enabled: true, noCooldown: false, applyWholeParty: true, speedMultiplier: 3 }),
    charge: feature({ enabled: true, instant: true }),
  })
  assert.equal(combatTuningStatusMatchesRequest(status.cooldown, {
    enabled: true,
    noCooldown: false,
    speedMultiplier: 3,
    applyWholeParty: true,
  }, 'cooldown'), true)
  assert.equal(combatTuningStatusMatchesRequest(status.charge, {
    enabled: true,
    instant: true,
    speedMultiplier: 2,
  }, 'charge'), true)

  assert.throws(() => normalizeCombatTuningStatus({
    cooldown: feature({ enabled: 'true' }),
    charge: feature(),
  }), /enabled.*布尔值/)
  assert.throws(() => normalizeCombatTuningStatus({
    cooldown: feature({ rvas: [1], currentBytes: [] }),
    charge: feature(),
  }), /写入点格式无效/)
})
