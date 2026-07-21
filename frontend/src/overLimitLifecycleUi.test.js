import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const component = readFileSync(new URL('./components/OverLimit.vue', import.meta.url), 'utf8')
const bindings = readFileSync(new URL('../wailsjs/go/backend/App.js', import.meta.url), 'utf8')
const declarations = readFileSync(new URL('../wailsjs/go/backend/App.d.ts', import.meta.url), 'utf8')

test('OverLimit view owns its acquire generation and queues cleanup on unmount', () => {
  assert.match(component, /onBeforeUnmount/)
  assert.match(component, /OverLimitAcquire\(nextRuntimeAcquireRequestID\(\)\)/)
  assert.match(component, /OverLimitRelease/)
  assert.match(component, /lifecycleEpoch/)
  assert.match(component, /enableRequest/)
  assert.match(component, /await\s+enableRequest/)
  const cleanup = component.slice(component.indexOf('onBeforeUnmount(() =>'), component.indexOf('function formatHex'))
  assert.match(cleanup, /const ownerToken = hookOwnerToken/)
  assert.match(cleanup, /queueRuntimeLeaseRelease\([^;]*ownerToken[^;]*OverLimitRelease/)
  assert.match(component, /if \(!isCurrent\(epoch\)\) \{[\s\S]*queueRuntimeLeaseRelease\([^;]*acquiredOwnerToken[^;]*OverLimitRelease/)
  assert.doesNotMatch(component, /OverLimit(?:Enable|Disable)/)
})

test('Wails exposes the owner-scoped OverLimit lifecycle', () => {
  assert.match(bindings, /export function OverLimitAcquire\(arg1\)/)
  assert.match(bindings, /export function OverLimitRelease\(arg1\)/)
  assert.match(bindings, /export function OverLimitSetAllOwned\(arg1, arg2\)/)
  assert.match(declarations, /OverLimitAcquire\(arg1:number\):Promise<backend\.OverLimitStatus>/)
  assert.match(declarations, /OverLimitRelease\(arg1:string\):Promise<backend\.OverLimitStatus>/)
  assert.match(declarations, /OverLimitSetAllOwned\(arg1:string,arg2:Array<backend\.OverLimitUpdate>\):Promise<backend\.OverLimitStatus>/)
})
