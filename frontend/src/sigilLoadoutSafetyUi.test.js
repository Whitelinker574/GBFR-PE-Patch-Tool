import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

const source = fs.readFileSync(new URL('./components/SigilLoadoutRestore.vue', import.meta.url), 'utf8')

test('十二因子复刻在连接与首条写入前完成整批预检', () => {
  assert.match(source, /SigilMemoryValidateLoadout/)
  const start = source.indexOf('async function startWrite()')
  const end = source.indexOf('\nfunction removeEntry', start)
  assert.ok(start >= 0 && end > start, 'startWrite function must remain discoverable')
  const body = source.slice(start, end)
  const snapshotAt = body.indexOf('writeSnapshot = freezeSigilLoadout(entries.value, normalizeEntry)')
  const validateAt = body.indexOf('await SigilMemoryValidateLoadout(writeSnapshot)')
  const enableAt = body.indexOf('await SigilMemoryAcquire(nextRuntimeAcquireRequestID())')
  const firstWriteAt = body.indexOf('await SigilMemoryUpdateOwned(ownerToken,')
  assert.ok(snapshotAt >= 0, 'batch preflight must freeze an immutable copy of the visible list')
  assert.ok(validateAt >= 0, 'batch preflight must be awaited')
  assert.ok(snapshotAt < validateAt, 'the immutable write snapshot must be captured before preflight')
  assert.ok(validateAt < enableAt, 'batch preflight must run before attaching/enabling the hook')
  assert.ok(validateAt < firstWriteAt, 'batch preflight must run before the first memory write')
})

test('每一条复刻都提交当次轮询捕获的目标地址', () => {
	const calls = [...source.matchAll(/SigilMemoryUpdateOwned\(ownerToken,\s*\{\s*\.\.\.target,\s*expectedSelectedAddr:\s*selectedAddr/g)]
	assert.equal(calls.length, 2, 'poll write and immediate first write must both freeze selectedAddr')
})
