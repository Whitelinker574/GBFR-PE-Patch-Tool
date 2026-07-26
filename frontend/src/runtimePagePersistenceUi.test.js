import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = path => readFileSync(new URL(path, import.meta.url), 'utf8')
const shell = read('./components/PatchTool.vue')

test('runtime editors retain their lease and draft state across page navigation', () => {
  const cache = shell.slice(shell.indexOf('const cachedRuntimePages'), shell.indexOf('const state = reactive'))
  for (const id of ['sigilMemory', 'loadout', 'wrightstoneMemory', 'summon', 'overlimit', 'runtime', 'formulaSampler']) {
    assert.match(cache, new RegExp(`\\b${id}:`), `${id} must use the bounded runtime page cache`)
  }
  assert.match(shell, /<KeepAlive>\s*<component v-if="activeCachedRuntimePage"/)
  assert.doesNotMatch(shell, /<KeepAlive[^>]*\bmax=/)
})

test('cached polling pages pause UI clocks while hidden and resume the same session', () => {
  const contracts = [
    ['./components/SigilMemoryGenerator.vue', /onDeactivated\(stopPolling\)/, /onActivated\(\(\) => \{ if \(status\.hooked\) startPolling\(\) \}\)/],
    ['./components/WrightstoneMemoryGenerator.vue', /onDeactivated\(stopPolling\)/, /onActivated\(\(\) => \{ if \(status\.hooked\) startPolling\(\) \}\)/],
    ['./components/SigilLoadoutRestore.vue', /onDeactivated\(stopPolling\)/, /onActivated\(\(\) => \{[\s\S]*?isActive\.value[\s\S]*?window\.setTimeout\(poll, POLL_DELAY\)/],
    ['./components/FormulaSampler.vue', /onDeactivated\(stopObservationTimer\)/, /onActivated\(startObservationTimer\)/],
  ]
  for (const [path, paused, resumed] of contracts) {
    const source = read(path)
    assert.match(source, paused, `${path} must pause its UI clock when cached`)
    assert.match(source, resumed, `${path} must resume without reacquiring a different owner`)
  }
})

test('cold navigation keeps the old page until code and approved images are ready', () => {
  assert.match(shell, /await waitForTool\(id\)/)
  assert.match(shell, /warmImage\(functionArt\[id\], asset\?\.art\?\.variants\?\.full\?\.url\)/)
  assert.match(shell, /navigationError\.value = \{ id, message:/)
  assert.match(shell, /await afterNextPaint\(\)/)
  assert.match(shell, /<button v-if="navigationError"[^>]*@click="selectTool\(navigationError\.id\)"/)
})
