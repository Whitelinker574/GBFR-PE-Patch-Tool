import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/FormulaSampler.vue', import.meta.url), 'utf8')

test('formula sampler is a dedicated strict read-only A/B/A/B page', () => {
  assert.match(source, /data-page="formula-sampler"/)
  assert.match(source, /严格只读|Strict read-only/)
  assert.match(source, /FORMULA_PHASES/)
  for (const phase of ['A1', 'B1', 'A2', 'B2']) assert.match(source, new RegExp(phase))
  assert.match(source, /FormulaSamplerAttach/)
  assert.match(source, /FormulaSamplerCaptureOwned/)
  assert.match(source, /FormulaSamplerCloseOwned/)
  assert.match(source, /FormulaSamplerExport/)
	assert.match(source, /FormulaSamplerAttach\(selectedHash\.value, selectedExperimentType\.value\)/)
	assert.match(source, /本轮唯一变更类型|One changed variable/)
})

test('an attach that finishes after navigation closes only its own read-only session', () => {
  assert.match(source, /let disposed\s*=\s*false/)
  assert.match(source, /if\s*\(disposed\)[\s\S]*?FormulaSamplerCloseOwned\(status\.sessionToken\)/)
  assert.match(source, /onBeforeUnmount\([\s\S]*?disposed\s*=\s*true[\s\S]*?FormulaSamplerCloseOwned\(sampler\.value\.sessionToken\)/)
})

test('formula sampler never exposes process identifiers or raw addresses', () => {
  assert.doesNotMatch(source, /\bpid\b/i)
  assert.doesNotMatch(source, /moduleBase|memoryAddress|rawAddress/i)
})

test('capture and export controls follow evidence state and remain responsive', () => {
  assert.match(source, /:disabled="[^\"]*!connected[^\"]*busy[^\"]*complete[^\"]*"/)
  assert.match(source, /:disabled="[^\"]*!complete[^\"]*busy[^\"]*"/)
  assert.match(source, /container-name:\s*formula-sampler/)
  assert.match(source, /@container formula-sampler \(max-width:\s*620px\)/)
})

test('formula sampler can scan defense and damage-cap candidates and reveals the exact export path', () => {
  assert.match(source, /\['defense', '防御力'/)
  assert.match(source, /\['damage_cap', '伤害上限'/)
  assert.match(source, /const lastExportPath = ref\(''\)/)
  assert.match(source, /保存路径|Saved to/)
  assert.match(source, /\{\{ lastExportPath \}\}/)
})
