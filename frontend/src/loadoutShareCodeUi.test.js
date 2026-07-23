import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const read = path => readFileSync(new URL(path, import.meta.url), 'utf8')
const editor = read('./components/LoadoutEditor.vue')
const dialog = read('./components/LoadoutShareCodeDialog.vue')
const backend = read('../../internal/backend/loadout_share_code.go')
const binding = read('../wailsjs/go/backend/App.js')

test('loadout editor exposes online and offline share workflows without replacing file import and export', () => {
  assert.match(editor, />分享码<\/button>/u)
  assert.match(editor, />导出单套<\/button>/u)
  assert.match(editor, />导入单套<\/button>/u)
  assert.match(editor, /<LoadoutShareCodeDialog[\s\S]*?@generate="generateShareCode"[\s\S]*?@publish="publishShareCode"[\s\S]*?@import="importShareCode"/u)
  assert.match(editor, /compatibleLoadoutShareCode\(rawCode\)/u)
  assert.match(editor, /LoadoutImportShortCode\(props\.savePath, props\.charaHash, rawCode\)/u)
  assert.match(editor, /PublishLoadoutShare\(props\.savePath, selectedLoadout\.value\.unitId\)/u)
  assert.match(editor, /importDraft\.value = draft/u)
})

test('share dialog prioritises readable short codes and keeps long codes collapsed', () => {
  assert.match(dialog, /生成短链接/u)
  assert.match(dialog, /复制短码/u)
  assert.match(dialog, /复制链接/u)
  assert.match(dialog, /<details class="offline-section">/u)
  assert.match(dialog, /较短 Unicode 码/u)
  assert.match(dialog, /纯 ASCII 兼容码/u)
  assert.match(dialog, /读取剪贴板/u)
  assert.match(dialog, /解析并选择导入范围/u)
  assert.match(dialog, /不会直接写入存档/u)
  assert.match(dialog, /catch\s*\{\s*legacyCopy\(\)/u)
  assert.match(dialog, /field\.focus\(\)/u)
})

test('share code backend owns versioned framing compression limits and checksum verification', () => {
  assert.match(backend, /loadoutShareCodeFrameMagic\s*=\s*"GBLC"/u)
  assert.match(backend, /loadoutShareCodeFrameVersion\s*=\s*2/u)
  assert.match(backend, /loadoutShareCodeLegacyFrameVersion\s*=\s*1/u)
  assert.match(backend, /msgpack\.Marshal/u)
  assert.match(backend, /brotli\.BestCompression/u)
  assert.match(backend, /crc32\.ChecksumIEEE/u)
  assert.match(backend, /io\.LimitReader/u)
  assert.match(backend, /func \(a \*App\) LoadoutShareCode/u)
  assert.match(backend, /func \(a \*App\) LoadoutImportCode/u)
  assert.match(read('../../internal/backend/loadout_share_online.go'), /func \(a \*App\) PublishLoadoutShare/u)
  assert.match(read('../../internal/backend/loadout_share_online.go'), /func \(a \*App\) LoadoutImportShortCode/u)
  assert.match(binding, /export function LoadoutShareCode/u)
  assert.match(binding, /export function LoadoutImportCode/u)
  assert.match(binding, /export function PublishLoadoutShare/u)
  assert.match(binding, /export function LoadoutImportShortCode/u)
})
