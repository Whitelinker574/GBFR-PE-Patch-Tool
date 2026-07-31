import { readFile, stat, writeFile, mkdir, readdir } from 'node:fs/promises'
import { gzipSync } from 'node:zlib'
import { dirname, isAbsolute, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

function parseArgs(argv) {
  const options = { enforce: false }
  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index]
    if (value === '--enforce') options.enforce = true
    else if (value === '--dist') options.dist = argv[++index]
    else if (value === '--budget') options.budget = argv[++index]
    else if (value === '--output') options.output = argv[++index]
    else throw new Error(`Unknown argument: ${value}`)
  }
  if (!options.dist) throw new Error('--dist is required')
  if (!options.budget) throw new Error('--budget is required')
  return options
}

function manifestEntry(manifest) {
  const entries = Object.entries(manifest).filter(([, item]) => item?.isEntry)
  if (entries.length !== 1) throw new Error(`Expected one frontend entry, found ${entries.length}`)
  return entries[0]
}

function collectInitialAssets(manifest, entryKey) {
  const visited = new Set()
  const js = new Set()
  const css = new Set()

  function visit(key) {
    if (visited.has(key)) return
    visited.add(key)
    const item = manifest[key]
    if (!item) throw new Error(`Manifest import is missing: ${key}`)
    if (item.file?.endsWith('.js')) js.add(item.file)
    for (const file of item.css || []) css.add(file)
    for (const dependency of item.imports || []) visit(dependency)
  }

  visit(entryKey)
  return { js: [...js].sort(), css: [...css].sort() }
}

async function assetMetrics(distPath, files) {
  const metrics = []
  for (const file of files) {
    const bytes = await readFile(join(distPath, file))
    metrics.push({ file, bytes: bytes.byteLength, gzipBytes: gzipSync(bytes).byteLength })
  }
  return metrics
}

async function walkFiles(root, relativePath = '') {
  const entries = await readdir(join(root, relativePath), { withFileTypes: true })
  const files = []
  for (const entry of entries) {
    const child = join(relativePath, entry.name)
    if (entry.isDirectory()) files.push(...await walkFiles(root, child))
    else if (entry.isFile()) files.push(child)
  }
  return files
}

export async function measureBundle({ dist, budget }) {
  const distPath = resolve(dist)
  const budgetPath = resolve(budget)
  const manifestPath = join(distPath, '.vite', 'manifest.json')
  const [manifest, limits, distInfo] = await Promise.all([
    readFile(manifestPath, 'utf8').then(JSON.parse),
    readFile(budgetPath, 'utf8').then(JSON.parse),
    stat(distPath),
  ])
  if (!distInfo.isDirectory()) throw new Error(`Frontend dist is not a directory: ${distPath}`)
  if (limits.schemaVersion !== 1) throw new Error(`Unsupported performance budget schema: ${limits.schemaVersion}`)

  const [entryKey] = manifestEntry(manifest)
  const initial = collectInitialAssets(manifest, entryKey)
  const [js, css] = await Promise.all([
    assetMetrics(distPath, initial.js),
    assetMetrics(distPath, initial.css),
  ])
  const allJsFiles = [...new Set(Object.values(manifest).map(item => item?.file).filter(file => file?.endsWith('.js')))].sort()
  const allCssFiles = [...new Set(Object.values(manifest).flatMap(item => item?.css || []).filter(file => file?.endsWith('.css')))].sort()
  const [asyncJs, asyncCss, distFiles] = await Promise.all([
    assetMetrics(distPath, allJsFiles.filter(file => !initial.js.includes(file))),
    assetMetrics(distPath, allCssFiles.filter(file => !initial.css.includes(file))),
    walkFiles(distPath),
  ])
  const rasterFiles = distFiles.filter(file => /\.(?:png|webp|jpe?g)$/iu.test(file))
  const functionAssetFiles = rasterFiles.filter(file => file.replaceAll('\\', '/').includes('generated/function-assets/'))
  const rasterStats = await Promise.all(rasterFiles.map(async file => ({ file: file.replaceAll('\\', '/'), bytes: (await stat(join(distPath, file))).size })))
  const functionAssetStats = rasterStats.filter(item => item.file.includes('generated/function-assets/'))
  const sum = values => values.reduce((total, item) => total + item.gzipBytes, 0)
  const max = (values, field) => values.reduce((largest, item) => Math.max(largest, item[field] || 0), 0)
  const totals = {
    initialJsGzipBytes: sum(js),
    initialCssGzipBytes: sum(css),
    largestAsyncJsGzipBytes: max(asyncJs, 'gzipBytes'),
    largestAsyncCssGzipBytes: max(asyncCss, 'gzipBytes'),
    largestRasterBytes: max(rasterStats, 'bytes'),
    largestFunctionAssetBytes: max(functionAssetStats, 'bytes'),
    functionAssetsTotalBytes: functionAssetStats.reduce((total, item) => total + item.bytes, 0),
  }
  return {
    schemaVersion: 1,
    measuredAt: new Date().toISOString(),
    entry: entryKey,
    initial: { js, css },
    async: { js: asyncJs, css: asyncCss },
    media: { rasterCount: rasterFiles.length, functionAssetCount: functionAssetFiles.length },
    totals,
    limits: {
      initialJsGzipBytes: limits.initialJsGzipBytes,
      initialCssGzipBytes: limits.initialCssGzipBytes,
      largestAsyncJsGzipBytes: limits.largestAsyncJsGzipBytes,
      largestAsyncCssGzipBytes: limits.largestAsyncCssGzipBytes,
      largestRasterBytes: limits.largestRasterBytes,
      largestFunctionAssetBytes: limits.largestFunctionAssetBytes,
      functionAssetsTotalBytes: limits.functionAssetsTotalBytes,
    },
  }
}

export function budgetFailures(report) {
  return Object.entries(report.limits)
    .filter(([name, limit]) => Number.isFinite(limit) && report.totals[name] > limit)
    .map(([name, limit]) => `${name}: ${report.totals[name]} > ${limit}`)
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  const report = await measureBundle(options)
  const failures = budgetFailures(report)
  if (options.output) {
    const outputPath = isAbsolute(options.output) ? options.output : resolve(options.output)
    await mkdir(dirname(outputPath), { recursive: true })
    await writeFile(outputPath, `${JSON.stringify(report, null, 2)}\n`, 'utf8')
  }
  process.stdout.write(`${JSON.stringify(report, null, 2)}\n`)
  if (options.enforce && failures.length) {
    throw new Error(`Frontend performance budget exceeded:\n${failures.join('\n')}`)
  }
}

const executedPath = process.argv[1] ? resolve(process.argv[1]) : ''
if (executedPath === fileURLToPath(import.meta.url)) {
  main().catch(error => {
    process.stderr.write(`${error.message}\n`)
    process.exitCode = 1
  })
}
