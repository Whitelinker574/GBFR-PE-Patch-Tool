import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

import { uiTranslations } from './i18n-ui.js'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const home = readFileSync(new URL('./components/HomeJournal.vue', import.meta.url), 'utf8')
const detector = readFileSync(new URL('./components/RuntimeLoadoutDetector.vue', import.meta.url), 'utf8')
const assetManifest = JSON.parse(readFileSync(new URL('../public/generated/function-assets/manifest.json', import.meta.url), 'utf8'))

test('party detection, spatial controls, and selected-item diagnostics share one persistent implementation but expose separate destinations', () => {
  assert.match(shell, /runtimeMonitor:\s*\(\)\s*=>\s*import\(['"]\.\/RuntimePatchMonitor\.vue['"]\)/)
  assert.match(shell, /const RuntimePatchMonitor = asyncPage\(['"]runtimeMonitor['"]\)/)
  assert.match(shell, /runtimeMonitor:\s*\{\s*group:\s*['"]liveExtras['"]/)
  assert.match(shell, /spatialTools:\s*\{\s*group:\s*['"]liveExtras['"]/)
  assert.match(shell, /selectedItemMonitor:\s*\{\s*group:\s*['"]tools['"]/)
  assert.match(shell, /formulaSampler:\s*\{\s*group:\s*['"]tools['"]/)
  assert.match(shell, /id:\s*['"]liveExtras['"][\s\S]*?items:\s*\[['"]runtimeMonitor['"]/)
  assert.match(shell, /id:\s*['"]tools['"][^\n]*items:\s*\[[^\]]*['"]selectedItemMonitor['"][^\]]*['"]formulaSampler['"]/)
  assert.doesNotMatch(shell, /id:\s*['"]monitor['"]/)
  for (const group of ['save', 'tools']) {
    const match = shell.match(new RegExp(`\\{ id: '${group}'[^\\n]+items: \\[([^\\]]*)\\]`))
    assert.ok(match, `${group} navigation entry must exist`)
    assert.doesNotMatch(match[1], /['"]runtimeMonitor['"]/, `${group} must not contain party detection`)
  }
  assert.match(shell, /const runtimeMonitorMounted = ref\(false\)/)
  assert.match(shell, /if \(RUNTIME_MONITOR_MODES\[value\]\) runtimeMonitorMounted\.value = true/)
  assert.match(shell, /<RuntimePatchMonitor\s+v-if="runtimeMonitorMounted"\s+v-show="isRuntimeMonitorTab"\s+:mode="runtimeMonitorMode"\s+:page-active="isRuntimeMonitorTab"\s+@status="showStatus"\s+@deploy-loadout="deployRuntimeLoadout"\s*\/>/)
})

test('the home journal names party detection and spatial tools as separate user goals', () => {
  assert.doesNotMatch(home, /id:\s*['"]monitor['"]/)
  assert.match(home, /id:\s*['"]runtimeMonitor['"],\s*icon:\s*['"]队['"],\s*title:\s*['"]队友配装持续检测['"]/)
  assert.match(home, /id:\s*['"]spatialTools['"],\s*icon:\s*['"]标['"],\s*title:\s*['"]坐标与移动工具['"]/)
})

test('read-only monitoring does not surface the save-backup drawer', () => {
  assert.match(shell, /<SaveBackupDrawer\s+v-if="!\['formulaSampler', 'selectedItemMonitor'\]\.includes\(activeTab\)"\s+@status="showStatus"\s*\/>/)
})

test('runtime monitoring reserves unique function-specific portrait and sticker assets', () => {
  assert.match(assetManifest.assets.runtimeMonitor.art.variants.display.url, /runtime-monitor-official-edge-safe\.display\./)
  assert.match(assetManifest.assets.runtimeMonitor.sticker.variants.display.url, /runtime-monitor\.display\./)
  assert.match(shell, /\.tool-stage\[data-tool="runtimeMonitor"\]\s*\{\s*--art-scale:/)
  assert.match(shell, /runtimeMonitor:\s*\{[\s\S]*?speaker:\s*'尤斯提斯'/)
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
    '后台归档连续稳定的队伍配装批次': 'Archive consecutive stable party-loadout batches',
    '只读后台检测': 'Read-Only Background Detection',
    '只读 · 自动归档稳定队伍批次': 'Read Only · Automatic Stable Party Archive',
    '常驻归档连续稳定的队伍配装批次，并提供选中物品读取、稳定坐标诊断、一次性传送和按住移动的持续坐标飞行。': 'Continuously archive stable party-loadout batches, with selected-item reading, stable coordinate diagnostics, one-shot teleporting, and hold-to-move continuous coordinate flight.',
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
  assert.match(shell, /function toolTabTitle\(id\)[\s\S]*?id === 'runtimeMonitor'[\s\S]*?Automatic background detection after start[\s\S]*?开启后自动后台检测/)
  assert.match(shell, /function toolTagLabel\(id\)[\s\S]*?id === 'runtimeMonitor'[\s\S]*?'Background'[\s\S]*?'后台'/)
  assert.match(shell, /:title="toolTabTitle\(id\)"/)
  assert.match(shell, /class="switcher-tag live">\{\{ toolTagLabel\(id\) \}\}/)
})
