import assert from 'node:assert/strict'
import test from 'node:test'

import {
  beginPerformanceMeasure,
  clearPerformanceSamples,
  installPerformanceMonitor,
  performanceSnapshot,
  recordPerformanceSample,
} from './performanceMonitor.js'

test('performance samples are bounded and copied out of the internal store', () => {
  clearPerformanceSamples()
  for (let index = 0; index < 300; index += 1) recordPerformanceSample('sample', index, { index })
  const snapshot = performanceSnapshot()
  assert.equal(snapshot.length, 256)
  assert.equal(snapshot[0].detail.index, 44)
  snapshot[0].detail.index = -1
  assert.equal(performanceSnapshot()[0].detail.index, 44)
})

test('measures complete once and invalid durations are ignored', async () => {
  clearPerformanceSamples()
  const finish = beginPerformanceMeasure('page-switch', { from: 'home' })
  await new Promise(resolve => setTimeout(resolve, 2))
  finish({ to: 'loadout' })
  finish({ to: 'ignored' })
  recordPerformanceSample('invalid', Number.NaN)
  const snapshot = performanceSnapshot()
  assert.equal(snapshot.length, 1)
  assert.equal(snapshot[0].name, 'page-switch')
  assert.equal(snapshot[0].detail.to, 'loadout')
  assert.ok(snapshot[0].duration >= 0)
})

test('installed diagnostics expose read-only snapshot and clear operations', () => {
  const target = {}
  installPerformanceMonitor(target)
  assert.equal(typeof target.__GBFR_PERFORMANCE__.snapshot, 'function')
  assert.equal(typeof target.__GBFR_PERFORMANCE__.clear, 'function')
  assert.ok(Object.isFrozen(target.__GBFR_PERFORMANCE__))
})
