import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const root = path.dirname(fileURLToPath(import.meta.url))
const read = relativePath => fs.readFileSync(path.join(root, relativePath), 'utf8')

test('retired mission and commendation EXE editors are absent from the application', () => {
  const shell = read('components/PatchTool.vue')
  const backend = read('../../app.go')

  for (const retiredText of ['挑战次数', '点赞数值']) {
    assert.doesNotMatch(shell, new RegExp(retiredText))
    assert.doesNotMatch(backend, new RegExp(`Name:\\s+"${retiredText}"`))
  }
  assert.doesNotMatch(shell, /\bPatchFile\b/)
  assert.doesNotMatch(backend, /func \(a \*App\) PatchFile\b/)
})

test('live feature pages do not expose the historical CT 0.8.4 source label', () => {
  const userFacingSources = [
    '../../app.go',
    'components/CT084Features.vue',
    'ct084FeatureView.js',
    'ct084RuntimeMonitorView.js',
    'ct084Translations.js',
    'i18n-ui.js',
  ]

  for (const source of userFacingSources) {
    assert.doesNotMatch(read(source), /CT 0\.8\.4/, source)
  }
})
