import assert from 'node:assert/strict'
import test from 'node:test'

import {
  FORMULA_PHASES,
  formulaNextPhase,
  formulaPhaseCopy,
  normalizeFormulaPanel,
  normalizeFormulaSamplerStatus,
} from './formulaSamplerView.js'

test('formula sampler presents the reversible A/B/A/B workflow in the required order', () => {
  assert.deepEqual(FORMULA_PHASES, ['A1', 'B1', 'A2', 'B2'])
  assert.equal(formulaNextPhase([]), 'A1')
  assert.equal(formulaNextPhase([{ phase: 'A1' }]), 'B1')
  assert.equal(formulaNextPhase([{ phase: 'A1' }, { phase: 'B1' }]), 'A2')
  assert.equal(formulaNextPhase([{ phase: 'A1' }, { phase: 'B1' }, { phase: 'A2' }]), 'B2')
  assert.equal(formulaNextPhase(FORMULA_PHASES.map(phase => ({ phase }))), null)
})

test('phase copy tells the operator exactly when to change and restore one variable', () => {
  assert.match(formulaPhaseCopy('A1', 'zh').instruction, /基准|不要改/)
  assert.match(formulaPhaseCopy('B1', 'zh').instruction, /只改一项/)
  assert.match(formulaPhaseCopy('A2', 'zh').instruction, /恢复.*A1/)
  assert.match(formulaPhaseCopy('B2', 'zh').instruction, /重复.*B1/)
  assert.match(formulaPhaseCopy('B1', 'en').instruction, /one variable/i)
  assert.throws(() => formulaPhaseCopy('C1', 'zh'), /unknown formula phase/i)
})

test('status normalization rejects invented panel values and preserves evidence fields', () => {
  assert.deepEqual(normalizeFormulaSamplerStatus(null), {
    connected: false,
    complete: false,
    events: [],
    nextPhase: 'A1',
    sessionToken: '',
		experimentType: '',
  })

  const normalized = normalizeFormulaSamplerStatus({
    connected: true,
    sessionToken: 'formula-7',
		experimentType: 'sigil',
    complete: false,
    events: [{
      phase: 'A1',
      panel: {
        characterHash: 'aabbccdd', hp: 12345, attack: 6789,
        stunPower: 250.5, critRate: 83, runtimeVerified: true,
      },
    }],
  })
  assert.equal(normalized.nextPhase, 'B1')
  assert.equal(normalized.events[0].panel.characterHash, 'AABBCCDD')
  assert.equal(normalized.events[0].panel.runtimeVerified, true)
  assert.equal(normalized.events[0].panel.hp, 12345)
  assert.equal(normalized.sessionToken, 'formula-7')
	assert.equal(normalized.experimentType, 'sigil')

  assert.throws(() => normalizeFormulaSamplerStatus({ events: [{ phase: 'A1', panel: {
    characterHash: 'AABBCCDD', hp: '—', attack: 1, stunPower: 1, critRate: 1,
  } }] }), /panel hp/i)
})

test('connected formula sampler status requires an owner token', () => {
  assert.throws(() => normalizeFormulaSamplerStatus({ connected: true, events: [] }), /owner token/i)
})

test('runtime panel normalization preserves identity, raw scale and stable field evidence', () => {
  const panel = normalizeFormulaPanel({
    characterHash: '4d0a60c3', runtimeId: '4d0a60c3', candidateObjectHash: '00000000',
    identitySource: 'map_key', hp: 132977, attack: 111259, stunPower: 306,
    rawStunPower: 30.599998, critRate: 124, runtimeVerified: true,
    hpField: { rawType: 'i32', relativeOffset: 4, rawBits: '0x00020771', displayScale: 1, stableReads: 3 },
    attackField: { rawType: 'i32', relativeOffset: 8, rawBits: '0x0001B29B', displayScale: 1, stableReads: 3 },
    stunField: { rawType: 'f32', relativeOffset: 16, rawBits: '0x41F4CCCC', displayScale: 10, stableReads: 3 },
    critField: { rawType: 'f32', relativeOffset: 20, rawBits: '0x42F80000', displayScale: 1, stableReads: 3 },
  })
  assert.equal(panel.characterHash, '4D0A60C3')
  assert.equal(panel.runtimeId, '4D0A60C3')
  assert.equal(panel.identitySource, 'map_key')
  assert.equal(panel.rawStunPower, 30.599998)
  assert.equal(panel.stunField.displayScale, 10)
  assert.equal(panel.stunField.stableReads, 3)
})
