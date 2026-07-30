import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds))

export function parseWindowSize(value) {
  const match = /^(\d+)x(\d+)$/i.exec(String(value || '').trim())
  if (!match) throw new Error('--window-size must use WIDTHxHEIGHT')
  const width = Number(match[1])
  const height = Number(match[2])
  if (width < 640 || height < 480) throw new Error('--window-size must be at least 640x480')
  return { width, height }
}

export function parseArgs(argv) {
  const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const options = {
    endpoint: process.env.GBFR_QA_CDP || 'http://127.0.0.1:9223',
    cycles: 50,
    detectorActive: false,
    enforce: false,
    hiddenSeconds: 5,
    output: path.join(repoRoot, '.qa-webview2', 'runtime-stress.json'),
    url: '',
    windowSize: null,
  }
  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index]
    if (value === '--endpoint') options.endpoint = argv[++index]
    else if (value === '--cycles') options.cycles = Number(argv[++index])
    else if (value === '--hidden-seconds') options.hiddenSeconds = Number(argv[++index])
    else if (value === '--budget') options.budget = path.resolve(argv[++index])
    else if (value === '--output') options.output = path.resolve(argv[++index])
    else if (value === '--url') options.url = argv[++index]
    else if (value === '--window-size') options.windowSize = parseWindowSize(argv[++index])
    else if (value === '--detector-active') options.detectorActive = true
    else if (value === '--enforce') options.enforce = true
    else throw new Error(`Unknown argument: ${value}`)
  }
  if (!Number.isInteger(options.cycles) || options.cycles < 1) throw new Error('--cycles must be a positive integer')
  if (!Number.isFinite(options.hiddenSeconds) || options.hiddenSeconds <= 0) {
    throw new Error('--hidden-seconds must be a positive number')
  }
  if (options.detectorActive && options.url) throw new Error('--detector-active requires a packaged Wails target')
  if (!options.budget) throw new Error('--budget is required')
  return options
}

export function percentile(values, quantile) {
  if (!Array.isArray(values) || values.length === 0) return 0
  const sorted = [...values].sort((left, right) => left - right)
  const index = Math.max(0, Math.ceil(sorted.length * quantile) - 1)
  return sorted[Math.min(index, sorted.length - 1)]
}

export function summarizeTimings(values) {
  const round = value => Math.round(value * 10) / 10
  return {
    p50Ms: round(percentile(values, 0.5)),
    p95Ms: round(percentile(values, 0.95)),
    maxMs: round(Math.max(0, ...values)),
  }
}

class CdpClient {
  constructor(endpoint) {
    this.endpoint = endpoint
    this.nextId = 0
    this.pending = new Map()
  }

  async connect() {
    const pages = await fetch(`${this.endpoint}/json/list`).then(response => response.json())
    const target = pages.find(page => page.type === 'page' && String(page.url).startsWith('http://wails.localhost/'))
      || pages.find(page => page.type === 'page')
    if (!target) throw new Error('No debuggable page target was found.')
    this.target = { id: target.id, title: target.title, url: target.url }
    this.socket = new WebSocket(target.webSocketDebuggerUrl)
    await new Promise((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error('Timed out opening the CDP WebSocket')), 5000)
      this.socket.addEventListener('open', () => { clearTimeout(timeout); resolve() }, { once: true })
      this.socket.addEventListener('error', event => { clearTimeout(timeout); reject(event.error || new Error('CDP WebSocket failed')) }, { once: true })
    })
    this.socket.addEventListener('message', event => {
      const message = JSON.parse(event.data)
      const waiter = this.pending.get(message.id)
      if (!waiter) return
      this.pending.delete(message.id)
      if (message.error) waiter.reject(new Error(JSON.stringify(message.error)))
      else waiter.resolve(message.result)
    })
    await this.send('Runtime.enable')
    await this.send('Page.enable')
    await this.send('Performance.enable')
    await this.send('HeapProfiler.enable')
  }

  send(method, params = {}) {
    const id = ++this.nextId
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`Timed out waiting for CDP method ${method}`))
      }, 30000)
      this.pending.set(id, {
        resolve: value => { clearTimeout(timeout); resolve(value) },
        reject: error => { clearTimeout(timeout); reject(error) },
      })
      this.socket.send(JSON.stringify({ id, method, params }))
    })
  }

  async evaluate(expression) {
    const result = await this.send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true, userGesture: true })
    if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description || result.exceptionDetails.text)
    return result.result.value
  }

  async metrics() {
    const response = await this.send('Performance.getMetrics')
    return Object.fromEntries(response.metrics.map(item => [item.name, item.value]))
  }

  close() {
    this.socket?.close()
  }
}

