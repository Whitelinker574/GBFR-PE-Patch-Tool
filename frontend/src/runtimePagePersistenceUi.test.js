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
  assert.match(shell, /<KeepAlive>\s*<component v-if="activeCachedRuntimePage"[^>]*:key="activeTab"/)
  assert.doesNotMatch(shell, /<KeepAlive[^>]*\bmax=/)
})

test('loadout and managed mod workspaces retain unfinished drafts across navigation', () => {
  for (const component of ['LoadoutViewer', 'NaturalDropLab', 'AudioMixerLab', 'CameraLab', 'VirtualSigilLab']) {
    assert.match(shell, new RegExp(`<KeepAlive>\\s*<${component} v-if="activeTab === '[^"]+'"`), `${component} must be cached instead of destroyed on navigation`)
  }
  assert.doesNotMatch(shell, /if \(id !== 'loadoutPresets'\) loadoutEditing\.value = false/)
})

test('cached polling pages pause UI clocks while hidden and resume the same session', () => {
  const contracts = [
    ['./components/SigilMemoryGenerator.vue', /onDeactivated\(stopPolling\)/, /onActivated\(\(\) => \{ if \(status\.hooked\) startPolling\(\) \}\)/],
    ['./components/WrightstoneMemoryGenerator.vue', /onDeactivated\(stopPolling\)/, /onActivated\(\(\) => \{ if \(status\.hooked\) startPolling\(\) \}\)/],
    ['./components/SigilLoadoutRestore.vue', /onDeactivated\(stopPolling\)/, /onActivated\(\(\) => \{[\s\S]*?isActive\.value[\s\S]*?window\.setTimeout\(poll, POLL_DELAY\)/],
    ['./components/FormulaSampler.vue', /onDeactivated\(stopObservationTimer\)/, /onActivated\(startObservationTimer\)/],
    ['./components/LoadoutEditor.vue', /onDeactivated\(pauseRuntimePanelLive\)/, /onActivated\(resumeRuntimePanelLive\)/],
    ['./components/LogsBattleArchive.vue', /onDeactivated\(deactivateLivePolling\)/, /onActivated\(activateLivePolling\)/],
    ['./components/RuntimeQOLLab.vue', /onDeactivated\(deactivatePolling\)/, /onActivated\(activatePolling\)/],
  ]
  for (const [path, paused, resumed] of contracts) {
    const source = read(path)
    assert.match(source, paused, `${path} must pause its UI clock when cached`)
    assert.match(source, resumed, `${path} must resume without reacquiring a different owner`)
  }
  assert.match(read('./components/LogsBattleArchive.vue'), /if \(!componentActive\) return/)
  assert.match(read('./components/RuntimeQOLLab.vue'), /if \(!componentActive\) return/)
})

test('cold navigation keeps the old page until code and approved images are ready', () => {
  assert.match(shell, /await waitForTool\(id\)/)
  assert.match(shell, /warmImage\(functionArt\[id\]\)/)
  assert.match(shell, /warmImage\(functionStickers\[id\]\)/)
  assert.match(shell, /navigationError\.value = \{ id, message:/)
  assert.match(shell, /await afterNextPaint\(\)/)
  assert.match(shell, /<button v-if="navigationError"[^>]*@click="selectTool\(navigationError\.id\)"/)
})

test('workspace scroll is isolated per page instead of leaking across navigation', () => {
  assert.match(shell, /const workspaceScroll = ref\(null\)/)
  assert.match(shell, /const pageScrollPositions = new Map\(\)/)
  assert.match(shell, /pageScrollPositions\.set\(previousPage, workspaceScroll\.value\.scrollTop\)/)
  assert.match(shell, /workspaceScroll\.value\.scrollTop = pageScrollPositions\.get\(id\) \|\| 0/)
  assert.match(shell, /<div ref="workspaceScroll" class="workspace-scroll"/)
})
