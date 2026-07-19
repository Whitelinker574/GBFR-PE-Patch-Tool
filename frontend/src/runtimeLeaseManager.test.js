import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import test from 'node:test'

const managerURL = new URL('./runtimeLeaseManager.js', import.meta.url)
const manager = existsSync(managerURL) ? await import(managerURL) : null

test('runtime acquire request IDs use one strictly increasing safe-integer timeline', () => {
  assert.ok(manager, 'runtimeLeaseManager.js must exist')
  const before = Date.now()
  const ids = Array.from({ length: 2048 }, () => manager.nextRuntimeAcquireRequestID())

  assert.ok(ids[0] >= before * 1024, 'the first ID must use a reload-safe time seed')
  for (let index = 0; index < ids.length; index += 1) {
    assert.ok(Number.isSafeInteger(ids[index]), `request ID ${ids[index]} must be a safe integer`)
    assert.ok(ids[index] > 0, 'request IDs must be positive')
    if (index > 0) assert.ok(ids[index] > ids[index - 1], 'request IDs must be strictly increasing across features')
  }
})

test('a failed owned release stays registered and can be retried with its callback', async () => {
  assert.ok(manager, 'runtimeLeaseManager.js must exist')
  const scope = `test-${Date.now()}-${Math.random()}`
  const token = 'owner-token'
  let attempts = 0
  const release = async (receivedToken) => {
    attempts += 1
    assert.equal(receivedToken, token)
    if (attempts === 1) throw new Error('transient release failure')
    return { ownerToken: '' }
  }

  await assert.rejects(
    manager.releaseRuntimeLease(scope, token, release),
    /transient release failure/,
  )
  assert.equal(manager.pendingRuntimeLeaseReleaseCount(), 1, 'failed release must remain owned by the retry registry')

  const retryResults = await manager.retryPendingRuntimeLeaseReleases()
  assert.equal(attempts, 2, 'retry must reuse the retained release callback')
  assert.equal(retryResults.length, 1)
  assert.equal(retryResults[0].status, 'fulfilled')
  assert.equal(manager.pendingRuntimeLeaseReleaseCount(), 0, 'only a resolved release/stale no-op may leave the registry')
})

test('the release owner is notified only when a retained retry finally succeeds', async () => {
  assert.ok(manager, 'runtimeLeaseManager.js must exist')
  const scope = `release-observer-${Date.now()}-${Math.random()}`
  const token = 'observed-owner-token'
  const notifications = []
  let attempts = 0
  const release = async () => {
    attempts += 1
    if (attempts === 1) throw new Error('first release attempt failed')
    return { ownerToken: '', restored: true }
  }

  await assert.rejects(
    manager.releaseRuntimeLease(scope, token, release, notification => notifications.push(notification)),
    /first release attempt failed/,
  )
  assert.deepEqual(notifications, [], 'a transient rejection is not a completed release')

  await manager.retryPendingRuntimeLeaseReleases()
  assert.equal(notifications.length, 1, 'the eventual successful retry must notify its UI owner exactly once')
  assert.equal(notifications[0].scope, scope)
  assert.equal(notifications[0].ownerToken, token)
  assert.deepEqual(notifications[0].result, { ownerToken: '', restored: true })
})