const pages = [
  { group: 0, tool: 'loadoutPresets' },
  { group: 2, tool: 'loadout' },
  { group: 3, tool: 'runtimeQOL' },
  { group: 2, tool: 'runtimeMonitor' },
  { group: 0, tool: 'saveDiff' },
]

const groups = [
  { id: 'save', firstTool: 'loadoutPresets' },
  { id: 'memory', firstTool: 'runtime' },
  { id: 'loadoutFlow', firstTool: 'runtimeMonitor' },
  { id: 'runtimeTools', firstTool: 'runtimeQOL' },
  { id: 'tools', firstTool: 'naturalDrop' },
]

async function waitFor(client, expression, label, timeout = 30000) {
  const started = Date.now()
  while (Date.now() - started < timeout) {
    if (await client.evaluate(`Boolean(${expression})`)) return
    await delay(100)
  }
  throw new Error(`Timed out waiting for ${label}`)
}

async function navigate(client, target) {
  await client.evaluate(`document.querySelectorAll('.nav-item')[${target.group}]?.click()`)
  const group = groups[target.group]
  await waitFor(client, `document.querySelector('.tool-switcher-shell')?.dataset.group === ${JSON.stringify(group.id)}`, `${group.id} switcher`)
  const selector = `[data-tool="${target.tool}"]`
  await waitFor(client, `Boolean(document.querySelector('.tool-switcher ${selector}'))`, `${target.tool} switcher entry`)
  await client.evaluate(`document.querySelector('.tool-switcher ${selector}')?.click()`)
  await waitFor(client, `document.querySelector('.tool-stage')?.dataset.tool === ${JSON.stringify(target.tool)}`, target.tool)
  await delay(180)
}

function sample(metrics) {
  return {
    jsHeapUsedBytes: Math.round(metrics.JSHeapUsedSize || 0),
    nodes: Math.round(metrics.Nodes || 0),
    documents: Math.round(metrics.Documents || 0),
    taskDurationSeconds: metrics.TaskDuration || 0,
  }
}

function failuresFor(report, limits) {
  const checks = [
    ['retainedJsHeapGrowthBytes', report.summary.retainedJsHeapGrowthBytes, limits.maxRetainedJsHeapGrowthBytes],
    ['peakJsHeapGrowthBytes', report.summary.peakJsHeapGrowthBytes, limits.maxPeakJsHeapGrowthBytes],
    ['nodeGrowth', report.summary.nodeGrowth, limits.maxNodeGrowth],
    ['documentGrowth', report.summary.documentGrowth, limits.maxDocumentGrowth],
    ['longTasksOver100ms', report.summary.longTasksOver100ms, limits.maxLongTasksOver100ms],
    ['hiddenTaskMsPerSecond', report.summary.hiddenTaskMsPerSecond, limits.maxHiddenTaskMsPerSecond],
  ]
  return checks.filter(([, actual, limit]) => Number.isFinite(limit) && actual > limit).map(([name, actual, limit]) => `${name}: ${actual} > ${limit}`)
}

