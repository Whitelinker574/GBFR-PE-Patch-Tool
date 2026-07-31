import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import {
  createShareImageExportLifecycle,
  ShareImageExportCancelledError,
  withShareImageExportTimeout,
} from './shareImageExportLifecycle.js'

test('share-image export times out when a render stage never settles', async () => {
  const neverSettles = new Promise(() => {})
  let cancelled = 0

  await assert.rejects(
    withShareImageExportTimeout(
      () => neverSettles,
      20,
      () => new Error('share-image total timeout'),
      () => { cancelled += 1 },
    ),
    /share-image total timeout/u,
  )
  assert.equal(cancelled, 1)
})

test('disposing a share-image export prevents late download and clipboard effects', async () => {
  const lifecycle = createShareImageExportLifecycle()
  const generation = lifecycle.begin()
  let releaseRender
  const render = new Promise(resolve => { releaseRender = resolve })
  let downloads = 0
  let clipboardWrites = 0

  const operation = (async () => {
    await render
    lifecycle.assertCurrent(generation)
    downloads += 1
    clipboardWrites += 1
  })()

  lifecycle.dispose()
  releaseRender('data:image/png;base64,late')

  await assert.rejects(operation, ShareImageExportCancelledError)
  assert.equal(downloads, 0)
  assert.equal(clipboardWrites, 0)
})

test('share-image workshop connects the lifecycle guard to both export effects and unmount', () => {
  const source = readFileSync(new URL('./components/LoadoutShareWorkshop.vue', import.meta.url), 'utf8')

  assert.match(source, /renderTask = Promise\.resolve\(\)\.then\(\(\) => renderPNG\(generation\)\)[\s\S]*?withShareImageExportTimeout/u)
  assert.match(source, /timeoutReported = true[\s\S]*?exportLifecycle\.invalidate\(generation\)/u)
  assert.match(source, /finally \{[\s\S]*?if \(renderTask\) await renderTask\.catch\(\(\) => \{\}\)[\s\S]*?exportBusy\.value = false/u)
  assert.match(source, /exportLifecycle\.assertCurrent\(generation\)[\s\S]*?const outputPath = await SaveLoadoutSharePNG\(filename,\s*url\)[\s\S]*?if \(!outputPath\) throw new ShareImageExportCancelledError\(\)/u)
  assert.match(source, /exportLifecycle\.assertCurrent\(generation\)[\s\S]*?navigator\.clipboard\.write/u)
  assert.match(source, /<fieldset class="share-controls ui-card is-flat" :disabled="exportBusy"/u)
  assert.match(source, /onBeforeUnmount\(\(\) => \{[\s\S]*?exportLifecycle\.dispose\(\)[\s\S]*?exportBusy\.value = false/u)
})
