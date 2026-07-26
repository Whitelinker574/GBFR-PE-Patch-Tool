import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

import { uiTranslations } from './i18n-ui.js'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const home = readFileSync(new URL('./components/HomeJournal.vue', import.meta.url), 'utf8')
const detector = readFileSync(new URL('./components/RuntimeLoadoutDetector.vue', import.meta.url), 'utf8')
const assetManifest = JSON.parse(readFileSync(new URL('../public/generated/function-assets/manifest.json', import.meta.url), 'utf8'))

test('runtime monitor is routed as its own read-only memory-monitoring category', () => {
  assert.match(shell, /runtimeMonitor:\s*\(\)\s*=>\s*import\(['"]\.\/RuntimePatchMonitor\.vue['"]\)/)
  assert.match(shell, /const RuntimePatchMonitor = asyncPage\(['"]runtimeMonitor['"]\)/)
  assert.match(shell, /runtimeMonitor:\s*\{\s*group:\s*['"]monitor['"]/)
  assert.match(shell, /id:\s*['"]monitor['"][\s\S]*?items:\s*\[['"]runtimeMonitor['"],\s*['"]formulaSampler['"]\]/)
  for (const group of ['save', 'memory']) {
    const match = shell.match(new RegExp(`\\{ id: '${group}'[^\\n]+items: \\[([^\\]]*)\\]`))
    assert.ok(match, `${group} navigation entry must exist`)
    assert.doesNotMatch(match[1], /['"]runtimeMonitor['"]/, `${group} must not contain the read-only monitor`)
  }
  assert.match(shell, /const runtimeMonitorMounted = ref\(false\)/)
  assert.match(shell, /if \(value === 'runtimeMonitor'\) runtimeMonitorMounted\.value = true/)
  assert.match(shell, /<RuntimePatchMonitor\s+v-if="runtimeMonitorMounted"\s+v-show="activeTab === 'runtimeMonitor'"\s+@status="showStatus"\s+@deploy-loadout="deployRuntimeLoadout"\s*\/>/)
})

test('the home journal exposes monitoring separately from injection and save editing', () => {
  assert.match(home, /id:\s*['"]monitor['"],\s*mark:\s*['"]测['"],\s*label:\s*['"]内存监测['"]/)
  assert.match(home, /id:\s*['"]runtimeMonitor['"],\s*icon:\s*['"]测['"],\s*title:\s*['"]角色配装检测['"]/);
  assert.match(home, /后台自动归档每场任务的队伍配装/)
})

test('read-only monitoring does not surface the save-backup drawer', () => {
  assert.match(shell, /<SaveBackupDrawer\s+v-if="currentMeta\.group !== 'monitor'"\s+@status="showStatus"\s*\/>/)
})

test('runtime monitoring reserves unique function-specific portrait and sticker assets', () => {
  assert.match(assetManifest.assets.runtimeMonitor.art.variants.full.url, /runtime-monitor-official-edge-safe\.full\./)
  assert.match(assetManifest.assets.runtimeMonitor.sticker.variants.full.url, /runtime-monitor\.full\./)
  assert.match(shell, /\.tool-stage\[data-tool="runtimeMonitor"\]\s*\{\s*--art-scale:/)
  assert.match(shell, /runtimeMonitor:\s*\{[\s\S]*?speaker:\s*'尤斯塔斯'/)
  assert.doesNotMatch(shell, /runtimeMonitor:\s*\{[\s\S]*?speaker:\s*'碧'/)
  for (const path of [
    './assets/gbfr/cutouts/runtime-monitor-official-edge-safe.webp',
    './assets/gbfr/stickers/runtime-monitor.webp',
  ]) {
    assert.ok(existsSync(new URL(path, import.meta.url)), `${path} must be present in the production asset set`)
  }
})

test('the quest page uses Yodarha\'s verified Simplified Chinese name', () => {
  assert.match(shell, /patchQuest:\s*\{[\s\S]*?speaker:\s*'尤达拉哈'/)
  assert.doesNotMatch(shell, /speaker:\s*'尤达哈拉'/)
})

test('new shell and home copy has exact English localization', () => {
  const expected = {
    '内存监测': 'Memory Monitoring',
    '只读读取运行中游戏数据，不修改物品或存档': 'Read live game data without modifying items or save data',
    '角色配装检测': 'Character Loadout Detection',
    '后台自动归档每场任务的队伍配装': 'Automatically archive party loadouts for every quest',
    '只读后台检测': 'Read-Only Background Detection',
    '只读 · 自动记录任务配装': 'Read Only · Automatic Quest Loadout Archive',
    '一次开启后常驻后台，自动识别每场任务的稳定队伍，并把角色配装按场次保存在本机。': 'Start once to continuously identify stable quest parties in the background and archive their character loadouts locally by quest.',
    '开启角色配装检测': 'Start Character Loadout Detection',
    '检测器只读游戏数据，可与其他连接功能同时使用；选中物品读取是同页的独立工具。': 'The detector reads game data only and can run alongside other connections. Selected-item reading is a separate tool on the same page.',
    '开启后自动后台检测': 'Automatic background detection after start',
  }
  for (const [chinese, english] of Object.entries(expected)) {
    assert.equal(uiTranslations[chinese], english, `missing exact translation for ${chinese}`)
  }
})

test('loadout detection is a persistent background service with local quest history', () => {
  assert.match(detector, /RuntimeLoadoutDetectorStart/)
  assert.match(detector, /RuntimeLoadoutDetectorStatus/)
  assert.match(detector, /RuntimeLoadoutDetectorHistory/)
  assert.match(detector, /EventsOn\(DETECTOR_STATUS_EVENT, next => \{ void acceptStatus\(next\) \}\)/)
  assert.match(detector, /const DETECTOR_STATUS_EVENT = 'runtime-loadout-detector:status'/)
  assert.doesNotMatch(detector, /setInterval|pollTimer/)
  assert.match(detector, /onBeforeUnmount\(\(\) => \{[\s\S]*?stopStatusEvents\(\)/)
  assert.doesNotMatch(detector.match(/onBeforeUnmount\([\s\S]*?\n\}\)/)?.[0] || '', /RuntimeLoadoutDetectorStop/)
  assert.match(detector, /record\.members/)
  assert.match(detector, /RuntimeLoadoutDetectorPublish/)
  assert.match(detector, /RuntimeLoadoutDetectorShare/)
  assert.match(shell, /id === 'runtimeMonitor'[^\n]+开启后自动后台检测/)
  assert.match(shell, /id === 'runtimeMonitor'[^\n]+class="switcher-tag live">后台/)
})
