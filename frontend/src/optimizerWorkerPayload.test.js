import assert from 'node:assert/strict'
import test from 'node:test'
import { reactive } from 'vue'
import { createOptimizerWorkerMessage } from './utils/optimizerWorkerPayload.js'

test('optimizer worker messages strip Vue proxies before postMessage', () => {
  const reactivePayload = reactive({
    targets: [{ name: '追击', cap: 15 }],
    scenario: {
      evidence: { traits: [{ traitId: 'SKILL_001_00', levels: [1, 2, 3] }] },
      directionProfile: { actionType: 'normal' },
    },
  })

  const message = createOptimizerWorkerMessage(7, reactivePayload, true, { solveFixedRoute: true })

  assert.doesNotThrow(() => structuredClone(message))
  assert.deepEqual(message, {
    id: 7,
    payload: {
      targets: [{ name: '追击', cap: 15 }],
      scenario: {
        evidence: { traits: [{ traitId: 'SKILL_001_00', levels: [1, 2, 3] }] },
        directionProfile: { actionType: 'normal' },
      },
    },
    solveAllDomains: true,
    solveFixedRoute: true,
  })
})
