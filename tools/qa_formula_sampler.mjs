import fs from 'node:fs'
import path from 'node:path'

const endpoint = process.env.GBFR_QA_CDP || 'http://127.0.0.1:9223'
const outputDirectory = path.resolve('.qa-webview2', 'formula-sampler-ui')
const sizes = [
  { name: '2560x1392', width: 2560, height: 1392 },
  { name: '1680x1050', width: 1680, height: 1050 },
  { name: '1280x720', width: 1280, height: 720 },
  { name: '960x620', width: 960, height: 620 },
]

let socket
let nextID = 0
const pending = new Map()
const consoleErrors = []

const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds))

function send(method, params = {}) {
  const id = ++nextID
  socket.send(JSON.stringify({ id, method, params }))
  return new Promise((resolve, reject) => pending.set(id, { resolve, reject }))
}

async function evaluate(expression) {
  const response = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true, userGesture: true })
  if (response.exceptionDetails) throw new Error(response.exceptionDetails.exception?.description || response.exceptionDetails.text)
  return response.result.value
}

async function waitFor(expression, label, timeout = 30000) {
  const started = Date.now()
  while (Date.now() - started < timeout) {
    if (await evaluate(`Boolean(${expression})`)) return
    await delay(120)
  }
  throw new Error(`Timed out waiting for ${label}`)
}

async function connect() {
  const pages = await fetch(`${endpoint}/json/list`).then(response => response.json())
  const target = pages.find(page => page.type === 'page' && page.url === 'http://wails.localhost/')
  if (!target) throw new Error('Wails WebView2 target not found')
  socket = new WebSocket(target.webSocketDebuggerUrl)
  await new Promise((resolve, reject) => {
    socket.addEventListener('open', resolve, { once: true })
    socket.addEventListener('error', reject, { once: true })
  })
  socket.addEventListener('message', event => {
    const message = JSON.parse(event.data)
    if (message.method === 'Runtime.consoleAPICalled' && ['error', 'assert'].includes(message.params.type)) {
      consoleErrors.push(message.params.args.map(item => item.value ?? item.description ?? '').join(' '))
      return
    }
    if (message.method === 'Runtime.exceptionThrown') {
      consoleErrors.push(message.params.exceptionDetails?.exception?.description || message.params.exceptionDetails?.text || 'exception')
      return
    }
    const waiter = pending.get(message.id)
    if (!waiter) return
    pending.delete(message.id)
    if (message.error) waiter.reject(new Error(JSON.stringify(message.error)))
    else waiter.resolve(message.result)
  })
  await send('Runtime.enable')
  await send('Page.enable')
}

async function openFormulaSampler() {
  await waitFor(`document.querySelectorAll('.nav-item').length >= 4`, 'primary navigation')
  await evaluate(`document.querySelectorAll('.nav-item')[2]?.click()`)
  await waitFor(`document.querySelector('.tool-stage')`, 'monitor group')
  if (await evaluate(`document.querySelector('.tool-stage')?.dataset.tool !== 'formulaSampler'`)) {
    await waitFor(`document.querySelectorAll('.tool-switcher button').length >= 2`, 'monitor tabs')
    await evaluate(`document.querySelectorAll('.tool-switcher button')[1]?.click()`)
  }
  await waitFor(`document.querySelector('.tool-stage')?.dataset.tool === 'formulaSampler'`, 'formula sampler page')
  await waitFor(`document.querySelector('.formula-sampler') && getComputedStyle(document.querySelector('.tool-stage'), '::before').backgroundImage.includes('formula-sampler')`, 'formula sampler assets')
}

