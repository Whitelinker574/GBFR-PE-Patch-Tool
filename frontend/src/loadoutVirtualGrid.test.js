import assert from 'node:assert/strict'
import test from 'node:test'

async function loadVirtualGrid() {
  try {
    return await import('./loadoutVirtualGrid.js')
  } catch {
    return {}
  }
}

test('virtual factor grid derives responsive columns and caps them at six', async () => {
  const { resolveVirtualGridColumns } = await loadVirtualGrid()
  assert.equal(typeof resolveVirtualGridColumns, 'function')
  assert.equal(resolveVirtualGridColumns(300), 1)
  assert.equal(resolveVirtualGridColumns(900), 3)
  assert.equal(resolveVirtualGridColumns(10_000), 6)
})

test('1269 factors render a small overscanned window instead of the full bag', async () => {
  const { resolveVirtualGridWindow } = await loadVirtualGrid()
  assert.equal(typeof resolveVirtualGridWindow, 'function')
  const window = resolveVirtualGridWindow({
    itemCount: 1269,
    viewportWidth: 900,
    viewportHeight: 420,
    scrollTop: 4750,
  })
  assert.equal(window.columns, 3)
  assert.equal(window.startIndex % window.columns, 0)
  assert.ok(window.endIndex - window.startIndex <= 60)
  assert.ok(window.startIndex > 0)
  assert.ok(window.endIndex < 1269)
})

test('six-column 4K viewport never exceeds the 60-card DOM budget', async () => {
  const { resolveVirtualGridWindow } = await loadVirtualGrid()
  const window = resolveVirtualGridWindow({
    itemCount: 1269,
    viewportWidth: 10_000,
    viewportHeight: 560,
    scrollTop: 1000,
  })
  assert.equal(window.columns, 6)
  assert.ok(window.endIndex - window.startIndex <= 60, `rendered ${window.endIndex - window.startIndex} cards`)
})

test('virtual factor grid reaches the final 1269th factor at the bottom', async () => {
  const { resolveVirtualGridWindow } = await loadVirtualGrid()
  assert.equal(typeof resolveVirtualGridWindow, 'function')
  const initial = resolveVirtualGridWindow({
    itemCount: 1269,
    viewportWidth: 900,
    viewportHeight: 420,
    scrollTop: 0,
  })
  const bottom = resolveVirtualGridWindow({
    itemCount: 1269,
    viewportWidth: 900,
    viewportHeight: 420,
    scrollTop: initial.totalHeight,
  })
  assert.equal(bottom.endIndex, 1269)
  assert.ok(bottom.startIndex < bottom.endIndex)
  assert.ok(bottom.endIndex - bottom.startIndex <= 60)
})

test('virtual factor grid clamps an old scroll offset after search narrows results', async () => {
  const { resolveVirtualGridWindow } = await loadVirtualGrid()
  assert.equal(typeof resolveVirtualGridWindow, 'function')
  const narrowed = resolveVirtualGridWindow({
    itemCount: 12,
    viewportWidth: 900,
    viewportHeight: 420,
    scrollTop: 50_000,
  })
  assert.equal(narrowed.startIndex, 0)
  assert.equal(narrowed.endIndex, 12)
  assert.equal(narrowed.totalHeight, 371)
})

test('empty virtual factor results have a zero-sized valid window', async () => {
  const { resolveVirtualGridWindow } = await loadVirtualGrid()
  assert.equal(typeof resolveVirtualGridWindow, 'function')
  assert.deepEqual(resolveVirtualGridWindow({
    itemCount: 0,
    viewportWidth: 900,
    viewportHeight: 420,
    scrollTop: 100,
  }), {
    columns: 3,
    startIndex: 0,
    endIndex: 0,
    offsetTop: 0,
    totalHeight: 0,
  })
})
