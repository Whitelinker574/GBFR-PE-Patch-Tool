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

test('managed runtime companions stay visible from every page and refresh without releasing ownership', () => {
  assert.match(shell, /const runtimeCompanionStates = reactive\(/)
  assert.match(shell, /function updateRuntimeCompanionState\(value\)/)
  assert.match(shell, /GetRuntimeCompanionSummary/)
  assert.match(shell, /void refreshRuntimeCompanionSummary\(\)/)
  assert.match(shell, /document\.hidden/)
  assert.match(shell, /visibilitychange/)
  assert.match(shell, /v-for="companion in activeRuntimeCompanions"/)
  assert.match(shell, /@click="selectTool\(companion\.route\)"/)
  assert.match(shell, /runtimeMonitor:\s*\{ id: 'runtimeMonitor'/, 'background loadout detection must have a shell status entry')
  assert.match(shell, /runtimeMonitor:\s*\['配装检测', 'Loadout detector'\]/)
  assert.match(shell, /spatialTools:\s*\['空间移动', 'Spatial controls'\]/)
  assert.match(shell, /selectedItemMonitor:\s*\['物品捕获', 'Item capture'\]/)
  assert.match(shell, /taskRewardMultiplier:\s*\['任务奖励', 'Quest rewards'\]/)
  assert.match(shell, /taskRewardMultiplier:\s*'naturalDrop'/)
  assert.match(shell, /@runtime-state="updateRuntimeCompanionState"/)
  assert.match(shell, /companion\.stateLabel/)
  assert.match(shell, /已开启 · 点击返回对应页面关闭/)
  for (const component of ['AudioMixerLab', 'CameraLab', 'VirtualSigilLab']) {
    assert.match(shell, new RegExp(`<${component} v-if="activeTab === '[^"]+'"[^>]*@runtime-state="updateRuntimeCompanionState"`))
    const source = read(`./components/${component}.vue`)
    assert.match(source, /defineEmits\(\['status', 'runtime-state'\]\)/)
    assert.match(source, /emit\('runtime-state',/)
    assert.doesNotMatch(source, /onDeactivated\([^)]*Remove(?:AudioMixer|Camera|VirtualSigil)Mod/)
  }
  assert.match(read('./components/AudioMixerLab.vue'), /onActivated\(\(\) => \{ void refresh\(\) \}\)/)
  assert.match(read('./components/CameraLab.vue'), /onActivated\(\(\) => \{ void refresh\(\) \}\)/)
  for (const component of ['AudioMixerLab', 'CameraLab']) {
    const source = read(`./components/${component}.vue`)
    assert.match(source, /const runtimeActive = computed\(\(\) => workspace\.value\?\.installed === true && workspace\.value\?\.owned === true && workspace\.value\?\.state === 'active'\)/)
  }
  assert.match(read('./components/VirtualSigilLab.vue'), /\.virtual-workspace\s*\{[^}]*grid-template-columns:minmax\(0,1fr\)/s, 'virtual-sigil workspace must stay single-column at desktop widths so its actions remain reachable')
  assert.match(read('./components/VirtualSigilLab.vue'), /\.virtual-lab\s*\{[^}]*max-width:1600px[^}]*margin-inline:0 auto/s)
  assert.match(read('./components/VirtualSigilLab.vue'), /\.virtual-dock\s*\{[^}]*flex-direction:column/s)
})

test('CT status appears only for enabled or recovering features and names small active sets', () => {
  assert.match(shell, /activeFeatures:\s*\[\]/)
  assert.match(shell, /const showCTFeatureStatus = computed\(\(\) => ctFeatureSession\.releasePending \|\| ctFeatureSession\.activeCount > 0 \|\| ctFeatureSession\.recoveryCount > 0\)/)
  assert.match(shell, /names\.length === 1/)
  assert.match(shell, /names\.length === 2/)
  assert.match(shell, /v-if="showCTFeatureStatus"/)
  assert.doesNotMatch(shell, /v-if="ctFeatureSession\.connected \|\| ctFeatureSession\.releasePending"/)
  assert.match(shell, /@click="selectTool\(ctFeatureSession\.route\)"/)
  assert.match(shell, /summary\?\.id === 'runtimePatches'/)
  assert.match(shell, /\.titlebar-runtime-sessions\s*\{[^}]*overflow-x:auto;/s)
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
