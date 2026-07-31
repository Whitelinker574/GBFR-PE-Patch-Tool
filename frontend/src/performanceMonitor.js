const MAX_SAMPLES = 256
const samples = []
let longTaskObserver = null

function clock() {
  return typeof performance !== 'undefined' && typeof performance.now === 'function'
    ? performance.now()
    : Date.now()
}

export function recordPerformanceSample(name, duration, detail = {}) {
  const milliseconds = Number(duration)
  if (!name || !Number.isFinite(milliseconds) || milliseconds < 0) return
  samples.push({ name: String(name), duration: milliseconds, at: Date.now(), detail: { ...detail } })
  if (samples.length > MAX_SAMPLES) samples.splice(0, samples.length - MAX_SAMPLES)
}

export function beginPerformanceMeasure(name, detail = {}) {
  const startedAt = clock()
  let completed = false
  return (extraDetail = {}) => {
    if (completed) return
    completed = true
    recordPerformanceSample(name, clock() - startedAt, { ...detail, ...extraDetail })
  }
}

export function performanceSnapshot() {
  return samples.map(sample => ({ ...sample, detail: { ...sample.detail } }))
}

export function clearPerformanceSamples() {
  samples.length = 0
}

export function installPerformanceMonitor(target = globalThis) {
  const Observer = target.PerformanceObserver
  if (!longTaskObserver && typeof Observer === 'function') {
    try {
      longTaskObserver = new Observer(list => {
        for (const entry of list.getEntries()) {
          recordPerformanceSample('long-task', entry.duration, { startTime: entry.startTime })
        }
      })
      longTaskObserver.observe({ type: 'longtask', buffered: true })
    } catch {
      longTaskObserver = null
    }
  }
  target.__GBFR_PERFORMANCE__ = Object.freeze({
    snapshot: performanceSnapshot,
    clear: clearPerformanceSamples,
  })
}
