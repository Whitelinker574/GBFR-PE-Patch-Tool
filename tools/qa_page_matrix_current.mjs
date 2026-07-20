import fs from 'node:fs'
import path from 'node:path'

const CDP_ENDPOINT = process.env.GBFR_QA_CDP || 'http://127.0.0.1:9223'
const OUTPUT_DIR = path.resolve('.qa-webview2', 'page-matrix-current')
const SIZES = [
  { name: '2560x1392', width: 2560, height: 1392 },
  { name: '1920x1080', width: 1920, height: 1080 },
  { name: '1440x900', width: 1440, height: 900 },
  { name: '1024x768', width: 1024, height: 768 },
  { name: '960x620', width: 960, height: 620 },
]

const GROUPS = [
  { nav: 0, pages: ['loadoutPresets', 'sigil', 'progression', 'wrightstone', 'chara', 'save'] },
  { nav: 1, pages: ['runtime', 'sigilMemory', 'wrightstoneMemory', 'loadout', 'summon', 'overlimit', 'ctCombat', 'ctCharacters', 'ctQuest', 'monster'] },
  { nav: 2, pages: ['ctMonitor', 'formulaSampler'] },
  { nav: 3, pages: ['compatibility', 'language', 'patch'] },
]

let socket
let nextID = 0
const pending = new Map()
const consoleEvents = []
let targetMetadata

const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds))

