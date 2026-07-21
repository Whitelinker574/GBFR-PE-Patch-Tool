import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const appJS = readFileSync(new URL('../wailsjs/go/backend/App.js', import.meta.url), 'utf8')
const appTypes = readFileSync(new URL('../wailsjs/go/backend/App.d.ts', import.meta.url), 'utf8')
const models = readFileSync(new URL('../wailsjs/go/models.ts', import.meta.url), 'utf8')

test('formula sampler Wails binding carries the required experiment type', () => {
  assert.match(appJS, /export function FormulaSamplerAttach\(arg1, arg2\)/)
  assert.match(appJS, /FormulaSamplerAttach[\s\S]*?\(arg1,\s*arg2\)/)
  assert.match(appTypes, /FormulaSamplerAttach\(arg1:string,arg2:string\)/)
  assert.match(models, /class FormulaSamplerStatus[\s\S]*?experimentType:\s*string/)
})

test('formula sampler exposes only owner-token capture and close operations', () => {
  assert.match(appJS, /FormulaSamplerCaptureOwned\(arg1, arg2\)/)
  assert.match(appJS, /FormulaSamplerCloseOwned\(arg1\)/)
  assert.match(appJS, /FormulaSamplerExport\(arg1\)/)
  assert.match(appTypes, /FormulaSamplerExport\(arg1:string\)/)
  assert.doesNotMatch(appJS, /export function FormulaSamplerCapture\(/)
  assert.doesNotMatch(appJS, /export function FormulaSamplerClose\(/)
})