async function measure(size) {
  await evaluate(`window.runtime.WindowSetSize(${size.width}, ${size.height})`)
  await delay(550)
  return evaluate(`(() => {
    const panel = document.querySelector('.tool-panel')
    const page = document.querySelector('.formula-sampler')
    const stage = document.querySelector('.tool-stage')
    const sticker = document.querySelector('.sidebar-mascot-img')
    const controls = [...document.querySelectorAll('.formula-sampler button,.formula-sampler select')]
    const outside = controls.filter(element => {
      const rect = element.getBoundingClientRect()
      return rect.left < -1 || rect.right > innerWidth + 1
    }).map(element => (element.innerText || element.value || '').trim())
    const clipped = [...document.querySelectorAll('.formula-sampler label,.formula-sampler button,.formula-sampler header span')]
      .filter(element => element.scrollWidth > element.clientWidth + 2 && getComputedStyle(element).textOverflow !== 'ellipsis')
      .map(element => (element.innerText || '').trim()).filter(Boolean)
    const text = page?.innerText || ''
    return {
      requested: ${JSON.stringify(size)},
      viewport: { width: innerWidth, height: innerHeight, dpr: devicePixelRatio },
      pageWidth: page?.clientWidth || 0,
      pageScrollWidth: page?.scrollWidth || 0,
      panelWidth: panel?.clientWidth || 0,
      panelScrollWidth: panel?.scrollWidth || 0,
      outside,
      clipped,
      brokenImages: [...document.images].filter(image => !image.complete || image.naturalWidth === 0).map(image => image.currentSrc || image.src),
      artLoaded: Boolean(stage && getComputedStyle(stage, '::before').backgroundImage.includes('formula-sampler')),
      stickerLoaded: Boolean(sticker?.complete && sticker.naturalWidth > 0),
      parchmentVisible: getComputedStyle(document.querySelector('.app-window'), '::before').backgroundImage.includes('parchment-ui-v2'),
      titleButtons: document.querySelectorAll('.titlebar-controls .win-btn').length,
      hasExperimentTypes: document.querySelectorAll('.experiment-picker option').length === 12,
      hasFourPhases: document.querySelectorAll('.phase-card').length === 4,
      hasStrictReadOnly: /严格只读|Strict read-only/.test(text),
      suspicious: ['undefined', '[object Object]', 'NaN', '渲染失败'].filter(token => text.includes(token)),
    }
  })()`)
}

async function capture(filename) {
  const response = await send('Page.captureScreenshot', { format: 'png', fromSurface: true, captureBeyondViewport: false })
  fs.writeFileSync(path.join(outputDirectory, filename), Buffer.from(response.data, 'base64'))
}

async function run() {
  fs.mkdirSync(outputDirectory, { recursive: true })
  await connect()
  await openFormulaSampler()
  const records = []
  for (const size of sizes) {
    const record = await measure(size)
    record.failures = [
      record.pageScrollWidth > record.pageWidth + 1 && 'formula page horizontal overflow',
      record.panelScrollWidth > record.panelWidth + 1 && 'tool panel horizontal overflow',
      record.outside.length && `controls outside viewport: ${record.outside.join(', ')}`,
      record.clipped.length && `unexpected clipped text: ${record.clipped.join(', ')}`,
      record.brokenImages.length && `${record.brokenImages.length} broken images`,
      !record.artLoaded && 'portrait not loaded',
      !record.stickerLoaded && 'sticker not loaded',
      !record.parchmentVisible && 'parchment background missing',
      record.titleButtons !== 3 && `title button count ${record.titleButtons}`,
      !record.hasExperimentTypes && 'experiment type list incomplete',
      !record.hasFourPhases && 'A/B/A/B cards incomplete',
      !record.hasStrictReadOnly && 'strict read-only label missing',
      record.suspicious.length && `suspicious text: ${record.suspicious.join(', ')}`,
    ].filter(Boolean)
    await capture(`${size.name}--formulaSampler.png`)
    records.push(record)
    console.log(`${record.failures.length ? 'FAIL' : 'PASS'} ${size.name}${record.failures.length ? `: ${record.failures.join('; ')}` : ''}`)
  }
  const report = {
    captureMethod: 'Wails WebView2 CDP Page.captureScreenshot',
    sizes,
    passed: records.filter(record => !record.failures.length).length,
    total: records.length,
    consoleErrors,
    records,
  }
  fs.writeFileSync(path.join(outputDirectory, 'results.json'), `${JSON.stringify(report, null, 2)}\n`)
  socket.close()
  if (report.passed !== report.total || consoleErrors.length) process.exitCode = 1
}

run().catch(error => {
  console.error(error.stack || error)
  if (socket?.readyState === WebSocket.OPEN) socket.close()
  process.exitCode = 1
})