async function run(options) {
  const budget = JSON.parse(await fs.readFile(options.budget, 'utf8'))
  const limits = budget.runtimeStress
  if (!limits) throw new Error('runtimeStress limits are missing from the performance budget')
  const client = new CdpClient(options.endpoint)
  await client.connect()
  let detectorStartedHere = false
  try {
    const windowInfo = await client.send('Browser.getWindowForTarget', { targetId: client.target.id })
    if (options.windowSize) {
      await client.send('Browser.setWindowBounds', {
        windowId: windowInfo.windowId,
        bounds: { windowState: 'normal', ...options.windowSize },
      })
      await delay(300)
    }
    if (options.url) {
      const mockSource = `(() => {
        const versions = new Set(['GetAppVersion'])
        const emptyStrings = new Set(['AutoDetect', 'SelectFile', 'SelectSaveFile', 'SelectDirectory'])
        const lists = /(?:List|History|Slots|Catalog|Presets|Characters|Inventory|Records|Items|Entries)$/
        const callable = name => (...args) => {
          if (versions.has(name)) return Promise.resolve('v2.0.3')
          if (emptyStrings.has(name)) return Promise.resolve('')
          if (lists.test(name)) return Promise.resolve([])
          return Promise.resolve(undefined)
        }
        const branch = name => new Proxy(callable(name), {
          get: (_target, child) => branch(String(child)),
          apply: (_target, _this, args) => callable(name)(...args),
        })
        globalThis.go = branch('go')
        globalThis.runtime = {
          WindowMinimise() {}, WindowUnminimise() {}, WindowShow() {}, WindowToggleMaximise() {}, Quit() {},
          EventsOn() { return () => {} }, EventsOff() {}, EventsEmit() {},
        }
      })()`
      await client.send('Page.addScriptToEvaluateOnNewDocument', { source: mockSource })
      await client.send('Page.navigate', { url: options.url })
      await waitFor(client, `document.querySelector('.app-window')`, 'application shell')
      await delay(500)
      client.target.url = options.url
    }
    if (options.detectorActive) {
      const detectorStatus = await client.evaluate(`window.go?.backend?.App?.RuntimeLoadoutDetectorStatus?.()`)
      if (!detectorStatus?.enabled) {
        await client.evaluate(`window.go.backend.App.RuntimeLoadoutDetectorStart()`)
        detectorStartedHere = true
      }
    }
    await client.evaluate(`window.__GBFR_PERFORMANCE__?.clear?.()`)
    const coldSamples = []
    for (const target of pages) {
      const started = performance.now()
      await navigate(client, target)
      coldSamples.push({
        tool: target.tool,
        elapsedMs: Math.round((performance.now() - started) * 10) / 10,
      })
    }
    await client.send('HeapProfiler.collectGarbage')
    const baseline = sample(await client.metrics())
    let peak = baseline
    const samples = []
    for (let index = 0; index < options.cycles; index += 1) {
      const target = pages[index % pages.length]
      const started = performance.now()
      await navigate(client, target)
      const current = sample(await client.metrics())
      peak = {
        jsHeapUsedBytes: Math.max(peak.jsHeapUsedBytes, current.jsHeapUsedBytes),
        nodes: Math.max(peak.nodes, current.nodes),
        documents: Math.max(peak.documents, current.documents),
        taskDurationSeconds: Math.max(peak.taskDurationSeconds, current.taskDurationSeconds),
      }
      samples.push({ index: index + 1, tool: target.tool, elapsedMs: Math.round((performance.now() - started) * 10) / 10, ...current })
    }
    await client.send('HeapProfiler.collectGarbage')
    const final = sample(await client.metrics())
    const longTasks = await client.evaluate(`window.__GBFR_PERFORMANCE__?.snapshot?.().filter(item => item.name === 'long-task') || []`)
    const beforeHidden = sample(await client.metrics())
    await client.send('Browser.setWindowBounds', { windowId: windowInfo.windowId, bounds: { windowState: 'minimized' } })
    await delay(options.hiddenSeconds * 1000)
    const afterHidden = sample(await client.metrics())
    await client.send('Browser.setWindowBounds', { windowId: windowInfo.windowId, bounds: { windowState: 'normal' } })
    const report = {
      schemaVersion: 1,
      measuredAt: new Date().toISOString(),
      target: client.target,
      mode: options.url ? 'browser-with-wails-api-stub' : 'wails-webview2',
      cycles: options.cycles,
      detectorActiveDuringHidden: options.detectorActive,
      hiddenSeconds: options.hiddenSeconds,
      windowSize: options.windowSize,
      pages: pages.map(item => item.tool),
      baseline,
      peak,
      final,
      summary: {
        retainedJsHeapGrowthBytes: Math.max(0, final.jsHeapUsedBytes - baseline.jsHeapUsedBytes),
        peakJsHeapGrowthBytes: Math.max(0, peak.jsHeapUsedBytes - baseline.jsHeapUsedBytes),
        nodeGrowth: Math.max(0, final.nodes - baseline.nodes),
        documentGrowth: Math.max(0, final.documents - baseline.documents),
        longTasksOver100ms: longTasks.filter(item => item.duration >= 100).length,
        hiddenTaskMsPerSecond: Math.round(
          Math.max(0, afterHidden.taskDurationSeconds - beforeHidden.taskDurationSeconds)
          * 1000
          / options.hiddenSeconds,
        ),
        coldPageSwitch: summarizeTimings(coldSamples.map(item => item.elapsedMs)),
        pageSwitch: summarizeTimings(samples.map(item => item.elapsedMs)),
      },
      longTasks: longTasks.slice(-50),
      coldSamples,
      samples,
      limits,
    }
    report.failures = failuresFor(report, limits)
    await fs.mkdir(path.dirname(options.output), { recursive: true })
    await fs.writeFile(options.output, `${JSON.stringify(report, null, 2)}\n`)
    process.stdout.write(`${JSON.stringify({ output: options.output, summary: report.summary, failures: report.failures }, null, 2)}\n`)
    if (options.enforce && report.failures.length) throw new Error(`Runtime stress budget exceeded:\n${report.failures.join('\n')}`)
  } finally {
    if (detectorStartedHere) {
      try {
        await client.evaluate(`window.go.backend.App.RuntimeLoadoutDetectorStop()`)
      } catch (error) {
        process.stderr.write(`Failed to restore runtime detector state: ${error.message}\n`)
      }
    }
    client.close()
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const options = parseArgs(process.argv.slice(2))
  run(options).catch(error => {
    process.stderr.write(`${error.message}\n`)
    process.exitCode = 1
  })
}
