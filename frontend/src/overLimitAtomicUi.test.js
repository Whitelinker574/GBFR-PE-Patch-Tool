import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const component = readFileSync(new URL('./components/OverLimit.vue', import.meta.url), 'utf8')
const bindings = readFileSync(new URL('../wailsjs/go/main/App.js', import.meta.url), 'utf8')
const declarations = readFileSync(new URL('../wailsjs/go/main/App.d.ts', import.meta.url), 'utf8')

test('超限能力 UI 只发起一次四槽原子写入', () => {
  assert.match(component, /OverLimitSetAllOwned/)
  assert.match(component, /OverLimitSetAllOwned\(ownerToken,\s*edits\.map/)
	assert.match(component, /if \(!ownerToken\) throw new Error/)
	assert.match(component, /const expectedSelectedAddr\s*=\s*Number\(status\.selectedAddr\s*\|\|\s*0\)/)
	assert.match(component, /expectedSelectedAddr/)
  assert.doesNotMatch(component, /OverLimitSetSlot/)
  assert.doesNotMatch(component, /OverLimitCommit/)
  assert.doesNotMatch(component, /Promise\.all|edits\.reduce/)
})

test('Wails 绑定暴露四槽批量接口', () => {
  assert.match(bindings, /export function OverLimitSetAllOwned\(arg1, arg2\)/)
  assert.match(bindings, /\['OverLimitSetAllOwned'\]\(arg1, arg2\)/)
  assert.match(declarations, /OverLimitSetAllOwned\(arg1:string,arg2:Array<main\.OverLimitUpdate>\):Promise<main\.OverLimitStatus>/)
})
