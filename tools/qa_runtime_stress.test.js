import assert from 'node:assert/strict'
import test from 'node:test'

import { parseArgs, parseWindowSize, percentile, summarizeTimings } from './qa_runtime_stress.mjs'

test('runtime stress accepts a release-length hidden interval', () => {
  const options = parseArgs([
    '--budget',
    './tools/performance-budget.json',
    '--hidden-seconds',
    '60',
  ])
  assert.equal(options.hiddenSeconds, 60)
})

test('runtime stress can require the persistent detector during idle measurement', () => {
  const options = parseArgs([
    '--budget',
    './tools/performance-budget.json',
    '--detector-active',
  ])
  assert.equal(options.detectorActive, true)
})

test('runtime stress rejects an invalid hidden interval', () => {
  assert.throws(
    () => parseArgs(['--budget', './tools/performance-budget.json', '--hidden-seconds', '0']),
    /--hidden-seconds must be a positive number/,
  )
})

test('runtime stress parses a packaged-window viewport', () => {
  assert.deepEqual(parseWindowSize('960x640'), { width: 960, height: 640 })
  assert.throws(() => parseWindowSize('960-by-640'), /--window-size must use WIDTHxHEIGHT/)
})

test('page-switch timing summary reports deterministic percentiles', () => {
  assert.equal(percentile([10, 20, 30, 40, 50], 0.5), 30)
  assert.equal(percentile([10, 20, 30, 40, 50], 0.95), 50)
  assert.deepEqual(summarizeTimings([10, 20, 30, 40, 50]), {
    p50Ms: 30,
    p95Ms: 50,
    maxMs: 50,
  })
})
