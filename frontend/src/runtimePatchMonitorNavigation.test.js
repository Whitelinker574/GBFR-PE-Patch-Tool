import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

import { uiTranslations } from './i18n-ui.js'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const home = readFileSync(new URL('./components/HomeJournal.vue', import.meta.url), 'utf8')

test('runtime monitor is routed as its own read-only memory-monitoring category', () => {
  assert.match(shell, /import RuntimePatchMonitor from ['"]\.\/RuntimePatchMonitor\.vue['"]/)
  assert.match(shell, /runtimeMonitor:\s*\{\s*group:\s*['"]monitor['"]/)
  assert.match(shell, /id:\s*['"]monitor['"][\s\S]*?items:\s*\[['"]runtimeMonitor['"],\s*['"]formulaSampler['"]\]/)
  for (const group of ['save', 'memory']) {
    const match = shell.match(new RegExp(`\\{ id: '${group}'[^\\n]+items: \\[([^\\]]*)\\]`))
    assert.ok(match, `${group} navigation entry must exist`)
    assert.doesNotMatch(match[1], /['"]runtimeMonitor['"]/, `${group} must not contain the read-only monitor`)
  }
  assert.match(shell, /<RuntimePatchMonitor\s+v-else-if="activeTab === 'runtimeMonitor'"\s+@status="showStatus"\s*\/>/)
})

test('the home journal exposes monitoring separately from injection and save editing', () => {
  assert.match(home, /id:\s*['"]monitor['"],\s*mark:\s*['"]测['"],\s*label:\s*['"]内存监测['"]/)
  assert.match(home, /id:\s*['"]runtimeMonitor['"],\s*icon:\s*['"]测['"],\s*title:\s*['"]运行监测['"]/);
  assert.match(home, /队伍快照、选中素材与关键物品/)
})

test('read-only monitoring does not surface the save-backup drawer', () => {
  assert.match(shell, /<SaveBackupDrawer\s+v-if="currentMeta\.group !== 'monitor'"\s+@status="showStatus"\s*\/>/)
})

test('runtime monitoring reserves unique function-specific portrait and sticker assets', () => {
  assert.match(shell, /const runtimeMonitorArt = new URL\(['"]\.\.\/assets\/gbfr\/cutouts\/runtime-monitor-official-edge-safe\.webp['"], import\.meta\.url\)\.href/)
  assert.match(shell, /const runtimeMonitorSticker = new URL\(['"]\.\.\/assets\/gbfr\/stickers\/runtime-monitor\.webp['"], import\.meta\.url\)\.href/)
  assert.match(shell, /runtimeMonitor:\s*runtimeMonitorArt/)
  assert.match(shell, /runtimeMonitor:\s*runtimeMonitorSticker/)
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
    '运行监测': 'Runtime Monitor',
    '队伍快照、选中素材与关键物品': 'Party snapshots, selected materials, and key items',
    '只读监测': 'Read-Only Monitoring',
    '只读 · 需连接游戏': 'Read Only · Game Connection Required',
  }
  for (const [chinese, english] of Object.entries(expected)) {
    assert.equal(uiTranslations[chinese], english, `missing exact translation for ${chinese}`)
  }
})
