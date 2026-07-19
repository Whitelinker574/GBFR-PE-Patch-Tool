import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/SigilMemoryGenerator.vue', import.meta.url), 'utf8')

test('live sigil editor canonicalizes the game empty hash at its UI boundary', () => {
  assert.match(source, /const EMPTY_HASH\s*=\s*0x887AE0B0/)
  assert.match(source, /function isEmptyTraitHash\(/)
  assert.match(source, /function normaliseSecondaryHash\(/)
  assert.match(source, /secondaryTraitHash:\s*normaliseSecondaryHash\(next\.secondaryTraitHash\)/)
  assert.match(source, /secondaryTraitLevel:\s*isEmptyTraitHash\(next\.secondaryTraitHash\)\s*\?\s*0/)
  assert.match(source, /form\.secondaryTraitHash\s*=\s*normaliseSecondaryHash\(entry\.secondaryTraitHash\)/)
})
