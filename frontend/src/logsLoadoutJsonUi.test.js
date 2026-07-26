import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')

test('Relink Logs copied JSON has explicit paste, clipboard, file, and preview actions', () => {
  assert.match(viewer, /导入角色 JSON/)
  assert.match(viewer, /粘贴 Relink Logs 复制的角色 JSON/)
  assert.match(viewer, /读取剪贴板/)
  assert.match(viewer, /选择 JSON 文件/)
  assert.match(viewer, /解析并预览/)
  assert.match(viewer, /ParseLogsLoadoutJSON/)
  assert.match(viewer, /SelectLogsLoadoutJSON/)
  assert.match(viewer, /navigator\.clipboard\?\.readText/)
  assert.match(viewer, /logsSourceKind\.value\s*=\s*'json'/)
  assert.match(viewer, /logsJSONPayload\.value\s*=\s*''/)
})

test('JSON import reuses Logs candidate cards, preview, and selective save deployment', () => {
  assert.match(viewer, /logsCandidates\.value\s*=\s*await ParseLogsLoadoutJSON/)
  assert.match(viewer, /previewLogsCandidate\(candidate\)/)
  assert.match(viewer, /deployLogsCandidate\(candidate\)/)
  assert.match(viewer, /<CapturedLoadoutPreview/)
})

test('Logs candidates can be published directly with a reusable title and copied link', () => {
  assert.match(viewer, /PublishLogsLoadoutShare/)
  assert.match(viewer, /openLogsPublish\(candidate\)/)
  assert.match(viewer, /上传分享/)
  assert.match(viewer, /分享标题/)
  assert.match(viewer, /maxlength="80"/)
  assert.match(viewer, /标题可以与其他配装重复/)
  assert.match(viewer, /完全相同的配装会沿用原短码和首次标题/)
  assert.match(viewer, /await PublishLogsLoadoutShare\(logsPublishCandidate\.value, logsPublishTitle\.value\.trim\(\)\)/)
  assert.match(viewer, /await navigator\.clipboard\.writeText\(value\)/)
  assert.match(viewer, /上传并复制链接/)
})

test('JSON dialog has a bounded responsive layout and exact English copy', () => {
  assert.match(viewer, /class="logs-json-backdrop"/)
  assert.match(viewer, /class="logs-json-dialog ui-card"/)
  assert.match(viewer, /class="ui-textarea logs-json-textarea"/)
  assert.match(viewer, /max-width:\s*min\(680px,calc\(100vw - 32px\)\)/)
  assert.match(viewer, /max-height:\s*calc\(100vh - 32px\)/)
  for (const english of [
    'Import Character JSON',
    'Paste character JSON copied from Relink Logs',
    'Read Clipboard',
    'Choose JSON File',
    'Parse & Preview',
    'Publish & Copy Link',
    'Share Title',
    'Titles may be reused.',
    'Waiting for an import source',
    'No External Loadouts Loaded',
    'Paste character JSON copied from Relink Logs or choose a logs.db',
  ]) assert.match(viewer, new RegExp(english.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
})
