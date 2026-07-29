import assert from 'node:assert/strict'
import { existsSync, readFileSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import test from 'node:test'

import { budgetFailures, measureBundle } from './measure_frontend_bundle.mjs'

const dist = new URL('../frontend/dist/', import.meta.url)
const budget = new URL('./performance-budget.json', import.meta.url)
const patchCore = new URL('../internal/backend/resources/patch_core.dll', import.meta.url)

test('integrated native runtime stays within its release size budget', () => {
  const limits = JSON.parse(readFileSync(budget, 'utf8'))
  assert.ok(limits.nativePatchCoreBytes > 0)
  assert.ok(statSync(patchCore).size <= limits.nativePatchCoreBytes)
})

test('production entry stays within the initial bundle budgets', async (context) => {
  if (!existsSync(new URL('.vite/manifest.json', dist))) {
    context.skip('frontend production build has not been generated')
    return
  }
  const report = await measureBundle({
    dist: fileURLToPath(dist),
    budget: fileURLToPath(budget),
  })
  assert.equal(report.schemaVersion, 1)
  assert.ok(report.initial.js.length >= 1)
  assert.ok(report.initial.css.length >= 1)
  assert.ok(report.async.js.length >= 1)
  assert.ok(report.media.functionAssetCount >= 1)
  assert.deepEqual(budgetFailures(report), [])
})

test('budget failures report the exact exceeded metric', () => {
  const failures = budgetFailures({
    totals: { initialJsGzipBytes: 300, initialCssGzipBytes: 100 },
    limits: { initialJsGzipBytes: 250, initialCssGzipBytes: 100 },
  })
  assert.deepEqual(failures, ['initialJsGzipBytes: 300 > 250'])
})
