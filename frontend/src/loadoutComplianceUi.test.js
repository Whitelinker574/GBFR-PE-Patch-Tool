import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')

test('loadout editor removes the large compliance report from the editing surface', () => {
  assert.match(source, /LoadoutCheckCompliance/)
  assert.doesNotMatch(source, /const complianceReport = ref\(null\)/)
  assert.doesNotMatch(source, /complianceReport\.items|scheduleCompliance|compliance-panel/)
  assert.doesNotMatch(source, /写入检查与提示/)
  assert.match(source, /天然因子组合与等级只作提醒/)
})

test('the final write repeats the same compliance check before opening confirmation', () => {
  assert.match(source, /function buildWriteRequest\(\)/)
  const apply = source.indexOf('async function apply()')
  const preflight = source.indexOf('LoadoutCheckCompliance', apply)
  const confirm = source.indexOf('confirmDialog.value?.ask', apply)
  assert.ok(apply >= 0 && preflight > apply && confirm > preflight)
  assert.match(source, /if \(!preflight\?\.writable\)/)
})

test('writable legality warnings never disable the persistent save action', () => {
  const invalidBody = source.match(/const writeInvalid = computed\(\(\) => \{([\s\S]*?)\n\}\)/)?.[1] || ''
  assert.doesNotMatch(invalidBody, /complianceReport/)
  assert.doesNotMatch(source, /固定组合/)
  assert.match(source, /class="editor-save-button[^\"]*"[^>]*:disabled="applying \|\| writeInvalid"/)
})
