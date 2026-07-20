import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')

test('loadout editor exposes the backend write preflight as a per-factor compliance report', () => {
  assert.match(source, /LoadoutCheckCompliance/)
  assert.match(source, /const complianceReport = ref\(null\)/)
  assert.match(source, /写入合规检测/)
  assert.match(source, /complianceReport\.items/)
  assert.match(source, /item\.message/)
  assert.match(source, /合法|固定组合|未证明|不可写/)
})

test('the final write repeats the same compliance check before opening confirmation', () => {
  assert.match(source, /function buildWriteRequest\(\)/)
  const apply = source.indexOf('async function apply()')
  const preflight = source.indexOf('LoadoutCheckCompliance', apply)
  const confirm = source.indexOf('confirmDialog.value?.ask', apply)
  assert.ok(apply >= 0 && preflight > apply && confirm > preflight)
  assert.match(source, /complianceReport\.value && !complianceReport\.value\.writable/)
})
