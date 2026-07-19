import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const trackerUrl = new URL('./sigilLoadoutSelection.js', import.meta.url)
const component = readFileSync(new URL('./components/SigilLoadoutRestore.vue', import.meta.url), 'utf8')

async function loadTracker() {
  assert.ok(existsSync(trackerUrl), 'selection tracker must exist')
  return import(trackerUrl.href)
}

test('recording accepts identical sigil contents when their captured addresses differ', async () => {
  const { createSelectionTracker, takeSelectionAddress } = await loadTracker()
  const tracker = createSelectionTracker()

  assert.equal(takeSelectionAddress(tracker, 0x101000), 0x101000)
  assert.equal(takeSelectionAddress(tracker, 0x101000), 0, 'stable polls must not duplicate the current row')
  assert.equal(takeSelectionAddress(tracker, 0x202000), 0x202000, 'content equality must not hide a distinct row')
})

test('returning to an already handled address does not duplicate the recorded row', async () => {
  const { createSelectionTracker, takeSelectionAddress } = await loadTracker()
  const tracker = createSelectionTracker()

  assert.equal(takeSelectionAddress(tracker, 0x101000), 0x101000)
  assert.equal(takeSelectionAddress(tracker, 0), 0, 'a released pointer is only a selection edge')
  assert.equal(takeSelectionAddress(tracker, 0x101000), 0, 'revisiting the same inventory record must stay deduplicated')
})

test('write progression is driven by captured address edges instead of resulting contents', async () => {
  const { createSelectionTracker, takeSelectionAddress } = await loadTracker()
  const tracker = createSelectionTracker()
  const identicalContents = 'same-sigil:same-traits:same-levels'

  const first = { selectedAddr: 0x101000, contents: identicalContents }
  const second = { selectedAddr: 0x202000, contents: identicalContents }
  assert.equal(takeSelectionAddress(tracker, first.selectedAddr), first.selectedAddr)
  assert.equal(takeSelectionAddress(tracker, 0), 0)
  assert.equal(takeSelectionAddress(tracker, second.selectedAddr), second.selectedAddr)
})

test('component freezes each accepted address and does not use content signatures as capture identity', () => {
  assert.match(component, /takeSelectionAddress/)
  assert.match(component, /expectedSelectedAddr:\s*selectedAddr/)
  assert.doesNotMatch(component, /seen\.has\(currentSignature\)|lastSignature/)
})
