import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const home = readFileSync(new URL('./components/HomeJournal.vue', import.meta.url), 'utf8')
const assetManifest = JSON.parse(readFileSync(new URL('../public/generated/function-assets/manifest.json', import.meta.url), 'utf8'))

test('formula sampler remains a strict read-only diagnostics page under tools and settings', () => {
  assert.match(shell, /formulaSampler:\s*\(\)\s*=>\s*import\(['"]\.\/FormulaSampler\.vue['"]\)/)
  assert.match(shell, /const FormulaSampler = asyncPage\(['"]formulaSampler['"]\)/)
  assert.match(shell, /formulaSampler:\s*\{\s*group:\s*['"]tools['"]/)
  assert.match(shell, /id:\s*['"]tools['"][^\n]*items:\s*\[[^\]]*['"]formulaSampler['"]/)
  assert.doesNotMatch(shell, /id:\s*['"]monitor['"]/)
  assert.match(shell, /runtimeMonitor:\s*\{\s*group:\s*['"]loadoutFlow['"]/)
  assert.match(shell, /cachedRuntimePages = Object\.freeze\(\{[\s\S]*?formulaSampler:\s*FormulaSampler/)
  assert.match(shell, /<KeepAlive>[\s\S]*?<component v-if="activeCachedRuntimePage"/)
  assert.doesNotMatch(home, /id:\s*['"]monitor['"]/)
})

test('formula sampler reserves page-specific portrait and sticker assets', () => {
  assert.match(assetManifest.assets.formulaSampler.art.variants.display.url, /formula-sampler-official-edge-safe\.display\./)
  assert.match(assetManifest.assets.formulaSampler.sticker.variants.display.url, /formula-sampler\.display\./)
  assert.match(shell, /asset\.art\.variants\.display\.url/)
  assert.match(shell, /asset\.sticker\.variants\.display\.url/)
  assert.ok(existsSync(new URL('./assets/gbfr/cutouts/formula-sampler-official-edge-safe.webp', import.meta.url)))
  assert.ok(existsSync(new URL('./assets/gbfr/stickers/formula-sampler.webp', import.meta.url)))
})
