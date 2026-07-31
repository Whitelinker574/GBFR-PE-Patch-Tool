import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const home = readFileSync(new URL('./components/HomeJournal.vue', import.meta.url), 'utf8')
const runtimeMonitor = readFileSync(new URL('./components/RuntimePatchMonitor.vue', import.meta.url), 'utf8')
const virtualSigils = readFileSync(new URL('./components/VirtualSigilLab.vue', import.meta.url), 'utf8')

test('party loadout detection, spatial controls, and low-frequency memory inspection are separate destinations', () => {
  assert.match(shell, /runtimeMonitor:\s*\{[\s\S]*?title:\s*'队友配装持续检测'/)
  assert.match(shell, /spatialTools:\s*\{[\s\S]*?title:\s*'坐标与移动工具'/)
  assert.match(shell, /selectedItemMonitor:\s*\{[\s\S]*?group:\s*'tools'[\s\S]*?title:\s*'选中物品查看（只读）'/)
  assert.match(shell, /formulaSampler:\s*\{[\s\S]*?group:\s*'tools'/)
  assert.doesNotMatch(shell, /\{ id: 'monitor',/)
  assert.match(shell, /runtimeMonitorMode/)
  assert.match(shell, /:mode="runtimeMonitorMode"/)
  assert.match(runtimeMonitor, /mode:\s*\{[\s\S]*?validator:\s*value => \['party', 'spatial', 'items'\]\.includes\(value\)/)
  assert.doesNotMatch(runtimeMonitor, /class="monitor-tabs/)
})

test('runtime and game-file features use names that describe the actual user outcome', () => {
  assert.match(shell, /runtimeQOL:\s*\{[\s\S]*?title:\s*'显示与房间工具'/)
  assert.match(shell, /naturalDrop:\s*\{[\s\S]*?title:\s*'掉落与锻造规则（游戏文件）'/)
  assert.match(shell, /不会直接向存档背包添加物品/)
  assert.doesNotMatch(shell, /title:\s*'游戏便利运行时'/)
  assert.doesNotMatch(shell, /title:\s*'天然掉落部署'/)
  assert.match(home, /title:\s*'队友配装持续检测'/)
  assert.match(home, /title:\s*'显示与房间工具'/)
  assert.match(home, /title:\s*'坐标与移动工具'/)
})

test('collapsed navigation renders one aligned 48px control instead of nested selected frames', () => {
  assert.match(shell, /\.sidebar-collapsed \.primary-nav\s*\{[^}]*width:48px;[^}]*align-self:center;/s)
  assert.match(shell, /\.sidebar-collapsed \.nav-item\s*\{[^}]*width:48px;[^}]*height:48px;[^}]*padding:0;/s)
  assert.match(shell, /\.sidebar-collapsed \.nav-mark\s*\{[^}]*width:100%;[^}]*height:100%;[^}]*border:0;/s)
  assert.match(shell, /\.sidebar-collapsed \.nav-item\.active\s*\{[^}]*box-shadow:inset 3px 0 0 var\(--selected-bar\);/s)
})

test('selected virtual-sigil character labels use an explicit high-contrast foreground', () => {
  assert.match(virtualSigils, /\.character-strip button\.active\s*\{[^}]*color:var\(--text-on-accent\);[^}]*background:var\(--accent\);/s)
  assert.match(virtualSigils, /\.character-strip button\.active :is\(span,small\)\s*\{[^}]*color:var\(--text-on-accent\);/s)
  assert.match(virtualSigils, /\.virtual-slots article\.active\s*\{[^}]*color:var\(--text-on-accent\);[^}]*background:var\(--accent\);/s)
  assert.match(virtualSigils, /\.virtual-slots article\.active :is\(\.slot-index,b,small\)\s*\{[^}]*color:var\(--text-on-accent\);/s)
  assert.match(virtualSigils, /\.inventory-list button\.selected\s*\{[^}]*color:var\(--text-on-accent\);[^}]*background:var\(--accent\);/s)
  assert.match(virtualSigils, /\.inventory-list button\.selected :is\(b,small,em\)\s*\{[^}]*color:var\(--text-on-accent\);/s)
})
