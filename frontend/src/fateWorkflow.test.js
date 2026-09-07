import test from 'node:test'
import assert from 'node:assert/strict'
import { fateFieldNeedsWrite } from './fateWorkflow.js'

test('Fate selection preserves completed non-binary mission states from issue 23', () => {
  for (const currentValue of [1, 2, 3, 30, 0xffffffff]) {
    assert.equal(fateFieldNeedsWrite({ field: 'missionState', currentValue, allowedTargetValues: [1] }), false)
  }
  assert.equal(fateFieldNeedsWrite({ field: 'missionState', currentValue: 0, allowedTargetValues: [1] }), true)
  assert.equal(fateFieldNeedsWrite({ field: 'episodeState', currentValue: 2, allowedTargetValues: [30] }), true)
  assert.equal(fateFieldNeedsWrite({ field: 'episodeState', currentValue: 30, allowedTargetValues: [30] }), false)
})
