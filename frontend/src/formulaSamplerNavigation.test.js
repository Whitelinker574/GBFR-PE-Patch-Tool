import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const home = readFileSync(new URL('./components/HomeJournal.vue', import.meta.url), 'utf8')

test('formula sampler is routed after the general runtime monitor in the read-only group', () => {
  assert.match(shell, /import FormulaSampler from ['"]\.\/FormulaSampler\.vue['"]/)
  assert.match(shell, /id:\s*['"]monitor['"][\s\S]*?items:\s*\[['"]ctMonitor['"],\s*['"]formulaSampler['"]\]/)
  assert.match(shell, /<FormulaSampler\s+v-else-if="activeTab === 'formulaSampler'"\s+@status="showStatus"\s*\/>/)
  assert.match(home, /id:\s*['"]formulaSampler['"],[\s\S]*?title:\s*['"]公式采样['"]/)
})

test('formula sampler reserves page-specific portrait and sticker assets', () => {
  assert.match(shell, /const formulaSamplerArt = new URL\(['"]\.\.\/assets\/gbfr\/cutouts\/formula-sampler-official-edge-safe\.webp['"], import\.meta\.url\)\.href/)
  assert.match(shell, /const formulaSamplerSticker = new URL\(['"]\.\.\/assets\/gbfr\/stickers\/formula-sampler\.webp['"], import\.meta\.url\)\.href/)
  assert.match(shell, /formulaSampler:\s*formulaSamplerArt/)
  assert.match(shell, /formulaSampler:\s*formulaSamplerSticker/)
  assert.ok(existsSync(new URL('./assets/gbfr/cutouts/formula-sampler-official-edge-safe.webp', import.meta.url)))
  assert.ok(existsSync(new URL('./assets/gbfr/stickers/formula-sampler.webp', import.meta.url)))
})
