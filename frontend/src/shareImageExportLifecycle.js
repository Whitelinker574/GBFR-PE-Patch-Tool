export class ShareImageExportCancelledError extends Error {
  constructor() {
    super('share-image export is no longer current')
    this.name = 'ShareImageExportCancelledError'
  }
}

export function createShareImageExportLifecycle() {
  let generation = 0
  let disposed = false
  const isCurrent = candidate => !disposed && candidate !== 0 && candidate === generation

  return {
    begin() {
      if (disposed) throw new ShareImageExportCancelledError()
      generation += 1
      return generation
    },
    isCurrent,
    assertCurrent(candidate) {
      if (!isCurrent(candidate)) throw new ShareImageExportCancelledError()
    },
    invalidate(candidate) {
      if (candidate === generation) generation += 1
    },
    dispose() {
      disposed = true
      generation += 1
    },
    get disposed() {
      return disposed
    },
  }
}

export function withShareImageExportTimeout(work, timeoutMs, createTimeoutError, onTimeout) {
  if (typeof work !== 'function') return Promise.reject(new TypeError('share-image export work must be a function'))
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) return Promise.reject(new RangeError('share-image export timeout must be positive'))

  return new Promise((resolve, reject) => {
    let settled = false
    const finish = (callback, value) => {
      if (settled) return
      settled = true
      globalThis.clearTimeout(timer)
      callback(value)
    }
    const timer = globalThis.setTimeout(() => {
      let error
      try {
        if (typeof onTimeout === 'function') onTimeout()
        error = typeof createTimeoutError === 'function'
          ? createTimeoutError()
          : new Error('share-image export timed out')
      } catch (nextError) {
        error = nextError
      }
      finish(reject, error)
    }, timeoutMs)

    Promise.resolve()
      .then(work)
      .then(value => finish(resolve, value), error => finish(reject, error))
  })
}