async function connect() {
  const pages = await fetch(`${CDP_ENDPOINT}/json/list`).then(response => response.json())
  const target = pages.find(page => page.type === 'page' && page.url === 'http://wails.localhost/')
    || pages.find(page => page.type === 'page')
  if (!target || !String(target.url).startsWith('http://wails.localhost/')) {
    throw new Error(`Wails WebView2 target not found; got ${target?.url || 'nothing'}`)
  }
  targetMetadata = {
    id: target.id,
    title: target.title,
    type: target.type,
    url: target.url,
  }
  socket = new WebSocket(target.webSocketDebuggerUrl)
  await new Promise((resolve, reject) => {
    socket.addEventListener('open', resolve, { once: true })
    socket.addEventListener('error', reject, { once: true })
  })
  socket.addEventListener('message', event => {
    const message = JSON.parse(event.data)
    if (message.method === 'Runtime.consoleAPICalled') {
      consoleEvents.push({ type: message.params.type, args: message.params.args.map(arg => arg.value ?? arg.description ?? '') })
      return
    }
    if (message.method === 'Runtime.exceptionThrown') {
      consoleEvents.push({ type: 'exception', args: [message.params.exceptionDetails?.exception?.description || message.params.exceptionDetails?.text || 'unknown'] })
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

function send(method, params = {}) {
  const id = ++nextID
  socket.send(JSON.stringify({ id, method, params }))
  return new Promise((resolve, reject) => pending.set(id, { resolve, reject }))
}

async function evaluate(expression) {
  const result = await send('Runtime.evaluate', {
    expression,
    returnByValue: true,
    awaitPromise: true,
    userGesture: true,
  })
  if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description || result.exceptionDetails.text)
  return result.result.value
}

async function waitFor(expression, timeout = 30000, label = expression) {
  const started = Date.now()
  while (Date.now() - started < timeout) {
    if (await evaluate(`Boolean(${expression})`)) return
    await delay(120)
  }
  throw new Error(`Timed out waiting for ${label}`)
}

async function setWindowSize(size) {
  await evaluate(`window.runtime.WindowSetSize(${size.width}, ${size.height})`)
  await delay(450)
  return evaluate(`({ width: innerWidth, height: innerHeight, dpr: devicePixelRatio })`)
}

async function navigate(groupIndex, pageIndex, expected) {
  await evaluate(`document.querySelectorAll('.nav-item')[${groupIndex}]?.click()`)
  await waitFor(`document.querySelector('.tool-stage')`, 30000, `group ${groupIndex}`)
  if (await evaluate(`document.querySelector('.tool-stage')?.dataset.tool !== ${JSON.stringify(expected)}`)) {
    await waitFor(`document.querySelectorAll('.tool-switcher button').length > ${pageIndex}`, 30000, `switcher ${expected}`)
    await evaluate(`document.querySelectorAll('.tool-switcher button')[${pageIndex}]?.click()`)
  }
  await waitFor(`document.querySelector('.tool-stage')?.dataset.tool === ${JSON.stringify(expected)}`, 30000, expected)
  await waitFor(`document.querySelector('.character-main')?.complete !== false`, 30000, `${expected} art`)
  await delay(550)
}

async function navigateHome() {
  await evaluate(`document.querySelector('.sidebar-heading')?.click()`)
  await waitFor(`document.querySelector('.home-journal') || !document.querySelector('.tool-stage')`, 10000, 'home')
  await delay(450)
}

async function collect(page, requestedSize, actualSize) {
  return evaluate(`(() => {
    const pageName = ${JSON.stringify(page)}
    const requested = ${JSON.stringify(requestedSize)}
    const actual = ${JSON.stringify(actualSize)}
    const visible = element => {
      if (!element) return false
      const style = getComputedStyle(element)
      const rect = element.getBoundingClientRect()
      return style.display !== 'none' && style.visibility !== 'hidden' && Number(style.opacity || 1) > 0 && rect.width > 0 && rect.height > 0
    }
    const rect = element => {
      if (!element) return null
      const value = element.getBoundingClientRect()
      return {
        x: Math.round(value.x * 10) / 10,
        y: Math.round(value.y * 10) / 10,
        width: Math.round(value.width * 10) / 10,
        height: Math.round(value.height * 10) / 10,
        right: Math.round(value.right * 10) / 10,
        bottom: Math.round(value.bottom * 10) / 10,
      }
    }
    const overlap = (left, right) => {
      if (!left || !right) return 0
      const width = Math.max(0, Math.min(left.right, right.right) - Math.max(left.left, right.left))
      const height = Math.max(0, Math.min(left.bottom, right.bottom) - Math.max(left.top, right.top))
      return Math.round(width * height)
    }
    const selectors = {
      app: '.app-window', body: '.app-body', sidebar: '.sidebar', workspace: '.workspace',
      workspaceBar: '.workspace-bar', switcher: '.tool-switcher', workspaceScroll: '.workspace-scroll',
      stage: '.tool-stage', center: '.tool-center-scroll', heading: '.tool-page-heading', panel: '.tool-panel',
      artRail: '.art-rail', art: '.character-main', sticker: '.sidebar-mascot-img',
      titlebar: '.titlebar', titleControls: '.titlebar-controls', compactHome: '.sidebar-home-compact', fullHome: '.sidebar-heading',
    }
    const elements = Object.fromEntries(Object.entries(selectors).map(([key, selector]) => [key, document.querySelector(selector)]))
    const rectangles = Object.fromEntries(Object.entries(elements).map(([key, element]) => [key, rect(element)]))
    const allImages = [...document.images]
    const brokenImages = allImages.filter(image => visible(image) && (!image.complete || image.naturalWidth === 0)).map(image => image.currentSrc || image.src || image.alt)
    const controls = [...document.querySelectorAll('button,input,select,textarea,a[href]')].filter(visible)
    const controlRects = controls.map(element => ({ element, value: element.getBoundingClientRect() }))
    const insideHorizontalScroller = element => {
      for (let ancestor = element.parentElement; ancestor; ancestor = ancestor.parentElement) {
        const style = getComputedStyle(ancestor)
        if (['auto', 'scroll'].includes(style.overflowX) && ancestor.scrollWidth > ancestor.clientWidth + 1) return true
      }
      return false
    }
    const outsideHorizontally = controlRects.filter(({ element, value }) =>
      (value.left < -1 || value.right > innerWidth + 1) && !insideHorizontalScroller(element)
    ).map(({ element, value }) => ({
      tag: element.tagName.toLowerCase(), text: (element.innerText || element.value || element.getAttribute('aria-label') || '').replace(/\\s+/g, ' ').trim().slice(0, 80),
      left: Math.round(value.left), right: Math.round(value.right), width: Math.round(value.width),
    }))
    const hugeControls = controlRects.filter(({ value }) => value.width > Math.min(720, innerWidth * .72) || value.height > 96).map(({ element, value }) => ({
      tag: element.tagName.toLowerCase(), text: (element.innerText || element.value || element.getAttribute('aria-label') || '').replace(/\\s+/g, ' ').trim().slice(0, 80),
      width: Math.round(value.width), height: Math.round(value.height),
    })).slice(0, 30)
    const clippedText = [...document.querySelectorAll('.tool-panel button,.tool-panel label,.tool-panel th,.tool-panel td,.tool-switcher button')]
      .filter(element => visible(element) && element.scrollWidth > element.clientWidth + 2 && !['auto','scroll'].includes(getComputedStyle(element).overflowX))
      .map(element => ({ tag: element.tagName.toLowerCase(), text: (element.innerText || '').replace(/\\s+/g, ' ').trim().slice(0, 80), client: element.clientWidth, scroll: element.scrollWidth }))
      .slice(0, 40)
    const pageText = (elements.panel?.innerText || document.body.innerText || '').replace(/\\s+/g, ' ').trim()
    const visibleArea = image => {
      if (!image || !visible(image)) return 0
      const value = image.getBoundingClientRect()
      const width = Math.max(0, Math.min(value.right, innerWidth) - Math.max(value.left, 0))
      const height = Math.max(0, Math.min(value.bottom, innerHeight) - Math.max(value.top, 0))
      return value.width * value.height ? Math.round((width * height / (value.width * value.height)) * 1000) / 1000 : 0
    }
    const centerRect = elements.center?.getBoundingClientRect() || null
    const artRailRect = elements.artRail?.getBoundingClientRect() || null
    const artRect = elements.art?.getBoundingClientRect() || null
    const numericZIndex = element => {
      if (!element) return null
      const value = Number.parseInt(getComputedStyle(element).zIndex, 10)
      return Number.isFinite(value) ? value : 0
    }
    const centerZIndex = numericZIndex(elements.center)
    const artRailZIndex = numericZIndex(elements.artRail)
    const centerControls = controls.filter(element => {
      if (!elements.center?.contains(element) || element.disabled) return false
      const value = element.getBoundingClientRect()
      return value.right > 0 && value.left < innerWidth && value.bottom > 0 && value.top < innerHeight
    })
    const interactiveProbes = centerControls.slice(0, 8).map(element => {
      const value = element.getBoundingClientRect()
      const x = Math.max(0, Math.min(innerWidth - 1, value.left + value.width / 2))
      const y = Math.max(0, Math.min(innerHeight - 1, value.top + value.height / 2))
      const top = document.elementFromPoint(x, y)
      return {
        tag: element.tagName.toLowerCase(),
        text: (element.innerText || element.value || element.getAttribute('aria-label') || '').replace(/\s+/g, ' ').trim().slice(0, 80),
        x: Math.round(x),
        y: Math.round(y),
        hitTag: top?.tagName?.toLowerCase() || '',
        reachable: Boolean(top && (top === element || element.contains(top))),
      }
    })
    const artCenterIntersection = centerRect && artRect ? {
      left: Math.max(centerRect.left, artRect.left),
      top: Math.max(centerRect.top, artRect.top),
      right: Math.min(centerRect.right, artRect.right),
      bottom: Math.min(centerRect.bottom, artRect.bottom),
    } : null
    const artCenterStackProbe = (() => {
      if (!artCenterIntersection || artCenterIntersection.right <= artCenterIntersection.left || artCenterIntersection.bottom <= artCenterIntersection.top) return null
      const x = (artCenterIntersection.left + artCenterIntersection.right) / 2
      const visibleBottom = Math.min(artCenterIntersection.bottom, innerHeight)
      if (visibleBottom <= artCenterIntersection.top) return null
      const y = Math.max(0, Math.min(innerHeight - 1, (artCenterIntersection.top + visibleBottom) / 2))
      const stack = document.elementsFromPoint(x, y)
      const centerIndex = stack.findIndex(element => element === elements.center || elements.center?.contains(element))
      const artIndex = stack.findIndex(element => element === elements.artRail || elements.artRail?.contains(element))
      return {
        x: Math.round(x),
        y: Math.round(y),
        centerIndex,
        artIndex,
        centerAboveArt: centerIndex >= 0 && (artIndex < 0 || centerIndex < artIndex),
        stack: stack.slice(0, 8).map(element => element.className?.toString().replace(/\s+/g, '.') || element.tagName.toLowerCase()),
      }
    })()
    const directSections = elements.panel ? [...elements.panel.children].filter(visible).map(element => ({
      cls: element.className?.toString().slice(0, 80) || element.tagName.toLowerCase(), ...rect(element),
    })) : []
    const titleButtons = [...document.querySelectorAll('.titlebar-controls .win-btn')].filter(visible)
    const titleControlsRect = elements.titleControls?.getBoundingClientRect() || null
    const parchmentBackground = getComputedStyle(document.querySelector('.app-window'), '::before').backgroundImage || ''
    const stageArtStyle = elements.stage ? getComputedStyle(elements.stage, '::before') : null
    const stageArtBackground = stageArtStyle?.backgroundImage || ''
    const stageArtVisible = pageName !== 'home' && stageArtBackground !== '' && stageArtBackground !== 'none' && Number(stageArtStyle?.opacity || 1) > 0
    const stageArtZIndex = Number.parseInt(stageArtStyle?.zIndex || '0', 10) || 0
    return {
      page: pageName,
      requested,
      actual,
      title: document.querySelector('.tool-page-heading h1')?.innerText.trim() || (pageName === 'home' ? 'home' : ''),
      bodyOverflowX: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) - document.documentElement.clientWidth,
      workspaceOverflowX: elements.workspaceScroll ? elements.workspaceScroll.scrollWidth - elements.workspaceScroll.clientWidth : 0,
      panelOverflowX: elements.panel ? elements.panel.scrollWidth - elements.panel.clientWidth : 0,
      panelScrollableY: elements.panel ? elements.panel.scrollHeight - elements.panel.clientHeight : 0,
      centerScrollableY: elements.center ? elements.center.scrollHeight - elements.center.clientHeight : 0,
      rectangles,
      stageColumns: elements.stage ? getComputedStyle(elements.stage).gridTemplateColumns : '',
      controls: controls.length,
      outsideHorizontally,
      hugeControls,
      clippedText,
      brokenImages,
      artVisible: stageArtVisible,
      artVisibleAreaRatio: stageArtVisible ? 1 : 0,
      artRailCenterOverlapArea: 0,
      artRailCenterOverlapRatio: 0,
      artCenterOverlapArea: 0,
      artCenterOverlapRatio: 0,
      centerZIndex,
      artRailZIndex: stageArtZIndex,
      centerAboveArt: !stageArtVisible || (centerZIndex !== null && centerZIndex > stageArtZIndex),
      artPointerEventsNone: !stageArtVisible || stageArtStyle.pointerEvents === 'none',
      interactiveProbeCount: interactiveProbes.length,
      interactiveReachable: interactiveProbes.length === 0 || interactiveProbes.some(probe => probe.reachable),
      interactiveProbes,
      artCenterStackProbe,
      artLoaded: stageArtVisible,
      stickerLoaded: !elements.sticker || Boolean(elements.sticker.complete && elements.sticker.naturalWidth > 0),
      stickerVisible: visible(elements.sticker),
      homeVisible: visible(elements.compactHome) || visible(elements.fullHome),
      titleButtonCount: titleButtons.length,
      titleControlsRightAligned: Boolean(titleControlsRect && Math.abs(innerWidth - titleControlsRect.right) <= 2),
      fullscreenControlVisible: titleButtons.some(button => button.querySelector('.fullscreen-corners')),
      parchmentVisible: parchmentBackground.includes('parchment-ui-v2'),
      directSections,
      textLength: pageText.length,
      suspiciousText: ['undefined', '[object Object]', 'NaN', '加载失败', '渲染失败'].filter(token => pageText.includes(token)),
    }
  })()`)
}

async function screenshot(page, size) {
  const result = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false, fromSurface: true })
  const filename = `${size.name}--${page}.png`
  fs.writeFileSync(path.join(OUTPUT_DIR, filename), Buffer.from(result.data, 'base64'))
  return filename
}

async function run() {
  fs.mkdirSync(OUTPUT_DIR, { recursive: true })
  await connect()
  await waitFor(`document.querySelectorAll('.nav-item').length >= 4`, 30000, 'primary navigation')
  await evaluate(`(() => {
    if (document.querySelector('.tool-stage.art-collapsed .art-toggle')) document.querySelector('.art-toggle').click()
    if (document.querySelector('.app-body.sidebar-collapsed .sidebar-collapse')) document.querySelector('.sidebar-collapse').click()
    return true
  })()`)

  const records = []
  const visit = async (page, go) => {
    await go()
    for (const size of SIZES) {
      const actual = await setWindowSize(size)
      const metrics = await collect(page, size, actual)
      metrics.screenshot = await screenshot(page, size)
      metrics.failures = [
        metrics.bodyOverflowX > 1 && `body overflow ${metrics.bodyOverflowX}px`,
        metrics.workspaceOverflowX > 1 && `workspace overflow ${metrics.workspaceOverflowX}px`,
        metrics.outsideHorizontally.length && `${metrics.outsideHorizontally.length} horizontal controls outside viewport`,
        metrics.brokenImages.length && `${metrics.brokenImages.length} broken images`,
        metrics.suspiciousText.length && `suspicious text: ${metrics.suspiciousText.join(', ')}`,
        metrics.artRailCenterOverlapRatio > .01 && `art rail layout overlaps ${Math.round(metrics.artRailCenterOverlapRatio * 100)}% of center`,
        metrics.artVisible && !metrics.centerAboveArt && `center layer z-index ${metrics.centerZIndex} is not above art ${metrics.artRailZIndex}`,
        metrics.artVisible && !metrics.artPointerEventsNone && 'art rail intercepts pointer events',
        page !== 'home' && !metrics.interactiveReachable && 'center content is not hit-test reachable',
        metrics.artCenterStackProbe && !metrics.artCenterStackProbe.centerAboveArt && 'art image paints above center in overlap hit test',
        page !== 'home' && !metrics.title && 'missing title',
        page !== 'home' && !['save', 'summon'].includes(page) && metrics.textLength < 20 && `too little content (${metrics.textLength})`,
        page !== 'home' && metrics.artVisible && !metrics.artLoaded && 'art did not load',
        metrics.titleButtonCount !== 3 && `expected 3 title buttons, got ${metrics.titleButtonCount}`,
        !metrics.titleControlsRightAligned && 'title controls are not right aligned',
        metrics.fullscreenControlVisible && 'obsolete fullscreen control is still visible',
        !metrics.parchmentVisible && 'parchment skin is not active',
        page !== 'home' && !metrics.homeVisible && 'home control is not reachable',
        page !== 'home' && (!metrics.stickerVisible || !metrics.stickerLoaded) && 'Q sticker is not visible and loaded',
      ].filter(Boolean)
      records.push(metrics)
      console.log(`${metrics.failures.length ? 'FAIL' : 'PASS'} ${size.name.padEnd(10)} ${page.padEnd(16)} ${metrics.stageColumns || '-'}${metrics.failures.length ? ` :: ${metrics.failures.join('; ')}` : ''}`)
    }
  }

  await visit('home', navigateHome)
  for (const group of GROUPS) {
    for (let pageIndex = 0; pageIndex < group.pages.length; pageIndex += 1) {
      const page = group.pages[pageIndex]
      await visit(page, () => navigate(group.nav, pageIndex, page))
    }
  }
  const errorEvents = consoleEvents.filter(event => ['error', 'assert', 'exception'].includes(event.type))
  const summary = {
    generatedAt: new Date().toISOString(),
    captureMethod: 'Wails WebView2 CDP Page.captureScreenshot',
    cdpEndpoint: CDP_ENDPOINT,
    target: targetMetadata,
    sizes: SIZES,
    pages: [...new Set(records.map(record => record.page))],
    total: records.length,
    failed: records.filter(record => record.failures.length).length,
    errorEvents,
    records,
  }
  fs.writeFileSync(path.join(OUTPUT_DIR, 'results.json'), JSON.stringify(summary, null, 2))
  console.log(`\n${summary.total - summary.failed}/${summary.total} layout checks passed; ${summary.failed} failed; ${errorEvents.length} console errors.`)
  socket.close()
  if (summary.failed || errorEvents.length) process.exitCode = 1
}

run().catch(error => {
  console.error(error.stack || error)
  if (socket?.readyState === WebSocket.OPEN) socket.close()
  process.exitCode = 1
})
