const REQUEST_ID_BUCKET_SIZE = 1024
const MAX_REQUEST_ID = Number.MAX_SAFE_INTEGER
const MAX_TIME_SEED = MAX_REQUEST_ID - REQUEST_ID_BUCKET_SIZE
const RELEASE_RETRY_BASE_MS = 250
const RELEASE_RETRY_MAX_MS = 8000

let lastRuntimeAcquireRequestID = Math.min(MAX_TIME_SEED, Math.trunc(Date.now()) * REQUEST_ID_BUCKET_SIZE)
const pendingRuntimeReleases = new Map()
let retryTimer = null
let retryTimerAt = 0

export function nextRuntimeAcquireRequestID() {
  const timeSeed = Math.min(MAX_TIME_SEED, Math.trunc(Date.now()) * REQUEST_ID_BUCKET_SIZE)
  const requestID = Math.max(timeSeed, lastRuntimeAcquireRequestID + 1)
  if (!Number.isSafeInteger(requestID) || requestID <= 0 || requestID > MAX_REQUEST_ID) {
    throw new Error('runtime acquire request ID space is exhausted')
  }
  lastRuntimeAcquireRequestID = requestID
  return requestID
}

function releaseKey(scope, ownerToken) {
  const feature = String(scope || '').trim()
  const token = String(ownerToken || '').trim()
  if (!feature) throw new TypeError('runtime release scope is required')
  if (!token) throw new TypeError('runtime owner token is required')
  return { key: `${feature}\u0000${token}`, feature, token }
}

function retryDelay(failures) {
  return Math.min(RELEASE_RETRY_MAX_MS, RELEASE_RETRY_BASE_MS * (2 ** Math.max(0, failures - 1)))
}

function schedulePendingRuntimeReleaseRetry() {
  if (!pendingRuntimeReleases.size) {
    if (retryTimer !== null) clearTimeout(retryTimer)
    retryTimer = null
    retryTimerAt = 0
    return
  }

  const nextAttemptAt = Math.min(...[...pendingRuntimeReleases.values()]
    .filter(entry => !entry.inFlight)
    .map(entry => entry.nextAttemptAt))
  if (!Number.isFinite(nextAttemptAt)) return
  if (retryTimer !== null && retryTimerAt <= nextAttemptAt) return
  if (retryTimer !== null) clearTimeout(retryTimer)

  retryTimerAt = nextAttemptAt
  retryTimer = setTimeout(() => {
    retryTimer = null
    retryTimerAt = 0
    void retryDueRuntimeLeaseReleases()
  }, Math.max(0, nextAttemptAt - Date.now()))
  retryTimer?.unref?.()
}

function registerRuntimeRelease(scope, ownerToken, releaseCallback) {
  if (typeof releaseCallback !== 'function') throw new TypeError('runtime release callback is required')
  const { key, feature, token } = releaseKey(scope, ownerToken)
  const existing = pendingRuntimeReleases.get(key)
  if (existing) return existing

  const entry = {
    key,
    scope: feature,
    token,
    releaseCallback,
    failures: 0,
    nextAttemptAt: Date.now(),
    inFlight: null,
  }
  pendingRuntimeReleases.set(key, entry)
  return entry
}

async function attemptRuntimeRelease(entry) {
  if (entry.inFlight) return entry.inFlight

  const releaseAttempt = Promise.resolve().then(() => entry.releaseCallback(entry.token))
  entry.inFlight = releaseAttempt
  try {
    const result = await releaseAttempt
    if (pendingRuntimeReleases.get(entry.key) === entry) pendingRuntimeReleases.delete(entry.key)
    return result
  } catch (error) {
    if (pendingRuntimeReleases.get(entry.key) === entry) {
      entry.failures += 1
      entry.nextAttemptAt = Date.now() + retryDelay(entry.failures)
    }
    throw error
  } finally {
    if (pendingRuntimeReleases.get(entry.key) === entry) entry.inFlight = null
    schedulePendingRuntimeReleaseRetry()
  }
}

async function retryDueRuntimeLeaseReleases() {
  const now = Date.now()
  const due = [...pendingRuntimeReleases.values()].filter(entry => !entry.inFlight && entry.nextAttemptAt <= now)
  const results = await Promise.allSettled(due.map(entry => attemptRuntimeRelease(entry)))
  results.forEach((result, index) => {
    if (result.status === 'rejected') {
      console.error(`Runtime lease release retry failed for ${due[index].scope}`, result.reason)
    }
  })
  schedulePendingRuntimeReleaseRetry()
}

export function releaseRuntimeLease(scope, ownerToken, releaseCallback) {
  return attemptRuntimeRelease(registerRuntimeRelease(scope, ownerToken, releaseCallback))
}

export function queueRuntimeLeaseRelease(scope, ownerToken, releaseCallback) {
  const entry = registerRuntimeRelease(scope, ownerToken, releaseCallback)
  void attemptRuntimeRelease(entry).catch((error) => {
    console.error(`Runtime lease release queued for retry after ${entry.scope} cleanup failed`, error)
  })
  return entry.key
}

export function retryPendingRuntimeLeaseReleases() {
  return Promise.allSettled([...pendingRuntimeReleases.values()].map(entry => attemptRuntimeRelease(entry)))
}

export function pendingRuntimeLeaseReleaseCount() {
  return pendingRuntimeReleases.size
}
