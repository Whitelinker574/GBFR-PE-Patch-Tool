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

test('live feature pages do not expose retired source labels', () => {
  const userFacingSources = [
    '../../app.go',
    'components/RuntimePatchFeatures.vue',
    'runtimePatchFeatureView.js',
    'runtimePatchMonitorView.js',
    'runtimePatchTranslations.js',
    'i18n-ui.js',
  ]

  for (const source of userFacingSources) {
    assert.doesNotMatch(read(source), /runtime patch 0\.8\.4/, source)
  }
})
