import assert from 'node:assert/strict'
import test from 'node:test'

import { createOperationGate, freezeSigilLoadout } from './runtimeOperationGate.js'

test('operation gate rejects a second async start until the first token finishes', async () => {
  const gate = createOperationGate()
  const first = gate.begin('start-write')
  assert.ok(first)
  assert.equal(gate.busy, true)
  assert.equal(gate.begin('start-write'), null)

  let release
  const pending = new Promise(resolve => { release = resolve })
  const work = (async () => {
    await pending
    assert.equal(gate.isCurrent(first), true)
    gate.finish(first)
  })()
  release()
  await work

  assert.equal(gate.busy, false)
  const second = gate.begin('start-write')
  assert.ok(second)
  assert.notEqual(second.id, first.id)
})

test('invalidating the gate makes late promise results stale', async () => {
  const gate = createOperationGate()
  const token = gate.begin('refresh')
  await Promise.resolve()
  gate.invalidate()
  assert.equal(gate.isCurrent(token), false)
  assert.equal(gate.busy, false)
})

test('preflight loadout snapshot cannot change when the visible list is edited later', () => {
  const entries = [{ sigilHash: 1, sigilLevel: 15 }]
  const snapshot = freezeSigilLoadout(entries, entry => ({ ...entry }))
  entries[0].sigilHash = 2
  entries.push({ sigilHash: 3, sigilLevel: 15 })

  assert.deepEqual(snapshot, [{ sigilHash: 1, sigilLevel: 15 }])
  assert.equal(Object.isFrozen(snapshot), true)
  assert.equal(Object.isFrozen(snapshot[0]), true)
})
