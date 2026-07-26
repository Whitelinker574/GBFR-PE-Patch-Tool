import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')
const monitor = readFileSync(new URL('./components/RuntimePatchMonitor.vue', import.meta.url), 'utf8')
const detector = readFileSync(new URL('./components/RuntimeLoadoutDetector.vue', import.meta.url), 'utf8')
const preview = readFileSync(new URL('./components/CapturedLoadoutPreview.vue', import.meta.url), 'utf8')

test('GBFR Logs import is a dedicated page instead of a save-row modal action', () => {
  const saveRow = viewer.match(/<div class="save-row[\s\S]*?<\/div>/)?.[0] || ''
  assert.doesNotMatch(saveRow, /从 GBFR Logs 导入/)
  assert.match(viewer, /class="logs-library-entry[^"]*"/)
  assert.match(viewer, /mode\.value\s*=\s*'logs'/)
  assert.doesNotMatch(viewer, /logs-import-backdrop|logs-import-dialog|<Teleport/)
})

test('Logs candidates expose exact character icons, preview, and deploy as separate actions', () => {
  assert.match(viewer, /characterAssetIcon\(candidate\.characterHash\)/)
  assert.match(viewer, /previewLogsCandidate\(candidate\)/)
  assert.match(viewer, /deployLogsCandidate\(candidate\)/)
  assert.match(viewer, /返回配装预设/)
  assert.match(viewer, /返回 Logs 配装库/)
  assert.match(viewer, /<CapturedLoadoutPreview/)
  assert.match(viewer, /candidate\.protocolLabel/)
  assert.match(viewer, /candidate\.capturedFields/)
  assert.match(viewer, /candidate\.missingFields/)
  assert.match(viewer, /selectedLogsCandidate\.warnings/)
  assert.match(viewer, /Detected Protocol/)
  assert.match(viewer, /Captured Log Fields/)
  assert.match(viewer, /Not captured/)
  assert.doesNotMatch(viewer, /source-label="GBFR Logs v1/)
})

test('Logs library header keeps compact actions and a readable responsive hierarchy', () => {
  assert.match(viewer, /class="logs-library-header ui-card"/)
  assert.match(viewer, /class="back-button logs-back-button ui-btn is-ghost is-sm"/)
  assert.match(viewer, /class="logs-source-button ui-btn is-primary"/)
  assert.match(viewer, /class="logs-library-main"/)
  assert.match(viewer, /class="logs-library-meta"/)
  assert.match(viewer, /class="logs-source-status"/)
  assert.match(viewer, /\.logs-library-main\s*\{[^}]*grid-template-columns:\s*56px minmax\(0,1fr\) auto/is)
  assert.match(viewer, /\.logs-source-button\s*\{[^}]*justify-self:\s*end[^}]*width:\s*max-content/is)
  assert.match(viewer, /@container loadout-viewer \(max-width:560px\)[\s\S]*?\.logs-source-button\s*\{[^}]*width:\s*100%/is)
  assert.doesNotMatch(viewer, /\.subpage-bar > \.back-button[^}]*width:\s*100%/is)
})

test('Logs empty state explains both copied JSON and each supported database location', () => {
  assert.match(viewer, /可以粘贴 Relink Logs 复制的角色 JSON/)
  assert.match(viewer, /数据库在哪里？/)
  assert.match(viewer, /GBFR Logs、Endless、Relink Logs：[\s\S]*?logs\.db/)
  assert.match(viewer, /%APPDATA%\\\\app\.skymeter\.relink/)
  assert.match(viewer, /app\.astralledger\.relink/)
  assert.match(viewer, /不要选择 logs\.db-wal 或 logs\.db-shm/)
  assert.match(viewer, /class="logs-database-location"/)
})

test('Logs deploy without a selected save returns to save selection with the pending import intact', () => {
  assert.match(viewer, /if\s*\(!savePath\.value\s*\|\|\s*!groups\.value\.length\)\s*\{[^}]*mode\.value\s*=\s*'view'/s)
  assert.match(viewer, /已暂存/)
  assert.match(viewer, /请先选择目标存档/)
})

test('background detector shows exact character icons and opens a full preview with a back button', () => {
  assert.match(monitor, /<RuntimeLoadoutDetector/)
  assert.match(detector, /characterAssetIcon\(member\.characterHash\)/)
  assert.match(detector, /openPreview\(record, member\)/)
  assert.match(detector, /closest\('\.tool-center-scroll,\.workspace-scroll'\)\?\.scrollTo\(\{ top: 0 \}\)/)
  assert.match(detector, /<CapturedLoadoutPreview/)
  assert.match(detector, /返回任务记录/)
  assert.match(detector, /titles\[titleKey\(preview\.record, preview\.member\)\]/)
})

test('captured preview uses bounded responsive grids and protects long text from overflow', () => {
  assert.match(preview, /grid-template-columns:\s*repeat\(auto-fit,minmax\(min\(100%,280px\),1fr\)\)/)
  assert.match(preview, /min-width:\s*0/)
  assert.match(preview, /overflow-wrap:\s*anywhere/)
  assert.match(preview, /@container captured-preview \(max-width:720px\)/)
  assert.match(preview, /traitAssetIcon/)
  assert.match(preview, /weaponAssetIcon/)
  assert.match(preview, /summonAssetIcon/)
  assert.match(preview, /角色技能/)
  assert.match(preview, /召唤石/)
  assert.match(preview, /查看全部专精节点/)
  assert.match(preview, /合并技能等级/)
  assert.match(preview, /preview-main-columns/)
  assert.match(preview, /combinedIcon\(skill\)[\s\S]*?v-else aria-hidden="true"/)
  assert.match(preview, /\.combined-row > img,\.combined-row > i/)
})
