import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('./components/MiscTools.vue', import.meta.url), 'utf8')

test('overlapping material and inventory runtime patches are mutually disabled in the UI', () => {
  assert.match(source, /materialConsumeStatus\.enabled[^\n]*inventorySet45Enabled|inventorySet45Enabled[^\n]*materialConsumeStatus\.enabled/)
  assert.match(source, /素材不消耗[^<\n]*占用|占用[^<\n]*素材不消耗/)
  assert.match(source, /小钳蟹[^<\n]*占用|占用[^<\n]*小钳蟹/)
})

test('disconnect clears the inventory hook UI state and timer', () => {
  assert.match(source, /function clearConnectionState\(\)[\s\S]*stopInventorySet45Timer\(\)/)
  assert.match(source, /function clearConnectionState\(\)[\s\S]*inventorySet45Enabled\.value\s*=\s*false/)
  assert.match(source, /async function disconnect\(\)[\s\S]*clearConnectionState\(\)/)
})
